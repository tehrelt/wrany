package http

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"

	"github.com/wrany/tracking-gateway/internal/domain"
)

// sendWSError encodes and writes an error message to the WebSocket connection.
// Write errors are logged but not returned; the read loop handles disconnect.
func sendWSError(conn *websocket.Conn, requestID string, code domain.IngestionErrorCode, msg string) {
	payload, err := json.Marshal(ErrorPayload{Code: string(code), Message: msg})
	if err != nil {
		log.Printf("websocket: marshal error payload: %v", err)
		return
	}
	out := WsMessage{
		Type:      MsgTypeError,
		RequestID: requestID,
		Payload:   payload,
	}
	data, err := json.Marshal(out)
	if err != nil {
		log.Printf("websocket: marshal error message: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("websocket: write error message: %v", err)
	}
}

// sendWSMessage encodes and writes an arbitrary message to the WebSocket connection.
func sendWSMessage(conn *websocket.Conn, msgType, requestID string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	out := WsMessage{
		Type:      msgType,
		RequestID: requestID,
		Payload:   raw,
	}
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}
