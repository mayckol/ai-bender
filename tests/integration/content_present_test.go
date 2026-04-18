package integration_test

import (
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/mayckol/ai-bender/internal/agent"
	embedded "github.com/mayckol/ai-bender/internal/embed"
	"github.com/mayckol/ai-bender/internal/group"
	"github.com/mayckol/ai-bender/internal/skill"
)

// TestSlashCommands_PresentAndParse: T050. Accepts either `SKILL.md` or
// `SKILL.md.tmpl` (feature 003-init-optional-skills introduced templated
// skills; `plan` carries a conditional mistakeinator block).
func TestSlashCommands_PresentAndParse(t *testing.T) {
	want := []string{"cry", "plan", "tdd", "ghu", "implement", "bender-doctor", "bender-bootstrap"}
	for _, name := range want {
		t.Run(name, func(t *testing.T) {
			var (
				data []byte
				err  error
			)
			for _, leaf := range []string{"SKILL.md", "SKILL.md.tmpl"} {
				data, err = fs.ReadFile(embedded.FS(), path.Join("claude/skills", name, leaf))
				if err == nil {
					break
				}
			}
			if err != nil {
				t.Fatalf("missing slash-command skill %q (neither SKILL.md nor SKILL.md.tmpl): %v", name, err)
			}
			// Strip a very minimal set of template directives so
			// ParseFrontmatter sees a clean document; the loader does the
			// same at LoadCatalog time with a stronger render. Frontmatter
			// lives at the top of every file, outside any conditional.
			s, err := skill.ParseFrontmatter(data)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			if s.Name != name {
				t.Fatalf("frontmatter name mismatch: got %q want %q", s.Name, name)
			}
			if !strings.Contains(s.Body, "$ARGUMENTS") && name != "bender-doctor" && name != "bender-bootstrap" {
				t.Errorf("%s: expected an $ARGUMENTS reference in the body so Claude can substitute user input", name)
			}
		})
	}
}

// TestAllSkills_Parse: T051. Every embedded skill must parse cleanly.
func TestAllSkills_Parse(t *testing.T) {
	cat, warnings, err := skill.LoadCatalog(embedded.FS(), nil)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("warnings (informational):\n  %s", strings.Join(warnings, "\n  "))
	}
	// Lean catalog: 7 slash commands + 20 worker skills (2 per agent × 10 agents).
	const wantMin, wantMax = 25, 31
	if cat.Len() < wantMin || cat.Len() > wantMax {
		t.Fatalf("expected default skill count in [%d, %d], got %d", wantMin, wantMax, cat.Len())
	}
	t.Logf("loaded %d default skills", cat.Len())
}

// TestAllAgents_Parse: T052. Every embedded agent must parse cleanly.
func TestAllAgents_Parse(t *testing.T) {
	reg, warnings, err := agent.LoadRegistry(embedded.FS(), nil)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("warnings:\n  %s", strings.Join(warnings, "\n  "))
	}
	if reg.Len() != 11 {
		var names []string
		for _, a := range reg.All() {
			names = append(names, a.Name)
		}
		t.Fatalf("expected exactly 11 default agents, got %d: %v", reg.Len(), names)
	}
	want := map[string]bool{
		"crafter": true, "tester": true, "reviewer": true, "linter": true,
		"architect": true, "scribe": true, "scout": true, "sentinel": true,
		"benchmarker": true, "surgeon": true, "mistakeinator": true,
	}
	for _, a := range reg.All() {
		if !want[a.Name] {
			t.Errorf("unexpected agent: %q", a.Name)
		}
		delete(want, a.Name)
	}
	for missing := range want {
		t.Errorf("missing default agent: %q", missing)
	}
}

// TestDefaultGroups_Parse: ensure groups.yaml is valid and contains the canonical entries.
func TestDefaultGroups_Parse(t *testing.T) {
	groups, err := group.LoadFromFS(embedded.FS(), "bender/groups.yaml")
	if err != nil {
		t.Fatalf("LoadFromFS: %v", err)
	}
	for _, want := range []string{"pre-implementation-checks", "security-sweep"} {
		if _, ok := groups[want]; !ok {
			t.Errorf("missing default group %q", want)
		}
	}
}
