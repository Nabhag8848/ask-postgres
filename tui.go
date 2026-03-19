package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type tuiModel struct {
	cfg   appConfig
	agent *agent
	store *sessionStore
	sess  session

	ctx       context.Context
	runCancel context.CancelFunc

	width  int
	height int
	bodyH  int
	bodyW  int

	input   textinput.Model
	output  viewport.Model
	outText string

	spin spinner.Model
	dots int

	inputHistory []string
	histIdx      int
	histDraft    string

	theme     theme
	themes    []theme
	themeOpen bool
	themeSel  int

	commands   []slashCommand
	cmdOpen    bool
	cmdMatches []slashCommand
	cmdSel     int

	modelPickerOpen bool
	modelOptions    []string
	modelSel        int

	sessionPickerOpen bool
	sessionList       []session
	sessionSel        int

	streaming bool
	seenToken bool
	events    <-chan agentEvent
	pending   *pendingAssistant

	err string
}

type pendingAssistant struct {
	UserMessageID   string
	AssistantMsgID  string
	StartedAt       time.Time
	InputChars      int
	OutputChars     int
	StreamedChunks  int
	Model           string
	Tools           []toolRecord
	openToolIndexes []int

	InputTokEst  int
	OutputTokEst int
}

func newTUImodel(cfg appConfig, agent *agent, store *sessionStore, sess session) tuiModel {
	themes := defaultThemes()
	active := themes[0]
	gc := loadGlobalConfig()
	if gc.Theme != "" {
		for _, t := range themes {
			if t.Name == gc.Theme {
				active = t
				break
			}
		}
	}

	in := textinput.New()
	in.Placeholder = "Ask about your Postgres… (try: “largest tables?”, “describe app.orders”, “revenue by day”)"
	in.Focus()
	in.CharLimit = 2000
	in.Prompt = active.Prompt.Render("› ")

	vp := viewport.New(0, 0)
	vp.SetContent("")

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = active.Accent

	m := tuiModel{
		cfg:     cfg,
		agent:   agent,
		store:   store,
		sess:    sess,
		input:   in,
		output:  vp,
		outText: "",
		spin:    sp,
		histIdx: -1,
		theme:   active,
		themes:  themes,
		commands: []slashCommand{
			{Cmd: "/session", Desc: "Switch/resume session (/session <id>)"},
			{Cmd: "/theme", Desc: "Choose theme"},
			{Cmd: "/model", Desc: "Choose model"},
			{Cmd: "/copy", Desc: "Copy last response to clipboard"},
			{Cmd: "/clear", Desc: "Clear transcript"},
			{Cmd: "/exit", Desc: "Exit"},
			{Cmd: "/quit", Desc: "Exit"},
		},
		modelOptions: []string{
			"gpt-4.1-mini",
			"gpt-4.1",
			"gpt-4o-mini",
			"gpt-4o",
		},
	}
	m.rebuildTranscriptFromSession()
	m.inputHistory = loadCommandHistory()
	return m
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}

type agentMsg struct {
	ev agentEvent
}

