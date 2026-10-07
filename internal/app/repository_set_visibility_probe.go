package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposeed"
	"github.com/danielhanold/docket/internal/reposetup"
)

// repository_set_visibility_probe.go — the state a visibility switch decides
// over, and its read-only preconditions and warnings. Nothing here writes,
// except the fetches that bring each remote's metadata tip into the object
// store so ancestry and blobs can be read.

// repositoryIdentityKeys are the repository-only keys whose values must agree
// with origin's default-branch .docket.yml before going shared publishes
// anything: they decide where every record lives.
var repositoryIdentityKeys = []string{"integration_branch", "changes_dir", "adrs_dir", "results_dir"}

// visibilityLinkListCap bounds how many paths the absolute-link warning names.
const visibilityLinkListCap = 20

// visibilityState is read without trusting the detected mode: both layouts
// (private via privateLayoutOf), layout.Detect, whether <common>/docket and
// <common>/dckt are dirs, origin's docket ref, the dckt remote URL ("" =
// unconfigured) and its dckt ref, origin's default tip, the worktree list, local
// docket/dckt tips ("" = absent), whether .docket.local.yml exists, a config.yml
// waiting in either state folder, and the journal — plus the derived facts
// each phase's done predicate reads.
type visibilityState struct {
	sc                              setupContext
	facts                           reposetup.Facts
	common, primary                 string
	shared, private                 layout.Layout
	current                         layout.Mode
	sharedStateDir, privateStateDir bool
	originDocket, bareDckt          gitcli.RemoteRef
	storeDckt                       gitcli.RemoteRef // the default store's dckt branch, read from disk while the dckt remote is unconfigured
	dcktURL, defaultTip             string
	worktrees                       []gitcli.WorktreeInfo
	localDocket, localDckt          string
	localConfig                     bool
	pendingConfig                   string
	journal                         switchJournal
	journalPresent                  bool

	// Derived facts.
	primaryBranch             string // the primary checkout's branch, short ("" when detached)
	primaryHead               string
	bareHoldsOrigin           bool // the bare dckt equals or descends from origin's docket (or origin has none and the bare one exists)
	originHoldsBare           bool // origin's docket equals or descends from the bare dckt (or the bare one is absent and origin's exists)
	storeHoldsOrigin          bool // the default store's dckt equals or descends from origin's docket (or origin has none and the store's exists)
	excludeBlock              bool // .git/info/exclude carries the valid managed block
	excludeBlockPresent       bool // .git/info/exclude carries any `# dckt:` block
	sharedCheckoutRegistered  bool
	sharedCheckoutReady       bool // on the docket branch with hooks off
	privateCheckoutRegistered bool
	privateCheckoutReady      bool // on the dckt branch with hooks off
	recordWorkingTree         bool // the ownership record lists a surface outside .git/
	recordGitDir              bool // the ownership record lists a surface under .git/
	privateInstructions       bool // .git/dckt/AGENTS.md exists
	headSharedFiles           bool // HEAD still holds a file the shared layout commits
	headPresentCommitPaths    []string
	headGitignoreValid        bool
	worktreeGitignoreValid    bool
	commitPathsDirty          bool
	foldSource                string
	identityKeysAligned       bool
	sharedConfigWritten       bool
	privateVisibilityAligned  bool
	sharedVisibilityAligned   bool
	sharedSurfacesSettled     bool // every authorized dispatch surface is settled on disk (read only going shared, once shared)
}

