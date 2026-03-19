package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveBaseDir returns the root directory for all pgwatch-copilot data.
// Override with PGWATCH_COPILOT_HOME; defaults to ~/.pgwatch-copilot.
func ResolveBaseDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("PGWATCH_COPILOT_HOME")); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pgwatch-copilot"), nil
}
