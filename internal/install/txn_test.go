package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
