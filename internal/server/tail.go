package server

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/mayckol/ai-bender/internal/event"
	"github.com/mayckol/ai-bender/internal/session"
)

// sessionTail streams new events from one session's events.jsonl plus state.json
// rewrites to a single subscriber. Stop() closes the underlying watcher and
// terminates the goroutine.
type sessionTail struct {
	dir     string
	onEvent func(*event.Event)
	onState func(*session.State)
	onErr   func(error)

	watcher *fsnotify.Watcher
	mu      sync.Mutex
	offset  int64
	carry   []byte
	done    chan struct{}
}

// newSessionTail starts watching `<dir>/events.jsonl` and `<dir>/state.json`.
// An initial drain of the existing events/state runs before this function
// returns so subscribers receive a consistent snapshot.
func newSessionTail(
	dir string,
	onEvent func(*event.Event),
	onState func(*session.State),
	onErr func(error),
) (*sessionTail, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return nil, err
	}
	t := &sessionTail{
		dir:     dir,
		onEvent: onEvent,
		onState: onState,
		onErr:   onErr,
		watcher: w,
		done:    make(chan struct{}),
	}
	t.drain()
	t.readState()
	go t.loop()
	return t, nil
}

func (t *sessionTail) Stop() {
	_ = t.watcher.Close()
	select {
	case <-t.done:
	default:
		close(t.done)
	}
}

func (t *sessionTail) loop() {
	for {
		select {
		case <-t.done:
			return
		case ev, ok := <-t.watcher.Events:
			if !ok {
				return
			}
			base := filepath.Base(ev.Name)
			switch base {
			case "events.jsonl":
				if ev.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					t.drain()
				} else if ev.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
					t.resetOffset()
					t.drain()
				}
			case "state.json":
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
					t.readState()
				}
			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			if t.onErr != nil {
				t.onErr(err)
			}
		}
	}
}

func (t *sessionTail) resetOffset() {
	t.mu.Lock()
	t.offset = 0
	t.carry = nil
	t.mu.Unlock()
}

func (t *sessionTail) drain() {
	path := filepath.Join(t.dir, "events.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) && t.onErr != nil {
			t.onErr(err)
		}
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if info.Size() < t.offset {
		t.offset = 0
		t.carry = nil
	}
	if info.Size() == t.offset {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		if t.onErr != nil {
			t.onErr(err)
		}
		return
	}
	defer f.Close()

	if _, err := f.Seek(t.offset, 0); err != nil {
		if t.onErr != nil {
			t.onErr(err)
		}
		return
	}

	buf := make([]byte, info.Size()-t.offset)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		if t.onErr != nil {
			t.onErr(err)
		}
		return
	}
	t.offset += int64(n)

	chunk := append(t.carry, buf[:n]...)
	lastNewline := bytes.LastIndexByte(chunk, '\n')
	var complete []byte
	if lastNewline >= 0 {
		complete = chunk[:lastNewline+1]
		t.carry = append([]byte(nil), chunk[lastNewline+1:]...)
	} else {
		t.carry = chunk
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(complete))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		ev, perr := event.UnmarshalEvent(line)
		if perr != nil {
			if t.onErr != nil {
				t.onErr(perr)
			}
			continue
		}
		if t.onEvent != nil {
			t.onEvent(ev)
		}
	}
}

func (t *sessionTail) readState() {
	s, err := session.LoadState(t.dir)
	if err != nil {
		if errors.Is(err, session.ErrNoState) {
			return
		}
		if t.onErr != nil {
			t.onErr(err)
		}
		return
	}
	if t.onState != nil {
		t.onState(s)
	}
}

// rootWatcher watches sessions/ for newly created session directories and
// reports their id via onAdded. Used to push `session-added` events on the
// list page.
type rootWatcher struct {
	w       *fsnotify.Watcher
	seen    map[string]struct{}
	mu      sync.Mutex
	onAdded func(string)
	onErr   func(error)
	done    chan struct{}
}

func newRootWatcher(root string, onAdded func(string), onErr func(error)) (*rootWatcher, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := w.Add(root); err != nil {
		_ = w.Close()
		return nil, err
	}
	seen := make(map[string]struct{})
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				seen[e.Name()] = struct{}{}
			}
		}
	}
	rw := &rootWatcher{w: w, seen: seen, onAdded: onAdded, onErr: onErr, done: make(chan struct{})}
	go rw.loop()
	return rw, nil
}

func (rw *rootWatcher) Stop() {
	_ = rw.w.Close()
	select {
	case <-rw.done:
	default:
		close(rw.done)
	}
}

func (rw *rootWatcher) loop() {
	for {
		select {
		case <-rw.done:
			return
		case ev, ok := <-rw.w.Events:
			if !ok {
				return
			}
			if ev.Op&fsnotify.Create == 0 {
				continue
			}
			info, err := os.Stat(ev.Name)
			if err != nil || !info.IsDir() {
				continue
			}
			id := filepath.Base(ev.Name)
			rw.mu.Lock()
			if _, had := rw.seen[id]; had {
				rw.mu.Unlock()
				continue
			}
			rw.seen[id] = struct{}{}
			rw.mu.Unlock()
			if rw.onAdded != nil {
				rw.onAdded(id)
			}
		case err, ok := <-rw.w.Errors:
			if !ok {
				return
			}
			if rw.onErr != nil {
				rw.onErr(err)
			}
		}
	}
}
