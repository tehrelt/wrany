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

// Run applies all pending migrations from migrationsPath against databaseURL.
// Returns nil if no migrations are pending (migrate.ErrNoChange is swallowed).
// Callers must treat errors as fatal — the service must not start if migrations fail.
func Run(databaseURL, migrationsPath string) error {
	// golang-migrate pgx/v5 driver requires "pgx5://" scheme.
	dbURL := toPgx5URL(databaseURL)
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

// pathToFileURL converts a filesystem path to a file: URL that golang-migrate's
// source/file driver accepts on both Linux and Windows.
//
// golang-migrate parses the URL with net/url.Parse and reads:
//
//	p = u.Opaque + u.Host + u.Path
//
// Using "file:" + forwardSlashPath (no "//") keeps the path as u.Opaque on
// Windows (e.g. "file:C:/foo" → Opaque="C:/foo") and as u.Path on Linux
// (e.g. "file:/tmp/foo" → Path="/tmp/foo"), both of which resolve correctly.
func pathToFileURL(p string) string {
	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	return "file:" + filepath.ToSlash(p)
}