// probeVisibility reads the switch's state. Every probe error is returned:
// liveness, presence, and ancestry are never guessed.
func probeVisibility(ctx context.Context, d SetupDeps, sc setupContext, facts reposetup.Facts) (visibilityState, error) {
	git := d.Git
	repo := sc.repo
	st := visibilityState{
		sc:         sc,
		facts:      facts,
		common:     repo.CommonDir,
		primary:    repo.PrimaryWorktree,
		shared:     layout.SharedLayout(repo.CommonDir, repo.PrimaryWorktree),
		defaultTip: facts.RemoteDefaultBranch.Tip,
	}
	priv, err := privateLayoutOf(ctx, git, repo)
	if err != nil {
		return st, err
	}
	st.private = priv
	if st.current, err = layout.Detect(st.common); err != nil {
		return st, err
	}
	if st.sharedStateDir, err = isDirAt(st.shared.StateDir); err != nil {
		return st, err
	}
	if st.privateStateDir, err = isDirAt(st.private.StateDir); err != nil {
		return st, err
	}

	// Remote tips, fetched so their objects are local.
	if st.originDocket, err = probeAndFetch(ctx, git, repo, originRemote, metadataRef(st.shared)); err != nil {
		return st, fmt.Errorf("reading origin's docket branch: %w", err)
	}
	url, uerr := git.RemoteURL(ctx, repo, metadataRemote(st.private))
	switch {
	case uerr == nil:
		st.dcktURL = url
		if st.bareDckt, err = probeAndFetch(ctx, git, repo, metadataRemote(st.private), metadataRef(st.private)); err != nil {
			return st, fmt.Errorf("reading the dckt branch at %s: %w", url, err)
		}
	case isRemoteUnconfigured(uerr):
		st.bareDckt = gitcli.RemoteRef{State: gitcli.RemoteRefAbsent}
		if st.storeDckt, err = defaultStoreTip(ctx, git, st.private); err != nil {
			return st, fmt.Errorf("reading the dckt branch in %s: %w", st.private.DefaultBareRemote, err)
		}
		if st.storeDckt.State == gitcli.RemoteRefFound {
			st.storeHoldsOrigin = st.originDocket.State != gitcli.RemoteRefFound
			if !st.storeHoldsOrigin {
				if st.storeHoldsOrigin, err = storeHoldsCommit(ctx, git, st.private, st.storeDckt.Commit, st.originDocket.Commit); err != nil {
					return st, err
				}
			}
		}
	default:
		return st, uerr
	}
	if st.bareHoldsOrigin, err = remoteHolds(ctx, git, repo, st.bareDckt, st.originDocket); err != nil {
		return st, err
	}
	if st.originHoldsBare, err = remoteHolds(ctx, git, repo, st.originDocket, st.bareDckt); err != nil {
		return st, err
	}

	// Worktrees and local branches.
	if st.worktrees, err = git.ListWorktrees(ctx, repo); err != nil {
		return st, err
	}
	if st.sharedCheckoutRegistered, st.sharedCheckoutReady, err = checkoutReady(ctx, git, st.worktrees, st.shared); err != nil {
		return st, err
	}
	if st.privateCheckoutRegistered, st.privateCheckoutReady, err = checkoutReady(ctx, git, st.worktrees, st.private); err != nil {
		return st, err
	}
	if st.localDocket, err = localBranchTip(ctx, git, repo, metadataRef(st.shared)); err != nil {
		return st, err
	}
	if st.localDckt, err = localBranchTip(ctx, git, repo, metadataRef(st.private)); err != nil {
		return st, err
	}
	cs, err := git.WorktreeCheckoutState(ctx, st.primary)
	if err != nil {
		return st, err
	}
	st.primaryHead = string(cs.Head)
	if b, ok := shortBranch(cs.Branch); ok && !cs.Detached {
		st.primaryBranch = b
	}

	// Config files and the journal.
	localYML, localPresent, err := readOptionalFile(filepath.Join(st.primary, ".docket.local.yml"))
	if err != nil {
		return st, err
	}
	st.localConfig = localPresent
	var pendingBytes []byte
	for _, dir := range []string{st.private.StateDir, st.shared.StateDir} {
		p := filepath.Join(dir, layout.PrivateConfigFile)
		b, ok, err := readOptionalFile(p)
		if err != nil {
			return st, err
		}
		if ok {
			st.pendingConfig, pendingBytes = p, b
			break
		}
	}
	if st.journal, st.journalPresent, err = loadSwitchJournal(st.common); err != nil {
		return st, err
	}

	// Ignore blocks.
	exclude, _, err := readOptionalFile(filepath.Join(st.common, "info", "exclude"))
	if err != nil {
		return st, err
	}
	st.excludeBlock = reposetup.ValidExcludeBlock(exclude)
	st.excludeBlockPresent = bytes.Contains(exclude, []byte(reposetup.ExcludeStart))
	worktreeIgnore, _, err := readOptionalFile(filepath.Join(st.primary, gitignoreRel))
	if err != nil {
		return st, err
	}
	st.worktreeGitignoreValid = reposetup.ValidGitignoreBlock(worktreeIgnore)

	// The ownership record and the private instructions file.
	rec, err := reposeed.LoadRecord(reposeed.RecordPath(st.common, layout.StateName(st.common)))
	if err != nil {
		return st, err
	}
	if rec != nil {
		for _, s := range rec.Surfaces {
			if strings.HasPrefix(s.Path, ".git/") {
				st.recordGitDir = true
			} else {
				st.recordWorkingTree = true
			}
		}
	}
	if _, st.privateInstructions, err = readOptionalFile(layout.PrivateInstructionsPath(st.common)); err != nil {
		return st, err
	}

	// What HEAD and the working tree hold of the committed shared files.
	if err := st.readHeadFiles(ctx, git); err != nil {
		return st, err
	}
	changes, err := git.ChangedPaths(ctx, st.primary)
	if err != nil {
		return st, err
	}
	for _, c := range changes {
		for _, p := range visibilityCommitPaths {
			if string(c.Path) == p {
				st.commitPathsDirty = true
			}
		}
	}

	// Config facts.
	if err := st.readConfigFacts(ctx, git, pendingBytes, localYML); err != nil {
		return st, err
	}
	return st, nil
}

