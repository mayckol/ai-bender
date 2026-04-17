package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mayckol/ai-bender/internal/event"
	"github.com/mayckol/ai-bender/internal/session"
)

// Config controls server behavior. ProjectRoot is the project whose
// `.bender/sessions` directory is watched and served.
type Config struct {
	ProjectRoot string
	Addr        string // e.g. ":4317"
}

// New builds the HTTP handler tree. The server itself is constructed by the
// caller via http.Server so they can control timeouts, TLS, and lifecycle.
func New(cfg Config) (http.Handler, error) {
	if cfg.ProjectRoot == "" {
		return nil, errors.New("server: ProjectRoot is required")
	}
	root, err := filepath.Abs(cfg.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("server: resolve project root: %w", err)
	}

	indexHTML, err := clientFS.ReadFile("assets/index.html")
	if err != nil {
		return nil, fmt.Errorf("server: load index.html: %w", err)
	}
	clientJS, err := clientFS.ReadFile("assets/client.js")
	if err != nil {
		return nil, fmt.Errorf("server: load client.js: %w", err)
	}
	stylesCSS, err := clientFS.ReadFile("assets/styles.css")
	if err != nil {
		return nil, fmt.Errorf("server: load styles.css: %w", err)
	}

	h := &handler{
		root:      root,
		indexHTML: indexHTML,
		clientJS:  clientJS,
		stylesCSS: stylesCSS,
	}
	return h, nil
}

type handler struct {
	root      string
	indexHTML []byte
	clientJS  []byte
	stylesCSS []byte
}

var sessionRoute = regexp.MustCompile(`^/api/sessions/([^/]+)(?:/(stream|report))?$`)

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/sessions":
		if r.Method == http.MethodDelete {
			h.deleteAllSessions(w, r)
			return
		}
		h.listSessions(w, r)
		return
	case path == "/api/sessions/stream":
		h.streamSessions(w, r)
		return
	case path == "/client.js":
		writeBytes(w, "application/javascript; charset=utf-8", h.clientJS)
		return
	case path == "/styles.css":
		writeBytes(w, "text/css; charset=utf-8", h.stylesCSS)
		return
	}

	if m := sessionRoute.FindStringSubmatch(path); m != nil {
		id := m[1]
		suffix := m[2]

		if r.Method == http.MethodDelete && suffix == "" {
			h.deleteSession(w, r, id)
			return
		}

		dir, err := session.ResolveSessionDir(h.root, id)
		if err != nil {
			http.Error(w, fmt.Sprintf("session not found: %s", id), http.StatusNotFound)
			return
		}
		switch suffix {
		case "stream":
			h.streamSession(w, r, id, dir)
		case "report":
			h.serveReport(w, r, id)
		default:
			h.snapshotSession(w, r, dir)
		}
		return
	}

	// Catch-all: serve the SPA shell for any other path so client-side routing works.
	writeBytes(w, "text/html; charset=utf-8", h.indexHTML)
}

// GET /api/sessions
func (h *handler) listSessions(w http.ResponseWriter, _ *http.Request) {
	listings, err := session.List(h.root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	summaries := make([]sessionSummary, 0, len(listings))
	for _, l := range listings {
		summaries = append(summaries, buildSummary(l, listings))
	}
	writeJSON(w, http.StatusOK, summaries)
}

func buildSummary(l session.Listing, peers []session.Listing) sessionSummary {
	sum := sessionSummary{
		ID:              l.ID,
		State:           l.State,
		DurationMS:      l.Duration.Milliseconds(),
		Agents:          []string{},
		Skills:          []string{},
		EffectiveStatus: computeEffectiveStatus(l.State, peers),
	}
	if events, err := session.SummarizeEvents(l.Path); err == nil {
		sum.Agents = events.Agents
		sum.Skills = events.Skills
	}
	return sum
}

// GET /api/sessions/:id
func (h *handler) snapshotSession(w http.ResponseWriter, _ *http.Request, dir string) {
	snap, err := loadSnapshot(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.enrichSnapshotWithEffectiveStatus(snap)
	writeJSON(w, http.StatusOK, snap)
}

func (h *handler) enrichSnapshotWithEffectiveStatus(snap *sessionSnapshot) {
	if snap == nil {
		return
	}
	peers, err := session.List(h.root)
	if err != nil {
		return
	}
	snap.EffectiveStatus = computeEffectiveStatus(snap.State, peers)
}

// GET /api/sessions/:id/stream (SSE)
func (h *handler) streamSession(w http.ResponseWriter, r *http.Request, _ string, dir string) {
	sw, err := newSSEWriter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer sw.close()

	snap, err := loadSnapshot(dir)
	if err != nil {
		_ = sw.writeRaw("error", err.Error())
		return
	}
	h.enrichSnapshotWithEffectiveStatus(snap)
	if err := sw.write("snapshot", snap); err != nil {
		return
	}

	ctx := r.Context()
	stop := make(chan struct{})

	tail, err := newSessionTail(
		dir,
		func(ev *event.Event) {
			if err := sw.write("event", ev); err != nil {
				select {
				case <-stop:
				default:
					close(stop)
				}
			}
		},
		func(s *session.State) {
			if err := sw.write("state-patch", s); err != nil {
				select {
				case <-stop:
				default:
					close(stop)
				}
			}
		},
		func(err error) {
			_ = sw.writeRaw("error", err.Error())
		},
	)
	if err != nil {
		_ = sw.writeRaw("error", err.Error())
		return
	}
	defer tail.Stop()

	select {
	case <-ctx.Done():
	case <-stop:
	}
}

// GET /api/sessions/stream (SSE, list-level)
//
// Re-emits a full snapshot every 1s instead of relying on fsnotify. The list
// is tiny (a few KB at most) and polling catches both new session directories
// AND status transitions inside existing ones — fsnotify's kqueue backend on
// macOS misses the latter and is flaky for the former. 1s keeps the inter-
// session gap (prior session ending → next session appearing) imperceptible
// while staying cheap. Preact re-keys rows by id, so unchanged rows don't
// re-render client-side.
func (h *handler) streamSessions(w http.ResponseWriter, r *http.Request) {
	sw, err := newSSEWriter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer sw.close()

	emit := func() bool {
		listings, err := session.List(h.root)
		if err != nil {
			_ = sw.writeRaw("error", err.Error())
			return false
		}
		summaries := make([]sessionSummary, 0, len(listings))
		for _, l := range listings {
			summaries = append(summaries, buildSummary(l, listings))
		}
		return sw.write("snapshot", summaries) == nil
	}

	if !emit() {
		return
	}

	ctx := r.Context()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !emit() {
				return
			}
		}
	}
}

