package artifact

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateFilename_RejectsReservedRunes(t *testing.T) {
	for _, name := range []string{
		"foo:bar.md",
		"foo<bar.md",
		"foo>bar.md",
		`foo"bar.md`,
		"foo|bar.md",
		"foo?bar.md",
		"foo*bar.md",
		"foo/bar.md",
		`foo\bar.md`,
	} {
		if err := ValidateFilename(name); !errors.Is(err, ErrUnsafeFilename) {
			t.Errorf("ValidateFilename(%q): want ErrUnsafeFilename, got %v", name, err)
		}
	}
}

func TestValidateFilename_AcceptsSafe(t *testing.T) {
	for _, name := range []string{
		"users-need-roles-2026-04-16T14-03-22.md",
		"events.jsonl",
		"state.json",
		"constitution.md",
	} {
		if err := ValidateFilename(name); err != nil {
			t.Errorf("ValidateFilename(%q): want nil, got %v", name, err)
		}
	}
}

func TestValidateFilename_RejectsEmpty(t *testing.T) {
	if err := ValidateFilename(""); err == nil {
		t.Fatal("ValidateFilename(\"\"): want error, got nil")
	}
}

func TestValidateFilename_RejectsDotPrefixesShort(t *testing.T) {
	for _, name := range []string{".", ".."} {
		if err := ValidateFilename(name); err == nil {
			t.Errorf("ValidateFilename(%q): want error, got nil", name)
		}
	}
}

func TestValidateFilename_AcceptsHiddenLongerThanTwo(t *testing.T) {
	if err := ValidateFilename(".gitignore"); err != nil {
		t.Errorf("ValidateFilename(.gitignore): want nil, got %v", err)
	}
}

func TestValidateFilename_TimestampFormatIsSafe(t *testing.T) {
	// The TimestampFormat constant must produce safe filenames.
	if strings.ContainsAny(TimestampFormat, ":<>\"|?*/\\") {
		t.Fatalf("TimestampFormat %q contains a reserved character", TimestampFormat)
	}
}
