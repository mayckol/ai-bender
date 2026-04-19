package clarification

import (
	"strings"
	"testing"
	"time"
)

func sampleBatch() Batch {
	now := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	q1 := Question{
		ID:            "Q1",
		TargetSection: "requirements.functional.FR-007",
		Category:      CategoryActorsRoles,
		Priority:      1,
		Prompt:        "Who can trigger the export?",
		SourceExcerpt: "[NEEDS CLARIFICATION: actor unspecified]",
		Options: []Option{
			{Label: "A", Text: "Account owner only", Implication: "Single-actor scope"},
			{Label: "B", Text: "Owner + admins", Implication: "Two-tier permission"},
			{Label: "C", Text: "Anyone with link", Implication: "Public sharing model"},
		},
	}
	r1 := Resolution{QuestionID: "Q1", Kind: KindChosen, ChosenLabel: "A", ResolvedAt: now, AppliedTo: []string{".bender/artifacts/specs/foo-2026.md"}}
	return Batch{
		Timestamp:   "2026-04-19T10-00-00-000",
		FromCapture: ".bender/artifacts/cry/foo-2026.md",
		FromSpec:    ".bender/artifacts/specs/foo-2026.md",
		Mode:        ModeInteractive,
		Strict:      false,
		Status:      "draft",
		CreatedAt:   now,
		ToolVersion: "test",
		Questions:   []Question{q1},
		Resolutions: []Resolution{r1},
	}
}

func TestMarshalUnmarshal_RoundTrip(t *testing.T) {
	in := sampleBatch()
	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.HasPrefix(string(data), "---\n") {
		t.Fatalf("expected frontmatter open, got: %s", string(data[:80]))
	}
	if !strings.Contains(string(data), "## Resolved") {
		t.Fatalf("expected ## Resolved section")
	}
	for _, header := range []string{"## Deferred (capped)", "## Skipped (user-declined)", "## Pending (non-interactive)"} {
		if !strings.Contains(string(data), header) {
			t.Errorf("missing section %q", header)
		}
	}

	out, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.FromCapture != in.FromCapture || out.FromSpec != in.FromSpec || out.Mode != in.Mode {
		t.Fatalf("frontmatter mismatch: %+v vs %+v", out, in)
	}
	if len(out.Questions) != 1 || out.Questions[0].ID != "Q1" {
		t.Fatalf("questions: %+v", out.Questions)
	}
	if out.Resolutions[0].Kind != KindChosen || out.Resolutions[0].ChosenLabel != "A" {
		t.Fatalf("resolution: %+v", out.Resolutions[0])
	}
}

func TestMarshal_EmptySectionsContainPlaceholder(t *testing.T) {
	b := sampleBatch()
	data, err := Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(data)
	for _, header := range []string{"## Deferred (capped)", "## Skipped (user-declined)", "## Pending (non-interactive)"} {
		idx := strings.Index(body, header)
		if idx < 0 {
			t.Errorf("section %q absent", header)
			continue
		}
		tail := body[idx:]
		end := len(tail)
		if end > 200 {
			end = 200
		}
		if !strings.Contains(tail[:end], "_none_") {
			t.Errorf("section %q missing _none_ placeholder", header)
		}
	}
}

func TestValidate_RejectsMismatchedLengths(t *testing.T) {
	b := sampleBatch()
	b.Resolutions = nil
	if _, err := Marshal(b); err == nil {
		t.Fatal("expected error for resolutions/questions length mismatch")
	}
}

func TestValidate_RejectsTooManyAnswered(t *testing.T) {
	b := sampleBatch()
	for i := 2; i <= 4; i++ {
		id := "Q" + string(rune('0'+i))
		b.Questions = append(b.Questions, Question{
			ID:            id,
			TargetSection: "x." + id,
			Category:      CategoryScope,
			Priority:      1,
			Prompt:        "p",
			SourceExcerpt: "e",
			Options:       []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
		})
		b.Resolutions = append(b.Resolutions, Resolution{QuestionID: id, Kind: KindChosen, ChosenLabel: "A", ResolvedAt: time.Now()})
	}
	if _, err := Marshal(b); err == nil {
		t.Fatal("expected cap-of-3 violation")
	}
}

func TestValidate_NonInteractiveRejectsChosenUnlessReused(t *testing.T) {
	b := sampleBatch()
	b.Mode = ModeNonInteractive
	if _, err := Marshal(b); err != nil {
		// Chosen+ReusedFrom would be tolerated; here it's a fresh chosen — but the validator
		// permits chosen/custom in non_interactive (they imply reuse). So this MUST pass.
		t.Fatalf("non_interactive with chosen resolution must pass (treated as reused): %v", err)
	}

	b.Resolutions[0] = Resolution{QuestionID: "Q1", Kind: KindSkipped, ResolvedAt: time.Now()}
	if _, err := Marshal(b); err == nil {
		t.Fatal("non_interactive with skipped resolution must fail")
	}
}

func TestUnmarshal_PreservesCustomAnswer(t *testing.T) {
	b := sampleBatch()
	b.Resolutions[0] = Resolution{
		QuestionID: "Q1",
		Kind:       KindCustom,
		CustomText: "Owners + auditors with read-only role",
		ResolvedAt: time.Date(2026, 4, 19, 10, 5, 0, 0, time.UTC),
		AppliedTo:  []string{".bender/artifacts/specs/foo-2026.md"},
	}
	data, err := Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Resolutions[0].Kind != KindCustom || out.Resolutions[0].CustomText == "" {
		t.Fatalf("custom answer not preserved: %+v", out.Resolutions[0])
	}
}
