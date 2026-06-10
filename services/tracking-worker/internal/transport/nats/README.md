# transport/nats

NATS JetStream consumers and adapters.

Consumes location events from NATS JetStream durable consumers and passes them
to usecase handlers. Uses event contracts from `libs/events`.
Calls usecase interfaces; must not contain business logic.
