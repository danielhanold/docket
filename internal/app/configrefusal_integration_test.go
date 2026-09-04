//go:build integration

package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// --- invalid-configuration refusal diagnostics (change 0403) -----------------
//
// Every command that refuses on an invalid .docket.yml must lift the
// resolver's diagnostics into its findings array — code, severity, and a
// .docket.yml:<line> ref — and name each ref in its human text, while the
// result / reason / exit stay exactly what they were before the lift.
// The fixture is invalidConfigYML (three error defects at lines 2, 6, 7;
// semantics pinned by TestInvalidConfigFixtureSemantics).

// wantConfigRefusalRefs maps the expected finding codes to their refs.
// Keep in lockstep with TestInvalidConfigFixtureSemantics.
var wantConfigRefusalRefs = map[string]string{
	"unknown-key":   ".docket.yml:2",
	"invalid-type":  ".docket.yml:6",
	"invalid-value": ".docket.yml:7",
}

// assertConfigRefusalFindings asserts the lifted error findings and the human
// refs for one refusing command's result.
func assertConfigRefusalFindings(t *testing.T, findings []reposetup.Finding, human string) {
	t.Helper()
	var errs []reposetup.Finding
	for _, f := range findings {
		if f.Severity == reposetup.Severity("error") {
			errs = append(errs, f)
		}
	}
	if len(errs) != 3 {
		t.Fatalf("error findings = %d (%+v), want 3", len(errs), findings)
	}
	seen := map[string]bool{}
	for _, f := range errs {
		want, ok := wantConfigRefusalRefs[f.Code]
		if !ok || seen[f.Code] {
			t.Errorf("unexpected or duplicate finding code %q (%+v)", f.Code, f)
			continue
		}
		seen[f.Code] = true
		if f.Ref != want {
			t.Errorf("finding %q ref = %q, want %q", f.Code, f.Ref, want)
		}
		if f.Message == "" {
			t.Errorf("finding %q has an empty message", f.Code)
		}
	}
	for _, ref := range wantConfigRefusalRefs {
		if !strings.Contains(human, ref) {
			t.Errorf("human text lacks ref %q:\n%s", ref, human)
		}
	}
}

// TestIntegrationRepoCheckInvalidConfigDiagnostics: check still refuses
// unsupported-config with exit 2 (state unknown), and now carries the three
// findings in JSON and their refs in the human text.
func TestIntegrationRepoCheckInvalidConfigDiagnostics(t *testing.T) {
	r := newInitRepo(t, invalidConfigYML, nil)
	res := r.runCheck(t)

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q (%s), want %q", res.Result, res.HumanText(), ResultUnsupportedConfig)
	}
	if res.RepositoryState != string(reposetup.StateUnknown) {
		t.Errorf("repository_state = %q, want unknown", res.RepositoryState)
	}
	if code := res.CheckExitCode(); code != 2 {
		t.Errorf("exit = %d, want 2 (state unknown, unchanged by the lift)", code)
	}
	human := res.HumanText()
	if !strings.HasPrefix(human, "repository check: "+string(ResultUnsupportedConfig)+": ") {
		t.Errorf("human header changed: %q", human)
	}
	assertConfigRefusalFindings(t, res.Findings, human)
}

// TestIntegrationRepoPrepareInvalidConfigDiagnostics: prepare still refuses
// (unsupported-config / refused), and now carries the structured findings the
// Step-0 contract promises, plus the refs in its human text.
func TestIntegrationRepoPrepareInvalidConfigDiagnostics(t *testing.T) {
	r := newInitRepo(t, invalidConfigYML, nil)
	res := runPrepareAt(t, r.invocation)

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q (%s), want %q", res.Result, res.HumanText(), ResultUnsupportedConfig)
	}
	if res.Disposition != PrepareDispositionRefused {
		t.Errorf("disposition = %q, want refused", res.Disposition)
	}
	human := res.HumanText()
	if !strings.HasPrefix(human, "repository prepare: "+PrepareDispositionRefused+": ") {
		t.Errorf("human header changed: %q", human)
	}
	assertConfigRefusalFindings(t, res.Findings, human)
}
