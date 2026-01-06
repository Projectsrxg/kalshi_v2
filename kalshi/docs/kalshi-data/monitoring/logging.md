# Logging

Structured logging standards for the Kalshi Data Platform.

---

## Overview

All components use Go's `slog` package for structured logging:
- JSON format in production
- Text format for local development
- Consistent field naming across components

---

## Logger Configuration

### Production

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)
```

Output:
```json
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"snapshot poller started","component":"snapshot-poller","poll_interval":"1m0s"}
```

### Development

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
slog.SetDefault(logger)
```

Output:
```
time=2024-01-15T10:30:00Z level=INFO msg="snapshot poller started" component=snapshot-poller poll_interval=1m0s
```

---

## Log Levels

| Level | When to Use | Examples |
|-------|-------------|----------|
| `ERROR` | Failures requiring attention | DB connection failed, critical component crashed |
| `WARN` | Degraded but operational | Message dropped, retry needed, sequence gap |
| `INFO` | Significant state changes | Component started/stopped, config loaded |
| `DEBUG` | Detailed operational info | Message processed, batch flushed |

### Level Guidelines

**ERROR** - Something is broken and needs immediate attention:
```go
logger.Error("database connection failed",
    "err", err,
    "host", cfg.DBHost,
)
```

**WARN** - Something unexpected but system continues:
```go
logger.Warn("message buffer full, dropping message",
    "ticker", ticker,
    "buffer_size", bufferSize,
)
```

**INFO** - Normal operations worth noting:
```go
logger.Info("writer started",
    "writer", "trade",
    "batch_size", cfg.BatchSize,
)
```

**DEBUG** - Verbose details for troubleshooting:
```go
logger.Debug("batch flushed",
    "writer", "orderbook",
    "rows", len(batch),
    "duration", duration,
)
```

---

## Standard Fields

All logs must include the `component` field:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `component` | string | Yes | Component name |
| `err` | error | On errors | Error details |
| `ticker` | string | If applicable | Market ticker |
| `duration` | duration | On operations | How long it took |

### Component Names

| Component | Value |
|-----------|-------|
| Market Registry | `market-registry` |
| Connection Manager | `connection-manager` |
| WebSocket Client | `websocket-client` |
| Message Router | `message-router` |
| Orderbook Writer | `writer-orderbook` |
| Trade Writer | `writer-trade` |
| Ticker Writer | `writer-ticker` |
| Snapshot Writer | `writer-snapshot` |
| Snapshot Poller | `snapshot-poller` |
| Deduplicator | `deduplicator` |

### Creating Component Logger

```go
type TradeWriter struct {
    logger *slog.Logger
    // ...
}

func NewTradeWriter(cfg Config) *TradeWriter {
    return &TradeWriter{
        logger: slog.Default().With("component", "writer-trade"),
        // ...
    }
}
```

---

## Common Log Patterns

### Startup

```go
logger.Info("component started",
    "config_key", cfg.Value,
)
```

### Shutdown

```go
logger.Info("component stopping")
// ... cleanup ...
logger.Info("component stopped")
```

### Error with Context

```go
logger.Error("failed to fetch orderbook",
    "ticker", ticker,
    "err", err,
    "attempt", attempt,
)
```

### Operation Complete

```go
logger.Debug("batch inserted",
    "rows", len(batch),
    "conflicts", conflicts,
    "duration", time.Since(start),
)
```

### Periodic Status

```go
logger.Info("poll cycle complete",
    "markets", len(markets),
    "fetched", fetched,
    "errors", errors,
    "duration", duration,
)
```

---

## Field Naming Conventions

| Convention | Examples |
|------------|----------|
| Snake case | `poll_interval`, `batch_size` |
| Units in name | `duration_ms`, `size_bytes` |
| Boolean as adjective | `is_healthy`, `has_error` |
| Counts as plural | `rows`, `errors`, `markets` |

### Common Fields

| Field | Type | Usage |
|-------|------|-------|
| `ticker` | string | Market identifier |
| `event_ticker` | string | Event identifier |
| `sid` | int64 | Subscription ID |
| `seq` | int64 | Sequence number |
| `err` | error | Error object |
| `duration` | duration | Time elapsed |
| `count` | int | Generic count |
| `rows` | int | Database rows |
| `attempt` | int | Retry attempt number |

---

## Error Logging

### Do

```go
// Include context
logger.Error("batch insert failed",
    "writer", "trade",
    "rows", len(batch),
    "err", err,
)

// Use structured error
logger.Warn("sequence gap detected",
    "ticker", ticker,
    "expected", expected,
    "got", got,
    "gap_size", got - expected,
)
```

### Don't

```go
// Don't use fmt.Sprintf
logger.Error(fmt.Sprintf("failed to insert %d rows: %v", len(batch), err))

// Don't log and return error (pick one)
logger.Error("insert failed", "err", err)
return err  // Caller will also log
```

---

## Log Sampling

For high-frequency operations, use sampling to reduce log volume:

```go
var logCounter atomic.Int64

func (w *Writer) processMessage(msg Message) {
    // Log every 1000th message at DEBUG
    if logCounter.Add(1) % 1000 == 0 {
        w.logger.Debug("processed messages",
            "count", 1000,
        )
    }
}
```

---

## Log Aggregation

### Log Shipping

Logs are shipped to CloudWatch using one of these methods:

| Method | Use Case | Configuration |
|--------|----------|---------------|
| CloudWatch Agent | EC2 instances | `/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json` |
| Fluent Bit | Containers/ECS | Sidecar container with CloudWatch output plugin |
| stdout capture | ECS/Fargate | Task definition `awslogs` driver |

**CloudWatch Agent Config Example:**

```json
{
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/kalshi-gatherer.log",
            "log_group_name": "/kalshi/gatherers",
            "log_stream_name": "{instance_id}",
            "timezone": "UTC"
          }
        ]
      }
    }
  }
}
```

**Fluent Bit Config Example:**

```ini
[OUTPUT]
    Name              cloudwatch_logs
    Match             *
    region            us-east-1
    log_group_name    /kalshi/gatherers
    log_stream_prefix gatherer-
    auto_create_group true
```

### CloudWatch Logs Queries

```bash
# Filter by component
{ $.component = "snapshot-poller" }

# Filter by level
{ $.level = "ERROR" }

# Filter by ticker
{ $.ticker = "AAPL-YES" }
```

### Log Insights Query

```sql
fields @timestamp, @message
| filter component = "writer-trade"
| filter level = "ERROR"
| sort @timestamp desc
| limit 100
```

---

## Sensitive Data

Never log:
- API keys or tokens
- Full orderbook data (too verbose)
- Personal user information

Acceptable:
- Market tickers (public)
- Prices (public)
- Counts and aggregates
- Error messages

```go
// Bad - logs full response body
logger.Debug("received response", "body", string(body))

// Good - logs summary
logger.Debug("received orderbook",
    "ticker", ticker,
    "yes_levels", len(ob.Yes),
    "no_levels", len(ob.No),
)
```

---

## Testing Logs

Capture logs in tests:

```go
func TestWriter(t *testing.T) {
    var buf bytes.Buffer
    logger := slog.New(slog.NewJSONHandler(&buf, nil))

    w := NewWriter(cfg)
    w.logger = logger

    // ... run test ...

    // Assert log contents
    assert.Contains(t, buf.String(), "batch inserted")
}
```
