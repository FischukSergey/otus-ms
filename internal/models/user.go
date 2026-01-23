package models

import "time"

// User представляет модель пользователя в системе.
type User struct {
	UUID       string     `db:"uuid"`
	Email      string     `db:"email"`
	FirstName  string     `db:"first_name"`
	LastName   string     `db:"last_name"`
	MiddleName *string    `db:"middle_name"`
	Role       string     `db:"role"`
	CreatedAt  time.Time  `db:"created_at"`
	LastLogin  *time.Time `db:"last_login"`
	UpdatedAt  time.Time  `db:"updated_at"`
	Deleted    bool       `db:"deleted"`
	DeletedAt  *time.Time `db:"deleted_at"`
}
