package tui

import "github.com/charmbracelet/lipgloss"

type theme struct {
	bg      lipgloss.Color
	panel   lipgloss.Color
	ink     lipgloss.Color
	muted   lipgloss.Color
	copper  lipgloss.Color
	mint    lipgloss.Color
	warning lipgloss.Color
	danger  lipgloss.Color
	steel   lipgloss.Color
	pillBg  lipgloss.Color
}

var harbor = theme{
	bg:      lipgloss.Color("#101412"),
	panel:   lipgloss.Color("#18201d"),
	ink:     lipgloss.Color("#e6e0d0"),
	muted:   lipgloss.Color("#89958b"),
	copper:  lipgloss.Color("#d68655"),
	mint:    lipgloss.Color("#89c7a4"),
	warning: lipgloss.Color("#e4b363"),
	danger:  lipgloss.Color("#dd6b6b"),
	steel:   lipgloss.Color("#84a3b8"),
	pillBg:  lipgloss.Color("#1f2a26"),
}

type styles struct {
	root         lipgloss.Style
	header       lipgloss.Style
	mark         lipgloss.Style
	tab          lipgloss.Style
	activeTab    lipgloss.Style
	column       lipgloss.Style
	columnHot    lipgloss.Style
	card         lipgloss.Style
	selected     lipgloss.Style
	meta         lipgloss.Style
	metaSelected lipgloss.Style
	tag          lipgloss.Style
	tagPill      lipgloss.Style
	status       lipgloss.Style
	overdue      lipgloss.Style
	timer        lipgloss.Style
	help         lipgloss.Style
	empty        lipgloss.Style
	input        lipgloss.Style
	errorBox     lipgloss.Style
}

func newStyles(t theme) styles {
	return styles{
		root: lipgloss.NewStyle().
			Background(t.bg).
			Foreground(t.ink),
		header: lipgloss.NewStyle().
			Foreground(t.ink).
			Bold(true).
			Padding(0, 1),
		mark: lipgloss.NewStyle().
			Foreground(t.bg).
			Background(t.copper).
			Bold(true).
			Padding(0, 1),
		tab: lipgloss.NewStyle().
			Foreground(t.muted).
			Padding(0, 1),
		activeTab: lipgloss.NewStyle().
			Foreground(t.bg).
			Background(t.ink).
			Bold(true).
			Padding(0, 1),
		column: lipgloss.NewStyle().
			Background(t.panel).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(t.muted).
			Padding(0, 1),
		columnHot: lipgloss.NewStyle().
			Background(t.panel).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(t.copper).
			Padding(0, 1),
		card: lipgloss.NewStyle().
			Foreground(t.ink).
			Background(t.panel).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(t.bg).
			Padding(0, 1).
			MarginBottom(0),
		selected: lipgloss.NewStyle().
			Foreground(t.bg).
			Background(t.mint).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(t.mint).
			Padding(0, 1).
			MarginBottom(0),
		meta: lipgloss.NewStyle().
			Foreground(t.muted),
		metaSelected: lipgloss.NewStyle().
			Foreground(t.mint).
			Faint(true),
		tag: lipgloss.NewStyle().
			Foreground(t.copper),
		tagPill: lipgloss.NewStyle().
			Foreground(t.copper).
			Background(t.pillBg),
		status: lipgloss.NewStyle().
			Foreground(t.steel),
		overdue: lipgloss.NewStyle().
			Foreground(t.danger).
			Bold(true),
		timer: lipgloss.NewStyle().
			Foreground(t.bg).
			Background(t.warning).
			Bold(true).
			Padding(0, 1).
			MarginTop(1),
		help: lipgloss.NewStyle().
			Foreground(t.muted).
			Padding(0, 1),
		empty: lipgloss.NewStyle().
			Foreground(t.muted).
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(t.panel).
			Padding(1, 2),
		input: lipgloss.NewStyle().
			Foreground(t.ink).
			Background(t.panel).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(t.copper).
			Padding(0, 1),
		errorBox: lipgloss.NewStyle().
			Foreground(t.ink).
			Background(t.danger).
			Padding(0, 1),
	}
}
