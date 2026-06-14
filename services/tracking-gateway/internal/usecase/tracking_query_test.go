package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// stubTrackingQueryRepo is a minimal in-memory stub for tests.
type stubTrackingQueryRepo struct {
	points  []domain.TrackingPoint
	summary domain.TrackingSummary
	err     error

	capturedFilter      domain.TrackingPointFilter
	capturedTrackFilter domain.TrackFilter
	fastSegmentPoints   []domain.FastSegmentSourcePoint
	capturedFastFilter  domain.FastSegmentFilter
}

func (s *stubTrackingQueryRepo) GetPoints(
	_ context.Context, f domain.TrackingPointFilter,
) ([]domain.TrackingPoint, string, error) {
	s.capturedFilter = f
	return s.points, "", s.err
}

func (s *stubTrackingQueryRepo) GetSummary(
	_ context.Context, f domain.TrackingPointFilter,
) (domain.TrackingSummary, error) {
	s.capturedFilter = f
	return s.summary, s.err
}

func (s *stubTrackingQueryRepo) DeletePoint(_ context.Context, _, _ string) error {
	return s.err
}

func (s *stubTrackingQueryRepo) GetTrack(
	_ context.Context, f domain.TrackFilter,
) ([]domain.TrackSegment, error) {
	s.capturedTrackFilter = f
	return nil, s.err
}

func (s *stubTrackingQueryRepo) GetFastSegmentPoints(
	_ context.Context, f domain.FastSegmentFilter,
) ([]domain.FastSegmentSourcePoint, error) {
	s.capturedFastFilter = f
	return s.fastSegmentPoints, s.err
}

var (
	now  = time.Now().UTC()
	from = now.Add(-1 * time.Hour)
	to   = now
)

func TestGetPoints_LimitDefault(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, _, err := uc.GetPoints(context.Background(), usecase.GetPointsInput{
		UserID: "u1", From: from, To: to, Limit: 0,
	})
	require.NoError(t, err)
	assert.Equal(t, 1000, stub.capturedFilter.Limit)
}

func TestGetPoints_LimitCapped(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, _, err := uc.GetPoints(context.Background(), usecase.GetPointsInput{
		UserID: "u1", From: from, To: to, Limit: 9999,
	})
	require.NoError(t, err)
	assert.Equal(t, 5000, stub.capturedFilter.Limit)
}

func TestGetPoints_MissingFrom(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, _, err := uc.GetPoints(context.Background(), usecase.GetPointsInput{
		UserID: "u1", To: to,
	})
	assert.ErrorIs(t, err, usecase.ErrFromRequired)
}

func TestGetPoints_MissingTo(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, _, err := uc.GetPoints(context.Background(), usecase.GetPointsInput{
		UserID: "u1", From: from,
	})
	assert.ErrorIs(t, err, usecase.ErrToRequired)
}

func TestGetPoints_InvalidRange(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, _, err := uc.GetPoints(context.Background(), usecase.GetPointsInput{
		UserID: "u1", From: to, To: from,
	})
	assert.ErrorIs(t, err, usecase.ErrInvalidRange)
}

func TestGetPoints_RangeTooLarge(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, _, err := uc.GetPoints(context.Background(), usecase.GetPointsInput{
		UserID: "u1",
		From:   now.Add(-32 * 24 * time.Hour),
		To:     now,
	})
	assert.ErrorIs(t, err, usecase.ErrRangeTooLarge)
}

func TestGetPoints_PropagatesRepoError(t *testing.T) {
	repoErr := errors.New("db down")
	stub := &stubTrackingQueryRepo{err: repoErr}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, _, err := uc.GetPoints(context.Background(), usecase.GetPointsInput{
		UserID: "u1", From: from, To: to,
	})
	assert.ErrorIs(t, err, repoErr)
}

func TestGetSummary_ValidationErrors(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, err := uc.GetSummary(context.Background(), usecase.GetSummaryInput{
		UserID: "u1", To: to,
	})
	assert.ErrorIs(t, err, usecase.ErrFromRequired)

	_, err = uc.GetSummary(context.Background(), usecase.GetSummaryInput{
		UserID: "u1", From: from,
	})
	assert.ErrorIs(t, err, usecase.ErrToRequired)
}

func TestGetTrack_DefaultsMissingSettings(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)

	_, err := uc.GetTrack(context.Background(), usecase.GetTrackInput{
		UserID: "u1", From: from, To: to,
	})
	require.NoError(t, err)
	assert.Equal(t, 2.0, stub.capturedTrackFilter.SpeedThresholdMps)
	assert.Equal(t, 60, stub.capturedTrackFilter.MinStaySec)
	assert.Equal(t, 30, stub.capturedTrackFilter.MinMoveSec)
}

