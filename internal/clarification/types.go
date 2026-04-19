package clarification

import "time"

// Category enumerates the fixed taxonomy used to prioritize clarifications
// (scope > security/privacy > UX > technical detail). The order in
// orderedCategories below mirrors the priority ranking used by Question.Priority.
type Category string

const (
	CategoryScope              Category = "scope"
	CategoryActorsRoles        Category = "actors_roles"
	CategoryDataModel          Category = "data_model"
	CategoryNonFunctional      Category = "non_functional"
	CategorySecurityPrivacy    Category = "security_privacy"
	CategoryIntegrationBoundary Category = "integration_boundary"
)

// Mode distinguishes interactive prompting from automation-friendly emission.
type Mode string

const (
	ModeInteractive    Mode = "interactive"
	ModeNonInteractive Mode = "non_interactive"
)

// ResolutionKind enumerates the terminal states a Question can land in.
// chosen / custom are reusable across runs; skipped, deferred_by_cap and
// pending_noninteractive are intentionally re-prompted on the next run.
type ResolutionKind string

const (
	KindChosen                ResolutionKind = "chosen"
	KindCustom                ResolutionKind = "custom"
	KindSkipped               ResolutionKind = "skipped"
	KindDeferredByCap         ResolutionKind = "deferred_by_cap"
	KindPendingNonInteractive ResolutionKind = "pending_noninteractive"
)

// Option is one suggested answer for a Question.
type Option struct {
	Label       string `yaml:"label" json:"label"`
	Text        string `yaml:"text" json:"text"`
	Implication string `yaml:"implication" json:"implication"`
}

// Question is a single detected ambiguity awaiting a practitioner decision.
type Question struct {
	ID            string   `yaml:"id" json:"id"`
	TargetSection string   `yaml:"target_section" json:"target_section"`
	Category      Category `yaml:"category" json:"category"`
	Priority      int      `yaml:"priority" json:"priority"`
	Prompt        string   `yaml:"prompt" json:"prompt"`
	Options       []Option `yaml:"options" json:"options"`
	SourceExcerpt string   `yaml:"source_excerpt" json:"source_excerpt"`
}

// Resolution is the practitioner's response to a Question.
type Resolution struct {
	QuestionID  string         `yaml:"question_id" json:"question_id"`
	Kind        ResolutionKind `yaml:"kind" json:"kind"`
	ChosenLabel string         `yaml:"chosen_label,omitempty" json:"chosen_label,omitempty"`
	CustomText  string         `yaml:"custom_text,omitempty" json:"custom_text,omitempty"`
	ResolvedAt  time.Time      `yaml:"resolved_at" json:"resolved_at"`
	AppliedTo   []string       `yaml:"applied_to,omitempty" json:"applied_to,omitempty"`
}

// Batch is the full set of questions and their resolutions for one /plan run.
type Batch struct {
	Timestamp   string       `yaml:"timestamp" json:"timestamp"`
	FromCapture string       `yaml:"from_capture" json:"from_capture"`
	FromSpec    string       `yaml:"from_spec" json:"from_spec"`
	Mode        Mode         `yaml:"mode" json:"mode"`
	Strict      bool         `yaml:"strict" json:"strict"`
	ReusedFrom  string       `yaml:"reused_from,omitempty" json:"reused_from,omitempty"`
	CreatedAt   time.Time    `yaml:"created_at" json:"created_at"`
	ToolVersion string       `yaml:"tool_version" json:"tool_version"`
	Status      string       `yaml:"status" json:"status"`
	Questions   []Question   `yaml:"questions" json:"questions"`
	Resolutions []Resolution `yaml:"resolutions" json:"resolutions"`
}

// FindResolution returns the Resolution for the given Question ID, or nil if
// none exists. Callers use this to test reuse and apply-to-spec lookups.
func (b *Batch) FindResolution(questionID string) *Resolution {
	for i := range b.Resolutions {
		if b.Resolutions[i].QuestionID == questionID {
			return &b.Resolutions[i]
		}
	}
	return nil
}

// ResolvedCount returns the number of Resolutions in a terminal answered state
// (chosen or custom). Used by event payload counters.
func (b *Batch) ResolvedCount() int {
	return b.countByKind(KindChosen) + b.countByKind(KindCustom)
}

// PendingCount returns the count of pending_noninteractive resolutions.
func (b *Batch) PendingCount() int { return b.countByKind(KindPendingNonInteractive) }

// SkippedCount returns the count of explicitly skipped resolutions.
func (b *Batch) SkippedCount() int { return b.countByKind(KindSkipped) }

// DeferredCount returns the count of resolutions deferred by the question cap.
func (b *Batch) DeferredCount() int { return b.countByKind(KindDeferredByCap) }

func (b *Batch) countByKind(k ResolutionKind) int {
	n := 0
	for i := range b.Resolutions {
		if b.Resolutions[i].Kind == k {
			n++
		}
	}
	return n
}
