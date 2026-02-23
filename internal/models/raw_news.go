package models

import "time"

// RawNews представляет сырую новость из RSS/Atom фида.
// Это сообщение отправляется в Kafka топик raw_news.
type RawNews struct {
	ID          string    `json:"id"`           // UUID
	SourceID    string    `json:"source_id"`    // ID источника
	Title       string    `json:"title"`        // Заголовок
	Description string    `json:"description"`  // Краткое описание
	Content     string    `json:"content"`      // Полный текст (если есть)
	URL         string    `json:"url"`          // Ссылка на оригинал
	PublishedAt time.Time `json:"published_at"` // Дата публикации
	CollectedAt time.Time `json:"collected_at"` // Дата сбора
	Author      string    `json:"author"`       // Автор (опционально)
	ImageURL    string    `json:"image_url"`    // URL изображения (опционально)
	RawData     string    `json:"raw_data"`     // Сырые данные для отладки
}
