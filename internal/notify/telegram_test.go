package notify

import (
	"encoding/json"
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
