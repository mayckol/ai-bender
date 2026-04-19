package pr

import (
	"fmt"
	"strings"
)

// SelectAdapter returns the first registered adapter whose Detect returns
// true for remoteURL. adapters is the ordered registry the caller maintains.
// Returns ErrNoAdapter (wrapped) when none match.
func SelectAdapter(adapters []Adapter, remoteURL string) (Adapter, error) {
	for _, a := range adapters {
		if a.Detect(remoteURL) {
			return a, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNoAdapter, remoteURL)
}

// IsGitHubURL reports whether remoteURL looks like a GitHub HTTPS or SSH
// remote. Intentionally permissive — exact hostname heuristics live here so
// gh.go stays focused on shelling out.
func IsGitHubURL(remoteURL string) bool {
	u := strings.ToLower(strings.TrimSpace(remoteURL))
	switch {
	case strings.HasPrefix(u, "https://github.com/"),
		strings.HasPrefix(u, "git@github.com:"),
		strings.HasPrefix(u, "ssh://git@github.com/"):
		return true
	}
	return false
}

// IsGitLabURL reports whether remoteURL looks like a GitLab remote.
func IsGitLabURL(remoteURL string) bool {
	u := strings.ToLower(strings.TrimSpace(remoteURL))
	switch {
	case strings.HasPrefix(u, "https://gitlab.com/"),
		strings.HasPrefix(u, "git@gitlab.com:"),
		strings.HasPrefix(u, "ssh://git@gitlab.com/"):
		return true
	}
	return false
}

// DefaultAdapters returns the adapter registry shipped with bender. Callers
// that need to inject a fake adapter for tests can assemble their own slice
// and pass it to SelectAdapter directly.
func DefaultAdapters(runner ExecRunner) []Adapter {
	return []Adapter{
		NewGitHubAdapter(runner),
		NewGitLabAdapter(runner),
	}
}
