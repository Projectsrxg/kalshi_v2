// Package connection implements the Connection Manager component.
//
// The Connection Manager:
//   - Maintains 150 WebSocket connections per gatherer
//   - 144 orderbook connections (250 markets each)
//   - 6 global connections (trades, tickers, lifecycle)
//   - Handles reconnection with exponential backoff
//   - Routes incoming messages to the Message Router
package connection
