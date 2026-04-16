package integration_test

import (
	"strings"
	"testing"
)

// TestUpdate_CheckPrintsCurrent: T106.
func TestUpdate_CheckPrintsCurrent(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	out, err := runBender(t, bin, root, "update", "--check")
	if err != nil {
		t.Fatalf("update --check: %v\n%s", err, out)
	}
	if !strings.Contains(out, "current:") {
		t.Fatalf("expected 'current:' line in output:\n%s", out)
	}
}

// TestUpdate_NoChannel_ExitsExpected: T107.
// v1 has no release channel; `bender update` (without --check) MUST exit 20.
func TestUpdate_NoChannel_ExitsExpected(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	out, err := runBender(t, bin, root, "update")
	if err == nil {
		t.Fatalf("expected non-zero exit; got success:\n%s", out)
	}
	if !strings.Contains(out, "no release channel") {
		t.Fatalf("expected 'no release channel' message in output:\n%s", out)
	}
}
