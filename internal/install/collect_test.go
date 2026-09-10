package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func collectorTree(t *testing.T, roots UserRoots, suffix string) string {
	t.Helper()
	payload := samplePayload()
	payload["agents/docket-build-standard.md"] = []byte("---\nname: docket-build-standard\n---\n" + suffix + "\n")
	m := sampleManifest(t, payload)
	if _, _, err := EnsureVersionTree(roots, m, openFrom(payload)); err != nil {
		t.Fatalf("EnsureVersionTree: %v", err)
	}
	return m.AssetSetID
}

func writeCollectorState(t *testing.T, roots UserRoots, id string) {
	t.Helper()
	if err := WriteStateAtomic(roots.StatePath(), referenceState(id)); err != nil {
		t.Fatalf("WriteStateAtomic: %v", err)
	}
}

func collectionEntry(t *testing.T, out CollectionOutcome, path string) CollectionEntry {
	t.Helper()
	parent, err := canonicalPath(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(parent, filepath.Base(path))
	for _, entry := range out.Entries {
		if entry.Path == path {
			return entry
		}
	}
	t.Fatalf("no collection entry for %s in %#v", path, out.Entries)
	return CollectionEntry{}
}

func collectionEntryID(t *testing.T, out CollectionOutcome, id string) CollectionEntry {
	t.Helper()
	for _, entry := range out.Entries {
		if entry.AssetSetID == id {
			return entry
		}
	}
	t.Fatalf("no collection entry for asset set %s in %#v", id, out.Entries)
	return CollectionEntry{}
}

func canonicalCandidateRoot(t *testing.T, roots UserRoots, id string) string {
	t.Helper()
	root, err := canonicalPath(filepath.Dir(roots.VersionDir(id)))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCollectClassifiesReferencedEligibleAndMalformedCandidates(t *testing.T) {
	roots := versionRoots(t)
	referenced := collectorTree(t, roots, "referenced")
	eligible := collectorTree(t, roots, "eligible")
	writeCollectorState(t, roots, referenced)
	badFile := filepath.Join(roots.VersionsDir(), "plain-file")
	if err := os.WriteFile(badFile, []byte("not a tree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	badLink := filepath.Join(roots.VersionsDir(), "linked")
	if err := os.Symlink(roots.Home, badLink); err != nil {
		t.Fatal(err)
	}
	malformed := filepath.Join(roots.VersionsDir(), "malformed")
	if err := os.Mkdir(malformed, versionDirMode); err != nil {
		t.Fatal(err)
	}

	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if out.Err != nil {
		t.Fatalf("Collect: %v", out.Err)
	}
	if !out.Applied {
		t.Fatal("collection reported no applied work")
	}
	if got := collectionEntry(t, out, filepath.Dir(roots.VersionDir(referenced))).Status; got != "referenced" {
		t.Errorf("referenced status = %q", got)
	}
	if got := collectionEntry(t, out, filepath.Dir(roots.VersionDir(eligible))).Status; got != "collected" {
		t.Errorf("eligible status = %q", got)
	}
	for _, path := range []string{badFile, badLink, malformed} {
		if got := collectionEntry(t, out, path).Status; got != "unverified" {
			t.Errorf("%s status = %q, want unverified", path, got)
		}
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("unverified candidate %s was removed: %v", path, err)
		}
	}
	if _, err := os.Lstat(filepath.Dir(roots.VersionDir(eligible))); !os.IsNotExist(err) {
		t.Errorf("eligible candidate %s remains: %v; outcome=%#v", eligible, err, out)
	}
	for i := 1; i < len(out.Entries); i++ {
		if out.Entries[i-1].Path > out.Entries[i].Path {
			t.Fatalf("entries are not path-sorted: %#v", out.Entries)
		}
	}
}

func TestCollectMissingOrInvalidStateRetainsExistingVersions(t *testing.T) {
	for _, tc := range []struct {
		name string
		bad  bool
	}{
		{"missing", false},
		{"invalid", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roots := versionRoots(t)
			id := collectorTree(t, roots, tc.name)
			if tc.bad {
				if err := os.MkdirAll(filepath.Dir(roots.StatePath()), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(roots.StatePath(), []byte("{unknown}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
			if out.Err == nil {
				t.Fatal("Collect succeeded without trustworthy state")
			}
			root := filepath.Dir(roots.VersionDir(id))
			if got := collectionEntry(t, out, root).Status; got != "unverified" {
				t.Fatalf("status = %q, want unverified", got)
			}
			if _, err := os.Lstat(root); err != nil {
				t.Fatalf("tree was removed: %v", err)
			}
		})
	}
}

func TestCollectFreshMachineIsCleanNoOp(t *testing.T) {
	roots := versionRoots(t)
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if out.Err != nil || out.Applied || len(out.Entries) != 0 || len(out.Pending) != 0 {
		t.Fatalf("fresh Collect = %#v", out)
	}
}

type homeSnapshotEntry struct {
	Path string
	Mode fs.FileMode
	Data []byte
	Link string
}

func snapshotHome(t *testing.T, home string) []homeSnapshotEntry {
	t.Helper()
	var out []homeSnapshotEntry
	err := filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		entry := homeSnapshotEntry{Path: rel, Mode: info.Mode()}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			entry.Link, err = os.Readlink(path)
		case info.Mode().IsRegular():
			entry.Data, err = os.ReadFile(path)
		}
		out = append(out, entry)
		return err
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func TestDryRunCollectionIsByteAndModeReadOnly(t *testing.T) {
	roots := versionRoots(t)
	keep := collectorTree(t, roots, "keep")
	eligible := collectorTree(t, roots, "eligible")
	writeCollectorState(t, roots, keep)
	// Dry-run may only observe an existing mutex; create and release it first.
	lk, err := acquireInstallLock(roots)
	if err != nil {
		t.Fatal(err)
	}
	lk.release()
	before := snapshotHome(t, roots.Home)
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}, DryRun: true})
	after := snapshotHome(t, roots.Home)
	if out.Err != nil || out.Applied {
		t.Fatalf("dry Collect = %#v", out)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("dry-run changed home\nbefore=%#v\nafter=%#v", before, after)
	}
	if got := collectionEntry(t, out, filepath.Dir(roots.VersionDir(eligible))).Status; got != "collected" {
		t.Fatalf("dry eligible status = %q", got)
	}
	if _, err := os.Lstat(roots.CollectionJournalPath()); !os.IsNotExist(err) {
		t.Fatalf("dry-run created a journal: %v", err)
	}
}

func TestDryRunCollectionFreshMachineCreatesNothing(t *testing.T) {
	roots := versionRoots(t)
	before := snapshotHome(t, roots.Home)
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}, DryRun: true})
	after := snapshotHome(t, roots.Home)
	if out.Err != nil || out.Applied || len(out.Entries) != 0 {
		t.Fatalf("fresh dry Collect = %#v", out)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("fresh dry-run changed home")
	}
	if _, err := os.Lstat(roots.LockPath()); !os.IsNotExist(err) {
		t.Fatalf("fresh dry-run created lock: %v", err)
	}
}

