# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder
WORKDIR /workspace

ARG TARGETOS=linux
ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOWORK=off

COPY libs/events/go.mod ./libs/events/
COPY libs/eventbus/go.mod libs/eventbus/go.sum ./libs/eventbus/
COPY libs/observability/go.mod libs/observability/go.sum ./libs/observability/
COPY services/tracking-worker/go.mod services/tracking-worker/go.sum ./services/tracking-worker/

WORKDIR /workspace/services/tracking-worker
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /workspace
COPY libs/events ./libs/events
COPY libs/eventbus ./libs/eventbus
COPY libs/observability ./libs/observability
COPY services/tracking-worker ./services/tracking-worker

WORKDIR /workspace/services/tracking-worker
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build \
      -trimpath -ldflags="-s -w" \
      -o /out/worker ./cmd/tracking-worker \
    && GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build \
      -trimpath -ldflags="-s -w" -tags postgres \
      -o /out/migrate github.com/golang-migrate/migrate/v4/cmd/migrate

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache curl
COPY --from=builder --chown=65532:65532 /out/worker /out/migrate ./
COPY services/tracking-worker/infra/migrations ./infra/migrations
COPY --chmod=755 services/tracking-worker/entrypoint.sh ./
USER 65532:65532
EXPOSE 8081
ENTRYPOINT ["./entrypoint.sh"]
