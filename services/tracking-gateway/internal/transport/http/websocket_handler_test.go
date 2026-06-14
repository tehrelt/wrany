package http_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libevents "github.com/wrany/libs/events"
	"github.com/wrany/libs/eventbus"
	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/domain"
	httptransport "github.com/wrany/tracking-gateway/internal/transport/http"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// --- stubs ---

type stubDeviceLookup struct {
	registered map[string]bool
}

func (s *stubDeviceLookup) FindByUserAndDeviceID(_ context.Context, userID, deviceID uuid.UUID) (*domain.Device, error) {
	key := userID.String() + ":" + deviceID.String()
	if s.registered[key] {
		return &domain.Device{UserID: userID, DeviceID: deviceID}, nil
	}
	return nil, domain.ErrDeviceNotFound
}

type stubDedupRepo struct {
	published map[string]struct{}
}

func newStubDedupRepo() *stubDedupRepo {
	return &stubDedupRepo{published: make(map[string]struct{})}
}

func (s *stubDedupRepo) key(userID, deviceID uuid.UUID, eventID string) string {
	return userID.String() + ":" + deviceID.String() + ":" + eventID
}

func (s *stubDedupRepo) IsDuplicate(_ context.Context, userID, deviceID uuid.UUID, eventID string) (bool, error) {
	_, ok := s.published[s.key(userID, deviceID, eventID)]
	return ok, nil
}

func (s *stubDedupRepo) MarkPublished(_ context.Context, userID, deviceID uuid.UUID, eventID string) error {
	s.published[s.key(userID, deviceID, eventID)] = struct{}{}
	return nil
}

type stubPublisher struct {
	published int
	failWith  error
}

func (p *stubPublisher) Publish(_ context.Context, _ string, _ libevents.Envelope) error {
	if p.failWith != nil {
		return p.failWith
	}
	p.published++
	return nil
}

var _ eventbus.Publisher = (*stubPublisher)(nil)

// --- test server builder ---

type wsTestEnv struct {
	server   *httptest.Server
	userID   uuid.UUID
	deviceID uuid.UUID
	devices  *stubDeviceLookup
	dedup    *stubDedupRepo
	pub      *stubPublisher
}

func newWSTestEnv(t *testing.T) *wsTestEnv {
	t.Helper()

	userID := uuid.New()
	deviceID := uuid.New()
	registered := map[string]bool{
		userID.String() + ":" + deviceID.String(): true,
	}

	devices := &stubDeviceLookup{registered: registered}
	dedup := newStubDedupRepo()
	pub := &stubPublisher{}

	cfg := config.Config{
		WSMaxMessageSizeBytes: 262144,
		WSReadDeadlineSec:     60,
		WSWriteDeadlineSec:    10,
		WSPingIntervalSec:     30,
		WSMaxBatchSize:        100,
		WSAllowedOrigins:      []string{"http://localhost:3000"},
	}

	ingestionUC := usecase.NewTrackerIngestionUseCase(devices, dedup, pub, "test", 100, slog.Default())
	trackerH := httptransport.NewTrackerHandler(ingestionUC, cfg, nil)

	// Wrap in AuthMiddleware stub that injects the fixed userID.
	mux := http.NewServeMux()
	mux.Handle("GET /v1/ws/tracker", injectUserID(userID, trackerH))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &wsTestEnv{
		server:   srv,
		userID:   userID,
		deviceID: deviceID,
		devices:  devices,
		dedup:    dedup,
		pub:      pub,
	}
}

// injectUserID injects a fixed userID into the request context (replaces AuthMiddleware in tests).
func injectUserID(userID uuid.UUID, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(httptransport.WithUserID(r.Context(), userID)))
	})
}

func dialWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/ws/tracker"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func sendMsg(t *testing.T, conn *websocket.Conn, msgType, requestID string, payload any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	msg := httptransport.WsMessage{Type: msgType, RequestID: requestID, Payload: raw}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, data))
}

func readMsg(t *testing.T, conn *websocket.Conn) httptransport.WsMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var msg httptransport.WsMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	return msg
}

