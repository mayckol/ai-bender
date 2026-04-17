// Package workspace materialises the embedded defaults tree into a project's
// `.claude/` (Claude Code–native artefacts: agents, skills) and `.bender/`
// (bender-owned config: pipeline.yaml, groups.yaml) plus, in registry.go,
// manages the global multi-project registry at `~/.bender/workspace.yaml`.
package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	embedded "github.com/mayckol/ai-bender/internal/embed"
)

// ScaffoldOptions controls a single materialisation pass.
type ScaffoldOptions struct {
	ProjectRoot string
	Force       bool
}

// ScaffoldResult summarises what changed during a Scaffold call.
type ScaffoldResult struct {
	Created     []string
	Preserved   []string
	Overwritten []string
}

// embeddedTrees lists every top-level prefix in the embedded `defaults/` FS
// that we mirror onto disk. Adding a new bender-owned tree = one line here.
var embeddedTrees = []string{"claude", "bender"}

// Scaffold walks every top-level tree in the embedded defaults and
// materialises files under the project root. `claude/<path>` mirrors to
// `<root>/.claude/<path>`; `bender/<path>` mirrors to `<root>/.bender/<path>`.
// Files that already exist are preserved unless Force is true.
func Scaffold(opts ScaffoldOptions) (*ScaffoldResult, error) {
	if opts.ProjectRoot == "" {
		return nil, errors.New("workspace: ProjectRoot is required")
	}
	if err := os.MkdirAll(opts.ProjectRoot, 0o755); err != nil {
		return nil, err
	}
	res := &ScaffoldResult{}
	root := embedded.FS()
	for _, tree := range embeddedTrees {
		if _, err := fs.Stat(root, tree); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("stat embedded %s: %w", tree, err)
		}
		err := fs.WalkDir(root, tree, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			dest := destinationFor(opts.ProjectRoot, p)
			if dest == "" {
				return nil
			}
			exists := false
			if _, err := os.Stat(dest); err == nil {
				exists = true
			}
			if exists && !opts.Force {
				res.Preserved = append(res.Preserved, dest)
				return nil
			}
			data, err := fs.ReadFile(root, p)
			if err != nil {
				return fmt.Errorf("read embedded %s: %w", p, err)
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
			if exists {
				res.Overwritten = append(res.Overwritten, dest)
			} else {
				res.Created = append(res.Created, dest)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// destinationFor maps an embedded path to its on-disk destination.
// `claude/<rel>` → `<root>/.claude/<rel>`; `bender/<rel>` → `<root>/.bender/<rel>`.
// Unknown prefixes return "" so the walker skips them.
func destinationFor(projectRoot, embeddedPath string) string {
	for _, tree := range embeddedTrees {
		if rel, ok := strings.CutPrefix(embeddedPath, tree+"/"); ok {
			return filepath.Join(projectRoot, "."+tree, filepath.FromSlash(rel))
		}
	}
	return ""
}
