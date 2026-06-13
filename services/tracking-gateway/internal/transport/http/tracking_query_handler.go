package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wrany/tracking-gateway/internal/usecase"
)

type TrackingQueryHandler struct {
	uc *usecase.TrackingQueryUsecase
}

func NewTrackingQueryHandler(uc *usecase.TrackingQueryUsecase) *TrackingQueryHandler {
	return &TrackingQueryHandler{uc: uc}
}

// GetPoints godoc
// @Summary      List raw GPS points for the authenticated user
// @Tags         tracking
// @Produce      json
// @Param        device_id  query     string  false  "Filter by device UUID"
// @Param        from       query     string  true   "Start of time range (RFC3339)"
// @Param        to         query     string  true   "End of time range (RFC3339)"
// @Param        limit      query     int     false  "Max results per page (default 1000, max 5000)"
// @Param        cursor     query     string  false  "Pagination cursor from previous response"
// @Success      200        {object}  TrackingPointsEnv
// @Failure      400        {object}  ApiError
// @Failure      401        {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/tracking/points [get]
func (h *TrackingQueryHandler) GetPoints(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()

	from, to, err := parseDateRange(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))

	points, nextCursor, err := h.uc.GetPoints(r.Context(), usecase.GetPointsInput{
		UserID:   userID.String(),
		DeviceID: q.Get("device_id"),
		From:     from,
		To:       to,
		Limit:    limit,
		Cursor:   q.Get("cursor"),
	})
	if err != nil {
		if isValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]TrackingPoint, 0, len(points))
	for _, p := range points {
		items = append(items, TrackingPoint{
			EventID:      p.EventID,
			DeviceID:     p.DeviceID,
			RecordedAt:   p.RecordedAt.UTC().Format(time.RFC3339),
			Lat:          p.Lat,
			Lon:          p.Lon,
			AccuracyM:    p.AccuracyM,
			SpeedMps:     p.SpeedMps,
			BearingDeg:   p.BearingDeg,
			ActivityType: p.ActivityType,
		})
	}

	var cursor *string
	if nextCursor != "" {
		cursor = &nextCursor
	}

	writeJSON(w, http.StatusOK, TrackingPointsResponse{
		Items:      items,
		NextCursor: cursor,
	})
}

// GetSummary godoc
// @Summary      Get aggregated stats for a time range
// @Tags         tracking
// @Produce      json
// @Param        device_id  query     string  false  "Filter by device UUID"
// @Param        from       query     string  true   "Start of time range (RFC3339)"
// @Param        to         query     string  true   "End of time range (RFC3339)"
// @Success      200        {object}  TrackingSummaryEnv
// @Failure      400        {object}  ApiError
// @Failure      401        {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/tracking/summary [get]
func (h *TrackingQueryHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()

	from, to, err := parseDateRange(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	summary, err := h.uc.GetSummary(r.Context(), usecase.GetSummaryInput{
		UserID:   userID.String(),
		DeviceID: q.Get("device_id"),
		From:     from,
		To:       to,
	})
	if err != nil {
		if isValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := TrackingSummary{
		PointsCount: summary.PointsCount,
		DurationSec: summary.DurationSec,
		AvgSpeedMps: summary.AvgSpeedMps,
		MaxSpeedMps: summary.MaxSpeedMps,
	}
	if summary.FirstRecordedAt != nil {
		s := summary.FirstRecordedAt.UTC().Format(time.RFC3339)
		resp.FirstRecordedAt = &s
	}
	if summary.LastRecordedAt != nil {
		s := summary.LastRecordedAt.UTC().Format(time.RFC3339)
		resp.LastRecordedAt = &s
	}

	writeJSON(w, http.StatusOK, resp)
}

// DeletePoint godoc
// @Summary      Delete a single GPS point by event_id
// @Tags         tracking
// @Param        event_id  path  string  true  "Event UUID"
// @Success      204
// @Failure      401  {object}  ApiError
// @Failure      404  {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/tracking/points/{event_id} [delete]
func (h *TrackingQueryHandler) DeletePoint(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	eventID := r.PathValue("event_id")
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "event_id is required")
		return
	}

	if err := h.uc.DeletePoint(r.Context(), usecase.DeletePointInput{
		UserID:  userID.String(),
		EventID: eventID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "point not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetTrack godoc
// @Summary      Get simplified track: stationary clusters collapsed to a centroid
// @Tags         tracking
// @Produce      json
// @Param        device_id  query     string  false  "Filter by device UUID"
// @Param        from       query     string  true   "Start of time range (RFC3339)"
// @Param        to         query     string  true   "End of time range (RFC3339)"
// @Success      200        {object}  TrackEnv
// @Failure      400        {object}  ApiError
// @Failure      401        {object}  ApiError
// @Security     BearerAuth
// @Router       /v1/tracking/track [get]
func (h *TrackingQueryHandler) GetTrack(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()

	from, to, err := parseDateRange(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var speedThreshold *float64
	if value := q.Get("speed_threshold_mps"); value != "" {
		parsed, parseErr := strconv.ParseFloat(value, 64)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid speed_threshold_mps")
			return
		}
		speedThreshold = &parsed
	}
	var minStaySec *int
	if value := q.Get("min_stay_sec"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid min_stay_sec")
			return
		}
		minStaySec = &parsed
	}
	var minMoveSec *int
	if value := q.Get("min_move_sec"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid min_move_sec")
			return
		}
		minMoveSec = &parsed
	}

	segments, err := h.uc.GetTrack(r.Context(), usecase.GetTrackInput{
		UserID:            userID.String(),
		DeviceID:          q.Get("device_id"),
		From:              from,
		To:                to,
		SpeedThresholdMps: speedThreshold,
		MinStaySec:        minStaySec,
		MinMoveSec:        minMoveSec,
	})
	if err != nil {
		if isValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]TrackSegmentItem, 0, len(segments))
	for _, s := range segments {
		items = append(items, TrackSegmentItem{
			Kind:            string(s.Kind),
			SegmentID:       s.SegmentID,
			EventID:         s.EventID,
			RecordedAt:      s.RecordedAt.UTC().Format(time.RFC3339),
			PeriodEnd:       s.PeriodEnd.UTC().Format(time.RFC3339),
			Lat:             s.Lat,
			Lon:             s.Lon,
			SpeedMps:        s.SpeedMps,
			AccuracyM:       s.AccuracyM,
			StayDurationSec: s.StayDurationSec,
			MergedCount:     s.MergedCount,
		})
	}

	writeJSON(w, http.StatusOK, TrackResponse{Items: items})
}

func parseDateRange(fromStr, toStr string) (time.Time, time.Time, error) {
	if fromStr == "" {
		return time.Time{}, time.Time{}, usecase.ErrFromRequired
	}
	if toStr == "" {
		return time.Time{}, time.Time{}, usecase.ErrToRequired
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, usecase.ErrFromRequired
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, usecase.ErrToRequired
	}
	return from, to, nil
}

func isValidationError(err error) bool {
	return err == usecase.ErrFromRequired ||
		err == usecase.ErrToRequired ||
		err == usecase.ErrInvalidRange ||
		err == usecase.ErrRangeTooLarge
}
