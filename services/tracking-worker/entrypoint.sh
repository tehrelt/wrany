#!/bin/sh
set -e

# Track worker migrations separately from gateway migrations.
case "$DATABASE_URL" in
  *\?*) WORKER_DB_URL="${DATABASE_URL}&x-migrations-table=tracking_worker_schema_migrations" ;;
  *)    WORKER_DB_URL="${DATABASE_URL}?x-migrations-table=tracking_worker_schema_migrations" ;;
esac

./migrate -path ./infra/migrations -database "$WORKER_DB_URL" up
exec ./worker
