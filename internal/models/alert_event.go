package models

import "time"

// AlertEvent хранит событие доставки алерта.
type AlertEvent struct {
	ID             string     `json:"id" db:"id"`
	RuleID         string     `json:"ruleId" db:"rule_id"`
	NewsID         string     `json:"newsId" db:"news_id"`
	UserUUID       string     `json:"userUuid" db:"user_uuid"`
	Keyword        string     `json:"keyword" db:"keyword"`
	DeliveryStatus string     `json:"deliveryStatus" db:"delivery_status"`
	ErrorMessage   string     `json:"errorMessage,omitempty" db:"error_message"`
	SentAt         *time.Time `json:"sentAt,omitempty" db:"sent_at"`
	CreatedAt      time.Time  `json:"createdAt" db:"created_at"`
}
