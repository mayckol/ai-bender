package pipeline

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EvaluateVariables resolves every declared variable into a concrete value.
// The result map is what a `when` expression evaluates against at walk time.
//
// Kinds:
//
//   - glob_nonempty_with_status: runs the glob under projectRoot; true iff at
//     least one file matches AND every matched file's YAML frontmatter has a
//     `status` key whose value equals RequireStatus.
//   - plan_flag: reads the latest approved plan under
//     `.bender/artifacts/plan/plan-*.md`; returns the boolean value of the
//     named frontmatter key. Missing plan or missing key → false.
//   - literal: returns the declared Value verbatim.
//
// Unknown kinds return an error (the validator should already have rejected them).
func EvaluateVariables(p *Pipeline, projectRoot string) (map[string]any, error) {
	out := make(map[string]any, len(p.Variables))
	// Deterministic iteration so that logs & events are reproducible.
	names := make([]string, 0, len(p.Variables))
	for name := range p.Variables {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := p.Variables[name]
		v, err := evaluateVariable(def, projectRoot)
		if err != nil {
			return nil, fmt.Errorf("variable %q: %w", name, err)
		}
		out[name] = v
	}
	return out, nil
}

func evaluateVariable(def VariableDef, projectRoot string) (any, error) {
	switch def.Kind {
	case VarLiteral:
		return def.Value, nil
	case VarGlobApproved:
		return evalGlobApproved(projectRoot, def.Pattern, def.RequireStatus)
	case VarPlanFlag:
		return evalPlanFlag(projectRoot, def.Flag)
	}
	return false, fmt.Errorf("unknown kind %q", def.Kind)
}

// evalGlobApproved returns true iff the glob pattern (relative to projectRoot)
// matches at least one file and every matched file's frontmatter has
// `status: <requireStatus>`. Uses path/filepath pattern matching.
func evalGlobApproved(projectRoot, pattern, requireStatus string) (bool, error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	full := filepath.Join(projectRoot, filepath.FromSlash(pattern))
	matches, err := globRecursive(projectRoot, pattern)
	if err != nil {
		return false, fmt.Errorf("glob %s: %w", full, err)
	}
	if len(matches) == 0 {
		return false, nil
	}
	for _, m := range matches {
		status, err := readFrontmatterKey(m, "status")
		if err != nil {
			return false, nil // unreadable frontmatter counts as "not approved".
		}
		if status != requireStatus {
			return false, nil
		}
	}
	return true, nil
}

// globRecursive supports the `**` wildcard by walking under the first static
// segment (everything before the first wildcard). The `**` segment matches
// zero or more path components; other wildcards (`*`, `?`) delegate to
// path/filepath.Match on each component.
func globRecursive(projectRoot, pattern string) ([]string, error) {
	parts := strings.Split(filepath.ToSlash(pattern), "/")
	staticEnd := len(parts)
	for i, p := range parts {
		if strings.ContainsAny(p, "*?[") {
			staticEnd = i
			break
		}
	}
	base := filepath.Join(append([]string{projectRoot}, parts[:staticEnd]...)...)
	info, err := os.Stat(base)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		// Static path pointed straight at a file; match rest as literal.
		if matchPattern(parts[staticEnd:], nil) {
			return []string{base}, nil
		}
		return nil, nil
	}
	var hits []string
	err = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(projectRoot, p)
		relParts := strings.Split(filepath.ToSlash(rel), "/")
		if matchPattern(parts, relParts) {
			hits = append(hits, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hits, nil
}

func matchPattern(pattern, target []string) bool {
	if len(pattern) == 0 {
		return len(target) == 0
	}
	head := pattern[0]
	if head == "**" {
		if matchPattern(pattern[1:], target) {
			return true
		}
		for i := range target {
			if matchPattern(pattern[1:], target[i+1:]) {
				return true
			}
		}
		return false
	}
	if len(target) == 0 {
		return false
	}
	ok, _ := filepath.Match(head, target[0])
	if !ok {
		return false
	}
	return matchPattern(pattern[1:], target[1:])
}

func evalPlanFlag(projectRoot, flag string) (bool, error) {
	planDir := filepath.Join(projectRoot, ".bender", "artifacts", "plan")
	entries, err := os.ReadDir(planDir)
	if err != nil {
		return false, nil
	}
	// Take the most recent plan-*.md (lexical sort — timestamps are ISO-ish).
	var latest string
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "plan-") || !strings.HasSuffix(n, ".md") {
			continue
		}
		if n > latest {
			latest = n
		}
	}
	if latest == "" {
		return false, nil
	}
	val, err := readFrontmatterKey(filepath.Join(planDir, latest), flag)
	if err != nil || val == "" {
		return false, nil
	}
	return val == "true", nil
}

// readFrontmatterKey parses a YAML frontmatter block delimited by `---` and
// returns the string value for the given top-level key. Only scalars are
// returned (no maps/sequences). Empty string on miss.
func readFrontmatterKey(path, key string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inside := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if !inside {
				inside = true
				continue
			}
			break
		}
		if !inside {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == key {
			v = strings.TrimSpace(v)
			v = strings.Trim(v, "\"'")
			return v, nil
		}
	}
	return "", scanner.Err()
}
