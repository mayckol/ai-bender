package ui

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/mayckol/ai-bender/internal/catalog"
)

// FormInput carries everything the selection form needs to render: the live
// catalog and the baseline selection (either the catalog defaults for a
// fresh workspace or the persisted manifest for a re-run).
type FormInput struct {
	Catalog  *catalog.Catalog
	Baseline map[string]bool

	// BaselineOpenPR is the prior persisted value for the "open PR on
	// successful /ghu" preference. Feature 007. Defaults to false when the
	// project has no persisted preference yet.
	BaselineOpenPR bool
}

// FormOutput is the user's confirmed selection. Cancel returns an error
// (ErrCancelled) rather than a partial FormOutput — callers treat cancel
// as "do nothing, exit cleanly" via the os.Exit 130 contract.
type FormOutput struct {
	Selection map[string]bool

	// OpenPROnSuccess is the user's answer to the feature-007 PR confirm.
	// Callers persist it to .bender/selection.yaml via selection.Save.
	OpenPROnSuccess bool
}

// ErrCancelled signals the user hit Ctrl-C / Escape in the checkbox list.
// init.go translates this into exit code 130 and skips all writes.
var ErrCancelled = errors.New("ui: selection cancelled by user")

// Form is the contract the init command depends on. The concrete
// huh-backed implementation lives in this package; tests inject a
// scripted fake that returns a deterministic selection.
type Form interface {
	Run(FormInput) (FormOutput, error)
}

// NewForm returns the default huh-backed implementation.
func NewForm() Form { return &huhForm{} }

type huhForm struct{}

func (huhForm) Run(in FormInput) (FormOutput, error) {
	if in.Catalog == nil {
		return FormOutput{}, errors.New("ui: form requires a catalog")
	}

	optional := in.Catalog.OptionalIDs()
	var selected []string
	for _, id := range optional {
		if in.Baseline[id] {
			selected = append(selected, id)
		}
	}

	options := make([]huh.Option[string], 0, len(optional))
	for _, id := range optional {
		comp := in.Catalog.Components[id]
		label := fmt.Sprintf("%s — %s", id, comp.Description)
		options = append(options, huh.NewOption(label, id))
	}

	openPR := in.BaselineOpenPR

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Optional components").
				Description("Toggle which optional components to install. Mandatory agents and skills are always installed.").
				Options(options...).
				Value(&selected),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Open a pull request on successful /ghu runs?").
				Description("When enabled, bender opens a PR via the git-host adapter after every successful /ghu. Disabled: no change from today's manual `bender session pr` flow.").
				Value(&openPR),
		),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return FormOutput{}, ErrCancelled
		}
		return FormOutput{}, err
	}

	out := FormOutput{Selection: map[string]bool{}, OpenPROnSuccess: openPR}
	// Start from mandatory-always-true; optional-false by default, flipped
	// on for ids the user checked.
	for id, comp := range in.Catalog.Components {
		out.Selection[id] = !comp.Optional
	}
	for _, id := range selected {
		out.Selection[id] = true
	}
	return out, nil
}
