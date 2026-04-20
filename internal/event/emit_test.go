package event

import (
	"bufio"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func mkSessionDir(t *testing.T) (root, sessionID, dir string) {
	t.Helper()
	root = t.TempDir()
	sessionID = "2026-04-20T13-15-22-abc"
	dir = filepath.Join(root, sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return root, sessionID, dir
}

func TestEmit_AppendsOneLinePerCall(t *testing.T) {
	root, sid, dir := mkSessionDir(t)
	fixed := time.Date(2026, 4, 20, 13, 15, 22, 0, time.UTC)
	for i := 0; i < 5; i++ {
		err := Emit(EmitParams{
			SessionsRoot: root,
			SessionID:    sid,
			Type:         TypeAgentProgress,
			ActorKind:    ActorAgent,
			ActorName:    "scout",
			Payload:      map[string]any{"agent": "scout", "percent": i * 20, "current_step": "scanning"},
			Timestamp:    fixed,
		})
		if err != nil {
			t.Fatalf("Emit #%d: %v", i, err)
		}
	}
	f, err := os.Open(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		ev, err := UnmarshalEvent(line)
		if err != nil {
			t.Fatalf("line %d parse: %v", count, err)
		}
		if ev.Type != TypeAgentProgress {
			t.Errorf("line %d: type=%s", count, ev.Type)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if count != 5 {
		t.Fatalf("want 5 lines, got %d", count)
	}
}

func TestEmit_RejectsMissingSessionDir(t *testing.T) {
	root := t.TempDir()
	err := Emit(EmitParams{
		SessionsRoot: root,
		SessionID:    "does-not-exist",
		Type:         TypeOrchProgress,
		ActorKind:    ActorOrchestrator,
		ActorName:    "ghu",
		Payload:      map[string]any{"percent": 10, "current_step": "scout"},
	})
	if err == nil {
		t.Fatal("expected error for missing session dir")
	}
}

func TestEmit_RejectsInvalidEvent(t *testing.T) {
	root, sid, _ := mkSessionDir(t)
	err := Emit(EmitParams{
		SessionsRoot: root,
		SessionID:    sid,
		Type:         "not_a_real_type",
		ActorKind:    ActorOrchestrator,
		ActorName:    "ghu",
		Payload:      map[string]any{},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestEmit_ConcurrentDoesNotInterleave(t *testing.T) {
	root, sid, dir := mkSessionDir(t)
	const n = 40
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = Emit(EmitParams{
				SessionsRoot: root,
				SessionID:    sid,
				Type:         TypeAgentProgress,
				ActorKind:    ActorAgent,
				ActorName:    "scout",
				Payload:      map[string]any{"agent": "scout", "percent": i, "current_step": "x"},
			})
		}(i)
	}
	wg.Wait()
	f, err := os.Open(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		if _, err := UnmarshalEvent(scanner.Bytes()); err != nil {
			t.Fatalf("line %d: %v", count, err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if count != n {
		t.Fatalf("want %d lines, got %d", n, count)
	}
}

func TestEmit_DefaultsTimestampToNow(t *testing.T) {
	root, sid, dir := mkSessionDir(t)
	before := time.Now().UTC().Add(-time.Second)
	err := Emit(EmitParams{
		SessionsRoot: root,
		SessionID:    sid,
		Type:         TypeOrchProgress,
		ActorKind:    ActorOrchestrator,
		ActorName:    "ghu",
		Payload:      map[string]any{"percent": 10, "current_step": "scout"},
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	after := time.Now().UTC().Add(time.Second)
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	ev, err := UnmarshalEvent(data[:len(data)-1])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.Timestamp.Before(before) || ev.Timestamp.After(after) {
		t.Fatalf("timestamp out of range: %s not in [%s,%s]", ev.Timestamp, before, after)
	}
}
