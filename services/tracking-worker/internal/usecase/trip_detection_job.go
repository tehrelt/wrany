package usecase

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	eventbus "github.com/wrany/libs/eventbus"
	tripevents "github.com/wrany/libs/events/trip"
	"github.com/wrany/tracking-worker/internal/domain"
	"github.com/wrany/tracking-worker/internal/usecase/noise"
)

// TripDetectionRepository is the storage interface required by TripDetectionJob.
// The implementation applies TripDetectionBatch atomically in a single transaction.
type TripDetectionRepository interface {
	// LoadDistinctUserDevicePairs returns pairs that have raw points before the given time
	// and may need detection work.
	LoadDistinctUserDevicePairs(ctx context.Context, before time.Time) ([]domain.UserDevicePair, error)
	// LoadState returns the persisted state for a pair; returns a default IDLE state if none exists.
	LoadState(ctx context.Context, userID, deviceID uuid.UUID) (domain.TripDetectionState, error)
	FetchUnprocessedPoints(ctx context.Context, userID, deviceID uuid.UUID, from, to time.Time) ([]domain.RawLocationPoint, error)
	FetchProcessedHistory(ctx context.Context, userID, deviceID uuid.UUID, before time.Time, limit int) ([]domain.ProcessedLocationPoint, error)
	// ApplyBatch persists all detection results atomically.
	ApplyBatch(ctx context.Context, batch domain.TripDetectionBatch) error
}

// TripDetectionJob runs one detection cycle per call to RunOnce.
type TripDetectionJob struct {
	repo     TripDetectionRepository
	uc       *TripDetectionUseCase
	pub      eventbus.Publisher
	producer string
	cfg      domain.TripDetectionConfig
	noise    *noise.Pipeline
}

// NewTripDetectionJob wires all dependencies.
func NewTripDetectionJob(
	repo TripDetectionRepository,
	pub eventbus.Publisher,
	producer string,
	cfg domain.TripDetectionConfig,
	noiseCfg domain.NoiseConfig,
) *TripDetectionJob {
	return &TripDetectionJob{
		repo:     repo,
		uc:       NewTripDetectionUseCase(cfg),
		pub:      pub,
		producer: producer,
		cfg:      cfg,
		noise:    noise.NewPipeline(noiseCfg, nil),
	}
}

// RunOnce processes all user/device pairs that have unprocessed raw points.
// Errors per pair are logged and do not abort other pairs.
func (j *TripDetectionJob) RunOnce(ctx context.Context) error {
	now := time.Now()
	watermarkBoundary := now.Add(-time.Duration(j.cfg.LateArrivalWindowSec) * time.Second)

	pairs, err := j.repo.LoadDistinctUserDevicePairs(ctx, watermarkBoundary)
	if err != nil {
		return err
	}

	for _, p := range pairs {
		if err := j.processPair(ctx, p.UserID, p.DeviceID, now); err != nil {
			log.Printf("trip_detection_job: pair %s/%s: %v", p.UserID, p.DeviceID, err)
		}
	}
	return nil
}

const maxLookback = 24 * time.Hour

func (j *TripDetectionJob) processPair(ctx context.Context, userID, deviceID uuid.UUID, now time.Time) error {
	state, err := j.repo.LoadState(ctx, userID, deviceID)
	if err != nil {
		return err
	}

	to := now.Add(-time.Duration(state.LateArrivalWindowSec) * time.Second)
	var from time.Time
	if state.LastProcessedAt != nil {
		from = state.LastProcessedAt.Add(-time.Duration(state.LateArrivalWindowSec) * time.Second)
	} else {
		from = to.Add(-maxLookback)
	}

	if !to.After(from) {
		return nil
	}

	points, err := j.repo.FetchUnprocessedPoints(ctx, userID, deviceID, from, to)
	if err != nil {
		return err
	}

	history, err := j.repo.FetchProcessedHistory(ctx, userID, deviceID, to, 100)
	if err != nil {
		return err
	}
	noiseResult := j.noise.ProcessBatch(history, points, state.LastProcessedAt, now)
	result := j.uc.ProcessBatch(state, noiseResult.Accepted, now)

	batch := buildBatch(result)
	batch.ProcessedPoints = noiseResult.Processed
	if err := j.repo.ApplyBatch(ctx, batch); err != nil {
		return err
	}

	j.publishEvents(ctx, result.Commands, now)
	return nil
}