func waitAgentEvent(ch <-chan agentEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return agentMsg{ev: ev}
	}
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		if m.streaming {
			m.dots = (m.dots + 1) % 4
		}
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.themeOpen && !m.streaming {
				m.themeOpen = false
				return m, nil
			}
			if m.modelPickerOpen && !m.streaming {
				m.modelPickerOpen = false
				return m, nil
			}
			if m.sessionPickerOpen && !m.streaming {
				m.sessionPickerOpen = false
				return m, nil
			}
			if m.cmdOpen && !m.streaming {
				m.cmdOpen = false
				m.cmdMatches = nil
				m.cmdSel = 0
				return m, nil
			}
			if m.streaming && m.runCancel != nil {
				m.runCancel()
				m.runCancel = nil
				m.streaming = false
				m.seenToken = false
				m.err = "cancelled"
				return m, nil
			}
			_ = m.cleanupSessionIfEmpty()
			return m, tea.Quit
		case "up", "ctrl+p":
			if m.streaming {
				return m, nil
			}
			if m.themeOpen {
				if m.themeSel > 0 {
					m.themeSel--
				}
				return m, nil
			}
			if m.modelPickerOpen {
				if m.modelSel > 0 {
					m.modelSel--
				}
				return m, nil
			}
			if m.sessionPickerOpen {
				if m.sessionSel > 0 {
					m.sessionSel--
				}
				return m, nil
			}
			if m.cmdOpen {
				if m.cmdSel > 0 {
					m.cmdSel--
				}
				return m, nil
			}
			m.historyUp()
			return m, nil
		case "down", "ctrl+n":
			if m.streaming {
				return m, nil
			}
			if m.themeOpen {
				if m.themeSel < len(m.themes)-1 {
					m.themeSel++
				}
				return m, nil
			}
			if m.modelPickerOpen {
				if m.modelSel < len(m.modelOptions)-1 {
					m.modelSel++
				}
				return m, nil
			}
			if m.sessionPickerOpen {
				if m.sessionSel < len(m.sessionList)-1 {
					m.sessionSel++
				}
				return m, nil
			}
			if m.cmdOpen {
				if m.cmdSel < len(m.cmdMatches)-1 {
					m.cmdSel++
				}
				return m, nil
			}
			m.historyDown()
			return m, nil
		case "ctrl+l":
			m.clearTranscript()
			return m, nil
		case "enter":
			if m.streaming {
				return m, nil
			}
			if m.sessionPickerOpen {
				if len(m.sessionList) > 0 && m.sessionSel >= 0 && m.sessionSel < len(m.sessionList) {
					sel := m.sessionList[m.sessionSel]
					_ = m.switchToSession(sel.ID)
				}
				m.sessionPickerOpen = false
				m.layout()
				return m, nil
			}
			if m.themeOpen {
				if len(m.themes) > 0 && m.themeSel >= 0 && m.themeSel < len(m.themes) {
					m.applyTheme(m.themes[m.themeSel])
					_ = m.persistSession()
					m.savePreferences()
				}
				m.themeOpen = false
				m.layout()
				return m, nil
			}
			if m.modelPickerOpen {
				if len(m.modelOptions) > 0 && m.modelSel >= 0 && m.modelSel < len(m.modelOptions) {
					model := m.modelOptions[m.modelSel]
					m.cfg.Model = model
					m.agent.SetModel(model)
					_ = m.persistSession()
					m.savePreferences()
				}
				m.modelPickerOpen = false
				m.layout()
				return m, nil
			}
			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}
			if m.cmdOpen {
				// If the palette is open, run the selected match.
				if len(m.cmdMatches) > 0 && m.cmdSel >= 0 && m.cmdSel < len(m.cmdMatches) {
					cmd := m.cmdMatches[m.cmdSel].Cmd
					m.input.SetValue("")
					m.cmdOpen = false
					m.cmdMatches = nil
					m.cmdSel = 0
					return m, m.runSlashCommand(cmd)
				}
				// No matches; just close.
				m.cmdOpen = false
				return m, nil
			}
			if strings.HasPrefix(prompt, "/") {
				m.input.SetValue("")
				return m, m.runSlashCommand(prompt)
			}
			m.pushHistory(prompt)
			m.histIdx = -1
			m.histDraft = ""
			m.input.SetValue("")
			m.appendOutput("\n" + userLine(prompt) + "\n")
			userMsgID, _ := newSessionID()
			assistantMsgID, _ := newSessionID()
			m.appendSessionMessage(sessionMessage{
				ID:        userMsgID,
				Role:      "user",
				Content:   prompt,
				CreatedAt: time.Now(),
				Meta: messageMeta{
					Model:            m.cfg.Model,
					SessionMessageNo: len(m.sess.Messages) + 1,
				},
			})
			m.pending = &pendingAssistant{
				UserMessageID:   userMsgID,
				AssistantMsgID:  assistantMsgID,
				StartedAt:       time.Now(),
				InputChars:      len(prompt),
				Model:           m.cfg.Model,
				Tools:           nil,
				openToolIndexes: nil,
				InputTokEst:     approxTokenCountChars(len(prompt)),
				OutputTokEst:    0,
			}
			m.streaming = true
			m.seenToken = false
			m.dots = 0
			m.err = ""
			runCtx, cancel := context.WithCancel(context.Background())
			m.runCancel = cancel
			m.events = m.agent.Run(runCtx, prompt)
			return m, waitAgentEvent(m.events)
		}

	case agentMsg:
		switch msg.ev.Type {
		case agentEventToken:
			m.seenToken = true
			m.appendOutput(msg.ev.Text)
			if m.pending != nil {
				m.pending.OutputChars += len(msg.ev.Text)
				m.pending.StreamedChunks++
				m.pending.OutputTokEst += approxTokenCountChars(len(msg.ev.Text))
			}
			return m, waitAgentEvent(m.events)
		case agentEventToolStart:
			if m.pending != nil {
				toolID, _ := newSessionID()
				rec := toolRecord{
					ID:        toolID,
					Name:      deriveToolName(msg.ev.Text),
					Input:     oneLine(msg.ev.Text),
					StartedAt: time.Now(),
				}
				m.pending.Tools = append(m.pending.Tools, rec)
				m.pending.openToolIndexes = append(m.pending.openToolIndexes, len(m.pending.Tools)-1)
			}
			return m, waitAgentEvent(m.events)
		case agentEventToolEnd:
			if m.pending != nil {
				if len(m.pending.openToolIndexes) > 0 {
					i := m.pending.openToolIndexes[0]
					m.pending.openToolIndexes = m.pending.openToolIndexes[1:]
					if i >= 0 && i < len(m.pending.Tools) {
						m.pending.Tools[i].Output = oneLine(msg.ev.Text)
						m.pending.Tools[i].EndedAt = time.Now()
					}
				}
			}
			return m, waitAgentEvent(m.events)
		case agentEventError:
			m.streaming = false
			if m.runCancel != nil {
				m.runCancel = nil
			}
			m.err = msg.ev.Text
			if m.pending != nil {
				// Save an assistant error message record for traceability.
				m.appendSessionMessage(sessionMessage{
					ID:        m.pending.AssistantMsgID,
					Role:      "assistant",
					Content:   "error: " + msg.ev.Text,
					CreatedAt: m.pending.StartedAt,
					Usage: usageStats{
						InputTokens:     approxTokenCountChars(m.pending.InputChars),
						OutputTokens:    approxTokenCountChars(m.pending.OutputChars),
						TotalTokens:     approxTokenCountChars(m.pending.InputChars) + approxTokenCountChars(m.pending.OutputChars),
						OutputChars:     m.pending.OutputChars,
						ReasoningTokens: 0,
					},
					Tools: m.pending.Tools,
					Meta: messageMeta{
						Model:            m.pending.Model,
						StreamedChunks:   m.pending.StreamedChunks,
						SessionMessageNo: len(m.sess.Messages) + 1,
					},
				})
				m.pending = nil
				_ = m.persistSession()
			}
			return m, nil
		case agentEventDone:
			m.streaming = false
			if m.runCancel != nil {
				m.runCancel = nil
			}
			// Marker-only, no "assistant" label.
			final := strings.TrimSpace(msg.ev.Text)
			m.appendOutput("\n" + assistantHeader() + final + "\n")
			if m.pending != nil {
				inTok := approxTokenCountChars(m.pending.InputChars)
				outTok := approxTokenCountChars(max(m.pending.OutputChars, len(final)))
				m.appendSessionMessage(sessionMessage{
					ID:        m.pending.AssistantMsgID,
					Role:      "assistant",
					Content:   final,
					CreatedAt: m.pending.StartedAt,
					Usage: usageStats{
						InputTokens:     max(inTok, m.pending.InputTokEst),
						OutputTokens:    max(outTok, m.pending.OutputTokEst),
						TotalTokens:     max(inTok, m.pending.InputTokEst) + max(outTok, m.pending.OutputTokEst),
						ReasoningTokens: 0,
						OutputChars:     max(m.pending.OutputChars, len(final)),
					},
					Tools: m.pending.Tools,
					Meta: messageMeta{
						Model:            m.pending.Model,
						StreamedChunks:   m.pending.StreamedChunks,
						SessionMessageNo: len(m.sess.Messages) + 1,
					},
				})
				m.pending = nil
			}
			_ = m.persistSession()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.updateCommandPalette()
	return m, cmd
}

