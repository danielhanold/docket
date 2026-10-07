package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposeed"
	"github.com/danielhanold/docket/internal/reposetup"
)

// repository_set_visibility_private.go — the going-private executors of
// `docket repository set-visibility private`, and the helpers they share with
// going shared: publishing the identical metadata history under the other
// branch name, stripping the dispatch block from a committed instructions
// file, and disowning working-tree surfaces in the ownership record. Every
// executor re-reads the remote tip it acts on; none reuses a gather-time value.

// localConfigRel is the clone-local configuration file of a shared repository.
const localConfigRel = ".docket.local.yml"

// cursorRuleRel is the committed Cursor dispatch rule of a shared repository.
const cursorRuleRel = ".cursor/rules/docket-dispatch.mdc"

// visibilityHistoryConflict is publishVisibilityHistory's refusal: the remote
// ref holds history that is unrelated to, or diverged from, the tip being
// published. Two backlogs are never merged, so a human resolves it.
type visibilityHistoryConflict struct {
	Remote    gitcli.RemoteName
	Ref       gitcli.RefName
	RemoteTip gitcli.ObjectID
	Tip       gitcli.ObjectID
}

func (e *visibilityHistoryConflict) Error() string {
	return fmt.Sprintf("%s on %s is at %s, which is unrelated to or diverged from %s; two backlogs are never merged",
		e.Ref, e.Remote, e.RemoteTip, e.Tip)
}

// publishVisibilityHistory makes remote's ref hold tip without rewriting history:
// create-only when absent; a fast-forward lease (expected = current remote tip) when
// that tip is an ancestor of tip; nothing when it equals or descends from tip.
// Unrelated or diverged history refuses naming both tips. Re-read afterwards: the
// remote must hold tip or a descendant.
func publishVisibilityHistory(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName, tip gitcli.ObjectID) (changed bool, err error) {
	cur, err := git.ProbeRemoteBranch(ctx, repo, remote, ref)
	if err != nil {
		return false, fmt.Errorf("reading %s on %s: %w", ref, remote, err)
	}
	switch {
	case cur.State != gitcli.RemoteRefFound:
		out, err := git.PushCreateLease(ctx, repo, remote, ref, tip)
		if err != nil {
			return false, fmt.Errorf("publishing %s to %s: %w", ref, remote, err)
		}
		switch out.Disposition {
		case gitcli.PushApplied:
			changed = true
		case gitcli.PushLeaseLost:
			return false, fmt.Errorf("%s on %s was created at %s while the switch published it; re-run to preview the new state", ref, remote, out.Remote)
		default:
			return false, fmt.Errorf("publishing %s to %s failed; re-run to retry", ref, remote)
		}
	case cur.Commit == tip:
		// Already published.
	default:
		// Bring the remote tip's objects in so ancestry can be read.
		rev, err := git.FetchBranch(ctx, repo, remote, ref)
		if err != nil {
			return false, fmt.Errorf("fetching %s from %s: %w", ref, remote, err)
		}
		remoteTip := rev.Commit
		if remoteTip == tip {
			break
		}
		descends, err := git.IsAncestorIgnoringReplacements(ctx, repo, tip, remoteTip)
		if err != nil {
			return false, err
		}
		if descends {
			break // the remote already holds tip and more
		}
		behind, err := git.IsAncestorIgnoringReplacements(ctx, repo, remoteTip, tip)
		if err != nil {
			return false, err
		}
		if !behind {
			return false, &visibilityHistoryConflict{Remote: remote, Ref: ref, RemoteTip: remoteTip, Tip: tip}
		}
		out, err := git.PushLease(ctx, repo, remote, ref, tip, remoteTip)
		if err != nil {
			return false, fmt.Errorf("fast-forwarding %s on %s: %w", ref, remote, err)
		}
		switch out.Disposition {
		case gitcli.PushApplied:
			changed = true
		case gitcli.PushLeaseLost:
			return false, fmt.Errorf("%s on %s moved to %s while the switch fast-forwarded it; re-run to preview the new state", ref, remote, out.Remote)
		default:
			return false, fmt.Errorf("fast-forwarding %s on %s failed; re-run to retry", ref, remote)
		}
	}

	// Re-read: the remote must hold tip or a descendant of it.
	after, err := git.ProbeRemoteBranch(ctx, repo, remote, ref)
	if err != nil {
		return changed, fmt.Errorf("re-reading %s on %s: %w", ref, remote, err)
	}
	if after.State != gitcli.RemoteRefFound {
		return changed, fmt.Errorf("%s is absent from %s after the publish", ref, remote)
	}
	if after.Commit == tip {
		return changed, nil
	}
	rev, err := git.FetchBranch(ctx, repo, remote, ref)
	if err != nil {
		return changed, fmt.Errorf("fetching %s from %s: %w", ref, remote, err)
	}
	held, err := git.IsAncestorIgnoringReplacements(ctx, repo, tip, rev.Commit)
	if err != nil {
		return changed, err
	}
	if !held {
		return changed, fmt.Errorf("%s on %s is at %s after the publish, which does not hold %s", ref, remote, rev.Commit, tip)
	}
	return changed, nil
}

