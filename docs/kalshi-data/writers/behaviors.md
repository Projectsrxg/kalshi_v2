# Behaviors

Price conversion, batching, and insert patterns for Writers.

---

## Price Conversion

Writers are responsible for converting `*_dollars` strings to internal integer format.

### Conversion Formula

```go
// Router passes: "0.52" or "0.5250" (subpenny)
// Writer converts to: 52000 or 52500
func dollarsToInternal(dollars string) int {
    f, _ := strconv.ParseFloat(dollars, 64)
    return int(f * 100000)
}
```

### Conversion Examples

| Input (dollars) | Output (internal) | Notes |
|-----------------|-------------------|-------|
| `"0.52"` | `52000` | Standard penny pricing |
| `"0.5250"` | `52500` | Subpenny (0.1 cent) |
| `"0.5255"` | `52550` | Subpenny (0.01 cent) |
| `"0.99"` | `99000` | Near 100% |
| `"0.9999"` | `99990` | Tail pricing |
| `"0.01"` | `1000` | Near 0% |

### Pipeline

```
Kalshi API: price_dollars = "0.52" or "0.5250"
     ↓
Router:    PriceDollars = "0.52" (pass-through string)
     ↓
Writer:    dollarsToInternal("0.52") = 52000
     ↓
DB:        price = 52000 (INTEGER column)
```

---

## Side Conversion

Convert string sides to booleans for database storage:

```go
func sideToBoolean(side string) bool {
    return side == "yes"  // TRUE = yes, FALSE = no
}
```

---

## Field Mappings

### Trade Writer

| Router Field | Conversion | DB Column | Type |
|--------------|------------|-----------|------|
| `TradeID` | pass-through | `trade_id` | UUID |
| `ExchangeTs` | pass-through | `exchange_ts` | BIGINT |
| `ReceivedAt` | `.UnixMicro()` | `received_at` | BIGINT |
| `Ticker` | pass-through | `ticker` | VARCHAR |
| `YesPriceDollars` | `dollarsToInternal()` | `price` | INTEGER |
| `Size` | pass-through | `size` | INTEGER |
| `TakerSide` | `sideToBoolean()` | `taker_side` | BOOLEAN |
| `SID` | pass-through | `sid` | BIGINT |

```go
func (w *TradeWriter) transform(msg TradeMsg) tradeRow {
    return tradeRow{
        TradeID:    msg.TradeID,
        ExchangeTs: msg.ExchangeTs,
        ReceivedAt: msg.ReceivedAt.UnixMicro(),
        Ticker:     msg.Ticker,
        Price:      dollarsToInternal(msg.YesPriceDollars),
        Size:       msg.Size,
        TakerSide:  sideToBoolean(msg.TakerSide),
        SID:        msg.SID,
    }
}
```

**Notes:**
- `NoPriceDollars` is not stored separately. In binary markets, YES price + NO price = $1.00, so NO price can be derived: `100000 - price`.
- `event_ticker` exists in schema but is not populated by gatherer (nullable, for future use).

### Orderbook Delta

| Router Field | Conversion | DB Column | Type |
|--------------|------------|-----------|------|
| `ExchangeTs` | pass-through | `exchange_ts` | BIGINT |
| `ReceivedAt` | `.UnixMicro()` | `received_at` | BIGINT |
| `Seq` | pass-through | `seq` | BIGINT |
| `Ticker` | pass-through | `ticker` | VARCHAR |
| `Side` | `sideToBoolean()` | `side` | BOOLEAN |
| `PriceDollars` | `dollarsToInternal()` | `price` | INTEGER |
| `Delta` | pass-through | `size_delta` | INTEGER |
| `SID` | pass-through | `sid` | BIGINT |

```go
func (w *OrderbookWriter) transformDelta(msg OrderbookMsg) orderbookDeltaRow {
    return orderbookDeltaRow{
        ExchangeTs: msg.ExchangeTs,
        ReceivedAt: msg.ReceivedAt.UnixMicro(),
        Seq:        msg.Seq,
        Ticker:     msg.Ticker,
        Side:       sideToBoolean(msg.Side),
        Price:      dollarsToInternal(msg.PriceDollars),
        SizeDelta:  msg.Delta,
        SID:        msg.SID,
    }
}
```

### Orderbook Snapshot (WebSocket)

Kalshi API provides **bids only** per side. Asks are derived from the opposite side's bids:
- YES bid at price X = NO ask at price (100000 - X)
- NO bid at price X = YES ask at price (100000 - X)

