package clarification

import "path/filepath"

// ArtifactDir is the per-plan-set directory where clarifications artifacts live.
// Co-located with the rest of the plan set so /plan confirm's existing glob
// already covers them.
const ArtifactDir = ".bender/artifacts/plan"

// ArtifactPath returns the canonical clarifications artifact path for the
// given shared plan-set timestamp. Empty timestamp returns empty path so the
// caller can detect a misuse rather than silently producing
// `.bender/artifacts/plan/clarifications-.md`.
func ArtifactPath(timestamp string) string {
	if timestamp == "" {
		return ""
	}
	return filepath.Join(ArtifactDir, "clarifications-"+timestamp+".md")
}
