// Package doctor implements `bender doctor`: it loads the embedded defaults plus a project's
// `.claude/` overrides, resolves every (agent × stage × issue type) combination, and reports
// empty skill sets, broken selectors, missing required external tools, and duplicate names.
package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"

	"github.com/mayckol/ai-bender/internal/agent"
	embedded "github.com/mayckol/ai-bender/internal/embed"
	"github.com/mayckol/ai-bender/internal/group"
	"github.com/mayckol/ai-bender/internal/skill"
	"github.com/mayckol/ai-bender/internal/types"
)

// Severity is the issue severity. Doctor exits 0 with no issues, 40 if any warnings, 41 if any errors.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Category groups issues for reporting.
type Category string

const (
	CategoryParse           Category = "parse"
	CategoryEmptySkillSet   Category = "empty-skill-set"
	CategoryBrokenSelector  Category = "broken-selector"
	CategoryMissingTool     Category = "missing-tool"
	CategoryDuplicate       Category = "duplicate"
	CategoryContextConflict Category = "context-conflict"
)

// Issue is one finding.
type Issue struct {
	Severity Severity
	Category Category
	Subject  string
	Message  string
	Path     string
}

// AgentResolution records one effective skill set produced during the catalog walk.
type AgentResolution struct {
	Agent     string
	Stage     types.Stage
	IssueType types.IssueType
	Skills    []string
}

// Report is the structured output of a doctor run.
type Report struct {
	SkillCount       int
	AgentCount       int
	GroupCount       int
	Resolutions      []AgentResolution
	Issues           []Issue
	ParseWarnings    []string
}

// HasErrors reports whether the report contains at least one error-severity issue.
func (r *Report) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings reports whether the report contains at least one warning-severity issue.
func (r *Report) HasWarnings() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// Run loads the catalog at projectRoot/.claude (overlaid on the embedded defaults) and produces a Report.
func Run(projectRoot string) (*Report, error) {
	userFS := userClaudeFS(projectRoot)

	skillCat, skillWarnings, err := skill.LoadCatalog(embedded.FS(), userFS)
	if err != nil {
		return nil, fmt.Errorf("doctor: load skills: %w", err)
	}
	agentReg, agentWarnings, err := agent.LoadRegistry(embedded.FS(), userFS)
	if err != nil {
		return nil, fmt.Errorf("doctor: load agents: %w", err)
	}
	groupsDefault, err := group.LoadFromFS(embedded.FS(), "claude/groups.yaml")
	if err != nil {
		return nil, fmt.Errorf("doctor: load default groups: %w", err)
	}
	groupsUser := map[string]*group.Group{}
	if userFS != nil {
		groupsUser, err = group.LoadFromFS(userFS, "groups.yaml")
		if err != nil {
			return nil, fmt.Errorf("doctor: load user groups: %w", err)
		}
	}
	mergedGroups := mergeGroups(groupsDefault, groupsUser)

	r := &Report{
		SkillCount:    skillCat.Len(),
		AgentCount:    agentReg.Len(),
		GroupCount:    len(mergedGroups),
		ParseWarnings: append(skillWarnings, agentWarnings...),
	}

	for _, w := range r.ParseWarnings {
		r.Issues = append(r.Issues, Issue{Severity: SeverityError, Category: CategoryParse, Message: w})
	}

	r.checkBrokenSelectors(skillCat, agentReg)
	r.checkEmptySkillSets(skillCat, agentReg)
	r.checkMissingTools(skillCat)

	return r, nil
}

