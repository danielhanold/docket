package gitcli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Operation labels for the owned-rebase and owned-ref surface.
const (
	rebaseBeginOp    Operation = "rebase-begin"
	rebaseStateOp    Operation = "rebase-state"
	rebaseContinueOp Operation = "rebase-continue"
	rebaseAbortOp    Operation = "rebase-abort"
	setOwnedRefOp    Operation = "set-owned-ref"
	deleteOwnedRefOp Operation = "delete-owned-ref"
)

// ownedRefRequiredPrefix fences the owned-ref primitives: SetOwnedRef,
// DeleteOwnedRef, and the orig/base anchors BeginRebase writes may only ever
// name a ref beneath it. It is Docket's private ref namespace; a ref outside it
// (a real branch, a tag) is never a Docket-owned scratch ref and must never be
// created, moved, or deleted through these primitives.
const ownedRefRequiredPrefix = "refs/docket/"

// rebaseEnv forces every rebase step non-interactive: GIT_EDITOR/GIT_SEQUENCE_EDITOR
// resolve to `true`, so a commit-message or todo-list editor invocation exits 0
// with the buffer unchanged rather than blocking on a terminal that a dispatched
// worker does not have. These are appended per-command (never mutating the base
// environment) and carry no repository redirection.
var rebaseEnv = []string{"GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true"}

// RebaseDisposition is the structural outcome of an owned rebase step.
type RebaseDisposition string

const (
	// RebaseUnchanged: the branch was already based on the target — no rewrite,
	// HEAD unmoved. RebaseState also reports it for "no rebase in progress".
	RebaseUnchanged RebaseDisposition = "unchanged"
	// RebaseRebased: the rebase rewrote (or advanced) the branch to a new tip.
	RebaseRebased RebaseDisposition = "rebased"
	// RebaseConflicted: the rebase stopped with unmerged paths the caller must
	// resolve; UnmergedPaths carries them.
	RebaseConflicted RebaseDisposition = "conflicted"
	// RebaseInProgressForeign: a rebase was already underway that this call did
	// not start (BeginRebase), or one that is stopped at no resolvable conflict
	// (RebaseState). Never adopted or reset here.
	RebaseInProgressForeign RebaseDisposition = "in-progress-foreign"
	// RebaseFailed: the rebase failed for a reason that is neither a clean
	// completion nor a resolvable conflict.
	RebaseFailed RebaseDisposition = "failed"
)

// RebaseStatus is the classified state of a worktree's rebase. HeadOID is the
// worktree HEAD at observation time (the last-applied commit while conflicted).
// UnmergedPaths is the exact, deduplicated, NUL-read set of conflicted paths,
// non-empty only when Disposition is conflicted.
type RebaseStatus struct {
	Disposition   RebaseDisposition
	HeadOID       ObjectID
	UnmergedPaths []string
}

