# Overall Architecture

Comprehensive view of the Kalshi Data Platform.

---

## System Overview

```mermaid
flowchart TB
    subgraph External["External Services"]
        KALSHI_REST[("Kalshi REST API<br/>api.elections.kalshi.com")]
        KALSHI_WS[("Kalshi WebSocket<br/>wss://api.elections.kalshi.com/trade-api/ws/v2")]
    end

    subgraph AWS["AWS us-east-1"]
        subgraph AZ1["us-east-1a"]
            subgraph G1["Gatherer 1 (EC2 t4g.2xlarge)"]
                G1_MR[Market Registry<br/>In-Memory]
                G1_CM[Connection Manager<br/>150 WebSockets]
                G1_RT[Message Router]
                G1_SP[Snapshot Poller]
                G1_TW[Trade Writer]
                G1_OW[Orderbook Writer]
                G1_KW[Ticker Writer]
                G1_SW[Snapshot Writer]
            end
            G1_TS[("TimescaleDB<br/>:5432<br/>kalshi_ts")]
        end

        subgraph AZ2["us-east-1b"]
            subgraph G2["Gatherer 2 (EC2 t4g.2xlarge)"]
                G2_MR[Market Registry<br/>In-Memory]
                G2_CM[Connection Manager<br/>150 WebSockets]
                G2_RT[Message Router]
                G2_SP[Snapshot Poller]
                G2_TW[Trade Writer]
                G2_OW[Orderbook Writer]
                G2_KW[Ticker Writer]
                G2_SW[Snapshot Writer]
            end
            G2_TS[("TimescaleDB<br/>:5432<br/>kalshi_ts")]
        end

        subgraph AZ3["us-east-1c"]
            subgraph G3["Gatherer 3 (EC2 t4g.2xlarge)"]
                G3_MR[Market Registry<br/>In-Memory]
                G3_CM[Connection Manager<br/>150 WebSockets]
                G3_RT[Message Router]
                G3_SP[Snapshot Poller]
                G3_TW[Trade Writer]
                G3_OW[Orderbook Writer]
                G3_KW[Ticker Writer]
                G3_SW[Snapshot Writer]
            end
            G3_TS[("TimescaleDB<br/>:5432<br/>kalshi_ts")]

            subgraph DEDUP["Deduplicator (EC2 t4g.xlarge)"]
                D_SYNC[Sync Loops<br/>4 time-series tables]
                D_API[API Sync<br/>markets/events]
                D_DEDUP[Deduplication<br/>Engine]
                D_EXPORT[S3 Exporter]
            end
        end

        subgraph Storage["Managed Storage"]
            RDS[("Production RDS<br/>db.t4g.large<br/>TimescaleDB")]
            S3[("S3<br/>kalshi-data-archive<br/>Parquet")]
        end

        subgraph Monitoring["Monitoring"]
            PROM[Prometheus]
            GRAF[Grafana]
            ALERT[AlertManager]
        end
    end

    %% Kalshi API connections
    KALSHI_REST -->|"GET /markets<br/>GET /orderbook"| G1_MR & G2_MR & G3_MR
    KALSHI_REST -->|"GET /orderbook<br/>every 15min"| G1_SP & G2_SP & G3_SP
    KALSHI_REST -->|"GET /markets<br/>GET /events"| D_API
    KALSHI_WS -->|"ticker, trade,<br/>orderbook_delta,<br/>market_lifecycle"| G1_CM & G2_CM & G3_CM

    %% Gatherer 1 internal flow
    G1_MR -->|market events| G1_CM
    G1_CM -->|raw messages| G1_RT
    G1_RT -->|trades| G1_TW
    G1_RT -->|deltas| G1_OW
    G1_RT -->|tickers| G1_KW
    G1_SP -->|snapshots| G1_SW
    G1_TW & G1_OW & G1_KW & G1_SW -->|batch insert| G1_TS

    %% Gatherer 2 internal flow
    G2_MR -->|market events| G2_CM
    G2_CM -->|raw messages| G2_RT
    G2_RT -->|trades| G2_TW
    G2_RT -->|deltas| G2_OW
    G2_RT -->|tickers| G2_KW
    G2_SP -->|snapshots| G2_SW
    G2_TW & G2_OW & G2_KW & G2_SW -->|batch insert| G2_TS

    %% Gatherer 3 internal flow
    G3_MR -->|market events| G3_CM
    G3_CM -->|raw messages| G3_RT
    G3_RT -->|trades| G3_TW
    G3_RT -->|deltas| G3_OW
    G3_RT -->|tickers| G3_KW
    G3_SP -->|snapshots| G3_SW
    G3_TW & G3_OW & G3_KW & G3_SW -->|batch insert| G3_TS

    %% Deduplicator connections
    G1_TS & G2_TS & G3_TS -->|"SQL poll<br/>100ms"| D_SYNC
    D_SYNC --> D_DEDUP
    D_API --> D_DEDUP
    D_DEDUP -->|"INSERT<br/>ON CONFLICT DO NOTHING"| RDS
    RDS -->|"SELECT<br/>hourly/daily"| D_EXPORT
    D_EXPORT -->|"Parquet"| S3

    %% Monitoring
    G1_CM & G2_CM & G3_CM -.->|metrics :9090| PROM
    D_SYNC -.->|metrics :9090| PROM
    PROM --> GRAF
    PROM --> ALERT
```

