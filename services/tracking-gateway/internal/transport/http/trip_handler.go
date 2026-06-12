package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

func tripToItem(t domain.Trip) TripItem {
	item := TripItem{
		ID:          t.ID,
		UserID:      t.UserID,
		DeviceID:    t.DeviceID,
		Status:      string(t.Status),
		StartedAt:   t.StartedAt.UTC().Format(time.RFC3339),
		StartLat:    t.StartLat,
		StartLon:    t.StartLon,
		EndLat:      t.EndLat,
		EndLon:      t.EndLon,
		DistanceM:   t.DistanceM,
		DurationSec: t.DurationSec,
		PointsCount: t.PointsCount,
		CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if t.EndedAt != nil {
		s := t.EndedAt.UTC().Format(time.RFC3339)
		item.EndedAt = &s
	}
	return item
}

// TripHandler handles GET /v1/trips, GET /v1/trips/{id}, GET /v1/trips/{id}/points.
type TripHandler struct {
	uc *usecase.TripQueryUsecase
}

func NewTripHandler(uc *usecase.TripQueryUsecase) *TripHandler {
	return &TripHandler{uc: uc}
}

// ListTrips godoc
// @Summary      List detected trips for the authenticated user
// @Tags         trips
// @Produce      json
// @Param        status     query  string  false  "Filter by status: TRIP_ACTIVE or TRIP_COMPLETED"
// @Param        device_id  query  string  false  "Filter by device UUID"
// @Param        limit      query  int     false  "Max results per page (default 20, max 100)"
// @Param        cursor     query  string  false  "Pagination cursor from previous response"
// @Success      200  {object}  TripListEnv
// @Failure      401  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/trips [get]
func (h *TripHandler) ListTrips(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	trips, nextCursor, err := h.uc.ListTrips(r.Context(), usecase.ListTripsInput{
		UserID:   userID.String(),
		DeviceID: q.Get("device_id"),
		Status:   q.Get("status"),
		Limit:    limit,
		Cursor:   q.Get("cursor"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]TripItem, 0, len(trips))
	for _, t := range trips {
		items = append(items, tripToItem(t))
	}

	var cursor *string
	if nextCursor != "" {
		cursor = &nextCursor
	}

	writeJSON(w, http.StatusOK, TripListResponse{Items: items, NextCursor: cursor})
}

// GetTrip godoc
// @Summary      Get a single trip by ID
// @Tags         trips
// @Produce      json
// @Param        id  path  string  true  "Trip UUID"
// @Success      200  {object}  TripEnv
// @Failure      401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/trips/{id} [get]
func (h *TripHandler) GetTrip(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tripID := r.PathValue("id")
	if tripID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	trip, err := h.uc.GetTrip(r.Context(), usecase.GetTripInput{
		UserID: userID.String(),
		TripID: tripID,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrTripNotFound) {
			writeError(w, http.StatusNotFound, "trip not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, tripToItem(trip))
}

// GetTripPoints godoc
// @Summary      Get GPS points for a trip (polyline)
// @Tags         trips
// @Produce      json
// @Param        id      path   string  true   "Trip UUID"
// @Param        limit   query  int     false  "Max results per page (default 500, max 5000)"
// @Param        cursor  query  string  false  "Pagination cursor from previous response"
// @Success      200  {object}  TripPointsEnv
// @Failure      401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/trips/{id}/points [get]
func (h *TripHandler) GetTripPoints(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tripID := r.PathValue("id")
	if tripID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	pts, nextCursor, err := h.uc.GetTripPoints(r.Context(), usecase.GetTripPointsInput{
		UserID: userID.String(),
		TripID: tripID,
		Limit:  limit,
		Cursor: q.Get("cursor"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]TripPointItem, 0, len(pts))
	for _, p := range pts {
		items = append(items, TripPointItem{
			EventID:    p.EventID,
			TripID:     p.TripID,
			RecordedAt: p.RecordedAt.UTC().Format(time.RFC3339),
			Lat:        p.Lat,
			Lon:        p.Lon,
		})
	}

	var cursor *string
	if nextCursor != "" {
		cursor = &nextCursor
	}

	writeJSON(w, http.StatusOK, TripPointsResponse{Items: items, NextCursor: cursor})
}
