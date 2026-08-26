package app

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"
)

// These are the durable gate-record store tests (change 0334, Task 1). Each
// builds a real temporary git repository via git init (reusing the package's
// runGit/gitIdentity/requireRealGit fixture helpers) so the store's git
// common-dir rooting, cross-repo refusal, and linked-worktree resolution are
// exercised against real git, not a mock. The store is the generalization of
// scripts/lib/docket-dispatch-dir.sh's durable-dir conventions.

// newGateRepo initializes a temp git repo with a deterministic identity and one
// seed commit (a commit is required before `git worktree add` can attach a
// linked worktree). It returns the repo's working-tree path.
func newGateRepo(t *testing.T) string {
	t.Helper()
	requireRealGit(t)
	dir := t.TempDir()
	runGit(t, dir, "init")
	gitIdentity(t, dir)
	writeRepoFile(t, dir, "seed.txt", "seed\n")
	runGit(t, dir, "add", "seed.txt")
	runGit(t, dir, "commit", "-m", "seed")
	return dir
}

// sampleGateRecord is a fully-populated non-authoritative record (Schema and
// Repo are stamped by the store, so they are left zero here).
func sampleGateRecord() GateRecord {
	return GateRecord{
		Target:        "docket-implement-next",
		CreatedAt:     1700000000,
		DispatchEpoch: 1700000005,
		BeforeIDs:     []int{12, 34, 56},
		AttributedID:  0,
		Retry:         RetryUnused,
		Disposition:   "gate-armed",
		Terminal:      false,
	}
}

func TestGateRecordMintLoadRoundTrip(t *testing.T) {
	repo := newGateRepo(t)
	rec := sampleGateRecord()

	key, err := MintGateRecord(repo, rec)
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(key) {
		t.Fatalf("minted key %q does not match ^[a-z0-9-]+$", key)
	}
	if len(key) < 15 || len(key) > 128 {
		t.Fatalf("minted key %q length %d out of bounds", key, len(key))
	}
	if !regexp.MustCompile(`^implement-next-`).MatchString(key) {
		t.Fatalf("minted key %q is not prefixed implement-next-", key)
	}

	got, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if got.Schema != gateSchemaVersion {
		t.Errorf("Schema = %d, want %d", got.Schema, gateSchemaVersion)
	}
	if got.Repo == "" {
		t.Errorf("Repo is empty; want the canonical common-dir path")
	}
	if got.Target != rec.Target {
		t.Errorf("Target = %q, want %q", got.Target, rec.Target)
	}
	if got.CreatedAt != rec.CreatedAt {
		t.Errorf("CreatedAt = %d, want %d", got.CreatedAt, rec.CreatedAt)
	}
	if got.DispatchEpoch != rec.DispatchEpoch {
		t.Errorf("DispatchEpoch = %d, want %d", got.DispatchEpoch, rec.DispatchEpoch)
	}
	if !slices.Equal(got.BeforeIDs, rec.BeforeIDs) {
		t.Errorf("BeforeIDs = %v, want %v", got.BeforeIDs, rec.BeforeIDs)
	}
	if got.AttributedID != rec.AttributedID {
		t.Errorf("AttributedID = %d, want %d", got.AttributedID, rec.AttributedID)
	}
	if got.Retry != rec.Retry {
		t.Errorf("Retry = %q, want %q", got.Retry, rec.Retry)
	}
	if got.Disposition != rec.Disposition {
		t.Errorf("Disposition = %q, want %q", got.Disposition, rec.Disposition)
	}
	if got.Terminal != rec.Terminal {
		t.Errorf("Terminal = %v, want %v", got.Terminal, rec.Terminal)
	}
}

