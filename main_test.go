package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveConfigAutodetectFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Chdir(tmp)

	mustRun(t, "git", "init", "-q")
	mustRun(t, "git", "remote", "add", "origin", "git@github.com:acme/demo.git")
	if err := os.MkdirAll(".github/workflows", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"release.yaml", "ci.yml"} {
		if err := os.WriteFile(filepath.Join(".github/workflows", f), []byte("name: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := resolveConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Watches) != 1 {
		t.Fatalf("want 1 watch, got %d", len(cfg.Watches))
	}
	w := cfg.Watches[0]
	if w.Repo != "acme/demo" {
		t.Errorf("repo: got %q, want %q", w.Repo, "acme/demo")
	}
	if want := []string{"ci.yml", "release.yaml"}; !reflect.DeepEqual(w.Workflows, want) {
		t.Errorf("workflows: got %v, want %v (sorted)", w.Workflows, want)
	}
}

// The temp dir is not a git checkout, so detection fails and the saved
// config is returned untouched.
func TestResolveConfigSavedOnlyWhenDetectFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Chdir(tmp)

	yaml := "watches:\n  - repo: saved/repo\n    workflows: [ci.yml]\n"
	if err := os.WriteFile("config.yaml", []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Watches) != 1 || cfg.Watches[0].Repo != "saved/repo" {
		t.Fatalf("saved config not returned as-is: %+v", cfg)
	}
}

func TestResolveConfigMergesDetectedRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Chdir(tmp)

	yaml := "watches:\n  - repo: saved/repo\n    workflows: [ci.yml]\n"
	if err := os.WriteFile("config.yaml", []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDetectableRepo(t, "git@github.com:acme/demo.git", "ci.yml")

	cfg, err := resolveConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Watches) != 2 {
		t.Fatalf("want 2 watches, got %d: %+v", len(cfg.Watches), cfg)
	}
	if cfg.Watches[0].Repo != "saved/repo" {
		t.Errorf("first watch: got %q, want %q", cfg.Watches[0].Repo, "saved/repo")
	}
	if cfg.Watches[1].Repo != "acme/demo" {
		t.Errorf("second watch: got %q, want %q", cfg.Watches[1].Repo, "acme/demo")
	}
	if want := []string{"ci.yml"}; !reflect.DeepEqual(cfg.Watches[1].Workflows, want) {
		t.Errorf("detected workflows: got %v, want %v", cfg.Watches[1].Workflows, want)
	}
}

func TestResolveConfigDedupesSavedRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Chdir(tmp)

	yaml := "watches:\n  - repo: acme/demo\n    workflows: [deploy.yml]\n"
	if err := os.WriteFile("config.yaml", []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDetectableRepo(t, "git@github.com:acme/demo.git", "ci.yml")

	cfg, err := resolveConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Watches) != 1 {
		t.Fatalf("want 1 watch, got %d: %+v", len(cfg.Watches), cfg)
	}
	w := cfg.Watches[0]
	if w.Repo != "acme/demo" {
		t.Errorf("repo: got %q, want %q", w.Repo, "acme/demo")
	}
	if want := []string{"deploy.yml"}; !reflect.DeepEqual(w.Workflows, want) {
		t.Errorf("workflows: got %v, want %v (saved entry untouched)", w.Workflows, want)
	}
}

func TestResolveConfigMergePreservesTopLevelFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Chdir(tmp)

	yaml := "mobile_idle_minutes: 7\nwatches:\n  - repo: saved/repo\n    workflows: [ci.yml]\n"
	if err := os.WriteFile("config.yaml", []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDetectableRepo(t, "git@github.com:acme/demo.git", "ci.yml")

	cfg, err := resolveConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Watches) != 2 {
		t.Fatalf("merge path not taken, got %d watches: %+v", len(cfg.Watches), cfg)
	}
	if cfg.MobileIdleMinutes != 7 {
		t.Errorf("MobileIdleMinutes: got %d, want 7", cfg.MobileIdleMinutes)
	}
}

func writeDetectableRepo(t *testing.T, origin string, workflows ...string) {
	t.Helper()
	mustRun(t, "git", "init", "-q")
	mustRun(t, "git", "remote", "add", "origin", origin)
	if err := os.MkdirAll(".github/workflows", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range workflows {
		if err := os.WriteFile(filepath.Join(".github/workflows", f), []byte("name: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResolveConfigBothFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Chdir(tmp)

	mustRun(t, "git", "init", "-q")

	_, err := resolveConfig()
	if err == nil {
		t.Fatal("expected error when no config and no origin remote")
	}
	if !strings.Contains(err.Error(), "auto-detect failed") {
		t.Errorf("error should mention auto-detect failure, got: %v", err)
	}
}

// A config file that exists but does not load is a hard error: falling back to
// auto-detection here would silently drop every saved watch.
func TestResolveConfigBrokenConfigIsError(t *testing.T) {
	tests := []struct {
		name, yaml string
	}{
		{"malformed yaml", "watches:\n  - repo: \"saved/repo\"\n   workflows: [\n"},
		{"no watches", "watches: []\n"},
		{"bad notify rule", "watches:\n  - repo: \"saved/repo\"\n    notify:\n      only: sometimes\n"},
		{"fractional idle minutes", "mobile_idle_minutes: 2.5\nwatches:\n  - repo: \"saved/repo\"\n"},
		{"out of range idle minutes", "mobile_idle_minutes: 999999999\nwatches:\n  - repo: \"saved/repo\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)
			t.Chdir(tmp)

			if err := os.WriteFile("config.yaml", []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			writeDetectableRepo(t, "git@github.com:acme/demo.git", "ci.yml")

			cfg, err := resolveConfig()
			if err == nil {
				t.Fatalf("resolveConfig() succeeded with %+v, want error", cfg)
			}
			if len(cfg.Watches) != 0 {
				t.Errorf("watches silently replaced by auto-detection: %+v", cfg.Watches)
			}
		})
	}
}

// An unrelated tool's ./config.yaml must not fail startup: the bare name is
// shared, so anything without a watches key is somebody else's file.
func TestResolveConfigIgnoresForeignLocalConfig(t *testing.T) {
	tests := []struct {
		name, yaml string
	}{
		{"foreign mapping", "server:\n  port: 8080\n"},
		{"bare list", "- a\n- b\n"},
		{"bare scalar", "42\n"},
		{"empty file", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)
			t.Chdir(tmp)

			if err := os.WriteFile("config.yaml", []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			writeDetectableRepo(t, "git@github.com:acme/demo.git", "ci.yml")

			cfg, err := resolveConfig()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cfg.Watches) != 1 || cfg.Watches[0].Repo != "acme/demo" {
				t.Fatalf("want auto-detected watch only, got %+v", cfg.Watches)
			}
		})
	}
}

func TestResolveConfigUnreadableConfigIsError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any file regardless of mode")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Chdir(tmp)

	if err := os.WriteFile("config.yaml", []byte("watches:\n  - repo: \"saved/repo\"\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	writeDetectableRepo(t, "git@github.com:acme/demo.git", "ci.yml")

	cfg, err := resolveConfig()
	if err == nil {
		t.Fatalf("resolveConfig() succeeded with %+v, want error", cfg)
	}
	if len(cfg.Watches) != 0 {
		t.Errorf("watches silently replaced by auto-detection: %+v", cfg.Watches)
	}
}

func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
