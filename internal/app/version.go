package app

import (
	"fmt"

	"github.com/danielhanold/docket/internal/buildinfo"
)

// VersionResult reports injected build identity.
type VersionResult struct {
	Envelope
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// Version is the `docket version` operation.
func Version(info buildinfo.Info) VersionResult {
	return VersionResult{
		Envelope:  NewEnvelope("version", ResultApplied),
		Version:   info.Version,
		Commit:    info.Commit,
		BuildDate: info.BuildDate,
	}
}

// HumanText renders the one-line default text form.
func (r VersionResult) HumanText() string {
	return fmt.Sprintf("docket %s (commit %s, built %s)", r.Version, r.Commit, r.BuildDate)
}
