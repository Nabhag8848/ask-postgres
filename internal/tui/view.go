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
	headerLeft := th.Title.Render("ask-postgres") + "  " + th.Meta.Render(fmt.Sprintf("db=%s", safeDSNHint(m.cfg.DSN))) + "  " + th.Meta.Render("session="+sessionLabel)
	headerInner := truncateWithEllipsis(headerLeft, max(40, m.width))
	header := th.HeaderBar.Width(max(40, m.width)).Render(headerInner)
	if m.err != "" {
		header += "\n" + th.Error.Render("error: "+m.err)
	}

	body := m.renderBody()

	leftStatus := "enter: send  " + "Ctrl+L" + ": clear  esc: cancel/quit"
	if m.streaming && !m.seenToken {
		leftStatus = m.spin.View() + "  Thinking" + animatedDots(m.dots) + "  " + th.Meta.Render("(Esc to cancel)")
	} else if m.streaming && m.seenToken {
		leftStatus = m.spin.View() + "  Generating" + animatedDots(m.dots) + "  " + th.Meta.Render("(Esc to cancel)")
	}

	rightStatus := th.Meta.Render("model: " + m.cfg.Model + m.liveTokenUsageStatus())
	w := max(40, m.width)
	// StatusBar uses Padding(0,1); inner text area is two cells narrower than Width.
	statusContentW := max(1, w-2)
	statusInner := m.statusLine(th.Hint.Render(leftStatus), rightStatus, statusContentW)
	statusInner = ansi.Truncate(statusInner, statusContentW, "")
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
	w := max(40, m.width)
	if m.shouldShowWelcome() {
		return m.renderWelcomePanel(w, m.bodyH)
	}
	contentH := lipgloss.Height(m.outText)
	if strings.TrimSpace(m.outText) == "" {
		contentH = 0
	}
	if m.bodyH > 0 && contentH > m.bodyH {
		left := m.output.View()
		left = lipgloss.NewStyle().Width(m.output.Width).Height(m.output.Height).Render(left)
		return left
	}
	return lipgloss.NewStyle().Width(w).Render(m.outText)
}

func (m Model) renderPrompt() string {
	th := m.theme

	inline := lipgloss.NewStyle().Padding(0, 2).Width(max(10, m.width))
	switch {
	case m.shortcutsOpen:
		return inline.Render(m.renderShortcuts())
	case m.themeOpen:
		return inline.Render(m.renderThemePicker())
	case m.modelPickerOpen:
		return inline.Render(m.renderModelPicker())
	case m.sessionPickerOpen:
		return inline.Render(m.renderSessionPicker())
	case m.customPickerOpen:
		return inline.Render(m.renderCustomPicker())
	case m.settingsOpen:
		return inline.Render(m.renderSettingsPicker())
	}

	box := th.PromptBox.Copy().
		Border(th.PromptBorder).
		BorderForeground(th.BorderColor).
		Padding(0, 1).
		Width(max(10, m.width-2))

	promptGlyph := th.Prompt.Render("> ")
	inputView := m.input.View()
	if m.cmdOpen && len(m.cmdMatches) > 0 && m.cmdSel >= 0 && m.cmdSel < len(m.cmdMatches) {
		sel := m.cmdMatches[m.cmdSel]
		typed := m.input.Value()
		ghost := ""
		typedLower := strings.ToLower(typed)
		cmdLower := strings.ToLower(sel.Cmd)
		if strings.HasPrefix(cmdLower, typedLower) && len(sel.Cmd) > len(typed) {
			ghost = sel.Cmd[len(typed):]
		}
		if ghost != "" {
			inputView = typed + th.Meta.Render(ghost)
		}
	}
	lines := strings.Split(inputView, "\n")
	for i := range lines {
		if i == 0 {
			lines[i] = promptGlyph + lines[i]
		} else {
			lines[i] = "  " + lines[i]
		}
	}
	inputView = strings.Join(lines, "\n")
	prompt := box.Render(inputView)
	if m.cmdOpen {
		palette := lipgloss.NewStyle().Padding(0, 2).Width(max(10, m.width)).Render(m.renderCommandPalette())
		prompt = prompt + "\n" + palette
	}
	return prompt
}

func (m Model) promptHeight() int {
	switch {
	case m.shortcutsOpen:
		return 16
	case m.themeOpen:
		return min(8, len(m.themes)) + 3
	case m.modelPickerOpen:
		if len(m.modelOptions) == 0 {
			return modelPickerOverheadLines
		}
		return m.modelPickerVisibleCount() + modelPickerOverheadLines
	case m.sessionPickerOpen:
		// sessionPickerConfirmHeight: title + help + question + note + y/n line
		// (see renderSessionPicker when sessionDeleteConfirm); keep in sync if layout changes.
		const sessionPickerConfirmHeight = 7
		if m.sessionDeleteConfirm {
			return sessionPickerConfirmHeight
		}
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
	case m.settingsOpen:
		if m.settingsFormOpen {
			return 17
		}
		return 7
	default:
		inputLines := max(1, m.input.LineCount())
		base := inputLines + 2
		if m.cmdOpen {
			n := min(6, len(m.cmdMatches))
			if len(m.cmdMatches) > 6 {
				n++
			}
			base += n + 1
		}
		return base
	}
}

func (m Model) statusLine(left, right string, contentW int) string {
	w := max(1, contentW)
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
