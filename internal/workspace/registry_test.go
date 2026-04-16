package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// withIsolatedRegistry redirects the registry to a per-test directory by setting XDG_CONFIG_HOME.
func withIsolatedRegistry(tb testing.TB) {
	tb.Helper()
	dir := tb.TempDir()
	tb.Setenv("XDG_CONFIG_HOME", dir)
}

func TestRegister_AcceptsValidPath(t *testing.T) {
	withIsolatedRegistry(t)
	root := t.TempDir()
	name, entry, err := Register("api", root)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if name != "api" {
		t.Fatalf("name: got %q want api", name)
	}
	abs, _ := filepath.Abs(root)
	if entry.Path != abs {
		t.Fatalf("entry path mismatch: got %q want %q", entry.Path, abs)
	}
}

func TestRegister_AutoDerivesNameFromBasename(t *testing.T) {
	withIsolatedRegistry(t)
	root := filepath.Join(t.TempDir(), "MyService_v2")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	name, _, err := Register("", root)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if name != "myservice-v2" {
		t.Fatalf("auto name: got %q want myservice-v2", name)
	}
}

func TestRegister_RejectsBadName(t *testing.T) {
	withIsolatedRegistry(t)
	root := t.TempDir()
	_, _, err := Register("BadName", root)
	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("got %v want ErrInvalidName", err)
	}
}

func TestRegister_RejectsMissingPath(t *testing.T) {
	withIsolatedRegistry(t)
	_, _, err := Register("missing", "/no/such/path/xyz")
	if !errors.Is(err, ErrPathNotDir) {
		t.Fatalf("got %v want ErrPathNotDir", err)
	}
}

func TestRegister_RejectsDuplicateName(t *testing.T) {
	withIsolatedRegistry(t)
	root1 := t.TempDir()
	root2 := t.TempDir()
	if _, _, err := Register("api", root1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Register("api", root2); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("got %v want ErrDuplicateName", err)
	}
}

func TestList_MarksCurrentAndMissing(t *testing.T) {
	withIsolatedRegistry(t)
	root := t.TempDir()
	if _, _, err := Register("api", root); err != nil {
		t.Fatal(err)
	}
	gone := t.TempDir()
	if _, _, err := Register("ghost", gone); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	list, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 listings, got %d", len(list))
	}
	statuses := map[string]Status{}
	for _, l := range list {
		statuses[l.Name] = l.Status
	}
	if statuses["api"] != StatusCurrent {
		t.Errorf("api: got %s want current", statuses["api"])
	}
	if statuses["ghost"] != StatusMissing {
		t.Errorf("ghost: got %s want missing", statuses["ghost"])
	}
}

func TestResolve_FlagWins(t *testing.T) {
	withIsolatedRegistry(t)
	root1 := t.TempDir()
	root2 := t.TempDir()
	if _, _, err := Register("a", root1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Register("b", root2); err != nil {
		t.Fatal(err)
	}
	name, path, err := Resolve("b", root1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "b" || path != mustAbs(root2) {
		t.Fatalf("got name=%q path=%q want b/%q", name, path, mustAbs(root2))
	}
}

func TestResolve_CwdWinsOverDefault(t *testing.T) {
	withIsolatedRegistry(t)
	root1 := t.TempDir()
	root2 := t.TempDir()
	if _, _, err := Register("a", root1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Register("b", root2); err != nil {
		t.Fatal(err)
	}
	// Default is "a" (first-registered). cwd inside root2 should resolve to "b".
	name, _, err := Resolve("", root2)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "b" {
		t.Fatalf("got %q want b (cwd-resident project)", name)
	}
}

func TestResolve_FallsBackToDefault(t *testing.T) {
	withIsolatedRegistry(t)
	root := t.TempDir()
	if _, _, err := Register("a", root); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	name, _, err := Resolve("", other)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "a" {
		t.Fatalf("got %q want a (default fallback)", name)
	}
}

func mustAbs(p string) string {
	abs, _ := filepath.Abs(p)
	return abs
}
