package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file is the `docket repository prepare` service: the sole shared Step-0
// operation that every operating skill runs before any typed mutation. It
// replaces the frozen Bash `docket.sh preflight` with one native, structured
// repository-preparation operation. It discovers the repository, pins the
// authoritative topology, classifies once, requires the fixed Go-v1 docket-branch
// topology, and — for a healthy remote whose local `.docket` checkout is absent or
// cleanly behind — attaches or fast-forwards it idempotently. It creates no
// planning record, makes no lifecycle decision, and pushes no remote mutation
// (spec §Purpose and interface). Attachment and fast-forward are the only Git
// effects, both keyed on the re-read remote metadata revision so a lost-response
// retry converges by re-reading topology (spec §State and refusal behavior).
//
// Legacy `DOCKET_*` preflight/env transport → typed PrepareContext mapping.
// Derived from `grep -rn 'DOCKET_[A-Z_]*' skills/ scripts/docket-status.sh
// scripts/lib/` over the maintained consumers. Each consumed variable is either
// carried as a typed PrepareContext field or dropped as unused transport (spec
// §Structured result: "does not retain legacy DOCKET_* transport variables merely
// because preflight once printed them"):
//
//   - DOCKET_SCRIPTS_DIR, DOCKET_BASH_PATH → DROPPED: facade-location transport;
//     the native binary needs no shell/script path.
//   - DOCKET_MODE → DROPPED: main mode is gone in Go v1 (only the fixed `docket`
//     metadata branch / `.docket` worktree remains), so there is nothing to carry.
//   - DOCKET_GI_START/END, DOCKET_GI_LEGACY_START/END, DOCKET_GI_CORE_ENTRIES,
//     DOCKET_GI_HARNESS_TOKENS, DOCKET_GI_DISPATCH_HARNESSES → DROPPED: managed
//     `.gitignore` block markers/entries, owned by init/migrate rendering, not a
//     Step-0 context value.
//   - DOCKET_SYNC_ATTEMPTS, DOCKET_SYNC_BACKOFF → DROPPED: the facade's shell
//     retry-tuning knobs; native sync is a single re-read-keyed effect.
//   - DOCKET_RUNTIME_VALUE, DOCKET_RUNTIME_DEEP, DOCKET_RUNTIME_COUNT → DROPPED:
//     runtime-probe transport, unrelated to repository preparation.
//   - DOCKET_LIVENESS_WHY, DOCKET_LIVENESS_CLASS → DROPPED: claim-liveness
//     transport owned by status/maintenance, not prepare.
//   - DOCKET_STATUSES(_ACTIVE/_TERMINAL), DOCKET_PRIORITIES, DOCKET_PRIORITY_DEFAULT,
//     DOCKET_CHANGE_TYPE_RESERVED, DOCKET_CHANGE_TYPES_DEFAULT → DROPPED: lifecycle
//     vocabulary owned by the domain/status layer; prepare carries topology, not
//     the change taxonomy.
//   - DOCKET_DISPATCH_RETENTION_DAYS → DROPPED: dispatch-retention policy owned by
//     maintenance sweep, not a Step-0 value.
//   - DOCKET_PREFLIGHT_TEST_SLEEP_CMD → DROPPED: a bash-test-only injection seam.
//
// Carried forward as typed fields (the resolved authority current consumers read):
//   - repository root / origin identity      → PrepareContext.RepoRoot / OriginURL
//   - default / integration / metadata branch → PrepareContext.{Default,Integration,Metadata}Branch
//     names plus their pinned …Revision fields
//   - fixed `.docket` worktree path           → PrepareContext.MetadataWorktreePath
//   - resolved changes / ADR / results dirs   → PrepareContext.{ChangesDir,AdrsDir,ResultsDir}
//   - supported finalize/test configuration   → PrepareContext.Finalize (mirrors the
//     supported config.Effective finalize fields)
//   - resolved workflow skill bindings        → PrepareContext.Skills
//   - configuration diagnostics               → PrepareContext.ConfigDiagnostics

// OperationRepositoryPrepare is the operation key `repository prepare` records.
const OperationRepositoryPrepare = "repository.prepare"

