package app

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// This file is the read-only `repository check` service. It gathers the same
// bounded authoritative reads init/migrate gather (a targeted remote fetch is
// the only Git effect it performs — no working-tree, index, local-branch,
// config, ownership, worktree-registration, or remote-ref write), augments them
// with the check-only postcondition probes the healthy classification needs,
// classifies once, and returns machine-readable health findings with a 0/1/2
// exit that encodes a non-failure. Every committed-ignore guarantee is proven
// from the integration COMMIT tree, never the working tree (learning
// gitignore-guarantee-must-be-committed).

// OperationRepositoryCheck is the operation key `repository check` records.
const OperationRepositoryCheck = "repository.check"

// RepositoryCheckResult is the protocol-v1 document `repository check` returns.
// It is read-only, so a determinable read is a no-op success and an
// undeterminable authority is an invalid-state family result; neither maps its
// process exit through app.ExitCode — CheckExitCode owns the 0/1/2 contract.
type RepositoryCheckResult struct {
	Envelope
	RepositoryState string              `json:"repository_state"`
	Findings        []reposetup.Finding `json:"findings"`
	Revisions       map[string]string   `json:"revisions"`
	human           string
}

// HumanText renders the human summary: the repository state, then one block per
// finding naming its code, severity, ref, message, and remedy.
func (r RepositoryCheckResult) HumanText() string {
	if r.human != "" {
		return r.human
	}
	var b strings.Builder
	fmt.Fprintf(&b, "repository check: %s (%s)", r.Result, r.RepositoryState)
	for _, f := range r.Findings {
		b.WriteString("\n")
		fmt.Fprintf(&b, "- [%s] %s", f.Severity, f.Code)
		if f.Ref != "" {
			fmt.Fprintf(&b, " (%s)", f.Ref)
		}
		if f.Message != "" {
			fmt.Fprintf(&b, "\n  %s", f.Message)
		}
		if f.Remedy != "" {
			fmt.Fprintf(&b, "\n  remedy: %s", f.Remedy)
		}
	}
	return b.String()
}

// CheckExitCode maps the classified state and its findings to the 0/1/2 check
// contract via reposetup.CheckExit. CheckExit reads only the state and whether
// any findings exist, so it is reconstructed from the result's own fields — 1
// (diagnosed action required) is not a hard failure and JSON consumers read
// findings, never the exit code (learning exit-code-encodes-a-non-failure).
func (r RepositoryCheckResult) CheckExitCode() int {
	return reposetup.CheckExit(reposetup.Classification{State: reposetup.State(r.RepositoryState)}, r.Findings)
}

// RunRepositoryCheck reports repository health without writing anything. It
// gathers the authoritative remote/topology facts, augments them with the
// check-only postcondition probes, gathers report-only frontmatter findings from
// the metadata corpus, classifies once, and returns the ordered findings and the
// 0/1/2 exit.
func RunRepositoryCheck(ctx context.Context, d SetupDeps) RepositoryCheckResult {
	facts, sc, err := GatherSetupFacts(ctx, d, false)
	if err != nil {
		return checkGatherFailure(err)
	}

	// The metadata-topology postconditions are meaningful only once the remote
	// docket branch is proven present; a fresh/legacy repository classifies from
	// the base facts alone. Every augmentation probe is a bounded read that maps
	// its own errors to the safe Unknown value, never to a false absence.
	var fm []reposetup.RepairFinding
	if facts.RemoteMetadata.Presence == reposetup.PresencePresent {
		augmentCheckFacts(ctx, d.Git, &facts, sc)
		fm = gatherFrontmatterFindings(ctx, d.Git, sc)
	}

	cls := reposetup.Classify(facts)
	findings := reposetup.EvaluateHealth(cls, facts, fm)
	return newCheckResult(cls, facts, findings)
}

