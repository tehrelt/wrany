package migrations

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Run applies all pending migrations from migrationsPath against databaseURL
// using the default schema_migrations table.
func Run(databaseURL, migrationsPath string) error {
	return RunWithTable(databaseURL, migrationsPath, "")
}

// RunWithTable is like Run but uses a custom migrations table name.
// Use a unique table per service when multiple services share one Postgres instance.
func RunWithTable(databaseURL, migrationsPath, tableName string) error {
	dbURL := toPgx5URL(databaseURL)
	if tableName != "" {
		sep := "?"
		if strings.Contains(dbURL, "?") {
			sep = "&"
		}
		dbURL += sep + "x-migrations-table=" + tableName
	}
	sourceURL := pathToFileURL(migrationsPath)

	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("migrations: close source error: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("migrations: close db error: %v", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	v, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("get version: %w", err)
	}
	log.Printf("migrations: version=%d dirty=%v", v, dirty)
	return nil
}

// toPgx5URL rewrites postgres:// / postgresql:// → pgx5:// for the migrate pgx/v5 driver.
func toPgx5URL(dsn string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(dsn, prefix) {
			return "pgx5://" + dsn[len(prefix):]
		}
	}
	return dsn
}

// pathToFileURL converts a filesystem path to a file: URL for golang-migrate's
// source/file driver. Works on both Linux and Windows.
func pathToFileURL(p string) string {
	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	return "file:" + filepath.ToSlash(p)
}