| Router Field | Conversion | DB Column | Type |
|--------------|------------|-----------|------|
| `ReceivedAt` | `.UnixMicro()` | `snapshot_ts` | BIGINT |
| - | `0` | `exchange_ts` | BIGINT |
| `Ticker` | pass-through | `ticker` | VARCHAR |
| - | `"ws"` | `source` | VARCHAR |
| `Yes` | `priceLevelsToJSONB()` | `yes_bids` | JSONB |
| `No` (derived) | `deriveAsksFromBids()` | `yes_asks` | JSONB |
| `No` | `priceLevelsToJSONB()` | `no_bids` | JSONB |
| `Yes` (derived) | `deriveAsksFromBids()` | `no_asks` | JSONB |
| `SID` | pass-through | `sid` | BIGINT |

```go
func (w *OrderbookWriter) transformSnapshot(msg OrderbookMsg) orderbookSnapshotRow {
    yesBids := priceLevelsToJSONB(msg.Yes)
    noBids := priceLevelsToJSONB(msg.No)

    // Derive asks from opposite bids
    // YES bid at price X means NO ask at (100000 - X)
    yesAsks := deriveAsksFromBids(msg.No)   // NO bids → YES asks
    noAsks := deriveAsksFromBids(msg.Yes)   // YES bids → NO asks

    bestYesBid := extractBestPrice(msg.Yes)
    bestYesAsk := extractBestAskFromBids(msg.No)  // Best NO bid → Best YES ask

    return orderbookSnapshotRow{
        SnapshotTs:  msg.ReceivedAt.UnixMicro(),
        ExchangeTs:  0,  // WS snapshots don't have exchange timestamp
        Ticker:      msg.Ticker,
        Source:      "ws",
        YesBids:     yesBids,
        YesAsks:     yesAsks,
        NoBids:      noBids,
        NoAsks:      noAsks,
        BestYesBid:  bestYesBid,
        BestYesAsk:  bestYesAsk,
        Spread:      bestYesAsk - bestYesBid,
        SID:         msg.SID,
    }
}

func priceLevelsToJSONB(levels []PriceLevel) []byte {
    result := make([]map[string]int, len(levels))
    for i, level := range levels {
        result[i] = map[string]int{
            "price": dollarsToInternal(level.Dollars),
            "size":  level.Quantity,
        }
    }
    data, _ := json.Marshal(result)
    return data
}

// deriveAsksFromBids converts bids to asks on the opposite side
// YES bid at X = NO ask at (100000 - X)
func deriveAsksFromBids(bids []PriceLevel) []byte {
    asks := make([]map[string]int, len(bids))
    for i, bid := range bids {
        bidPrice := dollarsToInternal(bid.Dollars)
        askPrice := 100000 - bidPrice
        asks[i] = map[string]int{
            "price": askPrice,
            "size":  bid.Quantity,
        }
    }
    data, _ := json.Marshal(asks)
    return data
}

func extractBestPrice(levels []PriceLevel) int {
    if len(levels) == 0 {
        return 0
    }
    return dollarsToInternal(levels[0].Dollars)
}

func extractBestAskFromBids(bids []PriceLevel) int {
    if len(bids) == 0 {
        return 0
    }
    // Best bid on opposite side = best ask
    return 100000 - dollarsToInternal(bids[0].Dollars)
}
```

### Ticker

| Router Field | Conversion | DB Column | Type |
|--------------|------------|-----------|------|
| `ExchangeTs` | pass-through | `exchange_ts` | BIGINT |
| `ReceivedAt` | `.UnixMicro()` | `received_at` | BIGINT |
| `Ticker` | pass-through | `ticker` | VARCHAR |
| `YesBidDollars` | `dollarsToInternal()` | `yes_bid` | INTEGER |
| `YesAskDollars` | `dollarsToInternal()` | `yes_ask` | INTEGER |
| `PriceDollars` | `dollarsToInternal()` | `last_price` | INTEGER |
| `Volume` | pass-through | `volume` | BIGINT |
| `OpenInterest` | pass-through | `open_interest` | BIGINT |
| `DollarVolume` | pass-through | `dollar_volume` | BIGINT |
| `DollarOpenInterest` | pass-through | `dollar_open_interest` | BIGINT |
| `SID` | pass-through | `sid` | BIGINT |

```go
func (w *TickerWriter) transform(msg TickerMsg) tickerRow {
    return tickerRow{
        ExchangeTs:         msg.ExchangeTs,
        ReceivedAt:         msg.ReceivedAt.UnixMicro(),
        Ticker:             msg.Ticker,
        YesBid:             dollarsToInternal(msg.YesBidDollars),
        YesAsk:             dollarsToInternal(msg.YesAskDollars),
        LastPrice:          dollarsToInternal(msg.PriceDollars),
        Volume:             msg.Volume,
        OpenInterest:       msg.OpenInterest,
        DollarVolume:       msg.DollarVolume,
        DollarOpenInterest: msg.DollarOpenInterest,
        SID:                msg.SID,
    }
}
```