// newCheckResult stamps the envelope and human text for a computed check
// outcome. A determinable state is a read-only no-op; an undeterminable
// authority (state unknown) is an invalid-state family result. The exit is
// computed separately by CheckExitCode.
func newCheckResult(cls reposetup.Classification, facts reposetup.Facts, findings []reposetup.Finding) RepositoryCheckResult {
	result := ResultNoOp
	if cls.State == reposetup.StateUnknown {
		result = ResultInvalidState
	}
	out := RepositoryCheckResult{
		Envelope:        NewEnvelope(OperationRepositoryCheck, result),
		RepositoryState: string(cls.State),
		Findings:        findings,
		Revisions:       checkRevisions(facts),
	}
	return out
}

// checkRevisions names the authoritative revisions the check compared, so a JSON
// consumer sees exactly what was read.
func checkRevisions(facts reposetup.Facts) map[string]string {
	rev := map[string]string{}
	if facts.RemoteDefaultBranch.Tip != "" {
		rev["remote-default"] = facts.RemoteDefaultBranch.Tip
	}
	if facts.RemoteIntegration.Tip != "" {
		rev["remote-integration"] = facts.RemoteIntegration.Tip
	}
	if facts.RemoteMetadata.Tip != "" {
		rev["remote-metadata"] = facts.RemoteMetadata.Tip
	}
	if facts.LocalMetadata.Tip != "" {
		rev["local-metadata"] = facts.LocalMetadata.Tip
	}
	return rev
}

// augmentCheckFacts fills the postcondition facts the base gatherer deliberately
// leaves unproven (the metadata root shape is not inspected at gather time; the
// local worktree/branch state is a read preflight only check needs). Each probe
// performs a bounded read and maps its own error to the safe Unknown/zero value,
// never to a false absence — so a probe that could not run can never let the
// classifier read healthy.
func augmentCheckFacts(ctx context.Context, git *gitcli.Client, f *reposetup.Facts, sc setupContext) {
	metaRef := gitcli.RefName(branchRefPrefix + sc.metadataBranch)

	// Metadata root shape: a single parentless root reachable from the tip is the
	// docket orphan shape; extra parents, a non-root tip, or more than one root is
	// foreign. (Task-11 refines this with receipt / legacy-equivalent tree checks;
	// the single-orphan-root test is sufficient for the init-created shape.)
	if sc.metadataTip != "" {
		// The base gatherer proves the metadata branch PRESENT via ls-remote (its OID
		// only); on a clone that never fetched docket the commit object is not yet
		// local, so a root-shape probe would error into RootUnknown. Fetch the branch
		// first so both this probe and gatherFrontmatterFindings read a local object.
		// The fetch updates only a remote-tracking ref (the read-only contract
		// excludes refs/remotes/*); a fetch error leaves the ls-remote tip in place and
		// RootCommits maps its own failure to the safe RootUnknown, never a false shape.
		metaTip := sc.metadataTip
		if rev, ferr := git.FetchBranch(ctx, sc.repo, setupRemote(), metaRef); ferr == nil {
			metaTip = string(rev.Commit)
		}
		roots, err := git.RootCommits(ctx, sc.repo, gitcli.ObjectID(metaTip))
		switch {
		case err != nil:
			f.MetadataRoot = reposetup.RootUnknown
		case len(roots) == 1 && string(roots[0]) == metaTip:
			f.MetadataRoot = reposetup.RootParentless
		default:
			f.MetadataRoot = reposetup.RootForeign
		}
	}

	// Local metadata branch and its synchronization with the remote docket tip.
	localTip, lerr := git.ResolveRef(ctx, sc.repo, metaRef)
	if lerr == nil {
		f.LocalMetadata.Presence = reposetup.PresencePresent
		f.LocalMetadata.Tip = string(localTip)
	} else if fail, ok := gitcli.AsFailure(lerr); ok && fail.Kind == gitcli.KindRefUnavailable {
		f.LocalMetadata.Presence = reposetup.PresenceAbsent
	} // any other error leaves LocalMetadata Unknown

	// The .docket metadata worktree: clean, synchronized with the remote tip, and
	// hooks disabled. Probed only when the worktree is actually present.
	if f.DocketWorktree.Presence == reposetup.PresencePresent {
		worktreeDir := filepath.Join(sc.repo.PrimaryWorktree, docketWorktreeName)
		f.DocketWorktree.Clean = worktreeCleanPresence(ctx, git, worktreeDir)
		f.DocketWorktree.Synchronized = synchronizedPresence(f.LocalMetadata, f.RemoteMetadata)
		f.DocketWorktree.HooksOff = hooksOffPresence(ctx, git, worktreeDir)
	}

	// A repository whose remote is fully migrated — a parentless docket seed with
	// the integration surface already pruned — but whose local .docket attachment
	// did not finish is a resumable PARTIAL, not a terminal conflict. Keyed on the
	// same authoritative postconditions a migration resumes from, so a concurrent
	// check that observes this window classifies as partial (the idempotent-resume
	// remedy) rather than falling through to postconditions-unmet. It never fires
	// for a healthy repository (its local branch is present and its worktree is
	// registered) nor for the seed-published-live-surface window (live surface
	// present), which the classifier already routes to partial on its own.
	if f.MetadataRoot == reposetup.RootParentless &&
		f.LiveSurface == reposetup.PresenceAbsent &&
		(f.LocalMetadata.Presence != reposetup.PresencePresent ||
			f.DocketWorktree.Registered != reposetup.PresencePresent) {
		f.PartialPhase = reposetup.PartialIntegrationPruned
	}

	// Committed guarantees and surface facts proven from the integration COMMIT
	// tree, never the working tree.
	if sc.sourceRevision != "" {
		f.CommittedIgnoreBlock = committedIgnorePresence(ctx, git, sc.repo, sc.sourceRevision)
		f.LegacyConfigKey = committedLegacyKeyPresence(ctx, git, sc.repo, sc.sourceRevision)
	}

	// Primary worktree on the integration branch, and the init-planned managed
	// paths not yet reviewed and committed. The pending set is derived from the
	// committed-ignore fact (a .gitignore whose managed block is in the working
	// tree but not yet in the integration commit is exactly one pending review),
	// so a probe that mis-reads the guarantee from the working tree would drop the
	// pending path too — the committed-ignore guard and the needs-review signal
	// share one authority.
	f.PrimaryOnIntegration = primaryOnIntegrationPresence(ctx, git, sc.repo, sc.integrationBranch)
	f.PendingReviewPaths = pendingReviewPaths(ctx, git, sc.repo, f.CommittedIgnoreBlock)

	// Authorized parent-facing surfaces: absent a drift probe (Task 11 owns the
	// surfaces-drift determination), an authorized declaration is reported as
	// agreeing rather than spuriously conflicting.
	if f.SurfacesAuthorized {
		f.SurfacesAgree = reposetup.PresencePresent
	}
}

