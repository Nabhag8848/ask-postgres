package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type session struct {
	ID        string           `json:"id"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Turns     []chatTurn       `json:"turns,omitempty"`
	Messages  []sessionMessage `json:"messages,omitempty"`
}

type sessionMessage struct {
	ID        string       `json:"id"`
	Role      string       `json:"role"` // user|assistant|system
	Content   string       `json:"content"`
	CreatedAt time.Time    `json:"created_at"`
	Usage     usageStats   `json:"usage,omitempty"`
	Tools     []toolRecord `json:"tools,omitempty"`
	Meta      messageMeta  `json:"meta,omitempty"`
}

type usageStats struct {
	InputTokens     int `json:"input_tokens,omitempty"`
	OutputTokens    int `json:"output_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	TotalTokens     int `json:"total_tokens,omitempty"`
	OutputChars     int `json:"output_chars,omitempty"`
}

type toolRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Input     string    `json:"input,omitempty"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

type messageMeta struct {
	Model            string `json:"model,omitempty"`
	StreamedChunks   int    `json:"streamed_chunks,omitempty"`
	SessionMessageNo int    `json:"session_message_no,omitempty"`
}

type sessionStore struct {
	dir string
}

func newSessionStore() (*sessionStore, error) {
	dir, err := resolveSessionsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("MkdirAll(%s): %w", dir, err)
	}
	return &sessionStore{dir: dir}, nil
}

func resolveSessionsDir() (string, error) {
	// Cross-platform default: store under a dotfolder in the user's home:
	//   ~/.pgwatch-copilot/sessions
	//
	// Override with:
	//   PGWATCH_COPILOT_HOME=/some/path
	homeOverride := strings.TrimSpace(os.Getenv("PGWATCH_COPILOT_HOME"))
	if homeOverride != "" {
		return filepath.Join(homeOverride, "sessions"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("UserHomeDir: %w", err)
	}
	return filepath.Join(home, ".pgwatch-copilot", "sessions"), nil
}

func (s *sessionStore) New() (session, error) {
	id, err := newSessionID()
	if err != nil {
		return session{}, err
	}
	now := time.Now()
	sess := session{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.Save(sess); err != nil {
		return session{}, err
	}
	return sess, nil
}

func (s *sessionStore) Save(sess session) error {
	sess.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	path := filepath.Join(s.dir, sess.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write tmp session: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename session: %w", err)
	}
	return nil
}

func (s *sessionStore) Load(id string) (session, error) {
	path := filepath.Join(s.dir, id+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return session{}, fmt.Errorf("read session: %w", err)
	}
	var sess session
	if err := json.Unmarshal(b, &sess); err != nil {
		return session{}, fmt.Errorf("unmarshal session: %w", err)
	}
	if sess.ID == "" {
		sess.ID = id
	}
	return sess, nil
}

func (s *sessionStore) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove session: %w", err)
	}
	return nil
}

func (sess session) IsEmpty() bool {
	return len(sess.Messages) == 0 && len(sess.Turns) == 0
}

func (s *sessionStore) List() ([]session, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("ReadDir: %w", err)
	}
	var out []session
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		id := name[:len(name)-len(".json")]
		sess, err := s.Load(id)
		if err != nil {
			continue
		}
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func newSessionID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
