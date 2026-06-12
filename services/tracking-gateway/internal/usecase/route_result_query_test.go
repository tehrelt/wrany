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

// --- mocks ---

type mockRouteResultRepo struct {
	result   domain.RouteResult
	attempts []domain.TripAttempt
	cursor   string
	err      error
}

func (m *mockRouteResultRepo) GetRouteResult(_ context.Context, _ string) (domain.RouteResult, error) {
	return m.result, m.err
}

func (m *mockRouteResultRepo) ListRouteAttempts(_ context.Context, _ domain.TripAttemptFilter) ([]domain.TripAttempt, string, error) {
	return m.attempts, m.cursor, m.err
}

type mockRouteQueryRepoForResult struct {
	route domain.Route
	err   error
}

func (m *mockRouteQueryRepoForResult) GetRoute(_ context.Context, _, _ string) (domain.Route, error) {
	return m.route, m.err
}

func (m *mockRouteQueryRepoForResult) ListRoutes(_ context.Context, _ domain.RouteFilter) ([]domain.Route, string, error) {
	return nil, "", nil
}

func (m *mockRouteQueryRepoForResult) ListRouteTrips(_ context.Context, _ domain.RouteTripFilter) ([]domain.RouteTrip, string, error) {
	return nil, "", nil
}

func (m *mockRouteQueryRepoForResult) GetRoutePoints(_ context.Context, _, _ string) ([]domain.RoutePoint, error) {
	return nil, nil
}

// --- helpers ---

func newResultUC(resultRepo usecase.RouteResultRepo, routeRepo usecase.RouteQueryRepo) *usecase.RouteResultQueryUsecase {
	return usecase.NewRouteResultQueryUsecase(resultRepo, routeRepo)
}

// --- tests ---

func TestRouteResultQueryUsecase_GetRouteResult_RouteNotFound(t *testing.T) {
	uc := newResultUC(
		&mockRouteResultRepo{},
		&mockRouteQueryRepoForResult{err: usecase.ErrRouteNotFound},
	)

	_, err := uc.GetRouteResult(context.Background(), usecase.GetRouteResultInput{
		UserID:  "user-1",
		RouteID: "unknown",
	})
	assert.ErrorIs(t, err, usecase.ErrRouteNotFound)
}

