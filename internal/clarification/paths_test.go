package clarification

import "testing"

func TestArtifactPath(t *testing.T) {
	cases := []struct {
		ts   string
		want string
	}{
		{"", ""},
		{"2026-04-19T10-00-00-000", ".bender/artifacts/plan/clarifications-2026-04-19T10-00-00-000.md"},
		{"abc", ".bender/artifacts/plan/clarifications-abc.md"},
	}
	for _, c := range cases {
		if got := ArtifactPath(c.ts); got != c.want {
			t.Errorf("ArtifactPath(%q): got %q want %q", c.ts, got, c.want)
		}
	}
}
