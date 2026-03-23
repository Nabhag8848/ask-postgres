package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderThemePicker() string {
	preview := m.theme
	if len(m.themes) > 0 && m.themeSel >= 0 && m.themeSel < len(m.themes) {
		preview = m.themes[m.themeSel]
	}

	title := preview.Title.Render("Select theme")
	help := preview.Meta.Render("↑/↓ preview • enter apply • esc cancel")

	visible := min(8, len(m.themes))
	start := 0
	if m.themeSel >= visible {
		start = m.themeSel - visible + 1
	}

	var b strings.Builder
	for i := 0; i < visible; i++ {
		idx := start + i
		if idx >= len(m.themes) {
			break
		}
		name := m.themes[idx].Name
		if idx == m.themeSel {
			b.WriteString(preview.CmdSelected.Render(preview.SelectGlyph + name))
		} else {
			b.WriteString(preview.CmdItem.Render("  " + name))
		}
		if i < visible-1 {
			b.WriteByte('\n')
		}
	}

	return title + "\n" + help + "\n" + b.String()
}

func (m Model) renderModelPicker() string {
	th := m.theme
	if len(m.modelOptions) == 0 {
		return th.Meta.Render("no models configured")
	}

	title := th.Title.Render("Select model")
	help := th.Meta.Render("↑/↓ to move • enter to select • esc to cancel")

	visible := min(10, len(m.modelOptions))
	start := 0
	if m.modelSel >= visible {
		start = m.modelSel - visible + 1
	}

	var b strings.Builder
	for i := 0; i < visible; i++ {
		idx := start + i
		if idx >= len(m.modelOptions) {
			break
		}
		name := m.modelOptions[idx]
		if idx == m.modelSel {
			b.WriteString(th.CmdSelected.Render(th.SelectGlyph + name))
		} else {
			b.WriteString(th.CmdItem.Render("  " + name))
		}
		if i < visible-1 {
			b.WriteByte('\n')
		}
	}

	return title + "\n" + help + "\n" + b.String()
}

func (m Model) renderSessionPicker() string {
	th := m.theme
	title := th.Title.Render("Select session")
	help := th.Meta.Render("↑/↓ to move • enter to open • " + "Ctrl+D" + " to delete • esc to cancel")

	if len(m.sessionList) == 0 {
		return title + "\n" + help + "\n" + th.Meta.Render("no sessions found")
	}

	if m.sessionDeleteConfirm && m.sessionSel >= 0 && m.sessionSel < len(m.sessionList) {
		sel := m.sessionList[m.sessionSel]
		label := sel.DisplayName()
		q := th.User.Render("Delete session ") + th.Accent.Render(label) + th.User.Render("?")
		yn := th.Meta.Render("enter / y to confirm  •  n / esc to cancel")
		help2 := th.Meta.Render("This removes the file, transcript, and agent memory for that session.")
		return title + "\n" + help + "\n" + q + "\n" + help2 + "\n" + yn
	}

	visible := min(10, len(m.sessionList))
	start := 0
	if m.sessionSel >= visible {
		start = m.sessionSel - visible + 1
	}

	var b strings.Builder
	for i := 0; i < visible; i++ {
		idx := start + i
		if idx >= len(m.sessionList) {
			break
		}
		s := m.sessionList[idx]
		label := s.DisplayName()
		idHint := ""
		if s.Name != "" {
			idHint = th.Meta.Render("  (" + s.ID[:min(8, len(s.ID))] + ")")
		}
		line := label + idHint + th.Meta.Render("  ") + th.Meta.Render(s.UpdatedAt.Format("2006-01-02 15:04"))
		if idx == m.sessionSel {
			b.WriteString(th.CmdSelected.Render(th.SelectGlyph + line))
		} else {
			b.WriteString(th.CmdItem.Render("  " + line))
		}
		if i < visible-1 {
			b.WriteByte('\n')
		}
	}

	return title + "\n" + help + "\n" + b.String()
}

