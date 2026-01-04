# Deployment

Infrastructure setup, build process, and deployment topology.

---

## Overview

The Kalshi Data Platform runs as two separate binaries from a monorepo:

| Binary | Instances | Role |
|--------|-----------|------|
| `gatherer` | 3 (one per AZ) | Collect all market data independently |
| `deduplicator` | 1 | Merge data from gatherers, write to production |

```mermaid
flowchart TD
    subgraph "Build (CI/CD)"
        REPO[Monorepo] --> BUILD[go build]
        BUILD --> G_BIN[gatherer binary]
        BUILD --> D_BIN[deduplicator binary]
    end

    subgraph "us-east-1a"
        G1[EC2: Gatherer 1<br/>t4g.2xlarge]
        G1_TS[(TimescaleDB)]
        G1_PG[(PostgreSQL)]
        G1 --> G1_TS & G1_PG
    end

    subgraph "us-east-1b"
        G2[EC2: Gatherer 2<br/>t4g.2xlarge]
        G2_TS[(TimescaleDB)]
        G2_PG[(PostgreSQL)]
        G2 --> G2_TS & G2_PG
    end

    subgraph "us-east-1c"
        G3[EC2: Gatherer 3<br/>t4g.2xlarge]
        G3_TS[(TimescaleDB)]
        G3_PG[(PostgreSQL)]
        G3 --> G3_TS & G3_PG
        DEDUP[EC2: Deduplicator<br/>t4g.xlarge]
    end

    subgraph "RDS (us-east-1a)"
        PROD[(Production RDS<br/>TimescaleDB)]
    end

    subgraph "S3 (Regional)"
        S3[(Parquet Archives)]
    end

    G_BIN --> G1 & G2 & G3
    D_BIN --> DEDUP

    DEDUP -->|poll| G1_TS & G2_TS & G3_TS
    DEDUP -->|poll| G1_PG & G2_PG & G3_PG
    DEDUP -->|write| PROD
    DEDUP -->|export| S3
```

---

## Key Principles

### Monorepo, Multiple Binaries

Single codebase produces two binaries:

```
kalshi-data/
├── cmd/
│   ├── gatherer/main.go      → gatherer binary
│   └── deduplicator/main.go  → deduplicator binary
├── internal/                  → shared packages
└── ...
```

- Same repo, same Go modules
- Shared code in `internal/` (database clients, message types, etc.)
- Different entrypoints with different component compositions

### No Shared Runtime

- Each gatherer runs independently on its own EC2 instance
- Deduplicator runs on a separate EC2 instance
- **No message queues** - IPC via database polling only
- Each gatherer has its own local databases

### Database as IPC

```mermaid
flowchart LR
    subgraph "Gatherer 1"
        G1[gatherer] --> TS1[(TimescaleDB)]
    end

    subgraph "Deduplicator"
        D[deduplicator]
    end

    TS1 -->|SQL poll| D
    D -->|SQL write| PROD[(Production RDS)]
```

Deduplicator polls gatherer databases using cursor-based sync. No message broker required.

---

## Documentation

| Document | Description |
|----------|-------------|
| [Binaries](binaries.md) | Monorepo structure and build commands |
| [Infrastructure](infrastructure.md) | AWS resource specifications |
| [Terraform](terraform.md) | Infrastructure-as-Code templates |
| [IPC](ipc.md) | Interprocess communication via database polling |
| [Startup](startup.md) | Initialization order and systemd configuration |

---

## Quick Reference

### Instance Summary

| Component | Instance Type | vCPU | RAM | Storage | Count |
|-----------|--------------|------|-----|---------|-------|
| Gatherer | t4g.2xlarge | 8 | 32GB | 200GB gp3 | 3 |
| Deduplicator | t4g.xlarge | 4 | 16GB | 50GB gp3 | 1 |
| Production RDS | db.t4g.large | 2 | 8GB | 500GB gp3 | 1 |

### Network

| Component | Inbound | Outbound |
|-----------|---------|----------|
| Gatherer | Deduplicator (5432) | Kalshi API (443) |
| Deduplicator | None | Gatherers (5432), RDS (5432), S3 (443) |
| RDS | Deduplicator (5432) | None |

### Startup Order

1. **RDS** - Must be available first
2. **Gatherers** - Can start in parallel, connect to Kalshi independently
3. **Deduplicator** - Waits for at least one gatherer to be healthy
