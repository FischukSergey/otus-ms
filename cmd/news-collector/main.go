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

	"github.com/redis/go-redis/v9"

	mainserviceclient "github.com/FischukSergey/otus-ms/internal/clients/mainservice"
	"github.com/FischukSergey/otus-ms/internal/config"
	"github.com/FischukSergey/otus-ms/internal/keycloak"
	"github.com/FischukSergey/otus-ms/internal/logger"
	"github.com/FischukSergey/otus-ms/internal/services/collector"
	redisstate "github.com/FischukSergey/otus-ms/internal/store/collector"
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

	// Инициализируем Redis клиент (хранилище операционного состояния сбора)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect to redis %s: %w", cfg.Redis.Addr, err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			appLogger.Error("redis close error", "error", err)
		}
	}()
	appLogger.Info("redis connected", "addr", cfg.Redis.Addr, "db", cfg.Redis.DB)

	// Собираем сервис сбора новостей
	stateStore := redisstate.NewRedisStateStore(redisClient)
	dedupStore := redisstate.NewRedisDedupStore(redisClient, cfg.Collector.GetDedupTTL())
	parser := collector.NewParser(cfg.Collector.ParseTimeout, appLogger)
	collectorService := collector.NewService(
		grpcClient,
		stateStore,
		dedupStore,
		parser,
		appLogger,
		collector.ServiceConfig{
			MaxWorkers:  cfg.Collector.MaxWorkers,
			MaxRetries:  cfg.Collector.MaxRetries,
			MaxErrCount: cfg.Collector.MaxErrCount,
			BaseBackoff: cfg.Collector.GetDeactivationBaseBackoff(),
			MaxBackoff:  cfg.Collector.GetDeactivationMaxBackoff(),
		},
	)

	// Первичная загрузка источников при старте
	appLogger.Info("fetching news sources from main-service via gRPC...")
	collectorService.RefreshSources(ctx)

	// Запускаем планировщик
	scheduler := collector.NewScheduler(collectorService, appLogger)
	if err := scheduler.Start(ctx); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	defer scheduler.Stop()

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
