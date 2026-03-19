package session

import "time"

// ChatTurn pairs a user prompt with an assistant response.
type ChatTurn struct {
	User      string `json:"user,omitempty"`
	Assistant string `json:"assistant,omitempty"`
}

// Session represents a single conversation persisted on disk.
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Turns     []ChatTurn `json:"turns,omitempty"`
	Messages  []Message  `json:"messages,omitempty"`
}

// Message is a single chat message within a session.
type Message struct {
	ID        string      `json:"id"`
	Role      string      `json:"role"` // user|assistant|system
	Content   string      `json:"content"`
	CreatedAt time.Time   `json:"created_at"`
	Usage     UsageStats  `json:"usage,omitempty"`
	Tools     []ToolRecord `json:"tools,omitempty"`
	Meta      MessageMeta `json:"meta,omitempty"`
}

// UsageStats tracks estimated token consumption for a message.
type UsageStats struct {
	InputTokens     int `json:"input_tokens,omitempty"`
	OutputTokens    int `json:"output_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	TotalTokens     int `json:"total_tokens,omitempty"`
	OutputChars     int `json:"output_chars,omitempty"`
}

// ToolRecord captures a single tool invocation within a message.
type ToolRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Input     string    `json:"input,omitempty"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

// MessageMeta holds auxiliary metadata for a message.
type MessageMeta struct {
	Model            string `json:"model,omitempty"`
	StreamedChunks   int    `json:"streamed_chunks,omitempty"`
	SessionMessageNo int    `json:"session_message_no,omitempty"`
}

// IsEmpty reports whether the session contains no messages or turns.
func (s Session) IsEmpty() bool {
	return len(s.Messages) == 0 && len(s.Turns) == 0
}
