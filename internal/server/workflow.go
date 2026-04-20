package server

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/mayckol/ai-bender/internal/event"
	"github.com/mayckol/ai-bender/internal/session"
)

// listWorkflowSessions returns every on-disk session whose state.json carries
// the given workflow id, ordered by StartedAt ascending (oldest first so the
// UI can concatenate their timelines in natural order).
func listWorkflowSessions(projectRoot, workflowID string) ([]session.Listing, error) {
	all, err := session.List(projectRoot)
	if err != nil {
		return nil, err
	}
	out := make([]session.Listing, 0, len(all))
	for _, l := range all {
		if l.State.WorkflowID == workflowID {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].State.StartedAt.Before(out[j].State.StartedAt)
	})
	return out, nil
}

// workflowSnapshot is the read-model for GET /api/workflows/:id.
type workflowSnapshot struct {
	WorkflowID string           `json:"workflow_id"`
	Sessions   []sessionSummary `json:"sessions"`
	Events     []*event.Event   `json:"events"`
}

// GET /api/workflows/:id
func (h *handler) snapshotWorkflow(w http.ResponseWriter, _ *http.Request, workflowID string) {
	listings, err := listWorkflowSessions(h.root, workflowID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(listings) == 0 {
		http.Error(w, fmt.Sprintf("workflow not found: %s", workflowID), http.StatusNotFound)
		return
	}

	snap := &workflowSnapshot{
		WorkflowID: workflowID,
		Sessions:   make([]sessionSummary, 0, len(listings)),
	}
	for _, l := range listings {
		snap.Sessions = append(snap.Sessions, buildSummary(l, listings))
		evs, rerr := readEvents(filepath.Join(l.Path, "events.jsonl"))
		if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
			http.Error(w, rerr.Error(), http.StatusInternalServerError)
			return
		}
		snap.Events = append(snap.Events, evs...)
	}
	sort.SliceStable(snap.Events, func(i, j int) bool {
		return snap.Events[i].Timestamp.Before(snap.Events[j].Timestamp)
	})
	writeJSON(w, http.StatusOK, snap)
}

// GET /api/workflows/:id/stream
//
// Opens one sessionTail per member session and forwards every event/state
// update through a single SSE writer. New sessions joining the workflow mid-
// stream are picked up by a 1s sessions-poll; existing tails keep running.
func (h *handler) streamWorkflow(w http.ResponseWriter, r *http.Request, workflowID string) {
	sw, err := newSSEWriter(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer sw.close()

	listings, err := listWorkflowSessions(h.root, workflowID)
	if err != nil {
		_ = sw.writeRaw("error", err.Error())
		return
	}

	initial := &workflowSnapshot{WorkflowID: workflowID}
	for _, l := range listings {
		initial.Sessions = append(initial.Sessions, buildSummary(l, listings))
		evs, _ := readEvents(filepath.Join(l.Path, "events.jsonl"))
		initial.Events = append(initial.Events, evs...)
	}
	sort.SliceStable(initial.Events, func(i, j int) bool {
		return initial.Events[i].Timestamp.Before(initial.Events[j].Timestamp)
	})
	if err := sw.write("snapshot", initial); err != nil {
		return
	}

	ctx := r.Context()
	stop := make(chan struct{})
	closeStop := func() {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}

	onEvent := func(ev *event.Event) {
		if err := sw.write("event", ev); err != nil {
			closeStop()
		}
	}
	onState := func(st *session.State) {
		if err := sw.write("state-patch", st); err != nil {
			closeStop()
		}
	}
	onErr := func(err error) {
		_ = sw.writeRaw("error", err.Error())
	}

	type tailHandle struct {
		id   string
		stop func()
	}
	var handles []tailHandle
	defer func() {
		for _, h := range handles {
			h.stop()
		}
	}()

	attach := func(dir, id string) {
		t, terr := newSessionTail(dir, onEvent, onState, onErr)
		if terr != nil {
			onErr(terr)
			return
		}
		handles = append(handles, tailHandle{id: id, stop: t.Stop})
	}

	attached := map[string]bool{}
	for _, l := range listings {
		attach(l.Path, l.ID)
		attached[l.ID] = true
	}

	// Poll every second for newly-joined sessions (a later /ghu appearing on
	// the same workflow after the initial snapshot). This mirrors the list-
	// level streamer's cadence.
	timer := newSecondTicker()
	defer timer.stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-timer.ch():
			fresh, ferr := listWorkflowSessions(h.root, workflowID)
			if ferr != nil {
				onErr(ferr)
				continue
			}
			for _, l := range fresh {
				if attached[l.ID] {
					continue
				}
				attach(l.Path, l.ID)
				attached[l.ID] = true
			}
		}
	}
}
