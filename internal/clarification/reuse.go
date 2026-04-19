package clarification

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LookupReusable scans dir for prior `clarifications-*.md` artifacts whose
// frontmatter `from_capture` matches capturePath. Returns the most recent
// (lex-order on filename suffix) Batch with only chosen|custom resolutions
// retained — Skipped, Deferred, and Pending are intentionally re-prompted.
//
// Returns an empty Batch and a wrapped fs error if dir cannot be read; an
// empty Batch and nil if no matching artifact is found.
func LookupReusable(dir, capturePath string) (Batch, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Batch{}, nil
		}
		return Batch{}, err
	}
	var candidates []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "clarifications-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		candidates = append(candidates, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(candidates)))
	for _, name := range candidates {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		b, err := Unmarshal(data)
		if err != nil {
			continue
		}
		if b.FromCapture != capturePath {
			continue
		}
		return harvestReusable(b, filepath.Join(dir, name)), nil
	}
	return Batch{}, nil
}

func harvestReusable(b Batch, sourcePath string) Batch {
	out := Batch{
		ReusedFrom:  sourcePath,
		FromCapture: b.FromCapture,
	}
	keep := map[string]bool{}
	for _, r := range b.Resolutions {
		if r.Kind == KindChosen || r.Kind == KindCustom {
			out.Resolutions = append(out.Resolutions, r)
			keep[r.QuestionID] = true
		}
	}
	for _, q := range b.Questions {
		if keep[q.ID] {
			out.Questions = append(out.Questions, q)
		}
	}
	return out
}

// MergeReuse copies prior chosen/custom resolutions into detected when their
// TargetSection matches a detected Question. Returns the merged batch — the
// caller looks at b.Resolutions to discover which questions are still
// outstanding via freshQuestionIDs.
func MergeReuse(detected, prior Batch) Batch {
	if len(prior.Resolutions) == 0 {
		return detected
	}
	priorByTarget := map[string]Resolution{}
	priorQ := map[string]Question{}
	for _, q := range prior.Questions {
		priorQ[q.ID] = q
	}
	for _, r := range prior.Resolutions {
		q, ok := priorQ[r.QuestionID]
		if !ok {
			continue
		}
		priorByTarget[q.TargetSection] = r
	}

	out := detected
	out.ReusedFrom = prior.ReusedFrom
	for _, q := range detected.Questions {
		if r, ok := priorByTarget[q.TargetSection]; ok {
			if existing := out.FindResolution(q.ID); existing != nil {
				continue
			}
			r.QuestionID = q.ID
			out.Resolutions = append(out.Resolutions, r)
		}
	}
	return out
}

// ErrNoReuseTarget is returned by MergeReuse only if both arguments are
// invalid; reserved for future strict-mode callers.
var ErrNoReuseTarget = errors.New("clarification: no detected questions match prior resolutions")
