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

// repository_set_visibility.go — `docket repository set-visibility
// <shared|private>`: moves an existing repository between the shared and the
// private layout, keeping every record. It follows repository_migrate.go's
// two-pass authorization: an unauthorized run previews the phases with a
// composite pin over the remote tips it read; an authorized run re-proves that
// pin and runs every pending phase. Each phase's completion is read from STATE
// (refs on each remote, folder names, worktree registrations, config files),
// never from the detected mode alone, so a re-run resumes where an interrupted
// one stopped. It never force-pushes, never deletes a foreign ref, never rolls
// back, and makes no metadata commit: the identical history is published under
// the target branch name.

// OperationRepositorySetVisibility is the operation key `repository
// set-visibility` records.
const OperationRepositorySetVisibility = "repository.set-visibility"

// visibilityConfirmationRequiredState is the repository_state an unauthorized
// preview carries.
const visibilityConfirmationRequiredState = "confirmation-required"

// The closed phase statuses a result reports.
const (
	visibilityPhaseDone    = "done"
	visibilityPhasePending = "pending"
	visibilityPhaseApplied = "applied"
	visibilityPhaseKept    = "kept"
	visibilityPhaseSkipped = "skipped"
)

// The closed commit statuses a result reports.
const (
	visibilityCommitPlanned   = "planned"
	visibilityCommitCommitted = "committed"
	visibilityCommitFailed    = "failed"
)

// visibilityPrivateFlagsMessage is the invalid-input text for a going-private
// flag passed with the shared target.
const visibilityPrivateFlagsMessage = "--metadata-remote, --delete-shared-branch and --remove-shared-files apply only when switching to private"

// SetVisibilityOptions carries the target and the two-pass authorization the
// CLI resolves. Authorized is true only via --yes or an interactive confirmed
// preview; ExpectedSource is the composite pin the preview showed ("" on the
// preview pass). The three going-private options are refused with Target
// shared.
type SetVisibilityOptions struct {
	Target             string // "shared" | "private"
	Authorized         bool
	ExpectedSource     string // the preview's pin ("" on the preview pass)
	MetadataRemote     string // going private only
	DeleteSharedBranch bool   // going private only
	RemoveSharedFiles  bool   // going private only
}

// VisibilityPhase is one phase row: its name, its status (done, pending,
// applied, kept, skipped), and the preview detail.
type VisibilityPhase struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// VisibilityCommit is one integration-branch commit the switch plans or made:
// its fixed subject, the branch it lands on, the paths it holds, the commit id
// once made, and its status (planned, committed, failed).
type VisibilityCommit struct {
	Subject string   `json:"subject"`
	Branch  string   `json:"branch"`
	Paths   []string `json:"paths"`
	Commit  string   `json:"commit,omitempty"`
	Status  string   `json:"status"`
}

// RepositorySetVisibilityResult is the protocol-v1 document `repository
// set-visibility` returns. RepositoryState is confirmation-required on a
// preview, else the mode the repository is in after the run (or the classified
// state of a refusal). SourceRevision is the composite pin.
type RepositorySetVisibilityResult struct {
	Envelope
	RepositoryState string              `json:"repository_state"`
	Current         string              `json:"current"`
	Target          string              `json:"target"`
	SourceRevision  string              `json:"source_revision"`
	Phases          []VisibilityPhase   `json:"phases"`
	Commits         []VisibilityCommit  `json:"commits,omitempty"`
	Warnings        []string            `json:"warnings,omitempty"`
	PendingLocal    []string            `json:"pending_local,omitempty"`
	Lessons         string              `json:"lessons,omitempty"`
	BackupRemote    string              `json:"backup_remote,omitempty"`
	Findings        []reposetup.Finding `json:"findings,omitempty"`
	human           string
}

// HumanText renders the human summary.
func (r RepositorySetVisibilityResult) HumanText() string {
	if r.human != "" {
		return r.human
	}
	return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.RepositoryState)
}

