// Package mistakes owns reading and appending entries in the Mistakes
// Artifact (`.bender/artifacts/mistakes.md`). The file is append-only; each
// entry is a Markdown block with YAML frontmatter. Consumers include the
// bg-mistakeinator-record skill (for writes) and any planning-related
// skill (for reads) that surfaces scope-matching guidance to the model.
package mistakes

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultArtifactPath is where the Mistakes Artifact lives relative to the
// workspace root.
const DefaultArtifactPath = ".bender/artifacts/mistakes.md"

// Entry is one recorded mistake. `Prefer` may be empty.
type Entry struct {
	ID      string    `yaml:"id"`
	Scope   string    `yaml:"scope"`
	Tags    []string  `yaml:"tags,omitempty"`
	Created time.Time `yaml:"created"`
	Title   string    `yaml:"-"`
	Avoid   string    `yaml:"-"`
	Prefer  string    `yaml:"-"`
}

// Read parses the artifact at path and returns the list of entries in file
// order. Malformed frontmatter anywhere in the file causes the whole file
// to be rejected — callers see an empty list (absent file returns
// (nil, nil) so callers can treat "no artifact" and "no entries" uniformly).
func Read(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return parse(data)
}

// Append writes a new entry to the artifact, creating the file and parent
// directory if necessary. Dedupes by `id`: if an entry with the same id
// already exists, the file is not modified and `dup` is true.
func Append(path string, e Entry) (dup bool, err error) {
	existing, err := Read(path)
	if err != nil {
		return false, err
	}
	for _, prev := range existing {
		if prev.ID == e.ID {
			return true, nil
		}
	}
	if e.Created.IsZero() {
		e.Created = time.Now().UTC()
	}
	block, err := format(e)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.Write(block); err != nil {
		return false, err
	}
	return false, nil
}

// Filter returns entries whose `Scope` is reachable from the change set.
// An entry matches if any of the following holds:
//  1. Its scope is an exact match for any changed path.
//  2. Its scope is a directory prefix of any changed path (entry scope =
//     "internal/userservice" matches changed path "internal/userservice/x.go").
//  3. Its scope is prefixed "tag:" and the bare tag appears in `tags`.
func Filter(entries []Entry, changedPaths []string, tags []string) []Entry {
	tagSet := map[string]struct{}{}
	for _, t := range tags {
		tagSet[t] = struct{}{}
	}
	var out []Entry
	for _, e := range entries {
		if strings.HasPrefix(e.Scope, "tag:") {
			if _, ok := tagSet[strings.TrimPrefix(e.Scope, "tag:")]; ok {
				out = append(out, e)
			}
			continue
		}
		for _, p := range changedPaths {
			if p == e.Scope || strings.HasPrefix(p, e.Scope+"/") || strings.HasPrefix(p, e.Scope+string(os.PathSeparator)) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// parse scans `data` for `---` frontmatter blocks and collects the trailing
// Markdown body. Invalid frontmatter is treated as a fatal parse error for
// the whole file.
func parse(data []byte) ([]Entry, error) {
	var out []Entry
	// Split on lines that contain exactly "---".
	sep := []byte("\n---\n")
	// Prepend a newline so a leading "---" at byte 0 is still a delimiter.
	buf := append([]byte("\n"), data...)
	chunks := bytes.Split(buf, sep)
	if len(chunks) < 3 {
		if bytes.TrimSpace(data) == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("mistakes: malformed artifact (no frontmatter blocks)")
	}
	// chunks[0] is pre-frontmatter (empty for a well-formed file); pairs of
	// subsequent chunks are (frontmatter, body) … unless the file ends with
	// a frontmatter that has no body.
	for i := 1; i+1 < len(chunks); i += 2 {
		fm := chunks[i]
		body := chunks[i+1]
		var e Entry
		if err := yaml.Unmarshal(fm, &e); err != nil {
			return nil, fmt.Errorf("mistakes: parse frontmatter: %w", err)
		}
		title, avoid, prefer := splitBody(body)
		e.Title = title
		e.Avoid = avoid
		e.Prefer = prefer
		if e.ID == "" {
			return nil, fmt.Errorf("mistakes: entry missing id")
		}
		out = append(out, e)
	}
	return out, nil
}

func splitBody(body []byte) (title, avoid, prefer string) {
	for _, line := range strings.Split(string(body), "\n") {
		switch {
		case title == "" && strings.HasPrefix(line, "## "):
			title = strings.TrimPrefix(line, "## ")
		case strings.HasPrefix(line, "**Avoid:**"):
			avoid = strings.TrimSpace(strings.TrimPrefix(line, "**Avoid:**"))
		case strings.HasPrefix(line, "**Prefer:**"):
			prefer = strings.TrimSpace(strings.TrimPrefix(line, "**Prefer:**"))
		}
	}
	return
}

func format(e Entry) ([]byte, error) {
	fm, err := yaml.Marshal(struct {
		ID      string    `yaml:"id"`
		Scope   string    `yaml:"scope"`
		Tags    []string  `yaml:"tags,omitempty"`
		Created time.Time `yaml:"created"`
	}{e.ID, e.Scope, e.Tags, e.Created})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("\n---\n")
	buf.Write(fm)
	buf.WriteString("---\n\n")
	if e.Title != "" {
		fmt.Fprintf(&buf, "## %s\n\n", e.Title)
	}
	fmt.Fprintf(&buf, "**Avoid:** %s\n", e.Avoid)
	if e.Prefer != "" {
		fmt.Fprintf(&buf, "\n**Prefer:** %s\n", e.Prefer)
	}
	return buf.Bytes(), nil
}
