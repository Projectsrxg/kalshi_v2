# Kalshi Data Platform

## Overview

- [Platform Overview](./platform_overview.md)

## Architecture

High-level system design.

### [architecture/](./architecture/)
- [System Design](./architecture/system-design.md) - End-to-end architecture, deployment model
- [Data Flow](./architecture/data-flow.md) - Message lifecycle from WebSocket to storage
- [Data Model (Gatherer)](./architecture/data-model.md) - Gatherer-local TimescaleDB schema
- [Data Model (Production)](./architecture/data-model-production.md) - Production RDS schema
- [Deduplicator](./architecture/deduplicator.md) - Sync logic, cursor management, batch processing
- [Reliability](./architecture/reliability.md) - Failure modes, recovery, consistency guarantees
- [Scalability](./architecture/scalability.md) - Horizontal scaling strategy, bottlenecks

## Components

Detailed design for each subsystem.

### [market-registry/](./market-registry/)
Market discovery, tracking, and lifecycle management.

### [connection-manager/](./connection-manager/)
WebSocket connection pool and market assignment.

### [websocket/](./websocket/)
WebSocket client implementation.

### [message-router/](./message-router/)
Message demultiplexing and validation.

### [writers/](./writers/)
Data serialization and batching.

### [storage/](./storage/)
Persistence layer design.

### [recovery/](./recovery/)
Failure handling and data recovery.

### [monitoring/](./monitoring/)
Observability and operations.
