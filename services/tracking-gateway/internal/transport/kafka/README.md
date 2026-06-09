# transport/kafka

Kafka producers and adapters.

Publishes accepted location events to Kafka topics after the gateway
receives them from Android tracker clients via WebSocket.
Calls usecase interfaces; must not contain business logic.