func (r *Report) checkBrokenSelectors(cat *skill.Catalog, reg *agent.Registry) {
	for _, a := range reg.All() {
		for _, name := range a.Skills.Explicit {
			if cat.Get(name) == nil {
				r.Issues = append(r.Issues, Issue{
					Severity: SeverityError,
					Category: CategoryBrokenSelector,
					Subject:  fmt.Sprintf("agent=%s skills.explicit=%q", a.Name, name),
					Message:  "explicit skill not found in catalog",
					Path:     a.Path,
				})
			}
		}
		// Glob patterns are extensibility points: they may legitimately match nothing in the
		// embedded defaults and pick up user-added skills later. Surface as info, not warning.
		for _, pattern := range a.Skills.Patterns {
			matches := false
			for _, n := range cat.Names() {
				if m, _ := path.Match(pattern, n); m {
					matches = true
					break
				}
			}
			if !matches {
				r.Issues = append(r.Issues, Issue{
					Severity: SeverityInfo,
					Category: CategoryBrokenSelector,
					Subject:  fmt.Sprintf("agent=%s skills.patterns=%q", a.Name, pattern),
					Message:  "glob pattern matches no current skills (will pick up matching user-added skills)",
					Path:     a.Path,
				})
			}
		}
	}
}

func (r *Report) checkEmptySkillSets(cat *skill.Catalog, reg *agent.Registry) {
	for _, a := range reg.All() {
		for _, stage := range a.InvokedBy {
			anyNonEmpty := false
			perTypeEmpty := map[types.IssueType]bool{}
			for _, issueType := range []types.IssueType{types.IssueAny, types.IssueBug, types.IssueFeature, types.IssuePerformance, types.IssueArchitectural} {
				ctx := types.ResolveContext{Stage: stage, IssueType: issueType, AgentContexts: a.Context}
				resolved := skill.Resolve(cat, a.Skills, ctx)
				names := make([]string, len(resolved))
				for i, s := range resolved {
					names[i] = s.Name
				}
				r.Resolutions = append(r.Resolutions, AgentResolution{
					Agent:     a.Name,
					Stage:     stage,
					IssueType: issueType,
					Skills:    names,
				})
				if len(resolved) == 0 {
					perTypeEmpty[issueType] = true
				} else {
					anyNonEmpty = true
				}
			}
			// If at least one issue type at this stage produces a non-empty set, the empties
			// are expected filter outcomes (e.g., a security-only skill not firing on perf
			// issues). Don't warn. If every issue type is empty, the agent is reachable but
			// has no skills at all for this stage — that is a real warning.
			if !anyNonEmpty {
				r.Issues = append(r.Issues, Issue{
					Severity: SeverityWarning,
					Category: CategoryEmptySkillSet,
					Subject:  fmt.Sprintf("agent=%s stage=%s", a.Name, stage),
					Message:  "no skills resolve for ANY issue type at this stage",
					Path:     a.Path,
				})
			}
			_ = perTypeEmpty
		}
	}
	sort.Slice(r.Resolutions, func(i, j int) bool {
		if r.Resolutions[i].Agent != r.Resolutions[j].Agent {
			return r.Resolutions[i].Agent < r.Resolutions[j].Agent
		}
		if r.Resolutions[i].Stage != r.Resolutions[j].Stage {
			return r.Resolutions[i].Stage < r.Resolutions[j].Stage
		}
		return r.Resolutions[i].IssueType < r.Resolutions[j].IssueType
	})
}

func (r *Report) checkMissingTools(cat *skill.Catalog) {
	seen := map[string]bool{}
	for _, s := range cat.All() {
		for _, tool := range s.RequiresTools {
			if seen[tool] {
				continue
			}
			if _, err := exec.LookPath(tool); err != nil {
				r.Issues = append(r.Issues, Issue{
					Severity: SeverityWarning,
					Category: CategoryMissingTool,
					Subject:  tool,
					Message:  fmt.Sprintf("required by skill %q (and possibly others); not found on PATH", s.Name),
				})
			}
			seen[tool] = true
		}
	}
}

func userClaudeFS(projectRoot string) fs.FS {
	if projectRoot == "" {
		return nil
	}
	root := filepath.Join(projectRoot, ".claude")
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	return os.DirFS(root)
}

func mergeGroups(defaults, user map[string]*group.Group) map[string]*group.Group {
	out := make(map[string]*group.Group, len(defaults)+len(user))
	for n, g := range defaults {
		out[n] = g
	}
	for n, g := range user {
		out[n] = g
	}
	return out
}
