package app

import (
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
)

// These are the real-git transaction integration tests for the claim → attach
// half of the agent workflow: they drive the landed ContextImplementation,
// ChangeClaim, ChangeReconcile, ChangeAttachPlan, and WorkspacePrepare operations
// through a real transaction.Engine over real bare-remote temporary repositories,
// in BOTH metadata modes (main and docket, via planRepoModes). The topology
// builders, the git oracle helpers (originTip/originFile/originCommitPaths/
// blobVersionAt/originFeatureBranches), and the invocation-clone node builder are
// reused from status_git_test.go / planning_git_test.go — this file invents no
// third harness. The attach fixtures (attachHappyPlan/attachBacklinkBlock) are
// reused from change_attach_git_test.go.
//
// The concurrency properties these tests pin cannot be faked: an independent
// writer must ACTUALLY diverge the contended path on the origin between the
// context read and the mutation, so the operation's own fresh-origin re-read is
// what discovers the divergence (learnings green-suite-untested-branch and
// cas-re-read-fresh-origin). Every "loser" assertion therefore reads the bare
// origin, never the invocation clone's stale local tree.

// buildReadyChange renders a proposed, unstacked, build-ready change record: the
// canonical groomable record with trivial flipped true, so EvaluateReadiness
// reports build-ready (design present) and ClaimEligibility passes without a
// separate spec artifact. It is the fixture the claim race/retry tests start
// from — a change a fresh context read reports claim-eligible.
func buildReadyChange(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "trivial: false\n", "trivial: true\n", 1)
}

// commitPlanFile writes one plan artifact into a feature workspace and commits it
// with the ADR-0094 plan-path trailer, returning the new head. It is the writer
// half every attach test needs — the plan-writer's single-artifact commit.
func commitPlanFile(t *testing.T, wp, planPath, content, trailerPath string) string {
	t.Helper()
	writeRepoFile(t, wp, planPath, content)
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "-m", "write plan", "--trailer", "Docket-Plan-Path: "+trailerPath)
	return runGit(t, wp, "rev-parse", "HEAD")
}

// --- claim race: two claimants, same context version, one loses cleanly -----

// --- claim retry after a lost response: replay, never a second claim --------

// --- reconcile against an independent writer's divergence --------------------

// --- attach-plan lands change record + board in ONE metadata commit ----------

// --- 0335: refresh-claim succeeds when the re-rendered board is unchanged ---

// planningDepsForClock is planningDepsFor with an explicit clock, so a refresh
// run can happen at a later wall time than the claim that preceded it against the
// same origin — the record's claimed_at/updated then genuinely change while the
// inline board (which shows none of those fields) re-renders byte-identical.
func planningDepsForClock(t *testing.T, dir string, clock transaction.Clock) realNode {
	t.Helper()
	client := newGitClient(t)
	engine, err := transaction.NewEngine(client, clock)
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	return realNode{
		dir: dir,
		deps: PlanningDeps{
			Client: client,
			Engine: engine,
			Reader: NewGitStatusReader(client),
			Clock:  clock,
		},
	}
}

// --- effective base consumed from the domain resolver, not hard-coded --------