// worktreeCleanPresence reports whether the .docket worktree has no
// uncommitted/untracked change. A status read error is the safe Unknown.
func worktreeCleanPresence(ctx context.Context, git *gitcli.Client, worktreeDir string) reposetup.Presence {
	changes, err := git.ChangedPaths(ctx, worktreeDir)
	if err != nil {
		return reposetup.PresenceUnknown
	}
	if len(changes) == 0 {
		return reposetup.PresencePresent
	}
	return reposetup.PresenceAbsent
}

// synchronizedPresence reports whether the local metadata tip equals the remote
// docket tip. Either tip unknown leaves the fact Unknown; equal is Present,
// unequal is Absent.
func synchronizedPresence(local, remote reposetup.BranchFact) reposetup.Presence {
	if local.Presence != reposetup.PresencePresent || remote.Presence != reposetup.PresencePresent {
		return reposetup.PresenceUnknown
	}
	if local.Tip == "" || remote.Tip == "" {
		return reposetup.PresenceUnknown
	}
	if local.Tip == remote.Tip {
		return reposetup.PresencePresent
	}
	return reposetup.PresenceAbsent
}

// hooksOffPresence reports whether the .docket worktree's hooks are disabled the
// way init leaves them (a per-worktree core.hooksPath pointing at an existing
// dir). A probe error is the safe Unknown.
func hooksOffPresence(ctx context.Context, git *gitcli.Client, worktreeDir string) reposetup.Presence {
	off, err := git.WorktreeHooksDisabled(ctx, worktreeDir)
	if err != nil {
		return reposetup.PresenceUnknown
	}
	if off {
		return reposetup.PresencePresent
	}
	return reposetup.PresenceAbsent
}

