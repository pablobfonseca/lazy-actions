package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type WatchEntry struct {
	Repo      string   `yaml:"repo"`
	Workflows []string `yaml:"workflows"`
}

type Config struct {
	Watches []WatchEntry `yaml:"watches"`
}

func loadConfig() (Config, error) {
	paths := []string{
		"config.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "gh-action-monitor", "config.yaml"),
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", p, err)
		}
		if len(cfg.Watches) == 0 {
			return Config{}, fmt.Errorf("no watches defined in %s", p)
		}
		return cfg, nil
	}

	return Config{}, fmt.Errorf("config.yaml not found in current directory or ~/.config/gh-action-monitor/")
}
