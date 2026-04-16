package agent

import (
	"strings"
	"testing"
)

const validAgentMD = `---
name: crafter
purpose: Implement production code.
persona_hint: Precise, conservative, minimum-diff.
write_scope:
  allow: ["**/*.go", "**/*.ts"]
  deny:  ["**/*_test.go", "docs/**"]
skills:
  patterns: ["bg-crafter-*", "check-*"]
  tags:
    none_of: [destructive]
context: [bg]
invoked_by: [ghu, implement]
---
# crafter
Implement production code with minimal diffs.
`

func TestParseAgent_AcceptsValid(t *testing.T) {
	a, err := ParseAgent([]byte(validAgentMD))
	if err != nil {
		t.Fatalf("ParseAgent: %v", err)
	}
	if a.Name != "crafter" {
		t.Fatalf("name: got %q", a.Name)
	}
	if !strings.Contains(a.Body, "# crafter") {
		t.Fatalf("body missing heading: %q", a.Body)
	}
	if len(a.WriteScope.Allow) != 2 || len(a.WriteScope.Deny) != 2 {
		t.Fatalf("write_scope not parsed: %+v", a.WriteScope)
	}
}

func TestParseAgent_RejectsBadName(t *testing.T) {
	bad := strings.Replace(validAgentMD, "name: crafter", "name: Crafter", 1)
	if _, err := ParseAgent([]byte(bad)); err == nil {
		t.Fatal("expected error for invalid name")
	}
}

func TestParseAgent_RequiresInvokedBy(t *testing.T) {
	bad := strings.Replace(validAgentMD, "invoked_by: [ghu, implement]", "invoked_by: []", 1)
	if _, err := ParseAgent([]byte(bad)); err == nil {
		t.Fatal("expected error for empty invoked_by")
	}
}

func TestParseAgent_RejectsUnknownStage(t *testing.T) {
	bad := strings.Replace(validAgentMD, "invoked_by: [ghu, implement]", "invoked_by: [whatever]", 1)
	if _, err := ParseAgent([]byte(bad)); err == nil {
		t.Fatal("expected error for unknown invoked_by stage")
	}
}
