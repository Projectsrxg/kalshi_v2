// Package market implements the Market Registry component.
//
// The Market Registry:
//   - Discovers markets via REST API on startup
//   - Receives live updates via market_lifecycle WebSocket channel
//   - Maintains in-memory registry of active markets
//   - Notifies Connection Manager of market additions/removals
package market
