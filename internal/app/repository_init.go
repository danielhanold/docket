package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/reposetup"
)

// OperationRepositoryInit is the operation key `repository init` records.
const OperationRepositoryInit = "repository.init"

// initRootSubject is the commit subject of the orphan metadata root. It mirrors
// reposetup.initRootSubject; init composes the root directly (rather than through
// reposetup.PlanInit) because it must attempt the authoritative create-only
// publish from any not-yet-published state, whereas PlanInit refuses every
// non-fresh input by design.
const initRootSubject = "docket: initialize metadata branch"

// gitignoreRel is the repo-relative managed gitignore path the init edit writes
// and names as a pending review path.
const gitignoreRel = ".gitignore"

// docketYMLRel is the repo-relative config path the generated test-policy edit
// writes (init and migrate share it).
const docketYMLRel = ".docket.yml"

// RepositoryOpResult is the protocol-v1 document a repository mutation
// (init/migrate) returns. RepositoryState carries the reposetup.State the
// operation ended in; needs-review is a field on an applied/no-op result (never
// a new Result spelling), and PendingPaths names the integration-worktree edits
// left unstaged for human review. Findings carries any health findings a refusal
// wants to surface.
type RepositoryOpResult struct {
	Envelope
	RepositoryState string              `json:"repository_state"`
	PendingPaths    []string            `json:"pending_paths,omitempty"`
	MetadataTip     string              `json:"metadata_revision,omitempty"`
	SourceRevision  string              `json:"source_revision,omitempty"`
	Findings        []reposetup.Finding `json:"findings,omitempty"`
	human           string
}

// HumanText renders the one-line human summary; a refusal carries its remedy in
// the human string, and a success names the state and every pending path.
func (r RepositoryOpResult) HumanText() string {
	if r.human != "" {
		return r.human
	}
	line := fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.RepositoryState)
	if len(r.PendingPaths) > 0 {
		line += "\npending review: " + strings.Join(r.PendingPaths, ", ")
	}
	return line
}

// newRepositoryOpResult stamps the envelope for a repository mutation outcome.
func newRepositoryOpResult(operation string, result Result, out RepositoryOpResult) RepositoryOpResult {
	out.Envelope = NewEnvelope(operation, result)
	return out
}