// BeginRebase rebases worktreeDir's checked-out branch onto the exact baseOID.
// It refuses unless no rebase is already in progress (else in-progress-foreign,
// nothing touched), the worktree HEAD equals expectedHead, and the tree is
// clean. It records the owned recovery anchors <ownedRefPrefix>/orig (the
// pre-rebase head) and <ownedRefPrefix>/base (the target) BEFORE running git, so
// a crash mid-rewrite still leaves both the original tip and the intended base
// provable. ownedRefPrefix must live beneath refs/docket/. The result is
// classified structurally: unchanged, rebased, conflicted (with UnmergedPaths),
// or failed.
func (c *Client) BeginRebase(ctx context.Context, worktreeDir string, expectedHead, baseOID ObjectID, ownedRefPrefix string) (RebaseStatus, error) {
	if worktreeDir == "" {
		return RebaseStatus{}, newFailure(rebaseBeginOp, KindInvalidRequest, "worktree dir is empty", nil)
	}
	if err := validateObjectID(expectedHead); err != nil {
		return RebaseStatus{}, newFailure(rebaseBeginOp, KindInvalidRequest, "invalid expected head id", err)
	}
	if err := validateObjectID(baseOID); err != nil {
		return RebaseStatus{}, newFailure(rebaseBeginOp, KindInvalidRequest, "invalid base id", err)
	}
	origRef := RefName(ownedRefPrefix + "/orig")
	baseRef := RefName(ownedRefPrefix + "/base")
	if err := validateOwnedRef(origRef); err != nil {
		return RebaseStatus{}, newFailure(rebaseBeginOp, KindInvalidRequest, "owned ref prefix does not yield an owned refs/docket/ ref", err)
	}
	if err := validateOwnedRef(baseRef); err != nil {
		return RebaseStatus{}, newFailure(rebaseBeginOp, KindInvalidRequest, "owned ref prefix does not yield an owned refs/docket/ ref", err)
	}

	// A rebase already underway is foreign to this call: never reset, adopt, or
	// overwrite it, and never write our anchors over it.
	inProgress, f := c.rebaseInProgress(ctx, rebaseBeginOp, worktreeDir)
	if f != nil {
		return RebaseStatus{}, f
	}
	if inProgress {
		head, hf := c.worktreeHead(ctx, rebaseBeginOp, worktreeDir)
		if hf != nil {
			return RebaseStatus{}, hf
		}
		return RebaseStatus{Disposition: RebaseInProgressForeign, HeadOID: head}, nil
	}

	// Exact expected head, then a clean tree — both proven before any mutation so
	// a refusal leaves the worktree and the owned refs untouched.
	head, hf := c.worktreeHead(ctx, rebaseBeginOp, worktreeDir)
	if hf != nil {
		return RebaseStatus{}, hf
	}
	if head != expectedHead {
		return RebaseStatus{}, newFailure(rebaseBeginOp, KindInvalidRepository, "worktree HEAD is not at the expected commit", nil)
	}
	changes, err := c.ChangedPaths(ctx, worktreeDir)
	if err != nil {
		return RebaseStatus{}, err
	}
	if len(changes) > 0 {
		return RebaseStatus{}, newFailure(rebaseBeginOp, KindInvalidRepository, "worktree has uncommitted changes", nil)
	}

	// Recovery anchors first, then the rewrite.
	if f := c.setOwnedRefInDir(ctx, rebaseBeginOp, worktreeDir, origRef, expectedHead); f != nil {
		return RebaseStatus{}, f
	}
	if f := c.setOwnedRefInDir(ctx, rebaseBeginOp, worktreeDir, baseRef, baseOID); f != nil {
		return RebaseStatus{}, f
	}
	// baseOID is validated full-hex, so it carries no option/pathspec meaning.
	res, f := c.run(ctx, runRequest{
		op:   rebaseBeginOp,
		dir:  worktreeDir,
		args: []string{"rebase", string(baseOID)},
		env:  rebaseEnv,
	})
	if f != nil {
		return RebaseStatus{}, f
	}
	return c.classifyRebaseResult(ctx, rebaseBeginOp, worktreeDir, res, expectedHead)
}

// RebaseState reads a worktree's current rebase state without mutating anything:
// no rebase in progress reports unchanged with the current HEAD; a rebase halted
// at unmerged paths reports conflicted with them; a rebase in progress at no
// resolvable conflict reports in-progress-foreign. Ownership of an in-progress
// rebase is the caller's to decide (against its receipt); this primitive only
// reports the mechanical git state.
func (c *Client) RebaseState(ctx context.Context, worktreeDir string) (RebaseStatus, error) {
	if worktreeDir == "" {
		return RebaseStatus{}, newFailure(rebaseStateOp, KindInvalidRequest, "worktree dir is empty", nil)
	}
	inProgress, f := c.rebaseInProgress(ctx, rebaseStateOp, worktreeDir)
	if f != nil {
		return RebaseStatus{}, f
	}
	head, hf := c.worktreeHead(ctx, rebaseStateOp, worktreeDir)
	if hf != nil {
		return RebaseStatus{}, hf
	}
	if !inProgress {
		return RebaseStatus{Disposition: RebaseUnchanged, HeadOID: head}, nil
	}
	paths, pf := c.unmergedPaths(ctx, rebaseStateOp, worktreeDir)
	if pf != nil {
		return RebaseStatus{}, pf
	}
	if len(paths) > 0 {
		return RebaseStatus{Disposition: RebaseConflicted, HeadOID: head, UnmergedPaths: paths}, nil
	}
	return RebaseStatus{Disposition: RebaseInProgressForeign, HeadOID: head}, nil
}

