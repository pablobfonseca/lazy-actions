package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// loadDotEnv reads KEY=VALUE pairs from .env files and sets them in the process
// environment without overriding variables that are already set. It is silent
// if no .env file exists.
func loadDotEnv() {
	paths := []string{
		".env",
		filepath.Join(os.Getenv("HOME"), ".config", "gh-action-monitor", ".env"),
	}

	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			val = strings.Trim(strings.TrimSpace(val), `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, val)
			}
		}
		f.Close()
	}
}
