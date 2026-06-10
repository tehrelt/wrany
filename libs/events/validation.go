package events

import (
	"errors"
	"fmt"
	"time"
)

// ValidationError aggregates all field-level problems found in one pass,
// so callers see every issue at once instead of fixing them one by one.
type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("events: validation failed: %v", e.Issues)
}

// NewValidationError returns a *ValidationError if issues is non-empty, nil otherwise.
func NewValidationError(issues []string) error {
	if len(issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: issues}
}

// IsValidationError reports whether err is (or wraps) a *ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// Validate checks envelope-level required fields.
// correlation_id is optional at the envelope level: it is required for
// location events (enforced by the location payload constructor) and
// optional for derived events.
func (e Envelope) Validate() error {
	var issues []string
	if e.EventID == "" {
		issues = append(issues, "event_id is required")
	}
	if e.EventType == "" {
		issues = append(issues, "event_type is required")
	}
	if e.EventVersion < 1 {
		issues = append(issues, "event_version must be >= 1")
	}
	if e.OccurredAt.IsZero() {
		issues = append(issues, "occurred_at is required")
	}
	if e.ProducedAt.IsZero() {
		issues = append(issues, "produced_at is required")
	}
	if e.Producer == "" {
		issues = append(issues, "producer is required")
	}
	if len(e.Payload) == 0 {
		issues = append(issues, "payload is required")
	}
	return NewValidationError(issues)
}

// RequireNonEmpty appends an issue when value is empty. Helper for payload validators.
func RequireNonEmpty(issues []string, field, value string) []string {
	if value == "" {
		return append(issues, field+" is required")
	}
	return issues
}

// RequireNonZeroTime appends an issue when t is the zero time. Helper for payload validators.
func RequireNonZeroTime(issues []string, field string, t time.Time) []string {
	if t.IsZero() {
		return append(issues, field+" is required")
	}
	return issues
}

// RequireRange appends an issue when value is outside [min, max]. Helper for payload validators.
func RequireRange(issues []string, field string, value, min, max float64) []string {
	if value < min || value > max {
		return append(issues, fmt.Sprintf("%s must be within [%v, %v], got %v", field, min, max, value))
	}
	return issues
}

// RequireNonNegative appends an issue when value is negative. Helper for payload validators.
func RequireNonNegative(issues []string, field string, value float64) []string {
	if value < 0 {
		return append(issues, fmt.Sprintf("%s must be >= 0, got %v", field, value))
	}
	return issues
}
