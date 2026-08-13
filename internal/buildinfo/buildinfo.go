// Package buildinfo owns injected build identity and running-toolchain facts.
package buildinfo

import "runtime"

// Development-build defaults. A release build may override each via the Go
// linker, e.g.:
//
//	go build -ldflags "-X github.com/danielhanold/docket/internal/buildinfo.Version=v1.0.0 \
//	  -X github.com/danielhanold/docket/internal/buildinfo.Commit=<sha> \
//	  -X github.com/danielhanold/docket/internal/buildinfo.BuildDate=<date>" ./cmd/docket
//
// This change documents and tests that seam; release packaging is change 0317's.
var (
	Version   = "development"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Info is the build identity the version operation reports.
type Info struct {
	Version   string
	Commit    string
	BuildDate string
}

// Current returns the identity of this binary.
func Current() Info { return Info{Version: Version, Commit: Commit, BuildDate: BuildDate} }

// RuntimeFacts are the running toolchain and target tuple. They are a value,
// not live reads, so operations stay deterministic under test injection.
type RuntimeFacts struct {
	GoVersion string
	GOOS      string
	GOARCH    string
}

// CurrentRuntime reads the facts of the running binary.
func CurrentRuntime() RuntimeFacts {
	return RuntimeFacts{GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}
