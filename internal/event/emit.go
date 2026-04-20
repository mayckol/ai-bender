package event

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EmitParams carries the inputs for Emit. Using a struct keeps the signature
// within the project's 3-parameter limit and makes the call sites self-documenting.
type EmitParams struct {
	SessionsRoot string
	SessionID    string
	Type         Type
	ActorKind    ActorKind
	ActorName    string
	Payload      map[string]any
	Timestamp    time.Time
}

var emitMu sync.Mutex

// Emit appends exactly one JSON-encoded event line to
// <SessionsRoot>/<SessionID>/events.jsonl, fsyncs, and closes. The call is
// serialised in-process by a package-level mutex so concurrent goroutines
// cannot interleave partial writes. Cross-process safety relies on POSIX
// O_APPEND atomicity for writes below PIPE_BUF, which events.jsonl lines
// comfortably satisfy in practice.
func Emit(p EmitParams) error {
	if p.SessionsRoot == "" {
		return errors.New("emit: sessions_root required")
	}
	if p.SessionID == "" {
		return errors.New("emit: session_id required")
	}
	ts := p.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	ev := &Event{
		SchemaVersion: SchemaVersion,
		SessionID:     p.SessionID,
		Timestamp:     ts,
		Actor:         Actor{Kind: p.ActorKind, Name: p.ActorName},
		Type:          p.Type,
		Payload:       p.Payload,
	}
	if err := ev.Validate(); err != nil {
		return fmt.Errorf("emit: %w", err)
	}
	if err := ev.ValidatePayload(); err != nil {
		return fmt.Errorf("emit: %w", err)
	}
	sessionDir := filepath.Join(p.SessionsRoot, p.SessionID)
	if _, err := os.Stat(sessionDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("emit: session dir missing: %s", sessionDir)
		}
		return fmt.Errorf("emit: stat session dir: %w", err)
	}
	line, err := ev.MarshalJSONLine()
	if err != nil {
		return fmt.Errorf("emit: marshal: %w", err)
	}
	path := filepath.Join(sessionDir, "events.jsonl")

	emitMu.Lock()
	defer emitMu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0o644)
	if err != nil {
		return fmt.Errorf("emit: open events.jsonl: %w", err)
	}
	if _, err := f.Write(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("emit: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("emit: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("emit: close: %w", err)
	}
	return nil
}
