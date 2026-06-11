package usecase_test

import (
	"context"
	"errors"
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

	capturedFilter domain.TrackingPointFilter
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
