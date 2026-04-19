package clarification

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// frontmatter is the on-disk YAML header for a clarifications artifact.
// Field order is intentional and mirrors contracts/clarifications-artifact.md.
type frontmatter struct {
	FromCapture string    `yaml:"from_capture"`
	FromSpec    string    `yaml:"from_spec"`
	Status      string    `yaml:"status"`
	Mode        Mode      `yaml:"mode"`
	Strict      bool      `yaml:"strict"`
	ReusedFrom  string    `yaml:"reused_from"`
	CreatedAt   time.Time `yaml:"created_at"`
	ToolVersion string    `yaml:"tool_version"`
}

// Marshal serializes a Batch into the canonical markdown artifact format
// described by contracts/clarifications-artifact.md. Returns an error if the
// Batch violates any invariant from data-model.md.
func Marshal(b Batch) ([]byte, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	fm := frontmatter{
		FromCapture: b.FromCapture,
		FromSpec:    b.FromSpec,
		Status:      defaultIfBlank(b.Status, "draft"),
		Mode:        b.Mode,
		Strict:      b.Strict,
		ReusedFrom:  b.ReusedFrom,
		CreatedAt:   b.CreatedAt.UTC(),
		ToolVersion: b.ToolVersion,
	}
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("clarification: marshal frontmatter: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")
	buf.WriteString(fmt.Sprintf("# Plan-Stage Clarifications — %s\n\n", deriveSlug(b.FromCapture)))
	buf.WriteString(fmt.Sprintf("Source capture: `%s`\n", b.FromCapture))
	buf.WriteString(fmt.Sprintf("Source spec draft: `%s`\n\n", b.FromSpec))

	resolved, deferred, skipped, pending := partition(b)
	writeSection(&buf, "## Resolved", resolved, b)
	writeSection(&buf, "## Deferred (capped)", deferred, b)
	writeSection(&buf, "## Skipped (user-declined)", skipped, b)
	writeSection(&buf, "## Pending (non-interactive)", pending, b)

	return buf.Bytes(), nil
}

// Unmarshal parses a clarifications artifact back into a Batch. The parser is
// permissive about whitespace but strict about section headers and Q-block
// shape — the inverse of Marshal must round-trip cleanly.
func Unmarshal(data []byte) (Batch, error) {
	out := Batch{}
	body, fm, err := splitFrontmatter(data)
	if err != nil {
		return out, err
	}
	out.FromCapture = fm.FromCapture
	out.FromSpec = fm.FromSpec
	out.Status = fm.Status
	out.Mode = fm.Mode
	out.Strict = fm.Strict
	out.ReusedFrom = fm.ReusedFrom
	out.CreatedAt = fm.CreatedAt
	out.ToolVersion = fm.ToolVersion

	for _, block := range splitQBlocks(body) {
		q, r, err := parseQBlock(block)
		if err != nil {
			return out, err
		}
		out.Questions = append(out.Questions, q)
		out.Resolutions = append(out.Resolutions, r)
	}
	return out, nil
}

// validate enforces the data-model invariants before serialization. Errors
// here are programmer bugs, not user errors — the runner layer must build a
// well-formed Batch.
func (b *Batch) validate() error {
	if len(b.Resolutions) != len(b.Questions) {
		return fmt.Errorf("clarification: %d resolutions but %d questions",
			len(b.Resolutions), len(b.Questions))
	}
	seen := map[string]bool{}
	for _, q := range b.Questions {
		if q.ID == "" || q.TargetSection == "" {
			return errors.New("clarification: question id and target_section required")
		}
		if seen[q.ID] {
			return fmt.Errorf("clarification: duplicate question id %q", q.ID)
		}
		seen[q.ID] = true
		if q.Priority < 1 || q.Priority > 4 {
			return fmt.Errorf("clarification: question %s priority=%d (want 1..4)", q.ID, q.Priority)
		}
		if len(q.Options) < 3 || len(q.Options) > 4 {
			return fmt.Errorf("clarification: question %s has %d options (want 3..4)", q.ID, len(q.Options))
		}
	}
	resolvedIDs := map[string]bool{}
	for _, r := range b.Resolutions {
		if !seen[r.QuestionID] {
			return fmt.Errorf("clarification: resolution references unknown question %q", r.QuestionID)
		}
		if resolvedIDs[r.QuestionID] {
			return fmt.Errorf("clarification: duplicate resolution for question %q", r.QuestionID)
		}
		resolvedIDs[r.QuestionID] = true
	}
	answered := b.ResolvedCount()
	if answered > 3 {
		return fmt.Errorf("clarification: %d answered resolutions exceed cap of 3", answered)
	}
	if b.Mode == ModeNonInteractive {
		for _, r := range b.Resolutions {
			if r.Kind == KindChosen || r.Kind == KindCustom {
				continue // allowed via reuse
			}
			if r.Kind != KindPendingNonInteractive && r.Kind != KindDeferredByCap {
				return fmt.Errorf("clarification: non_interactive resolution %s has kind %q (must be pending_noninteractive or reused chosen/custom)",
					r.QuestionID, r.Kind)
			}
		}
	}
	return nil
}

