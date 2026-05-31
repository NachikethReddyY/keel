package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"keel/internal/ledger"
)

type ParseError struct {
	Line int
	Msg  string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func Parse(input string) (ledger.Ledger, []ParseError) {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	var out ledger.Ledger
	var errs []ParseError
	var current *ledger.Task
	seen := map[string]int{}
	seenRecurring := map[string]int{}

	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "- [") {
			if strings.Contains(trimmed, " recur:") || strings.HasPrefix(strings.TrimPrefix(trimmed, "- [ ] "), "recur:") {
				recurring, err := parseRecurringLine(trimmed, lineNo)
				if err != nil {
					errs = append(errs, *err)
					current = nil
					continue
				}
				if first, ok := seenRecurring[recurring.ID]; ok {
					errs = append(errs, ParseError{Line: lineNo, Msg: fmt.Sprintf("duplicate recurring id %s first seen on line %d", recurring.ID, first)})
				}
				seenRecurring[recurring.ID] = lineNo
				out.Recurring = append(out.Recurring, recurring)
				current = nil
				continue
			}
			task, err := parseTaskLine(trimmed, lineNo)
			if err != nil {
				errs = append(errs, *err)
				current = nil
				continue
			}
			if first, ok := seen[task.ID]; ok {
				errs = append(errs, ParseError{Line: lineNo, Msg: fmt.Sprintf("duplicate id %s first seen on line %d", task.ID, first)})
			}
			seen[task.ID] = lineNo
			out.Tasks = append(out.Tasks, task)
			current = &out.Tasks[len(out.Tasks)-1]
			continue
		}
		if current != nil && (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) && trimmed != "" {
			note := strings.TrimSpace(line)
			if strings.HasPrefix(note, "- [") {
				subtask, err := parseSubtaskLine(note, lineNo)
				if err != nil {
					errs = append(errs, *err)
					continue
				}
				current.Subtasks = append(current.Subtasks, subtask)
				continue
			}
			if strings.HasPrefix(note, "@time ") {
				session, err := parseTimeSession(note)
				if err != nil {
					errs = append(errs, ParseError{Line: lineNo, Msg: err.Error()})
					continue
				}
				current.TimeSessions = append(current.TimeSessions, session)
				continue
			}
			current.Notes = append(current.Notes, note)
		}
	}

	return out, errs
}

func parseRecurringLine(line string, lineNo int) (ledger.RecurringTask, *ParseError) {
	checkClose := strings.Index(line, "]")
	if checkClose < 4 {
		return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "recurring task must start with - [ ]"}
	}
	check := strings.TrimSpace(line[3:checkClose])
	if check != "" {
		return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "recurring task checkbox must be blank"}
	}
	recurring := ledger.RecurringTask{Line: lineNo, Kind: ledger.RecurrenceDaily, Priority: ledger.PriorityP2}
	rest := strings.TrimSpace(line[checkClose+1:])
	fields := strings.Fields(rest)
	titleAt := len(fields)
	seenToken := map[string]bool{}
	for i, field := range fields {
		if !isToken(field) {
			titleAt = i
			break
		}
		switch {
		case strings.HasPrefix(field, "recur:"):
			if seenToken["recur"] {
				return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "duplicate recur token"}
			}
			seenToken["recur"] = true
			recurring.ID = strings.TrimPrefix(field, "recur:")
		case strings.HasPrefix(field, "every:"):
			if seenToken["every"] {
				return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "duplicate every token"}
			}
			seenToken["every"] = true
			recurring.Kind = ledger.RecurrenceKind(strings.TrimPrefix(field, "every:"))
		case strings.HasPrefix(field, "days:"):
			if seenToken["days"] {
				return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "duplicate days token"}
			}
			seenToken["days"] = true
			days, err := parseDays(strings.TrimPrefix(field, "days:"))
			if err != nil {
				return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: err.Error()}
			}
			recurring.Days = days
		case strings.HasPrefix(field, "at:"):
			if seenToken["at"] {
				return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "duplicate at token"}
			}
			seenToken["at"] = true
			times, err := parseTimes(strings.TrimPrefix(field, "at:"))
			if err != nil {
				return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: err.Error()}
			}
			recurring.Times = times
		case strings.HasPrefix(field, "priority:"):
			if seenToken["priority"] {
				return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "duplicate priority token"}
			}
			seenToken["priority"] = true
			recurring.Priority = ledger.Priority(strings.TrimPrefix(field, "priority:"))
		case strings.HasPrefix(field, "category:"):
			if seenToken["category"] {
				return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "duplicate category token"}
			}
			seenToken["category"] = true
			recurring.Category = strings.TrimPrefix(field, "category:")
		case strings.HasPrefix(field, "+"):
			recurring.Tags = append(recurring.Tags, strings.TrimPrefix(field, "+"))
		default:
			recurring.Meta = append(recurring.Meta, field)
		}
	}
	if titleAt < len(fields) {
		recurring.Title = strings.Join(fields[titleAt:], " ")
	}
	if recurring.ID == "" {
		return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "missing recur:<id> token"}
	}
	if recurring.Title == "" {
		return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "missing recurring task title"}
	}
	if !validRecurrenceKind(recurring.Kind) {
		return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "every must be daily, weekly, or custom"}
	}
	if (recurring.Kind == ledger.RecurrenceWeekly || recurring.Kind == ledger.RecurrenceCustom) && len(recurring.Days) == 0 {
		return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "weekly/custom recurring tasks require days:<days>"}
	}
	if !validPriority(recurring.Priority) {
		return ledger.RecurringTask{}, &ParseError{Line: lineNo, Msg: "priority must be p0, p1, p2, or p3"}
	}
	return recurring, nil
}

