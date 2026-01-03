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

---

## Error Handling

| Error | Behavior |
|-------|----------|
| JSON parse error | Log warning, increment metric, skip message |
| Unknown message type | Log warning, increment metric, skip message |
| Output buffer full | Log warning, increment metric, drop message |

No retries. Dropped messages are acceptable because:
- REST snapshot polling provides backup (1-minute resolution)
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