// DELETE /api/sessions/:id
func (h *handler) deleteSession(w http.ResponseWriter, _ *http.Request, id string) {
	if err := session.Clear(h.root, id); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, fmt.Sprintf("session not found: %s", id), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/sessions?all=true
func (h *handler) deleteAllSessions(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("all") != "true" {
		http.Error(w, "add ?all=true to confirm a wipe of every session", http.StatusBadRequest)
		return
	}
	removed, err := session.ClearAll(h.root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed": removed})
}

// GET /api/sessions/:id/report
func (h *handler) serveReport(w http.ResponseWriter, _ *http.Request, id string) {
	path := reportPath(h.root, id)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeBytes(w, "text/markdown; charset=utf-8", data)
}

// Session id suffix stripping — the /ghu skill writes
// `.bender/artifacts/ghu/run-<timestamp>-report.md` where <timestamp> matches
// the leading `YYYY-MM-DDTHH-MM-SS` portion of the session id.
var sessionTSHead = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2})`)

func reportPath(projectRoot, sessionID string) string {
	ts := sessionID
	if m := sessionTSHead.FindStringSubmatch(sessionID); len(m) > 0 {
		ts = m[1]
	}
	return filepath.Join(projectRoot, ".bender", "artifacts", "ghu", fmt.Sprintf("run-%s-report.md", ts))
}

type sessionSummary struct {
	ID              string        `json:"id"`
	State           session.State `json:"state"`
	DurationMS      int64         `json:"duration_ms"`
	Agents          []string      `json:"agents"`
	Skills          []string      `json:"skills"`
	EffectiveStatus string        `json:"effective_status"`
}

type sessionSnapshot struct {
	State           session.State  `json:"state"`
	Events          []*event.Event `json:"events"`
	EffectiveStatus string         `json:"effective_status"`
}

func loadSnapshot(dir string) (*sessionSnapshot, error) {
	state, err := session.LoadState(dir)
	if err != nil {
		return nil, err
	}
	events, err := readEvents(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		return nil, err
	}
	return &sessionSnapshot{State: *state, Events: events, EffectiveStatus: string(state.Status)}, nil
}

// computeEffectiveStatus derives the status to render in the UI. It exists
// because a `/cry` draft session ends at `awaiting_confirm` — an honest
// record of what happened in THAT session — but once a later `/cry confirm`
// session lands, the user's mental model is "this is done". We keep the
// draft's `state.json` untouched (no retroactive mutation) and surface an
// effective status derived from the sibling set.
//
// Rule: if `state.status == "awaiting_confirm"` AND a sibling session in
// `peers` has `command = "<this.command> confirm"` with a later `started_at`
// AND terminal `status = "completed"`, effective status is `completed`.
// Otherwise, effective status equals `state.status`.
func computeEffectiveStatus(state session.State, peers []session.Listing) string {
	if state.Status != "awaiting_confirm" {
		return state.Status
	}
	confirmCmd := strings.TrimSpace(state.Command) + " confirm"
	for _, p := range peers {
		if p.State.Command != confirmCmd {
			continue
		}
		if !p.State.StartedAt.After(state.StartedAt) {
			continue
		}
		if p.State.Status == "completed" {
			return "completed"
		}
	}
	return state.Status
}

func readEvents(path string) ([]*event.Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []*event.Event{}, nil
		}
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	out := make([]*event.Event, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		ev, perr := event.UnmarshalEvent([]byte(line))
		if perr != nil {
			continue // Skip malformed; CLI `bender sessions validate` surfaces them.
		}
		out = append(out, ev)
	}
	return out, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeBytes(w http.ResponseWriter, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
