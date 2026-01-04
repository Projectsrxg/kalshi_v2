# Behaviors

Event handling, command correlation, and sequence tracking for Connection Manager.

---

## Handling MarketChange Events

Uses worker pool to avoid blocking on subscribe timeouts.

```go
func (m *manager) handleMarketChanges() {
    defer m.wg.Done()

    changes := m.registry.SubscribeChanges()

    // Worker pool for non-blocking subscribes
    workCh := make(chan MarketChange, 100)
    for i := 0; i < 10; i++ {
        m.wg.Add(1)
        go m.subscribeWorker(workCh)
    }

    for {
        select {
        case <-m.ctx.Done():
            close(workCh)
            return
        case change := <-changes:
            select {
            case workCh <- change:
            default:
                m.logger.Warn("subscribe worker backpressure, dropping event")
                m.metrics.DroppedEvents.Inc()
            }
        }
    }
}

func (m *manager) subscribeWorker(workCh <-chan MarketChange) {
    defer m.wg.Done()

    for change := range workCh {
        m.handleMarketChange(change)
    }
}

func (m *manager) handleMarketChange(change MarketChange) {
    switch change.EventType {
    case "created":
        if change.NewStatus == "active" {
            m.subscribeOrderbook(change.Ticker)
        }

    case "status_change":
        if change.NewStatus == "active" && change.OldStatus != "active" {
            m.subscribeOrderbook(change.Ticker)
        } else if change.NewStatus != "active" && change.OldStatus == "active" {
            m.unsubscribeOrderbook(change.Ticker)
        }

    case "settled":
        m.unsubscribeOrderbook(change.Ticker)
    }
}
```

| Event | Condition | Action |
|-------|-----------|--------|
| `created` | `status == active` | Subscribe orderbook |
| `status_change` | `inactive → active` | Subscribe orderbook |
| `status_change` | `active → inactive` | Unsubscribe orderbook |
| `settled` | - | Unsubscribe orderbook |

---

## Least-Loaded Market Assignment

New markets are assigned to the orderbook connection with fewest active subscriptions.

```go
func (m *manager) selectOrderbookConn() *connState {
    var minConn *connState
    minCount := math.MaxInt

    for _, conn := range m.orderbookConns {
        if conn == nil || !conn.client.IsConnected() {
            continue
        }
        conn.mu.Lock()
        count := len(conn.markets)
        conn.mu.Unlock()
        if count < minCount {
            minCount = count
            minConn = conn
        }
    }
    return minConn
}

func (m *manager) subscribeOrderbook(ticker string) {
    conn := m.selectOrderbookConn()
    if conn == nil {
        m.logger.Error("no healthy orderbook connections", "ticker", ticker)
        return
    }

    // Track assignment (store connection ID, not index)
    m.marketConnMu.Lock()
    m.marketToConn[ticker] = conn.id
    m.marketConnMu.Unlock()

    conn.mu.Lock()
    conn.markets[ticker] = struct{}{}
    conn.mu.Unlock()

    // Send subscribe command
    m.subscribe(conn, "orderbook_delta", ticker)
}

func (m *manager) unsubscribeOrderbook(ticker string) {
    m.marketConnMu.RLock()
    connID, ok := m.marketToConn[ticker]
    m.marketConnMu.RUnlock()

    if !ok {
        return
    }

    // Convert connection ID (7-150) to array index (0-143)
    conn := m.orderbookConns[connID-7]

    conn.mu.Lock()
    delete(conn.markets, ticker)
    conn.mu.Unlock()

    // Find SID for this ticker and unsubscribe
    m.subsMu.RLock()
    var sid int64
    for s, sub := range m.subs {
        if sub.Ticker == ticker && sub.Channel == "orderbook_delta" {
            sid = s
            break
        }
    }
    m.subsMu.RUnlock()

    if sid != 0 {
        m.unsubscribe(conn, sid)
    }

    m.marketConnMu.Lock()
    delete(m.marketToConn, ticker)
    m.marketConnMu.Unlock()
}
```

---

## Command/Response Correlation

WebSocket Client sends all messages to one channel. Connection Manager separates command responses from data messages.

