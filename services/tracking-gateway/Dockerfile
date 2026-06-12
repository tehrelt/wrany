FROM golang:1.25-alpine AS builder
WORKDIR /workspace

# --- dependency layer (cached unless go.mod/go.sum change) ---
COPY go.work go.work.sum ./
COPY libs/events/go.mod        libs/events/go.mod
COPY libs/eventbus/go.mod      libs/eventbus/go.mod
COPY libs/observability/go.mod libs/observability/go.mod
COPY services/tracking-gateway/go.mod services/tracking-gateway/go.mod
COPY services/tracking-gateway/go.sum services/tracking-gateway/go.sum

WORKDIR /workspace/services/tracking-gateway
ENV GOWORK=off
RUN go mod download

# --- source layer ---
WORKDIR /workspace
COPY libs/events        ./libs/events
COPY libs/eventbus      ./libs/eventbus
COPY libs/observability ./libs/observability
COPY services/tracking-gateway ./services/tracking-gateway

WORKDIR /workspace/services/tracking-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./cmd/tracking-gateway
RUN CGO_ENABLED=0 GOOS=linux go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 \
    && cp "$(go env GOPATH)/bin/migrate" /out/migrate

FROM alpine:3.19
RUN apk add --no-cache curl
WORKDIR /app
COPY --from=builder /out/gateway .
COPY --from=builder /out/migrate .
COPY services/tracking-gateway/infra/migrations ./infra/migrations
COPY services/tracking-gateway/entrypoint.sh .
RUN sed -i 's/\r//' entrypoint.sh && chmod +x entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
