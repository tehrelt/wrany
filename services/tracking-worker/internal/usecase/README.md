# usecase

Application business flows and repository interfaces.

Defines trip detection, route matching, loop detection, and personal record logic.
Depends only on domain. Must not import transport, Kafka, or SQL driver types.
Repository interfaces are declared here; implementations live in storage/.