func (m Model) renderCustomPicker() string {
	th := m.theme
	title := th.Title.Render("Custom commands")
	help := th.Meta.Render("↑/↓ to move • enter to run • esc to cancel")

	if len(m.customList) == 0 {
		return title + "\n" + help + "\n" + th.Meta.Render("no custom commands saved")
	}

	visible := min(10, len(m.customList))
	start := 0
	if m.customSel >= visible {
		start = m.customSel - visible + 1
	}

	nameW := 0
	for _, c := range m.customList {
		if len(c.Name) > nameW {
			nameW = len(c.Name)
		}
	}

	var b strings.Builder
	for i := 0; i < visible; i++ {
		idx := start + i
		if idx >= len(m.customList) {
			break
		}
		c := m.customList[idx]
		prompt := c.Prompt
		maxPromptW := max(20, m.width-nameW-20)
		if len(prompt) > maxPromptW {
			prompt = prompt[:maxPromptW] + "\u2026"
		}
		padded := c.Name + strings.Repeat(" ", nameW-len(c.Name))
		line := th.Accent.Render("/"+padded) + th.Meta.Render("  "+prompt)
		if idx == m.customSel {
			b.WriteString(th.CmdSelected.Render(th.SelectGlyph + line))
		} else {
			b.WriteString(th.CmdItem.Render("  " + line))
		}
		if i < visible-1 {
			b.WriteByte('\n')
		}
	}

	footer := th.Meta.Render("delete with: /delete-custom <name>")
	return title + "\n" + help + "\n" + b.String() + "\n" + footer
}

func (m Model) renderCommandPalette() string {
	th := m.theme
	total := len(m.cmdMatches)
	if total == 0 {
		return th.Meta.Render("no commands")
	}

	visible := min(6, total)
	start := 0
	if m.cmdSel >= visible {
		start = m.cmdSel - visible + 1
	}

	var b strings.Builder
	for i := 0; i < visible; i++ {
		idx := start + i
		if idx >= total {
			break
		}
		c := m.cmdMatches[idx]
		line := c.Cmd
		if c.Prompt != "" {
			line += th.Meta.Render("  [custom]  " + c.Desc)
		} else if c.Desc != "" {
			line += th.Meta.Render("  " + c.Desc)
		}

		if idx == m.cmdSel {
			b.WriteString(th.CmdSelected.Render(th.SelectGlyph + line))
		} else {
			b.WriteString(th.CmdItem.Render("  " + line))
		}
		if i != visible-1 {
			b.WriteByte('\n')
		}
	}
	if total > visible {
		b.WriteString("\n" + th.Meta.Render(fmt.Sprintf("  (%d more)", total-visible)))
	}
	return b.String()
}

func (m Model) renderShortcuts() string {
	th := m.theme
	title := th.Title.Render("Shortcuts")

	pad := func(s string, w int) string {
		if len(s) >= w {
			return s
		}
		return s + strings.Repeat(" ", w-len(s))
	}

	type shortcut struct {
		key  string
		desc string
	}
	items := []shortcut{
		{"?", "show this shortcuts panel"},
		{"/", "open command palette"},
		{"Enter", "send prompt / confirm selection"},
		{"Ctrl+J", "new line in prompt"},
		{"Tab", "autocomplete command (cycle matches)"},
		{"\u2191", "previous history / navigate up"},
		{"\u2193", "next history / navigate down"},
		{"Ctrl+L", "clear transcript"},
		{"Ctrl+D", "delete session (in session picker, then confirm)"},
		{"Esc", "cancel query / close picker / quit"},
		{"Ctrl+C", "same as Esc"},
	}

	keyW := 0
	for _, s := range items {
		if len(s.key) > keyW {
			keyW = len(s.key)
		}
	}
	keyW += 2

	var b strings.Builder
	for i, s := range items {
		b.WriteString(th.Accent.Render(pad(s.key, keyW)) + th.Meta.Render(s.desc))
		if i < len(items)-1 {
			b.WriteByte('\n')
		}
	}

	footer := th.Meta.Render("press any key to dismiss")
	return title + "\n\n" + b.String() + "\n\n" + footer
}
