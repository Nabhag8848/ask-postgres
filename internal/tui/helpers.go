package tui

import (
	"strings"

	"pgwatch-copilot/internal/session"

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
