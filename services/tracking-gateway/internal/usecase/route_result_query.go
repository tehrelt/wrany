package usecase

import (
	"context"
	"errors"

	"github.com/wrany/tracking-gateway/internal/domain"
)

const (
	defaultAttemptsLimit = 50
	maxAttemptsLimit     = 200
)

// RouteResultRepo fetches personal records data from storage.
type RouteResultRepo interface {
	GetRouteResult(ctx context.Context, routeID string) (domain.RouteResult, error)
	ListRouteAttempts(ctx context.Context, f domain.TripAttemptFilter) ([]domain.TripAttempt, string, error)
}

// RouteResultQueryUsecase handles personal records queries.
type RouteResultQueryUsecase struct {
	resultRepo RouteResultRepo
	routeRepo  RouteQueryRepo // reused for ownership check
}

func NewRouteResultQueryUsecase(resultRepo RouteResultRepo, routeRepo RouteQueryRepo) *RouteResultQueryUsecase {
	return &RouteResultQueryUsecase{resultRepo: resultRepo, routeRepo: routeRepo}
}

// GetRouteResultInput is the input for GetRouteResult.
type GetRouteResultInput struct {
	UserID  string
	RouteID string
}

// ListRouteAttemptsInput is the input for ListRouteAttempts.
type ListRouteAttemptsInput struct {
	UserID  string
	RouteID string
	Limit   int
	Cursor  string
}

// GetRouteResult returns personal records for a route owned by the given user.
func (u *RouteResultQueryUsecase) GetRouteResult(ctx context.Context, in GetRouteResultInput) (domain.RouteResult, error) {
	if _, err := u.routeRepo.GetRoute(ctx, in.RouteID, in.UserID); err != nil {
		if errors.Is(err, ErrRouteNotFound) {
			return domain.RouteResult{}, ErrRouteNotFound
		}
		return domain.RouteResult{}, err
	}

	result, err := u.resultRepo.GetRouteResult(ctx, in.RouteID)
	if err != nil {
		return domain.RouteResult{}, err
	}

	result.Comparison = computeComparison(result.Best, result.Latest)
	return result, nil
}

// ListRouteAttempts returns a paginated list of attempts for a route.
func (u *RouteResultQueryUsecase) ListRouteAttempts(ctx context.Context, in ListRouteAttemptsInput) ([]domain.TripAttempt, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultAttemptsLimit
	}
	if limit > maxAttemptsLimit {
		limit = maxAttemptsLimit
	}

	if _, err := u.routeRepo.GetRoute(ctx, in.RouteID, in.UserID); err != nil {
		if errors.Is(err, ErrRouteNotFound) {
			return nil, "", ErrRouteForbidden
		}
		return nil, "", err
	}

	return u.resultRepo.ListRouteAttempts(ctx, domain.TripAttemptFilter{
		RouteID: in.RouteID,
		UserID:  in.UserID,
		Limit:   limit,
		Cursor:  in.Cursor,
	})
}

// computeComparison derives the latest-vs-best comparison; returns nil when best or latest is absent.
func computeComparison(best, latest *domain.TripResult) *domain.RouteResultComparison {
	if best == nil || latest == nil {
		return nil
	}
	diff := latest.DurationSec - best.DurationSec
	var pct float64
	if best.DurationSec > 0 {
		pct = float64(diff) / float64(best.DurationSec) * 100
	}
	return &domain.RouteResultComparison{
		LatestVsBestSec:     diff,
		LatestVsBestPercent: pct,
	}
}
