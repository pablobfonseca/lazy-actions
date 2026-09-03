package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, yaml string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("HOME", dir)
}

func TestLoadNotifyRules(t *testing.T) {
	writeConfig(t, `watches:
  - repo: "o/r"
    workflows: ["ci.yml"]
    notify:
      only: failures
      quiet: ["22:00-08:00"]
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.Watches[0].Notify
	if !r.FailuresOnly() {
		t.Errorf("FailuresOnly() = false, want true")
	}
	if len(r.Quiet) != 1 || r.Quiet[0] != "22:00-08:00" {
		t.Errorf("Quiet = %v", r.Quiet)
	}
}

func TestLoadNotifyRulesInvalid(t *testing.T) {
	tests := []struct {
		name, yaml string
	}{
		{"bad only", `watches:
  - repo: "o/r"
    workflows: ["ci.yml"]
    notify:
      only: sometimes
`},
		{"bad quiet format", `watches:
  - repo: "o/r"
    workflows: ["ci.yml"]
    notify:
      quiet: ["10pm-8am"]
`},
		{"empty window", `watches:
  - repo: "o/r"
    workflows: ["ci.yml"]
    notify:
      quiet: ["08:00-08:00"]
`},
		{"out of range", `watches:
  - repo: "o/r"
    workflows: ["ci.yml"]
    notify:
      quiet: ["25:00-26:00"]
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeConfig(t, tt.yaml)
			if _, err := Load(); err == nil {
				t.Error("Load() succeeded, want error")
			}
		})
	}
}

func TestFailuresOnly(t *testing.T) {
	tests := []struct {
		only string
		want bool
	}{
		{"", false},
		{"all", false},
		{"failures", true},
	}
	for _, tt := range tests {
		if got := (NotifyRules{Only: tt.only}).FailuresOnly(); got != tt.want {
			t.Errorf("FailuresOnly(%q) = %v, want %v", tt.only, got, tt.want)
		}
	}
}

func at(hhmm string) time.Time {
	tm, err := time.Parse("15:04", hhmm)
	if err != nil {
		panic(err)
	}
	return time.Date(2026, 8, 19, tm.Hour(), tm.Minute(), 0, 0, time.Local)
}

func TestInQuiet(t *testing.T) {
	tests := []struct {
		name  string
		quiet []string
		at    string
		want  bool
	}{
		{"no windows", nil, "12:00", false},
		{"inside plain window", []string{"13:00-14:00"}, "13:30", true},
		{"start inclusive", []string{"13:00-14:00"}, "13:00", true},
		{"end exclusive", []string{"13:00-14:00"}, "14:00", false},
		{"outside plain window", []string{"13:00-14:00"}, "12:59", false},
		{"wrap evening side", []string{"22:00-08:00"}, "23:30", true},
		{"wrap morning side", []string{"22:00-08:00"}, "07:59", true},
		{"wrap outside", []string{"22:00-08:00"}, "12:00", false},
		{"wrap end exclusive", []string{"22:00-08:00"}, "08:00", false},
		{"second window matches", []string{"13:00-14:00", "22:00-08:00"}, "23:00", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NotifyRules{Quiet: tt.quiet}
			if got := r.InQuiet(at(tt.at)); got != tt.want {
				t.Errorf("InQuiet(%s) = %v, want %v", tt.at, got, tt.want)
			}
		})
	}
}

func TestLoadMobileIdleMinutes(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{"valid", "mobile_idle_minutes: 5\n", 5, false},
		{"omitted", "", 0, false},
		{"zero", "mobile_idle_minutes: 0\n", 0, false},
		{"upper bound", "mobile_idle_minutes: 1440\n", 1440, false},
		{"negative", "mobile_idle_minutes: -3\n", 0, true},
		{"fractional", "mobile_idle_minutes: 2.5\n", 0, true},
		{"over bound", "mobile_idle_minutes: 1441\n", 0, true},
		{"duration overflow", "mobile_idle_minutes: 999999999\n", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeConfig(t, tt.value+`watches:
  - repo: "o/r"
    workflows: ["ci.yml"]
`)
			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() = %d, want error", cfg.MobileIdleMinutes)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.MobileIdleMinutes != tt.want {
				t.Errorf("MobileIdleMinutes = %d, want %d", cfg.MobileIdleMinutes, tt.want)
			}
		})
	}
}