func (m *tuiModel) clearTranscript() {
	m.outText = ""
	m.output.SetContent("")
	m.output.GotoBottom()
	m.err = ""
	m.sess.Messages = nil
	m.pending = nil
}

func (m *tuiModel) appendSessionMessage(msg sessionMessage) {
	if msg.ID == "" {
		id, _ := newSessionID()
		msg.ID = id
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	m.sess.Messages = append(m.sess.Messages, msg)
}

func (m *tuiModel) rebuildTranscriptFromSession() {
	m.outText = ""
	// Prefer rich messages when available.
	if len(m.sess.Messages) > 0 {
		for _, msg := range m.sess.Messages {
			switch msg.Role {
			case "user":
				m.outText += "\n" + userLine(strings.TrimSpace(msg.Content)) + "\n"
			case "assistant":
				m.outText += "\n" + assistantHeader() + strings.TrimSpace(msg.Content) + "\n"
			default:
				m.outText += "\n" + strings.TrimSpace(msg.Content) + "\n"
			}
		}
		return
	}
	// Legacy fallback.
	for _, t := range m.sess.Turns {
		m.outText += "\n" + userLine(strings.TrimSpace(t.User)) + "\n"
		m.outText += "\n" + assistantHeader() + strings.TrimSpace(t.Assistant) + "\n"
	}
}

func deriveToolName(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return "tool"
	}
	// Handles logs like "tool: sql_readonly" or raw function payload snippets.
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
	// Rough heuristic for English-like text.
	t := chars / 4
	if t < 1 {
		return 1
	}
	return t
}

