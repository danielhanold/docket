package app

import (
	"context"
	"fmt"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposetup"
)

// OperationRepositoryConfigureTests is the operation key `repository
// configure-tests` records.
const OperationRepositoryConfigureTests = "repository.configure-tests"

// ConfigureTestsOptions carries configure-tests' one input. Command is nil when
// --command was not passed (discovery runs) and non-nil whenever it was, even
// when empty: an explicit empty value is refused, never read as absent.
type ConfigureTestsOptions struct {
	Command *string
}

// RunRepositoryConfigureTests is the setup-time upgrade path for an
// already-initialized repository: it (re)generates the pending, UNSTAGED
// `.docket.yml` test-policy edit from a fresh suite discovery over the primary
// worktree. It is the human-runnable sibling of the discovery init/migrate
// already perform, for a repository whose test policy was never configured or
// whose suite has since appeared.
//
// With `--command` (o.Command non-nil) it validates the command before any
// read, skips discovery, and plans both gates `local` with that command,
// overwriting the existing policy. Invalid input is an invalid-input refusal
// with nothing read or written.
//
// It requires the HEALTHY docket topology — it is an upgrade path, not a
// bootstrap — and refuses every other state with the remedy valid there: a fresh
// repository is pointed at `docket repository init`, a legacy one at `docket
// repository migrate`, and any other unhealthy state at `docket repository
// check`. It classifies once, never commits, and never stages: the generated
// edit rides back as a pending review path exactly as init's does. A discovery
// that writes nothing reports what it actually found (configureTestsDiscoveryText).
func RunRepositoryConfigureTests(ctx context.Context, d SetupDeps, o ConfigureTestsOptions) OperationResult {
	// Validate the whole input before any repository read or write.
	var explicit string
	if o.Command != nil {
		cmd, verr := reposetup.ExplicitTestCommand(*o.Command)
		if verr != nil {
			out := newRepositoryOpResult(OperationRepositoryConfigureTests, ResultInvalidInput, RepositoryOpResult{})
			out.human = fmt.Sprintf("%s: %s: %s", OperationRepositoryConfigureTests, ResultInvalidInput, verr.Error())
			return out
		}
		explicit = cmd
	}

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

	// Write the `.docket.yml` edit UNSTAGED when it applies — the same pending
	// posture init uses. An explicit command plans both gates local with it;
	// otherwise discovery runs. A file already carrying these exact settings
	// writes nothing (idempotent no-op); an ambiguous discovery writes nothing.
	var (
		pendingPath string
		wrote       bool
		discovery   reposetup.DiscoveryOutcome
		werr        error
	)
	if o.Command != nil {
		pendingPath, wrote, werr = ensureExplicitTestCommand(sc.repo.PrimaryWorktree, explicit)
	} else {
		pendingPath, wrote, discovery, werr = ensureTestPolicyConfig(sc.repo.PrimaryWorktree, sc.cfg)
	}
	if werr != nil {
		return repositoryInternalFailure(OperationRepositoryConfigureTests, cls.State, "generating the test-policy config", werr)
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
	if o.Command != nil {
		out.human = configureTestsExplicitText(state, wrote, pendingPath, explicit)
	} else {
		out.human = configureTestsDiscoveryText(state, wrote, pendingPath, discovery, sc.cfg)
	}
	return out
}

// configureTestsExplicitText renders the --command outcome.
func configureTestsExplicitText(state reposetup.State, wrote bool, pendingPath, cmd string) string {
	if wrote {
		return fmt.Sprintf("test policy set: build and finalize gates `local` running `%s` (%s); review and commit the pending path: %s",
			cmd, state, pendingPath)
	}
	return fmt.Sprintf("%s: %s (%s): build and finalize gates are already `local` running `%s`; nothing to write",
		OperationRepositoryConfigureTests, ResultNoOp, state, cmd)
}

// configureTestsDiscoveryText renders the discovery outcome truthfully per
// kind, never one generic "already configured" line: none reports the resolved
// gates and the --command remedy (the none plan is preserve-explicit on gate,
// so the gates are not assumed off); ambiguous names every candidate's command;
// configured names both resolved commands plus any per-gate gap; detected with
// no change names the command already in place.
func configureTestsDiscoveryText(state reposetup.State, wrote bool, pendingPath string, outcome reposetup.DiscoveryOutcome, cfg config.Effective) string {
	noneRemedy := fmt.Sprintf("re-run with `%s` to set both gates to `local` with your suite command", reposetup.ConfigureTestsCommandRemedy)
	if wrote {
		text := fmt.Sprintf("test policy generated (%s); review and commit the pending path: %s", state, pendingPath)
		if outcome.Kind == reposetup.DiscoveryNone {
			text += "\nno supported test suite was found, so no test command was written; after committing, " + noneRemedy
		}
		return text
	}
	var body string
	switch outcome.Kind {
	case reposetup.DiscoveryNone:
		body = fmt.Sprintf("no supported test suite was found, so no test command was written (build gate `%s`, finalize gate `%s`); %s",
			cfg.Build.Gate.Value, cfg.Finalize.Gate.Value, noneRemedy)
	case reposetup.DiscoveryAmbiguous:
		body = fmt.Sprintf("test discovery found more than one suite, so nothing was written: %s; re-run `%s` with the one to use",
			reposetup.DescribeCandidates(outcome.Candidates), reposetup.ConfigureTestsCommandRemedy)
	case reposetup.DiscoveryDetected:
		body = fmt.Sprintf("the test policy is already configured (build and finalize: `%s`); nothing to write", outcome.Command)
	default: // configured
		// The configured short-circuit fires as soon as EITHER gate's command is
		// set, so a repo with one gate configured and the other `gate: local` +
		// empty command reaches this no-op while `docket repository check` still
		// flags the gap: name that gate and its remedy.
		body = fmt.Sprintf("the test policy is already configured (build: %s; finalize: %s); nothing to write",
			describeCommand(cfg.Build.TestCommand.Value), describeCommand(cfg.Finalize.TestCommand.Value))
		if gap := reposetup.ConfigureTestsGapNote(cfg); gap != "" {
			body += "\n" + gap
		}
	}
	return fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryConfigureTests, ResultNoOp, state, body)
}

// describeCommand renders a resolved test command for an operator: the
// backticked command, or "no command" when unset.
func describeCommand(cmd string) string {
	if cmd == "" {
		return "no command"
	}
	return "`" + cmd + "`"
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
