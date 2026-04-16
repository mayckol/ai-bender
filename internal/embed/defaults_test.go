package embed

import (
	"io/fs"
	"testing"
)

func TestFS_ContainsClaudeRoot(t *testing.T) {
	root := FS()
	if _, err := fs.Stat(root, "claude/groups.yaml"); err != nil {
		t.Fatalf("expected claude/groups.yaml in embedded FS, got: %v", err)
	}
	if _, err := fs.Stat(root, "claude/bender.yaml.tmpl"); err != nil {
		t.Fatalf("expected claude/bender.yaml.tmpl in embedded FS, got: %v", err)
	}
}

func TestFS_WalkableUnderClaude(t *testing.T) {
	root := FS()
	count := 0
	err := fs.WalkDir(root, "claude", func(_ string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least the claude/ root and the seed files")
	}
}
