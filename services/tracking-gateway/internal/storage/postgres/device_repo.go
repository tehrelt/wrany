package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wrany/tracking-gateway/internal/domain"
)

type DeviceRepo struct {
	db *pgxpool.Pool
}

func NewDeviceRepo(db *pgxpool.Pool) *DeviceRepo {
	return &DeviceRepo{db: db}
}

func (r *DeviceRepo) Upsert(ctx context.Context, d *domain.Device) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO devices (id, user_id, device_id, name, platform, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			last_seen_at = EXCLUDED.last_seen_at,
			updated_at   = EXCLUDED.updated_at,
			name         = COALESCE(EXCLUDED.name, devices.name),
			platform     = COALESCE(EXCLUDED.platform, devices.platform)
	`,
		d.ID, d.UserID, d.DeviceID, d.Name, d.Platform,
		d.LastSeenAt, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (r *DeviceRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, device_id, name, platform, last_seen_at, created_at, updated_at
		FROM devices WHERE user_id = $1 ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Device
	for rows.Next() {
		d := &domain.Device{}
		if err := rows.Scan(
			&d.ID, &d.UserID, &d.DeviceID,
			&d.Name, &d.Platform,
			&d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
