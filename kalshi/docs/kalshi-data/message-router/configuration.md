# Configuration

Config options and metrics for Message Router.

---

## Config Struct

```go
type RouterConfig struct {
    // Output buffer sizes
    OrderbookBufferSize int  // 5000
    TradeBufferSize     int  // 1000
    TickerBufferSize    int  // 1000
}
```

---

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `OrderbookBufferSize` | int | 5000 | Buffer size for orderbook channel to Writer |
| `TradeBufferSize` | int | 1000 | Buffer size for trade channel to Writer |
| `TickerBufferSize` | int | 1000 | Buffer size for ticker channel to Writer |

**Buffer sizing rationale:**
- Orderbook has highest volume (snapshots + deltas per market)
- Trade and ticker are global channels with lower volume
- Buffers absorb temporary writer slowdowns

### Peak Load Estimates

| Data Type | Peak Rate | Buffer | Headroom |
|-----------|-----------|--------|----------|
| Orderbook deltas | ~1000/sec | 5000 | 5 seconds |
| Trades | ~100/sec | 1000 | 10 seconds |
| Tickers | ~100/sec | 1000 | 10 seconds |

Buffers are sized to handle ~5-10 seconds of backpressure from slow Writers.

---

## Buffer Overflow Handling

### Overflow Behavior

When a buffer is full, the router uses **non-blocking sends**:

```go
select {
case r.orderbookCh <- msg:
    return true  // Sent successfully
default:
    r.logger.Warn("orderbook buffer full, dropping message", ...)
    r.metrics.DroppedMessages.WithLabelValues("orderbook").Inc()
    return false  // Dropped
}
```

**Key behaviors:**
1. **No blocking** - Router never waits; it immediately drops and continues
2. **Logging** - Each drop is logged with message details (ticker, type, trade_id)
3. **Metrics** - `router_messages_dropped_total` increments per drop
4. **Silent data loss** - Dropped messages are NOT retried or queued

### Data Recovery

Dropped messages cause temporary data gaps. Recovery depends on data type:

| Data Type | Recovery Mechanism | Resolution |
|-----------|--------------------|------------|
| **Orderbook deltas** | REST snapshot polling | 15-minute worst-case gap |
| **Orderbook snapshots** | REST snapshot polling | Next poll cycle |
| **Trades** | Other gatherers (deduplicator) | Immediate via redundancy |
| **Tickers** | Next ticker message | Seconds (continuous updates) |

### Why This Design?

1. **Non-blocking is critical** - Blocking the router would cascade backpressure to WebSocket reads, causing socket buffer overflows and connection drops
2. **Redundancy is sufficient** - 3 independent gatherers mean a drop on one is covered by others
3. **Periodic polling fills gaps** - REST snapshot poller provides 15-minute resolution backup
4. **Drops are rare** - Under normal load, buffers never fill; drops indicate system issues

### Monitoring Drop Rates

**Alert thresholds:**

| Condition | Severity | Action |
|-----------|----------|--------|
| `dropped > 0` over 5 min | Warning | Investigate writer performance |
| `dropped > 100` over 5 min | Critical | Check DB, increase buffer, scale writers |
| Continuous drops | Critical | System cannot keep up; reduce subscription count |

---

## Error Handling

| Error | Behavior |
|-------|----------|
| JSON parse error | Log warning, increment metric, skip message |
| Unknown message type | Log warning, increment metric, skip message |
| Output buffer full | Log warning, increment metric, drop message |

No retries. Dropped messages are acceptable because:
- REST snapshot polling provides backup (15-minute resolution)
- Deduplicator pulls from 3 independent gatherers
- `ON CONFLICT DO NOTHING` handles any duplicates

---

## Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `router_messages_received_total` | Counter | `type` | Messages received from Connection Manager |
| `router_messages_routed_total` | Counter | `type` | Messages successfully routed to Writers |
| `router_messages_dropped_total` | Counter | `type` | Messages dropped due to buffer full |
| `router_parse_errors_total` | Counter | `type` | JSON parse failures |
| `router_unknown_messages_total` | Counter | - | Messages with unknown type |

### Labels

| Metric | Labels |
|--------|--------|
| `messages_received_total` | `type` (orderbook_snapshot, orderbook_delta, trade, ticker) |
| `messages_routed_total` | `type` |
| `messages_dropped_total` | `type` (orderbook, trade, ticker) |
| `parse_errors_total` | `type` |

---

## Example Prometheus Queries

```promql
# Routing success rate
sum(rate(router_messages_routed_total[5m])) /
sum(rate(router_messages_received_total[5m]))

# Drop rate by type
rate(router_messages_dropped_total[5m])

# Parse error rate
rate(router_parse_errors_total[5m])
```

---

## Tuning

### Buffer Full

If `router_messages_dropped_total` is increasing:

1. **Check writer health** - Is the downstream Writer keeping up?
2. **Increase buffer size** - Add headroom for bursts
3. **Check DB performance** - Writers may be blocked on slow inserts

### High Parse Errors

If `router_parse_errors_total` is increasing:

1. **Check Kalshi API changes** - Schema may have changed
2. **Review error logs** - See which message types are failing
3. **Update parsing code** - Handle new fields or format changes