// The closed disposition vocabulary a prepare result carries (spec §Structured
// result). It is distinct from the protocol-v1 Envelope.Result taxonomy: the
// disposition is prepare's own applied/no-op/refused/error verdict, while the
// envelope maps it into the shared result family for exit-code presentation.
const (
	PrepareDispositionApplied = "applied"
	PrepareDispositionNoOp    = "no-op"
	PrepareDispositionRefused = "refused"
	PrepareDispositionError   = "error"
)

// PrepareOptions carries the only caller input: the repository directory. All
// configuration and topology facts are resolved from authoritative sources.
type PrepareOptions struct {
	RepoDir string
}

// PrepareFinalize mirrors exactly the supported config.Effective finalize fields
// — no more, so a future finalize field is a deliberate addition here, not an
// accidental generic passthrough.
type PrepareFinalize struct {
	Gate              string `json:"gate"`
	TestCommand       string `json:"test_command"`
	RequirePRApproval bool   `json:"require_pr_approval"`
}

// PrepareSkills carries the resolved workflow skill-role bindings. In Go v1 the
// skill roles are deferred capabilities: a valid repository leaves them unset, so
// these resolve to their empty defaults today. They are typed (never a generic
// map) so a later resolver fills named fields rather than reintroducing a stringly
// transport blob.
type PrepareSkills struct {
	Brainstorm string `json:"brainstorm,omitempty"`
	Plan       string `json:"plan,omitempty"`
	Build      string `json:"build,omitempty"`
	Review     string `json:"review,omitempty"`
	Finish     string `json:"finish,omitempty"`
}

// PrepareContext is the closed typed context a workflow carries forward (spec
// §Structured result). It is a closed struct derived from the consumer inventory:
// no generic string map, no DOCKET_* names, and no shell-quoted or eval-shaped
// values.
type PrepareContext struct {
	RepoRoot  string `json:"repo_root"`
	OriginURL string `json:"origin_url,omitempty"`

	DefaultBranch             string `json:"default_branch"`
	DefaultBranchRevision     string `json:"default_branch_revision,omitempty"`
	IntegrationBranch         string `json:"integration_branch"`
	IntegrationBranchRevision string `json:"integration_branch_revision,omitempty"`
	MetadataBranch            string `json:"metadata_branch"`
	MetadataBranchRevision    string `json:"metadata_branch_revision,omitempty"`

	MetadataWorktreePath string `json:"metadata_worktree_path"`

	ChangesDir string `json:"changes_dir"`
	AdrsDir    string `json:"adrs_dir"`
	ResultsDir string `json:"results_dir"`

	Finalize PrepareFinalize `json:"finalize"`
	Skills   PrepareSkills   `json:"skills"`

	ConfigDiagnostics []string `json:"config_diagnostics,omitempty"`
}

// RepositoryPrepareResult is the protocol-v1 document `repository prepare`
// returns. Disposition is the closed prepare vocabulary; Context is present only
// on an applied/no-op outcome; Findings carry the refusal/error diagnosis; Notices
// carry non-fatal repository-state observations (unresolved probe diagnostics).
type RepositoryPrepareResult struct {
	Envelope
	Disposition     string              `json:"disposition"`
	RepositoryState string              `json:"repository_state"`
	Context         *PrepareContext     `json:"context,omitempty"`
	Findings        []reposetup.Finding `json:"findings,omitempty"`
	Notices         []string            `json:"notices,omitempty"`
	human           string
}

// HumanText renders the human summary: the disposition and repository state, then
// one block per finding, then any notices. Agents consume JSON; this is the short
// redacted human view.
func (r RepositoryPrepareResult) HumanText() string {
	if r.human != "" {
		return r.human
	}
	var b strings.Builder
	fmt.Fprintf(&b, "repository prepare: %s (%s)", r.Disposition, r.RepositoryState)
	for _, f := range r.Findings {
		b.WriteString("\n")
		fmt.Fprintf(&b, "- [%s] %s", f.Severity, f.Code)
		if f.Message != "" {
			fmt.Fprintf(&b, "\n  %s", f.Message)
		}
		if f.Remedy != "" {
			fmt.Fprintf(&b, "\n  remedy: %s", f.Remedy)
		}
	}
	for _, n := range r.Notices {
		fmt.Fprintf(&b, "\nnotice: %s", n)
	}
	return b.String()
}

// prepareAction is the single Git effect a healthy-topology verdict plans. The
// zero value is prepareActionNone (a no-op / refusal plans nothing).
type prepareAction int

