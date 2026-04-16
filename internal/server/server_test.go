package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSessionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	id := "2026-04-16T22-04-07-f6g"
	dir := filepath.Join(root, ".bender", "sessions", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"schema_version":1,"session_id":"2026-04-16T22-04-07-f6g","command":"/ghu","started_at":"2026-04-16T22:04:07Z","completed_at":"2026-04-16T22:04:28Z","status":"completed"}`
	events := strings.Join([]string{
		`{"schema_version":1,"session_id":"2026-04-16T22-04-07-f6g","timestamp":"2026-04-16T22:04:07Z","actor":{"kind":"orchestrator","name":"ghu"},"type":"session_started","payload":{"command":"/ghu","invoker":"u","working_dir":"/x"}}`,
		`{"schema_version":1,"session_id":"2026-04-16T22-04-07-f6g","timestamp":"2026-04-16T22:04:28Z","actor":{"kind":"orchestrator","name":"ghu"},"type":"session_completed","payload":{"status":"completed","duration_ms":21000}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func newTestServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	h, err := New(Config{ProjectRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(h)
}

func TestListSessions(t *testing.T) {
	root := writeSessionFixture(t)
	srv := newTestServer(t, root)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1 session, got %d", len(out))
	}
	if out[0]["id"] != "2026-04-16T22-04-07-f6g" {
		t.Fatalf("unexpected id: %v", out[0]["id"])
	}
}

func TestSnapshotSession(t *testing.T) {
	root := writeSessionFixture(t)
	srv := newTestServer(t, root)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/sessions/2026-04-16T22-04-07-f6g")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var snap struct {
		State  map[string]any   `json:"state"`
		Events []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if snap.State["session_id"] != "2026-04-16T22-04-07-f6g" {
		t.Fatalf("unexpected state: %v", snap.State)
	}
	if len(snap.Events) != 2 {
		t.Fatalf("want 2 events, got %d", len(snap.Events))
	}
}

func TestSnapshotSession_NotFound(t *testing.T) {
	root := writeSessionFixture(t)
	srv := newTestServer(t, root)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/sessions/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestServeReport_NotFound(t *testing.T) {
	root := writeSessionFixture(t)
	srv := newTestServer(t, root)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/sessions/2026-04-16T22-04-07-f6g/report")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestSPAShellServed(t *testing.T) {
	root := writeSessionFixture(t)
	srv := newTestServer(t, root)
	defer srv.Close()

	for _, path := range []string{"/", "/sessions/abc"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s: want text/html, got %q", path, ct)
		}
		resp.Body.Close()
	}
}

func TestStaticAssets(t *testing.T) {
	root := writeSessionFixture(t)
	srv := newTestServer(t, root)
	defer srv.Close()

	for _, tc := range []struct {
		path        string
		contentType string
	}{
		{"/client.js", "application/javascript"},
		{"/styles.css", "text/css"},
	} {
		resp, err := http.Get(srv.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("%s: want 200, got %d", tc.path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, tc.contentType) {
			t.Fatalf("%s: want %s, got %q", tc.path, tc.contentType, ct)
		}
		resp.Body.Close()
	}
}

func TestReportPath_StripsSessionSuffix(t *testing.T) {
	got := reportPath("/proj", "2026-04-16T22-04-07-f6g")
	want := filepath.Join("/proj", ".bender", "artifacts", "ghu", "run-2026-04-16T22-04-07-report.md")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
