package clarification

import (
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/event"
)

func batchWithCounts(answered, pending, skipped, deferred int) Batch {
	now := time.Now().UTC()
	b := Batch{
		Timestamp:   "2026-04-19T10-00-00-000",
		FromCapture: "cap.md",
		FromSpec:    "spec.md",
		Mode:        ModeInteractive,
		CreatedAt:   now,
		Status:      "draft",
	}
	id := 0
	emit := func(kind ResolutionKind) {
		id++
		qid := "Q" + string(rune('0'+id))
		b.Questions = append(b.Questions, Question{
			ID: qid, TargetSection: "x." + qid, Category: CategoryScope, Priority: 1,
			Prompt: "p", SourceExcerpt: "e",
			Options: []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
		})
		r := Resolution{QuestionID: qid, Kind: kind, ResolvedAt: now}
		if kind == KindChosen {
			r.ChosenLabel = "A"
		}
		b.Resolutions = append(b.Resolutions, r)
	}
	for i := 0; i < answered; i++ {
		emit(KindChosen)
	}
	for i := 0; i < pending; i++ {
		emit(KindPendingNonInteractive)
	}
	for i := 0; i < skipped; i++ {
		emit(KindSkipped)
	}
	for i := 0; i < deferred; i++ {
		emit(KindDeferredByCap)
	}
	return b
}

func TestBuildEvent_RequestedHasZeroCountersInitially(t *testing.T) {
	b := batchWithCounts(0, 0, 0, 0)
	b.Questions = append(b.Questions, Question{
		ID: "Q1", TargetSection: "x.Q1", Category: CategoryScope, Priority: 1,
		Prompt: "p", SourceExcerpt: "e",
		Options: []Option{{"A", "a", "i"}, {"B", "b", "i"}, {"C", "c", "i"}},
	})
	b.Resolutions = append(b.Resolutions, Resolution{QuestionID: "Q1", Kind: KindPendingNonInteractive, ResolvedAt: time.Now().UTC()})

	ev, err := BuildEvent(event.TypeClarificationsRequested, b, "sess", "path.md")
	if err != nil {
		t.Fatalf("BuildEvent: %v", err)
	}
	if ev.Type != event.TypeClarificationsRequested {
		t.Fatalf("type: got %q", ev.Type)
	}
	if ev.Actor.Kind != event.ActorStage || ev.Actor.Name != "plan" {
		t.Fatalf("actor: %+v", ev.Actor)
	}
	if ev.Payload["question_count"].(int) != 1 {
		t.Fatalf("question_count: %+v", ev.Payload)
	}
}

func TestBuildEvent_ResolvedRejectsPending(t *testing.T) {
	b := batchWithCounts(1, 1, 0, 0)
	if _, err := BuildEvent(event.TypeClarificationsResolved, b, "sess", "p"); err == nil {
		t.Fatal("expected resolved-with-pending rejection")
	}
}

func TestBuildEvent_PendingRequiresPending(t *testing.T) {
	b := batchWithCounts(1, 0, 0, 0)
	if _, err := BuildEvent(event.TypeClarificationsPending, b, "sess", "p"); err == nil {
		t.Fatal("expected pending-without-pending rejection")
	}
}

func TestBuildEvent_CountersMustSum(t *testing.T) {
	b := batchWithCounts(1, 0, 0, 0)
	b.Questions = append(b.Questions, Question{ID: "Q9"})
	if _, err := BuildEvent(event.TypeClarificationsResolved, b, "sess", "p"); err == nil {
		t.Fatal("expected counter-mismatch rejection")
	}
}

func TestBuildEvent_ValidateAgainstEnvelope(t *testing.T) {
	b := batchWithCounts(2, 0, 1, 0)
	ev, err := BuildEvent(event.TypeClarificationsResolved, b, "sess", "path.md")
	if err != nil {
		t.Fatalf("BuildEvent: %v", err)
	}
	if err := ev.Validate(); err != nil {
		t.Fatalf("envelope validate: %v", err)
	}
	if err := ev.ValidatePayload(); err != nil {
		t.Fatalf("payload validate: %v", err)
	}
}