// SourceRev exposes the composite pin a preview showed, so the CLI's
// interactive confirm flow re-invokes pinned to exactly that state.
func (r RepositorySetVisibilityResult) SourceRev() string { return r.SourceRevision }

// ConfirmationRequired reports whether this result is a preview awaiting --yes.
func (r RepositorySetVisibilityResult) ConfirmationRequired() bool {
	return r.RepositoryState == visibilityConfirmationRequiredState
}

// visibilityStep is one phase of a switch: done is read from state, detail is
// the preview line, commits marks a phase that makes an integration-branch
// commit, and run executes it.
type visibilityStep struct {
	name    string
	done    bool   // read from state
	detail  string // preview line
	commits bool   // makes an integration-branch commit
	run     func(ctx context.Context, x *visibilityRun) error
}

// visibilityRun is what an executing phase reads and reports into. An
// executor sets stop to end the run successfully after its phase, leaving the
// later phases pending for a re-run.
type visibilityRun struct {
	d    SetupDeps
	o    SetVisibilityOptions
	st   visibilityState
	res  *RepositorySetVisibilityResult
	stop bool
}

// markPhase sets the status (and, when non-empty, the detail) of the named
// phase row, so an executor can report kept or skipped instead of applied.
func (x *visibilityRun) markPhase(name, status, detail string) {
	for i := range x.res.Phases {
		if x.res.Phases[i].Name != name {
			continue
		}
		x.res.Phases[i].Status = status
		if detail != "" {
			x.res.Phases[i].Detail = detail
		}
		return
	}
}

// visibilityRefusal is an executor's refusal a human resolves: it maps to
// invalid-state with its text, never to an external failure.
type visibilityRefusal struct {
	state reposetup.State
	msg   string
}

func (e *visibilityRefusal) Error() string { return e.msg }

// visibilityInternal is a defect-shaped stop (a phase with no executor).
type visibilityInternal struct{ err error }

func (e *visibilityInternal) Error() string { return e.err.Error() }
func (e *visibilityInternal) Unwrap() error { return e.err }

