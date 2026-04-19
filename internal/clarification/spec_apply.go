package clarification

import (
	"fmt"
	"regexp"
	"strings"
)

// markerPattern matches a single [NEEDS CLARIFICATION: …] marker. The capture
// group is the inner reason text — discarded after rewrite.
var markerPattern = regexp.MustCompile(`\[NEEDS CLARIFICATION:[^\]]*\]`)

// Apply rewrites [NEEDS CLARIFICATION: …] markers in spec for every
// Resolution that ended in chosen or custom. Returns the rewritten spec bytes
// and the list of `applied to` paths the caller should record (always one
// entry — the spec path itself — when at least one rewrite landed).
//
// The match key is Question.TargetSection. The function expects the caller to
// pass questions+resolutions paired by index (Batch invariant). Markers whose
// target line cannot be located are left untouched and reported via the
// returned skipped slice so the caller can decide to keep the marker as a
// `[NEEDS CLARIFICATION]` reminder.
func Apply(spec []byte, specPath string, b Batch) (rewritten []byte, applied []string, skipped []string, err error) {
	out := string(spec)
	rewritten = nil
	rewroteAny := false

	for _, q := range b.Questions {
		r := b.FindResolution(q.ID)
		if r == nil {
			continue
		}
		switch r.Kind {
		case KindChosen, KindCustom:
		default:
			continue
		}

		replacement, repErr := resolutionText(q, r)
		if repErr != nil {
			err = repErr
			return
		}

		newOut, ok := rewriteAt(out, q.TargetSection, replacement)
		if !ok {
			skipped = append(skipped, q.ID)
			continue
		}
		out = newOut
		rewroteAny = true
	}

	rewritten = []byte(out)
	if rewroteAny {
		applied = []string{specPath}
	}
	return
}

func resolutionText(q Question, r *Resolution) (string, error) {
	if r.Kind == KindCustom {
		if r.CustomText == "" {
			return "", fmt.Errorf("clarification: question %s custom resolution missing text", q.ID)
		}
		return r.CustomText, nil
	}
	for _, o := range q.Options {
		if o.Label == r.ChosenLabel {
			return o.Text, nil
		}
	}
	return "", fmt.Errorf("clarification: question %s chosen label %q not in options", q.ID, r.ChosenLabel)
}

// rewriteAt locates the line whose target_section identifier appears (e.g.,
// `FR-007`, `actor`, or any literal substring callers use as TargetSection)
// and replaces the first marker on that line with replacement. Returns the
// updated text and a found flag.
//
// The TargetSection format is conventional, not enforced; this function uses
// a simple substring match on the dotted path's last segment so callers can
// pass either `requirements.functional.FR-007` or `FR-007` and still match.
func rewriteAt(spec, targetSection, replacement string) (string, bool) {
	key := lastSegment(targetSection)
	if key == "" {
		return spec, false
	}

	lines := strings.Split(spec, "\n")
	for i, line := range lines {
		if !strings.Contains(line, key) {
			continue
		}
		if !markerPattern.MatchString(line) {
			continue
		}
		lines[i] = markerPattern.ReplaceAllString(line, replacement)
		return strings.Join(lines, "\n"), true
	}
	return spec, false
}

func lastSegment(targetSection string) string {
	if idx := strings.LastIndex(targetSection, "."); idx >= 0 {
		return targetSection[idx+1:]
	}
	return targetSection
}
