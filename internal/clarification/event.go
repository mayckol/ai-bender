package clarification

import (
	"errors"
	"fmt"
	"time"

	"github.com/mayckol/ai-bender/internal/event"
)

// BuildEvent assembles a clarification lifecycle event for the given Batch.
// kind MUST be one of:
//   - event.TypeClarificationsRequested
//   - event.TypeClarificationsResolved
//   - event.TypeClarificationsPending
//
// The Batch counters MUST sum to len(Questions); BuildEvent rejects any other
// shape so the caller surfaces the bug instead of writing a malformed event.
func BuildEvent(kind event.Type, b Batch, sessionID, artifactPath string) (event.Event, error) {
	if sessionID == "" {
		return event.Event{}, errors.New("clarification: session_id required for event")
	}
	switch kind {
	case event.TypeClarificationsRequested,
		event.TypeClarificationsResolved,
		event.TypeClarificationsPending:
	default:
		return event.Event{}, fmt.Errorf("clarification: unsupported event type %q", kind)
	}

	resolved := b.ResolvedCount()
	pending := b.PendingCount()
	skipped := b.SkippedCount()
	deferred := b.DeferredCount()
	total := len(b.Questions)
	if resolved+pending+skipped+deferred != total {
		return event.Event{}, fmt.Errorf("clarification: counter sum (%d) != question count (%d)",
			resolved+pending+skipped+deferred, total)
	}

	if kind == event.TypeClarificationsResolved && pending > 0 {
		return event.Event{}, errors.New("clarification: clarifications_resolved cannot carry pending entries")
	}
	if kind == event.TypeClarificationsPending && pending == 0 {
		return event.Event{}, errors.New("clarification: clarifications_pending requires at least one pending entry")
	}

	payload := map[string]any{
		"artifact_path":   artifactPath,
		"question_count":  total,
		"resolved_count":  resolved,
		"pending_count":   pending,
		"skipped_count":   skipped,
		"deferred_count":  deferred,
		"reused_from":     b.ReusedFrom,
	}

	return event.Event{
		SchemaVersion: event.SchemaVersion,
		SessionID:     sessionID,
		Timestamp:     time.Now().UTC(),
		Actor:         event.Actor{Kind: event.ActorStage, Name: "plan"},
		Type:          kind,
		Payload:       payload,
	}, nil
}
