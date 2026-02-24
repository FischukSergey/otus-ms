package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FischukSergey/otus-ms/internal/models"
	"github.com/FischukSergey/otus-ms/internal/services/user"
)

func TestValidatePersonName(t *testing.T) {
	v := validator.New()
	_ = v.RegisterValidation("personname", user.ValidatePersonName)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid latin name",
			input:   "John",
			wantErr: false,
		},
		{
			name:    "valid cyrillic name",
			input:   "Иван",
			wantErr: false,
		},
		{
			name:    "valid name with space",
			input:   "Mary Jane",
			wantErr: false,
		},
		{
			name:    "valid name with hyphen",
			input:   "Jean-Pierre",
			wantErr: false,
		},
		{
			name:    "valid name with apostrophe",
			input:   "O'Brien",
			wantErr: false,
		},
		{
			name:    "valid name with dot",
			input:   "St.John",
			wantErr: false,
		},
		{
			name:    "empty name is valid",
			input:   "",
			wantErr: false,
		},
		{
			name:    "invalid name with numbers",
			input:   "John123",
			wantErr: true,
		},
		{
			name:    "invalid name with special chars",
			input:   "John@Smith",
			wantErr: true,
		},
		{
			name:    "invalid name with emoji",
			input:   "John😀",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Name string `validate:"personname"`
			}

			ts := testStruct{Name: tt.input}
			err := v.Struct(ts)

			if tt.wantErr {
				assert.Error(t, err, "Expected validation error for input: %s", tt.input)
			} else {
				assert.NoError(t, err, "Expected no validation error for input: %s", tt.input)
			}
		})
	}
}

func TestCreateUserValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     user.CreateRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: user.CreateRequest{
				UUID:      "123e4567-e89b-12d3-a456-426614174000",
				Email:     "test@example.com",
				FirstName: "John",
				LastName:  "Doe",
			},
			wantErr: false,
		},
		{
			name: "empty UUID",
			req: user.CreateRequest{
				UUID:  "",
				Email: "test@example.com",
			},
			wantErr: true,
			errMsg:  "validation error",
		},
		{
			name: "invalid UUID format",
			req: user.CreateRequest{
				UUID:  "invalid-uuid",
				Email: "test@example.com",
			},
			wantErr: true,
			errMsg:  "validation error",
		},
		{
			name: "empty email",
			req: user.CreateRequest{
				UUID:  "123e4567-e89b-12d3-a456-426614174000",
				Email: "",
			},
			wantErr: true,
			errMsg:  "validation error",
		},
		{
			name: "invalid email",
			req: user.CreateRequest{
				UUID:  "123e4567-e89b-12d3-a456-426614174000",
				Email: "not-an-email",
			},
			wantErr: true,
			errMsg:  "validation error",
		},
		{
			name: "email too long",
			req: user.CreateRequest{
				UUID:  "123e4567-e89b-12d3-a456-426614174000",
				Email: string(make([]byte, 260)) + "@example.com",
			},
			wantErr: true,
			errMsg:  "validation error",
		},
		{
			name: "first name with invalid characters",
			req: user.CreateRequest{
				UUID:      "123e4567-e89b-12d3-a456-426614174000",
				Email:     "test@example.com",
				FirstName: "John123",
			},
			wantErr: true,
			errMsg:  "validation error",
		},
		{
			name: "empty names are allowed",
			req: user.CreateRequest{
				UUID:      "123e4567-e89b-12d3-a456-426614174000",
				Email:     "test@example.com",
				FirstName: "",
				LastName:  "",
			},
			wantErr: false,
		},
		{
			name: "valid cyrillic names",
			req: user.CreateRequest{
				UUID:      "123e4567-e89b-12d3-a456-426614174000",
				Email:     "test@example.com",
				FirstName: "Иван",
				LastName:  "Иванов",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем минимальную имплементацию репозитория для теста
			mockRepo := &mockRepository{}
			service := user.NewService(mockRepo)

			err := service.CreateUser(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetUserUUIDValidation(t *testing.T) {
	mockRepo := &mockRepository{}
	service := user.NewService(mockRepo)

	tests := []struct {
		name    string
		uuid    string
		wantErr error
	}{
		{
			name:    "valid UUID",
			uuid:    "123e4567-e89b-12d3-a456-426614174000",
			wantErr: nil,
		},
		{
			name:    "invalid UUID format",
			uuid:    "not-a-uuid",
			wantErr: user.ErrInvalidUUID,
		},
		{
			name:    "empty UUID",
			uuid:    "",
			wantErr: user.ErrInvalidUUID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.GetUser(context.Background(), tt.uuid)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeleteUserUUIDValidation(t *testing.T) {
	mockRepo := &mockRepository{}
	service := user.NewService(mockRepo)

	tests := []struct {
		name    string
		uuid    string
		wantErr error
	}{
		{
			name:    "valid UUID",
			uuid:    "123e4567-e89b-12d3-a456-426614174000",
			wantErr: nil,
		},
		{
			name:    "invalid UUID format",
			uuid:    "not-a-uuid",
			wantErr: user.ErrInvalidUUID,
		},
		{
			name:    "empty UUID",
			uuid:    "",
			wantErr: user.ErrInvalidUUID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.DeleteUser(context.Background(), tt.uuid)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetAllUsers(t *testing.T) {
	tests := []struct {
		name      string
		repoUsers []*models.User
		repoErr   error
		wantLen   int
		wantErr   bool
	}{
		{
			name: "returns all users",
			repoUsers: []*models.User{
				{UUID: "123e4567-e89b-12d3-a456-426614174000", Email: "a@example.com", Role: "admin"},
				{UUID: "223e4567-e89b-12d3-a456-426614174001", Email: "b@example.com", Role: "user1C"},
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:      "empty table returns empty slice",
			repoUsers: []*models.User{},
			wantLen:   0,
			wantErr:   false,
		},
		{
			name:    "repository error is propagated",
			repoErr: errors.New("db error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockRepository{
				getAllUsers: tt.repoUsers,
				getAllErr:   tt.repoErr,
			}
			service := user.NewService(mockRepo)

			result, err := service.GetAllUsers(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

// mockRepository - минимальная mock-реализация для тестов.
type mockRepository struct {
	getAllUsers []*models.User
	getAllErr   error
}

func (m *mockRepository) Create(_ context.Context, _ *models.User) error {
	return nil
}

func (m *mockRepository) GetByUUID(_ context.Context, _ string) (*models.User, error) {
	return &models.User{}, nil
}

func (m *mockRepository) GetAll(_ context.Context) ([]*models.User, error) {
	return m.getAllUsers, m.getAllErr
}

func (m *mockRepository) SoftDelete(_ context.Context, _ string) error {
	return nil
}
