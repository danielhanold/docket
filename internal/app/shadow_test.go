package app

import (
	"encoding/json"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/install"
)

// checkEnvelopeNotShadowed marshals an OperationResult and reports every
// reserved envelope field whose marshalled value diverges from Env(). Go's
// encoding/json silently prefers a depth-0 field over the embedded
// Envelope's field of the same JSON name, so a shadowing field would drop
// the envelope's value from the protocol with no compile or runtime error.
func checkEnvelopeNotShadowed(r OperationResult) []string {
	var problems []string

	raw, err := json.Marshal(r)
	if err != nil {
		return []string{"marshal failed: " + err.Error()}
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		return []string{"unmarshal failed: " + err.Error()}
	}

	env := r.Env()
	want := map[string]any{
		"protocol_version": float64(env.ProtocolVersion),
		"operation":        env.Operation,
		"result":           string(env.Result),
	}
	for _, name := range []string{"protocol_version", "operation", "result"} {
		v, ok := got[name]
		if !ok {
			problems = append(problems, name+": absent from marshalled JSON")
			continue
		}
		if v != want[name] {
			problems = append(problems, name+": marshalled value diverges from Env()")
		}
	}
	return problems
}

// TestEnvelopeNotShadowed guards the spec's no-shadowing invariant: every
// operation result must marshal the embedded Envelope's own
// protocol_version, operation, and result values.
//
// New operations register here — append one line to the slice below.
func TestEnvelopeNotShadowed(t *testing.T) {
	cases := []struct {
		name   string
		result OperationResult
	}{
		{"version", Version(buildinfo.Info{Version: "1.2.3", Commit: "abc", BuildDate: "2026-01-01"})},
		{"diagnostic.runtime", DiagnosticRuntime(buildinfo.RuntimeFacts{GoVersion: "go1.24.0", GOOS: "darwin", GOARCH: "arm64"})},
		{"cli", CLIError(ReasonInvalidArguments, "unknown flag")},
		{"diagnostic.config", DiagnosticConfig(sparseSources(), mainCtx(), false)},
		{"config.preflight", DiagnosticConfig(blockedSources(), mainCtx(), true)},
		{"install", NewInstallResult(OperationInstall, install.Outcome{Applied: true, Mode: install.ModeRelease})},
		{"install.check", NewInstallResult(OperationInstallCheck, install.Outcome{
			Reason: install.ReasonInstallationRequired, Err: install.ErrNotInstalled})},
		{"development.install", NewInstallResult(OperationDevelopmentInstall, install.Outcome{
			Mode: install.ModeDevelopment, Reason: install.ReasonBuildFailed, Err: install.ErrBuildFailed})},
		{"status", NewStatusResult(ResultApplied, StatusResult{})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if problems := checkEnvelopeNotShadowed(tc.result); len(problems) != 0 {
				t.Errorf("envelope fields shadowed or missing: %v", problems)
			}
		})
	}
}

// shadowedResult exists only to prove the guard bites: its depth-0 Result
// field outranks the embedded Envelope's `result` during marshalling.
type shadowedResult struct {
	Envelope
	Result string `json:"result"`
}

func (shadowedResult) HumanText() string { return "shadowed" }

// TestEnvelopeNotShadowedDetectsShadowing is the mutation evidence for the
// guard above: on a struct that does shadow a reserved name, the helper must
// report the divergence rather than pass.
func TestEnvelopeNotShadowedDetectsShadowing(t *testing.T) {
	bad := shadowedResult{
		Envelope: NewEnvelope("version", ResultApplied),
		Result:   "definitely-not-applied",
	}

	problems := checkEnvelopeNotShadowed(bad)
	if len(problems) == 0 {
		t.Fatal("guard did not detect a shadowed `result` field")
	}
	found := false
	for _, p := range problems {
		if p == "result: marshalled value diverges from Env()" {
			found = true
		}
	}
	if !found {
		t.Errorf("guard reported %v, want a divergence on `result`", problems)
	}
}
