# Recovery Runbooks

Step-by-step procedures for common recovery scenarios.

---

## Quick Reference

| Scenario | Urgency | Runbook |
|----------|---------|---------|
| Single gatherer down | Low | [Replace Gatherer](#replace-gatherer) |
| All gatherers down | Critical | [Emergency Gatherer Recovery](#emergency-gatherer-recovery) |
| Deduplicator down | Medium | [Restart Deduplicator](#restart-deduplicator) |
| Production RDS unreachable | Critical | [RDS Recovery](#rds-recovery) |
| High sequence gaps | Medium | [Investigate Gaps](#investigate-sequence-gaps) |
| WebSocket reconnection loop | Medium | [Connection Troubleshooting](#connection-troubleshooting) |
| Data verification | Low | [Verify Data Integrity](#verify-data-integrity) |

---

## Replace Gatherer

**Trigger:** Single gatherer unhealthy, CloudWatch alarm

**Impact:** None (other gatherers have data)

**Time:** 15-30 minutes

### Steps

1. **Confirm failure**
   ```bash
   # Check gatherer health
   curl http://gatherer-1.internal:8080/health

   # Check EC2 instance status
   aws ec2 describe-instance-status --instance-id i-xxx
   ```

2. **Check if auto-recovery is working**
   ```bash
   # Systemd should auto-restart
   ssh gatherer-1 "systemctl status gatherer"
   ```

3. **If process won't start, check logs**
   ```bash
   ssh gatherer-1 "journalctl -u gatherer -n 100"
   ```

4. **If instance is unhealthy, launch replacement**
   ```bash
   # Terminate failed instance
   aws ec2 terminate-instances --instance-ids i-xxx

   # Auto-scaling launches replacement, or manually:
   aws ec2 run-instances \
     --image-id ami-xxx \
     --instance-type t4g.2xlarge \
     --subnet-id subnet-xxx \
     --user-data file://gatherer-init.sh
   ```

5. **Verify new gatherer is collecting data**
   ```bash
   # Check health
   curl http://gatherer-new:8080/health

   # Check recent data
   psql -h gatherer-new -d kalshi_ts -c \
     "SELECT COUNT(*) FROM trades
      WHERE received_at > extract(epoch from now()) * 1000000 - 60000000"
   ```

6. **Update deduplicator if needed**
   ```bash
   # If gatherer hostname changed
   ssh deduplicator "vim /etc/gatherer/config.yaml"
   ssh deduplicator "systemctl restart deduplicator"
   ```

---

## Emergency Gatherer Recovery

**Trigger:** All gatherers down, AllGatherersDown alert

**Impact:** Data collection stopped

**Time:** 10-15 minutes

### Steps

1. **Assess the situation**
   ```bash
   # Check all gatherer health endpoints
   for g in gatherer-{1,2,3}; do
     echo "$g: $(curl -s http://$g.internal:8080/health || echo 'UNREACHABLE')"
   done

   # Check Kalshi API status
   curl -s https://api.elections.kalshi.com/trade-api/v2/exchange/status
   ```

2. **If Kalshi API is down**
   - Nothing to do except wait
   - Monitor Kalshi status page
   - Gatherers will auto-reconnect when API recovers

3. **If gatherers are crashed**
   ```bash
   # Attempt restart on each
   for g in gatherer-{1,2,3}; do
     ssh $g "systemctl restart gatherer" &
   done
   wait
   ```

4. **If instances are terminated/unreachable**
   ```bash
   # Launch new instances in parallel
   for az in a b c; do
     aws ec2 run-instances \
       --image-id ami-xxx \
       --instance-type t4g.2xlarge \
       --subnet-id subnet-$az \
       --user-data file://gatherer-init.sh &
   done
   wait
   ```

5. **Verify recovery**
   ```bash
   # Check data flow resumed
   watch -n 5 'curl -s http://gatherer-1:8080/health'

   # Check trades are being recorded
   psql -h gatherer-1 -d kalshi_ts -c \
     "SELECT COUNT(*) FROM trades
      WHERE received_at > extract(epoch from now()) * 1000000 - 60000000"
   ```

6. **Check deduplicator is syncing**
   ```bash
   curl http://deduplicator:9090/metrics | grep dedup_sync_lag
   ```

---

## Restart Deduplicator

**Trigger:** DeduplicatorSyncStalled alert, deduplicator unhealthy

**Impact:** Production RDS not receiving updates (gatherers buffering)

**Time:** 5-10 minutes

### Steps

1. **Check current status**
   ```bash
   curl http://deduplicator:8080/health
   curl http://deduplicator:9090/metrics | grep -E "dedup_(sync_lag|rds)"
   ```

2. **Check logs for errors**
   ```bash
   ssh deduplicator "journalctl -u deduplicator -n 200 --no-pager"
   ```

3. **Restart service**
   ```bash
   ssh deduplicator "systemctl restart deduplicator"
   ```

4. **Monitor catch-up**
   ```bash
   # Watch sync lag decrease
   watch -n 5 'curl -s http://deduplicator:9090/metrics | grep dedup_sync_lag'
   ```

5. **Verify data flow to RDS**
   ```bash
   # Check recent inserts in production
   psql -h prod-rds -d kalshi_prod -c \
     "SELECT COUNT(*) FROM trades
      WHERE received_at > extract(epoch from now()) * 1000000 - 300000000"
   ```

---

## RDS Recovery

**Trigger:** ProductionRDSUnreachable alert

**Impact:** Deduplicator cannot write, gatherers buffering

**Time:** 30-60 minutes

### Steps

1. **Check RDS status**
   ```bash
   aws rds describe-db-instances --db-instance-identifier kalshi-prod
   ```

2. **Check for ongoing events**
   ```bash
   aws rds describe-events --source-identifier kalshi-prod --duration 60
   ```

3. **If Multi-AZ failover in progress**
   - Wait for automatic failover (60-120 seconds)
   - Deduplicator will auto-reconnect

4. **If instance needs manual intervention**

   **Option A: Point-in-time recovery**
   ```bash
   aws rds restore-db-instance-to-point-in-time \
     --source-db-instance-identifier kalshi-prod \
     --target-db-instance-identifier kalshi-prod-restored \
     --restore-time "2024-01-15T10:30:00Z"
   ```

   **Option B: Restore from snapshot**
   ```bash
   aws rds restore-db-instance-from-db-snapshot \
     --db-instance-identifier kalshi-prod-restored \
     --db-snapshot-identifier kalshi-prod-2024-01-15
   ```

5. **Update connection strings**
   ```bash
   # Update deduplicator config
   ssh deduplicator "vim /etc/deduplicator/config.yaml"
   # Change: production.host: kalshi-prod-restored.xxx.rds.amazonaws.com
   ssh deduplicator "systemctl restart deduplicator"
   ```

6. **Verify recovery**
   ```bash
   # Check deduplicator can connect
   curl http://deduplicator:8080/health

   # Check data is flowing
   psql -h kalshi-prod-restored -d kalshi_prod -c \
     "SELECT MAX(received_at) FROM trades"
   ```

---

## Investigate Sequence Gaps

**Trigger:** SequenceGapsDetected alert, high gap rate

**Time:** 15-30 minutes

### Steps

1. **Check current gap rate**
   ```bash
   curl http://gatherer-1:9090/metrics | grep sequence_gaps
   ```

2. **Identify affected markets**
   ```bash
   # Check which markets have gaps
   curl http://gatherer-1:9090/metrics | grep writer_seq_gaps | sort -t'=' -k2 -rn | head -20
   ```

3. **Check connection health**
   ```bash
   curl http://gatherer-1:9090/metrics | grep conn_manager_connections_healthy
   ```

4. **Check for reconnection activity**
   ```bash
   curl http://gatherer-1:9090/metrics | grep reconnects_total
   ```

5. **Check Kalshi API status**
   ```bash
   curl https://api.elections.kalshi.com/trade-api/v2/exchange/status
   ```

6. **If gaps are from buffer overflow**
   ```bash
   # Check dropped messages
   curl http://gatherer-1:9090/metrics | grep dropped

   # Consider increasing buffer sizes in config
   # router.orderbook_buffer_size: 20000
   ```

7. **Verify data completeness via deduplication**
   ```bash
   # Check if other gatherers have the data
   for g in gatherer-{1,2,3}; do
     echo "$g: $(psql -h $g -d kalshi_ts -t -c \
       "SELECT COUNT(*) FROM trades WHERE exchange_ts BETWEEN $START AND $END")"
   done
   ```

---

## Connection Troubleshooting

**Trigger:** HighReconnectRate alert, unstable connections

**Time:** 15-30 minutes

### Steps

1. **Check reconnection rate**
   ```bash
   curl http://gatherer-1:9090/metrics | grep reconnects_total
   ```

2. **Check error types**
   ```bash
   curl http://gatherer-1:9090/metrics | grep ws_client_errors
   ```

3. **Check network connectivity**
   ```bash
   # Test WebSocket endpoint
   ssh gatherer-1 "curl -I https://api.elections.kalshi.com"

   # Check DNS resolution
   ssh gatherer-1 "dig api.elections.kalshi.com"

   # Check for packet loss
   ssh gatherer-1 "ping -c 10 api.elections.kalshi.com"
   ```

4. **Check for rate limiting**
   ```bash
   # Look for 429 or rate limit errors in logs
   ssh gatherer-1 "journalctl -u gatherer | grep -i 'rate\|429\|limit'"
   ```

5. **Check system resources**
   ```bash
   ssh gatherer-1 "top -bn1 | head -20"
   ssh gatherer-1 "free -m"
   ssh gatherer-1 "netstat -an | grep ESTABLISHED | wc -l"
   ```

6. **If issue persists, restart gatherer**
   ```bash
   ssh gatherer-1 "systemctl restart gatherer"
   ```

---

## Verify Data Integrity

**Trigger:** Routine check, after incident

**Time:** 10-15 minutes

### Steps

1. **Compare gatherer data**
   ```bash
   # Count trades across gatherers for recent hour
   for g in gatherer-{1,2,3}; do
     echo "$g trades: $(psql -h $g -d kalshi_ts -t -c \
       "SELECT COUNT(*) FROM trades
        WHERE received_at > extract(epoch from now()) * 1000000 - 3600000000")"
   done
   ```

2. **Check production vs gatherer totals**
   ```bash
   # Production count
   psql -h prod-rds -d kalshi_prod -t -c \
     "SELECT COUNT(*) FROM trades
      WHERE received_at > extract(epoch from now()) * 1000000 - 3600000000"

   # Should match max of gatherer counts
   ```

3. **Check for duplicate rate**
   ```bash
   curl http://deduplicator:9090/metrics | grep dedup_duplicate_rate
   # Expected: 50-67% (2-3 gatherers sending same data)
   ```

4. **Verify snapshot coverage**
   ```bash
   # REST snapshots should be every ~15 minutes
   psql -h prod-rds -d kalshi_prod -c \
     "SELECT ticker, COUNT(*), MAX(snapshot_ts)
      FROM orderbook_snapshots
      WHERE source = 'rest'
        AND snapshot_ts > extract(epoch from now()) * 1000000 - 3600000000
      GROUP BY ticker
      HAVING COUNT(*) < 50"  -- Expect ~60 per hour
   ```

5. **Check market coverage**
   ```bash
   # Active markets should all have recent data
   psql -h prod-rds -d kalshi_prod -c \
     "SELECT m.ticker, MAX(t.exchange_ts) as last_trade
      FROM markets m
      LEFT JOIN trades t ON t.ticker = m.ticker
      WHERE m.market_status = 'open'
      GROUP BY m.ticker
      HAVING MAX(t.exchange_ts) < extract(epoch from now()) * 1000000 - 86400000000
      OR MAX(t.exchange_ts) IS NULL"
   ```

---

## Post-Incident Checklist

After any incident:

- [ ] Verify all gatherers healthy
- [ ] Verify deduplicator syncing (lag < 10s)
- [ ] Verify production RDS accessible
- [ ] Check data integrity (counts match)
- [ ] Review metrics for anomalies
- [ ] Update incident log
- [ ] Schedule post-mortem if significant