// readHeadFiles records which of the shared layout's committed files the
// primary checkout's HEAD still holds.
func (st *visibilityState) readHeadFiles(ctx context.Context, git *gitcli.Client) error {
	dispatchStart := []byte(document.MarkerSpelling(instructionsBlockName) + ":start")
	for _, rel := range visibilityCommitPaths {
		b, found, err := readCommitBlob(ctx, git, st.sc.repo, st.primaryHead, rel)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		held := false
		switch rel {
		case gitignoreRel:
			st.headGitignoreValid = reposetup.ValidGitignoreBlock(b)
			held = bytes.Contains(b, []byte(reposetup.GitignoreStart))
		case "AGENTS.md", "CLAUDE.md":
			held = bytes.Contains(b, dispatchStart)
		default:
			held = true
		}
		if held {
			st.headSharedFiles = true
			st.headPresentCommitPaths = append(st.headPresentCommitPaths, rel)
		}
	}
	return nil
}

// readConfigFacts decides the config phases' done predicates: where a fold
// reads from, whether the identity keys agree with origin, whether the
// committed half is written, and whether each layer's explicit visibility
// already matches.
func (st *visibilityState) readConfigFacts(ctx context.Context, git *gitcli.Client, pending, localYML []byte) error {
	originYML, originFound, err := readDefaultDocketYML(ctx, git, *st)
	if err != nil {
		return err
	}
	switch {
	case st.pendingConfig != "":
		st.foldSource = "the configuration already waiting in " + st.pendingConfig
	case originFound:
		st.foldSource = "origin's " + st.sc.defaultBranch + " .docket.yml"
	default:
		_, headFound, err := readCommitBlob(ctx, git, st.sc.repo, st.primaryHead, ".docket.yml")
		if err != nil {
			return err
		}
		if headFound {
			st.foldSource = "the primary checkout's HEAD .docket.yml"
		} else {
			st.foldSource = "an empty configuration"
		}
	}

	st.privateVisibilityAligned = true
	st.identityKeysAligned = true
	st.sharedConfigWritten = true
	if st.pendingConfig != "" {
		vis, err := reposetup.ConfigLeafValues(pending, []string{"visibility"})
		if err != nil {
			return fmt.Errorf("reading %s: %w", st.pendingConfig, err)
		}
		if v, ok := vis["visibility"]; ok && v != string(layout.Private) {
			st.privateVisibilityAligned = false
		}
		want, err := reposetup.ConfigLeafValues(pending, repositoryIdentityKeys)
		if err != nil {
			return fmt.Errorf("reading %s: %w", st.pendingConfig, err)
		}
		if len(want) > 0 {
			have := map[string]string{}
			if originFound {
				if have, err = reposetup.ConfigLeafValues(originYML, repositoryIdentityKeys); err != nil {
					return fmt.Errorf("reading origin's .docket.yml: %w", err)
				}
			}
			for k, v := range want {
				if have[k] != v {
					st.identityKeysAligned = false
				}
			}
		}
		localKeys, _, err := readOptionalFile(filepath.Join(filepath.Dir(st.pendingConfig), layout.PrivateLocalKeysFile))
		if err != nil {
			return err
		}
		committed, _, err := reposetup.SplitPrivateConfig(pending, localKeys, config.RepoOnlyPaths())
		if err != nil {
			return fmt.Errorf("splitting %s: %w", st.pendingConfig, err)
		}
		onDisk, _, err := readOptionalFile(filepath.Join(st.primary, ".docket.yml"))
		if err != nil {
			return err
		}
		st.sharedConfigWritten = bytes.Equal(onDisk, committed)
	}

	st.sharedVisibilityAligned = true
	if st.localConfig {
		vis, err := reposetup.ConfigLeafValues(localYML, []string{"visibility"})
		if err != nil {
			return fmt.Errorf("reading .docket.local.yml: %w", err)
		}
		if v, ok := vis["visibility"]; ok && v != string(layout.Shared) {
			st.sharedVisibilityAligned = false
		}
	}
	return nil
}

