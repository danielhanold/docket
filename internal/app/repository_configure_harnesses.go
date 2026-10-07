package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
)

// OperationRepositoryConfigureHarnesses is the operation key `repository
// configure-harnesses` records.
const OperationRepositoryConfigureHarnesses = "repository.configure-harnesses"

// ConfigureHarnessesOptions carries configure-harnesses' one input: the
// --harnesses value and the CLI's interactive chooser.
type ConfigureHarnessesOptions struct {
	Harnesses HarnessesOptions
}

// RunRepositoryConfigureHarnesses changes an initialized repository's
// agent_harnesses through init's shared choose-harnesses step
// (resolveHarnessChoice, then applyHarnessChoice): it writes the selection to
// the repository's config target and refreshes the dispatch surfaces the new
// value authorizes, removing those of dropped harnesses.
//
// Input is validated before any repository read: with neither --harnesses nor a
// chooser it refuses invalid-input, and a bad selection is invalid-input. It
// needs no clean checkout and no at-tip primary, and it admits needs-review, so
// it can run again before the previous edit is committed. A fresh repository is
// pointed at `docket repository init`, a legacy one at `docket repository
// migrate`, and any other unhealthy state at `docket repository check`. It never
// commits and never stages: a shared repository's edits ride back as pending
// review paths.
func RunRepositoryConfigureHarnesses(ctx context.Context, d SetupDeps, o ConfigureHarnessesOptions) OperationResult {
	const op = OperationRepositoryConfigureHarnesses
	if !o.Harnesses.Set && o.Harnesses.Chooser == nil {
		out := newRepositoryOpResult(op, ResultInvalidInput, RepositoryOpResult{})
		out.human = fmt.Sprintf("%s: %s: there is no terminal to show the checklist on; pass --harnesses <list> (for example --harnesses claude,cursor), or --harnesses none to record that no agents are used",
			op, ResultInvalidInput)
		return out
	}
	flagSel, refusal := parseHarnessesFlag(op, o.Harnesses)
	if refusal != nil {
		return *refusal
	}

	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		return repositoryGatherFailure(op, err)
	}
	// The healthy postconditions are the check-only augmentation probes, as for
	// configure-tests; they are meaningful only once the remote docket branch is
	// present.
	if facts.RemoteMetadata.Presence == reposetup.PresencePresent {
		augmentCheckFacts(ctx, d.Git, &facts, sc)
	}
	cls, refused := configureHarnessesGuard(facts)
	if refused != nil {
		return *refused
	}

	_, display, _ := repoConfigTarget(sc)
	choice, cerr := resolveHarnessChoice(ctx, harnessChoiceInput{flag: flagSel, chooser: o.Harnesses.Chooser, policy: harnessPolicyConfigure,
		cfg: sc.cfg, repoDeclared: harnessRepoDeclared(sc.cfg), configDisplay: display, detect: detectHarnesses})
	if errors.Is(cerr, ErrHarnessChoiceCancelled) {
		return harnessChoiceCancelled(op, cls.State)
	}
	if cerr != nil {
		return repositoryInternalFailure(op, cls.State, "choosing agent harnesses", cerr)
	}

	applied, herr := applyHarnessChoice(ctx, d.Git, sc, choice)
	if herr != nil {
		return harnessApplyFailure(op, cls.State, herr)
	}
	pending, perr := workingTreePendingPaths(ctx, d.Git, sc, facts.CommittedIgnoreBlock)
	if perr != nil {
		return repositoryExternalFailure(op, cls.State, "listing the pending review paths", perr)
	}

	result := ResultNoOp
	if applied.wroteConfig || applied.wroteSurfaces {
		result = ResultApplied
	}
	state := cls.State
	if len(pending) > 0 && state == reposetup.StateHealthy {
		state = reposetup.StateNeedsReview
	}
	out := newRepositoryOpResult(op, result, RepositoryOpResult{
		RepositoryState: string(state),
		PendingPaths:    pending,
		SourceRevision:  sc.sourceRevision,
	})
	list := "`" + reposetup.FormatHarnessList(choice.selection) + "`"
	switch {
	case result == ResultNoOp:
		out.human = fmt.Sprintf("%s: %s (%s): agent_harnesses is already %s and its instructions are current; nothing to write", op, ResultNoOp, state, list)
	case display == layout.PrivateConfigDisplay:
		out.human = fmt.Sprintf("agent harnesses set to %s (%s); wrote %s", list, state, layout.PrivateConfigDisplay)
	case len(pending) == 0:
		out.human = fmt.Sprintf("agent harnesses set to %s (%s)", list, state)
	default:
		out.human = fmt.Sprintf("agent harnesses set to %s (%s); review and commit the pending paths: %s", list, state, strings.Join(pending, ", "))
	}
	setHarnessResult(&out, choice, applied)
	return out
}

// configureHarnessesGuard classifies once and admits healthy, needs-review with
// the primary checkout on the integration branch, or either once init's
// clean-checkout and at-tip preconditions are set aside;
// every other state is an invalid-state refusal whose remedy is valid there.
// Pure over the gathered facts.
func configureHarnessesGuard(facts reposetup.Facts) (reposetup.Classification, *RepositoryOpResult) {
	cls := reposetup.Classify(facts)
	refuse := func(remedy string) (reposetup.Classification, *RepositoryOpResult) {
		out := newRepositoryOpResult(OperationRepositoryConfigureHarnesses, ResultInvalidState, RepositoryOpResult{RepositoryState: string(cls.State)})
		out.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryConfigureHarnesses, ResultInvalidState, cls.State, remedy)
		return cls, &out
	}
	switch cls.State {
	case reposetup.StateHealthy:
		return cls, nil
	case reposetup.StateNeedsReview:
		// Needs-review is decided before the healthy postconditions, so it does not
		// prove the primary checkout is on the integration branch; require that as
		// the healthy path does (an unknown probe is not a proven absence).
		if facts.PrimaryOnIntegration != reposetup.PresenceAbsent {
			return cls, nil
		}
		return refuse("the primary checkout is not on the integration branch; run `docket repository check` and resolve the reported findings first")
	case reposetup.StateFresh:
		return refuse("repository is not initialized; run `docket repository init` (it takes --harnesses too)")
	case reposetup.StateLegacy:
		return refuse("repository has a legacy single-branch layout; run `docket repository migrate`, then re-run `" + reposetup.ConfigureHarnessesCommand + "`")
	}
	relaxed := facts
	relaxed.PrimaryClean, relaxed.PrimaryAtRemoteTip = reposetup.PresencePresent, reposetup.PresencePresent
	if s := reposetup.Classify(relaxed).State; s == reposetup.StateHealthy || s == reposetup.StateNeedsReview {
		return cls, nil
	}
	return refuse("repository is not in a healthy state; run `docket repository check` and resolve the reported findings first")
}

// workingTreePendingPaths lists, sorted, the docket-managed working-tree paths
// pending review in the primary worktree — the set repository check names
// (collectPendingReviewPaths), so a surface removed this run counts as pending
// too and an unrelated .gitignore edit does not once the managed block is
// committed. Unlike check, a status read error is returned. A private
// repository has none: its config and surfaces live outside the working tree.
func workingTreePendingPaths(ctx context.Context, git *gitcli.Client, sc setupContext, committedIgnore reposetup.Presence) ([]string, error) {
	if sc.layout.Mode == layout.Private {
		return nil, nil
	}
	return collectPendingReviewPaths(ctx, git, sc.repo, sc.layout.Mode, committedIgnore)
}
