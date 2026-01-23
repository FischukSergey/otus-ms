package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// ErrUserNotFound возвращается, когда пользователь не найден.
var ErrUserNotFound = errors.New("user not found")

// Repository предоставляет методы для работы с пользователями в БД.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository создает новый экземпляр репозитория.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

// Create создает нового пользователя в БД.
func (r *Repository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (uuid, email, first_name, last_name, middle_name, role, created_at, updated_at, deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.Deleted = false

	_, err := r.db.Exec(ctx, query,
		user.UUID,
		user.Email,
		user.FirstName,
		user.LastName,
		user.MiddleName,
		user.Role,
		user.CreatedAt,
		user.UpdatedAt,
		user.Deleted,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByUUID возвращает пользователя по UUID.
// Возвращает всех пользователей, включая помеченных как удаленные (deleted=true).
func (r *Repository) GetByUUID(ctx context.Context, uuid string) (*models.User, error) {
	query := `
		SELECT uuid, email, first_name, last_name, middle_name, role, 
		       created_at, last_login, updated_at, deleted, deleted_at
		FROM users
		WHERE uuid = $1
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, uuid).Scan(
		&user.UUID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.MiddleName,
		&user.Role,
		&user.CreatedAt,
		&user.LastLogin,
		&user.UpdatedAt,
		&user.Deleted,
		&user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by uuid: %w", err)
	}

	return &user, nil
}

// SoftDelete помечает пользователя как удаленного (мягкое удаление).
func (r *Repository) SoftDelete(ctx context.Context, uuid string) error {
	query := `
		UPDATE users
		SET deleted = true, deleted_at = $1, updated_at = $1
		WHERE uuid = $2 AND deleted = false
	`

	now := time.Now()
	result, err := r.db.Exec(ctx, query, now, uuid)
	if err != nil {
		return fmt.Errorf("failed to soft delete user: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}