func TestCollectLockContention(t *testing.T) {
	roots := versionRoots(t)
	lk, err := acquireInstallLock(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lk.release()
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if !errors.Is(out.Err, ErrInstallLocked) || out.Applied {
		t.Fatalf("contended Collect = %#v", out)
	}
}

func TestCollectFailedQuarantineAndRetryCompletion(t *testing.T) {
	roots := versionRoots(t)
	keep := collectorTree(t, roots, "keep")
	eligible := collectorTree(t, roots, "eligible")
	writeCollectorState(t, roots, keep)
	boom := errors.New("rename refused")
	var renames []string
	fsys := collectionFailFS{rename: func(old, new string) error {
		renames = append(renames, old+" -> "+new)
		if filepath.Base(new) == filepath.Base(roots.CollectionQuarantineDir()) {
			return boom
		}
		return os.Rename(old, new)
	}}
	first := Collect(CollectOptions{Roots: roots, FS: fsys})
	if first.Err == nil || collectionEntry(t, first, filepath.Dir(roots.VersionDir(eligible))).Status != "failed" {
		t.Fatalf("failed Collect keep=%s eligible=%s renames=%#v outcome=%#v", keep, eligible, renames, first)
	}
	if len(first.Pending) == 0 {
		t.Fatal("failed collection reported no pending recovery")
	}
	second := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if second.Err != nil {
		t.Fatalf("retry Collect: %v", second.Err)
	}
	if _, err := os.Lstat(filepath.Dir(roots.VersionDir(eligible))); !os.IsNotExist(err) {
		t.Fatalf("retry did not remove eligible tree: %v", err)
	}
}

func TestCollectPendingRollbackRefusesDeletion(t *testing.T) {
	roots := versionRoots(t)
	keep := collectorTree(t, roots, "keep")
	eligible := collectorTree(t, roots, "eligible")
	writeCollectorState(t, roots, keep)
	txnDir := filepath.Join(roots.TransactionsDir(), "20000101T000000.000000000-1-aaaa")
	if err := os.MkdirAll(txnDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(txnDir, journalPlanFile), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if out.Err == nil || len(out.Pending) == 0 {
		t.Fatalf("Collect with rollback journal = %#v", out)
	}
	if _, err := os.Lstat(filepath.Dir(roots.VersionDir(eligible))); err != nil {
		t.Fatalf("pending rollback allowed deletion: %v", err)
	}
}

func TestCollectLegacyAndUnreadableCandidates(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		roots := versionRoots(t)
		keep := collectorTree(t, roots, "keep")
		writeCollectorState(t, roots, keep)
		manifest, legacyAssets := legacyV1Fixture(t)
		if err := os.MkdirAll(roots.VersionsDir(), versionsDirMode); err != nil {
			t.Fatal(err)
		}
		legacyRoot := filepath.Join(roots.VersionsDir(), filepath.Base(filepath.Dir(legacyAssets)))
		if err := os.Rename(filepath.Dir(legacyAssets), legacyRoot); err != nil {
			t.Fatal(err)
		}
		out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
		if out.Err != nil {
			t.Fatalf("Collect legacy: %v", out.Err)
		}
		entry := collectionEntry(t, out, legacyRoot)
		if entry.Status != CollectionStatusCollected || entry.AssetSetID != manifest.AssetSetID {
			t.Fatalf("legacy entry = %#v", entry)
		}
		if _, err := os.Lstat(legacyRoot); !os.IsNotExist(err) {
			t.Fatalf("legacy tree remains: %v", err)
		}
	})

	t.Run("unreadable", func(t *testing.T) {
		roots := versionRoots(t)
		keep := collectorTree(t, roots, "keep")
		candidate := collectorTree(t, roots, "unreadable")
		writeCollectorState(t, roots, keep)
		assetsDir := roots.VersionDir(candidate)
		if err := os.Chmod(assetsDir, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(assetsDir, versionDirMode)
		out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
		if out.Err != nil {
			t.Fatalf("Collect unreadable: %v", out.Err)
		}
		if got := collectionEntry(t, out, filepath.Dir(assetsDir)).Status; got != CollectionStatusUnverified {
			t.Fatalf("unreadable status = %q", got)
		}
		if _, err := os.Lstat(filepath.Dir(assetsDir)); err != nil {
			t.Fatalf("unreadable tree was removed: %v", err)
		}
	})
}

func TestCollectPartialDeleteAndRetryCompletion(t *testing.T) {
	roots := versionRoots(t)
	keep := collectorTree(t, roots, "keep")
	eligible := collectorTree(t, roots, "partial")
	writeCollectorState(t, roots, keep)
	removed := 0
	boom := errors.New("remove interrupted")
	fsys := collectionFailFS{remove: func(path string) error {
		if strings.HasSuffix(path, ".md") {
			removed++
			if removed == 2 {
				return boom
			}
		}
		return os.Remove(path)
	}}
	first := Collect(CollectOptions{Roots: roots, FS: fsys})
	if first.Err == nil || !errors.Is(first.Err, ErrCollectionPending) || len(first.Pending) == 0 {
		t.Fatalf("partial Collect = %#v", first)
	}
	if _, err := os.Lstat(filepath.Dir(roots.VersionDir(eligible))); !os.IsNotExist(err) {
		t.Fatalf("partially deleted tree remained published: %v", err)
	}
	second := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if second.Err != nil || !second.Applied {
		t.Fatalf("retry Collect = %#v", second)
	}
	if _, err := os.Lstat(roots.CollectionJournalPath()); !os.IsNotExist(err) {
		t.Fatalf("retry left journal: %v", err)
	}
}

func TestDryRunCollectionReportsPendingJournalsWithoutRecovery(t *testing.T) {
	t.Run("rollback", func(t *testing.T) {
		roots := versionRoots(t)
		if err := os.MkdirAll(filepath.Join(roots.TransactionsDir(), "20000101T000000.000000000-1-aaaa"), 0o700); err != nil {
			t.Fatal(err)
		}
		plan := filepath.Join(roots.TransactionsDir(), "20000101T000000.000000000-1-aaaa", journalPlanFile)
		if err := os.WriteFile(plan, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		lk, err := acquireInstallLock(roots)
		if err != nil {
			t.Fatal(err)
		}
		lk.release()
		before := snapshotHome(t, roots.Home)
		out := Collect(CollectOptions{Roots: roots, FS: RealFS{}, DryRun: true})
		after := snapshotHome(t, roots.Home)
		if out.Err == nil || len(out.Pending) != 1 {
			t.Fatalf("dry pending rollback = %#v", out)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("dry pending rollback changed bytes or modes")
		}
	})

	t.Run("collection", func(t *testing.T) {
		roots, _, journal := collectionFixture(t)
		if err := writeCollectionJournal(RealFS{}, roots, &journal); err != nil {
			t.Fatal(err)
		}
		lk, err := acquireInstallLock(roots)
		if err != nil {
			t.Fatal(err)
		}
		lk.release()
		before := snapshotHome(t, roots.Home)
		out := Collect(CollectOptions{Roots: roots, FS: RealFS{}, DryRun: true})
		after := snapshotHome(t, roots.Home)
		if !errors.Is(out.Err, ErrCollectionPending) || len(out.Pending) != 1 {
			t.Fatalf("dry pending collection = %#v", out)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("dry pending collection changed bytes or modes")
		}
	})
}

func TestCollectReproofProtectsChangedCandidate(t *testing.T) {
	roots := versionRoots(t)
	keep := collectorTree(t, roots, "keep")
	candidate := collectorTree(t, roots, "reproof")
	writeCollectorState(t, roots, keep)
	root := canonicalCandidateRoot(t, roots, candidate)
	manifest := filepath.Join(root, versionManifest)
	original := collectBeforeQuarantine
	defer func() { collectBeforeQuarantine = original }()
	collectBeforeQuarantine = func(path string) {
		if path != root {
			return
		}
		if err := os.Chmod(manifest, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifest, []byte("{}\n"), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if out.Err == nil || collectionEntryID(t, out, candidate).Status != CollectionStatusFailed {
		t.Fatalf("changed candidate Collect = %#v", out)
	}
	if _, err := os.Lstat(root); err != nil {
		t.Fatalf("changed candidate left published namespace: %v", err)
	}
}

func TestCollectReferenceRefreshProtectsNewReference(t *testing.T) {
	roots := versionRoots(t)
	keep := collectorTree(t, roots, "keep")
	candidate := collectorTree(t, roots, "new-reference")
	writeCollectorState(t, roots, keep)
	root := canonicalCandidateRoot(t, roots, candidate)
	original := collectBeforeReferenceRefresh
	defer func() { collectBeforeReferenceRefresh = original }()
	collectBeforeReferenceRefresh = func(path string) {
		if path == root {
			writeCollectorState(t, roots, candidate)
		}
	}
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if out.Err != nil {
		t.Fatalf("Collect: %v", out.Err)
	}
	if got := collectionEntryID(t, out, candidate).Status; got != CollectionStatusReferenced {
		t.Fatalf("newly referenced status = %q", got)
	}
	if _, err := os.Lstat(root); err != nil {
		t.Fatalf("newly referenced tree was removed: %v", err)
	}
}

func TestCollectStrictContainmentProtectsPrefixSibling(t *testing.T) {
	roots := versionRoots(t)
	keep := collectorTree(t, roots, "keep")
	candidate := collectorTree(t, roots, "containment")
	writeCollectorState(t, roots, keep)
	root := canonicalCandidateRoot(t, roots, candidate)
	stageRoots := roots
	stageRoots.DataRoot = filepath.Join(roots.DataRoot, "stage")
	if got := collectorTree(t, stageRoots, "containment"); got != candidate {
		t.Fatalf("duplicate candidate id = %s, want %s", got, candidate)
	}
	prefixSibling := roots.VersionsDir() + "-evil"
	if err := os.Rename(stageRoots.VersionsDir(), prefixSibling); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(prefixSibling, filepath.Base(root))
	original := collectBeforeQuarantine
	defer func() { collectBeforeQuarantine = original }()
	collectBeforeQuarantine = func(path string) {
		if path != root {
			return
		}
		if err := os.Rename(roots.VersionsDir(), roots.VersionsDir()+"-away"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(prefixSibling, roots.VersionsDir()); err != nil {
			t.Fatal(err)
		}
	}
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if out.Err == nil || collectionEntryID(t, out, candidate).Status != CollectionStatusFailed {
		t.Fatalf("escaped candidate Collect = %#v", out)
	}
	if _, err := os.Lstat(protected); err != nil {
		t.Fatalf("prefix-sibling protected tree was removed: %v", err)
	}
}

func TestCollectLstatKindProtectsSymlinkAlias(t *testing.T) {
	roots := versionRoots(t)
	keep := collectorTree(t, roots, "keep")
	candidate := collectorTree(t, roots, "kind")
	writeCollectorState(t, roots, candidate)
	root := canonicalCandidateRoot(t, roots, candidate)
	alias := filepath.Join(roots.VersionsDir(), "00-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	original := collectBeforeReferenceRefresh
	defer func() { collectBeforeReferenceRefresh = original }()
	collectBeforeReferenceRefresh = func(path string) {
		if path == root {
			writeCollectorState(t, roots, keep)
		}
	}
	out := Collect(CollectOptions{Roots: roots, FS: RealFS{}})
	if out.Err != nil {
		t.Fatalf("Collect: %v", out.Err)
	}
	if got := collectionEntry(t, out, alias).Status; got != CollectionStatusUnverified {
		t.Fatalf("symlink alias status = %q", got)
	}
	if _, err := os.Lstat(root); err != nil {
		t.Fatalf("aliased tree was removed: %v", err)
	}
}
