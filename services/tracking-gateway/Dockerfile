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
COPY services/tracking-gateway/go.mod services/tracking-gateway/go.sum ./services/tracking-gateway/

WORKDIR /workspace/services/tracking-gateway
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /workspace
COPY libs/events ./libs/events
COPY libs/eventbus ./libs/eventbus
COPY libs/observability ./libs/observability
COPY services/tracking-gateway ./services/tracking-gateway

WORKDIR /workspace/services/tracking-gateway
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go run github.com/swaggo/swag/cmd/swag@v1.16.6 init \
      -g cmd/tracking-gateway/main.go \
      --outputTypes json,go \
      --parseDependency \
      --parseInternal \
    && GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build \
      -trimpath -ldflags="-s -w" \
      -o /out/gateway ./cmd/tracking-gateway \
    && GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build \
      -trimpath -ldflags="-s -w" -tags postgres \
      -o /out/migrate github.com/golang-migrate/migrate/v4/cmd/migrate

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache curl
COPY --from=builder --chown=65532:65532 /out/gateway /out/migrate ./
COPY services/tracking-gateway/infra/migrations ./infra/migrations
COPY --chmod=755 services/tracking-gateway/entrypoint.sh ./
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
