package app

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `workspace commit-spec` operation: it commits a copy of the
// change's metadata spec onto its feature branch, at the spec's own path, so a
// PR carries exactly the spec copy plus code. The copy is a pure function of the
// metadata spec and the change (specCopyBytes): the spec's docket:backlink block
// is dropped and one plain change line (specCopyChangeLine) is inserted after its
// title. The operation is idempotent — a feature head that already carries the
// exact copy is a no-op — and a revised metadata spec refreshes the copy with a
// second commit.
//
// The commit is built from the feature head's own tree (never the worktree
// index), and the checked-out branch then moves to it by compare-and-swap from
// the inspected head (gitcli.FastForwardCheckedOutBranch). Nothing in the
// worktree is touched until the commit object exists, a commit made since the
// inspection refuses with nothing changed, and a refused tree update rolls the
// branch back — so a failed call leaves the workspace exactly as it found it.

// OperationWorkspaceCommitSpec is the operation key commit-spec records in its
// envelope.
const OperationWorkspaceCommitSpec = "workspace.commit-spec"

// The dispositions commit-spec reports in WorkspaceOpResult.Disposition.
const (
	// SpecCopyCommitted: the first copy was committed (applied).
	SpecCopyCommitted = "committed"
	// SpecCopyRefreshed: a revised spec replaced the copy (applied).
	SpecCopyRefreshed = "refreshed"
	// SpecCopyCurrent: the feature head already carries the exact copy (no-op).
	SpecCopyCurrent = "current"
	// SpecCopyNoSpec: the change links no spec (a trivial change), so there is
	// no copy (no-op).
	SpecCopyNoSpec = "no-spec"
)

// The stable machine reasons commit-spec reports for its typed refusals.
const (
	// ReasonSpecCopyNotReady: the workspace is not in the ready (clean, owned)
	// state, so nothing is written into it.
	ReasonSpecCopyNotReady = "workspace-not-ready"
	// ReasonSpecCopyMissing: the record's spec: field names no file on the
	// metadata tip.
	ReasonSpecCopyMissing = "spec-missing"
	// ReasonSpecCopyPathOccupied: the integration branch already holds different
	// bytes at the spec path, or the feature head holds something other than a
	// regular file there — the copy would collide with content it does not own.
	ReasonSpecCopyPathOccupied = "spec-path-occupied"
	// ReasonSpecCopyMalformed: the metadata spec's managed markers do not parse,
	// so no copy is built from it.
	ReasonSpecCopyMalformed = "spec-malformed"
)

// specCopyFileMode is the tree mode every spec copy is committed with.
const specCopyFileMode = gitcli.FileMode("100644")

// specCopyChangeLine renders the one plain line the feature-branch spec copy
// carries after its title. It has no URL, so it never needs updating. It is the
// single seam a visibility mode would change (a private repository omits it).
func specCopyChangeLine(c domain.Change) string {
	return fmt.Sprintf("Change %04d — %s", int(c.ID()), c.Title())
}

// specCopyBytes renders the copy of a change's metadata spec that rides the PR:
// the spec without its docket:backlink block, with specCopyChangeLine inserted
// after the first top-level "# " title outside fenced code (or at the top when
// the spec has no title). Line endings follow the source. Malformed managed
// markers refuse — the copy is never built from a spec the parser rejects.
func specCopyBytes(spec []byte, c domain.Change) ([]byte, error) {
	doc, err := document.Parse(spec)
	if err != nil {
		return nil, fmt.Errorf("spec copy: %w", err)
	}
	le := doc.LineEnding()
	if le == "" {
		le = "\n"
	}
	body := spec
	if _, ok := doc.Block(backlinkBlockName); ok {
		var ps document.PatchSet
		ps.RemoveBlock(backlinkBlockName)
		if body, err = doc.Apply(ps); err != nil {
			return nil, fmt.Errorf("spec copy: removing backlink: %w", err)
		}
	}
	text := strings.TrimLeft(string(body), "\r\n")
	line := specCopyChangeLine(c)
	end, ok := firstTitleLineEnd(text)
	if !ok {
		if text == "" {
			return []byte(line + le), nil
		}
		return []byte(line + le + le + text), nil
	}
	head, tail := text[:end], text[end:]
	if !strings.HasSuffix(head, "\n") {
		// The title is the spec's last, unterminated line.
		head += le
	}
	sep := le
	if tail != "" && !strings.HasPrefix(tail, "\n") && !strings.HasPrefix(tail, "\r\n") {
		// The title runs straight into text: keep the change line its own
		// paragraph rather than gluing it to the next line.
		sep = le + le
	}
	return []byte(head + le + line + sep + tail), nil
}

