package artifact

import (
	"strings"
	"testing"
	"time"
)

func TestNow_ProducesSafeFilenameComponent(t *testing.T) {
	got := Now()
	if err := ValidateFilename(got + ".md"); err != nil {
		t.Fatalf("Now()=%q produced an unsafe filename: %v", got, err)
	}
	if strings.ContainsRune(got, ':') {
		t.Fatalf("Now()=%q must not contain a colon", got)
	}
}

func TestFormatTime_IsDeterministic(t *testing.T) {
	ref := time.Date(2026, 4, 16, 14, 3, 22, 0, time.UTC)
	if got := FormatTime(ref); got != "2026-04-16T14-03-22" {
		t.Fatalf("FormatTime: got %q want 2026-04-16T14-03-22", got)
	}
}

func TestParseTimestamp_RoundTrip(t *testing.T) {
	ref := time.Date(2026, 4, 16, 14, 3, 22, 0, time.UTC)
	formatted := FormatTime(ref)
	parsed, err := ParseTimestamp(formatted)
	if err != nil {
		t.Fatalf("ParseTimestamp: %v", err)
	}
	if !parsed.Equal(ref) {
		t.Fatalf("round trip lost time: ref=%s parsed=%s", ref, parsed)
	}
}

func TestWithCollisionSuffix_NoCollision(t *testing.T) {
	got, err := WithCollisionSuffix("base", func(string) bool { return false })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "base" {
		t.Fatalf("got %q want base", got)
	}
}

func TestWithCollisionSuffix_AppendsSuffix(t *testing.T) {
	taken := map[string]bool{"base": true, "base-1": true}
	got, err := WithCollisionSuffix("base", func(s string) bool { return taken[s] })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "base-2" {
		t.Fatalf("got %q want base-2", got)
	}
}

func TestWithCollisionSuffix_RequiresCallback(t *testing.T) {
	if _, err := WithCollisionSuffix("base", nil); err == nil {
		t.Fatal("expected error when exists callback is nil")
	}
}
