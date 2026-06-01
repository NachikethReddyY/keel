package ledger

import (
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusTodo    Status = "todo"
	StatusDoing   Status = "doing"
	StatusDone    Status = "done"
	StatusBlocked Status = "blocked"
)

type Priority string

const (
	PriorityP0 Priority = "p0"
	PriorityP1 Priority = "p1"
	PriorityP2 Priority = "p2"
	PriorityP3 Priority = "p3"
)

type Task struct {
	ID           string
	Title        string
	Status       Status
	Priority     Priority
	Due          string
	Category     string
	Tags         []string
	Meta         []string
	Subtasks     []Subtask
	TimeSessions []TimeSession
	Notes        []string
	Line         int
}

type RecurrenceKind string

const (
	RecurrenceDaily  RecurrenceKind = "daily"
	RecurrenceWeekly RecurrenceKind = "weekly"
	RecurrenceCustom RecurrenceKind = "custom"
)

type RecurringTask struct {
	ID       string
	Title    string
	Kind     RecurrenceKind
	Days     []time.Weekday
	Times    []string
	Priority Priority
	Category string
	Tags     []string
	Meta     []string
	Line     int
}

type Subtask struct {
	ID     string
	Title  string
	Done   bool
	Status Status
}

type TimeSession struct {
	Start time.Time
	End   time.Time
}

func (t Task) IsDone() bool {
	return t.Status == StatusDone
}

func (t Task) IsDueToday(now time.Time) bool {
	if t.Due == "" || t.IsDone() {
		return false
	}
	return t.Due <= now.Format("2006-01-02")
}

func (t Task) IsOverdue(now time.Time) bool {
	if t.Due == "" || t.IsDone() {
		return false
	}
	return t.Due < now.Format("2006-01-02")
}

func (t Task) Checkbox() string {
	if t.IsDone() {
		return "x"
	}
	return " "
}

func (t Task) CanonicalLine() string {
	parts := []string{fmt.Sprintf("- [%s]", t.Checkbox()), "id:" + t.ID, "status:" + string(t.Status)}
	if t.Priority != "" {
		parts = append(parts, "priority:"+string(t.Priority))
	}
	if t.Due != "" {
		parts = append(parts, "due:"+t.Due)
	}
	if t.Category != "" {
		parts = append(parts, "category:"+t.Category)
	}
	for _, tag := range t.Tags {
		parts = append(parts, "+"+tag)
	}
	parts = append(parts, t.Meta...)
	if strings.TrimSpace(t.Title) != "" {
		parts = append(parts, strings.TrimSpace(t.Title))
	}
	return strings.Join(parts, " ")
}

func (t Task) ActiveSession() (TimeSession, bool) {
	for _, session := range t.TimeSessions {
		if session.End.IsZero() {
			return session, true
		}
	}
	return TimeSession{}, false
}

func (t Task) TrackedDuration(now time.Time) time.Duration {
	var total time.Duration
	for _, session := range t.TimeSessions {
		end := session.End
		if end.IsZero() {
			end = now
		}
		if end.After(session.Start) {
			total += end.Sub(session.Start)
		}
	}
	return total
}

type Ledger struct {
	Tasks     []Task
	Recurring []RecurringTask
}

func (l Ledger) NextByStatus(status Status) []Task {
	var tasks []Task
	for _, task := range l.Tasks {
		if task.Status == status {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (l Ledger) ActiveTimer() (string, TimeSession, bool) {
	for _, task := range l.Tasks {
		if session, ok := task.ActiveSession(); ok {
			return task.ID, session, true
		}
	}
	return "", TimeSession{}, false
}

func (r RecurringTask) DueOn(now time.Time) bool {
	switch r.Kind {
	case RecurrenceDaily:
		return true
	case RecurrenceWeekly, RecurrenceCustom:
		for _, day := range r.Days {
			if day == now.Weekday() {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (r RecurringTask) DueTodayTasks(now time.Time) []Task {
	if !r.DueOn(now) {
		return nil
	}
	times := r.Times
	if len(times) == 0 {
		times = []string{""}
	}
	var tasks []Task
	for _, at := range times {
		title := r.Title
		if at != "" {
			title = fmt.Sprintf("%s @ %s", title, at)
		}
		tasks = append(tasks, Task{
			ID:       "rec:" + r.ID,
			Title:    title,
			Status:   StatusTodo,
			Priority: r.Priority,
			Due:      now.Format("2006-01-02"),
			Category: r.Category,
			Tags:     append([]string{"recurring"}, r.Tags...),
			Meta:     append([]string{"source:" + r.ID}, r.Meta...),
		})
	}
	return tasks
}

func (r RecurringTask) CanonicalLine() string {
	parts := []string{"- [ ]", "recur:" + r.ID, "every:" + string(r.Kind)}
	if len(r.Days) > 0 {
		parts = append(parts, "days:"+formatDays(r.Days))
	}
	if len(r.Times) > 0 {
		parts = append(parts, "at:"+strings.Join(r.Times, ","))
	}
	if r.Priority != "" {
		parts = append(parts, "priority:"+string(r.Priority))
	}
	if r.Category != "" {
		parts = append(parts, "category:"+r.Category)
	}
	for _, tag := range r.Tags {
		parts = append(parts, "+"+tag)
	}
	parts = append(parts, r.Meta...)
	if strings.TrimSpace(r.Title) != "" {
		parts = append(parts, strings.TrimSpace(r.Title))
	}
	return strings.Join(parts, " ")
}

func formatDays(days []time.Weekday) string {
	names := make([]string, 0, len(days))
	for _, day := range days {
		switch day {
		case time.Monday:
			names = append(names, "mon")
		case time.Tuesday:
			names = append(names, "tue")
		case time.Wednesday:
			names = append(names, "wed")
		case time.Thursday:
			names = append(names, "thu")
		case time.Friday:
			names = append(names, "fri")
		case time.Saturday:
			names = append(names, "sat")
		case time.Sunday:
			names = append(names, "sun")
		}
	}
	return strings.Join(names, ",")
}
