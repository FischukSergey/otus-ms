// @title           OtusMS Auth-Proxy API
// @version         1.0.0
// @description     API для аутентификации пользователей через Keycloak в проекте OtusMS.
// @termsOfService  http://swagger.io/terms/

// @contact.name   OtusMS Support
// @contact.url    https://github.com/FischukSergey/otus-ms

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      fishouk-otus-ms.ru
// @BasePath  /
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
	"github.com/FischukSergey/otus-ms/internal/keycloak"
	"github.com/FischukSergey/otus-ms/internal/logger"
)

var configPath = flag.String("config", "configs/config.auth-proxy.local.yaml", "Path to config file")

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("run app: %v", err)
	}
}

func run() error {
	// Парсим и валидируем конфигурацию
	cfg, err := config.ParseAndValidate(*configPath)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Создаем логгер на основе конфигурации
	appLogger := logger.NewLogger(cfg.Log)
	appLogger.Info("Auth-Proxy service starting...")

	// Проверяем наличие конфигурации Keycloak
	if !cfg.Keycloak.IsConfigured() {
		return errors.New("keycloak configuration is incomplete: please provide url, realm, client_id and client_secret")
	}

	// Создаем клиент Keycloak
	appLogger.Info("Initializing Keycloak client",
		"url", cfg.Keycloak.URL,
		"realm", cfg.Keycloak.Realm,
		"client_id", cfg.Keycloak.ClientID,
	)

	keycloakClient := keycloak.NewClient(
		cfg.Keycloak.URL,
		cfg.Keycloak.Realm,
		cfg.Keycloak.ClientID,
		cfg.Keycloak.ClientSecret,
	)

	appLogger.Info("Keycloak client initialized successfully")

	// Создаем контекст для отслеживания сигналов прерывания
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	// Создаем API сервер
	apiServer := NewAPIServer(&APIServerDeps{
		Addr:           cfg.Servers.Client.Addr,
		Config:         &cfg,
		Logger:         appLogger,
		KeycloakClient: keycloakClient,
	})

	// Запускаем API сервер в отдельной горутине
	go func() {
		appLogger.Info("Starting API server", "addr", cfg.Servers.Client.Addr)
		if err := apiServer.Start(); err != nil {
			appLogger.Error("API server error", "error", err)
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

	appLogger.Info("Auth-Proxy service stopped successfully")
	return nil
}
