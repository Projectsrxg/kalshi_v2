# Poller Package

Snapshot Poller - polls REST API for orderbook snapshots as backup.

## Purpose

Provides backup data source for:
- Gap recovery after WebSocket disconnections
- Verification of orderbook state
- Historical snapshot archive

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `interval` | 15m | Polling interval |
| `concurrency` | 10 | Concurrent REST requests |

## Usage

```go
p := poller.New(cfg.Poller, apiClient, db)
p.Start(ctx)
```

## Data

Snapshots are stored with `source="rest"` to distinguish from WebSocket-derived data.
