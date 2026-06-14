package usecase

import (
	"context"
	"errors"
	"math"
	"slices"
	"time"

	"github.com/wrany/tracking-gateway/internal/domain"
)

const (
	defaultPointsLimit = 1000
	maxPointsLimit     = 5000
	maxRangeDays       = 31
)

// TrackingQueryRepo is the storage interface for raw location points.
type TrackingQueryRepo interface {
	GetPoints(ctx context.Context, filter domain.TrackingPointFilter) ([]domain.TrackingPoint, string, error)
	GetSummary(ctx context.Context, filter domain.TrackingPointFilter) (domain.TrackingSummary, error)
	DeletePoint(ctx context.Context, userID, eventID string) error
	GetTrack(ctx context.Context, filter domain.TrackFilter) ([]domain.TrackSegment, error)
	GetFastSegmentPoints(ctx context.Context, filter domain.FastSegmentFilter) ([]domain.FastSegmentSourcePoint, error)
}

// TrackingQueryUsecase handles read queries for raw location points.
type TrackingQueryUsecase struct {
	repo TrackingQueryRepo
}

func NewTrackingQueryUsecase(repo TrackingQueryRepo) *TrackingQueryUsecase {
	return &TrackingQueryUsecase{repo: repo}
}

// GetPointsInput is the caller-supplied filter before normalization.
type GetPointsInput struct {
	UserID   string
	DeviceID string
	From     time.Time
	To       time.Time
	Limit    int
	Cursor   string
}

var (
	ErrFromRequired             = errors.New("from is required")
	ErrToRequired               = errors.New("to is required")
	ErrInvalidRange             = errors.New("from must be before to")
	ErrRangeTooLarge            = errors.New("time range must not exceed 31 days")
	ErrInvalidFastSegmentPreset = errors.New("preset must be soft, normal, or strict")
	ErrInvalidFastSegmentLimit  = errors.New("limit must be 5, 10, or 20")
)

func (u *TrackingQueryUsecase) GetPoints(ctx context.Context, in GetPointsInput) ([]domain.TrackingPoint, string, error) {
	if err := validateRange(in.From, in.To); err != nil {
		return nil, "", err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = defaultPointsLimit
	}
	if limit > maxPointsLimit {
		limit = maxPointsLimit
	}

	filter := domain.TrackingPointFilter{
		UserID:   in.UserID,
		DeviceID: in.DeviceID,
		From:     in.From,
		To:       in.To,
		Limit:    limit,
		Cursor:   in.Cursor,
	}
	return u.repo.GetPoints(ctx, filter)
}

// GetSummaryInput is the caller-supplied filter for summary queries.
type GetSummaryInput struct {
	UserID   string
	DeviceID string
	From     time.Time
	To       time.Time
}

func (u *TrackingQueryUsecase) GetSummary(ctx context.Context, in GetSummaryInput) (domain.TrackingSummary, error) {
	if err := validateRange(in.From, in.To); err != nil {
		return domain.TrackingSummary{}, err
	}

	filter := domain.TrackingPointFilter{
		UserID:   in.UserID,
		DeviceID: in.DeviceID,
		From:     in.From,
		To:       in.To,
	}
	return u.repo.GetSummary(ctx, filter)
}

// DeletePointInput identifies a single point to delete.
type DeletePointInput struct {
	UserID  string
	EventID string
}

func (u *TrackingQueryUsecase) DeletePoint(ctx context.Context, in DeletePointInput) error {
	return u.repo.DeletePoint(ctx, in.UserID, in.EventID)
}

const (
	defaultSpeedThresholdMps = 2.0
	defaultMinStaySec        = 60
	defaultMinMoveSec        = 30
)

// GetTrackInput is the caller-supplied filter for the simplified track query.
type GetTrackInput struct {
	UserID            string
	DeviceID          string
	From              time.Time
	To                time.Time
	SpeedThresholdMps *float64
	MinStaySec        *int
	MinMoveSec        *int
}