func partition(b Batch) (resolved, deferred, skipped, pending []Question) {
	for _, q := range b.Questions {
		r := b.FindResolution(q.ID)
		if r == nil {
			continue
		}
		switch r.Kind {
		case KindChosen, KindCustom:
			resolved = append(resolved, q)
		case KindDeferredByCap:
			deferred = append(deferred, q)
		case KindSkipped:
			skipped = append(skipped, q)
		case KindPendingNonInteractive:
			pending = append(pending, q)
		}
	}
	for _, slice := range [][]Question{resolved, deferred, skipped, pending} {
		sort.SliceStable(slice, func(i, j int) bool { return slice[i].ID < slice[j].ID })
	}
	return
}

func writeSection(buf *bytes.Buffer, header string, qs []Question, b Batch) {
	buf.WriteString(header + "\n\n")
	if len(qs) == 0 {
		buf.WriteString("_none_\n\n")
		return
	}
	for _, q := range qs {
		writeQBlock(buf, q, b.FindResolution(q.ID))
	}
}

func writeQBlock(buf *bytes.Buffer, q Question, r *Resolution) {
	buf.WriteString(fmt.Sprintf("### %s — %s — Priority %d\n\n", q.ID, q.Category, q.Priority))
	buf.WriteString(fmt.Sprintf("**Target section**: `%s`\n\n", q.TargetSection))
	buf.WriteString("**Source excerpt**:\n\n")
	for _, line := range strings.Split(strings.TrimRight(q.SourceExcerpt, "\n"), "\n") {
		buf.WriteString("> " + line + "\n")
	}
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("**Question**: %s\n\n", q.Prompt))
	buf.WriteString("| Option | Answer | Implication |\n")
	buf.WriteString("|--------|--------|-------------|\n")
	for _, o := range q.Options {
		buf.WriteString(fmt.Sprintf("| %s | %s | %s |\n", o.Label, o.Text, o.Implication))
	}
	buf.WriteString("\n")

	resolutionLabel := "Pending"
	switch r.Kind {
	case KindChosen:
		resolutionLabel = r.ChosenLabel
	case KindCustom:
		resolutionLabel = "Custom"
	case KindSkipped:
		resolutionLabel = "Skipped"
	case KindDeferredByCap:
		resolutionLabel = "Deferred"
	case KindPendingNonInteractive:
		resolutionLabel = "Pending"
	}
	buf.WriteString(fmt.Sprintf("**Resolution**: %s\n", resolutionLabel))
	buf.WriteString(fmt.Sprintf("**Resolved at**: %s\n", r.ResolvedAt.UTC().Format(time.RFC3339)))
	if len(r.AppliedTo) > 0 {
		buf.WriteString(fmt.Sprintf("**Applied to**: `%s`\n", strings.Join(r.AppliedTo, "`, `")))
	} else {
		buf.WriteString("**Applied to**: _none_\n")
	}
	if r.Kind == KindCustom && r.CustomText != "" {
		buf.WriteString(fmt.Sprintf("\n**Custom answer**: %s\n", r.CustomText))
	}
	buf.WriteString("\n---\n\n")
}

func defaultIfBlank(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func deriveSlug(capturePath string) string {
	base := capturePath
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".md")
	return base
}

// splitFrontmatter peels off the leading `---\n...\n---\n` block.
func splitFrontmatter(data []byte) (body []byte, fm frontmatter, err error) {
	const sep = "---\n"
	if !bytes.HasPrefix(data, []byte(sep)) {
		return nil, fm, errors.New("clarification: missing frontmatter open")
	}
	rest := data[len(sep):]
	idx := bytes.Index(rest, []byte("\n"+sep))
	if idx < 0 {
		return nil, fm, errors.New("clarification: missing frontmatter close")
	}
	if err := yaml.Unmarshal(rest[:idx], &fm); err != nil {
		return nil, fm, fmt.Errorf("clarification: parse frontmatter: %w", err)
	}
	return rest[idx+len("\n"+sep):], fm, nil
}

