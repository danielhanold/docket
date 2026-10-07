// The interactive harness picker `repository init` and `repository
// configure-harnesses` show on a terminal. This file is docket's only importer
// of github.com/charmbracelet/huh: internal/app sees only the app.HarnessChooser
// seam, so the terminal UI dependency never reaches the service layer.
package cli

import (
	"context"
	"errors"

	"github.com/charmbracelet/huh"

	"github.com/danielhanold/docket/internal/app"
)

// runHarnessForm runs the picker form. It is a seam so a test can stand in for
// the terminal.
var runHarnessForm = func(ctx context.Context, f *huh.Form) error { return f.RunWithContext(ctx) }

// newHarnessForm builds a checklist of every harness in req.Options order with
// req.Preselected pre-checked (huh selects the options already in *picked).
func newHarnessForm(req app.HarnessChoiceRequest, picked *[]string) *huh.Form {
	*picked = append([]string{}, req.Preselected...)
	field := huh.NewMultiSelect[string]().
		Title("Which coding agents should get docket's instructions in this repository?").
		Description("Space toggles, Enter confirms. Written to " + req.ConfigPath + "; confirm with nothing checked to record that no agents are used.").
		Value(picked).
		Options(huh.NewOptions(req.Options...)...)
	return huh.NewForm(huh.NewGroup(field))
}

// huhHarnessChooser is the production app.HarnessChooser: it shows the picker
// and maps a user abort (Ctrl-C or Esc) to app.ErrHarnessChoiceCancelled. A
// confirmed empty checklist is a non-nil empty selection, which records that no
// agents are used.
func huhHarnessChooser(ctx context.Context, req app.HarnessChoiceRequest) ([]string, error) {
	var picked []string
	err := runHarnessForm(ctx, newHarnessForm(req, &picked))
	if errors.Is(err, huh.ErrUserAborted) {
		return nil, app.ErrHarnessChoiceCancelled
	}
	if err != nil {
		return nil, err
	}
	if picked == nil {
		picked = []string{}
	}
	return picked, nil
}
