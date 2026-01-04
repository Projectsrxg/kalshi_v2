# Runbooks

Operational procedures for the Kalshi Data Platform.

---

## Gatherer Down

**Alert:** `SingleGathererDown` or `AllGatherersDown`

### Symptoms
- Gatherer not responding to health checks
- No metrics being scraped
- No data flowing to local TimescaleDB

### Diagnosis

1. **Check EC2 instance status**
   ```bash
   aws ec2 describe-instance-status --instance-ids <instance-id>
   ```

2. **SSH to instance and check process**
   ```bash
   ssh gatherer-1
   systemctl status kalshi-gatherer
   journalctl -u kalshi-gatherer -n 100
   ```

3. **Check TimescaleDB connectivity**
   ```bash
   psql -h localhost -U gatherer -d kalshi -c "SELECT 1"
   ```

4. **Check WebSocket connections**
   ```bash
   curl http://localhost:9090/metrics | grep ws_client_connected
   ```

### Resolution

| Cause | Action |
|-------|--------|
| Process crashed | `systemctl restart kalshi-gatherer` |
| OOM killed | Increase instance size or tune batch sizes |
| Disk full | Clear old data, increase EBS size |
| Network issue | Check security groups, VPC routing |
| TimescaleDB down | Restart TimescaleDB, check disk space |

### Verification
```bash
# Confirm process is running
systemctl status kalshi-gatherer

# Confirm metrics are being scraped
curl http://localhost:9090/metrics | grep writer_inserts_total

# Confirm data is flowing
psql -c "SELECT COUNT(*) FROM trades WHERE exchange_ts > extract(epoch from now() - interval '1 minute') * 1000000"
```

---

## WebSocket Down

**Alert:** `AllWebSocketsDisconnected`

### Symptoms
- No messages being received
- Connection count is 0
- Reconnection rate is high

### Diagnosis

1. **Check connection status**
   ```bash
   curl http://localhost:9090/metrics | grep conn_manager_connections
   ```

2. **Check WebSocket client logs**
   ```bash
   journalctl -u kalshi-gatherer | grep -i websocket
   ```

3. **Test Kalshi API directly**
   ```bash
   wscat -c wss://api.elections.kalshi.com
   ```

4. **Check network connectivity**
   ```bash
   curl -I https://api.elections.kalshi.com/trade-api/v2/exchange/status
   ```

### Resolution

| Cause | Action |
|-------|--------|
| Kalshi API down | Wait for recovery, monitor status page |
| Auth expired | Restart gatherer to refresh auth |
| Network blocked | Check security groups, NAT gateway |
| Rate limited | Reduce subscription rate, stagger reconnects |

### Verification
```bash
# Check connections are restored
curl http://localhost:9090/metrics | grep "conn_manager_connections_healthy"

# Check messages are flowing
curl http://localhost:9090/metrics | grep "ws_client_messages_received_total"
```

---

## High Error Rate

**Alert:** `HighWriterErrorRate`

### Symptoms
- Error rate > 1%
- Writer metrics showing failures
- Data may be missing

### Diagnosis

1. **Identify error type**
   ```bash
   curl http://localhost:9090/metrics | grep writer_errors_total
   ```

2. **Check writer logs**
   ```bash
   journalctl -u kalshi-gatherer | grep -E "(ERROR|WARN).*writer"
   ```

3. **Check database status**
   ```bash
   psql -c "SELECT * FROM pg_stat_activity WHERE state != 'idle'"
   ```

4. **Check disk space**
   ```bash
   df -h
   ```

### Resolution

| Error Type | Cause | Action |
|------------|-------|--------|
| `connection` | DB unreachable | Restart DB, check network |
| `constraint` | Schema mismatch | Check for schema changes |
| `timeout` | Slow inserts | Increase pool size, reduce batch |

### Verification
```bash
# Error rate should drop
curl http://localhost:9090/metrics | grep writer_errors_total

# Inserts should continue
curl http://localhost:9090/metrics | grep writer_inserts_total
```

---

## Deduplicator Lag

**Alert:** `DeduplicatorSyncStalled`

### Symptoms
- Sync lag exceeds 30 seconds
- Data not appearing in production RDS
- Deduplicator health check failing

### Diagnosis

1. **Check deduplicator status**
   ```bash
   curl http://deduplicator:8080/health
   ```

2. **Check gatherer connectivity**
   ```bash
   curl http://deduplicator:9090/metrics | grep dedup_gatherer_health
   ```

3. **Check RDS connectivity**
   ```bash
   psql -h production-rds -U deduplicator -d kalshi -c "SELECT 1"
   ```

4. **Check deduplicator logs**
   ```bash
   journalctl -u kalshi-deduplicator -n 100
   ```

### Resolution

| Cause | Action |
|-------|--------|
| Gatherer(s) unreachable | Fix gatherer connectivity |
| RDS unreachable | Check RDS status, security groups |
| High duplicate rate | Normal with 3 gatherers, investigate if >80% |
| Slow queries | Check RDS performance, add indexes |

### Verification
```bash
# Sync lag should decrease
curl http://deduplicator:9090/metrics | grep dedup_sync_lag_seconds

# All gatherers healthy
curl http://deduplicator:8080/health | jq '.gatherers'
```

