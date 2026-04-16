---
name: fg-plan-decompose-tasks
context: fg
description: "Decompose the spec into a numbered, dependency-ordered task list."
provides: [plan, decompose]
stages: [plan]
applies_to: [any]
---

# fg-plan-decompose-tasks

Produce `artifacts/plan/tasks-<ts>.md` with T001+ tasks, agent_hints, depends_on (no cycles), affected_files, acceptance criteria.
