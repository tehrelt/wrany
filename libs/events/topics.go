package events

// StreamName is the single JetStream stream holding all WR any% events.
const StreamName = "WRANY_EVENTS"

// Subjects for concrete event types (version suffix is part of the contract).
const (
	SubjectLocationEvents = "location.events.v1"
	SubjectTripStarted    = "trip.started.v1"
	SubjectTripUpdated    = "trip.updated.v1"
	SubjectTripCompleted  = "trip.completed.v1"
	SubjectRouteMatched   = "route.matched.v1"
	SubjectDeadLetter     = "dead-letter.v1"
)

// Message headers set by publishers.
//
// HeaderMsgID maps to NATS deduplication: JetStream drops messages with a
// duplicate Nats-Msg-Id within the stream's dedup window. This is best-effort
// publisher retry protection, not global business idempotency.
const (
	HeaderMsgID         = "Nats-Msg-Id"
	HeaderEventType     = "Wrany-Event-Type"
	HeaderCorrelationID = "Wrany-Correlation-Id"
	HeaderUserID        = "Wrany-User-Id"
	HeaderDeviceID      = "Wrany-Device-Id"
)

// StreamSubjects returns the wildcard subject filters the stream is bound to.
// Returns a fresh slice on every call so callers cannot mutate shared state.
func StreamSubjects() []string {
	return []string{
		"location.events.*",
		"trip.*",
		"route.*",
		"dead-letter.*",
	}
}

// ConsumerName builds a durable consumer name following the project convention:
// <service>-<domain>-consumer, e.g. "tracking-worker-location-consumer".
func ConsumerName(service, domain string) string {
	return service + "-" + domain + "-consumer"
}
