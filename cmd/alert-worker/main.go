// Package main является точкой входа alert-worker.
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

	mainserviceclient "github.com/FischukSergey/otus-ms/internal/clients/mainservice"
	"github.com/FischukSergey/otus-ms/internal/config"
	"github.com/FischukSergey/otus-ms/internal/keycloak"
	"github.com/FischukSergey/otus-ms/internal/logger"
	"github.com/FischukSergey/otus-ms/internal/services/alertworker"
)

var configPath = flag.String("config", "configs/config.alert-worker.local.yaml", "Path to config file")

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("run alert-worker: %v", err)
	}
}

func run() error {
	cfg, err := config.ParseAndValidate(*configPath)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if !cfg.Kafka.IsConfigured() || cfg.Kafka.TopicNewsAlerts == "" {
		return errors.New("kafka is not configured for alert-worker: brokers and topic_news_alerts are required")
	}
	if !cfg.Telegram.IsConfigured() {
		return errors.New("telegram configuration is incomplete: bot_token and project_chat_id are required")
	}
	if cfg.MainService.GRPCAddr == "" {
		return errors.New("main_service.grpc_addr is required")
	}
	if !cfg.Keycloak.IsConfigured() {
		return errors.New("keycloak configuration is incomplete for service account auth")
	}

	appLogger := logger.NewLogger(cfg.Log)
	appLogger.Info("alert-worker starting", "env", cfg.Global.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	keycloakClient := keycloak.NewClient(
		cfg.Keycloak.URL,
		cfg.Keycloak.Realm,
		cfg.Keycloak.ClientID,
		cfg.Keycloak.ClientSecret,
	)

	grpcClient, err := mainserviceclient.NewGRPCClient(
		cfg.MainService.GRPCAddr,
		keycloakClient,
		appLogger,
	)
	if err != nil {
		return fmt.Errorf("init grpc client: %w", err)
	}
	defer func() {
		if closeErr := grpcClient.Close(); closeErr != nil {
			appLogger.Error("grpc client close error", "error", closeErr)
		}
	}()

	sender := alertworker.NewTelegramSender(cfg.Telegram)
	svc := alertworker.NewService(cfg.Kafka, grpcClient, sender, appLogger)

	workerErr := make(chan error, 1)
	go func() {
		workerErr <- svc.Run(ctx)
	}()

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

	select {
	case <-ctx.Done():
		appLogger.Info("shutdown signal received")
	case err := <-workerErr:
		if err != nil {
			appLogger.Error("alert-worker exited with error", "error", err)
			cancel()
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := apiServer.Stop(shutdownCtx); err != nil {
		appLogger.Error("api server shutdown error", "error", err)
	}

	select {
	case err := <-workerErr:
		if err != nil {
			return fmt.Errorf("alert-worker shutdown error: %w", err)
		}
	case <-shutdownCtx.Done():
		appLogger.Warn("alert-worker shutdown timed out")
	}

	return nil
}
