package artifact

import (
	"strings"
	"testing"
)

func TestSlug_Basic(t *testing.T) {
	cases := map[string]string{
		"users need roles":             "users-need-roles",
		"  Users Need Roles!  ":        "users-need-roles",
		"Add OAuth2 / API integration": "add-oauth2-api-integration",
		"":                             "untitled",
		"!!!":                          "untitled",
		"already-kebab-case":           "already-kebab-case",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q): got %q want %q", in, got, want)
		}
	}
}

func TestSlug_BoundedLength(t *testing.T) {
	long := strings.Repeat("abc-", 200)
	got := Slug(long)
	if len(got) > 80 {
		t.Fatalf("Slug exceeded 80 chars: %d", len(got))
	}
}

func TestSlug_ProducesSafeFilenameComponent(t *testing.T) {
	got := Slug("Add user roles, with OAuth2 + API")
	if err := ValidateFilename(got + ".md"); err != nil {
		t.Fatalf("Slug produced an unsafe filename: %v", err)
	}
}
