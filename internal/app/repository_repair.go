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

// repository_repair.go — `docket repository repair`: authorized mechanical repair
// of every repairable finding `docket repository check` reports on an
// ALREADY-MIGRATED repository — the frontmatter repair roster and the derived
// views (inline board, artifact-links blocks, ADR index). Migrate only migrates
// (change 0496); this is the one repair path on a migrated repository.
//
// It reads the SAME corpus check reads (readCheckCorpus at the pinned remote
// docket tip), applies the frontmatter repairs in memory first, recomputes the
// derived-view findings over the REPAIRED corpus, and publishes the composed
// bytes as ONE descendant of the pinned tip under an exact owned lease (learning
// cas-re-read-fresh-origin / decide-and-act-on-the-same-copy). An unauthorized run
// returns the preview; an authorized run re-proves the pinned revision — a moved
// tip is contention, never an overwrite. Non-repairable findings are listed as
// manual review and never touched; a malformed managed marker keeps its
// manual-review posture (AGENTS.md marker order/balance rule).

// OperationRepositoryRepair is the operation key `repository repair` records.
const OperationRepositoryRepair = "repository.repair"

// repositoryRepairSubject is the commit subject of the repair descendant on the
// docket metadata branch.
const repositoryRepairSubject = "docket: repository repair"

// repairConfirmationRequiredState is the repository_state an unauthorized
// preview carries.
const repairConfirmationRequiredState = "confirmation-required"

// RepairOptions carries the two-pass authorization the CLI resolves. Authorized
// is true only via --yes or an interactive confirmed preview; ExpectedSource is
// the pinned metadata tip the preview showed ("" on the first, preview, pass).
type RepairOptions struct {
	Authorized     bool
	ExpectedSource string
	// PRBacklinks selects the PR-backlink repair instead of the metadata
	// repairs; it requires SetupDeps.GitHub.
	PRBacklinks bool
}

// RepositoryRepairResult is the protocol-v1 document `repository repair`
// returns. SourceRevision is the pinned metadata tip the run read and keyed on;
// MetadataTip is the published repair descendant. Repairs are the frontmatter
// roster repairs, RepairedViews the derived-view files, RepairedFiles the union
// written (or, on a preview, planned). ManualReview lists what the repair will
// not touch.
type RepositoryRepairResult struct {
	Envelope
	RepositoryState string                    `json:"repository_state"`
	SourceRevision  string                    `json:"source_revision"`
	MetadataTip     string                    `json:"metadata_revision,omitempty"`
	Repairs         []reposetup.RepairFinding `json:"repairs,omitempty"`
	RepairedViews   []string                  `json:"repaired_views,omitempty"`
	RepairedFiles   []string                  `json:"repaired_files,omitempty"`
	ManualReview    []string                  `json:"manual_review,omitempty"`
	PendingLocal    []string                  `json:"pending_local,omitempty"`
	// PRBacklinks is the --pr-backlinks preview or report, one row per PR.
	PRBacklinks []PRBacklinkRepair `json:"pr_backlinks,omitempty"`
	// Findings carries a resolver diagnosis lifted from a gather failure.
	Findings []reposetup.Finding `json:"findings,omitempty"`
	human    string
}

// HumanText renders the human summary.
func (r RepositoryRepairResult) HumanText() string {
	if r.human != "" {
		return r.human
	}
	return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.RepositoryState)
}

// SourceRev exposes the pinned metadata tip a preview showed, so the CLI's
// interactive confirm flow re-invokes pinned to exactly that copy.
func (r RepositoryRepairResult) SourceRev() string { return r.SourceRevision }

// ConfirmationRequired reports whether this result is a preview awaiting --yes —
// the only result the CLI's interactive flow prompts on.
func (r RepositoryRepairResult) ConfirmationRequired() bool {
	return r.RepositoryState == repairConfirmationRequiredState
}

