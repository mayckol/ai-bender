package clarification

import (
	"context"
	"testing"
	"time"
)

func TestRunNonInteractive_FillsAllAsPending(t *testing.T) {
	b := mkBatchN(3)
	out, err := RunNonInteractive(context.Background(), b, false)
	if err != nil {
		t.Fatalf("RunNonInteractive: %v", err)
	}
	if out.Mode != ModeNonInteractive {
		t.Fatalf("mode: %q", out.Mode)
	}
	if len(out.Resolutions) != 3 {
		t.Fatalf("resolutions: %d", len(out.Resolutions))
	}
	for _, r := range out.Resolutions {
		if r.Kind != KindPendingNonInteractive {
			t.Errorf("resolution %s kind: got %q want pending", r.QuestionID, r.Kind)
		}
	}
}

func TestRunNonInteractive_PreservesExistingResolutions(t *testing.T) {
	b := mkBatchN(2)
	b.Resolutions = []Resolution{{QuestionID: "Q1", Kind: KindChosen, ChosenLabel: "A", ResolvedAt: time.Now().UTC()}}
	out, err := RunNonInteractive(context.Background(), b, true)
	if err != nil {
		t.Fatalf("RunNonInteractive: %v", err)
	}
	if !out.Strict {
		t.Fatalf("expected strict=true")
	}
	if len(out.Resolutions) != 2 {
		t.Fatalf("resolutions: %d", len(out.Resolutions))
	}
	if out.Resolutions[0].Kind != KindChosen {
		t.Fatalf("Q1 should remain chosen, got %+v", out.Resolutions[0])
	}
	if out.Resolutions[1].Kind != KindPendingNonInteractive {
		t.Fatalf("Q2 should be pending, got %+v", out.Resolutions[1])
	}
}