func TestGateRecordWrongRepo(t *testing.T) {
	repoA := newGateRepo(t)
	repoB := newGateRepo(t)

	key, err := MintGateRecord(repoA, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord(repoA): %v", err)
	}

	// Simulate the record ending up under repo B's store (a stale copy, a moved
	// .git). The embedded Repo still names repo A, so a load from repo B must be
	// refused as wrong-repo rather than trusted.
	rootA, err := gateRoot(repoA)
	if err != nil {
		t.Fatalf("gateRoot(repoA): %v", err)
	}
	rootB, err := gateRoot(repoB)
	if err != nil {
		t.Fatalf("gateRoot(repoB): %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rootB, key), 0o755); err != nil {
		t.Fatal(err)
	}
	buf, err := os.ReadFile(filepath.Join(rootA, key, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, key, "record.json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = LoadGateRecord(repoB, key)
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateWrongRepo {
		t.Fatalf("LoadGateRecord(repoB, key) = %v, want ErrGateWrongRepo", err)
	}
}

func TestGateRecordMalformedKey(t *testing.T) {
	repo := newGateRepo(t)
	for _, key := range []string{"../escape", "", "UPPER", repeat("a", 300)} {
		_, err := LoadGateRecord(repo, key)
		gse, ok := AsGateStoreError(err)
		if !ok || gse.Kind != ErrGateMalformedKey {
			t.Errorf("LoadGateRecord(%q) = %v, want ErrGateMalformedKey", key, err)
		}
	}

	// Key validation MUST precede any filesystem or git touch: a malformed key
	// against a path that is not even a git repo still returns malformed-key,
	// never a git/IO error.
	_, err := LoadGateRecord(filepath.Join(t.TempDir(), "not-a-repo"), "../escape")
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateMalformedKey {
		t.Errorf("malformed key against non-repo = %v, want ErrGateMalformedKey (validated before fs)", err)
	}
}

func TestGateRecordCorruptAndSchema(t *testing.T) {
	repo := newGateRepo(t)
	root, err := gateRoot(repo)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
	}

	// Corrupt JSON.
	kCorrupt, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, kCorrupt, "record.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadGateRecord(repo, kCorrupt)
	if gse, ok := AsGateStoreError(err); !ok || gse.Kind != ErrGateCorruptRecord {
		t.Errorf("corrupt JSON load = %v, want ErrGateCorruptRecord", err)
	}

	// Unsupported schema.
	kSchema, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	rec99 := `{"schema":99,"repo":"whatever","target":"docket-implement-next","retry":"unused"}`
	if err := os.WriteFile(filepath.Join(root, kSchema, "record.json"), []byte(rec99), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadGateRecord(repo, kSchema)
	if gse, ok := AsGateStoreError(err); !ok || gse.Kind != ErrGateCorruptRecord {
		t.Errorf("schema 99 load = %v, want ErrGateCorruptRecord", err)
	}
}

func TestGateRecordSaveDurableReload(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}

	loaded, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	loaded.Disposition = "gate-done run-complete"
	loaded.Terminal = true
	loaded.AttributedID = 42
	if err := SaveGateRecord(repo, key, loaded); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}

	// A fresh call reads only from disk — there is no shared in-memory state, so
	// this proves restart durability.
	again, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord (reload): %v", err)
	}
	if again.Disposition != "gate-done run-complete" || !again.Terminal || again.AttributedID != 42 {
		t.Errorf("reloaded record = %+v; save did not persist durably", again)
	}
}

func TestGateRecordLinkedWorktreeSameRecord(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}

	wt := filepath.Join(t.TempDir(), "linked")
	runGit(t, repo, "worktree", "add", wt)

	// The linked worktree shares the same git common dir, so the record minted
	// through the main worktree resolves to the SAME record here.
	got, err := LoadGateRecord(wt, key)
	if err != nil {
		t.Fatalf("LoadGateRecord(linked worktree): %v", err)
	}
	if got.Target != "docket-implement-next" || !slices.Equal(got.BeforeIDs, []int{12, 34, 56}) {
		t.Errorf("linked-worktree load = %+v; want the same record", got)
	}
}

func TestGateRetryConsumeOnceThenFalse(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}

	first, err := ConsumeGateRetry(repo, key)
	if err != nil {
		t.Fatalf("ConsumeGateRetry (first): %v", err)
	}
	if !first {
		t.Fatalf("first ConsumeGateRetry = false, want true")
	}

	// The JSON mirror is flipped to consumed (the marker is authority; the field
	// is the readable mirror).
	loaded, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord after consume: %v", err)
	}
	if loaded.Retry != RetryConsumed {
		t.Errorf("Retry = %q after consume, want %q", loaded.Retry, RetryConsumed)
	}

	second, err := ConsumeGateRetry(repo, key)
	if err != nil {
		t.Fatalf("ConsumeGateRetry (second): %v", err)
	}
	if second {
		t.Errorf("second ConsumeGateRetry = true, want false (permit already spent)")
	}
}

func TestGateRetryConcurrentExactlyOne(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}

	const n = 16
	var wg sync.WaitGroup
	results := make(chan bool, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, cerr := ConsumeGateRetry(repo, key)
			if cerr != nil {
				t.Errorf("ConsumeGateRetry: %v", cerr)
			}
			results <- ok
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	trues := 0
	for ok := range results {
		if ok {
			trues++
		}
	}
	if trues != 1 {
		t.Fatalf("concurrent ConsumeGateRetry granted %d permits, want exactly 1", trues)
	}
}

func TestGatePruneRetentionWindow(t *testing.T) {
	repo := newGateRepo(t)
	root, err := gateRoot(repo)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
	}

	terminal := sampleGateRecord()
	terminal.Terminal = true
	nonterminal := sampleGateRecord()
	nonterminal.Terminal = false

	kOld, err := MintGateRecord(repo, terminal) // terminal, backdated past window -> pruned
	if err != nil {
		t.Fatal(err)
	}
	kFresh, err := MintGateRecord(repo, terminal) // terminal, inside window -> kept
	if err != nil {
		t.Fatal(err)
	}
	kLive, err := MintGateRecord(repo, nonterminal) // nonterminal, backdated past window -> kept
	if err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-8 * 24 * time.Hour)
	for _, k := range []string{kOld, kLive} {
		rp := filepath.Join(root, k, "record.json")
		if err := os.Chtimes(rp, past, past); err != nil {
			t.Fatal(err)
		}
	}

	PruneGateRecords(repo)

	if _, err := os.Stat(filepath.Join(root, kOld)); !os.IsNotExist(err) {
		t.Errorf("terminal record backdated past retention was not pruned (stat err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, kFresh)); err != nil {
		t.Errorf("terminal record inside the window was pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, kLive)); err != nil {
		t.Errorf("nonterminal record backdated past the window was age-pruned: %v", err)
	}
}

// repeat returns s repeated n times (a tiny local helper so the malformed-key
// case can build an over-long key without importing strings just for this).
func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
