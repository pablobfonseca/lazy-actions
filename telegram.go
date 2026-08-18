package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
	"time"
)

// TelegramNotifier delivers run updates to a Telegram chat via the Bot API.
// Credentials are read from the TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID
// environment variables (typically populated from a .env file at startup).
type TelegramNotifier struct {
	token  string
	chatID string
	client *http.Client
}

func NewTelegramNotifier() *TelegramNotifier {
	return &TelegramNotifier{
		token:  strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		chatID: strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Configured reports whether both a bot token and chat id are available.
func (t *TelegramNotifier) Configured() bool {
	return t.token != "" && t.chatID != ""
}

// Send delivers a notification to the configured chat. Best-effort: like the
// desktop notifier, delivery errors are intentionally ignored.
func (t *TelegramNotifier) Send(n notification) {
	if !t.Configured() {
		return
	}

	text := "<b>" + html.EscapeString(n.title) + "</b>\n" + html.EscapeString(mobileMessage(n))
	if n.openURL != "" {
		text += fmt.Sprintf("\n<a href=%q>View run</a>", n.openURL)
	}

	payload := map[string]string{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	resp, err := t.client.Post(
		"https://api.telegram.org/bot"+t.token+"/sendMessage",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return
	}
	resp.Body.Close()
}
