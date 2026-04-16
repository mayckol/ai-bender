package discovery

// Run walks root once and dispatches every detector against the resulting probe. Items the binary
// cannot decide on its own (purpose, conventions, glossary) are reported in Result.Pending so the
// constitution renderer can mark them `pending: true` for Claude to fill via the slash commands.
func Run(root string) (Result, error) {
	probe, err := Walk(root)
	if err != nil {
		return Result{}, err
	}
	r := Result{
		Stack:        DetectStack(probe),
		Structure:    DetectStructure(probe),
		Tests:        DetectTests(probe),
		Lint:         DetectLint(probe),
		Build:        DetectBuild(probe),
		CICD:         DetectCICD(probe),
		Dependencies: DetectDependencies(probe),
	}
	r.Pending = derivePending(r)
	return r, nil
}

func derivePending(r Result) []string {
	var pending []string
	pending = append(pending, "purpose: derived from README/manifests by /cry or /bender-bootstrap")
	pending = append(pending, "conventions: naming/error/DI/architecture pattern (run /bender-bootstrap)")
	pending = append(pending, "glossary: recurring domain terms (run /bender-bootstrap)")
	if r.Stack.IsZero() {
		pending = append(pending, "stack: no manifest detected; add one or document manually")
	}
	if r.Structure.IsZero() {
		pending = append(pending, "structure: no recognisable folder layout")
	}
	if r.Tests.IsZero() {
		pending = append(pending, "tests: no test framework detected")
	}
	if r.Build.IsZero() {
		pending = append(pending, "build: no build tool detected")
	}
	if r.CICD.IsZero() {
		pending = append(pending, "cicd: no CI provider detected")
	}
	if len(r.Dependencies) == 0 {
		pending = append(pending, "dependencies: no dependency manifest parsed")
	}
	return pending
}
