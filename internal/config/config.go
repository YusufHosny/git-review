package config

import (
	"fmt"
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
	Theme              string  `yaml:"theme"`
	LineNumbers        string  `yaml:"line_numbers"`
	SplitWidthRatio    float64 `yaml:"split_width_ratio"`
	ShowCommentsInline bool    `yaml:"show_comments_inline"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "git-review"), nil
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

	if dir, err := configDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(dir, "config.yaml")); err == nil {
			_ = yaml.Unmarshal(data, &cfg)
		}
	}

	for _, env := range []string{"GIT_REVIEW_EDITOR", "EDITOR", "VISUAL"} {
		if cfg.Editor == "" {
			cfg.Editor = os.Getenv(env)
		}
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
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	cfg := Load()
	cfg.UI.Theme = themeName

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
