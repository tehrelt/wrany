package postgres_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/wrany/tracking-worker/internal/migrations"
)

// newTestDB spins up a postgis/postgis:16-3.4 container, applies worker
// migrations, and returns a ready pool.  The cleanup function must be deferred.
func newTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgis/postgis:16-3.4",
		tcpostgres.WithDatabase("wrany_test"),
		tcpostgres.WithUsername("wrany"),
		tcpostgres.WithPassword("wrany"),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				WaitingFor: wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(120 * time.Second),
			},
		}),
	)
	require.NoError(t, err)

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	_, file, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(file), "..", "..", "..", "infra", "migrations")
	require.NoError(t, migrations.RunWithTable(dsn, migrationsPath, "tracking_worker_schema_migrations"))

	db, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	return db, func() {
		db.Close()
		_ = ctr.Terminate(ctx)
	}
}
