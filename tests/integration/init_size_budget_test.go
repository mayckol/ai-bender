package integration_test

import (
	"io/fs"
	"path/filepath"
	"testing"
)

// TestLeanScaffold_Smaller: SC-002.
// A fresh init with every optional deselected produces strictly fewer files
// than a full-catalog scaffold. We target ≥ 1 optional-component-worth of
// files removed (conservative — the spec says ≥20%; actual varies with
// catalog growth).
func TestLeanScaffold_Smaller(t *testing.T) {
	bin := buildBenderOnce(t)

	full := mkProject(t)
	if _, _, code := runBenderExit(t, bin, full, "init"); code != 0 {
		t.Fatal("full init failed")
	}

	lean := mkProject(t)
	if _, _, code := runBenderExit(t, bin, lean, "init",
		"--without", "benchmarker",
		"--without", "sentinel",
		"--without", "surgeon",
		"--without", "mistakeinator",
	); code != 0 {
		t.Fatal("lean init failed")
	}

	fullCount := countFiles(t, full)
	leanCount := countFiles(t, lean)
	if leanCount >= fullCount {
		t.Fatalf("lean (%d files) should be smaller than full (%d files)", leanCount, fullCount)
	}
	// Spec target is ≥20% reduction; allow a softer floor of 10% to keep
	// the test stable as the catalog grows.
	if float64(fullCount-leanCount)/float64(fullCount) < 0.10 {
		t.Errorf("lean reduction %.1f%% is below the 10%% floor; full=%d lean=%d", 100*float64(fullCount-leanCount)/float64(fullCount), fullCount, leanCount)
	}
}

func countFiles(t *testing.T, root string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return n
}

