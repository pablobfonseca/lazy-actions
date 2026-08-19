package config

import (
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
