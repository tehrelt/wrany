FROM golang:1.25-alpine AS builder
WORKDIR /workspace

# --- dependency layer (cached unless go.mod/go.sum change) ---
COPY go.work go.work.sum ./
COPY libs/events/go.mod        libs/events/go.mod
COPY libs/eventbus/go.mod      libs/eventbus/go.mod
COPY libs/observability/go.mod libs/observability/go.mod
COPY services/tracking-worker/go.mod services/tracking-worker/go.mod
COPY services/tracking-worker/go.sum services/tracking-worker/go.sum

WORKDIR /workspace/services/tracking-worker
ENV GOWORK=off
RUN go mod download

# --- source layer ---
WORKDIR /workspace
COPY libs/events        ./libs/events
COPY libs/eventbus      ./libs/eventbus
COPY libs/observability ./libs/observability
COPY services/tracking-worker ./services/tracking-worker

WORKDIR /workspace/services/tracking-worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/tracking-worker
RUN CGO_ENABLED=0 GOOS=linux go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 \
    && cp "$(go env GOPATH)/bin/migrate" /out/migrate

FROM alpine:3.19
RUN apk add --no-cache curl
WORKDIR /app
COPY --from=builder /out/worker .
COPY --from=builder /out/migrate .
COPY services/tracking-worker/infra/migrations ./infra/migrations
COPY services/tracking-worker/entrypoint.sh .
RUN sed -i 's/\r//' entrypoint.sh && chmod +x entrypoint.sh
EXPOSE 8081
ENTRYPOINT ["./entrypoint.sh"]
