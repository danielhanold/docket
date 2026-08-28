package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the `docket repository migrate` service: the explicit,
// authorized conversion of a legacy single-branch Bash-era repository onto the
// docket topology. The remote migration is a two-commit publication sequence
// under create-only / exact-lease protection, decided and acted on the ONE
// pinned integration revision (learning decide-and-act-on-the-same-copy): a
// parentless seed commit copies the whole changes/ADR/specs corpus onto the
// orphan metadata branch, then an integration descendant prunes the legacy
// planning surface. Every branch change keys on a re-read remote postcondition,
// never a local proxy (learning idempotency-keying), and no forbidden Git effect
// (force-push, foreign-ref deletion, published-branch rollback) is ever emitted.

// OperationRepositoryMigrate is the operation key `repository migrate` records.
const OperationRepositoryMigrate = "repository.migrate"

// migrateSeedSubject and migratePruneSubject are the two migration commit
// subjects. The seed is the parentless metadata root; the prune is the
// integration descendant that removes the legacy surface.
const (
	migrateSeedSubject  = "docket: seed metadata branch from migration"
	migratePruneSubject = "docket: prune legacy planning surface from integration"
)

// blobMode is the regular-file mode every migrated record/config/surface blob
// carries when composed into a tree.
const blobMode = gitcli.FileMode("100644")

// MigrateOptions carries the two-pass authorization the CLI resolves. Authorized
// is true only via --yes or an interactive confirmed preview; RepairAuthorized
// is --repair-frontmatter; ExpectedSource is the pinned integration OID the
// preview showed ("" on the first, preview, pass) — the service returns
// contended if the fresh authoritative integration tip has moved off it.
type MigrateOptions struct {
	Authorized       bool
	RepairAuthorized bool
	ExpectedSource   string
}

// RepositoryMigrateResult is the protocol-v1 document `repository migrate`
// returns. SourceRevision is the pinned integration OID the whole migration read
// and keyed on; MetadataTip and IntegrationTip name the two published commits;
// CopyPrefixes/RemovedPaths name the exact copy and removal sets; Repairs carries
// the applied frontmatter repairs; PendingLocal names the exact local
// synchronization remedy when the primary could not be fast-forwarded in place.
type RepositoryMigrateResult struct {
	Envelope
	RepositoryState string                    `json:"repository_state"`
	SourceRevision  string                    `json:"source_revision"`
	MetadataTip     string                    `json:"metadata_revision,omitempty"`
	IntegrationTip  string                    `json:"integration_revision,omitempty"`
	CopyPrefixes    []string                  `json:"copy_prefixes"`
	RemovedPaths    []string                  `json:"removed_paths"`
	Repairs         []reposetup.RepairFinding `json:"repairs,omitempty"`
	PendingLocal    []string                  `json:"pending_local,omitempty"`
	human           string
}

// HumanText renders the human summary: a refusal or preview carries its full
// text in the human string; a success names the migrated revisions and any
// pending local synchronization step.
func (r RepositoryMigrateResult) HumanText() string {
	if r.human != "" {
		return r.human
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s (%s)", r.Operation, r.Result, r.RepositoryState)
	if r.MetadataTip != "" {
		fmt.Fprintf(&b, "\nmetadata: %s", r.MetadataTip)
	}
	if r.IntegrationTip != "" {
		fmt.Fprintf(&b, "\nintegration: %s", r.IntegrationTip)
	}
	if len(r.PendingLocal) > 0 {
		fmt.Fprintf(&b, "\npending local sync: %s", strings.Join(r.PendingLocal, "; "))
	}
	return b.String()
}

// SourceRev exposes the pinned integration OID a preview showed, so the CLI's
// interactive confirm flow can re-invoke the service with ExpectedSource set to
// exactly the copy the human saw (learning decide-and-act-on-the-same-copy).
func (r RepositoryMigrateResult) SourceRev() string { return r.SourceRevision }

// migratePhase is the routing verdict over the gathered remote postconditions.
type migratePhase int

const (
	phaseRefuse          migratePhase = iota // a refusal was produced
	phaseLegacyFull                          // legacy: full seed + prune + local finish
	phaseResumePrune                         // partial (seed published, surface still live): resume at prune
	phaseResumeLocal                         // remote fully migrated, local attachment incomplete: local steps only
	phaseAlreadyMigrated                     // remote migrated and local attached: no-op
)

// RunRepositoryMigrate converts a legacy repository onto the docket topology
// under explicit authorization. It gathers facts, pins the ONE authoritative
// integration revision, routes on the re-read remote postconditions, and — once
// authorized — publishes the seed and prune commits under create-only/exact-lease
// protection with re-read verification between every branch change.
func RunRepositoryMigrate(ctx context.Context, d SetupDeps, o MigrateOptions) RepositoryMigrateResult {
	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		return migrateGatherFailure(err)
	}
	// Before any phase logic, remove exactly the owned transient worktrees/refs an
	// abrupt death of a prior invocation may have left (recognized by ownership
	// shape; a user worktree or ambiguous registration is preserved and reported).
	debris := sweepSetupDebris(ctx, d.Git, sc.repo)
	return mergeMigrateDebris(migratePhaseDispatch(ctx, d, o, facts, sc), debris)
}

