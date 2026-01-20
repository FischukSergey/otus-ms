package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FischukSergey/otus-ms/internal/config"
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

	// Создаем и запускаем API сервер
	apiServer := NewAPIServer(APIServerDeps{
		Addr:   cfg.Servers.Client.Addr,
		Config: cfg,
		Logger: logger,
	})

	// Запускаем сервер в отдельной горутине
	go func() {
		logger.Info("Starting API server", "addr", cfg.Servers.Client.Addr)
		if err := apiServer.Start(); err != nil {
			logger.Error("API server error", "error", err)
		}
	}()

	// Ждем сигнал завершения
	<-ctx.Done()
	logger.Info("Shutting down gracefully...")

	// Graceful shutdown с таймаутом
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := apiServer.Stop(shutdownCtx); err != nil {
		logger.Error("Error during shutdown", "error", err)
		return err
	}

	logger.Info("Server stopped successfully")
	return nil
}
