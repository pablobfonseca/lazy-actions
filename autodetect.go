package main

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNotAutoDetectable means the CWD isn't a git checkout with a GitHub
// origin and a `.github/workflows/` directory. Callers should fall back to
// a user-supplied config.
var ErrNotAutoDetectable = errors.New("cannot auto-detect watches from current directory")

// DetectFromCWD inspects the current working directory and builds a single
// WatchEntry from `git remote get-url origin` and `.github/workflows/*.{yml,yaml}`.
// Returns ErrNotAutoDetectable (wrapped) for any recoverable failure so main
// can distinguish "no detection possible" from hard errors.
func DetectFromCWD() (WatchEntry, error) {
	repo, err := detectRepoSlug()
	if err != nil {
		return WatchEntry{}, err
	}
	workflows, err := detectWorkflows()
	if err != nil {
		return WatchEntry{}, err
	}
	return WatchEntry{Repo: repo, Workflows: workflows}, nil
}

func detectRepoSlug() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("%w: git remote not available", ErrNotAutoDetectable)
	}
	url := strings.TrimSpace(string(out))
	return parseGitHubSlug(url)
}

// parseGitHubSlug extracts "owner/repo" from common GitHub URL shapes:
//   - git@github.com:owner/repo.git
//   - https://github.com/owner/repo.git
//   - ssh://git@github.com/owner/repo.git
//
// Returns ErrNotAutoDetectable for non-GitHub URLs. Exported indirectly via
// DetectFromCWD; kept package-private so the parsing lives next to its one
// caller.
func parseGitHubSlug(url string) (string, error) {
	rest := url
	switch {
	case strings.HasPrefix(rest, "git@github.com:"):
		rest = strings.TrimPrefix(rest, "git@github.com:")
	case strings.HasPrefix(rest, "ssh://git@github.com/"):
		rest = strings.TrimPrefix(rest, "ssh://git@github.com/")
	case strings.HasPrefix(rest, "https://github.com/"):
		rest = strings.TrimPrefix(rest, "https://github.com/")
	case strings.HasPrefix(rest, "http://github.com/"):
		rest = strings.TrimPrefix(rest, "http://github.com/")
	default:
		return "", fmt.Errorf("%w: origin %q is not a github.com URL", ErrNotAutoDetectable, url)
	}
	rest = strings.TrimSuffix(rest, ".git")
	rest = strings.TrimSuffix(rest, "/")
	if parts := strings.Split(rest, "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("%w: cannot parse owner/repo from %q", ErrNotAutoDetectable, url)
	}
	return rest, nil
}

func detectWorkflows() ([]string, error) {
	var files []string
	for _, pat := range []string{".github/workflows/*.yml", ".github/workflows/*.yaml"} {
		matches, _ := filepath.Glob(pat)
		files = append(files, matches...)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: no workflow files under .github/workflows/", ErrNotAutoDetectable)
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	sort.Strings(names)
	return names, nil
}
