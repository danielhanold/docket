package app

import (
	"context"
	"fmt"

	"github.com/danielhanold/docket/internal/reposetup"
)

// OperationRepositoryConfigureTests is the operation key `repository
// configure-tests` records.
const OperationRepositoryConfigureTests = "repository.configure-tests"

// RunRepositoryConfigureTests is the setup-time upgrade path for an
// already-initialized repository: it (re)generates the pending, UNSTAGED
// `.docket.yml` test-policy edit from a fresh suite discovery over the primary
// worktree. It is the human-runnable sibling of the discovery init/migrate
// already perform, for a repository whose test policy was never configured or
// whose suite has since appeared.
//
// It requires the HEALTHY docket topology — it is an upgrade path, not a
// bootstrap — and refuses every other state with the remedy valid there: a fresh
// repository is pointed at `docket repository init`, a legacy one at `docket
// repository migrate`, and any other unhealthy state at `docket repository
// check`. It classifies once, never commits, and never stages: the generated
// edit rides back as a pending review path exactly as init's does. An ambiguous
// discovery writes nothing and names the candidate families so a human chooses.
func RunRepositoryConfigureTests(ctx context.Context, d SetupDeps) OperationResult {
	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		return repositoryGatherFailure(OperationRepositoryConfigureTests, err)
	}

	// The healthy postconditions (metadata root shape, the .docket worktree's
	// clean/synchronized/hooks-off state, the committed ignore block) are the
	// check-only augmentation probes, not the base gather — the same reads
	// `docket repository check` runs before it classifies. Configure-tests admits
	// only the healthy topology, so it must classify over the same augmented facts;
	// the augmentation is meaningful only once the remote docket branch is present.
	if facts.RemoteMetadata.Presence == reposetup.PresencePresent {
		augmentCheckFacts(ctx, d.Git, &facts, sc)
	}

	cls, refusal := configureTestsGuard(facts)
	if refusal != nil {
		return *refusal
	}

	// Discover the suite over the primary worktree and write the generated
	// `.docket.yml` edit UNSTAGED when it applies — the same helper (and pending
	// posture) init uses. A configured pair or a file already carrying these exact
	// settings writes nothing (idempotent no-op); an ambiguous outcome writes
	// nothing and rides back as a note.
	pendingPath, wrote, discovery, derr := ensureTestPolicyConfig(sc.repo.PrimaryWorktree, sc.cfg)
	if derr != nil {
		return repositoryInternalFailure(OperationRepositoryConfigureTests, cls.State, "generating the test-policy config", derr)
	}

	result := ResultNoOp
	state := cls.State
	var pending []string
	if wrote {
		result = ResultApplied
		state = reposetup.StateNeedsReview
		pending = []string{pendingPath}
	}

	out := newRepositoryOpResult(OperationRepositoryConfigureTests, result, RepositoryOpResult{
		RepositoryState: string(state),
		PendingPaths:    pending,
		SourceRevision:  sc.sourceRevision,
	})
	if wrote {
		out.human = fmt.Sprintf("test policy generated (%s); review and commit the pending path: %s", state, pendingPath)
	} else {
		out.human = fmt.Sprintf("%s: %s (%s): the test policy is already configured; nothing to write",
			OperationRepositoryConfigureTests, result, state)
		// The configured short-circuit fires as soon as EITHER gate's command is
		// set, so a repo with one gate configured and the other `gate: local` +
		// empty command reaches this no-op while `docket repository check` still
		// flags the gap. Name that gate and the by-hand completion instead of
		// stranding the operator at a bare "nothing to write".
		if discovery.Kind == reposetup.DiscoveryConfigured {
			if gap := reposetup.ConfigureTestsGapNote(sc.cfg); gap != "" {
				out.human += "\n" + gap
			}
		}
	}
	if note := testDiscoveryNote(discovery); note != "" {
		out.human += "\n" + note
	}
	return out
}

// configureTestsGuard classifies once and admits only the healthy docket
// topology; every other state is a typed invalid-state refusal whose remedy is
// valid in exactly that state. It is pure over the gathered facts so the refusal
// mapping is unit-testable without a real repository. A nil refusal means
// configure-tests may proceed to discover and write.
func configureTestsGuard(facts reposetup.Facts) (reposetup.Classification, *RepositoryOpResult) {
	cls := reposetup.Classify(facts)
	switch cls.State {
	case reposetup.StateHealthy:
		return cls, nil
	case reposetup.StateFresh:
		r := configureTestsRefusal(cls.State,
			"repository is not initialized; run `docket repository init`, then re-run `docket repository configure-tests`")
		return cls, &r
	case reposetup.StateLegacy:
		r := configureTestsRefusal(cls.State,
			"repository has a legacy single-branch layout; run `docket repository migrate`, then re-run `docket repository configure-tests`")
		return cls, &r
	default:
		r := configureTestsRefusal(cls.State,
			"repository is not in a healthy state; run `docket repository check` and resolve the reported findings before configuring tests")
		return cls, &r
	}
}

// configureTestsRefusal builds a configure-tests refusal: an invalid-state
// envelope naming the classified state and carrying the state-valid remedy.
func configureTestsRefusal(state reposetup.State, remedy string) RepositoryOpResult {
	out := newRepositoryOpResult(OperationRepositoryConfigureTests, ResultInvalidState, RepositoryOpResult{
		RepositoryState: string(state),
	})
	out.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryConfigureTests, ResultInvalidState, state, remedy)
	return out
}
