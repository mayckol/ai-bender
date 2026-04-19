package worktree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultRootRelPath is the repo-relative default location for session
// worktrees. Resolved from clarification Q1 in specs/004-worktree-flow.
const DefaultRootRelPath = ".bender/worktrees"

// ErrPlacementRefused is returned (wrapped) when the configured worktree root
// violates git's worktree placement constraints. Callers map this to exit
// code 12.
var ErrPlacementRefused = errors.New("placement refused")

// ResolveRoot returns the absolute path under which session worktrees are
// created for the repo rooted at repoRoot. It reads `.bender/config.yaml`'s
// `worktree.root` key when present, falling back to DefaultRootRelPath.
//
// Relative override paths are interpreted relative to repoRoot; absolute
// override paths are used as-is. The returned path is cleaned but not
// validated — call ValidatePlacement before creating a worktree under it.
func ResolveRoot(repoRoot string) (string, error) {
	cfgPath := filepath.Join(repoRoot, ".bender", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Join(repoRoot, DefaultRootRelPath), nil
		}
		return "", fmt.Errorf("worktree: read config: %w", err)
	}
	var cfg struct {
		Worktree struct {
			Root string `yaml:"root"`
		} `yaml:"worktree"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("worktree: parse config: %w", err)
	}
	root := strings.TrimSpace(cfg.Worktree.Root)
	if root == "" {
		return filepath.Join(repoRoot, DefaultRootRelPath), nil
	}
	if filepath.IsAbs(root) {
		return filepath.Clean(root), nil
	}
	return filepath.Clean(filepath.Join(repoRoot, root)), nil
}

// ValidatePlacement rejects a candidate worktree path when it would violate
// git's worktree placement constraints. Specifically:
//
//   - must not be inside the main repo's .git directory,
//   - must not be the main working tree itself,
//   - must not be an existing path on disk.
//
// Overlap with other registered worktrees is detected separately by calling
// `git worktree list --porcelain` at create time — that requires the runner,
// so it lives in the Create flow rather than here.
func ValidatePlacement(repoRoot, candidate string) error {
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Clean(filepath.Join(repoRoot, candidate))
	} else {
		candidate = filepath.Clean(candidate)
	}
	repoRoot = filepath.Clean(repoRoot)
	gitDir := filepath.Join(repoRoot, ".git")

	if candidate == repoRoot {
		return fmt.Errorf("%w: worktree cannot be the main working tree", ErrPlacementRefused)
	}
	if candidate == gitDir || strings.HasPrefix(candidate+string(filepath.Separator), gitDir+string(filepath.Separator)) {
		return fmt.Errorf("%w: worktree cannot live inside .git/", ErrPlacementRefused)
	}
	if _, err := os.Stat(candidate); err == nil {
		return fmt.Errorf("%w: path already exists: %s", ErrPlacementRefused, candidate)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("worktree: stat candidate: %w", err)
	}
	return nil
}
