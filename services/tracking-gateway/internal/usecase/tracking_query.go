package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/wrany/tracking-gateway/internal/domain"
)

const (
	defaultPointsLimit = 1000
	maxPointsLimit     = 5000
	maxRangeDays       = 31
)

// TrackingQueryRepo is the read-only storage interface for raw location points.
type TrackingQueryRepo interface {
	GetPoints(ctx context.Context, filter domain.TrackingPointFilter) ([]domain.TrackingPoint, string, error)
	GetSummary(ctx context.Context, filter domain.TrackingPointFilter) (domain.TrackingSummary, error)
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
	ErrFromRequired    = errors.New("from is required")
	ErrToRequired      = errors.New("to is required")
	ErrInvalidRange    = errors.New("from must be before to")
	ErrRangeTooLarge   = errors.New("time range must not exceed 31 days")
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
