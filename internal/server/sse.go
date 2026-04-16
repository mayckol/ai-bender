package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// sseWriter writes Server-Sent Events to a single HTTP response. All writes
// are serialized and the writer becomes a no-op after Close.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
	closed  bool
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("server: ResponseWriter does not support flushing (SSE requires HTTP/1.1)")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &sseWriter{w: w, flusher: flusher}, nil
}

// write serializes the payload to JSON and emits one SSE record. Returns the
// first error (broken pipe / client disconnect) so the caller can stop work.
func (s *sseWriter) write(event string, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("server: marshal sse payload: %w", err)
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		s.closed = true
		return err
	}
	s.flusher.Flush()
	return nil
}

// writeRaw emits a string data payload (no JSON wrapping). Used for error
// frames where the payload already carries a human-readable message.
func (s *sseWriter) writeRaw(event, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		s.closed = true
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *sseWriter) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}
