package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"keel/internal/config"
	"keel/internal/ledger"
	"keel/internal/store"
)

type filter int

const (
	filterAll filter = iota
	filterToday
	filterDone
	filterRecurring
)

var filterNames = []string{"All", "Today", "Done", "Recurring"}

type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch
	modeAdd
	modeAddSubtask
	modeAddRecurring
	modeEditTitle
	modeEditDue
	modeEditCategory
	modeEditTags
)

type saveMsg struct {
	snapshot store.Snapshot
	err      error
}

type tickMsg time.Time
type syncMsg struct {
	snapshot store.Snapshot
	err      error
}

type model struct {
	cfg      config.Config
	snapshot store.Snapshot
	styles   styles
	width    int
	height   int
	filter   filter
	cursor   int
	err      error
	status   string
	input    textinput.Model
	list     viewport.Model
	detail   viewport.Model
	mode     inputMode
	editID   string
	query    string
}

func New(snapshot store.Snapshot, cfg config.Config) tea.Model {
	input := textinput.New()
	input.Placeholder = "filter tasks"
	input.Prompt = "search "
	input.CharLimit = 120
	input.Width = 44

	return model{
		cfg:      cfg,
		snapshot: snapshot,
		styles:   newStyles(harbor),
		input:    input,
		list:     viewport.New(32, 8),
		detail:   viewport.New(32, 8),
		status:   "ready",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tick(), autosync(m.cfg.LedgerPath))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.Width = max(24, (msg.Width*2)/3)
		m.detail.Width = max(24, msg.Width/3)
		m.list.Height = max(6, msg.Height-10)
		m.detail.Height = max(6, msg.Height-10)
		return m, nil
	case saveMsg:
		if msg.err != nil {
			if errors.Is(msg.err, store.ErrConflict) {
				m.err = errors.New("ledger changed on disk; press r to reload before saving")
			} else {
				m.err = msg.err
			}
			m.status = "save failed"
			return m, nil
		}
		m.snapshot = msg.snapshot
		m.status = "saved"
		m.err = nil
		m.clampCursor()
		return m, nil
	case tickMsg:
		return m, tick()
	case syncMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "autosync failed"
			return m, autosync(m.cfg.LedgerPath)
		}
		if msg.snapshot.Hash != m.snapshot.Hash {
			m.snapshot = msg.snapshot
			m.status = "autosynced"
			m.err = nil
			m.clampCursor()
		}
		return m, autosync(m.cfg.LedgerPath)
	case tea.KeyMsg:
		if m.mode != modeNormal {
			return m.updateInput(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "h", "left":
			m.filter = filter((int(m.filter) + len(filterNames) - 1) % len(filterNames))
			m.cursor = 0
		case "l", "right":
			m.filter = filter((int(m.filter) + 1) % len(filterNames))
			m.cursor = 0
		case "j", "down":
			m.move(1)
		case "k", "up":
			m.move(-1)
		case "g":
			m.cursor = 0
		case "G":
			m.cursor = max(0, m.visibleCount()-1)
		case "/":
			m.beginInput(modeSearch, "search ", "filter tasks", m.query)
			return m, textinput.Blink
		case "a":
			if m.filter == filterRecurring {
				m.beginInput(modeAddRecurring, "recur ", "daily 09:00 Make pasta | weekly mon 10:00 Review | mon,wed 08:30 Stretch", "")
				return m, textinput.Blink
			}
			m.beginInput(modeAdd, "add ", "new task title", "")
			return m, textinput.Blink
		case "A":
			if task, ok := m.selectedTask(); ok {
				m.editID = task.ID
				m.beginInput(modeAddSubtask, "subtask ", "new subtask title", "")
				return m, textinput.Blink
			}
		case "e":
			if task, ok := m.selectedTask(); ok {
				m.editID = task.ID
				m.beginInput(modeEditTitle, "title ", "task title", task.Title)
				return m, textinput.Blink
			}
		case "u":
			if task, ok := m.selectedTask(); ok {
				m.editID = task.ID
				m.beginInput(modeEditDue, "due ", "YYYY-MM-DD or blank", task.Due)
				return m, textinput.Blink
			}
		case "c":
			if task, ok := m.selectedTask(); ok {
				m.editID = task.ID
				m.beginInput(modeEditCategory, "category ", "category name or blank", task.Category)
				return m, textinput.Blink
			}
		case "t":
			if task, ok := m.selectedTask(); ok {
				m.editID = task.ID
				m.beginInput(modeEditTags, "tags ", "comma or space separated tags", strings.Join(task.Tags, " "))
				return m, textinput.Blink
			}
		case "p":
			return m.cyclePriority()
		case "b":
			return m.setSelected(ledger.StatusBlocked)
		case "D":
			return m.setSelected(ledger.StatusDone)
		case "s":
			return m.cycleSelected()
		case "T":
			return m.toggleTimer()
		case "r":
			return m.reload()
		}
	}
	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.endInput()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		switch m.mode {
		case modeSearch:
			m.query = value
			m.cursor = 0
			m.endInput()
			return m, nil
		case modeAdd:
			m.endInput()
			if value == "" {
				return m, nil
			}
			task := ledger.Task{
				ID:       ledger.NewID(time.Now()),
				Title:    value,
				Status:   ledger.StatusTodo,
				Priority: ledger.PriorityP2,
			}
			m.snapshot.Ledger.Tasks = append(m.snapshot.Ledger.Tasks, task)
			m.filter = filterAll
			m.query = ""
			m.cursor = len(m.visibleTasks()) - 1
			m.status = "added"
			return m, m.save()
		case modeAddSubtask:
			id := m.editID
			m.endInput()
			if value == "" {
				return m, nil
			}
			return m.updateTask(id, func(task *ledger.Task) {
				task.Subtasks = append(task.Subtasks, ledger.Subtask{
					ID:     fmt.Sprintf("S-%d", len(task.Subtasks)+1),
					Title:  value,
					Status: ledger.StatusTodo,
				})
			})
		case modeAddRecurring:
			m.endInput()
			if value == "" {
				return m, nil
			}
			recurring, err := parseRecurringInput(value, time.Now())
			if err != nil {
				m.err = err
				m.status = "recurring rejected"
				return m, nil
			}
			m.snapshot.Ledger.Recurring = append(m.snapshot.Ledger.Recurring, recurring)
			m.cursor = len(m.snapshot.Ledger.Recurring) - 1
			m.status = "recurring added"
			return m, m.save()
		case modeEditTitle:
			id := m.editID
			m.endInput()
			if value == "" {
				m.status = "title unchanged"
				return m, nil
			}
			return m.updateTask(id, func(task *ledger.Task) {
				task.Title = value
			})
		case modeEditDue:
			id := m.editID
			m.endInput()
			if value != "" {
				if _, err := time.Parse("2006-01-02", value); err != nil {
					m.err = errors.New("due date must be YYYY-MM-DD")
					m.status = "edit rejected"
					return m, nil
				}
			}
			return m.updateTask(id, func(task *ledger.Task) {
				task.Due = value
			})
		case modeEditCategory:
			id := m.editID
			m.endInput()
			return m.updateTask(id, func(task *ledger.Task) {
				task.Category = sanitizeToken(value)
			})
		case modeEditTags:
			id := m.editID
			m.endInput()
			return m.updateTask(id, func(task *ledger.Task) {
				task.Tags = parseTagInput(value)
			})
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) beginInput(mode inputMode, prompt, placeholder, value string) {
	m.mode = mode
	m.input.Prompt = prompt
	m.input.Placeholder = placeholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
	m.err = nil
}