// repositoryRepairPlan is the pure repair decision over one corpus read: the
// repairable frontmatter findings, the repairable derived-view findings computed
// over the frontmatter-REPAIRED corpus, the manual-review lines, and the final
// composed bytes per written file.
type repositoryRepairPlan struct {
	frontmatter []reposetup.RepairFinding
	derived     []reposetup.DerivedFinding
	manual      []string
	files       []string          // sorted, de-duplicated union of written paths
	contents    map[string][]byte // final composed bytes per file in files
}

// RunRepositoryRepair repairs every mechanically repairable finding `repository
// check` reports on an already-migrated repository, under the two-pass
// authorization model.
func RunRepositoryRepair(ctx context.Context, d SetupDeps, o RepairOptions) RepositoryRepairResult {
	if o.PRBacklinks {
		return runPRBacklinkRepair(ctx, d, o)
	}
	sc, metadataTip, refusal := repairPreflight(ctx, d)
	if refusal != nil {
		return *refusal
	}
	corpus, err := readCheckCorpus(ctx, d.Git, sc)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "reading the metadata corpus", err)
	}
	plan, err := planRepositoryRepair(sc, corpus)
	if err != nil {
		return repairInternalFailure(reposetup.StateHealthy, "planning the repairs", err)
	}
	if len(plan.files) == 0 {
		return repairNoOp(metadataTip, plan.manual)
	}
	if !o.Authorized {
		return repairConfirmationRequired(sc, metadataTip, plan)
	}
	// Decide-and-act on the same copy: an authorized re-invocation acts only on
	// the exact metadata revision its preview showed.
	if migrateSourceMoved(o.ExpectedSource, metadataTip) {
		return repairContended(metadataTip, o.ExpectedSource)
	}
	return executeRepositoryRepair(ctx, d.Git, sc, metadataTip, plan)
}

// repairPreflight runs the shared gather and topology routing every repair mode
// starts with and returns the setup context and the pinned metadata tip, or the
// refusal to return verbatim.
func repairPreflight(ctx context.Context, d SetupDeps) (setupContext, string, *RepositoryRepairResult) {
	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		r := repairFromMigrateResult(migrateGatherFailure(err))
		return setupContext{}, "", &r
	}
	phase, refusal := migrateRoute(ctx, d.Git, facts, &sc)
	if refusal != nil {
		if refusal.RepositoryState == string(reposetup.StateFresh) {
			r := repairRefusal(reposetup.StateFresh,
				"the repository has no docket metadata branch; run `docket repository init` to create it")
			return setupContext{}, "", &r
		}
		r := repairFromMigrateResult(*refusal)
		return setupContext{}, "", &r
	}
	switch phase {
	case phaseAlreadyMigrated:
		// proceed
	case phaseLegacyFull:
		r := repairRefusal(reposetup.StateLegacy,
			"the repository has a legacy single-branch planning surface; run `docket repository migrate` to convert it, then re-run `docket repository repair`")
		return setupContext{}, "", &r
	case phaseResumePrune:
		r := repairRefusal(reposetup.StatePartial,
			"an interrupted migration still has a live planning surface on the integration branch; run `docket repository migrate` to finish it, then re-run `docket repository repair`")
		return setupContext{}, "", &r
	case phaseResumeLocal:
		r := repairRefusal(reposetup.StateNeedsReview,
			"the local .docket metadata attachment is incomplete; run `docket repository migrate` to finish attaching it, then re-run `docket repository repair`")
		return setupContext{}, "", &r
	default:
		r := repairInternalFailure(reposetup.StateUnknown, "routing the repository topology",
			fmt.Errorf("unexpected topology phase %d", phase))
		return setupContext{}, "", &r
	}

	metadataTip := sc.metadataTip
	if metadataTip == "" {
		r := repairExternalFailure(reposetup.StateHealthy, "pinning the metadata revision",
			errors.New("no authoritative metadata tip is available"))
		return setupContext{}, "", &r
	}
	return sc, metadataTip, nil
}