// RunRepositorySetVisibility moves an existing repository between shared and
// private visibility under the two-pass authorization model.
func RunRepositorySetVisibility(ctx context.Context, d SetupDeps, o SetVisibilityOptions) RepositorySetVisibilityResult {
	// 1. Input, before any read.
	if msg := validateSetVisibilityInput(o); msg != "" {
		return visibilityInvalidInput(o.Target, msg)
	}

	// 2. Gather.
	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		return visibilityFromMigrateResult(migrateGatherFailure(err), o.Target)
	}

	// 3. Set up?
	if r := visibilitySetupRefusal(ctx, d.Git, facts, sc, o.Target); r != nil {
		return *r
	}

	// 4. Debris, then the state probe.
	debris := sweepSetupDebris(ctx, d.Git, sc.repo)
	st, err := probeVisibility(ctx, d, sc, facts)
	if err != nil {
		return visibilityExternalFailure(o.Target, "", reposetup.StateUnknown, "reading the repository's visibility state", err)
	}
	current := string(st.current)

	// 5. Live runs and live gates, over both existing state folders.
	lines, err := visibilityLiveRunLines(st)
	if err != nil {
		return visibilityExternalFailure(o.Target, current, reposetup.StateUnknown, "checking for live runs", err)
	}
	if len(lines) > 0 {
		return visibilityRefusalResult(o.Target, current, reposetup.StateNeedsReview,
			"a run or gate is still live in this repository; settle each one, then re-run:\n  "+strings.Join(lines, "\n  "))
	}

	// 6. Metadata sync.
	if msg, err := visibilityMetadataSyncRefusal(ctx, d.Git, st); err != nil {
		return visibilityExternalFailure(o.Target, current, reposetup.StateUnknown, "checking the metadata worktree", err)
	} else if msg != "" {
		return visibilityRefusalResult(o.Target, current, reposetup.StateNeedsReview, msg)
	}

	// Going shared, the commit phase is complete only when every dispatch
	// surface agent_harnesses authorizes is already settled on disk, which is
	// decidable only once the repository reads shared.
	if o.Target == string(layout.Shared) && st.current == layout.Shared {
		if st.sharedSurfacesSettled, err = repoSurfacesSettled(ctx, d.Git, st.primary); err != nil {
			return visibilityExternalFailure(o.Target, current, reposetup.StateUnknown, "reading the dispatch instructions", err)
		}
	}

	// 7. Plan; refuse the user's own edits to a path a pending commit holds.
	steps := planVisibility(st, o)
	if visibilityCommitsPending(steps) {
		conflicts, err := commitPathConflicts(ctx, d.Git, st.sc.repo, st.common, visibilityCommitPaths)
		if err != nil {
			return visibilityExternalFailure(o.Target, current, reposetup.StateUnknown, "checking the integration-branch paths", err)
		}
		if len(conflicts) > 0 {
			return visibilityRefusalResult(o.Target, current, reposetup.StateNeedsReview,
				"commit or set aside these edits, then re-run:\n  "+strings.Join(conflicts, "\n  "))
		}
	}

	// 8. Pin and warnings.
	pin := visibilityPin(st)
	res := RepositorySetVisibilityResult{
		Current:        current,
		Target:         o.Target,
		SourceRevision: pin,
		Phases:         visibilityPhaseRows(steps),
		Commits:        visibilityPlannedCommits(st, o, steps),
		Warnings:       visibilityWarnings(ctx, d, st, o),
		PendingLocal:   debris.pending(),
	}

	// 9. Nothing pending, or a preview.
	if !visibilityAnyPending(steps) {
		res.Envelope = NewEnvelope(OperationRepositorySetVisibility, ResultNoOp)
		res.RepositoryState = current
		res.Commits = nil
		res.human = fmt.Sprintf("repository set-visibility: already %s; nothing to do", current)
		return res
	}
	if !o.Authorized {
		res.Envelope = NewEnvelope(OperationRepositorySetVisibility, ResultInvalidState)
		res.RepositoryState = visibilityConfirmationRequiredState
		res.human = visibilityPreviewText(st, o, steps, res) +
			"\nconfirmation required: re-run with --yes to authorize this switch"
		return res
	}

	// 10. Authorized: re-prove the pin, then run every pending phase.
	if migrateSourceMoved(o.ExpectedSource, pin) {
		res.Envelope = NewEnvelope(OperationRepositorySetVisibility, ResultContended)
		res.RepositoryState = current
		res.SourceRevision = o.ExpectedSource
		res.human = fmt.Sprintf("set-visibility contended: the repository moved since the preview (pinned %s, now %s); re-run to preview the new state",
			o.ExpectedSource, pin)
		return res
	}
	x := &visibilityRun{d: d, o: o, st: st, res: &res}
	if err := runVisibilitySteps(ctx, x, steps); err != nil {
		return visibilityStopped(res, err)
	}
	res.Envelope = NewEnvelope(OperationRepositorySetVisibility, ResultApplied)
	mode, derr := layout.Detect(st.common)
	if derr != nil {
		res.RepositoryState = string(reposetup.StateUnknown)
	} else {
		res.RepositoryState = string(mode)
	}
	if mode == layout.Shared && o.Target == string(layout.Shared) && res.BackupRemote == "" {
		// A resumed run whose dckt remote was already removed still names the
		// backup it kept, when that bare repository is on disk.
		if url := visibilityBackupURL(st); url != "" {
			if ok, _ := isDirAt(url); ok {
				res.BackupRemote = url
			}
		}
	}
	res.human = visibilityAppliedText(res)
	return res
}

// validateSetVisibilityInput is the pre-read input check; "" is valid.
func validateSetVisibilityInput(o SetVisibilityOptions) string {
	switch o.Target {
	case string(layout.Private):
		if _, err := absMetadataRemote(o.MetadataRemote); err != nil {
			return fmt.Sprintf("--metadata-remote %q cannot be made absolute: %v", o.MetadataRemote, err)
		}
		return ""
	case string(layout.Shared):
		if o.MetadataRemote != "" || o.DeleteSharedBranch || o.RemoveSharedFiles {
			return visibilityPrivateFlagsMessage
		}
		return ""
	default:
		return fmt.Sprintf("the target must be shared or private, not %q", o.Target)
	}
}

