package tui

import (
	"fmt"
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
	"create-custom": true, "customs": true, "delete-custom": true, "settings": true,
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
		m.sessionDeleteConfirm = false
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
				m.sessionDeleteConfirm = false
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
		return nil
	case "/settings":
		m.initSettingsForm()
		m.settingsOpen = true
		m.themeOpen = false
		m.modelPickerOpen = false
		m.sessionPickerOpen = false
		m.customPickerOpen = false
		m.sessionDeleteConfirm = false
		m.cmdOpen = false
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
	if m.themeOpen || m.modelPickerOpen || m.sessionPickerOpen || m.customPickerOpen || m.settingsOpen {
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
		return
	}

	curLine := m.currentLine()
	if !strings.HasPrefix(curLine, "/") {
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
		return
	}

	for _, ln := range strings.Split(m.input.Value(), "\n") {
		if strings.TrimSpace(ln) != "" && strings.TrimSpace(ln) != strings.TrimSpace(curLine) {
			m.cmdOpen = false
			m.cmdMatches = nil
			m.cmdSel = 0
			return
		}
	}

	full := strings.ToLower(strings.TrimSpace(curLine))
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
				th.Accent.Render("/session") + th.Meta.Render("           ") + "Open session picker (" + "Ctrl+D" + " to delete with confirm), or " + th.Accent.Render("/session <name|id>") + " (creates if missing)",
				th.Accent.Render("/session rename") + th.Meta.Render("    ") + "Rename current session: " + th.Accent.Render("/session rename <name>"),
				th.Accent.Render("/model") + th.Meta.Render("             ") + "Open model picker to switch LLM",
				th.Accent.Render("/theme") + th.Meta.Render("             ") + "Open theme picker with live preview",
				th.Accent.Render("/settings") + th.Meta.Render("          ") + "Manage LLM provider API keys",
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
				th.Accent.Render("Ctrl+J") + th.Meta.Render("          ") + "New line in prompt",
				th.Accent.Render("Ctrl+L") + th.Meta.Render("           ") + "Clear transcript",
				th.Accent.Render("\u2191") + th.Meta.Render("                ") + "Navigate prompt history (previous) / picker up",
				th.Accent.Render("\u2193") + th.Meta.Render("                ") + "Navigate prompt history (next) / picker down",
				th.Accent.Render("Tab") + th.Meta.Render("              ") + "Autocomplete command / move between settings fields",
				th.Accent.Render("Ctrl+U") + th.Meta.Render("           ") + "Clear current API key field (in settings form)",
				th.Accent.Render("Ctrl+D") + th.Meta.Render("      ") + "Delete session in session picker (confirm with enter / y)",
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

// adoptLoadedSession applies an already-loaded session to the TUI: in-memory
// sess, agent chat history, input history draft, and the rendered transcript
// viewport. It does not read from disk or persist the previous session — callers
// handle that (e.g. switchToSession runs cleanup + persist first).
func (m *Model) adoptLoadedSession(sess session.Session) {
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
}

// switchToSession persists the current session (if non-empty), removes empty
// session files when appropriate, then resolves the provided ref as:
// 1) exact session id, 2) exact session name (case-insensitive), or
// 3) create a new session named ref if no match exists.
// Finally it delegates to adoptLoadedSession for shared UI/agent state.
func (m *Model) switchToSession(id string) error {
	if m.store == nil {
		return nil
	}
	ref := strings.TrimSpace(id)
	if ref == "" {
		return nil
	}

	var (
		sess session.Session
		err  error
	)
	targetID := ""
	if list, lerr := m.store.List(); lerr == nil {
		for _, s := range list {
			if s.ID == ref || strings.EqualFold(s.Name, ref) {
				targetID = s.ID
				break
			}
		}
	}

	if targetID != "" {
		sess, err = m.store.Load(targetID)
		if err != nil {
			return err
		}
	} else {
		sess, err = m.store.New()
		if err != nil {
			return err
		}
		sess.Name = ref
		if err := m.store.Save(sess); err != nil {
			return err
		}
	}

	_ = m.cleanupSessionIfEmpty()
	_ = m.persistSession()

	m.adoptLoadedSession(sess)
	return nil
}

// confirmDeleteSelectedSession removes the highlighted session from disk. If it
// was the active session, clears agent memory and transcript and switches to
// another session or creates a new empty one.
func (m Model) confirmDeleteSelectedSession() (Model, tea.Cmd) {
	p := m
	p.sessionDeleteConfirm = false
	if p.store == nil || len(p.sessionList) == 0 || p.sessionSel < 0 || p.sessionSel >= len(p.sessionList) {
		p.layout()
		return p, nil
	}

	id := p.sessionList[p.sessionSel].ID
	wasCurrent := id == p.sess.ID

	if wasCurrent {
		if p.streaming && p.runCancel != nil {
			p.runCancel()
			p.runCancel = nil
		}
		p.streaming = false
		p.seenToken = false
		p.promptQueue = nil
		p.pending = nil
	}

	if err := p.store.Delete(id); err != nil {
		p.err = err.Error()
		p.layout()
		return p, nil
	}

	list, err := p.store.List()
	if err != nil {
		p.err = err.Error()
		list = nil
	}
	p.sessionList = list
	pp := &p

	if wasCurrent {
		pp.agent.ClearHistory()
		if len(p.sessionList) > 0 {
			sess, lerr := p.store.Load(p.sessionList[0].ID)
			if lerr != nil {
				p.err = lerr.Error()
				newSess, nerr := p.store.New()
				if nerr != nil {
					p.err = fmt.Sprintf("%v; %v", lerr, nerr)
				} else {
					pp.adoptLoadedSession(newSess)
				}
			} else {
				pp.adoptLoadedSession(sess)
			}
			p.sessionSel = 0
		} else {
			newSess, nerr := p.store.New()
			if nerr != nil {
				p.err = nerr.Error()
			} else {
				pp.adoptLoadedSession(newSess)
			}
			p.sessionList, _ = p.store.List()
			p.sessionSel = 0
		}
	} else {
		if p.sessionSel >= len(p.sessionList) {
			if len(p.sessionList) == 0 {
				p.sessionSel = 0
			} else {
				p.sessionSel = len(p.sessionList) - 1
			}
		}
	}

	p.layout()
	return p, nil
}
