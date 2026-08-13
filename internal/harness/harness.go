// Package harness defines the planning contract every supported agent harness
// implements. It is pure computation: an adapter reads the asset catalog and
// the resolved configuration and returns the targets an installation would
// have, and nothing in this package or its children touches the filesystem —
// applying targets belongs to internal/install.
package harness

import (
	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/install"
)

// InstallMode selects what an adapter's rendered paths point at: a release
// install links into the immutable version tree, a development install links
// into the canonical source checkout.
type InstallMode string

const (
	ModeRelease     InstallMode = "release"
	ModeDevelopment InstallMode = "development"
)

// PlanInput is everything an adapter may read. It carries values only — no
// filesystem handle, no config loader, no repository layer.
type PlanInput struct {
	Assets    assets.Catalog
	Mode      InstallMode
	AssetsDir string // release: immutable version-tree assets dir; development: canonical source root
	Roots     install.UserRoots
	Agents    config.AgentsTable // resolved built-in ⊕ global
}

// Detection is the read-only answer to "is this harness present for this
// user?": Root names the directory the answer was read from, whether or not it
// exists.
type Detection struct {
	Present bool
	Root    string
}

// Adapter is one harness's renderer. Plan must be deterministic: the same
// PlanInput yields the same targets, in the same order.
type Adapter interface {
	Name() string
	Detect(install.UserRoots) Detection
	Plan(PlanInput) ([]install.Target, error)
}

// Order is the fixed planning order. Planning, state, and reporting all iterate
// it, so the produced plan never depends on map iteration.
var Order = []string{"claude", "codex", "cursor", "opencode"}
