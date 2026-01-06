# Testing TODO

## Current Coverage (as of 2024-01)

| Package | Coverage | Status |
|---------|----------|--------|
| api | 96.2% | Good |
| config | 100% | Complete |
| database | 57.1% | Needs integration tests |
| market | 67.2% | Needs more unit tests |
| model | N/A | Type definitions only |
| poller | 98.4% | Good |
| version | 100% | Complete |

## Remaining Work

### database package (57.1% → target 90%+)

**Blocked by:** Need real PostgreSQL/TimescaleDB or mock library

- [ ] `Pools.Ping()` - requires real database connections
- [ ] `Connect()` success path - requires real PostgreSQL server
- [ ] `NewPools()` second connection failure - requires first to succeed, second to fail

**Options:**
1. Add integration tests with `//go:build integration` tag that run against real databases
2. Use `github.com/pashagolub/pgxmock` to mock pgxpool

### market package (67.2% → target 90%+)

**Blocked by:** Complex timing/concurrency scenarios

- [ ] `reconcile()` - periodic reconciliation with status changes
- [ ] `reconciliationLoop()` - ticker-based loop
- [ ] `lifecycleLoop()` - WebSocket message handling (placeholder/TODO in code)
- [ ] `handleLifecycleMessage()` - WebSocket parsing (not yet implemented)
- [ ] `fetchMarket()` - unused helper function
- [ ] `initialSync()` error path when `GetAllMarkets` fails
- [ ] `Start()` with lifecycle source set (starts lifecycleLoop goroutine)

**Options:**
1. Use shorter reconcile intervals in tests with mock server
2. Test lifecycle loop by closing the channel

### api package (96.2% → target 98%+)

- [ ] Edge cases in pagination cursor handling
- [ ] Specific retry timing scenarios

### poller package (98.4% → target 100%)

- [ ] `Stop()` timeout path - removed flaky test, could use channel-based synchronization instead

## Integration Tests (Future)

Create `*_integration_test.go` files with build tag:

```go
//go:build integration

package api

func TestRealKalshiAPI(t *testing.T) {
    // Tests against https://demo-api.kalshi.co/trade-api/v2
}
```

Run with: `go test -tags=integration ./...`

## Notes

- Some code paths require real external services (databases, APIs)
- Timing-dependent tests are intentionally avoided to prevent flakiness
- WebSocket lifecycle handling in market package is a placeholder (TODO in code)
