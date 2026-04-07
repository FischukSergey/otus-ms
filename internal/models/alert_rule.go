package models

import "time"

// AlertRule хранит правило алертинга пользователя.
type AlertRule struct {
	ID              string    `json:"id" db:"id"`
	UserUUID        string    `json:"userUuid" db:"user_uuid"`
	Keyword         string    `json:"keyword" db:"keyword"`
	IsActive        bool      `json:"isActive" db:"is_active"`
	ChannelType     string    `json:"channelType" db:"channel_type"`
	ChannelTarget   string    `json:"channelTarget,omitempty" db:"channel_target"`
	CooldownSeconds int       `json:"cooldownSeconds" db:"cooldown_seconds"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" db:"updated_at"`
}
