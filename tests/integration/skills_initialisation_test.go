package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mayckol/ai-bender/internal/workspace"
)

// The sentinel comment that MUST appear exactly once inside every write-heavy
// skill's SKILL.md and MUST NOT appear anywhere in a read-only skill's.
// Defined verbatim in specs/005-worktree-followups/contracts/skill-worktree-step.md.
const worktreeBlockSentinel = "END WORKTREE PROVISIONING BLOCK"
const worktreeBlockHeading = "### Worktree provisioning"

// scaffoldIntoTemp materialises the embedded defaults into a fresh tempdir and
// returns the root. Uses workspace.Scaffold directly so no CLI process is
// spawned — this keeps the test self-contained.
func scaffoldIntoTemp(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	_, err := workspace.Scaffold(workspace.ScaffoldOptions{ProjectRoot: root})
	if err != nil {
		t.Fatalf("workspace.Scaffold: %v", err)
	}
	return root
}

func readSkill(t *testing.T, root, rel string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

// US2 — /ghu skill carries the worktree provisioning block.
func TestGhuSkill_ContainsWorktreeProvisioningBlock(t *testing.T) {
	root := scaffoldIntoTemp(t)
	body := readSkill(t, root, ".claude/skills/ghu/SKILL.md")
	if !strings.Contains(body, worktreeBlockHeading) {
		t.Errorf("ghu SKILL.md missing heading %q", worktreeBlockHeading)
	}
	if !strings.Contains(body, worktreeBlockSentinel) {
		t.Errorf("ghu SKILL.md missing sentinel %q", worktreeBlockSentinel)
	}
	if !strings.Contains(body, "bender worktree create") {
		t.Errorf("ghu SKILL.md does not reference `bender worktree create`")
	}
	// Must not have the sentinel more than once — that would indicate a
	// duplicated paste.
	if strings.Count(body, worktreeBlockSentinel) != 1 {
		t.Errorf("ghu SKILL.md sentinel must appear exactly once; got %d", strings.Count(body, worktreeBlockSentinel))
	}
}

// US2 — /ghu skill's YAML frontmatter is unchanged after the edit.
func TestGhuSkill_RetainsExistingFrontmatter(t *testing.T) {
	root := scaffoldIntoTemp(t)
	body := readSkill(t, root, ".claude/skills/ghu/SKILL.md")
	for _, want := range []string{
		"name: ghu",
		"user-invocable: true",
		"context: fg",
		"stages: [ghu]",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ghu SKILL.md frontmatter drift: missing %q", want)
		}
	}
}

// US3 — /implement skill carries the same block.
func TestImplementSkill_ContainsWorktreeProvisioningBlock(t *testing.T) {
	root := scaffoldIntoTemp(t)
	body := readSkill(t, root, ".claude/skills/implement/SKILL.md")
	if !strings.Contains(body, worktreeBlockSentinel) {
		t.Errorf("implement SKILL.md missing sentinel %q", worktreeBlockSentinel)
	}
	if !strings.Contains(body, "bender worktree create") {
		t.Errorf("implement SKILL.md does not reference `bender worktree create`")
	}
}

// US4 — read-only skills MUST NOT carry the block.
func TestReadOnlySkills_OmitWorktreeProvisioningBlock(t *testing.T) {
	root := scaffoldIntoTemp(t)
	// Walk `.claude/skills/` and inspect every skill that is NOT /ghu or /implement.
	skillsRoot := filepath.Join(root, ".claude/skills")
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}
	writeHeavy := map[string]bool{"ghu": true, "implement": true}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if writeHeavy[e.Name()] {
			continue
		}
		// Some skills carry SKILL.md.tmpl instead of SKILL.md (feature 003 templating).
		for _, fname := range []string{"SKILL.md", "SKILL.md.tmpl"} {
			p := filepath.Join(skillsRoot, e.Name(), fname)
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if strings.Contains(string(raw), worktreeBlockSentinel) {
				t.Errorf("read-only skill %s must not contain the worktree block", e.Name())
			}
		}
	}
}
