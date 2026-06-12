package logger

import (
	"log/slog"
	"os"
)

// New returns a JSON slog.Logger with the service name attached to every record.
func New(service string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h).With("service", service)
}
