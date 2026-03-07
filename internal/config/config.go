// Package config содержит структуры конфигурации приложения и логику их парсинга.
package config

import "time"

// Config представляет конфигурацию приложения.
type Config struct {
	Global      GlobalConfig      `yaml:"global"`
	Log         LogConfig         `yaml:"log"`
	Servers     ServersConfig     `yaml:"servers"`
	DB          DBConfig          `yaml:"db"`
	Keycloak    KeycloakConfig    `yaml:"keycloak"`
	JWT         JWTConfig         `yaml:"jwt"`
	MainService MainServiceConfig `yaml:"main_service"`
	RateLimiter RateLimiterConfig `yaml:"rate_limiter"`
	Redis       RedisConfig       `yaml:"redis"`
	Collector   CollectorConfig   `yaml:"collector"`
}

// GlobalConfig представляет глобальные настройки.
type GlobalConfig struct {
	// добавляем валидацию: обязательное поле, значения из {"local", "dev", "stage", "prod"}.
	Env string `yaml:"env" validate:"required,oneof=local dev stage prod"`
}

// LogConfig представляет настройки логирования.
type LogConfig struct {
	// добавляем валидацию: обязательное поле, значения из {"debug", "info", "warn", "error"}.
	Level string `yaml:"level" validate:"required,oneof=debug info warn error"`
	// Формат вывода логов: json для продакшена, text для локальной разработки.
	Format string `yaml:"format" validate:"required,oneof=json text"`
	// Имя сервиса для идентификации в логах.
	ServiceName string `yaml:"service_name" validate:"required"`
}

// ServersConfig представляет настройки серверов.
type ServersConfig struct {
	Debug   DebugServerConfig   `yaml:"debug"`
	Client  ClientServerConfig  `yaml:"client"`
	Metrics MetricsServerConfig `yaml:"metrics"`
	GRPC    GRPCServerConfig    `yaml:"grpc"`
}

// GRPCServerConfig представляет настройки gRPC сервера.
type GRPCServerConfig struct {
	// Адрес gRPC сервера — опциональный, включается только для сервисов с gRPC.
	Addr string `yaml:"addr" validate:"omitempty,hostname_port"`
}

// IsConfigured проверяет, что gRPC сервер настроен.
func (g GRPCServerConfig) IsConfigured() bool {
	return g.Addr != ""
}

// DebugServerConfig представляет настройки отладочного сервера.
type DebugServerConfig struct {
	// Опциональное поле - используется только в сервисах с debug endpoints.
	Addr string `yaml:"addr" validate:"omitempty,hostname_port"`
}

// ClientServerConfig представляет настройки клиентского API сервера.
type ClientServerConfig struct {
	Addr         string   `yaml:"addr" validate:"required,hostname_port"`
	AllowOrigins []string `yaml:"allow_origins"`
}

// MetricsServerConfig представляет настройки сервера метрик.
type MetricsServerConfig struct {
	// Опциональное поле - используется только в сервисах с метриками.
	Addr string `yaml:"addr" validate:"omitempty,hostname_port"`
}

// DBConfig представляет настройки базы данных.
// Опциональная конфигурация - используется только в сервисах с БД.
type DBConfig struct {
	Name        string `yaml:"name"`
	User        string `yaml:"user"`
	Password    string `yaml:"password"`
	Host        string `yaml:"host"`
	Port        string `yaml:"port"`
	SSLMode     string `yaml:"ssl_mode"`
	SSLRootCert string `yaml:"ssl_root_cert"`
	SSLKey      string `yaml:"ssl_key"`
}

// IsConfigured проверяет, что конфигурация БД заполнена.
func (d DBConfig) IsConfigured() bool {
	return d.Name != "" && d.User != "" && d.Password != "" && d.Host != "" && d.Port != ""
}

// KeycloakConfig представляет настройки Keycloak для авторизации.
// Опциональная конфигурация - используется только в Auth-Proxy сервисе.
type KeycloakConfig struct {
	URL          string `yaml:"url" validate:"omitempty,url"`
	Realm        string `yaml:"realm"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret" env:"KEYCLOAK_CLIENT_SECRET"`
}

// IsConfigured проверяет, что конфигурация Keycloak заполнена.
func (k KeycloakConfig) IsConfigured() bool {
	return k.URL != "" && k.Realm != "" && k.ClientID != "" && k.ClientSecret != ""
}

// Issuer возвращает issuer URL для JWT токенов (realm URL).
// Формат: {keycloak_url}/realms/{realm_name}.
func (k KeycloakConfig) Issuer() string {
	return k.URL + "/realms/" + k.Realm
}

