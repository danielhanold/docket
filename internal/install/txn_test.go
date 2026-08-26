package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
)

// ---------------------------------------------------------------------------
// Filesystem seams the transaction tests drive
// ---------------------------------------------------------------------------

// injectFS wraps an FSOps and can refuse a chosen mutation. Every rollback
// assertion in this file rests on it: a transaction is only journaled if the
// world it half-changed can be put back, and the only way to observe that is to
// make the filesystem fail somewhere the engine did not choose.
type injectFS struct {
	inner FSOps
	// fail is consulted before each mutation reaches the real filesystem. A
	// non-nil error is returned to the engine instead of performing it.
	fail func(op, path string) error
}

func (f *injectFS) check(op, path string) error {
	if f.fail == nil {
		return nil
	}
	return f.fail(op, path)
}

func (f *injectFS) WriteFile(path string, data []byte, mode os.FileMode) error {
	if err := f.check("WriteFile", path); err != nil {
		return err
	}
	return f.inner.WriteFile(path, data, mode)
}

func (f *injectFS) Chmod(path string, mode os.FileMode) error {
	if err := f.check("Chmod", path); err != nil {
		return err
	}
	return f.inner.Chmod(path, mode)
}

// Rename reports the destination: it is the path whose contents an interrupted
// rename decides, so it is the one a targeted injection needs to name.
func (f *injectFS) Rename(old, new string) error {
	if err := f.check("Rename", new); err != nil {
		return err
	}
	return f.inner.Rename(old, new)
}

func (f *injectFS) Symlink(target, path string) error {
	if err := f.check("Symlink", path); err != nil {
		return err
	}
	return f.inner.Symlink(target, path)
}

func (f *injectFS) Remove(path string) error {
	if err := f.check("Remove", path); err != nil {
		return err
	}
	return f.inner.Remove(path)
}

func (f *injectFS) MkdirAll(path string, mode os.FileMode) error {
	if err := f.check("MkdirAll", path); err != nil {
		return err
	}
	return f.inner.MkdirAll(path, mode)
}

// failAtCall refuses the nth mutation (1-based) and lets every other call
// through — including the rollback's own writes, which must still succeed after
// the apply has failed.
func failAtCall(n int) func(op, path string) error {
	calls := 0
	return func(op, path string) error {
		calls++
		if calls == n {
			return fmt.Errorf("injected failure on %s(%s)", op, path)
		}
		return nil
	}
}

// recordCalls records the verb of every mutation without refusing any.
func recordCalls(ops *[]string) func(op, path string) error {
	return func(op, path string) error {
		*ops = append(*ops, op)
		return nil
	}
}

// ---------------------------------------------------------------------------
// World snapshots
// ---------------------------------------------------------------------------

// worldEntry is one filesystem object as a rollback assertion must compare it:
// kind, permissions, and either the exact bytes or the exact link text.
type worldEntry struct {
	kind string // "dir" | "file" | "symlink"
	mode fs.FileMode
	data string
	link string
}

// snapshotWorld records every object under root, following nothing. A rollback
// is proven by comparing two of these, so it must capture what a partial apply
// could disturb: content, mode, link text, and the mere existence of a
// directory or a leftover staging file.
func snapshotWorld(t *testing.T, root string) map[string]worldEntry {
	t.Helper()
	out := map[string]worldEntry{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			dest, err := os.Readlink(p)
			if err != nil {
				return err
			}
			out[rel] = worldEntry{kind: "symlink", link: dest}
		case info.IsDir():
			out[rel] = worldEntry{kind: "dir", mode: info.Mode().Perm()}
		default:
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[rel] = worldEntry{kind: "file", mode: info.Mode().Perm(), data: string(b)}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotWorld(%s): %v", root, err)
	}
	return out
}

func assertWorld(t *testing.T, want, got map[string]worldEntry, context string) {
	t.Helper()
	keys := map[string]bool{}
	for k := range want {
		keys[k] = true
	}
	for k := range got {
		keys[k] = true
	}
	var sorted []string
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		w, okW := want[k]
		g, okG := got[k]
		switch {
		case !okW:
			t.Errorf("%s: %s appeared: %+v", context, k, g)
		case !okG:
			t.Errorf("%s: %s vanished: %+v", context, k, w)
		case w != g:
			t.Errorf("%s: %s differs\n want %+v\n  got %+v", context, k, w, g)
		}
	}
}

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

const userRules = "# Rules\n\nuser prose\n"

type fixture struct {
	roots   UserRoots
	targets string // every plan target lives under here; the data root does not
	source  string // the immutable-tree stand-in a skill link points at
	plan    []Inspection
}

// newFixture builds a plan exercising every step shape the engine supports:
// a created file, an updated file, a created symlink whose parent directory
// does not exist yet, a managed block inserted into a file docket did not
// author, a managed block replaced in a file that already carries one, and a
// no-op that must never become a step.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()

	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", home, err)
	}
	roots, err := ResolveRoots(
		func() (string, error) { return home, nil },
		func(string) string { return "" },
	)
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}

	targets := filepath.Join(base, "targets")
	source := filepath.Join(targets, "source", "skills", "docket-build")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", source, err)
	}
	writeFileOrDie(t, filepath.Join(source, "SKILL.md"), "skill\n")
	writeFileOrDie(t, filepath.Join(targets, "agents", "old-agent.md"), "old\n")
	writeFileOrDie(t, filepath.Join(targets, "agents", "stable.md"), "stable\n")
	writeFileOrDie(t, filepath.Join(targets, "dispatch", "rules.md"), userRules)
	writeFileOrDie(t, filepath.Join(targets, "dispatch", "managed.md"), managedFile("old interior\n"))

	f := &fixture{roots: roots, targets: targets, source: source}
	f.plan = []Inspection{
		{
			Target: Target{
				Path: filepath.Join(targets, "agents", "new-agent.md"), Kind: KindFile,
				Content: []byte("new agent\n"), Role: "agent",
			},
			Disposition: DispositionCreate,
		},
		{
			Target: Target{
				Path: filepath.Join(targets, "agents", "old-agent.md"), Kind: KindFile,
				Content: []byte("fresh\n"), Role: "agent",
			},
			Disposition: DispositionUpdate,
		},
		{
			Target: Target{
				Path: filepath.Join(targets, "skills", "docket-build"), Kind: KindSymlink,
				LinkTarget: source, Role: "skill",
			},
			Disposition: DispositionCreate,
		},
		{
			Target: Target{
				Path: filepath.Join(targets, "dispatch", "rules.md"), Kind: KindManagedBlock,
				BlockName: "dispatch", Annotation: "managed by docket",
				Content: []byte("line one\nline two\n"), Role: "dispatch",
			},
			Disposition: DispositionUpdate,
		},
		{
			Target: Target{
				Path: filepath.Join(targets, "dispatch", "managed.md"), Kind: KindManagedBlock,
				BlockName: "dispatch", Content: []byte("new interior\n"), Role: "dispatch",
			},
			Disposition: DispositionUpdate,
		},
		{
			Target: Target{
				Path: filepath.Join(targets, "agents", "stable.md"), Kind: KindFile,
				Content: []byte("stable\n"), Role: "agent",
			},
			Disposition: DispositionNoop,
		},
	}
	return f
}

