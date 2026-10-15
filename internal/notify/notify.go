// Package notify sends state changes to webhooks and Slack.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Event is a confirmed state change of one check.
type Event struct {
	Name    string    `json:"name"`
	URL     string    `json:"url"`
	From    string    `json:"from"`
	To      string    `json:"to"`
	Reasons []string  `json:"reasons"`
	At      time.Time `json:"at"`
}

// Notifier delivers events.
type Notifier interface {
	Name() string
	Notify(ctx context.Context, e Event) error
}

var client = &http.Client{Timeout: 10 * time.Second}

func post(ctx context.Context, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-checker")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// Webhook posts the event as JSON.
type Webhook struct{ URL string }

// Name identifies the notifier in logs.
func (w Webhook) Name() string { return "webhook" }

// Notify sends the event.
func (w Webhook) Notify(ctx context.Context, e Event) error { return post(ctx, w.URL, e) }

// Slack posts a message to an incoming webhook.
type Slack struct{ URL string }

// Name identifies the notifier in logs.
func (s Slack) Name() string { return "slack" }

// Notify sends the event as a Slack message.
func (s Slack) Notify(ctx context.Context, e Event) error {
	return post(ctx, s.URL, map[string]string{"text": SlackText(e)})
}

// SlackText formats an event for Slack.
func SlackText(e Event) string {
	icon := map[string]string{"UP": ":large_green_circle:", "WARN": ":large_yellow_circle:", "DOWN": ":red_circle:"}[e.To]
	text := fmt.Sprintf("%s *%s* is %s (was %s) · %s", icon, e.Name, e.To, e.From, e.URL)
	if len(e.Reasons) > 0 {
		text += "\n> " + strings.Join(e.Reasons, "\n> ")
	}
	return text
}

// Discord posts to a Discord channel webhook.
type Discord struct{ URL string }

// Name identifies the notifier in logs.
func (d Discord) Name() string { return "discord" }

// Notify sends the event as a Discord message (Markdown).
func (d Discord) Notify(ctx context.Context, e Event) error {
	return post(ctx, d.URL, map[string]string{"content": PlainText(e, "**")})
}

// Teams posts a MessageCard to a Microsoft Teams incoming webhook.
type Teams struct{ URL string }

// Name identifies the notifier in logs.
func (t Teams) Name() string { return "teams" }

// Notify sends the event as a Teams card.
func (t Teams) Notify(ctx context.Context, e Event) error {
	color := map[string]string{"UP": "2EB67D", "WARN": "ECB22E", "DOWN": "E01E5A"}[e.To]
	return post(ctx, t.URL, map[string]any{
		"@type":      "MessageCard",
		"@context":   "https://schema.org/extensions",
		"summary":    fmt.Sprintf("%s is %s", e.Name, e.To),
		"themeColor": color,
		"title":      fmt.Sprintf("%s is %s (was %s)", e.Name, e.To, e.From),
		"text":       strings.Join(append([]string{e.URL}, e.Reasons...), "<br>"),
	})
}

// TelegramAPI is the Bot API base URL; tests replace it.
var TelegramAPI = "https://api.telegram.org"

// Telegram sends a message through a bot.
type Telegram struct{ Token, ChatID string }

// ParseTelegram reads "TOKEN:CHAT_ID". The bot token itself contains a colon,
// so the chat id is what follows the last one.
func ParseTelegram(s string) (Telegram, error) {
	i := strings.LastIndex(s, ":")
	if i <= 0 || i == len(s)-1 || !strings.Contains(s[:i], ":") {
		return Telegram{}, fmt.Errorf("--telegram wants BOT_TOKEN:CHAT_ID (e.g. 123456:ABC-DEF:-1001234567890)")
	}
	return Telegram{Token: s[:i], ChatID: s[i+1:]}, nil
}

// Name identifies the notifier in logs.
func (t Telegram) Name() string { return "telegram" }

// Notify sends the event as a Telegram message.
func (t Telegram) Notify(ctx context.Context, e Event) error {
	return post(ctx, TelegramAPI+"/bot"+t.Token+"/sendMessage", map[string]string{"chat_id": t.ChatID, "text": PlainText(e, "")})
}

// PlainText formats an event with an emoji; bold wraps the check name.
func PlainText(e Event, bold string) string {
	icon := map[string]string{"UP": "🟢", "WARN": "🟡", "DOWN": "🔴"}[e.To]
	text := fmt.Sprintf("%s %s%s%s is %s (was %s) · %s", icon, bold, e.Name, bold, e.To, e.From, e.URL)
	for _, r := range e.Reasons {
		text += "\n→ " + r
	}
	return text
}
