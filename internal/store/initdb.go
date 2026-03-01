package store

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Options содержит параметры подключения к базе данных.
//
//go:generate options-gen -out-filename=storage_options.gen.go -from-struct=Options
type Options struct {
	dbName        string `option:"mandatory"`
	dbUser        string `option:"mandatory"`
	dbPassword    string `option:"mandatory"`
	dbHost        string `option:"mandatory"`
	dbPort        string `option:"mandatory"`
	dbSSLMode     string `option:"optional" default:"disable"`
	dbSSLRootCert string
	dbSSLKey      string
}

// Storage is the database connection.
type Storage struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewStorage creates a new database connection.
func NewStorage(ctx context.Context, opts Options) (*Storage, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options for storage: %v", err)
	}

	// Экранируем пароль для URL
	escapedPassword := url.QueryEscape(opts.dbPassword)

	// Используем net.JoinHostPort для корректного формирования host:port
	hostPort := net.JoinHostPort(opts.dbHost, opts.dbPort)
	dbconn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
		opts.dbUser, escapedPassword, hostPort, opts.dbName, opts.dbSSLMode)

	pool, err := pgxpool.New(ctx, dbconn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &Storage{
		db:     pool,
		logger: slog.Default(),
	}, nil
}

// SetLogger устанавливает logger для storage.
func (s *Storage) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// DB возвращает пул подключений к базе данных.
func (s *Storage) DB() *pgxpool.Pool {
	return s.db
}

// Close closes the database connection pool.
func (s *Storage) Close() {
	s.db.Close()
}

// Ping the database.
func (s *Storage) Ping(ctx context.Context) error {
	return s.db.Ping(ctx)
}