// firstTitleLineEnd returns the offset just past the line terminator (or EOF) of
// the first line of text that starts with "# " outside fenced code, and whether
// there is one. It scans with walkUnfencedLines, the same fence rules
// scanTopHeadings applies, so heading-shaped text inside a fence is content.
func firstTitleLineEnd(text string) (int, bool) {
	end, found := 0, false
	walkUnfencedLines([]byte(text), func(line []byte, _, lineEnd int) bool {
		if bytes.HasPrefix(line, []byte("# ")) {
			end, found = lineEnd, true
			return false
		}
		return true
	})
	return end, found
}

// WorkspaceCommitSpec commits the change's spec copy onto its feature branch.
// It refuses unless the change is in-progress and its workspace is ready (clean
// and owned); a change with no spec is a no-op. The copy is built from the spec
// on the pinned metadata tip; an integration branch that already holds different
// bytes at the spec path refuses (the copy would collide on merge). The feature
// head is compared byte-for-byte: equal is a no-op, absent commits "Add design
// spec: <title>", different commits "Update design spec: <title>".
func WorkspaceCommitSpec(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req WorkspaceIDRequest) WorkspaceOpResult {
	const op = OperationWorkspaceCommitSpec
	wc, refusal := loadWorkspaceContext(ctx, deps, repoDir, req.ID, op)
	if refusal != nil {
		return *refusal
	}
	if wc.change.Status() != domain.StatusInProgress {
		return newWorkspaceResult(op, ResultInvalidState, WorkspaceOpResult{
			ID:      req.ID,
			Reason:  ReasonWorkspaceNotInProgress,
			Message: fmt.Sprintf("change %04d is %q, not in-progress; the spec copy is committed only onto a claimed change's workspace", req.ID, wc.change.RawStatus()),
		})
	}

	specPath := strings.TrimSpace(wc.change.Spec().Value)
	if specPath == "" {
		return newWorkspaceResult(op, ResultNoOp, WorkspaceOpResult{
			ID:          req.ID,
			Disposition: SpecCopyNoSpec,
			Message:     fmt.Sprintf("change %04d links no spec; there is no spec copy to commit", req.ID),
		})
	}

	target, tRefusal := resolveWorkspaceTarget(op, wc)
	if tRefusal != nil {
		return *tRefusal
	}
	insp, err := wdeps.Service.Inspect(ctx, workspace.InspectRequest{Repository: wc.repo, Target: target})
	if err != nil {
		return mapWorkspaceFailure(op, req.ID, err)
	}
	// refuse builds a typed refusal that carries the workspace facts read so far.
	refuse := func(result Result, reason, msg string) WorkspaceOpResult {
		return newWorkspaceResult(op, result, WorkspaceOpResult{
			ID:         req.ID,
			Path:       specPath,
			FeatureRef: string(target.FeatureRef),
			Head:       string(insp.HeadCommit),
			State:      string(insp.Kind),
			Reason:     reason,
			Message:    msg,
		})
	}
	if insp.Kind != workspace.StateReady {
		// The state kind only — never the dirty paths themselves.
		return refuse(ResultInvalidState, ReasonSpecCopyNotReady,
			fmt.Sprintf("change %04d's workspace is %q, not ready; commit or discard local work before committing the spec copy", req.ID, insp.Kind))
	}

	// The copy is built from the spec on the pinned metadata tip.
	art, err := deps.Reader.ReadArtifact(ctx, wc.pin, sourceMetadata, specPath)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return refuse(result, reason, err.Error())
	}
	if !art.Found {
		return refuse(ResultInvalidState, ReasonSpecCopyMissing,
			fmt.Sprintf("change %04d links spec %s, which does not exist on the metadata branch", req.ID, specPath))
	}
	copyBytes, err := specCopyBytes(art.Data, wc.change)
	if err != nil {
		return refuse(ResultInvalidState, ReasonSpecCopyMalformed,
			fmt.Sprintf("the metadata spec %s cannot be copied: %v", specPath, err))
	}

	// The integration branch must not already hold different bytes at the path:
	// the PR would collide with content the change does not own.
	integ, err := deps.Reader.ReadArtifact(ctx, wc.pin, sourceIntegration, specPath)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return refuse(result, reason, err.Error())
	}
	if integ.Found && !bytes.Equal(integ.Data, copyBytes) {
		return refuse(ResultInvalidState, ReasonSpecCopyPathOccupied,
			fmt.Sprintf("the integration branch already holds different content at %s; the spec copy would collide with it", specPath))
	}

	// Compare against the feature head itself.
	src, err := deps.Client.OpenObjectSource(ctx, wc.repo, gitcli.Revision{Commit: insp.HeadCommit})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return refuse(result, reason, err.Error())
	}
	blobs, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(specPath)})
	if err != nil {
		result, reason := classifyStatusError(ctx, classifyGitFailure(err))
		return refuse(result, reason, err.Error())
	}
	subject, disposition := "Add design spec: "+wc.change.Title(), SpecCopyCommitted
	if len(blobs) == 1 && blobs[0].Found {
		if !strings.HasPrefix(string(blobs[0].Blob.Mode), "100") {
			return refuse(ResultInvalidState, ReasonSpecCopyPathOccupied,
				fmt.Sprintf("the feature head holds a non-regular entry (mode %s) at %s; the spec copy is never written over it", blobs[0].Blob.Mode, specPath))
		}
		if bytes.Equal(blobs[0].Blob.Bytes, copyBytes) {
			return newWorkspaceResult(op, ResultNoOp, WorkspaceOpResult{
				ID:          req.ID,
				Path:        specPath,
				FeatureRef:  string(target.FeatureRef),
				BaseRef:     string(target.BaseRef),
				BaseCommit:  string(insp.BaseCommit),
				Head:        string(insp.HeadCommit),
				Disposition: SpecCopyCurrent,
			})
		}
		subject, disposition = "Update design spec: "+wc.change.Title(), SpecCopyRefreshed
	}

	// Run mutation fence: a cancelled, superseded, or completing run that owns
	// this worktree admits no new commit. A local write, so no publication
	// identity is journaled.
	done, ferr := admitWorkflowMutation(repoDir, op, nil)
	if ferr != nil {
		return workspaceFenceRefusalFor(op, req.ID, ferr)
	}

	head, err := commitSpecCopy(ctx, deps.Client, wc.repo, insp, target, specPath, copyBytes, subject)
	if err != nil {
		result, reason := commitSpecFailure(ctx, err)
		out := refuse(result, reason, fmt.Sprintf("committing the spec copy failed: %v", err))
		done(mutationJournalOutcome(out.Result))
		return out
	}
	out := newWorkspaceResult(op, ResultApplied, WorkspaceOpResult{
		ID:          req.ID,
		Path:        specPath,
		FeatureRef:  string(target.FeatureRef),
		BaseRef:     string(target.BaseRef),
		BaseCommit:  string(insp.BaseCommit),
		Head:        string(head),
		Disposition: disposition,
	})
	done(mutationJournalOutcome(out.Result))
	return out
}