// RunRepositoryInit initializes the docket metadata topology on a fresh
// repository and converges idempotently on a re-run. It classifies once, refuses
// every non-fresh state the spec forbids (legacy → migrate, unknown/foreign →
// check, a dirty primary → the supported contract), then performs the six init
// effects: a parentless empty-tree root with a versioned receipt, a create-only
// publish that adopts an already-published exact shape and refuses a foreign one,
// the local branch + persistent .docket worktree, disabled worktree hooks, the
// unstaged managed .gitignore edit plus (only when authorized) the parent-facing
// dispatch surfaces, and the ownership record for the surfaces it owns. It never
// prompts and never reads stdin.
func RunRepositoryInit(ctx context.Context, d SetupDeps) RepositoryOpResult {
	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		return repositoryGatherFailure(OperationRepositoryInit, err)
	}

	cls, refusal := initGuard(facts)
	if refusal != nil {
		return *refusal
	}

	// Recovery: remove exactly the owned transient worktrees/refs an abrupt death
	// of a prior invocation may have left (recognized by ownership shape; a user
	// worktree or ambiguous registration is preserved and reported). Its report
	// rides back on the pending review paths.
	debris := sweepSetupDebris(ctx, d.Git, sc.repo)

	// Effects 1–2: build a parentless empty-tree root with the OpInitRoot receipt
	// and publish it under create-only protection. The create-only push is the
	// authoritative metadata operation: on success we created it; on a lost lease
	// the shared ownership verifier decides at the reread remote tip — adopt a
	// verified init-equivalent lineage AT ITS TIP (descendants preserved), or
	// refuse a migration-seeded, foreign, or unreadable branch — the remote is
	// never overwritten or reset to the seed.
	metaRef := gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName)
	metadataTip, createdRemote, refusal := publishOrAdoptMetadataRoot(ctx, d.Git, sc.repo, metaRef, sc.sourceRevision, sc.defaultBranch)
	if refusal != nil {
		return *refusal
	}

	// Effect 3: create or adopt the local branch and attach the persistent
	// root-level .docket worktree. Idempotent — a re-run finds it already
	// registered and does nothing.
	worktreePath := filepath.Join(sc.repo.PrimaryWorktree, docketWorktreeName)
	createdWorktree, err := ensureMetadataWorktree(ctx, d.Git, sc.repo, worktreePath, metaRef, metadataTip)
	if err != nil {
		return repositoryExternalFailure(OperationRepositoryInit, cls.State, "attaching the .docket worktree", err)
	}

	// Effect 4: disable Git hooks for the metadata worktree. Idempotent.
	if err := d.Git.DisableWorktreeHooks(ctx, worktreePath); err != nil {
		return repositoryExternalFailure(OperationRepositoryInit, cls.State, "disabling metadata-worktree hooks", err)
	}

	// Effect 5: prepare the managed .gitignore edit (unstaged) and, only when an
	// explicit repository/repository-local agent_harnesses declaration authorizes
	// them, the parent-facing dispatch surfaces via the same install machinery the
	// installer's repository phase uses.
	pending := []string{gitignoreRel}
	wroteGitignore, err := ensureManagedGitignore(sc.repo.PrimaryWorktree)
	if err != nil {
		return repositoryInternalFailure(OperationRepositoryInit, cls.State, "preparing the managed .gitignore block", err)
	}

	wroteSurfaces := false
	if facts.SurfacesAuthorized {
		surfacePending, changed, serr := installAuthorizedSurfaces(ctx, d.Git, sc.repo.PrimaryWorktree)
		if serr != nil {
			return mapSurfaceFailure(cls.State, serr)
		}
		pending = append(pending, surfacePending...)
		wroteSurfaces = changed
	}

	// Test policy: discover the suite from the primary worktree and write the
	// generated `.docket.yml` edit as another pending, UNSTAGED review path —
	// exactly the managed-.gitignore posture. Generated config is human-gated, so
	// init never stages it; an ambiguous outcome writes nothing and rides back as
	// a note rather than failing init.
	docketYMLPending, wroteDocketYML, discovery, derr := ensureTestPolicyConfig(sc.repo.PrimaryWorktree, sc.cfg)
	if derr != nil {
		return repositoryInternalFailure(OperationRepositoryInit, cls.State, "generating the test-policy config", derr)
	}
	if docketYMLPending != "" {
		pending = append(pending, docketYMLPending)
	}

	pending = append(pending, debris.pending()...)
	sort.Strings(pending)

	// The metadata topology is established with pending integration-worktree
	// edits: the state is needs-review. A run that changed nothing reports no-op;
	// one that created a remote branch, worktree, block, or surface reports
	// applied. Both exit 0.
	result := ResultNoOp
	if createdRemote || createdWorktree || wroteGitignore || wroteSurfaces || wroteDocketYML {
		result = ResultApplied
	}

	out := newRepositoryOpResult(OperationRepositoryInit, result, RepositoryOpResult{
		RepositoryState: string(reposetup.StateNeedsReview),
		PendingPaths:    pending,
		MetadataTip:     string(metadataTip),
		SourceRevision:  sc.sourceRevision,
	})
	out.human = fmt.Sprintf("repository initialized (%s); review and commit the pending paths: %s",
		reposetup.StateNeedsReview, strings.Join(pending, ", "))
	if note := testDiscoveryNote(discovery); note != "" {
		out.human += "\n" + note
	}
	return out
}

// ensureTestPolicyConfig discovers the suite from the primary worktree and, when
// a write applies, writes the generated `.docket.yml` test policy UNSTAGED to the
// working tree (the managed-.gitignore posture: generated config is human-gated,
// never staged). It returns the pending review path ("" when nothing was
// written), whether it changed the file, and the discovery outcome so the caller
// can report an ambiguous result. A malformed existing config or a probe fault is
// an error with the file untouched.
func ensureTestPolicyConfig(primaryWorktree string, cfg config.Effective) (pendingPath string, wrote bool, outcome reposetup.DiscoveryOutcome, err error) {
	tree := newOSTree(primaryWorktree)
	docketYMLAbs := filepath.Join(primaryWorktree, docketYMLRel)
	existing, rerr := os.ReadFile(docketYMLAbs)
	if rerr != nil {
		if !os.IsNotExist(rerr) {
			return "", false, reposetup.DiscoveryOutcome{}, rerr
		}
		existing = nil
	}
	bytes, outcome, perr := reposetup.TestPolicyEdit(cfg, existing, tree)
	if perr != nil {
		return "", false, reposetup.DiscoveryOutcome{}, perr
	}
	if bytes == nil {
		return "", false, outcome, nil
	}
	if werr := os.WriteFile(docketYMLAbs, bytes, 0o644); werr != nil {
		return "", false, outcome, werr
	}
	return docketYMLRel, true, outcome, nil
}

