# Deployment

Infrastructure and deployment configuration for Kalshi Data Platform.

## Structure

```
deploy/
└── terraform/          # Infrastructure-as-Code
```

## Architecture

```mermaid
flowchart TD
    subgraph AZ1[us-east-1a]
        G1[Gatherer 1]
    end
    subgraph AZ2[us-east-1b]
        G2[Gatherer 2]
    end
    subgraph AZ3[us-east-1c]
        G3[Gatherer 3]
    end

    G1 --> DEDUP[Deduplicator]
    G2 --> DEDUP
    G3 --> DEDUP
    DEDUP --> RDS[(Production RDS)]
    DEDUP --> S3[(S3)]
```

## Resources

- **EC2**: 3 gatherer instances + 1 deduplicator
- **RDS**: Production TimescaleDB
- **S3**: Data archival bucket
- **VPC**: Private networking with NAT
