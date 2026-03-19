package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type globalConfig struct {
	Model string `json:"model,omitempty"`
	Theme string `json:"theme,omitempty"`
}

func configFilePath() (string, error) {
	base, err := resolveBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.json"), nil
}

func loadGlobalConfig() globalConfig {
	path, err := configFilePath()
	if err != nil {
		return globalConfig{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return globalConfig{}
	}
	var cfg globalConfig
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func saveGlobalConfig(cfg globalConfig) {
	path, err := configFilePath()
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
