package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveBaseDir returns the root directory for all ask-postgres data.
// Override with ASK_POSTGRES_HOME; defaults to ~/.ask-postgres.
func ResolveBaseDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("ASK_POSTGRES_HOME")); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ask-postgres"), nil
}
