# Command Binaries

This directory contains the main entry points for the Kalshi Data Platform binaries.

## Binaries

| Binary | Description |
|--------|-------------|
| `gatherer` | Collects market data via REST and WebSocket APIs |
| `deduplicator` | Merges data from all gatherers into production database |

## Building

```bash
# Build both binaries
make build

# Build for production (Linux ARM64)
make build-linux-arm64
```

## Running

```bash
# Gatherer
./bin/gatherer --config /etc/kalshi/gatherer.yaml

# Deduplicator
./bin/deduplicator --config /etc/kalshi/deduplicator.yaml
```

## Deployment

- **Gatherers**: 3 instances, one per availability zone
- **Deduplicator**: 1 instance, polls all gatherers
