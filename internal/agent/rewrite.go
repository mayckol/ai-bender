package agent

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/mayckol/ai-bender/internal/config"
)

// RewriteFile parses the agent file at path, applies ov to the frontmatter,
// and writes the result back if it would change. The body (everything after
// the closing `---`) is preserved byte-for-byte. Returns changed=true when
// the file contents differ from what was on disk.
//
// Intended for `bender apply-config`: idempotent, so re-running with the
// same config produces no diff on the second pass.
func RewriteFile(path string, ov config.AgentOverride) (bool, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("agent rewrite: read %s: %w", path, err)
	}
	a, err := ParseAgent(original)
	if err != nil {
		return false, fmt.Errorf("agent rewrite: parse %s: %w", path, err)
	}

	ApplyOverride(a, ov)

	rendered, err := renderAgentFile(&a.Frontmatter, a.Body)
	if err != nil {
		return false, fmt.Errorf("agent rewrite: render %s: %w", path, err)
	}
	if bytes.Equal(original, rendered) {
		return false, nil
	}
	if err := os.WriteFile(path, rendered, 0o644); err != nil {
		return false, fmt.Errorf("agent rewrite: write %s: %w", path, err)
	}
	return true, nil
}

// renderAgentFile produces the `---\n<yaml>\n---\n<body>` bytes for an agent.
// The YAML uses 2-space indent (yaml.v3 default) and preserves struct field
// order, which matches the shape a user would hand-author.
func renderAgentFile(fm *Frontmatter, body string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(fm); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}

	buf.WriteString("---\n")
	buf.WriteString(body)
	return buf.Bytes(), nil
}
