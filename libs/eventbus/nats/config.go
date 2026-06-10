// Package nats implements the eventbus abstractions on top of NATS JetStream.
package nats

import (
	"errors"
	"os"

	"github.com/wrany/libs/events"
)

// ErrMissingURL is returned when NATS_URL is not set.
var ErrMissingURL = errors.New("nats: NATS_URL is required")

// Config holds NATS connection settings.
type Config struct {
	// URL is the NATS server URL, e.g. "nats://nats:4222". Required.
	URL string
	// Stream is the JetStream stream name. Defaults to events.StreamName.
	Stream string
}

// LoadConfig reads configuration from environment variables
// (NATS_URL required, NATS_STREAM optional).
func LoadConfig() (Config, error) {
	return configFromValues(os.Getenv("NATS_URL"), os.Getenv("NATS_STREAM"))
}

func configFromValues(url, stream string) (Config, error) {
	if url == "" {
		return Config{}, ErrMissingURL
	}
	if stream == "" {
		stream = events.StreamName
	}
	return Config{URL: url, Stream: stream}, nil
}
