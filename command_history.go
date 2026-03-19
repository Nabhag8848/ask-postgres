package main

import (
	"os"
	"path/filepath"
	"strings"
)

const maxHistoryLines = 500

func resolveBaseDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("PGWATCH_COPILOT_HOME")); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pgwatch-copilot"), nil
}

func historyFilePath() (string, error) {
	base, err := resolveBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "history"), nil
}

func loadCommandHistory() []string {
	path, err := historyFilePath()
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
	if len(lines) > maxHistoryLines {
		lines = lines[len(lines)-maxHistoryLines:]
	}
	return lines
}

func appendCommandHistory(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}
	path, err := historyFilePath()
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

func saveCommandHistory(lines []string) {
	path, err := historyFilePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if len(lines) > maxHistoryLines {
		lines = lines[len(lines)-maxHistoryLines:]
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
