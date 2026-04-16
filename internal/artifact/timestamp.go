package artifact

import (
	"fmt"
	"time"
)

// TimestampFormat is the canonical format used for every filename produced by ai-bender.
// Hyphens replace colons so the format is safe on Windows file systems.
const TimestampFormat = "2006-01-02T15-04-05"

// Now returns the current UTC time formatted with TimestampFormat.
func Now() string {
	return FormatTime(time.Now().UTC())
}

// FormatTime formats t (which the caller MUST pass in UTC) using TimestampFormat.
func FormatTime(t time.Time) string {
	return t.UTC().Format(TimestampFormat)
}

// ParseTimestamp parses a TimestampFormat string back into a time.Time.
func ParseTimestamp(s string) (time.Time, error) {
	return time.Parse(TimestampFormat, s)
}

// WithCollisionSuffix returns base if existsFn(base) is false; otherwise it appends "-1", "-2", ...
// until an unused name is produced. exists must be a side-effect-free check.
func WithCollisionSuffix(base string, exists func(string) bool) (string, error) {
	if exists == nil {
		return "", fmt.Errorf("WithCollisionSuffix: exists callback is required")
	}
	if !exists(base) {
		return base, nil
	}
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !exists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not resolve collision for %q after 1000 attempts", base)
}