func (m *model) endInput() {
	m.mode = modeNormal
	m.editID = ""
	m.input.Blur()
	m.input.SetValue("")
	m.input.Prompt = "search "
	m.input.Placeholder = "filter tasks"
}

func (m model) View() string {
	if m.width == 0 {
		return "Keel"
	}
	var body string
	if len(m.snapshot.Errors) > 0 {
		body = m.renderErrors()
	} else {
		body = m.renderWorkbench()
	}
	parts := []string{m.header()}
	if timer := m.timerBar(); timer != "" {
		parts = append(parts, timer)
	}
	if input := m.inputBar(); input != "" {
		parts = append(parts, input)
	}
	parts = append(parts, body, m.footer())
	view := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return m.styles.root.Width(m.width).Height(m.height).Render(view)
}

func (m model) header() string {
	filters := make([]string, 0, len(filterNames))
	for i, name := range filterNames {
		count := m.countFor(filter(i))
		label := fmt.Sprintf("%s %d", name, count)
		if filter(i) == m.filter {
			filters = append(filters, m.styles.activeTab.Render(label))
		} else {
			filters = append(filters, m.styles.tab.Render(label))
		}
	}
	left := lipgloss.JoinHorizontal(lipgloss.Center, m.styles.mark.Render("KEEL"), m.styles.header.Render("shared task file"))
	right := m.styles.meta.Render(m.snapshot.Path)
	line := lipgloss.JoinHorizontal(lipgloss.Center, left, strings.Repeat(" ", max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right)-1)), right)
	return lipgloss.JoinVertical(lipgloss.Left, line, lipgloss.JoinHorizontal(lipgloss.Left, filters...))
}

func (m model) inputBar() string {
	if m.mode != modeNormal {
		return m.styles.input.Width(max(30, m.width-6)).Render(m.input.View())
	}
	if m.query == "" {
		return ""
	}
	return m.styles.input.Width(max(30, m.width-6)).Render("search " + m.query)
}

