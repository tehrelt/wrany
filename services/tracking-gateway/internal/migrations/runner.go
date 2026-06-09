package migrations

import (
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Run applies all pending migrations from migrationsPath against databaseURL.
// Returns nil if no migrations are pending (migrate.ErrNoChange is swallowed).
// Exits non-zero if any error occurs — callers must treat errors as fatal.
func Run(databaseURL, migrationsPath string) error {
	sourceURL := "file://" + migrationsPath

	m, err := migrate.New(sourceURL, databaseURL)
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
