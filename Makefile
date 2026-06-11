.PHONY: up down logs reset check-postgis db-shell test \
        nats-check nats-streams nats-init \
        migrate-up migrate-down migrate-version \
        migrate-worker-up migrate-worker-down migrate-worker-version \
        openapi-generate openapi-merge openapi-check swagger-up

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

reset:
	docker compose down -v
	docker compose up -d --build

check-postgis:
	docker compose exec postgres psql -U $${POSTGRES_USER:-wrany} -d $${POSTGRES_DB:-wrany} -c "SELECT PostGIS_Version();"

db-shell:
	docker compose exec postgres psql -U $${POSTGRES_USER:-wrany} -d $${POSTGRES_DB:-wrany}

test:
	cd libs/events && go test ./...
	cd libs/eventbus && go test ./...
	cd services/tracking-gateway && go test ./...
	cd services/tracking-worker && go test ./...

# NATS JetStream
# All commands use `docker compose exec nats` — no hardcoded network names.

nats-check:
	docker compose exec nats wget -q -O - 'http://localhost:8222/healthz?js-enabled-only=true'

nats-streams:
	docker compose exec nats wget -qO- 'http://localhost:8222/jsz?streams=1'

# Creates (or updates) the WRANY_EVENTS stream via the Go adapter.
# Requires NATS to be up (make up first).
nats-init:
	cd libs/eventbus && NATS_URL=nats://127.0.0.1:4222 go test -tags integration -run TestEnsureStream_Idempotent -v ./nats/...

# Migrations (tracking-gateway)
# Requires: migrate CLI — https://github.com/golang-migrate/migrate
# Install: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
MIGRATIONS_DIR := services/tracking-gateway/infra/migrations

migrate-up:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

migrate-version:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" version

# Migrations (tracking-worker)
# Owns raw_location_points and future worker-specific tables.
# Uses a separate migration path — does NOT overlap with tracking-gateway migrations.
WORKER_MIGRATIONS_DIR := services/tracking-worker/infra/migrations

migrate-worker-up:
	migrate -path $(WORKER_MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-worker-down:
	migrate -path $(WORKER_MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

migrate-worker-version:
	migrate -path $(WORKER_MIGRATIONS_DIR) -database "$(DATABASE_URL)" version

# ── OpenAPI ──────────────────────────────────────────────────────────────────

# Generate per-service OpenAPI specs from backend annotations.
# Requires swag CLI: go install github.com/swaggo/swag/cmd/swag@latest
openapi-generate:
	@mkdir -p docs/openapi/generated
	cd services/tracking-gateway && swag init \
		-g cmd/tracking-gateway/main.go \
		--output ../../docs/openapi/generated \
		--outputTypes json \
		--parseDependency \
		--parseInternal
	@mv docs/openapi/generated/swagger.json docs/openapi/generated/tracking-gateway.json
	@echo "Generated: docs/openapi/generated/tracking-gateway.json"

# Merge per-service specs into combined.json.
# Requires: docs/openapi/generated/ populated (run make openapi-generate first).
openapi-merge:
	@mkdir -p docs/openapi/generated
	npx --yes openapi-merge-cli --config docs/openapi/merge/openapi-merge.json
	@echo "Merged: docs/openapi/generated/combined.json"

# Check that spec generation succeeds (used in CI).
openapi-check:
	@mkdir -p /tmp/swag-check
	cd services/tracking-gateway && swag init \
		-g cmd/tracking-gateway/main.go \
		--output /tmp/swag-check \
		--outputTypes json \
		--parseDependency \
		--parseInternal
	@echo "openapi-check passed: tracking-gateway spec generates without errors"

# Build and start Swagger UI container (separate profile — not included in make up).
# The image embeds the OpenAPI spec generated from backend annotations at build time.
# Re-run after changing handler annotations to pick up the new spec.
swagger-up:
	docker compose --profile tools up swagger-ui -d --build
	@echo "Swagger UI: http://localhost:8088"