// readDefaultDocketYML reads .docket.yml at origin's default-branch tip.
func readDefaultDocketYML(ctx context.Context, git *gitcli.Client, st visibilityState) ([]byte, bool, error) {
	if st.defaultTip == "" {
		return nil, false, nil
	}
	return readCommitBlob(ctx, git, st.sc.repo, st.defaultTip, ".docket.yml")
}

// probeAndFetch probes remote's ref and, when found, fetches it so its objects
// are local. A fetched tip that differs from the probe is the fetched one.
func probeAndFetch(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName) (gitcli.RemoteRef, error) {
	rr, err := git.ProbeRemoteBranch(ctx, repo, remote, ref)
	if err != nil {
		return rr, err
	}
	if rr.State != gitcli.RemoteRefFound {
		return rr, nil
	}
	rev, err := git.FetchBranch(ctx, repo, remote, ref)
	if err != nil {
		return rr, err
	}
	rr.Commit = rev.Commit
	return rr, nil
}

// remoteHolds reports whether holder equals or descends from source; an absent
// source is held by any present holder.
func remoteHolds(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, holder, source gitcli.RemoteRef) (bool, error) {
	if holder.State != gitcli.RemoteRefFound {
		return false, nil
	}
	if source.State != gitcli.RemoteRefFound || holder.Commit == source.Commit {
		return true, nil
	}
	return git.IsAncestor(ctx, repo, source.Commit, holder.Commit)
}

// checkoutReady reports whether l's metadata worktree is registered and, when
// it is, whether it is on l's metadata branch with hooks off.
func checkoutReady(ctx context.Context, git *gitcli.Client, wts []gitcli.WorktreeInfo, l layout.Layout) (registered, ready bool, err error) {
	wt, ok := worktreeAt(wts, l.MetadataWorktree)
	if !ok {
		return false, false, nil
	}
	if wt.Prunable || wt.Branch != metadataRef(l) {
		return true, false, nil
	}
	off, err := git.WorktreeHooksDisabled(ctx, l.MetadataWorktree)
	if err != nil {
		return true, false, err
	}
	return true, off, nil
}

// worktreeAt finds the registration at path.
func worktreeAt(wts []gitcli.WorktreeInfo, path string) (gitcli.WorktreeInfo, bool) {
	for _, wt := range wts {
		if filepath.Clean(wt.Path) == filepath.Clean(path) {
			return wt, true
		}
	}
	return gitcli.WorktreeInfo{}, false
}

