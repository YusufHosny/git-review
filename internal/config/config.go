package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Editor      string   `yaml:"editor"`
	DefaultBase string   `yaml:"default_base"`
	UI          UIConfig `yaml:"ui"`
}

type UIConfig struct {
	Theme            string  `yaml:"theme"`
	LineNumbers      string  `yaml:"line_numbers"`
	SplitWidthRatio  float64 `yaml:"split_width_ratio"`
	ShowCommentsInline bool  `yaml:"show_comments_inline"`
}

func Load() Config {
	cfg := Config{
		UI: UIConfig{
			Theme:              "dark",
			LineNumbers:        "absolute",
			SplitWidthRatio:    0.5,
			ShowCommentsInline: true,
		},
	}

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "git-review", "config.yaml")

	if data, err := os.ReadFile(configPath); err == nil {
		_ = yaml.Unmarshal(data, &cfg)
	}

	if cfg.Editor == "" {
		cfg.Editor = os.Getenv("GIT_REVIEW_EDITOR")
	}
	if cfg.Editor == "" {
		cfg.Editor = os.Getenv("EDITOR")
	}
	if cfg.Editor == "" {
		cfg.Editor = os.Getenv("VISUAL")
	}
	if cfg.Editor == "" {
		cfg.Editor = "vi"
	}

	if cfg.DefaultBase == "" {
		cfg.DefaultBase = os.Getenv("GIT_REVIEW_BASE")
	}

	return cfg
}

func SaveTheme(themeName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".config", "git-review")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	configPath := filepath.Join(dir, "config.yaml")

	cfg := Load()
	cfg.UI.Theme = themeName

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}
