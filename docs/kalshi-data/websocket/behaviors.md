# Behaviors

Read loop, heartbeat, and command handling for WebSocket Client.

---

## Read Loop

Returns raw bytes - no parsing. Message Router handles parsing.

```go
func (c *client) readLoop() {
    defer close(c.messages)

    for {
        _, data, err := c.conn.ReadMessage()
        if err != nil {
            c.errors <- err
            return
        }

        // Check if this is a command response (for Subscribe/Unsubscribe)
        if c.isCommandResponse(data) {
            c.routeResponse(data)
            continue
        }

        // Data message - send raw bytes to channel
        select {
        case c.messages <- data:
        default:
            c.logger.Warn("message buffer full, dropping message")
        }
    }
}

func (c *client) isCommandResponse(data []byte) bool {
    // Quick check for response types
    return bytes.Contains(data, []byte(`"type":"subscribed"`)) ||
           bytes.Contains(data, []byte(`"type":"unsubscribed"`)) ||
           bytes.Contains(data, []byte(`"type":"error"`))
}
```

---

## Response Routing

Commands block waiting for their response. Uses map of pending requests.

```go
type pendingRequest struct {
    respChan chan Response
}

func (c *client) routeResponse(data []byte) {
    var resp Response
    if err := json.Unmarshal(data, &resp); err != nil {
        c.logger.Warn("failed to parse response", "err", err)
        return
    }

    c.pendingMu.Lock()
    pending, ok := c.pending[resp.ID]
    if ok {
        delete(c.pending, resp.ID)
    }
    c.pendingMu.Unlock()

    if ok {
        pending.respChan <- resp
    }
}
```

---

## Heartbeat Monitor

Server sends ping frames every 10 seconds. Client responds with pong. Monitor detects stale connections.

```go
func (c *client) heartbeatLoop() {
    c.conn.SetPingHandler(func(data string) error {
        c.mu.Lock()
        c.lastPingAt = time.Now()
        c.mu.Unlock()

        return c.conn.WriteControl(
            websocket.PongMessage,
            []byte(data),
            time.Now().Add(time.Second),
        )
    })

    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-c.done:
            return
        case <-ticker.C:
            c.mu.RLock()
            lastPing := c.lastPingAt
            c.mu.RUnlock()

            if time.Since(lastPing) > c.cfg.PingTimeout {
                c.logger.Warn("no ping received, connection stale")
                c.errors <- ErrStaleConnection
                return
            }
        }
    }
}
```

---

## Subscribe

Sends command, waits for confirmation, returns subscription IDs.

```go
func (c *client) Subscribe(ctx context.Context, channels []string, tickers []string) ([]SubscriptionResult, error) {
    if !c.connected {
        return nil, ErrNotConnected
    }

    id := atomic.AddInt64(&c.cmdID, 1)
    respChan := make(chan Response, len(channels))

    // Register pending request
    c.pendingMu.Lock()
    c.pending[id] = &pendingRequest{respChan: respChan}
    c.pendingMu.Unlock()

    defer func() {
        c.pendingMu.Lock()
        delete(c.pending, id)
        c.pendingMu.Unlock()
    }()

    // Send command
    cmd := Command{
        ID:  id,
        Cmd: "subscribe",
        Params: SubscribeParams{
            Channels:      channels,
            MarketTickers: tickers,
        },
    }

    c.writeMu.Lock()
    err := c.conn.WriteJSON(cmd)
    c.writeMu.Unlock()
    if err != nil {
        return nil, fmt.Errorf("write failed: %w", err)
    }

    // Wait for responses (one per channel)
    results := make([]SubscriptionResult, 0, len(channels))
    timeout := time.After(c.cfg.ResponseTimeout)

    for i := 0; i < len(channels); i++ {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-timeout:
            return nil, ErrTimeout
        case resp := <-respChan:
            if resp.Type == "error" {
                var errMsg ErrorMsg
                json.Unmarshal(resp.Msg, &errMsg)
                return nil, fmt.Errorf("subscribe error: %s - %s", errMsg.Code, errMsg.Message)
            }

            var subMsg SubscribedMsg
            json.Unmarshal(resp.Msg, &subMsg)
            results = append(results, SubscriptionResult{
                SID:     subMsg.SID,
                Channel: subMsg.Channel,
            })
        }
    }

    return results, nil
}
```

---

## Unsubscribe

```go
func (c *client) Unsubscribe(ctx context.Context, sids []int64) error {
    if !c.connected {
        return ErrNotConnected
    }

    id := atomic.AddInt64(&c.cmdID, 1)
    respChan := make(chan Response, 1)

    c.pendingMu.Lock()
    c.pending[id] = &pendingRequest{respChan: respChan}
    c.pendingMu.Unlock()

    defer func() {
        c.pendingMu.Lock()
        delete(c.pending, id)
        c.pendingMu.Unlock()
    }()

    cmd := Command{
        ID:  id,
        Cmd: "unsubscribe",
        Params: UnsubscribeParams{SIDs: sids},
    }

    c.writeMu.Lock()
    err := c.conn.WriteJSON(cmd)
    c.writeMu.Unlock()
    if err != nil {
        return fmt.Errorf("write failed: %w", err)
    }

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(c.cfg.ResponseTimeout):
        return ErrTimeout
    case resp := <-respChan:
        if resp.Type == "error" {
            var errMsg ErrorMsg
            json.Unmarshal(resp.Msg, &errMsg)
            return fmt.Errorf("unsubscribe error: %s - %s", errMsg.Code, errMsg.Message)
        }
        return nil
    }
}
```

---

## Update Subscription

```go
func (c *client) UpdateSubscription(ctx context.Context, sid int64, action string, tickers []string) error {
    if !c.connected {
        return ErrNotConnected
    }

    id := atomic.AddInt64(&c.cmdID, 1)
    respChan := make(chan Response, 1)

    c.pendingMu.Lock()
    c.pending[id] = &pendingRequest{respChan: respChan}
    c.pendingMu.Unlock()

    defer func() {
        c.pendingMu.Lock()
        delete(c.pending, id)
        c.pendingMu.Unlock()
    }()

    cmd := Command{
        ID:  id,
        Cmd: "update_subscription",
        Params: UpdateParams{
            SID:           sid,
            Action:        action,
            MarketTickers: tickers,
        },
    }

    c.writeMu.Lock()
    err := c.conn.WriteJSON(cmd)
    c.writeMu.Unlock()
    if err != nil {
        return fmt.Errorf("write failed: %w", err)
    }

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(c.cfg.ResponseTimeout):
        return ErrTimeout
    case resp := <-respChan:
        if resp.Type == "error" {
            var errMsg ErrorMsg
            json.Unmarshal(resp.Msg, &errMsg)
            return fmt.Errorf("update error: %s - %s", errMsg.Code, errMsg.Message)
        }
        return nil
    }
}
```
