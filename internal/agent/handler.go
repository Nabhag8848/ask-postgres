package agent

import (
	"context"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
)

// callbacksHandler bridges langchaingo callbacks into the Event channel.
type callbacksHandler struct {
	out chan<- Event
}

func (h *callbacksHandler) HandleText(ctx context.Context, text string) {
	select {
	case <-ctx.Done():
		return
	case h.out <- Event{Type: EventToken, Text: text}:
	}
}
func (h *callbacksHandler) HandleLLMStart(context.Context, []string) {}
func (h *callbacksHandler) HandleLLMGenerateContentStart(context.Context, []llms.MessageContent) {
}
func (h *callbacksHandler) HandleLLMGenerateContentEnd(context.Context, *llms.ContentResponse) {}
func (h *callbacksHandler) HandleLLMError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	case h.out <- Event{Type: EventError, Text: err.Error()}:
	}
}
func (h *callbacksHandler) HandleChainStart(context.Context, map[string]any) {}
func (h *callbacksHandler) HandleChainEnd(context.Context, map[string]any)   {}
func (h *callbacksHandler) HandleChainError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	case h.out <- Event{Type: EventError, Text: err.Error()}:
	}
}
func (h *callbacksHandler) HandleToolStart(ctx context.Context, input string) {
	select {
	case <-ctx.Done():
		return
	case h.out <- Event{Type: EventToolStart, Text: input}:
	}
}
func (h *callbacksHandler) HandleToolEnd(ctx context.Context, output string) {
	select {
	case <-ctx.Done():
		return
	case h.out <- Event{Type: EventToolEnd, Text: output}:
	}
}
func (h *callbacksHandler) HandleToolError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	case h.out <- Event{Type: EventError, Text: err.Error()}:
	}
}
func (h *callbacksHandler) HandleAgentAction(context.Context, schema.AgentAction) {}
func (h *callbacksHandler) HandleAgentFinish(context.Context, schema.AgentFinish) {}
func (h *callbacksHandler) HandleRetrieverStart(context.Context, string)          {}
func (h *callbacksHandler) HandleRetrieverEnd(context.Context, string, []schema.Document) {
}
func (h *callbacksHandler) HandleStreamingFunc(ctx context.Context, chunk []byte) {
	select {
	case <-ctx.Done():
		return
	case h.out <- Event{Type: EventToken, Text: string(chunk)}:
	}
}
