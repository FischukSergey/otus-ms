package user

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/FischukSergey/otus-ms/internal/models"
	userRepo "github.com/FischukSergey/otus-ms/internal/store/user"
)

// Ошибки валидации.
var (
	ErrInvalidUUID = errors.New("invalid UUID format")
)

// personNameRegex определяет допустимые символы для имен: буквы (латиница, кириллица),
// пробелы, дефисы, апострофы, точки.
var personNameRegex = regexp.MustCompile(`^[\p{L}\s\-'.]+$`)

// Repository определяет интерфейс для работы с репозиторием пользователей.
type Repository interface {
	Create(ctx context.Context, user *models.User) error
	GetByUUID(ctx context.Context, uuid string) (*models.User, error)
	SoftDelete(ctx context.Context, uuid string) error
}

// Service предоставляет бизнес-логику для работы с пользователями.
type Service struct {
	repo      Repository
	validator *validator.Validate
}

// NewService создает новый экземпляр сервиса пользователей.
func NewService(repo Repository) *Service {
	v := validator.New()

	// Регистрируем кастомный валидатор для имен
	_ = v.RegisterValidation("personname", ValidatePersonName)

	return &Service{
		repo:      repo,
		validator: v,
	}
}

// ValidatePersonName проверяет, что строка содержит только допустимые символы для имени.
func ValidatePersonName(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true // пустые имена разрешены
	}
	return personNameRegex.MatchString(value)
}

// CreateUser создает нового пользователя.
// Выполняет валидацию входных данных и вызывает репозиторий для сохранения.
func (s *Service) CreateUser(ctx context.Context, req CreateRequest) error {
	// Валидация запроса
	if err := s.validator.Struct(req); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Создаем модель пользователя
	user := &models.User{
		UUID:       req.UUID,
		Email:      req.Email,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		MiddleName: req.MiddleName,
		Role:       "user1C", // Роль по умолчанию из схемы БД
	}

	// Сохраняем пользователя
	if err := s.repo.Create(ctx, user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUser возвращает пользователя по UUID.
// Возвращает пользователей независимо от статуса deleted.
func (s *Service) GetUser(ctx context.Context, uuidStr string) (*Response, error) {
	// Валидируем формат UUID
	if _, err := uuid.Parse(uuidStr); err != nil {
		return nil, ErrInvalidUUID
	}

	user, err := s.repo.GetByUUID(ctx, uuidStr)
	if err != nil {
		if err == userRepo.ErrUserNotFound {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Преобразуем модель в DTO
	response := &Response{
		UUID:       user.UUID,
		Email:      user.Email,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		MiddleName: user.MiddleName,
		Role:       user.Role,
		CreatedAt:  user.CreatedAt,
		Deleted:    user.Deleted,
		DeletedAt:  user.DeletedAt,
	}

	return response, nil
}

// DeleteUser выполняет мягкое удаление пользователя.
func (s *Service) DeleteUser(ctx context.Context, uuidStr string) error {
	// Валидируем формат UUID
	if _, err := uuid.Parse(uuidStr); err != nil {
		return ErrInvalidUUID
	}

	if err := s.repo.SoftDelete(ctx, uuidStr); err != nil {
		if err == userRepo.ErrUserNotFound {
			return err
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
