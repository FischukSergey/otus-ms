// Package user содержит бизнес-логику и DTO для работы с пользователями.
package user

import "time"

// CreateRequest представляет запрос на создание пользователя.
type CreateRequest struct {
	UUID       string  `json:"uuid" validate:"required,uuid"`
	Email      string  `json:"email" validate:"required,email,max=255"`
	FirstName  string  `json:"firstName" validate:"max=255,omitempty,personname"`
	LastName   string  `json:"lastName" validate:"max=255,omitempty,personname"`
	MiddleName *string `json:"middleName" validate:"omitempty,max=255,personname"`
}

// Response представляет ответ с данными пользователя.
type Response struct {
	UUID       string     `json:"uuid"`
	Email      string     `json:"email"`
	FirstName  string     `json:"firstName"`
	LastName   string     `json:"lastName"`
	MiddleName *string    `json:"middleName,omitempty"`
	Role       string     `json:"role"`
	CreatedAt  time.Time  `json:"createdAt"`
	Deleted    bool       `json:"deleted"`
	DeletedAt  *time.Time `json:"deletedAt,omitempty"`
}