func messagesToTurns(msgs []sessionMessage) []chatTurn {
	if len(msgs) == 0 {
		return nil
	}
	var out []chatTurn
	pendingUser := ""
	for _, m := range msgs {
		switch m.Role {
		case "user":
			pendingUser = m.Content
		case "assistant":
			out = append(out, chatTurn{
				User:      pendingUser,
				Assistant: m.Content,
			})
			pendingUser = ""
		}
	}
	return out
}

func (m *tuiModel) pushHistory(prompt string) {
	if strings.TrimSpace(prompt) == "" {
		return
	}
	if n := len(m.inputHistory); n > 0 && m.inputHistory[n-1] == prompt {
		return
	}
	m.inputHistory = append(m.inputHistory, prompt)
	if len(m.inputHistory) > maxHistoryLines {
		m.inputHistory = m.inputHistory[len(m.inputHistory)-maxHistoryLines:]
	}
	appendCommandHistory(prompt)
}

func (m *tuiModel) historyUp() {
	if len(m.inputHistory) == 0 {
		return
	}
	if m.histIdx == -1 {
		m.histDraft = m.input.Value()
		m.histIdx = len(m.inputHistory) - 1
		m.input.SetValue(m.inputHistory[m.histIdx])
		return
	}
	if m.histIdx > 0 {
		m.histIdx--
		m.input.SetValue(m.inputHistory[m.histIdx])
	}
}

