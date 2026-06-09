.PHONY: up down logs reset check-postgis db-shell test

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
	cd services/tracking-gateway && go test ./...
	cd services/tracking-worker && go test ./...
