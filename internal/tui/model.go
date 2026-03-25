package tui

import (
	"context"
	"os"
	"strings"
	"time"

	"ask-postgres/internal/agent"
	"ask-postgres/internal/config"
	"ask-postgres/internal/custom"
	"ask-postgres/internal/history"
	"ask-postgres/internal/session"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// modelIDPlaceholderCmd is a palette-only hint; Enter/Tab expand to "/model " for typing a real id.
const modelIDPlaceholderCmd = "/model <id>"

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

	input   textarea.Model
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

	sessionPickerOpen    bool
	sessionList          []session.Session
	sessionSel           int
	sessionDeleteConfirm bool

	customStore      *custom.Store
	customPickerOpen bool
	customList       []custom.Command
	customSel        int

	settingsOpen     bool
	settingsFormOpen bool
	settingsMenuSel  int
	settingsInputs   []textinput.Model
	settingsSel      int

	shortcutsOpen bool

	streaming   bool
	seenToken   bool
	events      <-chan agent.Event
	promptQueue []string
	pending     *pendingAssistant

	err string
}

type pendingAssistant struct {
	SessionID       string
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

	in := textarea.New()
	in.Placeholder = "Type a question, / for commands, ? for shortcuts"
	in.Focus()
	in.CharLimit = 4000
	in.ShowLineNumbers = false
	in.Prompt = ""
	in.SetHeight(1)
	in.SetWidth(80)
	in.MaxHeight = 10
	in.FocusedStyle.CursorLine = lipgloss.NewStyle()
	in.FocusedStyle.Base = lipgloss.NewStyle()
	in.BlurredStyle.CursorLine = lipgloss.NewStyle()
	in.BlurredStyle.Base = lipgloss.NewStyle()
	in.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

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
			{Cmd: "/session", Desc: "Switch/resume session (/session <name|id>, creates if missing)"},
			{Cmd: "/session rename", Desc: "Rename current session (/session rename <name>)"},
			{Cmd: "/theme", Desc: "Choose theme"},
			{Cmd: "/model", Desc: "Choose model (picker)"},
			{Cmd: modelIDPlaceholderCmd, Desc: "Set model by provider id (replace <id>)"},
			{Cmd: "/settings", Desc: "Manage provider settings"},
			{Cmd: "/copy", Desc: "Copy last response to clipboard"},
			{Cmd: "/clear", Desc: "Clear transcript"},
			{Cmd: "/create-custom", Desc: "Save a custom command (/create-custom <name> <prompt>)"},
			{Cmd: "/customs", Desc: "Browse and run saved custom commands"},
			{Cmd: "/delete-custom", Desc: "Delete a custom command (/delete-custom <name>)"},
			{Cmd: "/exit", Desc: "Exit"},
		},
		modelOptions: buildModelOptions(),
	}
	m.initSettingsForm()
	if cs, err := custom.NewStore(); err == nil {
		m.customStore = cs
	}
	m.rebuildTranscriptFromSession()
	m.inputHistory = history.Load()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spin.Tick)
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
		if m.settingsOpen {
			return m.handleSettingsKey(msg)
		}
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
				if m.sessionDeleteConfirm {
					m.sessionDeleteConfirm = false
					m.layout()
					return m, nil
				}
				m.sessionPickerOpen = false
				m.sessionDeleteConfirm = false
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
				if m.sessionDeleteConfirm {
					return m, nil
				}
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
				if m.sessionDeleteConfirm {
					return m, nil
				}
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
		case "ctrl+d":
			if m.sessionPickerOpen && !m.sessionDeleteConfirm && len(m.sessionList) > 0 {
				m.sessionDeleteConfirm = true
				m.layout()
				return m, nil
			}
		case "y", "Y":
			if m.sessionPickerOpen && m.sessionDeleteConfirm {
				return m.confirmDeleteSelectedSession()
			}
		case "n", "N":
			if m.sessionPickerOpen && m.sessionDeleteConfirm {
				m.sessionDeleteConfirm = false
				m.layout()
				return m, nil
			}
		case "tab":
			if m.cmdOpen && len(m.cmdMatches) > 0 {
				idx := m.cmdSel
				if idx < 0 || idx >= len(m.cmdMatches) {
					idx = 0
				}
				sel := m.cmdMatches[idx]
				insert := sel.Cmd + " "
				if sel.Cmd == modelIDPlaceholderCmd {
					insert = "/model "
				}
				m.setInputValue(insert)
				m.input.CursorEnd()
				m.updateCommandPalette()
				return m, nil
			}
			return m, nil
		case "enter":
			return m.handleEnter()
		case "ctrl+j":
			m.input.InsertString("\n")
			m.input.SetHeight(min(10, max(1, m.input.LineCount())))
			m.layout()
			return m, nil
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
	m.input.SetHeight(min(10, max(1, m.input.LineCount())))
	m.layout()
	m.updateCommandPalette()
	return m, cmd
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.sessionPickerOpen {
		if m.sessionDeleteConfirm {
			return m.confirmDeleteSelectedSession()
		}
		if len(m.sessionList) > 0 && m.sessionSel >= 0 && m.sessionSel < len(m.sessionList) {
			sel := m.sessionList[m.sessionSel]
			_ = m.switchToSession(sel.ID)
		}
		m.sessionPickerOpen = false
		m.sessionDeleteConfirm = false
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
	curLine := strings.TrimSpace(m.currentLine())
	if m.cmdOpen {
		if len(m.cmdMatches) > 0 && m.cmdSel >= 0 && m.cmdSel < len(m.cmdMatches) {
			sel := m.cmdMatches[m.cmdSel]
			if sel.Cmd == modelIDPlaceholderCmd {
				m.resetInput()
				m.setInputValue("/model ")
				m.input.CursorEnd()
				m.cmdOpen = false
				m.cmdMatches = nil
				m.cmdSel = 0
				m.layout()
				m.updateCommandPalette()
				return m, nil
			}
			m.resetInput()
			m.cmdOpen = false
			m.cmdMatches = nil
			m.cmdSel = 0
			if sel.Prompt != "" {
				return m, m.submitPrompt(sel.Prompt)
			}
			if strings.Contains(curLine, " ") {
				return m, m.runSlashCommand(curLine)
			}
			return m, m.runSlashCommand(sel.Cmd)
		}
		m.cmdOpen = false
		m.cmdMatches = nil
		m.cmdSel = 0
	}
	if strings.HasPrefix(prompt, "/") && !strings.Contains(prompt, "\n") {
		m.resetInput()
		return m, m.runSlashCommand(prompt)
	}
	return m, m.submitPrompt(prompt)
}

