package keycloak

// TokenResponse представляет ответ от Keycloak с токенами.
// OAuth2/OIDC стандарт требует snake_case для JSON полей.
//
//nolint:tagliatelle // OAuth2 RFC 6749 требует snake_case
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
}

// LoginRequest представляет запрос на логин пользователя.
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest представляет запрос на обновление токена.
//
//nolint:tagliatelle // OAuth2 RFC 6749 требует snake_case
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest представляет запрос на logout пользователя.
//
//nolint:tagliatelle // OAuth2 RFC 6749 требует snake_case
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ErrorResponse представляет ошибку от Keycloak.
//
//nolint:tagliatelle // OAuth2 RFC 6749 требует snake_case
type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// RegisterRequest представляет запрос на регистрацию нового пользователя.
type RegisterRequest struct {
	Email      string `json:"email" validate:"required,email,max=255"`
	Password   string `json:"password" validate:"required,min=8,max=128"`
	FirstName  string `json:"firstName" validate:"required,max=255"`
	LastName   string `json:"lastName" validate:"required,max=255"`
	MiddleName string `json:"middleName,omitempty" validate:"omitempty,max=255"`
}

// User внутренняя модель для создания пользователя в Keycloak.
type User struct {
	Email      string
	Password   string
	FirstName  string
	LastName   string
	MiddleName string
}