// migratePhaseDispatch is RunRepositoryMigrate's body once facts are gathered and
// debris is swept: it routes on the re-read remote postconditions and drives the
// selected phase to a result. It is split out so the debris sweep can wrap it
// without threading its report through every branch.
func migratePhaseDispatch(ctx context.Context, d SetupDeps, o MigrateOptions, facts reposetup.Facts, sc setupContext) RepositoryMigrateResult {
	phase, refusal := migrateRoute(ctx, d.Git, facts, sc)
	if refusal != nil {
		return *refusal
	}
	if phase == phaseAlreadyMigrated {
		return migrateNoOp(sc.sourceRevision)
	}
	if phase == phaseResumeLocal {
		// The remote is fully migrated; only the local attachment is incomplete.
		// Perform only the clean fast-forward/worktree/hook/ownership steps that
		// are still provably safe — no remote write, no re-authorization.
		return migrateResumeLocal(ctx, d.Git, d.hooks, facts, sc)
	}

	sourceRevision := sc.sourceRevision
	if sourceRevision == "" {
		return migrateExternalFailure(reposetup.StateLegacy, "pinning the integration source revision",
			errors.New("no authoritative integration tip is available"))
	}
	// Decide-and-act on the same copy: an authorized re-invocation must be acting
	// on the exact revision its preview showed. A fresh tip means the remote moved.
	if migrateSourceMoved(o.ExpectedSource, sourceRevision) {
		return migrateContended(sourceRevision, o.ExpectedSource)
	}
	if facts.PrimaryClean == reposetup.PresenceAbsent {
		return migrateRefusal(reposetup.StateLegacy,
			"the primary worktree has uncommitted changes; commit or set them aside, then re-run")
	}

	// Read + validate the whole active/archived corpus and plan the closed
	// frontmatter repairs. This runs on the preview pass too, so the confirmation
	// preview carries the complete repair diff.
	mr, err := gatherMigrationRepairs(ctx, d.Git, sc, sourceRevision)
	if err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "reading the migration corpus", err)
	}

	// The ConfigEdit predicate reads the COMMITTED .docket.yml bytes at the
	// pinned source revision — the exact bytes the execution phase edits — so
	// the plan and the edit share one authority. An absent file is nil bytes.
	docketYML, _, yerr := readCommitBlob(ctx, d.Git, sc.repo, sourceRevision, ".docket.yml")
	if yerr != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "reading the committed .docket.yml", yerr)
	}
	plan, perr := reposetup.PlanMigration(sc.cfg, docketYML, sourceRevision, mr.repairable)
	if perr != nil {
		return migrateInternalFailure(reposetup.StateLegacy, "planning the migration", perr)
	}
	preview := migratePreviewText(sc, plan, mr, sourceRevision)

	// Two-pass authorization. An unauthorized run returns the full plan for
	// confirmation; an authorized run with repairs present but not opted in is
	// refused, naming --repair-frontmatter, before any write.
	switch decideMigrateAuthorization(o, len(mr.repairable) > 0) {
	case migrateConfirmRequired:
		return migrateConfirmationRequired(sourceRevision, plan, mr, preview)
	case migrateRepairRequired:
		return migrateRepairAuthorizationRequired(sourceRevision, plan, mr, preview)
	}

	// Authorized. Publish the remote postconditions, then finish locally.
	return migrateExecute(ctx, d.Git, d.hooks, facts, sc, plan, mr, phase)
}

// migrateRoute classifies the gathered remote postconditions into a migration
// phase or a refusal. When the metadata branch is present it fetches it and
// probes the root shape (metadataRootParentless) directly against the re-read
// remote tip — the base gatherer deliberately leaves the root shape unproven so
// the authoritative adopt/conflict decision keys on the promised remote state,
// not a gather-time proxy. Every probe error is surfaced, never read as a clean
// absence (learning probe-error-is-not-clean-absence).
func migrateRoute(ctx context.Context, git *gitcli.Client, facts reposetup.Facts, sc setupContext) (migratePhase, *RepositoryMigrateResult) {
	if u := migrateUnknownProbes(facts); len(u) > 0 {
		r := migrateRefusal(reposetup.StateUnknown,
			"a required repository probe could not be resolved ("+strings.Join(u, ", ")+"); run `docket repository check` after ensuring the remote is configured and reachable")
		return phaseRefuse, &r
	}
	if facts.DocketWorktree.Foreign {
		r := migrateRefusal(reposetup.StateConflict,
			"the .docket path is a foreign directory or conflicting registration; run `docket repository check` and resolve it manually")
		return phaseRefuse, &r
	}

	switch facts.RemoteMetadata.Presence {
	case reposetup.PresenceAbsent:
		if facts.LiveSurface == reposetup.PresencePresent {
			return phaseLegacyFull, nil
		}
		r := migrateRefusal(reposetup.StateFresh,
			"the repository has no legacy planning surface to migrate; run `docket repository init` to create the metadata topology")
		return phaseRefuse, &r
	case reposetup.PresencePresent:
		// Fetch the published metadata branch so its object is local, then re-read
		// its tip authoritatively (ls-remote gave only the id at gather time). The
		// branch decision keys on this re-read, never a local proxy.
		rev, ferr := git.FetchBranch(ctx, sc.repo, setupRemote(), gitcli.RefName(branchRefPrefix+sc.metadataBranch))
		if ferr != nil {
			r := migrateExternalFailure(reposetup.StateConflict, "re-reading the published metadata branch", ferr)
			return phaseRefuse, &r
		}
		parentless, err := metadataRootParentless(ctx, git, sc.repo, rev.Commit)
		if err != nil {
			r := migrateExternalFailure(reposetup.StateConflict, "inspecting the published metadata branch", err)
			return phaseRefuse, &r
		}
		if !parentless {
			// A foreign metadata advance (a non-orphan tip, or more than one root):
			// refuse. Never overwrite it.
			r := migrateRefusal(reposetup.StateConflict,
				"the remote docket branch is not a single-parentless-root docket metadata branch; inspect and resolve it manually, then run `docket repository check`")
			return phaseRefuse, &r
		}
		if facts.LiveSurface == reposetup.PresencePresent {
			// Seed published, integration still live: resume at the prune phase.
			return phaseResumePrune, nil
		}
		// Both remote postconditions are proven. If the local .docket attachment is
		// still incomplete, finish only the local steps; otherwise it is a no-op.
		if facts.DocketWorktree.Registered == reposetup.PresencePresent {
			return phaseAlreadyMigrated, nil
		}
		return phaseResumeLocal, nil
	default:
		r := migrateRefusal(reposetup.StateUnknown,
			"the remote docket branch presence could not be resolved; run `docket repository check`")
		return phaseRefuse, &r
	}
}

