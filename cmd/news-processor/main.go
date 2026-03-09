// Package main является точкой входа сервиса обработки новостей.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	mainserviceclient "github.com/FischukSergey/otus-ms/internal/clients/mainservice"
	"github.com/FischukSergey/otus-ms/internal/config"
	"github.com/FischukSergey/otus-ms/internal/keycloak"
	"github.com/FischukSergey/otus-ms/internal/logger"
	"github.com/FischukSergey/otus-ms/internal/services/processor"
)

var configPath = flag.String("config", "configs/config.news-processor.local.yaml", "Path to config file")

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("run news-processor: %v", err)
	}
}

func run() error {
	cfg, err := config.ParseAndValidate(*configPath)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	appLogger := logger.NewLogger(cfg.Log)
	appLogger.Info("news-processor starting",
		"env", cfg.Global.Env,
		"main_service_grpc", cfg.MainService.GRPCAddr,
		"kafka_brokers", cfg.Kafka.Brokers,
	)

	if !cfg.Kafka.IsConfigured() {
		return fmt.Errorf("kafka is not configured: brokers and topic_raw_news are required")
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	// Инициализируем Keycloak клиент для получения service account JWT
	keycloakClient := keycloak.NewClient(
		cfg.Keycloak.URL,
		cfg.Keycloak.Realm,
		cfg.Keycloak.ClientID,
		cfg.Keycloak.ClientSecret,
	)

	// Инициализируем gRPC клиент main-service
	grpcClient, err := mainserviceclient.NewGRPCClient(
		cfg.MainService.GRPCAddr,
		keycloakClient,
		appLogger,
	)
	if err != nil {
		return fmt.Errorf("init grpc client: %w", err)
	}
	defer func() {
		if err := grpcClient.Close(); err != nil {
			appLogger.Error("grpc client close error", "error", err)
		}
	}()

	// Инициализируем сервис обработки новостей
	processorService := processor.NewService(
		cfg.Kafka,
		cfg.Processor,
		grpcClient,
		appLogger,
	)

	// Запускаем конвейер обработки в отдельной горутине
	processorErr := make(chan error, 1)
	go func() {
		processorErr <- processorService.Run(ctx)
	}()

	// Запускаем HTTP сервер (health check)
	apiServer := NewAPIServer(&APIServerDeps{
		Addr:   cfg.Servers.Client.Addr,
		Logger: appLogger,
	})

	go func() {
		appLogger.Info("starting API server", "addr", cfg.Servers.Client.Addr)
		if err := apiServer.Start(); err != nil {
			appLogger.Error("API server error", "error", err)
		}
	}()

	// Ждём сигнал завершения или ошибку процессора
	select {
	case <-ctx.Done():
		appLogger.Info("shutdown signal received")
	case err := <-processorErr:
		if err != nil {
			appLogger.Error("processor exited with error", "error", err)
			cancel()
		}
	}

	appLogger.Info("shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := apiServer.Stop(shutdownCtx); err != nil {
		appLogger.Error("API server shutdown error", "error", err)
	}

	// Ждём завершения конвейера обработки
	select {
	case err := <-processorErr:
		if err != nil {
			return fmt.Errorf("processor shutdown error: %w", err)
		}
	case <-shutdownCtx.Done():
		appLogger.Warn("processor shutdown timed out")
	}

	appLogger.Info("news-processor stopped successfully")
	return nil
}