func (m *tuiModel) historyDown() {
	if len(m.inputHistory) == 0 {
		return
	}
	if m.histIdx == -1 {
		return
	}
	if m.histIdx < len(m.inputHistory)-1 {
		m.histIdx++
		m.input.SetValue(m.inputHistory[m.histIdx])
		return
	}
	// Back to draft
	m.histIdx = -1
	m.input.SetValue(m.histDraft)
}

func (m *tuiModel) runSlashCommand(cmd string) tea.Cmd {
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
	case "/session":
		if arg != "" {
			if err := m.switchToSession(arg); err != nil {
				m.appendOutput("\n" + assistantHeader() + "Could not open session " + arg + ": " + err.Error() + "\n")
			}
			return nil
		}
		// Open picker
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
		// Initialize selection to current model if present.
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
		// Session-scoped clear: wipe current session history + transcript and persist.
		m.clearTranscript()
		m.agent.ClearHistory()
		m.sess.Turns = nil
		// If the user cleared everything, delete the session file entirely.
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
		// Show a small hint in the transcript.
		m.appendOutput("\n" + assistantHeader() + "Unknown command. Try /clear, /exit, /model, or /theme.\n")
		return nil
	}
}

func (m tuiModel) lastAssistantContent() string {
	for i := len(m.sess.Messages) - 1; i >= 0; i-- {
		if m.sess.Messages[i].Role == "assistant" {
			return strings.TrimSpace(m.sess.Messages[i].Content)
		}
	}
	return ""
}

type slashCommand struct {
	Cmd  string
	Desc string
}