// migrateUnknownProbes lists the required probes the gatherer could not prove, so
// an errored probe routes to a check-me refusal rather than a fabricated absence.
func migrateUnknownProbes(f reposetup.Facts) []string {
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

// decideMigrateAuthorization is the pure two-pass authorization gate. Its three
// outcomes are the whole authorization matrix: an unauthorized run needs
// confirmation; an authorized run whose plan carries repairs the caller did not
// opt into needs --repair-frontmatter; everything else proceeds. Dropping the
// RepairAuthorized conjunct is the mutation probe the --yes-alone default test
// pins.
type migrateAuthorization int

const (
	migrateProceed migrateAuthorization = iota
	migrateConfirmRequired
	migrateRepairRequired
)

func decideMigrateAuthorization(o MigrateOptions, hasRepairable bool) migrateAuthorization {
	if !o.Authorized {
		return migrateConfirmRequired
	}
	if hasRepairable && !o.RepairAuthorized {
		return migrateRepairRequired
	}
	return migrateProceed
}

// migrateSourceMoved reports whether an authorized run's ExpectedSource no longer
// names the fresh authoritative integration tip — the decide-and-act-on-the-same-
// copy contention check. An empty ExpectedSource (the first, preview, pass) never
// contends.
func migrateSourceMoved(expected, fresh string) bool {
	return expected != "" && expected != fresh
}

// migrationRepairs is the read-and-planned corpus: the repairable findings (to
// apply and digest), the non-repairable diagnostics (for the preview), the
// repaired bytes per record, and every corpus record's original bytes so the
// zero-error precondition can validate the complete repaired candidate.
type migrationRepairs struct {
	repairable  []reposetup.RepairFinding
	diagnostics []reposetup.RepairFinding
	repaired    map[string][]byte
	archived    map[string]bool
	records     []corpusRecord
}

// gatherMigrationRepairs reads every active/archived change record (and the ADR
// and learnings records the snapshot validates alongside them) from the pinned
// source tree, plans the closed frontmatter repairs per change record, and
// applies the repairable ones in memory. It never writes and never touches a
// branch.
func gatherMigrationRepairs(ctx context.Context, git *gitcli.Client, sc setupContext, sourceRevision string) (migrationRepairs, error) {
	var mr migrationRepairs
	mr.repaired = map[string][]byte{}
	mr.archived = map[string]bool{}

	src, err := git.OpenObjectSource(ctx, sc.repo, gitcli.Revision{Commit: gitcli.ObjectID(sourceRevision)})
	if err != nil {
		return mr, err
	}
	entries, err := src.ListTree(ctx, corpusPrefixes(sc.cfg))
	if err != nil {
		return mr, err
	}
	type meta struct {
		kind     repository.RecordKind
		location repository.RecordLocation
	}
	var paths []gitcli.RepoPath
	var metas []meta
	for _, e := range entries {
		if e.Type != "blob" {
			continue
		}
		kind, loc, ok := classifyCorpusPath(sc.cfg, string(e.Path))
		if !ok {
			continue
		}
		paths = append(paths, e.Path)
		metas = append(metas, meta{kind: kind, location: loc})
	}
	if len(paths) == 0 {
		return mr, nil
	}
	blobs, err := src.ReadBlobs(ctx, paths)
	if err != nil {
		return mr, err
	}
	for i, br := range blobs {
		if !br.Found {
			continue
		}
		rec := corpusRecord{
			path:     string(br.Path),
			bytes:    br.Blob.Bytes,
			kind:     metas[i].kind,
			location: metas[i].location,
		}
		mr.records = append(mr.records, rec)
		if rec.kind != repository.KindChange {
			continue
		}
		archived := rec.location == repository.LocationArchive
		mr.archived[rec.path] = archived
		findings, perr := reposetup.PlanRepairs(rec.path, rec.bytes, archived)
		if perr != nil {
			continue
		}
		var repairableForRec []reposetup.RepairFinding
		for _, f := range findings {
			if f.Repairable {
				repairableForRec = append(repairableForRec, f)
				mr.repairable = append(mr.repairable, f)
			} else {
				mr.diagnostics = append(mr.diagnostics, f)
			}
		}
		if len(repairableForRec) > 0 {
			applied, aerr := reposetup.ApplyRepairs(rec.bytes, repairableForRec)
			if aerr != nil {
				return mr, aerr
			}
			mr.repaired[rec.path] = applied
		}
	}
	return mr, nil
}

// repairedCandidateErrors validates the COMPLETE repaired candidate: it parses
// every corpus record (substituting the repaired bytes where a repair applied)
// and returns the error-severity findings a whole-corpus BuildSnapshot names. A
// non-empty result blocks the migration before any branch change — this is the
// full-corpus validation the seed-publication guard depends on.
func repairedCandidateErrors(cfg config.Effective, mr migrationRepairs) []string {
	inputs := make([]repository.InputDocument, 0, len(mr.records))
	var blocking []string
	for _, r := range mr.records {
		src := r.bytes
		if rep, ok := mr.repaired[r.path]; ok {
			src = rep
		}
		doc, err := document.Parse(src)
		if err != nil {
			blocking = append(blocking, r.path+": frontmatter is undecodable")
			continue
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: r.kind, Location: r.location, Path: r.path, Document: doc,
		})
	}
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: cfg, Documents: inputs})
	if err != nil {
		return append(blocking, "corpus snapshot could not be built: "+err.Error())
	}
	for _, f := range build.Report.Findings() {
		if f.Severity != domain.SeverityError {
			continue
		}
		blocking = append(blocking, f.Entity.Path+": "+string(f.Code))
	}
	return blocking
}

