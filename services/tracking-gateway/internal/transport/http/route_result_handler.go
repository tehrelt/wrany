package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// RouteResultHandler handles GET /v1/routes/{route_id}/results and /attempts.
type RouteResultHandler struct {
	uc *usecase.RouteResultQueryUsecase
}

func NewRouteResultHandler(uc *usecase.RouteResultQueryUsecase) *RouteResultHandler {
	return &RouteResultHandler{uc: uc}
}

// GetRouteResult godoc
// @Summary      Get personal records summary for a route
// @Tags         routes
// @Produce      json
// @Param        route_id  path  string  true  "Route UUID"
// @Success      200  {object}  RouteResultEnv
// @Failure      401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/routes/{route_id}/results [get]
func (h *RouteResultHandler) GetRouteResult(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	routeID := r.PathValue("route_id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "route_id is required")
		return
	}

	result, err := h.uc.GetRouteResult(r.Context(), usecase.GetRouteResultInput{
		UserID:  userID.String(),
		RouteID: routeID,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			writeError(w, http.StatusNotFound, "route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, routeResultToResponse(result))
}

// ListRouteAttempts godoc
// @Summary      List attempts for a route (paginated)
// @Tags         routes
// @Produce      json
// @Param        route_id  path   string  true   "Route UUID"
// @Param        limit     query  int     false  "Max results per page (default 50, max 200)"
// @Param        cursor    query  string  false  "Pagination cursor from previous response"
// @Success      200  {object}  RouteAttemptListEnv
// @Failure      401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/routes/{route_id}/attempts [get]
func (h *RouteResultHandler) ListRouteAttempts(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	routeID := r.PathValue("route_id")
	if routeID == "" {
		writeError(w, http.StatusBadRequest, "route_id is required")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	attempts, nextCursor, err := h.uc.ListRouteAttempts(r.Context(), usecase.ListRouteAttemptsInput{
		UserID:  userID.String(),
		RouteID: routeID,
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

	out := make([]TripAttemptItem, 0, len(attempts))
	for _, a := range attempts {
		out = append(out, tripAttemptToItem(a))
	}

	var cursor *string
	if nextCursor != "" {
		cursor = &nextCursor
	}
	writeJSON(w, http.StatusOK, RouteAttemptListResponse{Items: out, NextCursor: cursor})
}

func routeResultToResponse(r domain.RouteResult) RouteResultResponse {
	res := RouteResultResponse{
		RouteID:       r.RouteID,
		AttemptsCount: r.AttemptsCount,
	}
	if r.Best != nil {
		item := tripResultToItem(*r.Best)
		res.Best = &item
	}
	if r.Latest != nil {
		item := tripResultToItem(*r.Latest)
		res.Latest = &item
	}
	if r.Comparison != nil {
		c := RouteResultComparisonItem{
			LatestVsBestSec:     r.Comparison.LatestVsBestSec,
			LatestVsBestPercent: r.Comparison.LatestVsBestPercent,
		}
		res.Comparison = &c
	}
	return res
}

func tripResultToItem(tr domain.TripResult) TripResultItem {
	return TripResultItem{
		TripID:      tr.TripID,
		StartedAt:   tr.StartedAt.UTC().Format(time.RFC3339),
		DurationSec: tr.DurationSec,
		DistanceM:   tr.DistanceM,
		AvgSpeedMps: tr.AvgSpeedMps,
	}
}

func tripAttemptToItem(a domain.TripAttempt) TripAttemptItem {
	item := TripAttemptItem{
		TripID:      a.TripID,
		StartedAt:   a.StartedAt.UTC().Format(time.RFC3339),
		DurationSec: a.DurationSec,
		DistanceM:   a.DistanceM,
		AvgSpeedMps: a.AvgSpeedMps,
		MatchScore:  a.MatchScore,
		IsBest:      a.IsBest,
	}
	if a.EndedAt != nil {
		s := a.EndedAt.UTC().Format(time.RFC3339)
		item.EndedAt = &s
	}
	return item
}
