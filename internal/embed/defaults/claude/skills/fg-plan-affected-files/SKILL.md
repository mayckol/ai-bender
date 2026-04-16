---
name: fg-plan-affected-files
context: fg
description: "List the files each task is expected to touch (informs scout caching and write-conflict prevention)."
provides: [plan, files]
stages: [plan]
applies_to: [any]
---

# fg-plan-affected-files

Per task, list affected_files as glob patterns. Include both existing files to modify and new files to create.