// migrateExecute performs the authorized two-commit remote publication and the
// local finish. It requires zero error findings on the complete repaired
// candidate before composing any commit, publishes the seed under create-only
// protection, verifies its exact remote postcondition before pruning, publishes
// the integration descendant under an exact source-revision lease, verifies that
// postcondition, and only then attaches the local metadata worktree.
func migrateExecute(ctx context.Context, git *gitcli.Client, hooks setupHooks, facts reposetup.Facts, sc setupContext, plan reposetup.MigrationPlan, mr migrationRepairs, phase migratePhase) RepositoryMigrateResult {
	sourceRevision := sc.sourceRevision
	sourceOID := gitcli.ObjectID(sourceRevision)
	docketRef := gitcli.RefName(branchRefPrefix + sc.metadataBranch)
	integrationRef := gitcli.RefName(branchRefPrefix + sc.integrationBranch)

	// Full-corpus validation BEFORE any branch change. A non-repairable error in
	// the repaired candidate blocks the migration with nothing written.
	if blocking := repairedCandidateErrors(sc.cfg, mr); len(blocking) > 0 {
		return migrateBlocked(sourceRevision, blocking)
	}

	// Compose the seed tree: the whole copy-set prefixes plus every repaired
	// record's bytes. Only prefixes that exist in the source are mounted (an
	// absent prefix is not an error here — the copy set is spec-fixed, the source
	// may legitimately lack one).
	var seedOps []gitcli.TreeOp
	includedPrefixes := make([]string, 0, len(plan.Copy.Prefixes))
	for _, pfx := range plan.Copy.Prefixes {
		exists, err := sourcePrefixExists(ctx, git, sc.repo, sourceOID, pfx)
		if err != nil {
			return migrateExternalFailure(reposetup.StateLegacy, "probing the copy prefix "+pfx, err)
		}
		if !exists {
			continue
		}
		includedPrefixes = append(includedPrefixes, pfx)
		seedOps = append(seedOps, gitcli.TreeOp{IncludePrefix: &gitcli.IncludePrefixOp{From: sourceOID, Prefix: gitcli.RepoPath(pfx)}})
	}
	for _, p := range sortedRepairedPaths(mr.repaired) {
		seedOps = append(seedOps, gitcli.TreeOp{PutBlob: &gitcli.PutBlobOp{Path: gitcli.RepoPath(p), Content: mr.repaired[p], Mode: blobMode}})
	}
	seedTree, err := git.BuildTree(ctx, sc.repo, "", seedOps)
	if err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "composing the seed tree", err)
	}

	// The copy digest fingerprints exactly what the seed tree carries: git's own
	// content-addressed tree OID over the composed copy set (learning
	// idempotency-keying — a re-compose yields the same tree and the same digest).
	seedReceipt := plan.SeedReceipt
	seedReceipt.CopyDigest = string(seedTree)

	// Publish or adopt the metadata seed. On a resume (the seed already published,
	// integration still live) the durable remote seed is adopted — and, when the
	// pinned source has moved on since it was seeded, reconciled to the current
	// seed under its exact owned lease — rather than re-created.
	var metadataTip gitcli.ObjectID
	if phase == phaseResumePrune {
		tip, refusal := reconcileResumeSeed(ctx, git, hooks, sc, docketRef, seedReceipt, seedTree)
		if refusal != nil {
			return *refusal
		}
		metadataTip = tip
	} else {
		if err := fire(hooks.beforeSeedPush); err != nil {
			return migrateExternalFailure(reposetup.StateLegacy, "seeding the metadata branch (interrupted before publication)", err)
		}
		seedCommit, err := git.CommitTree(ctx, sc.repo, seedTree, nil, migrateSeedSubject, toGitcliTrailers(seedReceipt.Trailers()))
		if err != nil {
			return migrateExternalFailure(reposetup.StateLegacy, "creating the metadata seed commit", err)
		}
		tip, refusal := publishSeed(ctx, git, sc.repo, docketRef, seedCommit)
		if refusal != nil {
			return *refusal
		}
		metadataTip = tip
		if err := fire(hooks.afterSeedPush); err != nil {
			return migrateExternalFailure(reposetup.StateLegacy, "seeding the metadata branch (response lost after publication)", err)
		}
	}

	// Verify every seed path FROM THE REMOTE before pruning (learning
	// idempotency-keying / response-loss safety): the branch decision keys on the
	// re-read remote tree, never the local commit we believe we pushed. This is the
	// deferred mutation probe (c) target — reading the local ref here instead would
	// let a lost response or a tampered remote seed pass unnoticed.
	if refusal := verifySeedPublished(ctx, git, sc, docketRef, metadataTip, seedTree, includedPrefixes); refusal != nil {
		return *refusal
	}

	// Compose the integration descendant: prune the legacy planning surface,
	// remove the legacy config key (when present), establish the managed gitignore
	// block, and re-land any repaired ARCHIVED record (the archive stays on
	// integration; active records are removed with the prefix).
	pruneOps, removedPaths, err := composePruneOps(ctx, git, sc, sourceOID, plan, mr)
	if err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "composing the integration descendant", err)
	}
	pruneTree, err := git.BuildTree(ctx, sc.repo, sourceOID, pruneOps)
	if err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "composing the integration descendant tree", err)
	}
	pruneReceipt := plan.PruneReceipt
	pruneReceipt.MetadataRevision = string(metadataTip)
	pruneCommit, err := git.CommitTree(ctx, sc.repo, pruneTree, []gitcli.ObjectID{sourceOID}, migratePruneSubject, toGitcliTrailers(pruneReceipt.Trailers()))
	if err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "creating the integration prune commit", err)
	}

	if err := fire(hooks.beforePrunePush); err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "pruning the integration branch (interrupted before publication)", err)
	}

	// Publish the integration descendant under an exact source-revision lease: it
	// applies only if the remote integration tip is still exactly the pinned
	// source (learning cas-re-read-fresh-origin). A moved tip is contention, never
	// an overwrite.
	pushOut, err := git.PushLease(ctx, sc.repo, setupRemote(), integrationRef, pruneCommit, sourceOID)
	if err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "publishing the pruned integration branch", err)
	}
	switch pushOut.Disposition {
	case gitcli.PushApplied:
		// proceed
	case gitcli.PushLeaseLost:
		return migrateContended(string(pushOut.Remote), sourceRevision)
	default:
		return migrateExternalFailure(reposetup.StateLegacy, "publishing the pruned integration branch", errors.New("integration lease push failed"))
	}

	// Re-read the integration postcondition byte-exactly.
	integrationTip, err := git.FetchBranch(ctx, sc.repo, setupRemote(), integrationRef)
	if err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "re-reading the pruned integration branch", err)
	}
	if integrationTip.Commit != pruneCommit {
		return migrateContended(string(integrationTip.Commit), sourceRevision)
	}

	// Both remote postconditions are durable and re-read-verified. An afterPrunePush
	// seam that errors here simulates a lost response / abrupt death right after the
	// prune landed: the remote migration stands, and a re-run finishes locally.
	if err := fire(hooks.afterPrunePush); err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "pruning the integration branch (response lost after publication)", err)
	}

	// Both remote postconditions are durable. The beforeLocalFinish seam fires here
	// — the LocalMovedAfterPublish scenario advances the local primary through it
	// (returning nil so the finish still runs and reports the pending local sync).
	if err := fire(hooks.beforeLocalFinish); err != nil {
		return migrateExternalFailure(reposetup.StateLegacy, "finishing the local attachment", err)
	}

	pendingLocal := migrateLocalFinish(ctx, git, facts, sc, docketRef, metadataTip, sourceOID)

	return migrateApplied(sc, metadataTip, pruneCommit, sourceRevision, includedPrefixes, removedPaths, mr.repairable, pendingLocal)
}

