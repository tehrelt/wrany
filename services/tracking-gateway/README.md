# tracking-gateway

Go WebSocket gateway service. Accepts location events from Android tracker clients and writes them to Kafka.

## Port

`8080` (host-exposed)

## Environment Variables

| Variable     | Default | Description        |
|--------------|---------|--------------------|
| GATEWAY_PORT | 8080    | HTTP listener port |

## Health Check

```bash
curl localhost:8080/healthz
# {"status":"ok"}
```
