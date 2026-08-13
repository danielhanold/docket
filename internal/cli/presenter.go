package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/danielhanold/docket/internal/app"
)

// Presenter performs the sole protocol write. An operation computes its
// complete result before presentation, so a failure cannot leave partial
// stdout behind.
type Presenter struct {
	Stdout io.Writer
	Stderr io.Writer
	JSON   bool
}

// Present renders one fully-computed result and returns its exit code.
// JSON mode: one compact document plus one newline on stdout, nothing on
// stderr. Human mode: the result's text on stdout.
func (p Presenter) Present(r app.OperationResult) int {
	if p.JSON {
		buf, err := json.Marshal(r)
		if err != nil {
			// Marshal of our own typed structs cannot fail on real inputs;
			// this is a genuinely unexpected diagnostic, so stderr-only.
			fmt.Fprintf(p.Stderr, "docket: internal error: %v\n", err)
			return app.ExitCode(app.ResultInternalError)
		}
		fmt.Fprintf(p.Stdout, "%s\n", buf)
		return app.ExitCode(r.Env().Result)
	}
	fmt.Fprintln(p.Stdout, r.HumanText())
	return app.ExitCode(r.Env().Result)
}

// PresentHumanError routes a handled human-mode CLI failure to stderr and
// leaves stdout empty.
func (p Presenter) PresentHumanError(r app.CLIErrorResult) int {
	fmt.Fprintln(p.Stderr, r.HumanText())
	return app.ExitCode(r.Env().Result)
}