const (
	prepareActionNone prepareAction = iota
	prepareActionAttach
	prepareActionFastForward
)

// prepareSync is the ancestry relationship between the local metadata branch tip
// and the pinned remote metadata revision. The zero value is the SAFE
// prepareSyncUnknown — an unproven relationship never fast-forwards.
type prepareSync int

const (
	prepareSyncUnknown  prepareSync = iota
	prepareSyncCurrent              // local == remote
	prepareSyncBehind               // local is a strict ancestor of remote (fast-forwardable)
	prepareSyncAhead                // remote is a strict ancestor of local
	prepareSyncDiverged             // neither is an ancestor of the other
)

// prepareVerdict is prepareRoute's pure decision: the closed disposition, the
// planned Git effect (attach/fast-forward/none), the pinned target revision that
// effect keys on, the repository state to report, and the single finding a
// refusal/error carries. It is the whole refusal/attach/fast-forward matrix, made
// unit-testable without a real repository.
type prepareVerdict struct {
	disposition string
	action      prepareAction
	targetRev   string
	state       reposetup.State
	finding     *reposetup.Finding
}

// prepareRoute maps the gathered (and locally augmented) facts plus the computed
// sync relationship to a verdict. It is pure so every disposition row is pinned by
// a unit test asserting both the disposition and its mechanism (finding code and
// remedy). The ladder is deliberately ordered and fail-closed: an unresolved
// required topology probe is an error before anything else; a foreign `.docket`
// path refuses; no remote metadata is a fresh/legacy refusal naming the exact
// remedy; a non-parentless or still-live remote is refused without a local touch;
// only a proven-clean, registered, strictly-behind or current worktree fast-
// forwards or is a no-op, and an absent worktree attaches.
func prepareRoute(f reposetup.Facts, sync prepareSync) prepareVerdict {
	// 1. Fail-closed: a required topology probe that could not be resolved is an
	// error, never a fabricated absence that could authorize an attach.
	if u := prepareUnresolvedTopology(f); len(u) > 0 {
		return prepareErrorVerdict(reposetup.Finding{
			Code:     "prepare-topology-unresolved",
			Severity: reposetup.SeverityError,
			Message:  "A required repository topology probe could not be resolved: " + strings.Join(u, ", ") + ".",
			Remedy:   "Ensure the remote is configured and reachable, then run `docket repository check`.",
		})
	}

	// 2. A foreign `.docket` directory or conflicting registration refuses before
	// any topology decision — it is never overwritten or adopted.
	if f.DocketWorktree.Foreign {
		return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
			Code:     "docket-dir-foreign",
			Severity: reposetup.SeverityError,
			Ref:      docketWorktreeName,
			Message:  "The .docket path is a foreign directory or a conflicting worktree registration.",
			Remedy:   "Inspect the .docket path and resolve it manually with a human, then run `docket repository check`.",
		})
	}

	// 3. No remote metadata topology at all: fresh points at init, legacy at
	// migrate. Preparation never initializes or migrates implicitly.
	if f.RemoteMetadata.Presence == reposetup.PresenceAbsent {
		if f.LiveSurface == reposetup.PresencePresent {
			return prepareRefuseVerdict(reposetup.StateLegacy, reposetup.Finding{
				Code:     "repository-legacy",
				Severity: reposetup.SeverityError,
				Message:  "Legacy single-branch layout: a live planning surface exists without a docket metadata branch.",
				Remedy:   "Run `docket repository migrate` to convert this repository to the docket metadata topology.",
			})
		}
		return prepareRefuseVerdict(reposetup.StateFresh, reposetup.Finding{
			Code:     "repository-fresh",
			Severity: reposetup.SeverityError,
			Message:  "Repository is not initialized: no docket metadata branch and no live surface.",
			Remedy:   "Run `docket repository init` to create the docket metadata branch and worktree.",
		})
	}

	// 4. Require the fixed Go-v1 docket-branch topology: the remote metadata branch
	// must be a single parentless docket root. The metadata root is a three-valued
	// proof, and the two non-parentless values are NOT the same refusal. An
	// UNREADABLE root (RootUnknown — reachable while RemoteMetadata.Presence is
	// proven present, because the ls-remote presence probe and the object fetch are
	// independent) is a reachability refusal: the branch could not be read, so it is
	// never collapsed into foreign. Only a PROVEN foreign root (RootForeign,
	// readable evidence with no ownership proof) is the "resolve it with a human"
	// refusal. This mirrors reposetup.Classify — which guards `== RootForeign`
	// positively and reads RootUnknown as a generic conflict (postconditions-unmet),
	// never foreign — and repository_check.go's augmentCheckFacts, which feeds the
	// same RootUnknown through that classifier. Both refuse without a local touch.
	if f.MetadataRoot == reposetup.RootUnknown {
		return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
			Code:     "metadata-root-unresolved",
			Severity: reposetup.SeverityError,
			Message:  "The remote docket branch topology could not be read: its root ancestry is unproven.",
			Remedy:   "Ensure the remote is reachable and the docket branch is fetchable, then run `docket repository check`.",
		})
	}
	if f.MetadataRoot == reposetup.RootForeign {
		return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
			Code:     "metadata-root-foreign",
			Severity: reposetup.SeverityError,
			Message:  "The remote docket branch is not a docket-created parentless orphan root.",
			Remedy:   "Inspect the remote docket branch and resolve it manually with a human, then run `docket repository check`.",
		})
	}
	// A metadata branch published alongside a still-live integration surface is an
	// unfinished migration, not a preparable repository.
	if f.LiveSurface == reposetup.PresencePresent {
		return prepareRefuseVerdict(reposetup.StatePartial, reposetup.Finding{
			Code:     "migration-incomplete",
			Severity: reposetup.SeverityWarning,
			Message:  "A migration seeded the metadata branch but did not finish pruning the integration surface.",
			Remedy:   "The interrupted migration is safe to resume. Run `docket repository migrate`; it is idempotent.",
		})
	}

	// 5. Local `.docket` worktree. Absent is the idempotent attach; an unproven
	// presence is fail-closed.
	switch f.DocketWorktree.Presence {
	case reposetup.PresenceAbsent:
		return prepareApplyVerdict(prepareActionAttach, f.RemoteMetadata.Tip)
	case reposetup.PresenceUnknown:
		return prepareLocalUnknownVerdict()
	}

	// Present: it must be registered to this repository on the metadata branch.
	if f.DocketWorktree.Registered != reposetup.PresencePresent {
		return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
			Code:     "docket-worktree-ambiguous-registration",
			Severity: reposetup.SeverityError,
			Ref:      docketWorktreeName,
			Message:  "The .docket worktree is present but not provably registered to this repository on the metadata branch.",
			Remedy:   "Inspect the .docket worktree registration and resolve it manually with a human, then run `docket repository check`.",
		})
	}

	// A dirty (or unproven-clean) worktree refuses: prepare never overwrites,
	// resets, or stashes local content.
	switch f.DocketWorktree.Clean {
	case reposetup.PresenceAbsent:
		return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
			Code:     "metadata-worktree-dirty",
			Severity: reposetup.SeverityError,
			Ref:      docketWorktreeName,
			Message:  "The .docket metadata worktree has uncommitted or untracked changes.",
			Remedy:   "Commit or set aside the changes in the .docket metadata worktree, then re-run.",
		})
	case reposetup.PresenceUnknown:
		return prepareLocalUnknownVerdict()
	}

	// 6. Clean, registered worktree: synchronize by the pinned remote revision,
	// keyed on the ancestry relationship. Only a strictly-behind local fast-
	// forwards; ahead and diverged refuse without a touch.
	switch sync {
	case prepareSyncCurrent:
		return prepareVerdict{disposition: PrepareDispositionNoOp, action: prepareActionNone, state: reposetup.StateHealthy}
	case prepareSyncBehind:
		return prepareApplyVerdict(prepareActionFastForward, f.RemoteMetadata.Tip)
	case prepareSyncAhead:
		return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
			Code:     "local-metadata-ahead",
			Severity: reposetup.SeverityError,
			Message:  "The local docket branch is ahead of the remote docket branch (it carries commits the remote does not).",
			Remedy:   "Reconcile the local docket branch with the remote manually with a human before any repository operation.",
		})
	case prepareSyncDiverged:
		return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
			Code:     "local-metadata-diverged",
			Severity: reposetup.SeverityError,
			Message:  "The local docket branch has diverged from the remote docket branch.",
			Remedy:   "Reconcile the local and remote docket branches manually with a human before any repository operation.",
		})
	default:
		return prepareLocalUnknownVerdict()
	}
}

