# Data Flow

Message lifecycle from Kalshi APIs to production storage.

---

## End-to-End Flow

```mermaid
flowchart LR
    subgraph Kalshi["Kalshi API"]
        REST[REST API]
        WS[WebSocket API]
    end

    subgraph Gatherer["Gatherer Instance"]
        MR[Market Registry]
        CONN[Connection Manager]
        ROUTER[Message Router]
        LOCAL_TS[(TimescaleDB\ntrades/orderbook)]
        LOCAL_PG[(PostgreSQL\nmarkets/events)]
    end

    subgraph Deduplicator
        POLL[Poller]
        DEDUP[Deduplicator]
    end

    subgraph Production
        RDS[(Production RDS)]
        S3[(S3)]
    end

    REST --> MR
    MR --> CONN
    MR --> LOCAL_PG
    WS --> CONN
    CONN --> ROUTER
    ROUTER --> LOCAL_TS
    LOCAL_TS --> POLL
    LOCAL_PG --> POLL
    POLL --> DEDUP
    DEDUP --> RDS
    DEDUP --> S3
```

---

## Stage 0: Market Discovery

The Market Registry uses both REST API and WebSocket for market discovery.

### Exchange Status Check

Before connecting, verify the exchange is operational.

```mermaid
sequenceDiagram
    participant MR as Market Registry
    participant REST as Kalshi REST

    MR->>REST: GET /exchange/status
    REST-->>MR: {exchange_active, trading_active}
    alt exchange_active: false
        MR->>MR: Wait for exchange_estimated_resume_time
        MR->>REST: Retry GET /exchange/status
    end
```

### Initial Sync (REST)

On startup, fetch all markets via REST API.

```mermaid
sequenceDiagram
    participant MR as Market Registry
    participant REST as Kalshi REST
    participant DB as Local TimescaleDB
    participant CM as Connection Manager

    MR->>REST: GET /exchange/status
    REST-->>MR: exchange_active: true
    MR->>REST: GET /markets (paginated)
    REST-->>MR: All markets
    MR->>REST: GET /events (paginated)
    REST-->>MR: All events
    MR->>DB: INSERT markets, events
    MR->>CM: Subscribe to all active markets
    MR->>CM: Subscribe to market_lifecycle channel
```

### Live Updates (WebSocket)

After initial sync, receive real-time market updates via `market_lifecycle` channel.

```mermaid
sequenceDiagram
    participant WS as Kalshi WS
    participant MR as Market Registry
    participant REST as Kalshi REST
    participant DB as Local TimescaleDB
    participant CM as Connection Manager

    WS->>MR: market_lifecycle: created
    MR->>REST: GET /markets/{ticker}
    REST-->>MR: Full market data
    MR->>DB: INSERT market
    MR->>CM: Subscribe to orderbook, trades

    WS->>MR: market_lifecycle: status_change
    MR->>DB: UPDATE market status
    alt Market closed
        MR->>CM: Unsubscribe from orderbook, trades
    end

    WS->>MR: market_lifecycle: settled
    MR->>DB: UPDATE market result
    MR->>CM: Unsubscribe from orderbook, trades
```

### Periodic Reconciliation (REST)

Backup polling to catch any missed WebSocket events.

```mermaid
sequenceDiagram
    participant MR as Market Registry
    participant REST as Kalshi REST
    participant DB as Local TimescaleDB

    loop Every 5-10 minutes
        MR->>REST: GET /markets?status=open
        REST-->>MR: Market list
        MR->>DB: UPSERT markets (reconcile)
    end
```

**Data Sources:**

| Source | Purpose | Frequency |
|--------|---------|-----------|
| REST `GET /markets` | Initial sync, reconciliation | On startup, every 5-10 min |
| REST `GET /events` | Event metadata | Every 10 min |
| REST `GET /markets/{ticker}/orderbook` | Snapshot recovery after gap | On demand |
| WS `market_lifecycle` | New markets, status changes, settlements | Real-time |

**market_lifecycle Event Types:**

| Event | Action |
|-------|--------|
| `created` | Fetch full market via REST, subscribe to data channels |
| `status_change` | Update DB, adjust subscriptions if closed |
| `settled` | Update result, unsubscribe from data channels |

---

## Stage 1: WebSocket Ingestion

```mermaid
sequenceDiagram
    participant K as Kalshi WS
    participant CM as Connection Manager
    participant WS as WebSocket Client
    participant MR as Message Router

    CM->>WS: Connect to wss://api.elections.kalshi.com
    WS->>K: Subscribe to orderbook_delta, trade, ticker
    K-->>WS: Subscription confirmed

    loop Message Stream
        K->>WS: orderbook_delta message
        WS->>MR: Route message
        MR->>MR: Validate sequence number
        MR->>MR: Parse and transform
    end
```