// visibilitySetupRefusal refuses a repository that is not set up: an unknown
// required probe, a live planning surface (migrate first), or neither a
// metadata branch on origin nor a configured dckt remote (init first).
func visibilitySetupRefusal(ctx context.Context, git *gitcli.Client, facts reposetup.Facts, sc setupContext, target string) *RepositorySetVisibilityResult {
	var unknown []string
	if facts.RemoteConfigured == reposetup.PresenceUnknown {
		unknown = append(unknown, "remote-configured")
	}
	if facts.RemoteDefaultBranch.Presence == reposetup.PresenceUnknown {
		unknown = append(unknown, "remote-default-branch")
	}
	if facts.RemoteIntegration.Presence == reposetup.PresenceUnknown {
		unknown = append(unknown, "remote-integration-branch")
	}
	if facts.LiveSurface == reposetup.PresenceUnknown {
		unknown = append(unknown, "live-surface")
	}
	origin, oerr := git.ProbeRemoteBranch(ctx, sc.repo, originRemote, metadataRef(layout.SharedLayout(sc.repo.CommonDir, sc.repo.PrimaryWorktree)))
	if oerr != nil {
		unknown = append(unknown, "origin-metadata-branch")
	}
	_, uerr := git.RemoteURL(ctx, sc.repo, gitcli.RemoteName(layout.PrivateName))
	dcktConfigured := uerr == nil
	if uerr != nil && !isRemoteUnconfigured(uerr) {
		unknown = append(unknown, "dckt-remote")
	}
	// A second clone whose origin branch is gone and whose dckt remote is not
	// configured yet is still set up when another clone on this machine
	// published the history to the default store. An origin URL that names no
	// store means no store can exist.
	storeHeld := false
	if oerr == nil && origin.State != gitcli.RemoteRefFound && uerr != nil && isRemoteUnconfigured(uerr) {
		if priv, perr := privateLayoutOf(ctx, git, sc.repo); perr == nil {
			ref, serr := defaultStoreTip(ctx, git, priv)
			if serr != nil {
				unknown = append(unknown, "private-store")
			}
			storeHeld = ref.State == gitcli.RemoteRefFound
		}
	}
	current := string(sc.layout.Mode)
	if len(unknown) > 0 {
		r := visibilityRefusalResult(target, current, reposetup.StateUnknown,
			"a required repository probe could not be resolved ("+strings.Join(unknown, ", ")+"); run `docket repository check` after ensuring the remote is configured and reachable")
		return &r
	}
	if facts.LiveSurface == reposetup.PresencePresent {
		r := visibilityRefusalResult(target, current, reposetup.StateLegacy,
			"the integration branch still carries a live planning surface; run `docket repository migrate` first")
		return &r
	}
	if origin.State != gitcli.RemoteRefFound && !dcktConfigured && !storeHeld {
		r := visibilityRefusalResult(target, current, reposetup.StateFresh,
			"the repository has no docket metadata branch on origin and no dckt remote; run `docket repository init` first")
		return &r
	}
	return nil
}

// planVisibility returns the target direction's phases.
func planVisibility(st visibilityState, o SetVisibilityOptions) []visibilityStep {
	if o.Target == string(layout.Private) {
		return planToPrivate(st, o)
	}
	return planToShared(st, o)
}

