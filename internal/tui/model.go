package tui

import (
	"context"
	"strings"
	"time"

	"pgwatch-copilot/internal/agent"
	"pgwatch-copilot/internal/config"
	"pgwatch-copilot/internal/custom"
	"pgwatch-copilot/internal/history"
	"pgwatch-copilot/internal/session"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Config carries the runtime values the TUI needs from the app layer.
type Config struct {
	DSN   string
	Model string
}

// Model is the Bubble Tea model for the entire TUI.
type Model struct {
	cfg   Config
	agent *agent.Agent
	store *session.Store
	sess  session.Session

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
	tabCycle   int
	tabPrefix  string

	modelPickerOpen bool
	modelOptions    []string
	modelSel        int

	sessionPickerOpen bool
	sessionList       []session.Session
	sessionSel        int

	customStore      *custom.Store
	customPickerOpen bool
	customList       []custom.Command
	customSel        int

	shortcutsOpen bool

	streaming    bool
	seenToken    bool
	events       <-chan agent.Event
	promptQueue  []string
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
	Tools           []session.ToolRecord
	openToolIndexes []int

	InputTokEst  int
	OutputTokEst int
}

// New creates the TUI model, wiring together the agent, store, and session.
func New(cfg Config, ag *agent.Agent, store *session.Store, sess session.Session) Model {
	themes := defaultThemes()
	active := themes[0]
	gc := config.Load()
	if gc.Theme != "" {
		for _, t := range themes {
			if t.Name == gc.Theme {
				active = t
				break
			}
		}
	}

	in := textinput.New()
	in.Placeholder = "Type a question, / for commands, ? for shortcuts"
	in.Focus()
	in.CharLimit = 2000
	in.Prompt = active.Prompt.Render("> ")

	vp := viewport.New(0, 0)
	vp.SetContent("")

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = active.Accent

	m := Model{
		cfg:     cfg,
		agent:   ag,
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
			{Cmd: "/help", Desc: "Show all commands and keybindings"},
			{Cmd: "/session", Desc: "Switch/resume session (/session <id>)"},
			{Cmd: "/session rename", Desc: "Rename current session (/session rename <name>)"},
			{Cmd: "/theme", Desc: "Choose theme"},
			{Cmd: "/model", Desc: "Choose model"},
			{Cmd: "/copy", Desc: "Copy last response to clipboard"},
			{Cmd: "/clear", Desc: "Clear transcript"},
			{Cmd: "/create-custom", Desc: "Save a custom command (/create-custom <name> <prompt>)"},
			{Cmd: "/customs", Desc: "Browse and run saved custom commands"},
			{Cmd: "/delete-custom", Desc: "Delete a custom command (/delete-custom <name>)"},
			{Cmd: "/exit", Desc: "Exit"},
		},
		modelOptions: []string{
			"gpt-4.1-mini",
			"gpt-4.1",
			"gpt-4o-mini",
			"gpt-4o",
			"o3-mini",
			"o4-mini",
			"claude-sonnet-4-20250514",
			"claude-3.5-haiku-20241022",
			"gemini-2.5-flash",
			"gemini-2.5-pro",
		},
	}
	if cs, err := custom.NewStore(); err == nil {
		m.customStore = cs
	}
	m.rebuildTranscriptFromSession()
	m.inputHistory = history.Load()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}

type agentMsg struct {
	ev agent.Event
}