func makeLocationBatch(deviceID uuid.UUID, eventIDs ...string) httptransport.LocationBatchPayload {
	events := make([]httptransport.LocationEventMsg, len(eventIDs))
	for i, id := range eventIDs {
		events[i] = httptransport.LocationEventMsg{
			EventID:   id,
			RecordedAt: time.Now().UTC().Format(time.RFC3339),
			Lat:       55.75,
			Lon:       37.62,
			AccuracyM: 10.0,
		}
	}
	return httptransport.LocationBatchPayload{DeviceID: deviceID.String(), Events: events}
}

// --- tests ---

func TestWS_LocationBatchBeforeSessionStart(t *testing.T) {
	env := newWSTestEnv(t)
	conn := dialWS(t, env.server)

	sendMsg(t, conn, httptransport.MsgTypeLocationBatch, "req-1", makeLocationBatch(env.deviceID, "evt-1"))

	msg := readMsg(t, conn)
	assert.Equal(t, httptransport.MsgTypeError, msg.Type)
	var errPayload httptransport.ErrorPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &errPayload))
	assert.Equal(t, string(domain.ErrCodeSessionNotAccepted), errPayload.Code)
}

func TestWS_SessionStart_UnknownDevice(t *testing.T) {
	env := newWSTestEnv(t)
	conn := dialWS(t, env.server)

	unknownDevice := uuid.New()
	sendMsg(t, conn, httptransport.MsgTypeSessionStart, "req-1", httptransport.SessionStartPayload{
		DeviceID: unknownDevice.String(),
	})

	msg := readMsg(t, conn)
	assert.Equal(t, httptransport.MsgTypeError, msg.Type)
	var errPayload httptransport.ErrorPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &errPayload))
	assert.Equal(t, string(domain.ErrCodeDeviceNotRegistered), errPayload.Code)
}

func TestWS_SessionStart_InvalidDeviceIDFormat(t *testing.T) {
	env := newWSTestEnv(t)
	conn := dialWS(t, env.server)

	sendMsg(t, conn, httptransport.MsgTypeSessionStart, "req-1", httptransport.SessionStartPayload{
		DeviceID: "not-a-uuid",
	})

	msg := readMsg(t, conn)
	assert.Equal(t, httptransport.MsgTypeError, msg.Type)
	var errPayload httptransport.ErrorPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &errPayload))
	assert.Equal(t, string(domain.ErrCodeValidationError), errPayload.Code)
}

func TestWS_SessionStart_Success(t *testing.T) {
	env := newWSTestEnv(t)
	conn := dialWS(t, env.server)

	sendMsg(t, conn, httptransport.MsgTypeSessionStart, "req-1", httptransport.SessionStartPayload{
		DeviceID: env.deviceID.String(),
	})

	msg := readMsg(t, conn)
	assert.Equal(t, httptransport.MsgTypeSessionAccepted, msg.Type)
	assert.Equal(t, "req-1", msg.RequestID)

	var accepted httptransport.SessionAcceptedPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &accepted))
	assert.NotEmpty(t, accepted.SessionID)
	assert.Equal(t, 100, accepted.Config.MaxBatchSize)
}

func TestWS_LocationBatch_ValidEvents(t *testing.T) {
	env := newWSTestEnv(t)
	conn := dialWS(t, env.server)

	// Start session first.
	sendMsg(t, conn, httptransport.MsgTypeSessionStart, "req-1", httptransport.SessionStartPayload{DeviceID: env.deviceID.String()})
	msg := readMsg(t, conn)
	require.Equal(t, httptransport.MsgTypeSessionAccepted, msg.Type)

	// Send batch.
	sendMsg(t, conn, httptransport.MsgTypeLocationBatch, "req-2", makeLocationBatch(env.deviceID, "evt-1", "evt-2"))

	ackMsg := readMsg(t, conn)
	assert.Equal(t, httptransport.MsgTypeLocationBatchAck, ackMsg.Type)
	assert.Equal(t, "req-2", ackMsg.RequestID)

	var ack httptransport.LocationBatchAckPayload
	require.NoError(t, json.Unmarshal(ackMsg.Payload, &ack))
	assert.ElementsMatch(t, []string{"evt-1", "evt-2"}, ack.Accepted)
	assert.Empty(t, ack.Duplicated)
	assert.Empty(t, ack.Rejected)
	assert.Equal(t, 2, env.pub.published)
}