// planToPrivate is the going-private phase list, each with its done predicate
// read from state; a step without an executor stops an authorized run.
func planToPrivate(st visibilityState, o SetVisibilityOptions) []visibilityStep {
	wantURL := st.private.DefaultBareRemote
	if st.dcktURL != "" {
		wantURL = st.dcktURL
	}
	if o.MetadataRemote != "" {
		if abs, err := absMetadataRemote(o.MetadataRemote); err == nil {
			wantURL = abs
		}
	}
	sourceName := "the dckt branch"
	if st.originDocket.State == gitcli.RemoteRefFound {
		sourceName = "origin's docket branch"
	}
	steps := []visibilityStep{
		{
			name:   "metadata-remote",
			done:   st.dcktURL != "" && st.dcktURL == wantURL,
			detail: "point the dckt remote at " + wantURL,
			run:    privateMetadataRemotePhase,
		},
		{
			name: "publish",
			// A clone that has not configured its dckt remote yet reads the
			// default store directly: another clone on this machine may already
			// have published the history there.
			done:   st.bareHoldsOrigin || (st.dcktURL == "" && wantURL == st.private.DefaultBareRemote && st.storeHoldsOrigin),
			detail: "push the identical metadata history of " + sourceName + " to the dckt branch at " + wantURL,
			run:    privatePublishPhase,
		},
		{
			name:   "config",
			done:   !st.localConfig && st.pendingConfig != "",
			detail: "fold the configuration into " + layout.PrivateConfigDisplay + " (from " + st.foldSource + ")",
			run:    privateConfigPhase,
		},
		{
			name:   "state-folder",
			done:   st.privateStateDir && !st.sharedStateDir,
			detail: "rename " + st.shared.StateDir + " to " + st.private.StateDir,
			run:    privateStateFolderPhase,
		},
		{
			name:   "ignore",
			done:   st.excludeBlock,
			detail: "add the managed block to .git/info/exclude",
			run:    privateIgnorePhase,
		},
		{
			name:   "metadata-worktree",
			done:   st.privateCheckoutReady && !st.sharedCheckoutRegistered && st.localDocket == "",
			detail: "move the metadata checkout from " + st.shared.MetadataWorktree + " to " + st.private.MetadataWorktree,
			run:    privateMetadataWorktreePhase,
		},
		{
			name:   "instructions",
			done:   st.privateStateDir && !st.recordWorkingTree && (!st.facts.SurfacesAuthorized || st.privateInstructions),
			detail: "install the dispatch instructions into " + layout.PrivateInstructionsDisplay,
			run:    privateInstructionsPhase,
		},
	}
	if o.DeleteSharedBranch {
		steps = append(steps, visibilityStep{
			name:   "delete-shared-branch",
			done:   st.originDocket.State != gitcli.RemoteRefFound,
			detail: "delete origin's docket branch once the dckt branch holds the same tip",
			run:    privateDeleteSharedBranchPhase,
		})
	}
	// A removal commit already journaled resumes even when the re-run omits
	// the flag: its edits are in the working tree, waiting for the commit.
	if o.RemoveSharedFiles || (st.journalPresent && st.journal.Subject == visibilityRemoveSubject) {
		steps = append(steps, visibilityStep{
			name:    "remove-shared-files",
			done:    !st.journalPresent && !st.headSharedFiles,
			detail:  "remove .docket.yml, the .gitignore block, and the dispatch instructions in one local commit",
			commits: true,
			run:     privateRemoveSharedFilesPhase,
		})
	}
	steps = append(steps, visibilityStep{
		name:   "align-visibility",
		done:   st.privateVisibilityAligned,
		detail: "set visibility: private in " + layout.PrivateConfigDisplay,
		run:    privateAlignVisibilityPhase,
	})
	return steps
}

