# transport/nats

NATS JetStream publishers and adapters.

Publishes accepted location events to NATS JetStream subjects after the gateway
receives them from Android tracker clients via WebSocket. Uses the shared
`Publisher` abstraction from `libs/eventbus` and event contracts from `libs/events`.
Calls usecase interfaces; must not contain business logic.
