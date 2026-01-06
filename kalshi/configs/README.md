# Configuration Files

YAML configuration files for Kalshi Data Platform binaries.

## Files

| File | Description |
|------|-------------|
| `gatherer.example.yaml` | Example gatherer configuration |
| `deduplicator.example.yaml` | Example deduplicator configuration |

## Local Development

Copy example files and customize:

```bash
cp gatherer.example.yaml gatherer.local.yaml
cp deduplicator.example.yaml deduplicator.local.yaml
```

`.local.yaml` files are gitignored and safe for local secrets.

## Environment Variables

Configuration supports `${VAR}` syntax for environment variable substitution:

```yaml
database:
  password: ${POSTGRES_PASSWORD}
```

## Required Variables

### Gatherer

- `POSTGRES_USER`, `POSTGRES_PASSWORD` - Local PostgreSQL
- `TIMESCALE_USER`, `TIMESCALE_PASSWORD` - Local TimescaleDB

### Deduplicator

- `GATHERER_1_USER`, `GATHERER_1_PASSWORD` - Gatherer 1 database
- `GATHERER_2_USER`, `GATHERER_2_PASSWORD` - Gatherer 2 database
- `GATHERER_3_USER`, `GATHERER_3_PASSWORD` - Gatherer 3 database
- `PROD_RDS_HOST`, `PROD_RDS_USER`, `PROD_RDS_PASSWORD` - Production RDS
- `S3_BUCKET` (optional) - S3 export bucket