// stripDispatchBlock removes the `dispatch` managed block; remove = only whitespace
// is left. Malformed markers error (manual review); no block -> (src, false, nil).
func stripDispatchBlock(src []byte) (out []byte, remove bool, err error) {
	doc, err := document.Parse(src)
	if err != nil {
		return nil, false, err
	}
	if _, ok := doc.Block(instructionsBlockName); !ok {
		return src, false, nil
	}
	var patch document.PatchSet
	patch.RemoveBlock(instructionsBlockName)
	out, err = doc.Apply(patch)
	if err != nil {
		return nil, false, err
	}
	return out, len(bytes.TrimSpace(out)) == 0, nil
}

// disownWorkingTreeSurfaces rewrites the ownership record without surfaces outside
// .git/, so a private-mode install never retires them. Absent record -> (false, nil).
// A record left with no surface is removed.
func disownWorkingTreeSurfaces(recordPath string) (bool, error) {
	rec, err := reposeed.LoadRecord(recordPath)
	if err != nil || rec == nil {
		return false, err
	}
	kept := make([]reposeed.SurfaceRecord, 0, len(rec.Surfaces))
	for _, s := range rec.Surfaces {
		if strings.HasPrefix(s.Path, ".git/") {
			kept = append(kept, s)
		}
	}
	if len(kept) == len(rec.Surfaces) {
		return false, nil
	}
	if len(kept) == 0 {
		if err := os.Remove(recordPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
		return true, nil
	}
	rec.Surfaces = kept
	b, err := reposeed.EncodeRecord(rec)
	if err != nil {
		return false, err
	}
	return true, writeFileReplacing(recordPath, b, 0o644)
}

// --- the default store, read before the dckt remote exists ------------------

// storeRepository addresses the default store's bare repository directly, so
// a clone that has not configured its dckt remote yet can read what another
// clone on this machine already published there.
func storeRepository(priv layout.Layout) gitcli.Repository {
	return gitcli.Repository{PrimaryWorktree: priv.DefaultBareRemote, CommonDir: priv.DefaultBareRemote}
}

// defaultStoreTip reads the dckt branch of the default store from disk: absent
// when the store or its branch does not exist. Any other probe error is
// returned.
func defaultStoreTip(ctx context.Context, git *gitcli.Client, priv layout.Layout) (gitcli.RemoteRef, error) {
	absent := gitcli.RemoteRef{State: gitcli.RemoteRefAbsent}
	if priv.DefaultBareRemote == "" {
		return absent, nil
	}
	ok, err := isDirAt(priv.DefaultBareRemote)
	if err != nil || !ok {
		return absent, err
	}
	tip, err := localBranchTip(ctx, git, storeRepository(priv), metadataRef(priv))
	if err != nil || tip == "" {
		return absent, err
	}
	return gitcli.RemoteRef{State: gitcli.RemoteRefFound, Commit: gitcli.ObjectID(tip)}, nil
}

// storeHoldsCommit reports whether the store's tip equals or descends from
// commit, read inside the store: a commit the store lacks is not held.
func storeHoldsCommit(ctx context.Context, git *gitcli.Client, priv layout.Layout, tip, commit gitcli.ObjectID) (bool, error) {
	if commit == tip {
		return true, nil
	}
	repo := storeRepository(priv)
	if _, err := git.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: commit}); err != nil {
		if f, ok := gitcli.AsFailure(err); ok && f.Kind == gitcli.KindRefUnavailable {
			return false, nil
		}
		return false, err
	}
	return git.IsAncestor(ctx, repo, commit, tip)
}

