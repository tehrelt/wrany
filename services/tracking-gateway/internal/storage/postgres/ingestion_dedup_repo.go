package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IngestionDedupRepo persists and queries the ingestion dedup ledger.
// The ledger records events that have been successfully published to NATS,
// enabling protocol-level duplicated responses to retrying clients.
//
// Dedup scope: (user_id, device_id, event_id) — not global. Two different
// devices may independently generate the same event_id without conflict.
type IngestionDedupRepo struct {
	db *pgxpool.Pool
}

func NewIngestionDedupRepo(db *pgxpool.Pool) *IngestionDedupRepo {
	return &IngestionDedupRepo{db: db}
}

// IsDuplicate reports whether (userID, deviceID, eventID) exists in the ledger.
func (r *IngestionDedupRepo) IsDuplicate(ctx context.Context, userID, deviceID uuid.UUID, eventID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM ingested_location_events
			WHERE user_id = $1 AND device_id = $2 AND event_id = $3
		)`,
		userID, deviceID, eventID,
	).Scan(&exists)
	return exists, err
}

// MarkPublished inserts the event into the ledger after a successful NATS PubAck.
// ON CONFLICT DO NOTHING — a conflict here is non-fatal: it means a concurrent
// goroutine already inserted the same (user_id, device_id, event_id) after its own
// PubAck. The publish is already confirmed; the ACK to the client stays successful.
func (r *IngestionDedupRepo) MarkPublished(ctx context.Context, userID, deviceID uuid.UUID, eventID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO ingested_location_events (user_id, device_id, event_id, received_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT DO NOTHING`,
		userID, deviceID, eventID, time.Now().UTC(),
	)
	return err
}