func parseSubtaskLine(line string, lineNo int) (ledger.Subtask, *ParseError) {
	checkClose := strings.Index(line, "]")
	if checkClose < 4 {
		return ledger.Subtask{}, &ParseError{Line: lineNo, Msg: "subtask must start with - [ ] or - [x]"}
	}
	check := strings.TrimSpace(line[3:checkClose])
	subtask := ledger.Subtask{Status: ledger.StatusTodo}
	if strings.EqualFold(check, "x") {
		subtask.Done = true
		subtask.Status = ledger.StatusDone
	} else if check != "" {
		return ledger.Subtask{}, &ParseError{Line: lineNo, Msg: "subtask checkbox must be blank or x"}
	}
	rest := strings.TrimSpace(line[checkClose+1:])
	fields := strings.Fields(rest)
	titleAt := len(fields)
	for i, field := range fields {
		if !isToken(field) {
			titleAt = i
			break
		}
		switch {
		case strings.HasPrefix(field, "sub:"):
			subtask.ID = strings.TrimPrefix(field, "sub:")
		case strings.HasPrefix(field, "status:"):
			subtask.Status = ledger.Status(strings.TrimPrefix(field, "status:"))
			subtask.Done = subtask.Status == ledger.StatusDone
		}
	}
	if titleAt < len(fields) {
		subtask.Title = strings.Join(fields[titleAt:], " ")
	}
	if subtask.Title == "" {
		return ledger.Subtask{}, &ParseError{Line: lineNo, Msg: "missing subtask title"}
	}
	if subtask.ID == "" {
		subtask.ID = fmt.Sprintf("S-%d", lineNo)
	}
	if !validStatus(subtask.Status) {
		return ledger.Subtask{}, &ParseError{Line: lineNo, Msg: "subtask status must be todo, doing, done, or blocked"}
	}
	return subtask, nil
}

func parseTimeSession(line string) (ledger.TimeSession, error) {
	fields := strings.Fields(strings.TrimPrefix(line, "@time "))
	values := map[string]string{}
	for _, field := range fields {
		key, value, ok := strings.Cut(field, ":")
		if !ok || key == "" || value == "" {
			return ledger.TimeSession{}, fmt.Errorf("invalid @time token %q", field)
		}
		values[key] = value
	}
	startRaw := values["start"]
	if startRaw == "" {
		return ledger.TimeSession{}, fmt.Errorf("@time requires start:<RFC3339>")
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return ledger.TimeSession{}, fmt.Errorf("@time start must be RFC3339")
	}
	session := ledger.TimeSession{Start: start}
	if endRaw := values["end"]; endRaw != "" {
		end, err := time.Parse(time.RFC3339, endRaw)
		if err != nil {
			return ledger.TimeSession{}, fmt.Errorf("@time end must be RFC3339")
		}
		if end.Before(start) {
			return ledger.TimeSession{}, fmt.Errorf("@time end must be after start")
		}
		session.End = end
	}
	if durationRaw := values["duration"]; durationRaw != "" {
		if _, err := strconv.Atoi(durationRaw); err != nil {
			return ledger.TimeSession{}, fmt.Errorf("@time duration must be seconds")
		}
	}
	return session, nil
}

