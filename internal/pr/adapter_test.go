package pr

import (
	"testing"
)

// TestAdapterInterface_CompileTimeContract asserts every shipped adapter
// satisfies the Adapter interface. Behavioural coverage per adapter lives
// in gh_test.go and glab_test.go.
func TestAdapterInterface_CompileTimeContract(t *testing.T) {
	var _ Adapter = (*GitHubAdapter)(nil)
	var _ Adapter = (*GitLabAdapter)(nil)
}
