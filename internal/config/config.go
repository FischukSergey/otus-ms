package config

// Config представляет конфигурацию приложения.
type Config struct {
	Global  GlobalConfig  `yaml:"global"`
	Log     LogConfig     `yaml:"log"`
	Servers ServersConfig `yaml:"servers"`
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
}

// ServersConfig представляет настройки серверов.
type ServersConfig struct {
	Debug  DebugServerConfig  `yaml:"debug"`
	Client ClientServerConfig `yaml:"client"`
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
