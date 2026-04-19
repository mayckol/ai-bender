package clarification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
)

// Prompter abstracts the interactive picker so tests can inject a fake without
// running a real TTY. The default production Prompter (HuhPrompter) wraps
// huh.NewSelect.
type Prompter interface {
	Ask(ctx context.Context, q Question) (Resolution, error)
}

// CustomLabel and SkipLabel are the synthetic option values appended below the
// suggested A–D answers. They are not stored in Question.Options because they
// are part of the renderer contract, not the Question itself.
const (
	customLabel = "__custom__"
	skipLabel   = "__skip__"
)

// HuhPrompter is the production Prompter using huh forms.
type HuhPrompter struct{}

// Ask renders one huh.NewSelect for the question and returns the chosen
// Resolution. If the practitioner picks Custom, a follow-up huh.NewInput is
// shown to capture the free-form text.
func (HuhPrompter) Ask(ctx context.Context, q Question) (Resolution, error) {
	if err := ctx.Err(); err != nil {
		return Resolution{QuestionID: q.ID, Kind: KindPendingNonInteractive, ResolvedAt: time.Now().UTC()}, err
	}

	options := make([]huh.Option[string], 0, len(q.Options)+2)
	for _, o := range q.Options {
		options = append(options, huh.NewOption(fmt.Sprintf("%s. %s — %s", o.Label, o.Text, o.Implication), o.Label))
	}
	options = append(options,
		huh.NewOption("Custom — type your own answer", customLabel),
		huh.NewOption("Skip — leave [NEEDS CLARIFICATION] in the spec", skipLabel),
	)

	var picked string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title(fmt.Sprintf("%s — %s — Priority %d", q.ID, q.Category, q.Priority)).
				Description(fmt.Sprintf("%s\n\nContext:\n%s", q.Prompt, q.SourceExcerpt)),
			huh.NewSelect[string]().Options(options...).Value(&picked),
		),
	)
	if err := form.RunWithContext(ctx); err != nil {
		return Resolution{QuestionID: q.ID, Kind: KindPendingNonInteractive, ResolvedAt: time.Now().UTC()}, err
	}

	switch picked {
	case skipLabel:
		return Resolution{QuestionID: q.ID, Kind: KindSkipped, ResolvedAt: time.Now().UTC()}, nil
	case customLabel:
		var custom string
		input := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Custom answer").Value(&custom),
		))
		if err := input.RunWithContext(ctx); err != nil {
			return Resolution{QuestionID: q.ID, Kind: KindPendingNonInteractive, ResolvedAt: time.Now().UTC()}, err
		}
		if custom == "" {
			return Resolution{QuestionID: q.ID, Kind: KindSkipped, ResolvedAt: time.Now().UTC()}, nil
		}
		return Resolution{QuestionID: q.ID, Kind: KindCustom, CustomText: custom, ResolvedAt: time.Now().UTC()}, nil
	default:
		if picked == "" {
			return Resolution{QuestionID: q.ID, Kind: KindSkipped, ResolvedAt: time.Now().UTC()}, errors.New("clarification: no selection captured")
		}
		return Resolution{QuestionID: q.ID, Kind: KindChosen, ChosenLabel: picked, ResolvedAt: time.Now().UTC()}, nil
	}
}

// RunInteractive walks every Question in the Batch through the supplied
// Prompter and returns the Batch with its Resolutions populated. Resolutions
// already present (e.g. from reuse) are kept as-is and the Question is skipped.
func RunInteractive(ctx context.Context, b Batch, p Prompter) (Batch, error) {
	if p == nil {
		return b, errors.New("clarification: nil Prompter")
	}
	existing := map[string]bool{}
	for _, r := range b.Resolutions {
		if r.Kind == KindChosen || r.Kind == KindCustom {
			existing[r.QuestionID] = true
		}
	}
	for _, q := range b.Questions {
		if existing[q.ID] {
			continue
		}
		if b.FindResolution(q.ID) != nil {
			continue
		}
		r, err := p.Ask(ctx, q)
		if err != nil {
			b.Resolutions = append(b.Resolutions, Resolution{
				QuestionID: q.ID,
				Kind:       KindPendingNonInteractive,
				ResolvedAt: time.Now().UTC(),
			})
			return b, err
		}
		b.Resolutions = append(b.Resolutions, r)
	}
	return b, nil
}
