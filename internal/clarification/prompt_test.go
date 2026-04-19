package clarification

import (
	"context"
	"errors"
	"testing"
	"time"
)

type scriptedPrompter struct {
	answers map[string]Resolution
	err     error
}

func (s *scriptedPrompter) Ask(_ context.Context, q Question) (Resolution, error) {
	if s.err != nil {
		return Resolution{}, s.err
	}
	if r, ok := s.answers[q.ID]; ok {
		r.QuestionID = q.ID
		r.ResolvedAt = time.Now().UTC()
		return r, nil
	}
	return Resolution{QuestionID: q.ID, Kind: KindSkipped, ResolvedAt: time.Now().UTC()}, nil
}

func mkBatchN(n int) Batch {
	b := Batch{Mode: ModeInteractive, Status: "draft"}
	for i := 1; i <= n; i++ {
		id := "Q" + string(rune('0'+i))
		b.Questions = append(b.Questions, Question{
			ID: id, TargetSection: "x." + id, Category: CategoryScope, Priority: 1,
			Prompt: "p", SourceExcerpt: "e",
			Options: []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
		})
	}
	return b
}

func TestRunInteractive_AppliesChosen(t *testing.T) {
	b := mkBatchN(2)
	p := &scriptedPrompter{answers: map[string]Resolution{
		"Q1": {Kind: KindChosen, ChosenLabel: "B"},
		"Q2": {Kind: KindChosen, ChosenLabel: "C"},
	}}
	out, err := RunInteractive(context.Background(), b, p)
	if err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}
	if len(out.Resolutions) != 2 {
		t.Fatalf("resolutions: %+v", out.Resolutions)
	}
	if out.Resolutions[0].Kind != KindChosen || out.Resolutions[0].ChosenLabel != "B" {
		t.Fatalf("Q1 resolution: %+v", out.Resolutions[0])
	}
	if out.Resolutions[1].ChosenLabel != "C" {
		t.Fatalf("Q2 chosen label: %+v", out.Resolutions[1])
	}
}

func TestRunInteractive_SkipPath(t *testing.T) {
	b := mkBatchN(1)
	p := &scriptedPrompter{} // unanswered → fake returns Skipped
	out, err := RunInteractive(context.Background(), b, p)
	if err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}
	if out.Resolutions[0].Kind != KindSkipped {
		t.Fatalf("expected Skipped, got %+v", out.Resolutions[0])
	}
}

func TestRunInteractive_PreservesExistingResolutions(t *testing.T) {
	b := mkBatchN(2)
	b.Resolutions = []Resolution{
		{QuestionID: "Q1", Kind: KindChosen, ChosenLabel: "A", ResolvedAt: time.Now()},
	}
	p := &scriptedPrompter{answers: map[string]Resolution{
		"Q2": {Kind: KindChosen, ChosenLabel: "C"},
	}}
	out, err := RunInteractive(context.Background(), b, p)
	if err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}
	if len(out.Resolutions) != 2 {
		t.Fatalf("expected 2 resolutions, got %d", len(out.Resolutions))
	}
	if out.Resolutions[0].QuestionID != "Q1" || out.Resolutions[0].ChosenLabel != "A" {
		t.Fatalf("existing Q1 resolution lost: %+v", out.Resolutions[0])
	}
}

func TestRunInteractive_PrompterErrorMarksPending(t *testing.T) {
	b := mkBatchN(1)
	p := &scriptedPrompter{err: errors.New("boom")}
	_, err := RunInteractive(context.Background(), b, p)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunInteractive_NilPrompterReturnsError(t *testing.T) {
	if _, err := RunInteractive(context.Background(), mkBatchN(1), nil); err == nil {
		t.Fatal("expected nil-prompter error")
	}
}