func (m model) timerBar() string {
	activeID, session, ok := m.snapshot.Ledger.ActiveTimer()
	if !ok {
		return ""
	}
	task, found := m.taskByID(activeID)
	title := activeID
	if found {
		title = task.Title
	}
	elapsed := time.Since(session.Start)
	label := fmt.Sprintf("Timer  %s  %s  press T on this task to stop", formatDuration(elapsed), title)
	return m.styles.timer.Width(max(30, m.width-2)).Render(label)
}

func (m model) footer() string {
	status := m.status
	if m.err != nil {
		status = m.styles.errorBox.Render(m.err.Error())
	}
	help := "h/l filter  j/k move  a task  A subtask  e title  u due  c category  t tags  p priority  s status  D done  T timer  / search  r reload  q quit"
	return lipgloss.JoinVertical(lipgloss.Left, m.styles.status.Render(status), m.styles.help.Render(help))
}

func (m model) renderWorkbench() string {
	if m.filter == filterRecurring {
		return m.renderRecurringWorkbench()
	}
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		message := "No tasks here."
		if m.query != "" {
			message = "No matches for " + m.query + "."
		}
		return m.styles.empty.Width(max(44, m.width-6)).Render(message + "\n\nPress a to capture work. The ledger lives at " + m.snapshot.Path + ".")
	}

	listWidth := max(44, (m.width*2)/3)
	content, selectionStart, selectionEnd := m.taskListContent(tasks, listWidth-4)
	m.list.Width = listWidth
	m.list.Height = max(6, m.height-10)
	m.list.SetContent(content)
	m.ensureSelectionVisible(selectionStart, selectionEnd)

	detail := m.renderDetail(tasks[min(m.cursor, len(tasks)-1)])
	return lipgloss.JoinHorizontal(lipgloss.Top, m.styles.column.Width(listWidth).Render(m.list.View()), detail)
}

func (m model) renderRecurringWorkbench() string {
	items := m.visibleRecurring()
	if len(items) == 0 {
		return m.styles.empty.Width(max(44, m.width-6)).Render("No recurring tasks yet.\n\nPress a to add one. Examples: daily 09:00 Make pasta, weekly mon 10:00 Review, mon,wed,fri 08:30 Stretch.")
	}
	listWidth := max(44, (m.width*2)/3)
	content, selectionStart, selectionEnd := m.recurringListContent(items, listWidth-4)
	m.list.Width = listWidth
	m.list.Height = max(6, m.height-10)
	m.list.SetContent(content)
	m.ensureSelectionVisible(selectionStart, selectionEnd)
	detail := m.renderRecurringDetail(items[min(m.cursor, len(items)-1)])
	return lipgloss.JoinHorizontal(lipgloss.Top, m.styles.column.Width(listWidth).Render(m.list.View()), detail)
}

func (m model) recurringCard(recurring ledger.RecurringTask, selected bool, width int) string {
	meta := fmt.Sprintf("%s  %s", recurring.ID, recurringSchedule(recurring))
	if recurring.Priority != "" {
		meta += "  " + string(recurring.Priority)
	}
	if recurring.Category != "" {
		meta += "  " + recurring.Category
	}
	if len(recurring.Tags) > 0 {
		meta += "  +" + strings.Join(recurring.Tags, " +")
	}
	style := m.styles.card.Width(width)
	if selected {
		style = m.styles.selected.Width(width)
	}
	return style.Render(recurring.Title + "\n" + m.styles.meta.Render(meta))
}

func (m model) renderRecurringDetail(recurring ledger.RecurringTask) string {
	var b strings.Builder
	b.WriteString(recurring.ID + "\n")
	b.WriteString(recurring.Title + "\n\n")
	b.WriteString(fmt.Sprintf("schedule %s\n", recurringSchedule(recurring)))
	b.WriteString(fmt.Sprintf("priority %s\n", recurring.Priority))
	b.WriteString(fmt.Sprintf("category %s\n", blank(recurring.Category)))
	if len(recurring.Tags) > 0 {
		b.WriteString("tags     +" + strings.Join(recurring.Tags, " +") + "\n")
	}
	if len(recurring.DueTodayTasks(time.Now())) > 0 {
		b.WriteString("\ntoday    appears in All and Today\n")
	}
	b.WriteString("\nRecurring definitions are edited in the ledger for now. Full file: keel edit.")
	m.detail.SetContent(b.String())
	return m.styles.column.Width(max(24, m.width/3)).Render(m.detail.View())
}