func TestWS_LocationBatch_DuplicateEvent(t *testing.T) {
	env := newWSTestEnv(t)
	conn := dialWS(t, env.server)

	sendMsg(t, conn, httptransport.MsgTypeSessionStart, "req-1", httptransport.SessionStartPayload{DeviceID: env.deviceID.String()})
	readMsg(t, conn) // session.accepted

	// First batch — accept evt-1.
	sendMsg(t, conn, httptransport.MsgTypeLocationBatch, "req-2", makeLocationBatch(env.deviceID, "evt-1"))
	readMsg(t, conn) // ack

	// Second batch — same evt-1 → duplicated.
	sendMsg(t, conn, httptransport.MsgTypeLocationBatch, "req-3", makeLocationBatch(env.deviceID, "evt-1"))
	ackMsg := readMsg(t, conn)

	var ack httptransport.LocationBatchAckPayload
	require.NoError(t, json.Unmarshal(ackMsg.Payload, &ack))
	assert.Empty(t, ack.Accepted)
	assert.Equal(t, []string{"evt-1"}, ack.Duplicated)
	// Published only once.
	assert.Equal(t, 1, env.pub.published)
}

func TestWS_LocationBatch_PartialInvalidEvents(t *testing.T) {
	env := newWSTestEnv(t)
	conn := dialWS(t, env.server)

	sendMsg(t, conn, httptransport.MsgTypeSessionStart, "req-1", httptransport.SessionStartPayload{DeviceID: env.deviceID.String()})
	readMsg(t, conn)

	batch := httptransport.LocationBatchPayload{
		DeviceID: env.deviceID.String(),
		Events: []httptransport.LocationEventMsg{
			{EventID: "good-1", RecordedAt: time.Now().UTC().Format(time.RFC3339), Lat: 55, Lon: 37, AccuracyM: 5},
			{EventID: "bad-1", RecordedAt: time.Now().UTC().Format(time.RFC3339), Lat: 999, Lon: 37, AccuracyM: 5},
		},
	}
	sendMsg(t, conn, httptransport.MsgTypeLocationBatch, "req-2", batch)
	ackMsg := readMsg(t, conn)

	var ack httptransport.LocationBatchAckPayload
	require.NoError(t, json.Unmarshal(ackMsg.Payload, &ack))
	assert.Equal(t, []string{"good-1"}, ack.Accepted)
	assert.Equal(t, "bad-1", ack.Rejected[0].EventID)
}

func TestWS_Origin_EmptyOriginAllowed(t *testing.T) {
	env := newWSTestEnv(t)
	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + "/v1/ws/tracker"
	// Dial without setting Origin header (empty).
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "empty origin should be allowed")
	defer conn.Close()
	_ = resp
}

func TestWS_Origin_AllowedBrowserOrigin(t *testing.T) {
	env := newWSTestEnv(t)
	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + "/v1/ws/tracker"
	headers := http.Header{"Origin": []string{"http://localhost:3000"}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	require.NoError(t, err, "allowed origin should be accepted")
	defer conn.Close()
}

func TestWS_Origin_DisallowedBrowserOrigin(t *testing.T) {
	env := newWSTestEnv(t)
	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + "/v1/ws/tracker"
	headers := http.Header{"Origin": []string{"https://evil.example.com"}}
	_, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	require.Error(t, err, "disallowed origin should be rejected")
}

func TestWS_PingPong(t *testing.T) {
	env := newWSTestEnv(t)
	conn := dialWS(t, env.server)

	sendMsg(t, conn, httptransport.MsgTypePing, "req-ping", struct{}{})

	msg := readMsg(t, conn)
	assert.Equal(t, httptransport.MsgTypePong, msg.Type)
	assert.Equal(t, "req-ping", msg.RequestID)
}