---

## Component Detail

### Gatherer Components

```mermaid
flowchart LR
    subgraph Gatherer["Gatherer Process"]
        subgraph Discovery["Market Discovery"]
            MR[Market Registry]
            MR_CACHE[(In-Memory Cache<br/>200K-600K markets)]
        end

        subgraph WebSocket["WebSocket Layer"]
            CM[Connection Manager]
            subgraph Pool["Connection Pool (150)"]
                WS1[Conn 1-2: ticker]
                WS2[Conn 3-4: trade]
                WS3[Conn 5-6: lifecycle]
                WS4[Conn 7-150: orderbook]
            end
        end

        subgraph Routing["Message Processing"]
            RT[Message Router]
            PARSE[JSON Parser]
            VALID[Validator]
        end

        subgraph Writers["Batch Writers"]
            TW[Trade Writer<br/>batch: 1000<br/>flush: 100ms]
            OW[Orderbook Writer<br/>batch: 5000<br/>flush: 50ms]
            KW[Ticker Writer<br/>batch: 1000<br/>flush: 100ms]
            SW[Snapshot Writer<br/>batch: 100<br/>flush: 1s]
        end

        subgraph Backup["REST Backup"]
            SP[Snapshot Poller<br/>every 15min]
        end
    end

    subgraph LocalDB["Local Database"]
        TS[("TimescaleDB :5432<br/>trades<br/>orderbook_deltas<br/>orderbook_snapshots<br/>tickers")]
    end

    MR --> MR_CACHE
    MR_CACHE --> CM
    CM --> Pool
    Pool --> RT
    RT --> PARSE --> VALID
    VALID --> TW & OW & KW
    SP --> SW
    TW & OW & KW & SW --> TS
```

---

### Deduplicator Components

```mermaid
flowchart TB
    subgraph Sources["Data Sources (Read)"]
        G1_TS[("Gatherer 1<br/>TimescaleDB")]
        G2_TS[("Gatherer 2<br/>TimescaleDB")]
        G3_TS[("Gatherer 3<br/>TimescaleDB")]
        KALSHI[("Kalshi REST API")]
    end

    subgraph Deduplicator["Deduplicator Process"]
        subgraph SyncLoops["Parallel Sync Loops"]
            TRADES[Trades Sync<br/>poll: 100ms]
            DELTAS[Deltas Sync<br/>poll: 100ms]
            TICKERS[Tickers Sync<br/>poll: 100ms]
            SNAPS[Snapshots Sync<br/>poll: 1s]
        end

        subgraph APISync["API Sync"]
            MARKETS[Markets Sync<br/>poll: 5m]
            EVENTS[Events Sync<br/>poll: 5m]
        end

        subgraph Dedup["Deduplication"]
            MERGE[Merge 3 Sources]
            UNIQUE[Unique by PK]
        end

        subgraph Export["S3 Export"]
            EXPORTER[S3 Exporter<br/>hourly/daily]
            PARQUET[Parquet Writer]
        end

        CURSORS[(Cursor State<br/>per gatherer<br/>per table)]
    end

    subgraph Destinations["Destinations (Write)"]
        RDS[("Production RDS<br/>TimescaleDB")]
        S3[("S3 Bucket<br/>Parquet Archive")]
    end

    G1_TS & G2_TS & G3_TS --> TRADES & DELTAS & TICKERS & SNAPS
    KALSHI --> MARKETS & EVENTS

    TRADES & DELTAS & TICKERS & SNAPS & MARKETS & EVENTS --> MERGE
    MERGE --> UNIQUE
    UNIQUE --> RDS

    CURSORS <--> TRADES & DELTAS & TICKERS & SNAPS

    RDS --> EXPORTER
    EXPORTER --> PARQUET --> S3
```

