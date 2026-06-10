package http_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httptransport "github.com/wrany/tracking-gateway/internal/transport/http"
)

func TestProtocol_EncodeDecodeSessionStart(t *testing.T) {
	payload := httptransport.SessionStartPayload{
		DeviceID:   "b0d34c19-ef5e-4e35-bd30-1d6680245c10",
		AppVersion: "1.0.0",
		Platform:   "android",
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	msg := httptransport.WsMessage{
		Type:      httptransport.MsgTypeSessionStart,
		RequestID: "req_001",
		Payload:   raw,
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var got httptransport.WsMessage
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, httptransport.MsgTypeSessionStart, got.Type)
	assert.Equal(t, "req_001", got.RequestID)

	var gotPayload httptransport.SessionStartPayload
	require.NoError(t, json.Unmarshal(got.Payload, &gotPayload))
	assert.Equal(t, "b0d34c19-ef5e-4e35-bd30-1d6680245c10", gotPayload.DeviceID)
}

func TestProtocol_EncodeDecodeSessionAccepted(t *testing.T) {
	payload := httptransport.SessionAcceptedPayload{
		SessionID:  "session-abc",
		ServerTime: time.Now().UTC().Truncate(time.Second),
		Config:     httptransport.SessionCfgMsg{MaxBatchSize: 100, RecommendedFlushIntervalSec: 10},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var got httptransport.SessionAcceptedPayload
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "session-abc", got.SessionID)
	assert.Equal(t, 100, got.Config.MaxBatchSize)
}

func TestProtocol_EncodeDecodeLocationBatchAck(t *testing.T) {
	payload := httptransport.LocationBatchAckPayload{
		Accepted:   []string{"evt_001"},
		Duplicated: []string{"evt_002"},
		Rejected: []httptransport.RejectedMsg{
			{EventID: "evt_003", Reason: "invalid_latitude"},
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var got httptransport.LocationBatchAckPayload
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, []string{"evt_001"}, got.Accepted)
	assert.Equal(t, []string{"evt_002"}, got.Duplicated)
	assert.Equal(t, "evt_003", got.Rejected[0].EventID)
	assert.Equal(t, "invalid_latitude", got.Rejected[0].Reason)
}

func TestProtocol_ErrorPayload(t *testing.T) {
	payload := httptransport.ErrorPayload{Code: "DEVICE_NOT_REGISTERED", Message: "device not registered"}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var got httptransport.ErrorPayload
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "DEVICE_NOT_REGISTERED", got.Code)
}

func TestProtocol_LocationEventMsg_OptionalFields(t *testing.T) {
	speed := 1.5
	bearing := 90.0
	actType := "walking"
	conf := 0.9
	battery := 0.75

	msg := httptransport.LocationEventMsg{
		EventID:            "evt-1",
		RecordedAt:         "2026-06-10T12:00:00Z",
		Lat:                55.75,
		Lon:                37.62,
		AccuracyM:          8.5,
		SpeedMps:           &speed,
		BearingDeg:         &bearing,
		ActivityType:       &actType,
		ActivityConfidence: &conf,
		BatteryLevel:       &battery,
	}

	raw, err := json.Marshal(msg)
	require.NoError(t, err)

	var got httptransport.LocationEventMsg
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "evt-1", got.EventID)
	require.NotNil(t, got.SpeedMps)
	assert.Equal(t, 1.5, *got.SpeedMps)
	require.NotNil(t, got.ActivityType)
	assert.Equal(t, "walking", *got.ActivityType)
}

func TestProtocol_LocationEventMsg_NilOptionalFields(t *testing.T) {
	msg := httptransport.LocationEventMsg{
		EventID:   "evt-1",
		RecordedAt: "2026-06-10T12:00:00Z",
		Lat:       55.75,
		Lon:       37.62,
		AccuracyM: 8.5,
	}

	raw, err := json.Marshal(msg)
	require.NoError(t, err)

	// Nil optional fields must be omitted from JSON.
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.NotContains(t, m, "speed_mps")
	assert.NotContains(t, m, "bearing_deg")
	assert.NotContains(t, m, "activity_type")
}
