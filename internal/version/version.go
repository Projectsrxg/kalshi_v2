// Package version provides build-time version information.
//
// Variables are set at build time via ldflags:
//
//	go build -ldflags "-X github.com/rickgao/kalshi-data/internal/version.Version=1.0.0 \
//	                   -X github.com/rickgao/kalshi-data/internal/version.Commit=$(git rev-parse --short HEAD) \
//	                   -X github.com/rickgao/kalshi-data/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

// Build-time variables (set via ldflags)
var (
	// Version is the semantic version (e.g., "1.0.0")
	Version = "dev"

	// Commit is the git commit hash (short form)
	Commit = "unknown"

	// BuildTime is the UTC build timestamp (ISO 8601)
	BuildTime = "unknown"
)

// String returns a formatted version string.
func String() string {
	return Version + " (" + Commit + ") built " + BuildTime
}
