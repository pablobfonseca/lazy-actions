package config

import (
	"errors"
	"fmt"
	"io/fs"
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

// ErrNotFound reports the absence of a config file, as opposed to one that
// exists but is unreadable: callers fall back to auto-detection only for this.
var ErrNotFound = errors.New("config.yaml not found in current directory or ~/.config/gh-action-monitor/")

const maxMobileIdleMinutes = 24 * 60

func Load() (Config, error) {
	paths := []struct {
		path string
		// The bare ./config.yaml name is not ours alone, so a document there
		// that is not a mapping with a watches key belongs to another tool
		// and is skipped rather than failing startup.
		shared bool
	}{
		{"config.yaml", true},
		{filepath.Join(os.Getenv("HOME"), ".config", "gh-action-monitor", "config.yaml"), false},
	}

	for _, src := range paths {
		data, err := os.ReadFile(src.path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return Config{}, fmt.Errorf("reading %s: %w", src.path, err)
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", src.path, err)
		}
		root := documentRoot(&doc)
		if root == nil || root.Kind != yaml.MappingNode {
			if src.shared {
				continue
			}
			return Config{}, fmt.Errorf("parsing %s: want a mapping with a watches key", src.path)
		}
		var raw rawConfig
		if err := root.Decode(&raw); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", src.path, err)
		}
		if src.shared && raw.Watches.IsZero() {
			continue
		}
		var cfg Config
		if err := root.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", src.path, err)
		}
		if len(cfg.Watches) == 0 {
			return Config{}, fmt.Errorf("no watches defined in %s", src.path)
		}
		for _, w := range cfg.Watches {
			if err := w.Notify.validate(); err != nil {
				return Config{}, fmt.Errorf("%s: watch %s: %w", src.path, w.Repo, err)
			}
		}
		if raw.MobileIdleMinutes.Tag == "!!float" {
			return Config{}, fmt.Errorf("%s: mobile_idle_minutes must be written as an integer, got %s", src.path, raw.MobileIdleMinutes.Value)
		}
		if cfg.MobileIdleMinutes < 0 || cfg.MobileIdleMinutes > maxMobileIdleMinutes {
			return Config{}, fmt.Errorf("%s: mobile_idle_minutes must be between 0 and %d (24h), got %d", src.path, maxMobileIdleMinutes, cfg.MobileIdleMinutes)
		}
		return cfg, nil
	}

	return Config{}, ErrNotFound
}

// documentRoot returns the single document's top-level node, or nil for an
// empty file.
func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		return doc.Content[0]
	}
	return nil
}

// rawConfig keeps the undecoded scalars: yaml.v3 truncates a float into an int
// field without complaining, and an absent watches key is indistinguishable
// from an empty one after decoding into Config.
type rawConfig struct {
	Watches           yaml.Node `yaml:"watches"`
	MobileIdleMinutes yaml.Node `yaml:"mobile_idle_minutes"`
}
