package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
)

// TestCapabilitiesEnvelopeAndShape pins the protocol-v1 envelope, the separately
// versioned capability_version, the top-level field names (they are protocol),
// and that input command order is preserved verbatim — sorting is the walker's
// job, asserted at the cli level, not this document constructor's.
func TestCapabilitiesEnvelopeAndShape(t *testing.T) {
	res := Capabilities(buildinfo.Info{Version: "1.2.3", Commit: "abc1234", BuildDate: "2026-08-13"},
		[]CapabilityCommand{
			{ID: "b.op", Argv: []string{"docket", "b", "op"}, Effects: []string{"read"}},
			{ID: "a.op", Argv: []string{"docket", "a", "op"}, Effects: []string{"metadata-write"}, Signature: "--request <file>"},
		},
		GlobalInvocation{Flags: []GlobalFlag{{Name: "json", Type: "bool", Default: "false", Usage: "emit protocol-v1 JSON on stdout"}}})

	env := res.Env()
	if env.Operation != "capabilities" || env.Result != ResultApplied || env.ProtocolVersion != ProtocolVersion {
		t.Fatalf("envelope = %+v", env)
	}
	if res.CapabilityVersion != CapabilityVersion {
		t.Fatalf("capability_version = %d, want %d", res.CapabilityVersion, CapabilityVersion)
	}

	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	// Top-level field names are protocol.
	for _, key := range []string{"protocol_version", "operation", "result", "capability_version", "binary", "global", "commands"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("missing field %q in %s", key, b)
		}
	}
	// The binary block reuses version.go's identity vocabulary exactly, so
	// consumers see one identity spelling across operations.
	for _, key := range []string{"version", "commit", "build_date"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("binary block missing identity field %q in %s", key, b)
		}
	}
	if res.Binary.Version != "1.2.3" || res.Binary.Commit != "abc1234" || res.Binary.BuildDate != "2026-08-13" {
		t.Errorf("binary identity = %+v, want the info fixture verbatim", res.Binary)
	}

	// Input order preserved verbatim.
	if len(res.Commands) != 2 || res.Commands[0].ID != "b.op" || res.Commands[1].ID != "a.op" {
		t.Fatalf("commands reordered: %+v", res.Commands)
	}
}

// TestCapabilitiesHumanText pins the compact human rendering: a versioned
// header with the command count, then one line per command carrying id, argv,
// signature, and effects.
func TestCapabilitiesHumanText(t *testing.T) {
	res := Capabilities(buildinfo.Info{Version: "1.2.3", Commit: "abc1234", BuildDate: "2026-08-13"},
		[]CapabilityCommand{
			{ID: "a.op", Argv: []string{"docket", "a", "op"}, Signature: "--request <file>", Effects: []string{"read"}},
		}, GlobalInvocation{})

	txt := res.HumanText()
	if !strings.Contains(txt, "capabilities v1") || !strings.Contains(txt, "1 command") {
		t.Errorf("header missing or wrong count: %q", txt)
	}
	for _, want := range []string{"a.op", "docket a op", "--request <file>", "read"} {
		if !strings.Contains(txt, want) {
			t.Errorf("human text missing %q: %q", want, txt)
		}
	}
}
