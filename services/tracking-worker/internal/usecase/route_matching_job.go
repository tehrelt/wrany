package usecase

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/wrany/tracking-worker/internal/domain"
	routeevents "github.com/wrany/libs/events/route"
	"github.com/wrany/libs/eventbus"
)

// RouteMatchingRepository is the storage interface required by RouteMatchingJob.
type RouteMatchingRepository interface {
	FindUnmatchedTrips(ctx context.Context, minPoints, limit int) ([]domain.Trip, error)
	FindTripPointsWithCoords(ctx context.Context, tripID uuid.UUID) ([]domain.GeoPoint, error)
	FindCandidateRoutes(ctx context.Context, userID uuid.UUID, startLat, startLon, radiusM float64) ([]domain.Route, error)
	FindRouteTemplate(ctx context.Context, routeID uuid.UUID) ([]domain.GeoPoint, error)
	InsertRoute(ctx context.Context, r domain.Route) error
	InsertRouteTrip(ctx context.Context, rt domain.RouteTrip) error
	IncrRouteStats(ctx context.Context, routeID uuid.UUID, lastTripID uuid.UUID) error
}

// RouteMatchingJob runs periodically and assigns completed trips to routes.
type RouteMatchingJob struct {
	repo     RouteMatchingRepository
	uc       *RouteMatchingUseCase
	pub      eventbus.Publisher
	producer string
}

func NewRouteMatchingJob(
	repo RouteMatchingRepository,
	pub eventbus.Publisher,
	producer string,
	cfg domain.RouteMatchConfig,
) *RouteMatchingJob {
	return &RouteMatchingJob{
		repo:     repo,
		uc:       NewRouteMatchingUseCase(cfg),
		pub:      pub,
		producer: producer,
	}
}

const routeMatchBatchSize = 50

func (j *RouteMatchingJob) RunOnce(ctx context.Context) error {
	trips, err := j.repo.FindUnmatchedTrips(ctx, j.uc.cfg.MinTripPoints, routeMatchBatchSize)
	if err != nil {
		return err
	}
	for _, trip := range trips {
		if err := j.processTrip(ctx, trip); err != nil {
			log.Printf("route_matching_job: trip %s: %v", trip.ID, err)
		}
	}
	return nil
}

func (j *RouteMatchingJob) processTrip(ctx context.Context, trip domain.Trip) error {
	tripPoints, err := j.repo.FindTripPointsWithCoords(ctx, trip.ID)
	if err != nil {
		return err
	}
	if len(tripPoints) < j.uc.cfg.MinTripPoints {
		return nil
	}

	startLat, startLon := trip.StartLat, trip.StartLon
	candidates, err := j.repo.FindCandidateRoutes(ctx, trip.UserID, startLat, startLon, j.uc.cfg.StartRadiusM)
	if err != nil {
		return err
	}

	templateByID := make(map[string][]domain.GeoPoint, len(candidates))
	for _, c := range candidates {
		tpl, err := j.repo.FindRouteTemplate(ctx, c.ID)
		if err != nil {
			log.Printf("route_matching_job: load template %s: %v", c.ID, err)
			continue
		}
		templateByID[c.ID.String()] = tpl
	}

	match := j.uc.FindBestMatch(trip, tripPoints, candidates, templateByID)

	now := time.Now()
	var routeID uuid.UUID
	var score float64

	if match.RouteID == "" {
		// No match → create new route from this trip.
		routeID = uuid.New()
		score = 1.0
		endLat, endLon := 0.0, 0.0
		if trip.EndLat != nil {
			endLat = *trip.EndLat
		}
		if trip.EndLon != nil {
			endLon = *trip.EndLon
		}
		deviceID := &trip.DeviceID
		newRoute := domain.Route{
			ID:          routeID,
			UserID:      trip.UserID,
			DeviceID:    deviceID,
			Status:      "active",
			StartLat:    startLat,
			StartLon:    startLon,
			EndLat:      endLat,
			EndLon:      endLon,
			DistanceM:   trip.DistanceM,
			TripsCount:  1,
			Template:    tripPoints,
			FirstTripID: trip.ID,
			LastTripID:  trip.ID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := j.repo.InsertRoute(ctx, newRoute); err != nil {
			return err
		}
	} else {
		routeID, err = uuid.Parse(match.RouteID)
		if err != nil {
			return err
		}
		score = match.Score
		if err := j.repo.IncrRouteStats(ctx, routeID, trip.ID); err != nil {
			return err
		}
	}

	rt := domain.RouteTrip{
		RouteID:     routeID,
		TripID:      trip.ID,
		UserID:      trip.UserID,
		DeviceID:    trip.DeviceID,
		MatchScore:  score,
		MatchedAt:   now,
		DurationSec: trip.DurationSec,
		DistanceM:   trip.DistanceM,
	}
	if err := j.repo.InsertRouteTrip(ctx, rt); err != nil {
		return err
	}

	j.publishMatched(ctx, trip.ID.String(), routeID.String(), trip.UserID.String(), score, now)
	return nil
}

func (j *RouteMatchingJob) publishMatched(ctx context.Context, tripID, routeID, userID string, score float64, now time.Time) {
	score = math.Max(0, math.Min(1, score))
	ev, err := routeevents.NewMatchedEvent(
		uuid.New().String(), now, j.producer, tripID,
		routeevents.MatchedPayload{
			TripID:     tripID,
			RouteID:    routeID,
			UserID:     userID,
			MatchedAt:  now,
			MatchScore: score,
		},
	)
	if err != nil {
		log.Printf("route_matching_job: build matched event: %v", err)
		return
	}
	if err := j.pub.Publish(ctx, "route.matched.v1", ev); err != nil {
		log.Printf("route_matching_job: publish matched event: %v", err)
	}
}
