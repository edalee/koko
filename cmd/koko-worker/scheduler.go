package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

const (
	StatusOK      = "ok"
	StatusFailed  = "failed"
	StatusOffline = "offline" // a network error: retried until the internet is back
	StatusRetry   = "retry"   // any other failure, with tries left
	retryInterval = 5 * time.Minute
	// maxFailures is how many non-network failures a slot gets before it is
	// final. Network errors do not count. A reported error is final at once.
	maxFailures = 3
	wakeLead    = 2 * time.Minute
)

// RunRecord is the outcome of one scheduled slot, such as tono at 12:00 today.
type RunRecord struct {
	Job      string    `json:"job"`
	Slot     time.Time `json:"slot"`
	Status   string    `json:"status"`
	At       time.Time `json:"at"`
	Message  string    `json:"message,omitempty"`
	Attempts int       `json:"attempts"` // every run of the slot
	Failures int       `json:"failures"` // runs that failed for a reason other than the network
	NextTry  time.Time `json:"nextTry,omitempty"`
}

// State is kept on disk, because launchd restarts the worker and memory is lost.
type State struct {
	Runs         map[string]RunRecord  `json:"runs"`         // key: slotKey
	TonoReviewed map[string]time.Time  `json:"tonoReviewed"` // key: "owner/repo#12@sha"
	TonoResults  map[string]TonoResult `json:"tonoResults"`  // key: "owner/repo#12@sha"
	// TimesAdded is when each active run time first appeared, keyed "job@HH:MM".
	// A slot counts only on days when its time was active before the slot came.
	TimesAdded map[string]time.Time `json:"timesAdded"`
	WakeBooked time.Time            `json:"wakeBooked,omitempty"` // only set when pmset succeeded
	WakeTried  time.Time            `json:"wakeTried,omitempty"`  // last wake attempted, booked or not
}

func slotKey(job string, slot time.Time) string {
	return job + "@" + slot.Format("2006-01-02 15:04")
}

func loadState(path string) State {
	st := State{Runs: map[string]RunRecord{}, TonoReviewed: map[string]time.Time{}}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &st)
	}
	if st.Runs == nil {
		st.Runs = map[string]RunRecord{}
	}
	if st.TonoReviewed == nil {
		st.TonoReviewed = map[string]time.Time{}
	}
	if st.TimesAdded == nil {
		st.TimesAdded = map[string]time.Time{}
	}
	if st.TonoResults == nil {
		st.TonoResults = map[string]TonoResult{}
	}
	return st
}

// TonoResult is tono's outcome for one commit of one PR.
type TonoResult struct {
	URL      string            `json:"url"`
	Title    string            `json:"title"`
	Repo     string            `json:"repo"`
	Number   int               `json:"number"`
	SHA      string            `json:"sha"`
	Mine     bool              `json:"mine"`
	At       time.Time         `json:"at"`
	Verdicts map[string]string `json:"verdicts,omitempty"` // pass -> verdictReady, verdictFollowUps or verdictNotMergeable
	Failed   string            `json:"failed,omitempty"`
	Comments []string          `json:"comments,omitempty"` // URLs of the PR comments posted
	// Unposted are comments not yet posted. The next tono run posts them.
	Unposted []string `json:"unposted,omitempty"`
	// PostTries counts failed attempts to post. At maxPostTries the review
	// counts as failed, and its unposted comments are dropped.
	PostTries int `json:"postTries,omitempty"`
	// Report and Posted belong to the old stand-up thread. Pruning still
	// removes report files that old results point to.
	Report string `json:"report,omitempty"`
	Posted bool   `json:"posted,omitempty"`
}

func timeKey(job, clock string) string { return job + "@" + clock }

// syncTimes records when each active run time appeared, and forgets times
// that are no longer active. A time is active if the worker and its job are
// switched on. So a time moved to earlier than now, or a job switched on
// after its time, waits for the next day instead of running at once.
func syncTimes(cfg Config, st *State, now time.Time) {
	active := map[string]bool{}
	if cfg.Enabled {
		for _, job := range allJobs {
			jc := cfg.Jobs[job]
			if !jc.Enabled {
				continue
			}
			for _, clock := range jc.Times {
				active[timeKey(job, clock)] = true
			}
		}
	}
	for k := range active {
		if _, ok := st.TimesAdded[k]; !ok {
			st.TimesAdded[k] = now
		}
	}
	for k := range st.TimesAdded {
		if !active[k] {
			delete(st.TimesAdded, k)
		}
	}
}

func saveState(path string, st State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// lockFile takes an exclusive lock on path, waiting if another process holds
// it. Call the returned function to release it.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// lockFree is true if nobody holds the lock at path right now.
func lockFree(path string) bool {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return true
	}
	defer func() { _ = f.Close() }()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return false
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return true
}

