package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/install"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposeed"
	"github.com/danielhanold/docket/internal/reposetup"
)

// repository_set_visibility_shared.go — the going-shared executors of `docket
// repository set-visibility shared`. The private config is split back into a
// committed .docket.yml (written while still private, where nothing reads it)
// and the clone-local .docket.local.yml (written only after the state-folder
// rename, since a private repository refuses a .docket.local.yml beside its
// config). The bare dckt history is published to origin's docket branch under
// a create-only or fast-forward lease; the bare repository itself is kept as a
// backup. Every executor re-reads the remote tip it acts on.

// lessonsHeader introduces the promoted lessons a going-shared run reports.
const lessonsHeader = "promoted lessons from the private instructions file — move what you want into the committed AGENTS.md:"

// pendingPrivateConfig is the private config still waiting to be split: the
// config.yml in whichever state folder holds it, and the saved clone-local
// keys beside it. path is "" when no private config remains.
type pendingPrivateConfig struct {
	path      string
	data      []byte
	localKeys []byte
}

// readPendingPrivateConfig reads the private config fresh from disk: the
// private state folder first (before the rename), then the shared one (after).
func readPendingPrivateConfig(st visibilityState) (pendingPrivateConfig, error) {
	for _, dir := range []string{st.private.StateDir, st.shared.StateDir} {
		p := filepath.Join(dir, layout.PrivateConfigFile)
		data, ok, err := readOptionalFile(p)
		if err != nil {
			return pendingPrivateConfig{}, err
		}
		if !ok {
			continue
		}
		keys, _, err := readOptionalFile(filepath.Join(dir, layout.PrivateLocalKeysFile))
		if err != nil {
			return pendingPrivateConfig{}, err
		}
		return pendingPrivateConfig{path: p, data: data, localKeys: keys}, nil
	}
	return pendingPrivateConfig{}, nil
}

// splitSharedConfig splits the pending private config into its committed and
// clone-local halves and validates the committed half as .docket.yml before
// anything is written.
func splitSharedConfig(st visibilityState, pc pendingPrivateConfig) (committed, local []byte, err error) {
	committed, local, err = reposetup.SplitPrivateConfig(pc.data, pc.localKeys, config.RepoOnlyPaths())
	if err != nil {
		return nil, nil, privateRefusal(reposetup.StateNeedsReview,
			"the private configuration "+pc.path+" cannot be split: "+err.Error()+"; fix it by hand, then re-run")
	}
	if _, _, err := config.Resolve([]config.Source{{Layer: config.LayerRepository, Name: docketYMLRel, Data: committed}},
		config.ResolveContext{DefaultBranch: st.sc.defaultBranch}); err != nil {
		return nil, nil, privateRefusal(reposetup.StateNeedsReview,
			"the .docket.yml split from "+pc.path+" does not resolve: "+err.Error()+"; fix the private configuration by hand, then re-run")
	}
	return committed, local, nil
}

// identityKeysRemedy is the stop-and-push remedy of the identity-keys phase.
func identityKeysRemedy(st visibilityState) string {
	return fmt.Sprintf("get this commit onto %s on origin (push it, or merge it through a pull request), then re-run `docket repository set-visibility shared`",
		st.sc.defaultBranch)
}