// prepareUnresolvedTopology lists the required topology probes prepare could not
// prove, mirroring the classifier's unknown ladder. A non-empty result is
// fail-closed: the repository state is undeterminable and no local effect is safe.
func prepareUnresolvedTopology(f reposetup.Facts) []string {
	var u []string
	if f.RemoteConfigured == reposetup.PresenceUnknown {
		u = append(u, "remote-configured")
	}
	if f.RemoteDefaultBranch.Presence == reposetup.PresenceUnknown {
		u = append(u, "remote-default-branch")
	}
	if f.RemoteIntegration.Presence == reposetup.PresenceUnknown {
		u = append(u, "remote-integration-branch")
	}
	if f.RemoteMetadata.Presence == reposetup.PresenceUnknown {
		u = append(u, "remote-metadata-branch")
	}
	if f.RemoteMetadata.Presence == reposetup.PresenceAbsent && f.LiveSurface == reposetup.PresenceUnknown {
		u = append(u, "live-surface")
	}
	return u
}

// prepareLocalUnknownVerdict is the shared refusal for a healthy-topology
// repository whose local worktree/sync state could not be proven. It refuses
// rather than guessing: an unproven local state never attaches or fast-forwards.
func prepareLocalUnknownVerdict() prepareVerdict {
	return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
		Code:     "prepare-local-state-unknown",
		Severity: reposetup.SeverityError,
		Ref:      docketWorktreeName,
		Message:  "The local .docket worktree or its synchronization with the remote metadata branch could not be resolved.",
		Remedy:   "Run `docket repository check` after ensuring the .docket worktree is readable and the remote is reachable.",
	})
}

