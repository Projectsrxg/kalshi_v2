# Lifecycle

Connection states and sequences for WebSocket Client.

---

## State Machine

```mermaid
stateDiagram-v2
    [*] --> Disconnected
    Disconnected --> Connecting: Connect()
    Connecting --> Connected: Handshake OK
    Connecting --> Disconnected: Handshake failed
    Connected --> Disconnected: Error/Close
    Connected --> Disconnected: Close()
    Disconnected --> [*]
```

---

## Connect Sequence

```mermaid
sequenceDiagram
    participant CM as Connection Manager
    participant WS as WebSocket Client
    participant K as Kalshi WS

    CM->>WS: Connect(ctx)
    WS->>K: WebSocket handshake + auth headers
    K-->>WS: 101 Switching Protocols
    WS->>WS: Start read loop goroutine
    WS->>WS: Start heartbeat monitor goroutine
    WS-->>CM: nil (connected)
```

---

## Authentication

API key included as header during WebSocket handshake:

```go
func (c *client) Connect(ctx context.Context) error {
    headers := http.Header{
        "Authorization": []string{fmt.Sprintf("Bearer %s", c.cfg.APIKey)},
    }

    conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.cfg.URL, headers)
    if err != nil {
        return fmt.Errorf("dial failed: %w", err)
    }

    c.conn = conn
    c.connected = true

    go c.readLoop()
    go c.heartbeatLoop()

    return nil
}
```

---

## Close Sequence

```mermaid
sequenceDiagram
    participant CM as Connection Manager
    participant WS as WebSocket Client

    CM->>WS: Close()
    WS->>WS: Close done channel
    WS->>WS: Wait for goroutines to exit
    WS->>WS: Close WebSocket connection
    WS-->>CM: nil
```

```go
func (c *client) Close() error {
    c.mu.Lock()
    if !c.connected {
        c.mu.Unlock()
        return nil
    }
    c.connected = false
    c.mu.Unlock()

    close(c.done)

    return c.conn.Close()
}
```

**Note**: Client does NOT attempt reconnection. That's Connection Manager's job.