// planToShared is the going-shared phase list, each with its done predicate
// read from state; a step without an executor stops an authorized run.
func planToShared(st visibilityState, _ SetVisibilityOptions) []visibilityStep {
	backup := visibilityBackupURL(st)
	return []visibilityStep{
		{
			name:    "identity-keys",
			done:    st.identityKeysAligned,
			detail:  "commit the repository identity keys to .docket.yml and get that commit onto origin's " + st.sc.defaultBranch + " first",
			commits: !st.identityKeysAligned,
			run:     sharedIdentityKeysPhase,
		},
		{
			name:   "publish",
			done:   st.originHoldsBare,
			detail: "push the identical metadata history of the dckt branch to origin's docket branch",
			run:    sharedPublishPhase,
		},
		{
			name:   "instructions",
			done:   !st.privateInstructions && !st.recordGitDir,
			detail: "retire " + layout.PrivateInstructionsDisplay + " (its promoted lessons are reported)",
			run:    sharedInstructionsPhase,
		},
		{
			name:   "config-committed",
			done:   st.sharedConfigWritten,
			detail: "write .docket.yml from the private configuration",
			run:    sharedConfigCommittedPhase,
		},
		{
			name:   "state-folder",
			done:   st.sharedStateDir && !st.privateStateDir,
			detail: "rename " + st.private.StateDir + " to " + st.shared.StateDir,
			run:    sharedStateFolderPhase,
		},
		{
			name:   "config-local",
			done:   st.sharedStateDir && !st.privateStateDir && st.pendingConfig == "",
			detail: "restore the clone-local keys to .docket.local.yml and retire the private configuration",
			run:    sharedConfigLocalPhase,
		},
		{
			name:   "ignore",
			done:   !st.excludeBlockPresent && st.worktreeGitignoreValid,
			detail: "move the managed ignore block from .git/info/exclude to .gitignore",
			run:    sharedIgnorePhase,
		},
		{
			name:   "metadata-worktree",
			done:   st.sharedCheckoutReady && !st.privateCheckoutRegistered && st.localDckt == "" && st.dcktURL == "",
			detail: "move the metadata checkout to " + st.shared.MetadataWorktree + " and remove the dckt remote",
			run:    sharedMetadataWorktreePhase,
		},
		{
			name:    "commit",
			done:    !st.journalPresent && st.headGitignoreValid && !st.commitPathsDirty && st.current == layout.Shared && st.sharedSurfacesSettled,
			detail:  "commit .docket.yml, .gitignore, and the dispatch instructions in one local commit",
			commits: true,
			run:     sharedCommitPhase,
		},
		{
			name:   "backup",
			done:   true,
			detail: "the bare repository " + backup + " is kept as a backup",
		},
		{
			name:   "align-visibility",
			done:   st.sharedVisibilityAligned,
			detail: "set visibility: shared in .docket.local.yml",
			run:    sharedAlignVisibilityPhase,
		},
	}
}

// runVisibilitySteps runs each pending step in order, records it applied
// (unless its executor reported otherwise), then fires the afterVisibilityPhase
// seam. An error stops the run, keeping the phases so far; a pending step with
// no executor is an internal error.
func runVisibilitySteps(ctx context.Context, x *visibilityRun, steps []visibilityStep) error {
	for _, s := range steps {
		if s.done {
			continue
		}
		if s.run == nil {
			return &visibilityInternal{err: fmt.Errorf("phase %s has no executor", s.name)}
		}
		if err := s.run(ctx, x); err != nil {
			return err
		}
		for i := range x.res.Phases {
			if x.res.Phases[i].Name == s.name && x.res.Phases[i].Status == visibilityPhasePending {
				x.res.Phases[i].Status = visibilityPhaseApplied
			}
		}
		hook := x.d.hooks.afterVisibilityPhase
		name := s.name
		if err := fire(func() error {
			if hook == nil {
				return nil
			}
			return hook(name)
		}); err != nil {
			return err
		}
		if x.stop {
			return nil
		}
	}
	return nil
}

// visibilityAnyPending reports whether any step is still to run.
func visibilityAnyPending(steps []visibilityStep) bool {
	for _, s := range steps {
		if !s.done {
			return true
		}
	}
	return false
}

// visibilityCommitsPending reports whether a pending step makes a commit.
func visibilityCommitsPending(steps []visibilityStep) bool {
	for _, s := range steps {
		if !s.done && s.commits {
			return true
		}
	}
	return false
}

// visibilityPhaseRows projects the steps into their result rows.
func visibilityPhaseRows(steps []visibilityStep) []VisibilityPhase {
	rows := make([]VisibilityPhase, 0, len(steps))
	for _, s := range steps {
		status := visibilityPhasePending
		if s.done {
			status = visibilityPhaseDone
		}
		rows = append(rows, VisibilityPhase{Name: s.name, Status: status, Detail: s.detail})
	}
	return rows
}