func TestGetTrack_AllowsZeroDurations(t *testing.T) {
	stub := &stubTrackingQueryRepo{}
	uc := usecase.NewTrackingQueryUsecase(stub)
	threshold, zero := 0.5, 0

	_, err := uc.GetTrack(context.Background(), usecase.GetTrackInput{
		UserID: "u1", From: from, To: to,
		SpeedThresholdMps: &threshold,
		MinStaySec:        &zero,
		MinMoveSec:        &zero,
	})
	require.NoError(t, err)
	assert.Equal(t, 0.5, stub.capturedTrackFilter.SpeedThresholdMps)
	assert.Zero(t, stub.capturedTrackFilter.MinStaySec)
	assert.Zero(t, stub.capturedTrackFilter.MinMoveSec)
}

func TestGetFastSegments_RanksContinuousFastRun(t *testing.T) {
	base := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	speeds := []float64{1, 1, 4, 4, 4, 1}
	points := make([]domain.FastSegmentSourcePoint, 0, len(speeds)+1)
	lon := 37.0
	points = append(points, domain.FastSegmentSourcePoint{
		DeviceID: "d1", EventID: "e0", RecordedAt: base,
		Lat: 55, Lon: lon, SegmentID: 1,
	})
	for i, speed := range speeds {
		lon += speed * 10 / (111320 * math.Cos(55*math.Pi/180))
		points = append(points, domain.FastSegmentSourcePoint{
			DeviceID: "d1", EventID: fmt.Sprintf("e%d", i+1),
			RecordedAt: base.Add(time.Duration(i+1) * 10 * time.Second),
			Lat:        55, Lon: lon, SegmentID: 1,
		})
	}
	stub := &stubTrackingQueryRepo{fastSegmentPoints: points}
	uc := usecase.NewTrackingQueryUsecase(stub)

	segments, err := uc.GetFastSegments(context.Background(), usecase.GetFastSegmentsInput{
		UserID: "u1", From: base.Add(-time.Minute), To: base.Add(time.Minute),
		Preset: domain.FastSegmentPresetNormal, Limit: 5,
	})
	require.NoError(t, err)
	require.Len(t, segments, 1)
	assert.Equal(t, 1, segments[0].Rank)
	assert.Equal(t, int64(30), segments[0].DurationSec)
	assert.InDelta(t, 4, segments[0].AvgSpeedMps, 0.02)
	assert.Len(t, segments[0].Points, 4)
}

func TestGetFastSegments_ValidatesOptions(t *testing.T) {
	uc := usecase.NewTrackingQueryUsecase(&stubTrackingQueryRepo{})

	_, err := uc.GetFastSegments(context.Background(), usecase.GetFastSegmentsInput{
		UserID: "u1", From: from, To: to, Preset: "unknown", Limit: 5,
	})
	assert.ErrorIs(t, err, usecase.ErrInvalidFastSegmentPreset)

	_, err = uc.GetFastSegments(context.Background(), usecase.GetFastSegmentsInput{
		UserID: "u1", From: from, To: to,
		Preset: domain.FastSegmentPresetSoft, Limit: 7,
	})
	assert.ErrorIs(t, err, usecase.ErrInvalidFastSegmentLimit)
}

func TestGetFastSegments_DoesNotCrossSegmentBreak(t *testing.T) {
	base := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	points := []domain.FastSegmentSourcePoint{
		{DeviceID: "d1", EventID: "e0", RecordedAt: base, Lat: 55, Lon: 37, SegmentID: 1},
		{DeviceID: "d1", EventID: "e1", RecordedAt: base.Add(10 * time.Second), Lat: 55, Lon: 37.001, SegmentID: 1},
		{DeviceID: "d1", EventID: "e2", RecordedAt: base.Add(20 * time.Second), Lat: 55, Lon: 37.002, SegmentID: 2},
		{DeviceID: "d1", EventID: "e3", RecordedAt: base.Add(30 * time.Second), Lat: 55, Lon: 37.003, SegmentID: 2},
	}
	uc := usecase.NewTrackingQueryUsecase(&stubTrackingQueryRepo{fastSegmentPoints: points})

	segments, err := uc.GetFastSegments(context.Background(), usecase.GetFastSegmentsInput{
		UserID: "u1", From: base.Add(-time.Minute), To: base.Add(time.Minute),
		Preset: domain.FastSegmentPresetSoft, Limit: 5,
	})
	require.NoError(t, err)
	assert.Empty(t, segments)
}