// prepareApplyVerdict builds an applied verdict for a planned Git effect keyed on
// the pinned target revision.
func prepareApplyVerdict(action prepareAction, targetRev string) prepareVerdict {
	return prepareVerdict{disposition: PrepareDispositionApplied, action: action, targetRev: targetRev, state: reposetup.StateHealthy}
}

// prepareRefuseVerdict builds a refused verdict naming the classified state and a
// single finding whose remedy is valid in exactly that state.
func prepareRefuseVerdict(state reposetup.State, f reposetup.Finding) prepareVerdict {
	return prepareVerdict{disposition: PrepareDispositionRefused, action: prepareActionNone, state: state, finding: &f}
}

// prepareErrorVerdict builds an error verdict for an undeterminable repository
// state (a failed observation). The state is unknown and no local effect runs.
func prepareErrorVerdict(f reposetup.Finding) prepareVerdict {
	return prepareVerdict{disposition: PrepareDispositionError, action: prepareActionNone, state: reposetup.StateUnknown, finding: &f}
}

// buildPrepareContext assembles the closed typed context from the resolved
// configuration, the discovered repository, the pinned revisions, and the origin
// URL. It is pure so the typed-field wiring is unit-testable without Git: no
// generic map, no DOCKET_* key, no shell-quoted value.
func buildPrepareContext(cfg config.Effective, sc setupContext, f reposetup.Facts, originURL string) *PrepareContext {
	return &PrepareContext{
		RepoRoot:                  sc.repo.PrimaryWorktree,
		OriginURL:                 originURL,
		DefaultBranch:             sc.defaultBranch,
		DefaultBranchRevision:     f.RemoteDefaultBranch.Tip,
		IntegrationBranch:         sc.integrationBranch,
		IntegrationBranchRevision: f.RemoteIntegration.Tip,
		MetadataBranch:            reposetup.MetadataBranchName,
		MetadataBranchRevision:    f.RemoteMetadata.Tip,
		MetadataWorktreePath:      filepath.Join(sc.repo.PrimaryWorktree, docketWorktreeName),
		ChangesDir:                cfg.ChangesDir.Value,
		AdrsDir:                   cfg.ADRsDir.Value,
		ResultsDir:                cfg.ResultsDir.Value,
		Finalize: PrepareFinalize{
			Gate:              cfg.Finalize.Gate.Value,
			TestCommand:       cfg.Finalize.TestCommand.Value,
			RequirePRApproval: cfg.Finalize.RequirePRApproval.Value,
		},
		// Skills roles are deferred capabilities in Go v1; a valid repository leaves
		// them unset, so they resolve to their empty defaults today.
		Skills: PrepareSkills{},
	}
}

