.PHONY: up down logs reset check-postgis db-shell test \
        nats-check nats-streams nats-init \
        migrate-up migrate-down migrate-version \
        migrate-worker-up migrate-worker-down migrate-worker-version \
        swagger-gen swagger-up ts-client web-up web-build \
        observability-up observability-down observability-logs \
        prometheus-check grafana-check loki-check \
        android-install android-install-debug android-devices

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

# Regenerate services/tracking-gateway/docs/ from Go annotations.
# Requires swag CLI: go install github.com/swaggo/swag/cmd/swag@latest
# Run after changing handler annotations; commit the result.
swagger-gen:
	cd services/tracking-gateway && GOWORK=off swag init \
		-g cmd/tracking-gateway/main.go \
		--outputTypes json,go \
		--parseDependency \
		--parseInternal
	@echo "Generated: services/tracking-gateway/docs/"

# Start Swagger UI (no build — spec is served live by tracking-gateway).
# Requires: make up (tracking-gateway must be running on :8080).
swagger-up:
	docker compose --profile tools up swagger-ui -d
	@echo "Swagger UI: http://localhost:8088"

# ── Observability ─────────────────────────────────────────────────────────────

# Start Prometheus, Grafana, Loki, Promtail, postgres-exporter, nats-exporter.
# Requires: make up (core services must be running).
observability-up:
	docker compose --profile observability up -d
	@echo "Prometheus: http://localhost:9090"
	@echo "Grafana:    http://localhost:3001 (admin / $$GRAFANA_PASSWORD)"
	@echo "Loki:       http://localhost:3100"

observability-down:
	docker compose --profile observability down

observability-logs:
	docker compose --profile observability logs -f

prometheus-check:
	curl -sf http://localhost:9090/-/ready && echo "Prometheus OK" || echo "Prometheus NOT READY"

grafana-check:
	curl -sf http://localhost:3001/api/health && echo "Grafana OK" || echo "Grafana NOT READY"

loki-check:
	curl -sf http://localhost:3100/ready && echo "Loki OK" || echo "Loki NOT READY"

# ── Web ───────────────────────────────────────────────────────────────────────

# Start web dev server via Docker Compose (profile: web).
web-up:
	docker compose --profile web up web -d
	@echo "Web dev server: http://localhost:3000"

# Build web app for production (local, no Docker).
web-build:
	cd apps/web && npm run build

# Generate TypeScript types for apps/web from the tracking-gateway OpenAPI spec.
# Requires: make swagger-gen first to ensure spec is up to date.
# Requires: npx (Node.js must be installed).
ts-client:
	cd apps/web && ORVAL_INPUT=../../services/tracking-gateway/docs/swagger.json \
		npx orval --config orval.config.local.ts
	@echo "Generated: apps/web/src/api/generated/schema.d.ts"

# ── Android tracker ─────────────────────────────────────────────────────────────

ANDROID_DIR := apps/android-tracker/android
# Target device serial. Override: make android-install DEVICE=<serial>
DEVICE ?= 2B171FDH20069Y

# Build and install the release APK onto $(DEVICE).
android-install:
	cd $(ANDROID_DIR) && ANDROID_SERIAL=$(DEVICE) ./gradlew installRelease

# Build and install the debug APK onto $(DEVICE).
android-install-debug:
	cd $(ANDROID_DIR) && ANDROID_SERIAL=$(DEVICE) ./gradlew installDebug

# List attached adb devices.
android-devices:
	adb devices -l