// localBranchTip resolves a local branch; an unresolvable ref is "".
func localBranchTip(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, ref gitcli.RefName) (string, error) {
	id, err := git.ResolveRef(ctx, repo, ref)
	if err != nil {
		if f, ok := gitcli.AsFailure(err); ok && f.Kind == gitcli.KindRefUnavailable {
			return "", nil
		}
		return "", err
	}
	return string(id), nil
}

// isDirAt reports whether p is a directory; absence is false, any other probe
// error is returned.
func isDirAt(p string) (bool, error) {
	fi, err := os.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	}
	return fi.IsDir(), nil
}

// readOptionalFile reads p; absence is (nil, false, nil).
func readOptionalFile(p string) ([]byte, bool, error) {
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// visibilityPin is the composite pin over the remote tips the switch reads.
func visibilityPin(st visibilityState) string {
	tip := func(r gitcli.RemoteRef) string {
		if r.State == gitcli.RemoteRefFound {
			return string(r.Commit)
		}
		return "absent"
	}
	def := st.defaultTip
	if def == "" {
		def = "absent"
	}
	dckt := st.bareDckt
	if dckt.State != gitcli.RemoteRefFound {
		dckt = st.storeDckt // read from the default store while the dckt remote is unconfigured
	}
	return fmt.Sprintf("origin-docket=%s dckt=%s origin-default=%s", tip(st.originDocket), tip(dckt), def)
}

// --- preconditions -----------------------------------------------------------

// visibilityLiveRunLines lists every live run and every busy gate lock under
// both existing state folders, each with its remedy.
func visibilityLiveRunLines(st visibilityState) ([]string, error) {
	var lines []string
	for _, dir := range []string{st.shared.StateDir, st.private.StateDir} {
		ok, err := isDirAt(dir)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		runs, err := liveRunsUnder(dir)
		if err != nil {
			return nil, err
		}
		for _, r := range runs {
			line := fmt.Sprintf("run %s (%s", r.Key, r.State)
			if r.ChangeID != "" {
				line += ", change " + r.ChangeID
			}
			lines = append(lines, line+"): "+r.Remedy)
		}
		locks, err := gatedrive.BusyWorktreeLocks(dir)
		if err != nil {
			return nil, err
		}
		for _, l := range locks {
			lines = append(lines, "a gate holds "+filepath.Join(l, "busy.lock")+holderNote(l)+": wait for it to finish, or stop its run")
		}
	}
	return lines, nil
}

// holderNote names a busy lock's holder.json when it is readable (a
// diagnostic only: it names the last holder).
func holderNote(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "holder.json"))
	if err != nil {
		return ""
	}
	var n gatedrive.HolderNote
	if json.Unmarshal(b, &n) != nil {
		return ""
	}
	var parts []string
	if n.ChangeID != "" {
		parts = append(parts, "change "+n.ChangeID)
	}
	if n.RunDir != "" {
		parts = append(parts, "run "+n.RunDir)
	}
	if len(parts) == 0 {
		return ""
	}
	return " (last holder: " + strings.Join(parts, ", ") + ")"
}