// RunRepositoryPrepare is the sole shared Step-0 operation. It discovers the
// repository and origin, pins the topology and loads configuration, gathers the
// classifier facts augmented with the local worktree/sync state, classifies once
// through prepareRoute, and — for a healthy topology — attaches an absent local
// `.docket` worktree or fast-forwards a clean strictly-behind one to the pinned
// remote metadata revision. Attachment and fast-forward are the only mutations;
// both are idempotent and keyed on the re-read remote state, so a lost-response
// retry converges. It never initializes or migrates implicitly and never pushes.
func RunRepositoryPrepare(ctx context.Context, d SetupDeps, o PrepareOptions) RepositoryPrepareResult {
	if o.RepoDir != "" {
		d.RepoDir = o.RepoDir
	}
	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		return prepareGatherFailure(err)
	}

	// Augment the base facts with the local metadata/worktree/sync postconditions a
	// healthy-topology decision needs. These probes run only when the remote
	// metadata branch is proven present; a fresh/legacy repository is decided from
	// the base facts alone. Every probe maps its own error to the safe Unknown
	// value, never a false absence.
	sync := prepareSyncUnknown
	if facts.RemoteMetadata.Presence == reposetup.PresencePresent {
		sync = prepareAugment(ctx, d.Git, &facts, sc)
	}

	verdict := prepareRoute(facts, sync)
	notices := prepareNotices(sc)

	switch verdict.disposition {
	case PrepareDispositionApplied:
		if execErr := prepareExecute(ctx, d.Git, sc, verdict); execErr != nil {
			return prepareEffectFailure(verdict, execErr, notices)
		}
		return prepareContextResult(ctx, d.Git, ResultApplied, PrepareDispositionApplied, sc, facts, notices)
	case PrepareDispositionNoOp:
		return prepareContextResult(ctx, d.Git, ResultNoOp, PrepareDispositionNoOp, sc, facts, notices)
	case PrepareDispositionRefused:
		return prepareDiagnosisResult(ResultInvalidState, PrepareDispositionRefused, verdict, notices)
	default: // PrepareDispositionError
		return prepareDiagnosisResult(ResultExternalFailed, PrepareDispositionError, verdict, notices)
	}
}

// prepareAugment fills the local metadata/worktree/sync facts the base gatherer
// deliberately leaves unproven, and returns the ancestry relationship between the
// local metadata tip and the pinned remote metadata revision. Every probe maps its
// own error to the safe Unknown value so a probe that could not run can never let
// the router read healthy.
func prepareAugment(ctx context.Context, git *gitcli.Client, f *reposetup.Facts, sc setupContext) prepareSync {
	metaRef := gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName)

	// Metadata root shape at the FETCHED remote docket tip (fetch it first so the
	// object is local on a clone that never fetched docket). The shared ownership
	// verifier — the same one repository_check.go's augmentCheckFacts consumes via
	// `verifyMetadataOwnership(` — decides whether the tip's sole parentless-root
	// lineage is a verified docket seed root (RootParentless: a native init/migrate
	// receipt or a receiptless legacy-equivalent tree, with ANY number of permitted
	// descendants and merges, so the root need NOT equal the tip — a real
	// multi-commit docket branch is owned, not foreign), readable evidence with no
	// ownership proof (RootForeign), or unreadable evidence (RootUnknown). This
	// replaces the earlier copied root-equals-tip predicate, which misclassified
	// every real multi-commit docket chain as RootForeign and refused it. A fetch
	// error is the safe RootUnknown, which the router refuses on rather than reading
	// parentless (never a stale object, never a false shape); the fetched tip is the
	// single authority the ownership proof is computed at.
	metaTip := sc.metadataTip
	if metaTip != "" {
		rev, ferr := git.FetchBranch(ctx, sc.repo, setupRemote(), metaRef)
		if ferr != nil {
			f.MetadataRoot = reposetup.RootUnknown
		} else {
			metaTip = string(rev.Commit)
			f.RemoteMetadata.Tip = metaTip
			own := verifyMetadataOwnership(ctx, git, sc.repo, rev.Commit, gitcli.ObjectID(sc.sourceRevision), sc.defaultBranch)
			f.MetadataRoot = own.Shape
		}
	}

	// Local metadata branch tip.
	if localTip, lerr := git.ResolveRef(ctx, sc.repo, metaRef); lerr == nil {
		f.LocalMetadata.Presence = reposetup.PresencePresent
		f.LocalMetadata.Tip = string(localTip)
	} else if fail, ok := gitcli.AsFailure(lerr); ok && fail.Kind == gitcli.KindRefUnavailable {
		f.LocalMetadata.Presence = reposetup.PresenceAbsent
	}

	// The .docket worktree clean state, probed only when the worktree is present.
	if f.DocketWorktree.Presence == reposetup.PresencePresent {
		f.DocketWorktree.Clean = worktreeCleanPresence(ctx, git, filepath.Join(sc.repo.PrimaryWorktree, docketWorktreeName))
		f.DocketWorktree.Synchronized = synchronizedPresence(f.LocalMetadata, f.RemoteMetadata)
	}

	return prepareSyncRelationship(ctx, git, sc.repo, f.LocalMetadata.Tip, metaTip)
}

