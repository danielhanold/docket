package app

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/buildinfo"
)

// CapabilityVersion identifies the capability-catalog contract, versioned
// separately from the protocol envelope: a consumer refuses a version it does
// not support, fail-closed, without having to refuse the whole protocol.
const CapabilityVersion = 1

// CapabilityCommand is one public executable command in the catalog. It is a
// plain carrier so internal/app carries no dependency on internal/cli (which
// depends on app): the cli adapter converts its own CapabilityEntry into this.
// The JSON tags are protocol and must stay identical to the cli-side type.
type CapabilityCommand struct {
	ID        string   `json:"id"`
	Argv      []string `json:"argv"`
	Signature string   `json:"signature,omitempty"`
	Effects   []string `json:"effects"`
}

// GlobalFlag is one root-persistent flag, represented once at document level
// rather than restated per command.
type GlobalFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
	Usage   string `json:"usage"`
}

// GlobalInvocation carries the invocation-wide flags every command inherits
// (today exactly --json).
type GlobalInvocation struct {
	Flags []GlobalFlag `json:"flags"`
}

// BinaryIdentity is the binary's build identity in the catalog. Its fields
// mirror VersionResult's identity fields exactly (version/commit/build_date),
// so a consumer reads one identity vocabulary across `docket version` and
// `docket capabilities`.
type BinaryIdentity struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// CapabilitiesResult is the protocol-v1 capability catalog document: the
// envelope, the separately versioned capability_version, the binary identity,
// the invocation-global flags, and the sorted command catalog.
type CapabilitiesResult struct {
	Envelope
	CapabilityVersion int                 `json:"capability_version"`
	Binary            BinaryIdentity      `json:"binary"`
	Global            GlobalInvocation    `json:"global"`
	Commands          []CapabilityCommand `json:"commands"`
}

// Capabilities builds the catalog document. The command ordering is the
// caller's — the walker sorts by id before handing them over; this constructor
// preserves whatever order it is given.
func Capabilities(info buildinfo.Info, commands []CapabilityCommand, global GlobalInvocation) CapabilitiesResult {
	return CapabilitiesResult{
		Envelope:          NewEnvelope("capabilities", ResultApplied),
		CapabilityVersion: CapabilityVersion,
		Binary: BinaryIdentity{
			Version:   info.Version,
			Commit:    info.Commit,
			BuildDate: info.BuildDate,
		},
		Global:   global,
		Commands: commands,
	}
}

// HumanText renders the compact default text form: a versioned header with the
// command count, then one line per command — id, argv, signature, effects.
func (r CapabilitiesResult) HumanText() string {
	var b strings.Builder
	noun := "commands"
	if len(r.Commands) == 1 {
		noun = "command"
	}
	fmt.Fprintf(&b, "capabilities v%d — %d %s", r.CapabilityVersion, len(r.Commands), noun)
	for _, c := range r.Commands {
		line := strings.Join(c.Argv, " ")
		if c.Signature != "" {
			line += " " + c.Signature
		}
		fmt.Fprintf(&b, "\n  %s  %s  [%s]", c.ID, line, strings.Join(c.Effects, " "))
	}
	return b.String()
}
