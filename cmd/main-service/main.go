package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/FischukSergey/otus-ms/internal/config"
	"github.com/FischukSergey/otus-ms/internal/jwks"
	"github.com/FischukSergey/otus-ms/internal/logger"
	"github.com/FischukSergey/otus-ms/internal/metrics"
	"github.com/FischukSergey/otus-ms/internal/store"
)

var configPath = flag.String("config", "configs/config.local.yaml", "Path to config file")

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("run app: %v", err)
	}
}

func run() error {
	cfg, err := config.ParseAndValidate(*configPath)
	if err != nil {
		return err
	}

	// Проверяем наличие конфигурации БД (main-service требует БД)
	if !cfg.DB.IsConfigured() {
		return errors.New("database configuration is incomplete: please provide name, user, password, host and port")
	}

	// Создаем логгер на основе конфигурации
	// Будет использоваться tint (цветной вывод) для format=text
	// или JSON для format=json
	appLogger := logger.NewLogger(cfg.Log)

	// Инициализируем Prometheus метрики
	appLogger.Info("Initializing Prometheus metrics...")
	metrics.Init()
	appLogger.Info("Prometheus metrics initialized successfully")

	// Создаем контекст для отслеживания сигналов прерывания
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	// Инициализируем подключение к БД
	appLogger.Info("Initializing database connection",
		"host", cfg.DB.Host,
		"port", cfg.DB.Port,
		"database", cfg.DB.Name,
	)

	storage, err := store.NewStorage(ctx, store.NewOptions(
		cfg.DB.Name,                         // dbName
		cfg.DB.User,                         // dbUser
		cfg.DB.Password,                     // dbPassword
		cfg.DB.Host,                         // dbHost
		cfg.DB.Port,                         // dbPort
		store.WithDbSSLMode(cfg.DB.SSLMode), // optional SSL mode
	))
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}
	// Гарантируем закрытие соединений при любом выходе из функции
	defer func() {
		appLogger.Info("Closing database connections...")
		storage.Close()
		appLogger.Info("Database connections closed")
	}()

	// Устанавливаем logger для storage
	storage.SetLogger(appLogger)

	appLogger.Info("Database connection established successfully")

	// Запускаем миграции
	appLogger.Info("Running database migrations...")
	if err := storage.RunMigrations(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	appLogger.Info("Database migrations completed successfully")

	// Инициализируем JWKS Manager (если настроен)
	var jwksManager *jwks.Manager
	if cfg.JWT.IsConfigured() && cfg.JWT.JWKSURL != "" {
		appLogger.Info("Initializing JWKS Manager...")
		var err error
		jwksManager, err = jwks.NewManager(
			cfg.JWT.JWKSURL,
			cfg.JWT.GetCacheDuration(),
			appLogger,
		)
		if err != nil {
			return fmt.Errorf("init JWKS Manager: %w", err)
		}
		// Закрываем менеджер при остановке
		defer func() {
			appLogger.Info("Closing JWKS Manager...")
			if err := jwksManager.Close(); err != nil {
				appLogger.Error("Error closing JWKS Manager", "error", err)
			}
		}()
		appLogger.Info("JWKS Manager initialized successfully")
	} else {
		appLogger.Info("JWKS Manager not configured, JWT validation will be disabled")
	}

	// Создаем API сервер
	apiServer := NewAPIServer(&APIServerDeps{
		Addr:        cfg.Servers.Client.Addr,
		Config:      cfg,
		Logger:      appLogger,
		Storage:     storage,
		JWKSManager: jwksManager,
	})

	// Создаем и запускаем Debug сервер
	debugServer := NewDebugServer(&DebugServerDeps{
		Addr:   cfg.Servers.Debug.Addr,
		Logger: appLogger,
	})

	// Создаем и запускаем Metrics сервер
	metricsServer := NewMetricsServer(&MetricsServerDeps{
		Addr:   cfg.Servers.Metrics.Addr,
		Logger: appLogger,
	})

	// Запускаем API сервер в отдельной горутине
	go func() {
		appLogger.Info("Starting API server", "addr", cfg.Servers.Client.Addr)
		if err := apiServer.Start(); err != nil {
			appLogger.Error("API server error", "error", err)
		}
	}()

	// Запускаем Debug сервер в отдельной горутине
	go func() {
		appLogger.Info("Starting Debug server", "addr", cfg.Servers.Debug.Addr)
		if err := debugServer.Start(); err != nil {
			appLogger.Error("Debug server error", "error", err)
		}
	}()

	// Запускаем Metrics сервер в отдельной горутине
	go func() {
		appLogger.Info("Starting Metrics server", "addr", cfg.Servers.Metrics.Addr)
		if err := metricsServer.Start(); err != nil {
			appLogger.Error("Metrics server error", "error", err)
		}
	}()

	// Ждем сигнал завершения
	<-ctx.Done()
	appLogger.Info("Shutting down gracefully...")

	// Graceful shutdown с таймаутом
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Останавливаем API сервер
	if err := apiServer.Stop(shutdownCtx); err != nil {
		appLogger.Error("Error during API server shutdown", "error", err)
		return err
	}

	// Останавливаем Debug сервер
	if err := debugServer.Stop(shutdownCtx); err != nil {
		appLogger.Error("Error during Debug server shutdown", "error", err)
		return err
	}

	// Останавливаем Metrics сервер
	if err := metricsServer.Stop(shutdownCtx); err != nil {
		appLogger.Error("Error during Metrics server shutdown", "error", err)
		return err
	}

	appLogger.Info("Server stopped successfully")
	return nil
}
