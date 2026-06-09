FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker

FROM alpine:3.19
RUN apk add --no-cache curl
WORKDIR /app
COPY --from=builder /app/worker .
EXPOSE 8081
ENTRYPOINT ["./worker"]
