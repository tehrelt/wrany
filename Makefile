.PHONY: up down logs reset check-postgis db-shell test \
        nats-check nats-streams nats-init \
        migrate-up migrate-down migrate-version

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
# CLI commands run in a disposable nats-box container — no host install needed.
COMPOSE_NETWORK := wrany_wrany-net
NATS_BOX := docker run --rm --network $(COMPOSE_NETWORK) natsio/nats-box:latest
NATS_SERVER := nats://nats:4222

nats-check:
	docker compose exec nats wget -q -O - 'http://localhost:8222/healthz?js-enabled-only=true'

nats-streams:
	$(NATS_BOX) nats --server $(NATS_SERVER) stream ls

nats-init:
	$(NATS_BOX) sh -c "nats --server $(NATS_SERVER) stream info WRANY_EVENTS >/dev/null 2>&1 || nats --server $(NATS_SERVER) stream add WRANY_EVENTS --subjects 'location.events.*,trip.*,route.*,dead-letter.*' --storage file --retention limits --discard old --dupe-window 2m --replicas 1 --defaults"

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
