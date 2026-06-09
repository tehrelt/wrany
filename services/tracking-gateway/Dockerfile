FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./cmd/tracking-gateway
# Build migrate CLI with postgres driver tag
RUN CGO_ENABLED=0 GOOS=linux go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1 \
    && cp "$(go env GOPATH)/bin/migrate" /out/migrate

FROM alpine:3.19
RUN apk add --no-cache curl
WORKDIR /app
COPY --from=builder /out/gateway .
COPY --from=builder /out/migrate .
COPY infra/migrations ./infra/migrations
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