// JWTConfig содержит настройки для JWT валидации.
// Используется в main-service для проверки JWT токенов от Keycloak.
type JWTConfig struct {
	// JWKS URL для автоматической загрузки публичных ключей
	JWKSURL       string `yaml:"jwks_url"`       // URL к JWKS endpoint Keycloak
	Issuer        string `yaml:"issuer"`         // URL Keycloak realm
	Audience      string `yaml:"audience"`       // Client ID
	CacheDuration int    `yaml:"cache_duration"` // Время кеширования JWKS в секундах (по умолчанию 600 = 10 минут)
	SkipVerify    bool   `yaml:"skip_verify"`    // Пропустить проверку подписи JWT (только для тестов!)
}

// IsConfigured проверяет, что конфигурация JWT заполнена.
func (j JWTConfig) IsConfigured() bool {
	// В тестовом режиме (skip_verify) JWT всегда считается настроенным
	// Токены принимаются без проверки подписи, issuer опционален
	if j.SkipVerify {
		return true
	}
	// В production режиме требуется JWKS URL и issuer
	return j.JWKSURL != "" && j.Issuer != ""
}

// GetCacheDuration возвращает время кеширования JWKS или значение по умолчанию.
func (j JWTConfig) GetCacheDuration() int {
	if j.CacheDuration > 0 {
		return j.CacheDuration
	}
	return 600 // 10 минут по умолчанию
}

// MainServiceConfig представляет настройки Main Service API.
// Используется в Auth-Proxy для создания пользователей при регистрации.
type MainServiceConfig struct {
	URL      string `yaml:"url"      env:"MAIN_SERVICE_URL"      env-default:"http://localhost:38080"`
	GRPCAddr string `yaml:"grpc_addr" env:"MAIN_SERVICE_GRPC_ADDR" env-default:"localhost:50051"`
}

// IsConfigured проверяет, что конфигурация Main Service заполнена.
func (m MainServiceConfig) IsConfigured() bool {
	return m.URL != ""
}

// RateLimiterConfig содержит настройки Rate Limiter для auth-proxy.
// Если RedisAddr не задан — лимитер не активируется.
type RateLimiterConfig struct {
	RedisAddr     string `yaml:"redis_addr"`
	RedisPassword string `yaml:"redis_password"`
	MaxAttempts   int    `yaml:"max_attempts"   env-default:"10"`
	WindowSeconds int    `yaml:"window_seconds" env-default:"60"`
}

// IsConfigured проверяет, задан ли адрес Redis.
func (r RateLimiterConfig) IsConfigured() bool {
	return r.RedisAddr != ""
}

// RedisConfig содержит настройки подключения к Redis.
// Используется в news-collector для хранения операционного состояния сбора.
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// IsConfigured проверяет, задан ли адрес Redis.
func (r RedisConfig) IsConfigured() bool {
	return r.Addr != ""
}

// CollectorConfig содержит настройки сервиса сбора новостей (news-collector).
type CollectorConfig struct {
	MaxWorkers      int           `yaml:"max_workers"`
	MaxRetries      int           `yaml:"max_retries"`
	MaxErrCount     int           `yaml:"max_error_count"`
	ParseTimeout    time.Duration `yaml:"parse_timeout"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	DedupTTL        time.Duration `yaml:"dedup_ttl"`
	// DeactivationBaseBackoff — начальный период circuit-breaker деактивации источника.
	// Удваивается с каждой деактивацией. По умолчанию 15 минут.
	DeactivationBaseBackoff time.Duration `yaml:"deactivation_base_backoff"`
	// DeactivationMaxBackoff — максимальный период деактивации. По умолчанию 24 часа.
	DeactivationMaxBackoff time.Duration `yaml:"deactivation_max_backoff"`
}

// GetDedupTTL возвращает TTL дедупликации или 7 дней если поле не задано.
func (c CollectorConfig) GetDedupTTL() time.Duration {
	if c.DedupTTL > 0 {
		return c.DedupTTL
	}
	return 7 * 24 * time.Hour
}

// GetDeactivationBaseBackoff возвращает начальный backoff или 15 минут по умолчанию.
func (c CollectorConfig) GetDeactivationBaseBackoff() time.Duration {
	if c.DeactivationBaseBackoff > 0 {
		return c.DeactivationBaseBackoff
	}
	return 15 * time.Minute
}

// GetDeactivationMaxBackoff возвращает максимальный backoff или 24 часа по умолчанию.
func (c CollectorConfig) GetDeactivationMaxBackoff() time.Duration {
	if c.DeactivationMaxBackoff > 0 {
		return c.DeactivationMaxBackoff
	}
	return 24 * time.Hour
}