**Message Types:**

| Channel | Data | Table |
|---------|------|-------|
| `orderbook_delta` | Snapshots + deltas | `orderbook_deltas`, `orderbook_snapshots` |
| `trade` | Public trades | `trades` |
| `ticker` | Price/volume updates | `tickers` |

---

## Stage 1.5: REST Snapshot Polling

Every 1 minute, poll orderbook snapshots for all subscribed markets as backup.

```mermaid
sequenceDiagram
    participant SP as Snapshot Poller
    participant MR as Market Registry
    participant REST as Kalshi REST
    participant SW as Snapshot Writer
    participant TS as TimescaleDB

    loop Every 1 minute
        SP->>MR: Get subscribed markets
        MR-->>SP: Market list
        loop For each market
            SP->>REST: GET /markets/{ticker}/orderbook
            REST-->>SP: Orderbook snapshot
            SP->>SW: Write snapshot
            SW->>TS: INSERT (source='rest')
        end
    end
```

**Purpose:** Ensures at least 1-minute resolution orderbook data even if WebSocket deltas are missed.

---

## Stage 2: Local Storage

```mermaid
sequenceDiagram
    participant MR as Message Router
    participant OW as Orderbook Writer
    participant TW as Trade Writer
    participant TK as Ticker Writer
    participant TS as TimescaleDB
    participant PG as PostgreSQL

    MR->>OW: orderbook_delta
    OW->>TS: INSERT ... ON CONFLICT DO NOTHING

    MR->>TW: trade
    TW->>TS: INSERT ... ON CONFLICT DO NOTHING

    MR->>TK: ticker
    TK->>TS: INSERT ... ON CONFLICT DO NOTHING
```

Each gatherer has two local databases:

| Database | Tables | Purpose |
|----------|--------|---------|
| TimescaleDB | trades, orderbook_deltas, orderbook_snapshots, tickers | Time-series data (hypertables) |
| PostgreSQL | series, events, markets | Relational data |

---

## Stage 3: Deduplication

```mermaid
sequenceDiagram
    participant G1 as Gatherer 1
    participant G2 as Gatherer 2
    participant G3 as Gatherer 3
    participant D as Deduplicator
    participant RDS as Production RDS

    loop Every N seconds
        D->>G1: SELECT * WHERE ts > last_sync
        D->>G2: SELECT * WHERE ts > last_sync
        D->>G3: SELECT * WHERE ts > last_sync
        D->>D: Merge and deduplicate
        D->>RDS: INSERT ... ON CONFLICT DO NOTHING
        D->>D: Update last_sync cursor
    end
```

**Deduplication Keys (Kalshi exchange identifiers):**

| Data Type | Unique Key |
|-----------|------------|
| Orderbook deltas | `(ticker, exchange_ts, seq, price, side)` |
| Orderbook snapshots | `(ticker, snapshot_ts, source)` |
| Trades | `trade_id` |
| Tickers | `(ticker, exchange_ts)` |
| Markets | `ticker` |

---

## Stage 4: Cold Storage Export

```mermaid
flowchart LR
    RDS[(Production RDS)] --> EXPORT[Export Job]
    EXPORT --> PARQUET[Parquet Files]
    PARQUET --> S3[(S3)]

    subgraph S3
        RAW[raw/] --> GLACIER[Glacier 30d]
        PROC[processed/] --> IA[IA 90d]
        AGG[aggregates/]
    end
```

Periodic export job writes Parquet files to S3 with lifecycle policies.

---

## Message Formats

### Orderbook Delta (Inbound)

```json
{
  "type": "orderbook_delta",
  "sid": 12345,
  "seq": 100,
  "msg": {
    "market_ticker": "INXD-25JAN10-T4600",
    "price": 56,
    "delta": 100,
    "side": "yes",
    "ts": 1704067200
  }
}
```

### Trade (Inbound)

```json
{
  "type": "trade",
  "sid": 12346,
  "msg": {
    "market_ticker": "INXD-25JAN10-T4600",
    "trade_id": "abc123",
    "count": 50,
    "yes_price": 56,
    "no_price": 44,
    "taker_side": "yes",
    "ts": 1704067200
  }
}
```

---

## Throughput Estimates

| Stage | Throughput |
|-------|------------|
| WebSocket ingestion | 2000 msg/s per gatherer |
| Local DB writes | 2000 inserts/s (batched) |
| Deduplicator sync | Configurable (e.g., every 5s) |
| Production RDS writes | ~6000 inserts/s peak (3 gatherers) |
