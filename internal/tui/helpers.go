package tui

import (
	"strings"

	"ask-postgres/internal/session"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func safeDSNHint(dsn string) string {
	if strings.Contains(dsn, "@") {
		parts := strings.SplitN(dsn, "@", 2)
		return "…@" + parts[1]
	}
	if len(dsn) > 48 {
		return dsn[:48] + "…"
	}
	return dsn
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

func userLine(prompt string) string {
	return currentTheme.UserMark.Render(currentTheme.UserGlyph) + prompt
}

func assistantHeader() string {
	return currentTheme.AssistantMark.Render(currentTheme.AssistantGlyph)
}

func truncateWithEllipsis(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return ansi.Truncate(s, maxW, "…")
}

func animatedDots(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "."
	case 2:
		return ".."
	default:
		return "..."
	}
}

func deriveToolName(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return "tool"
	}
	l := strings.ToLower(s)
	for _, n := range []string{"schema_overview", "describe_table", "sql_readonly"} {
		if strings.Contains(l, n) {
			return n
		}
	}
	parts := strings.Fields(s)
	if len(parts) > 0 {
		return strings.Trim(parts[0], ":")
	}
	return "tool"
}

// friendlyToolName maps internal tool ids to short, non-technical labels for the UI.
func friendlyToolName(internal string) string {
	switch internal {
	case "schema_overview":
		return "Listing tables"
	case "describe_table":
		return "Inspecting a table"
	case "sql_readonly":
		return "Reading data (safe)"
	default:
		return internal
	}
}

func approxTokenCountChars(chars int) int {
	if chars <= 0 {
		return 0
	}
	t := chars / 4
	if t < 1 {
		return 1
	}
	return t
}

func messagesToTurns(msgs []session.Message) []session.ChatTurn {
	if len(msgs) == 0 {
		return nil
	}
	var out []session.ChatTurn
	pendingUser := ""
	for _, m := range msgs {
		switch m.Role {
		case "user":
			pendingUser = m.Content
		case "assistant":
			out = append(out, session.ChatTurn{
				User:      pendingUser,
				Assistant: m.Content,
			})
			pendingUser = ""
		}
	}
	return out
}

func (m *Model) resetInput() {
	m.input.SetValue("")
	m.input.SetHeight(1)
}

func (m *Model) setInputValue(s string) {
	m.input.SetValue(s)
	m.input.SetHeight(min(10, max(1, m.input.LineCount())))
}

func (m *Model) currentLine() string {
	lines := strings.Split(m.input.Value(), "\n")
	if row := m.input.Line(); row >= 0 && row < len(lines) {
		return lines[row]
	}
	return m.input.Value()
}

// isCuratedModel reports whether id is in the /model picker list (same as buildModelOptions).
func (m *Model) isCuratedModel(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, opt := range m.modelOptions {
		if opt == id {
			return true
		}
	}
	return false
}

func (m *Model) openModelPicker() {
	m.modelPickerOpen = true
	m.settingsOpen = false
	m.cmdOpen = false
	m.cmdMatches = nil
	m.cmdSel = 0
	for i, opt := range m.modelOptions {
		if opt == m.cfg.Model {
			m.modelSel = i
			break
		}
	}
	m.layout()
}
