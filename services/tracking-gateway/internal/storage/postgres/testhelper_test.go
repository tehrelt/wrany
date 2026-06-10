package postgres_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/wrany/tracking-gateway/internal/migrations"
)

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

	require.NoError(t, migrations.Run(dsn, migrationsPath))

	db, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		_ = ctr.Terminate(ctx)
	}
	return db, cleanup
}

// insertTestUser inserts a minimal user row and returns the user_id.
// Required because ingested_location_events.user_id FK references users.id.
func insertTestUser(t *testing.T, db *pgxpool.Pool) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	_, err := db.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash, created_at, updated_at)
		 VALUES ($1, $2, 'hash', now(), now())`,
		userID, userID.String()+"@test.com",
	)
	require.NoError(t, err)
	return userID
}
