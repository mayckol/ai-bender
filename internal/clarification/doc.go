// Package clarification persists, reuses, prompts for, and emits events about
// plan-stage clarifications: the structured ambiguity questions presented to a
// practitioner mid-/plan, mirroring the spec-kit /clarify UX. Detection itself
// stays in the plan skill prompt; this package owns the artifact format
// (.bender/artifacts/plan/clarifications-<timestamp>.md), the reuse-match
// against prior runs, the huh-driven interactive picker, the non-interactive
// fallback, and the new clarifications_requested / clarifications_resolved /
// clarifications_pending event payloads.
package clarification