// updateState changes the state on disk under a file lock. It reloads first,
// so two processes (the scheduler and a "Run now") never overwrite each
// other's changes with a stale copy.
func updateState(path string, change func(*State)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	unlock, err := lockFile(path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	st := loadState(path)
	change(&st)
	return saveState(path, st)
}

// pruneState drops run records older than a week, and tono review marks and
// results older than 60 days, so the file stays small.
func pruneState(st *State, now time.Time) {
	for k, r := range st.Runs {
		if now.Sub(r.Slot) > 7*24*time.Hour {
			delete(st.Runs, k)
		}
	}
	for k, t := range st.TonoReviewed {
		if now.Sub(t) > 60*24*time.Hour {
			delete(st.TonoReviewed, k)
		}
	}
	for k, r := range st.TonoResults {
		if now.Sub(r.At) > 60*24*time.Hour {
			if r.Report != "" {
				_ = os.Remove(r.Report)
			}
			delete(st.TonoResults, k)
		}
	}
}

// DueRun is one job to run now. Slots lists every slot it covers: after a
// long sleep, tono may have missed 09:30 and 12:00, and one run covers both.
type DueRun struct {
	Job   string
	Slots []time.Time
}

// dueRuns lists the jobs to run at now. Only today's slots count: a slot
// missed on an earlier day is dropped, not caught up. A slot whose time was
// set after it came round waits for the next day (see syncTimes).
func dueRuns(cfg Config, st State, now time.Time) []DueRun {
	now = now.In(cfg.location())
	if !cfg.Enabled || !isWeekday(now) {
		return nil
	}
	var out []DueRun
	for _, job := range allJobs {
		jc, ok := cfg.Jobs[job]
		if !ok || !jc.Enabled {
			continue
		}
		var slots []time.Time
		for _, clock := range jc.Times {
			slot := atClock(now, clock)
			if now.Before(slot) {
				continue
			}
			if added, ok := st.TimesAdded[timeKey(job, clock)]; !ok || !added.Before(slot) {
				continue // the time was set after this slot came round
			}
			rec, seen := st.Runs[slotKey(job, slot)]
			retry := rec.Status == StatusRetry || rec.Status == StatusOffline
			if !seen || (retry && !now.Before(rec.NextTry)) {
				slots = append(slots, slot)
			}
		}
		if len(slots) > 0 {
			sort.Slice(slots, func(i, j int) bool { return slots[i].Before(slots[j]) })
			out = append(out, DueRun{Job: job, Slots: slots})
		}
	}
	return out
}

// recordRun stores the outcome for every slot the run covered.
func recordRun(st *State, run DueRun, status, message string, now time.Time) {
	for _, slot := range run.Slots {
		key := slotKey(run.Job, slot)
		rec := st.Runs[key]
		rec.Job, rec.Slot, rec.Status, rec.At, rec.Message = run.Job, slot, status, now, message
		rec.Attempts++
		if status == StatusRetry || status == StatusFailed {
			rec.Failures++
		}
		rec.NextTry = time.Time{}
		if status == StatusRetry || status == StatusOffline {
			rec.NextTry = now.Add(retryInterval)
		}
		st.Runs[key] = rec
	}
}

// manualSlots is what a real "Run now" of job covers, so the scheduler does
// not repeat it. The stand-up is once a day, so it covers all of today's
// slots. Other jobs cover the slots that are already due.
func manualSlots(cfg Config, job string, now time.Time) []time.Time {
	now = now.In(cfg.location())
	var out []time.Time
	for _, clock := range cfg.Jobs[job].Times {
		slot := atClock(now, clock)
		if job == JobStandup || !now.Before(slot) {
			out = append(out, slot)
		}
	}
	return out
}

// failures is the highest non-network failure count across the run's slots.
func failures(st State, run DueRun) int {
	n := 0
	for _, slot := range run.Slots {
		if a := st.Runs[slotKey(run.Job, slot)].Failures; a > n {
			n = a
		}
	}
	return n
}

// nextSlot is the first enabled slot after now, within the next 8 days.
func nextSlot(cfg Config, now time.Time) (time.Time, bool) {
	now = now.In(cfg.location())
	if !cfg.Enabled {
		return time.Time{}, false
	}
	var best time.Time
	for d := 0; d <= 8; d++ {
		day := now.AddDate(0, 0, d)
		if !isWeekday(day) {
			continue
		}
		for _, job := range allJobs {
			jc := cfg.Jobs[job]
			if !jc.Enabled {
				continue
			}
			for _, clock := range jc.Times {
				slot := atClock(day, clock)
				if slot.After(now) && (best.IsZero() || slot.Before(best)) {
					best = slot
				}
			}
		}
		if !best.IsZero() {
			return best, true
		}
	}
	return time.Time{}, false
}

// nextWake is when the Mac should wake for the next slot.
func nextWake(cfg Config, now time.Time) (time.Time, bool) {
	slot, ok := nextSlot(cfg, now)
	if !ok {
		return time.Time{}, false
	}
	return slot.Add(-wakeLead), true
}