// --- the going-private executors -----------------------------------------------

// privateRefusal is an executor refusal a human resolves.
func privateRefusal(state reposetup.State, msg string) error {
	return &visibilityRefusal{state: state, msg: msg}
}

// privateMetadataRemotePhase sets up the bare remote and the dckt git remote.
func privateMetadataRemotePhase(ctx context.Context, x *visibilityRun) error {
	_, _, err := ensurePrivateMetadataRemote(ctx, x.d.Git, x.st.sc.repo, x.st.private, x.o.MetadataRemote, x.st.dcktURL != "")
	if err == nil {
		return nil
	}
	var mre *metadataRemoteError
	if errors.As(err, &mre) && mre.Conflict {
		msg := strings.ReplaceAll(mre.Err.Error(), "`docket repository init --metadata-remote <url>`",
			"`docket repository set-visibility private --metadata-remote <url>`")
		return privateRefusal(reposetup.StateConflict, msg)
	}
	return err
}

// privatePublishPhase pushes origin's docket history, re-read fresh, to the
// dckt branch of the bare remote.
func privatePublishPhase(ctx context.Context, x *visibilityRun) error {
	git, repo := x.d.Git, x.st.sc.repo
	remote, ref := metadataRemote(x.st.private), metadataRef(x.st.private)
	url, err := git.RemoteURL(ctx, repo, remote)
	if err != nil {
		return fmt.Errorf("reading the dckt remote's URL: %w", err)
	}
	origin, err := probeAndFetch(ctx, git, repo, originRemote, metadataRef(x.st.shared))
	if err != nil {
		return fmt.Errorf("reading origin's docket branch: %w", err)
	}
	if origin.State != gitcli.RemoteRefFound {
		bare, err := git.ProbeRemoteBranch(ctx, repo, remote, ref)
		if err != nil {
			return fmt.Errorf("reading the dckt branch at %s: %w", url, err)
		}
		if bare.State == gitcli.RemoteRefFound {
			return nil // the bare remote already holds the only history
		}
		return privateRefusal(reposetup.StateNeedsReview, fmt.Sprintf(
			"no metadata history is reachable: origin has no docket branch and %s holds no dckt branch; re-run with --metadata-remote <url of the bare repository that holds it>", url))
	}
	if _, err := publishVisibilityHistory(ctx, git, repo, remote, ref, origin.Commit); err != nil {
		var conflict *visibilityHistoryConflict
		if errors.As(err, &conflict) {
			return privateRefusal(reposetup.StateConflict, fmt.Sprintf(
				"the dckt branch at %s is at %s, which is unrelated to or diverged from origin's docket branch at %s; two backlogs are never merged; point --metadata-remote at a bare repository of this repository's own, or reconcile that dckt branch by hand",
				url, conflict.RemoteTip, conflict.Tip))
		}
		return err
	}
	return nil
}

