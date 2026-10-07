package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
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

// The two fixed subjects of every integration-branch commit a visibility switch
// makes. They carry no body, no trailer, and no mode vocabulary
// (TestVisibilitySubjectsCarryNoModeVocabulary).
const (
	visibilityRemoveSubject = "Remove docket configurations from repository"
	visibilityAddSubject    = "Add docket configurations to repository"
)

// visibilityCommitPaths are the integration-branch paths a switch may commit:
// the committed config, the managed .gitignore block, and every dispatch
// surface reposeed plans for a shared repository.
var visibilityCommitPaths = append([]string{docketYMLRel, gitignoreRel}, reposeed.SurfacePaths()...)

// switchJournalFile is the write-ahead journal, kept in layout.StateDirOf(common)
// so it travels with a state-folder rename.
const switchJournalFile = "visibility-switch.json"

// digestDeleted is the journal digest of a path the switch removed.
const digestDeleted = "deleted"

// switchJournal: path -> digest of what the switch wrote ("deleted" for a removal).
type switchJournal struct {
	Subject string            `json:"subject"`
	Paths   map[string]string `json:"paths"`
}

func switchJournalPath(commonDir string) string {
	return filepath.Join(layout.StateDirOf(commonDir), switchJournalFile)
}

func validVisibilitySubject(s string) bool {
	return s == visibilityRemoveSubject || s == visibilityAddSubject
}

func contentDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// pathDigest is "sha256:<hex>" of a regular file, "link:<target>" of a symlink,
// and "deleted" when abs is absent; anything else (a directory, a device) errors.
func pathDigest(abs string) (string, error) {
	fi, err := os.Lstat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return digestDeleted, nil
	case err != nil:
		return "", err
	}
	switch {
	case fi.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(abs)
		if err != nil {
			return "", err
		}
		return "link:" + target, nil
	case fi.Mode().IsRegular():
		b, err := os.ReadFile(abs)
		if err != nil {
			return "", err
		}
		return contentDigest(b), nil
	default:
		return "", fmt.Errorf("%s is neither a regular file nor a symlink", abs)
	}
}

// headPathDigest is pathDigest's counterpart for rel in commit: a symlink entry
// (mode 120000) is "link:<blob>", an absent entry "deleted".
func headPathDigest(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, commit gitcli.ObjectID, rel string) (string, error) {
	src, err := git.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: commit})
	if err != nil {
		return "", err
	}
	res, err := src.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(rel)})
	if err != nil {
		return "", err
	}
	if len(res) != 1 {
		return "", fmt.Errorf("reading %s at %s: expected one result, got %d", rel, commit, len(res))
	}
	if !res[0].Found {
		return digestDeleted, nil
	}
	if res[0].Blob.Mode == "120000" {
		return "link:" + string(res[0].Blob.Bytes), nil
	}
	return contentDigest(res[0].Blob.Bytes), nil
}

// loadSwitchJournal reads the journal; ok is false when none exists. A journal
// that does not decode, or names an unknown subject, is an error, never absence.
func loadSwitchJournal(commonDir string) (switchJournal, bool, error) {
	p := switchJournalPath(commonDir)
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return switchJournal{}, false, nil
	}
	if err != nil {
		return switchJournal{}, false, err
	}
	var j switchJournal
	if err := json.Unmarshal(b, &j); err != nil {
		return switchJournal{}, false, fmt.Errorf("visibility switch journal %s is unreadable: %w", p, err)
	}
	if !validVisibilitySubject(j.Subject) {
		return switchJournal{}, false, fmt.Errorf("visibility switch journal %s names an unknown commit subject %q", p, j.Subject)
	}
	for rel := range j.Paths {
		if err := validateSwitchRel(rel); err != nil {
			return switchJournal{}, false, fmt.Errorf("visibility switch journal %s: %w", p, err)
		}
	}
	if j.Paths == nil {
		j.Paths = map[string]string{}
	}
	return j, true, nil
}

// saveSwitchJournal writes the journal through a temp file beside it + rename.
func saveSwitchJournal(commonDir string, j switchJournal) error {
	if !validVisibilitySubject(j.Subject) {
		return fmt.Errorf("visibility switch journal: unknown commit subject %q", j.Subject)
	}
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return writeFileReplacing(switchJournalPath(commonDir), append(b, '\n'), 0o600)
}

// clearSwitchJournal removes the journal; an absent one is already clear.
func clearSwitchJournal(commonDir string) error {
	if err := os.Remove(switchJournalPath(commonDir)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// validateSwitchRel accepts only a clean, slash-separated path inside the checkout.
func validateSwitchRel(rel string) error {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "\\") ||
		path.Clean(rel) != rel || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return fmt.Errorf("invalid checkout-relative path %q", rel)
	}
	return nil
}

