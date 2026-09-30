package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const focusMarker = "[koko-worker:focus]"

var timeOffTitle = regexp.MustCompile(`(?i)\b(ooo|out of office|holiday|vacation|leave|sick|vab)\b`)

// gcalTime is Google's start or end: dateTime for timed events, date for all-day ones.
type gcalTime struct {
	DateTime string `json:"dateTime"`
	Date     string `json:"date"`
}

// GCalEvent is the part of a Google Calendar event the jobs use.
type GCalEvent struct {
	ID           string   `json:"id"`
	Summary      string   `json:"summary"`
	Status       string   `json:"status"`       // confirmed, tentative, cancelled
	Transparency string   `json:"transparency"` // "transparent" means free. Missing means busy.
	EventType    string   `json:"eventType"`    // DEFAULT, OUT_OF_OFFICE, FOCUS_TIME, WORKING_LOCATION, ...
	Description  string   `json:"description"`
	Start        gcalTime `json:"start"`
	End          gcalTime `json:"end"`
	Attendees    []struct {
		Self           bool   `json:"self"`
		ResponseStatus string `json:"responseStatus"` // accepted, declined, tentative, needsAction
	} `json:"attendees"`
}

func (e GCalEvent) allDay() bool { return e.Start.DateTime == "" && e.Start.Date != "" }

// myResponse is your own attendee status, or "" for an event without attendees.
func (e GCalEvent) myResponse() string {
	for _, a := range e.Attendees {
		if a.Self {
			return a.ResponseStatus
		}
	}
	return ""
}

func (e GCalEvent) times(loc *time.Location) (time.Time, time.Time, error) {
	if e.allDay() {
		s, err := time.ParseInLocation("2006-01-02", e.Start.Date, loc)
		if err != nil {
			return s, s, err
		}
		en, err := time.ParseInLocation("2006-01-02", e.End.Date, loc)
		return s, en, err
	}
	s, err := time.Parse(time.RFC3339, e.Start.DateTime)
	if err != nil {
		return s, s, err
	}
	en, err := time.Parse(time.RFC3339, e.End.DateTime)
	return s.In(loc), en.In(loc), err
}

// Gap is a free slot to book.
type Gap struct{ Start, End time.Time }

// findFocusGaps returns today's free gaps of at least MinMinutes inside the
// focus window, from now on.
//
// Timed events block unless you declined them, they are marked free, they are
// cancelled, or they are a working-location marker. Tentative and unanswered
// invites block. Focus blocks already booked block too, so a second run on
// the same day books nothing. An all-day event that is marked busy, or looks
// like time off, blocks the whole day.
func findFocusGaps(events []GCalEvent, fc FocusConfig, now time.Time) []Gap {
	if !isWeekday(now) {
		return nil
	}
	loc := now.Location()
	var busy []Gap
	for _, e := range events {
		if e.Status == "cancelled" || e.EventType == "WORKING_LOCATION" || e.myResponse() == "declined" {
			continue
		}
		if e.allDay() {
			if e.Transparency != "transparent" || timeOffTitle.MatchString(e.Summary) {
				return nil
			}
			continue
		}
		if e.Transparency == "transparent" {
			continue
		}
		s, en, err := e.times(loc)
		if err != nil {
			continue
		}
		busy = append(busy, Gap{s, en})
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })

	cursor := atClock(now, fc.WindowStart)
	windowEnd := atClock(now, fc.WindowEnd)
	if now.After(cursor) {
		// If the run starts late, round up to the next 5 minutes, so blocks begin on a clean time.
		rounded := now.Truncate(time.Minute)
		if m := rounded.Minute() % 5; m != 0 || rounded.Before(now) {
			rounded = rounded.Add(time.Duration(5-m) * time.Minute)
		}
		cursor = rounded
	}

	var gaps []Gap
	for _, b := range busy {
		if !b.Start.Before(windowEnd) {
			break
		}
		if b.Start.After(cursor) {
			gaps = append(gaps, Gap{cursor, b.Start})
		}
		if b.End.After(cursor) {
			cursor = b.End
		}
	}
	if cursor.Before(windowEnd) {
		gaps = append(gaps, Gap{cursor, windowEnd})
	}

	min := time.Duration(fc.MinMinutes) * time.Minute
	var out []Gap
	for _, g := range gaps {
		if g.End.Sub(g.Start) >= min {
			out = append(out, g)
		}
	}
	return out
}

