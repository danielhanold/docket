package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
)

// repository_migrate_repair.go — migrate's second job: authorized mechanical
// repair of deterministic derived-view drift on an ALREADY-HEALTHY repository.
//
// `repository check` finds the drift (repository_check.go's derivedViewFindings);
// this path recomputes exactly the canonical derived bytes — the inline board,
// the ADR index, and each change record's managed artifact-links block — and
// republishes the docket metadata branch as a single descendant of its pinned
// tip under an exact owned lease (learning cas-re-read-fresh-origin /
// decide-and-act-on-the-same-copy). It reuses the base migration's
// preview/authorization model: an unauthorized run returns the exact pinned
// revision and file set for confirmation; an authorized run re-proves the pinned
// revision (a moved tip is contention, never an overwrite) before writing.
//
// It NEVER rewrites authored content: only fully-generated whole files (board,
// ADR index) and the interior of a well-formed managed block are touched. A
// malformed/unbalanced managed marker is NON-repairable — the record fails to
// parse, so it is never rewritten (AGENTS.md marker order/balance rule) — and is
// reported as a manual-review diagnostic in the preview.

// migrateRepairSubject is the commit subject the derived-view repair descendant
// carries on the docket metadata branch.
const migrateRepairSubject = "docket: repair derived views"

// migrateHealthyRepair repairs deterministic derived-view drift on a healthy
// repository. With nothing repairable it is the idempotent no-op the caller
// previously returned directly.
func migrateHealthyRepair(ctx context.Context, d SetupDeps, o MigrateOptions, sc setupContext) RepositoryMigrateResult {
	metadataTip := sc.metadataTip
	if metadataTip == "" {
		return migrateExternalFailure(reposetup.StateHealthy, "pinning the metadata revision",
			errors.New("no authoritative metadata tip is available"))
	}

	corpus, err := readCheckCorpus(ctx, d.Git, sc)
	if err != nil {
		return migrateExternalFailure(reposetup.StateHealthy, "reading the metadata corpus", err)
	}
	snap, ok := buildCorpusSnapshot(sc.cfg, corpus.records)
	if !ok {
		return migrateInternalFailure(reposetup.StateHealthy, "building the metadata snapshot",
			errors.New("the metadata corpus could not be built into a snapshot"))
	}

	repairable, diagnostics := splitDerivedFindings(derivedViewFindings(sc.cfg, corpus))

	// Derived-view repair is opt-in through the same authorization flag the
	// frontmatter repairs use (--repair-frontmatter). Without the opt-in a healthy
	// repository stays the idempotent no-op it has always been — a plain
	// `migrate`/`migrate --yes` never silently rewrites the metadata branch. The
	// drift is still surfaced by `docket repository check`; migrate repairs it only
	// when explicitly asked. Nothing repairable is likewise a no-op (any malformed
	// diagnostic is left for the human, and check reports it).
	if len(repairable) == 0 || !o.RepairAuthorized {
		return migrateNoOp(metadataTip)
	}

	files := derivedRepairFiles(repairable)
	preview := derivedRepairPreviewText(sc, metadataTip, repairable, diagnostics)

	// Two-pass authorization: the repair opt-in without --yes returns the preview
	// (exact pinned revision + file set) for confirmation.
	if !o.Authorized {
		return derivedRepairConfirmationRequired(metadataTip, files, preview)
	}
	// Decide-and-act on the same copy: an authorized re-invocation must act on the
	// exact metadata revision its preview showed. A moved tip is contention.
	if migrateSourceMoved(o.ExpectedSource, metadataTip) {
		return migrateContended(metadataTip, o.ExpectedSource)
	}

	return executeDerivedRepair(ctx, d.Git, sc, metadataTip, snap, corpus, repairable, files)
}

// splitDerivedFindings partitions derived-view findings into the mechanically
// repairable set and the manual-review diagnostics, preserving order.
func splitDerivedFindings(findings []reposetup.DerivedFinding) (repairable, diagnostics []reposetup.DerivedFinding) {
	for _, f := range findings {
		if f.Repairable {
			repairable = append(repairable, f)
		} else {
			diagnostics = append(diagnostics, f)
		}
	}
	return repairable, diagnostics
}

