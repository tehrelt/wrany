package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// RouteHandler handles GET /v1/routes endpoints.
type RouteHandler struct {
	uc *usecase.RouteQueryUsecase
}

func NewRouteHandler(uc *usecase.RouteQueryUsecase) *RouteHandler {
	return &RouteHandler{uc: uc}
}

func routeToItem(r domain.Route) RouteItem {
	return RouteItem{
		ID:         r.ID,
		UserID:     r.UserID,
		Name:       r.Name,
		Status:     r.Status,
		StartLat:   r.StartLat,
		StartLon:   r.StartLon,
		EndLat:     r.EndLat,
		EndLon:     r.EndLon,
		DistanceM:  r.DistanceM,
		TripsCount: r.TripsCount,
		CreatedAt:  r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func routeTripToItem(rt domain.RouteTrip) RouteTripItem {
	item := RouteTripItem{
		TripID:      rt.TripID,
		MatchScore:  rt.MatchScore,
		MatchedAt:   rt.MatchedAt.UTC().Format(time.RFC3339),
		DurationSec: rt.DurationSec,
		DistanceM:   rt.DistanceM,
		StartedAt:   rt.StartedAt.UTC().Format(time.RFC3339),
	}
	if rt.EndedAt != nil {
		s := rt.EndedAt.UTC().Format(time.RFC3339)
		item.EndedAt = &s
	}
	return item
}

// ListRoutes godoc
// @Summary      List routes for the authenticated user
// @Tags         routes
// @Produce      json
// @Param        device_id  query  string  false  "Filter by device UUID"
// @Param        limit      query  int     false  "Max results per page (default 50, max 200)"
// @Param        cursor     query  string  false  "Pagination cursor from previous response"
// @Success      200  {object}  RouteListEnv
// @Failure      401  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/routes [get]
func (h *RouteHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	routes, nextCursor, err := h.uc.ListRoutes(r.Context(), usecase.ListRoutesInput{
		UserID:   userID.String(),
		DeviceID: q.Get("device_id"),
		Limit:    limit,
		Cursor:   q.Get("cursor"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]RouteItem, 0, len(routes))
	for _, rt := range routes {
		items = append(items, routeToItem(rt))
	}

	var cursor *string
	if nextCursor != "" {
		cursor = &nextCursor
	}
	writeJSON(w, http.StatusOK, RouteListResponse{Items: items, NextCursor: cursor})
}

// GetRoute godoc
// @Summary      Get a single route by ID
// @Tags         routes
// @Produce      json
// @Param        id  path  string  true  "Route UUID"
// @Success      200  {object}  RouteEnv
// @Failure      401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/routes/{id} [get]
func (h *RouteHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	routeID := r.PathValue("id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	route, err := h.uc.GetRoute(r.Context(), userID.String(), routeID)
	if err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, routeToItem(route))
}

// DeleteRoute godoc
// @Summary      Delete a route
// @Description  Removes the route and its trip-match links. The underlying trips are preserved.
// @Tags         routes
// @Param        id  path  string  true  "Route UUID"
// @Success      204
// @Failure      401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/routes/{id} [delete]
func (h *RouteHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	routeID := r.PathValue("id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.uc.DeleteRoute(r.Context(), userID.String(), routeID); err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListRouteTrips godoc
// @Summary      List trips attached to a route
// @Tags         routes
// @Produce      json
// @Param        id      path   string  true   "Route UUID"
// @Param        limit   query  int     false  "Max results per page (default 50, max 200)"
// @Param        cursor  query  string  false  "Pagination cursor from previous response"
// @Success      200  {object}  RouteTripListEnv
// @Failure      401  {object}  ApiError
// @Failure      403  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/routes/{id}/trips [get]
func (h *RouteHandler) ListRouteTrips(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	routeID := r.PathValue("id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	items, nextCursor, err := h.uc.ListRouteTrips(r.Context(), usecase.ListRouteTripsInput{
		RouteID: routeID,
		UserID:  userID.String(),
		Limit:   limit,
		Cursor:  q.Get("cursor"),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrRouteForbidden) {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]RouteTripItem, 0, len(items))
	for _, rt := range items {
		out = append(out, routeTripToItem(rt))
	}

	var cursor *string
	if nextCursor != "" {
		cursor = &nextCursor
	}
	writeJSON(w, http.StatusOK, RouteTripListResponse{Items: out, NextCursor: cursor})
}

// GetRoutePoints godoc
// @Summary      Get template polyline points of a route
// @Tags         routes
// @Produce      json
// @Param        id  path  string  true  "Route UUID"
// @Success      200  {object}  RoutePointsEnv
// @Failure      401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/routes/{id}/points [get]
func (h *RouteHandler) GetRoutePoints(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	routeID := r.PathValue("id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	pts, err := h.uc.GetRoutePoints(r.Context(), userID.String(), routeID)
	if err != nil {
		if errors.Is(err, usecase.ErrRouteForbidden) {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]RoutePointItem, 0, len(pts))
	for _, p := range pts {
		out = append(out, RoutePointItem{Lat: p.Lat, Lon: p.Lon})
	}
	writeJSON(w, http.StatusOK, out)
}