// reconcileResumeSeed adopts the durable remote seed on a resume, reconciling it
// to the current source when needed. It re-reads the published metadata tip and
// tree from the remote (never a local proxy) and:
//   - if the published tree already equals the seed recomposed from the current
//     pinned source, adopts it as-is (a docket receipt is NOT required — an exact
//     legacy-equivalent bash-era seed proves the same postcondition);
//   - if the published tree differs but the seed carries docket's own seed
//     receipt naming a DIFFERENT source revision, the source has moved on since
//     the seed was published: it updates docket to the current seed under its
//     exact owned lease (never a force), which the caller then re-verifies;
//   - otherwise (no receipt we can trust, or a receipt claiming THIS source while
//     the tree disagrees — a tampered seed) it refuses as a conflict, destroying
//     nothing.
func reconcileResumeSeed(ctx context.Context, git *gitcli.Client, hooks setupHooks, sc setupContext, docketRef gitcli.RefName, seedReceipt reposetup.Receipt, seedTree gitcli.ObjectID) (gitcli.ObjectID, *RepositoryMigrateResult) {
	rev, err := git.FetchBranch(ctx, sc.repo, setupRemote(), docketRef)
	if err != nil {
		r := migrateExternalFailure(reposetup.StateConflict, "re-reading the published metadata seed", err)
		return "", &r
	}
	metadataTip := rev.Commit

	equal, err := remoteTreeEquals(ctx, git, sc.repo, metadataTip, seedTree)
	if err != nil {
		r := migrateExternalFailure(reposetup.StateConflict, "reading the published seed tree", err)
		return "", &r
	}
	if equal {
		// Already exactly the current seed (a docket seed re-run, or an exact
		// legacy-equivalent bash seed). Adopt it and proceed to prune.
		return metadataTip, nil
	}

	rec, ok, rerr := publishedSeedReceipt(ctx, git, sc.repo, metadataTip)
	if rerr != nil {
		r := migrateExternalFailure(reposetup.StateConflict, "reading the published seed receipt", rerr)
		return "", &r
	}
	if !ok || rec.Operation != reposetup.OpMigrateSeed || rec.SourceRevision == "" || rec.SourceRevision == sc.sourceRevision {
		// No receipt we can trust (a bash seed that is not exactly current), or a
		// receipt claiming the current source while the tree disagrees (a tampered
		// seed). Refuse — never overwrite what we cannot prove is a stale copy of
		// our own seed.
		r := migrateRefusal(reposetup.StateConflict,
			"the published metadata seed does not match the seed for the current source and is not a docket-authored seed for an earlier source we can safely update; inspect and resolve it manually, then run `docket repository check`")
		return "", &r
	}

	// Provably docket's own seed for an EARLIER source (the pinned source moved on
	// since it was published — planning bytes changed). Update docket to the
	// re-validated current seed under its exact owned lease.
	newReceipt := seedReceipt
	newReceipt.CopyDigest = string(seedTree)
	newSeed, cerr := git.CommitTree(ctx, sc.repo, seedTree, nil, migrateSeedSubject, toGitcliTrailers(newReceipt.Trailers()))
	if cerr != nil {
		r := migrateExternalFailure(reposetup.StateLegacy, "re-composing the updated metadata seed commit", cerr)
		return "", &r
	}
	// The owned-lease push keys on metadataTip — the FRESH re-read of the published
	// seed above (learning cas-re-read-fresh-origin). A second writer that advances
	// remote docket through this seam loses the lease here, never an overwrite: the
	// migration contends and the foreign advance stays intact.
	if err := fire(hooks.beforeMetadataLeasePush); err != nil {
		r := migrateExternalFailure(reposetup.StateLegacy, "updating the metadata seed under its owned lease (interrupted before publication)", err)
		return "", &r
	}
	out, perr := git.PushLease(ctx, sc.repo, setupRemote(), docketRef, newSeed, metadataTip)
	if perr != nil {
		r := migrateExternalFailure(reposetup.StateLegacy, "updating the metadata seed under its owned lease", perr)
		return "", &r
	}
	switch out.Disposition {
	case gitcli.PushApplied:
		return newSeed, nil
	case gitcli.PushLeaseLost:
		r := migrateContended(string(out.Remote), sc.sourceRevision)
		return "", &r
	default:
		r := migrateExternalFailure(reposetup.StateLegacy, "updating the metadata seed under its owned lease", errors.New("owned-lease seed update failed"))
		return "", &r
	}
}

// migrateResumeLocal finishes a migration whose remote postconditions are already
// durable but whose local .docket attachment is incomplete (a run that died
// after the prune landed, or a bash-migrated repository whose worktree was never
// attached). It performs ONLY the local steps — no remote write and no
// re-authorization — and reports the applied result with any pending local sync.
func migrateResumeLocal(ctx context.Context, git *gitcli.Client, hooks setupHooks, facts reposetup.Facts, sc setupContext) RepositoryMigrateResult {
	if err := fire(hooks.beforeLocalFinish); err != nil {
		return migrateExternalFailure(reposetup.StateHealthy, "finishing the local attachment", err)
	}
	metadataTip := gitcli.ObjectID(sc.metadataTip)
	sourceOID := gitcli.ObjectID(sc.sourceRevision)
	docketRef := gitcli.RefName(branchRefPrefix + sc.metadataBranch)
	pendingLocal := migrateLocalFinish(ctx, git, facts, sc, docketRef, metadataTip, sourceOID)
	return migrateApplied(sc, metadataTip, sourceOID, sc.sourceRevision, []string{}, []string{}, nil, pendingLocal)
}

// mergeMigrateDebris folds a debris sweep's operator-facing report into a
// successful migration result, so a preserved ambiguous registration or a
// retained-debris warning rides back to the caller. It never changes a refusal or
// a contended result — those already carry their own disposition.
func mergeMigrateDebris(res RepositoryMigrateResult, debris setupDebrisReport) RepositoryMigrateResult {
	lines := debris.pending()
	if len(lines) == 0 {
		return res
	}
	if res.Result == ResultApplied || res.Result == ResultNoOp {
		res.PendingLocal = append(res.PendingLocal, lines...)
		if res.RepositoryState == string(reposetup.StateHealthy) {
			res.RepositoryState = string(reposetup.StateNeedsReview)
		}
	}
	return res
}

