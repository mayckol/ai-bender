package clarification

import (
	"strings"
	"testing"
	"time"
)

func TestApply_RewritesMarkerForChosenResolution(t *testing.T) {
	spec := strings.Join([]string{
		"# Spec",
		"## Requirements",
		"- **FR-007**: System MUST authenticate via [NEEDS CLARIFICATION: auth method]",
		"- **FR-008**: System MUST log events",
	}, "\n")

	q := Question{
		ID:            "Q1",
		TargetSection: "requirements.functional.FR-007",
		Category:      CategorySecurityPrivacy,
		Priority:      1,
		Prompt:        "Which auth method?",
		SourceExcerpt: "System MUST authenticate via [NEEDS CLARIFICATION: auth method]",
		Options: []Option{
			{Label: "A", Text: "OAuth2", Implication: "Federated identity"},
			{Label: "B", Text: "Email/password", Implication: "Self-hosted"},
			{Label: "C", Text: "SSO via SAML", Implication: "Enterprise SSO"},
		},
	}
	r := Resolution{QuestionID: "Q1", Kind: KindChosen, ChosenLabel: "A", ResolvedAt: time.Now()}
	b := Batch{Questions: []Question{q}, Resolutions: []Resolution{r}}

	got, applied, skipped, err := Apply([]byte(spec), "specs/foo/spec.md", b)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected no skipped, got %v", skipped)
	}
	if len(applied) != 1 || applied[0] != "specs/foo/spec.md" {
		t.Fatalf("applied: %v", applied)
	}
	if !strings.Contains(string(got), "authenticate via OAuth2") {
		t.Fatalf("rewrite failed:\n%s", string(got))
	}
	if strings.Contains(string(got), "[NEEDS CLARIFICATION: auth method]") {
		t.Fatalf("marker should be gone:\n%s", string(got))
	}
	if !strings.Contains(string(got), "FR-008**: System MUST log events") {
		t.Fatalf("unrelated lines must be preserved:\n%s", string(got))
	}
}

func TestApply_LeavesMarkerWhenSkipped(t *testing.T) {
	spec := "## R\n- FR-007: x [NEEDS CLARIFICATION: y]\n"
	q := Question{
		ID: "Q1", TargetSection: "FR-007", Category: CategoryScope, Priority: 1,
		Prompt: "p", SourceExcerpt: "x",
		Options: []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
	}
	r := Resolution{QuestionID: "Q1", Kind: KindSkipped, ResolvedAt: time.Now()}
	got, applied, skipped, err := Apply([]byte(spec), "spec.md", Batch{Questions: []Question{q}, Resolutions: []Resolution{r}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("applied should be empty for skipped, got %v", applied)
	}
	if len(skipped) != 0 {
		// Skipped resolutions are not "skipped rewrites" — they are intentionally not applied.
		// The skipped slice is for resolutions that wanted to apply but couldn't locate a target.
		t.Fatalf("skipped should be empty when resolution kind is Skipped, got %v", skipped)
	}
	if !strings.Contains(string(got), "[NEEDS CLARIFICATION: y]") {
		t.Fatalf("marker should remain on skip:\n%s", string(got))
	}
}

func TestApply_ReportsSkippedWhenTargetMissing(t *testing.T) {
	spec := "## R\n- FR-008: nothing here\n"
	q := Question{
		ID: "Q1", TargetSection: "FR-007", Category: CategoryScope, Priority: 1,
		Prompt: "p", SourceExcerpt: "x",
		Options: []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
	}
	r := Resolution{QuestionID: "Q1", Kind: KindChosen, ChosenLabel: "A", ResolvedAt: time.Now()}
	_, _, skipped, err := Apply([]byte(spec), "spec.md", Batch{Questions: []Question{q}, Resolutions: []Resolution{r}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(skipped) != 1 || skipped[0] != "Q1" {
		t.Fatalf("expected Q1 in skipped, got %v", skipped)
	}
}

func TestApply_CustomTextReplacesMarker(t *testing.T) {
	spec := "FR-009: foo [NEEDS CLARIFICATION: bar]"
	q := Question{
		ID: "Q1", TargetSection: "FR-009", Category: CategoryScope, Priority: 1,
		Prompt: "p", SourceExcerpt: "x",
		Options: []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
	}
	r := Resolution{QuestionID: "Q1", Kind: KindCustom, CustomText: "owners + auditors", ResolvedAt: time.Now()}
	got, _, _, err := Apply([]byte(spec), "spec.md", Batch{Questions: []Question{q}, Resolutions: []Resolution{r}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(got), "foo owners + auditors") {
		t.Fatalf("custom rewrite failed: %s", string(got))
	}
}
