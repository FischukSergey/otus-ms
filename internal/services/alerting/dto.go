// Package alerting содержит DTO и бизнес-логику alerting MVP.
package alerting

import "time"

// CreateRuleRequest представляет запрос на создание правила.
type CreateRuleRequest struct {
	Keyword         string `json:"keyword"`
	ChannelType     string `json:"channelType"`
	ChannelTarget   string `json:"channelTarget"`
	CooldownSeconds int    `json:"cooldownSeconds"`
}

// UpdateRuleRequest представляет запрос на обновление правила.
type UpdateRuleRequest struct {
	Keyword         string `json:"keyword"`
	ChannelType     string `json:"channelType"`
	ChannelTarget   string `json:"channelTarget"`
	CooldownSeconds int    `json:"cooldownSeconds"`
	IsActive        bool   `json:"isActive"`
}

// RuleResponse представляет правило алертинга.
type RuleResponse struct {
	ID              string    `json:"id"`
	UserUUID        string    `json:"userUuid"`
	Keyword         string    `json:"keyword"`
	IsActive        bool      `json:"isActive"`
	ChannelType     string    `json:"channelType"`
	ChannelTarget   string    `json:"channelTarget,omitempty"`
	CooldownSeconds int       `json:"cooldownSeconds"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// EventResponse представляет событие доставки алерта.
type EventResponse struct {
	ID             string     `json:"id"`
	RuleID         string     `json:"ruleId"`
	NewsID         string     `json:"newsId"`
	UserUUID       string     `json:"userUuid"`
	Keyword        string     `json:"keyword"`
	DeliveryStatus string     `json:"deliveryStatus"`
	ErrorMessage   string     `json:"errorMessage,omitempty"`
	SentAt         *time.Time `json:"sentAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// ListEventsRequest задает фильтры списка событий.
type ListEventsRequest struct {
	Limit  int
	Offset int
	Status string
}
