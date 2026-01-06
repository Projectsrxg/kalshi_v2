package version

import (
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		// Save original values
		origVersion := Version
		origCommit := Commit
		origBuildTime := BuildTime
		defer func() {
			Version = origVersion
			Commit = origCommit
			BuildTime = origBuildTime
		}()

		// Set default values
		Version = "dev"
		Commit = "unknown"
		BuildTime = "unknown"

		result := String()

		if !strings.Contains(result, "dev") {
			t.Errorf("String() = %q, should contain 'dev'", result)
		}
		if !strings.Contains(result, "unknown") {
			t.Errorf("String() = %q, should contain 'unknown'", result)
		}
		if !strings.Contains(result, "built") {
			t.Errorf("String() = %q, should contain 'built'", result)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		// Save original values
		origVersion := Version
		origCommit := Commit
		origBuildTime := BuildTime
		defer func() {
			Version = origVersion
			Commit = origCommit
			BuildTime = origBuildTime
		}()

		// Set custom values
		Version = "1.2.3"
		Commit = "abc1234"
		BuildTime = "2024-01-15T10:00:00Z"

		result := String()

		expected := "1.2.3 (abc1234) built 2024-01-15T10:00:00Z"
		if result != expected {
			t.Errorf("String() = %q, want %q", result, expected)
		}
	})

	t.Run("format validation", func(t *testing.T) {
		result := String()

		// Should contain parentheses around commit
		if !strings.Contains(result, "(") || !strings.Contains(result, ")") {
			t.Errorf("String() = %q, should contain parentheses", result)
		}

		// Should contain "built" before build time
		if !strings.Contains(result, "built") {
			t.Errorf("String() = %q, should contain 'built'", result)
		}
	})
}

func TestDefaultValues(t *testing.T) {
	// Test that package variables are initialized with defaults
	// Note: These might be overwritten by ldflags in production builds
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Commit == "" {
		t.Error("Commit should not be empty")
	}
	if BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}
}
