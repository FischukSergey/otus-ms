package models

import "time"

// NewsBrief представляет краткую карточку новости для UI.
type NewsBrief struct {
	Topic     string    `db:"topic"`
	Source    string    `db:"source"`
	URL       string    `db:"url"`
	CreatedAt time.Time `db:"created_at"`
}
