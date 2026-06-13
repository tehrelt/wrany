package usecase

import (
	"context"
	"errors"

	"github.com/wrany/tracking-gateway/internal/domain"
)

const (
	defaultTripsLimit = 20
	maxTripsLimit     = 100
	defaultPtsLimit   = 500
	maxPtsLimit       = 5000
)

var (
	ErrTripNotFound  = errors.New("trip not found")
	ErrTripForbidden = errors.New("trip not found") // same message — no information leakage
)

// TripQueryRepo is the storage interface for trips.
type TripQueryRepo interface {
	ListTrips(ctx context.Context, f domain.TripFilter) ([]domain.Trip, string, error)
	GetTrip(ctx context.Context, userID, tripID string) (domain.Trip, error)
	GetTripPoints(ctx context.Context, f domain.TripPointFilter) ([]domain.TripPoint, string, error)
	DeleteTrip(ctx context.Context, userID, tripID string) error
}

// TripQueryUsecase handles read queries for trips.
type TripQueryUsecase struct {
	repo TripQueryRepo
}

func NewTripQueryUsecase(repo TripQueryRepo) *TripQueryUsecase {
	return &TripQueryUsecase{repo: repo}
}

// ListTripsInput is the caller-supplied filter.
type ListTripsInput struct {
	UserID   string
	DeviceID string
	Status   string
	Limit    int
	Cursor   string
}

func (u *TripQueryUsecase) ListTrips(ctx context.Context, in ListTripsInput) ([]domain.Trip, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultTripsLimit
	}
	if limit > maxTripsLimit {
		limit = maxTripsLimit
	}

	var status domain.TripStatus
	if in.Status != "" {
		status = domain.TripStatus(in.Status)
	}

	return u.repo.ListTrips(ctx, domain.TripFilter{
		UserID:   in.UserID,
		DeviceID: in.DeviceID,
		Status:   status,
		Limit:    limit,
		Cursor:   in.Cursor,
	})
}

// GetTripInput identifies a single trip.
type GetTripInput struct {
	UserID string
	TripID string
}

func (u *TripQueryUsecase) GetTrip(ctx context.Context, in GetTripInput) (domain.Trip, error) {
	return u.repo.GetTrip(ctx, in.UserID, in.TripID)
}

// DeleteTrip removes a trip owned by the user. Cascades to trip points, route
// matches, and any routes seeded by this trip. Returns ErrTripNotFound if the
// trip does not exist or is not owned by the user.
func (u *TripQueryUsecase) DeleteTrip(ctx context.Context, in GetTripInput) error {
	return u.repo.DeleteTrip(ctx, in.UserID, in.TripID)
}

// GetTripPointsInput is the caller-supplied filter for trip points.
type GetTripPointsInput struct {
	UserID string
	TripID string
	Limit  int
	Cursor string
}

func (u *TripQueryUsecase) GetTripPoints(ctx context.Context, in GetTripPointsInput) ([]domain.TripPoint, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultPtsLimit
	}
	if limit > maxPtsLimit {
		limit = maxPtsLimit
	}

	return u.repo.GetTripPoints(ctx, domain.TripPointFilter{
		TripID: in.TripID,
		UserID: in.UserID,
		Limit:  limit,
		Cursor: in.Cursor,
	})
}
