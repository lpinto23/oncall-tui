package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	OutputDir string `json:"output_dir"`
}

func defaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		OutputDir: filepath.Join(home, "oncall-incidents"),
	}
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "oncall-tui", "config.json"), nil
}

func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := defaultConfig()
		_ = Save(cfg)
		return cfg, nil
	}
	if err != nil {
		return defaultConfig(), err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
