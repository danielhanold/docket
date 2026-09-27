package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClassifyRunEpochError (change 0463): every run-epoch registry failure maps to
// a stable protocol result and a fixed reason token. A not-found epoch is the named
// unknown-run-epoch. A presented value wrapped into the error chain never leaks into
// the reason.
func TestClassifyRunEpochError(t *testing.T) {
	const presented = "0790b760e26444866ef2e156ba383326"
	cases := []struct {
		kind   EpochErrorKind
		res    Result
		reason string
	}{
		{ErrEpochNotFound, ResultInvalidInput, "unknown-run-epoch"},
		{ErrEpochMismatch, ResultInvalidInput, "stale-run-epoch"},
		{ErrEpochAmbiguous, ResultInvalidInput, "epoch-ambiguous"},
		{ErrEpochNotActive, ResultInvalidInput, "epoch-not-active"},
		{ErrEpochOwnerAmbiguous, ResultInvalidInput, "epoch-owner-ambiguous"},
		{ErrEpochOwnerUnresolved, ResultInvalidInput, "epoch-owner-unresolved"},
		{ErrEpochCorrupt, ResultInternalError, "epoch-corrupt"},
		{ErrEpochIO, ResultInternalError, "epoch-io"},
	}
	for _, tc := range cases {
		err := fmt.Errorf("start refused for %s: %w", presented, epochErr(tc.kind, "find-dir-by-id", errors.New(presented)))
		res, reason, ok := ClassifyRunEpochError(err)
		if !ok || res != tc.res || reason != tc.reason {
			t.Errorf("%s: got (%s, %q, %v), want (%s, %q, true)", tc.kind, res, reason, ok, tc.res, tc.reason)
		}
		if strings.Contains(reason, presented) {
			t.Errorf("%s: reason leaked the presented value: %q", tc.kind, reason)
		}
	}
	if _, _, ok := ClassifyRunEpochError(errors.New("plain failure")); ok {
		t.Error("a non-epoch error must not classify")
	}
	if _, _, ok := ClassifyRunEpochError(ErrStaleRunEpoch); ok {
		t.Error("a mutation-fence error is not an epoch-registry error")
	}
}

// TestRunEpochNextAction (change 0463): unknown-run-epoch tells the caller which
// gate-armed field goes where. Its text is distinct from the stale-linkage remedy.
// Other reasons carry no invented message.
func TestRunEpochNextAction(t *testing.T) {
	unknown := RunEpochNextAction(ReasonUnknownRunEpoch)
	for _, want := range []string{"--run-epoch", "--gate-context", "gate-armed <key> <epoch> <dispatch-context>"} {
		if !strings.Contains(unknown, want) {
			t.Errorf("unknown-run-epoch message must mention %q, got %q", want, unknown)
		}
	}
	stale := RunEpochNextAction("stale-run-epoch")
	if stale == "" || stale == unknown {
		t.Errorf("stale-run-epoch needs its own message, got %q", stale)
	}
	if got := RunEpochNextAction("epoch-io"); got != "" {
		t.Errorf("an unmapped reason must yield no message, got %q", got)
	}
}

// TestCheckRunEpochLinkage (change 0463): the agent.enter preflight answers with a
// typed EpochError. Not-found covers both a gate key with no epoch and a gate key
// that does not exist (the pair names no epoch). Mismatch covers a different
// recorded id. A matching pair is nil.
func TestCheckRunEpochLinkage(t *testing.T) {
	repo := newGateRepo(t)
	bare := mintTestGateKey(t, repo)
	if err := CheckRunEpochLinkage(repo, bare, "0790b760e26444866ef2e156ba383326"); !isEpochKind(err, ErrEpochNotFound) {
		t.Fatalf("gate key without an epoch: got %v, want epoch-not-found", err)
	}

	withEpoch := mintTestGateKey(t, repo)
	ep, err := MintEpochRecord(repo, withEpoch, "463")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	if err := CheckRunEpochLinkage(repo, withEpoch, ep.EpochID); err != nil {
		t.Fatalf("matching pair must pass, got %v", err)
	}
	if err := CheckRunEpochLinkage(repo, withEpoch, "0790b760e26444866ef2e156ba383326"); !isEpochKind(err, ErrEpochMismatch) {
		t.Fatalf("wrong epoch id: got %v, want epoch-mismatch", err)
	}

	gone := mintTestGateKey(t, repo)
	root, rerr := gateRoot(repo)
	if rerr != nil {
		t.Fatalf("gateRoot: %v", rerr)
	}
	if err := os.RemoveAll(filepath.Join(root, gone)); err != nil {
		t.Fatalf("remove gate dir: %v", err)
	}
	if err := CheckRunEpochLinkage(repo, gone, ep.EpochID); !isEpochKind(err, ErrEpochNotFound) {
		t.Fatalf("absent gate key: got %v, want epoch-not-found", err)
	}
}
