// Package store управляет подключением к БД и применением SQL-миграций.
package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migration представляет одну SQL-миграцию базы данных.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// RunMigrations применяет все непримённые миграции в порядке возрастания версии.
func (s *Storage) RunMigrations(ctx context.Context) error {
	// Создаем таблицу для отслеживания миграций
	if err := s.createMigrationsTable(ctx); err != nil {
		return err
	}

	// Получаем список миграций
	migrations, err := s.loadMigrations()
	if err != nil {
		return err
	}

	// Выполняем неприменённые миграции
	for _, migration := range migrations {
		applied, err := s.isMigrationApplied(ctx, migration.Version)
		if err != nil {
			return err
		}

		if !applied {
			if err := s.applyMigration(ctx, migration); err != nil {
				return fmt.Errorf("apply migration %d: %w", migration.Version, err)
			}
		}
	}

	return nil
}

// createMigrationsTable создает таблицу для отслеживания примененных миграций.
func (s *Storage) createMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT NOW()
		)
	`

	_, err := s.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	return nil
}

// loadMigrations загружает все миграции из embedded файлов.
func (s *Storage) loadMigrations() ([]Migration, error) {
	var migrations []Migration

	err := fs.WalkDir(migrationFiles, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		// Извлекаем версию из имени файла (например, 001_initial_schema.sql -> 1)
		filename := filepath.Base(path)
		parts := strings.SplitN(filename, "_", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid migration filename format: %s", filename)
		}

		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid version in filename %s: %w", filename, err)
		}

		// Читаем содержимое файла
		content, err := migrationFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", path, err)
		}

		migration := Migration{
			Version: version,
			Name:    strings.TrimSuffix(filename, ".sql"),
			SQL:     string(content),
		}

		migrations = append(migrations, migration)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}

	// Сортируем по версии
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// isMigrationApplied проверяет, была ли миграция уже применена.
func (s *Storage) isMigrationApplied(ctx context.Context, version int) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)"

	err := s.db.QueryRow(ctx, query, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %d: %w", version, err)
	}

	return exists, nil
}

// applyMigration применяет миграцию в транзакции.
func (s *Storage) applyMigration(ctx context.Context, migration Migration) error {
	s.logger.Info("Applying migration",
		"version", migration.Version,
		"name", migration.Name)

	// Начинаем транзакцию
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Выполняем SQL миграции
	_, err = tx.Exec(ctx, migration.SQL)
	if err != nil {
		return fmt.Errorf("execute migration SQL: %w", err)
	}

	// Записываем информацию о примененной миграции
	_, err = tx.Exec(ctx,
		"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
		migration.Version, migration.Name)
	if err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	s.logger.Info("Migration applied successfully",
		"version", migration.Version,
		"name", migration.Name)

	return nil
}

// GetAppliedMigrations возвращает список примененных миграций (для отладки).
func (s *Storage) GetAppliedMigrations(ctx context.Context) ([]Migration, error) {
	query := `
		SELECT version, name 
		FROM schema_migrations 
		ORDER BY version
	`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	var migrations []Migration
	for rows.Next() {
		var migration Migration
		err := rows.Scan(&migration.Version, &migration.Name)
		if err != nil {
			return nil, fmt.Errorf("scan migration: %w", err)
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}
