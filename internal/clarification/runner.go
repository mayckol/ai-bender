package clarification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mayckol/ai-bender/internal/event"
)

// Options controls a single Run invocation. Constructed by the Cobra
// subcommand wrapper from CLI flags / env vars.
type Options struct {
	NonInteractive bool
	Strict         bool
	FromSpec       string
	FromCapture    string
	Timestamp      string
	OutPath        string
	SessionDir     string
	InputPath      string
	ToolVersion    string

	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	Prompter Prompter
}

// Run is the entry point used by `bender clarify`. It reads a Batch JSON
// document (from stdin or InputPath) describing detected ambiguities, applies
// reuse, prompts the practitioner (or fills pending entries non-interactively),
// persists the clarifications artifact, applies resolutions back into the
// spec draft, and emits the new clarifications_* events.
//
// Returns nil on success, an error on misuse / IO failure. In non-interactive
// strict mode with at least one pending Resolution, Run returns ErrPending so
// the Cobra wrapper can exit non-zero.
func Run(ctx context.Context, opts Options) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Prompter == nil {
		opts.Prompter = HuhPrompter{}
	}
	if opts.Timestamp == "" {
		return errors.New("clarification: --timestamp required")
	}
	if opts.FromSpec == "" || opts.FromCapture == "" {
		return errors.New("clarification: --from-spec and --from-capture required")
	}
	if opts.OutPath == "" {
		opts.OutPath = ArtifactPath(opts.Timestamp)
	}

	batch, err := loadInputBatch(opts)
	if err != nil {
		return err
	}
	batch.Timestamp = opts.Timestamp
	batch.FromCapture = opts.FromCapture
	batch.FromSpec = opts.FromSpec
	batch.ToolVersion = opts.ToolVersion
	batch.CreatedAt = time.Now().UTC()
	batch.Status = "draft"
	if opts.NonInteractive {
		batch.Mode = ModeNonInteractive
		batch.Strict = opts.Strict
	} else {
		batch.Mode = ModeInteractive
		batch.Strict = false
	}

	prior, _ := LookupReusable(filepath.Dir(opts.OutPath), opts.FromCapture)
	if len(prior.Resolutions) > 0 {
		batch = MergeReuse(batch, prior)
	}

	freshlyAsked := freshQuestionIDs(batch)

	switch batch.Mode {
	case ModeInteractive:
		if len(freshlyAsked) > 0 {
			if err := emitEvent(opts.SessionDir, event.TypeClarificationsRequested, batch, opts.OutPath); err != nil {
				return err
			}
			batch, err = RunInteractive(ctx, batch, opts.Prompter)
			if err != nil {
				return finalizeAwaitingClarification(opts, batch, err)
			}
		}
	case ModeNonInteractive:
		if len(freshlyAsked) > 0 {
			batch = fillPending(batch)
		}
	}

	if err := persistArtifact(opts.OutPath, batch); err != nil {
		return err
	}

	if err := applyToSpec(opts.FromSpec, &batch); err != nil {
		return err
	}

	if err := persistArtifact(opts.OutPath, batch); err != nil {
		return err
	}

	switch batch.Mode {
	case ModeInteractive:
		if len(freshlyAsked) > 0 {
			if err := emitEvent(opts.SessionDir, event.TypeClarificationsResolved, batch, opts.OutPath); err != nil {
				return err
			}
		}
	case ModeNonInteractive:
		if len(freshlyAsked) > 0 {
			if err := emitEvent(opts.SessionDir, event.TypeClarificationsPending, batch, opts.OutPath); err != nil {
				return err
			}
			if opts.Strict {
				if err := setSessionAwaitingClarification(opts.SessionDir, opts.OutPath); err != nil {
					return err
				}
				return ErrPending
			}
		}
	}
	return nil
}

// ErrPending is returned by Run when non-interactive strict mode finished with
// at least one pending Resolution. The Cobra wrapper exits with status 2.
var ErrPending = errors.New("clarification: pending resolutions in strict mode")

func loadInputBatch(opts Options) (Batch, error) {
	var src io.Reader = opts.Stdin
	if opts.InputPath != "" {
		f, err := os.Open(opts.InputPath)
		if err != nil {
			return Batch{}, fmt.Errorf("clarification: open input: %w", err)
		}
		defer f.Close()
		src = f
	}
	data, err := io.ReadAll(src)
	if err != nil {
		return Batch{}, fmt.Errorf("clarification: read input: %w", err)
	}
	var b Batch
	if err := json.Unmarshal(data, &b); err != nil {
		return Batch{}, fmt.Errorf("clarification: parse input JSON: %w", err)
	}
	return b, nil
}

func freshQuestionIDs(b Batch) []string {
	answered := map[string]bool{}
	for _, r := range b.Resolutions {
		if r.Kind == KindChosen || r.Kind == KindCustom {
			answered[r.QuestionID] = true
		}
	}
	var out []string
	for _, q := range b.Questions {
		if !answered[q.ID] {
			out = append(out, q.ID)
		}
	}
	return out
}

func fillPending(b Batch) Batch {
	have := map[string]bool{}
	for _, r := range b.Resolutions {
		have[r.QuestionID] = true
	}
	now := time.Now().UTC()
	for _, q := range b.Questions {
		if !have[q.ID] {
			b.Resolutions = append(b.Resolutions, Resolution{
				QuestionID: q.ID,
				Kind:       KindPendingNonInteractive,
				ResolvedAt: now,
			})
		}
	}
	return b
}

func persistArtifact(path string, b Batch) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("clarification: mkdir artifact dir: %w", err)
	}
	data, err := Marshal(b)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("clarification: write artifact: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("clarification: rename artifact: %w", err)
	}
	return nil
}

func applyToSpec(specPath string, b *Batch) error {
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("clarification: read spec: %w", err)
	}
	rewritten, applied, _, err := Apply(specBytes, specPath, *b)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		return nil
	}
	if err := os.WriteFile(specPath, rewritten, 0o644); err != nil {
		return fmt.Errorf("clarification: write spec: %w", err)
	}
	for i := range b.Resolutions {
		switch b.Resolutions[i].Kind {
		case KindChosen, KindCustom:
			b.Resolutions[i].AppliedTo = applied
		}
	}
	return nil
}

func emitEvent(sessionDir string, kind event.Type, b Batch, artifactPath string) error {
	if sessionDir == "" {
		return nil
	}
	ev, err := BuildEvent(kind, b, filepath.Base(sessionDir), artifactPath)
	if err != nil {
		return err
	}
	line, err := ev.MarshalJSONLine()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return fmt.Errorf("clarification: mkdir session: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(sessionDir, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("clarification: open events.jsonl: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("clarification: append event: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("clarification: sync events.jsonl: %w", err)
	}
	return nil
}

func finalizeAwaitingClarification(opts Options, b Batch, cause error) error {
	_ = persistArtifact(opts.OutPath, b)
	_ = setSessionAwaitingClarification(opts.SessionDir, opts.OutPath)
	return cause
}

// setSessionAwaitingClarification updates state.json (if present) to mark the
// session as awaiting clarification answers and points at the partial artifact.
func setSessionAwaitingClarification(sessionDir, artifactPath string) error {
	if sessionDir == "" {
		return nil
	}
	statePath := filepath.Join(sessionDir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("clarification: read state: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("clarification: parse state: %w", err)
	}
	raw["status"] = "awaiting_clarification"
	raw["clarifications_artifact"] = artifactPath
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	tmp := statePath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, statePath)
}