func (m model) groupedTaskRows(tasks []ledger.Task, width int) []string {
	var rows []string
	now := time.Now()

	// Separate overdue from regular tasks
	var overdueTasks, regularTasks []ledger.Task
	for _, task := range tasks {
		if task.IsOverdue(now) {
			overdueTasks = append(overdueTasks, task)
		} else {
			regularTasks = append(regularTasks, task)
		}
	}

	taskIndex := 0

	// Overdue section at the top with warning-style header
	if len(overdueTasks) > 0 {
		rows = append(rows, m.styles.overdue.Width(width).Render("Overdue"))
		for _, task := range overdueTasks {
			rows = append(rows, m.taskCard(task, taskIndex == m.cursor, width))
			taskIndex++
		}
	}

	// Regular tasks grouped by date, then priority
	lastDate := ""
	lastPriority := ledger.Priority("")
	for _, task := range regularTasks {
		dateLabel := dueGroupLabel(task.Due)
		if dateLabel != lastDate {
			rows = append(rows, m.styles.status.Width(width).Render(dateLabel))
			lastDate = dateLabel
			lastPriority = ""
		}
		if task.Priority != lastPriority {
			rows = append(rows, m.styles.meta.Width(width).Render(priorityLabel(task.Priority)))
			lastPriority = task.Priority
		}
		rows = append(rows, m.taskCard(task, taskIndex == m.cursor, width))
		taskIndex++
	}
	return rows
}

func (m model) doneGroupedTaskRows(tasks []ledger.Task, width int) []string {
	var rows []string
	if len(tasks) == 0 {
		return rows
	}

	// Group tasks by due date, preserving task order (already sorted newest-first)
	dateMap := make(map[string][]ledger.Task)
	var dateOrder []string
	for _, task := range tasks {
		due := task.Due
		if _, ok := dateMap[due]; !ok {
			dateOrder = append(dateOrder, due)
		}
		dateMap[due] = append(dateMap[due], task)
	}

	// Sort date groups descending (already sorted by sortTasks, but ensure group order)
	sort.SliceStable(dateOrder, func(i, j int) bool {
		if dateOrder[i] == "" {
			return false
		}
		if dateOrder[j] == "" {
			return true
		}
		return dateOrder[i] > dateOrder[j]
	})

	// Render each date group
	taskIndex := 0
	for _, date := range dateOrder {
		groupTasks := dateMap[date]

		// Compute total tracked time for this day
		var dayTotal time.Duration
		for _, t := range groupTasks {
			dayTotal += t.TrackedDuration(time.Now())
		}

		// Date header with day total
		label := doneGroupLabel(date, dayTotal)
		rows = append(rows, m.styles.status.Width(width).Render(label))

		for _, task := range groupTasks {
			rows = append(rows, m.taskCard(task, taskIndex == m.cursor, width))
			taskIndex++
		}
	}

	return rows
}

func doneGroupLabel(due string, total time.Duration) string {
	totalStr := formatDuration(total)
	if due == "" {
		return "No date  (" + totalStr + ")"
	}
	parsed, err := time.Parse("2006-01-02", due)
	if err != nil {
		return due + "  (" + totalStr + ")"
	}
	today := time.Now().Format("2006-01-02")
	label := parsed.Format("2 Jan 2006")
	if due == today {
		label = "Today — " + label
	}
	if total > 0 {
		return fmt.Sprintf("%s  (%s)", label, totalStr)
	}
	return label
}