// visibilityPlannedCommits is the planned commit row when a pending step
// commits: the fixed subject, the primary checkout's branch, and the paths with
// what happens to each.
func visibilityPlannedCommits(st visibilityState, o SetVisibilityOptions, steps []visibilityStep) []VisibilityCommit {
	if !visibilityCommitsPending(steps) {
		return nil
	}
	subject, paths := visibilityAddSubject, []string{".docket.yml (write)", ".gitignore (write)", "the dispatch instructions agent_harnesses names (write)"}
	if o.Target == string(layout.Private) {
		subject, paths = visibilityRemoveSubject, nil
		for _, p := range st.headPresentCommitPaths {
			action := "write"
			if p == ".docket.yml" || p == ".cursor/rules/docket-dispatch.mdc" {
				action = "delete"
			}
			paths = append(paths, p+" ("+action+")")
		}
	}
	return []VisibilityCommit{{Subject: subject, Branch: st.primaryBranch, Paths: paths, Status: visibilityCommitPlanned}}
}

// visibilityPreviewText renders the confirmation preview.
func visibilityPreviewText(st visibilityState, o SetVisibilityOptions, steps []visibilityStep, res RepositorySetVisibilityResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "docket repository set-visibility %s — plan\n", o.Target)
	fmt.Fprintf(&b, "  repository: %s\n", st.primary)
	fmt.Fprintf(&b, "  current:    %s\n", st.current)
	fmt.Fprintf(&b, "  target:     %s\n", o.Target)
	fmt.Fprintf(&b, "  pinned:     %s\n", res.SourceRevision)
	fmt.Fprintf(&b, "  phases:\n")
	for _, s := range steps {
		status := visibilityPhasePending
		if s.done {
			status = visibilityPhaseDone
		}
		fmt.Fprintf(&b, "    [%s] %s: %s\n", status, s.name, s.detail)
	}
	for _, c := range res.Commits {
		fmt.Fprintf(&b, "  commit:\n")
		fmt.Fprintf(&b, "    branch:  %s\n", c.Branch)
		fmt.Fprintf(&b, "    message: %s\n", c.Subject)
		fmt.Fprintf(&b, "    paths:\n")
		for _, p := range c.Paths {
			fmt.Fprintf(&b, "      %s\n", p)
		}
	}
	writeVisibilityWarnings(&b, res.Warnings)
	writeVisibilityPending(&b, res.PendingLocal)
	return strings.TrimRight(b.String(), "\n")
}

func writeVisibilityWarnings(b *strings.Builder, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintf(b, "  warnings:\n")
	for _, w := range warnings {
		fmt.Fprintf(b, "    - %s\n", w)
	}
}

func writeVisibilityPending(b *strings.Builder, pending []string) {
	if len(pending) == 0 {
		return
	}
	fmt.Fprintf(b, "  pending:\n")
	for _, p := range pending {
		fmt.Fprintf(b, "    - %s\n", p)
	}
}

// visibilityAppliedText renders a completed run.
func visibilityAppliedText(res RepositorySetVisibilityResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "repository set-visibility: %s -> %s (%s)\n", res.Current, res.Target, res.RepositoryState)
	for _, p := range res.Phases {
		fmt.Fprintf(&b, "  [%s] %s\n", p.Status, p.Name)
	}
	for _, c := range res.Commits {
		fmt.Fprintf(&b, "  commit %s on %s: %s (%s)\n", c.Commit, c.Branch, c.Subject, c.Status)
	}
	if res.BackupRemote != "" {
		fmt.Fprintf(&b, "  backup remote kept: %s\n", res.BackupRemote)
	}
	if res.Lessons != "" {
		fmt.Fprintf(&b, "  %s\n", lessonsText(res.Lessons))
	}
	writeVisibilityWarnings(&b, res.Warnings)
	writeVisibilityPending(&b, res.PendingLocal)
	return strings.TrimRight(b.String(), "\n")
}

