package app

import (
	"bytes"
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
	"github.com/danielhanold/docket/internal/render"
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
	var corpusExtra []reposetup.Finding
	var testConfig *reposetup.Finding
	if facts.RemoteMetadata.Presence == reposetup.PresencePresent {
		augmentCheckFacts(ctx, d.Git, &facts, sc)
		corpus, rerr := readCheckCorpus(ctx, d.Git, sc)
		fm, corpusExtra = checkCorpusOutcome(sc.cfg, corpus, rerr)
		// A local build/finalize gate with no configured command (or a still-declared
		// legacy `auto`) is a setup gap, not a topology fault: it surfaces here with
		// the `docket repository configure-tests` remedy. Read the committed
		// repository-layer `.docket.yml` bytes so the legacy-`auto` signal keys on
		// exactly what the resolver read.
		testConfig = reposetup.TestConfigFinding(sc.cfg, readCommittedDocketYML(sc.repo.PrimaryWorktree))
	}

	cls := reposetup.Classify(facts)
	findings := reposetup.EvaluateHealth(cls, facts, fm)
	findings = append(findings, corpusExtra...)
	if testConfig != nil {
		findings = append(findings, *testConfig)
	}
	return newCheckResult(cls, facts, findings)
}

// readCommittedDocketYML returns the primary worktree's `.docket.yml` bytes —
// the repository-layer config the resolver reads — or nil when it is absent or
// unreadable. The health finding tolerates nil (it then rests on the resolved
// config alone), so an absent or transiently unreadable file is never an error
// here.
func readCommittedDocketYML(primaryWorktree string) []byte {
	b, err := os.ReadFile(filepath.Join(primaryWorktree, docketYMLRel))
	if err != nil {
		return nil
	}
	return b
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
	metaRef := gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName)

	// Metadata root shape: the shared ownership verifier decides, at the FETCHED
	// remote docket tip, whether the tip's sole parentless-root lineage is a
	// verified docket seed root (RootParentless — a native init/migrate receipt or
	// a receiptless legacy-equivalent tree, with any number of permitted
	// descendants and merges; the root need not equal the tip), readable evidence
	// with no ownership proof (RootForeign), or unreadable evidence (RootUnknown).
	// The fetched tip is the single authority (learning
	// decide-and-act-on-the-same-copy): it becomes RemoteMetadata's reported
	// revision and the synchronizedPresence comparison's remote side, so the
	// reported tip is exactly the tip the ownership proof was computed at.
	if sc.metadataTip != "" {
		// The base gatherer proves the metadata branch PRESENT via ls-remote (its OID
		// only); on a clone that never fetched docket the commit object is not yet
		// local. Fetch the branch first so both the ownership probe and
		// gatherFrontmatterFindings read a local object. The fetch updates only a
		// remote-tracking ref (the read-only contract excludes refs/remotes/*).
		rev, ferr := git.FetchBranch(ctx, sc.repo, setupRemote(), metaRef)
		if ferr != nil {
			// A fetch error is unknown even if an older object happens to be available
			// locally: never fall back to the ls-remote tip and never prove ownership
			// from a stale object. Unknown, never a false shape.
			f.MetadataRoot = reposetup.RootUnknown
			sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "metadata-fetch", Err: ferr})
		} else {
			f.RemoteMetadata.Tip = string(rev.Commit)
			own := verifyMetadataOwnership(ctx, git, sc.repo, rev.Commit, gitcli.ObjectID(sc.sourceRevision), sc.defaultBranch)
			f.MetadataRoot = own.Shape
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

// checkCorpus is everything the report-only check reads from the pinned metadata
// corpus: the classified records, the two whole-file derived views (board and
// ADR index), and the link context the artifact-links renderer needs. A view's
// present flag distinguishes an absent file (no drift possible) from an empty
// one.
type checkCorpus struct {
	records  []corpusRecord
	link     render.LinkContext
	board    corpusFile
	adrIndex corpusFile
}

// corpusFile is one whole-file derived view read from the corpus: its bytes and
// whether it exists at all.
type corpusFile struct {
	present bool
	bytes   []byte
}

// readCheckCorpus reads the metadata corpus from the remote docket tip: the
// classified change/ADR/spec records, the inline board, the ADR index, and the
// origin-derived link context. It never writes. A read error is RETURNED, never
// swallowed into a clean absence — the caller surfaces it as an error finding so
// the check never fabricates absence (learning probe-error-is-not-clean-absence).
func readCheckCorpus(ctx context.Context, git *gitcli.Client, sc setupContext) (checkCorpus, error) {
	var corpus checkCorpus
	src, err := git.OpenObjectSource(ctx, sc.repo, gitcli.Revision{Commit: gitcli.ObjectID(sc.metadataTip)})
	if err != nil {
		return corpus, err
	}
	entries, err := src.ListTree(ctx, corpusPrefixes(sc.cfg))
	if err != nil {
		return corpus, err
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
	if len(paths) > 0 {
		blobs, err := src.ReadBlobs(ctx, paths)
		if err != nil {
			return corpus, err
		}
		for i, br := range blobs {
			if !br.Found {
				continue
			}
			corpus.records = append(corpus.records, corpusRecord{
				path:     string(br.Path),
				bytes:    br.Blob.Bytes,
				kind:     metas[i].kind,
				location: metas[i].location,
			})
		}
	}

	// The two whole-file derived views. A ReadBlobs error is a corpus read
	// failure; an absent file is a Found=false result, not an error.
	viewBlobs, err := src.ReadBlobs(ctx, []gitcli.RepoPath{
		gitcli.RepoPath(boardCorpusPath(sc.cfg)),
		gitcli.RepoPath(adrIndexCorpusPath(sc.cfg)),
	})
	if err != nil {
		return corpus, err
	}
	if len(viewBlobs) == 2 {
		corpus.board = corpusFile{present: viewBlobs[0].Found, bytes: viewBlobs[0].Blob.Bytes}
		corpus.adrIndex = corpusFile{present: viewBlobs[1].Found, bytes: viewBlobs[1].Blob.Bytes}
	}

	// The link context, derived from origin exactly as the authoritative writers
	// derive it (link_context.go), so the artifact-links comparison renders the
	// same bytes a mutation would have written. A RemoteURL failure is a read
	// error, fail-closed.
	remoteURL, uerr := git.RemoteURL(ctx, sc.repo, setupRemote())
	if uerr != nil {
		return corpus, uerr
	}
	// linkContextOf is the sole LinkContext constructor (link_context.go / the
	// 0341 shape guard): route through it so RepoWebURL can never be silently
	// dropped, exactly as the authoritative writers do.
	corpus.link = linkContextOf(StatusPin{RepoWebURL: githubWebURL(remoteURL)})
	return corpus, nil
}

// boardCorpusPath and adrIndexCorpusPath name the two whole-file derived views
// in the corpus: the inline board under the changes directory and the ADR index
// README under the ADR directory.
func boardCorpusPath(cfg config.Effective) string {
	return filepath.ToSlash(filepath.Join(cfg.ChangesDir.Value, "BOARD.md"))
}

func adrIndexCorpusPath(cfg config.Effective) string {
	return filepath.ToSlash(filepath.Join(cfg.ADRsDir.Value, "README.md"))
}

// checkCorpusOutcome assembles the corpus-derived findings for the check. A
// non-nil readErr surfaces as an error-severity corpus-unreadable finding — a
// read failure is never fabricated into a clean absence — and no corpus records
// are inspected. Otherwise it returns the frontmatter repair findings (fed
// through EvaluateHealth) and the derived-view drift findings lifted to health
// findings. It is pure so both branches are unit-testable without Git.
func checkCorpusOutcome(cfg config.Effective, corpus checkCorpus, readErr error) ([]reposetup.RepairFinding, []reposetup.Finding) {
	if readErr != nil {
		return nil, []reposetup.Finding{corpusUnreadableFinding(readErr)}
	}
	fm := corpusFindings(cfg, corpus.records)
	var extra []reposetup.Finding
	for _, df := range derivedViewFindings(cfg, corpus) {
		extra = append(extra, df.Finding())
	}
	return fm, extra
}

// corpusUnreadableFinding is the error-severity finding a corpus read failure
// yields. The check reports the failure rather than a clean absence.
func corpusUnreadableFinding(err error) reposetup.Finding {
	return reposetup.Finding{
		Code:     reposetup.CodeCorpusUnreadable,
		Severity: reposetup.SeverityError,
		Message:  "the metadata corpus could not be read: " + err.Error(),
		Remedy:   "Ensure the remote docket branch is reachable and its objects are fetched, then re-run `docket repository check`.",
	}
}

// derivedBytesDiffer is the ONE byte comparison behind all three derived views:
// board, ADR index, and each record's artifact-links block are each rendered to
// their canonical bytes and compared to the stored bytes through this helper.
func derivedBytesDiffer(canonical, stored []byte) bool {
	return !bytes.Equal(canonical, stored)
}

// derivedViewFindings compares each canonical derived view against its stored
// bytes and returns the ordered drift findings: board first, then ADR index,
// then each change record's artifact-links block. It renders every view from the
// SAME candidate snapshot the corpus builds, through the canonical renderers, so
// a difference is exactly the drift a mutation would have avoided. A snapshot
// that cannot be built yields no derived findings (the frontmatter path already
// surfaces the blocking errors).
func derivedViewFindings(cfg config.Effective, corpus checkCorpus) []reposetup.DerivedFinding {
	snap, ok := buildCorpusSnapshot(cfg, corpus.records)
	if !ok {
		return nil
	}
	var out []reposetup.DerivedFinding

	if corpus.board.present {
		if canonical, err := renderCanonicalBoard(snap, boardPresentation(cfg)); err == nil {
			if derivedBytesDiffer(canonical, corpus.board.bytes) {
				out = append(out, reposetup.DerivedFinding{
					View:       reposetup.DerivedViewBoard,
					Code:       reposetup.CodeBoardStale,
					Path:       boardCorpusPath(cfg),
					Repairable: true,
					Message:    "the inline board differs from the canonical render of the current metadata.",
				})
			}
		}
	}

	if corpus.adrIndex.present {
		if canonical, err := renderCanonicalADRIndex(snap); err == nil {
			if derivedBytesDiffer(canonical, corpus.adrIndex.bytes) {
				out = append(out, reposetup.DerivedFinding{
					View:       reposetup.DerivedViewADRIndex,
					Code:       reposetup.CodeADRIndexStale,
					Path:       adrIndexCorpusPath(cfg),
					Repairable: true,
					Message:    "the ADR index differs from the canonical render of the current metadata.",
				})
			}
		}
	}

	out = append(out, artifactLinkFindings(snap, corpus)...)
	return out
}

// artifactLinkFindings compares each change record's managed artifact-links block
// against the canonical render from snap. Records are visited in path order. A
// record whose managed markers are unbalanced is a NON-repairable malformed
// finding (the automatic repair must never touch an unbalanced block, per
// AGENTS.md); a record whose block is stale or absent-but-required is a
// repairable finding. A record that fails to parse for a non-marker reason is
// left to the frontmatter path.
func artifactLinkFindings(snap domain.Snapshot, corpus checkCorpus) []reposetup.DerivedFinding {
	byPath := map[string]domain.Change{}
	for _, c := range snap.Changes() {
		byPath[c.Path()] = c
	}
	recs := append([]corpusRecord(nil), corpus.records...)
	sort.Slice(recs, func(i, j int) bool { return recs[i].path < recs[j].path })

	var out []reposetup.DerivedFinding
	for _, rec := range recs {
		if rec.kind != repository.KindChange {
			continue
		}
		doc, err := document.Parse(rec.bytes)
		if err != nil {
			if markerMalformed(err) {
				out = append(out, reposetup.DerivedFinding{
					View:       reposetup.DerivedViewArtifactLinks,
					Code:       reposetup.CodeArtifactLinksMalformed,
					Path:       rec.path,
					Repairable: false,
					Message:    "the managed artifact-links markers are unbalanced or malformed: " + err.Error(),
				})
			}
			continue
		}
		change, ok := byPath[rec.path]
		if !ok {
			continue // not in the snapshot (e.g. dropped by validation); frontmatter path owns it
		}
		body, berr := render.ArtifactBlockContent(change, snap, corpus.link)
		if berr != nil {
			continue // an unresolvable ADR reference is a validation finding, not drift
		}
		if _, present := doc.Block("artifacts"); !present {
			if body != "" {
				out = append(out, reposetup.DerivedFinding{
					View:       reposetup.DerivedViewArtifactLinks,
					Code:       reposetup.CodeArtifactLinksMissing,
					Path:       rec.path,
					Repairable: true,
					Message:    "the record carries artifacts but has no managed artifact-links block.",
				})
			}
			continue
		}
		candidate, aerr := applyArtifactBlock(doc, body)
		if aerr != nil {
			continue
		}
		if derivedBytesDiffer(candidate, rec.bytes) {
			out = append(out, reposetup.DerivedFinding{
				View:       reposetup.DerivedViewArtifactLinks,
				Code:       reposetup.CodeArtifactLinksStale,
				Path:       rec.path,
				Repairable: true,
				Message:    "the managed artifact-links block differs from the canonical render.",
			})
		}
	}
	return out
}

// applyArtifactBlock rewrites the record's managed artifacts block to body
// through document's own ReplaceBlock, returning the candidate record bytes — the
// exact writer path the authoritative mutations use, so a byte comparison against
// the stored record measures real drift.
func applyArtifactBlock(doc document.Document, body string) ([]byte, error) {
	var ps document.PatchSet
	ps.ReplaceBlock("artifacts", body)
	return doc.Apply(ps)
}

// markerMalformed reports whether a document parse error is a malformed/imbalanced
// managed-marker error (as opposed to an undecodable frontmatter error).
func markerMalformed(err error) bool {
	var de *document.Error
	if !errors.As(err, &de) {
		return false
	}
	return de.Kind == document.KindMalformedMarker || de.Kind == document.KindMarkerImbalance
}

// buildCorpusSnapshot builds the domain snapshot from the decodable corpus
// records, skipping any record whose frontmatter cannot be parsed (those are
// named by the frontmatter path). ok is false when the whole-corpus build fails.
func buildCorpusSnapshot(cfg config.Effective, recs []corpusRecord) (domain.Snapshot, bool) {
	inputs := make([]repository.InputDocument, 0, len(recs))
	for _, r := range recs {
		doc, err := document.Parse(r.bytes)
		if err != nil {
			continue
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: r.kind, Location: r.location, Path: r.path, Document: doc,
		})
	}
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: cfg, Documents: inputs})
	if err != nil {
		return domain.Snapshot{}, false
	}
	return build.Snapshot, true
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