// privateConfigPhase folds the committed and clone-local configuration into the
// private config, written into the state folder before it is renamed (so the
// rename is the one atomic flip), then removes .docket.local.yml. A config
// already waiting there is kept; only the local file goes.
func privateConfigPhase(ctx context.Context, x *visibilityRun) error {
	st := x.st
	localPath := filepath.Join(st.primary, localConfigRel)
	if st.pendingConfig == "" {
		committed, err := privateFoldCommitted(ctx, x)
		if err != nil {
			return err
		}
		local, _, err := readOptionalFile(localPath)
		if err != nil {
			return err
		}
		folded, dropped, err := reposetup.FoldPrivateConfig(committed, local, config.RepoOnlyPaths())
		if err != nil {
			return privateRefusal(reposetup.StateNeedsReview, "the configuration cannot be folded into "+layout.PrivateConfigDisplay+": "+err.Error()+"; fix .docket.yml or .docket.local.yml, then re-run")
		}
		if _, _, err := config.Resolve([]config.Source{{Layer: config.LayerRepository, Name: layout.PrivateConfigDisplay, Data: folded}},
			config.ResolveContext{DefaultBranch: st.sc.defaultBranch}); err != nil {
			return privateRefusal(reposetup.StateNeedsReview, "the folded "+layout.PrivateConfigDisplay+" does not resolve: "+err.Error()+"; fix .docket.yml or .docket.local.yml, then re-run")
		}
		dir := st.shared.StateDir
		if !st.sharedStateDir && st.privateStateDir {
			dir = st.private.StateDir
		}
		if local != nil {
			if err := writeFileReplacing(filepath.Join(dir, layout.PrivateLocalKeysFile), local, 0o644); err != nil {
				return err
			}
		}
		if err := writeFileReplacing(filepath.Join(dir, layout.PrivateConfigFile), folded, 0o644); err != nil {
			return err
		}
		if len(dropped) > 0 {
			x.res.Warnings = append(x.res.Warnings, "not carried into the private config: "+strings.Join(dropped, ", "))
		}
	}
	if err := os.Remove(localPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// privateFoldCommitted reads the committed half of the fold: .docket.yml at
// origin's default-branch tip, else at the primary checkout's HEAD, else none.
func privateFoldCommitted(ctx context.Context, x *visibilityRun) ([]byte, error) {
	for _, rev := range []string{x.st.defaultTip, x.st.primaryHead} {
		if rev == "" {
			continue
		}
		b, found, err := readCommitBlob(ctx, x.d.Git, x.st.sc.repo, rev, docketYMLRel)
		if err != nil {
			return nil, err
		}
		if found {
			return b, nil
		}
	}
	return nil, nil
}

// privateStateFolderPhase renames <common>/docket to <common>/dckt. From here
// the repository detects private. It never writes into an existing dckt.
func privateStateFolderPhase(_ context.Context, x *visibilityRun) error {
	from, to := x.st.shared.StateDir, x.st.private.StateDir
	if _, err := os.Lstat(to); err == nil {
		return privateRefusal(reposetup.StateConflict, to+" already exists beside "+from+"; inspect both by hand")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(from, to)
}

// privateIgnorePhase writes the neutral managed block into .git/info/exclude.
func privateIgnorePhase(_ context.Context, x *visibilityRun) error {
	if _, err := ensureExcludeFile(filepath.Join(x.st.common, "info", "exclude")); err != nil {
		var mal *reposetup.MalformedExcludeError
		if errors.As(err, &mal) {
			return privateRefusal(reposetup.StateNeedsReview, err.Error()+"; fix the markers by hand, then re-run")
		}
		return err
	}
	return nil
}

// privateMetadataWorktreePhase removes the .docket checkout cleanly, attaches
// the private checkout on the dckt branch with hooks off, and deletes the local
// docket branch when the dckt branch holds it.
func privateMetadataWorktreePhase(ctx context.Context, x *visibilityRun) error {
	git, repo, st := x.d.Git, x.st.sc.repo, x.st
	wts, err := git.ListWorktrees(ctx, repo)
	if err != nil {
		return err
	}
	if wt, ok := worktreeAt(wts, st.shared.MetadataWorktree); ok {
		remove := git.RemoveWorktreeClean
		if wt.Prunable {
			remove = git.RemoveWorktree // its folder is already gone
		}
		if err := remove(ctx, repo, st.shared.MetadataWorktree); err != nil {
			return fmt.Errorf("removing the metadata checkout %s: %w", st.shared.MetadataWorktree, err)
		}
	}
	if err := os.MkdirAll(st.private.CheckoutsDir, 0o755); err != nil {
		return err
	}
	if err := removeStaleOwnCheckout(st.private); err != nil {
		return err
	}
	bare, err := probeAndFetch(ctx, git, repo, metadataRemote(st.private), metadataRef(st.private))
	if err != nil {
		return fmt.Errorf("reading the dckt branch: %w", err)
	}
	if bare.State != gitcli.RemoteRefFound {
		return errors.New("the dckt remote holds no dckt branch; re-run to publish it")
	}
	if _, err := ensureMetadataWorktree(ctx, git, repo, st.private.MetadataWorktree, metadataRef(st.private), bare.Commit); err != nil {
		return fmt.Errorf("attaching the metadata checkout %s: %w", st.private.MetadataWorktree, err)
	}
	if err := git.DisableWorktreeHooks(ctx, st.private.MetadataWorktree); err != nil {
		return fmt.Errorf("disabling metadata-checkout hooks: %w", err)
	}

	shared := metadataRef(st.shared)
	tip, err := localBranchTip(ctx, git, repo, shared)
	if err != nil || tip == "" {
		return err
	}
	held := gitcli.ObjectID(tip) == bare.Commit
	if !held {
		if held, err = git.IsAncestor(ctx, repo, gitcli.ObjectID(tip), bare.Commit); err != nil {
			return err
		}
	}
	if !held {
		x.markPhase("metadata-worktree", visibilityPhaseKept, "")
		x.res.PendingLocal = append(x.res.PendingLocal,
			"the local docket branch has commits the dckt branch lacks; inspect it, then delete it by hand")
		return nil
	}
	return git.DeleteLocalBranchChecked(ctx, repo, shared, gitcli.ObjectID(tip))
}

// privateInstructionsPhase disowns the committed working-tree surfaces (a
// private install would otherwise retire them), then installs the dispatch
// block into the private instructions file when agent_harnesses authorizes it.
func privateInstructionsPhase(ctx context.Context, x *visibilityRun) error {
	if _, err := disownWorkingTreeSurfaces(reposeed.RecordPath(x.st.common, layout.PrivateName)); err != nil {
		return fmt.Errorf("rewriting the ownership record: %w", err)
	}
	if _, _, err := installAuthorizedSurfaces(ctx, x.d.Git, x.st.primary); err != nil {
		return fmt.Errorf("installing the private dispatch instructions: %w", err)
	}
	return nil
}

// privateDeleteSharedBranchPhase deletes origin's docket branch under an exact
// lease, only when it is at the very tip the dckt branch holds; any mismatch
// keeps it and reports both tips.
func privateDeleteSharedBranchPhase(ctx context.Context, x *visibilityRun) error {
	git, repo := x.d.Git, x.st.sc.repo
	origin, err := probeAndFetch(ctx, git, repo, originRemote, metadataRef(x.st.shared))
	if err != nil {
		return fmt.Errorf("reading origin's docket branch: %w", err)
	}
	if origin.State != gitcli.RemoteRefFound {
		return nil
	}
	bare, err := git.ProbeRemoteBranch(ctx, repo, metadataRemote(x.st.private), metadataRef(x.st.private))
	if err != nil {
		return fmt.Errorf("reading the dckt branch: %w", err)
	}
	keep := func(originTip gitcli.ObjectID) error {
		bareTip := "absent"
		if bare.State == gitcli.RemoteRefFound {
			bareTip = string(bare.Commit)
		}
		x.markPhase("delete-shared-branch", visibilityPhaseKept, "")
		x.res.PendingLocal = append(x.res.PendingLocal, fmt.Sprintf(
			"origin's docket branch is at %s, the dckt branch at %s; nothing was deleted", originTip, bareTip))
		return nil
	}
	if bare.State != gitcli.RemoteRefFound || bare.Commit != origin.Commit {
		return keep(origin.Commit)
	}
	out, err := git.DeleteRemoteRefLease(ctx, repo, originRemote, metadataRef(x.st.shared), origin.Commit)
	if err != nil {
		return fmt.Errorf("deleting origin's docket branch: %w", err)
	}
	switch out.Disposition {
	case gitcli.PushApplied:
		return nil
	case gitcli.PushLeaseLost:
		return keep(out.Remote)
	default:
		return errors.New("deleting origin's docket branch failed; re-run to retry")
	}
}

// privateRemoveSharedFilesPhase removes .docket.yml, the managed .gitignore
// block, the dispatch blocks, and the Cursor rule — each target computed from
// the path's HEAD blob — through the journal, then commits exactly those paths
// with the fixed subject. The commit stays local.
func privateRemoveSharedFilesPhase(ctx context.Context, x *visibilityRun) error {
	git, repo, st := x.d.Git, x.st.sc.repo, x.st
	cs, err := git.WorktreeCheckoutState(ctx, st.primary)
	if err != nil {
		return err
	}
	if cs.Detached {
		return privateRefusal(reposetup.StateNeedsReview, "the primary checkout "+st.primary+" has a detached HEAD; check out a branch to commit on, then re-run")
	}
	branch, _ := shortBranch(cs.Branch)
	targets, err := removalTargets(ctx, git, repo, cs.Head)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if err := writeSwitchPath(st.common, st.primary, visibilityRemoveSubject, t.rel, t.content); err != nil {
			return err
		}
	}
	row := VisibilityCommit{Subject: visibilityRemoveSubject, Branch: branch, Status: visibilityCommitCommitted}
	out, err := commitSwitchJournal(ctx, git, repo, st.common)
	row.Commit, row.Paths = out.Commit, out.Paths
	if err != nil {
		row.Status = visibilityCommitFailed
		for _, t := range targets {
			row.Paths = append(row.Paths, t.rel)
		}
		x.res.Commits = []VisibilityCommit{row}
		return fmt.Errorf("%w (the edits are journaled; re-run `docket repository set-visibility private --remove-shared-files` to retry the commit)", err)
	}
	x.res.Commits = []VisibilityCommit{row}
	if !out.Complete {
		return errors.New("a commit hook changed what the switch committed; the journal is kept: review the commit, then re-run `docket repository set-visibility private --remove-shared-files`")
	}
	return nil
}

// removalTarget is one path the removal commit writes (content) or deletes (nil).
type removalTarget struct {
	rel     string
	content []byte
}

// removalTargets computes, from the HEAD blobs, what the removal commit does to
// each committed shared path. A CLAUDE.md symlink goes only with a deleted
// AGENTS.md; malformed markers refuse for manual review.
func removalTargets(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, head gitcli.ObjectID) ([]removalTarget, error) {
	src, err := git.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: head})
	if err != nil {
		return nil, err
	}
	paths := make([]gitcli.RepoPath, 0, len(visibilityCommitPaths))
	for _, p := range visibilityCommitPaths {
		paths = append(paths, gitcli.RepoPath(p))
	}
	blobs, err := src.ReadBlobs(ctx, paths)
	if err != nil {
		return nil, err
	}
	at := map[string]gitcli.Blob{}
	for _, b := range blobs {
		if b.Found {
			at[string(b.Path)] = b.Blob
		}
	}
	isLink := func(b gitcli.Blob) bool { return b.Mode == "120000" }
	var out []removalTarget
	if _, ok := at[docketYMLRel]; ok {
		out = append(out, removalTarget{rel: docketYMLRel})
	}
	if b, ok := at[gitignoreRel]; ok && !isLink(b) {
		stripped, changed, err := reposetup.RemoveGitignoreBlock(b.Bytes)
		if err != nil {
			return nil, privateRefusal(reposetup.StateNeedsReview, gitignoreRel+": "+err.Error()+"; fix the markers by hand, then re-run")
		}
		if changed {
			if len(bytes.TrimSpace(stripped)) == 0 {
				stripped = nil
			}
			out = append(out, removalTarget{rel: gitignoreRel, content: stripped})
		}
	}
	agentsDeleted := false
	for _, rel := range []string{"AGENTS.md", "CLAUDE.md"} {
		b, ok := at[rel]
		if !ok {
			continue
		}
		if isLink(b) {
			if rel == "CLAUDE.md" && agentsDeleted {
				out = append(out, removalTarget{rel: rel})
			}
			continue
		}
		stripped, remove, err := stripDispatchBlock(b.Bytes)
		if err != nil {
			return nil, privateRefusal(reposetup.StateNeedsReview, rel+": "+err.Error()+"; fix the dispatch block markers by hand, then re-run")
		}
		switch {
		case remove:
			out = append(out, removalTarget{rel: rel})
			agentsDeleted = agentsDeleted || rel == "AGENTS.md"
		case !bytes.Equal(stripped, b.Bytes):
			out = append(out, removalTarget{rel: rel, content: stripped})
		}
	}
	if _, ok := at[cursorRuleRel]; ok {
		out = append(out, removalTarget{rel: cursorRuleRel})
	}
	return out, nil
}

// privateAlignVisibilityPhase sets an explicit, different visibility in the
// private config to private.
func privateAlignVisibilityPhase(_ context.Context, x *visibilityRun) error {
	p := layout.PrivateConfigPath(x.st.common)
	existing, ok, err := readOptionalFile(p)
	if err != nil || !ok {
		return err
	}
	edited, changed, err := reposetup.RenderVisibilityEdit(existing, string(layout.Private))
	if err != nil {
		return privateRefusal(reposetup.StateNeedsReview, layout.PrivateConfigDisplay+": "+err.Error())
	}
	if !changed {
		return nil
	}
	return writeFileReplacing(p, edited, 0o644)
}