// visibilityStopped maps a stopped run to its result, keeping the phases so far.
func visibilityStopped(res RepositorySetVisibilityResult, err error) RepositorySetVisibilityResult {
	var ref *visibilityRefusal
	var internal *visibilityInternal
	switch {
	case errors.As(err, &ref):
		res.Envelope = NewEnvelope(OperationRepositorySetVisibility, ResultInvalidState)
		res.RepositoryState = string(ref.state)
		res.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositorySetVisibility, ResultInvalidState, ref.state, ref.msg)
	case errors.As(err, &internal):
		res.Envelope = NewEnvelope(OperationRepositorySetVisibility, ResultInternalError)
		res.RepositoryState = string(reposetup.StateUnknown)
		res.human = fmt.Sprintf("%s: %s: %s", OperationRepositorySetVisibility, ResultInternalError, internal.Error())
	default:
		res.Envelope = NewEnvelope(OperationRepositorySetVisibility, ResultExternalFailed)
		res.RepositoryState = string(reposetup.StateUnknown)
		res.human = fmt.Sprintf("%s: %s: %s", OperationRepositorySetVisibility, ResultExternalFailed, err.Error())
	}
	var b strings.Builder
	b.WriteString(res.human + "\n")
	for _, p := range res.Phases {
		fmt.Fprintf(&b, "  [%s] %s\n", p.Status, p.Name)
	}
	if res.Lessons != "" {
		// The private instructions file is already gone: report its lessons
		// even though a later phase stopped the run.
		fmt.Fprintf(&b, "  %s\n", lessonsText(res.Lessons))
	}
	writeVisibilityPending(&b, res.PendingLocal)
	res.human = strings.TrimRight(b.String(), "\n")
	return res
}

// --- result constructors -----------------------------------------------------

func newVisibilityResult(result Result, target, current string, out RepositorySetVisibilityResult) RepositorySetVisibilityResult {
	out.Envelope = NewEnvelope(OperationRepositorySetVisibility, result)
	out.Target = target
	out.Current = current
	return out
}

// visibilityInvalidInput refuses a bad target or a misplaced flag.
func visibilityInvalidInput(target, msg string) RepositorySetVisibilityResult {
	out := newVisibilityResult(ResultInvalidInput, target, "", RepositorySetVisibilityResult{})
	out.human = fmt.Sprintf("%s: %s: %s", OperationRepositorySetVisibility, ResultInvalidInput, msg)
	return out
}

// visibilityRefusalResult builds an invalid-state refusal naming its remedy.
func visibilityRefusalResult(target, current string, state reposetup.State, remedy string) RepositorySetVisibilityResult {
	out := newVisibilityResult(ResultInvalidState, target, current, RepositorySetVisibilityResult{RepositoryState: string(state)})
	out.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositorySetVisibility, ResultInvalidState, state, remedy)
	return out
}

// visibilityExternalFailure builds an external-failed result naming the stage.
func visibilityExternalFailure(target, current string, state reposetup.State, stage string, err error) RepositorySetVisibilityResult {
	out := newVisibilityResult(ResultExternalFailed, target, current, RepositorySetVisibilityResult{RepositoryState: string(state)})
	out.human = fmt.Sprintf("%s: %s while %s: %s", OperationRepositorySetVisibility, ResultExternalFailed, stage, err.Error())
	return out
}

// visibilityFromMigrateResult re-stamps a shared gather refusal as a
// repository.set-visibility result, keeping its result, state, findings, and
// remedy text.
func visibilityFromMigrateResult(m RepositoryMigrateResult, target string) RepositorySetVisibilityResult {
	out := newVisibilityResult(m.Result, target, "", RepositorySetVisibilityResult{
		RepositoryState: m.RepositoryState,
		Findings:        m.Findings,
	})
	out.human = strings.Replace(m.HumanText(), OperationRepositoryMigrate, OperationRepositorySetVisibility, 1)
	return out
}
