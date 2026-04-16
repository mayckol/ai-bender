package skill

import (
	"strings"
	"testing"
)

const validSkillMD = `---
name: check-data-model
context: bg
description: Verify data model in plan matches current code.
provides: [check, data-model, validation]
stages: [plan, ghu]
applies_to: [feature, architectural]
inputs:
  - .bender/artifacts/plan/data-model-*.md
outputs:
  - .bender/checks/data-model-<timestamp>.json
---
# check-data-model
Compare the resolved data model against the plan.
`

func TestParseFrontmatter_AcceptsValid(t *testing.T) {
	s, err := ParseFrontmatter([]byte(validSkillMD))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if s.Name != "check-data-model" {
		t.Fatalf("name: got %q want check-data-model", s.Name)
	}
	if s.Context != "bg" {
		t.Fatalf("context: got %q want bg", s.Context)
	}
	if !strings.Contains(s.Body, "# check-data-model") {
		t.Fatalf("body did not include heading: %q", s.Body)
	}
}

func TestParseFrontmatter_RejectsMissingFrontmatter(t *testing.T) {
	if _, err := ParseFrontmatter([]byte("# no frontmatter\n")); err != ErrMissingFrontmatter {
		t.Fatalf("got %v want ErrMissingFrontmatter", err)
	}
}

func TestParseFrontmatter_RejectsBadName(t *testing.T) {
	bad := strings.Replace(validSkillMD, "name: check-data-model", "name: BadName", 1)
	if _, err := ParseFrontmatter([]byte(bad)); err == nil {
		t.Fatal("expected validation error for bad name")
	}
}

func TestParseFrontmatter_RejectsUnknownContext(t *testing.T) {
	bad := strings.Replace(validSkillMD, "context: bg", "context: xyz", 1)
	if _, err := ParseFrontmatter([]byte(bad)); err == nil {
		t.Fatal("expected validation error for unknown context")
	}
}

func TestParseFrontmatter_RejectsEmptyProvides(t *testing.T) {
	bad := strings.Replace(validSkillMD, "provides: [check, data-model, validation]", "provides: []", 1)
	if _, err := ParseFrontmatter([]byte(bad)); err == nil {
		t.Fatal("expected validation error for empty provides")
	}
}

func TestParseFrontmatter_RejectsUnknownStage(t *testing.T) {
	bad := strings.Replace(validSkillMD, "stages: [plan, ghu]", "stages: [plan, fakestage]", 1)
	if _, err := ParseFrontmatter([]byte(bad)); err == nil {
		t.Fatal("expected validation error for unknown stage")
	}
}

func TestParseFrontmatter_RejectsUnknownIssueType(t *testing.T) {
	bad := strings.Replace(validSkillMD, "applies_to: [feature, architectural]", "applies_to: [whatever]", 1)
	if _, err := ParseFrontmatter([]byte(bad)); err == nil {
		t.Fatal("expected validation error for unknown applies_to")
	}
}
