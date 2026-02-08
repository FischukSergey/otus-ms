package config

// Config представляет конфигурацию приложения.
type Config struct {
	Global  GlobalConfig  `yaml:"global"`
	Log     LogConfig     `yaml:"log"`
	Servers ServersConfig `yaml:"servers"`
	DB      DBConfig      `yaml:"db"`
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
	// добавляем валидацию: обязательное поле, значение должно быть в формате "host:port".
	Addr string `yaml:"addr" validate:"required,hostname_port"`
}

// ClientServerConfig представляет настройки клиентского API сервера.
type ClientServerConfig struct {
	Addr         string   `yaml:"addr" validate:"required,hostname_port"`
	AllowOrigins []string `yaml:"allow_origins"`
}

// MetricsServerConfig представляет настройки сервера метрик.
type MetricsServerConfig struct {
	Addr string `yaml:"addr" validate:"required,hostname_port"`
}

// DBConfig представляет настройки базы данных.
type DBConfig struct {
	Name        string `yaml:"name" validate:"required"`
	User        string `yaml:"user" validate:"required"`
	Password    string `yaml:"password" validate:"required"`
	Host        string `yaml:"host" validate:"required"`
	Port        string `yaml:"port" validate:"required"`
	SSLMode     string `yaml:"ssl_mode"`
	SSLRootCert string `yaml:"ssl_root_cert"`
	SSLKey      string `yaml:"ssl_key"`
}
