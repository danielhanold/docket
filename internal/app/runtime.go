package app

import (
	"fmt"

	"github.com/danielhanold/docket/internal/buildinfo"
)

// approvedTargets is the released-product tuple set. supported_target: false
// is data under an applied result, not an inspection failure.
var approvedTargets = map[string]struct{}{
	"darwin/amd64": {},
	"darwin/arm64": {},
	"linux/amd64":  {},
	"linux/arm64":  {},
}

// RuntimeResult reports the running toolchain and target tuple.
type RuntimeResult struct {
	Envelope
	GoVersion       string `json:"go_version"`
	GoOS            string `json:"go_os"`
	GoArch          string `json:"go_arch"`
	SupportedTarget bool   `json:"supported_target"`
}

// DiagnosticRuntime is the `docket diagnostic runtime` operation. It reads
// only the injected facts — never the repository, configuration, Git, or gh.
func DiagnosticRuntime(facts buildinfo.RuntimeFacts) RuntimeResult {
	_, supported := approvedTargets[facts.GOOS+"/"+facts.GOARCH]
	return RuntimeResult{
		Envelope:        NewEnvelope("diagnostic.runtime", ResultApplied),
		GoVersion:       facts.GoVersion,
		GoOS:            facts.GOOS,
		GoArch:          facts.GOARCH,
		SupportedTarget: supported,
	}
}

// HumanText renders the four labeled lines in stable order.
func (r RuntimeResult) HumanText() string {
	return fmt.Sprintf("go_version: %s\ngo_os: %s\ngo_arch: %s\nsupported_target: %t",
		r.GoVersion, r.GoOS, r.GoArch, r.SupportedTarget)
}