// committedIgnorePresence proves the managed .gitignore block from the
// integration COMMIT tree — never the working tree (learning
// gitignore-guarantee-must-be-committed). A read error is the safe Unknown.
func committedIgnorePresence(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, rev string) reposetup.Presence {
	blob, found, err := readCommitBlob(ctx, git, repo, rev, gitignoreRel)
	if err != nil {
		return reposetup.PresenceUnknown
	}
	if found && reposetup.ValidGitignoreBlock(blob) {
		return reposetup.PresencePresent
	}
	return reposetup.PresenceAbsent
}

// committedLegacyKeyPresence reports whether the integration COMMIT tree's
// .docket.yml still carries the top-level metadata_branch key. A read or edit
// error is the safe Unknown.
func committedLegacyKeyPresence(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, rev string) reposetup.Presence {
	blob, found, err := readCommitBlob(ctx, git, repo, rev, ".docket.yml")
	if err != nil || !found {
		if err != nil {
			return reposetup.PresenceUnknown
		}
		return reposetup.PresenceAbsent
	}
	_, removed, eerr := reposetup.RemoveMetadataBranchKey(blob)
	if eerr != nil {
		return reposetup.PresenceUnknown
	}
	if removed {
		return reposetup.PresencePresent
	}
	return reposetup.PresenceAbsent
}

// readCommitBlob reads one repo-relative path from a pinned commit tree,
// returning its bytes and whether it was found. It is the single boundary the
// committed-guarantee probes read through.
func readCommitBlob(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, rev, rel string) ([]byte, bool, error) {
	src, err := git.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: gitcli.ObjectID(rev)})
	if err != nil {
		return nil, false, err
	}
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(rel)})
	if err != nil {
		return nil, false, err
	}
	if len(results) != 1 || !results[0].Found {
		return nil, false, nil
	}
	return results[0].Blob.Bytes, true, nil
}

// primaryOnIntegrationPresence reports whether the primary worktree is on the
// integration branch, read from the authoritative worktree registration. A list
// error is the safe Unknown.
func primaryOnIntegrationPresence(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, integrationBranch string) reposetup.Presence {
	wts, err := git.ListWorktrees(ctx, repo)
	if err != nil {
		return reposetup.PresenceUnknown
	}
	want := gitcli.RefName(branchRefPrefix + integrationBranch)
	for _, wt := range wts {
		if filepath.Clean(wt.Path) != filepath.Clean(repo.PrimaryWorktree) {
			continue
		}
		if wt.Branch == want {
			return reposetup.PresencePresent
		}
		return reposetup.PresenceAbsent
	}
	return reposetup.PresenceUnknown
}

// pendingReviewPaths lists the init-planned managed surfaces not yet committed to
// the integration tree — exactly the paths a needs-review classification names.
//
// The managed .gitignore is pending precisely when its block is in the working
// tree but not yet in the integration commit, so its pending status is read from
// the committed-ignore fact (committedIgnore != Present) rather than re-derived:
// the committed-ignore guarantee and the .gitignore needs-review signal share one
// authority, so a probe that mis-read the guarantee from the working tree drops
// the pending path too. The parent-facing dispatch surfaces are pending when they
// differ from HEAD in the working tree. A status error yields no surface pending
// paths (the safe empty), leaving the split to the other postconditions.
func pendingReviewPaths(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, committedIgnore reposetup.Presence) []string {
	seen := map[string]bool{}
	var pending []string

	// The .gitignore edit: pending iff its managed block is present in the working
	// tree but the integration commit does not yet carry it.
	if committedIgnore != reposetup.PresencePresent {
		wt, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, gitignoreRel))
		if err == nil && reposetup.ValidGitignoreBlock(wt) {
			seen[gitignoreRel] = true
			pending = append(pending, gitignoreRel)
		}
	}

	// The parent-facing dispatch surfaces: pending when changed in the working
	// tree relative to HEAD.
	changes, err := git.ChangedPaths(ctx, repo.PrimaryWorktree)
	if err == nil {
		for _, ch := range changes {
			rel := string(ch.Path)
			if rel == gitignoreRel {
				continue // the .gitignore is decided by the committed-ignore fact above
			}
			if docketManagedWorktreePaths[rel] && !seen[rel] {
				seen[rel] = true
				pending = append(pending, rel)
			}
		}
	}

	sort.Strings(pending)
	return pending
}

