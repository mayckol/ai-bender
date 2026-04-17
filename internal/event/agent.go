package event

// ResponsibleAgent returns the canonical agent name for an event.
//
// The mental model is simple: during inline stages (/cry, /plan, /tdd,
// /bender-bootstrap, /ghu --inline) all work happens in the "main" conversation,
// and during /ghu --bg the orchestrator delegates to specific worker agents
// (crafter, tester, reviewer, …). We want the viewer to badge events as
// "main" when there is no specific worker agent responsible, and use the
// worker name otherwise.
//
// Precedence:
//  1. payload.agent             — required on most v1 events during agent work.
//  2. payload.dispatched_agent  — set on orchestrator_decision events whose
//                                  decision targets a specific agent.
//  3. actor.name when actor.kind == "agent".
//  4. "main" for every other actor kind (orchestrator, stage, user, sink) —
//     unnamed or inline work is attributed to the main conversation.
func ResponsibleAgent(e *Event) string {
	if e == nil {
		return ""
	}
	if agent, ok := stringField(e.Payload, "agent"); ok {
		return agent
	}
	if agent, ok := stringField(e.Payload, "dispatched_agent"); ok {
		return agent
	}
	if e.Actor.Kind == ActorAgent && e.Actor.Name != "" {
		return e.Actor.Name
	}
	return "main"
}

// SkillName extracts the skill name from an event if this event is a skill_*
// event (skill_invoked, skill_completed, skill_failed). Returns "" otherwise.
func SkillName(e *Event) string {
	if e == nil {
		return ""
	}
	switch e.Type {
	case TypeSkillInvoked, TypeSkillCompleted, TypeSkillFailed:
		if s, ok := stringField(e.Payload, "skill"); ok {
			return s
		}
	}
	return ""
}

func stringField(payload map[string]any, key string) (string, bool) {
	if payload == nil {
		return "", false
	}
	v, ok := payload[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}
