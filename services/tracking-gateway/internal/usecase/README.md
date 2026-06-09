# usecase

Application business flows and repository interfaces.

Defines what the application does. Depends only on domain.
Must not import transport, HTTP, Kafka, gRPC, or SQL driver types.
Repository interfaces are declared here; implementations live in storage/.
