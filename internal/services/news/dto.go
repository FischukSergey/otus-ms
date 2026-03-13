// Package news содержит DTO и бизнес-логику чтения новостей.
package news

import "time"

// Response представляет запись новости для HTTP ответа.
type Response struct {
	Topic     string    `json:"topic"`
	Source    string    `json:"source"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}
