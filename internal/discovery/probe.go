// Package discovery runs heuristic, no-AI detection over a project tree to populate the constitution.
//
// Detectors share a single file walk to keep the I/O budget small (SC-001 requires `bender init`
// under 30 s on representative projects). They consume a Probe rather than touching the filesystem
// directly so each detector is unit-testable in isolation.
package discovery

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Probe is the read-only interface detectors use to inspect a project tree.
type Probe interface {
	Has(path string) bool
	Read(path string) ([]byte, error)
	HasGlob(pattern string) bool
	CountByExt(ext string) int
	TopLevelFiles() []string
	TopLevelDirs() []string
	HasAnyUnder(dir string) bool
}

// MaxWalkDepth bounds how deep the discovery walker descends. The detectors only need a few levels
// to find manifests, CI configs, and folder shape; deeper walks waste I/O.
const MaxWalkDepth = 4

// SkipDirs lists directory names the walker never descends into.
var SkipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	".venv":        {},
	"venv":         {},
	"__pycache__":  {},
	".idea":        {},
	".vscode":      {},
	"target":       {},
}

// fileIndex implements Probe over a precomputed snapshot.
type fileIndex struct {
	root           string
	allFiles       map[string]struct{}
	topLevelFiles  []string
	topLevelDirs   []string
	extCounts      map[string]int
	subtreePresent map[string]bool
}

// Walk produces a Probe by walking root once, bounded to MaxWalkDepth and SkipDirs.
func Walk(root string) (Probe, error) {
	idx := &fileIndex{
		root:           root,
		allFiles:       map[string]struct{}{},
		extCounts:      map[string]int{},
		subtreePresent: map[string]bool{},
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !rootInfo.IsDir() {
		return nil, errors.New("discovery: root is not a directory")
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if _, skip := SkipDirs[d.Name()]; skip {
				return filepath.SkipDir
			}
			depth := strings.Count(rel, "/") + 1
			if depth > MaxWalkDepth {
				return filepath.SkipDir
			}
			if depth == 1 {
				idx.topLevelDirs = append(idx.topLevelDirs, rel)
			}
			idx.subtreePresent[rel] = true
			return nil
		}
		idx.allFiles[rel] = struct{}{}
		if !strings.Contains(rel, "/") {
			idx.topLevelFiles = append(idx.topLevelFiles, rel)
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if ext != "" {
			idx.extCounts[ext]++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(idx.topLevelFiles)
	sort.Strings(idx.topLevelDirs)
	return idx, nil
}

func (i *fileIndex) Has(path string) bool {
	_, ok := i.allFiles[path]
	return ok
}

func (i *fileIndex) Read(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(i.root, filepath.FromSlash(path)))
}

func (i *fileIndex) HasGlob(pattern string) bool {
	for f := range i.allFiles {
		if matched, _ := filepath.Match(pattern, f); matched {
			return true
		}
	}
	return false
}

func (i *fileIndex) CountByExt(ext string) int {
	return i.extCounts[strings.ToLower(ext)]
}

func (i *fileIndex) TopLevelFiles() []string { return append([]string(nil), i.topLevelFiles...) }
func (i *fileIndex) TopLevelDirs() []string  { return append([]string(nil), i.topLevelDirs...) }

func (i *fileIndex) HasAnyUnder(dir string) bool {
	for f := range i.allFiles {
		if strings.HasPrefix(f, dir+"/") || f == dir {
			return true
		}
	}
	return i.subtreePresent[dir]
}
