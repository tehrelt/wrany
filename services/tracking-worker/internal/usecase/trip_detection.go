package usecase

import (
	"time"

	"github.com/google/uuid"
	"github.com/wrany/tracking-worker/internal/domain"
	"github.com/wrany/tracking-worker/internal/usecase/noise"
)

type CommandKind int8

const (
	CmdCreateTrip CommandKind = iota
	CmdUpdateTrip
	CmdCompleteTrip
)

type TripCommand struct {
	Kind   CommandKind
	TripID uuid.UUID
	Trip   *domain.Trip

	BackfillSince *time.Time

	DeltaDistanceM   float64
	DeltaPointsCount int
	LastPointAt      *time.Time
	LastPointLat     *float64
	LastPointLon     *float64

	EndedAt *time.Time
	EndLat  *float64
	EndLon  *float64
	Points  []domain.TripPoint
}

type ProcessBatchResult struct {
	NewState domain.TripDetectionState
	Commands []TripCommand
}

type TripDetectionUseCase struct {
	cfg      domain.TripDetectionConfig
	movement noise.MovementWindowAnalyzer
}

func NewTripDetectionUseCase(cfg domain.TripDetectionConfig) *TripDetectionUseCase {
	noiseCfg := domain.DefaultNoiseConfig()
	noiseCfg.GoodAccuracyM = 30
	noiseCfg.MovementMinSpeedMps = cfg.MovementMinSpeedMps
	noiseCfg.RunningMaxSpeedMps = cfg.MovementMaxSpeedMps
	noiseCfg.MovementGoodPoints = cfg.MotionMinGoodPoints
	noiseCfg.ActivityConfidence = cfg.ActivityConfidence
	return &TripDetectionUseCase{
		cfg:      cfg,
		movement: noise.WindowMovementAnalyzer{Config: noiseCfg},
	}
}

