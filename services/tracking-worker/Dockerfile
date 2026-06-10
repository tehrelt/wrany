FROM golang:1.25-alpine AS builder
WORKDIR /workspace

# Copy workspace descriptor and all module sources.
COPY go.work go.work.sum ./
COPY libs/events ./libs/events
COPY libs/eventbus ./libs/eventbus
COPY services/tracking-worker ./services/tracking-worker

WORKDIR /workspace/services/tracking-worker
# Disable workspace mode — use go.mod replace directives only.
ENV GOWORK=off
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/tracking-worker
# Build migrate CLI with postgres driver tag.
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
