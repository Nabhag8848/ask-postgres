package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/prompts"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/tools"

	"github.com/jackc/pgx/v5/pgxpool"
)

type agentConfig struct {
	Model      string
	OpenAIBase string

	MaxRows int
	Timeout time.Duration

	Pool *pgxpool.Pool
}

type agent struct {
	cfg     agentConfig
	mu      sync.Mutex
	history []chatTurn
}

type chatTurn struct {
	User      string
	Assistant string
}

type agentEventType string

const (
	agentEventToken     agentEventType = "token"
	agentEventToolStart agentEventType = "tool_start"
	agentEventToolEnd   agentEventType = "tool_end"
	agentEventError     agentEventType = "error"
	agentEventDone      agentEventType = "done"
)

type agentEvent struct {
	Type agentEventType
	Text string
}

func newAgent(_ context.Context, cfg agentConfig) (*agent, error) {
	if cfg.Pool == nil {
		return nil, errors.New("missing pg pool")
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4.1-mini"
	}
	if cfg.MaxRows <= 0 {
		cfg.MaxRows = 200
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &agent{cfg: cfg}, nil
}

func (a *agent) Run(ctx context.Context, userPrompt string) <-chan agentEvent {
	events := make(chan agentEvent, 256)

	go func() {
		defer close(events)

		handler := &tuiCallbacksHandler{out: events}

		llm, err := a.newLLM(handler)
		if err != nil {
			events <- agentEvent{Type: agentEventError, Text: err.Error()}
			return
		}

		toolset := []tools.Tool{
			newSchemaOverviewTool(a.cfg.Pool),
			newDescribeTableTool(a.cfg.Pool),
			newSQLReadonlyTool(a.cfg.Pool, a.cfg.MaxRows, a.cfg.Timeout),
		}

		sys := strings.TrimSpace(`
You are a Postgres analyst assistant running inside a terminal UI.
You have access to tools. Prefer tools over guessing.

Safety constraints:
- You must only use the provided tools to access the database.
- Use sql_readonly for ad-hoc SELECT queries only.

Rules:
- Before querying an unfamiliar table/alias, call describe_table to confirm column names.
- If a query errors (e.g. missing column), use describe_table and retry with correct columns.

When proposing a query, keep it efficient and safe (limit rows).
Summarize findings concisely and propose next checks.
`)

		openAIOpts := agents.NewOpenAIOption()
		extra := a.historyMessageFormatters()
		agt := agents.NewOpenAIFunctionsAgent(
			llm,
			toolset,
			openAIOpts.WithSystemMessage(sys),
			openAIOpts.WithExtraMessages(extra),
		)
		exec := agents.NewExecutor(agt,
			agents.WithCallbacksHandler(handler),
			agents.WithMaxIterations(8),
		)

		out, err := exec.Call(ctx, map[string]any{"input": userPrompt})
		if err != nil {
			events <- agentEvent{Type: agentEventError, Text: err.Error()}
			return
		}

		// Executor typically returns { "output": "..."} but we also guard.
		final := ""
		if v, ok := out["output"]; ok {
			final, _ = v.(string)
		}
		if final == "" {
			b, _ := json.MarshalIndent(out, "", "  ")
			final = string(b)
		}

		a.appendHistory(userPrompt, final)
		events <- agentEvent{Type: agentEventDone, Text: final}
	}()

	return events
}

func (a *agent) historyMessageFormatters() []prompts.MessageFormatter {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Keep only a small window to avoid huge prompts.
	const maxTurns = 12
	h := a.history
	if len(h) > maxTurns {
		h = h[len(h)-maxTurns:]
	}

	out := make([]prompts.MessageFormatter, 0, len(h)*2)
	for _, t := range h {
		if strings.TrimSpace(t.User) != "" {
			out = append(out, prompts.NewHumanMessagePromptTemplate(t.User, nil))
		}
		if strings.TrimSpace(t.Assistant) != "" {
			out = append(out, prompts.NewAIMessagePromptTemplate(t.Assistant, nil))
		}
	}
	return out
}

func (a *agent) appendHistory(user, assistant string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = append(a.history, chatTurn{User: user, Assistant: assistant})
	if len(a.history) > 12 {
		a.history = a.history[len(a.history)-12:]
	}
}

func (a *agent) newLLM(handler callbacks.Handler) (llms.Model, error) {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		return nil, errors.New("missing OPENAI_API_KEY")
	}

	model := a.Model()
	opts := []openai.Option{
		openai.WithToken(key),
		openai.WithModel(model),
		openai.WithCallback(handler),
	}
	if strings.TrimSpace(a.cfg.OpenAIBase) != "" {
		opts = append(opts, openai.WithBaseURL(strings.TrimSpace(a.cfg.OpenAIBase)))
	}
	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("openai.New: %w", err)
	}
	return llm, nil
}

func (a *agent) SetModel(model string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Model = model
}

func (a *agent) Model() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(a.cfg.Model) == "" {
		return "gpt-4.1-mini"
	}
	return a.cfg.Model
}

type tuiCallbacksHandler struct {
	out chan<- agentEvent
}

func (h *tuiCallbacksHandler) HandleText(ctx context.Context, text string) {
	select {
	case <-ctx.Done():
		return
	case h.out <- agentEvent{Type: agentEventToken, Text: text}:
	}
}
func (h *tuiCallbacksHandler) HandleLLMStart(context.Context, []string) {}
func (h *tuiCallbacksHandler) HandleLLMGenerateContentStart(context.Context, []llms.MessageContent) {
}
func (h *tuiCallbacksHandler) HandleLLMGenerateContentEnd(context.Context, *llms.ContentResponse) {}
func (h *tuiCallbacksHandler) HandleLLMError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	case h.out <- agentEvent{Type: agentEventError, Text: err.Error()}:
	}
}
func (h *tuiCallbacksHandler) HandleChainStart(context.Context, map[string]any) {}
func (h *tuiCallbacksHandler) HandleChainEnd(context.Context, map[string]any)   {}
func (h *tuiCallbacksHandler) HandleChainError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	case h.out <- agentEvent{Type: agentEventError, Text: err.Error()}:
	}
}
func (h *tuiCallbacksHandler) HandleToolStart(ctx context.Context, input string) {
	select {
	case <-ctx.Done():
		return
	case h.out <- agentEvent{Type: agentEventToolStart, Text: input}:
	}
}
func (h *tuiCallbacksHandler) HandleToolEnd(ctx context.Context, output string) {
	select {
	case <-ctx.Done():
		return
	case h.out <- agentEvent{Type: agentEventToolEnd, Text: output}:
	}
}
func (h *tuiCallbacksHandler) HandleToolError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	case h.out <- agentEvent{Type: agentEventError, Text: err.Error()}:
	}
}
func (h *tuiCallbacksHandler) HandleAgentAction(context.Context, schema.AgentAction) {}
func (h *tuiCallbacksHandler) HandleAgentFinish(context.Context, schema.AgentFinish) {}
func (h *tuiCallbacksHandler) HandleRetrieverStart(context.Context, string)          {}
func (h *tuiCallbacksHandler) HandleRetrieverEnd(context.Context, string, []schema.Document) {
}
func (h *tuiCallbacksHandler) HandleStreamingFunc(ctx context.Context, chunk []byte) {
	// Many providers send partial deltas here; Bubble Tea can render it directly.
	select {
	case <-ctx.Done():
		return
	case h.out <- agentEvent{Type: agentEventToken, Text: string(chunk)}:
	}
}