// prepareSyncRelationship computes the ancestry relationship between the local
// metadata tip and the pinned remote metadata revision. An empty tip or a probe
// error is the safe Unknown — an unproven relationship never fast-forwards.
func prepareSyncRelationship(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, localTip, remoteTip string) prepareSync {
	if localTip == "" || remoteTip == "" {
		return prepareSyncUnknown
	}
	if localTip == remoteTip {
		return prepareSyncCurrent
	}
	local := gitcli.ObjectID(localTip)
	remote := gitcli.ObjectID(remoteTip)
	localBehind, err := git.IsAncestor(ctx, repo, local, remote)
	if err != nil {
		return prepareSyncUnknown
	}
	remoteBehind, err := git.IsAncestor(ctx, repo, remote, local)
	if err != nil {
		return prepareSyncUnknown
	}
	switch {
	case localBehind:
		return prepareSyncBehind
	case remoteBehind:
		return prepareSyncAhead
	default:
		return prepareSyncDiverged
	}
}

// prepareExecute performs the single planned idempotent Git effect. Attachment
// materializes the local checkout of the already-valid remote topology;
// fast-forward advances a clean strictly-behind worktree to the pinned remote
// revision. Both key on the re-read remote state so a lost-response retry
// converges by re-reading topology.
func prepareExecute(ctx context.Context, git *gitcli.Client, sc setupContext, verdict prepareVerdict) error {
	worktreePath := filepath.Join(sc.repo.PrimaryWorktree, docketWorktreeName)
	metaRef := gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName)
	target := gitcli.ObjectID(verdict.targetRev)
	switch verdict.action {
	case prepareActionAttach:
		if _, err := ensureMetadataWorktree(ctx, git, sc.repo, worktreePath, metaRef, target); err != nil {
			return err
		}
		return git.DisableWorktreeHooks(ctx, worktreePath)
	case prepareActionFastForward:
		return prepareFastForwardWorktree(ctx, git, sc.repo, worktreePath, metaRef, target)
	default:
		return nil
	}
}

// prepareFastForwardWorktree advances the clean, strictly-behind `.docket`
// worktree to the pinned remote metadata revision. The precondition (clean AND a
// strict ancestor of the target) is proven by the router, so removing the clean
// worktree, deleting the checked-out branch at its exact known tip, and re-adding
// the branch at the target loses no content and is a true fast-forward composed
// from the existing worktree primitives. It is idempotent: a re-run whose worktree
// is already absent re-attaches at the same target.
func prepareFastForwardWorktree(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, worktreePath string, metaRef gitcli.RefName, target gitcli.ObjectID) error {
	oldTip, err := git.ResolveRef(ctx, repo, metaRef)
	if err != nil {
		return err
	}
	if err := git.RemoveWorktreeClean(ctx, repo, worktreePath); err != nil {
		return err
	}
	if err := git.DeleteLocalBranchChecked(ctx, repo, metaRef, oldTip); err != nil {
		return err
	}
	if err := git.AddBranchWorktree(ctx, repo, worktreePath, metaRef, target); err != nil {
		return err
	}
	return git.DisableWorktreeHooks(ctx, worktreePath)
}

