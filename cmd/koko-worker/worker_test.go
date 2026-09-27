package main

import (
	"context"
	"os"
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
	empty := State{Runs: map[string]RunRecord{}}

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
		st := State{Runs: map[string]RunRecord{}}
		recordRun(&st, DueRun{Job: JobStandup, Slots: []time.Time{monday(7, 0)}}, StatusOK, "", monday(7, 1))
		recordRun(&st, DueRun{Job: JobFocus, Slots: []time.Time{monday(9, 15)}}, StatusFailed, "boom", monday(9, 16))
		if got := dueString(dueRuns(cfg, st, monday(9, 20))); got != "" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("retry waits five minutes", func(t *testing.T) {
		st := State{Runs: map[string]RunRecord{}}
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
		if got := dueString(dueRuns(c, empty, monday(9, 40))); got != "standup[07:00] focus[09:15]" {
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

func TestCutForSlack(t *testing.T) {
	short := "fine"
	if cutForSlack(short, "/log") != short {
		t.Error("short text changed")
	}
	long := strings.Repeat("line of review text\n", 1000)
	got := cutForSlack(long, "/log")
	if len(got) > slackMaxText+100 || !strings.Contains(got, "`/log`") {
		t.Errorf("cut to %d chars, log link present: %v", len(got), strings.Contains(got, "/log"))
	}
}

func TestMeetings(t *testing.T) {
	events := []GCalEvent{
		ev("09:30", "09:45", title("Standup"), response("accepted")),
		ev("10:00", "11:00", title("Skipped"), response("declined")),
		ev("11:00", "12:00", title("Focus"), func(e *GCalEvent) { e.EventType = "FOCUS_TIME" }),
		ev("13:00", "14:00", title("Planning"), response("needsAction")),
		ev("14:00", "15:00", title("Focus"), func(e *GCalEvent) { e.Description = focusMarker }),
	}
	got := strings.Join(meetings(events, stockholm), " | ")
	want := "09:30–09:45 Standup | 13:00–14:00 Planning _(not answered)_"
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

func TestFormatTonoReport(t *testing.T) {
	report := "Six findings.\n\n```json\n[\n  {\"file\": \"requirements.txt\", \"line\": 7, \"summary\": \"The ceiling lands in prod.\", \"failure_scenario\": \"long\"},\n  {\"file\": \"ci.yaml\", \"summary\": \"No pin.\"}\n]\n```\n\nDone."
	got := formatTonoReport(report)
	want := "Six findings.\n\n• `requirements.txt:7` The ceiling lands in prod.\n• `ci.yaml` No pin.\n\nDone."
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	odd := "```json\n{\"not\": \"a list\"}\n```"
	if formatTonoReport(odd) != odd {
		t.Error("a block that is not a findings list must stay as it is")
	}
}

func TestToSlackMarkdown(t *testing.T) {
	in := "## Surviving findings\n\n1. **MED, confirmed:** the reason is void."
	want := "*Surviving findings*\n\n1. *MED, confirmed:* the reason is void."
	if got := toSlackMarkdown(in); got != want {
		t.Errorf("got %q", got)
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
	st := State{Runs: map[string]RunRecord{}}
	recordRun(&st, DueRun{Job: JobStandup, Slots: manualSlots(cfg, JobStandup, monday(6, 55))}, StatusOK, "run by hand", monday(6, 56))
	if d := dueString(dueRuns(cfg, st, monday(7, 0))); d != "" {
		t.Errorf("stand-up run by hand is due again: %q", d)
	}
}

func TestFailuresSkipNetworkRetries(t *testing.T) {
	st := State{Runs: map[string]RunRecord{}}
	cfg := testConfig()
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
