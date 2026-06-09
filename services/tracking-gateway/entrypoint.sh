#!/bin/sh
set -e

echo "tracking-gateway: running database migrations..."
./migrate -path ./infra/migrations -database "${DATABASE_URL}" up
echo "tracking-gateway: migrations applied successfully."

exec ./gateway