// recordSwitchDigest journals rel -> digest under subject, refusing a journal
// that already belongs to the other subject and holds a path. A journal of the
// other subject holding no path (the marker an abandoned going-shared run
// opened) has no commit pending, so it is replaced.
func recordSwitchDigest(commonDir, subject, rel, digest string) error {
	if !validVisibilitySubject(subject) {
		return fmt.Errorf("unknown commit subject %q", subject)
	}
	if err := validateSwitchRel(rel); err != nil {
		return err
	}
	j, ok, err := loadSwitchJournal(commonDir)
	if err != nil {
		return err
	}
	if ok && j.Subject != subject && len(j.Paths) > 0 {
		return fmt.Errorf("a pending switch commit %q is journaled; it must complete before %q", j.Subject, subject)
	}
	if !ok || j.Subject != subject {
		j = switchJournal{Subject: subject, Paths: map[string]string{}}
	}
	j.Paths[rel] = digest
	return saveSwitchJournal(commonDir, j)
}

// writeSwitchPath journals the digest of content (nil = delete) BEFORE writing
// <primary>/<rel> atomically (or removing it); subject must match an existing journal's.
func writeSwitchPath(commonDir, primary, subject, rel string, content []byte) error {
	digest := digestDeleted
	if content != nil {
		digest = contentDigest(content)
	}
	if err := recordSwitchDigest(commonDir, subject, rel, digest); err != nil {
		return err
	}
	abs := filepath.Join(primary, filepath.FromSlash(rel))
	if content == nil {
		if err := os.Remove(abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	mode := fs.FileMode(0o644)
	if fi, err := os.Lstat(abs); err == nil && fi.Mode().IsRegular() {
		mode = fi.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return writeFileReplacing(abs, content, mode)
}

// journalSwitchPath records rel's current on-disk digest (after install wrote a surface).
func journalSwitchPath(commonDir, primary, subject, rel string) error {
	if err := validateSwitchRel(rel); err != nil {
		return err
	}
	digest, err := pathDigest(filepath.Join(primary, filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	return recordSwitchDigest(commonDir, subject, rel, digest)
}

// unjournalSwitchPath drops rel from the journal; an absent journal or entry
// is already clear.
func unjournalSwitchPath(commonDir, rel string) error {
	j, ok, err := loadSwitchJournal(commonDir)
	if err != nil || !ok {
		return err
	}
	if _, has := j.Paths[rel]; !has {
		return nil
	}
	delete(j.Paths, rel)
	return saveSwitchJournal(commonDir, j)
}

// writeFileReplacing writes content to p through a same-directory temp file and
// a rename, creating p's state folder when it is missing.
func writeFileReplacing(p string, content []byte, mode fs.FileMode) error {
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(p)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, p)
}

// commitPathConflicts: one refusal line per path in ChangedPaths whose working-tree
// digest is not its journal digest (or is unjournaled), plus a detached HEAD.
// A probe error is returned.
func commitPathConflicts(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, commonDir string, paths []string) ([]string, error) {
	primary := repo.PrimaryWorktree
	st, err := git.WorktreeCheckoutState(ctx, primary)
	if err != nil {
		return nil, err
	}
	var out []string
	if st.Detached {
		out = append(out, fmt.Sprintf("the primary checkout %s has a detached HEAD; check out a branch to commit on", primary))
	}
	changes, err := git.ChangedPaths(ctx, primary)
	if err != nil {
		return nil, err
	}
	changed := make(map[string]bool, len(changes))
	for _, c := range changes {
		changed[string(c.Path)] = true
	}
	j, _, err := loadSwitchJournal(commonDir)
	if err != nil {
		return nil, err
	}
	for _, rel := range paths {
		if !changed[rel] {
			continue
		}
		want, journaled := j.Paths[rel]
		if journaled {
			got, err := pathDigest(filepath.Join(primary, filepath.FromSlash(rel)))
			if err != nil {
				return nil, err
			}
			if got == want {
				continue
			}
		}
		out = append(out, fmt.Sprintf("%s has uncommitted changes; commit or stash them first", rel))
	}
	return out, nil
}

type visibilityCommitOutcome struct {
	Commit   string // "" when nothing was committed
	Paths    []string
	Complete bool // HEAD holds every journaled path
}

// commitSwitchJournal commits the journaled paths whose HEAD digest differs from the
// journal, with its subject, via CommitOwnPaths, then clears the journal. When HEAD
// already holds them all (death between commit and cleanup) it commits nothing and
// clears. A commit failure leaves journal and edits in place. A journaled path
// whose working tree no longer matches the journal refuses: committing it would
// sweep someone else's edit into the switch's commit.
func commitSwitchJournal(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, commonDir string) (visibilityCommitOutcome, error) {
	j, ok, err := loadSwitchJournal(commonDir)
	if err != nil {
		return visibilityCommitOutcome{}, err
	}
	if !ok {
		return visibilityCommitOutcome{Complete: true}, nil
	}
	primary := repo.PrimaryWorktree
	pending, err := pendingSwitchPaths(ctx, git, repo, j)
	if err != nil {
		return visibilityCommitOutcome{}, err
	}
	if len(pending) == 0 {
		if err := clearSwitchJournal(commonDir); err != nil {
			return visibilityCommitOutcome{}, err
		}
		return visibilityCommitOutcome{Complete: true}, nil
	}
	repoPaths := make([]gitcli.RepoPath, 0, len(pending))
	for _, rel := range pending {
		got, err := pathDigest(filepath.Join(primary, filepath.FromSlash(rel)))
		if err != nil {
			return visibilityCommitOutcome{}, err
		}
		if got != j.Paths[rel] {
			return visibilityCommitOutcome{}, fmt.Errorf("%s changed after the switch wrote it; commit or restore it, then re-run", rel)
		}
		repoPaths = append(repoPaths, gitcli.RepoPath(rel))
	}
	head, err := git.CommitOwnPaths(ctx, primary, repoPaths, j.Subject)
	if err != nil {
		return visibilityCommitOutcome{}, fmt.Errorf("committing %s: %w; the edits stay in the working tree and a re-run retries the commit", strings.Join(pending, ", "), err)
	}
	out := visibilityCommitOutcome{Commit: string(head), Paths: pending}
	left, err := pendingSwitchPaths(ctx, git, repo, j)
	if err != nil {
		return out, err
	}
	if len(left) > 0 {
		return out, nil // a hook rewrote content: keep the journal for a re-run
	}
	if err := clearSwitchJournal(commonDir); err != nil {
		return out, err
	}
	out.Complete = true
	return out, nil
}

// pendingSwitchPaths lists, sorted, the journaled paths whose HEAD digest
// differs from the journal's.
func pendingSwitchPaths(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, j switchJournal) ([]string, error) {
	st, err := git.WorktreeCheckoutState(ctx, repo.PrimaryWorktree)
	if err != nil {
		return nil, err
	}
	rels := make([]string, 0, len(j.Paths))
	for rel := range j.Paths {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	var pending []string
	for _, rel := range rels {
		got, err := headPathDigest(ctx, git, repo, st.Head, rel)
		if err != nil {
			return nil, err
		}
		if got != j.Paths[rel] {
			pending = append(pending, rel)
		}
	}
	return pending, nil
}

// visibilityCommitPlan is what the pending integration-branch commit touches:
// guard is every path the run writes or commits (the user's own edits to any of
// them refuse), rows are the paths the commit will actually change, each with
// what happens to it.
type visibilityCommitPlan struct {
	guard []string
	rows  []string
}

// planVisibilityCommit computes the pending commit's paths from the same
// sources its executors use: the removal targets of HEAD going private, and
// going shared the committed config, the managed .gitignore block, and the
// dispatch surfaces reposeed plans for the shared layout (the ones the managed
// block ignores excepted). With the identity-keys phase pending, the run
// commits .docket.yml alone and stops.
func planVisibilityCommit(ctx context.Context, git *gitcli.Client, st visibilityState, o SetVisibilityOptions, steps []visibilityStep) (visibilityCommitPlan, error) {
	var plan visibilityCommitPlan
	if !visibilityCommitsPending(steps) {
		return plan, nil
	}
	if o.Target == string(layout.Private) {
		if st.primaryHead == "" {
			return plan, nil
		}
		targets, err := removalTargets(ctx, git, st.sc.repo, gitcli.ObjectID(st.primaryHead))
		if err != nil {
			return plan, err
		}
		for _, t := range targets {
			action := "write"
			if t.content == nil {
				action = "delete"
			}
			plan.guard = append(plan.guard, t.rel)
			plan.rows = append(plan.rows, t.rel+" ("+action+")")
		}
		return plan, nil
	}

	pending := map[string]bool{}
	for _, s := range steps {
		pending[s.name] = !s.done
	}
	// add records a path the run writes or commits; final is its content once
	// the run is through (nil: it is absent), so it is a row iff that differs
	// from HEAD.
	add := func(rel string, final []byte, present bool) error {
		plan.guard = append(plan.guard, rel)
		want := digestDeleted
		if present {
			want = contentDigest(final)
		}
		have := digestDeleted
		if st.primaryHead != "" {
			var err error
			if have, err = headPathDigest(ctx, git, st.sc.repo, gitcli.ObjectID(st.primaryHead), rel); err != nil {
				return err
			}
		}
		if want != have {
			plan.rows = append(plan.rows, rel+" (write)")
		}
		return nil
	}

	pc, err := readPendingPrivateConfig(st)
	if err != nil {
		return plan, err
	}
	_, ymlJournaled := st.journal.Paths[docketYMLRel]
	ymlWritten := pc.path != "" && (pending["identity-keys"] || pending["config-committed"])
	if ymlWritten || ymlJournaled {
		final, present, err := readOptionalFile(filepath.Join(st.primary, docketYMLRel))
		if err != nil {
			return plan, err
		}
		if pc.path != "" {
			if final, _, err = splitSharedConfig(st, pc); err != nil {
				return plan, err
			}
			present = true
		}
		if err := add(docketYMLRel, final, present); err != nil {
			return plan, err
		}
	}
	if pending["identity-keys"] {
		return plan, nil
	}

	ignore, _, err := readOptionalFile(filepath.Join(st.primary, gitignoreRel))
	if err != nil {
		return plan, err
	}
	_, ignoreJournaled := st.journal.Paths[gitignoreRel]
	if pending["ignore"] {
		ensured, changed, err := reposetup.EnsureGitignoreBlock(ignore)
		if err != nil {
			return plan, privateRefusal(reposetup.StateNeedsReview, gitignoreRel+": "+err.Error()+"; fix the markers by hand, then re-run")
		}
		if changed {
			ignore, ignoreJournaled = ensured, true
		}
	}
	if ignoreJournaled {
		if err := add(gitignoreRel, ignore, true); err != nil {
			return plan, err
		}
	}

	surfaces, err := plannedSharedSurfaces(ctx, git, st, pc)
	if err != nil {
		return plan, err
	}
	for _, t := range surfaces {
		rel, err := filepath.Rel(st.primary, t.Path)
		if err != nil {
			return plan, err
		}
		rel = filepath.ToSlash(rel)
		plan.guard = append(plan.guard, rel)
		insp, err := install.InspectTarget(t, nil, nil)
		if err != nil {
			return plan, err
		}
		changed := insp.Disposition != install.DispositionNoop
		if !changed {
			disk, err := pathDigest(t.Path)
			if err != nil {
				return plan, err
			}
			head := digestDeleted
			if st.primaryHead != "" {
				if head, err = headPathDigest(ctx, git, st.sc.repo, gitcli.ObjectID(st.primaryHead), rel); err != nil {
					return plan, err
				}
			}
			changed = disk != head
		}
		if changed {
			plan.rows = append(plan.rows, rel+" (write)")
		}
	}
	return plan, nil
}

// plannedSharedSurfaces is the dispatch surfaces the going-shared commit phase
// installs, minus the ones the managed .gitignore block ignores: planned from
// the split of the private config while one is pending, else as install's
// repository phase resolves them once the repository reads shared. No
// authorization plans nothing.
func plannedSharedSurfaces(ctx context.Context, git *gitcli.Client, st visibilityState, pc pendingPrivateConfig) ([]install.Target, error) {
	runTracker, err := buildRunTracker()
	if err != nil {
		return nil, err
	}
	var targets []install.Target
	if pc.path != "" {
		committed, local, err := splitSharedConfig(st, pc)
		if err != nil {
			return nil, err
		}
		sources := []config.Source{{Layer: config.LayerRepository, Name: docketYMLRel, Data: committed}}
		if local != nil {
			sources = append(sources, config.Source{Layer: config.LayerRepositoryLocal, Name: localConfigRel, Data: local})
		}
		snap, _, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: st.sc.defaultBranch})
		if err != nil {
			return nil, privateRefusal(reposetup.StateNeedsReview, "the configuration split from "+pc.path+" does not resolve: "+err.Error()+"; fix it by hand, then re-run")
		}
		ah := snap.Effective.AgentHarnesses
		if !ah.Explicit || !isRepositoryLayer(ah.Provenance.Layer) {
			return nil, nil
		}
		if targets, _, err = reposeed.Plan(reposeed.PlanInput{
			WorktreeRoot:  st.primary,
			Harnesses:     ah.Value,
			RunTracker:    runTracker,
			ClaudeMDState: classifyClaudeMD(st.primary),
		}); err != nil {
			return nil, err
		}
	} else if st.current == layout.Shared {
		phase, _, _, err := ResolveRepoPhase(ctx, git, st.primary, nil, runTracker, nil, config.ResolveContext{DefaultBranch: "main"})
		if err != nil {
			return nil, err
		}
		if phase == nil || !phase.Authorized {
			return nil, nil
		}
		targets = phase.Targets
	}
	ignored := map[string]bool{}
	for _, e := range reposetup.GitignoreEntries() {
		ignored[e] = true
	}
	out := make([]install.Target, 0, len(targets))
	for _, t := range targets {
		rel, err := filepath.Rel(st.primary, t.Path)
		if err != nil {
			return nil, err
		}
		if !ignored[filepath.ToSlash(rel)] {
			out = append(out, t)
		}
	}
	return out, nil
}