func (m model) renderErrors() string {
	var rows []string
	rows = append(rows, m.styles.errorBox.Render("Ledger needs attention"))
	for _, err := range m.snapshot.Errors {
		rows = append(rows, m.styles.card.Width(max(40, m.width-6)).Render(err.Error()))
	}
	rows = append(rows, m.styles.help.Render("Run keel doctor, or keel doctor --fix after reviewing what will be preserved."))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m model) renderDetail(task ledger.Task) string {
	var b strings.Builder
	b.WriteString(task.ID + "\n")
	b.WriteString(task.Title + "\n\n")
	b.WriteString(fmt.Sprintf("status   %s\n", task.Status))
	b.WriteString(fmt.Sprintf("priority %s\n", task.Priority))
	b.WriteString(fmt.Sprintf("due      %s\n", blank(task.Due)))
	b.WriteString(fmt.Sprintf("category %s\n", blank(task.Category)))
	b.WriteString(fmt.Sprintf("tracked  %s\n", formatDuration(task.TrackedDuration(time.Now()))))
	if task.IsOverdue(time.Now()) {
		b.WriteString("overdue  !!  \n")
	}
	if session, ok := task.ActiveSession(); ok {
		b.WriteString(fmt.Sprintf("timer    running since %s\n", session.Start.Format("15:04")))
	}
	if len(task.Tags) > 0 {
		b.WriteString("tags     +" + strings.Join(task.Tags, " +") + "\n")
	}
	if len(task.Subtasks) > 0 {
		b.WriteString("\nsubtasks\n")
		for _, subtask := range task.Subtasks {
			check := " "
			if subtask.Done || subtask.Status == ledger.StatusDone {
				check = "x"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s\n", check, subtask.Title))
		}
	}
	if len(task.Notes) > 0 {
		b.WriteString("\nnotes\n")
		for _, note := range task.Notes {
			b.WriteString("- " + note + "\n")
		}
	}
	b.WriteString("\nEdit: e title, u due, c category, t tags, A subtask. Timer: T start/stop. Full file: keel edit.")
	m.detail.SetContent(b.String())
	return m.styles.column.Width(max(24, m.width/3)).Render(m.detail.View())
}

func (m model) taskCard(task ledger.Task, selected bool, width int) string {
	metaStyle := m.styles.meta
	tagStyle := m.styles.tag
	if selected {
		metaStyle = m.styles.metaSelected
		tagStyle = m.styles.metaSelected
	}

	var segments []string

	segments = append(segments, metaStyle.Render(task.ID))
	segments = append(segments, metaStyle.Render(string(task.Status)))
	segments = append(segments, metaStyle.Render(string(task.Priority)))

	if task.IsOverdue(time.Now()) {
		segments = append(segments, m.styles.overdue.Render("OVERDUE"))
	}

	if task.Due != "" {
		segments = append(segments, metaStyle.Render("due "+task.Due))
	}
	if task.Category != "" && !selected {
		segments = append(segments, m.styles.tagPill.Render(task.Category))
	} else if task.Category != "" {
		segments = append(segments, metaStyle.Render(task.Category))
	}
	if _, ok := task.ActiveSession(); ok {
		segments = append(segments, metaStyle.Render("timer "+formatDuration(task.TrackedDuration(time.Now()))))
	} else if tracked := task.TrackedDuration(time.Now()); tracked > 0 {
		if !selected {
			segments = append(segments, tagStyle.Render("tracked "+formatDuration(tracked)))
		} else {
			segments = append(segments, metaStyle.Render("tracked "+formatDuration(tracked)))
		}
	}
	for _, tag := range task.Tags {
		segments = append(segments, tagStyle.Render("+"+tag))
	}
	if len(task.Subtasks) > 0 {
		segments = append(segments, metaStyle.Render(fmt.Sprintf("%d subtasks", len(task.Subtasks))))
	}

	metaLine := lipgloss.JoinHorizontal(lipgloss.Left, segments...)

	style := m.styles.card.Width(width)
	if selected {
		style = m.styles.selected.Width(width)
	}
	return style.Render(task.Title + "\n" + metaLine)
}

func (m model) visibleTasks() []ledger.Task {
	base := m.tasksFor(m.filter)
	query := strings.ToLower(strings.TrimSpace(m.query))
	if query == "" {
		return base
	}
	var tasks []ledger.Task
	for _, task := range base {
		if strings.Contains(strings.ToLower(taskSearchText(task)), query) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (m model) visibleRecurring() []ledger.RecurringTask {
	recurring := m.snapshot.Ledger.Recurring
	query := strings.ToLower(strings.TrimSpace(m.query))
	if query == "" {
		return recurring
	}
	var items []ledger.RecurringTask
	for _, item := range recurring {
		if strings.Contains(strings.ToLower(recurringSearchText(item)), query) {
			items = append(items, item)
		}
	}
	return items
}

func (m model) tasksFor(filter filter) []ledger.Task {
	tasks := m.rawTasksFor(filter)
	sortTasks(tasks, filter)
	return tasks
}

func (m model) rawTasksFor(filter filter) []ledger.Task {
	now := time.Now()
	switch filter {
	case filterToday:
		return append(m.todayTasks(), m.recurringDueTasks(now)...)
	case filterDone:
		return m.snapshot.Ledger.NextByStatus(ledger.StatusDone)
	default:
		var tasks []ledger.Task
		for _, task := range m.snapshot.Ledger.Tasks {
			if task.Status != ledger.StatusDone {
				tasks = append(tasks, task)
			}
		}
		tasks = append(tasks, m.recurringDueTasks(now)...)
		return tasks
	}
}

func (m model) countFor(filter filter) int {
	if filter == filterRecurring {
		return len(m.visibleRecurring())
	}
	return len(m.tasksFor(filter))
}

func (m *model) move(delta int) {
	count := m.visibleCount()
	if count == 0 {
		m.cursor = 0
		return
	}
	m.cursor = min(max(0, m.cursor+delta), count-1)
}

func (m *model) clampCursor() {
	m.cursor = min(max(0, m.cursor), max(0, m.visibleCount()-1))
}

func (m model) visibleCount() int {
	if m.filter == filterRecurring {
		return len(m.visibleRecurring())
	}
	return len(m.visibleTasks())
}

func (m model) cycleSelected() (tea.Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	next := ledger.StatusDoing
	switch task.Status {
	case ledger.StatusDoing:
		next = ledger.StatusDone
	case ledger.StatusDone:
		next = ledger.StatusTodo
	case ledger.StatusBlocked:
		next = ledger.StatusTodo
	}
	return m.updateTask(task.ID, func(task *ledger.Task) {
		task.Status = next
	})
}

func (m model) cyclePriority() (tea.Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	next := ledger.PriorityP1
	switch task.Priority {
	case ledger.PriorityP0:
		next = ledger.PriorityP1
	case ledger.PriorityP1:
		next = ledger.PriorityP2
	case ledger.PriorityP2:
		next = ledger.PriorityP3
	default:
		next = ledger.PriorityP0
	}
	return m.updateTask(task.ID, func(task *ledger.Task) {
		task.Priority = next
	})
}

func (m model) toggleTimer() (tea.Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	now := time.Now().Truncate(time.Second)
	activeID, _, active := m.snapshot.Ledger.ActiveTimer()
	if active && activeID != task.ID {
		m.err = fmt.Errorf("timer already running on %s; select it and press T to stop", activeID)
		m.status = "timer unchanged"
		return m, nil
	}
	return m.updateTask(task.ID, func(task *ledger.Task) {
		for i := range task.TimeSessions {
			if task.TimeSessions[i].End.IsZero() {
				task.TimeSessions[i].End = now
				return
			}
		}
		task.TimeSessions = append(task.TimeSessions, ledger.TimeSession{Start: now})
		if task.Status == ledger.StatusTodo {
			task.Status = ledger.StatusDoing
		}
	})
}

func (m model) setSelected(status ledger.Status) (tea.Model, tea.Cmd) {
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	return m.updateTask(task.ID, func(task *ledger.Task) {
		task.Status = status
	})
}

func (m model) selectedTask() (ledger.Task, bool) {
	if m.filter == filterRecurring {
		return ledger.Task{}, false
	}
	tasks := m.visibleTasks()
	if len(tasks) == 0 || m.cursor >= len(tasks) {
		return ledger.Task{}, false
	}
	return tasks[m.cursor], true
}

func (m model) taskByID(id string) (ledger.Task, bool) {
	for _, task := range m.snapshot.Ledger.Tasks {
		if task.ID == id {
			return task, true
		}
	}
	return ledger.Task{}, false
}

func (m model) updateTask(id string, change func(*ledger.Task)) (tea.Model, tea.Cmd) {
	for i := range m.snapshot.Ledger.Tasks {
		if m.snapshot.Ledger.Tasks[i].ID == id {
			change(&m.snapshot.Ledger.Tasks[i])
			m.status = "edited"
			m.err = nil
			return m, m.save()
		}
	}
	m.status = "task not found"
	return m, nil
}

func (m model) save() tea.Cmd {
	path := m.cfg.LedgerPath
	l := m.snapshot.Ledger
	hash := m.snapshot.Hash
	return func() tea.Msg {
		if err := store.Save(path, l, hash); err != nil {
			return saveMsg{err: err}
		}
		snap, err := store.Load(path)
		return saveMsg{snapshot: snap, err: err}
	}
}

func (m model) reload() (tea.Model, tea.Cmd) {
	path := m.cfg.LedgerPath
	return m, func() tea.Msg {
		snap, err := store.Load(path)
		return saveMsg{snapshot: snap, err: err}
	}
}

func (m model) todayTasks() []ledger.Task {
	now := time.Now()
	var tasks []ledger.Task
	for _, task := range m.snapshot.Ledger.Tasks {
		if task.IsDueToday(now) || task.Status == ledger.StatusDoing {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (m model) recurringDueTasks(now time.Time) []ledger.Task {
	var tasks []ledger.Task
	for _, recurring := range m.snapshot.Ledger.Recurring {
		tasks = append(tasks, recurring.DueTodayTasks(now)...)
	}
	return tasks
}

func (m *model) ensureSelectionVisible(start, end int) {
	if m.list.Height <= 0 {
		return
	}
	if start < m.list.YOffset {
		m.list.SetYOffset(start)
		return
	}
	bottom := m.list.YOffset + m.list.Height
	if end > bottom {
		m.list.SetYOffset(max(0, end-m.list.Height))
	}
}

func (m model) taskListContent(tasks []ledger.Task, width int) (string, int, int) {
	var b strings.Builder
	b.WriteString(m.styles.header.Render(m.listTitle()) + "\n")
	selectionStart, selectionEnd := 0, 0
	line := 1
	if (m.filter == filterAll || m.filter == filterDone) && m.query == "" {
		rows := m.groupedTaskRowsWithRanges(tasks, width)
		for i, row := range rows {
			if i > 0 {
				b.WriteString("\n")
				line++
			}
			if row.selected {
				selectionStart = line
				selectionEnd = line + row.height
			}
			b.WriteString(row.rendered)
			line += row.height - 1
		}
	} else {
		for i, task := range tasks {
			if i > 0 {
				b.WriteString("\n")
				line++
			}
			rendered := m.taskCard(task, i == m.cursor, width)
			height := lipgloss.Height(rendered)
			if i == m.cursor {
				selectionStart = line
				selectionEnd = line + height
			}
			b.WriteString(rendered)
			line += height - 1
		}
	}
	return b.String(), selectionStart, selectionEnd
}

func (m model) recurringListContent(items []ledger.RecurringTask, width int) (string, int, int) {
	var b strings.Builder
	b.WriteString(m.styles.header.Render("Recurring tasks") + "\n")
	selectionStart, selectionEnd := 0, 0
	line := 1
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n")
			line++
		}
		rendered := m.recurringCard(item, i == m.cursor, width)
		height := lipgloss.Height(rendered)
		if i == m.cursor {
			selectionStart = line
			selectionEnd = line + height
		}
		b.WriteString(rendered)
		line += height - 1
	}
	return b.String(), selectionStart, selectionEnd
}

func (m model) listTitle() string {
	title := fmt.Sprintf("%s tasks", filterNames[m.filter])
	if m.query != "" {
		title += " matching " + m.query
	}
	return title
}

type renderedRow struct {
	rendered string
	height   int
	selected bool
}

func (m model) groupedTaskRowsWithRanges(tasks []ledger.Task, width int) []renderedRow {
	var rows []renderedRow
	now := time.Now()
	var overdueTasks, regularTasks []ledger.Task
	for _, task := range tasks {
		if task.IsOverdue(now) {
			overdueTasks = append(overdueTasks, task)
		} else {
			regularTasks = append(regularTasks, task)
		}
	}
	taskIndex := 0
	if len(overdueTasks) > 0 {
		r := m.styles.overdue.Width(width).Render("Overdue")
		rows = append(rows, renderedRow{rendered: r, height: lipgloss.Height(r)})
		for _, task := range overdueTasks {
			r := m.taskCard(task, taskIndex == m.cursor, width)
			rows = append(rows, renderedRow{rendered: r, height: lipgloss.Height(r), selected: taskIndex == m.cursor})
			taskIndex++
		}
	}
	lastDate := ""
	lastPriority := ledger.Priority("")
	for _, task := range regularTasks {
		dateLabel := dueGroupLabel(task.Due)
		if dateLabel != lastDate {
			r := m.styles.status.Width(width).Render(dateLabel)
			rows = append(rows, renderedRow{rendered: r, height: lipgloss.Height(r)})
			lastDate = dateLabel
			lastPriority = ""
		}
		if task.Priority != lastPriority {
			r := m.styles.meta.Width(width).Render(priorityLabel(task.Priority))
			rows = append(rows, renderedRow{rendered: r, height: lipgloss.Height(r)})
			lastPriority = task.Priority
		}
		r := m.taskCard(task, taskIndex == m.cursor, width)
		rows = append(rows, renderedRow{rendered: r, height: lipgloss.Height(r), selected: taskIndex == m.cursor})
		taskIndex++
	}
	return rows
}

func blank(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func autosync(path string) tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		snap, err := store.Load(path)
		return syncMsg{snapshot: snap, err: err}
	})
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	totalMinutes := int(duration.Round(time.Minute).Minutes())
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	if minutes == 0 && duration > 0 {
		return "<1m"
	}
	return fmt.Sprintf("%dm", minutes)
}

func sortTasks(tasks []ledger.Task, filter filter) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a := tasks[i]
		b := tasks[j]
		if filter == filterDone {
			// Done tasks: newest first, no-date at the bottom
			if a.Due == "" && b.Due == "" {
				return strings.ToLower(a.Title) < strings.ToLower(b.Title)
			}
			if a.Due == "" {
				return false
			}
			if b.Due == "" {
				return true
			}
			if a.Due != b.Due {
				return a.Due > b.Due
			}
			if priorityRank(a.Priority) != priorityRank(b.Priority) {
				return priorityRank(a.Priority) < priorityRank(b.Priority)
			}
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		}
		if filter == filterAll {
			if dueSortKey(a.Due) != dueSortKey(b.Due) {
				return dueSortKey(a.Due) < dueSortKey(b.Due)
			}
			if priorityRank(a.Priority) != priorityRank(b.Priority) {
				return priorityRank(a.Priority) < priorityRank(b.Priority)
			}
		}
		if priorityRank(a.Priority) != priorityRank(b.Priority) {
			return priorityRank(a.Priority) < priorityRank(b.Priority)
		}
		return strings.ToLower(a.Title) < strings.ToLower(b.Title)
	})
}

