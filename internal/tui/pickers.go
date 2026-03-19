package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderThemePicker() string {
	active := m.theme
	preview := active
	if len(m.themes) > 0 && m.themeSel >= 0 && m.themeSel < len(m.themes) {
		preview = m.themes[m.themeSel]
	}

	title := preview.Title.Render("Select theme")
	help := preview.Meta.Render("↑/↓ preview • enter apply • esc cancel")

	maxItems := min(8, len(m.themes))
	start := 0
	if m.themeSel >= maxItems {
		start = m.themeSel - maxItems + 1
	}

	var list strings.Builder
	for i := 0; i < maxItems; i++ {
		idx := start + i
		if idx >= len(m.themes) {
			break
		}
		name := m.themes[idx].Name
		if idx == m.themeSel {
			list.WriteString(preview.CmdSelected.Render(preview.SelectGlyph + name))
		} else {
			list.WriteString(preview.CmdItem.Render("  " + name))
		}
		if idx != start+maxItems-1 && idx != len(m.themes)-1 {
			list.WriteByte('\n')
		}
	}

	sample := preview.UserMark.Render(preview.UserGlyph) + "show me customers and orders\n\n" +
		preview.AssistantMark.Render(preview.AssistantGlyph) + "Sure — I'll query `app.customers` and `app.orders` and summarize.\n"

	previewBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(preview.BorderColor).
		Padding(1, 2).
		Width(44).
		Render(sample)

	listBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(preview.BorderColor).
		Padding(1, 2).
		Width(22).
		Render(list.String())

	content := lipgloss.JoinHorizontal(lipgloss.Top, listBox, lipgloss.NewStyle().Render("  "), previewBox)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(preview.BorderColor).
		Padding(1, 2).
		Width(min(78, max(54, m.width-4))).
		Render(title + "\n" + help + "\n\n" + content)

	if preview.Name != active.Name {
		box += "\n" + preview.Meta.Render("previewing: "+preview.Name+"  (current: "+active.Name+")")
	}

	return lipgloss.Place(m.width, m.output.Height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderModelPicker() string {
	th := m.theme
	if len(m.modelOptions) == 0 {
		return th.Meta.Render("no models configured")
	}

	title := th.Title.Render("Select model")
	help := th.Meta.Render("↑/↓ to move • enter to select • esc to cancel")

	maxItems := min(10, len(m.modelOptions))
	start := 0
	if m.modelSel >= maxItems {
		start = m.modelSel - maxItems + 1
	}

	var b strings.Builder
	for i := 0; i < maxItems; i++ {
		idx := start + i
		if idx >= len(m.modelOptions) {
			break
		}
		name := m.modelOptions[idx]
		line := name
		if idx == m.modelSel {
			b.WriteString(th.CmdSelected.Render(th.SelectGlyph + line))
		} else {
			b.WriteString(th.CmdItem.Render("  " + line))
		}
		if idx != start+maxItems-1 && idx != len(m.modelOptions)-1 {
			b.WriteByte('\n')
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderColor).
		Padding(1, 2).
		Width(min(64, max(30, m.width-4))).
		Render(title + "\n" + help + "\n\n" + b.String())

	return lipgloss.Place(m.width, m.output.Height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderSessionPicker() string {
	th := m.theme
	title := th.Title.Render("Select session")
	help := th.Meta.Render("↑/↓ to move • enter to resume • esc to cancel")
	if len(m.sessionList) == 0 {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(th.BorderColor).
			Padding(1, 2).
			Width(min(64, max(30, m.width-4))).
			Render(title + "\n" + help + "\n\n" + th.Meta.Render("no sessions found"))
		return lipgloss.Place(m.width, m.output.Height, lipgloss.Center, lipgloss.Center, box)
	}

	maxItems := min(10, len(m.sessionList))
	start := 0
	if m.sessionSel >= maxItems {
		start = m.sessionSel - maxItems + 1
	}

	var b strings.Builder
	for i := 0; i < maxItems; i++ {
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
		if idx != start+maxItems-1 && idx != len(m.sessionList)-1 {
			b.WriteByte('\n')
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderColor).
		Padding(1, 2).
		Width(min(72, max(40, m.width-4))).
		Render(title + "\n" + help + "\n\n" + b.String())
	return lipgloss.Place(m.width, m.output.Height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderCustomPicker() string {
	th := m.theme
	title := th.Title.Render("Custom commands")
	help := th.Meta.Render("↑/↓ to move • enter to run • esc to cancel")
	if len(m.customList) == 0 {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(th.BorderColor).
			Padding(1, 2).
			Width(min(72, max(40, m.width-4))).
			Render(title + "\n" + help + "\n\n" + th.Meta.Render("no custom commands saved"))
		return lipgloss.Place(m.width, m.output.Height, lipgloss.Center, lipgloss.Center, box)
	}

	maxItems := min(10, len(m.customList))
	start := 0
	if m.customSel >= maxItems {
		start = m.customSel - maxItems + 1
	}

	nameW := 0
	for _, c := range m.customList {
		if len(c.Name) > nameW {
			nameW = len(c.Name)
		}
	}

	var b strings.Builder
	for i := 0; i < maxItems; i++ {
		idx := start + i
		if idx >= len(m.customList) {
			break
		}
		c := m.customList[idx]
		prompt := c.Prompt
		maxPromptW := max(20, m.width-nameW-20)
		if len(prompt) > maxPromptW {
			prompt = prompt[:maxPromptW] + "…"
		}
		padded := c.Name + strings.Repeat(" ", nameW-len(c.Name))
		line := th.Accent.Render("/"+padded) + th.Meta.Render("  "+prompt)
		if idx == m.customSel {
			b.WriteString(th.CmdSelected.Render(th.SelectGlyph + line))
		} else {
			b.WriteString(th.CmdItem.Render("  " + line))
		}
		if idx != start+maxItems-1 && idx != len(m.customList)-1 {
			b.WriteByte('\n')
		}
	}

	footer := th.Meta.Render("Delete with: /delete-custom <name>")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderColor).
		Padding(1, 2).
		Width(min(80, max(50, m.width-4))).
		Render(title + "\n" + help + "\n\n" + b.String() + "\n\n" + footer)
	return lipgloss.Place(m.width, m.output.Height, lipgloss.Center, lipgloss.Center, box)
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
