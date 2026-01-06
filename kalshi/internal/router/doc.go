// Package router implements the Message Router component.
//
// The Message Router:
//   - Routes WebSocket messages to appropriate writers
//   - Uses non-blocking buffered channels
//   - Handles buffer overflow by dropping oldest messages
//   - Tracks metrics for routing performance
package router