// splitQBlocks returns each `### …` block found anywhere in the body, in
// document order. Section headers (`## Resolved`, etc.) are skipped because
// the Resolution.Kind embedded in each block disambiguates the section.
func splitQBlocks(body []byte) [][]byte {
	var blocks [][]byte
	lines := strings.Split(string(body), "\n")
	var cur []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		blocks = append(blocks, []byte(strings.Join(cur, "\n")))
		cur = nil
	}
	inBlock := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "### "):
			flush()
			inBlock = true
			cur = append(cur, line)
		case strings.HasPrefix(line, "## "):
			flush()
			inBlock = false
		case strings.HasPrefix(line, "---") && inBlock:
			flush()
			inBlock = false
		case inBlock:
			cur = append(cur, line)
		}
	}
	flush()
	return blocks
}

func parseQBlock(block []byte) (Question, Resolution, error) {
	var (
		q Question
		r Resolution
	)
	lines := strings.Split(string(block), "\n")
	if len(lines) == 0 {
		return q, r, errors.New("clarification: empty Q-block")
	}
	header := strings.TrimPrefix(lines[0], "### ")
	parts := strings.SplitN(header, " — ", 3)
	if len(parts) != 3 {
		return q, r, fmt.Errorf("clarification: malformed Q-header %q", header)
	}
	q.ID = parts[0]
	q.Category = Category(parts[1])
	if _, err := fmt.Sscanf(parts[2], "Priority %d", &q.Priority); err != nil {
		return q, r, fmt.Errorf("clarification: parse priority: %w", err)
	}

	r.QuestionID = q.ID
	mode := ""
	var excerptLines []string
	var optionRows []string
	for _, line := range lines[1:] {
		switch {
		case strings.HasPrefix(line, "**Target section**:"):
			mode = "target"
		case strings.HasPrefix(line, "**Source excerpt**:"):
			mode = "excerpt"
		case strings.HasPrefix(line, "**Question**:"):
			mode = ""
			q.Prompt = strings.TrimSpace(strings.TrimPrefix(line, "**Question**:"))
		case strings.HasPrefix(line, "| Option"), strings.HasPrefix(line, "|--------"):
			mode = ""
		case strings.HasPrefix(line, "| "):
			mode = ""
			optionRows = append(optionRows, line)
		case strings.HasPrefix(line, "**Resolution**:"):
			mode = ""
			label := strings.TrimSpace(strings.TrimPrefix(line, "**Resolution**:"))
			r.Kind, r.ChosenLabel = decodeResolutionLabel(label)
		case strings.HasPrefix(line, "**Resolved at**:"):
			mode = ""
			ts := strings.TrimSpace(strings.TrimPrefix(line, "**Resolved at**:"))
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				r.ResolvedAt = t
			}
		case strings.HasPrefix(line, "**Applied to**:"):
			mode = ""
			value := strings.TrimSpace(strings.TrimPrefix(line, "**Applied to**:"))
			if value != "_none_" {
				value = strings.Trim(value, "`")
				for _, p := range strings.Split(value, "`, `") {
					if p != "" {
						r.AppliedTo = append(r.AppliedTo, p)
					}
				}
			}
		case strings.HasPrefix(line, "**Custom answer**:"):
			mode = ""
			r.CustomText = strings.TrimSpace(strings.TrimPrefix(line, "**Custom answer**:"))
		default:
			switch mode {
			case "target":
				if v := strings.TrimSpace(line); v != "" {
					q.TargetSection = strings.Trim(v, "`")
				}
			case "excerpt":
				if strings.HasPrefix(line, "> ") {
					excerptLines = append(excerptLines, strings.TrimPrefix(line, "> "))
				}
			}
		}
	}
	q.SourceExcerpt = strings.Join(excerptLines, "\n")
	for _, row := range optionRows {
		cells := splitTableRow(row)
		if len(cells) < 3 {
			continue
		}
		q.Options = append(q.Options, Option{
			Label:       cells[0],
			Text:        cells[1],
			Implication: cells[2],
		})
	}
	return q, r, nil
}

func splitTableRow(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	parts := strings.Split(row, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func decodeResolutionLabel(label string) (ResolutionKind, string) {
	switch label {
	case "Skipped":
		return KindSkipped, ""
	case "Deferred":
		return KindDeferredByCap, ""
	case "Pending":
		return KindPendingNonInteractive, ""
	case "Custom":
		return KindCustom, ""
	case "A", "B", "C", "D":
		return KindChosen, label
	default:
		return KindPendingNonInteractive, ""
	}
}