---

## Data Flow

### Write Path (Kalshi → Production)

```mermaid
sequenceDiagram
    participant K as Kalshi API
    participant G1 as Gatherer 1
    participant G2 as Gatherer 2
    participant G3 as Gatherer 3
    participant D as Deduplicator
    participant P as Production RDS

    par All Gatherers Receive
        K->>G1: WebSocket message
        K->>G2: WebSocket message
        K->>G3: WebSocket message
    end

    par Write to Local DB
        G1->>G1: Parse, validate, batch
        G1->>G1: INSERT into local TimescaleDB
        G2->>G2: Parse, validate, batch
        G2->>G2: INSERT into local TimescaleDB
        G3->>G3: Parse, validate, batch
        G3->>G3: INSERT into local TimescaleDB
    end

    loop Every 100ms
        par Poll All Gatherers
            D->>G1: SELECT WHERE received_at > cursor
            G1-->>D: [records batch]
            D->>G2: SELECT WHERE received_at > cursor
            G2-->>D: [records batch]
            D->>G3: SELECT WHERE received_at > cursor
            G3-->>D: [records batch]
        end

        D->>D: Deduplicate by unique key
        D->>P: INSERT ... ON CONFLICT DO NOTHING
        D->>D: Update cursors
    end
```

---

### Read Path (Analytics)

```mermaid
flowchart LR
    subgraph Hot["Hot Data (RDS)"]
        RDS[("Production RDS<br/>Recent data<br/>7-90 days")]
    end

    subgraph Cold["Cold Data (S3)"]
        S3[("S3 Parquet<br/>Historical data<br/>All time")]
    end

    subgraph Query["Query Engines"]
        DIRECT[Direct SQL<br/>psql, pgAdmin]
        ATHENA[AWS Athena<br/>Serverless SQL]
        SPARK[Spark/EMR<br/>Large analytics]
        DUCK[DuckDB<br/>Local analysis]
    end

    subgraph Apps["Applications"]
        DASH[Dashboards]
        ML[ML Pipelines]
        REPORT[Reports]
    end

    RDS --> DIRECT
    S3 --> ATHENA & SPARK & DUCK
    DIRECT & ATHENA & SPARK & DUCK --> DASH & ML & REPORT
```

---

## Network Topology

```mermaid
flowchart TB
    subgraph Internet
        KALSHI[Kalshi API<br/>api.elections.kalshi.com]
    end

    subgraph VPC["VPC 10.0.0.0/16"]
        subgraph Public["Public Subnets"]
            NAT1[NAT Gateway<br/>us-east-1a]
            NAT2[NAT Gateway<br/>us-east-1b]
            NAT3[NAT Gateway<br/>us-east-1c]
        end

        subgraph Private1["Private Subnet 10.0.1.0/24<br/>us-east-1a"]
            G1[Gatherer 1<br/>10.0.1.10]
        end

        subgraph Private2["Private Subnet 10.0.2.0/24<br/>us-east-1b"]
            G2[Gatherer 2<br/>10.0.2.10]
        end

        subgraph Private3["Private Subnet 10.0.3.0/24<br/>us-east-1c"]
            G3[Gatherer 3<br/>10.0.3.10]
            DEDUP[Deduplicator<br/>10.0.3.20]
        end

        subgraph Data["Data Subnet 10.0.10.0/24"]
            RDS[(RDS<br/>10.0.10.100)]
        end

        S3[S3 VPC Endpoint]
    end

    KALSHI <-->|HTTPS 443| NAT1 & NAT2 & NAT3
    NAT1 --> G1
    NAT2 --> G2
    NAT3 --> G3

    G1 & G2 & G3 -->|5432| DEDUP
    DEDUP -->|5432| RDS
    DEDUP --> S3
```

---

## Database Schema Overview

