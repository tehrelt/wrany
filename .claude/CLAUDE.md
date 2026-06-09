# WR any% — Claude Project Instructions

## Product Context

This is an automatic route tracking application.

The user does not manually press START or FINISH.
The system automatically detects:

- meaningful movement start;
- active trip;
- trip finish;
- loop routes;
- repeated routes;
- best personal results.

Architecture:

- Android tracker client: React Native
- Web analytics client: React
- Backend services: Go
- Tracker client to backend transport: WebSocket only
- Backend internal event bus: Kafka
- Storage: Postgres + PostGIS

Kafka must never be exposed to external clients.

The Android tracker sends location data to the backend through WebSocket.
The backend sends ACK only after accepted events are successfully written to Kafka.
Backend workers make final decisions about trip detection, route matching, and best laps.

---

## Mandatory Epic Workflow

Work strictly by epics.

One epic = one Git branch = one epic directory = one EPIC.md file.

Claude must not write production code before creating and filling the EPIC.md file.

For each epic:

1. Create or switch to branch:
   epic/<epic-number>-<meaningful-epic-name>

2. Create epic directory:
   .claude/epics/<epic-number>-<meaningful-epic-name>/

3. Create epic file:
   .claude/epics/<epic-number>-<meaningful-epic-name>/EPIC.md

4. Fill EPIC.md with:
   - Goal
   - Context
   - Problem Analysis
   - Best Practice Research
   - Solution Design
   - Task Decomposition
   - Acceptance Criteria
   - Test Plan
   - Documentation Plan

5. Only after that write code.

---

## Branch Rules

Never write production code in:

- main
- master
- develop
- dev
- staging
- production

Every epic must use a branch:

epic/<number>-<name>

Examples:

- epic/01-project-foundation
- epic/05-websocket-tracker-protocol
- epic/08-trip-detection-engine

Do not merge epic branches into main unless the user explicitly asks.

Do not switch to another epic until the current epic is completed.

---

## EPIC.md Required Structure

Each EPIC.md must contain:

# EPIC <number>: <name>

## Status

Planned | In Progress | Implemented | Tested | Done

## Goal

## Context

## Problem Analysis

## Best Practice Research

## Solution Design

## Architecture Notes

## Tasks

## Acceptance Criteria

## Test Plan

## Documentation Plan

## Implementation Log

## Final Report

---

## Implementation Rules

Do not mix multiple epics in one branch.

Do not implement features outside the current epic scope.

Do not introduce unnecessary technologies.

Prefer production-like code, but MVP simplifications are allowed if explicitly documented.

Always add tests where reasonable.

Always update EPIC.md Implementation Log after meaningful changes.

Before closing an epic:

- tests must pass;
- documentation must be updated;
- Final Report must be filled;
- git status must be clean.

---

## Commit Rules

Use meaningful commits.

Format:

epic(<number>): <short description>

Examples:

- epic(01): add local docker compose foundation
- epic(05): define websocket tracker protocol contracts
- epic(08): add trip detection state machine

Avoid:

- fix
- update
- wip
- final
- changes

---

## Go Service Architecture

All Go services must follow Clean Architecture / Hexagonal Architecture.

### Mandatory directory structure

```
services/<service-name>/
  cmd/<service-name>/main.go        # composition root only
  internal/
    app/app.go                      # wires everything, manages lifecycle
    config/config.go                # loads env vars
    domain/                         # types, entities, errors — no external deps
    usecase/                        # business logic — no transport/storage deps
    transport/
      http/                         # HTTP handlers and router
      kafka/                        # Kafka consumers/producers
      grpc/                         # gRPC handlers
    storage/
      postgres/                     # Postgres implementations
      memory/                       # In-memory implementations
```

### Dependency direction

```
transport → usecase → domain
storage   → usecase interfaces / domain
cmd/app   → wires everything together
```

### Rules

- `cmd/<service-name>/main.go` is composition root only:
  load config → init logger → build app → run → graceful shutdown.
- No handlers or business logic in `main.go`.
- HTTP handlers live in `internal/transport/http/`.
- Kafka adapters live in `internal/transport/kafka/`.
- gRPC handlers live in `internal/transport/grpc/`.
- Usecases live in `internal/usecase/`. Must not import transport or driver types.
- Storage implementations live in `internal/storage/<name>/`.
- Domain types/entities/errors live in `internal/domain/`. No external dependencies.
- Transport layer must not contain business logic.
- Storage layer must not contain business logic.

---

## Initial Epic List

1. Project Foundation
2. Auth & Device Registration
3. Android Background Location Tracking
4. Android Offline Event Queue
5. WebSocket Tracker Protocol
6. Tracking Gateway Service
7. Kafka Event Contracts
8. Trip Detection Engine
9. Loop Route Detection
10. Route Matching & Repeated Trips
11. Best Lap & Personal Records
12. Analytics API
13. Web Analytics Client
14. User Correction Tools
15. Observability & Reliability
16. MVP Hardening

Start with EPIC 1 only.

Before starting any epic, Claude must read .claude/EPICS.md.

Claude must use .claude/EPICS.md as the source of truth for:

- epic goal;
- scope;
- out of scope;
- dependencies;
- acceptance criteria;
- expected deliverables.

Claude must not invent epic scope if it is not described in .claude/EPICS.md.
If the epic description is not detailed enough, Claude must stop and ask the user for clarification before writing code.
