package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file is the `change kill` planning operation: a non-allocating terminal
// transition that relocates a proposed or in-progress change into the archive
// and lands every affected v1-owned derived view (the killed record's owned
// lifecycle fields, its refreshed updated date, its spliced ## Why killed
// section, its re-rendered artifact block; a metadata-resident linked spec's
// retargeted backlink; the inline board) as one validated atomic transaction.
// The domain owns legality: domain.Kill decides whether the current status may
// take the transition and yields the field changes — including the cleared
// claim stamp and branch a killed record must not carry — so this layer decides
// no lifecycle policy of its own. Killing edits an existing record, so it pins
// the submitted record version with an exact-blob entity expectation rather than
// an idempotency key. It inspects no process, branch, worktree, or PR state.
//
// Kill is a rename, not a reuse: the archive move is one MutationCreate at the
// archive path plus one MutationDelete of the active path in the same plan.
// Leaving the active file would keep the change visibly alive (presence-encoded
// state), so the delete is part of the transition. Identity checks compare by
// record content, not path — repository.ValidateEvolution already models the
// relocation, so no path-keyed check is added here.

// OperationChangeKill is the operation key `change kill` records in its result
// envelope and its transaction trailer.
const OperationChangeKill = "change.kill"

// whyKilledHeading is the owned authored section `change kill` replaces (or
// inserts when absent) with the caller's kill rationale.
const whyKilledHeading = "## Why killed"

