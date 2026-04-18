// Package ui owns the interactive terminal-facing components of the bender
// CLI. Today that is the checkbox selection form used by `bender init` when
// invoked in a TTY without `--with` / `--without` flags; the Form interface
// isolates the TUI dependency from the command layer so tests can supply a
// scripted fake backend.
package ui