func parseTaskLine(line string, lineNo int) (ledger.Task, *ParseError) {
	checkClose := strings.Index(line, "]")
	if checkClose < 4 {
		return ledger.Task{}, &ParseError{Line: lineNo, Msg: "task must start with - [ ] or - [x]"}
	}
	check := strings.TrimSpace(line[3:checkClose])
	task := ledger.Task{Line: lineNo, Status: ledger.StatusTodo, Priority: ledger.PriorityP2}
	checkedDone := false
	if strings.EqualFold(check, "x") {
		task.Status = ledger.StatusDone
		checkedDone = true
	} else if check != "" {
		return ledger.Task{}, &ParseError{Line: lineNo, Msg: "checkbox must be blank or x"}
	}

	rest := strings.TrimSpace(line[checkClose+1:])
	fields := strings.Fields(rest)
	titleAt := len(fields)
	seenToken := map[string]bool{}
	explicitStatus := false
	for i, field := range fields {
		if !isToken(field) {
			titleAt = i
			break
		}
		switch {
		case strings.HasPrefix(field, "id:"):
			if seenToken["id"] {
				return ledger.Task{}, &ParseError{Line: lineNo, Msg: "duplicate id token"}
			}
			seenToken["id"] = true
			task.ID = strings.TrimPrefix(field, "id:")
		case strings.HasPrefix(field, "status:"):
			if seenToken["status"] {
				return ledger.Task{}, &ParseError{Line: lineNo, Msg: "duplicate status token"}
			}
			seenToken["status"] = true
			explicitStatus = true
			task.Status = ledger.Status(strings.TrimPrefix(field, "status:"))
		case strings.HasPrefix(field, "priority:"):
			if seenToken["priority"] {
				return ledger.Task{}, &ParseError{Line: lineNo, Msg: "duplicate priority token"}
			}
			seenToken["priority"] = true
			task.Priority = ledger.Priority(strings.TrimPrefix(field, "priority:"))
		case strings.HasPrefix(field, "due:"):
			if seenToken["due"] {
				return ledger.Task{}, &ParseError{Line: lineNo, Msg: "duplicate due token"}
			}
			seenToken["due"] = true
			task.Due = strings.TrimPrefix(field, "due:")
		case strings.HasPrefix(field, "category:"):
			if seenToken["category"] {
				return ledger.Task{}, &ParseError{Line: lineNo, Msg: "duplicate category token"}
			}
			seenToken["category"] = true
			task.Category = strings.TrimPrefix(field, "category:")
		case strings.HasPrefix(field, "+"):
			task.Tags = append(task.Tags, strings.TrimPrefix(field, "+"))
		default:
			task.Meta = append(task.Meta, field)
		}
	}
	if titleAt < len(fields) {
		task.Title = strings.Join(fields[titleAt:], " ")
	}

	if task.ID == "" {
		return ledger.Task{}, &ParseError{Line: lineNo, Msg: "missing id:<id> token"}
	}
	if task.Title == "" {
		return ledger.Task{}, &ParseError{Line: lineNo, Msg: "missing task title"}
	}
	if !validStatus(task.Status) {
		return ledger.Task{}, &ParseError{Line: lineNo, Msg: "status must be todo, doing, done, or blocked"}
	}
	if checkedDone && explicitStatus && task.Status != ledger.StatusDone {
		return ledger.Task{}, &ParseError{Line: lineNo, Msg: "checked tasks must use status:done"}
	}
	if !validPriority(task.Priority) {
		return ledger.Task{}, &ParseError{Line: lineNo, Msg: "priority must be p0, p1, p2, or p3"}
	}
	if task.Due != "" {
		if _, err := time.Parse("2006-01-02", task.Due); err != nil {
			return ledger.Task{}, &ParseError{Line: lineNo, Msg: "due date must use YYYY-MM-DD"}
		}
	}
	return task, nil
}

