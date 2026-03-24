package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Global holds user-level preferences persisted across sessions.
type Global struct {
	Model           string `json:"model,omitempty"`
	Theme           string `json:"theme,omitempty"`
	OpenAIAPIKey    string `json:"openai_api_key,omitempty"`
	AnthropicAPIKey string `json:"anthropic_api_key,omitempty"`
	GoogleAPIKey    string `json:"google_api_key,omitempty"`
}

func filePath() (string, error) {
	base, err := ResolveBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.json"), nil
}

// Load reads the global config from disk. Returns zero-value on any error.
func Load() Global {
	path, err := filePath()
	if err != nil {
		return Global{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Global{}
	}
	var cfg Global
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

// Save persists the global config to disk, silently ignoring errors.
func Save(cfg Global) {
	path, err := filePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
