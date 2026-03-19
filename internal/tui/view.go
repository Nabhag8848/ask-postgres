package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() string {
	th := m.theme

	sessionLabel := m.sess.DisplayName()
	headerLeft := th.Title.Render("pgwatch-copilot") + "  " + th.Meta.Render(fmt.Sprintf("db=%s", safeDSNHint(m.cfg.DSN))) + "  " + th.Meta.Render("session=" + sessionLabel)
	headerInner := truncateWithEllipsis(headerLeft, max(40, m.width))
	header := th.HeaderBar.Width(max(40, m.width)).Render(headerInner)
	if m.err != "" {
		header += "\n" + th.Error.Render("error: "+m.err)
	}

	body := m.renderBody()

	leftStatus := "enter: send  ctrl+l: clear  esc: cancel/quit"
	if m.streaming && !m.seenToken {
		leftStatus = m.spin.View() + "  Thinking" + animatedDots(m.dots) + "  " + th.Meta.Render("(Esc to cancel)")
	} else if m.streaming && m.seenToken {
		leftStatus = m.spin.View() + "  Generating" + animatedDots(m.dots) + "  " + th.Meta.Render("(Esc to cancel)")
	}

	rightStatus := th.Meta.Render("model: " + m.cfg.Model + m.liveTokenUsageStatus())
	w := max(40, m.width)
	statusInner := m.statusLine(th.Hint.Render(leftStatus), rightStatus)
	statusInner = ansi.Truncate(statusInner, w, "")
	status := th.StatusBar.Width(w).Render(statusInner)

	prompt := m.renderPrompt()

	used := lipgloss.Height(header) + lipgloss.Height(body) + lipgloss.Height(status) + lipgloss.Height(prompt)
	fillerH := m.height - used
	if fillerH < 0 {
		fillerH = 0
	}
	filler := ""
	if fillerH > 0 {
		filler = strings.Repeat("\n", fillerH)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, prompt, filler)
}

func (m Model) liveTokenUsageStatus() string {
	if m.pending == nil {
		return ""
	}

	inTok := m.pending.InputTokEst
	outTok := m.pending.OutputTokEst
	total := inTok + outTok

	sep := "  \u00b7  "
	if m.streaming {
		switch m.dots {
		case 0, 2:
			sep = "  \u2022  "
		default:
			sep = "  \u00b7  "
		}
	}
	return sep + fmt.Sprintf("tok ~%d (in %d / out %d)", total, inTok, outTok)
}

func (m Model) renderBody() string {
	contentH := lipgloss.Height(m.outText)
	if strings.TrimSpace(m.outText) == "" {
		contentH = 0
	}
	if m.bodyH > 0 && contentH > m.bodyH {
		left := m.output.View()
		left = lipgloss.NewStyle().Width(m.output.Width).Height(m.output.Height).Render(left)
		return left
	}
	return lipgloss.NewStyle().Width(max(40, m.width)).Render(m.outText)
}

func (m Model) renderPrompt() string {
	th := m.theme

	switch {
	case m.themeOpen:
		return lipgloss.NewStyle().Padding(0, 2).Width(max(10, m.width)).Render(m.renderThemePicker())
	case m.modelPickerOpen:
		return lipgloss.NewStyle().Padding(0, 2).Width(max(10, m.width)).Render(m.renderModelPicker())
	case m.sessionPickerOpen:
		return lipgloss.NewStyle().Padding(0, 2).Width(max(10, m.width)).Render(m.renderSessionPicker())
	case m.customPickerOpen:
		return lipgloss.NewStyle().Padding(0, 2).Width(max(10, m.width)).Render(m.renderCustomPicker())
	}

	box := th.PromptBox.Copy().
		Border(th.PromptBorder).
		BorderForeground(th.BorderColor).
		Padding(0, 1).
		Width(max(10, m.width-2))

	content := m.input.View()
	if m.cmdOpen {
		content += "\n" + m.renderCommandPalette()
	}
	return box.Render(content)
}

func (m Model) promptHeight() int {
	switch {
	case m.themeOpen:
		return min(8, len(m.themes)) + 3
	case m.modelPickerOpen:
		return min(10, len(m.modelOptions)) + 3
	case m.sessionPickerOpen:
		n := min(10, len(m.sessionList))
		if n == 0 {
			n = 1
		}
		return n + 3
	case m.customPickerOpen:
		n := min(10, len(m.customList))
		if n == 0 {
			n = 1
		}
		return n + 4
	default:
		lines := 1
		if m.cmdOpen {
			lines += min(6, len(m.cmdMatches))
		}
		return lines + 2
	}
}

func (m Model) statusLine(left, right string) string {
	w := max(40, m.width)
	left = strings.ReplaceAll(left, "\n", " ")
	right = strings.ReplaceAll(right, "\n", " ")

	rw := lipgloss.Width(right)
	lw := lipgloss.Width(left)

	if lw+1+rw > w {
		availLeft := w - rw - 1
		if availLeft < 0 {
			availLeft = 0
		}
		left = truncateWithEllipsis(left, availLeft)
		lw = lipgloss.Width(left)
	}
	if lw+1+rw > w {
		availRight := w - lw - 1
		if availRight < 0 {
			availRight = 0
		}
		right = truncateWithEllipsis(right, availRight)
		rw = lipgloss.Width(right)
	}

	space := w - rw - lw
	if space < 1 {
		space = 1
	}
	return left + strings.Repeat(" ", space) + right
}