func isToken(field string) bool {
	if strings.HasPrefix(field, "+") {
		return len(field) > 1
	}
	if !strings.Contains(field, ":") {
		return false
	}
	key := strings.SplitN(field, ":", 2)[0]
	return key != ""
}

func validStatus(status ledger.Status) bool {
	switch status {
	case ledger.StatusTodo, ledger.StatusDoing, ledger.StatusDone, ledger.StatusBlocked:
		return true
	default:
		return false
	}
}

func validPriority(priority ledger.Priority) bool {
	switch priority {
	case ledger.PriorityP0, ledger.PriorityP1, ledger.PriorityP2, ledger.PriorityP3:
		return true
	default:
		return false
	}
}

func validRecurrenceKind(kind ledger.RecurrenceKind) bool {
	switch kind {
	case ledger.RecurrenceDaily, ledger.RecurrenceWeekly, ledger.RecurrenceCustom:
		return true
	default:
		return false
	}
}

func parseDays(raw string) ([]time.Weekday, error) {
	if raw == "" {
		return nil, fmt.Errorf("days cannot be empty")
	}
	parts := strings.Split(raw, ",")
	days := make([]time.Weekday, 0, len(parts))
	for _, part := range parts {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "mon", "monday":
			days = append(days, time.Monday)
		case "tue", "tues", "tuesday":
			days = append(days, time.Tuesday)
		case "wed", "wednesday":
			days = append(days, time.Wednesday)
		case "thu", "thur", "thurs", "thursday":
			days = append(days, time.Thursday)
		case "fri", "friday":
			days = append(days, time.Friday)
		case "sat", "saturday":
			days = append(days, time.Saturday)
		case "sun", "sunday":
			days = append(days, time.Sunday)
		default:
			return nil, fmt.Errorf("invalid day %q", part)
		}
	}
	return days, nil
}

func parseTimes(raw string) ([]string, error) {
	if raw == "" {
		return nil, fmt.Errorf("at cannot be empty")
	}
	parts := strings.Split(raw, ",")
	times := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if _, err := time.Parse("15:04", value); err != nil {
			return nil, fmt.Errorf("times must use HH:MM")
		}
		times = append(times, value)
	}
	return times, nil
}

func Render(l ledger.Ledger) string {
	var b strings.Builder
	b.WriteString("# Keel Tasks\n\n")
	for _, task := range l.Tasks {
		b.WriteString(task.CanonicalLine())
		b.WriteString("\n")
		for _, subtask := range task.Subtasks {
			b.WriteString("  ")
			b.WriteString(renderSubtask(subtask))
			b.WriteString("\n")
		}
		for _, session := range task.TimeSessions {
			b.WriteString("  ")
			b.WriteString(renderTimeSession(session))
			b.WriteString("\n")
		}
		for _, note := range task.Notes {
			b.WriteString("  ")
			b.WriteString(note)
			b.WriteString("\n")
		}
	}
	if len(l.Recurring) > 0 {
		b.WriteString("\n# Keel Recurring\n\n")
		for _, recurring := range l.Recurring {
			b.WriteString(recurring.CanonicalLine())
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderTimeSession(session ledger.TimeSession) string {
	parts := []string{"@time", "start:" + session.Start.Format(time.RFC3339)}
	if !session.End.IsZero() {
		parts = append(parts, "end:"+session.End.Format(time.RFC3339))
		parts = append(parts, fmt.Sprintf("duration:%d", int(session.End.Sub(session.Start).Seconds())))
	}
	return strings.Join(parts, " ")
}

func renderSubtask(subtask ledger.Subtask) string {
	check := " "
	status := subtask.Status
	if subtask.Done || status == ledger.StatusDone {
		check = "x"
		status = ledger.StatusDone
	}
	parts := []string{fmt.Sprintf("- [%s]", check), "sub:" + subtask.ID, "status:" + string(status)}
	if strings.TrimSpace(subtask.Title) != "" {
		parts = append(parts, strings.TrimSpace(subtask.Title))
	}
	return strings.Join(parts, " ")
}