func dueSortKey(due string) string {
	if due == "" {
		return "9999-99-99"
	}
	return due
}

func dueGroupLabel(due string) string {
	if due == "" {
		return "No date"
	}
	parsed, err := time.Parse("2006-01-02", due)
	if err != nil {
		return due
	}
	return fmt.Sprintf("%d/%d/%02d - what to do?", parsed.Day(), int(parsed.Month()), parsed.Year()%100)
}

func priorityRank(priority ledger.Priority) int {
	switch priority {
	case ledger.PriorityP0:
		return 0
	case ledger.PriorityP1:
		return 1
	case ledger.PriorityP2:
		return 2
	case ledger.PriorityP3:
		return 3
	default:
		return 4
	}
}

func priorityLabel(priority ledger.Priority) string {
	switch priority {
	case ledger.PriorityP0:
		return "Priority 0"
	case ledger.PriorityP1:
		return "Priority 1"
	case ledger.PriorityP2:
		return "Priority 2"
	case ledger.PriorityP3:
		return "Priority 3"
	default:
		return "Priority"
	}
}

func recurringSchedule(recurring ledger.RecurringTask) string {
	parts := []string{string(recurring.Kind)}
	if len(recurring.Days) > 0 {
		parts = append(parts, dayList(recurring.Days))
	}
	if len(recurring.Times) > 0 {
		parts = append(parts, strings.Join(recurring.Times, ","))
	}
	return strings.Join(parts, " ")
}