// visibilityMetadataSyncRefusal refuses a metadata worktree (at either layout's
// path) that is dirty, mid-operation, or holds commits its authoritative remote
// branch lacks. No registered worktree passes.
func visibilityMetadataSyncRefusal(ctx context.Context, git *gitcli.Client, st visibilityState) (string, error) {
	for _, l := range []layout.Layout{st.shared, st.private} {
		wt, ok := worktreeAt(st.worktrees, l.MetadataWorktree)
		if !ok || wt.Prunable {
			continue
		}
		clean, _, _ := worktreeCleanState(ctx, git, l.MetadataWorktree)
		switch clean {
		case reposetup.PresenceUnknown:
			return "", fmt.Errorf("the clean state of %s could not be read", l.MetadataWorktree)
		case reposetup.PresenceAbsent:
			return "commit or discard the changes in " + l.MetadataWorktree + ", then re-run", nil
		}
		auth := ""
		switch wt.Branch {
		case metadataRef(st.shared):
			if st.originDocket.State == gitcli.RemoteRefFound {
				auth = string(st.originDocket.Commit)
			} else if st.bareDckt.State == gitcli.RemoteRefFound {
				auth = string(st.bareDckt.Commit)
			}
		case metadataRef(st.private):
			if st.bareDckt.State == gitcli.RemoteRefFound {
				auth = string(st.bareDckt.Commit)
			}
		default:
			continue
		}
		head := string(wt.Head)
		published := head != "" && head == auth
		switch {
		case published || head == "":
		case auth != "":
			anc, err := git.IsAncestor(ctx, st.sc.repo, gitcli.ObjectID(head), gitcli.ObjectID(auth))
			if err != nil {
				return "", err
			}
			published = anc
		case wt.Branch == metadataRef(st.shared) && st.storeDckt.State == gitcli.RemoteRefFound:
			// Neither remote holds a branch this clone can read, but another
			// clone on this machine published the history to the default store.
			held, err := storeHoldsCommit(ctx, git, st.private, st.storeDckt.Commit, gitcli.ObjectID(head))
			if err != nil {
				return "", err
			}
			published = held
		}
		if !published {
			return "the metadata worktree " + l.MetadataWorktree + " holds commits its remote branch lacks; publish them with `docket repository prepare`, or resolve them by hand", nil
		}
	}
	return "", nil
}

// --- warnings ----------------------------------------------------------------

// visibilityWarnings collects the preview's warnings; none of them refuses.
func visibilityWarnings(ctx context.Context, d SetupDeps, st visibilityState, o SetVisibilityOptions) []string {
	var out []string
	if o.Target == string(layout.Private) {
		out = append(out, fmt.Sprintf("teammates' later writes to %s/%s will not be seen by this clone",
			st.shared.MetadataRemote, st.shared.MetadataBranch))
		if o.DeleteSharedBranch {
			out = append(out, "existing pull-request backlinks into that branch will stop resolving")
		}
	}
	out = append(out, absoluteLinkWarning(ctx, d.Git, st)...)
	if o.Target == string(layout.Private) {
		out = append(out, docketTextPRWarning(ctx, d, st)...)
	}
	return out
}

// visibilityMetadataTip is the metadata tip the warnings read: origin's docket
// branch, else the bare dckt branch ("" when neither exists).
func visibilityMetadataTip(st visibilityState) string {
	if st.originDocket.State == gitcli.RemoteRefFound {
		return string(st.originDocket.Commit)
	}
	if st.bareDckt.State == gitcli.RemoteRefFound {
		return string(st.bareDckt.Commit)
	}
	return ""
}

// absoluteLinkWarning warns about metadata files that still carry absolute
// links into the docket branch. A read error is its own warning.
func absoluteLinkWarning(ctx context.Context, git *gitcli.Client, st visibilityState) []string {
	originURL, err := git.RemoteURL(ctx, st.sc.repo, originRemote)
	if err != nil {
		return []string{"metadata files were not checked for absolute links: " + err.Error()}
	}
	webURL := githubWebURL(originURL)
	tip := visibilityMetadataTip(st)
	if webURL == "" || tip == "" {
		return nil
	}
	files, err := readMetadataMarkdown(ctx, git, st.sc.repo, tip)
	if err != nil {
		return []string{"metadata files were not checked for absolute links: " + err.Error()}
	}
	hits := absoluteMetadataLinks(webURL, layout.SharedName, files)
	if len(hits) == 0 {
		return nil
	}
	shown := hits
	more := ""
	if len(hits) > visibilityLinkListCap {
		shown = hits[:visibilityLinkListCap]
		more = fmt.Sprintf(", and %d more", len(hits)-visibilityLinkListCap)
	}
	return []string{fmt.Sprintf("%d metadata file(s) still carry absolute links into the docket branch; run `docket repository repair` to make the generated ones relative: %s%s",
		len(hits), strings.Join(shown, ", "), more)}
}