// commitSpecFailure maps a commitSpecCopy failure onto the protocol taxonomy. A
// refusal from the in-place fast-forward (the branch moved past the inspected
// head, an unfinished Git operation, or a working-tree update refused over a
// local change) means the workspace stopped being ready after the inspection —
// nothing changed, so it is the same workspace-not-ready refusal. Anything else
// takes the ordinary Git-failure classification.
func commitSpecFailure(ctx context.Context, err error) (Result, string) {
	if f, ok := gitcli.AsFailure(err); ok && f.Kind == gitcli.KindInvalidRepository {
		return ResultInvalidState, ReasonSpecCopyNotReady
	}
	return classifyStatusError(ctx, classifyGitFailure(err))
}

// commitSpecCopy builds the one-path commit on top of the inspected feature head
// — from the head's tree in a private index, never the worktree index — and moves
// the checked-out feature branch to it in place, by compare-and-swap from the
// inspected head. It returns the new head.
func commitSpecCopy(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, insp workspace.Inspection, target workspace.Target, specPath string, copyBytes []byte, subject string) (gitcli.ObjectID, error) {
	tree, err := client.BuildTree(ctx, repo, insp.HeadCommit, []gitcli.TreeOp{{PutBlob: &gitcli.PutBlobOp{
		Path:    gitcli.RepoPath(specPath),
		Content: copyBytes,
		Mode:    specCopyFileMode,
	}}})
	if err != nil {
		return "", err
	}
	commit, err := client.CommitTree(ctx, repo, tree, []gitcli.ObjectID{insp.HeadCommit}, subject, nil)
	if err != nil {
		return "", err
	}
	if err := client.FastForwardCheckedOutBranch(ctx, insp.Path, target.FeatureRef, insp.HeadCommit, commit); err != nil {
		return "", err
	}
	return commit, nil
}
