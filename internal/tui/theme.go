package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

type theme struct {
	Name string

	Title     lipgloss.Style
	Meta      lipgloss.Style
	Hint      lipgloss.Style
	Prompt    lipgloss.Style
	User      lipgloss.Style
	Assistant lipgloss.Style
	Accent    lipgloss.Style
	Error     lipgloss.Style

	UserMark      lipgloss.Style
	AssistantMark lipgloss.Style
	BorderColor   lipgloss.AdaptiveColor

	CmdItem     lipgloss.Style
	CmdSelected lipgloss.Style

	HeaderBar    lipgloss.Style
	StatusBar    lipgloss.Style
	PromptBox    lipgloss.Style
	PromptBorder lipgloss.Border

	UserGlyph      string
	AssistantGlyph string
	SelectGlyph    string

	Spinner spinner.Spinner
}

var currentTheme = defaultThemes()[0]

func defaultThemes() []theme {
	return []theme{
		darkTheme(),
		lightTheme(),
		draculaTheme(),
	}
}

func darkTheme() theme {
	accent := lipgloss.Color("#DA7756")
	fgDim := lipgloss.Color("#6B6B6B")
	user := lipgloss.Color("#DA7756")
	assistant := lipgloss.Color("#D4D4D4")
	errC := lipgloss.Color("#EF4444")
	border := lipgloss.AdaptiveColor{Light: "#404040", Dark: "#333333"}
	headerBg := lipgloss.AdaptiveColor{Light: "#E5E5E5", Dark: "#1A1A1A"}
	statusBg := lipgloss.AdaptiveColor{Light: "#E5E5E5", Dark: "#1A1A1A"}

	return theme{
		Name:      "Dark",
		Title:     lipgloss.NewStyle().Bold(true).Foreground(accent),
		Meta:      lipgloss.NewStyle().Foreground(fgDim),
		Hint:      lipgloss.NewStyle().Foreground(fgDim),
		Prompt:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		User:      lipgloss.NewStyle().Bold(true).Foreground(user),
		Assistant: lipgloss.NewStyle().Foreground(assistant),
		Accent:    lipgloss.NewStyle().Foreground(accent),
		Error:     lipgloss.NewStyle().Foreground(errC).Bold(true),

		UserMark:      lipgloss.NewStyle().Foreground(user).Bold(true),
		AssistantMark: lipgloss.NewStyle().Foreground(fgDim),
		BorderColor:   border,

		CmdItem:     lipgloss.NewStyle().Foreground(fgDim),
		CmdSelected: lipgloss.NewStyle().Foreground(accent).Bold(true),

		HeaderBar:    lipgloss.NewStyle().Background(headerBg).Padding(0, 1),
		StatusBar:    lipgloss.NewStyle().Background(statusBg).Padding(0, 1),
		PromptBox:    lipgloss.NewStyle(),
		PromptBorder: lipgloss.RoundedBorder(),

		UserGlyph:      "> ",
		AssistantGlyph: "  ",
		SelectGlyph:    "> ",

		Spinner: spinner.Line,
	}
}

func lightTheme() theme {
	accent := lipgloss.Color("#B35C37")
	fg := lipgloss.Color("#1A1A1A")
	fgDim := lipgloss.Color("#8C8C8C")
	user := lipgloss.Color("#B35C37")
	assistant := lipgloss.Color("#333333")
	errC := lipgloss.Color("#DC2626")
	border := lipgloss.AdaptiveColor{Light: "#C0C0C0", Dark: "#C0C0C0"}
	headerBg := lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#F0F0F0"}
	statusBg := lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#F0F0F0"}

	return theme{
		Name:      "Light",
		Title:     lipgloss.NewStyle().Bold(true).Foreground(accent),
		Meta:      lipgloss.NewStyle().Foreground(fgDim),
		Hint:      lipgloss.NewStyle().Foreground(fgDim),
		Prompt:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		User:      lipgloss.NewStyle().Bold(true).Foreground(user),
		Assistant: lipgloss.NewStyle().Foreground(assistant),
		Accent:    lipgloss.NewStyle().Foreground(accent),
		Error:     lipgloss.NewStyle().Foreground(errC).Bold(true),

		UserMark:      lipgloss.NewStyle().Foreground(user).Bold(true),
		AssistantMark: lipgloss.NewStyle().Foreground(fgDim),
		BorderColor:   border,

		CmdItem:     lipgloss.NewStyle().Foreground(fgDim),
		CmdSelected: lipgloss.NewStyle().Foreground(accent).Bold(true),

		HeaderBar:    lipgloss.NewStyle().Background(headerBg).Foreground(fg).Padding(0, 1),
		StatusBar:    lipgloss.NewStyle().Background(statusBg).Foreground(fg).Padding(0, 1),
		PromptBox:    lipgloss.NewStyle(),
		PromptBorder: lipgloss.RoundedBorder(),

		UserGlyph:      "> ",
		AssistantGlyph: "  ",
		SelectGlyph:    "> ",

		Spinner: spinner.Line,
	}
}

func draculaTheme() theme {
	purple := lipgloss.Color("#BD93F9")
	green := lipgloss.Color("#50FA7B")
	pink := lipgloss.Color("#FF79C6")
	cyan := lipgloss.Color("#8BE9FD")
	red := lipgloss.Color("#FF5555")
	comment := lipgloss.Color("#6272A4")

	border := lipgloss.AdaptiveColor{Light: "#6272A4", Dark: "#44475A"}
	headerBg := lipgloss.AdaptiveColor{Light: "#E8E8F0", Dark: "#21222C"}
	statusBg := lipgloss.AdaptiveColor{Light: "#E8E8F0", Dark: "#21222C"}

	return theme{
		Name:      "Dracula",
		Title:     lipgloss.NewStyle().Bold(true).Foreground(purple),
		Meta:      lipgloss.NewStyle().Foreground(comment),
		Hint:      lipgloss.NewStyle().Foreground(comment),
		Prompt:    lipgloss.NewStyle().Foreground(green).Bold(true),
		User:      lipgloss.NewStyle().Bold(true).Foreground(green),
		Assistant: lipgloss.NewStyle().Foreground(cyan),
		Accent:    lipgloss.NewStyle().Foreground(purple),
		Error:     lipgloss.NewStyle().Foreground(red).Bold(true),

		UserMark:      lipgloss.NewStyle().Foreground(pink).Bold(true),
		AssistantMark: lipgloss.NewStyle().Foreground(purple),
		BorderColor:   border,

		CmdItem:     lipgloss.NewStyle().Foreground(comment),
		CmdSelected: lipgloss.NewStyle().Foreground(green).Bold(true),

		HeaderBar:    lipgloss.NewStyle().Background(headerBg).Padding(0, 1),
		StatusBar:    lipgloss.NewStyle().Background(statusBg).Padding(0, 1),
		PromptBox:    lipgloss.NewStyle(),
		PromptBorder: lipgloss.RoundedBorder(),

		UserGlyph:      "> ",
		AssistantGlyph: "  ",
		SelectGlyph:    "> ",

		Spinner: spinner.Line,
	}
}

func (m *Model) applyTheme(t theme) {
	m.theme = t
	currentTheme = t
	m.input.Prompt = t.Prompt.Render("> ")
	m.spin.Style = t.Accent
	m.spin.Spinner = t.Spinner
}
