//go:build integration

package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/process"
)

// seedPublishLock creates a real publish lock file in key's run-key directory and
// returns its token. hold=true keeps it held by this test process (a live
// publisher) until release runs; release is also registered with t.Cleanup.
// hold=false leaves the file present and free: what a publisher that died
// mid-flight leaves behind.
func seedPublishLock(t *testing.T, repo, key string, hold bool) (string, func()) {
	t.Helper()
	dir, err := runKeyDir(repo, key, "test-seed-publish-lock")
	if err != nil {
		t.Fatalf("runKeyDir: %v", err)
	}
	token, err := runToken()
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}
	path, ok := publishLockPath(dir, token)
	if !ok {
		t.Fatalf("publishLockPath rejected minted token %q", token)
	}
	f, busy, err := process.TryExclusiveLock(path)
	if err != nil || busy {
		t.Fatalf("TryExclusiveLock: busy=%v err=%v", busy, err)
	}
	if !hold {
		_ = f.Close()
		return token, func() {}
	}
	var once sync.Once
	release := func() { once.Do(func() { _ = f.Close() }) }
	t.Cleanup(release)
	return token, release
}

// seedJournal replaces key's admitted-mutation journal with muts.
func seedJournal(t *testing.T, repo, key string, muts ...AdmittedMutation) {
	t.Helper()
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		r.AdmittedMutations = append([]AdmittedMutation(nil), muts...)
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
}

// journalEntry loads key's run record and returns journal entry i.
func journalEntry(t *testing.T, repo, key string, i int) AdmittedMutation {
	t.Helper()
	ep, _, err := LoadRunRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if i >= len(ep.AdmittedMutations) {
		t.Fatalf("journal has %d entries, want index %d", len(ep.AdmittedMutations), i)
	}
	return ep.AdmittedMutations[i]
}

// publishLockPathFor is the lock path token names under key's run-key directory.
func publishLockPathFor(t *testing.T, repo, key, token string) string {
	t.Helper()
	dir, err := runKeyDir(repo, key, "test-publish-lock-path")
	if err != nil {
		t.Fatalf("runKeyDir: %v", err)
	}
	path, ok := publishLockPath(dir, token)
	if !ok {
		t.Fatalf("publishLockPath rejected %q", token)
	}
	return path
}

func lockTestWSDesc() MutationPublication {
	return MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: strings.Repeat("a", 40)}
}

func lockTestPRDesc() MutationPublication {
	return MutationPublication{RepoHost: "github.com", RepoOwner: "o", RepoName: "r",
		HeadRef: "fix/w", HeadCommit: strings.Repeat("a", 40), BaseBranch: "main",
		TitleDigest: publicationDigest("pr-title", "t"), BodyDigest: publicationDigest("pr-body", "b")}
}