// sharedIdentityKeysPhase commits .docket.yml carrying the repository-identity
// keys the private config sets, then stops the run: nothing is published until
// origin's default branch carries those values, so no workflow ever runs on
// defaults. When HEAD already carries them it refuses with the same remedy and
// makes no second commit.
func sharedIdentityKeysPhase(ctx context.Context, x *visibilityRun) error {
	git, repo, st := x.d.Git, x.st.sc.repo, x.st
	pc, err := readPendingPrivateConfig(st)
	if err != nil || pc.path == "" {
		return err
	}
	committed, _, err := splitSharedConfig(st, pc)
	if err != nil {
		return err
	}
	cs, err := git.WorktreeCheckoutState(ctx, st.primary)
	if err != nil {
		return err
	}
	if cs.Detached {
		return privateRefusal(reposetup.StateNeedsReview, "the primary checkout "+st.primary+" has a detached HEAD; check out a branch to commit on, then re-run")
	}
	want, err := reposetup.ConfigLeafValues(pc.data, repositoryIdentityKeys)
	if err != nil {
		return fmt.Errorf("reading %s: %w", pc.path, err)
	}
	head, found, err := readCommitBlob(ctx, git, repo, string(cs.Head), docketYMLRel)
	if err != nil {
		return err
	}
	if found {
		have, err := reposetup.ConfigLeafValues(head, repositoryIdentityKeys)
		if err != nil {
			return fmt.Errorf("reading .docket.yml at HEAD: %w", err)
		}
		held := true
		for k, v := range want {
			if have[k] != v {
				held = false
			}
		}
		if held {
			return privateRefusal(reposetup.StateNeedsReview,
				"the repository identity keys are committed to .docket.yml but origin's "+st.sc.defaultBranch+" does not carry them yet; "+identityKeysRemedy(st))
		}
	}
	if err := writeSwitchPath(st.common, st.primary, visibilityAddSubject, docketYMLRel, committed); err != nil {
		return err
	}
	if err := commitSharedJournal(ctx, x, cs); err != nil {
		return err
	}
	x.res.PendingLocal = append(x.res.PendingLocal, identityKeysRemedy(st))
	x.stop = true
	return nil
}

// commitSharedJournal commits the journaled add-subject paths and records the
// commit row; a failure keeps the journal and the edits for a re-run.
func commitSharedJournal(ctx context.Context, x *visibilityRun, cs gitcli.CheckoutState) error {
	branch, _ := shortBranch(cs.Branch)
	out, err := commitSwitchJournal(ctx, x.d.Git, x.st.sc.repo, x.st.common)
	row := VisibilityCommit{Subject: visibilityAddSubject, Branch: branch, Commit: out.Commit, Paths: out.Paths, Status: visibilityCommitCommitted}
	if err != nil {
		row.Status = visibilityCommitFailed
		if j, ok, jerr := loadSwitchJournal(x.st.common); jerr == nil && ok {
			row.Paths = sortedJournalPaths(j)
		}
		x.res.Commits = []VisibilityCommit{row}
		return fmt.Errorf("%w (the edits are journaled; re-run `docket repository set-visibility shared` to retry the commit)", err)
	}
	if out.Commit == "" {
		x.res.Commits = nil
	} else {
		x.res.Commits = []VisibilityCommit{row}
	}
	if !out.Complete {
		return errors.New("a commit hook changed what the switch committed; the journal is kept: review the commit, then re-run `docket repository set-visibility shared`")
	}
	return nil
}

