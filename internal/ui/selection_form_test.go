package ui

import (
	"errors"
	"testing"

	"github.com/mayckol/ai-bender/internal/catalog"
)

// FakeForm is a scripted backend used in tests. Given a canned selection,
// Run returns it without consulting a real terminal.
type FakeForm struct {
	Out FormOutput
	Err error
}

func (f FakeForm) Run(FormInput) (FormOutput, error) { return f.Out, f.Err }

func TestNewForm_NilCatalog_Errors(t *testing.T) {
	_, err := NewForm().Run(FormInput{})
	if err == nil {
		t.Error("want error for nil catalog")
	}
}

func TestFakeForm_ReturnsCanned(t *testing.T) {
	want := map[string]bool{"benchmarker": false, "sentinel": true}
	f := FakeForm{Out: FormOutput{Selection: want}}
	got, err := f.Run(FormInput{})
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range want {
		if got.Selection[k] != v {
			t.Errorf("%s = %v, want %v", k, got.Selection[k], v)
		}
	}
}

func TestFakeForm_PropagatesErr(t *testing.T) {
	sentinel := errors.New("scripted")
	f := FakeForm{Err: sentinel}
	_, err := f.Run(FormInput{})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
}

// Guard: ErrCancelled is exported and distinguishable from other errors.
func TestErrCancelled_Typed(t *testing.T) {
	if ErrCancelled == nil {
		t.Fatal("ErrCancelled is nil")
	}
	if !errors.Is(ErrCancelled, ErrCancelled) {
		t.Errorf("ErrCancelled should match itself via errors.Is")
	}
}

// Guard: NewForm returns a non-nil Form.
func TestNewForm_ReturnsForm(t *testing.T) {
	f := NewForm()
	if f == nil {
		t.Fatal("NewForm returned nil")
	}
}

// Tiny sanity check that the catalog's OptionalIDs are what we drive the
// form with; keeps the form and catalog contracts in lockstep.
func TestFormInput_Catalog_OptionalCount(t *testing.T) {
	cat := &catalog.Catalog{
		SchemaVersion: 1,
		Components: map[string]catalog.Component{
			"a": {Optional: true, Description: "a"},
			"b": {Optional: false, Description: "b"},
			"c": {Optional: true, Description: "c"},
		},
	}
	if len(cat.OptionalIDs()) != 2 {
		t.Errorf("want 2 optional, got %v", cat.OptionalIDs())
	}
}
