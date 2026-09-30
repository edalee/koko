package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var stockholm, _ = time.LoadLocation("Europe/Stockholm")

// Monday 28 September 2026.
func monday(hour, minute int) time.Time {
	return time.Date(2026, 9, 28, hour, minute, 0, 0, stockholm)
}

var focusCfg = FocusConfig{WindowStart: "09:00", WindowEnd: "17:00", MinMinutes: 30}

type evOpt func(*GCalEvent)

func ev(start, end string, opts ...evOpt) GCalEvent {
	e := GCalEvent{
		Summary: "Meeting", Status: "confirmed", EventType: "DEFAULT",
		Start: gcalTime{DateTime: "2026-09-28T" + start + ":00+02:00"},
		End:   gcalTime{DateTime: "2026-09-28T" + end + ":00+02:00"},
	}
	for _, o := range opts {
		o(&e)
	}
	return e
}

func response(status string) evOpt {
	return func(e *GCalEvent) {
		e.Attendees = append(e.Attendees, struct {
			Self           bool   `json:"self"`
			ResponseStatus string `json:"responseStatus"`
		}{Self: true, ResponseStatus: status})
	}
}

func free(e *GCalEvent) { e.Transparency = "transparent" }

func title(t string) evOpt { return func(e *GCalEvent) { e.Summary = t } }

func allDay(e *GCalEvent) {
	e.Start = gcalTime{Date: "2026-09-28"}
	e.End = gcalTime{Date: "2026-09-29"}
}

func gapStrings(gaps []Gap) string {
	var parts []string
	for _, g := range gaps {
		parts = append(parts, g.Start.Format("15:04")+"-"+g.End.Format("15:04"))
	}
	return strings.Join(parts, " ")
}