// ChangeKillRequest is the closed, caller-supplied request for one kill. Path
// and Version pin the exact submitted record; WhyKilled is the non-empty
// authored ## Why killed section body. Authored text rides inside the string
// fields and is never interpolated into any shell command.
type ChangeKillRequest struct {
	ChangeID  int    `json:"change_id"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	WhyKilled string `json:"why_killed"`
}

// ChangeKillResult is the protocol-v1 document `change kill` returns. It embeds
// the envelope; ArchivePath carries the relocated record's new canonical path on
// a successful apply, and Findings carries every refusal or validation
// diagnostic (marshalled as [] never null).
type ChangeKillResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	ArchivePath string          `json:"archive_path,omitempty"`
	Revision    string          `json:"committed_revision,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders the one-line human summary of a kill outcome.
func (r ChangeKillResult) HumanText() string {
	switch r.Result {
	case ResultApplied:
		return fmt.Sprintf("change %04d killed (archived %s) — %s", r.ID, r.ArchivePath, r.Revision)
	default:
		return fmt.Sprintf("change kill: %s", r.Result)
	}
}

// newChangeKillResult stamps the envelope and normalizes Findings to an empty
// slice so the array marshals as [] on every path.
func newChangeKillResult(result Result, r ChangeKillResult) ChangeKillResult {
	r.Envelope = NewEnvelope(OperationChangeKill, result)
	if r.Findings == nil {
		r.Findings = []StatusFinding{}
	}
	return r
}

// changeKillReceipt is the canonical receipt persisted with a kill commit. Field
// order is alphabetical so json.Marshal emits the canonical, sorted-key compact
// form the engine's receipt validator requires.
type changeKillReceipt struct {
	ArchivePath string `json:"archive_path"`
	ID          int    `json:"id"`
	Op          string `json:"op"`
}

// ChangeKill validates the request, pins authoritative context, and drives one
// atomic transaction that kills the change (archiving it, splicing its
// ## Why killed section, retargeting a linked spec's backlink) and — when inline
// is enabled — re-renders the board. Every failure that predates the transaction
// (bad request shape, an empty rationale, a github board surface) returns
// without an engine call.
func ChangeKill(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeKillRequest) ChangeKillResult {
	findings := validateLifecycleShape(req.ChangeID, req.Path, req.Version)
	if strings.TrimSpace(req.WhyKilled) == "" {
		findings = append(findings, lifecycleFinding("empty-why_killed", "why_killed must be a non-empty authored section body"))
	}
	if len(findings) > 0 {
		return newChangeKillResult(ResultInvalidInput, ChangeKillResult{Findings: findings})
	}

	// Pin authoritative context: the metadata mode, branches, and resolved
	// configuration the board fence consults.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newChangeKillResult(result, ChangeKillResult{
			Findings: []StatusFinding{{Code: reason, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}
	eff := pin.Config.Effective

	// Board-surface fence: a github surface is an unsupported configuration,
	// refused before any transaction; otherwise learn whether inline is on.
	inline, err := fenceBoardSurface(eff)
	if err != nil {
		if pe, ok := asPlanningError(err); ok {
			return newChangeKillResult(pe.Result, ChangeKillResult{
				Findings: []StatusFinding{{Code: pe.Reason, Severity: string(domain.SeverityError), Message: pe.Message}},
			})
		}
		return newChangeKillResult(ResultInternalError, ChangeKillResult{
			Findings: []StatusFinding{{Code: ReasonStatusInternalError, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}

	// Discover the repository identity the transaction writes against.
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return newChangeKillResult(result, ChangeKillResult{
			Findings: []StatusFinding{{Code: reason, Severity: string(domain.SeverityError), Message: err.Error()}},
		})
	}

	op := changeKillOp{
		changeID:   req.ChangeID,
		path:       req.Path,
		whyKilled:  req.WhyKilled,
		eff:        eff,
		clock:      deps.Clock,
		inline:     inline,
		link:       render.LinkContext{MetadataBranch: metadataBranchOf(pin)},
		changesDir: eff.ChangesDir.Value,
	}

	res, execErr := deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + metadataBranchOf(pin)),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(req.Path),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})

	return changeKillResultFromOutcome(res, execErr)
}

// changeKillResultFromOutcome folds a transaction outcome into the result
// document. A refusal from this operation is always state-shaped (an illegal
// source status, a not-found record), so the refusal maps onto invalid-state.
func changeKillResultFromOutcome(res transaction.Result, execErr error) ChangeKillResult {
	result, _ := mapOutcome(res, execErr, ResultInvalidState)

	out := ChangeKillResult{Findings: findingsToStatus(res.Findings)}
	if result == ResultApplied {
		if rec, ok := decodeChangeKillReceipt(res.Receipt); ok {
			out.ID = rec.ID
			out.ArchivePath = rec.ArchivePath
		}
		out.Revision = string(res.AppliedCommit)
	}
	return newChangeKillResult(result, out)
}

// decodeChangeKillReceipt decodes a persisted receipt into its identity fields.
func decodeChangeKillReceipt(b []byte) (changeKillReceipt, bool) {
	if len(b) == 0 {
		return changeKillReceipt{}, false
	}
	var rec changeKillReceipt
	if err := json.Unmarshal(b, &rec); err != nil {
		return changeKillReceipt{}, false
	}
	return rec, true
}

// changeKillOp is the SemanticOperation the engine drives per attempt. Every
// field is fixed before the transaction; the state-dependent work (the domain
// legality gate, section splicing, field patching, relocation, rendering)
// re-runs from the attempt's own fresh state.
type changeKillOp struct {
	changeID   int
	path       string
	whyKilled  string
	eff        config.Effective
	clock      transaction.Clock
	inline     bool
	link       render.LinkContext
	changesDir string
}

func (o changeKillOp) Key() transaction.OperationKey { return OperationChangeKill }

// Plan gates the kill against the attempt's snapshot via domain.Kill, applies
// the domain's owned field changes (status, cleared claim + branch) plus the
// refreshed updated date, splices the ## Why killed section, relocates the
// record into the archive, re-renders the artifact block, retargets a
// metadata-resident linked spec's backlink, and assembles the closed plan: the
// archive create, the active delete, the optional spec replace, and the
// re-rendered board when inline is enabled.
func (o changeKillOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	snap := st.State.Snapshot

	c, out := snap.Change(domain.ChangeID(o.changeID))
	if out != domain.LookupFound {
		return refuseLifecycle("not-found", fmt.Sprintf("change %04d is not present in the current corpus", o.changeID))
	}

	// Domain legality gate: Kill decides whether the current status may take the
	// transition and yields the owned FieldChanges (including the cleared claim
	// stamp and branch). An illegal source status is a state-shaped refusal.
	result, fail := domain.Kill(c)
	if fail != nil {
		return refuseLifecyclePolicy(fail)
	}

	src, ok := st.State.Sources[o.path]
	if !ok {
		return refuseLifecycle("path-mismatch",
			fmt.Sprintf("no record source loaded at %q for change %04d", o.path, o.changeID))
	}

	// Splice the ## Why killed authored section first, over the exact source
	// bytes (replace inserts it at EOF when absent).
	edited, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: whyKilledHeading, Intent: render.SectionReplace, Markdown: o.whyKilled},
	})
	if err != nil {
		return refuseLifecycle("section-edit-failed", err.Error())
	}

	// First patch pass: the domain's owned kill FieldChanges plus the refreshed
	// updated date. The artifact block is filled in a second pass, because
	// rendering it needs the relocated candidate snapshot.
	doc1, err := document.Parse(edited)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: reparsing edited record: %w", err)
	}
	var ps document.PatchSet
	for _, fc := range result.Changed {
		ps.SetField(fc.Field, lifecycleFieldValue(fc.To))
	}
	// upsertField (not bare SetField): the updated: field is inserted when a record
	// lacks it (a Bash-era or hand-authored record), so this op degrades like the
	// ADR ops, which upsert the same field, rather than internal-erroring with a
	// KindMissingPatchTarget.
	upsertField(&ps, doc1, "updated", document.String(o.clock.Now().UTC().Format("2006-01-02")))
	intermediate, err := doc1.Apply(ps)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: patching record fields: %w", err)
	}

	// The candidate snapshot reflects the relocation: the record removed from the
	// active path and re-placed at the archive path. It resolves the artifact
	// block's rows, the retargeted spec backlink, and drives the board — and it is
	// byte-for-byte the state the engine's after-load will validate.
	archivePath := path.Join(o.changesDir, "archive",
		fmt.Sprintf("%s-%04d-%s.md", o.clock.Now().UTC().Format("2006-01-02"), o.changeID, c.Slug()))
	candidate, err := buildKillCandidate(o.eff, st.State.Documents, o.path, archivePath, intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	gc, gout := candidate.Change(domain.ChangeID(o.changeID))
	if gout != domain.LookupFound {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: killed record %04d absent from candidate snapshot", o.changeID)
	}

	body, err := render.ArtifactBlockContent(gc, candidate, o.link)
	if err != nil {
		return refuseLifecycle("artifact-render-failed", err.Error())
	}
	doc2, err := document.Parse(intermediate)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: reparsing patched record: %w", err)
	}
	var ps2 document.PatchSet
	// ReplaceBlock (not upsert) assumes the docket:artifacts block is present —
	// render.ChangeRecord always emits it for canonical v1 records, and the ADR ops
	// make the same assumption on the same corpus. A record without the block is
	// out of scope here (there is no v1 producer of one).
	ps2.ReplaceBlock("artifacts", body)
	finalBytes, err := doc2.Apply(ps2)
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: writing artifact block: %w", err)
	}

	// Archive move: create the archived record, delete the active path — one plan.
	files := []transaction.FileMutation{
		{Path: gitcli.RepoPath(archivePath), Kind: transaction.MutationCreate, Bytes: finalBytes},
		{Path: gitcli.RepoPath(o.path), Kind: transaction.MutationDelete},
	}

	// Retarget a metadata-resident linked spec's backlink to the archive path. A
	// spec outside the metadata tree or absent from it yields no spec mutation and
	// no failure.
	if specPath := gc.Spec().Value; specPath != "" {
		specBytes, present, err := readTreeBlob(ctx, st.Tree, specPath)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		if present {
			specDoc, err := document.Parse(specBytes)
			if err != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: parsing linked spec %q: %w", specPath, err)
			}
			// A spec present but carrying no docket:backlink managed block (a
			// hand-authored or Bash-era spec) has no block to retarget:
			// ReplaceBlock would fail KindMissingPatchTarget, surfacing as a
			// misleading internal-error. Skip the spec mutation instead, matching
			// the absent-spec contract — no spec mutation, no failure. The kill
			// still archives the change and updates the board.
			if _, ok := specDoc.Block("backlink"); ok {
				backlink, err := render.BacklinkContent(gc, o.link)
				if err != nil {
					return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: rendering spec backlink: %w", err)
				}
				var sps document.PatchSet
				sps.ReplaceBlock("backlink", backlinkInterior(backlink))
				specFinal, err := specDoc.Apply(sps)
				if err != nil {
					return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: retargeting spec backlink in %q: %w", specPath, err)
				}
				files = append(files, transaction.FileMutation{
					Path: gitcli.RepoPath(specPath), Kind: transaction.MutationReplace, Bytes: specFinal,
				})
			}
		}
	}

	if o.inline {
		boardBytes, err := render.Board(render.BoardInput{Snapshot: candidate})
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: rendering board: %w", err)
		}
		boardPath := path.Join(o.changesDir, "BOARD.md")
		kind, err := boardMutationKind(ctx, st.Tree, boardPath)
		if err != nil {
			return transaction.MutationPlan{}, transaction.OperationResult{}, err
		}
		files = append(files, transaction.FileMutation{
			Path: gitcli.RepoPath(boardPath), Kind: kind, Bytes: boardBytes,
		})
	}

	receipt, err := json.Marshal(changeKillReceipt{
		ArchivePath: archivePath, ID: o.changeID, Op: OperationChangeKill,
	})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, fmt.Errorf("change kill: encoding receipt: %w", err)
	}

	return transaction.MutationPlan{
		Files:         files,
		CommitSubject: fmt.Sprintf("change %04d killed", o.changeID),
		Receipt:       receipt,
	}, transaction.OperationResult{}, nil
}

