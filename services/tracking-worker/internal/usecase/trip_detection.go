package usecase

import (
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/wrany/tracking-worker/internal/domain"
)

// CommandKind identifies what persistence action the job must perform.
type CommandKind int8

const (
	// CmdCreateTrip: INSERT a new trip and link Points to it.
	CmdCreateTrip CommandKind = iota
	// CmdUpdateTrip: add Points and apply delta stats to an existing trip.
	CmdUpdateTrip
	// CmdCompleteTrip: mark a trip as TRIP_COMPLETED with end position.
	CmdCompleteTrip
)

// TripCommand is a persistence instruction produced by ProcessBatch.
// Commands must be applied in slice order within a single DB transaction.
type TripCommand struct {
	Kind   CommandKind
	TripID uuid.UUID

	// CmdCreateTrip: the initial trip row to INSERT.
	Trip *domain.Trip

	// CmdCreateTrip: if the candidate window opened before the current batch, the job must
	// also fetch raw_location_points with recorded_at in [BackfillSince, Points[0].RecordedAt)
	// and insert them into trip_points (ON CONFLICT DO NOTHING for idempotency).
	BackfillSince *time.Time

	// CmdUpdateTrip / CmdCompleteTrip: incremental stats to add to the trip row.
	DeltaDistanceM   float64
	DeltaPointsCount int
	// LastPointAt and LastPointLat/Lon update the effective trip end position.
	// The job sets: duration_sec = EXTRACT(EPOCH FROM last_point_at - started_at).
	LastPointAt  *time.Time
	LastPointLat *float64
	LastPointLon *float64

	// CmdCompleteTrip: final end fields.
	EndedAt *time.Time
	EndLat  *float64
	EndLon  *float64

	// Points to INSERT into trip_points (applies to all command kinds).
	Points []domain.TripPoint
}

// ProcessBatchResult is the output of a single ProcessBatch call.
type ProcessBatchResult struct {
	NewState domain.TripDetectionState
	// Commands must be applied in order by the caller.
	Commands []TripCommand
}

// TripDetectionUseCase implements pure state-machine logic for trip detection.
// It performs no I/O — all persistence is delegated to the caller via Commands.
type TripDetectionUseCase struct {
	cfg domain.TripDetectionConfig
}

// NewTripDetectionUseCase creates a use case with the given detection thresholds.
func NewTripDetectionUseCase(cfg domain.TripDetectionConfig) *TripDetectionUseCase {
	return &TripDetectionUseCase{cfg: cfg}
}

