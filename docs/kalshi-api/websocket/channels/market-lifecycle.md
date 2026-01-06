# Market Lifecycle Channel

Channel: `market_lifecycle_v2`

Market state changes and event creation notifications.

## Subscription

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {
    "channels": ["market_lifecycle_v2"],
    "market_ticker": "MARKET-TICKER"
  }
}
```

Market specification optional - omit to receive all lifecycle events.

**Auth**: Required

## Message

```json
{
  "type": "market_lifecycle_v2",
  "sid": 1,
  "seq": 1,
  "msg": {
    "market_ticker": "INXD-23SEP14-B4487",
    "event_type": "created",
    "open_ts": 1694635200,
    "close_ts": 1694721600,
    "determination_ts": 1694721600,
    "result": ""
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `market_lifecycle_v2` |
| `sid` | int | Subscription ID |
| `seq` | int | Sequence number |
| `msg.market_ticker` | string | Market ticker |
| `msg.event_type` | string | Event type (see below) |
| `msg.open_ts` | int | Market open timestamp (Unix seconds) |
| `msg.close_ts` | int | Market close timestamp (Unix seconds) |
| `msg.determination_ts` | int | Determination timestamp (Unix seconds) |
| `msg.result` | string | `yes`, `no`, `scalar`, or empty |

### Event Types

| Type | Description |
|------|-------------|
| `created` | New market created |
| `activated` | Market activated |
| `deactivated` | Market deactivated |
| `close_date_updated` | Close date changed |
| `determined` | Market outcome determined |
| `settled` | Market settled |

## Event Lifecycle Channel

The subscription also receives event-level lifecycle messages:

```json
{
  "type": "event_lifecycle",
  "sid": 5,
  "msg": {
    "event_ticker": "INXD-23SEP14",
    "title": "Event title",
    "series_ticker": "INXD"
  }
}
```