// derivedRepairFiles is the sorted, de-duplicated repo-relative file set the
// repairable findings touch.
func derivedRepairFiles(repairable []reposetup.DerivedFinding) []string {
	seen := map[string]bool{}
	var files []string
	for _, f := range repairable {
		if seen[f.Path] {
			continue
		}
		seen[f.Path] = true
		files = append(files, f.Path)
	}
	sort.Strings(files)
	return files
}

// executeDerivedRepair composes the corrected derived bytes, publishes them as a
// single descendant of the pinned metadata tip under an exact owned lease, and
// re-reads the postcondition. It writes ONLY the repairable files; authored
// content and every other blob are carried forward from the pinned tree.
func executeDerivedRepair(ctx context.Context, git *gitcli.Client, sc setupContext, metadataTip string, snap domain.Snapshot, corpus checkCorpus, repairable []reposetup.DerivedFinding, files []string) RepositoryMigrateResult {
	tipOID := gitcli.ObjectID(metadataTip)
	docketRef := gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName)

	recByPath := map[string]corpusRecord{}
	for _, r := range corpus.records {
		recByPath[r.path] = r
	}

	var ops []gitcli.TreeOp
	for _, f := range files {
		content, cerr := composeDerivedRepairBytes(sc, snap, corpus, recByPath, f, repairable)
		if cerr != nil {
			return migrateInternalFailure(reposetup.StateHealthy, "composing the repaired "+f, cerr)
		}
		ops = append(ops, gitcli.TreeOp{PutBlob: &gitcli.PutBlobOp{Path: gitcli.RepoPath(f), Content: content, Mode: blobMode}})
	}

	tree, err := git.BuildTree(ctx, sc.repo, tipOID, ops)
	if err != nil {
		return migrateExternalFailure(reposetup.StateHealthy, "composing the repaired metadata tree", err)
	}
	commit, err := git.CommitTree(ctx, sc.repo, tree, []gitcli.ObjectID{tipOID}, migrateRepairSubject, nil)
	if err != nil {
		return migrateExternalFailure(reposetup.StateHealthy, "creating the derived-view repair commit", err)
	}

	// Publish the repair descendant under an exact owned lease keyed on the pinned
	// tip: it applies only if the remote docket tip is still exactly the pinned
	// revision. A moved tip is contention, never an overwrite.
	out, err := git.PushLease(ctx, sc.repo, setupRemote(), docketRef, commit, tipOID)
	if err != nil {
		return migrateExternalFailure(reposetup.StateHealthy, "publishing the derived-view repair", err)
	}
	switch out.Disposition {
	case gitcli.PushApplied:
		// proceed
	case gitcli.PushLeaseLost:
		return migrateContended(string(out.Remote), metadataTip)
	default:
		return migrateExternalFailure(reposetup.StateHealthy, "publishing the derived-view repair",
			errors.New("docket lease push failed"))
	}

	// Re-read the docket postcondition byte-exactly.
	rev, err := git.FetchBranch(ctx, sc.repo, setupRemote(), docketRef)
	if err != nil {
		return migrateExternalFailure(reposetup.StateHealthy, "re-reading the repaired metadata branch", err)
	}
	if rev.Commit != commit {
		return migrateContended(string(rev.Commit), metadataTip)
	}

	return derivedRepairApplied(sc, string(commit), metadataTip, files)
}

