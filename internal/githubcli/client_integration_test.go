//go:build integration

package githubcli

import (
	"context"
	"testing"
)

// TestIntegrationHarnessEnvHygieneStripsRetargeting (f): GH_REPO/GH_HOST set in
// the parent are NOT visible to the fake, while an auth token (GH_TOKEN) survives
// so normal GitHub authentication channels remain available. It drives a real
// fake-gh subprocess (change 0333's integration partition), so it rides the
// TestIntegrationHarness shard alongside the fake-protocol self-tests.
func TestIntegrationHarnessEnvHygieneStripsRetargeting(t *testing.T) {
	c, log := newFakeClient(t,
		fakeScenario{Invocations: []fakeArm{prViewArm(samplePRJSON)}},
		withExtraEnv(
			"GH_REPO=evil/owner-repo",
			"GH_HOST=evil.example.invalid",
			"GH_TOKEN=gho_survivingtoken",
		),
	)
	_, f := c.run(context.Background(), runRequest{
		op:      "probe",
		dir:     t.TempDir(),
		args:    []string{"pr", "view", "7"},
		network: true,
	})
	if f != nil {
		t.Fatalf("run failed: %v", f)
	}
	recs := log.records(t)
	if len(recs) != 1 {
		t.Fatalf("witness records = %d, want 1", len(recs))
	}
	env := recs[0].Env
	if _, ok := env["GH_REPO"]; ok {
		t.Errorf("GH_REPO leaked to child: %q", env["GH_REPO"])
	}
	if _, ok := env["GH_HOST"]; ok {
		t.Errorf("GH_HOST leaked to child: %q", env["GH_HOST"])
	}
	if env["GH_TOKEN"] != "gho_survivingtoken" {
		t.Errorf("GH_TOKEN not preserved for auth: %q", env["GH_TOKEN"])
	}
}