// testDiscoveryNote renders the operator-facing note for an ambiguous test
// discovery: it names the candidate families and the remedy so a human resolves
// the choice. A non-ambiguous outcome has no note.
func testDiscoveryNote(outcome reposetup.DiscoveryOutcome) string {
	if outcome.Kind != reposetup.DiscoveryAmbiguous {
		return ""
	}
	fams := make([]string, 0, len(outcome.Candidates))
	for _, c := range outcome.Candidates {
		fams = append(fams, c.Family)
	}
	return fmt.Sprintf("test discovery was ambiguous (%s); no test policy was written — run `docket repository configure-tests` to choose one",
		strings.Join(fams, ", "))
}

// initGuard classifies once and returns the refusal for every state init must
// never create from, plus the supported-contract preflight. It is pure over the
// gathered facts so the refusal mapping is unit-testable without a real
// repository. A nil refusal means init may proceed to publish. Legacy points at
// migrate; an unknown probe, a foreign .docket directory, a dirty primary, or a
// primary off the remote integration tip points at the remedy valid in that
// exact state.
func initGuard(facts reposetup.Facts) (reposetup.Classification, *RepositoryOpResult) {
	cls := reposetup.Classify(facts)
	switch {
	case cls.State == reposetup.StateLegacy:
		r := initRefusal(reposetup.StateLegacy,
			"repository has a legacy single-branch planning surface; run `docket repository migrate` to convert it")
		return cls, &r
	case cls.State == reposetup.StateUnknown:
		r := initRefusal(reposetup.StateUnknown,
			"a required repository probe could not be resolved; run `docket repository check` after ensuring the remote is configured and reachable")
		return cls, &r
	case facts.DocketWorktree.Foreign:
		r := initRefusal(reposetup.StateConflict,
			"the .docket path is a foreign directory or conflicting registration; run `docket repository check` and resolve it manually")
		return cls, &r
	}
	if facts.PrimaryClean == reposetup.PresenceAbsent {
		r := initRefusal(cls.State,
			"the primary worktree has uncommitted changes; commit or set them aside, then re-run")
		return cls, &r
	}
	if facts.PrimaryAtRemoteTip == reposetup.PresenceAbsent {
		r := initRefusal(cls.State,
			"the primary worktree is not at the authoritative remote integration tip; synchronize it, then re-run")
		return cls, &r
	}
	return cls, nil
}