// publishSeed publishes the parentless seed commit under create-only protection.
// A create rejection here is always a conflict refusal: publishSeed never adopts
// an existing remote branch — it does not compare the remote to seedCommit — and
// the create-only push is never widened to an overwriting lease (this is the
// create-only protection mutation probe target). Resume-time adoption of an
// already-published seed is owned by reconcileResumeSeed, not by this function.
func publishSeed(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, docketRef gitcli.RefName, seedCommit gitcli.ObjectID) (gitcli.ObjectID, *RepositoryMigrateResult) {
	outcome, err := git.PushCreateLease(ctx, repo, setupRemote(), docketRef, seedCommit)
	if err != nil {
		r := migrateExternalFailure(reposetup.StateLegacy, "publishing the metadata seed", err)
		return "", &r
	}
	switch outcome.Disposition {
	case gitcli.PushApplied:
		return seedCommit, nil
	case gitcli.PushLeaseLost:
		r := migrateRefusal(reposetup.StateConflict,
			"the remote docket branch already exists and is not this migration's seed; inspect and resolve it manually, then run `docket repository check`")
		return "", &r
	default:
		r := migrateExternalFailure(reposetup.StateLegacy, "publishing the metadata seed", errors.New("create-only push failed"))
		return "", &r
	}
}

// verifySeedPublished re-reads the metadata branch from the remote and proves it
// carries exactly the composed seed tree at exactly metadataTip before any prune.
func verifySeedPublished(ctx context.Context, git *gitcli.Client, sc setupContext, docketRef gitcli.RefName, metadataTip, seedTree gitcli.ObjectID, prefixes []string) *RepositoryMigrateResult {
	rev, err := git.FetchBranch(ctx, sc.repo, setupRemote(), docketRef)
	if err != nil {
		r := migrateExternalFailure(reposetup.StateLegacy, "re-reading the published metadata seed", err)
		return &r
	}
	if rev.Commit != metadataTip {
		r := migrateRefusal(reposetup.StateConflict,
			"the remote docket branch moved after publication; run `docket repository check` and resolve it manually")
		return &r
	}
	remoteTree, err := git.TreeOID(ctx, sc.repo, metadataTip)
	if err != nil {
		r := migrateExternalFailure(reposetup.StateLegacy, "reading the published seed tree", err)
		return &r
	}
	if remoteTree != seedTree {
		r := migrateRefusal(reposetup.StateConflict,
			"the published metadata seed tree does not match the composed seed; run `docket repository check` and resolve it manually")
		return &r
	}
	return nil
}

// composePruneOps builds the integration-descendant tree ops and the exact
// removed-path set the result reports. Base is the source tree; the active
// prefix, board, and README are pruned, the legacy config key removed (when
// present), the managed gitignore block established, and each repaired ARCHIVED
// record re-landed (the archive remains on integration).
func composePruneOps(ctx context.Context, git *gitcli.Client, sc setupContext, sourceOID gitcli.ObjectID, plan reposetup.MigrationPlan, mr migrationRepairs) ([]gitcli.TreeOp, []string, error) {
	var ops []gitcli.TreeOp
	var removed []string

	ops = append(ops, gitcli.TreeOp{RemovePrefix: &gitcli.RemovePrefixOp{Prefix: gitcli.RepoPath(plan.Removal.ActiveDir)}})
	removed = append(removed, plan.Removal.ActiveDir+"/")
	ops = append(ops, gitcli.TreeOp{RemovePath: &gitcli.RemovePathOp{Path: gitcli.RepoPath(plan.Removal.BoardPath)}})
	removed = append(removed, plan.Removal.BoardPath)
	ops = append(ops, gitcli.TreeOp{RemovePath: &gitcli.RemovePathOp{Path: gitcli.RepoPath(plan.Removal.ReadmePath)}})
	removed = append(removed, plan.Removal.ReadmePath)

	src, err := git.OpenObjectSource(ctx, sc.repo, gitcli.Revision{Commit: sourceOID})
	if err != nil {
		return nil, nil, err
	}

	// The legacy metadata_branch key edit, byte-preserving, only when present.
	if plan.ConfigEdit {
		edited, changed, cerr := editedDocketYML(ctx, src)
		if cerr != nil {
			return nil, nil, cerr
		}
		if changed {
			ops = append(ops, gitcli.TreeOp{PutBlob: &gitcli.PutBlobOp{Path: gitcli.RepoPath(".docket.yml"), Content: edited, Mode: blobMode}})
		}
	}

	// The managed .gitignore block, merged into any existing user content.
	gitignore, gerr := mergedGitignore(ctx, src)
	if gerr != nil {
		return nil, nil, gerr
	}
	ops = append(ops, gitcli.TreeOp{PutBlob: &gitcli.PutBlobOp{Path: gitcli.RepoPath(gitignoreRel), Content: gitignore, Mode: blobMode}})

	// Repaired ARCHIVED records stay on integration and must carry the repair.
	for _, p := range sortedRepairedPaths(mr.repaired) {
		if mr.archived[p] {
			ops = append(ops, gitcli.TreeOp{PutBlob: &gitcli.PutBlobOp{Path: gitcli.RepoPath(p), Content: mr.repaired[p], Mode: blobMode}})
		}
	}
	return ops, removed, nil
}

// editedDocketYML reads .docket.yml from the source and removes the top-level
// metadata_branch key byte-preserving. changed is false when the key is absent.
func editedDocketYML(ctx context.Context, src gitcli.ObjectSource) ([]byte, bool, error) {
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(".docket.yml")})
	if err != nil {
		return nil, false, err
	}
	if len(results) != 1 || !results[0].Found {
		return nil, false, nil
	}
	edited, removed, eerr := reposetup.RemoveMetadataBranchKey(results[0].Blob.Bytes)
	if eerr != nil {
		return nil, false, eerr
	}
	if !removed {
		return nil, false, nil
	}
	return edited, true, nil
}

// mergedGitignore reads the source .gitignore (if any) and returns its bytes with
// the managed docket block present exactly once.
func mergedGitignore(ctx context.Context, src gitcli.ObjectSource) ([]byte, error) {
	var current []byte
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(gitignoreRel)})
	if err != nil {
		return nil, err
	}
	if len(results) == 1 && results[0].Found {
		current = results[0].Blob.Bytes
	}
	out, _, gerr := reposetup.EnsureGitignoreBlock(current)
	if gerr != nil {
		return nil, gerr
	}
	return out, nil
}

