package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/tracking-gateway/internal/storage/postgres"
)

func TestIngestionDedupRepo_IsDuplicate_NotFound(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewIngestionDedupRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()

	dup, err := repo.IsDuplicate(context.Background(), userID, deviceID, "evt-1")
	require.NoError(t, err)
	assert.False(t, dup)
}

func TestIngestionDedupRepo_MarkPublished_ThenIsDuplicate(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewIngestionDedupRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()

	require.NoError(t, repo.MarkPublished(context.Background(), userID, deviceID, "evt-1"))

	dup, err := repo.IsDuplicate(context.Background(), userID, deviceID, "evt-1")
	require.NoError(t, err)
	assert.True(t, dup)
}

func TestIngestionDedupRepo_MarkPublished_OnConflictDoNothing(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewIngestionDedupRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()

	// First insert succeeds.
	require.NoError(t, repo.MarkPublished(context.Background(), userID, deviceID, "evt-1"))
	// Second insert on conflict is non-fatal.
	require.NoError(t, repo.MarkPublished(context.Background(), userID, deviceID, "evt-1"))
}

func TestIngestionDedupRepo_SameEventDifferentDevice_NotDuplicate(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewIngestionDedupRepo(db)
	userID := insertTestUser(t, db)
	device1 := uuid.New()
	device2 := uuid.New()

	require.NoError(t, repo.MarkPublished(context.Background(), userID, device1, "evt-1"))

	// Same event_id on device2 is NOT a duplicate.
	dup, err := repo.IsDuplicate(context.Background(), userID, device2, "evt-1")
	require.NoError(t, err)
	assert.False(t, dup)
}

func TestIngestionDedupRepo_ConcurrentInsert_OnlyOneSucceeds(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewIngestionDedupRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()

	const goroutines = 10
	var wg sync.WaitGroup
	errors := make([]error, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			errors[idx] = repo.MarkPublished(context.Background(), userID, deviceID, "evt-race")
		}(i)
	}
	wg.Wait()

	// All calls should succeed (ON CONFLICT DO NOTHING absorbs conflicts).
	for i, err := range errors {
		assert.NoError(t, err, "goroutine %d error", i)
	}

	// Exactly one row inserted.
	var count int
	err := db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM ingested_location_events WHERE user_id=$1 AND device_id=$2 AND event_id=$3`,
		userID, deviceID, "evt-race",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
