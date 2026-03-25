package history

import (
	"os"
	"path/filepath"
	"strings"

	"ask-postgres/internal/config"
)

// MaxLines caps the number of history entries kept on disk.
const MaxLines = 500

func filePath() (string, error) {
	base, err := config.ResolveBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "history"), nil
}

// Load reads command history from disk.
func Load() []string {
	path, err := filePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	raw := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var lines []string
	for _, l := range raw {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > MaxLines {
		lines = lines[len(lines)-MaxLines:]
	}
	return lines
}

// Append adds a single entry to the history file.
func Append(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}
	path, err := filePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(entry + "\n")
}

// Save overwrites the history file with the given lines.
func Save(lines []string) {
	path, err := filePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if len(lines) > MaxLines {
		lines = lines[len(lines)-MaxLines:]
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