func dayList(days []time.Weekday) string {
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

func parseRecurringInput(input string, now time.Time) (ledger.RecurringTask, error) {
	fields := strings.Fields(input)
	if len(fields) < 2 {
		return ledger.RecurringTask{}, fmt.Errorf("use: daily 09:00 Title, weekly mon 09:00 Title, or mon,wed 09:00 Title")
	}
	recurring := ledger.RecurringTask{
		ID:       ledger.NewRecurringID(now),
		Kind:     ledger.RecurrenceCustom,
		Priority: ledger.PriorityP2,
	}
	index := 0
	switch strings.ToLower(fields[0]) {
	case "daily", "day", "everyday":
		recurring.Kind = ledger.RecurrenceDaily
		index = 1
	case "weekly", "week":
		recurring.Kind = ledger.RecurrenceWeekly
		if len(fields) < 3 {
			return ledger.RecurringTask{}, fmt.Errorf("weekly recurring tasks need a day and title")
		}
		days, err := parseInputDays(fields[1])
		if err != nil {
			return ledger.RecurringTask{}, err
		}
		recurring.Days = days
		index = 2
	default:
		days, err := parseInputDays(fields[0])
		if err != nil {
			return ledger.RecurringTask{}, err
		}
		recurring.Days = days
		index = 1
	}
	if index < len(fields) && looksLikeClock(fields[index]) {
		recurring.Times = strings.Split(fields[index], ",")
		index++
	}
	if index >= len(fields) {
		return ledger.RecurringTask{}, fmt.Errorf("recurring task needs a title")
	}
	for _, at := range recurring.Times {
		if _, err := time.Parse("15:04", at); err != nil {
			return ledger.RecurringTask{}, fmt.Errorf("times must use HH:MM")
		}
	}
	recurring.Title = strings.Join(fields[index:], " ")
	return recurring, nil
}

func parseInputDays(raw string) ([]time.Weekday, error) {
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
			return nil, fmt.Errorf("unknown day %q", part)
		}
	}
	return days, nil
}