```mermaid
erDiagram
    series ||--o{ events : contains
    events ||--o{ markets : contains
    markets ||--o{ trades : has
    markets ||--o{ orderbook_deltas : has
    markets ||--o{ orderbook_snapshots : has
    markets ||--o{ tickers : has

    series {
        varchar ticker PK
        text title
        varchar category
        bigint updated_at
    }

    events {
        varchar event_ticker PK
        varchar series_ticker FK
        text title
        varchar category
        bigint updated_at
    }

    markets {
        varchar ticker PK
        varchar event_ticker FK
        text title
        varchar market_status
        varchar trading_status
        bigint open_ts
        bigint close_ts
        bigint updated_at
    }

    trades {
        uuid trade_id PK
        bigint exchange_ts
        varchar ticker FK
        int price
        int size
        boolean taker_side
    }

    orderbook_deltas {
        varchar ticker PK
        bigint exchange_ts PK
        int price PK
        boolean side PK
        int size_delta
        bigint seq
    }

    orderbook_snapshots {
        varchar ticker PK
        bigint snapshot_ts PK
        varchar source PK
        jsonb yes_bids
        jsonb yes_asks
    }

    tickers {
        varchar ticker PK
        bigint exchange_ts PK
        int yes_bid
        int yes_ask
        int last_price
        bigint volume
    }
```

---

## Failure Domains

```mermaid
flowchart TB
    subgraph Critical["Critical (Immediate Page)"]
        ALL_G[All Gatherers Down]
        DEDUP_DOWN[Deduplicator Down]
        RDS_DOWN[Production RDS Down]
    end

    subgraph Degraded["Degraded (Warning)"]
        ONE_G[1-2 Gatherers Down]
        HIGH_LAG[Sync Lag > 30s]
        WS_PARTIAL[WebSocket < 140 conns]
    end

    subgraph Recoverable["Auto-Recoverable"]
        WS_RECONNECT[WebSocket Disconnect]
        BATCH_FAIL[Batch Write Failure]
        POLL_FAIL[Poll Failure]
    end

    ALL_G -->|"No data collection"| DATA_LOSS[Data Loss]
    DEDUP_DOWN -->|"Gatherers buffer"| CATCHUP[Catchup on Recovery]
    RDS_DOWN -->|"Dedup buffers"| CATCHUP

    ONE_G -->|"Others have full data"| NO_LOSS[No Data Loss]
    HIGH_LAG -->|"Monitor, investigate"| INVESTIGATE[Investigate]
    WS_PARTIAL -->|"Orderbook gaps"| GAPS[Potential Gaps]

    WS_RECONNECT -->|"Exponential backoff"| AUTO[Auto-Recover]
    BATCH_FAIL -->|"Retry with backoff"| AUTO
    POLL_FAIL -->|"Skip, retry next cycle"| AUTO
```

---

## Key Metrics

```mermaid
flowchart LR
    subgraph Gatherer["Gatherer Metrics"]
        G_WS[websocket_connections<br/>Target: 150]
        G_MARKETS[markets_active<br/>200K-600K]
        G_WRITES[writes_per_second<br/>~5K]
        G_LAG[message_lag_ms<br/>< 100ms]
    end

    subgraph Deduplicator["Deduplicator Metrics"]
        D_LAG[sync_lag_seconds<br/>< 5s]
        D_RECORDS[records_written_per_sec]
        D_DUPES[duplicates_skipped<br/>~66%]
        D_GATHERERS[gatherers_connected<br/>3]
    end

    subgraph Production["Production Metrics"]
        P_SIZE[database_size<br/>~500GB]
        P_CONNS[active_connections<br/>< 100]
        P_IOPS[disk_iops<br/>< 3000]
    end

    subgraph Export["Export Metrics"]
        E_LAG[export_lag_hours<br/>< 2h]
        E_SIZE[daily_export_gb<br/>~1GB]
    end
```

---

## Summary

| Layer | Components | Purpose |
|-------|------------|---------|
| **Collection** | 3 Gatherers | Redundant time-series capture from Kalshi |
| **Storage (Hot)** | 3 Local TimescaleDBs + 1 RDS | Low-latency reads, 7-90 day retention |
| **Storage (Cold)** | S3 Parquet | Historical archive, analytics |
| **Processing** | 1 Deduplicator | Merge, dedupe, API sync, export |
| **Monitoring** | Prometheus + Grafana | Metrics, alerts, dashboards |

**Data guarantees:**
- **Durability**: 3x redundant capture, no single point of failure for collection
- **Consistency**: Deduplication by exchange-provided keys
- **Latency**: < 500ms end-to-end (Kalshi → Production RDS)
- **Retention**: Forever for trades/snapshots, 30-90 days for high-volume tables (exported to S3)
