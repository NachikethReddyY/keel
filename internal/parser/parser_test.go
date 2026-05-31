package parser

import (
	"os"
	"strings"
	"testing"
	"time"

	"keel/internal/ledger"
)

func TestParseBasicLedger(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/basic.md")
	if err != nil {
		t.Fatal(err)
	}
	l, errs := Parse(string(raw))
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(l.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(l.Tasks))
	}
	if l.Tasks[0].ID != "K-20260530-9QRT" {
		t.Fatalf("unexpected first id: %s", l.Tasks[0].ID)
	}
	if len(l.Tasks[0].Notes) != 1 {
		t.Fatalf("expected first task note")
	}
}

func TestParseRejectsDuplicateIDs(t *testing.T) {
	input := strings.Join([]string{
		"- [ ] id:K-1 status:todo priority:p2 First",
		"- [ ] id:K-1 status:doing priority:p2 Second",
	}, "\n")
	_, errs := Parse(input)
	if len(errs) != 1 {
		t.Fatalf("expected duplicate id error, got %v", errs)
	}
}

func TestParseRejectsCheckboxStatusConflict(t *testing.T) {
	_, errs := Parse("- [x] id:K-1 status:todo priority:p2 Conflicted")
	if len(errs) != 1 {
		t.Fatalf("expected checkbox conflict, got %v", errs)
	}
}

func TestParseRejectsDuplicateCanonicalToken(t *testing.T) {
	_, errs := Parse("- [ ] id:K-1 id:K-2 status:todo priority:p2 Conflicted")
	if len(errs) != 1 {
		t.Fatalf("expected duplicate token error, got %v", errs)
	}
}

func TestRenderCanonicalRoundTrip(t *testing.T) {
	start := time.Date(2026, 5, 30, 9, 0, 0, 0, time.UTC)
	end := start.Add(45 * time.Minute)
	input := ledger.Ledger{Tasks: []ledger.Task{{
		ID:           "K-20260530-ABCD",
		Title:        "Write tests",
		Status:       ledger.StatusTodo,
		Priority:     ledger.PriorityP0,
		Due:          "2026-06-02",
		Category:     "engineering",
		Tags:         []string{"agent"},
		Meta:         []string{"owner:nr"},
		Subtasks:     []ledger.Subtask{{ID: "S-1", Title: "Keep fixture stable", Status: ledger.StatusTodo}},
		TimeSessions: []ledger.TimeSession{{Start: start, End: end}},
		Notes:        []string{"Keep it focused."},
	}}}
	rendered := Render(input)
	parsed, errs := Parse(rendered)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if parsed.Tasks[0].Meta[0] != "owner:nr" {
		t.Fatalf("metadata was not preserved: %#v", parsed.Tasks[0].Meta)
	}
	if parsed.Tasks[0].Category != "engineering" {
		t.Fatalf("category was not preserved: %s", parsed.Tasks[0].Category)
	}
	if got := parsed.Tasks[0].TrackedDuration(end); got != 45*time.Minute {
		t.Fatalf("unexpected tracked duration: %s", got)
	}
	if len(parsed.Tasks[0].Subtasks) != 1 {
		t.Fatalf("expected subtask round trip")
	}
}

func TestParseActiveTimeSession(t *testing.T) {
	input := strings.Join([]string{
		"- [ ] id:K-1 status:doing priority:p2 Timed task",
		"  @time start:2026-05-30T09:00:00Z",
	}, "\n")
	l, errs := Parse(input)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if _, ok := l.Tasks[0].ActiveSession(); !ok {
		t.Fatalf("expected active timer")
	}
}

func TestParseMultipleTimeSessions(t *testing.T) {
	input := strings.Join([]string{
		"- [ ] id:K-1 status:doing priority:p2 Timed task",
		"  @time start:2026-05-30T09:00:00Z end:2026-05-30T09:30:00Z duration:1800",
		"  @time start:2026-05-30T10:00:00Z end:2026-05-30T10:15:00Z duration:900",
	}, "\n")
	l, errs := Parse(input)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if got := l.Tasks[0].TrackedDuration(time.Now()); got != 45*time.Minute {
		t.Fatalf("unexpected tracked duration: %s", got)
	}
}

func TestParseSubtaskDoesNotCreateTopLevelTask(t *testing.T) {
	input := strings.Join([]string{
		"- [ ] id:K-1 status:todo priority:p2 Parent",
		"  - [ ] sub:S-1 status:todo Child",
	}, "\n")
	l, errs := Parse(input)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(l.Tasks) != 1 {
		t.Fatalf("expected one top-level task, got %d", len(l.Tasks))
	}
	if len(l.Tasks[0].Subtasks) != 1 {
		t.Fatalf("expected one subtask")
	}
}

func TestParseRecurringTasks(t *testing.T) {
	input := strings.Join([]string{
		"- [ ] recur:R-1 every:daily at:09:00 priority:p1 Make pasta",
		"- [ ] recur:R-2 every:custom days:mon,wed at:08:30,17:00 priority:p2 Stretch",
	}, "\n")
	l, errs := Parse(input)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(l.Tasks) != 0 {
		t.Fatalf("expected no normal tasks, got %d", len(l.Tasks))
	}
	if len(l.Recurring) != 2 {
		t.Fatalf("expected two recurring tasks, got %d", len(l.Recurring))
	}
	if !l.Recurring[0].DueOn(time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("daily recurrence should be due")
	}
	if l.Recurring[1].DueOn(time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("mon/wed recurrence should not be due on Sunday")
	}
}

func TestRenderRecurringRoundTrip(t *testing.T) {
	input := ledger.Ledger{Recurring: []ledger.RecurringTask{{
		ID:       "R-20260530-ABCD",
		Title:    "Make pasta",
		Kind:     ledger.RecurrenceWeekly,
		Days:     []time.Weekday{time.Sunday},
		Times:    []string{"18:30"},
		Priority: ledger.PriorityP1,
		Category: "home",
	}}}
	rendered := Render(input)
	parsed, errs := Parse(rendered)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(parsed.Recurring) != 1 {
		t.Fatalf("expected recurring round trip")
	}
	if parsed.Recurring[0].Title != "Make pasta" {
		t.Fatalf("unexpected recurring title: %s", parsed.Recurring[0].Title)
	}
	if parsed.Recurring[0].Category != "home" {
		t.Fatalf("recurring category was not preserved: %s", parsed.Recurring[0].Category)
	}
}

func TestParseRejectsDuplicateRecurringIDs(t *testing.T) {
	input := strings.Join([]string{
		"- [ ] recur:R-1 every:daily at:09:00 priority:p1 First",
		"- [ ] recur:R-1 every:daily at:10:00 priority:p1 Second",
	}, "\n")
	_, errs := Parse(input)
	if len(errs) != 1 {
		t.Fatalf("expected duplicate recurring id error, got %v", errs)
	}
}
