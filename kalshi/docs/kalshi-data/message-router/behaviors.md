# Behaviors

Routing logic, parsing, and timestamp conversion for Message Router.

---

## Message Routing

```go
func (r *router) route(raw RawMessage) {
    msgType, err := r.extractType(raw.Data)
    if err != nil {
        r.logger.Warn("failed to extract message type", "err", err)
        r.metrics.ParseErrors.Inc()
        return
    }

    var sent bool

    switch msgType {
    case "orderbook_snapshot":
        msg, err := r.parseOrderbookSnapshot(raw)
        if err != nil {
            r.logger.Warn("failed to parse orderbook snapshot", "err", err)
            r.metrics.ParseErrors.Inc()
            return
        }
        sent = r.sendOrderbook(msg)

    case "orderbook_delta":
        msg, err := r.parseOrderbookDelta(raw)
        if err != nil {
            r.logger.Warn("failed to parse orderbook delta", "err", err)
            r.metrics.ParseErrors.Inc()
            return
        }
        sent = r.sendOrderbook(msg)

    case "trade":
        msg, err := r.parseTrade(raw)
        if err != nil {
            r.logger.Warn("failed to parse trade", "err", err)
            r.metrics.ParseErrors.Inc()
            return
        }
        sent = r.sendTrade(msg)

    case "ticker":
        msg, err := r.parseTicker(raw)
        if err != nil {
            r.logger.Warn("failed to parse ticker", "err", err)
            r.metrics.ParseErrors.Inc()
            return
        }
        sent = r.sendTicker(msg)

    default:
        r.logger.Warn("unknown message type", "type", msgType)
        r.metrics.UnknownMessages.Inc()
        return
    }

    if sent {
        r.metrics.MessagesRouted.WithLabelValues(msgType).Inc()
    }
}
```

---

## Type Extraction

Fast path: extract `type` field without full JSON parse.

```go
func (r *router) extractType(data []byte) (string, error) {
    // Quick scan for "type": pattern
    var envelope struct {
        Type string `json:"type"`
    }
    if err := json.Unmarshal(data, &envelope); err != nil {
        return "", err
    }
    return envelope.Type, nil
}
```

---

## Parsing

### Orderbook Snapshot

```go
type orderbookSnapshotWire struct {
    Type string `json:"type"`
    SID  int64  `json:"sid"`
    Seq  int64  `json:"seq"`
    Msg  struct {
        MarketTicker string          `json:"market_ticker"`
        YesDollars   [][]interface{} `json:"yes_dollars"`   // [["0.52", qty], ...]
        NoDollars    [][]interface{} `json:"no_dollars"`
    } `json:"msg"`
}

func (r *router) parseOrderbookSnapshot(raw RawMessage) (OrderbookMsg, error) {
    var wire orderbookSnapshotWire
    if err := json.Unmarshal(raw.Data, &wire); err != nil {
        return OrderbookMsg{}, err
    }

    return OrderbookMsg{
        Type:       "snapshot",
        Ticker:     wire.Msg.MarketTicker,
        SID:        wire.SID,
        Seq:        wire.Seq,
        ReceivedAt: raw.ReceivedAt,
        SeqGap:     raw.SeqGap,
        GapSize:    raw.GapSize,
        Yes:        parsePriceLevels(wire.Msg.YesDollars),
        No:         parsePriceLevels(wire.Msg.NoDollars),
    }, nil
}

// parsePriceLevels converts [["0.52", 100], ["0.51", 200]] to []PriceLevel
func parsePriceLevels(levels [][]interface{}) []PriceLevel {
    result := make([]PriceLevel, 0, len(levels))
    for _, level := range levels {
        if len(level) < 2 {
            continue
        }
        dollars, _ := level[0].(string)
        qty, _ := level[1].(float64)
        result = append(result, PriceLevel{
            Dollars:  dollars,
            Quantity: int(qty),
        })
    }
    return result
}
```

### Orderbook Delta