```go
func (m *manager) readLoop(conn *connState) {
    defer m.wg.Done()
    defer close(conn.readLoopDone)

    for {
        select {
        case <-m.ctx.Done():
            return
        case msg, ok := <-conn.client.Messages():
            if !ok {
                return
            }

            // Try to parse as command response
            if resp, ok := m.tryParseResponse(msg.Data); ok {
                conn.routeResponse(resp)
                continue
            }

            // Route lifecycle messages to Market Registry
            if conn.role == "lifecycle" {
                select {
                case m.lifecycle <- msg.Data:
                case <-m.ctx.Done():
                    return
                }
                continue
            }

            // Check sequence for orderbook messages
            var seqGap bool
            var gapSize int
            if conn.role == "orderbook" {
                if sid, seq, ok := m.extractSequence(msg.Data); ok {
                    seqGap, gapSize = m.checkSequence(sid, seq)
                }
            }

            // Data message - forward to router (non-blocking)
            rawMsg := RawMessage{
                Data:       msg.Data,
                ConnID:     conn.id,
                ReceivedAt: msg.ReceivedAt,
                SeqGap:     seqGap,
                GapSize:    gapSize,
            }

            select {
            case m.router <- rawMsg:
            case <-m.ctx.Done():
                return
            default:
                m.logger.Warn("message buffer full, dropping")
                m.metrics.DroppedMessages.Inc()
            }
        }
    }
}

func (m *manager) tryParseResponse(data []byte) (Response, bool) {
    // Quick check for response markers
    if !bytes.Contains(data, []byte(`"id":`)) {
        return Response{}, false
    }

    var resp Response
    if err := json.Unmarshal(data, &resp); err != nil {
        return Response{}, false
    }

    // Valid response types
    switch resp.Type {
    case "subscribed", "unsubscribed", "error":
        return resp, true
    }

    return Response{}, false
}

func (c *connState) routeResponse(resp Response) {
    c.pendingMu.Lock()
    ch, ok := c.pending[resp.ID]
    if ok {
        delete(c.pending, resp.ID)
    }
    c.pendingMu.Unlock()

    if ok {
        ch <- resp
    }
}
```

---

## Subscribe/Unsubscribe Commands

```go
func (m *manager) subscribe(conn *connState, channel, ticker string) error {
    id := atomic.AddInt64(&conn.cmdID, 1)
    respCh := make(chan Response, 1)

    conn.pendingMu.Lock()
    conn.pending[id] = respCh
    conn.pendingMu.Unlock()

    defer func() {
        conn.pendingMu.Lock()
        delete(conn.pending, id)
        conn.pendingMu.Unlock()
    }()

    // Build command
    cmd := map[string]interface{}{
        "id":  id,
        "cmd": "subscribe",
        "params": map[string]interface{}{
            "channels": []string{channel},
        },
    }
    if ticker != "" {
        cmd["params"].(map[string]interface{})["market_ticker"] = ticker
    }

    data, _ := json.Marshal(cmd)
    if err := conn.client.Send(data); err != nil {
        return err
    }

    // Wait for response
    select {
    case <-m.ctx.Done():
        return m.ctx.Err()
    case <-time.After(m.cfg.SubscribeTimeout):
        return ErrTimeout
    case resp := <-respCh:
        if resp.Type == "error" {
            var errMsg ErrorMsg
            json.Unmarshal(resp.Msg, &errMsg)
            return fmt.Errorf("%s: %s", errMsg.Code, errMsg.Message)
        }

        // Track subscription
        var subMsg SubscribedMsg
        json.Unmarshal(resp.Msg, &subMsg)

        m.subsMu.Lock()
        m.subs[subMsg.SID] = &subscription{
            SID:     subMsg.SID,
            Channel: channel,
            ConnID:  conn.id,
            Ticker:  ticker,
        }
        m.subsMu.Unlock()

        return nil
    }
}

func (m *manager) unsubscribe(conn *connState, sid int64) error {
    id := atomic.AddInt64(&conn.cmdID, 1)
    respCh := make(chan Response, 1)

    conn.pendingMu.Lock()
    conn.pending[id] = respCh
    conn.pendingMu.Unlock()

    defer func() {
        conn.pendingMu.Lock()
        delete(conn.pending, id)
        conn.pendingMu.Unlock()
    }()

    cmd := map[string]interface{}{
        "id":  id,
        "cmd": "unsubscribe",
        "params": map[string]interface{}{
            "sids": []int64{sid},
        },
    }

    data, _ := json.Marshal(cmd)
    if err := conn.client.Send(data); err != nil {
        return err
    }

    select {
    case <-m.ctx.Done():
        return m.ctx.Err()
    case <-time.After(m.cfg.SubscribeTimeout):
        return ErrTimeout
    case resp := <-respCh:
        if resp.Type == "error" {
            var errMsg ErrorMsg
            json.Unmarshal(resp.Msg, &errMsg)
            return fmt.Errorf("%s: %s", errMsg.Code, errMsg.Message)
        }

        m.subsMu.Lock()
        delete(m.subs, sid)
        m.subsMu.Unlock()

        return nil
    }
}
```

---

## Sequence Gap Detection

Orderbook messages include sequence numbers. Detect gaps and log warnings.

```go
func (m *manager) checkSequence(sid int64, seq int64) (seqGap bool, gapSize int) {
    m.seqMu.Lock()
    defer m.seqMu.Unlock()

    last, exists := m.lastSeq[sid]
    if !exists {
        // First message for this subscription (snapshot)
        m.lastSeq[sid] = seq
        return false, 0
    }

    if seq != last+1 {
        gap := int(seq - last - 1)
        m.logger.Warn("sequence gap detected",
            "sid", sid,
            "expected", last+1,
            "got", seq,
            "gap", gap,
        )
        m.metrics.SequenceGaps.Inc()
        m.lastSeq[sid] = seq
        return true, gap
    }

    m.lastSeq[sid] = seq
    return false, 0
}
```

**On gap detection:** Log warning and continue. No resubscription. Backup data sources:
- REST snapshot polling (15-minute resolution)
- Deduplicator pulls from other gatherers
