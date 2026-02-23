// Package models содержит доменные модели приложения.
package models

import "time"

// RawNews представляет сырую новость из RSS/Atom фида.
// Это сообщение отправляется в Kafka топик raw_news.
type RawNews struct {
	ID          string    `json:"id"`          // UUID
	SourceID    string    `json:"sourceId"`    // ID источника
	Title       string    `json:"title"`       // Заголовок
	Description string    `json:"description"` // Краткое описание
	Content     string    `json:"content"`     // Полный текст (если есть)
	URL         string    `json:"url"`         // Ссылка на оригинал
	PublishedAt time.Time `json:"publishedAt"` // Дата публикации
	CollectedAt time.Time `json:"collectedAt"` // Дата сбора
	Author      string    `json:"author"`      // Автор (опционально)
	ImageURL    string    `json:"imageUrl"`    // URL изображения (опционально)
	RawData     string    `json:"rawData"`     // Сырые данные для отладки
}