func (m *tuiModel) updateCommandPalette() {
	if m.themeOpen {
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
		return
	}
	if m.modelPickerOpen {
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
		return
	}
	if m.sessionPickerOpen {
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
		return
	}
	if m.streaming {
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

func (m tuiModel) View() string {
	th := m.theme

	// Claude-code-ish structure:
	// 1) single-line title/status
	// 2) transcript area
	// 3) single-line status/hint
	// 4) prompt/input

	// Keep header single-line; model is shown in the status line.
	headerLeft := th.Title.Render("pgwatch-copilot") + "  " + th.Meta.Render(fmt.Sprintf("db=%s", safeDSNHint(m.cfg.DSN)))
	headerInner := truncateWithEllipsis(headerLeft, max(40, m.width))
	header := th.HeaderBar.Width(max(40, m.width)).Render(headerInner)
	if m.err != "" {
		header += "\n" + th.Error.Render("error: "+m.err)
	}

	body := m.renderBody()
	if m.themeOpen {
		body = m.renderThemePicker()
	}
	if m.modelPickerOpen {
		body = m.renderModelPicker()
	}
	if m.sessionPickerOpen {
		body = m.renderSessionPicker()
	}

	leftStatus := "enter: send  ctrl+l: clear  esc: cancel/quit"
	if m.streaming && !m.seenToken {
		leftStatus = m.spin.View() + "  Thinking" + animatedDots(m.dots) + "  " + th.Meta.Render("(Esc to cancel)")
	} else if m.streaming && m.seenToken {
		leftStatus = m.spin.View() + "  Generating" + animatedDots(m.dots) + "  " + th.Meta.Render("(Esc to cancel)")
	}

	rightStatus := th.Meta.Render("model: " + m.cfg.Model + m.liveTokenUsageStatus())
	w := max(40, m.width)
	statusInner := m.statusLine(th.Hint.Render(leftStatus), rightStatus)
	// Absolute guarantee: never wrap, even with ANSI + padding.
	statusInner = ansi.Truncate(statusInner, w, "")
	status := th.StatusBar.Width(w).Render(statusInner)
	if m.streaming && !m.seenToken {
		// already handled above
	} else if m.streaming && m.seenToken {
		// already handled above
	}
	prompt := m.renderPrompt()

	// If the transcript is short, keep the prompt adjacent to it (no forced
	// empty viewport height above the prompt).
	fillerH := 0
	if !m.themeOpen && !m.modelPickerOpen && !m.sessionPickerOpen {
		used := lipgloss.Height(header) + lipgloss.Height(body) + lipgloss.Height(status) + lipgloss.Height(prompt)
		fillerH = m.height - used
		if fillerH < 0 {
			fillerH = 0
		}
	}
	filler := ""
	if fillerH > 0 {
		filler = strings.Repeat("\n", fillerH)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, prompt, filler)
}

func (m tuiModel) liveTokenUsageStatus() string {
	if m.pending == nil {
		return ""
	}

	inTok := m.pending.InputTokEst
	outTok := m.pending.OutputTokEst
	total := inTok + outTok

	// Subtle animation: pulse the separator while streaming.
	sep := "  ·  "
	if m.streaming {
		switch m.dots {
		case 0, 2:
			sep = "  •  "
		default:
			sep = "  ·  "
		}
	}
	return sep + fmt.Sprintf("tok ~%d (in %d / out %d)", total, inTok, outTok)
}

func (m *tuiModel) persistSession() error {
	if m.store == nil {
		return nil
	}
	m.sess.Turns = messagesToTurns(m.sess.Messages)
	return m.store.Save(m.sess)
}

func (m *tuiModel) savePreferences() {
	saveGlobalConfig(globalConfig{
		Model: m.cfg.Model,
		Theme: m.theme.Name,
	})
}

func (m *tuiModel) switchToSession(id string) error {
	if m.store == nil {
		return nil
	}
	// Try load; if it doesn't exist, create a new session with that id.
	sess, err := m.store.Load(id)
	if err != nil {
		now := time.Now()
		sess = session{
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

func (m *tuiModel) cleanupSessionIfEmpty() error {
	if m.store == nil {
		return nil
	}
	// If the current session has no conversation, delete its file.
	if m.sess.IsEmpty() {
		return m.store.Delete(m.sess.ID)
	}
	return nil
}

func (m tuiModel) renderSessionPicker() string {
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
		line := s.ID + th.Meta.Render("  ") + th.Meta.Render(s.UpdatedAt.Format("2006-01-02 15:04"))
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

func (m tuiModel) statusLine(left, right string) string {
	// Right-align model outside the input border, like Claude Code.
	w := max(40, m.width)
	// Ensure this line never wraps by truncating when needed.
	left = strings.ReplaceAll(left, "\n", " ")
	right = strings.ReplaceAll(right, "\n", " ")

	rw := lipgloss.Width(right)
	lw := lipgloss.Width(left)

	// Prefer truncating the left hint first to preserve model/tokens.
	if lw+1+rw > w {
		availLeft := w - rw - 1
		if availLeft < 0 {
			availLeft = 0
		}
		left = truncateWithEllipsis(left, availLeft)
		lw = lipgloss.Width(left)
	}
	// If still too long, truncate right as well.
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
	// Hard truncate with no wrapping, ANSI-safe.
	return ansi.Truncate(s, maxW, "…")
}

func (m *tuiModel) layout() {
	// Reserve lines: header + status + prompt.
	// Prompt box is 3 lines (top border + content + bottom border).
	headerLines := 1
	if m.err != "" {
		headerLines = 2
	}
	promptLines := m.promptHeight()
	bodyH := max(0, m.height-headerLines-1-promptLines)
	bodyW := max(40, m.width)
	m.bodyH = bodyH
	m.bodyW = bodyW

	m.output = viewport.New(bodyW, bodyH)
	m.output.SetContent(m.outText)
	m.output.GotoBottom()
}

func (m *tuiModel) appendOutput(s string) {
	m.outText += s
	m.output.SetContent(m.outText)
	m.output.GotoBottom()
}

func (m tuiModel) renderBody() string {
	// Transcript: if content is shorter than available space, don't force a tall
	// viewport (so the prompt can sit directly after the last message).
	contentH := lipgloss.Height(m.outText)
	if strings.TrimSpace(m.outText) == "" {
		contentH = 0
	}
	if m.bodyH > 0 && contentH > m.bodyH {
		left := m.output.View()
		left = lipgloss.NewStyle().Width(m.output.Width).Height(m.output.Height).Render(left)
		return left
	}
	// Non-scroll mode (short transcript): render as-is without fixed height.
	return lipgloss.NewStyle().Width(max(40, m.width)).Render(m.outText)
}

func (m tuiModel) renderPrompt() string {
	th := m.theme
	box := th.PromptBox.Copy().
		Border(th.PromptBorder).
		BorderForeground(th.BorderColor).
		Padding(0, 1).
		Width(max(10, m.width-2))

	// Ensure the prompt line is visually distinct like Claude Code.
	content := m.input.View()
	if m.cmdOpen {
		content += "\n" + m.renderCommandPalette()
	}
	return box.Render(content)
}

func (m tuiModel) promptHeight() int {
	// 1 line input + optional palette lines + 2 border lines
	lines := 1
	if m.cmdOpen {
		lines += min(6, len(m.cmdMatches)) // show up to 6 commands
	}
	return lines + 2
}

func (m tuiModel) renderModelPicker() string {
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

func (m tuiModel) renderCommandPalette() string {
	th := m.theme
	maxItems := min(6, len(m.cmdMatches))
	if maxItems == 0 {
		return th.Meta.Render("no commands")
	}

	var b strings.Builder
	for i := 0; i < maxItems; i++ {
		c := m.cmdMatches[i]
		line := c.Cmd
		if c.Desc != "" {
			line += th.Meta.Render("  ") + th.Meta.Render(c.Desc)
		}

		if i == m.cmdSel {
			b.WriteString(th.CmdSelected.Render(th.SelectGlyph + line))
		} else {
			b.WriteString(th.CmdItem.Render("  " + line))
		}
		if i != maxItems-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func safeDSNHint(dsn string) string {
	// Avoid rendering full creds in the UI.
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

func tail[T any](in []T, n int) []T {
	if len(in) <= n {
		return in
	}
	return in[len(in)-n:]
}

func userLine(prompt string) string {
	// Uses currentTheme so it updates when /theme changes.
	return currentTheme.UserMark.Render(currentTheme.UserGlyph) + prompt
}

func assistantHeader() string {
	// Uses currentTheme so it updates when /theme changes.
	return currentTheme.AssistantMark.Render(currentTheme.AssistantGlyph)
}

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

	// Full-surface theming knobs
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

func (m *tuiModel) applyTheme(t theme) {
	m.theme = t
	currentTheme = t
	m.input.Prompt = t.Prompt.Render("› ")
	m.spin.Style = t.Accent
	m.spin.Spinner = t.Spinner
}

func (m tuiModel) renderThemePicker() string {
	active := m.theme
	preview := active
	if len(m.themes) > 0 && m.themeSel >= 0 && m.themeSel < len(m.themes) {
		preview = m.themes[m.themeSel]
	}

	title := preview.Title.Render("Select theme")
	help := preview.Meta.Render("↑/↓ preview • enter apply • esc cancel")

	// Left list
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

	// Right preview
	sample := preview.UserMark.Render(preview.UserGlyph) + "show me customers and orders\n\n" +
		preview.AssistantMark.Render(preview.AssistantGlyph) + "Sure — I’ll query `app.customers` and `app.orders` and summarize.\n"

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

	// Show note if preview differs from active.
	if preview.Name != active.Name {
		box += "\n" + preview.Meta.Render("previewing: "+preview.Name+"  (current: "+active.Name+")")
	}

	return lipgloss.Place(m.width, m.output.Height, lipgloss.Center, lipgloss.Center, box)
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