func looksLikeClock(value string) bool {
	for _, part := range strings.Split(value, ",") {
		if _, err := time.Parse("15:04", part); err != nil {
			return false
		}
	}
	return true
}

func parseTagInput(input string) []string {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	var tags []string
	seen := map[string]bool{}
	for _, field := range fields {
		tag := sanitizeToken(strings.TrimPrefix(strings.TrimSpace(field), "+"))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return tags
}

func sanitizeToken(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "category:"))
	value = strings.Trim(value, ",")
	value = strings.ReplaceAll(value, " ", "-")
	return strings.Trim(value, "+")
}

func taskSearchText(task ledger.Task) string {
	parts := []string{task.CanonicalLine(), task.Category, strings.Join(task.Tags, " ")}
	for _, tag := range task.Tags {
		parts = append(parts, "#"+tag, "+"+tag)
	}
	for _, subtask := range task.Subtasks {
		parts = append(parts, subtask.Title, string(subtask.Status))
	}
	parts = append(parts, task.Notes...)
	return strings.Join(parts, " ")
}

func recurringSearchText(recurring ledger.RecurringTask) string {
	parts := []string{recurring.CanonicalLine(), recurring.Category, strings.Join(recurring.Tags, " "), recurringSchedule(recurring)}
	for _, tag := range recurring.Tags {
		parts = append(parts, "#"+tag, "+"+tag)
	}
	return strings.Join(parts, " ")
}
