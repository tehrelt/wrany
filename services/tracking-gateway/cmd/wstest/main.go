// +build ignore

// Manual WebSocket smoke test. Run with:
//   go run ./cmd/wstest/main.go -token <jwt> -device <uuid>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	token := flag.String("token", "", "JWT access token")
	device := flag.String("device", "", "Device UUID")
	addr := flag.String("addr", "ws://localhost:8080/v1/ws/tracker", "WebSocket URL")
	flag.Parse()

	if *token == "" || *device == "" {
		log.Fatal("usage: -token <jwt> -device <uuid>")
	}

	headers := http.Header{"Authorization": []string{"Bearer " + *token}}
	conn, _, err := websocket.DefaultDialer.Dial(*addr, headers)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	fmt.Println("✓ Connected")

	send := func(typ, reqID string, payload any) {
		raw, _ := json.Marshal(payload)
		msg := struct {
			Type      string          `json:"type"`
			RequestID string          `json:"request_id"`
			Payload   json.RawMessage `json:"payload"`
		}{Type: typ, RequestID: reqID, Payload: raw}
		data, _ := json.Marshal(msg)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Fatalf("write: %v", err)
		}
	}

	read := func() map[string]any {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Fatalf("read: %v", err)
		}
		var m map[string]any
		json.Unmarshal(raw, &m)
		pretty, _ := json.MarshalIndent(m, "", "  ")
		fmt.Printf("← %s\n", pretty)
		return m
	}

	// session.start
	fmt.Println("\n→ session.start")
	send("session.start", "req-1", map[string]any{
		"device_id":   *device,
		"app_version": "1.0.0",
		"platform":    "android",
	})
	read()

	// location.batch
	fmt.Println("\n→ location.batch")
	send("location.batch", "req-2", map[string]any{
		"device_id": *device,
		"events": []map[string]any{{
			"event_id":    fmt.Sprintf("evt-%d", time.Now().UnixNano()),
			"recorded_at": time.Now().UTC().Format(time.RFC3339),
			"lat":         55.751244,
			"lon":         37.618423,
			"accuracy_m":  8.5,
			"speed_mps":   1.4,
			"activity_type": "walking",
		}},
	})
	read()

	fmt.Println("\n✓ Smoke test passed")
}
