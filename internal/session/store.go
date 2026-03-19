package session

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

	"pgwatch-copilot/internal/config"
)

// Store manages session persistence as JSON files on disk.
type Store struct {
	dir string
}

// NewStore creates a Store, ensuring the sessions directory exists.
func NewStore() (*Store, error) {
	dir, err := resolveSessionsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("MkdirAll(%s): %w", dir, err)
	}
	return &Store{dir: dir}, nil
}

func resolveSessionsDir() (string, error) {
	homeOverride := strings.TrimSpace(os.Getenv("PGWATCH_COPILOT_HOME"))
	if homeOverride != "" {
		return filepath.Join(homeOverride, "sessions"), nil
	}
	base, err := config.ResolveBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "sessions"), nil
}

// New creates and persists a fresh empty session.
func (s *Store) New() (Session, error) {
	id, err := NewID()
	if err != nil {
		return Session{}, err
	}
	now := time.Now()
	sess := Session{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.Save(sess); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// Save atomically writes a session to disk (tmp + rename).
func (s *Store) Save(sess Session) error {
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

// Load reads a session from disk by ID.
func (s *Store) Load(id string) (Session, error) {
	path := filepath.Join(s.dir, id+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return Session{}, fmt.Errorf("read session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(b, &sess); err != nil {
		return Session{}, fmt.Errorf("unmarshal session: %w", err)
	}
	if sess.ID == "" {
		sess.ID = id
	}
	return sess, nil
}

// Delete removes a session file. No error if it doesn't exist.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove session: %w", err)
	}
	return nil
}

// List returns all sessions ordered by most recently updated first.
func (s *Store) List() ([]Session, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("ReadDir: %w", err)
	}
	var out []Session
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

// Latest returns the most recently updated session, or an empty Session if none exist.
func (s *Store) Latest() (Session, bool) {
	list, err := s.List()
	if err != nil || len(list) == 0 {
		return Session{}, false
	}
	return list[0], true
}

// NewID generates a random 16-hex-character session identifier.
func NewID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