// StageAndContinueRebase stages EXACTLY the given repo-relative paths (the caller
// has already validated them against the live unmerged set) and continues the
// in-progress rebase non-interactively. The result is classified structurally:
// the next conflict (conflicted with its UnmergedPaths), a completed rewrite
// (rebased), or failed. It never runs `git add -A` and never resolves a path the
// caller did not name.
func (c *Client) StageAndContinueRebase(ctx context.Context, worktreeDir string, paths []string) (RebaseStatus, error) {
	if worktreeDir == "" {
		return RebaseStatus{}, newFailure(rebaseContinueOp, KindInvalidRequest, "worktree dir is empty", nil)
	}
	if len(paths) == 0 {
		return RebaseStatus{}, newFailure(rebaseContinueOp, KindInvalidRequest, "no paths to stage", nil)
	}
	for _, p := range paths {
		if err := validateRepoPath(RepoPath(p), false); err != nil {
			return RebaseStatus{}, newFailure(rebaseContinueOp, KindInvalidRequest, "invalid repo-relative path", err)
		}
	}
	// Stage only the named paths. GIT_LITERAL_PATHSPECS (set by the client) makes
	// each path a literal, so a name with pathspec-magic punctuation cannot escape
	// its scope.
	addArgs := append([]string{"add", "--"}, paths...)
	res, f := c.run(ctx, runRequest{op: rebaseContinueOp, dir: worktreeDir, args: addArgs})
	if f != nil {
		return RebaseStatus{}, f
	}
	if res.exitCode != 0 {
		return RebaseStatus{}, newFailure(rebaseContinueOp, KindCommandFailed, "git add failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	cres, cf := c.run(ctx, runRequest{
		op:   rebaseContinueOp,
		dir:  worktreeDir,
		args: []string{"rebase", "--continue"},
		env:  rebaseEnv,
	})
	if cf != nil {
		return RebaseStatus{}, cf
	}
	// No expected head to compare against on a continue: a completion is always a
	// rebased outcome.
	return c.classifyRebaseResult(ctx, rebaseContinueOp, worktreeDir, cres, "")
}

