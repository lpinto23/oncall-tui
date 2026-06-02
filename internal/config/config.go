package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	ModeEnriched = "enriched"
	ModeRaw      = "raw"
)

type Config struct {
	OutputDir   string `json:"output_dir"`
	DefaultMode string `json:"default_mode"`
}

func defaultConfig() Config {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	return Config{
		OutputDir:   filepath.Join(home, "oncall-incidents"),
		DefaultMode: ModeEnriched,
	}
}

func NormalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModeRaw:
		return ModeRaw
	case ModeEnriched:
		return ModeEnriched
	default:
		return ModeEnriched
	}
}

func IsValidMode(mode string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(mode))
	return trimmed == ModeRaw || trimmed == ModeEnriched
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "oncall-tui", "config.json"), nil
}

func Load() (Config, bool, error) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), true, nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), true, nil
	}
	if err != nil {
		return defaultConfig(), false, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), false, err
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = defaultConfig().OutputDir
	}
	cfg.DefaultMode = NormalizeMode(cfg.DefaultMode)
	return cfg, false, nil
}

func Save(cfg Config) error {
	cfg.DefaultMode = NormalizeMode(cfg.DefaultMode)
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = defaultConfig().OutputDir
	}

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
