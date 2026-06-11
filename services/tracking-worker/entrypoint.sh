#!/bin/sh
set -e

# Append x-migrations-table to the URL so the worker's migrations are tracked
# separately from the gateway's schema_migrations table.
case "$DATABASE_URL" in
  *\?*) WORKER_DB_URL="${DATABASE_URL}&x-migrations-table=tracking_worker_schema_migrations" ;;
  *)    WORKER_DB_URL="${DATABASE_URL}?x-migrations-table=tracking_worker_schema_migrations" ;;
esac

./migrate -path ./infra/migrations -database "$WORKER_DB_URL" up
exec ./worker
