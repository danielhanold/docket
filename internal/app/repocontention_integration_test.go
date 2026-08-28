//go:build integration

package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// This is the sequential migration-contention shard (prefix
// TestIntegrationRepoContention). Each test scripts its own legacy repository via
// the established internal/app harness (newInitRepo, runMigrate/runMigrateWithHooks,
// the origin* oracles from repomigration_integration_test.go and the setupHooks
// seam) and simulates a concurrent second writer INTERLEAVING with a migration by
// advancing an authoritative remote branch through a hook boundary — no real
// goroutines here (the -race concurrency lives in the reposetup_race shard). It
// proves the exact-lease pushes key on the FRESH re-read of the moved remote
// (learning cas-re-read-fresh-origin): a foreign advance loses the operation to a
// clean `contended` with no force and no overwrite, a non-planning advance rebuilds
// atop the fresh tip, and a concurrent `repository check` sees only the resumable
// `partial` classification, never a torn one.

// advanceRemoteDocketChild advances the origin metadata (docket) branch by one
// empty child commit pushed through the writer clone — a second writer moving the
// published seed forward — and returns the new remote tip.
func (r *initRepo) advanceRemoteDocketChild(t *testing.T, message string) string {
	t.Helper()
	runGit(t, r.writer, "fetch", "-q", "origin", "docket")
	runGit(t, r.writer, "checkout", "-q", "-B", "docket", "origin/docket")
	runGit(t, r.writer, "commit", "-q", "--allow-empty", "-m", message)
	runGit(t, r.writer, "push", "-q", "origin", "docket")
	return r.originTip(t, "docket")
}

// TestIntegrationRepoContentionMetadataLeaseLoss proves that when a resume-at-prune
// migration must UPDATE the published seed under its owned lease (the pinned source
// moved on since the seed was published, so its planning bytes changed), a second
// writer that advances the remote docket branch between the fresh re-read and the
// owned-lease push loses the migration to a clean `contended`: the exact lease is
// keyed on the fresh re-read (learning cas-re-read-fresh-origin), so it never
// force-overwrites the foreign advance, which stays intact.
func TestIntegrationRepoContentionMetadataLeaseLoss(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())

	// A first migration seeds the metadata branch (native seed, receipt naming the
	// then-current source) but crashes before the prune: the seed is durable and the
	// integration surface is still live.
	r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		afterSeedPush: func() error { return errors.New("crash after seed") },
	})
	if !r.remoteBranchExists(t, "docket") {
		t.Fatal("the seed push did not land before the injected crash")
	}
	seededTip := r.originTip(t, "docket")

	// The pinned source moves on with a PLANNING-byte change (a new ADR is copy-set
	// content), so the resume must re-validate and UPDATE the seed under its owned
	// lease rather than adopt it unchanged.
	r.advanceIntegration(t, "docs/adrs/0002-second-decision.md",
		"---\nid: 2\nslug: second-decision\nstatus: Accepted\ntitle: Second decision\n---\nContext.\n")

	// A second writer advances remote docket between the resume's fresh re-read and
	// its owned-lease push. The lease, keyed on the fresh re-read, must lose.
	var foreignTip string
	res := r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		beforeMetadataLeasePush: func() error {
			foreignTip = r.advanceRemoteDocketChild(t, "second writer advances docket")
			return nil
		},
	})

	if res.Result != ResultContended {
		t.Fatalf("a lost metadata lease = %q (%s), want contended", res.Result, res.HumanText())
	}
	if foreignTip == "" || foreignTip == seededTip {
		t.Fatalf("the second-writer advance did not move docket (seeded=%s foreign=%s)", seededTip, foreignTip)
	}
	if got := r.originTip(t, "docket"); got != foreignTip {
		t.Errorf("the foreign docket advance was overwritten (%s -> %s); the lease loss must not force", foreignTip, got)
	}
	// The integration surface was never pruned: nothing was destroyed on contention.
	if !contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
		t.Error("a contended metadata lease pruned the integration surface; nothing must be destroyed")
	}
}