func waitAgentEvent(ch <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return agentMsg{ev: ev}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.shortcutsOpen {
				m.shortcutsOpen = false
				return m, nil
			}
			if m.themeOpen {
				m.themeOpen = false
				return m, nil
			}
			if m.modelPickerOpen {
				m.modelPickerOpen = false
				return m, nil
			}
			if m.sessionPickerOpen {
				m.sessionPickerOpen = false
				return m, nil
			}
			if m.customPickerOpen {
				m.customPickerOpen = false
				return m, nil
			}
			if m.cmdOpen {
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
				m.promptQueue = nil
				m.err = "cancelled"
				return m, nil
			}
			_ = m.cleanupSessionIfEmpty()
			return m, tea.Quit
		case "up", "ctrl+p":
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
			if m.customPickerOpen {
				if m.customSel > 0 {
					m.customSel--
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
			if m.customPickerOpen {
				if m.customSel < len(m.customList)-1 {
					m.customSel++
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
		case "tab":
			if m.cmdOpen && len(m.cmdMatches) > 0 {
				idx := m.cmdSel
				if idx < 0 || idx >= len(m.cmdMatches) {
					idx = 0
				}
				sel := m.cmdMatches[idx]
				m.input.SetValue(sel.Cmd + " ")
				m.input.CursorEnd()
				m.updateCommandPalette()
				return m, nil
			}
			return m, nil
		case "enter":
			return m.handleEnter()
		}

	case agentMsg:
		return m.handleAgentEvent(msg)
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.shortcutsOpen {
			m.shortcutsOpen = false
			return m, nil
		}
		if keyMsg.String() == "?" && strings.TrimSpace(m.input.Value()) == "" {
			m.shortcutsOpen = true
			return m, nil
		}
	}

	prev := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prev {
		m.tabCycle = 0
		m.tabPrefix = ""
	}
	m.updateCommandPalette()
	return m, cmd
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
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
	if m.customPickerOpen {
		if len(m.customList) > 0 && m.customSel >= 0 && m.customSel < len(m.customList) {
			sel := m.customList[m.customSel]
			m.customPickerOpen = false
			m.layout()
			return m, m.submitPrompt(sel.Prompt)
		}
		m.customPickerOpen = false
		m.layout()
		return m, nil
	}
	prompt := strings.TrimSpace(m.input.Value())
	if prompt == "" {
		return m, nil
	}
	if m.cmdOpen {
		if len(m.cmdMatches) > 0 && m.cmdSel >= 0 && m.cmdSel < len(m.cmdMatches) {
			sel := m.cmdMatches[m.cmdSel]
			m.input.SetValue("")
			m.cmdOpen = false
			m.cmdMatches = nil
			m.cmdSel = 0
			if sel.Prompt != "" {
				return m, m.submitPrompt(sel.Prompt)
			}
			// If the user typed arguments (space in input), pass the full
			// input so args reach the command handler. Otherwise use the
			// palette selection (acts like tab-completion for partials).
			if strings.Contains(prompt, " ") {
				return m, m.runSlashCommand(prompt)
			}
			return m, m.runSlashCommand(sel.Cmd)
		}
		// No matches — close palette and fall through to normal handling.
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
	}
	if strings.HasPrefix(prompt, "/") {
		m.input.SetValue("")
		return m, m.runSlashCommand(prompt)
	}
	return m, m.submitPrompt(prompt)
}

// submitPrompt sends a prompt to the agent. If already streaming, the prompt
// is queued and will run automatically when the current request finishes.
func (m *Model) submitPrompt(prompt string) tea.Cmd {
	if m.streaming {
		m.promptQueue = append(m.promptQueue, prompt)
		m.input.SetValue("")
		m.appendOutput("\n" + currentTheme.Meta.Render("  (queued: "+prompt+")") + "\n")
		return nil
	}

	m.pushHistory(prompt)
	m.histIdx = -1
	m.histDraft = ""
	m.input.SetValue("")
	m.appendOutput("\n" + userLine(prompt) + "\n")

	userMsgID, _ := session.NewID()
	assistantMsgID, _ := session.NewID()
	now := currentTime()
	m.appendSessionMessage(session.Message{
		ID:        userMsgID,
		Role:      "user",
		Content:   prompt,
		CreatedAt: now,
		Meta: session.MessageMeta{
			Model:            m.cfg.Model,
			SessionMessageNo: len(m.sess.Messages) + 1,
		},
	})
	m.pending = &pendingAssistant{
		UserMessageID:  userMsgID,
		AssistantMsgID: assistantMsgID,
		StartedAt:      now,
		InputChars:     len(prompt),
		Model:          m.cfg.Model,
		InputTokEst:    approxTokenCountChars(len(prompt)),
	}
	m.streaming = true
	m.seenToken = false
	m.dots = 0
	m.err = ""
	runCtx, cancel := context.WithCancel(context.Background())
	m.runCancel = cancel
	m.events = m.agent.Run(runCtx, prompt)
	return waitAgentEvent(m.events)
}

func (m Model) handleAgentEvent(msg agentMsg) (tea.Model, tea.Cmd) {
	switch msg.ev.Type {
	case agent.EventToken:
		m.seenToken = true
		m.appendOutput(msg.ev.Text)
		if m.pending != nil {
			m.pending.OutputChars += len(msg.ev.Text)
			m.pending.StreamedChunks++
			m.pending.OutputTokEst += approxTokenCountChars(len(msg.ev.Text))
		}
		return m, waitAgentEvent(m.events)
	case agent.EventToolStart:
		if m.pending != nil {
			toolID, _ := session.NewID()
			rec := session.ToolRecord{
				ID:        toolID,
				Name:      deriveToolName(msg.ev.Text),
				Input:     oneLine(msg.ev.Text),
				StartedAt: currentTime(),
			}
			m.pending.Tools = append(m.pending.Tools, rec)
			m.pending.openToolIndexes = append(m.pending.openToolIndexes, len(m.pending.Tools)-1)
		}
		return m, waitAgentEvent(m.events)
	case agent.EventToolEnd:
		if m.pending != nil {
			if len(m.pending.openToolIndexes) > 0 {
				i := m.pending.openToolIndexes[0]
				m.pending.openToolIndexes = m.pending.openToolIndexes[1:]
				if i >= 0 && i < len(m.pending.Tools) {
					m.pending.Tools[i].Output = oneLine(msg.ev.Text)
					m.pending.Tools[i].EndedAt = currentTime()
				}
			}
		}
		return m, waitAgentEvent(m.events)
	case agent.EventError:
		m.streaming = false
		m.promptQueue = nil
		if m.runCancel != nil {
			m.runCancel = nil
		}
		m.err = msg.ev.Text
		if m.pending != nil {
			m.appendSessionMessage(session.Message{
				ID:        m.pending.AssistantMsgID,
				Role:      "assistant",
				Content:   "error: " + msg.ev.Text,
				CreatedAt: m.pending.StartedAt,
				Usage: session.UsageStats{
					InputTokens:  approxTokenCountChars(m.pending.InputChars),
					OutputTokens: approxTokenCountChars(m.pending.OutputChars),
					TotalTokens:  approxTokenCountChars(m.pending.InputChars) + approxTokenCountChars(m.pending.OutputChars),
					OutputChars:  m.pending.OutputChars,
				},
				Tools: m.pending.Tools,
				Meta: session.MessageMeta{
					Model:            m.pending.Model,
					StreamedChunks:   m.pending.StreamedChunks,
					SessionMessageNo: len(m.sess.Messages) + 1,
				},
			})
			m.pending = nil
			_ = m.persistSession()
		}
		return m, nil
	case agent.EventDone:
		m.streaming = false
		if m.runCancel != nil {
			m.runCancel = nil
		}
		final := strings.TrimSpace(msg.ev.Text)
		m.appendOutput("\n" + assistantHeader() + final + "\n")
		if m.pending != nil {
			inTok := approxTokenCountChars(m.pending.InputChars)
			outTok := approxTokenCountChars(max(m.pending.OutputChars, len(final)))
			m.appendSessionMessage(session.Message{
				ID:        m.pending.AssistantMsgID,
				Role:      "assistant",
				Content:   final,
				CreatedAt: m.pending.StartedAt,
				Usage: session.UsageStats{
					InputTokens:  max(inTok, m.pending.InputTokEst),
					OutputTokens: max(outTok, m.pending.OutputTokEst),
					TotalTokens:  max(inTok, m.pending.InputTokEst) + max(outTok, m.pending.OutputTokEst),
					OutputChars:  max(m.pending.OutputChars, len(final)),
				},
				Tools: m.pending.Tools,
				Meta: session.MessageMeta{
					Model:            m.pending.Model,
					StreamedChunks:   m.pending.StreamedChunks,
					SessionMessageNo: len(m.sess.Messages) + 1,
				},
			})
			m.pending = nil
		}
		_ = m.persistSession()
		if len(m.promptQueue) > 0 {
			next := m.promptQueue[0]
			m.promptQueue = m.promptQueue[1:]
			return m, m.submitPrompt(next)
		}
		return m, nil
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func currentTime() time.Time { return time.Now() }

func (m *Model) clearTranscript() {
	m.outText = ""
	m.output.SetContent("")
	m.output.GotoBottom()
	m.err = ""
	m.sess.Messages = nil
	m.pending = nil
}

func (m *Model) appendSessionMessage(msg session.Message) {
	if msg.ID == "" {
		id, _ := session.NewID()
		msg.ID = id
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = currentTime()
	}
	m.sess.Messages = append(m.sess.Messages, msg)
}

func (m *Model) rebuildTranscriptFromSession() {
	m.outText = ""
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
	for _, t := range m.sess.Turns {
		m.outText += "\n" + userLine(strings.TrimSpace(t.User)) + "\n"
		m.outText += "\n" + assistantHeader() + strings.TrimSpace(t.Assistant) + "\n"
	}
}

func (m *Model) pushHistory(prompt string) {
	if strings.TrimSpace(prompt) == "" {
		return
	}
	if n := len(m.inputHistory); n > 0 && m.inputHistory[n-1] == prompt {
		return
	}
	m.inputHistory = append(m.inputHistory, prompt)
	if len(m.inputHistory) > history.MaxLines {
		m.inputHistory = m.inputHistory[len(m.inputHistory)-history.MaxLines:]
	}
	history.Append(prompt)
}

func (m *Model) historyUp() {
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

func (m *Model) historyDown() {
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
	m.histIdx = -1
	m.input.SetValue(m.histDraft)
}

func (m *Model) layout() {
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

func (m *Model) appendOutput(s string) {
	m.outText += s
	m.output.SetContent(m.outText)
	m.output.GotoBottom()
}

func (m *Model) persistSession() error {
	if m.store == nil {
		return nil
	}
	m.sess.Turns = messagesToTurns(m.sess.Messages)
	return m.store.Save(m.sess)
}

func (m *Model) savePreferences() {
	config.Save(config.Global{
		Model: m.cfg.Model,
		Theme: m.theme.Name,
	})
}

func (m *Model) cleanupSessionIfEmpty() error {
	if m.store == nil {
		return nil
	}
	if m.sess.IsEmpty() {
		return m.store.Delete(m.sess.ID)
	}
	return nil
}
