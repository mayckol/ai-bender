---
name: bg-reviewer-pr-summary
context: bg
description: "Generate a PR description for the changes."
provides: [review, pr-summary]
stages: [ghu, implement]
applies_to: [any]
---

# bg-reviewer-pr-summary

Sections: Summary, Why, Changes, Tests, Findings (severity ≥ medium). Write to artifacts/ghu/reviews/<ts>/pr-summary.md.
