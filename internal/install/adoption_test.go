package install_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/install"
)

// This file is change 0322 Task B6: an end-to-end filesystem/adoption state
// matrix over the WIRED install path (B5). Each sub-test seeds a temp HOME in
// one starting state and drives a real release install through the four real
// harness adapters and the frozen legacy reproducer, asserting the observable
// contract: adoption is byte-exact, a conflict changes nothing, the transaction
// engine replaces atomically and rolls back fully, unrelated bytes are
// preserved, and state/install.json records Go ownership of every landed target.
//
// It reuses the corpus-seeding helpers from legacy_adoption_test.go
// (seedFullyPinnedLegacyTree, legacyAgentDest, legacyOptions) and the world
// fixture from service_test.go rather than re-inventing a legacy HOME.

// recordedByPath indexes a published state's target records by cleaned path, so
// a sub-test can assert Go ownership (a non-empty Harness) of an adopted or
// created target.
func recordedByPath(s *install.State) map[string]install.TargetRecord {
	out := map[string]install.TargetRecord{}
	if s == nil {
		return out
	}
	for _, rec := range s.Targets {
		out[filepath.Clean(rec.Path)] = rec
	}
	return out
}

// assertNoStagingLitter fails when a transaction left a staging temp file beside
// any destination under root. The engine writes every content target through a
// same-directory temp file and publishes it by rename; a finished transaction —
// committed or rolled back — owes the user's directories no `.docket-install`
// or `.tmp` litter. This is the observable tail of atomic replacement.
func assertNoStagingLitter(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(d.Name(), "docket-install") || strings.HasSuffix(d.Name(), ".tmp") {
			t.Errorf("staging file left behind (non-atomic tail): %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
}

// ---------------------------------------------------------------------------
// A failing FSOps for the rollback sub-test
// ---------------------------------------------------------------------------

// failingFS wraps a real FSOps and refuses one chosen mutation. It is the
// external-package analogue of txn_test.go's injectFS: rollback is the whole
// safety story of an installer that spans harness directories no single rename
// can cover, and a rollback nobody has watched happen is decoration.
type failingFS struct {
	inner install.FSOps
	fail  func(op, path string) error
}

func (f *failingFS) route(op, path string) error {
	if f.fail == nil {
		return nil
	}
	return f.fail(op, path)
}

func (f *failingFS) WriteFile(path string, data []byte, mode os.FileMode) error {
	if err := f.route("WriteFile", path); err != nil {
		return err
	}
	return f.inner.WriteFile(path, data, mode)
}

func (f *failingFS) Chmod(path string, mode os.FileMode) error {
	if err := f.route("Chmod", path); err != nil {
		return err
	}
	return f.inner.Chmod(path, mode)
}

// Rename is keyed on the destination — the path whose contents an interrupted
// rename decides, and the one a targeted injection needs to name.
func (f *failingFS) Rename(old, newp string) error {
	if err := f.route("Rename", newp); err != nil {
		return err
	}
	return f.inner.Rename(old, newp)
}

func (f *failingFS) Symlink(target, path string) error {
	if err := f.route("Symlink", path); err != nil {
		return err
	}
	return f.inner.Symlink(target, path)
}

func (f *failingFS) Remove(path string) error {
	if err := f.route("Remove", path); err != nil {
		return err
	}
	return f.inner.Remove(path)
}

func (f *failingFS) MkdirAll(path string, mode os.FileMode) error {
	if err := f.route("MkdirAll", path); err != nil {
		return err
	}
	return f.inner.MkdirAll(path, mode)
}

// ---------------------------------------------------------------------------
// The state matrix
// ---------------------------------------------------------------------------

// TestAdoptionStateMatrix drives the wired install over homes in each starting
// state. One sub-test per state, plus the cross-cutting rollback proof.
func TestAdoptionStateMatrix(t *testing.T) {
	// clean — nothing installed. Every target is a create; the whole install
	// applies atomically and state records Go ownership of each landed target.
	t.Run("clean", func(t *testing.T) {
		w := newWorld(t, allHarnessDirs...)

		out := install.Install(legacyOptions(w))
		if out.Reason != "" || out.Err != nil {
			t.Fatalf("clean install refused: reason=%q err=%v", out.Reason, out.Err)
		}
		if !out.Applied {
			t.Fatalf("clean install applied nothing; actions=%v", actionPaths(out))
		}
		if len(conflictPaths(out)) != 0 {
			t.Fatalf("clean install reported conflicts: %v", conflictPaths(out))
		}

		state := loadState(t, w.roots)
		if state == nil {
			t.Fatal("no installed state was published")
		}
		// Every recorded harness target carries Go ownership; the binary is the
		// one record attributed to no harness.
		for _, rec := range state.Targets {
			if rec.Role != "binary" && rec.Harness == "" {
				t.Errorf("clean-created target %s recorded with no harness (Go ownership)", rec.Path)
			}
		}
		assertNoStagingLitter(t, w.home)
	})

	// exactly-legacy — every seeded target is exact v0.9.2 bytes. All adopt (no
	// conflict), each is atomically replaced with the current Go render, and
	// state records Go ownership of each.
	t.Run("exactly-legacy", func(t *testing.T) {
		w := newWorld(t, allHarnessDirs...)
		seeded := seedFullyPinnedLegacyTree(t, w)

		out := install.Install(legacyOptions(w))
		if out.Reason != "" || out.Err != nil {
			t.Fatalf("exact-legacy install refused: reason=%q err=%v conflicts=%v", out.Reason, out.Err, conflictPaths(out))
		}
		if !out.Applied {
			t.Fatalf("exact-legacy install applied nothing; actions=%v", actionPaths(out))
		}

		// Adoption is provable from what did NOT happen: a present file can never
		// be a create, so a seeded legacy path that is neither a conflict nor a
		// create was accepted as docket's own — either replaced with the current
		// render (update) or already identical to it (no-op). With the reproducer
		// left nil (pre-B5) each of these would be a foreign-file conflict and the
		// whole install would refuse, so a clean apply here IS the adoption proof.
		recorded := recordedByPath(state(t, w))
		updates := 0
		for _, p := range seeded {
			if hasAction(out, install.OpConflict, p) {
				t.Errorf("exact legacy file reported as a conflict, not adopted: %s", p)
			}
			if hasAction(out, install.OpCreate, p) {
				t.Errorf("a seeded (present) legacy file was created rather than adopted: %s", p)
			}
			if hasAction(out, install.OpUpdate, p) {
				updates++
			}
			rec, ok := recorded[filepath.Clean(p)]
			if !ok {
				t.Errorf("state records no target for adopted legacy path %s", p)
				continue
			}
			if rec.Harness == "" {
				t.Errorf("adopted target %s recorded with no harness (Go ownership)", p)
			}
		}
		// At least one seeded legacy file differs from the current render, so the
		// byte-exact-legacy takeover branch (adopt-as-update) is genuinely
		// exercised, not merely a tree that would have been a no-op anyway.
		if updates == 0 {
			t.Errorf("no seeded legacy file was adopted as an update; the takeover branch was never exercised")
		}
		assertNoStagingLitter(t, w.home)
	})

	// partially-legacy — some targets are exact legacy bytes, some are absent.
	// The legacy ones adopt (update); the absent ones create; nothing conflicts.
	t.Run("partially-legacy", func(t *testing.T) {
		w := newWorld(t, allHarnessDirs...)
		seeded := seedFullyPinnedLegacyTree(t, w)

		// Make one seeded target absent: it must fall back to a plain create,
		// while its exact-legacy siblings still adopt.
		absent := legacyAgentDest(w, "claude", "docket-status.md")
		if err := os.Remove(absent); err != nil {
			t.Fatalf("removing a seeded target to make it absent: %v", err)
		}

		out := install.Install(legacyOptions(w))
		if out.Reason != "" || out.Err != nil {
			t.Fatalf("partial-legacy install refused: reason=%q err=%v conflicts=%v", out.Reason, out.Err, conflictPaths(out))
		}
		if !out.Applied {
			t.Fatalf("partial-legacy install applied nothing")
		}
		if len(conflictPaths(out)) != 0 {
			t.Fatalf("partial-legacy install reported conflicts: %v", conflictPaths(out))
		}
		if !hasAction(out, install.OpCreate, absent) {
			t.Errorf("the absent target was not created: %s (actions=%v)", absent, actionPaths(out))
		}
		updates := 0
		for _, p := range seeded {
			if p == absent {
				continue
			}
			if hasAction(out, install.OpConflict, p) {
				t.Errorf("exact legacy sibling reported as a conflict, not adopted: %s", p)
			}
			if hasAction(out, install.OpCreate, p) {
				t.Errorf("a present legacy sibling was created rather than adopted: %s", p)
			}
			if hasAction(out, install.OpUpdate, p) {
				updates++
			}
		}
		if updates == 0 {
			t.Errorf("no present legacy sibling was adopted as an update alongside the created absent one")
		}
		recorded := recordedByPath(state(t, w))
		if rec, ok := recorded[filepath.Clean(absent)]; !ok || rec.Harness == "" {
			t.Errorf("created target %s not recorded with Go ownership: rec=%+v ok=%v", absent, rec, ok)
		}
		assertNoStagingLitter(t, w.home)
	})

	// mixed-unknown — exact legacy targets plus one unrelated foreign file AT a
	// planned path, and one genuinely unrelated file at NO planned path. The
	// foreign planned path is a conflict for that path only; the whole install
	// refuses (there is no --force), so every byte on disk — the foreign file
	// and the unrelated file alike — is preserved.
	t.Run("mixed-unknown", func(t *testing.T) {
		w := newWorld(t, allHarnessDirs...)
		seedFullyPinnedLegacyTree(t, w)

		foreign := legacyAgentDest(w, "claude", "docket-adr.md")
		writeFile(t, foreign, "hand-written by the user, not docket\n")
		// A file docket never plans for: its bytes must survive a refused install.
		unrelated := w.path(".claude", "my-personal-notes.md")
		writeFile(t, unrelated, "keep me\n")

		before := snapshot(t, w.home)
		out := install.Install(legacyOptions(w))
		if out.Reason != install.ReasonOwnershipConflict {
			t.Fatalf("reason = %q, want %q (err=%v)", out.Reason, install.ReasonOwnershipConflict, out.Err)
		}
		if out.Applied {
			t.Fatal("a conflicting install must apply nothing")
		}
		conflicts := conflictPaths(out)
		if len(conflicts) != 1 || conflicts[0] != filepath.Clean(foreign) {
			t.Fatalf("conflicts = %v, want exactly [%s]", conflicts, filepath.Clean(foreign))
		}
		// Nothing was written: the exact-legacy siblings, the foreign file, and the
		// unrelated file are all exactly as they were.
		assertUnchanged(t, before, snapshot(t, w.home), "mixed-unknown refused install")
		if got := readFile(t, unrelated); got != "keep me\n" {
			t.Errorf("unrelated file mutated: %q", got)
		}
		if _, err := os.Stat(w.roots.StatePath()); !os.IsNotExist(err) {
			t.Errorf("state published despite a conflict: %v", err)
		}
	})

	// drifted — a legacy file with one byte mutated is no longer byte-exact, so
	// it is a conflict (not adopted); its exact-legacy siblings are the only
	// non-conflicts, so the drifted file is the sole reported conflict.
	t.Run("drifted", func(t *testing.T) {
		w := newWorld(t, allHarnessDirs...)
		seedFullyPinnedLegacyTree(t, w)

		drifted := legacyAgentDest(w, "opencode", "docket-status.md")
		body := readFile(t, drifted)
		// Flip exactly one byte: still shaped like the legacy file, but no longer
		// byte-exact, so the frozen reproducer cannot prove ownership.
		mutated := []byte(body)
		mutated[len(mutated)/2] ^= 0x20
		writeFile(t, drifted, string(mutated))

		before := snapshot(t, w.home)
		out := install.Install(legacyOptions(w))
		if out.Reason != install.ReasonOwnershipConflict {
			t.Fatalf("reason = %q, want %q (err=%v)", out.Reason, install.ReasonOwnershipConflict, out.Err)
		}
		if out.Applied {
			t.Fatal("a conflicting install must apply nothing")
		}
		conflicts := conflictPaths(out)
		if len(conflicts) != 1 || conflicts[0] != filepath.Clean(drifted) {
			t.Fatalf("conflicts = %v, want exactly the drifted file [%s]", conflicts, filepath.Clean(drifted))
		}
		assertUnchanged(t, before, snapshot(t, w.home), "drifted refused install")
	})

	// interrupted — an unpublished transaction journal from a killed run is on
	// disk. The next operation recovers it (rolls its pre-image back) before
	// planning, leaving no partial state, and then completes normally.
	t.Run("interrupted", func(t *testing.T) {
		w := newWorld(t, allHarnessDirs...)
		seedFullyPinnedLegacyTree(t, w)

		// First install adopts the exact-legacy tree and publishes state.
		if out := install.Install(legacyOptions(w)); out.Reason != "" || out.Err != nil {
			t.Fatalf("seeding install refused: reason=%q err=%v", out.Reason, out.Err)
		}
		installed := loadState(t, w.roots)
		if installed == nil {
			t.Fatal("no state after the seeding install")
		}

		// A killed run: an update transaction is journaled (pre-image captured)
		// and its destination half-rewritten, but nobody rolled it back.
		target := legacyAgentDest(w, "claude", "docket-brainstorm-consultant.md")
		recorded := readFile(t, target)
		pending := install.Target{Path: target, Kind: install.KindFile, Content: []byte("half-applied\n"), Role: "agent"}
		insp, err := install.InspectTarget(pending, installed, nil)
		if err != nil {
			t.Fatalf("InspectTarget: %v", err)
		}
		if insp.Disposition != install.DispositionUpdate {
			t.Fatalf("fixture: pending inspection is %q, want an update", insp.Disposition)
		}
		txn, err := install.BeginTxn(install.RealFS{}, w.roots, []install.Inspection{insp})
		if err != nil {
			t.Fatalf("BeginTxn: %v", err)
		}
		writeFile(t, target, "half-applied\n")
		if _, found, err := install.DetectRecovery(w.roots); err != nil || !found {
			t.Fatalf("the interrupted journal is not detectable: found=%v err=%v", found, err)
		}

		// The next operation recovers the journal, restoring the pre-image, and
		// leaves nothing partial behind.
		out := install.Install(legacyOptions(w))
		if out.Reason != "" || out.Err != nil {
			t.Fatalf("install over a pending journal refused: reason=%q err=%v", out.Reason, out.Err)
		}
		if !hasAction(out, install.OpRecover, filepath.Join(w.roots.TransactionsDir(), txn.ID())) {
			t.Errorf("recovery not reported: %v", out.Actions)
		}
		if got := readFile(t, target); got != recorded {
			t.Errorf("recovery did not restore the pre-image: got %q, want the recorded bytes", got)
		}
		if _, found, err := install.DetectRecovery(w.roots); err != nil || found {
			t.Errorf("a journal survived recovery: found=%v err=%v", found, err)
		}
		assertNoStagingLitter(t, w.home)
	})

	// rollback — an apply failure injected mid-transaction must roll the whole
	// installation back: every home byte returns to what it was, no journal is
	// left for a later run, and no state is published.
	t.Run("full-rollback-on-apply-failure", func(t *testing.T) {
		w := newWorld(t, allHarnessDirs...)
		before := snapshot(t, w.home)

		// Fail the third rename whose destination is under the home. The journal
		// (written under the data root during BeginTxn) is untouched, so the
		// transaction opens; the third published home target then fails, and the
		// two already published must be rolled back to absent.
		renames := 0
		fs := &failingFS{inner: install.RealFS{}, fail: func(op, path string) error {
			if op == "Rename" && strings.HasPrefix(path, w.home+string(os.PathSeparator)) {
				renames++
				if renames == 3 {
					return errors.New("injected apply failure")
				}
			}
			return nil
		}}
		o := legacyOptions(w)
		o.FS = fs

		out := install.Install(o)
		if out.Err == nil {
			t.Fatal("install succeeded despite an injected apply failure")
		}
		if out.Reason != install.ReasonFilesystemFailed {
			t.Errorf("reason = %q, want %q (err=%v)", out.Reason, install.ReasonFilesystemFailed, out.Err)
		}
		if out.Applied {
			t.Error("a rolled-back install reported applied work")
		}
		if renames < 3 {
			t.Fatalf("the injected failure was never reached (%d home renames); the test proves nothing", renames)
		}
		// Full rollback: the home is byte-for-byte what it was before the run.
		assertUnchanged(t, before, snapshot(t, w.home), "after a rolled-back apply")
		assertNoStagingLitter(t, w.home)
		if _, found, err := install.DetectRecovery(w.roots); err != nil || found {
			t.Errorf("a journal survived a completed rollback: found=%v err=%v", found, err)
		}
		if _, err := os.Stat(w.roots.StatePath()); !os.IsNotExist(err) {
			t.Errorf("state published despite a rolled-back apply: %v", err)
		}
	})
}

// state is a thin wrapper over loadState for the sub-tests above, kept local so
// the intent — "read the published record" — reads at the call site.
func state(t *testing.T, w *world) *install.State {
	t.Helper()
	return loadState(t, w.roots)
}