// sourcePrefixExists reports whether prefix has any tree leaf in the source
// commit, so an absent copy-set prefix is skipped rather than erroring the build.
func sourcePrefixExists(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, sourceOID gitcli.ObjectID, prefix string) (bool, error) {
	src, err := git.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: sourceOID})
	if err != nil {
		return false, err
	}
	entries, err := src.ListTree(ctx, []gitcli.RepoPath{gitcli.RepoPath(prefix)})
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

// migrateLocalFinish attaches the persistent .docket worktree at the seed tip,
// disables its hooks, installs any authorized parent-facing surfaces, and returns
// the pending local synchronization remedy for the primary worktree. The remote
// migration is already durable, so a local step that cannot complete is reported
// as pending_local, never rolled back onto the remote.
func migrateLocalFinish(ctx context.Context, git *gitcli.Client, facts reposetup.Facts, sc setupContext, docketRef gitcli.RefName, metadataTip, sourceOID gitcli.ObjectID) []string {
	var pending []string
	worktreePath := filepath.Join(sc.repo.PrimaryWorktree, docketWorktreeName)
	if _, err := ensureMetadataWorktree(ctx, git, sc.repo, worktreePath, docketRef, metadataTip); err != nil {
		pending = append(pending, "attach the .docket worktree: `docket repository check` then re-run `docket repository migrate` ("+err.Error()+")")
	} else if herr := git.DisableWorktreeHooks(ctx, worktreePath); herr != nil {
		pending = append(pending, "disable the .docket worktree hooks: re-run `docket repository migrate` ("+herr.Error()+")")
	}
	if facts.SurfacesAuthorized {
		if _, _, serr := installAuthorizedSurfaces(ctx, git, sc.repo.PrimaryWorktree); serr != nil {
			pending = append(pending, "review and install the authorized parent-facing surfaces: `docket install` ("+serr.Error()+")")
		}
	}
	pending = append(pending, migratePrimarySyncRemedy(ctx, git, sc, sourceOID, metadataTip))
	return pending
}

// migratePrimarySyncRemedy names the exact command that brings the local primary
// in line with the migrated integration branch. The primary working-tree
// fast-forward is not performed in place — a clean-fast-forward working-tree
// primitive is intentionally outside this service's Git surface — so the remedy
// is state-branched: a primary still at the pinned source is a plain
// fast-forward; a primary that moved past it is a reconcile.
func migratePrimarySyncRemedy(ctx context.Context, git *gitcli.Client, sc setupContext, sourceOID, metadataTip gitcli.ObjectID) string {
	primary := sc.repo.PrimaryWorktree
	branch := sc.integrationBranch
	remote := string(setupRemote())
	moved := false
	if wts, err := git.ListWorktrees(ctx, sc.repo); err == nil {
		for _, wt := range wts {
			if filepath.Clean(wt.Path) != filepath.Clean(primary) {
				continue
			}
			if string(wt.Head) != string(sourceOID) {
				moved = true
			}
		}
	}
	if moved {
		return fmt.Sprintf("your local %s has moved past the migrated tip; reconcile it: `git -C %s pull --rebase %s %s`", branch, primary, remote, branch)
	}
	return fmt.Sprintf("fast-forward your primary worktree to the migrated integration branch: `git -C %s merge --ff-only %s/%s`", primary, remote, branch)
}

// sortedKeys returns the map keys sorted, for a deterministic op order.
func sortedRepairedPaths(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// migratePreviewText renders the full confirmation plan: the resolved repo,
// remote, exact integration revision, destination branch, copy set, removal set,
// config edit, and the complete repair diff.
func migratePreviewText(sc setupContext, plan reposetup.MigrationPlan, mr migrationRepairs, sourceRevision string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "docket repository migrate — plan\n")
	fmt.Fprintf(&b, "  repository:  %s\n", sc.repo.PrimaryWorktree)
	fmt.Fprintf(&b, "  remote:      %s\n", setupRemote())
	fmt.Fprintf(&b, "  integration: %s @ %s\n", sc.integrationBranch, sourceRevision)
	fmt.Fprintf(&b, "  destination: %s (orphan metadata branch)\n", sc.metadataBranch)
	fmt.Fprintf(&b, "  copy set:    %s\n", strings.Join(plan.Copy.Prefixes, ", "))
	fmt.Fprintf(&b, "  removal set: %s/, %s, %s\n", plan.Removal.ActiveDir, plan.Removal.BoardPath, plan.Removal.ReadmePath)
	fmt.Fprintf(&b, "  config edit: %s\n", migrateConfigEditText(plan.ConfigEdit))
	if len(mr.repairable) == 0 && len(mr.diagnostics) == 0 {
		fmt.Fprintf(&b, "  repairs:     none\n")
	} else {
		fmt.Fprintf(&b, "  repairs:\n")
		for _, f := range mr.repairable {
			fmt.Fprintf(&b, "    [repair %s] %s (%s)\n%s\n", f.Code, f.Path, f.Field, indentPatch(f.Patch))
		}
		for _, f := range mr.diagnostics {
			fmt.Fprintf(&b, "    [manual] %s (%s): %s\n", f.Path, f.Field, f.Message)
		}
	}
	return b.String()
}

// migrateConfigEditText names the one .docket.yml edit, or its absence.
func migrateConfigEditText(edit bool) string {
	if edit {
		return "remove the legacy metadata_branch key from .docket.yml"
	}
	return "none"
}