```go
type orderbookDeltaWire struct {
    Type string `json:"type"`
    SID  int64  `json:"sid"`
    Seq  int64  `json:"seq"`
    Msg  struct {
        MarketTicker string `json:"market_ticker"`
        PriceDollars string `json:"price_dollars"`  // e.g. "0.52" or "0.5250"
        Delta        int    `json:"delta"`
        Side         string `json:"side"`
        Ts           int64  `json:"ts"`
    } `json:"msg"`
}

func (r *router) parseOrderbookDelta(raw RawMessage) (OrderbookMsg, error) {
    var wire orderbookDeltaWire
    if err := json.Unmarshal(raw.Data, &wire); err != nil {
        return OrderbookMsg{}, err
    }

    return OrderbookMsg{
        Type:         "delta",
        Ticker:       wire.Msg.MarketTicker,
        SID:          wire.SID,
        Seq:          wire.Seq,
        ReceivedAt:   raw.ReceivedAt,
        SeqGap:       raw.SeqGap,
        GapSize:      raw.GapSize,
        PriceDollars: wire.Msg.PriceDollars,
        Delta:        wire.Msg.Delta,
        Side:         wire.Msg.Side,
        ExchangeTs:   wire.Msg.Ts * 1_000_000,  // seconds → microseconds
    }, nil
}
```

### Trade

```go
type tradeWire struct {
    Type string `json:"type"`
    SID  int64  `json:"sid"`
    Seq  int64  `json:"seq"`
    Msg  struct {
        MarketTicker    string `json:"market_ticker"`
        TradeID         string `json:"trade_id"`
        Count           int    `json:"count"`           // We store as "size"
        YesPriceDollars string `json:"yes_price_dollars"`
        NoPriceDollars  string `json:"no_price_dollars"`
        TakerSide       string `json:"taker_side"`
        Ts              int64  `json:"ts"`
    } `json:"msg"`
}

func (r *router) parseTrade(raw RawMessage) (TradeMsg, error) {
    var wire tradeWire
    if err := json.Unmarshal(raw.Data, &wire); err != nil {
        return TradeMsg{}, err
    }

    return TradeMsg{
        Ticker:          wire.Msg.MarketTicker,
        TradeID:         wire.Msg.TradeID,
        Size:            wire.Msg.Count,  // Kalshi: "count" → internal: "size"
        YesPriceDollars: wire.Msg.YesPriceDollars,
        NoPriceDollars:  wire.Msg.NoPriceDollars,
        TakerSide:       wire.Msg.TakerSide,
        SID:             wire.SID,
        Seq:             wire.Seq,
        ExchangeTs:      wire.Msg.Ts * 1_000_000,
        ReceivedAt:      raw.ReceivedAt,
    }, nil
}
```

### Ticker

```go
type tickerWire struct {
    Type string `json:"type"`
    SID  int64  `json:"sid"`
    Msg  struct {
        MarketTicker       string `json:"market_ticker"`
        PriceDollars       string `json:"price_dollars"`
        YesBidDollars      string `json:"yes_bid_dollars"`
        YesAskDollars      string `json:"yes_ask_dollars"`
        NoBidDollars       string `json:"no_bid_dollars"`
        Volume             int64  `json:"volume"`
        OpenInterest       int64  `json:"open_interest"`
        DollarVolume       int64  `json:"dollar_volume"`
        DollarOpenInterest int64  `json:"dollar_open_interest"`
        Ts                 int64  `json:"ts"`
    } `json:"msg"`
}

func (r *router) parseTicker(raw RawMessage) (TickerMsg, error) {
    var wire tickerWire
    if err := json.Unmarshal(raw.Data, &wire); err != nil {
        return TickerMsg{}, err
    }

    return TickerMsg{
        Ticker:             wire.Msg.MarketTicker,
        PriceDollars:       wire.Msg.PriceDollars,
        YesBidDollars:      wire.Msg.YesBidDollars,
        YesAskDollars:      wire.Msg.YesAskDollars,
        NoBidDollars:       wire.Msg.NoBidDollars,
        Volume:             wire.Msg.Volume,
        OpenInterest:       wire.Msg.OpenInterest,
        DollarVolume:       wire.Msg.DollarVolume,
        DollarOpenInterest: wire.Msg.DollarOpenInterest,
        SID:                wire.SID,
        ExchangeTs:         wire.Msg.Ts * 1_000_000,
        ReceivedAt:         raw.ReceivedAt,
        // Note: ticker has no Seq field
    }, nil
}
```

