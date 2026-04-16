package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExportDocument is the schema produced by `bender sessions export <id>`.
// It is the v1 contract for ingestion by a future UI server.
type ExportDocument struct {
	SchemaVersion int               `json:"schema_version"`
	SessionID     string            `json:"session_id"`
	State         *State            `json:"state"`
	Events        []json.RawMessage `json:"events"`
}

// Export reads state.json + events.jsonl from sessionDir and emits the ExportDocument JSON to w.
// Uses RawMessage for events so we re-emit them exactly as they were appended (no field re-ordering).
func Export(sessionDir string, w io.Writer) error {
	state, err := LoadState(sessionDir)
	if err != nil {
		return err
	}
	events, err := readEventLines(sessionDir)
	if err != nil {
		return err
	}
	doc := ExportDocument{
		SchemaVersion: SchemaVersion,
		SessionID:     state.SessionID,
		State:         state,
		Events:        events,
	}
	// Compact (no SetIndent): pretty-printing rewrites json.RawMessage contents, breaking
	// the byte-identical round-trip guarantee in SC-009. UIs can pretty-print themselves.
	enc := json.NewEncoder(w)
	return enc.Encode(doc)
}

func readEventLines(sessionDir string) ([]json.RawMessage, error) {
	f, err := os.Open(filepath.Join(sessionDir, "events.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("session: open events: %w", err)
	}
	defer f.Close()
	var out []json.RawMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(line) == 0 {
			continue
		}
		out = append(out, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("session: scan events: %w", err)
	}
	return out, nil
}
