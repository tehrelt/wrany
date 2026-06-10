package http

import (
	"encoding/json"
	"time"
)

// WebSocket message type constants.
const (
	MsgTypeSessionStart     = "session.start"
	MsgTypeSessionAccepted  = "session.accepted"
	MsgTypeLocationBatch    = "location.batch"
	MsgTypeLocationBatchAck = "location.batch.ack"
	MsgTypeError            = "error"
	MsgTypePing             = "ping"
	MsgTypePong             = "pong"
)

// WsMessage is the common envelope for all WebSocket messages.
type WsMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// SessionStartPayload is the payload of session.start.
type SessionStartPayload struct {
	DeviceID   string `json:"device_id"`
	AppVersion string `json:"app_version,omitempty"`
	Platform   string `json:"platform,omitempty"`
}

// SessionAcceptedPayload is the payload of session.accepted.
type SessionAcceptedPayload struct {
	SessionID  string          `json:"session_id"`
	ServerTime time.Time       `json:"server_time"`
	Config     SessionCfgMsg   `json:"config"`
}

// SessionCfgMsg carries runtime config sent to the client on session.accepted.
type SessionCfgMsg struct {
	MaxBatchSize                int `json:"max_batch_size"`
	RecommendedFlushIntervalSec int `json:"recommended_flush_interval_sec"`
}

// LocationBatchPayload is the payload of location.batch.
type LocationBatchPayload struct {
	DeviceID string             `json:"device_id"`
	Events   []LocationEventMsg `json:"events"`
}

// LocationEventMsg is a single location data point in a batch.
type LocationEventMsg struct {
	EventID            string   `json:"event_id"`
	RecordedAt         string   `json:"recorded_at"`
	Lat                float64  `json:"lat"`
	Lon                float64  `json:"lon"`
	AccuracyM          float64  `json:"accuracy_m"`
	SpeedMps           *float64 `json:"speed_mps,omitempty"`
	BearingDeg         *float64 `json:"bearing_deg,omitempty"`
	ActivityType       *string  `json:"activity_type,omitempty"`
	ActivityConfidence *float64 `json:"activity_confidence,omitempty"`
	BatteryLevel       *float64 `json:"battery_level,omitempty"`
}

// LocationBatchAckPayload is the payload of location.batch.ack.
type LocationBatchAckPayload struct {
	Accepted   []string        `json:"accepted"`
	Duplicated []string        `json:"duplicated"`
	Rejected   []RejectedMsg   `json:"rejected"`
}

// RejectedMsg carries an event_id and the rejection reason.
type RejectedMsg struct {
	EventID string `json:"event_id"`
	Reason  string `json:"reason"`
}

// ErrorPayload is the payload of an error message.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
