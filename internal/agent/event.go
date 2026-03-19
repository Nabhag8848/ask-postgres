package agent

// EventType classifies the kind of streaming event emitted by the agent.
type EventType string

const (
	EventToken     EventType = "token"
	EventToolStart EventType = "tool_start"
	EventToolEnd   EventType = "tool_end"
	EventError     EventType = "error"
	EventDone      EventType = "done"
)

// Event is a single streaming event sent from the agent goroutine to the TUI.
type Event struct {
	Type EventType
	Text string
}
