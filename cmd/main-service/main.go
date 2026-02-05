package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FischukSergey/otus-ms/internal/config"
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

	// Создаем логгер
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Создаем контекст для отслеживания сигналов прерывания
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	// Инициализируем подключение к БД
	logger.Info("Initializing database connection",
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
	defer storage.Close()

	// Устанавливаем logger для storage
	storage.SetLogger(logger)

	logger.Info("Database connection established successfully")

	// Запускаем миграции
	logger.Info("Running database migrations...")
	if err := storage.RunMigrations(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	logger.Info("Database migrations completed successfully")

	// Создаем и запускаем API сервер
	apiServer := NewAPIServer(&APIServerDeps{
		Addr:    cfg.Servers.Client.Addr,
		Config:  cfg,
		Logger:  logger,
		Storage: storage,
	})

	// Создаем и запускаем Debug сервер
	debugServer := NewDebugServer(&DebugServerDeps{
		Addr:   cfg.Servers.Debug.Addr,
		Logger: logger,
	})

	// Запускаем API сервер в отдельной горутине
	go func() {
		logger.Info("Starting API server", "addr", cfg.Servers.Client.Addr)
		if err := apiServer.Start(); err != nil {
			logger.Error("API server error", "error", err)
		}
	}()

	// Запускаем Debug сервер в отдельной горутине
	go func() {
		logger.Info("Starting Debug server", "addr", cfg.Servers.Debug.Addr)
		if err := debugServer.Start(); err != nil {
			logger.Error("Debug server error", "error", err)
		}
	}()

	// Ждем сигнал завершения
	<-ctx.Done()
	logger.Info("Shutting down gracefully...")

	// Graceful shutdown с таймаутом
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Останавливаем API сервер
	if err := apiServer.Stop(shutdownCtx); err != nil {
		logger.Error("Error during API server shutdown", "error", err)
		return err
	}

	// Останавливаем Debug сервер
	if err := debugServer.Stop(shutdownCtx); err != nil {
		logger.Error("Error during Debug server shutdown", "error", err)
		return err
	}

	logger.Info("Server stopped successfully")
	return nil
}
