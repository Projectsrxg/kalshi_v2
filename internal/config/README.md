# Config Package

YAML configuration loading with environment variable substitution.

## Features

- Load YAML configuration files
- Environment variable substitution (`${VAR}` syntax)
- Validation of required fields
- Default value support

## Usage

```go
cfg, err := config.LoadGatherer("/etc/kalshi/gatherer.yaml")
if err != nil {
    log.Fatal(err)
}
```

## Configuration Files

See `configs/` directory for example configurations:
- `gatherer.example.yaml` - Gatherer configuration
- `deduplicator.example.yaml` - Deduplicator configuration

## Environment Variables

Sensitive values should be provided via environment variables:
- `POSTGRES_USER`, `POSTGRES_PASSWORD`
- `TIMESCALE_USER`, `TIMESCALE_PASSWORD`
- `PROD_RDS_HOST`, `PROD_RDS_USER`, `PROD_RDS_PASSWORD`