func TestFindFocusGaps(t *testing.T) {
	tests := []struct {
		name   string
		events []GCalEvent
		now    time.Time
		want   string
	}{
		{"empty day is one block", nil, monday(7, 0), "09:00-17:00"},
		{"fills between meetings", []GCalEvent{ev("09:30", "10:00"), ev("12:00", "13:00")}, monday(7, 0),
			"09:00-09:30 10:00-12:00 13:00-17:00"},
		{"drops gaps under 30 minutes", []GCalEvent{ev("09:20", "10:00"), ev("10:25", "17:00")}, monday(7, 0), ""},
		{"overlapping meetings", []GCalEvent{ev("10:00", "11:30"), ev("10:30", "11:00"), ev("11:15", "12:00")}, monday(7, 0),
			"09:00-10:00 12:00-17:00"},
		{"meetings outside the window", []GCalEvent{ev("07:00", "08:00"), ev("17:00", "18:00")}, monday(7, 0), "09:00-17:00"},
		{"meeting across the window start", []GCalEvent{ev("08:30", "09:45")}, monday(7, 0), "09:45-17:00"},
		{"declined does not block", []GCalEvent{ev("10:00", "11:00", response("declined"))}, monday(7, 0), "09:00-17:00"},
		{"free does not block", []GCalEvent{ev("13:00", "14:00", free)}, monday(7, 0), "09:00-17:00"},
		{"tentative blocks", []GCalEvent{ev("10:00", "11:00", response("tentative"))}, monday(7, 0), "09:00-10:00 11:00-17:00"},
		{"unanswered blocks", []GCalEvent{ev("10:00", "11:00", response("needsAction"))}, monday(7, 0), "09:00-10:00 11:00-17:00"},
		{"cancelled does not block", []GCalEvent{ev("10:00", "11:00", func(e *GCalEvent) { e.Status = "cancelled" })}, monday(7, 0), "09:00-17:00"},
		{"second run books nothing", []GCalEvent{ev("09:15", "17:00", func(e *GCalEvent) { e.EventType = "FOCUS_TIME" })}, monday(9, 15), ""},
		{"run at 09:15 starts at 09:15", nil, monday(9, 15), "09:15-17:00"},
		{"late run rounds up to 5 minutes", nil, monday(11, 2), "11:05-17:00"},
		{"after the window nothing", nil, monday(17, 10), ""},
		{"weekend nothing", nil, time.Date(2026, 9, 26, 7, 0, 0, 0, stockholm), ""},
		{"all-day time off blocks the day", []GCalEvent{ev("", "", allDay, free, title("OOO"))}, monday(7, 0), ""},
		{"all-day busy blocks the day", []GCalEvent{ev("", "", allDay)}, monday(7, 0), ""},
		{"all-day free event does not block", []GCalEvent{ev("", "", allDay, free, title("Offsite week"))}, monday(7, 0), "09:00-17:00"},
		{"out-of-office blocks its time", []GCalEvent{ev("13:00", "17:00", func(e *GCalEvent) { e.EventType = "OUT_OF_OFFICE" })}, monday(7, 0), "09:00-13:00"},
		{"working location ignored", []GCalEvent{ev("09:00", "17:00", func(e *GCalEvent) { e.EventType = "WORKING_LOCATION" })}, monday(7, 0), "09:00-17:00"},
		{"exactly 30 minutes kept", []GCalEvent{ev("09:30", "17:00")}, monday(7, 0), "09:00-09:30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gapStrings(findFocusGaps(tt.events, focusCfg, tt.now)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEventTimesKeepOtherZones(t *testing.T) {
	// Google returns some events in their organiser's zone. 11:00 Berlin is 11:00 Stockholm.
	e := GCalEvent{Start: gcalTime{DateTime: "2026-09-28T11:00:00+02:00"}, End: gcalTime{DateTime: "2026-09-28T11:30:00+02:00"}}
	s, _, err := e.times(stockholm)
	if err != nil || s.Format("15:04") != "11:00" {
		t.Fatalf("got %v %v", s, err)
	}
}

func testConfig() Config {
	cfg := defaultConfig()
	cfg.Enabled = true
	return cfg
}

// readyState is a state in which every time in cfg was set a week earlier.
func readyState(cfg Config) State {
	st := loadState("/nonexistent")
	syncTimes(cfg, &st, monday(0, 0).AddDate(0, 0, -7))
	return st
}

func dueString(runs []DueRun) string {
	var parts []string
	for _, r := range runs {
		var slots []string
		for _, s := range r.Slots {
			slots = append(slots, s.Format("15:04"))
		}
		parts = append(parts, r.Job+"["+strings.Join(slots, ",")+"]")
	}
	return strings.Join(parts, " ")
}

func TestDueRuns(t *testing.T) {
	cfg := testConfig()
	empty := readyState(cfg)

	t.Run("nothing before the first slot", func(t *testing.T) {
		if got := dueString(dueRuns(cfg, empty, monday(6, 59))); got != "" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("stand-up at 07:00", func(t *testing.T) {
		if got := dueString(dueRuns(cfg, empty, monday(7, 0))); got != "standup[07:00]" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("lid opened at 13:00 catches up once per job", func(t *testing.T) {
		got := dueString(dueRuns(cfg, empty, monday(13, 0)))
		if got != "standup[07:00] focus[09:15] tono[09:30,12:00]" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("done slots are not rerun", func(t *testing.T) {
		st := readyState(cfg)
		recordRun(&st, DueRun{Job: JobStandup, Slots: []time.Time{monday(7, 0)}}, StatusOK, "", monday(7, 1))
		recordRun(&st, DueRun{Job: JobFocus, Slots: []time.Time{monday(9, 15)}}, StatusFailed, "boom", monday(9, 16))
		if got := dueString(dueRuns(cfg, st, monday(9, 20))); got != "" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("retry waits five minutes", func(t *testing.T) {
		st := readyState(cfg)
		recordRun(&st, DueRun{Job: JobStandup, Slots: []time.Time{monday(7, 0)}}, StatusOffline, "offline", monday(7, 1))
		if got := dueString(dueRuns(cfg, st, monday(7, 5))); got != "" {
			t.Errorf("too early: got %q", got)
		}
		if got := dueString(dueRuns(cfg, st, monday(7, 6))); got != "standup[07:00]" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("weekend nothing", func(t *testing.T) {
		if got := dueRuns(cfg, empty, time.Date(2026, 9, 26, 13, 0, 0, 0, stockholm)); got != nil {
			t.Errorf("got %v", got)
		}
	})
	t.Run("switched off nothing", func(t *testing.T) {
		off := testConfig()
		off.Enabled = false
		if got := dueRuns(off, empty, monday(13, 0)); got != nil {
			t.Errorf("got %v", got)
		}
	})
	t.Run("job switched off", func(t *testing.T) {
		c := testConfig()
		c.Jobs[JobTono] = JobConfig{Enabled: false, Times: []string{"09:30"}}
		st := readyState(c)
		if got := dueString(dueRuns(c, st, monday(9, 40))); got != "standup[07:00] focus[09:15]" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("a missed day is not caught up", func(t *testing.T) {
		tuesday := monday(6, 0).AddDate(0, 0, 1)
		if got := dueString(dueRuns(cfg, empty, tuesday)); got != "" {
			t.Errorf("got %q", got)
		}
	})
}

func TestNextWake(t *testing.T) {
	cfg := testConfig()
	tests := []struct {
		now  time.Time
		want string
	}{
		{monday(6, 0), "Mon 06:58"},
		{monday(7, 0), "Mon 09:13"},
		{monday(9, 20), "Mon 09:28"},
		{monday(9, 30), "Mon 11:58"},
		{monday(14, 0), "Tue 06:58"},
		{time.Date(2026, 10, 2, 15, 0, 0, 0, stockholm), "Mon 06:58"}, // Friday afternoon
	}
	for _, tt := range tests {
		w, ok := nextWake(cfg, tt.now)
		if got := w.Format("Mon 15:04"); !ok || got != tt.want {
			t.Errorf("at %s: got %s, want %s", tt.now.Format("Mon 15:04"), got, tt.want)
		}
	}
	off := testConfig()
	off.Enabled = false
	if _, ok := nextWake(off, monday(6, 0)); ok {
		t.Error("switched off: want no wake")
	}
}

func TestBlockers(t *testing.T) {
	ready := PRDetail{ReviewDecision: "APPROVED", Mergeable: "MERGEABLE", MergeStateStatus: "CLEAN"}
	if b := ready.blockers(); len(b) != 0 {
		t.Errorf("ready PR has blockers: %v", b)
	}
	d := ready
	d.Checks = append(d.Checks,
		Check{Name: "test", Status: "COMPLETED", Conclusion: "FAILURE"},
		Check{Name: "lint", Status: "IN_PROGRESS"},
		Check{Name: "optional", Status: "COMPLETED", Conclusion: "SKIPPED"},
	)
	d.Mergeable = "CONFLICTING"
	got := strings.Join(d.blockers(), "; ")
	want := "merge conflicts; failing checks: test; checks still running: lint"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReviewQueue(t *testing.T) {
	pr := func(login, typ, created string) SearchPR {
		p := SearchPR{CreatedAt: created}
		p.Author.Login, p.Author.Type = login, typ
		return p
	}
	prs := []SearchPR{
		pr("dependabot[bot]", "Bot", "2026-01-01"),
		pr("jamie", "User", "2026-09-20"),
		pr("renovate", "User", "2026-01-02"),
		pr("edalee", "User", "2026-01-03"),
		pr("lam", "User", "2026-09-01"),
	}
	var got []string
	for _, p := range reviewQueue(prs, "edalee") {
		got = append(got, p.Author.Login)
	}
	if strings.Join(got, ",") != "lam,jamie" {
		t.Errorf("got %v", got)
	}
}

func TestParseStream(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"` + toolListEvents + `","input":{"calendarId":"primary"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"{\"events\":[{\"id\":\"a\",\"summary\":\"Standup\"}]}"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t2","name":"` + toolCreateEvent + `","input":{}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t2","is_error":true,"content":[{"type":"text","text":"Insufficient scope"}]}]}}`,
		`{"type":"result","result":"DONE","is_error":false}`,
	}, "\n")
	res, err := parseStream([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "DONE" || len(res.Calls) != 2 {
		t.Fatalf("got %+v", res)
	}
	if !strings.Contains(res.callsTo(toolListEvents)[0].Result, "Standup") {
		t.Error("list_events result lost")
	}
	create := res.callsTo(toolCreateEvent)[0]
	if !create.IsError || create.Result != "Insufficient scope" {
		t.Errorf("create result: %+v", create)
	}
	if _, err := parseStream([]byte(`{"type":"assistant"}`)); err == nil {
		t.Error("want an error when there is no result")
	}
}

func TestExtractJSON(t *testing.T) {
	for in, want := range map[string]string{
		"```json\n[{\"a\":1}]\n```": `[{"a":1}]`,
		"Here you go: [1,2] done":   `[1,2]`,
		`{"k":"v"}`:                 `{"k":"v"}`,
		"no json here":              "",
	} {
		if got := extractJSON(in); got != want {
			t.Errorf("extractJSON(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMeetings(t *testing.T) {
	events := []GCalEvent{
		ev("09:30", "09:45", title("Standup"), response("accepted")),
		ev("10:00", "11:00", title("Skipped"), response("declined")),
		ev("11:00", "12:00", title("Focus"), func(e *GCalEvent) { e.EventType = "FOCUS_TIME" }),
		ev("13:00", "14:00", title("Planning"), response("needsAction")),
		ev("14:00", "15:00", title("Focus"), func(e *GCalEvent) { e.Description = focusMarker }),
		ev("00:00", "00:00", title("Time report"), response("needsAction"), func(e *GCalEvent) {
			e.End.DateTime = "2026-09-29T00:00:00+02:00"
		}),
		ev("00:00", "00:00", title("Holiday"), func(e *GCalEvent) {
			e.Start = gcalTime{Date: "2026-09-28"}
			e.End = gcalTime{Date: "2026-09-29"}
		}),
	}
	got := strings.Join(meetings(events, stockholm), " | ")
	want := "09:30–09:45 Standup | 13:00–14:00 Planning _(not answered)_ | " +
		"All day: Time report _(not answered)_ | All day: Holiday"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := defaultConfig()
	if err := cfg.validate(); err != nil {
		t.Fatalf("defaults invalid: %v", err)
	}
	cfg.Jobs[JobTono] = JobConfig{Enabled: true, Times: []string{"9.30"}}
	if err := cfg.validate(); err == nil {
		t.Error("want an error for a bad time")
	}
}

func TestSudoersRuleIsNarrow(t *testing.T) {
	rule := sudoersRule()
	if strings.Contains(rule, "NOPASSWD: ALL") || strings.Count(rule, "/usr/bin/pmset schedule") != 2 {
		t.Errorf("rule too wide: %s", rule)
	}
}

func TestTonoLockKey(t *testing.T) {
	if got := tonoLockKey("epidemicsound/kalimba", 28); got != "epidemicsound_kalimba_28" {
		t.Errorf("got %q", got)
	}
	if got := tonoLockKey("epidemicsound/youtube-data-service", 5); got != "epidemicsound_youtube_data_service_5" {
		t.Errorf("got %q", got)
	}
}

func TestManualSlots(t *testing.T) {
	cfg := testConfig()
	// A stand-up run by hand at 06:55 covers the 07:00 stand-up.
	if got := manualSlots(cfg, JobStandup, monday(6, 55)); len(got) != 1 || got[0].Format("15:04") != "07:00" {
		t.Errorf("stand-up: got %v", got)
	}
	// Tono run by hand at 12:05 covers 09:30 and 12:00, not 14:00.
	got := manualSlots(cfg, JobTono, monday(12, 5))
	if len(got) != 2 || got[1].Format("15:04") != "12:00" {
		t.Errorf("tono: got %v", got)
	}
	st := readyState(cfg)
	recordRun(&st, DueRun{Job: JobStandup, Slots: manualSlots(cfg, JobStandup, monday(6, 55))}, StatusOK, "run by hand", monday(6, 56))
	if d := dueString(dueRuns(cfg, st, monday(7, 0))); d != "" {
		t.Errorf("stand-up run by hand is due again: %q", d)
	}
}

func TestFailuresSkipNetworkRetries(t *testing.T) {
	cfg := testConfig()
	st := readyState(cfg)
	run := DueRun{Job: JobStandup, Slots: []time.Time{monday(7, 0)}}
	recordRun(&st, run, StatusOffline, "no such host", monday(7, 1))
	recordRun(&st, run, StatusOffline, "no such host", monday(7, 6))
	if got := failures(st, run); got != 0 {
		t.Fatalf("network retries counted as failures: %d", got)
	}
	if d := dueString(dueRuns(cfg, st, monday(7, 11))); d != "standup[07:00]" {
		t.Errorf("offline slot not retried: %q", d)
	}
	recordRun(&st, run, StatusRetry, "boom", monday(7, 11))
	recordRun(&st, run, StatusRetry, "boom", monday(7, 16))
	if got := failures(st, run); got != 2 {
		t.Fatalf("failures = %d, want 2", got)
	}
	if got := st.Runs[slotKey(JobStandup, monday(7, 0))].Attempts; got != 4 {
		t.Errorf("attempts = %d, want 4", got)
	}
}

func TestUpdateStateKeepsConcurrentChanges(t *testing.T) {
	path := t.TempDir() + "/state.json"
	done := make(chan error)
	for i := 0; i < 20; i++ {
		go func(i int) {
			done <- updateState(path, func(st *State) {
				st.TonoReviewed[strings.Repeat("x", i+1)] = monday(9, 0)
			})
		}(i)
	}
	for i := 0; i < 20; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if got := len(loadState(path).TonoReviewed); got != 20 {
		t.Errorf("lost updates: %d of 20 kept", got)
	}
}

func TestLoadConfigKeepsDefaults(t *testing.T) {
	path := t.TempDir() + "/worker.json"
	// What the Koko UI sends when you only fill in Slack, plus a hand-edited job.
	body := `{"enabled": true, "tonoPath": "", "timeZone": "", "focus": {"windowStart": ""},
		"slack": {"botToken": "x", "userId": "U1"},
		"jobs": {"tono": {"enabled": true}, "focus": {"enabled": false, "times": [""]}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	def := defaultConfig()
	if cfg.TonoPath != def.TonoPath || cfg.TimeZone != def.TimeZone || cfg.Focus.WindowStart != "09:00" {
		t.Errorf("empty values not defaulted: %+v", cfg)
	}
	if got := strings.Join(cfg.Jobs[JobTono].Times, ","); got != "09:30,12:00,14:00" {
		t.Errorf("tono times = %q", got)
	}
	if cfg.Jobs[JobFocus].Enabled || strings.Join(cfg.Jobs[JobFocus].Times, ",") != "09:15" {
		t.Errorf("focus = %+v", cfg.Jobs[JobFocus])
	}
	if !cfg.Jobs[JobStandup].Enabled {
		t.Error("missing stand-up entry not defaulted")
	}
}

func TestWorkerPATH(t *testing.T) {
	got := workerPATH("/usr/bin:/custom/bin")
	if !strings.HasPrefix(got, agentPATH()) || !strings.HasSuffix(got, ":/custom/bin") || strings.Count(got, "/usr/bin:") != 1 {
		t.Errorf("got %q", got)
	}
}

// A timed-out tono must run its trap, which removes its lock. A plain kill
// would leave the lock and block every later review of that PR.
func TestGroupCommandRunsTrapOnTimeout(t *testing.T) {
	dir := t.TempDir()
	lock := dir + "/review.lock"
	script := dir + "/fake-tono"
	body := "#!/bin/bash\nmkdir " + lock + "\ntrap 'rmdir " + lock + "' EXIT INT TERM\nsleep 30 &\nwait\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	_ = groupCommand(ctx, script).Run()
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("took %s to stop", took)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Error("the trap did not run: the lock is still there")
	}
}

func TestTimeMovedEarlierWaitsForTomorrow(t *testing.T) {
	cfg := testConfig()
	st := readyState(cfg)
	recordRun(&st, DueRun{Job: JobStandup, Slots: []time.Time{monday(7, 0)}}, StatusOK, "", monday(7, 1))

	// At 09:00 you move the stand-up from 07:00 to 08:00.
	cfg.Jobs[JobStandup] = JobConfig{Enabled: true, Times: []string{"08:00"}}
	syncTimes(cfg, &st, monday(9, 0))
	if got := dueString(dueRuns(cfg, st, monday(9, 1))); got != "" {
		t.Errorf("ran again after the time moved: %q", got)
	}
	tuesday := monday(8, 0).AddDate(0, 0, 1)
	if got := dueString(dueRuns(cfg, st, tuesday)); got != "standup[08:00]" {
		t.Errorf("tomorrow: got %q", got)
	}
}

func TestJobSwitchedOnLateWaitsForTomorrow(t *testing.T) {
	cfg := testConfig()
	cfg.Jobs[JobTono] = JobConfig{Enabled: false, Times: []string{"09:30"}}
	st := readyState(cfg)
	cfg.Jobs[JobTono] = JobConfig{Enabled: true, Times: []string{"09:30"}}
	syncTimes(cfg, &st, monday(10, 0))
	if got := dueString(dueRuns(cfg, st, monday(10, 0))); strings.Contains(got, "tono") {
		t.Errorf("tono ran straight after being switched on: %q", got)
	}
}

func TestCatchUpAfterSleepStillWorks(t *testing.T) {
	// Times set last week, the Mac asleep all morning: the stand-up still runs at 13:00.
	cfg := testConfig()
	st := readyState(cfg)
	syncTimes(cfg, &st, monday(13, 0))
	if got := dueString(dueRuns(cfg, st, monday(13, 0))); !strings.HasPrefix(got, "standup[07:00]") {
		t.Errorf("got %q", got)
	}
}

func TestReadOnlyShims(t *testing.T) {
	dir := t.TempDir()
	realDir := dir + "/real"
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Fake gh and git that only say they ran.
	for _, name := range []string{"gh", "git"} {
		if err := os.WriteFile(realDir+"/"+name, []byte("#!/bin/bash\necho ran\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", realDir+":/usr/bin:/bin")
	shims, err := readOnlyShims(dir + "/shims")
	if err != nil {
		t.Fatal(err)
	}
	run := func(name string, args ...string) bool {
		out, err := exec.Command(shims+"/"+name, args...).CombinedOutput()
		return err == nil && strings.TrimSpace(string(out)) == "ran"
	}
	allowed := [][]string{
		{"gh", "pr", "view", "28"}, {"gh", "pr", "diff", "28"}, {"gh", "pr", "checkout", "28"},
		{"gh", "api", "repos/o/r/pulls/28/comments", "--jq", ".[].body"}, {"gh", "repo", "clone", "o/r"},
		{"gh", "run", "view", "123"}, {"gh", "auth", "status"},
		{"git", "log", "--oneline"}, {"git", "-C", ".", "diff", "main"}, {"git", "-c", "x=y", "show"},
	}
	refused := [][]string{
		{"gh", "pr", "comment", "28", "-b", "x"}, {"gh", "pr", "review", "28", "--approve"}, {"gh", "pr", "merge", "28"},
		{"gh", "api", "-X", "PATCH", "repos/o/r/issues/comments/1"}, {"gh", "api", "--method=POST", "x"},
		{"gh", "api", "repos/o/r/issues/28/comments", "-f", "body=x"}, {"gh", "api", "x", "--input", "f"},
		{"gh", "workflow", "run", "deploy"}, {"gh", "repo", "delete", "o/r"}, {"gh", "auth", "token"},
		{"gh", "secret", "list"}, {"gh", "issue", "create"},
		{"git", "push"}, {"git", "-C", ".", "push", "origin"}, {"git", "-c", "a=b", "push"},
	}
	for _, a := range allowed {
		if !run(a[0], a[1:]...) {
			t.Errorf("refused, want allowed: %v", a)
		}
	}
	for _, r := range refused {
		if run(r[0], r[1:]...) {
			t.Errorf("allowed, want refused: %v", r)
		}
	}
}

func TestBuildBlocks(t *testing.T) {
	long := strings.Repeat("a line of text for slack\n", 300) // about 7,500 characters
	msgs := buildBlocks([]string{"*Header*", "*Today*\n• No meetings", long})
	if len(msgs) != 1 {
		t.Fatalf("got %d messages", len(msgs))
	}
	var kinds []string
	for _, b := range msgs[0] {
		kinds = append(kinds, b["type"].(string))
		if b["type"] == "section" {
			if n := len(b["text"].(block)["text"].(string)); n > 3000 {
				t.Errorf("section of %d characters, over Slack's limit", n)
			}
		}
	}
	// header, divider, today, divider, then the long section in three parts.
	if got := strings.Join(kinds, ","); got != "section,divider,section,divider,section,section,section" {
		t.Errorf("got %s", got)
	}

	var many []string
	for i := 0; i < 40; i++ {
		many = append(many, "section")
	}
	msgs = buildBlocks(many) // 40 sections and 39 dividers: over 50 blocks
	if len(msgs) != 2 || msgs[1][0]["type"] != "section" {
		t.Fatalf("want 2 messages, the second starting with a section: got %d", len(msgs))
	}
	for _, m := range msgs {
		if len(m) > 50 {
			t.Errorf("message with %d blocks", len(m))
		}
	}
}

func TestParseVerdict(t *testing.T) {
	for text, want := range map[string]string{
		"blah\n> **Mergeable with follow-ups.** Six findings":     verdictFollowUps,
		"> **Not mergeable.** Seven findings":                     verdictNotMergeable,
		"PR is mergeable\n```\n> **Mergeable.** No findings\n```": verdictReady,
		"1. **Not mergeable** says the round":                     verdictNotMergeable,
		"nothing here":                                            "",
	} {
		if got := parseVerdict(text); got != want {
			t.Errorf("parseVerdict(%q) = %q, want %q", text, got, want)
		}
	}
	for _, c := range []struct {
		in   map[string]string
		want string
	}{
		{map[string]string{"review": verdictReady, "docs": verdictFollowUps, "comments": verdictFollowUps}, verdictReady},
		{map[string]string{"review": verdictFollowUps, "docs": verdictReady}, verdictFollowUps},
		{map[string]string{"review": verdictReady, "comments": verdictNotMergeable}, verdictNotMergeable},
		{nil, ""},
	} {
		if got := overallVerdict(c.in); got != c.want {
			t.Errorf("overallVerdict(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDraftComment(t *testing.T) {
	// The shape of a real verified log: the heading above the block varies.
	verified := "## Findings\n\n- one\n\n## Draft comment (not posted)\n\n```\n<!-- tono:review n=1 sha=6ec793f -->\n## Code Review #1\n\n> **Mergeable.** One finding.\n\n<sub>tono code review</sub>\n```\n\nTwo MCP servers failed.\n"
	want := "<!-- tono:review n=1 sha=6ec793f -->\n## Code Review #1\n\n> **Mergeable.** One finding.\n\n<sub>tono code review</sub>"
	if got := draftComment(verified); got != want {
		t.Errorf("got %q", got)
	}
	// A fenced block without a marker is not a draft.
	if got := draftComment("## Checked\n\n```json\n[]\n```\n"); got != "" {
		t.Errorf("want no draft, got %q", got)
	}
	lgtm := lgtmComment("55d37455abcdef")
	if !strings.HasPrefix(lgtm, "<!-- tono:lgtm sha=55d3745 -->\nLGTM 😃⭐😸") {
		t.Errorf("lgtm = %q", lgtm)
	}
}

func TestCommentsFor(t *testing.T) {
	draft := "<!-- tono:review n=1 sha=abc1234 -->\n## Code Review #1\n\n<sub>tono code review</sub>"
	cases := []struct {
		name string
		out  tonoOutcome
		want string // "draft", "lgtm" or "failed"
	}{
		{"drafts are posted", tonoOutcome{drafts: []string{draft}, passes: 3, verdicts: map[string]string{"review": verdictFollowUps}}, "draft"},
		{"clean, no verdicts", tonoOutcome{passes: 3, verdicts: map[string]string{}}, "lgtm"},
		{"clean, ready", tonoOutcome{passes: 3, verdicts: map[string]string{"review": verdictReady}}, "lgtm"},
		{"a pass did not finish", tonoOutcome{passes: 2, verdicts: map[string]string{}}, "failed"},
		{"drafts, but a pass did not finish", tonoOutcome{drafts: []string{draft}, passes: 2, verdicts: map[string]string{"review": verdictFollowUps}}, "failed"},
		{"follow-ups but no draft", tonoOutcome{passes: 3, verdicts: map[string]string{"review": verdictFollowUps}}, "failed"},
		{"not mergeable but no draft", tonoOutcome{passes: 3, verdicts: map[string]string{"review": verdictNotMergeable}}, "failed"},
		// overallVerdict needs a clean code review for "ready", so a lone docs
		// "mergeable" reads as follow-ups: no LGTM without a code verdict.
		{"docs mergeable, no code verdict", tonoOutcome{passes: 3, verdicts: map[string]string{"docs": verdictReady}}, "failed"},
		{"cut-short draft", tonoOutcome{drafts: []string{"<!-- tono:review n=1 -->\n## Code Review #1\n\n- **HIGH**: see"}, passes: 3}, "failed"},
	}
	for _, c := range cases {
		comments, failed := commentsFor(c.out, "abc1234def")
		got := "failed"
		switch {
		case failed != "" && len(comments) > 0:
			t.Errorf("%s: both comments and a failure", c.name)
		case failed != "":
		case len(comments) == 1 && strings.HasPrefix(comments[0], "<!-- tono:lgtm"):
			got = "lgtm"
		case len(comments) > 0:
			got = "draft"
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s (failed %q)", c.name, got, c.want, failed)
		}
	}
}

func TestPostPending(t *testing.T) {
	const key = "o/r#1@abc"
	// fakeEnv posts every body but "fails", and saves results in st.
	fakeEnv := func(st *State, posted *[]string, onGitHub map[string]string) Env {
		return Env{
			post: func(_ context.Context, _, body string) (string, error) {
				if body == "fails" {
					return "", fmt.Errorf("network")
				}
				*posted = append(*posted, body)
				return "https://github.com/o/r/pull/1#" + body, nil
			},
			findComment: func(_ context.Context, _ string, _ int, line string) (string, bool, error) {
				url, ok := onGitHub[line]
				return url, ok, nil
			},
			persist: func(change func(*State)) error { change(st); return nil },
		}
	}
	r := TonoResult{URL: "https://github.com/o/r/pull/1", Repo: "o/r", Number: 1}

	// A failed post stops the loop, keeps the rest and counts one try.
	st := &State{TonoResults: map[string]TonoResult{}}
	var posted []string
	r.Unposted = []string{"a", "fails", "b"}
	if err := postPending(context.Background(), fakeEnv(st, &posted, nil), key, r, false); err != nil {
		t.Fatal(err)
	}
	got := st.TonoResults[key]
	if strings.Join(posted, ",") != "a" || strings.Join(got.Unposted, ",") != "fails,b" || got.PostTries != 1 || len(got.Comments) != 1 {
		t.Errorf("after one failure: posted %v, result %+v", posted, got)
	}

	// The third failed try fails the review and drops what is left.
	got.PostTries = maxPostTries - 1
	posted = nil
	if err := postPending(context.Background(), fakeEnv(st, &posted, nil), key, got, true); err != nil {
		t.Fatal(err)
	}
	if got = st.TonoResults[key]; got.Failed == "" || len(got.Unposted) != 0 {
		t.Errorf("after the last try: %+v", got)
	}

	// A retry finds a comment that went through and does not post it twice.
	st = &State{TonoResults: map[string]TonoResult{}}
	posted = nil
	r.Unposted = []string{"<!-- tono:review n=1 sha=abc -->\nbody", "b"}
	onGitHub := map[string]string{"<!-- tono:review n=1 sha=abc -->": "https://github.com/o/r/pull/1#old"}
	if err := postPending(context.Background(), fakeEnv(st, &posted, onGitHub), key, r, true); err != nil {
		t.Fatal(err)
	}
	got = st.TonoResults[key]
	if strings.Join(posted, ",") != "b" || strings.Join(got.Comments, ",") != "https://github.com/o/r/pull/1#old,https://github.com/o/r/pull/1#b" {
		t.Errorf("retry: posted %v, comments %v", posted, got.Comments)
	}
}

func TestDraftCommentFences(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"tildes", "## Draft\n\n~~~\n<!-- tono:docs-check n=1 -->\nBody\n<sub>f</sub>\n~~~\n", "<!-- tono:docs-check n=1 -->\nBody\n<sub>f</sub>"},
		{"indented", "1. Draft:\n\n   ```\n   <!-- tono:review n=1 -->\n   Body\n   <sub>f</sub>\n   ```\n", "<!-- tono:review n=1 -->\nBody\n<sub>f</sub>"},
		{"inner code block", "```\n<!-- tono:review n=1 -->\nFix:\n```go\nx := 1\n```\n<sub>f</sub>\n```\n", "<!-- tono:review n=1 -->\nFix:\n```go\nx := 1\n```\n<sub>f</sub>"},
		{"longer outer fence", "````\n<!-- tono:review n=1 -->\n```\ncode\n```\n<sub>f</sub>\n````\n", "<!-- tono:review n=1 -->\n```\ncode\n```\n<sub>f</sub>"},
		{"marker only quoted in prose", "- The docs quote `<!-- tono:... -->` as an example.\n", ""},
	}
	for _, c := range cases {
		if got := draftComment(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	// A marker line outside any fence the finder can read is not clean.
	if !hasMarkerLine("## Draft\n\n<!-- tono:review n=1 -->\nBody\n") || hasMarkerLine("- quoted `<!-- tono:x -->`") {
		t.Error("hasMarkerLine")
	}
	out := tonoOutcome{passes: 3, unreadable: 1, verdicts: map[string]string{}}
	if c, failed := commentsFor(out, "abc"); failed == "" || c != nil {
		t.Error("an unreadable draft must fail, not post LGTM")
	}
}

func TestTonoScope(t *testing.T) {
	now := monday(9, 30)
	s := tonoScope{
		team: "epidemicsound/content-protection", days: 4,
		members: map[string]bool{"jamie-r-es": true, "edalee": true},
		since:   now.AddDate(0, 0, -4),
	}
	pr := func(login, created string) SearchPR {
		p := SearchPR{CreatedAt: created}
		p.Author.Login = login
		return p
	}
	for _, c := range []struct {
		pr   SearchPR
		want string
	}{
		{pr("jamie-r-es", "2026-09-27T10:00:00Z"), ""},
		{pr("Jamie-R-ES", "2026-09-25T08:00:00Z"), ""}, // logins are not case-sensitive
		{pr("jamie-r-es", "2026-09-20T10:00:00Z"), "opened more than 4 days ago"},
		{pr("someone-else", "2026-09-27T10:00:00Z"), "not opened by epidemicsound/content-protection"},
		{pr("edalee", "not a date"), "opened more than 4 days ago"},
	} {
		if got := s.outside(c.pr); got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.pr.Author.Login, c.pr.CreatedAt, got, c.want)
		}
	}
}

func TestOneReviewPerPR(t *testing.T) {
	st := loadState("/nonexistent")
	st.TonoReviewed[tonoKey("o/r", 12, "aaa")] = monday(9, 0)
	st.TonoResults[tonoKey("o/r", 12, "aaa")] = TonoResult{Repo: "o/r", Number: 12, SHA: "aaa", At: monday(9, 0)}
	st.TonoResults[tonoKey("o/r", 12, "bbb")] = TonoResult{Repo: "o/r", Number: 12, SHA: "bbb", At: monday(12, 0)}
	if !reviewedBefore(&st, "o/r", 12) {
		t.Error("a new commit must not bring a second review")
	}
	if reviewedBefore(&st, "o/r", 1) || reviewedBefore(&st, "o/r2", 12) {
		t.Error("other PRs count as reviewed")
	}
	if r, ok := latestResult(&st, "o/r", 12); !ok || r.SHA != "bbb" {
		t.Errorf("latest = %+v", r)
	}
}

func TestTonoScopeConfig(t *testing.T) {
	cfg := defaultConfig()
	if cfg.TonoTeam != "epidemicsound/content-protection" || cfg.TonoMaxAgeDays != 4 {
		t.Errorf("defaults: %q, %d", cfg.TonoTeam, cfg.TonoMaxAgeDays)
	}
	if !cfg.TonoMine || !cfg.TonoTeamPRs {
		t.Errorf("my PRs and team PRs should default on: %v, %v", cfg.TonoMine, cfg.TonoTeamPRs)
	}
	cfg.TonoTeam = "content-protection"
	if cfg.validate() == nil {
		t.Error("want an error for a team without its org")
	}
	cfg = defaultConfig()
	for _, bad := range []string{"kalimba", "epidemicsound/", "a/b/c"} {
		cfg.TonoRepos = []string{bad}
		if cfg.validate() == nil {
			t.Errorf("want an error for repo %q", bad)
		}
	}
}

func TestLoadConfigTonoPRs(t *testing.T) {
	load := func(body string) Config {
		t.Helper()
		path := t.TempDir() + "/worker.json"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	// A config from before the split keeps both kinds on.
	if cfg := load(`{"tonoTeam": "epidemicsound/content-protection"}`); !cfg.TonoMine || !cfg.TonoTeamPRs {
		t.Errorf("old config: %+v", cfg)
	}
	// The old "own PRs only" switch turns team PRs off.
	if cfg := load(`{"tonoOwnPRsOnly": true}`); !cfg.TonoMine || cfg.TonoTeamPRs {
		t.Errorf("tonoOwnPRsOnly: mine %v, team %v", cfg.TonoMine, cfg.TonoTeamPRs)
	}
	cfg := load(`{"tonoMine": false, "tonoRepos": [" epidemicsound/kalimba ", ""]}`)
	if cfg.TonoMine || strings.Join(cfg.TonoRepos, ",") != "epidemicsound/kalimba" {
		t.Errorf("mine %v, repos %q", cfg.TonoMine, cfg.TonoRepos)
	}
}

func TestTeamSection(t *testing.T) {
	pr := func(n int, author, created string) SearchPR {
		p := SearchPR{URL: fmt.Sprintf("https://github.com/epidemicsound/kalimba/pull/%d", n), Number: n, Title: "Change", CreatedAt: created}
		p.Repository.NameWithOwner = "epidemicsound/kalimba"
		p.Author.Login = author
		return p
	}
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, stockholm)
	cfg := defaultConfig()
	cfg.TonoRepos = []string{"epidemicsound/kalimba"}
	env := Env{cfg: cfg, now: now, state: &State{}}
	tally := &reviewTally{loaded: true, scope: tonoScope{
		team: cfg.TonoTeam, days: 4, members: map[string]bool{"alice": true}, since: now.AddDate(0, 0, -4),
	}}
	recent := "2026-09-29T10:00:00Z"
	prs := []SearchPR{
		pr(1, "alice", recent),                 // listed
		pr(2, "alice", recent),                 // asks for your review, so listed under "Needs your review"
		pr(3, "bob", recent),                   // not in the team
		pr(4, "alice", "2026-09-01T10:00:00Z"), // too old
	}
	got := teamSection(context.Background(), env, tally, prs, nil, prs[1:2])
	if !strings.Contains(got, "1 open team PRs") || !strings.Contains(got, "kalimba#1") ||
		strings.Contains(got, "kalimba#2") || strings.Contains(got, "kalimba#3") || strings.Contains(got, "kalimba#4") {
		t.Errorf("team section:\n%s", got)
	}
	env.cfg.TonoTeamPRs = false
	if s := teamSection(context.Background(), env, tally, prs, nil, nil); s != "" {
		t.Errorf("switched off, got:\n%s", s)
	}
}