---

## Timestamp Conversion

Kalshi sends timestamps as Unix seconds. We convert to microseconds for consistency with database storage.

```go
// Kalshi: Unix seconds (10 digits)
// Storage: Microseconds (16 digits)
exchangeTs := wire.Msg.Ts * 1_000_000
```

| Source | Format | Example |
|--------|--------|---------|
| Kalshi `ts` | Unix seconds | `1705328200` |
| Internal | Microseconds | `1705328200000000` |

**Two timestamps per message:**

| Timestamp | Source | Purpose |
|-----------|--------|---------|
| `ExchangeTs` | Kalshi's `ts` field (converted) | Event ordering, deduplication |
| `ReceivedAt` | Connection Manager | Latency measurement |

---

## Non-Blocking Sends

Avoid blocking the router if a writer is slow. Returns `true` if sent, `false` if dropped.

```go
func (r *router) sendOrderbook(msg OrderbookMsg) bool {
    select {
    case r.orderbookCh <- msg:
        return true
    default:
        r.logger.Warn("orderbook buffer full, dropping message",
            "ticker", msg.Ticker,
            "type", msg.Type,
        )
        r.metrics.DroppedMessages.WithLabelValues("orderbook").Inc()
        return false
    }
}

func (r *router) sendTrade(msg TradeMsg) bool {
    select {
    case r.tradeCh <- msg:
        return true
    default:
        r.logger.Warn("trade buffer full, dropping message",
            "ticker", msg.Ticker,
            "trade_id", msg.TradeID,
        )
        r.metrics.DroppedMessages.WithLabelValues("trade").Inc()
        return false
    }
}

func (r *router) sendTicker(msg TickerMsg) bool {
    select {
    case r.tickerCh <- msg:
        return true
    default:
        r.logger.Warn("ticker buffer full, dropping message",
            "ticker", msg.Ticker,
        )
        r.metrics.DroppedMessages.WithLabelValues("ticker").Inc()
        return false
    }
}
```

**On buffer full:** Log warning with message details, increment metric, return `false`. Do NOT block.

---

## Price Conversion

**Router does NOT convert prices.** It passes through `*_dollars` strings unchanged.

```
Kalshi API: price_dollars = "0.52" or "0.5250"
     ↓
Router:    PriceDollars = "0.52" (pass-through)
     ↓
Writer:    Converts to internal format (hundred-thousandths)
     ↓
DB:        price = 52000 (for "0.52") or 52500 (for "0.5250")
```

**Why Writers handle conversion:**
- Writers own the DB schema and know the internal format
- Subpenny precision (4+ decimal places) requires string parsing
- Centralized conversion logic in each Writer

**Conversion formula (at Writer):**
```go
// "0.52" → 52000, "0.5250" → 52500
func dollarsToInternal(dollars string) int {
    f, _ := strconv.ParseFloat(dollars, 64)
    return int(f * 100000)
}
```

---

## Sequence Gap Pass-Through

Sequence gaps are detected by Connection Manager. Router passes this info to Writers:

```go
// From RawMessage (Connection Manager)
raw.SeqGap   // true if gap before this message
raw.GapSize  // number of missed messages

// Copied to all message types
msg.SeqGap = raw.SeqGap
msg.GapSize = raw.GapSize
```

Writers can use this to trigger recovery (e.g., request REST snapshot).
