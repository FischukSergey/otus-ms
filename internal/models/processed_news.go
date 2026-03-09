// Package models содержит доменные модели приложения.
package models

import "time"

// ProcessedNews представляет обработанную новость от news-processor.
// Публикуется в Kafka топик processed_news и сохраняется в PostgreSQL через main-service.
type ProcessedNews struct {
	ID          string    `json:"id"`          // UUID (берётся из RawNews.ID)
	SourceID    string    `json:"sourceId"`    // ID источника
	Title       string    `json:"title"`       // Заголовок (очищенный)
	Summary     string    `json:"summary"`     // Краткое резюме (2-3 предложения)
	URL         string    `json:"url"`         // Ссылка на оригинал
	Category    string    `json:"category"`    // Категория: tech, politics, economy, sports, science, other
	Tags        []string  `json:"tags"`        // Ключевые теги
	PublishedAt time.Time `json:"publishedAt"` // Дата публикации (из RawNews)
	ProcessedAt time.Time `json:"processedAt"` // Дата обработки
}