// gatherFrontmatterFindings reads the metadata corpus from the remote docket tip
// and returns report-only frontmatter findings: the closed mechanical-repair
// roster from PlanRepairs per change record, plus any error-severity cross-record
// corpus finding BuildSnapshot names. It never writes and never repairs. A read
// error yields no findings — a check must not fabricate a repair.
func gatherFrontmatterFindings(ctx context.Context, git *gitcli.Client, sc setupContext) []reposetup.RepairFinding {
	src, err := git.OpenObjectSource(ctx, sc.repo, gitcli.Revision{Commit: gitcli.ObjectID(sc.metadataTip)})
	if err != nil {
		return nil
	}
	entries, err := src.ListTree(ctx, corpusPrefixes(sc.cfg))
	if err != nil {
		return nil
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
		return nil
	}
	blobs, err := src.ReadBlobs(ctx, paths)
	if err != nil {
		return nil
	}
	recs := make([]corpusRecord, 0, len(blobs))
	for i, br := range blobs {
		if !br.Found {
			continue
		}
		recs = append(recs, corpusRecord{
			path:     string(br.Path),
			bytes:    br.Blob.Bytes,
			kind:     metas[i].kind,
			location: metas[i].location,
		})
	}
	return corpusFindings(sc.cfg, recs)
}

// corpusRecord is one metadata record read for report-only validation.
type corpusRecord struct {
	path     string
	bytes    []byte
	kind     repository.RecordKind
	location repository.RecordLocation
}

// corpusFindings is the pure report-only validation over already-read records:
// PlanRepairs per change record for the closed repair roster, then a whole-corpus
// BuildSnapshot whose error-severity findings surface as non-repairable
// frontmatter findings. It is pure so it is unit-testable without Git.
func corpusFindings(cfg config.Effective, recs []corpusRecord) []reposetup.RepairFinding {
	var out []reposetup.RepairFinding
	covered := map[string]bool{}
	for _, r := range recs {
		if r.kind != repository.KindChange {
			continue
		}
		fnds, err := reposetup.PlanRepairs(r.path, r.bytes, r.location == repository.LocationArchive)
		if err != nil {
			continue
		}
		for _, f := range fnds {
			covered[f.Path] = true
			out = append(out, f)
		}
	}

	inputs := make([]repository.InputDocument, 0, len(recs))
	for _, r := range recs {
		doc, err := document.Parse(r.bytes)
		if err != nil {
			// An undecodable record is already named by PlanRepairs above (for a
			// change) or is not a change; do not feed a partial document to the
			// snapshot builder.
			continue
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: r.kind, Location: r.location, Path: r.path, Document: doc,
		})
	}
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: cfg, Documents: inputs})
	if err != nil {
		return out
	}
	for _, f := range build.Report.Findings() {
		if f.Severity != domain.SeverityError {
			continue
		}
		if covered[f.Entity.Path] {
			continue
		}
		out = append(out, reposetup.RepairFinding{
			Path:       f.Entity.Path,
			Field:      f.Field,
			Repairable: false,
			Message:    "metadata corpus finding: " + f.Code,
		})
	}
	return out
}

// checkGatherFailure maps a fact-gathering error to a check result whose exit is
// 2 (undeterminable authority). It never writes and never repairs; the state is
// unknown because the authoritative reads could not complete.
func checkGatherFailure(err error) RepositoryCheckResult {
	var rre *RepoResolutionError
	result := ResultExternalFailed
	switch {
	case errors.As(err, &rre):
		result = ResultUnsupportedConfig
	case errors.Is(err, ErrStatusInvalidInput):
		result = ResultInvalidInput
	}
	out := RepositoryCheckResult{
		Envelope:        NewEnvelope(OperationRepositoryCheck, result),
		RepositoryState: string(reposetup.StateUnknown),
	}
	out.human = fmt.Sprintf("repository check: %s: %s", result, err.Error())
	return out
}