func TestRouteResultQueryUsecase_GetRouteResult_ZeroAttempts(t *testing.T) {
	uc := newResultUC(
		&mockRouteResultRepo{result: domain.RouteResult{RouteID: "r1", AttemptsCount: 0}},
		&mockRouteQueryRepoForResult{route: domain.Route{ID: "r1"}},
	)

	res, err := uc.GetRouteResult(context.Background(), usecase.GetRouteResultInput{
		UserID: "u1", RouteID: "r1",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.AttemptsCount)
	assert.Nil(t, res.Best)
	assert.Nil(t, res.Latest)
	assert.Nil(t, res.Comparison)
}

func TestRouteResultQueryUsecase_GetRouteResult_ComparisonComputed(t *testing.T) {
	t0 := time.Now().UTC()
	bestTrip := &domain.TripResult{TripID: "t1", StartedAt: t0.Add(-10 * time.Minute), DurationSec: 600}
	latestTrip := &domain.TripResult{TripID: "t2", StartedAt: t0, DurationSec: 660}

	uc := newResultUC(
		&mockRouteResultRepo{result: domain.RouteResult{
			RouteID: "r1", AttemptsCount: 2, Best: bestTrip, Latest: latestTrip,
		}},
		&mockRouteQueryRepoForResult{route: domain.Route{ID: "r1"}},
	)

	res, err := uc.GetRouteResult(context.Background(), usecase.GetRouteResultInput{
		UserID: "u1", RouteID: "r1",
	})
	require.NoError(t, err)
	require.NotNil(t, res.Comparison)
	assert.Equal(t, int64(60), res.Comparison.LatestVsBestSec)
	assert.InDelta(t, 10.0, res.Comparison.LatestVsBestPercent, 0.01)
}

func TestRouteResultQueryUsecase_GetRouteResult_LatestIsBest(t *testing.T) {
	t0 := time.Now().UTC()
	sameTrip := &domain.TripResult{TripID: "t1", StartedAt: t0, DurationSec: 500}

	uc := newResultUC(
		&mockRouteResultRepo{result: domain.RouteResult{
			RouteID: "r1", AttemptsCount: 1, Best: sameTrip, Latest: sameTrip,
		}},
		&mockRouteQueryRepoForResult{route: domain.Route{ID: "r1"}},
	)

	res, err := uc.GetRouteResult(context.Background(), usecase.GetRouteResultInput{
		UserID: "u1", RouteID: "r1",
	})
	require.NoError(t, err)
	require.NotNil(t, res.Comparison)
	assert.Equal(t, int64(0), res.Comparison.LatestVsBestSec)
	assert.InDelta(t, 0.0, res.Comparison.LatestVsBestPercent, 0.001)
}

func TestRouteResultQueryUsecase_GetRouteResult_DivByZeroGuard(t *testing.T) {
	// If best.DurationSec == 0 (shouldn't happen in practice, but guard exists)
	t0 := time.Now().UTC()
	bestTrip := &domain.TripResult{TripID: "t1", StartedAt: t0, DurationSec: 0}
	latestTrip := &domain.TripResult{TripID: "t2", StartedAt: t0, DurationSec: 0}

	uc := newResultUC(
		&mockRouteResultRepo{result: domain.RouteResult{
			RouteID: "r1", AttemptsCount: 2, Best: bestTrip, Latest: latestTrip,
		}},
		&mockRouteQueryRepoForResult{route: domain.Route{ID: "r1"}},
	)

	res, err := uc.GetRouteResult(context.Background(), usecase.GetRouteResultInput{
		UserID: "u1", RouteID: "r1",
	})
	require.NoError(t, err)
	require.NotNil(t, res.Comparison)
	assert.Equal(t, int64(0), res.Comparison.LatestVsBestSec)
	assert.InDelta(t, 0.0, res.Comparison.LatestVsBestPercent, 0.001)
}

func TestRouteResultQueryUsecase_ListRouteAttempts_RouteNotFound(t *testing.T) {
	uc := newResultUC(
		&mockRouteResultRepo{},
		&mockRouteQueryRepoForResult{err: usecase.ErrRouteNotFound},
	)

	_, _, err := uc.ListRouteAttempts(context.Background(), usecase.ListRouteAttemptsInput{
		UserID: "u1", RouteID: "unknown", Limit: 10,
	})
	assert.ErrorIs(t, err, usecase.ErrRouteForbidden)
}

func TestRouteResultQueryUsecase_ListRouteAttempts_RepoError(t *testing.T) {
	repoErr := errors.New("db failure")
	uc := newResultUC(
		&mockRouteResultRepo{err: repoErr},
		&mockRouteQueryRepoForResult{route: domain.Route{ID: "r1"}},
	)

	_, _, err := uc.ListRouteAttempts(context.Background(), usecase.ListRouteAttemptsInput{
		UserID: "u1", RouteID: "r1", Limit: 10,
	})
	assert.ErrorIs(t, err, repoErr)
}

func TestRouteResultQueryUsecase_ListRouteAttempts_LimitClamped(t *testing.T) {
	captured := domain.TripAttemptFilter{}
	repo := &captureAttemptsRepo{captured: &captured}

	uc := newResultUC(repo, &mockRouteQueryRepoForResult{route: domain.Route{ID: "r1"}})

	_, _, _ = uc.ListRouteAttempts(context.Background(), usecase.ListRouteAttemptsInput{
		UserID: "u1", RouteID: "r1", Limit: 9999,
	})
	assert.Equal(t, 200, captured.Limit)
}

func TestRouteResultQueryUsecase_ListRouteAttempts_DefaultLimit(t *testing.T) {
	captured := domain.TripAttemptFilter{}
	repo := &captureAttemptsRepo{captured: &captured}

	uc := newResultUC(repo, &mockRouteQueryRepoForResult{route: domain.Route{ID: "r1"}})

	_, _, _ = uc.ListRouteAttempts(context.Background(), usecase.ListRouteAttemptsInput{
		UserID: "u1", RouteID: "r1", Limit: 0,
	})
	assert.Equal(t, 50, captured.Limit)
}

// captureAttemptsRepo captures the filter passed to ListRouteAttempts.
type captureAttemptsRepo struct {
	captured *domain.TripAttemptFilter
}

func (r *captureAttemptsRepo) GetRouteResult(_ context.Context, _ string) (domain.RouteResult, error) {
	return domain.RouteResult{}, nil
}

func (r *captureAttemptsRepo) ListRouteAttempts(_ context.Context, f domain.TripAttemptFilter) ([]domain.TripAttempt, string, error) {
	*r.captured = f
	return nil, "", nil
}
