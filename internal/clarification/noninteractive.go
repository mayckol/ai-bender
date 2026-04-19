package clarification

import (
	"context"
	"time"
)

// RunNonInteractive is the automation-friendly counterpart to RunInteractive.
// No prompt is shown. Every Question without an existing chosen|custom
// resolution receives a `pending_noninteractive` Resolution. The returned
// Batch carries Mode=ModeNonInteractive and the supplied Strict flag — the
// strict-exit decision belongs to the caller (Run) so this helper stays a
// pure transformation.
func RunNonInteractive(_ context.Context, b Batch, strict bool) (Batch, error) {
	b.Mode = ModeNonInteractive
	b.Strict = strict
	have := map[string]bool{}
	for _, r := range b.Resolutions {
		have[r.QuestionID] = true
	}
	now := time.Now().UTC()
	for _, q := range b.Questions {
		if have[q.ID] {
			continue
		}
		b.Resolutions = append(b.Resolutions, Resolution{
			QuestionID: q.ID,
			Kind:       KindPendingNonInteractive,
			ResolvedAt: now,
		})
	}
	return b, nil
}