// buildKillCandidate rebuilds the complete snapshot the attempt would see after
// the kill lands: every existing corpus document reclassified by its path, with
// the killed change's document removed from the active path and re-placed at the
// archive path with an archive location. The reclassification mirrors the
// planning loader so the candidate is the state the engine's after-load will
// validate.
func buildKillCandidate(eff config.Effective, docs map[string]document.Document, activePath, archivePath string, killedBytes []byte) (domain.Snapshot, error) {
	killedDoc, err := document.Parse(killedBytes)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("change kill: parsing killed record: %w", err)
	}

	paths := make([]string, 0, len(docs))
	for p := range docs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	inputs := make([]repository.InputDocument, 0, len(docs)+1)
	for _, p := range paths {
		if p == activePath {
			// The record relocates to the archive path below; drop the active entry.
			continue
		}
		kind, loc, ok := classifyCorpusPath(eff, p)
		if !ok {
			continue
		}
		inputs = append(inputs, repository.InputDocument{
			Kind: kind, Location: loc, Path: p, Document: docs[p],
		})
	}
	inputs = append(inputs, repository.InputDocument{
		Kind: repository.KindChange, Location: repository.LocationArchive, Path: archivePath, Document: killedDoc,
	})

	build, err := repository.BuildSnapshot(repository.BuildInput{Config: eff, Documents: inputs})
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("change kill: building candidate snapshot: %w", err)
	}
	return build.Snapshot, nil
}

// readTreeBlob reads one path from the base tree, returning its bytes and
// whether it exists.
func readTreeBlob(ctx context.Context, tree transaction.Tree, p string) ([]byte, bool, error) {
	results, err := tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(p)})
	if err != nil {
		return nil, false, fmt.Errorf("change kill: reading path %q: %w", p, err)
	}
	if len(results) != 1 || !results[0].Found {
		return nil, false, nil
	}
	return results[0].Blob.Bytes, true, nil
}

// backlinkInterior extracts the interior of a rendered docket:backlink block —
// the "> ↩ ..." line(s) between the two marker lines — so it can feed
// PatchSet.ReplaceBlock, which owns the markers. render.BacklinkContent emits the
// complete block; ReplaceBlock rewrites only the interior.
func backlinkInterior(block string) string {
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	if len(lines) < 2 {
		return ""
	}
	return strings.Join(lines[1:len(lines)-1], "\n")
}