// publishOrAdoptMetadataRoot builds a parentless empty-tree root carrying the
// OpInitRoot receipt and publishes it under create-only protection. It returns
// the metadata tip (the root it created, or the reread remote tip of an
// already-published init-equivalent lineage it adopted), whether it created the
// remote branch, and a non-nil refusal when the remote holds a migration-seeded,
// foreign, or unreadable branch. The create-only push is never widened to an
// overwriting lease — that is the guard the create-only protection mutation probe
// strips — and adoption of the reread tip never re-pushes or resets to the seed.
// sourceRevision (the pinned integration tip) and defaultBranch thread into the
// shared ownership verifier for the lost-lease inspection.
func publishOrAdoptMetadataRoot(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, metaRef gitcli.RefName, sourceRevision, defaultBranch string) (gitcli.ObjectID, bool, *RepositoryOpResult) {
	emptyTree, err := git.EmptyTreeOID(ctx, repo)
	if err != nil {
		r := repositoryExternalFailure(OperationRepositoryInit, reposetup.StateFresh, "resolving the empty tree", err)
		return "", false, &r
	}
	trailers := toGitcliTrailers(reposetup.Receipt{Operation: reposetup.OpInitRoot}.Trailers())
	root, err := git.CommitTree(ctx, repo, emptyTree, nil, initRootSubject, trailers)
	if err != nil {
		r := repositoryExternalFailure(OperationRepositoryInit, reposetup.StateFresh, "creating the metadata root commit", err)
		return "", false, &r
	}

	outcome, err := git.PushCreateLease(ctx, repo, setupRemote(), metaRef, root)
	if err != nil {
		r := repositoryExternalFailure(OperationRepositoryInit, reposetup.StateFresh, "publishing the metadata branch", err)
		return "", false, &r
	}

	switch outcome.Disposition {
	case gitcli.PushApplied:
		return root, true, nil
	case gitcli.PushLeaseLost:
		// The ref already exists at outcome.Remote. Fetch the object so the shared
		// ownership verifier can inspect its lineage locally at the reread tip, then
		// adopt a verified init-equivalent lineage (descendants preserved) or refuse
		// a migration-seeded, foreign, or unreadable branch. The create-only push
		// never overwrote it, and adoption of the reread tip must not either.
		if _, ferr := git.FetchBranch(ctx, repo, setupRemote(), metaRef); ferr != nil {
			r := repositoryExternalFailure(OperationRepositoryInit, reposetup.StateConflict, "reading the published metadata branch", ferr)
			return "", false, &r
		}
		own := verifyMetadataOwnership(ctx, git, repo, outcome.Remote, gitcli.ObjectID(sourceRevision), defaultBranch)
		switch own.Shape {
		case reposetup.RootUnknown:
			// Incomplete or unreadable evidence: an external failure retaining the
			// probe error, never a fabricated foreign or a silent adoption.
			r := repositoryExternalFailure(OperationRepositoryInit, reposetup.StateConflict, "inspecting the published metadata branch", own.Err)
			return "", false, &r
		case reposetup.RootForeign:
			r := initRefusal(reposetup.StateConflict,
				"the remote docket branch is not a verified docket metadata branch; inspect and resolve it manually, then run `docket repository check`")
			return "", false, &r
		}
		// Verified (RootParentless). Only an init-equivalent lineage — a native
		// OpInitRoot seed or a receiptless legacy-bootstrap empty root — is
		// init-adoptable; a migration-seeded lineage is recognized but refused, so a
		// broadened proof never becomes permission to initialize.
		if !initEquivalent(own) {
			r := initRefusal(reposetup.StateConflict,
				"the remote docket branch is an established migrated metadata branch; `docket repository init` cannot adopt it — run `docket repository check`")
			return "", false, &r
		}
		// Adopt the reread remote tip: descendants preserved, never re-pushed, never
		// reset to the seed.
		return outcome.Remote, false, nil
	default:
		r := repositoryExternalFailure(OperationRepositoryInit, reposetup.StateFresh, "publishing the metadata branch", errors.New("create-only push failed"))
		return "", false, &r
	}
}

// initEquivalent reports whether a verified lineage is init-adoptable: its
// verified ROOT carries the empty tree — a native OpInitRoot seed, or the
// receiptless legacy-bootstrap empty root. It tests the root's tree, never the
// current tip's tree, so descendants are permitted and preserved. A
// migration-seeded lineage (proofMigrateReceipt / proofLegacyEquivalent) is NOT
// init-equivalent: recognizing it never becomes permission to initialize.
func initEquivalent(own metadataOwnership) bool {
	return own.Shape == reposetup.RootParentless &&
		(own.Proof == proofInitReceipt || own.Proof == proofLegacyEmpty)
}

