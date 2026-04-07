package alertworker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/FischukSergey/otus-ms/internal/config"
	"github.com/FischukSergey/otus-ms/internal/models"
)

// TelegramSender отправляет алерты в Telegram Bot API.
type TelegramSender struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegramSender создает sender для Telegram Bot API.
func NewTelegramSender(cfg config.TelegramConfig) *TelegramSender {
	return &TelegramSender{
		botToken: cfg.BotToken,
		chatID:   cfg.ProjectChatID,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send отправляет сообщение в Telegram.
func (s *TelegramSender) Send(ctx context.Context, event models.NewsAlertEvent) error {
	payload := map[string]any{
		"chat_id":                  s.chatID,
		"text":                     buildTelegramMessage(event),
		"disable_web_page_preview": true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("telegram send failed with status %d", resp.StatusCode)
	}

	return nil
}

func buildTelegramMessage(event models.NewsAlertEvent) string {
	snippet := strings.TrimSpace(event.MatchedSnippet)
	if snippet == "" {
		snippet = "совпадение найдено в новости"
	}

	return fmt.Sprintf(
		"Alert: keyword \"%s\"\nTitle: %s\nURL: %s\nMatch: %s",
		event.Keyword,
		event.NewsTitle,
		event.NewsURL,
		snippet,
	)
}
