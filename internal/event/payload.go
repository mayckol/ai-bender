package event

import (
	"fmt"
	"sort"
	"strings"
)

// requiredPayloadFields lists the payload keys that each v1 event type MUST carry.
// Kept in one place so contracts/event-schema.md, skills/*/SKILL.md, and this validator
// stay in sync.
var requiredPayloadFields = map[Type][]string{
	TypeSessionStarted:   {"command", "invoker", "working_dir"},
	TypeStageStarted:     {"stage", "inputs"},
	TypeStageCompleted:   {"stage", "outputs"},
	TypeStageFailed:      {"stage", "error"},
	TypeOrchDecision:     {"decision_type"},
	TypeOrchProgress:     {"percent", "current_step"},
	TypeAgentStarted:     {"agent", "task_ids"},
	TypeAgentCompleted:   {"agent", "task_ids", "duration_ms"},
	TypeAgentFailed:      {"agent", "task_ids", "error"},
	TypeAgentBlocked:     {"agent", "task_ids", "error"},
	TypeAgentProgress:    {"agent", "percent", "current_step"},
	TypeSkillInvoked:     {"skill", "agent"},
	TypeSkillCompleted:   {"skill", "agent", "duration_ms"},
	TypeSkillFailed:      {"skill", "agent", "error"},
	TypeArtifactWritten:  {"path", "stage", "checksum", "bytes"},
	TypeFileChanged:      {"path", "lines_added", "lines_removed", "agent"},
	TypeFindingReported:  {"finding_id", "severity", "category", "title"},
	TypeSessionCompleted: {"status", "duration_ms"},

	TypeWorktreeCreated: {"path", "branch", "base_branch", "base_sha"},
	TypeWorktreeRemoved: {"path", "reason"},
	TypeWorktreeMissing: {"path"},
	TypePROpened:        {"remote", "branch_on_remote", "pr_url", "adapter", "opened_at"},
	TypePRUpdateRefused: {"existing_pr_url"},

	TypeClarificationsRequested: {"artifact_path", "question_count", "resolved_count", "pending_count", "skipped_count", "deferred_count"},
	TypeClarificationsResolved:  {"artifact_path", "question_count", "resolved_count", "pending_count", "skipped_count", "deferred_count"},
	TypeClarificationsPending:   {"artifact_path", "question_count", "resolved_count", "pending_count", "skipped_count", "deferred_count"},
}

// RequiredPayloadFields returns the payload keys that v1 requires for the given event type.
// Unknown types return nil (envelope validation catches them separately).
func RequiredPayloadFields(t Type) []string {
	out, ok := requiredPayloadFields[t]
	if !ok {
		return nil
	}
	cp := make([]string, len(out))
	copy(cp, out)
	return cp
}

// ValidatePayload checks that the event's payload carries every field required for its
// type (per contracts/event-schema.md). Missing-field errors are returned together in
// one message so the caller can fix them in a single pass.
//
// A nil or empty payload on a type with required fields is an error; unknown event
// types are tolerated here because Validate() already reports them via ErrUnknownType.
func (e *Event) ValidatePayload() error {
	required := RequiredPayloadFields(e.Type)
	if len(required) == 0 {
		return nil
	}
	var missing []string
	for _, key := range required {
		v, ok := e.Payload[key]
		if !ok || isZeroValue(v) {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("event %s: payload missing required field(s): %s", e.Type, strings.Join(missing, ", "))
}

// isZeroValue returns true for nil and the empty string so that a present-but-blank
// scalar counts as missing. Empty slices/maps and numeric zero are accepted — an
// empty `inputs: []` on a stage that legitimately has no upstream artifacts is a
// valid value, and `lines_removed: 0` is a legitimate line count.
func isZeroValue(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	return false
}
