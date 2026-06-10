FROM golang:1.25-alpine AS builder
WORKDIR /workspace

# Copy workspace descriptor and all module sources.
COPY go.work go.work.sum ./
COPY libs/events ./libs/events
COPY libs/eventbus ./libs/eventbus
COPY services/tracking-gateway ./services/tracking-gateway

WORKDIR /workspace/services/tracking-gateway
# Disable workspace mode so only go.mod replace directives are used.
# libs/events and libs/eventbus are available at ../../libs/* as declared.
ENV GOWORK=off
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./cmd/tracking-gateway
# Build migrate CLI with postgres driver tag
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