// readMetadataMarkdown reads every .md blob at the metadata tip.
func readMetadataMarkdown(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, tip string) (map[string][]byte, error) {
	src, err := git.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: gitcli.ObjectID(tip)})
	if err != nil {
		return nil, err
	}
	entries, err := src.ListTree(ctx, nil)
	if err != nil {
		return nil, err
	}
	var paths []gitcli.RepoPath
	for _, e := range entries {
		if e.Type == "blob" && strings.HasSuffix(string(e.Path), ".md") {
			paths = append(paths, e.Path)
		}
	}
	files := map[string][]byte{}
	if len(paths) == 0 {
		return files, nil
	}
	blobs, err := src.ReadBlobs(ctx, paths)
	if err != nil {
		return nil, err
	}
	for _, b := range blobs {
		if b.Found {
			files[string(b.Path)] = b.Blob.Bytes
		}
	}
	return files, nil
}

// absoluteMetadataLinks lists, sorted, the files whose bytes link absolutely
// into branch on the repository at webURL (a blob/ or tree/ URL). An empty
// webURL matches nothing.
func absoluteMetadataLinks(webURL, branch string, files map[string][]byte) []string {
	if webURL == "" {
		return nil
	}
	needles := [][]byte{
		[]byte(webURL + "/blob/" + branch + "/"),
		[]byte(webURL + "/tree/" + branch + "/"),
	}
	var out []string
	for p, b := range files {
		for _, n := range needles {
			if bytes.Contains(b, n) {
				out = append(out, p)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// docketTextPRWarning lists the open pull requests whose descriptions carry
// docket text; it never edits them, and an unchecked list is its own warning.
func docketTextPRWarning(ctx context.Context, d SetupDeps, st visibilityState) []string {
	if d.GitHub == nil {
		return []string{"open pull requests were not checked: no GitHub client is available"}
	}
	tip := visibilityMetadataTip(st)
	if tip == "" {
		return []string{"open pull requests were not checked: no metadata branch was readable"}
	}
	sc := st.sc
	sc.metadataTip = tip
	corpus, err := readCheckCorpus(ctx, d.Git, sc)
	if err != nil {
		return []string{"open pull requests were not checked: " + err.Error()}
	}
	snap, ok := buildCorpusSnapshot(sc.cfg, corpus.records)
	if !ok {
		return []string{"open pull requests were not checked: the metadata corpus could not be read"}
	}
	urls, err := listDocketTextPRs(ctx, d.GitHub, st.primary, snap.Changes())
	if err != nil {
		return []string{"open pull requests were not checked: " + err.Error()}
	}
	if len(urls) == 0 {
		return nil
	}
	return []string{"open pull requests whose descriptions carry docket text (listed, not edited): " + strings.Join(urls, ", ")}
}

// docketTextMarker is what a docket-written PR description carries.
const docketTextMarker = "<!-- docket:"

// listDocketTextPRs reads the pull requests of the in-progress and implemented
// changes in one batched read and returns, sorted by number, the URLs of the
// open ones whose description carries docket text.
func listDocketTextPRs(ctx context.Context, gh RepairGitHub, dir string, changes []domain.Change) ([]string, error) {
	var numbers []int
	for _, c := range changes {
		if s := c.Status(); s != domain.StatusInProgress && s != domain.StatusImplemented {
			continue
		}
		if !finalizeHasPRRef(c) {
			continue
		}
		if n, ok := parsePRNumber(c.PR().Value); ok {
			numbers = append(numbers, n)
		}
	}
	numbers = sweepDedupeSortAsc(numbers)
	if len(numbers) == 0 {
		return nil, nil
	}
	repo, err := gh.DiscoverRepository(ctx, dir)
	if err != nil {
		return nil, err
	}
	var urls []string
	for _, chunk := range sweepChunkInts(numbers, sweepPRBatchCap) {
		res, err := gh.ViewPullRequestsBatch(ctx, repo, chunk)
		if err != nil {
			return nil, err
		}
		for _, n := range chunk {
			r := res[n]
			if !r.Found || r.PR.State != githubcli.StateOpen || !strings.Contains(r.PR.Body, docketTextMarker) {
				continue
			}
			u := r.PR.URL
			if u == "" {
				u = "#" + strconv.Itoa(n)
			}
			urls = append(urls, u)
		}
	}
	return urls, nil
}