// buildBatch translates a ProcessBatchResult into the storage-layer batch struct.
func buildBatch(result ProcessBatchResult) domain.TripDetectionBatch {
	batch := domain.TripDetectionBatch{
		NewState:      result.NewState,
		BackfillSince: make(map[uuid.UUID]time.Time),
	}
	for _, cmd := range result.Commands {
		switch cmd.Kind {
		case CmdCreateTrip:
			batch.NewTrips = append(batch.NewTrips, cmd.Trip)
			if cmd.BackfillSince != nil {
				batch.BackfillSince[cmd.TripID] = *cmd.BackfillSince
			}
			batch.NewPoints = append(batch.NewPoints, cmd.Points...)
		case CmdUpdateTrip:
			batch.UpdatedTrips = append(batch.UpdatedTrips, domain.TripStatsDelta{
				TripID:     cmd.TripID,
				DeltaDistM: cmd.DeltaDistanceM,
				DeltaPts:   cmd.DeltaPointsCount,
				LastPtAt:   cmd.LastPointAt,
				LastLat:    cmd.LastPointLat,
				LastLon:    cmd.LastPointLon,
			})
			batch.NewPoints = append(batch.NewPoints, cmd.Points...)
		case CmdCompleteTrip:
			batch.CompletedTrips = append(batch.CompletedTrips, domain.TripCompletion{
				TripID:  cmd.TripID,
				EndedAt: *cmd.EndedAt,
				EndLat:  *cmd.EndLat,
				EndLon:  *cmd.EndLon,
			})
		}
	}
	return batch
}

// publishEvents fires NATS events for each command; errors are logged, not fatal.
func (j *TripDetectionJob) publishEvents(ctx context.Context, commands []TripCommand, now time.Time) {
	for _, cmd := range commands {
		var err error
		switch cmd.Kind {
		case CmdCreateTrip:
			t := cmd.Trip
			ev, buildErr := tripevents.NewStartedEvent(
				uuid.New().String(), now, j.producer, t.ID.String(),
				tripevents.StartedPayload{
					TripID:    t.ID.String(),
					UserID:    t.UserID.String(),
					DeviceID:  t.DeviceID.String(),
					StartedAt: t.StartedAt,
				},
			)
			if buildErr != nil {
				log.Printf("trip_detection_job: build started event: %v", buildErr)
				continue
			}
			err = j.pub.Publish(ctx, "trip.started.v1", ev)

		case CmdUpdateTrip:
			ev, buildErr := tripevents.NewUpdatedEvent(
				uuid.New().String(), now, j.producer, cmd.TripID.String(),
				tripevents.UpdatedPayload{
					TripID:    cmd.TripID.String(),
					UpdatedAt: now,
				},
			)
			if buildErr != nil {
				log.Printf("trip_detection_job: build updated event: %v", buildErr)
				continue
			}
			err = j.pub.Publish(ctx, "trip.updated.v1", ev)

		case CmdCompleteTrip:
			ev, buildErr := tripevents.NewCompletedEvent(
				uuid.New().String(), now, j.producer, cmd.TripID.String(),
				tripevents.CompletedPayload{
					TripID:      cmd.TripID.String(),
					CompletedAt: *cmd.EndedAt,
				},
			)
			if buildErr != nil {
				log.Printf("trip_detection_job: build completed event: %v", buildErr)
				continue
			}
			err = j.pub.Publish(ctx, "trip.completed.v1", ev)
		}
		if err != nil {
			log.Printf("trip_detection_job: publish %d: %v", cmd.Kind, err)
		}
	}
}
