// Package artifact provides path, slug, and timestamp helpers used to write artifacts safely
// across macOS, Linux, and Windows. All filenames produced or accepted here MUST be Windows-safe.
package artifact

import (
	"errors"
	"fmt"
	"strings"
)

// reservedRunes lists characters that are illegal in Windows file names (the most restrictive OS we target).
// We additionally reject path separators so that callers can't smuggle subdirectories through a filename argument.
var reservedRunes = []rune{':', '<', '>', '"', '|', '?', '*', '/', '\\'}

// ErrUnsafeFilename is returned when ValidateFilename rejects an input.
var ErrUnsafeFilename = errors.New("filename contains unsafe characters")

// ValidateFilename returns an error if name contains characters that would be rejected on Windows
// or that would smuggle path separators.
func ValidateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty", ErrUnsafeFilename)
	}
	for _, r := range name {
		for _, bad := range reservedRunes {
			if r == bad {
				return fmt.Errorf("%w: %q in %q", ErrUnsafeFilename, r, name)
			}
		}
	}
	if strings.HasPrefix(name, ".") && len(name) <= 2 {
		return fmt.Errorf("%w: reserved name %q", ErrUnsafeFilename, name)
	}
	return nil
}