// meetings lists today's events worth showing in the stand-up.
func meetings(events []GCalEvent, loc *time.Location) []string {
	var out []string
	for _, e := range events {
		if e.Status == "cancelled" || e.myResponse() == "declined" {
			continue
		}
		if e.EventType == "FOCUS_TIME" || e.EventType == "WORKING_LOCATION" || strings.Contains(e.Description, focusMarker) {
			continue
		}
		title := e.Summary
		if title == "" {
			title = "(no title)"
		}
		s, en, err := e.times(loc)
		if err != nil {
			continue
		}
		var line string
		if e.allDay() || spansWholeDays(s, en) {
			line = "All day: " + title
		} else {
			line = fmt.Sprintf("%s–%s %s", s.Format("15:04"), en.Format("15:04"), title)
		}
		if r := e.myResponse(); r == "needsAction" || r == "tentative" {
			line += " _(" + map[string]string{"needsAction": "not answered", "tentative": "tentative"}[r] + ")_"
		}
		out = append(out, line)
	}
	return out
}

// spansWholeDays reports a timed event that runs from one midnight to a later
// one. Some invites are sent this way instead of as all-day events.
func spansWholeDays(s, en time.Time) bool {
	midnight := func(t time.Time) bool { return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 }
	return en.After(s) && midnight(s) && midnight(en)
}

// listEvents fetches one day's events through the Google Calendar connector.
// Go reads the tool's raw result, so Claude never retypes the event data.
func listEvents(ctx context.Context, cr claudeRunner, cfg Config, day time.Time) ([]GCalEvent, error) {
	loc := cfg.location()
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	prompt := fmt.Sprintf(`Call %s exactly once with calendarId %q, startTime %q, endTime %q, timeZone %q, pageSize 250.
If the result has a nextPageToken, call it again with that pageToken until there is none.
Then reply with the single word DONE. Do not summarise the events.`,
		toolListEvents, cfg.CalendarID, start.Format(time.RFC3339), end.Format(time.RFC3339), cfg.TimeZone)

	res, err := cr.run(ctx, prompt, []string{toolListEvents}, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	calls := res.callsTo(toolListEvents)
	if len(calls) == 0 {
		return nil, fmt.Errorf("calendar: list_events was not called: %s", truncate(res.Text, 200))
	}
	var events []GCalEvent
	for _, c := range calls {
		if c.IsError {
			return nil, fmt.Errorf("calendar: %s", truncate(c.Result, 300))
		}
		var page struct {
			Events []GCalEvent `json:"events"`
		}
		if err := json.Unmarshal([]byte(c.Result), &page); err != nil {
			return nil, fmt.Errorf("calendar: unreadable list_events result: %s", truncate(c.Result, 200))
		}
		events = append(events, page.Events...)
	}
	return events, nil
}

// createFocusBlocks books each gap as a plain busy event tagged with
// focusMarker. It returns the gaps Google confirmed.
//
// Not Google's "Focus time" type: that type can decline new invites by
// itself, and the connector can neither show nor change that setting. A plain
// busy event never declines anything.
func createFocusBlocks(ctx context.Context, cr claudeRunner, cfg Config, gaps []Gap) ([]Gap, error) {
	var lines []string
	for i, g := range gaps {
		lines = append(lines, fmt.Sprintf("%d. startTime %q, endTime %q", i+1, g.Start.Format(time.RFC3339), g.End.Format(time.RFC3339)))
	}
	prompt := fmt.Sprintf(`Call %s once for each block below, and for nothing else.
Every call uses: calendarId %q, summary "Focus", description %q, timeZone %q, eventType "DEFAULT", availability "AVAILABILITY_BUSY", notificationLevel "NONE".
%s
Then reply with the single word DONE.`,
		toolCreateEvent, cfg.CalendarID, focusMarker, cfg.TimeZone, strings.Join(lines, "\n"))

	res, err := cr.run(ctx, prompt, []string{toolCreateEvent}, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	var booked []Gap
	for _, c := range res.callsTo(toolCreateEvent) {
		if c.IsError {
			continue
		}
		var ev GCalEvent
		if json.Unmarshal([]byte(c.Result), &ev) != nil || ev.ID == "" {
			continue
		}
		s, en, err := ev.times(cfg.location())
		if err == nil {
			booked = append(booked, Gap{s, en})
		}
	}
	if len(booked) < len(gaps) {
		return booked, fmt.Errorf("calendar: booked %d of %d focus blocks", len(booked), len(gaps))
	}
	return booked, nil
}
