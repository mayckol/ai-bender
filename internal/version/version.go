// Package version exposes the bender binary version.
//
// There are two ways to set it:
//
//  1. At build time via `-ldflags "-X github.com/mayckol/ai-bender/internal/version.Version=vX.Y.Z"`
//     (this is what the Makefile uses for release builds).
//  2. When built by `go install <module>@vX.Y.Z`, the ldflags are not applied; Resolve() falls back
//     to runtime/debug.ReadBuildInfo() and returns the module version embedded by the toolchain.
//
// Only when neither is available (e.g., `go build` from a dirty checkout with no tag) does it
// return "dev".
package version

import (
	"runtime/debug"
	"sync"
)

// Version is the build-time override. Do not read it directly from other packages — use Resolve().
var Version = "dev"

var (
	resolveOnce sync.Once
	resolved    string
)

// Resolve returns the best available version string: ldflags-injected first, then the embedded
// module version from debug.ReadBuildInfo(), then "dev".
func Resolve() string {
	resolveOnce.Do(func() {
		if Version != "" && Version != "dev" {
			resolved = Version
			return
		}
		if info, ok := debug.ReadBuildInfo(); ok {
			if v := info.Main.Version; v != "" && v != "(devel)" {
				resolved = v
				return
			}
		}
		resolved = Version
	})
	return resolved
}