// planRepositoryRepair is the pure repair decision. Frontmatter repairs are
// planned exactly as check reports them (corpusFindings) and applied in memory
// first (ApplyRepairs re-derives and re-proves each patch); the derived-view
// findings are then computed over the REPAIRED corpus, so a record needing both a
// frontmatter fix and an artifact-links re-render — or a frontmatter fix that
// changes what the board renders — comes out canonical in one pass. It never
// writes.
func planRepositoryRepair(sc setupContext, corpus checkCorpus) (repositoryRepairPlan, error) {
	cfg := sc.cfg
	p := repositoryRepairPlan{contents: map[string][]byte{}}

	byPath := map[string][]reposetup.RepairFinding{}
	for _, f := range corpusFindings(cfg, corpus.records) {
		if !f.Repairable {
			p.manual = append(p.manual, manualReviewLine("frontmatter-manual-review", f.Path, f.Message))
			continue
		}
		p.frontmatter = append(p.frontmatter, f)
		byPath[f.Path] = append(byPath[f.Path], f)
	}

	repaired := corpus
	repaired.records = make([]corpusRecord, len(corpus.records))
	for i, r := range corpus.records {
		if fs, ok := byPath[r.path]; ok {
			b, err := reposetup.ApplyRepairs(r.bytes, fs)
			if err != nil {
				return repositoryRepairPlan{}, fmt.Errorf("applying the frontmatter repairs to %s: %w", r.path, err)
			}
			r.bytes = b
			p.contents[r.path] = b
		}
		repaired.records[i] = r
	}

	repairable, diagnostics := splitDerivedFindings(derivedViewFindings(cfg, repaired))
	for _, d := range diagnostics {
		p.manual = append(p.manual, manualReviewLine(d.Code, d.Path, d.Message))
	}
	p.derived = repairable
	if len(repairable) > 0 {
		snap, ok := buildCorpusSnapshot(cfg, repaired.records)
		if !ok {
			return repositoryRepairPlan{}, errors.New("the repaired metadata corpus could not be built into a snapshot")
		}
		recByPath := map[string]corpusRecord{}
		for _, r := range repaired.records {
			recByPath[r.path] = r
		}
		for _, f := range derivedRepairFiles(repairable) {
			b, err := composeDerivedRepairBytes(sc, snap, repaired, recByPath, f)
			if err != nil {
				return repositoryRepairPlan{}, fmt.Errorf("composing the repaired %s: %w", f, err)
			}
			p.contents[f] = b
		}
	}

	for f := range p.contents {
		p.files = append(p.files, f)
	}
	sort.Strings(p.files)
	return p, nil
}

// manualReviewLine renders one manual-review entry.
func manualReviewLine(code, path, message string) string {
	return fmt.Sprintf("[%s] %s: %s", code, path, message)
}

// executeRepositoryRepair publishes the planned bytes as a single descendant of
// the pinned metadata tip under an exact owned lease and re-reads the
// postcondition byte-exactly. Only the planned files change; every other blob is
// carried forward from the pinned tree.
func executeRepositoryRepair(ctx context.Context, git *gitcli.Client, sc setupContext, metadataTip string, plan repositoryRepairPlan) RepositoryRepairResult {
	tipOID := gitcli.ObjectID(metadataTip)
	docketRef := metadataRef(sc.layout)

	ops := make([]gitcli.TreeOp, 0, len(plan.files))
	for _, f := range plan.files {
		ops = append(ops, gitcli.TreeOp{PutBlob: &gitcli.PutBlobOp{Path: gitcli.RepoPath(f), Content: plan.contents[f], Mode: blobMode}})
	}
	tree, err := git.BuildTree(ctx, sc.repo, tipOID, ops)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "composing the repaired metadata tree", err)
	}
	commit, err := git.CommitTree(ctx, sc.repo, tree, []gitcli.ObjectID{tipOID}, repositoryRepairSubject, nil)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "creating the repair commit", err)
	}
	out, err := git.PushLease(ctx, sc.repo, metadataRemote(sc.layout), docketRef, commit, tipOID)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "publishing the repair", err)
	}
	switch out.Disposition {
	case gitcli.PushApplied:
		// proceed
	case gitcli.PushLeaseLost:
		return repairContended(string(out.Remote), metadataTip)
	default:
		return repairExternalFailure(reposetup.StateHealthy, "publishing the repair", errors.New("docket lease push failed"))
	}
	rev, err := git.FetchBranch(ctx, sc.repo, metadataRemote(sc.layout), docketRef)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "re-reading the repaired metadata branch", err)
	}
	if rev.Commit != commit {
		return repairContended(string(rev.Commit), metadataTip)
	}
	return repairApplied(string(commit), metadataTip, plan)
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