func writeXDGConfig(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	xdg := filepath.Join(dir, ".config", "gh-action-monitor")
	if err := os.MkdirAll(xdg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	return dir
}

func TestLoadUnreadableConfig(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any file regardless of mode")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := os.WriteFile("config.yaml", []byte("watches:\n  - repo: \"o/r\"\n"), 0o000); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded on an unreadable config, want error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("unreadable config reported as ErrNotFound: %v", err)
	}
}

func TestLoadConfigIsDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := os.Mkdir("config.yaml", 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded with config.yaml as a directory, want error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("directory reported as ErrNotFound: %v", err)
	}
}

// ./config.yaml is not namespaced, so an unrelated tool's file with no watches
// key must be skipped rather than fail startup.
func TestLoadSharedPathWithoutWatchesFallsThrough(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)
	if err := os.WriteFile("config.yaml", []byte("server:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestLoadSharedPathWithoutWatchesUsesXDGConfig(t *testing.T) {
	writeXDGConfig(t, `watches:
  - repo: "o/r"
    workflows: ["ci.yml"]
`)
	if err := os.WriteFile("config.yaml", []byte("server:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Watches) != 1 || cfg.Watches[0].Repo != "o/r" {
		t.Fatalf("XDG config not used: %+v", cfg)
	}
}

// Nothing else owns the XDG filename, so it stays strict even without watches.
func nonMappingDocs() []struct{ name, yaml string } {
	return []struct{ name, yaml string }{
		{"bare list", "- a\n- b\n"},
		{"bare string", "just a string\n"},
		{"bare scalar", "42\n"},
		{"empty file", ""},
		{"null document", "null\n"},
	}
}

func TestLoadSharedPathNonMappingFallsThrough(t *testing.T) {
	for _, tt := range nonMappingDocs() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("HOME", dir)
			if err := os.WriteFile("config.yaml", []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := Load(); !errors.Is(err, ErrNotFound) {
				t.Fatalf("Load() error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestLoadXDGNonMappingIsError(t *testing.T) {
	for _, tt := range nonMappingDocs() {
		t.Run(tt.name, func(t *testing.T) {
			writeXDGConfig(t, tt.yaml)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() succeeded on a non-mapping XDG config, want error")
			}
			if errors.Is(err, ErrNotFound) {
				t.Errorf("non-mapping XDG config reported as ErrNotFound: %v", err)
			}
		})
	}
}

func TestLoadXDGWithoutWatchesIsError(t *testing.T) {
	writeXDGConfig(t, "server:\n  port: 8080\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded on an XDG config with no watches, want error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("XDG config with no watches reported as ErrNotFound: %v", err)
	}
}

func TestLoadMissingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", dir)

	_, err := Load()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestLoadBrokenConfigIsNotNotFound(t *testing.T) {
	tests := []struct {
		name, yaml string
	}{
		{"malformed yaml", "watches:\n  - repo: \"o/r\"\n   workflows: [\n"},
		{"no watches", "watches: []\n"},
		{"bad idle minutes", "mobile_idle_minutes: 2.5\nwatches:\n  - repo: \"o/r\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeConfig(t, tt.yaml)
			_, err := Load()
			if err == nil {
				t.Fatal("Load() succeeded, want error")
			}
			if errors.Is(err, ErrNotFound) {
				t.Errorf("broken config reported as ErrNotFound: %v", err)
			}
		})
	}
}
