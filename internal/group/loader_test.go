package group

import (
	"strings"
	"testing"
)

const sampleYAML = `groups:
  bootstrap:
    description: Run at bender init discovery phase.
    select:
      patterns: ["fg-bootstrap-*"]
    ordered: true
    halt_on_failure: false
  pre-implementation-checks:
    description: Validations run before crafter begins writing code.
    select:
      tags:
        any_of: [check, validation]
      context: [bg]
`

func TestParse_AcceptsValid(t *testing.T) {
	groups, err := Parse([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("got %d groups want 2", len(groups))
	}
	bs := groups["bootstrap"]
	if bs == nil {
		t.Fatal("bootstrap group missing")
	}
	if !bs.Ordered {
		t.Fatal("bootstrap.ordered must be true")
	}
	if bs.Select.Patterns[0] != "fg-bootstrap-*" {
		t.Fatalf("bootstrap pattern: %v", bs.Select.Patterns)
	}
}

func TestParse_RejectsEmptySelector(t *testing.T) {
	yaml := `groups:
  empty:
    description: Empty selector
    select: {}
`
	if _, err := Parse([]byte(yaml)); err == nil {
		t.Fatal("expected error for empty selector")
	}
}

func TestParse_RequiresDescription(t *testing.T) {
	yaml := `groups:
  no-desc:
    select:
      explicit: [foo]
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "description is required") {
		t.Fatalf("expected description-required error, got %v", err)
	}
}
