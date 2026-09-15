package app

import (
	"fmt"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
)

// VersionResult reports injected build identity.
type VersionResult struct {
	Envelope
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuildDate  string `json:"build_date"`
	AssetSetID string `json:"asset_set_id"`
}

// Version is the `docket version` operation.
func Version(info buildinfo.Info) VersionResult {
	manifest, _ := assets.EmbeddedManifest()
	return VersionResult{
		Envelope:   NewEnvelope("version", ResultApplied),
		Version:    info.Version,
		Commit:     info.Commit,
		BuildDate:  info.BuildDate,
		AssetSetID: manifest.AssetSetID,
	}
}

// HumanText renders the one-line default text form.
func (r VersionResult) HumanText() string {
	return fmt.Sprintf("docket %s (commit %s, built %s)", r.Version, r.Commit, r.BuildDate)
}
