package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/observ"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// TrackerHandler serves the WebSocket tracker endpoint GET /v1/ws/tracker.
type TrackerHandler struct {
	ingestion *usecase.TrackerIngestionUseCase
	upgrader  websocket.Upgrader
	cfg       config.Config
	metrics   *observ.GatewayMetrics
}

// NewTrackerHandler constructs a TrackerHandler with the correct upgrader config.
func NewTrackerHandler(ingestion *usecase.TrackerIngestionUseCase, cfg config.Config, metrics *observ.GatewayMetrics) *TrackerHandler {
	allowedOrigins := make(map[string]struct{}, len(cfg.WSAllowedOrigins))
	for _, o := range cfg.WSAllowedOrigins {
		allowedOrigins[o] = struct{}{}
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		// JWT auth already protects this endpoint — CSRF is not applicable.
		// Android sends a non-empty Origin that varies by device/OS version,
		// so we accept all origins here and rely on token validation.
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			if len(allowedOrigins) > 0 {
				_, ok := allowedOrigins[origin]
				return ok
			}
			return true
		},
	}
	return &TrackerHandler{ingestion: ingestion, upgrader: upgrader, cfg: cfg, metrics: metrics}
}

// ServeHTTP validates JWT (via AuthMiddleware), upgrades to WebSocket,
// and drives the connection lifecycle.
func (h *TrackerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Capture request context before upgrade — gorilla does not thread it through.
	ctx := r.Context()
	sessionID := uuid.NewString()

	// Enrich context logger with WS correlation fields.
	log := slog.Default().With("session_id", sessionID, "user_id", userID)

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("ws: upgrade failed", "err", err)
		return
	}
	log.Info("ws: connected")
	if h.metrics != nil {
		h.metrics.WSConnectionsTotal.Inc()
		h.metrics.WSConnectionsActive.Inc()
	}
	defer func() {
		if h.metrics != nil {
			h.metrics.WSConnectionsActive.Dec()
		}
		conn.Close()
	}()

	// Transport-level read limit (before JSON decode).
	conn.SetReadLimit(h.cfg.WSMaxMessageSizeBytes)

	readDeadline := time.Duration(h.cfg.WSReadDeadlineSec) * time.Second
	writeDeadline := time.Duration(h.cfg.WSWriteDeadlineSec) * time.Second
	pingInterval := time.Duration(h.cfg.WSPingIntervalSec) * time.Second

	// Pong resets the read deadline.
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(readDeadline))
	})
	if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		return
	}

	// Ping goroutine: sends periodic pings; exits when connClosed is closed.
	connClosed := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(writeDeadline))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-connClosed:
				return
			}
		}
	}()
	defer close(connClosed)

	h.readLoop(ctx, conn, log, writeDeadline)
}

// readLoop reads and dispatches messages until the connection closes.
// log is pre-enriched with session_id and user_id.
func (h *TrackerHandler) readLoop(ctx context.Context, conn *websocket.Conn, log *slog.Logger, writeDeadline time.Duration) {
	var session *domain.TrackerSession

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Warn("ws: unexpected close", "err", err)
			}
			return
		}

		var msg WsMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			sendWSError(conn, "", domain.ErrCodeValidationError, "invalid JSON")
			continue
		}

		_ = conn.SetWriteDeadline(time.Now().Add(writeDeadline))

		switch msg.Type {
		case MsgTypePing:
			if err := conn.WriteMessage(websocket.TextMessage, mustMarshal(WsMessage{Type: MsgTypePong, RequestID: msg.RequestID})); err != nil {
				return
			}

		case MsgTypeSessionStart:
			session = h.handleSessionStart(ctx, conn, log, msg)

		case MsgTypeLocationBatch:
			if session == nil {
				sendWSError(conn, msg.RequestID, domain.ErrCodeSessionNotAccepted, "send session.start first")
				continue
			}
			if stop := h.handleLocationBatch(ctx, conn, log, msg, session); stop {
				return
			}

		default:
			sendWSError(conn, msg.RequestID, domain.ErrCodeValidationError, fmt.Sprintf("unknown message type: %q", msg.Type))
		}
	}
}

