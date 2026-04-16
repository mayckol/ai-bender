---
name: fg-bootstrap-detect-cicd
context: fg
description: "Detect the CI provider and deployment targets."
provides: [detect, cicd]
stages: [bootstrap]
applies_to: [any]
---

# fg-bootstrap-detect-cicd

Look for .github/workflows/, .gitlab-ci.yml, .circleci/, Jenkinsfile, vercel.json, fly.toml, Procfile, k8s/Helm manifests. Update Build / CI section.