// submitPrompt sends a prompt to the agent. If already streaming, the prompt
// is queued and will run automatically when the current request finishes.
func (m *Model) submitPrompt(prompt string) tea.Cmd {
	if m.streaming {
		m.promptQueue = append(m.promptQueue, prompt)
		m.resetInput()
		m.appendOutput("\n" + currentTheme.Meta.Render("  (queued: "+prompt+")") + "\n")
		return nil
	}

	m.pushHistory(prompt)
	m.histIdx = -1
	m.histDraft = ""
	m.resetInput()
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
		SessionID:      m.sess.ID,
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
		if m.pending != nil && m.pending.SessionID == m.sess.ID {
			m.seenToken = true
			m.appendOutput(msg.ev.Text)
		}
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
				Name:      friendlyToolName(deriveToolName(msg.ev.Text)),
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
		if m.pending == nil || m.pending.SessionID == m.sess.ID {
			m.err = msg.ev.Text
		}
		if m.pending != nil {
			errMsg := session.Message{
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
					Model:          m.pending.Model,
					StreamedChunks: m.pending.StreamedChunks,
				},
			}
			_ = m.appendMessageToSessionByID(m.pending.SessionID, errMsg)
			m.pending = nil
		}
		return m, nil
	case agent.EventDone:
		m.streaming = false
		if m.runCancel != nil {
			m.runCancel = nil
		}
		final := strings.TrimSpace(msg.ev.Text)
		if m.pending != nil && m.pending.SessionID == m.sess.ID {
			m.appendOutput("\n" + assistantHeader() + final + "\n")
		}
		if m.pending != nil {
			inTok := approxTokenCountChars(m.pending.InputChars)
			outTok := approxTokenCountChars(max(m.pending.OutputChars, len(final)))
			doneMsg := session.Message{
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
					Model:          m.pending.Model,
					StreamedChunks: m.pending.StreamedChunks,
				},
			}
			_ = m.appendMessageToSessionByID(m.pending.SessionID, doneMsg)
			m.pending = nil
		}
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

func (m *Model) appendMessageToSessionByID(sessionID string, msg session.Message) error {
	if sessionID == "" || m.store == nil {
		return nil
	}
	if msg.ID == "" {
		id, _ := session.NewID()
		msg.ID = id
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = currentTime()
	}

	if m.sess.ID == sessionID {
		msg.Meta.SessionMessageNo = len(m.sess.Messages) + 1
		m.sess.Messages = append(m.sess.Messages, msg)
		return m.persistSession()
	}

	sess, err := m.store.Load(sessionID)
	if err != nil {
		return err
	}
	msg.Meta.SessionMessageNo = len(sess.Messages) + 1
	sess.Messages = append(sess.Messages, msg)
	sess.Turns = messagesToTurns(sess.Messages)
	return m.store.Save(sess)
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
		m.setInputValue(m.inputHistory[m.histIdx])
		return
	}
	if m.histIdx > 0 {
		m.histIdx--
		m.setInputValue(m.inputHistory[m.histIdx])
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
		m.setInputValue(m.inputHistory[m.histIdx])
		return
	}
	m.histIdx = -1
	m.setInputValue(m.histDraft)
}

