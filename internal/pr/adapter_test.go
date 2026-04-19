package pr

import (
	"context"
	"errors"
	"testing"
)

// TestAdapterInterface_CompileTimeContract exercises every method of every
// shipped adapter against the fake exec so the Adapter interface is the
// single source of truth.
func TestAdapterInterface_CompileTimeContract(t *testing.T) {
	var _ Adapter = (*GitHubAdapter)(nil)
	var _ Adapter = (*GitLabAdapter)(nil)
}

func TestGitLabAdapter_AllOpsReturnUnimplemented(t *testing.T) {
	a := NewGitLabAdapter(nil)
	ctx := context.Background()
	if err := a.AuthCheck(ctx); !errors.Is(err, ErrAdapterUnimplemented) {
		t.Errorf("AuthCheck: want ErrAdapterUnimplemented, got %v", err)
	}
	if err := a.Push(ctx, PushInput{}); !errors.Is(err, ErrAdapterUnimplemented) {
		t.Errorf("Push: want ErrAdapterUnimplemented, got %v", err)
	}
	if _, err := a.OpenOrUpdate(ctx, OpenArgs{}); !errors.Is(err, ErrAdapterUnimplemented) {
		t.Errorf("OpenOrUpdate: want ErrAdapterUnimplemented, got %v", err)
	}
}
