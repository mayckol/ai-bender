package version

import (
	"sync"
	"testing"
)

// resetForTest clears the resolveOnce memoization so tests can rerun Resolve under different
// Version values. It is ONLY for tests.
func resetForTest() {
	resolveOnce = sync.Once{}
	resolved = ""
}

func TestResolve_HonorsLdflagsOverride(t *testing.T) {
	t.Cleanup(resetForTest)
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "v1.2.3"
	resetForTest()
	if got := Resolve(); got != "v1.2.3" {
		t.Fatalf("Resolve with ldflags-style Version: got %q want v1.2.3", got)
	}
}

func TestResolve_FallsBackToBuildInfoWhenDev(t *testing.T) {
	t.Cleanup(resetForTest)
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "dev"
	resetForTest()
	got := Resolve()
	// In `go test` runs, debug.ReadBuildInfo().Main.Version is "(devel)" for the module under test
	// (the code is being tested, not installed). So Resolve returns "dev" — the documented fallback.
	// In a `go install <mod>@vX.Y.Z` build the Main.Version would be "vX.Y.Z" and Resolve would
	// return that. We can't simulate a real install here, but we can at least assert the fallback
	// is "dev" rather than the empty string.
	if got == "" {
		t.Fatalf("Resolve returned empty string; want non-empty fallback")
	}
	if got != "dev" && got[0] != 'v' {
		t.Fatalf("Resolve returned %q; want 'dev' or a semver string starting with 'v'", got)
	}
}

func TestResolve_Memoizes(t *testing.T) {
	t.Cleanup(resetForTest)
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "v9.9.9"
	resetForTest()
	first := Resolve()
	// Mutating Version after first Resolve must NOT change the answer because Resolve memoizes.
	Version = "v0.0.0"
	second := Resolve()
	if first != second {
		t.Fatalf("Resolve should memoize; first=%q second=%q", first, second)
	}
}