// indentPatch indents a repair patch preview so it reads as a nested block.
func indentPatch(patch []byte) string {
	if len(patch) == 0 {
		return "      (no diff)"
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(string(patch), "\n"), "\n") {
		fmt.Fprintf(&b, "      %s\n", line)
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- result constructors -----------------------------------------------------

// newMigrateResult stamps the envelope for a migration outcome.
func newMigrateResult(result Result, out RepositoryMigrateResult) RepositoryMigrateResult {
	out.Envelope = NewEnvelope(OperationRepositoryMigrate, result)
	return out
}

// migrateApplied is the success document naming both published revisions, the
// copy and removal sets, the applied repairs, and any pending local step.
func migrateApplied(sc setupContext, metadataTip, integrationTip gitcli.ObjectID, sourceRevision string, copyPrefixes, removedPaths []string, repairs []reposetup.RepairFinding, pendingLocal []string) RepositoryMigrateResult {
	state := reposetup.StateHealthy
	if len(pendingLocal) > 0 {
		state = reposetup.StateNeedsReview
	}
	out := newMigrateResult(ResultApplied, RepositoryMigrateResult{
		RepositoryState: string(state),
		SourceRevision:  sourceRevision,
		MetadataTip:     string(metadataTip),
		IntegrationTip:  string(integrationTip),
		CopyPrefixes:    copyPrefixes,
		RemovedPaths:    removedPaths,
		Repairs:         repairs,
		PendingLocal:    pendingLocal,
	})
	out.human = fmt.Sprintf("repository migrated: metadata %s, integration %s\npending local sync: %s",
		metadataTip, integrationTip, strings.Join(pendingLocal, "; "))
	return out
}

// migrateNoOp is the idempotent already-migrated document, keyed on the remote
// postconditions (metadata branch present, no live surface on integration).
func migrateNoOp(sourceRevision string) RepositoryMigrateResult {
	out := newMigrateResult(ResultNoOp, RepositoryMigrateResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  sourceRevision,
		CopyPrefixes:    []string{},
		RemovedPaths:    []string{},
	})
	out.human = "repository already migrated: the metadata branch is published and the legacy planning surface is gone"
	return out
}

// migrateConfirmationRequired is the unauthorized preview: the full plan, refused
// as invalid-state with reason confirmation-required. It names --yes.
func migrateConfirmationRequired(sourceRevision string, plan reposetup.MigrationPlan, mr migrationRepairs, preview string) RepositoryMigrateResult {
	out := newMigrateResult(ResultInvalidState, RepositoryMigrateResult{
		RepositoryState: "confirmation-required",
		SourceRevision:  sourceRevision,
		CopyPrefixes:    plan.Copy.Prefixes,
		RemovedPaths:    removalPaths(plan),
		Repairs:         mr.repairable,
	})
	out.human = preview + "\nconfirmation required: re-run with --yes to authorize this migration"
	return out
}

// migrateRepairAuthorizationRequired refuses an authorized run whose plan carries
// repairs the caller did not opt into, naming --repair-frontmatter, before any
// write.
func migrateRepairAuthorizationRequired(sourceRevision string, plan reposetup.MigrationPlan, mr migrationRepairs, preview string) RepositoryMigrateResult {
	out := newMigrateResult(ResultInvalidState, RepositoryMigrateResult{
		RepositoryState: "confirmation-required",
		SourceRevision:  sourceRevision,
		CopyPrefixes:    plan.Copy.Prefixes,
		RemovedPaths:    removalPaths(plan),
		Repairs:         mr.repairable,
	})
	out.human = preview + "\nmechanical frontmatter repairs are required: re-run with --repair-frontmatter (with --yes) to authorize them"
	return out
}

// migrateBlocked refuses the migration when the repaired candidate still carries
// a non-repairable error, before any branch change. It names the offending
// records.
func migrateBlocked(sourceRevision string, blocking []string) RepositoryMigrateResult {
	out := newMigrateResult(ResultInvalidState, RepositoryMigrateResult{
		RepositoryState: string(reposetup.StateLegacy),
		SourceRevision:  sourceRevision,
	})
	out.human = "migration blocked: the corpus carries non-repairable errors that must be fixed by hand first:\n  " +
		strings.Join(blocking, "\n  ")
	return out
}

// migrateContended reports that the authoritative integration tip differs from
// the copy the migration decided on. No branch was overwritten.
func migrateContended(freshTip, expected string) RepositoryMigrateResult {
	out := newMigrateResult(ResultContended, RepositoryMigrateResult{
		RepositoryState: string(reposetup.StateLegacy),
		SourceRevision:  expected,
		IntegrationTip:  freshTip,
	})
	out.human = fmt.Sprintf("migration contended: the integration branch moved to %s since the plan pinned %s; re-run to preview the new state",
		freshTip, expected)
	return out
}

// migrateRefusal builds an invalid-state refusal naming the classified state and
// a remedy valid in exactly that state.
func migrateRefusal(state reposetup.State, remedy string) RepositoryMigrateResult {
	out := newMigrateResult(ResultInvalidState, RepositoryMigrateResult{
		RepositoryState: string(state),
	})
	out.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryMigrate, ResultInvalidState, state, remedy)
	return out
}

// migrateGatherFailure maps a fact-gathering error to the migration result it
// classifies under, mirroring the init gatherer's mapping.
func migrateGatherFailure(err error) RepositoryMigrateResult {
	var rre *RepoResolutionError
	if errors.As(err, &rre) {
		out := newMigrateResult(ResultUnsupportedConfig, RepositoryMigrateResult{})
		out.human = fmt.Sprintf("%s: %s: %s", OperationRepositoryMigrate, ResultUnsupportedConfig, rre.Error())
		return out
	}
	if errors.Is(err, ErrStatusInvalidInput) {
		out := newMigrateResult(ResultInvalidInput, RepositoryMigrateResult{})
		out.human = fmt.Sprintf("%s: %s: %s", OperationRepositoryMigrate, ResultInvalidInput, err.Error())
		return out
	}
	out := newMigrateResult(ResultExternalFailed, RepositoryMigrateResult{})
	out.human = fmt.Sprintf("%s: %s: %s", OperationRepositoryMigrate, ResultExternalFailed, err.Error())
	return out
}

// migrateExternalFailure builds an external-failed result for a Git effect that
// failed mid-sequence, naming the stage.
func migrateExternalFailure(state reposetup.State, stage string, err error) RepositoryMigrateResult {
	out := newMigrateResult(ResultExternalFailed, RepositoryMigrateResult{
		RepositoryState: string(state),
	})
	out.human = fmt.Sprintf("%s: %s while %s: %s", OperationRepositoryMigrate, ResultExternalFailed, stage, err.Error())
	return out
}

// migrateInternalFailure builds an internal-error result for a defect-shaped
// failure mid-sequence.
func migrateInternalFailure(state reposetup.State, stage string, err error) RepositoryMigrateResult {
	out := newMigrateResult(ResultInternalError, RepositoryMigrateResult{
		RepositoryState: string(state),
	})
	out.human = fmt.Sprintf("%s: %s while %s: %s", OperationRepositoryMigrate, ResultInternalError, stage, err.Error())
	return out
}

// removalPaths renders the plan's removal set as the reported removed-path list.
func removalPaths(plan reposetup.MigrationPlan) []string {
	return []string{plan.Removal.ActiveDir + "/", plan.Removal.BoardPath, plan.Removal.ReadmePath}
}
