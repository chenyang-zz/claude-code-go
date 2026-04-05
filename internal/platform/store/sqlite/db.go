package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// DB wraps the minimum SQLite handle used by the session repository.
type DB struct {
	// Path identifies the backing SQLite file on disk.
	Path string
	// SQL stores the opened database handle.
	SQL *sql.DB
}

// Open opens one SQLite database file, creates parent directories when needed, and applies the minimum schema.
func Open(ctx context.Context, path string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("missing sqlite database path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite parent directory: %w", err)
	}

	handle, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db := &DB{
		Path: path,
		SQL:  handle,
	}
	if err := db.migrate(ctx); err != nil {
		_ = handle.Close()
		return nil, err
	}

	logger.DebugCF("sqlite_store", "opened sqlite database", map[string]any{
		"path": path,
	})
	return db, nil
}

// Close releases the underlying SQLite handle.
func (db *DB) Close() error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Close()
}

// migrate applies the minimum schema required by the session repository.
func (db *DB) migrate(ctx context.Context) error {
	if db == nil || db.SQL == nil {
		return fmt.Errorf("sqlite database is not initialized")
	}

	for _, migration := range sessionMigrations {
		if _, err := db.SQL.ExecContext(ctx, migration.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}
	}

	logger.DebugCF("sqlite_store", "applied sqlite migrations", map[string]any{
		"path":            db.Path,
		"migration_count": len(sessionMigrations),
	})
	return nil
}
