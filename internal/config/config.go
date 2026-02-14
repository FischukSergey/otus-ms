package config

// Config представляет конфигурацию приложения.
type Config struct {
	Global   GlobalConfig   `yaml:"global"`
	Log      LogConfig      `yaml:"log"`
	Servers  ServersConfig  `yaml:"servers"`
	DB       DBConfig       `yaml:"db"`
	Keycloak KeycloakConfig `yaml:"keycloak"`
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
	ClientSecret string `yaml:"client_secret"`
}

// IsConfigured проверяет, что конфигурация Keycloak заполнена.
func (k KeycloakConfig) IsConfigured() bool {
	return k.URL != "" && k.Realm != "" && k.ClientID != "" && k.ClientSecret != ""
}
