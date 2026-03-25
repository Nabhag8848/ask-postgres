package custom

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"ask-postgres/internal/config"
)

// Command maps a short name to a prompt that gets sent to the agent.
type Command struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// Store persists custom commands as a JSON file on disk.
type Store struct {
	path string
	cmds map[string]Command
}

// NewStore loads (or initialises) the custom commands store.
func NewStore() (*Store, error) {
	base, err := config.ResolveBaseDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(base, "custom_commands.json")
	s := &Store{path: path, cmds: make(map[string]Command)}
	s.load()
	return s, nil
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var cmds []Command
	if err := json.Unmarshal(data, &cmds); err != nil {
		return
	}
	for _, c := range cmds {
		s.cmds[c.Name] = c
	}
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.List(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// List returns all custom commands sorted by name.
func (s *Store) List() []Command {
	out := make([]Command, 0, len(s.cmds))
	for _, c := range s.cmds {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get looks up a custom command by name.
func (s *Store) Get(name string) (Command, bool) {
	c, ok := s.cmds[name]
	return c, ok
}

// Exists reports whether a custom command with the given name already exists.
func (s *Store) Exists(name string) bool {
	_, ok := s.cmds[name]
	return ok
}

// Add creates a new custom command and persists to disk.
// Returns an error if a command with the same name already exists.
func (s *Store) Add(name, prompt string) error {
	if _, ok := s.cmds[name]; ok {
		return fmt.Errorf("custom command %q already exists — delete it first with /delete-custom %s", name, name)
	}
	s.cmds[name] = Command{Name: name, Prompt: prompt}
	return s.save()
}

// Update overwrites the prompt of an existing custom command.
func (s *Store) Update(name, prompt string) error {
	if _, ok := s.cmds[name]; !ok {
		return fmt.Errorf("custom command %q not found", name)
	}
	s.cmds[name] = Command{Name: name, Prompt: prompt}
	return s.save()
}

// Delete removes a custom command and persists to disk.
func (s *Store) Delete(name string) error {
	if _, ok := s.cmds[name]; !ok {
		return fmt.Errorf("custom command %q not found", name)
	}
	delete(s.cmds, name)
	return s.save()
}