// composeDerivedRepairBytes recomputes the canonical bytes for one repaired file.
// The board and ADR index are whole-file renders; an artifact-links file is the
// record with its managed block rewritten (or, when absent, inserted after the
// frontmatter) — never any other authored byte.
func composeDerivedRepairBytes(sc setupContext, snap domain.Snapshot, corpus checkCorpus, recByPath map[string]corpusRecord, file string) ([]byte, error) {
	switch file {
	case boardCorpusPath(sc.cfg):
		return renderCanonicalBoard(snap, corpusBoardUnrenderable(sc.cfg, corpus.records), boardPresentation(sc.cfg))
	case adrIndexCorpusPath(sc.cfg):
		return renderCanonicalADRIndex(snap, corpusADRIndexUnrenderable(sc.cfg, corpus.records))
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

// --- result constructors -----------------------------------------------------

// newRepairResult stamps the envelope for a repair outcome.
func newRepairResult(result Result, out RepositoryRepairResult) RepositoryRepairResult {
	out.Envelope = NewEnvelope(OperationRepositoryRepair, result)
	return out
}

// repairPreviewText renders the confirmation preview: repo, the metadata remote
// the repair publishes to, exact pinned
// metadata revision, each repair as `[code] path` (frontmatter entries with their
// patch preview), and the manual-review list.
func repairPreviewText(sc setupContext, metadataTip string, plan repositoryRepairPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "docket repository repair — preview\n")
	fmt.Fprintf(&b, "  repository:  %s\n", sc.repo.PrimaryWorktree)
	fmt.Fprintf(&b, "  remote:      %s\n", metadataRemote(sc.layout))
	fmt.Fprintf(&b, "  metadata:    %s @ %s\n", sc.layout.MetadataBranch, metadataTip)
	fmt.Fprintf(&b, "  repairs:\n")
	for _, f := range plan.frontmatter {
		fmt.Fprintf(&b, "    [%s] %s\n", f.Code, f.Path)
		fmt.Fprintf(&b, "%s\n", indentPatch(f.Patch))
	}
	for _, f := range plan.derived {
		fmt.Fprintf(&b, "    [%s] %s\n", f.Code, f.Path)
	}
	writeManualReview(&b, plan.manual)
	return b.String()
}

// writeManualReview appends the manual-review block when there is one.
func writeManualReview(b *strings.Builder, manual []string) {
	if len(manual) == 0 {
		return
	}
	fmt.Fprintf(b, "  manual review (not repaired):\n")
	for _, m := range manual {
		fmt.Fprintf(b, "    %s\n", m)
	}
}

// repairConfirmationRequired is the unauthorized preview: invalid-state with
// repository_state confirmation-required, naming --yes. SourceRevision is the
// pinned metadata tip so the confirm flow re-proves the exact copy shown.
func repairConfirmationRequired(sc setupContext, metadataTip string, plan repositoryRepairPlan) RepositoryRepairResult {
	out := newRepairResult(ResultInvalidState, RepositoryRepairResult{
		RepositoryState: repairConfirmationRequiredState,
		SourceRevision:  metadataTip,
		Repairs:         plan.frontmatter,
		RepairedViews:   derivedRepairFiles(plan.derived),
		RepairedFiles:   plan.files,
		ManualReview:    plan.manual,
	})
	out.human = repairPreviewText(sc, metadataTip, plan) +
		"\nconfirmation required: re-run with --yes to authorize these repairs"
	return out
}

// repairApplied is the success document: new and prior tips, the repaired file
// set, and the local sync remedy (the remote advanced; .docket fast-forwards on
// the next `docket repository prepare`).
func repairApplied(newTip, priorTip string, plan repositoryRepairPlan) RepositoryRepairResult {
	pending := []string{
		"fast-forward your local .docket metadata worktree: re-run `docket repository prepare` to sync it to the repaired metadata revision",
	}
	out := newRepairResult(ResultApplied, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateNeedsReview),
		SourceRevision:  priorTip,
		MetadataTip:     newTip,
		Repairs:         plan.frontmatter,
		RepairedViews:   derivedRepairFiles(plan.derived),
		RepairedFiles:   plan.files,
		ManualReview:    plan.manual,
		PendingLocal:    pending,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "repository repaired: metadata %s (%d file(s))\npending local sync: %s\n",
		newTip, len(plan.files), strings.Join(pending, "; "))
	writeManualReview(&b, plan.manual)
	out.human = strings.TrimRight(b.String(), "\n")
	return out
}