// prepareContextResult builds an applied/no-op result carrying the closed typed
// context. The origin URL is a best-effort read: a failure leaves it empty and is
// not fatal to preparation.
func prepareContextResult(ctx context.Context, git *gitcli.Client, result Result, disposition string, sc setupContext, facts reposetup.Facts, notices []string) RepositoryPrepareResult {
	originURL, _ := git.RemoteURL(ctx, sc.repo, setupRemote())
	pc := buildPrepareContext(sc.cfg, sc, facts, originURL)
	out := RepositoryPrepareResult{
		Envelope:        NewEnvelope(OperationRepositoryPrepare, result),
		Disposition:     disposition,
		RepositoryState: string(reposetup.StateHealthy),
		Context:         pc,
		Notices:         notices,
	}
	out.human = fmt.Sprintf("repository prepare: %s (%s); metadata worktree %s", disposition, reposetup.StateHealthy, pc.MetadataWorktreePath)
	return out
}

// prepareDiagnosisResult builds a refused/error result carrying the single
// diagnosis finding and no context.
func prepareDiagnosisResult(result Result, disposition string, verdict prepareVerdict, notices []string) RepositoryPrepareResult {
	var findings []reposetup.Finding
	if verdict.finding != nil {
		findings = []reposetup.Finding{*verdict.finding}
	}
	out := RepositoryPrepareResult{
		Envelope:        NewEnvelope(OperationRepositoryPrepare, result),
		Disposition:     disposition,
		RepositoryState: string(verdict.state),
		Findings:        findings,
		Notices:         notices,
	}
	remedy := ""
	if verdict.finding != nil {
		remedy = ": " + verdict.finding.Remedy
	}
	out.human = fmt.Sprintf("repository prepare: %s (%s)%s", disposition, verdict.state, remedy)
	return out
}

// prepareEffectFailure downgrades an applied verdict to an error when its Git
// effect failed mid-sequence. The remote topology is untouched (prepare pushes
// nothing), so a re-run re-reads and retries.
func prepareEffectFailure(verdict prepareVerdict, err error, notices []string) RepositoryPrepareResult {
	stage := "attaching the .docket worktree"
	if verdict.action == prepareActionFastForward {
		stage = "fast-forwarding the .docket worktree"
	}
	out := RepositoryPrepareResult{
		Envelope:        NewEnvelope(OperationRepositoryPrepare, ResultExternalFailed),
		Disposition:     PrepareDispositionError,
		RepositoryState: string(reposetup.StateNeedsReview),
		Notices:         notices,
	}
	out.human = fmt.Sprintf("repository prepare: %s while %s: %s", PrepareDispositionError, stage, err.Error())
	return out
}

// prepareGatherFailure maps a fact-gathering error to a prepare result. An invalid
// configuration is a refused (unsupported-config) outcome — reached before any
// synchronization probe or effect; invalid input is an error (invalid-input);
// everything else is an error (external-failed). None reaches prepareRoute, so no
// local effect can run on a gather failure.
func prepareGatherFailure(err error) RepositoryPrepareResult {
	var rre *RepoResolutionError
	switch {
	case errors.As(err, &rre):
		out := RepositoryPrepareResult{
			Envelope:    NewEnvelope(OperationRepositoryPrepare, ResultUnsupportedConfig),
			Disposition: PrepareDispositionRefused,
		}
		out.human = fmt.Sprintf("repository prepare: %s: %s", PrepareDispositionRefused, rre.Error())
		return out
	case errors.Is(err, ErrStatusInvalidInput):
		out := RepositoryPrepareResult{
			Envelope:    NewEnvelope(OperationRepositoryPrepare, ResultInvalidInput),
			Disposition: PrepareDispositionError,
		}
		out.human = fmt.Sprintf("repository prepare: %s: %s", PrepareDispositionError, err.Error())
		return out
	default:
		out := RepositoryPrepareResult{
			Envelope:    NewEnvelope(OperationRepositoryPrepare, ResultExternalFailed),
			Disposition: PrepareDispositionError,
		}
		out.human = fmt.Sprintf("repository prepare: %s: %s", PrepareDispositionError, err.Error())
		return out
	}
}

// prepareNotices lifts the retained probe diagnostics into operator-facing
// notices, so an unresolved probe that did not change the disposition is never
// silent.
func prepareNotices(sc setupContext) []string {
	var out []string
	for _, d := range sc.diagnostics {
		if d.Err != nil {
			out = append(out, d.Probe+": "+d.Err.Error())
		}
	}
	return out
}
