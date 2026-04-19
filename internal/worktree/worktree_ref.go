package worktree

// This file intentionally minimal — keeps a stable symbol the integration
// test imports to prove the package compiles end-to-end even when the test
// only cares about a small subset of the surface.

// PackageVersion is bumped when the worktree package's public contract
// changes. Integration tests pin to this value so a renamed/removed symbol
// fails loudly instead of silently skipping tests.
const PackageVersion = "1.0.0"
