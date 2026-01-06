# Market Lifecycle Channel

Channel: `market_lifecycle`

Market state changes and event creation notifications.

## Subscription

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {
    "channels": ["market_lifecycle"],
    "market_ticker": "MARKET-TICKER"
  }
}
```

Market specification optional.

**Auth**: Not required

## Message

```json
{
  "type": "market_lifecycle",
  "sid": 1,
  "msg": {
    "market_ticker": "MARKET-TICKER",
    "event_type": "status_change",
    "old_status": "active",
    "new_status": "closed",
    "result": "",
    "ts": 1705328200
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `market_lifecycle` |
| `sid` | int | Subscription ID |
| `msg.market_ticker` | string | Market ID |
| `msg.event_type` | string | `status_change`, `created`, `settled` |
| `msg.old_status` | string | Previous status |
| `msg.new_status` | string | New status |
| `msg.result` | string | `yes`, `no`, or empty |
| `msg.ts` | int | Unix timestamp (seconds) |

### Event Types

| Type | Description |
|------|-------------|
| `created` | New market created |
| `status_change` | Market status changed |
| `settled` | Market settled |
