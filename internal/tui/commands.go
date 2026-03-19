package tui

import (
	"strings"

	"pgwatch-copilot/internal/session"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

type slashCommand struct {
	Cmd    string
	Desc   string
	Prompt string // non-empty for custom commands — sent to the agent directly
}

var builtinNames = map[string]bool{
	"help": true, "session": true, "theme": true, "model": true,
	"copy": true, "clear": true, "exit": true,
	"create-custom": true, "customs": true, "delete-custom": true,
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
	case "/create-custom":
		if len(fields) < 3 {
			m.appendOutput("\n" + assistantHeader() + "Usage: /create-custom <name> <prompt...>\n")
			m.appendOutput(assistantHeader() + "Example: /create-custom daily-revenue Show me revenue by day for the last 7 days\n")
			return nil
		}
		name := fields[1]
		if builtinNames[name] {
			m.appendOutput("\n" + assistantHeader() + "Cannot use \"" + name + "\" — conflicts with a built-in command.\n")
			return nil
		}
		prompt := strings.Join(fields[2:], " ")
		if m.customStore != nil {
			if err := m.customStore.Add(name, prompt); err != nil {
				m.appendOutput("\n" + assistantHeader() + "Error saving custom command: " + err.Error() + "\n")
				return nil
			}
		}
		m.appendOutput("\n" + assistantHeader() + "Saved custom command " + m.theme.Accent.Render("/"+name) + "\n")
		m.appendOutput(m.theme.Meta.Render("  Prompt: "+prompt) + "\n")
		m.appendOutput(m.theme.Meta.Render("  Run it by typing /"+name+" or selecting it from the command palette.") + "\n")
		return nil
	case "/customs":
		if m.customStore == nil {
			m.appendOutput("\n" + assistantHeader() + "Custom commands not available.\n")
			return nil
		}
		list := m.customStore.List()
		if len(list) == 0 {
			m.appendOutput("\n" + assistantHeader() + "No custom commands saved yet.\n")
			m.appendOutput(m.theme.Meta.Render("  Create one: /create-custom <name> <prompt...>") + "\n")
			return nil
		}
		m.customList = list
		m.customSel = 0
		m.customPickerOpen = true
		m.modelPickerOpen = false
		m.themeOpen = false
		m.sessionPickerOpen = false
		m.cmdOpen = false
		m.layout()
		return nil
	case "/delete-custom":
		if arg == "" {
			m.appendOutput("\n" + assistantHeader() + "Usage: /delete-custom <name>\n")
			return nil
		}
		if m.customStore == nil {
			return nil
		}
		if err := m.customStore.Delete(arg); err != nil {
			m.appendOutput("\n" + assistantHeader() + err.Error() + "\n")
			return nil
		}
		m.appendOutput("\n" + assistantHeader() + "Deleted custom command " + m.theme.Accent.Render("/"+arg) + "\n")
		return nil
	case "/session":
		if arg == "rename" {
			if len(fields) < 3 {
				m.appendOutput("\n" + assistantHeader() + "Usage: /session rename <new name>\n")
				return nil
			}
			newName := strings.Join(fields[2:], " ")
			m.sess.Name = newName
			_ = m.persistSession()
			m.appendOutput("\n" + assistantHeader() + "Session renamed to " + m.theme.Accent.Render(newName) + "\n")
			return nil
		}
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
	case "/exit":
		if m.streaming && m.runCancel != nil {
			m.runCancel()
			m.runCancel = nil
		}
		_ = m.cleanupSessionIfEmpty()
		return tea.Quit
	default:
		name := strings.TrimPrefix(base, "/")
		if m.customStore != nil {
			if c, ok := m.customStore.Get(name); ok {
				return m.submitPrompt(c.Prompt)
			}
		}
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
	if m.themeOpen || m.modelPickerOpen || m.sessionPickerOpen || m.customPickerOpen {
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

	full := strings.ToLower(strings.TrimSpace(raw))
	// Match only on the command portion (first word). Once the user adds a
	// space they're typing arguments — keep showing the matched command.
	cmdPart := full
	if idx := strings.IndexByte(full, ' '); idx != -1 {
		cmdPart = full[:idx]
	}

	m.cmdOpen = true
	m.cmdMatches = m.cmdMatches[:0]

	for _, c := range m.commands {
		if cmdPart == "/" || strings.HasPrefix(strings.ToLower(c.Cmd), cmdPart) {
			m.cmdMatches = append(m.cmdMatches, c)
		}
	}
	if m.customStore != nil {
		for _, c := range m.customStore.List() {
			cmdName := "/" + c.Name
			desc := c.Prompt
			if len(desc) > 50 {
				desc = desc[:50] + "\u2026"
			}
			sc := slashCommand{Cmd: cmdName, Desc: desc, Prompt: c.Prompt}
			if cmdPart == "/" || strings.HasPrefix(strings.ToLower(cmdName), cmdPart) {
				m.cmdMatches = append(m.cmdMatches, sc)
			}
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
				th.Accent.Render("/help") + th.Meta.Render("              ") + "Show this help message",
				th.Accent.Render("/session") + th.Meta.Render("           ") + "Open session picker, or " + th.Accent.Render("/session <id>") + " to switch",
				th.Accent.Render("/session rename") + th.Meta.Render("    ") + "Rename current session: " + th.Accent.Render("/session rename <name>"),
				th.Accent.Render("/model") + th.Meta.Render("             ") + "Open model picker to switch LLM",
				th.Accent.Render("/theme") + th.Meta.Render("             ") + "Open theme picker with live preview",
				th.Accent.Render("/copy") + th.Meta.Render("              ") + "Copy last assistant response to clipboard",
				th.Accent.Render("/clear") + th.Meta.Render("             ") + "Clear transcript and session history",
				th.Accent.Render("/create-custom") + th.Meta.Render("     ") + "Save a reusable command: " + th.Accent.Render("/create-custom <name> <prompt>"),
				th.Accent.Render("/customs") + th.Meta.Render("           ") + "Browse and run your saved custom commands",
				th.Accent.Render("/delete-custom") + th.Meta.Render("     ") + "Remove a custom command: " + th.Accent.Render("/delete-custom <name>"),
				th.Accent.Render("/exit") + th.Meta.Render("              ") + "Exit the application",
			},
		},
		{
			heading: "Keybindings",
			lines: []string{
				th.Accent.Render("Enter") + th.Meta.Render("            ") + "Send prompt / confirm selection",
				th.Accent.Render("Ctrl+L") + th.Meta.Render("           ") + "Clear transcript",
				th.Accent.Render("Up / Ctrl+P") + th.Meta.Render("      ") + "Previous input history / navigate picker up",
				th.Accent.Render("Down / Ctrl+N") + th.Meta.Render("    ") + "Next input history / navigate picker down",
				th.Accent.Render("Tab") + th.Meta.Render("              ") + "Autocomplete command (press again to cycle matches)",
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
