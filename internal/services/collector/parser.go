// Package collector реализует бизнес-логику сбора новостей из RSS/Atom источников.
package collector

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// Parser парсит RSS/Atom фиды и преобразует их в список RawNews.
type Parser struct {
	fp     *gofeed.Parser
	logger *slog.Logger
}

// NewParser создаёт Parser с заданным таймаутом HTTP-запросов.
func NewParser(timeout time.Duration, logger *slog.Logger) *Parser {
	fp := gofeed.NewParser()
	fp.Client = &http.Client{Timeout: timeout}

	return &Parser{
		fp:     fp,
		logger: logger,
	}
}

// ParseFeed загружает и парсит RSS/Atom фид по URL.
// Возвращает список RawNews с заполненными SourceID и CollectedAt.
func (p *Parser) ParseFeed(sourceID, url string) ([]*models.RawNews, error) {
	feed, err := p.fp.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", url, err)
	}

	collectedAt := time.Now()
	news := make([]*models.RawNews, 0, len(feed.Items))

	for _, item := range feed.Items {
		news = append(news, &models.RawNews{
			ID:          uuid.New().String(),
			SourceID:    sourceID,
			Title:       item.Title,
			Description: item.Description,
			Content:     extractContent(item),
			URL:         item.Link,
			PublishedAt: parsePublishedTime(item),
			CollectedAt: collectedAt,
			Author:      extractAuthor(item),
			ImageURL:    extractImageURL(item),
		})
	}

	return news, nil
}

// ParseFeedWithRetry парсит фид с повторными попытками при ошибке.
// Использует exponential backoff: 1s, 2s, 4s, ...
func (p *Parser) ParseFeedWithRetry(sourceID, url string, maxRetries int) ([]*models.RawNews, error) {
	var lastErr error

	for attempt := range maxRetries {
		news, err := p.ParseFeed(sourceID, url)
		if err == nil {
			return news, nil
		}

		lastErr = err
		backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second

		p.logger.Warn("retrying feed parse",
			"source_id", sourceID,
			"url", url,
			"attempt", attempt+1,
			"max_retries", maxRetries,
			"backoff", backoff,
			"error", err,
		)

		time.Sleep(backoff)
	}

	return nil, fmt.Errorf("all %d attempts failed for %s: %w", maxRetries, url, lastErr)
}

// extractContent возвращает полный текст статьи, если доступен, иначе — описание.
func extractContent(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
}

// parsePublishedTime возвращает время публикации или текущее время если поле отсутствует.
func parsePublishedTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}
	return time.Now()
}

// extractAuthor возвращает имя автора если доступно.
func extractAuthor(item *gofeed.Item) string {
	if item.Author != nil {
		return item.Author.Name
	}
	return ""
}

// extractImageURL возвращает URL изображения из поля Image или первого вложения-картинки.
func extractImageURL(item *gofeed.Item) string {
	if item.Image != nil {
		return item.Image.URL
	}
	for _, enc := range item.Enclosures {
		if enc.Type == "image/jpeg" || enc.Type == "image/png" || enc.Type == "image/webp" {
			return enc.URL
		}
	}
	return ""
}
