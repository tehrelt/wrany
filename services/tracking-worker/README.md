# tracking-worker

Go background worker service. Consumes location events from NATS JetStream and runs trip detection, route matching, and personal record logic.

## Port

`8081` (host-exposed for health checks)

## Environment Variables

| Variable    | Default | Description        |
|-------------|---------|--------------------|
| WORKER_PORT | 8081    | HTTP listener port |

## Health Check

```bash
curl localhost:8081/healthz
# {"status":"ok"}
```