**Note:** `NoBidDollars` is not stored separately. In binary markets, YES bid + NO bid = $1.00, so NO bid can be derived: `100000 - YesBid`.

---

## Batch Writing

Writers accumulate messages into batches before inserting.

### Flush Logic

```go
func (w *TradeWriter) flush() {
    if len(w.batch) == 0 {
        return
    }

    start := time.Now()

    err := w.batchInsert(w.batch)
    if err != nil {
        w.logger.Error("batch insert failed", "err", err, "count", len(w.batch))
        w.metrics.Errors.Inc()
    } else {
        w.metrics.Inserts.Add(float64(len(w.batch)))
    }

    w.metrics.FlushDuration.Observe(time.Since(start).Seconds())
    w.metrics.BatchSize.Observe(float64(len(w.batch)))

    // Reset batch
    w.batch = w.batch[:0]
}
```

### Batch Insert Pattern

Use `SendBatch` with `ON CONFLICT DO NOTHING` for deduplication:

```go
func (w *TradeWriter) batchInsert(rows []tradeRow) error {
    batch := &pgx.Batch{}
    for _, r := range rows {
        batch.Queue(`
            INSERT INTO trades (trade_id, exchange_ts, received_at, ticker, price, size, taker_side, sid)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
            ON CONFLICT (trade_id) DO NOTHING
        `, r.TradeID, r.ExchangeTs, r.ReceivedAt, r.Ticker, r.Price, r.Size, r.TakerSide, r.SID)
    }

    results := w.db.SendBatch(w.ctx, batch)
    defer results.Close()

    var conflicts int
    for i := 0; i < len(rows); i++ {
        ct, err := results.Exec()
        if err != nil {
            return err
        }
        if ct.RowsAffected() == 0 {
            conflicts++
        }
    }

    w.metrics.Conflicts.Add(float64(conflicts))
    return nil
}
```

**Why SendBatch, not COPY:** PostgreSQL's `COPY` protocol does not support `ON CONFLICT`. Since we receive data from 3 independent gatherers, duplicates are expected and must be handled gracefully.

---

## Insert Statements

### trades

```sql
INSERT INTO trades (trade_id, exchange_ts, received_at, ticker, price, size, taker_side, sid)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (trade_id) DO NOTHING
```

### orderbook_deltas

```sql
INSERT INTO orderbook_deltas (exchange_ts, received_at, seq, ticker, side, price, size_delta, sid)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (ticker, exchange_ts, seq, price, side) DO NOTHING
```

### orderbook_snapshots

```sql
INSERT INTO orderbook_snapshots (snapshot_ts, exchange_ts, ticker, source, yes_bids, yes_asks, no_bids, no_asks, best_yes_bid, best_yes_ask, spread, sid)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (ticker, snapshot_ts, source) DO NOTHING
```

### tickers

```sql
INSERT INTO tickers (exchange_ts, received_at, ticker, yes_bid, yes_ask, last_price, volume, open_interest, dollar_volume, dollar_open_interest, sid)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (ticker, exchange_ts) DO NOTHING
```

---

## Error Handling

| Error | Behavior |
|-------|----------|
| DB connection error | Log, increment error metric, retry on next flush |
| Constraint violation | Expected for duplicates, increment conflict metric |
| Invalid data | Log warning, skip row (don't fail batch) |

**Important:** Writers never block the Message Router. If inserts fail, data is lost for that batch, but the system continues. Redundancy from 3 gatherers and REST polling provides recovery.

---

## Handling Sequence Gaps

Orderbook Writer tracks `SeqGap` flag from messages:

```go
func (w *OrderbookWriter) handleMessage(msg OrderbookMsg) {
    if msg.SeqGap {
        w.logger.Warn("sequence gap detected",
            "ticker", msg.Ticker,
            "gap_size", msg.GapSize,
        )
        w.metrics.SeqGaps.WithLabelValues(msg.Ticker).Inc()
        // Note: No action needed - REST snapshot polling provides backup
    }

    // Process normally regardless of gap
    w.processDeltaOrSnapshot(msg)
}
```

Writers don't trigger recovery - that's handled by:
1. REST Snapshot Poller (1-minute polling)
2. Deduplicator (merges from 3 gatherers)
