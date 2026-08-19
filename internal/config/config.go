package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type WatchEntry struct {
	Repo      string      `yaml:"repo"`
	Workflows []string    `yaml:"workflows"`
	Notify    NotifyRules `yaml:"notify"`
}

type NotifyRules struct {
	Only  string   `yaml:"only"`
	Quiet []string `yaml:"quiet"`
}

func (r NotifyRules) FailuresOnly() bool {
	return r.Only == "failures"
}

func (r NotifyRules) InQuiet(t time.Time) bool {
	m := t.Hour()*60 + t.Minute()
	for _, w := range r.Quiet {
		start, end, err := parseQuietWindow(w)
		if err != nil {
			continue
		}
		if start < end {
			if m >= start && m < end {
				return true
			}
		} else {
			if m >= start || m < end {
				return true
			}
		}
	}
	return false
}

func (r NotifyRules) validate() error {
	switch r.Only {
	case "", "all", "failures":
	default:
		return fmt.Errorf("notify.only must be \"all\" or \"failures\", got %q", r.Only)
	}
	for _, w := range r.Quiet {
		if _, _, err := parseQuietWindow(w); err != nil {
			return err
		}
	}
	return nil
}

func parseQuietWindow(s string) (start, end int, err error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("quiet window %q: want HH:MM-HH:MM", s)
	}
	start, err = parseHHMM(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("quiet window %q: %w", s, err)
	}
	end, err = parseHHMM(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("quiet window %q: %w", s, err)
	}
	if start == end {
		return 0, 0, fmt.Errorf("quiet window %q: start and end must differ", s)
	}
	return start, end, nil
}

func parseHHMM(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("bad time %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("bad time %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("bad time %q", s)
	}
	return h*60 + m, nil
}

type Config struct {
	Watches           []WatchEntry `yaml:"watches"`
	MobileIdleMinutes int          `yaml:"mobile_idle_minutes"`
}

func Load() (Config, error) {
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
		for _, w := range cfg.Watches {
			if err := w.Notify.validate(); err != nil {
				return Config{}, fmt.Errorf("%s: watch %s: %w", p, w.Repo, err)
			}
		}
		if cfg.MobileIdleMinutes < 0 {
			return Config{}, fmt.Errorf("%s: mobile_idle_minutes must be >= 0, got %d", p, cfg.MobileIdleMinutes)
		}
		return cfg, nil
	}

	return Config{}, fmt.Errorf("config.yaml not found in current directory or ~/.config/gh-action-monitor/")
}
