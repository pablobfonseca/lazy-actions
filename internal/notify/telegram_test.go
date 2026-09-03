package notify

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type tgDecodedButton struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type tgDecodedPayload struct {
	ChatID      string `json:"chat_id"`
	Text        string `json:"text"`
	ParseMode   string `json:"parse_mode"`
	ReplyMarkup *struct {
		InlineKeyboard [][]tgDecodedButton `json:"inline_keyboard"`
	} `json:"reply_markup"`
}

func decodePayload(t *testing.T, data []byte) tgDecodedPayload {
	t.Helper()
	var p tgDecodedPayload
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTelegramPayloadButtonsFromActions(t *testing.T) {
	n := notification{
		title:   "✗ ci.yml",
		message: "Failed: build",
		openURL: "https://example.com/job/1",
		actions: []notificationAction{
			{"View Run", "https://example.com/run/1"},
			{"View Failed Job", "https://example.com/job/1"},
		},
	}
	data, err := telegramPayload("42", n)
	if err != nil {
		t.Fatal(err)
	}
	p := decodePayload(t, data)
	if p.ChatID != "42" || p.ParseMode != "HTML" {
		t.Errorf("chat_id=%q parse_mode=%q", p.ChatID, p.ParseMode)
	}
	if p.ReplyMarkup == nil {
		t.Fatal("no reply_markup")
	}
	kb := p.ReplyMarkup.InlineKeyboard
	if len(kb) != 2 || len(kb[0]) != 1 || len(kb[1]) != 1 {
		t.Fatalf("keyboard shape = %v", kb)
	}
	if kb[0][0] != (tgDecodedButton{"View Run", "https://example.com/run/1"}) {
		t.Errorf("row 0 = %v", kb[0][0])
	}
	if kb[1][0] != (tgDecodedButton{"View Failed Job", "https://example.com/job/1"}) {
		t.Errorf("row 1 = %v", kb[1][0])
	}
}

func TestTelegramPayloadFallbackButton(t *testing.T) {
	n := notification{title: "▶ ci.yml", message: "Started", openURL: "https://example.com/run/2"}
	data, err := telegramPayload("42", n)
	if err != nil {
		t.Fatal(err)
	}
	p := decodePayload(t, data)
	if p.ReplyMarkup == nil {
		t.Fatal("no reply_markup")
	}
	kb := p.ReplyMarkup.InlineKeyboard
	if len(kb) != 1 || len(kb[0]) != 1 || kb[0][0] != (tgDecodedButton{"View Run", "https://example.com/run/2"}) {
		t.Errorf("keyboard = %v", kb)
	}
}

func TestTelegramPayloadNoURLs(t *testing.T) {
	n := notification{title: "t", message: "m"}
	data, err := telegramPayload("42", n)
	if err != nil {
		t.Fatal(err)
	}
	if decodePayload(t, data).ReplyMarkup != nil {
		t.Error("reply_markup present, want absent")
	}
}

func TestTelegramPayloadTextHasNoLink(t *testing.T) {
	n := notification{title: "✓ <ci>", message: "Passed", openURL: "https://example.com/run/3"}
	data, err := telegramPayload("42", n)
	if err != nil {
		t.Fatal(err)
	}
	p := decodePayload(t, data)
	if want := "<b>✓ &lt;ci&gt;</b>\nPassed"; p.Text != want {
		t.Errorf("text = %q, want %q", p.Text, want)
	}
}

func TestTelegramPayloadSkipsInvalidButtons(t *testing.T) {
	tests := []struct {
		name    string
		actions []notificationAction
		want    []tgDecodedButton
	}{
		{
			name:    "no actions",
			actions: nil,
			want:    nil,
		},
		{
			name: "all urls present",
			actions: []notificationAction{
				{"View Run", "https://example.com/run/1"},
				{"View Failed Job", "https://example.com/job/1"},
			},
			want: []tgDecodedButton{
				{"View Run", "https://example.com/run/1"},
				{"View Failed Job", "https://example.com/job/1"},
			},
		},
		{
			name: "one empty url dropped",
			actions: []notificationAction{
				{"View Run", "https://example.com/run/1"},
				{"View Failed Job", "   "},
				{"View Artifacts", "https://example.com/artifacts/1"},
			},
			want: []tgDecodedButton{
				{"View Run", "https://example.com/run/1"},
				{"View Artifacts", "https://example.com/artifacts/1"},
			},
		},
		{
			name: "empty label dropped",
			actions: []notificationAction{
				{"", "https://example.com/run/1"},
				{"View Failed Job", "https://example.com/job/1"},
			},
			want: []tgDecodedButton{
				{"View Failed Job", "https://example.com/job/1"},
			},
		},
		{
			name: "every url empty",
			actions: []notificationAction{
				{"View Run", ""},
				{"View Failed Job", " "},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := telegramPayload("42", notification{title: "t", message: "m", actions: tt.actions})
			if err != nil {
				t.Fatal(err)
			}

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			_, hasMarkup := raw["reply_markup"]
			if hasMarkup != (len(tt.want) > 0) {
				t.Fatalf("reply_markup present = %v, want %v (payload %s)", hasMarkup, len(tt.want) > 0, data)
			}
			if !hasMarkup {
				return
			}

			kb := decodePayload(t, data).ReplyMarkup.InlineKeyboard
			if len(kb) != len(tt.want) {
				t.Fatalf("keyboard = %v, want %d rows", kb, len(tt.want))
			}
			for i, want := range tt.want {
				if len(kb[i]) != 1 || kb[i][0] != want {
					t.Errorf("row %d = %v, want [%v]", i, kb[i], want)
				}
			}
		})
	}
}

func TestTelegramLiveSend(t *testing.T) {
	if os.Getenv("TELEGRAM_LIVE_TEST") == "" {
		t.Skip("set TELEGRAM_LIVE_TEST=1 to send a real message")
	}
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	chatID := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))
	if token == "" || chatID == "" {
		t.Skip("TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID not set")
	}
	n := notification{
		title:   "✗ q5 inline-button test",
		message: "Failed: verify buttons render",
		openURL: "https://github.com/pablobfonseca/lazy-actions/actions",
		actions: []notificationAction{
			{"View Run", "https://github.com/pablobfonseca/lazy-actions/actions"},
			{"View Failed Job", "https://github.com/pablobfonseca/lazy-actions"},
		},
	}
	body, err := telegramPayload(chatID, n)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(out), `"ok":true`) {
		t.Fatalf("telegram rejected payload: %s", out)
	}
}
