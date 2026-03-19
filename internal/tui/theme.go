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
	mk := func(
		name string,
		accent, user, assistant, hint, errC, border lipgloss.AdaptiveColor,
		headerBg, statusBg, promptBg lipgloss.AdaptiveColor,
		promptBorder lipgloss.Border,
		userGlyph, assistantGlyph, selectGlyph string,
		spin spinner.Spinner,
	) theme {
		headerBar := lipgloss.NewStyle().Background(headerBg).Padding(0, 1)
		statusBar := lipgloss.NewStyle().Background(statusBg).Padding(0, 1)
		promptBox := lipgloss.NewStyle().Background(promptBg)

		return theme{
			Name:      name,
			Title:     lipgloss.NewStyle().Bold(true).Foreground(accent),
			Meta:      lipgloss.NewStyle().Faint(true),
			Hint:      lipgloss.NewStyle().Foreground(hint).Faint(true),
			Prompt:    lipgloss.NewStyle().Foreground(accent).Bold(true),
			User:      lipgloss.NewStyle().Bold(true).Foreground(user),
			Assistant: lipgloss.NewStyle().Bold(true).Foreground(assistant),
			Accent:    lipgloss.NewStyle().Foreground(accent),
			Error:     lipgloss.NewStyle().Foreground(errC).Bold(true),

			UserMark:      lipgloss.NewStyle().Foreground(user).Bold(true),
			AssistantMark: lipgloss.NewStyle().Foreground(assistant).Bold(true),
			BorderColor:   border,

			CmdItem:     lipgloss.NewStyle().Foreground(hint).Faint(true),
			CmdSelected: lipgloss.NewStyle().Foreground(accent).Bold(true),

			HeaderBar:    headerBar,
			StatusBar:    statusBar,
			PromptBox:    promptBox,
			PromptBorder: promptBorder,

			UserGlyph:      userGlyph,
			AssistantGlyph: assistantGlyph,
			SelectGlyph:    selectGlyph,

			Spinner: spin,
		}
	}

	border := lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#2A2F3A"}
	return []theme{
		mk(
			"Azure Pop",
			lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"},
			lipgloss.AdaptiveColor{Light: "#BE185D", Dark: "#F472B6"},
			lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"},
			lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"},
			lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"},
			border,
			lipgloss.AdaptiveColor{Light: "#EFF6FF", Dark: "#0B1220"},
			lipgloss.AdaptiveColor{Light: "#F8FAFC", Dark: "#0A0F1A"},
			lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#05070D"},
			lipgloss.RoundedBorder(),
			"› ",
			"❯ ",
			"▸ ",
			spinner.Line,
		),
		mk(
			"Violet Night",
			lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A78BFA"},
			lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FB7185"},
			lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"},
			lipgloss.AdaptiveColor{Light: "#92400E", Dark: "#FCD34D"},
			lipgloss.AdaptiveColor{Light: "#9F1239", Dark: "#FDA4AF"},
			border,
			lipgloss.AdaptiveColor{Light: "#FAF5FF", Dark: "#12081C"},
			lipgloss.AdaptiveColor{Light: "#FDF4FF", Dark: "#0F0718"},
			lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#090312"},
			lipgloss.ThickBorder(),
			"∙ ",
			"◆ ",
			"▶ ",
			spinner.Dot,
		),
		mk(
			"Solar Mint",
			lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#34D399"},
			lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#FDBA74"},
			lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#93C5FD"},
			lipgloss.AdaptiveColor{Light: "#7C2D12", Dark: "#FBBF24"},
			lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"},
			border,
			lipgloss.AdaptiveColor{Light: "#ECFEFF", Dark: "#07171A"},
			lipgloss.AdaptiveColor{Light: "#F0FDFA", Dark: "#061315"},
			lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#040B0B"},
			lipgloss.DoubleBorder(),
			"→ ",
			"↳ ",
			"▸ ",
			spinner.MiniDot,
		),
	}
}

func (m *Model) applyTheme(t theme) {
	m.theme = t
	currentTheme = t
	m.input.Prompt = t.Prompt.Render("› ")
	m.spin.Style = t.Accent
	m.spin.Spinner = t.Spinner
}
