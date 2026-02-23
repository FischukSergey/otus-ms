// Package main является точкой входа сервиса сбора новостей.
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
)

var configPath = flag.String("config", "configs/config.news-collector.local.yaml", "Path to config file")

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("run news-collector: %v", err)
	}
}

func run() error {
	cfg, err := config.ParseAndValidate(*configPath)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	appLogger := logger.NewLogger(cfg.Log)
	appLogger.Info("news-collector starting",
		"env", cfg.Global.Env,
		"main_service_grpc", cfg.MainService.GRPCAddr,
	)

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

	// Получаем seed-данные от main-service по gRPC
	appLogger.Info("fetching news sources from main-service via gRPC...")
	sources, err := grpcClient.GetNewsSources(ctx)
	if err != nil {
		// Не фатально для MVP — логируем и продолжаем запуск
		appLogger.Error("failed to fetch news sources from main-service", "error", err)
	} else {
		appLogger.Info("news sources received from main-service", "count", len(sources))
		for _, s := range sources {
			appLogger.Debug("source",
				"id", s.ID,
				"name", s.Name,
				"url", s.URL,
				"category", s.Category,
				"language", s.Language,
				"fetch_interval_sec", s.FetchInterval,
				"is_active", s.IsActive,
			)
		}
	}

	// Запускаем HTTP сервер (health check)
	apiServer := NewAPIServer(&APIServerDeps{
		Addr:   cfg.Servers.Client.Addr,
		Logger: appLogger,
	})

	go func() {
		appLogger.Info("Starting API server", "addr", cfg.Servers.Client.Addr)
		if err := apiServer.Start(); err != nil {
			appLogger.Error("API server error", "error", err)
		}
	}()

	// Ждём сигнал завершения
	<-ctx.Done()
	appLogger.Info("Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := apiServer.Stop(shutdownCtx); err != nil {
		appLogger.Error("Error during API server shutdown", "error", err)
		return err
	}

	appLogger.Info("news-collector stopped successfully")
	return nil
}