// TestIntegrationRepoContentionIntegrationLeaseLoss proves that when integration
// advances between the seed and the prune (a run crashed after seeding, then a
// second writer advanced integration with non-planning bytes), the resuming
// migration rebuilds the prune atop the FRESH re-read of the integration tip: the
// prune's parent is exactly the advanced tip (the lease value is the fresh
// re-read, learning cas-re-read-fresh-origin), the non-planning bytes survive, and
// the active surface is pruned.
func TestIntegrationRepoContentionIntegrationLeaseLoss(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())

	r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		afterSeedPush: func() error { return errors.New("crash after seed") },
	})
	if !r.remoteBranchExists(t, "docket") {
		t.Fatal("the seed push did not land before the injected crash")
	}

	// A second writer advances integration with bytes OUTSIDE the copy and removal
	// sets, so the seed is unchanged (adopted) and only the prune must rebuild.
	r.advanceIntegration(t, "docs/results/late.md", "# late results\n")
	freshTip := r.originTip(t, "main")

	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("resume after an integration advance = %q (%s), want applied", res.Result, res.HumanText())
	}
	// The prune keyed on the FRESH re-read: its parent is exactly the advanced tip.
	if parent := runGit(t, r.origin, "rev-parse", "main^"); parent != freshTip {
		t.Errorf("prune parent = %s, want the fresh integration tip %s; the lease used a stale re-read", parent, freshTip)
	}
	if !contains(r.originTreePaths(t, "main"), "docs/results/late.md") {
		t.Error("the rebuilt prune dropped the non-planning advance")
	}
	if contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
		t.Error("the rebuilt prune did not remove the active surface")
	}
}

// TestIntegrationRepoContentionCheckDuringMigrationSeesPartial proves a
// `repository check` that races a migration observes only the resumable `partial`
// classification (exit 1, the idempotent-resume remedy), never a torn state — both
// at the seed-published-live-surface boundary and at the remote-migrated-but-local-
// attach-incomplete boundary a fresh clone sees after the migration completes.
func TestIntegrationRepoContentionCheckDuringMigrationSeesPartial(t *testing.T) {
	t.Run("seed published, integration live", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
		r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
			afterSeedPush: func() error { return errors.New("crash after seed") },
		})
		if !r.remoteBranchExists(t, "docket") {
			t.Fatal("the seed push did not land before the injected crash")
		}

		res := r.runCheck(t)
		if res.RepositoryState != string(reposetup.StatePartial) {
			t.Errorf("mid-migration check state = %q (%s), want partial", res.RepositoryState, res.HumanText())
		}
		if code := res.CheckExitCode(); code != 1 {
			t.Errorf("mid-migration check exit = %d, want 1 (diagnosed, resumable)", code)
		}
		assertResumeRemedy(t, res.HumanText())
	})

	t.Run("remote migrated, local attach incomplete", func(t *testing.T) {
		r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())

		// Clone BEFORE the migration, so this clone never fetched the docket object:
		// the check must fetch the published metadata branch to prove its root shape,
		// otherwise the ls-remote presence probe leaves the root Unknown and the state
		// falls through to a torn conflict instead of the resumable partial.
		clone := cloneOrigin(t, r.origin)
		if res := r.runMigrate(t, MigrateOptions{Authorized: true}); res.Result != ResultApplied {
			t.Fatalf("migrate = %q (%s), want applied", res.Result, res.HumanText())
		}

		// The clone sees a remote whose migration completed but whose local attach
		// did not: a resumable partial.
		client := newGitClient(t)
		res := RunRepositoryCheck(context.Background(), SetupDeps{Git: client, RepoDir: clone})
		if res.RepositoryState != string(reposetup.StatePartial) {
			t.Errorf("attach-incomplete check state = %q (%s), want partial", res.RepositoryState, res.HumanText())
		}
		if code := res.CheckExitCode(); code != 1 {
			t.Errorf("attach-incomplete check exit = %d, want 1 (diagnosed, resumable)", code)
		}
		assertResumeRemedy(t, res.HumanText())
	})
}

// assertResumeRemedy fails unless the check's human text names the idempotent
// migrate-resume remedy (the printed remedy for a partial classification).
func assertResumeRemedy(t *testing.T, human string) {
	t.Helper()
	if !strings.Contains(human, "resume") || !strings.Contains(human, "migrate") {
		t.Errorf("partial remedy must name the idempotent migrate resume; got:\n%s", human)
	}
}
