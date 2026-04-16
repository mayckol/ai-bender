// Package workspace materialises the embedded `defaults/claude/` tree into a project's `.claude/`
// and (in registry.go) manages the global multi-project registry at `~/.bender/workspace.yaml`.
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

// Scaffold walks the embedded defaults tree and materialises every file under
// `<ProjectRoot>/.claude/`. Files that already exist are preserved unless Force is true.
//
// `defaults/claude/bender.yaml.tmpl` is materialised as `bender.yaml` at the project root (not under
// `.claude/`) so users can find it next to their `.gitignore` and other root-level config.
func Scaffold(opts ScaffoldOptions) (*ScaffoldResult, error) {
	if opts.ProjectRoot == "" {
		return nil, errors.New("workspace: ProjectRoot is required")
	}
	if err := os.MkdirAll(opts.ProjectRoot, 0o755); err != nil {
		return nil, err
	}
	res := &ScaffoldResult{}
	root := embedded.FS()
	err := fs.WalkDir(root, "claude", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		dest, isRootFile := destinationFor(opts.ProjectRoot, p)
		_ = isRootFile
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
	return res, nil
}

// destinationFor maps an embedded path like `claude/skills/foo/SKILL.md` to a destination on disk.
// `claude/bender.yaml.tmpl` is special-cased: it lands at <root>/bender.yaml.
func destinationFor(projectRoot, embeddedPath string) (string, bool) {
	if embeddedPath == "claude/bender.yaml.tmpl" {
		return filepath.Join(projectRoot, "bender.yaml"), true
	}
	rel := strings.TrimPrefix(embeddedPath, "claude/")
	return filepath.Join(projectRoot, ".claude", filepath.FromSlash(rel)), false
}