// ensureMetadataWorktree creates or adopts the local metadata branch and attaches
// the persistent .docket worktree. It is idempotent: an already-registered
// worktree on the metadata branch is left exactly as it is (no second worktree).
func ensureMetadataWorktree(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, worktreePath string, metaRef gitcli.RefName, metadataTip gitcli.ObjectID) (bool, error) {
	wts, err := git.ListWorktrees(ctx, repo)
	if err != nil {
		return false, err
	}
	for _, wt := range wts {
		if filepath.Clean(wt.Path) == filepath.Clean(worktreePath) && wt.Branch == metaRef {
			return false, nil // already attached on the metadata branch
		}
	}
	// A local metadata branch may already exist (a prior interrupted run); attach
	// it. Otherwise create the branch at the adopted tip and its worktree together.
	if _, rerr := git.ResolveRef(ctx, repo, metaRef); rerr == nil {
		if err := git.AttachBranchWorktree(ctx, repo, worktreePath, metaRef); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := git.AddBranchWorktree(ctx, repo, worktreePath, metaRef, metadataTip); err != nil {
		return false, err
	}
	return true, nil
}

// ensureManagedGitignore reads the primary worktree's .gitignore, computes the
// canonical managed-block form, and writes it back UNSTAGED when it differs. It
// returns whether it changed the file. Malformed managed markers are an error and
// the file is left untouched.
func ensureManagedGitignore(primaryWorktree string) (bool, error) {
	gitignorePath := filepath.Join(primaryWorktree, gitignoreRel)
	current, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	out, changed, err := reposetup.EnsureGitignoreBlock(current)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	if err := os.WriteFile(gitignorePath, out, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// installAuthorizedSurfaces plans the parent-facing dispatch surfaces for the
// repository's explicit opt-in and applies them through the same install target
// machinery the installer's repository phase uses: it inspects each target
// against the repository's own ownership record, refuses an unprovable surface,
// writes the reconciled surfaces UNSTAGED to the working tree, and publishes the
// ownership record — all in one journaled transaction. It returns the surface
// paths to name as pending review and whether anything changed.
func installAuthorizedSurfaces(ctx context.Context, git *gitcli.Client, primaryWorktree string) ([]string, bool, error) {
	runGate, err := buildRunGate()
	if err != nil {
		return nil, false, err
	}
	phase, _, err := ResolveRepoPhase(ctx, git, primaryWorktree, nil, runGate, nil)
	if err != nil {
		return nil, false, err
	}
	if phase == nil || !phase.Authorized {
		return nil, false, nil
	}

	roots, err := install.ResolveRoots(os.UserHomeDir, os.Getenv)
	if err != nil {
		return nil, false, err
	}

	changed, err := applyRepoPhaseSurfaces(phase, roots)
	if err != nil {
		return nil, false, err
	}

	pending := make([]string, 0, len(phase.Targets))
	for _, t := range phase.Targets {
		rel, rerr := filepath.Rel(filepath.Clean(primaryWorktree), filepath.Clean(t.Path))
		if rerr != nil {
			return nil, false, rerr
		}
		pending = append(pending, filepath.ToSlash(rel))
	}
	return pending, changed, nil
}

// applyRepoPhaseSurfaces inspects and writes the repository phase's surfaces and
// publishes its ownership record in one journaled install transaction. A surface
// that is not provably docket's refuses the whole apply before any write. A phase
// already converged (every surface a no-op and the record already exactly on
// disk) opens no transaction. It returns whether anything was written.
func applyRepoPhaseSurfaces(phase *install.RepoPhase, roots install.UserRoots) (bool, error) {
	inspections := make([]install.Inspection, 0, len(phase.Targets))
	steps := 0
	for _, t := range phase.Targets {
		insp, err := install.InspectTarget(t, phase.PriorState, nil)
		if err != nil {
			return false, err
		}
		if insp.Disposition == install.DispositionConflict {
			return false, fmt.Errorf("repository surface %s is not provably docket's", t.Path)
		}
		if insp.Disposition != install.DispositionNoop {
			steps++
		}
		inspections = append(inspections, insp)
	}

	recordSettled, err := repoRecordAlreadyOnDisk(phase.RecordPath, phase.RecordBytes)
	if err != nil {
		return false, err
	}
	if steps == 0 && len(phase.Removals) == 0 && recordSettled {
		return false, nil
	}

	txn, err := install.BeginTxnWithRemovals(install.RealFS{}, roots, inspections, phase.Removals)
	if err != nil {
		return false, err
	}
	if err := txn.Apply(); err != nil {
		return false, err
	}
	var docs []install.StateDoc
	if phase.RecordBytes != nil {
		docs = append(docs, install.StateDoc{Path: phase.RecordPath, Bytes: phase.RecordBytes})
	}
	if err := txn.CommitDocs(docs); err != nil {
		return false, err
	}
	return steps > 0 || len(phase.Removals) > 0, nil
}

// repoRecordAlreadyOnDisk reports whether the ownership record on disk already
// holds exactly the bytes this run would publish. An absent record is a
// difference; a read error other than "absent" is returned rather than mistaken
// for one.
func repoRecordAlreadyOnDisk(recordPath string, recordBytes []byte) (bool, error) {
	if recordBytes == nil {
		return true, nil
	}
	existing, err := os.ReadFile(recordPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return string(existing) == string(recordBytes), nil
}

// buildRunGate renders the dispatch run-gate payload from the embedded asset
// catalog — the same source the installer's repository phase renders surfaces
// from — so a repository init and an install agree on surface bytes.
func buildRunGate() ([]byte, error) {
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		return nil, err
	}
	return harness.RunGate(catalog)
}

// toGitcliTrailers maps reposetup's gitcli-free trailer pairs to gitcli.Trailer,
// the one place the two representations meet at the commit boundary.
func toGitcliTrailers(in []reposetup.Trailer) []gitcli.Trailer {
	out := make([]gitcli.Trailer, len(in))
	for i, t := range in {
		out[i] = gitcli.Trailer{Key: t.Key, Value: t.Value}
	}
	return out
}

// initRefusal builds a repository-init refusal: an invalid-state envelope naming
// the classified state and carrying the human remedy. The remedy is valid in
// exactly the state that produced it.
func initRefusal(state reposetup.State, remedy string) RepositoryOpResult {
	out := newRepositoryOpResult(OperationRepositoryInit, ResultInvalidState, RepositoryOpResult{
		RepositoryState: string(state),
	})
	out.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryInit, ResultInvalidState, state, remedy)
	return out
}

// repositoryGatherFailure maps a fact-gathering error to the repository result it
// classifies under: an invalid-configuration resolution error is unsupported
// config, an invalid-input discovery failure is invalid-input, and everything
// else is an external failure.
func repositoryGatherFailure(operation string, err error) RepositoryOpResult {
	var rre *RepoResolutionError
	if errors.As(err, &rre) {
		out := newRepositoryOpResult(operation, ResultUnsupportedConfig, RepositoryOpResult{})
		out.human = fmt.Sprintf("%s: %s: %s", operation, ResultUnsupportedConfig, rre.Error())
		return out
	}
	if errors.Is(err, ErrStatusInvalidInput) {
		out := newRepositoryOpResult(operation, ResultInvalidInput, RepositoryOpResult{})
		out.human = fmt.Sprintf("%s: %s: %s", operation, ResultInvalidInput, err.Error())
		return out
	}
	out := newRepositoryOpResult(operation, ResultExternalFailed, RepositoryOpResult{})
	out.human = fmt.Sprintf("%s: %s: %s", operation, ResultExternalFailed, err.Error())
	return out
}

// repositoryExternalFailure builds an external-failed result for a Git effect
// that failed mid-sequence, naming the stage and retaining the state.
func repositoryExternalFailure(operation string, state reposetup.State, stage string, err error) RepositoryOpResult {
	out := newRepositoryOpResult(operation, ResultExternalFailed, RepositoryOpResult{
		RepositoryState: string(state),
	})
	out.human = fmt.Sprintf("%s: %s while %s: %s", operation, ResultExternalFailed, stage, err.Error())
	return out
}

// repositoryInternalFailure builds an internal-error result for a defect-shaped
// failure (a malformed managed block, an encoding failure) mid-sequence.
func repositoryInternalFailure(operation string, state reposetup.State, stage string, err error) RepositoryOpResult {
	out := newRepositoryOpResult(operation, ResultInternalError, RepositoryOpResult{
		RepositoryState: string(state),
	})
	out.human = fmt.Sprintf("%s: %s while %s: %s", operation, ResultInternalError, stage, err.Error())
	return out
}

// mapSurfaceFailure maps a surface-installation failure to its result: an
// unprovable surface or ownership conflict is invalid-state (a human must
// resolve it), a resolution error carries its own reason, and everything else is
// an external failure.
func mapSurfaceFailure(state reposetup.State, err error) RepositoryOpResult {
	var rre *RepoResolutionError
	if errors.As(err, &rre) {
		out := newRepositoryOpResult(OperationRepositoryInit, ResultInvalidState, RepositoryOpResult{
			RepositoryState: string(state),
		})
		out.human = fmt.Sprintf("%s: %s: %s", OperationRepositoryInit, ResultInvalidState, rre.Error())
		return out
	}
	if strings.Contains(err.Error(), "not provably docket's") {
		out := newRepositoryOpResult(OperationRepositoryInit, ResultInvalidState, RepositoryOpResult{
			RepositoryState: string(state),
		})
		out.human = fmt.Sprintf("%s: %s: %s", OperationRepositoryInit, ResultInvalidState, err.Error())
		return out
	}
	return repositoryExternalFailure(OperationRepositoryInit, state, "installing parent-facing surfaces", err)
}