// sortedJournalPaths lists the journal's paths, sorted.
func sortedJournalPaths(j switchJournal) []string {
	paths := make([]string, 0, len(j.Paths))
	for p := range j.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// sharedPublishPhase pushes the bare dckt history, re-read fresh, to origin's
// docket branch: create-only when absent, a fast-forward when origin kept an
// older docket branch, a refusal when origin's branch is unrelated or diverged.
func sharedPublishPhase(ctx context.Context, x *visibilityRun) error {
	git, repo := x.d.Git, x.st.sc.repo
	remote, ref := metadataRemote(x.st.private), metadataRef(x.st.private)
	url, err := git.RemoteURL(ctx, repo, remote)
	if err != nil {
		if isRemoteUnconfigured(err) {
			return privateRefusal(reposetup.StateNeedsReview,
				"the dckt remote is not configured, so no metadata history is reachable to publish; run `docket repository init` to finish the private setup, then re-run")
		}
		return fmt.Errorf("reading the dckt remote's URL: %w", err)
	}
	bare, err := probeAndFetch(ctx, git, repo, remote, ref)
	if err != nil {
		return fmt.Errorf("reading the dckt branch at %s: %w", url, err)
	}
	if bare.State != gitcli.RemoteRefFound {
		return privateRefusal(reposetup.StateNeedsReview, "the dckt remote "+url+" holds no dckt branch, so no metadata history is reachable to publish")
	}
	if _, err := publishVisibilityHistory(ctx, git, repo, originRemote, metadataRef(x.st.shared), bare.Commit); err != nil {
		var conflict *visibilityHistoryConflict
		if errors.As(err, &conflict) {
			return privateRefusal(reposetup.StateConflict, fmt.Sprintf(
				"origin already has an unrelated docket branch (%s) and this repository's history is at %s; two backlogs are never merged; reconcile or remove origin's branch by hand",
				conflict.RemoteTip, conflict.Tip))
		}
		return err
	}
	return nil
}

// sharedInstructionsPhase retires the private instructions file: its promoted
// lessons (everything outside the dispatch block) are reported for the human to
// move, then the file is removed and the ownership record forgets every
// surface under .git/. Malformed markers refuse with the file untouched.
func sharedInstructionsPhase(_ context.Context, x *visibilityRun) error {
	common := x.st.common
	insPath := layout.PrivateInstructionsPath(common)
	content, ok, err := readOptionalFile(insPath)
	if err != nil {
		return err
	}
	if ok {
		lessons, err := SelectInstructionsSection(content, InstructionsSectionLessons)
		if err != nil {
			return privateRefusal(reposetup.StateNeedsReview, layout.PrivateInstructionsDisplay+": "+err.Error()+"; fix the dispatch block markers by hand, then re-run")
		}
		if lessons != nil {
			x.res.Lessons = string(lessons)
		}
		if err := os.Remove(insPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if _, err := disownSurfaces(reposeed.RecordPath(common, layout.StateName(common)), isWorkingTreeSurface); err != nil {
		return fmt.Errorf("rewriting the ownership record: %w", err)
	}
	return nil
}

// sharedConfigCommittedPhase writes .docket.yml from the private config's
// committed half, journaled for the add commit. A private repository never
// reads .docket.yml, so writing it before the rename changes nothing yet.
func sharedConfigCommittedPhase(_ context.Context, x *visibilityRun) error {
	pc, err := readPendingPrivateConfig(x.st)
	if err != nil || pc.path == "" {
		return err
	}
	committed, _, err := splitSharedConfig(x.st, pc)
	if err != nil {
		return err
	}
	return writeSwitchPath(x.st.common, x.st.primary, visibilityAddSubject, docketYMLRel, committed)
}

// sharedStateFolderPhase renames <common>/dckt to <common>/docket. From here
// the repository detects shared and .docket.yml resolves. It never writes into
// a non-empty docket folder.
func sharedStateFolderPhase(_ context.Context, x *visibilityRun) error {
	from, to := x.st.private.StateDir, x.st.shared.StateDir
	fi, err := os.Lstat(to)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case !fi.IsDir():
		return privateRefusal(reposetup.StateConflict, to+" exists beside "+from+" and is not a folder; inspect both by hand")
	default:
		entries, err := os.ReadDir(to)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return privateRefusal(reposetup.StateConflict, to+" already exists beside "+from+" and is not empty; inspect both by hand")
		}
		if err := os.Remove(to); err != nil {
			return err
		}
	}
	return os.Rename(from, to)
}

// sharedConfigLocalPhase restores the clone-local half of the private config
// to .docket.local.yml (ignored, never committed), validated together with the
// committed half, then retires the saved local keys and, last, the private
// config itself. It runs only after the rename: a private repository refuses a
// .docket.local.yml beside its config.
func sharedConfigLocalPhase(_ context.Context, x *visibilityRun) error {
	st := x.st
	cfgPath := filepath.Join(st.shared.StateDir, layout.PrivateConfigFile)
	keysPath := filepath.Join(st.shared.StateDir, layout.PrivateLocalKeysFile)
	data, ok, err := readOptionalFile(cfgPath)
	if err != nil {
		return err
	}
	if ok {
		keys, _, err := readOptionalFile(keysPath)
		if err != nil {
			return err
		}
		pc := pendingPrivateConfig{path: cfgPath, data: data, localKeys: keys}
		committed, local, err := splitSharedConfig(st, pc)
		if err != nil {
			return err
		}
		if local != nil {
			if _, _, err := config.Resolve([]config.Source{
				{Layer: config.LayerRepository, Name: docketYMLRel, Data: committed},
				{Layer: config.LayerRepositoryLocal, Name: localConfigRel, Data: local},
			}, config.ResolveContext{DefaultBranch: st.sc.defaultBranch}); err != nil {
				return privateRefusal(reposetup.StateNeedsReview,
					"the clone-local keys saved in "+keysPath+" do not resolve with .docket.yml: "+err.Error()+"; fix "+cfgPath+" by hand, then re-run")
			}
			localPath := filepath.Join(st.primary, localConfigRel)
			existing, present, err := readOptionalFile(localPath)
			if err != nil {
				return err
			}
			if present && string(existing) != string(local) {
				return privateRefusal(reposetup.StateConflict,
					localConfigRel+" already exists and differs from the clone-local keys saved in "+keysPath+"; merge them into "+localConfigRel+" by hand, delete "+keysPath+" and "+cfgPath+", then re-run")
			}
			if !present {
				if err := writeFileReplacing(localPath, local, 0o644); err != nil {
					return err
				}
			}
		}
	}
	if err := os.Remove(keysPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Remove(cfgPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// sharedIgnorePhase removes the `# dckt:` block from .git/info/exclude and
// writes the managed block into .gitignore, journaled for the add commit.
// Malformed markers refuse with the file untouched.
func sharedIgnorePhase(_ context.Context, x *visibilityRun) error {
	st := x.st
	excludePath := filepath.Join(st.common, "info", "exclude")
	exclude, ok, err := readOptionalFile(excludePath)
	if err != nil {
		return err
	}
	if ok {
		out, changed, err := reposetup.RemoveExcludeBlock(exclude)
		if err != nil {
			return privateRefusal(reposetup.StateNeedsReview, err.Error()+"; fix the markers in .git/info/exclude by hand, then re-run")
		}
		if changed {
			if err := writeFileReplacing(excludePath, out, 0o644); err != nil {
				return err
			}
		}
	}
	ignore, _, err := readOptionalFile(filepath.Join(st.primary, gitignoreRel))
	if err != nil {
		return err
	}
	out, changed, err := reposetup.EnsureGitignoreBlock(ignore)
	if err != nil {
		return privateRefusal(reposetup.StateNeedsReview, gitignoreRel+": "+err.Error()+"; fix the markers by hand, then re-run")
	}
	if !changed {
		return nil
	}
	return writeSwitchPath(st.common, st.primary, visibilityAddSubject, gitignoreRel, out)
}

// sharedMetadataWorktreePhase removes the private checkout cleanly, attaches
// .docket on the docket branch at origin's tip with hooks off, deletes the
// local dckt branch when origin's docket branch holds it, and removes the dckt
// remote. The bare repository it named is kept as a backup.
func sharedMetadataWorktreePhase(ctx context.Context, x *visibilityRun) error {
	git, repo, st := x.d.Git, x.st.sc.repo, x.st
	wts, err := git.ListWorktrees(ctx, repo)
	if err != nil {
		return err
	}
	if wt, ok := worktreeAt(wts, st.private.MetadataWorktree); ok {
		remove := git.RemoveWorktreeClean
		if wt.Prunable {
			remove = git.RemoveWorktree // its folder is already gone
		}
		if err := remove(ctx, repo, st.private.MetadataWorktree); err != nil {
			return fmt.Errorf("removing the metadata checkout %s: %w", st.private.MetadataWorktree, err)
		}
	}
	origin, err := probeAndFetch(ctx, git, repo, originRemote, metadataRef(st.shared))
	if err != nil {
		return fmt.Errorf("reading origin's docket branch: %w", err)
	}
	if origin.State != gitcli.RemoteRefFound {
		return errors.New("origin holds no docket branch; re-run to publish it")
	}
	if _, err := ensureMetadataWorktree(ctx, git, repo, st.shared.MetadataWorktree, metadataRef(st.shared), origin.Commit); err != nil {
		return fmt.Errorf("attaching the metadata checkout %s: %w", st.shared.MetadataWorktree, err)
	}
	if err := git.DisableWorktreeHooks(ctx, st.shared.MetadataWorktree); err != nil {
		return fmt.Errorf("disabling metadata-checkout hooks: %w", err)
	}

	privRef := metadataRef(st.private)
	tip, err := localBranchTip(ctx, git, repo, privRef)
	if err != nil {
		return err
	}
	if tip != "" {
		held := gitcli.ObjectID(tip) == origin.Commit
		if !held {
			if held, err = git.IsAncestor(ctx, repo, gitcli.ObjectID(tip), origin.Commit); err != nil {
				return err
			}
		}
		if held {
			if err := git.DeleteLocalBranchChecked(ctx, repo, privRef, gitcli.ObjectID(tip)); err != nil {
				return err
			}
		} else {
			x.markPhase("metadata-worktree", visibilityPhaseKept, "")
			x.res.PendingLocal = append(x.res.PendingLocal,
				"the local dckt branch has commits origin's docket branch lacks; inspect it, then delete it by hand")
		}
	}

	remote := metadataRemote(st.private)
	url, err := git.RemoteURL(ctx, repo, remote)
	switch {
	case err == nil:
		if err := git.RemoveRemote(ctx, repo, remote); err != nil {
			return fmt.Errorf("removing the dckt git remote: %w", err)
		}
		x.res.BackupRemote = url
	case isRemoteUnconfigured(err):
	default:
		return fmt.Errorf("reading the dckt git remote: %w", err)
	}
	return nil
}

// sharedCommitPhase installs the repository-level dispatch surfaces
// agent_harnesses authorizes (exactly as install's repository phase would),
// journals them, and commits every journaled path with the add subject. The
// commit stays local.
func sharedCommitPhase(ctx context.Context, x *visibilityRun) error {
	git, st := x.d.Git, x.st
	cs, err := git.WorktreeCheckoutState(ctx, st.primary)
	if err != nil {
		return err
	}
	if cs.Detached {
		return privateRefusal(reposetup.StateNeedsReview, "the primary checkout "+st.primary+" has a detached HEAD; check out a branch to commit on, then re-run")
	}
	surfaces, _, err := installAuthorizedSurfaces(ctx, git, st.primary)
	if err != nil {
		return fmt.Errorf("installing the dispatch instructions: %w", err)
	}
	for _, rel := range surfaces {
		if err := journalSwitchPath(st.common, st.primary, visibilityAddSubject, rel); err != nil {
			return err
		}
	}
	return commitSharedJournal(ctx, x, cs)
}

// sharedAlignVisibilityPhase sets an explicit, different visibility in
// .docket.local.yml to shared.
func sharedAlignVisibilityPhase(_ context.Context, x *visibilityRun) error {
	p := filepath.Join(x.st.primary, localConfigRel)
	existing, ok, err := readOptionalFile(p)
	if err != nil || !ok {
		return err
	}
	edited, changed, err := reposetup.RenderVisibilityEdit(existing, string(layout.Shared))
	if err != nil {
		return privateRefusal(reposetup.StateNeedsReview, localConfigRel+": "+err.Error())
	}
	if !changed {
		return nil
	}
	return writeFileReplacing(p, edited, 0o644)
}

// repoSurfacesSettled reports whether every repository dispatch surface
// agent_harnesses authorizes is already as install's repository phase would
// leave it (read-only). No authorization is settled.
func repoSurfacesSettled(ctx context.Context, git *gitcli.Client, primary string) (bool, error) {
	runTracker, err := buildRunTracker()
	if err != nil {
		return false, err
	}
	phase, _, _, err := ResolveRepoPhase(ctx, git, primary, nil, runTracker, nil, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		return false, err
	}
	if phase == nil || !phase.Authorized {
		return true, nil
	}
	for _, t := range phase.Targets {
		insp, err := install.InspectTarget(t, phase.PriorState, nil)
		if err != nil {
			return false, err
		}
		if insp.Disposition != install.DispositionNoop {
			return false, nil
		}
	}
	return true, nil
}

// visibilityBackupURL is the bare repository going shared keeps: the
// configured dckt remote, else the default store.
func visibilityBackupURL(st visibilityState) string {
	if st.dcktURL != "" {
		return st.dcktURL
	}
	return st.private.DefaultBareRemote
}

// lessonsText renders reported lessons under their header.
func lessonsText(lessons string) string {
	return lessonsHeader + "\n" + strings.TrimRight(lessons, "\n")
}
