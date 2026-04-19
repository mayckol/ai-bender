package clarification

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func plantArtifact(t *testing.T, dir, name string, b Batch) {
	t.Helper()
	data, err := Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLookupReusable_PicksMostRecentMatchingCapture(t *testing.T) {
	dir := t.TempDir()
	older := sampleBatch()
	older.Timestamp = "2026-04-19T09-00-00-000"
	older.FromCapture = ".bender/artifacts/cry/foo-2026.md"
	plantArtifact(t, dir, "clarifications-2026-04-19T09-00-00-000.md", older)

	newer := sampleBatch()
	newer.Timestamp = "2026-04-19T11-00-00-000"
	newer.FromCapture = ".bender/artifacts/cry/foo-2026.md"
	newer.Resolutions[0].ChosenLabel = "B"
	plantArtifact(t, dir, "clarifications-2026-04-19T11-00-00-000.md", newer)

	other := sampleBatch()
	other.Timestamp = "2026-04-19T12-00-00-000"
	other.FromCapture = ".bender/artifacts/cry/bar-2026.md"
	plantArtifact(t, dir, "clarifications-2026-04-19T12-00-00-000.md", other)

	got, err := LookupReusable(dir, ".bender/artifacts/cry/foo-2026.md")
	if err != nil {
		t.Fatalf("LookupReusable: %v", err)
	}
	if got.ReusedFrom == "" || filepath.Base(got.ReusedFrom) != "clarifications-2026-04-19T11-00-00-000.md" {
		t.Fatalf("expected newest matching artifact, got %q", got.ReusedFrom)
	}
	if len(got.Resolutions) != 1 || got.Resolutions[0].ChosenLabel != "B" {
		t.Fatalf("expected newer resolution carry-through, got %+v", got.Resolutions)
	}
}

func TestLookupReusable_IgnoresNonChosenResolutions(t *testing.T) {
	dir := t.TempDir()
	b := sampleBatch()
	b.Resolutions[0] = Resolution{QuestionID: "Q1", Kind: KindSkipped, ResolvedAt: time.Now().UTC()}
	plantArtifact(t, dir, "clarifications-2026-04-19T09-00-00-000.md", b)

	got, err := LookupReusable(dir, b.FromCapture)
	if err != nil {
		t.Fatalf("LookupReusable: %v", err)
	}
	if len(got.Resolutions) != 0 {
		t.Fatalf("expected zero resolutions (skipped not reusable), got %+v", got.Resolutions)
	}
}

func TestLookupReusable_EmptyDirReturnsNoError(t *testing.T) {
	dir := t.TempDir()
	got, err := LookupReusable(dir, "any.md")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got.Resolutions) != 0 {
		t.Fatalf("expected empty batch, got %+v", got)
	}
}

func TestMergeReuse_CopiesByTargetSection(t *testing.T) {
	prior := sampleBatch()
	prior.ReusedFrom = "/path/to/prior.md"
	prior.Resolutions[0].ChosenLabel = "A"

	detected := Batch{
		FromCapture: prior.FromCapture,
		Mode:        ModeInteractive,
		Questions: []Question{
			{
				ID:            "X1", // different ID than prior, same TargetSection
				TargetSection: prior.Questions[0].TargetSection,
				Category:      CategoryActorsRoles,
				Priority:      1,
				Prompt:        "rephrased",
				SourceExcerpt: "x",
				Options:       []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
			},
			{
				ID:            "X2",
				TargetSection: "totally.new",
				Category:      CategoryScope,
				Priority:      1,
				Prompt:        "p",
				SourceExcerpt: "x",
				Options:       []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
			},
		},
	}
	merged := MergeReuse(detected, prior)
	if merged.ReusedFrom != prior.ReusedFrom {
		t.Fatalf("ReusedFrom not propagated: %q", merged.ReusedFrom)
	}
	if len(merged.Resolutions) != 1 {
		t.Fatalf("expected 1 reused resolution, got %d", len(merged.Resolutions))
	}
	if merged.Resolutions[0].QuestionID != "X1" || merged.Resolutions[0].ChosenLabel != "A" {
		t.Fatalf("reused resolution wrong: %+v", merged.Resolutions[0])
	}
}

func TestMergeReuse_NoOpWhenPriorEmpty(t *testing.T) {
	d := sampleBatch()
	got := MergeReuse(d, Batch{})
	if len(got.Resolutions) != len(d.Resolutions) {
		t.Fatalf("merge with empty prior should be no-op")
	}
}
