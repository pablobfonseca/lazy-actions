package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const brrrEndpoint = "https://api.brrr.now/v1/"

// MobileNotifier delivers push notifications to the user's phone via the
// brrr.now webhook API. The token is read from the BRRR_TOKEN environment
// variable (typically populated from a .env file at startup).
type MobileNotifier struct {
	token  string
	client *http.Client
}

func NewMobileNotifier() *MobileNotifier {
	return &MobileNotifier{
		token:  strings.TrimSpace(os.Getenv("BRRR_TOKEN")),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Configured reports whether a brrr.now token is available to send with.
func (m *MobileNotifier) Configured() bool {
	return m.token != ""
}

// Send delivers a notification to the user's device. Best-effort: like the
// desktop notifier, delivery errors are intentionally ignored.
func (m *MobileNotifier) Send(n notification) {
	if m.token == "" {
		return
	}

	payload := map[string]string{
		"title":   n.title,
		"message": mobileMessage(n),
	}
	if n.openURL != "" {
		payload["open_url"] = n.openURL
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, brrrEndpoint+m.token, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// mobileMessage flattens the desktop notification's subtitle and message into a
// single line, since the mobile payload has no dedicated subtitle field.
func mobileMessage(n notification) string {
	if n.subtitle == "" {
		return n.message
	}
	return n.subtitle + " — " + n.message
}

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