func (u *TripDetectionUseCase) ProcessBatch(
	state domain.TripDetectionState,
	points []domain.ProcessedLocationPoint,
	now time.Time,
) ProcessBatchResult {
	result := ProcessBatchResult{NewState: state}
	var candidateWindow []domain.ProcessedLocationPoint
	var candidatePoints []domain.TripPoint
	var activePoints []domain.TripPoint
	var activeDistance float64
	var lastActive *domain.ProcessedLocationPoint

	flushActive := func() {
		if len(activePoints) == 0 {
			return
		}
		command := TripCommand{
			Kind: CmdUpdateTrip, TripID: *result.NewState.ActiveTripID,
			DeltaDistanceM: activeDistance, DeltaPointsCount: len(activePoints),
			Points: append([]domain.TripPoint(nil), activePoints...),
		}
		if lastActive != nil {
			lat, lon, _ := lastActive.Coordinates()
			at := lastActive.RecordedAt
			command.LastPointAt = &at
			command.LastPointLat = &lat
			command.LastPointLon = &lon
		}
		result.Commands = append(result.Commands, command)
		activePoints = nil
		activeDistance = 0
		lastActive = nil
	}

	for i := range points {
		point := &points[i]
		if !point.IsAccepted {
			continue
		}
		lat, lon, ok := point.Coordinates()
		if !ok {
			continue
		}
		tp := domain.TripPoint{
			UserID: point.UserID, DeviceID: point.DeviceID,
			EventID: point.EventID, RecordedAt: point.RecordedAt,
		}
		moving := point.IsMovementEvidence(
			u.cfg.MovementMinSpeedMps,
			u.cfg.MovementMaxSpeedMps,
			u.cfg.ActivityConfidence,
		)

		switch result.NewState.State {
		case domain.StateIdle:
			if moving {
				startCandidate(&result.NewState, point, lat, lon)
				candidateWindow = append(candidateWindow, *point)
				candidatePoints = append(candidatePoints, tp)
			}

		case domain.StateMotionCandidate:
			// A real stop, or a long gap since the previous point, abandons the
			// unconfirmed candidate. An isolated jitter point does NOT: we simply
			// don't count it and wait for the next point, so genuine movement
			// peppered with GPS noise still confirms with the correct start time.
			gapExceeded := result.NewState.LastProcessedAt != nil &&
				point.RecordedAt.Sub(*result.NewState.LastProcessedAt) >
					time.Duration(u.cfg.StopMinDurationSec)*time.Second
			if point.IsStationary || gapExceeded {
				resetCandidate(&result.NewState)
				candidateWindow = nil
				candidatePoints = nil
				if moving { // this point may itself open a fresh candidate
					startCandidate(&result.NewState, point, lat, lon)
					candidateWindow = append(candidateWindow, *point)
					candidatePoints = append(candidatePoints, tp)
				}
				break
			}

			candidateWindow = append(candidateWindow, *point)
			candidatePoints = append(candidatePoints, tp)
			if moving {
				result.NewState.CandidateDistanceM += point.DistanceDeltaM
				result.NewState.CandidateGoodPoints++
			}

			duration := point.RecordedAt.Sub(*result.NewState.CandidateStartedAt)
			enoughWindow := u.movement.Analyze(candidateWindow) ||
				result.NewState.CandidateGoodPoints >= u.cfg.MotionMinGoodPoints
			if duration >= time.Duration(u.cfg.MotionMinDurationSec)*time.Second &&
				result.NewState.CandidateDistanceM >= u.cfg.MotionMinDistanceM &&
				enoughWindow {
				tripID := uuid.New()
				startedAt := *result.NewState.CandidateStartedAt
				trip := &domain.Trip{
					ID: tripID, UserID: point.UserID, DeviceID: point.DeviceID,
					Status: domain.TripStatusActive, StartedAt: startedAt,
					StartLat:    *result.NewState.CandidateStartLat,
					StartLon:    *result.NewState.CandidateStartLon,
					DistanceM:   result.NewState.CandidateDistanceM,
					DurationSec: int64(duration.Seconds()),
					PointsCount: result.NewState.CandidateGoodPoints,
					CreatedAt:   now, UpdatedAt: now,
				}
				for index := range candidatePoints {
					candidatePoints[index].TripID = tripID
				}
				result.Commands = append(result.Commands, TripCommand{
					Kind: CmdCreateTrip, TripID: tripID, Trip: trip,
					BackfillSince: &startedAt, Points: candidatePoints,
				})
				result.NewState.State = domain.StateTripActive
				result.NewState.ActiveTripID = &tripID
				resetCandidate(&result.NewState)
				candidateWindow = nil
				candidatePoints = nil
			}

		case domain.StateTripActive:
			if point.IsStationary {
				flushActive()
				at := point.RecordedAt
				if point.StationarySince != nil {
					at = *point.StationarySince
				}
				result.NewState.State = domain.StateStopCandidate
				result.NewState.StopStartedAt = &at
				result.NewState.StopCenterLat = &lat
				result.NewState.StopCenterLon = &lon
			} else {
				tp.TripID = *result.NewState.ActiveTripID
				activePoints = append(activePoints, tp)
				activeDistance += point.DistanceDeltaM
				lastActive = point
			}

		case domain.StateStopCandidate:
			distanceFromCenter := noise.HaversineM(
				*result.NewState.StopCenterLat, *result.NewState.StopCenterLon,
				lat, lon,
			)
			if !point.IsStationary && distanceFromCenter > u.cfg.StopRadiusM {
				result.NewState.State = domain.StateTripActive
				result.NewState.StopStartedAt = nil
				result.NewState.StopCenterLat = nil
				result.NewState.StopCenterLon = nil
				tp.TripID = *result.NewState.ActiveTripID
				activePoints = append(activePoints, tp)
				activeDistance += point.DistanceDeltaM
				lastActive = point
			} else if point.RecordedAt.Sub(*result.NewState.StopStartedAt) >= time.Duration(u.cfg.StopMinDurationSec)*time.Second {
				tripID := *result.NewState.ActiveTripID
				endedAt := *result.NewState.StopStartedAt
				endLat, endLon := *result.NewState.StopCenterLat, *result.NewState.StopCenterLon
				result.Commands = append(result.Commands, TripCommand{
					Kind: CmdCompleteTrip, TripID: tripID,
					EndedAt: &endedAt, EndLat: &endLat, EndLon: &endLon,
				})
				result.NewState.State = domain.StateIdle
				result.NewState.ActiveTripID = nil
				result.NewState.StopStartedAt = nil
				result.NewState.StopCenterLat = nil
				result.NewState.StopCenterLon = nil
			}
		}

		at := point.RecordedAt
		result.NewState.LastProcessedAt = &at
		result.NewState.LastPointLat = &lat
		result.NewState.LastPointLon = &lon
	}

	if result.NewState.State == domain.StateTripActive {
		flushActive()
	}
	watermark := now.Add(-time.Duration(result.NewState.LateArrivalWindowSec) * time.Second)
	result.NewState.LastWatermarkAt = &watermark
	result.NewState.UpdatedAt = now
	return result
}

// startCandidate opens a fresh motion candidate anchored at point.
func startCandidate(state *domain.TripDetectionState, point *domain.ProcessedLocationPoint, lat, lon float64) {
	at, id := point.RecordedAt, point.EventID
	state.State = domain.StateMotionCandidate
	state.CandidateStartedAt = &at
	state.CandidateStartPointID = &id
	state.CandidateStartLat = &lat
	state.CandidateStartLon = &lon
	state.CandidateDistanceM = 0
	state.CandidateGoodPoints = 1
}

func resetCandidate(state *domain.TripDetectionState) {
	state.CandidateStartedAt = nil
	state.CandidateStartPointID = nil
	state.CandidateDistanceM = 0
	state.CandidateStartLat = nil
	state.CandidateStartLon = nil
	state.CandidateGoodPoints = 0
	if state.State == domain.StateMotionCandidate {
		state.State = domain.StateIdle
	}
}
