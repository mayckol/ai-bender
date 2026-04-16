package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Listing pairs a session directory with a parsed state.json.
type Listing struct {
	ID       string
	Path     string
	State    State
	Duration time.Duration // from state.started_at to the timestamp of the last event in events.jsonl
}

// SessionsRoot returns the absolute sessions directory inside projectRoot.
func SessionsRoot(projectRoot string) string {
	return filepath.Join(projectRoot, "artifacts", ".bender", "sessions")
}

// List returns every session under projectRoot's session directory, in deterministic id-sorted order.
// A missing root yields an empty slice (not an error) so the command can print "no sessions".
func List(projectRoot string) ([]Listing, error) {
	root := SessionsRoot(projectRoot)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("session: read sessions root: %w", err)
	}
	var out []Listing
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		state, err := LoadState(dir)
		if err != nil {
			// Skip directories that don't look like sessions; surface the parse failure to the caller.
			continue
		}
		duration := computeDuration(dir, state.StartedAt)
		out = append(out, Listing{
			ID:       e.Name(),
			Path:     dir,
			State:    *state,
			Duration: duration,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// computeDuration returns the wall time between state.started_at and the timestamp on the last
// well-formed event in events.jsonl. Returns 0 when the events file is absent or empty.
func computeDuration(sessionDir string, started time.Time) time.Duration {
	f, err := os.Open(filepath.Join(sessionDir, "events.jsonl"))
	if err != nil {
		return 0
	}
	defer f.Close()
	var last time.Time
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e struct {
			Timestamp time.Time `json:"timestamp"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if !e.Timestamp.IsZero() {
			last = e.Timestamp
		}
	}
	if last.IsZero() || started.IsZero() {
		return 0
	}
	return last.Sub(started)
}

// CopyEvents streams events.jsonl from sessionDir to w, byte-for-byte.
// This is what `bender sessions show` uses; the spec requires byte-identical re-emission.
func CopyEvents(sessionDir string, w io.Writer) error {
	f, err := os.Open(filepath.Join(sessionDir, "events.jsonl"))
	if err != nil {
		return fmt.Errorf("session: open events: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("session: copy events: %w", err)
	}
	return nil
}

// ResolveSessionDir maps a session id to its absolute directory and verifies it exists.
func ResolveSessionDir(projectRoot, id string) (string, error) {
	dir := filepath.Join(SessionsRoot(projectRoot), id)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("session: %q not found in %s", id, SessionsRoot(projectRoot))
	}
	return dir, nil
}