func (f *fixture) path(parts ...string) string {
	return filepath.Join(append([]string{f.targets}, parts...)...)
}

func readOrDie(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(b)
}

// assertNoStaging fails when any staging file the engine writes beside a
// destination is still there. A transaction that finishes — either way — owes
// the user's directories no litter.
func assertNoStaging(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(d.Name(), "docket-install") || strings.HasSuffix(d.Name(), ".tmp") {
			t.Errorf("staging file left behind: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
}

func journalCount(t *testing.T, roots UserRoots) int {
	t.Helper()
	entries, err := os.ReadDir(roots.TransactionsDir())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0
		}
		t.Fatalf("ReadDir(%s): %v", roots.TransactionsDir(), err)
	}
	return len(entries)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTxnApplyCreatesAndUpdates(t *testing.T) {
	f := newFixture(t)

	txn, err := BeginTxn(RealFS{}, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := readOrDie(t, f.path("agents", "new-agent.md")); got != "new agent\n" {
		t.Errorf("created file = %q, want %q", got, "new agent\n")
	}
	if got := readOrDie(t, f.path("agents", "old-agent.md")); got != "fresh\n" {
		t.Errorf("updated file = %q, want %q", got, "fresh\n")
	}
	if got := readOrDie(t, f.path("agents", "stable.md")); got != "stable\n" {
		t.Errorf("no-op target = %q, want it untouched", got)
	}

	link := f.path("skills", "docket-build")
	dest, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink(%s): %v", link, err)
	}
	if dest != f.source {
		t.Errorf("link destination = %q, want %q", dest, f.source)
	}

	// The whole file is compared, not just the block: every byte the user wrote
	// around a managed block is theirs, and an install that rewrites one of them
	// is a defect however correct the block is.
	wantRules := "<!-- docket:dispatch:start (managed by docket) -->\n" +
		"line one\nline two\n" +
		"<!-- docket:dispatch:end -->\n" + userRules
	if got := readOrDie(t, f.path("dispatch", "rules.md")); got != wantRules {
		t.Errorf("inserted block file =\n%q\nwant\n%q", got, wantRules)
	}
	wantManaged := managedFile("new interior\n")
	if got := readOrDie(t, f.path("dispatch", "managed.md")); got != wantManaged {
		t.Errorf("replaced block file =\n%q\nwant\n%q", got, wantManaged)
	}

	assertNoStaging(t, f.targets)

	// An applied but unpublished transaction still owns a journal: nothing is
	// finished until Commit says so.
	if n := journalCount(t, f.roots); n != 1 {
		t.Errorf("journal count after Apply = %d, want 1", n)
	}
}

func TestTxnApplyFailureRollsBack(t *testing.T) {
	// The clean run tells the table how many mutations an apply performs, so
	// every one of them gets an injection rather than a hand-guessed few.
	var ops []string
	func() {
		f := newFixture(t)
		ifs := &injectFS{inner: RealFS{}}
		txn, err := BeginTxn(ifs, f.roots, f.plan)
		if err != nil {
			t.Fatalf("BeginTxn: %v", err)
		}
		ifs.fail = recordCalls(&ops) // armed after Begin: only the apply is indexed
		if err := txn.Apply(); err != nil {
			t.Fatalf("clean Apply: %v", err)
		}
	}()
	if len(ops) == 0 {
		t.Fatal("a clean apply performed no mutations; the table would be vacuous")
	}

	for n := 1; n <= len(ops); n++ {
		t.Run(fmt.Sprintf("fail-at-%02d-%s", n, ops[n-1]), func(t *testing.T) {
			f := newFixture(t)
			before := snapshotWorld(t, f.targets)

			ifs := &injectFS{inner: RealFS{}}
			txn, err := BeginTxn(ifs, f.roots, f.plan)
			if err != nil {
				t.Fatalf("BeginTxn: %v", err)
			}
			ifs.fail = failAtCall(n)

			err = txn.Apply()
			if err == nil {
				t.Fatal("Apply succeeded despite an injected filesystem failure")
			}
			if !errors.Is(err, ErrApplyFailed) {
				t.Errorf("Apply error = %v, want it to wrap ErrApplyFailed", err)
			}

			assertWorld(t, before, snapshotWorld(t, f.targets), "after rollback")
			assertNoStaging(t, f.targets)

			// A completed rollback leaves nothing for a later run to recover.
			if _, found, err := DetectRecovery(f.roots); err != nil || found {
				t.Errorf("DetectRecovery after rollback = (found %v, err %v), want (false, nil)", found, err)
			}
		})
	}
}

func TestTxnNoTornFile(t *testing.T) {
	f := newFixture(t)
	dest := f.path("agents", "old-agent.md")
	before := snapshotWorld(t, f.targets)

	ifs := &injectFS{inner: RealFS{}}
	txn, err := BeginTxn(ifs, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	// Fail exactly between the staging write and the rename: the moment where a
	// non-atomic writer would have already truncated the user's file.
	fired := false
	ifs.fail = func(op, path string) error {
		if op == "Rename" && path == dest && !fired {
			fired = true
			return errors.New("injected rename failure")
		}
		return nil
	}

	// applySteps, not Apply: the rollback would restore the pre-image and hide a
	// torn write behind it. Atomicity has to hold on its own, before anything
	// repairs anything.
	if err := txn.applySteps(); err == nil {
		t.Fatal("applySteps succeeded despite an injected rename failure")
	}
	if !fired {
		t.Fatal("the injected rename was never reached; the test proves nothing")
	}
	if got := readOrDie(t, dest); got != "old\n" {
		t.Errorf("interrupted update left %q, want the complete old bytes %q", got, "old\n")
	}
	// The new content exists — it is simply not at the destination yet. Without
	// this the test would also pass on an engine that never wrote anything.
	staged := stagedSiblings(t, filepath.Dir(dest))
	if len(staged) != 1 {
		t.Fatalf("staging files beside %s = %v, want exactly the interrupted one", dest, staged)
	}
	if got := readOrDie(t, staged[0]); got != "fresh\n" {
		t.Errorf("staged content = %q, want the new bytes %q", got, "fresh\n")
	}

	// The transaction is still owed a rollback, and it must leave nothing behind.
	if err := txn.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after a failed rename")
	assertNoStaging(t, f.targets)
}

// stagedSiblings lists the engine's staging files in dir.
func stagedSiblings(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if strings.Contains(e.Name(), "docket-install") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

func TestRecoveryAfterInterrupt(t *testing.T) {
	f := newFixture(t)
	before := snapshotWorld(t, f.targets)

	ifs := &injectFS{inner: RealFS{}}
	txn, err := BeginTxn(ifs, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	// applySteps is the engine without its rollback: calling it directly is how
	// a test kills the process mid-transaction, leaving the journal behind
	// exactly as an interrupted run would.
	ifs.fail = failAtCall(5)
	if err := txn.applySteps(); err == nil {
		t.Fatal("applySteps succeeded despite an injected failure")
	}

	interrupted := snapshotWorld(t, f.targets)
	if len(interrupted) == len(before) && worldsEqual(before, interrupted) {
		t.Fatal("nothing was changed before the interruption; recovery would be vacuous")
	}

	// A fresh process: it knows only the roots.
	id, found, err := DetectRecovery(f.roots)
	if err != nil {
		t.Fatalf("DetectRecovery: %v", err)
	}
	if !found {
		t.Fatal("DetectRecovery found no journal after an interrupted transaction")
	}
	if err := Recover(RealFS{}, f.roots, id); err != nil {
		t.Fatalf("Recover(%s): %v", id, err)
	}

	assertWorld(t, before, snapshotWorld(t, f.targets), "after recovery")
	assertNoStaging(t, f.targets)
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery after Recover = (found %v, err %v), want (false, nil)", found, err)
	}
}

func worldsEqual(a, b map[string]worldEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

func TestCommitRemovesJournal(t *testing.T) {
	f := newFixture(t)

	txn, err := BeginTxn(RealFS{}, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var records []TargetRecord
	for _, insp := range f.plan {
		rec, err := RecordFor(insp.Target)
		if err != nil {
			t.Fatalf("RecordFor(%s): %v", insp.Target.Path, err)
		}
		records = append(records, rec)
	}
	state := &State{
		FormatVersion:  StateFormatVersion,
		ProductVersion: "0.1.0-dev",
		AssetProtocol:  1,
		AssetSetID:     "sha256:abc",
		Mode:           ModeRelease,
		Harnesses:      []string{"claude"},
		AgentDigest:    digestOf("agents"),
		Targets:        records,
	}
	if err := txn.Commit(f.roots.StatePath(), state); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	loaded, err := LoadState(f.roots.StatePath())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded == nil || len(loaded.Targets) != len(records) {
		t.Fatalf("published state = %+v, want %d targets", loaded, len(records))
	}
	if n := journalCount(t, f.roots); n != 0 {
		t.Errorf("journal count after Commit = %d, want 0", n)
	}
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery after Commit = (found %v, err %v), want (false, nil)", found, err)
	}
	assertNoStaging(t, f.targets)
}

func TestDetectRecoveryClean(t *testing.T) {
	f := newFixture(t)

	// Nothing has ever run: the transactions directory does not even exist.
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery on a fresh root = (found %v, err %v), want (false, nil)", found, err)
	}

	// A completed transaction leaves the same clean answer behind.
	txn, err := BeginTxn(RealFS{}, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, found, err := DetectRecovery(f.roots); err != nil || !found {
		t.Errorf("DetectRecovery before Commit = (found %v, err %v), want (true, nil)", found, err)
	}
	if err := txn.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery after Rollback = (found %v, err %v), want (false, nil)", found, err)
	}
}

func TestBeginTxnRefusesConflictedPlan(t *testing.T) {
	f := newFixture(t)
	before := snapshotWorld(t, f.targets)

	plan := append([]Inspection(nil), f.plan...)
	plan = append(plan, Inspection{
		Target: Target{
			Path: f.path("agents", "theirs.md"), Kind: KindFile,
			Content: []byte("ours\n"), Role: "agent",
		},
		Disposition: DispositionConflict,
		Reason:      ReasonOwnershipConflict,
	})

	if _, err := BeginTxn(RealFS{}, f.roots, plan); err == nil {
		t.Fatal("BeginTxn accepted a plan carrying a conflict")
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after a refused begin")
	if n := journalCount(t, f.roots); n != 0 {
		t.Errorf("journal count after a refused begin = %d, want 0", n)
	}
}

func TestBeginTxnRefusesDuplicateTarget(t *testing.T) {
	f := newFixture(t)
	dup := Inspection{
		Target: Target{
			Path: f.path("agents", "new-agent.md"), Kind: KindFile,
			Content: []byte("another writer\n"), Role: "agent",
		},
		Disposition: DispositionCreate,
	}
	plan := append(append([]Inspection(nil), f.plan...), dup)
	if _, err := BeginTxn(RealFS{}, f.roots, plan); err == nil {
		t.Fatal("BeginTxn accepted two steps writing the same file")
	}
	if n := journalCount(t, f.roots); n != 0 {
		t.Errorf("journal count after a refused begin = %d, want 0", n)
	}
}

func TestBeginTxnLeavesNoHalfJournal(t *testing.T) {
	f := newFixture(t)
	// A directory where a file belongs has no rollback material, so the capture
	// fails after the journal directory already exists — the one path where a
	// half-built journal could be left for a later run to "recover".
	dir := f.path("agents", "a-directory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", dir, err)
	}
	plan := append([]Inspection(nil), f.plan...)
	plan = append(plan, Inspection{
		Target: Target{
			Path: dir, Kind: KindFile, Content: []byte("ours\n"), Role: "agent",
		},
		Disposition: DispositionUpdate,
	})

	if _, err := BeginTxn(RealFS{}, f.roots, plan); err == nil {
		t.Fatal("BeginTxn accepted a destination it cannot restore")
	}
	if n := journalCount(t, f.roots); n != 0 {
		t.Errorf("journal count after a failed begin = %d, want 0", n)
	}
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery after a failed begin = (found %v, err %v), want (false, nil)", found, err)
	}
}

func TestTxnRefusesManagedBlockOnSymlink(t *testing.T) {
	f := newFixture(t)
	// The user keeps this dispatch file in a dotfiles checkout and links it into
	// place. Rewriting the block by rename would replace the link with a plain
	// file; the transaction must refuse and put everything back instead.
	real := f.path("dotfiles", "rules.md")
	writeFileOrDie(t, real, userRules)
	link := f.path("linked", "rules.md")
	symlinkOrDie(t, real, link)
	before := snapshotWorld(t, f.targets)

	plan := []Inspection{{
		Target: Target{
			Path: link, Kind: KindManagedBlock, BlockName: "dispatch",
			Annotation: "managed by docket", Content: []byte("line one\n"), Role: "dispatch",
		},
		Disposition: DispositionUpdate,
	}}
	txn, err := BeginTxn(RealFS{}, f.roots, plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	if err := txn.Apply(); err == nil {
		t.Fatal("Apply rewrote a managed block through a symlink")
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after refusing a symlinked block")
}

func TestTxnNoopPlanChangesNothing(t *testing.T) {
	f := newFixture(t)
	before := snapshotWorld(t, f.targets)

	noop := []Inspection{f.plan[len(f.plan)-1]}
	if noop[0].Disposition != DispositionNoop {
		t.Fatalf("fixture drift: last inspection is %q, want a no-op", noop[0].Disposition)
	}
	ifs := &injectFS{inner: RealFS{}}
	txn, err := BeginTxn(ifs, f.roots, noop)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	var ops []string
	ifs.fail = recordCalls(&ops)
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("a no-op plan performed mutations: %v", ops)
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after a no-op plan")
}

func TestRecoverRejectsUnsafeID(t *testing.T) {
	f := newFixture(t)
	for _, id := range []string{"", ".", "..", "../escape", "nested/id", string(filepath.Separator) + "abs"} {
		if err := Recover(RealFS{}, f.roots, id); err == nil {
			t.Errorf("Recover accepted unsafe transaction id %q", id)
		}
	}
}

func TestRecoverUnknownID(t *testing.T) {
	f := newFixture(t)
	if err := Recover(RealFS{}, f.roots, "20260101T000000Z-deadbeef"); err == nil {
		t.Error("Recover accepted a transaction id with no journal")
	}
}

// ---------------------------------------------------------------------------
// Removals
// ---------------------------------------------------------------------------

// staleRecords puts two retired targets on disk and returns the records that
// retire them. A removal is the one step whose mistake cannot be re-derived
// from the plan — the bytes are simply gone — so both halves are proven below:
// that it deletes, and that an interrupted transaction puts it back.
func staleRecords(t *testing.T, f *fixture) []TargetRecord {
	t.Helper()
	writeFileOrDie(t, f.path("agents", "stale.md"), "stale\n")
	symlinkOrDie(t, f.source, f.path("skills", "docket-old"))
	return []TargetRecord{
		{Path: f.path("agents", "stale.md"), Kind: KindFile, Role: "agent", SHA256: digestOf("stale\n")},
		{Path: f.path("skills", "docket-old"), Kind: KindSymlink, Role: "skill", LinkTarget: f.source},
	}
}

func TestTxnAppliesRemovals(t *testing.T) {
	f := newFixture(t)
	removals := staleRecords(t, f)

	txn, err := BeginTxnWithRemovals(RealFS{}, f.roots, f.plan, removals)
	if err != nil {
		t.Fatalf("BeginTxnWithRemovals: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, rec := range removals {
		if _, err := os.Lstat(rec.Path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s survived its removal step: %v", rec.Path, err)
		}
	}
	// The rest of the plan still landed: a removal is a step among steps.
	if got := readOrDie(t, f.path("agents", "new-agent.md")); got != "new agent\n" {
		t.Errorf("new-agent.md = %q", got)
	}
	assertNoStaging(t, f.targets)
}

func TestTxnRemovalRollsBack(t *testing.T) {
	var ops []string
	func() {
		f := newFixture(t)
		removals := staleRecords(t, f)
		ifs := &injectFS{inner: RealFS{}}
		txn, err := BeginTxnWithRemovals(ifs, f.roots, f.plan, removals)
		if err != nil {
			t.Fatalf("BeginTxnWithRemovals: %v", err)
		}
		ifs.fail = recordCalls(&ops)
		if err := txn.Apply(); err != nil {
			t.Fatalf("clean Apply: %v", err)
		}
	}()
	if len(ops) == 0 {
		t.Fatal("a clean apply performed no mutations; the table would be vacuous")
	}

	for n := 1; n <= len(ops); n++ {
		t.Run(fmt.Sprintf("fail-at-%02d-%s", n, ops[n-1]), func(t *testing.T) {
			f := newFixture(t)
			removals := staleRecords(t, f)
			before := snapshotWorld(t, f.targets)

			ifs := &injectFS{inner: RealFS{}}
			txn, err := BeginTxnWithRemovals(ifs, f.roots, f.plan, removals)
			if err != nil {
				t.Fatalf("BeginTxnWithRemovals: %v", err)
			}
			ifs.fail = failAtCall(n)

			if err := txn.Apply(); err == nil {
				t.Fatal("Apply succeeded despite an injected filesystem failure")
			}
			assertWorld(t, before, snapshotWorld(t, f.targets), "after rollback")
			assertNoStaging(t, f.targets)
			if _, found, err := DetectRecovery(f.roots); err != nil || found {
				t.Errorf("DetectRecovery after rollback = (found %v, err %v), want (false, nil)", found, err)
			}
		})
	}
}

// managedBlockSpan is the exact marker-to-marker byte range managedFile lays
// down for the dispatch block, so a test can assert the prose that survives its
// removal without hand-transcribing it.
func managedBlockSpan(interior string) string {
	return "<!-- docket:dispatch:start (managed by docket) -->\n" +
		interior + "<!-- docket:dispatch:end -->\n"
}

func TestTxnManagedBlockRemovalStripsOnlyTheBlock(t *testing.T) {
	f := newFixture(t)
	// managed.md carries a dispatch block wrapped in the user's own prose, seeded
	// by newFixture. Retiring the block must leave every surrounding byte exactly
	// as it was and the file itself in place — it is a rewrite, not a deletion.
	path := f.path("dispatch", "managed.md")
	interior := "old interior\n"
	proseOnly := strings.Replace(managedFile(interior), managedBlockSpan(interior), "", 1)

	removal := TargetRecord{Path: path, Kind: KindManagedBlock, BlockName: "dispatch", Role: "dispatch"}
	txn, err := BeginTxnWithRemovals(RealFS{}, f.roots, nil, []TargetRecord{removal})
	if err != nil {
		t.Fatalf("BeginTxnWithRemovals: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := readOrDie(t, path); got != proseOnly {
		t.Errorf("after removal =\n%q\nwant\n%q", got, proseOnly)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("a managed-block removal must leave the file in place: %v", err)
	}
	// The block's own bytes are gone; the surrounding prose is present verbatim.
	if strings.Contains(readOrDie(t, path), "docket:dispatch") {
		t.Errorf("the dispatch markers survived the removal")
	}
	assertNoStaging(t, f.targets)
}

func TestTxnManagedBlockRemovalRollsBack(t *testing.T) {
	// The removal rewrites managed.md; a later create fails, so the rewrite must
	// be undone byte-for-byte. The clean run first tells the table how many
	// mutations the pair performs, so every one gets an injection.
	makePlan := func(f *fixture) ([]Inspection, []TargetRecord) {
		insp := []Inspection{{
			Target: Target{
				Path: f.path("dispatch", "zzz.md"), Kind: KindFile,
				Content: []byte("later\n"), Role: "agent",
			},
			Disposition: DispositionCreate,
		}}
		rem := []TargetRecord{{
			Path: f.path("dispatch", "managed.md"), Kind: KindManagedBlock,
			BlockName: "dispatch", Role: "dispatch",
		}}
		return insp, rem
	}

	var ops []string
	func() {
		f := newFixture(t)
		insp, rem := makePlan(f)
		ifs := &injectFS{inner: RealFS{}}
		txn, err := BeginTxnWithRemovals(ifs, f.roots, insp, rem)
		if err != nil {
			t.Fatalf("BeginTxnWithRemovals: %v", err)
		}
		ifs.fail = recordCalls(&ops)
		if err := txn.Apply(); err != nil {
			t.Fatalf("clean Apply: %v", err)
		}
	}()
	if len(ops) == 0 {
		t.Fatal("a clean apply performed no mutations; the table would be vacuous")
	}

	for n := 1; n <= len(ops); n++ {
		t.Run(fmt.Sprintf("fail-at-%02d-%s", n, ops[n-1]), func(t *testing.T) {
			f := newFixture(t)
			insp, rem := makePlan(f)
			before := snapshotWorld(t, f.targets)

			ifs := &injectFS{inner: RealFS{}}
			txn, err := BeginTxnWithRemovals(ifs, f.roots, insp, rem)
			if err != nil {
				t.Fatalf("BeginTxnWithRemovals: %v", err)
			}
			ifs.fail = failAtCall(n)

			if err := txn.Apply(); err == nil {
				t.Fatal("Apply succeeded despite an injected filesystem failure")
			}
			assertWorld(t, before, snapshotWorld(t, f.targets), "after rollback")
			assertNoStaging(t, f.targets)
			if _, found, err := DetectRecovery(f.roots); err != nil || found {
				t.Errorf("DetectRecovery after rollback = (found %v, err %v), want (false, nil)", found, err)
			}
		})
	}
}

func TestTxnRefusesRemovalOfAMalformedMarkerFile(t *testing.T) {
	f := newFixture(t)
	// A dangling start marker cannot be parsed, so the block cannot be located
	// and the removal must refuse rather than let an unbounded range consume to
	// EOF. The refusal surfaces at apply, before any byte of the file is
	// rewritten.
	path := f.path("dispatch", "broken.md")
	malformed := "# Notes\n\n<!-- docket:dispatch:start (managed by docket) -->\nbody with no end marker\n"
	writeFileOrDie(t, path, malformed)
	before := snapshotWorld(t, f.targets)

	removal := TargetRecord{Path: path, Kind: KindManagedBlock, BlockName: "dispatch", Role: "dispatch"}
	txn, err := BeginTxnWithRemovals(RealFS{}, f.roots, nil, []TargetRecord{removal})
	if err != nil {
		t.Fatalf("BeginTxnWithRemovals: %v", err)
	}
	if err := txn.Apply(); err == nil {
		t.Fatal("Apply retired a block from a file whose markers are malformed")
	}
	if got := readOrDie(t, path); got != malformed {
		t.Errorf("the malformed file was disturbed:\n%q", got)
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after refusing a malformed removal")
	assertNoStaging(t, f.targets)
}

func TestTxnRefusesManagedBlockRemovalWithoutABlockName(t *testing.T) {
	f := newFixture(t)
	// A managed-block removal is now a first-class step, but the record must name
	// the block to retire: a nameless one cannot be located, so it is refused at
	// planning before any journal exists.
	_, err := BeginTxnWithRemovals(RealFS{}, f.roots, nil, []TargetRecord{{
		Path: f.path("dispatch", "managed.md"), Kind: KindManagedBlock, Role: "dispatch",
	}})
	if !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("err = %v, want ErrInvalidTarget", err)
	}
	if readOrDie(t, f.path("dispatch", "managed.md")) != managedFile("old interior\n") {
		t.Errorf("the file was disturbed anyway")
	}
	if journalCount(t, f.roots) != 0 {
		t.Errorf("a refused plan left a journal behind")
	}
}

func TestTxnRefusesToRemoveAManagedBlockThroughASymlink(t *testing.T) {
	f := newFixture(t)
	// The user keeps the dispatch file in a dotfiles checkout and links it into
	// place. Rewriting the block by rename would replace the link with a plain
	// file, so a managed-block removal on a symlinked path must refuse and put
	// everything back — the same guarantee the update path gives.
	real := f.path("dotfiles", "managed.md")
	writeFileOrDie(t, real, managedFile("old interior\n"))
	link := f.path("linked", "managed.md")
	symlinkOrDie(t, real, link)
	before := snapshotWorld(t, f.targets)

	removal := TargetRecord{Path: link, Kind: KindManagedBlock, BlockName: "dispatch", Role: "dispatch"}
	txn, err := BeginTxnWithRemovals(RealFS{}, f.roots, nil, []TargetRecord{removal})
	if err != nil {
		t.Fatalf("BeginTxnWithRemovals: %v", err)
	}
	if err := txn.Apply(); err == nil {
		t.Fatal("Apply rewrote a managed block through a symlink")
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after refusing a symlinked removal")
	assertNoStaging(t, f.targets)
}

func TestTxnRefusesRemovalOfAWrittenDestination(t *testing.T) {
	f := newFixture(t)
	// One plan that both writes and deletes a path has no defined outcome.
	_, err := BeginTxnWithRemovals(RealFS{}, f.roots, f.plan, []TargetRecord{{
		Path: f.path("agents", "old-agent.md"), Kind: KindFile, Role: "agent", SHA256: digestOf("old\n"),
	}})
	if !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("err = %v, want ErrInvalidTarget", err)
	}
	if journalCount(t, f.roots) != 0 {
		t.Errorf("a refused plan left a journal behind")
	}
}

// TestRecoveryAtEveryInterruptPoint is the restart half of the rollback table:
// for every mutation an apply performs, a process that dies right there leaves
// a journal a FRESH process can find and roll back deterministically. One
// hand-picked interrupt point proves recovery works somewhere; the phase it
// happens to land on is not the phase a real interruption chooses.
func TestRecoveryAtEveryInterruptPoint(t *testing.T) {
	var ops []string
	func() {
		f := newFixture(t)
		ifs := &injectFS{inner: RealFS{}}
		txn, err := BeginTxn(ifs, f.roots, f.plan)
		if err != nil {
			t.Fatalf("BeginTxn: %v", err)
		}
		ifs.fail = recordCalls(&ops)
		if err := txn.Apply(); err != nil {
			t.Fatalf("clean Apply: %v", err)
		}
	}()
	if len(ops) == 0 {
		t.Fatal("a clean apply performed no mutations; the table would be vacuous")
	}

	for n := 1; n <= len(ops); n++ {
		t.Run(fmt.Sprintf("interrupt-at-%02d-%s", n, ops[n-1]), func(t *testing.T) {
			f := newFixture(t)
			before := snapshotWorld(t, f.targets)

			ifs := &injectFS{inner: RealFS{}}
			txn, err := BeginTxn(ifs, f.roots, f.plan)
			if err != nil {
				t.Fatalf("BeginTxn: %v", err)
			}
			// applySteps is the engine without its rollback: calling it
			// directly is how a test kills the process mid-transaction.
			ifs.fail = failAtCall(n)
			if err := txn.applySteps(); err == nil {
				t.Fatal("applySteps succeeded despite an injected failure")
			}

			// A fresh process knows only the roots.
			id, found, err := DetectRecovery(f.roots)
			if err != nil {
				t.Fatalf("DetectRecovery: %v", err)
			}
			if !found {
				t.Fatal("an interrupted transaction left no journal to recover")
			}
			if err := Recover(RealFS{}, f.roots, id); err != nil {
				t.Fatalf("Recover(%s): %v", id, err)
			}

			assertWorld(t, before, snapshotWorld(t, f.targets), "after recovery")
			assertNoStaging(t, f.targets)
			if _, found, err := DetectRecovery(f.roots); err != nil || found {
				t.Errorf("DetectRecovery after Recover = (found %v, err %v), want (false, nil)", found, err)
			}
			if got := journalCount(t, f.roots); got != 0 {
				t.Errorf("%d journals survived recovery", got)
			}

			// Recovery is deterministic: a second recovery pass over the same
			// roots is a no-op rather than a second, different world.
			assertWorld(t, before, snapshotWorld(t, f.targets), "after a repeated recovery sweep")
		})
	}
}

// ---------------------------------------------------------------------------
// Disposition and pre-image agreement
// ---------------------------------------------------------------------------
//
// A plan is decided against one copy of the world and applied to another: the
// inspection classifies a destination, the journal captures its pre-image, and
// only then does anything get written. Between those moments the destination can
// change under the plan. Every test below is the engine refusing to act on a
// decision the disk no longer supports — before the write, so nothing of the
// user's is overwritten, and without a rollback, so nothing of the user's is
// deleted either.

func TestBeginTxnRefusesCreateWhoseTargetAppeared(t *testing.T) {
	f := newFixture(t)
	// The plan's create target: absent when the inspection classified it, and
	// written by somebody else in the moment before the transaction opened. The
	// journal would record "absent" and the apply would rename straight over it.
	interloper := f.path("agents", "new-agent.md")
	writeFileOrDie(t, interloper, "not ours\n")
	before := snapshotWorld(t, f.targets)

	_, err := BeginTxn(RealFS{}, f.roots, f.plan)
	if !errors.Is(err, ErrPlanStale) {
		t.Fatalf("BeginTxn err = %v, want it to wrap ErrPlanStale", err)
	}
	// The operation layer keys on ErrPlanConflict to report an ownership dead
	// end; a stale plan is the same dead end and must reach the same answer.
	if !errors.Is(err, ErrPlanConflict) {
		t.Errorf("BeginTxn err = %v, want it to wrap ErrPlanConflict too", err)
	}
	if got := readOrDie(t, interloper); got != "not ours\n" {
		t.Errorf("the file that appeared = %q, want it untouched", got)
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after a refused begin")
	if n := journalCount(t, f.roots); n != 0 {
		t.Errorf("journal count after a refused begin = %d, want 0", n)
	}
}

func TestBeginTxnRefusesUpdateWhoseTargetVanished(t *testing.T) {
	f := newFixture(t)
	// The plan's update target: present and provably docket's when it was
	// inspected, deleted before its bytes could be captured. Silently demoting it
	// to a create would be this transaction deciding for a copy of the world
	// nobody inspected.
	vanished := f.path("agents", "old-agent.md")
	if err := os.Remove(vanished); err != nil {
		t.Fatalf("Remove(%s): %v", vanished, err)
	}
	before := snapshotWorld(t, f.targets)

	_, err := BeginTxn(RealFS{}, f.roots, f.plan)
	if !errors.Is(err, ErrPlanStale) {
		t.Fatalf("BeginTxn err = %v, want it to wrap ErrPlanStale", err)
	}
	if _, err := os.Lstat(vanished); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the refused begin re-created %s: %v", vanished, err)
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after a refused begin")
	if n := journalCount(t, f.roots); n != 0 {
		t.Errorf("journal count after a refused begin = %d, want 0", n)
	}
}

func TestApplyRefusesACreateDestinationThatAppeared(t *testing.T) {
	f := newFixture(t)
	txn, err := BeginTxn(RealFS{}, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	// The window this test is about: the journal is complete, one of its
	// pre-images says "absent", and only now does somebody else write there.
	interloper := f.path("agents", "new-agent.md")
	writeFileOrDie(t, interloper, "theirs\n")
	before := snapshotWorld(t, f.targets)

	err = txn.Apply()
	if !errors.Is(err, ErrPlanStale) {
		t.Fatalf("Apply err = %v, want it to wrap ErrPlanStale", err)
	}
	// Both halves of failing closed. The apply did not overwrite the file, AND no
	// rollback ran to delete it: a pre-image recorded absent restores by removing
	// whatever is at the path, so a transaction that refused and then rolled back
	// would destroy the very file it had just declined to overwrite.
	if got := readOrDie(t, interloper); got != "theirs\n" {
		t.Errorf("the file that appeared = %q, want it untouched", got)
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after a refused apply")
	assertNoStaging(t, f.targets)
	// Nothing was applied, so nothing is owed a recovery — and a journal left
	// behind would hand a later Recover exactly that deletion to perform.
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery after a refused apply = (found %v, err %v), want (false, nil)", found, err)
	}
	if n := journalCount(t, f.roots); n != 0 {
		t.Errorf("journal count after a refused apply = %d, want 0", n)
	}
}

func TestApplyRefusesAnUpdateDestinationThatVanished(t *testing.T) {
	f := newFixture(t)
	txn, err := BeginTxn(RealFS{}, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	// The user deletes a journaled destination while the transaction is open. Its
	// bytes are in the journal, so an apply could proceed and a rollback could put
	// the file back — but both would act on a world nobody inspected, so the
	// transaction refuses and restores nothing.
	vanished := f.path("dispatch", "managed.md")
	if err := os.Remove(vanished); err != nil {
		t.Fatalf("Remove(%s): %v", vanished, err)
	}
	before := snapshotWorld(t, f.targets)

	err = txn.Apply()
	if !errors.Is(err, ErrPlanStale) {
		t.Fatalf("Apply err = %v, want it to wrap ErrPlanStale", err)
	}
	if _, err := os.Lstat(vanished); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the refused apply resurrected %s: %v", vanished, err)
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after a refused apply")
	assertNoStaging(t, f.targets)
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery after a refused apply = (found %v, err %v), want (false, nil)", found, err)
	}
}

func TestTxnRemovalsAreExemptFromDispositionAgreement(t *testing.T) {
	f := newFixture(t)
	removals := staleRecords(t, f)
	// A removal carries no inspection: its licence is the prior ownership record,
	// not a disposition. A retired target that has already vanished — before the
	// journal, or while it is open — must not block the upgrade that retires it.
	if err := os.Remove(removals[0].Path); err != nil {
		t.Fatalf("Remove(%s): %v", removals[0].Path, err)
	}

	txn, err := BeginTxnWithRemovals(RealFS{}, f.roots, f.plan, removals)
	if err != nil {
		t.Fatalf("BeginTxnWithRemovals refused a removal whose target had vanished: %v", err)
	}
	if err := os.Remove(removals[1].Path); err != nil {
		t.Fatalf("Remove(%s): %v", removals[1].Path, err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply refused a removal whose target vanished mid-transaction: %v", err)
	}
	for _, rec := range removals {
		if _, err := os.Lstat(rec.Path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s survived its removal step: %v", rec.Path, err)
		}
	}
	// The rest of the plan still landed: the exemption is for removals alone.
	if got := readOrDie(t, f.path("agents", "new-agent.md")); got != "new agent\n" {
		t.Errorf("new-agent.md = %q, want the plan applied around the removals", got)
	}
}

// ---------------------------------------------------------------------------
// Commit documents: the machine state and the repository record as one publish
// ---------------------------------------------------------------------------
//
// An installation that reconciles a repository publishes TWO ownership
// documents, and two renames cannot be one atomic act. The journal buys the
// missing atomicity for the pair exactly as it does for the targets: the
// documents' pre-images are captured before the first is published, so a
// synchronous failure between them rolls both — and every applied target — back,
// and an interrupted commit is recovered to the same side by the next run.

func assertDocMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s): %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s mode = %v, want %v", path, got, want)
	}
}

func TestCommitDocsPublishesBothDocuments(t *testing.T) {
	f := newFixture(t)
	docA := f.path("docs", "machine.json")
	docB := f.path("docs", "repo.json")

	txn, err := BeginTxn(RealFS{}, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	docs := []StateDoc{{Path: docA, Bytes: []byte("machine-state\n")}, {Path: docB, Bytes: []byte("repo-record\n")}}
	if err := txn.CommitDocs(docs); err != nil {
		t.Fatalf("CommitDocs: %v", err)
	}

	if got := readOrDie(t, docA); got != "machine-state\n" {
		t.Errorf("first document = %q", got)
	}
	if got := readOrDie(t, docB); got != "repo-record\n" {
		t.Errorf("second document = %q", got)
	}
	// A document is docket's own private record, never the user's to widen.
	assertDocMode(t, docA, 0o600)
	assertDocMode(t, docB, 0o600)
	// The machine plan still landed: the documents are the commit, not a
	// replacement for the apply.
	if got := readOrDie(t, f.path("agents", "new-agent.md")); got != "new agent\n" {
		t.Errorf("new-agent.md = %q", got)
	}
	// A published commit owns no journal and leaves nothing to recover.
	if n := journalCount(t, f.roots); n != 0 {
		t.Errorf("journal count after CommitDocs = %d, want 0", n)
	}
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery after CommitDocs = (found %v, err %v), want (false, nil)", found, err)
	}
	assertNoStaging(t, f.targets)
}

func TestCommitDocsRollsBackBothDocumentsOnPublishFailure(t *testing.T) {
	f := newFixture(t)
	docA := f.path("docs", "machine.json")
	docB := f.path("docs", "repo.json")
	// Prior bytes so the rollback exercises restore-from-backup, not merely
	// restore-by-removal.
	writeFileOrDie(t, docA, "old machine\n")
	writeFileOrDie(t, docB, "old repo\n")
	before := snapshotWorld(t, f.targets)

	ifs := &injectFS{inner: RealFS{}}
	txn, err := BeginTxn(ifs, f.roots, f.plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("clean Apply: %v", err)
	}
	// Fail the publish of the SECOND document, once — the rollback that follows
	// renames to the same path to restore it, and must be allowed through.
	fired := false
	ifs.fail = func(op, path string) error {
		if op == "Rename" && path == filepath.Clean(docB) && !fired {
			fired = true
			return errors.New("injected rename failure on the second document")
		}
		return nil
	}
	docs := []StateDoc{{Path: docA, Bytes: []byte("new machine\n")}, {Path: docB, Bytes: []byte("new repo\n")}}
	if err := txn.CommitDocs(docs); err == nil {
		t.Fatal("CommitDocs succeeded despite an injected failure on the second document")
	}
	if !fired {
		t.Fatal("the injected publish failure was never reached; the test proves nothing")
	}
	// The first document was already published, and the rollback put it back: a
	// commit that cannot publish every document publishes none, so both documents
	// read as their pre-images — and the whole machine world with them.
	if got := readOrDie(t, docA); got != "old machine\n" {
		t.Errorf("first document after rollback = %q, want the pre-image", got)
	}
	if got := readOrDie(t, docB); got != "old repo\n" {
		t.Errorf("second document after rollback = %q, want the pre-image", got)
	}
	assertWorld(t, before, snapshotWorld(t, f.targets), "after a failed second-document publish")
	assertNoStaging(t, f.targets)
	if _, found, err := DetectRecovery(f.roots); err != nil || found {
		t.Errorf("DetectRecovery after a rolled-back commit = (found %v, err %v), want (false, nil)", found, err)
	}
}

// TestRecoveryAtEveryCommitInterruptPoint is the document half of the recovery
// table: for every mutation the commit performs, a process that dies right there
// leaves a journal a FRESH process finds and rolls back — restoring BOTH
// documents and every applied target to the same side of the operation.
func TestRecoveryAtEveryCommitInterruptPoint(t *testing.T) {
	docsFor := func(f *fixture) []StateDoc {
		return []StateDoc{
			{Path: f.path("docs", "machine.json"), Bytes: []byte("new machine\n")},
			{Path: f.path("docs", "repo.json"), Bytes: []byte("new repo\n")},
		}
	}
	seedDocs := func(f *fixture) {
		writeFileOrDie(t, f.path("docs", "machine.json"), "old machine\n")
		writeFileOrDie(t, f.path("docs", "repo.json"), "old repo\n")
	}

	var ops []string
	func() {
		f := newFixture(t)
		seedDocs(f)
		ifs := &injectFS{inner: RealFS{}}
		txn, err := BeginTxn(ifs, f.roots, f.plan)
		if err != nil {
			t.Fatalf("BeginTxn: %v", err)
		}
		if err := txn.Apply(); err != nil {
			t.Fatalf("clean Apply: %v", err)
		}
		ifs.fail = recordCalls(&ops) // armed after apply: only the commit is indexed
		if err := txn.commitDocsApply(docsFor(f)); err != nil {
			t.Fatalf("clean commitDocsApply: %v", err)
		}
	}()
	if len(ops) == 0 {
		t.Fatal("a clean commit performed no mutations; the table would be vacuous")
	}

	for n := 1; n <= len(ops); n++ {
		t.Run(fmt.Sprintf("interrupt-at-%02d-%s", n, ops[n-1]), func(t *testing.T) {
			f := newFixture(t)
			seedDocs(f)
			before := snapshotWorld(t, f.targets)

			ifs := &injectFS{inner: RealFS{}}
			txn, err := BeginTxn(ifs, f.roots, f.plan)
			if err != nil {
				t.Fatalf("BeginTxn: %v", err)
			}
			if err := txn.Apply(); err != nil {
				t.Fatalf("clean Apply: %v", err)
			}
			// commitDocsApply is CommitDocs without its rollback: calling it
			// directly is how a test kills the process mid-commit, leaving the
			// journal behind exactly as an interrupted run would.
			ifs.fail = failAtCall(n)
			if err := txn.commitDocsApply(docsFor(f)); err == nil {
				t.Fatal("commitDocsApply succeeded despite an injected failure")
			}

			// A fresh process knows only the roots.
			id, found, err := DetectRecovery(f.roots)
			if err != nil {
				t.Fatalf("DetectRecovery: %v", err)
			}
			if !found {
				t.Fatal("an interrupted commit left no journal to recover")
			}
			if err := Recover(RealFS{}, f.roots, id); err != nil {
				t.Fatalf("Recover(%s): %v", id, err)
			}

			assertWorld(t, before, snapshotWorld(t, f.targets), "after recovery")
			assertNoStaging(t, f.targets)
			if _, found, err := DetectRecovery(f.roots); err != nil || found {
				t.Errorf("DetectRecovery after Recover = (found %v, err %v), want (false, nil)", found, err)
			}
			if got := journalCount(t, f.roots); got != 0 {
				t.Errorf("%d journals survived recovery", got)
			}
		})
	}
}

// TestCaptureRefusesAnUncheckableDisposition covers the branch planSteps makes
// unreachable: a write step whose disposition is neither a create nor an update
// cannot be checked against the disk at all, and the whole point of the capture
// is that no such step gets waved through. It is asserted at the function rather
// than through BeginTxn because planSteps refuses these one layer earlier —
// which is exactly why the backstop would otherwise never be observed failing.
func TestCaptureRefusesAnUncheckableDisposition(t *testing.T) {
	for _, disposition := range []Disposition{"", DispositionNoop, DispositionConflict, "invented"} {
		step := &journalStep{Path: "/nowhere/agent.md", Kind: KindFile, Disposition: disposition}
		if err := agreesWithDisposition(step, false); !errors.Is(err, ErrInvalidTarget) {
			t.Errorf("agreesWithDisposition(%q, absent) = %v, want ErrInvalidTarget", disposition, err)
		}
		if err := agreesWithDisposition(step, true); !errors.Is(err, ErrInvalidTarget) {
			t.Errorf("agreesWithDisposition(%q, present) = %v, want ErrInvalidTarget", disposition, err)
		}
	}
}

// The mode handed to WriteFile is a ceiling, not a promise: file creation
// filters it through the process umask, so under a restrictive runner (a
// detached gate, a hardened shell with umask 077) a trusted creation mode would
// install a 0700 binary and 0600 default files. Target modes are policy, so the
// engine must enforce them with an explicit chmod. This test runs a whole
// transaction — and a rollback — under umask 077 and demands exact modes.
//
// syscall.Umask is process-wide state, so this test must never call
// t.Parallel(); the deferred restore is what keeps it from leaking into the
// rest of the run.
func TestTxnModesAreUmaskProof(t *testing.T) {
	oldMask := syscall.Umask(0o077)
	defer syscall.Umask(oldMask)

	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", home, err)
	}
	roots, err := ResolveRoots(
		func() (string, error) { return home, nil },
		func(string) string { return "" },
	)
	if err != nil {
		t.Fatalf("ResolveRoots: %v", err)
	}

	dir := filepath.Join(base, "targets")
	existing := filepath.Join(dir, "existing.md")
	writeFileOrDie(t, existing, "old\n")
	// writeFileOrDie's own creation mode is umask-filtered too, so the
	// pre-condition this test rests on is pinned explicitly.
	if err := os.Chmod(existing, 0o644); err != nil {
		t.Fatalf("Chmod(%s): %v", existing, err)
	}

	binary := filepath.Join(dir, "bin", "docket")
	plain := filepath.Join(dir, "plain.md")
	plan := []Inspection{
		{
			Target:      Target{Path: binary, Kind: KindFile, Content: []byte("elf\n"), Mode: 0o755, Role: "binary"},
			Disposition: DispositionCreate,
		},
		{
			Target:      Target{Path: plain, Kind: KindFile, Content: []byte("plain\n"), Role: "agent"},
			Disposition: DispositionCreate,
		},
		{
			Target:      Target{Path: existing, Kind: KindFile, Content: []byte("fresh\n"), Role: "agent"},
			Disposition: DispositionUpdate,
		},
	}

	txn, err := BeginTxn(RealFS{}, roots, plan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertMode := func(path string, want fs.FileMode, context string) {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s: Stat(%s): %v", context, path, err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s: %s mode = %v, want %v", context, path, got, want)
		}
	}
	assertMode(binary, 0o755, "apply under umask 077")
	assertMode(plain, 0o644, "apply under umask 077")
	assertMode(existing, 0o644, "apply under umask 077")

	// Rollback restores a pre-image through the same staged write, so a
	// restored file's mode must be exact under the same umask. The plan updates
	// existing.md first (paths apply in sorted order), then fails publishing
	// zz.md, which rolls the update back.
	failing := filepath.Join(dir, "zz.md")
	rollbackPlan := []Inspection{
		{
			Target:      Target{Path: existing, Kind: KindFile, Content: []byte("newer\n"), Role: "agent"},
			Disposition: DispositionUpdate,
		},
		{
			Target:      Target{Path: failing, Kind: KindFile, Content: []byte("boom\n"), Role: "agent"},
			Disposition: DispositionCreate,
		},
	}
	ifs := &injectFS{inner: RealFS{}, fail: func(op, path string) error {
		if op == "Rename" && path == failing {
			return fmt.Errorf("injected failure on %s(%s)", op, path)
		}
		return nil
	}}
	txn2, err := BeginTxn(ifs, roots, rollbackPlan)
	if err != nil {
		t.Fatalf("BeginTxn: %v", err)
	}
	if err := txn2.Apply(); err == nil {
		t.Fatal("Apply succeeded despite an injected failure")
	}
	if got := readOrDie(t, existing); got != "fresh\n" {
		t.Errorf("rollback restored %q, want %q", got, "fresh\n")
	}
	assertMode(existing, 0o644, "rollback under umask 077")
}
