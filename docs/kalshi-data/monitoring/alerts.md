# Alerts

Prometheus alerting rules for the Kalshi Data Platform.

---

## Summary

| Severity | Count | Alerts |
|----------|-------|--------|
| Critical | 5 | AllGatherersDown, ProductionRDSUnreachable, DeduplicatorSyncStalled, NoMarketsPolled, AllWebSocketsDisconnected |
| Warning | 14 | SingleGathererDown, HighWriterErrorRate, HighFlushLatency, PollCycleApproachingLimit, HighFetchErrorRate, HighMessageDropRate, SequenceGapsDetected, HighReconnectRate, SingleRoleDown, DiskUsageHigh, ConnectionPoolExhaustion, ExportLagWarning, ExportErrorRate, S3UploadFailure |
| Info | 2 | MarketRegistryReconciling, HighDuplicateRate |
| **Total** | **21** | |

---

## External Dependencies

Some alerts require additional exporters:

| Alert | Required Exporter | Metrics Used |
|-------|-------------------|--------------|
| DiskUsageHigh | [node_exporter](https://github.com/prometheus/node_exporter) | `node_filesystem_size_bytes`, `node_filesystem_avail_bytes` |
| ConnectionPoolExhaustion | [postgres_exporter](https://github.com/prometheus-community/postgres_exporter) | `pg_stat_activity_count`, `pg_settings_max_connections` |

---

## Severity Levels

| Severity | Response Time | Notification |
|----------|---------------|--------------|
| `critical` | Immediate | PagerDuty page |
| `warning` | Within 1 hour | Slack channel |
| `info` | Next business day | Slack channel |

---

## Critical Alerts

### AllGatherersDown

All gatherers are unhealthy. Data collection has stopped.

```yaml
- alert: AllGatherersDown
  expr: sum(up{job="gatherers"}) == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "All gatherers are down"
    description: "No gatherer instances are responding. Data collection has stopped."
    runbook: "docs/kalshi-data/monitoring/runbooks.md#gatherer-down"
```

### ProductionRDSUnreachable

Deduplicator cannot connect to production RDS.

```yaml
- alert: ProductionRDSUnreachable
  expr: dedup_rds_connected == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Production RDS is unreachable"
    description: "Deduplicator cannot write to production RDS."
    runbook: "docs/kalshi-data/monitoring/runbooks.md#rds-unreachable"
```

### DeduplicatorSyncStalled

Deduplicator has not synced for over 5 minutes.

```yaml
- alert: DeduplicatorSyncStalled
  expr: dedup_sync_lag_seconds > 300
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Deduplicator sync stalled"
    description: "Deduplicator has not synced for {{ $value | humanizeDuration }}."
    runbook: "docs/kalshi-data/monitoring/runbooks.md#deduplicator-lag"
```

### NoMarketsPolled

Snapshot Poller is not polling any markets.

```yaml
- alert: NoMarketsPolled
  expr: poller_markets_polled == 0
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "No markets being polled"
    description: "Snapshot Poller reports 0 markets. Check Market Registry."
```

### AllWebSocketsDisconnected

All WebSocket connections on a gatherer are down.

```yaml
- alert: AllWebSocketsDisconnected
  expr: sum by (instance) (conn_manager_connections_healthy) == 0
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "All WebSocket connections down on {{ $labels.instance }}"
    description: "Gatherer has no healthy WebSocket connections."
    runbook: "docs/kalshi-data/monitoring/runbooks.md#websocket-down"
```

---

## Warning Alerts

### SingleGathererDown

One gatherer is unhealthy. Redundancy is reduced.

```yaml
- alert: SingleGathererDown
  expr: sum(up{job="gatherers"}) < 3
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "{{ $value }} of 3 gatherers healthy"
    description: "One or more gatherers are down. Redundancy is reduced."
    runbook: "docs/kalshi-data/monitoring/runbooks.md#gatherer-down"
```

### HighWriterErrorRate

Writer error rate exceeds 1%.

```yaml
- alert: HighWriterErrorRate
  expr: >
    rate(writer_errors_total[5m]) /
    rate(writer_inserts_total[5m]) > 0.01
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High error rate for {{ $labels.writer }} writer"
    description: "Error rate is {{ $value | humanizePercentage }}."
    runbook: "docs/kalshi-data/monitoring/runbooks.md#high-error-rate"
```

### HighFlushLatency

Writer flush latency P99 exceeds 1 second.

```yaml
- alert: HighFlushLatency
  expr: >
    histogram_quantile(0.99,
      sum(rate(writer_flush_duration_seconds_bucket[5m])) by (le, writer)
    ) > 1
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High flush latency for {{ $labels.writer }} writer"
    description: "P99 flush latency is {{ $value | humanizeDuration }}."
```

### PollCycleApproachingLimit

Snapshot Poller cycle is approaching 1-minute limit.

```yaml
- alert: PollCycleApproachingLimit
  expr: >
    histogram_quantile(0.95,
      sum(rate(poller_poll_duration_seconds_bucket[5m])) by (le)
    ) > 55
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "Poll cycle approaching 1-minute limit"
    description: "P95 poll duration is {{ $value }}s. May miss cycles."
```

### HighFetchErrorRate

Snapshot Poller fetch error rate exceeds 10%.

```yaml
- alert: HighFetchErrorRate
  expr: >
    rate(poller_fetch_errors_total[5m]) /
    (rate(poller_snapshots_fetched_total[5m]) + rate(poller_fetch_errors_total[5m])) > 0.1
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High Snapshot Poller fetch error rate"
    description: "Fetch error rate is {{ $value | humanizePercentage }}."
```

### HighMessageDropRate

Message Router is dropping messages.

```yaml
- alert: HighMessageDropRate
  expr: >
    sum(rate(router_messages_dropped_total[5m])) /
    sum(rate(router_messages_received_total[5m])) > 0.001
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Message Router dropping messages"
    description: "Drop rate is {{ $value | humanizePercentage }}. Check writer throughput."
```

### SequenceGapsDetected

Sequence gaps are being detected in orderbook data.

```yaml
- alert: SequenceGapsDetected
  expr: rate(conn_manager_sequence_gaps_total[5m]) > 0.1
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Sequence gaps detected"
    description: "{{ $value }} gaps/second. Data may be missing."
```

### HighReconnectRate

WebSocket connections are reconnecting frequently.

```yaml
- alert: HighReconnectRate
  expr: sum(rate(conn_manager_reconnects_total[5m])) by (instance) > 1
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High WebSocket reconnect rate on {{ $labels.instance }}"
    description: "{{ $value }} reconnects/second. Check network stability."
```

### SingleRoleDown

All connections for a WebSocket role are down.

```yaml
- alert: SingleRoleDown
  expr: >
    sum by (instance, role) (conn_manager_connections_healthy) == 0
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "All {{ $labels.role }} connections down on {{ $labels.instance }}"
    description: "No healthy connections for role {{ $labels.role }}. Check WebSocket connectivity."
```

### DiskUsageHigh

Disk usage exceeds 80%.

```yaml
- alert: DiskUsageHigh
  expr: >
    (node_filesystem_size_bytes{mountpoint="/"} - node_filesystem_avail_bytes{mountpoint="/"})
    / node_filesystem_size_bytes{mountpoint="/"} > 0.8
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Disk usage high on {{ $labels.instance }}"
    description: "Disk usage is {{ $value | humanizePercentage }}."
```

### ConnectionPoolExhaustion

Database connection pool is near exhaustion.

```yaml
- alert: ConnectionPoolExhaustion
  expr: >
    pg_stat_activity_count / pg_settings_max_connections > 0.8
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Database connection pool near exhaustion"
    description: "Using {{ $value | humanizePercentage }} of max connections."
```

---

## Info Alerts

### MarketRegistryReconciling

Market Registry is performing reconciliation.

```yaml
- alert: MarketRegistryReconciling
  expr: market_registry_reconcile_duration_seconds > 30
  for: 1m
  labels:
    severity: info
  annotations:
    summary: "Market Registry reconciliation taking long"
    description: "Reconciliation has been running for {{ $value }}s."
```

### HighDuplicateRate

Deduplicator seeing unusually high duplicate rate.

```yaml
- alert: HighDuplicateRate
  expr: dedup_duplicate_rate > 0.5
  for: 10m
  labels:
    severity: info
  annotations:
    summary: "High duplicate rate in deduplicator"
    description: "{{ $value | humanizePercentage }} of records are duplicates."
```

---

## Storage/Export Alerts

### ExportLagWarning

S3 export job is falling behind.

```yaml
- alert: ExportLagWarning
  expr: export_lag_seconds > 7200
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Export lag for {{ $labels.table }}"
    description: "Export lag is {{ $value | humanizeDuration }}. Expected < 2 hours."
```

### ExportErrorRate

Export jobs are failing frequently.

```yaml
- alert: ExportErrorRate
  expr: rate(export_errors_total[1h]) > 0.05
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High export error rate for {{ $labels.table }}"
    description: "Export error rate is {{ $value | humanizePercentage }}."
```

### S3UploadFailure

S3 upload is failing.

```yaml
- alert: S3UploadFailure
  expr: increase(export_errors_total{type="s3_upload"}[1h]) > 3
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "S3 upload failures for {{ $labels.table }}"
    description: "{{ $value }} S3 upload failures in the last hour."
```

---

## Alertmanager Configuration

```yaml
global:
  slack_api_url: 'https://hooks.slack.com/services/XXX'

route:
  receiver: 'slack-warnings'
  group_by: ['alertname', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  routes:
    - match:
        severity: critical
      receiver: 'pagerduty-critical'
      repeat_interval: 5m
    - match:
        severity: warning
      receiver: 'slack-warnings'
    - match:
        severity: info
      receiver: 'slack-info'

receivers:
  - name: 'pagerduty-critical'
    pagerduty_configs:
      - service_key: '<pagerduty-service-key>'
        severity: critical

  - name: 'slack-warnings'
    slack_configs:
      - channel: '#kalshi-alerts'
        title: '{{ .Status | toUpper }}: {{ .CommonAnnotations.summary }}'
        text: '{{ .CommonAnnotations.description }}'

  - name: 'slack-info'
    slack_configs:
      - channel: '#kalshi-info'
        title: '{{ .CommonAnnotations.summary }}'
```

---

## Alert Testing

Test alerts in development:

```bash
# Fire a test alert
curl -X POST http://alertmanager:9093/api/v1/alerts \
  -H "Content-Type: application/json" \
  -d '[{
    "labels": {
      "alertname": "TestAlert",
      "severity": "warning"
    },
    "annotations": {
      "summary": "Test alert",
      "description": "This is a test alert"
    }
  }]'
```
