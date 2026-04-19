package worktree

import (
	"context"
	"errors"
	"testing"
)

func TestBranchName_Canonical(t *testing.T) {
	got := BranchName("019a3b4c-e1f2-7c8d-9a0b-1c2d3e4f5061")
	want := "bender/session/019a3b4c-e1f2-7c8d-9a0b-1c2d3e4f5061"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestValidateSessionID_Accepts(t *testing.T) {
	for _, id := range []string{
		"abc",
		"019a3b4c-e1f2-7c8d-9a0b-1c2d3e4f5061",
		"a.b_c-1",
		"Z0",
	} {
		if err := ValidateSessionID(id); err != nil {
			t.Errorf("ValidateSessionID(%q): %v", id, err)
		}
	}
}

func TestValidateSessionID_Rejects(t *testing.T) {
	for _, id := range []string{
		"",
		"has space",
		"weird~ref",
		"/leading-slash",
		"-leading-dash",
		"bender/session/abc",
		"..double-dot",
		"has\\slash",
	} {
		if err := ValidateSessionID(id); err == nil {
			t.Errorf("ValidateSessionID(%q): expected error", id)
		}
	}
}

func TestBranchExists_PresentAndAbsent(t *testing.T) {
	ctx := context.Background()
	present := &FakeRunner{
		Responses: map[string]FakeResponse{
			"show-ref": {Err: nil},
		},
	}
	got, err := BranchExists(ctx, present, "/repo", "bender/session/x")
	if err != nil {
		t.Fatalf("present: %v", err)
	}
	if !got {
		t.Fatalf("present: expected true")
	}

	absent := &FakeRunner{
		Responses: map[string]FakeResponse{
			"show-ref": {Err: errors.New("exit status 1")},
		},
	}
	got, err = BranchExists(ctx, absent, "/repo", "bender/session/x")
	if err != nil {
		t.Fatalf("absent: %v", err)
	}
	if got {
		t.Fatalf("absent: expected false")
	}
}
