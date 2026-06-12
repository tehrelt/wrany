package usecase

import (
	"context"
	"errors"

	"github.com/wrany/tracking-gateway/internal/domain"
)

const (
	defaultRoutesLimit = 50
	maxRoutesLimit     = 200
	defaultRTLimit     = 50
	maxRTLimit         = 200
	defaultPtsLimitR   = 5000
)

var (
	ErrRouteNotFound  = errors.New("route not found")
	ErrRouteForbidden = errors.New("route forbidden")
)

type RouteQueryRepo interface {
	ListRoutes(ctx context.Context, f domain.RouteFilter) ([]domain.Route, string, error)
	GetRoute(ctx context.Context, routeID, userID string) (domain.Route, error)
	ListRouteTrips(ctx context.Context, f domain.RouteTripFilter) ([]domain.RouteTrip, string, error)
	GetRoutePoints(ctx context.Context, routeID, userID string) ([]domain.RoutePoint, error)
}

type RouteQueryUsecase struct {
	repo RouteQueryRepo
}

func NewRouteQueryUsecase(repo RouteQueryRepo) *RouteQueryUsecase {
	return &RouteQueryUsecase{repo: repo}
}

type ListRoutesInput struct {
	UserID   string
	DeviceID string
	Limit    int
	Cursor   string
}

type ListRouteTripsInput struct {
	RouteID string
	UserID  string
	Limit   int
	Cursor  string
}

func (u *RouteQueryUsecase) ListRoutes(ctx context.Context, in ListRoutesInput) ([]domain.Route, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultRoutesLimit
	}
	if limit > maxRoutesLimit {
		limit = maxRoutesLimit
	}
	return u.repo.ListRoutes(ctx, domain.RouteFilter{
		UserID:   in.UserID,
		DeviceID: in.DeviceID,
		Limit:    limit,
		Cursor:   in.Cursor,
	})
}

func (u *RouteQueryUsecase) GetRoute(ctx context.Context, userID, routeID string) (domain.Route, error) {
	r, err := u.repo.GetRoute(ctx, routeID, userID)
	if errors.Is(err, ErrRouteNotFound) {
		return domain.Route{}, ErrRouteNotFound
	}
	return r, err
}

func (u *RouteQueryUsecase) ListRouteTrips(ctx context.Context, in ListRouteTripsInput) ([]domain.RouteTrip, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultRTLimit
	}
	if limit > maxRTLimit {
		limit = maxRTLimit
	}
	// Verify route belongs to user.
	if _, err := u.repo.GetRoute(ctx, in.RouteID, in.UserID); err != nil {
		if errors.Is(err, ErrRouteNotFound) {
			return nil, "", ErrRouteForbidden
		}
		return nil, "", err
	}
	return u.repo.ListRouteTrips(ctx, domain.RouteTripFilter{
		RouteID: in.RouteID,
		UserID:  in.UserID,
		Limit:   limit,
		Cursor:  in.Cursor,
	})
}

func (u *RouteQueryUsecase) GetRoutePoints(ctx context.Context, userID, routeID string) ([]domain.RoutePoint, error) {
	if _, err := u.repo.GetRoute(ctx, routeID, userID); err != nil {
		if errors.Is(err, ErrRouteNotFound) {
			return nil, ErrRouteForbidden
		}
		return nil, err
	}
	return u.repo.GetRoutePoints(ctx, routeID, userID)
}