---

## RDS Unreachable

**Alert:** `ProductionRDSUnreachable`

### Symptoms
- Deduplicator cannot write to RDS
- Connection errors in logs
- Data not appearing in production

### Diagnosis

1. **Check RDS status**
   ```bash
   aws rds describe-db-instances --db-instance-identifier kalshi-production
   ```

2. **Test connectivity**
   ```bash
   nc -zv production-rds.xxx.rds.amazonaws.com 5432
   ```

3. **Check security groups**
   ```bash
   aws ec2 describe-security-groups --group-ids <sg-id>
   ```

4. **Check RDS metrics**
   ```bash
   # In AWS Console: RDS > Monitoring
   # Check CPU, connections, storage
   ```

### Resolution

| Cause | Action |
|-------|--------|
| RDS instance down | Check AWS console, failover if Multi-AZ |
| Connection limit | Scale up instance, add connection pooling |
| Storage full | Increase storage, enable autoscaling |
| Security group | Add deduplicator IP to inbound rules |

---

## Data Gap Recovery

**Alert:** Manual detection or user report

### Symptoms
- Missing trades for a time period
- Sequence gaps in orderbook data
- Query returns no data for specific timeframe

### Diagnosis

1. **Identify the gap**
   ```sql
   -- Find gaps in trades
   SELECT
       date_trunc('minute', to_timestamp(exchange_ts/1000000)) as minute,
       COUNT(*) as trades
   FROM trades
   WHERE ticker = 'EXAMPLE-TICKER'
   GROUP BY 1
   ORDER BY 1;
   ```

2. **Check if REST snapshots exist**
   ```sql
   SELECT snapshot_ts, source
   FROM orderbook_snapshots
   WHERE ticker = 'EXAMPLE-TICKER'
     AND snapshot_ts BETWEEN <start> AND <end>
   ORDER BY snapshot_ts;
   ```

3. **Check gatherer logs for that period**
   ```bash
   journalctl -u kalshi-gatherer --since "2024-01-15 10:00" --until "2024-01-15 10:30"
   ```

### Recovery Options

| Data Type | Recovery Method |
|-----------|-----------------|
| Trades | Cannot recover - use REST snapshot for closest state |
| Orderbook | REST snapshots provide 15-minute resolution |
| Ticker | Can interpolate from adjacent data |

### Manual Recovery

If admin API is implemented, trigger manual poll:

```bash
# Example: Trigger manual REST poll for specific market
# Note: Requires admin API endpoint (see architecture docs for implementation)
curl -X POST http://gatherer-1:8080/admin/poll-market?ticker=EXAMPLE-TICKER
```

Alternatively, restart Snapshot Poller to trigger immediate full poll cycle.

---

## Disk Usage High

**Alert:** `DiskUsageHigh`

### Symptoms
- Disk usage > 80%
- TimescaleDB may stop accepting writes
- Compression policies may be delayed

### Diagnosis

1. **Check disk usage**
   ```bash
   df -h /var/lib/postgresql
   ```

2. **Check table sizes**
   ```sql
   SELECT
       hypertable_name,
       pg_size_pretty(hypertable_size(format('%I.%I', hypertable_schema, hypertable_name)))
   FROM timescaledb_information.hypertables;
   ```

3. **Check chunk compression status**
   ```sql
   SELECT
       hypertable_name,
       chunk_name,
       is_compressed
   FROM timescaledb_information.chunks
   WHERE hypertable_name = 'trades'
   ORDER BY chunk_name DESC
   LIMIT 20;
   ```

### Resolution

1. **Run manual compression**
   ```sql
   SELECT compress_chunk(c.chunk)
   FROM (
       SELECT chunk FROM show_chunks('trades', older_than => interval '1 day')
       WHERE NOT is_compressed
   ) c;
   ```

2. **Drop old data (if retention policy not working)**
   ```sql
   SELECT drop_chunks('orderbook_deltas', older_than => interval '7 days');
   ```

3. **Increase EBS volume**
   ```bash
   aws ec2 modify-volume --volume-id <vol-id> --size 500
   # Then resize filesystem
   sudo growpart /dev/xvda 1
   sudo resize2fs /dev/xvda1
   ```

---

## Performance Degradation

### Symptoms
- High latency in dashboards
- Slow batch flushes
- Query timeouts

### Diagnosis

1. **Check writer latency**
   ```bash
   curl http://localhost:9090/metrics | grep writer_flush_duration
   ```

2. **Check database performance**
   ```sql
   SELECT * FROM pg_stat_activity WHERE state != 'idle' ORDER BY query_start;
   ```

3. **Check for blocking queries**
   ```sql
   SELECT * FROM pg_locks WHERE NOT granted;
   ```

### Resolution

| Symptom | Action |
|---------|--------|
| High flush latency | Increase batch size, reduce flush interval |
| Many active queries | Check for long-running queries, add indexes |
| Lock contention | Check for schema changes, VACUUM operations |
| High CPU | Scale up instance, optimize queries |

---

## Emergency Contacts

| Role | Contact |
|------|---------|
| On-call engineer | PagerDuty rotation |
| Database admin | #db-team Slack |
| AWS support | AWS Console > Support |
| Kalshi API issues | api-support@kalshi.com |
