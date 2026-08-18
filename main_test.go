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

func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
