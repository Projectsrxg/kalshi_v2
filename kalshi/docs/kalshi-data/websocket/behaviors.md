# Behaviors

Read loop, heartbeat, and send for WebSocket Client.

---

## Read Loop

Sends all raw bytes to messages channel. No parsing or filtering. Timestamps each message immediately when received.

```go
func (c *client) readLoop() {
    defer close(c.messages)

    for {
        select {
        case <-c.done:
            return
        default:
        }

        _, data, err := c.conn.ReadMessage()
        receivedAt := time.Now() // Capture timestamp immediately

        if err != nil {
            // Ignore errors after Close() is called
            select {
            case <-c.done:
                return
            default:
                c.errors <- err
                return
            }
        }

        msg := TimestampedMessage{
            Data:       data,
            ReceivedAt: receivedAt,
        }

        select {
        case c.messages <- msg:
        case <-c.done:
            return
        default:
            c.logger.Warn("message buffer full, dropping message")
            c.metrics.DroppedMessages.Inc()
        }
    }
}
```

**Dual timestamps:** Messages have two timestamps:
1. `ReceivedAt` (local): Captured here when `ReadMessage()` returns
2. `exchange_ts` (server): In message payload, parsed by downstream components

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

## Send

Writes raw bytes to connection. Serialized via mutex.

```go
func (c *client) Send(data []byte) error {
    c.mu.RLock()
    if !c.connected {
        c.mu.RUnlock()
        return ErrNotConnected
    }
    c.mu.RUnlock()

    c.writeMu.Lock()
    defer c.writeMu.Unlock()

    c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
    return c.conn.WriteMessage(websocket.TextMessage, data)
}
```
