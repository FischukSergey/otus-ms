package processor

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FischukSergey/otus-ms/internal/models"
)

type cachedAlertRules struct {
	mu    sync.RWMutex
	items []models.AlertRule
}

func (c *cachedAlertRules) set(items []models.AlertRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = items
}

func (c *cachedAlertRules) get() []models.AlertRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.items
}

func matchAlertEvents(processed *models.ProcessedNews, rules []models.AlertRule) []models.NewsAlertEvent {
	if len(rules) == 0 {
		return nil
	}

	titleLower := strings.ToLower(processed.Title)
	summaryLower := strings.ToLower(processed.Summary)
	contentLower := strings.ToLower(processed.Content)

	events := make([]models.NewsAlertEvent, 0, len(rules))
	for i := range rules {
		keyword := strings.TrimSpace(strings.ToLower(rules[i].Keyword))
		if keyword == "" {
			continue
		}

		matchedField := ""
		matchedSnippet := ""
		switch {
		case strings.Contains(titleLower, keyword):
			matchedField = "title"
			matchedSnippet = buildSnippet(processed.Title, keyword)
		case strings.Contains(summaryLower, keyword):
			matchedField = "summary"
			matchedSnippet = buildSnippet(processed.Summary, keyword)
		case strings.Contains(contentLower, keyword):
			matchedField = "content"
			matchedSnippet = buildSnippet(processed.Content, keyword)
		default:
			continue
		}

		events = append(events, models.NewsAlertEvent{
			EventID:        uuid.NewString(),
			RuleID:         rules[i].ID,
			UserUUID:       rules[i].UserUUID,
			NewsID:         processed.ID,
			Keyword:        keyword,
			MatchedField:   matchedField,
			MatchedSnippet: matchedSnippet,
			NewsTitle:      processed.Title,
			NewsURL:        processed.URL,
			CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		})
	}

	return events
}

func buildSnippet(text, keyword string) string {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return ""
	}

	lower := strings.ToLower(raw)
	idx := strings.Index(lower, strings.ToLower(keyword))
	if idx < 0 {
		return truncate(raw, 180)
	}

	start := max(idx-60, 0)
	end := min(idx+len(keyword)+60, len(raw))

	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(raw) {
		suffix = "..."
	}
	return prefix + raw[start:end] + suffix
}
