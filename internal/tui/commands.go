package tui

import (
	"strings"

	"pgwatch-copilot/internal/session"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

type slashCommand struct {
	Cmd  string
	Desc string
}

func (m *Model) runSlashCommand(cmd string) tea.Cmd {
	raw := strings.TrimSpace(cmd)
	fields := strings.Fields(raw)
	base := ""
	arg := ""
	if len(fields) > 0 {
		base = fields[0]
	}
	if len(fields) > 1 {
		arg = fields[1]
	}

	switch base {
	case "/help":
		m.appendOutput("\n" + m.renderHelp() + "\n")
		return nil
	case "/session":
		if arg != "" {
			if err := m.switchToSession(arg); err != nil {
				m.appendOutput("\n" + assistantHeader() + "Could not open session " + arg + ": " + err.Error() + "\n")
			}
			return nil
		}
		if m.store != nil {
			list, err := m.store.List()
			if err == nil {
				m.sessionList = list
				m.sessionSel = 0
				m.sessionPickerOpen = true
				m.modelPickerOpen = false
				m.themeOpen = false
				m.cmdOpen = false
				m.layout()
			}
		}
		return nil
	case "/theme":
		m.themeOpen = true
		m.modelPickerOpen = false
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
		for i := range m.themes {
			if m.themes[i].Name == m.theme.Name {
				m.themeSel = i
				break
			}
		}
		m.layout()
		return nil
	case "/model":
		m.modelPickerOpen = true
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
		return nil
	case "/copy":
		text := m.lastAssistantContent()
		if text == "" {
			m.appendOutput("\n" + assistantHeader() + "Nothing to copy.\n")
			return nil
		}
		if err := clipboard.WriteAll(text); err != nil {
			m.appendOutput("\n" + assistantHeader() + "Clipboard error: " + err.Error() + "\n")
			return nil
		}
		m.appendOutput("\n" + assistantHeader() + "Copied to clipboard.\n")
		return nil
	case "/clear":
		m.clearTranscript()
		m.agent.ClearHistory()
		m.sess.Turns = nil
		_ = m.cleanupSessionIfEmpty()
		_ = m.persistSession()
		return nil
	case "/exit", "/quit":
		if m.streaming && m.runCancel != nil {
			m.runCancel()
			m.runCancel = nil
		}
		return tea.Quit
	default:
		m.appendOutput("\n" + assistantHeader() + "Unknown command. Type /help for a list of available commands.\n")
		return nil
	}
}

func (m Model) lastAssistantContent() string {
	for i := len(m.sess.Messages) - 1; i >= 0; i-- {
		if m.sess.Messages[i].Role == "assistant" {
			return strings.TrimSpace(m.sess.Messages[i].Content)
		}
	}
	return ""
}

func (m *Model) updateCommandPalette() {
	if m.themeOpen || m.modelPickerOpen || m.sessionPickerOpen || m.streaming {
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
		return
	}

	raw := m.input.Value()
	if !strings.HasPrefix(raw, "/") {
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
		return
	}

	needle := strings.ToLower(strings.TrimSpace(raw))
	m.cmdOpen = true
	m.cmdMatches = m.cmdMatches[:0]

	for _, c := range m.commands {
		if needle == "/" || strings.HasPrefix(strings.ToLower(c.Cmd), needle) {
			m.cmdMatches = append(m.cmdMatches, c)
		}
	}
	if m.cmdSel >= len(m.cmdMatches) {
		m.cmdSel = max(0, len(m.cmdMatches)-1)
	}
}

func (m Model) renderHelp() string {
	th := m.theme
	title := th.Title.Render("pgwatch-copilot") + "  " + th.Meta.Render("— Postgres analyst powered by LLM tools")

	sections := []struct {
		heading string
		lines   []string
	}{
		{
			heading: "Commands",
			lines: []string{
				th.Accent.Render("/help") + th.Meta.Render("            ") + "Show this help message",
				th.Accent.Render("/session") + th.Meta.Render("         ") + "Open session picker, or " + th.Accent.Render("/session <id>") + " to switch directly",
				th.Accent.Render("/model") + th.Meta.Render("           ") + "Open model picker to switch LLM",
				th.Accent.Render("/theme") + th.Meta.Render("           ") + "Open theme picker with live preview",
				th.Accent.Render("/copy") + th.Meta.Render("            ") + "Copy last assistant response to clipboard",
				th.Accent.Render("/clear") + th.Meta.Render("           ") + "Clear transcript and session history",
				th.Accent.Render("/exit") + ", " + th.Accent.Render("/quit") + th.Meta.Render("     ") + "Exit the application",
			},
		},
		{
			heading: "Keybindings",
			lines: []string{
				th.Accent.Render("Enter") + th.Meta.Render("            ") + "Send prompt / confirm selection",
				th.Accent.Render("Ctrl+L") + th.Meta.Render("           ") + "Clear transcript",
				th.Accent.Render("Up / Ctrl+P") + th.Meta.Render("      ") + "Previous input history / navigate picker up",
				th.Accent.Render("Down / Ctrl+N") + th.Meta.Render("    ") + "Next input history / navigate picker down",
				th.Accent.Render("Esc") + th.Meta.Render("              ") + "Cancel running query / close picker / quit",
				th.Accent.Render("Ctrl+C") + th.Meta.Render("           ") + "Same as Esc",
			},
		},
		{
			heading: "Usage tips",
			lines: []string{
				"Type a natural-language question and the agent will query your database.",
				"The agent has three tools: " + th.Accent.Render("schema_overview") + ", " + th.Accent.Render("describe_table") + ", " + th.Accent.Render("sql_readonly") + ".",
				"All SQL is executed read-only with a timeout — your data is safe.",
				"Start typing " + th.Accent.Render("/") + " to open the command palette with fuzzy matching.",
			},
		},
	}

	var b strings.Builder
	b.WriteString(title + "\n")
	for _, sec := range sections {
		b.WriteString("\n" + th.User.Render(sec.heading) + "\n")
		for _, line := range sec.lines {
			b.WriteString("  " + line + "\n")
		}
	}
	return b.String()
}

func (m *Model) switchToSession(id string) error {
	if m.store == nil {
		return nil
	}
	sess, err := m.store.Load(id)
	if err != nil {
		now := currentTime()
		sess = session.Session{
			ID:        id,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := m.store.Save(sess); err != nil {
			return err
		}
	}

	_ = m.cleanupSessionIfEmpty()
	_ = m.persistSession()

	m.sess = sess
	m.agent.SetHistory(messagesToTurns(sess.Messages))
	if len(sess.Messages) == 0 {
		m.agent.SetHistory(sess.Turns)
	}

	m.histIdx = -1
	m.histDraft = ""

	m.rebuildTranscriptFromSession()
	m.output.SetContent(m.outText)
	m.output.GotoBottom()
	m.err = ""
	return nil
}
