//go:build integration

package githubcli

import (
	"context"
	"testing"
)

// RetargetPullRequest drives probe→act→verify for one exact PR's base through
// the protocol-faithful fake gh. Each case asserts BOTH the returned outcome and
// the witness log — whether an edit was issued — so a green result can never hide
// an unexercised or an extra external mutation.

// retRepo is the explicit repository identity every retarget query carries.
func retRepo() Repository { return Repository{Host: "github.com", Owner: "acme", Name: "widget"} }

// retViewArm scripts a `pr view` response (the by-number reprobe). Multiple such
// arms in a Sequential scenario are consumed in order (before-state, then
// after-state).
func retViewArm(stdout string, exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "view"}, Stdout: stdout, Exit: exit}
}

func retEditArm(exit int) fakeArm {
	return fakeArm{ArgvPrefix: []string{"pr", "edit"}, Exit: exit}
}

func TestIntegrationMergeRetargetProbeActVerify(t *testing.T) {
	const oldBase = "main"
	const newBase = "release"
	// The PR as first probed: open, at the old base.
	atOld := ensPRJSON(7, "OPEN", false, ensHead, ensHeadOid, oldBase, ensTitle, ensBody)
	atNew := ensPRJSON(7, "OPEN", false, ensHead, ensHeadOid, newBase, ensTitle, ensBody)
	oldVersion := mustDecodeOne(t, atOld).Version

	t.Run("retargeted", func(t *testing.T) {
		c, log := newFakeClient(t, fakeScenario{
			Sequential: true,
			Invocations: []fakeArm{
				retViewArm(atOld, 0), // probe: at old base
				retEditArm(0),        // act: pr edit --base
				retViewArm(atNew, 0), // verify: at new base
			},
		})
		out, pr, err := c.RetargetPullRequest(context.Background(), retRepo(), 7, oldVersion, newBase)
		if err != nil {
			t.Fatalf("RetargetPullRequest: %v", err)
		}
		if out != RetargetRetargeted {
			t.Fatalf("outcome = %q, want %q", out, RetargetRetargeted)
		}
		if pr.BaseBranch != newBase {
			t.Fatalf("verified base = %q, want %q", pr.BaseBranch, newBase)
		}
		if n := countArgv(log.records(t), "pr", "edit"); n != 1 {
			t.Fatalf("pr edit issued %d times, want 1", n)
		}
	})

	t.Run("already", func(t *testing.T) {
		// The promised end-state already holds: the PR is at newBase. No edit.
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm(atNew, 0)}})
		out, pr, err := c.RetargetPullRequest(context.Background(), retRepo(), 7, oldVersion, newBase)
		if err != nil {
			t.Fatalf("RetargetPullRequest: %v", err)
		}
		if out != RetargetAlready {
			t.Fatalf("outcome = %q, want %q", out, RetargetAlready)
		}
		if pr.BaseBranch != newBase {
			t.Fatalf("base = %q, want %q", pr.BaseBranch, newBase)
		}
		if n := countArgv(log.records(t), "pr", "edit"); n != 0 {
			t.Fatalf("pr edit issued %d times on an already-retargeted PR, want 0", n)
		}
	})

	t.Run("contended-version-drift", func(t *testing.T) {
		// The live PR version differs from ExpectedVersion: refuse, no edit.
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm(atOld, 0)}})
		out, _, err := c.RetargetPullRequest(context.Background(), retRepo(), 7, "sha256:stale-token-differs", newBase)
		if err != nil {
			t.Fatalf("RetargetPullRequest returned error on contended: %v", err)
		}
		if out != RetargetContended {
			t.Fatalf("outcome = %q, want %q", out, RetargetContended)
		}
		if n := countArgv(log.records(t), "pr", "edit"); n != 0 {
			t.Fatalf("pr edit issued %d times on version drift, want 0", n)
		}
	})

	t.Run("probe-error-unknown", func(t *testing.T) {
		// A probe that errors is never read as clean absence — retain, unknown.
		c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{retViewArm("", 1)}})
		out, _, err := c.RetargetPullRequest(context.Background(), retRepo(), 7, oldVersion, newBase)
		if out != RetargetUnknown {
			t.Fatalf("outcome = %q, want %q", out, RetargetUnknown)
		}
		if err == nil {
			t.Fatal("unknown outcome must carry a diagnostic error")
		}
		if n := countArgv(log.records(t), "pr", "edit"); n != 0 {
			t.Fatalf("pr edit issued %d times after probe error, want 0", n)
		}
	})
}