// handleSessionStart processes session.start and returns the new session (nil on failure).
// log is pre-enriched with session_id/user_id.
func (h *TrackerHandler) handleSessionStart(ctx context.Context, conn *websocket.Conn, log *slog.Logger, msg WsMessage) *domain.TrackerSession {
	var payload SessionStartPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sendWSError(conn, msg.RequestID, domain.ErrCodeValidationError, "invalid session.start payload")
		return nil
	}

	deviceID, err := uuid.Parse(payload.DeviceID)
	if err != nil {
		sendWSError(conn, msg.RequestID, domain.ErrCodeValidationError, "device_id must be a valid UUID")
		return nil
	}

	// Extract user_id from the enriched logger's context by pulling it from context.
	userID, _ := UserIDFromContext(ctx)
	session, err := h.ingestion.StartSession(ctx, userID, deviceID)
	if err != nil {
		if errors.Is(err, domain.ErrDeviceNotFound) {
			if h.metrics != nil {
				h.metrics.WSSessionsRejected.Inc()
			}
			sendWSError(conn, msg.RequestID, domain.ErrCodeDeviceNotRegistered, "device not registered for this user")
			return nil
		}
		log.Error("ws: start session", "device_id", deviceID, "err", err)
		sendWSError(conn, msg.RequestID, domain.ErrCodeInternalError, "internal error")
		return nil
	}

	accepted := SessionAcceptedPayload{
		SessionID:  session.ID,
		ServerTime: time.Now().UTC(),
		Config: SessionCfgMsg{
			MaxBatchSize:                h.cfg.WSMaxBatchSize,
			RecommendedFlushIntervalSec: 10,
		},
	}
	if err := sendWSMessage(conn, MsgTypeSessionAccepted, msg.RequestID, accepted); err != nil {
		log.Error("ws: send session.accepted", "device_id", deviceID, "err", err)
		return nil
	}
	if h.metrics != nil {
		h.metrics.WSSessionsAccepted.Inc()
	}
	log.Info("ws: session accepted", "device_id", deviceID)
	return session
}

// handleLocationBatch processes location.batch. Returns true if the connection should close.
// log is pre-enriched with session_id/user_id.
func (h *TrackerHandler) handleLocationBatch(ctx context.Context, conn *websocket.Conn, log *slog.Logger, msg WsMessage, session *domain.TrackerSession) bool {
	var payload LocationBatchPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sendWSError(conn, msg.RequestID, domain.ErrCodeValidationError, "invalid location.batch payload")
		return false
	}

	if len(payload.Events) == 0 {
		sendWSError(conn, msg.RequestID, domain.ErrCodeValidationError, "events array is empty")
		return false
	}

	domainEvents, parseRejects := parseLocationEvents(payload.Events)

	if h.metrics != nil {
		h.metrics.LocationBatchesReceived.Inc()
		h.metrics.LocationPointsReceived.Add(float64(len(domainEvents)))
	}

	result, err := h.ingestion.IngestBatch(ctx, session, domainEvents)
	if err != nil {
		if h.metrics != nil {
			h.metrics.LocationBatchesRejected.Inc()
		}
		code, ok := usecase.IngestionErrCode(err)
		if !ok {
			code = domain.ErrCodeInternalError
		}
		sendWSError(conn, msg.RequestID, code, err.Error())
		return false
	}

	if h.metrics != nil {
		h.metrics.LocationBatchesAcked.Inc()
	}

	allRejected := make([]RejectedMsg, 0, len(parseRejects)+len(result.Rejected))
	for _, r := range parseRejects {
		allRejected = append(allRejected, RejectedMsg{EventID: r.EventID, Reason: r.Reason})
	}
	for _, r := range result.Rejected {
		allRejected = append(allRejected, RejectedMsg{EventID: r.EventID, Reason: r.Reason})
	}

	ack := LocationBatchAckPayload{
		Accepted:   result.Accepted,
		Duplicated: result.Duplicated,
		Rejected:   allRejected,
	}
	if err := sendWSMessage(conn, MsgTypeLocationBatchAck, msg.RequestID, ack); err != nil {
		log.Error("ws: send location.batch.ack", "err", err)
		return true
	}
	return false
}

// parseLocationEvents converts wire-format events into domain events.
// Events failing time parsing are returned as rejects.
func parseLocationEvents(msgs []LocationEventMsg) ([]domain.LocationEvent, []domain.RejectedEvent) {
	events := make([]domain.LocationEvent, 0, len(msgs))
	var rejects []domain.RejectedEvent

	for _, m := range msgs {
		if m.EventID == "" {
			rejects = append(rejects, domain.RejectedEvent{EventID: "(unknown)", Reason: "event_id is required"})
			continue
		}
		t, err := time.Parse(time.RFC3339, m.RecordedAt)
		if err != nil {
			rejects = append(rejects, domain.RejectedEvent{EventID: m.EventID, Reason: "recorded_at must be RFC3339"})
			continue
		}
		ev := domain.LocationEvent{
			EventID:            m.EventID,
			RecordedAt:         t,
			Lat:                m.Lat,
			Lon:                m.Lon,
			AccuracyM:          m.AccuracyM,
			SpeedMps:           m.SpeedMps,
			BearingDeg:         m.BearingDeg,
			ActivityConfidence: m.ActivityConfidence,
			BatteryLevel:       m.BatteryLevel,
		}
		if m.ActivityType != nil {
			at := domain.ActivityType(*m.ActivityType)
			ev.ActivityType = &at
		}
		events = append(events, ev)
	}
	return events, rejects
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustMarshal: %v", err))
	}
	return b
}