func (u *TrackingQueryUsecase) GetTrack(ctx context.Context, in GetTrackInput) ([]domain.TrackSegment, error) {
	if err := validateRange(in.From, in.To); err != nil {
		return nil, err
	}
	threshold := defaultSpeedThresholdMps
	if in.SpeedThresholdMps != nil && *in.SpeedThresholdMps >= 0 {
		threshold = *in.SpeedThresholdMps
	}
	minStay := defaultMinStaySec
	if in.MinStaySec != nil && *in.MinStaySec >= 0 {
		minStay = *in.MinStaySec
	}
	minMove := defaultMinMoveSec
	if in.MinMoveSec != nil && *in.MinMoveSec >= 0 {
		minMove = *in.MinMoveSec
	}
	return u.repo.GetTrack(ctx, domain.TrackFilter{
		UserID:            in.UserID,
		DeviceID:          in.DeviceID,
		From:              in.From,
		To:                in.To,
		SpeedThresholdMps: threshold,
		MinStaySec:        minStay,
		MinMoveSec:        minMove,
	})
}

type GetFastSegmentsInput struct {
	UserID   string
	DeviceID string
	From     time.Time
	To       time.Time
	Preset   domain.FastSegmentPreset
	Limit    int
}

type fastSegmentConfig struct {
	percentile  float64
	minDuration time.Duration
}

type speedEdge struct {
	from      domain.FastSegmentSourcePoint
	to        domain.FastSegmentSourcePoint
	distanceM float64
	duration  time.Duration
	speedMps  float64
}

func (u *TrackingQueryUsecase) GetFastSegments(
	ctx context.Context,
	in GetFastSegmentsInput,
) ([]domain.FastSegment, error) {
	if err := validateRange(in.From, in.To); err != nil {
		return nil, err
	}

	cfg, ok := fastSegmentPresetConfig(in.Preset)
	if !ok {
		return nil, ErrInvalidFastSegmentPreset
	}
	if in.Limit != 5 && in.Limit != 10 && in.Limit != 20 {
		return nil, ErrInvalidFastSegmentLimit
	}

	points, err := u.repo.GetFastSegmentPoints(ctx, domain.FastSegmentFilter{
		UserID: in.UserID, DeviceID: in.DeviceID, From: in.From, To: in.To,
	})
	if err != nil {
		return nil, err
	}

	edges := buildSpeedEdges(points)
	if len(edges) == 0 {
		return []domain.FastSegment{}, nil
	}

	speeds := make([]float64, 0, len(edges))
	for _, edge := range edges {
		speeds = append(speeds, edge.speedMps)
	}
	slices.Sort(speeds)
	baseline := percentile(speeds, 0.5)
	threshold := percentile(speeds, cfg.percentile)
	segments := collectFastSegments(edges, threshold, baseline, cfg.minDuration)
	slices.SortFunc(segments, func(a, b domain.FastSegment) int {
		if a.UpliftPercent > b.UpliftPercent {
			return -1
		}
		if a.UpliftPercent < b.UpliftPercent {
			return 1
		}
		if a.AvgSpeedMps > b.AvgSpeedMps {
			return -1
		}
		if a.AvgSpeedMps < b.AvgSpeedMps {
			return 1
		}
		return a.StartedAt.Compare(b.StartedAt)
	})
	if len(segments) > in.Limit {
		segments = segments[:in.Limit]
	}
	for i := range segments {
		segments[i].Rank = i + 1
	}
	return segments, nil
}

func fastSegmentPresetConfig(preset domain.FastSegmentPreset) (fastSegmentConfig, bool) {
	switch preset {
	case domain.FastSegmentPresetSoft:
		return fastSegmentConfig{percentile: 0.75, minDuration: 15 * time.Second}, true
	case domain.FastSegmentPresetNormal:
		return fastSegmentConfig{percentile: 0.85, minDuration: 20 * time.Second}, true
	case domain.FastSegmentPresetStrict:
		return fastSegmentConfig{percentile: 0.95, minDuration: 30 * time.Second}, true
	default:
		return fastSegmentConfig{}, false
	}
}