func (m *Model) layout() {
	m.input.SetWidth(max(10, m.width-6))
	m.input.SetHeight(min(10, max(1, m.input.LineCount())))
	for i := range m.settingsInputs {
		m.settingsInputs[i].Width = max(30, m.width-24)
	}

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

func (m *Model) initSettingsForm() {
	gc := config.Load()
	m.settingsInputs = make([]textinput.Model, 3)

	makeInput := func(placeholder, value string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.SetValue(strings.TrimSpace(value))
		ti.CharLimit = 512
		ti.Width = max(30, m.width-24)
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '*'
		return ti
	}

	m.settingsInputs[0] = makeInput("sk-...", gc.OpenAIAPIKey)
	m.settingsInputs[1] = makeInput("sk-ant-...", gc.AnthropicAPIKey)
	m.settingsInputs[2] = makeInput("AIza...", gc.GoogleAPIKey)
	m.settingsSel = 0
	m.settingsMenuSel = 0
	m.settingsFormOpen = false
	if len(m.settingsInputs) > 0 {
		m.settingsInputs[0].Focus()
	}
}

func (m *Model) focusSettingsInput(idx int) {
	if idx < 0 || idx >= len(m.settingsInputs) {
		return
	}
	for i := range m.settingsInputs {
		if i == idx {
			m.settingsInputs[i].Focus()
			continue
		}
		m.settingsInputs[i].Blur()
	}
	m.settingsSel = idx
}

func (m *Model) availableKeyCount(openai, anthropic, google string) int {
	count := 0
	if strings.TrimSpace(openai) != "" {
		count++
	}
	if strings.TrimSpace(anthropic) != "" {
		count++
	}
	if strings.TrimSpace(google) != "" {
		count++
	}
	return count
}

func (m *Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.settingsFormOpen {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.settingsOpen = false
			m.layout()
			return m, nil
		case "up", "down", "tab", "shift+tab":
			// Single menu item for now; keep keys no-op and predictable.
			return m, nil
		case "enter":
			m.settingsFormOpen = true
			m.focusSettingsInput(0)
			m.layout()
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc", "ctrl+c":
		m.settingsFormOpen = false
		m.layout()
		return m, nil
	case "ctrl+u":
		if m.settingsSel >= 0 && m.settingsSel < len(m.settingsInputs) {
			m.settingsInputs[m.settingsSel].SetValue("")
		}
		return m, nil
	case "up", "shift+tab":
		next := m.settingsSel - 1
		if next < 0 {
			next = len(m.settingsInputs) - 1
		}
		m.focusSettingsInput(next)
		return m, nil
	case "down", "tab":
		next := m.settingsSel + 1
		if next >= len(m.settingsInputs) {
			next = 0
		}
		m.focusSettingsInput(next)
		return m, nil
	case "enter":
		openai := strings.TrimSpace(m.settingsInputs[0].Value())
		anthropic := strings.TrimSpace(m.settingsInputs[1].Value())
		google := strings.TrimSpace(m.settingsInputs[2].Value())

		// Empty field means "no override" and falls back to environment/.env.
		effectiveOpenAI := openai
		if effectiveOpenAI == "" {
			effectiveOpenAI = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		}
		effectiveAnthropic := anthropic
		if effectiveAnthropic == "" {
			effectiveAnthropic = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
		}
		effectiveGoogle := google
		if effectiveGoogle == "" {
			effectiveGoogle = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
		}

		if m.availableKeyCount(effectiveOpenAI, effectiveAnthropic, effectiveGoogle) == 0 {
			m.err = "at least one provider key is required (OpenAI, Anthropic, or Google)"
			return m, nil
		}

		gc := config.Load()
		gc.OpenAIAPIKey = openai
		gc.AnthropicAPIKey = anthropic
		gc.GoogleAPIKey = google
		config.Save(gc)

		// Apply overrides to this running process immediately.
		if openai != "" {
			_ = os.Setenv("OPENAI_API_KEY", openai)
		} else {
			_ = os.Unsetenv("OPENAI_API_KEY")
		}
		if anthropic != "" {
			_ = os.Setenv("ANTHROPIC_API_KEY", anthropic)
		} else {
			_ = os.Unsetenv("ANTHROPIC_API_KEY")
		}
		if google != "" {
			_ = os.Setenv("GOOGLE_API_KEY", google)
		} else {
			_ = os.Unsetenv("GOOGLE_API_KEY")
		}

		m.settingsOpen = false
		m.settingsFormOpen = false
		m.err = ""
		m.appendOutput("\n" + assistantHeader() + "Saved LLM provider API key settings.\n")
		m.layout()
		return m, nil
	}

	var cmd tea.Cmd
	m.settingsInputs[m.settingsSel], cmd = m.settingsInputs[m.settingsSel].Update(msg)
	return m, cmd
}

func (m *Model) persistSession() error {
	if m.store == nil {
		return nil
	}
	m.sess.Turns = messagesToTurns(m.sess.Messages)
	return m.store.Save(m.sess)
}

func (m *Model) savePreferences() {
	gc := config.Load()
	gc.Model = m.cfg.Model
	gc.Theme = m.theme.Name
	config.Save(gc)
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
