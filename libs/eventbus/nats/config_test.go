package nats

import (
	"errors"
	"testing"

	"github.com/wrany/libs/events"
)

func TestConfigFromValues(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		stream     string
		want       Config
		wantErr    error
	}{
		{
			name:    "missing url",
			url:     "",
			stream:  "ANY",
			wantErr: ErrMissingURL,
		},
		{
			name:   "default stream",
			url:    "nats://nats:4222",
			stream: "",
			want:   Config{URL: "nats://nats:4222", Stream: events.StreamName},
		},
		{
			name:   "explicit stream",
			url:    "nats://nats:4222",
			stream: "CUSTOM",
			want:   Config{URL: "nats://nats:4222", Stream: "CUSTOM"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configFromValues(tt.url, tt.stream)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("config = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	t.Setenv("NATS_URL", "nats://127.0.0.1:4222")
	t.Setenv("NATS_STREAM", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.URL != "nats://127.0.0.1:4222" || cfg.Stream != events.StreamName {
		t.Errorf("unexpected config: %+v", cfg)
	}
}
