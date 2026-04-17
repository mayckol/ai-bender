package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// CacheRoot returns the scout-cache root for a project. Clear / ClearAll wipe
// the per-session subdirectories under this root alongside the session dirs
// themselves so a cleared session takes its cached symbols + grep + module
// digests with it.
func CacheRoot(projectRoot string) string {
	return filepath.Join(projectRoot, ".bender", "cache")
}

// Clear removes a single session's on-disk data: the
// `<.bender/sessions/<id>/>` directory and the matching scout cache
// subdirectory if present. Returns fs.ErrNotExist when the session is absent
// so callers can render a friendly message.
func Clear(projectRoot, id string) error {
	sessDir := filepath.Join(SessionsRoot(projectRoot), id)
	info, err := os.Stat(sessDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return fmt.Errorf("session: stat %s: %w", sessDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("session: %s is not a directory", sessDir)
	}
	if err := os.RemoveAll(sessDir); err != nil {
		return fmt.Errorf("session: remove %s: %w", sessDir, err)
	}
	cacheDir := filepath.Join(CacheRoot(projectRoot), "scout", id)
	if err := os.RemoveAll(cacheDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("session: remove cache %s: %w", cacheDir, err)
	}
	return nil
}

// ClearAll removes every sessions/*/ directory and the whole cache root.
// Artifacts under `.bender/artifacts/` are preserved — they are authored
// outputs, not session logs. Returns the number of session directories
// removed.
func ClearAll(projectRoot string) (int, error) {
	root := SessionsRoot(projectRoot)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("session: read %s: %w", root, err)
	}
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if err := os.RemoveAll(dir); err != nil {
			return removed, fmt.Errorf("session: remove %s: %w", dir, err)
		}
		removed++
	}
	// Wipe the entire cache root — cheap, deterministic, matches "clear everything session-related".
	if err := os.RemoveAll(CacheRoot(projectRoot)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return removed, fmt.Errorf("session: remove cache root: %w", err)
	}
	return removed, nil
}
