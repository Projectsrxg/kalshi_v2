// Package poller implements the Snapshot Poller component.
//
// The Snapshot Poller:
//   - Polls REST API every 15 minutes for orderbook snapshots
//   - Provides backup data source for gap recovery
//   - Uses concurrent requests with rate limiting
//   - Stores snapshots with source="rest" marker
package poller
