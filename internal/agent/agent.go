package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"pgwatch-copilot/internal/config"
	"pgwatch-copilot/internal/pgtools"
	"pgwatch-copilot/internal/session"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/prompts"
	"github.com/tmc/langchaingo/tools"
)

// Config holds the parameters needed to construct an Agent.
type Config struct {
	Model      string
	OpenAIBase string

	MaxRows int
	Timeout time.Duration

	Pool *pgxpool.Pool
}

// Agent wraps the LLM + tool executor and manages chat history.
type Agent struct {
	cfg     Config
	mu      sync.Mutex
	history []session.ChatTurn
}

// New validates the config and returns a ready-to-use Agent.
func New(_ context.Context, cfg Config) (*Agent, error) {
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
	return &Agent{cfg: cfg}, nil
}

// Run sends a user prompt to the LLM and streams events back on the returned
// channel. The channel is closed when the agent finishes.
func (a *Agent) Run(ctx context.Context, userPrompt string) <-chan Event {
	events := make(chan Event, 256)

	go func() {
		defer close(events)

		handler := &callbacksHandler{out: events}

		llm, err := a.newLLM(ctx, handler)
		if err != nil {
			events <- Event{Type: EventError, Text: err.Error()}
			return
		}

		toolset := []tools.Tool{
			pgtools.NewSchemaOverview(a.cfg.Pool),
			pgtools.NewDescribeTable(a.cfg.Pool),
			pgtools.NewSQLReadonly(a.cfg.Pool, a.cfg.MaxRows, a.cfg.Timeout),
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
			events <- Event{Type: EventError, Text: err.Error()}
			return
		}

		final := ""
		if v, ok := out["output"]; ok {
			final, _ = v.(string)
		}
		if final == "" {
			b, _ := json.MarshalIndent(out, "", "  ")
			final = string(b)
		}

		a.AppendHistory(userPrompt, final)
		events <- Event{Type: EventDone, Text: final}
	}()

	return events
}

func (a *Agent) historyMessageFormatters() []prompts.MessageFormatter {
	a.mu.Lock()
	defer a.mu.Unlock()

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

// AppendHistory adds a user/assistant exchange to the rolling window.
func (a *Agent) AppendHistory(user, assistant string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = append(a.history, session.ChatTurn{User: user, Assistant: assistant})
	if len(a.history) > 12 {
		a.history = a.history[len(a.history)-12:]
	}
}

// SetHistory replaces the entire history with the given turns.
func (a *Agent) SetHistory(turns []session.ChatTurn) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = append([]session.ChatTurn(nil), turns...)
	if len(a.history) > 12 {
		a.history = a.history[len(a.history)-12:]
	}
}

// History returns a snapshot of the current chat history.
func (a *Agent) History() []session.ChatTurn {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]session.ChatTurn(nil), a.history...)
}

// ClearHistory removes all chat history.
func (a *Agent) ClearHistory() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = nil
}

func detectProvider(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude"):
		return "anthropic"
	case strings.HasPrefix(m, "gemini"):
		return "google"
	default:
		return "openai"
	}
}

func requireEnv(name string) (string, error) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return "", fmt.Errorf("missing %s environment variable", name)
	}
	return v, nil
}

func resolveAPIKey(envName string) (string, error) {
	cfg := config.Load()
	switch envName {
	case "OPENAI_API_KEY":
		if strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
			return strings.TrimSpace(cfg.OpenAIAPIKey), nil
		}
	case "ANTHROPIC_API_KEY":
		if strings.TrimSpace(cfg.AnthropicAPIKey) != "" {
			return strings.TrimSpace(cfg.AnthropicAPIKey), nil
		}
	case "GOOGLE_API_KEY":
		if strings.TrimSpace(cfg.GoogleAPIKey) != "" {
			return strings.TrimSpace(cfg.GoogleAPIKey), nil
		}
	}
	return requireEnv(envName)
}

func (a *Agent) newLLM(ctx context.Context, handler callbacks.Handler) (llms.Model, error) {
	model := a.Model()

	switch detectProvider(model) {
	case "anthropic":
		key, err := resolveAPIKey("ANTHROPIC_API_KEY")
		if err != nil {
			return nil, err
		}
		return anthropic.New(
			anthropic.WithToken(key),
			anthropic.WithModel(model),
		)

	case "google":
		key, err := resolveAPIKey("GOOGLE_API_KEY")
		if err != nil {
			return nil, err
		}
		return googleai.New(ctx,
			googleai.WithAPIKey(key),
			googleai.WithDefaultModel(model),
		)

	default:
		key, err := resolveAPIKey("OPENAI_API_KEY")
		if err != nil {
			return nil, err
		}
		opts := []openai.Option{
			openai.WithToken(key),
			openai.WithModel(model),
			openai.WithCallback(handler),
		}
		if strings.TrimSpace(a.cfg.OpenAIBase) != "" {
			opts = append(opts, openai.WithBaseURL(strings.TrimSpace(a.cfg.OpenAIBase)))
		}
		return openai.New(opts...)
	}
}

// SetModel changes the LLM model for subsequent runs.
func (a *Agent) SetModel(model string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Model = model
}

// Model returns the currently configured LLM model name.
func (a *Agent) Model() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(a.cfg.Model) == "" {
		return "gpt-4.1-mini"
	}
	return a.cfg.Model
}
