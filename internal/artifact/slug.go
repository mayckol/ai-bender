package artifact

import (
	"strings"
	"unicode"
)

// Slug derives a kebab-case slug from a free-form title. It is deterministic, ASCII-only,
// and bounded to 80 characters so the resulting filenames stay within filesystem limits.
func Slug(title string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case unicode.IsLetter(r) && r < 128:
			b.WriteRune(r)
			prevDash = false
		case unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case prevDash:
			// collapse separators
		default:
			b.WriteRune('-')
			prevDash = true
		}
		if b.Len() >= 80 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	return out
}