func buildSpeedEdges(points []domain.FastSegmentSourcePoint) []speedEdge {
	edges := make([]speedEdge, 0, len(points))
	for i := 1; i < len(points); i++ {
		from, to := points[i-1], points[i]
		if from.DeviceID != to.DeviceID || from.SegmentID != to.SegmentID {
			continue
		}
		duration := to.RecordedAt.Sub(from.RecordedAt)
		if duration <= 0 {
			continue
		}
		distance := haversineM(from.Lat, from.Lon, to.Lat, to.Lon)
		edges = append(edges, speedEdge{
			from: from, to: to, duration: duration,
			distanceM: distance, speedMps: distance / duration.Seconds(),
		})
	}
	return edges
}

func collectFastSegments(
	edges []speedEdge,
	threshold float64,
	baseline float64,
	minDuration time.Duration,
) []domain.FastSegment {
	segments := make([]domain.FastSegment, 0)
	run := make([]speedEdge, 0)

	flush := func() {
		if len(run) == 0 {
			return
		}
		duration := run[len(run)-1].to.RecordedAt.Sub(run[0].from.RecordedAt)
		if duration < minDuration {
			run = run[:0]
			return
		}
		distance := 0.0
		points := make([]domain.FastSegmentPoint, 0, len(run)+1)
		points = append(points, fastSegmentPoint(run[0].from))
		for _, edge := range run {
			distance += edge.distanceM
			points = append(points, fastSegmentPoint(edge.to))
		}
		avg := distance / duration.Seconds()
		uplift := 0.0
		if baseline > 0 {
			uplift = (avg/baseline - 1) * 100
		}
		segments = append(segments, domain.FastSegment{
			DeviceID:         run[0].from.DeviceID,
			StartedAt:        run[0].from.RecordedAt,
			EndedAt:          run[len(run)-1].to.RecordedAt,
			DurationSec:      int64(math.Round(duration.Seconds())),
			DistanceM:        distance,
			AvgSpeedMps:      avg,
			BaselineSpeedMps: baseline,
			UpliftPercent:    uplift,
			Points:           points,
		})
		run = run[:0]
	}

	for _, edge := range edges {
		continues := len(run) == 0 ||
			(run[len(run)-1].to.EventID == edge.from.EventID &&
				run[len(run)-1].to.DeviceID == edge.from.DeviceID &&
				run[len(run)-1].to.SegmentID == edge.from.SegmentID)
		if edge.speedMps < threshold || !continues {
			flush()
		}
		if edge.speedMps >= threshold {
			run = append(run, edge)
		}
	}
	flush()
	return segments
}

func fastSegmentPoint(point domain.FastSegmentSourcePoint) domain.FastSegmentPoint {
	return domain.FastSegmentPoint{
		Lat: point.Lat, Lon: point.Lon, RecordedAt: point.RecordedAt,
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := p * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	weight := position - float64(lower)
	return sorted[lower] + (sorted[upper]-sorted[lower])*weight
}

func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000.0
	toRad := math.Pi / 180
	phi1, phi2 := lat1*toRad, lat2*toRad
	dPhi := (lat2 - lat1) * toRad
	dLambda := (lon2 - lon1) * toRad
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func validateRange(from, to time.Time) error {
	if from.IsZero() {
		return ErrFromRequired
	}
	if to.IsZero() {
		return ErrToRequired
	}
	if !from.Before(to) {
		return ErrInvalidRange
	}
	if to.Sub(from) > maxRangeDays*24*time.Hour {
		return ErrRangeTooLarge
	}
	return nil
}
