package embed

import (
	"io/fs"
	"testing"
)

func TestFS_ContainsBenderConfig(t *testing.T) {
	root := FS()
	for _, p := range []string{"bender/groups.yaml", "bender/pipeline.yaml"} {
		if _, err := fs.Stat(root, p); err != nil {
			t.Fatalf("expected %s in embedded FS, got: %v", p, err)
		}
	}
}

func TestFS_ContainsClaudeArtefacts(t *testing.T) {
	// Spot-check that the Claude Code–native subtrees still ride along.
	root := FS()
	for _, p := range []string{"claude/agents", "claude/skills"} {
		info, err := fs.Stat(root, p)
		if err != nil {
			t.Fatalf("expected %s in embedded FS, got: %v", p, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s must be a directory", p)
		}
	}
}

func TestFS_WalkableUnderEachTopLevel(t *testing.T) {
	root := FS()
	for _, tree := range []string{"claude", "bender"} {
		count := 0
		err := fs.WalkDir(root, tree, func(_ string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", tree, err)
		}
		if count == 0 {
			t.Fatalf("expected entries under %s/", tree)
		}
	}
}
