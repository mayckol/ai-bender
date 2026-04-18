package mistakes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppend_CreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mistakes.md")
	dup, err := Append(path, Entry{
		ID:    "a",
		Scope: "internal/userservice",
		Title: "Sample",
		Avoid: "don't do X",
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if dup {
		t.Error("first append should not be a dup")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "id: a") {
		t.Errorf("missing id: %s", s)
	}
	if !strings.Contains(s, "Avoid:") {
		t.Errorf("missing Avoid: %s", s)
	}
}

func TestAppend_DedupesByID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mistakes.md")
	if _, err := Append(path, Entry{ID: "a", Scope: "x", Title: "X", Avoid: "x"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	dup, err := Append(path, Entry{ID: "a", Scope: "x", Title: "X", Avoid: "different"})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !dup {
		t.Error("second append with same id should be dup")
	}
	entries, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("want 1 entry, got %d", len(entries))
	}
	// The original Avoid text is preserved; the dup did not overwrite.
	if entries[0].Avoid != "x" {
		t.Errorf("dup overwrote original entry: %+v", entries[0])
	}
}

func TestRead_Absent_ReturnsNil(t *testing.T) {
	entries, err := Read(filepath.Join(t.TempDir(), "missing.md"))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if entries != nil {
		t.Errorf("want nil, got %v", entries)
	}
}

func TestRead_Malformed_Rejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mistakes.md")
	_ = os.WriteFile(path, []byte("not-frontmatter at all\n"), 0o644)
	if _, err := Read(path); err == nil {
		t.Error("want error for malformed artifact")
	}
}

func TestFilter_ByScope(t *testing.T) {
	entries := []Entry{
		{ID: "a", Scope: "internal/userservice", Title: "t", Avoid: "a"},
		{ID: "b", Scope: "internal/billing", Title: "t", Avoid: "a"},
	}
	got := Filter(entries, []string{"internal/userservice/user.go"}, nil)
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("prefix match failed: %v", got)
	}
}

func TestFilter_ByTag(t *testing.T) {
	entries := []Entry{
		{ID: "a", Scope: "tag:errors"},
		{ID: "b", Scope: "tag:perf"},
	}
	got := Filter(entries, nil, []string{"errors"})
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("tag match failed: %v", got)
	}
}

func TestAppend_RoundtripPreservesScopeAndTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mistakes.md")
	in := Entry{
		ID:      "u1",
		Scope:   "internal/userservice",
		Tags:    []string{"errors", "data-loss"},
		Created: time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
		Title:   "Swallowed errors",
		Avoid:   "don't",
		Prefer:  "do",
	}
	if _, err := Append(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1 entry, got %d", len(out))
	}
	got := out[0]
	if got.ID != in.ID || got.Scope != in.Scope || got.Avoid != in.Avoid || got.Prefer != in.Prefer {
		t.Errorf("roundtrip mismatch: %+v vs %+v", got, in)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags dropped: %v", got.Tags)
	}
}
