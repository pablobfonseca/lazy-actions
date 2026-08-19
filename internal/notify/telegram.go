package notify

import (
	"bytes"
	"encoding/json"
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

	body, err := telegramPayload(t.chatID, n)
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

type telegramButton struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

func telegramPayload(chatID string, n notification) ([]byte, error) {
	payload := map[string]any{
		"chat_id":    chatID,
		"text":       "<b>" + html.EscapeString(n.title) + "</b>\n" + html.EscapeString(mobileMessage(n)),
		"parse_mode": "HTML",
	}

	buttons := n.actions
	if len(buttons) == 0 && n.openURL != "" {
		buttons = []notificationAction{{"View Run", n.openURL}}
	}
	if len(buttons) > 0 {
		rows := make([][]telegramButton, 0, len(buttons))
		for _, a := range buttons {
			rows = append(rows, []telegramButton{{a.label, a.url}})
		}
		payload["reply_markup"] = map[string]any{"inline_keyboard": rows}
	}

	return json.Marshal(payload)
}