// repairNoOp is the idempotent nothing-to-repair document. Manual-review items,
// if any, are still listed — they are a human's to resolve.
func repairNoOp(metadataTip string, manual []string) RepositoryRepairResult {
	out := newRepairResult(ResultNoOp, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  metadataTip,
		ManualReview:    manual,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "repository repair: nothing to repair at metadata %s\n", metadataTip)
	writeManualReview(&b, manual)
	out.human = strings.TrimRight(b.String(), "\n")
	return out
}

// repairContended reports that the docket metadata tip differs from the copy the
// repair decided on. Nothing was overwritten.
func repairContended(freshTip, expected string) RepositoryRepairResult {
	out := newRepairResult(ResultContended, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  expected,
		MetadataTip:     freshTip,
	})
	out.human = fmt.Sprintf("repair contended: the docket metadata branch moved to %s since the repair pinned %s; re-run to preview the new state",
		freshTip, expected)
	return out
}

// repairRefusal builds an invalid-state refusal naming a remedy valid in exactly
// the classified state.
func repairRefusal(state reposetup.State, remedy string) RepositoryRepairResult {
	out := newRepairResult(ResultInvalidState, RepositoryRepairResult{RepositoryState: string(state)})
	out.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryRepair, ResultInvalidState, state, remedy)
	return out
}

// repairExternalFailure builds an external-failed result naming the stage.
func repairExternalFailure(state reposetup.State, stage string, err error) RepositoryRepairResult {
	out := newRepairResult(ResultExternalFailed, RepositoryRepairResult{RepositoryState: string(state)})
	out.human = fmt.Sprintf("%s: %s while %s: %s", OperationRepositoryRepair, ResultExternalFailed, stage, err.Error())
	return out
}

// repairInternalFailure builds an internal-error result naming the stage.
func repairInternalFailure(state reposetup.State, stage string, err error) RepositoryRepairResult {
	out := newRepairResult(ResultInternalError, RepositoryRepairResult{RepositoryState: string(state)})
	out.human = fmt.Sprintf("%s: %s while %s: %s", OperationRepositoryRepair, ResultInternalError, stage, err.Error())
	return out
}

// repairFromMigrateResult re-stamps a shared topology/gather refusal (built by
// the migrate route or gather-failure mapping) as a repository.repair result,
// keeping its result, state, findings, and remedy text.
func repairFromMigrateResult(m RepositoryMigrateResult) RepositoryRepairResult {
	out := newRepairResult(m.Result, RepositoryRepairResult{
		RepositoryState: m.RepositoryState,
		Findings:        m.Findings,
	})
	out.human = strings.Replace(m.HumanText(), OperationRepositoryMigrate, OperationRepositoryRepair, 1)
	return out
}
