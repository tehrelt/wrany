# WR any% Epic Registry

This file is the source of truth for epic scope.

Claude must read this file before starting any epic.

---

# EPIC 01: Project Foundation

## Goal

Create the local development foundation: repository structure, Docker Compose, Postgres/PostGIS, Kafka, Go service stubs, Makefile, README.

## Scope

- Docker Compose
- Postgres/PostGIS
- Kafka in KRaft mode
- Go workspace
- tracking-gateway stub
- tracking-worker stub
- apps directory placeholders
- Makefile
- README

## Out of Scope

- Real tracking logic
- Auth
- WebSocket protocol
- Android implementation
- Web implementation

---

# EPIC 02: Auth & Device Registration

## Goal

Implement the foundation for user authentication and device registration.

The backend must know which user and device are connecting before allowing tracker WebSocket sessions in future epics.

## Scope

### Backend

- Add auth-related domain structure.
- Add user model.
- Add device model.
- Add database tables for users and devices.
- Add password-based registration/login for MVP.
- Add JWT access token generation.
- Add refresh token strategy or documented MVP simplification.
- Add middleware for authenticated HTTP endpoints.
- Add device registration endpoint.
- Add endpoint to list current user devices.
- Add basic validation and error responses.
- Add tests.

### Database

Required tables:

- users
- devices
- refresh_tokens or sessions, if refresh tokens are implemented

Minimal user fields:

- id
- email
- password_hash
- created_at
- updated_at

Minimal device fields:

- id
- user_id
- device_id
- platform
- app_version
- device_name
- created_at
- last_seen_at

### API

MVP endpoints:

- POST /v1/auth/register
- POST /v1/auth/login
- POST /v1/auth/refresh, if refresh tokens are implemented
- POST /v1/devices/register
- GET /v1/devices
- GET /v1/me

### Security

- Passwords must be hashed.
- Plain passwords must never be stored.
- JWT secret must come from environment variables.
- Auth errors must not leak whether an email exists.
- Device registration must require a valid access token.

## Out of Scope

- OAuth providers
- Keycloak
- Social login
- Email verification
- Password reset
- Role-based permissions
- WebSocket auth implementation itself
- Android token storage implementation

WebSocket auth will be implemented in EPIC 5/6.

## Dependencies

Requires EPIC 1 to be completed.

## Acceptance Criteria

- User can register.
- User can login.
- Backend returns JWT access token.
- Authenticated endpoint /v1/me works.
- Unauthenticated request to protected endpoint is rejected.
- Authenticated user can register a device.
- Registered device is linked to user.
- Device list endpoint returns only current user's devices.
- Passwords are stored as hashes.
- Tests cover register/login/device registration.
- README documents auth endpoints and env variables.

---

# EPIC 03: Android Background Location Tracking

## Goal

Implement Android background location collection foundation in React Native.

## Scope

- Background location permission flow
- Foreground service strategy
- Activity recognition placeholder
- Local GPS event creation
- Battery-aware configuration

## Out of Scope

- WebSocket sync
- Offline event queue
- Backend trip detection

Depends on EPIC 2 only if authenticated config is needed.
