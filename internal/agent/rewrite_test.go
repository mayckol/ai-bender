package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mayckol/ai-bender/internal/config"
)

const sampleAgent = `---
name: crafter
purpose: "Apply patches"
persona_hint: "sharp focused engineer"
write_scope:
  allow:
    - "pkg/**"
    - "cmd/**"
  deny:
    - "internal/legacy/**"
skills:
  patterns:
    - "bg-crafter-*"
invoked_by:
  - ghu
---

# crafter

Lorem ipsum.
`

func writeSample(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "crafter.md")
	if err := os.WriteFile(path, []byte(sampleAgent), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRewriteFile_AppliesOverrideAndReportsChange(t *testing.T) {
	path := writeSample(t)
	ov := config.AgentOverride{}
	ov.Skills.Add = []string{"my-extra-skill"}
	ov.WriteScope.DenyAdd = []string{"vendor/**"}

	changed, err := RewriteFile(path, ov)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed=true on first apply")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{"my-extra-skill", "vendor/**"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q in rewritten file, got:\n%s", want, s)
		}
	}
	// Body must survive.
	if !strings.Contains(s, "# crafter") || !strings.Contains(s, "Lorem ipsum.") {
		t.Fatalf("body lost after rewrite:\n%s", s)
	}
}

func TestRewriteFile_Idempotent(t *testing.T) {
	path := writeSample(t)
	ov := config.AgentOverride{}
	ov.Skills.Add = []string{"my-extra-skill"}

	if _, err := RewriteFile(path, ov); err != nil {
		t.Fatal(err)
	}
	changed, err := RewriteFile(path, ov)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second apply with same override must be a no-op")
	}
}

func TestRewriteFile_NoOverrideMeansNoChange(t *testing.T) {
	// First canonicalise the file by applying an empty override so the
	// yaml.v3 field ordering matches the encoder's output; then assert the
	// second no-op pass is byte-identical.
	path := writeSample(t)
	empty := config.AgentOverride{}
	if _, err := RewriteFile(path, empty); err != nil {
		t.Fatal(err)
	}
	changed, err := RewriteFile(path, empty)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("applying an empty override twice must not drift")
	}
}