// ProcessBatch advances the state machine for one (user_id, device_id) pair.
// points must be ordered by recorded_at ASC and belong to a single user+device.
// now is the current wall-clock time used to advance the watermark.
func (u *TripDetectionUseCase) ProcessBatch(
	state domain.TripDetectionState,
	points []domain.RawLocationPoint,
	now time.Time,
) ProcessBatchResult {
	result := ProcessBatchResult{NewState: state}

	// candidatePoints: accumulated during MOTION_CANDIDATE in this batch.
	var candidatePoints []domain.TripPoint

	// activeTripPoints / activeTripDeltaDistM: accumulated during TRIP_ACTIVE in this batch.
	var activeTripPoints []domain.TripPoint
	var activeTripDeltaDistM float64
	// lastActivePoint: the last MOVING raw point seen in TRIP_ACTIVE (for end-position tracking).
	var lastActivePoint *domain.RawLocationPoint

	// flushActive emits a CmdUpdateTrip for all active-trip data accumulated so far.
	// Called before any state transition that leaves TRIP_ACTIVE.
	flushActive := func() {
		if len(activeTripPoints) == 0 {
			return
		}
		cmd := TripCommand{
			Kind:             CmdUpdateTrip,
			TripID:           *result.NewState.ActiveTripID,
			DeltaDistanceM:   activeTripDeltaDistM,
			DeltaPointsCount: len(activeTripPoints),
			Points:           append([]domain.TripPoint(nil), activeTripPoints...),
		}
		if lastActivePoint != nil {
			t := lastActivePoint.RecordedAt
			cmd.LastPointAt = &t
			cmd.LastPointLat = &lastActivePoint.Lat
			cmd.LastPointLon = &lastActivePoint.Lon
		}
		result.Commands = append(result.Commands, cmd)
		activeTripPoints = activeTripPoints[:0]
		activeTripDeltaDistM = 0
		lastActivePoint = nil
	}

	for i := range points {
		pt := &points[i]

		// --- Noise filtering ---

		if pt.AccuracyM > u.cfg.MaxAccuracyM {
			continue
		}
		if result.NewState.LastProcessedAt != nil && !pt.RecordedAt.After(*result.NewState.LastProcessedAt) {
			continue // duplicate or out-of-order past the processed mark
		}

		// Compute distance and calculated speed from the previous accepted point.
		var distFromPrev float64
		var calcSpeedMps float64
		hasPrev := result.NewState.LastPointLat != nil && result.NewState.LastPointLon != nil
		if hasPrev {
			distFromPrev = haversineM(*result.NewState.LastPointLat, *result.NewState.LastPointLon, pt.Lat, pt.Lon)
			if result.NewState.LastProcessedAt != nil {
				elapsed := pt.RecordedAt.Sub(*result.NewState.LastProcessedAt).Seconds()
				if elapsed > 0 {
					calcSpeedMps = distFromPrev / elapsed
				}
			}
			if calcSpeedMps > u.cfg.MaxSpeedJumpMps {
				continue // GPS jump — discard without updating last-point
			}
		}

		// Effective speed: sensor value is authoritative when present (it knows the device is
		// stationary even if GPS drift produces non-zero calculated speed). Fall back to
		// calculated speed only when no sensor speed is available.
		effectiveSpeed := calcSpeedMps
		if pt.SpeedMps != nil {
			effectiveSpeed = *pt.SpeedMps
		}
		const movingThresholdMps = 0.56 // ~2 km/h
		isMoving := effectiveSpeed >= movingThresholdMps

		tp := domain.TripPoint{
			UserID:     pt.UserID,
			DeviceID:   pt.DeviceID,
			EventID:    pt.EventID,
			RecordedAt: pt.RecordedAt,
		}

		// --- State machine ---

		switch result.NewState.State {

		case domain.StateIdle:
			if isMoving {
				t := pt.RecordedAt
				id := pt.EventID
				result.NewState.State = domain.StateMotionCandidate
				result.NewState.CandidateStartedAt = &t
				result.NewState.CandidateStartPointID = &id
				result.NewState.CandidateDistanceM = 0
				result.NewState.CandidateStartLat = &pt.Lat
				result.NewState.CandidateStartLon = &pt.Lon
				candidatePoints = append(candidatePoints, tp)
			}

		case domain.StateMotionCandidate:
			if !isMoving {
				// Motion broke — reset candidate.
				result.NewState.State = domain.StateIdle
				result.NewState.CandidateStartedAt = nil
				result.NewState.CandidateStartPointID = nil
				result.NewState.CandidateDistanceM = 0
				result.NewState.CandidateStartLat = nil
				result.NewState.CandidateStartLon = nil
				candidatePoints = candidatePoints[:0]
				break
			}
			result.NewState.CandidateDistanceM += distFromPrev
			candidatePoints = append(candidatePoints, tp)

			duration := pt.RecordedAt.Sub(*result.NewState.CandidateStartedAt)
			if int(duration.Seconds()) >= u.cfg.MotionMinDurationSec &&
				result.NewState.CandidateDistanceM >= u.cfg.MotionMinDistanceM {
				// Thresholds met: create the trip.
				tripID := uuid.New()
				startLat := *result.NewState.CandidateStartLat
				startLon := *result.NewState.CandidateStartLon
				backfillTime := *result.NewState.CandidateStartedAt
				trip := domain.Trip{
					ID:          tripID,
					UserID:      pt.UserID,
					DeviceID:    pt.DeviceID,
					Status:      domain.TripStatusActive,
					StartedAt:   backfillTime,
					StartLat:    startLat,
					StartLon:    startLon,
					DistanceM:   result.NewState.CandidateDistanceM,
					DurationSec: int64(duration.Seconds()),
					PointsCount: len(candidatePoints),
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				// Stamp TripID on all candidate points from this batch.
				batchCandidates := make([]domain.TripPoint, len(candidatePoints))
				for j, cp := range candidatePoints {
					cp.TripID = tripID
					batchCandidates[j] = cp
				}
				result.Commands = append(result.Commands, TripCommand{
					Kind:          CmdCreateTrip,
					TripID:        tripID,
					Trip:          &trip,
					BackfillSince: &backfillTime,
					Points:        batchCandidates,
				})
				result.NewState.State = domain.StateTripActive
				result.NewState.ActiveTripID = &tripID
				result.NewState.CandidateStartedAt = nil
				result.NewState.CandidateStartPointID = nil
				result.NewState.CandidateDistanceM = 0
				result.NewState.CandidateStartLat = nil
				result.NewState.CandidateStartLon = nil
				candidatePoints = candidatePoints[:0]
				// The current point is already in batchCandidates — do not add to activeTripPoints.
			}

		case domain.StateTripActive:
			if !isMoving {
				flushActive()
				t := pt.RecordedAt
				result.NewState.State = domain.StateStopCandidate
				result.NewState.StopStartedAt = &t
				result.NewState.StopCenterLat = &pt.Lat
				result.NewState.StopCenterLon = &pt.Lon
			} else {
				tp.TripID = *result.NewState.ActiveTripID
				activeTripPoints = append(activeTripPoints, tp)
				activeTripDeltaDistM += distFromPrev
				lastActivePoint = pt
			}

		case domain.StateStopCandidate:
			distFromCenter := haversineM(
				*result.NewState.StopCenterLat, *result.NewState.StopCenterLon,
				pt.Lat, pt.Lon,
			)
			if distFromCenter > u.cfg.StopRadiusM {
				// Moved out of stop zone: resume trip.
				result.NewState.State = domain.StateTripActive
				result.NewState.StopStartedAt = nil
				result.NewState.StopCenterLat = nil
				result.NewState.StopCenterLon = nil
				tp.TripID = *result.NewState.ActiveTripID
				activeTripPoints = append(activeTripPoints, tp)
				activeTripDeltaDistM += distFromPrev
				lastActivePoint = pt
			} else {
				stopDuration := pt.RecordedAt.Sub(*result.NewState.StopStartedAt)
				if int(stopDuration.Seconds()) >= u.cfg.StopMinDurationSec {
					// Long stop confirmed: complete the trip at stop_started_at.
					tripID := *result.NewState.ActiveTripID
					endedAt := *result.NewState.StopStartedAt
					endLat := *result.NewState.StopCenterLat
					endLon := *result.NewState.StopCenterLon
					result.Commands = append(result.Commands, TripCommand{
						Kind:    CmdCompleteTrip,
						TripID:  tripID,
						EndedAt: &endedAt,
						EndLat:  &endLat,
						EndLon:  &endLon,
					})
					result.NewState.State = domain.StateIdle
					result.NewState.ActiveTripID = nil
					result.NewState.StopStartedAt = nil
					result.NewState.StopCenterLat = nil
					result.NewState.StopCenterLon = nil
				}
				// else: stay in STOP_CANDIDATE
			}
		}

		// Update last accepted point.
		t := pt.RecordedAt
		result.NewState.LastProcessedAt = &t
		result.NewState.LastPointLat = &pt.Lat
		result.NewState.LastPointLon = &pt.Lon
	}

	// Flush any remaining active-trip accumulation at end of batch.
	if result.NewState.State == domain.StateTripActive {
		flushActive()
	}

	// Advance watermark: never move past now()-window so late-arriving points are still caught.
	newWatermark := now.Add(-time.Duration(result.NewState.LateArrivalWindowSec) * time.Second)
	result.NewState.LastWatermarkAt = &newWatermark
	result.NewState.UpdatedAt = now

	return result
}

// haversineM returns the great-circle distance in metres between two WGS-84 points.
func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthR = 6_371_000.0
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	return earthR * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