// AbortRebase aborts the in-progress rebase and verifies HEAD returned to
// origHead exactly; a post-abort HEAD that does not match origHead is an error
// (the caller passed the wrong orig, or the abort did not restore what it
// claimed). origHead must be a valid object id.
func (c *Client) AbortRebase(ctx context.Context, worktreeDir string, origHead ObjectID) error {
	if worktreeDir == "" {
		return newFailure(rebaseAbortOp, KindInvalidRequest, "worktree dir is empty", nil)
	}
	if err := validateObjectID(origHead); err != nil {
		return newFailure(rebaseAbortOp, KindInvalidRequest, "invalid orig head id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   rebaseAbortOp,
		dir:  worktreeDir,
		args: []string{"rebase", "--abort"},
		env:  rebaseEnv,
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(rebaseAbortOp, KindCommandFailed, "rebase --abort failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	head, hf := c.worktreeHead(ctx, rebaseAbortOp, worktreeDir)
	if hf != nil {
		return hf
	}
	if head != origHead {
		return newFailure(rebaseAbortOp, KindInvalidRepository, "HEAD after abort does not match the expected orig commit", nil)
	}
	return nil
}

// SetOwnedRef creates or moves an owned ref beneath refs/docket/ to oid. It
// refuses any ref outside that namespace before running git, so no real branch
// or tag can be written through it.
func (c *Client) SetOwnedRef(ctx context.Context, repo Repository, ref RefName, oid ObjectID) error {
	if f := c.setOwnedRefInDir(ctx, setOwnedRefOp, repo.PrimaryWorktree, ref, oid); f != nil {
		return f
	}
	return nil
}

// DeleteOwnedRef deletes an owned ref beneath refs/docket/. It refuses any ref
// outside that namespace before running git.
func (c *Client) DeleteOwnedRef(ctx context.Context, repo Repository, ref RefName) error {
	if err := validateOwnedRef(ref); err != nil {
		return newFailure(deleteOwnedRefOp, KindInvalidRequest, "ref is not an owned refs/docket/ ref", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   deleteOwnedRefOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"update-ref", "-d", string(ref)},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(deleteOwnedRefOp, KindCommandFailed, "update-ref -d failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}

// setOwnedRefInDir is the fenced update-ref used by SetOwnedRef and by
// BeginRebase's anchor writes: it validates the ref is beneath refs/docket/ and
// the oid is well-formed before running `update-ref <ref> <oid>` in dir. dir may
// be any worktree of the repository — every worktree shares the one ref store.
func (c *Client) setOwnedRefInDir(ctx context.Context, op Operation, dir string, ref RefName, oid ObjectID) *Failure {
	if err := validateOwnedRef(ref); err != nil {
		return newFailure(op, KindInvalidRequest, "ref is not an owned refs/docket/ ref", err)
	}
	if err := validateObjectID(oid); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid object id", err)
	}
	res, f := c.run(ctx, runRequest{op: op, dir: dir, args: []string{"update-ref", string(ref), string(oid)}})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(op, KindCommandFailed, "update-ref failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}

// classifyRebaseResult maps a rebase/continue run result to a RebaseStatus. A
// zero exit is a completion — unchanged when HEAD still equals expectedHead
// (compared only when a caller supplied one), else rebased. A non-zero exit with
// a rebase still in progress and unmerged paths is conflicted; anything else is
// failed.
func (c *Client) classifyRebaseResult(ctx context.Context, op Operation, worktreeDir string, res runResult, expectedHead ObjectID) (RebaseStatus, error) {
	head, hf := c.worktreeHead(ctx, op, worktreeDir)
	if hf != nil {
		return RebaseStatus{}, hf
	}
	if res.exitCode == 0 {
		disposition := RebaseRebased
		if expectedHead != "" && head == expectedHead {
			disposition = RebaseUnchanged
		}
		return RebaseStatus{Disposition: disposition, HeadOID: head}, nil
	}
	inProgress, f := c.rebaseInProgress(ctx, op, worktreeDir)
	if f != nil {
		return RebaseStatus{}, f
	}
	if inProgress {
		paths, pf := c.unmergedPaths(ctx, op, worktreeDir)
		if pf != nil {
			return RebaseStatus{}, pf
		}
		if len(paths) > 0 {
			return RebaseStatus{Disposition: RebaseConflicted, HeadOID: head, UnmergedPaths: paths}, nil
		}
	}
	return RebaseStatus{Disposition: RebaseFailed, HeadOID: head}, nil
}

// worktreeHead resolves HEAD to its exact commit in worktreeDir (a linked
// worktree has its own HEAD, distinct from the primary's), via
// `rev-parse --verify HEAD`. A non-resolvable HEAD is invalid-repository; a
// malformed id is invalid-output.
func (c *Client) worktreeHead(ctx context.Context, op Operation, worktreeDir string) (ObjectID, *Failure) {
	res, f := c.run(ctx, runRequest{op: op, dir: worktreeDir, args: []string{"rev-parse", "--verify", "HEAD"}})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(op, KindInvalidRepository, "cannot resolve worktree HEAD: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	lines := stdoutLines(res.stdout)
	if len(lines) != 1 {
		return "", newFailure(op, KindInvalidOutput, "unexpected rev-parse HEAD output", nil)
	}
	id := ObjectID(lines[0])
	if err := validateObjectID(id); err != nil {
		return "", newFailure(op, KindInvalidOutput, "rev-parse produced a malformed HEAD object id", err)
	}
	return id, nil
}

// rebaseInProgress reports whether a rebase is underway in worktreeDir. It asks
// git for the per-worktree rebase-state directory paths
// (`rev-parse --git-path rebase-merge` / `rebase-apply`) and checks their
// existence, so it is correct for a linked worktree whose state lives outside
// the shared common dir. A path git reports relative to the worktree is resolved
// against worktreeDir before the stat.
func (c *Client) rebaseInProgress(ctx context.Context, op Operation, worktreeDir string) (bool, *Failure) {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		res, f := c.run(ctx, runRequest{op: op, dir: worktreeDir, args: []string{"rev-parse", "--git-path", name}})
		if f != nil {
			return false, f
		}
		if res.exitCode != 0 {
			return false, newFailure(op, KindCommandFailed, "rev-parse --git-path failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
		}
		lines := stdoutLines(res.stdout)
		if len(lines) != 1 {
			return false, newFailure(op, KindInvalidOutput, "unexpected rev-parse --git-path output", nil)
		}
		p := lines[0]
		if !filepath.IsAbs(p) {
			p = filepath.Join(worktreeDir, p)
		}
		if _, err := os.Stat(p); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// unmergedPaths returns the deduplicated repo-relative paths of the unmerged
// index entries in worktreeDir, read NUL-delimited (`ls-files -u -z`) so a path
// carrying spaces, tabs, or high bytes is never quoted or split.
func (c *Client) unmergedPaths(ctx context.Context, op Operation, worktreeDir string) ([]string, *Failure) {
	res, f := c.run(ctx, runRequest{op: op, dir: worktreeDir, args: []string{"ls-files", "-u", "-z"}})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(op, KindCommandFailed, "ls-files -u failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return parseUnmergedZ(res.stdout), nil
}

// parseUnmergedZ extracts the unique paths from `ls-files -u -z` output. Each
// NUL-terminated record is "<mode> <object> <stage>\t<path>"; the path is
// everything after the single TAB and appears once per conflicting stage, so
// duplicates are collapsed while preserving first-seen order.
func parseUnmergedZ(out []byte) []string {
	var paths []string
	seen := map[string]bool{}
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		tab := bytes.IndexByte(rec, '\t')
		if tab < 0 {
			continue
		}
		p := string(rec[tab+1:])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

// validateOwnedRef requires a name that passes validateRefName and also lives
// beneath refs/docket/. It keys on that prefix shape rather than any
// enumerated spelling: the owned-ref namespace is Docket's, and nothing outside
// it may be written or deleted through the owned-ref primitives.
func validateOwnedRef(ref RefName) error {
	if err := validateRefName(ref); err != nil {
		return err
	}
	if !strings.HasPrefix(string(ref), ownedRefRequiredPrefix) {
		return errors.New("gitcli: ref is not under refs/docket/")
	}
	return nil
}