// composeDerivedRepairBytes recomputes the canonical bytes for one repaired file.
// The board and ADR index are whole-file renders; an artifact-links file is the
// record with its managed block rewritten (or, when absent, inserted after the
// frontmatter) — never any other authored byte.
func composeDerivedRepairBytes(sc setupContext, snap domain.Snapshot, corpus checkCorpus, recByPath map[string]corpusRecord, file string, repairable []reposetup.DerivedFinding) ([]byte, error) {
	switch file {
	case boardCorpusPath(sc.cfg):
		return renderCanonicalBoard(snap)
	case adrIndexCorpusPath(sc.cfg):
		return renderCanonicalADRIndex(snap)
	}
	// An artifact-links record.
	rec, ok := recByPath[file]
	if !ok {
		return nil, fmt.Errorf("record %s absent from the corpus", file)
	}
	doc, err := document.Parse(rec.bytes)
	if err != nil {
		return nil, err // a malformed record is never repairable and must not reach here
	}
	change, ok := snapshotChangeByPath(snap, file)
	if !ok {
		return nil, fmt.Errorf("record %s absent from the snapshot", file)
	}
	body, err := render.ArtifactBlockContent(change, snap, corpus.link)
	if err != nil {
		return nil, err
	}
	var ps document.PatchSet
	if _, present := doc.Block("artifacts"); present {
		ps.ReplaceBlock("artifacts", body)
	} else {
		// Missing block: insert the managed block at a deterministic location
		// (immediately after the frontmatter). This adds only managed marker lines
		// and the generated body; no authored byte is modified.
		ps.InsertBlock("artifacts", "generated — do not hand-edit", body, document.AfterFrontmatter)
	}
	return doc.Apply(ps)
}

// changeByPath finds the change whose canonical path is p.
func snapshotChangeByPath(snap domain.Snapshot, p string) (domain.Change, bool) {
	for _, c := range snap.Changes() {
		if c.Path() == p {
			return c, true
		}
	}
	return domain.Change{}, false
}

// derivedRepairPreviewText renders the confirmation preview: the resolved repo,
// remote, exact pinned metadata revision, the repaired file set, and any
// manual-review diagnostics the repair will NOT touch.
func derivedRepairPreviewText(sc setupContext, metadataTip string, repairable, diagnostics []reposetup.DerivedFinding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "docket repository migrate — derived-view repair\n")
	fmt.Fprintf(&b, "  repository:  %s\n", sc.repo.PrimaryWorktree)
	fmt.Fprintf(&b, "  remote:      %s\n", setupRemote())
	fmt.Fprintf(&b, "  metadata:    %s @ %s\n", reposetup.MetadataBranchName, metadataTip)
	fmt.Fprintf(&b, "  repairs:\n")
	for _, f := range repairable {
		fmt.Fprintf(&b, "    [%s] %s\n", f.Code, f.Path)
	}
	if len(diagnostics) > 0 {
		fmt.Fprintf(&b, "  manual review (not repaired):\n")
		for _, f := range diagnostics {
			fmt.Fprintf(&b, "    [%s] %s: %s\n", f.Code, f.Path, f.Message)
		}
	}
	return b.String()
}

// derivedRepairConfirmationRequired is the unauthorized preview: the repaired
// file set, refused as invalid-state with reason confirmation-required, naming
// --yes. SourceRevision is the pinned metadata tip so the CLI's confirm flow
// re-proves the exact copy the human saw.
func derivedRepairConfirmationRequired(metadataTip string, files []string, preview string) RepositoryMigrateResult {
	out := newMigrateResult(ResultInvalidState, RepositoryMigrateResult{
		RepositoryState: "confirmation-required",
		SourceRevision:  metadataTip,
		CopyPrefixes:    []string{},
		RemovedPaths:    []string{},
		RepairedViews:   files,
	})
	out.human = preview + "\nconfirmation required: re-run with --yes to authorize these derived-view repairs"
	return out
}

// derivedRepairApplied is the success document naming the repaired metadata
// revision, the exact repaired file set, and the local synchronization remedy for
// the .docket worktree (the remote docket branch advanced; the local worktree
// fast-forwards on the next `docket repository prepare`).
func derivedRepairApplied(sc setupContext, newTip, priorTip string, files []string) RepositoryMigrateResult {
	pending := []string{
		"fast-forward your local .docket metadata worktree: re-run `docket repository prepare` to sync it to the repaired metadata revision",
	}
	out := newMigrateResult(ResultApplied, RepositoryMigrateResult{
		RepositoryState: string(reposetup.StateNeedsReview),
		SourceRevision:  priorTip,
		MetadataTip:     newTip,
		CopyPrefixes:    []string{},
		RemovedPaths:    []string{},
		RepairedViews:   files,
		PendingLocal:    pending,
	})
	out.human = fmt.Sprintf("derived views repaired: metadata %s (%d file(s))\npending local sync: %s",
		newTip, len(files), strings.Join(pending, "; "))
	return out
}
