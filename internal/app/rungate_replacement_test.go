package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestEpochReplacementProof(t *testing.T) {
	for _, tc := range []string{"valid", "active-ancestor", "cancelled-ancestor", "wrong-change", "wrong-scope-change", "wrong-worktree", "cancelled-incoming", "missing-link", "cycle", "unrelated", "unknown-incoming"} {
		t.Run(tc, func(t *testing.T) {
			repo := newWorkingRepo(t, nil).invocation
			key, old := seedPriorEpoch(t, repo, EpochCancelled)
			deps, wdeps := resumeEpochDeps(t)
			wdeps.Service = resumeInspectService(repo)
			sp := &fakeScopePrep{grant: sampleScopeGrant()}
			armed := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repo, "implement-next", 5)
			if !armed.Armed {
				t.Fatalf("resume: %s", armed.HumanText())
			}
			next, worktree := armed.Epoch, repo
			mutate := func(k string, fn func(*EpochRecord)) {
				t.Helper()
				if err := epochCAS(repo, k, func(r *EpochRecord) error { fn(r); return nil }); err != nil {
					t.Fatal(err)
				}
			}
			switch tc {
			case "active-ancestor":
				mutate(key, func(r *EpochRecord) { r.State = EpochActive })
			case "cancelled-ancestor":
				mutate(key, func(r *EpochRecord) { r.State = EpochCancelled })
			case "wrong-change":
				mutate(key, func(r *EpochRecord) { r.ChangeID = "6" })
			case "wrong-worktree":
				worktree = testsupport.TempDir(t)
			case "cancelled-incoming":
				mutate(armed.Key, func(r *EpochRecord) { r.State = EpochCancelled })
			case "missing-link":
				mutate(key, func(r *EpochRecord) { r.ReplacementReserved = "missing" })
			case "cycle":
				mutate(key, func(r *EpochRecord) { r.ReplacementReserved = key })
			case "unrelated":
				_, old = seedPriorEpoch(t, repo, EpochCancelled)
			case "unknown-incoming":
				next = "unknown"
			}
			common, err := gateGitCommonDir(repo)
			if err != nil {
				t.Fatal(err)
			}
			change := "5"
			if tc == "wrong-scope-change" {
				change = "6"
			}
			release, err := epochReplacementResolver(common)(old, next, worktree, change)
			if release != nil {
				defer release()
			}
			if tc == "valid" {
				if release == nil || err != nil {
					t.Fatalf("valid replacement: %v", err)
				}
				// Cancellation uses this exact flock before fencing the epoch. It
				// cannot pass the proof/reservation boundary while the guard is held.
				dir, err := gateKeyDir(common, armed.Key, "test")
				if err != nil {
					t.Fatal(err)
				}
				lock, err := os.OpenFile(filepath.Join(dir, epochLockFileName), os.O_RDWR, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer lock.Close()
				if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
					t.Fatalf("cancellation lock not held: %v", err)
				}
				release()
				if err := epochCAS(repo, armed.Key, func(r *EpochRecord) error { r.State = EpochCancelling; return nil }); err != nil {
					t.Fatal(err)
				}
				guard, err := epochReplacementResolver(common)(old, next, worktree, change)
				if guard != nil {
					guard()
					t.Fatal("cancellation winner admitted")
				}
				if err != nil {
					t.Fatal(err)
				}
			} else if release != nil {
				t.Fatalf("%s must fail closed", tc)
			}
		})
	}
}
