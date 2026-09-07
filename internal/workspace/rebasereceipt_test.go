package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// plainService builds a real gitcli-backed Service with no repository. The
// receipt read/write/clear surface touches only the filesystem, so a bare
// service is all these round-trip proofs need.
func plainService(t *testing.T) *Service {
	t.Helper()
	requireGit(t)
	c, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	svc, err := NewService(c)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

// sampleReceipt is a fully valid receipt with distinct, well-formed heads so a
// round-trip proves every field survives byte-identically.
func sampleReceipt() RebaseReceipt {
	return RebaseReceipt{
		RepoIdentity:   "/repo/common.git",
		ChangeID:       "7",
		OrigHead:       strings.Repeat("a", 40),
		OrigRemoteHead: strings.Repeat("b", 40),
		BaseRef:        "refs/heads/main",
		BaseHead:       strings.Repeat("c", 40),
		Attempt:        "attempt-01",
		CreatedUTC:     time.Now().UTC().Format(time.RFC3339),
	}
}

// fullOID is a well-formed full (40-hex) object id for the resolver-budget
// reservation fixtures — the same shape validObjectID accepts for the heads.
var fullOID = strings.Repeat("d", 40)

// withBudget returns a mutator stamping the six resolver-budget fields onto a
// receipt, so a table can assemble a whole group in one call. An empty string
// leaves the corresponding field cleared.
func withBudget(version, limit, used, token, stopped, continuation string) func(*RebaseReceipt) {
	return func(r *RebaseReceipt) {
		r.ResolverBudgetVersion = version
		r.ResolverLimit = limit
		r.ResolverUsed = used
		r.ResolverReservationToken = token
		r.ResolverReservationStopped = stopped
		r.ResolverContinuationStarted = continuation
	}
}

// TestRebaseReceiptResolverBudgetValidation exercises the whole-group resolver
// budget rule from BOTH directions the shared validator governs: WriteRebaseReceipt
// must refuse an invalid group (and round-trip a valid one byte-identically), and
// ReadRebaseReceipt of a hand-written file carrying an invalid group must return an
// error — never clean absence (learnings: probe-error-is-not-clean-absence). A
// legacy receipt (whole group absent) stays valid, never corrupt.
func TestRebaseReceiptResolverBudgetValidation(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()

	cases := []struct {
		name   string
		mutate func(*RebaseReceipt)
		ok     bool
	}{
		{"legacy all-empty group", func(r *RebaseReceipt) {}, true},
		{"complete budget no reservation", withBudget("1", "3", "1", "", "", ""), true},
		{"outstanding reservation", withBudget("1", "3", "1", "tok-1", fullOID, ""), true},
		{"continuation started", withBudget("1", "3", "1", "tok-1", fullOID, "1"), true},
		{"used equals limit", withBudget("1", "3", "3", "", "", ""), true},
		{"unsupported version", withBudget("2", "3", "1", "", "", ""), false},
		{"limit zero", withBudget("1", "0", "0", "", "", ""), false},
		{"used exceeds limit", withBudget("1", "2", "3", "", "", ""), false},
		{"used negative", withBudget("1", "3", "-1", "", "", ""), false},
		{"non-decimal limit", withBudget("1", "three", "0", "", "", ""), false},
		{"partial group limit only", func(r *RebaseReceipt) { r.ResolverLimit = "3" }, false},
		{"token without stopped commit", withBudget("1", "3", "1", "tok-1", "", ""), false},
		{"stopped commit without token", withBudget("1", "3", "1", "", fullOID, ""), false},
		{"short stopped commit", withBudget("1", "3", "1", "tok-1", "abc123", ""), false},
		{"continuation without reservation", withBudget("1", "3", "1", "", "", "1"), false},
		{"continuation marker not 0/1 shape", withBudget("1", "3", "1", "tok-1", fullOID, "yes"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := sampleReceipt()
			tc.mutate(&r)

			// Write gate: valid groups round-trip byte-identically; invalid ones
			// are refused and leave nothing behind.
			dir := testsupport.TempDir(t)
			werr := svc.WriteRebaseReceipt(ctx, dir, r)
			if tc.ok {
				if werr != nil {
					t.Fatalf("WriteRebaseReceipt refused a valid budget group: %v", werr)
				}
				got, found, rerr := svc.ReadRebaseReceipt(ctx, dir)
				if rerr != nil || !found {
					t.Fatalf("ReadRebaseReceipt after valid write: found=%v err=%v", found, rerr)
				}
				if got != r {
					t.Fatalf("round trip mutated the receipt:\n got %+v\nwant %+v", got, r)
				}
			} else {
				if werr == nil {
					t.Errorf("WriteRebaseReceipt persisted an invalid budget group without refusal")
				}
				if _, found, rerr := svc.ReadRebaseReceipt(ctx, dir); rerr != nil || found {
					t.Errorf("receipt present after refused write: found=%v err=%v", found, rerr)
				}
			}

			// Read gate: a hand-written file (marshaled directly, bypassing the
			// write validator) carrying an invalid group must read back as an
			// error, never clean absence; a valid one reads back present.
			rdir := testsupport.TempDir(t)
			data, err := json.MarshalIndent(r, "", "  ")
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			if err := os.WriteFile(filepath.Join(rdir, "rebase-receipt.json"), append(data, '\n'), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			_, found, rerr := svc.ReadRebaseReceipt(ctx, rdir)
			if tc.ok {
				if rerr != nil || !found {
					t.Errorf("ReadRebaseReceipt of a valid hand-written group: found=%v err=%v; want present", found, rerr)
				}
			} else {
				if rerr == nil || found {
					t.Errorf("ReadRebaseReceipt of an invalid hand-written group: found=%v err=%v; want error", found, rerr)
				}
			}
		})
	}
}

// TestRebaseReceiptResolverBudgetHelpers proves HasResolverBudget distinguishes a
// legacy receipt from a budgeted one and ResolverBudget decodes the stored
// limit/used pair.
func TestRebaseReceiptResolverBudgetHelpers(t *testing.T) {
	legacy := sampleReceipt()
	if legacy.HasResolverBudget() {
		t.Errorf("legacy receipt reports HasResolverBudget=true; want false")
	}

	budgeted := sampleReceipt()
	withBudget("1", "3", "2", "", "", "")(&budgeted)
	if !budgeted.HasResolverBudget() {
		t.Errorf("budgeted receipt reports HasResolverBudget=false; want true")
	}
	limit, used, err := budgeted.ResolverBudget()
	if err != nil {
		t.Fatalf("ResolverBudget: %v", err)
	}
	if limit != 3 || used != 2 {
		t.Errorf("ResolverBudget = (%d, %d); want (3, 2)", limit, used)
	}
}

// TestRebaseReceiptRoundTrip proves the three-outcome receipt contract: a
// written receipt reads back equal (found), a cleared receipt reads cleanly
// absent (not found, no error), a second clear on an already-absent receipt is a
// no-op, and a corrupt file on the receipt path is an error — never mistaken for
// clean absence (learnings: probe-error-is-not-clean-absence).
func TestRebaseReceiptRoundTrip(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()
	dir := testsupport.TempDir(t)
	r := sampleReceipt()

	if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
		t.Fatalf("WriteRebaseReceipt: %v", err)
	}

	got, found, err := svc.ReadRebaseReceipt(ctx, dir)
	if err != nil {
		t.Fatalf("ReadRebaseReceipt after write: %v", err)
	}
	if !found {
		t.Fatalf("ReadRebaseReceipt after write: found=false; want true")
	}
	if got != r {
		t.Errorf("round-trip mismatch:\n got=%+v\nwant=%+v", got, r)
	}

	if err := svc.ClearRebaseReceipt(ctx, dir); err != nil {
		t.Fatalf("ClearRebaseReceipt: %v", err)
	}
	if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err != nil || found {
		t.Errorf("ReadRebaseReceipt after clear: found=%v err=%v; want cleanly absent (false, nil)", found, err)
	}
	// A clear on an already-absent receipt is idempotent, not an error.
	if err := svc.ClearRebaseReceipt(ctx, dir); err != nil {
		t.Errorf("ClearRebaseReceipt on absent receipt: %v; want nil", err)
	}

	// A present-but-corrupt file is an error, never clean absence.
	if err := os.WriteFile(filepath.Join(dir, "rebase-receipt.json"), []byte("{ not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err == nil || found {
		t.Errorf("ReadRebaseReceipt on corrupt file: found=%v err=%v; want (false, error)", found, err)
	}
}

// TestRebaseReceiptInvalidFieldsRefused proves the write boundary rejects a
// receipt with a malformed field rather than persisting garbage that a later
// read would have to reject as corrupt.
func TestRebaseReceiptInvalidFieldsRefused(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()
	dir := testsupport.TempDir(t)

	bad := sampleReceipt()
	bad.OrigRemoteHead = "not-a-sha"
	if err := svc.WriteRebaseReceipt(ctx, dir, bad); err == nil {
		t.Errorf("WriteRebaseReceipt with malformed head = nil error; want refusal")
	}
	// Nothing was persisted by the refused write.
	if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err != nil || found {
		t.Errorf("receipt present after refused write: found=%v err=%v; want cleanly absent", found, err)
	}
}

// TestRebaseReceiptGatePair proves the optional gate-continuation pair:
// both-set round-trips byte-identically alongside every other field, and a
// half-set pair is refused on write AND on read (the same single gate both
// channels pass through), never returned as valid.
func TestRebaseReceiptGatePair(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()

	t.Run("both-set-round-trips", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		r := sampleReceipt()
		r.GateDriveID = "drive-01"
		r.GateOwnerGeneration = "gen-01"
		if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
			t.Fatalf("WriteRebaseReceipt with gate pair: %v", err)
		}
		got, found, err := svc.ReadRebaseReceipt(ctx, dir)
		if err != nil || !found {
			t.Fatalf("ReadRebaseReceipt: found=%v err=%v", found, err)
		}
		if got != r {
			t.Fatalf("round trip mutated the receipt:\n got %+v\nwant %+v", got, r)
		}
	})

	t.Run("both-empty-round-trips-with-no-gate-keys", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		r := sampleReceipt() // pair empty
		if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
			t.Fatalf("WriteRebaseReceipt: %v", err)
		}
		// omitempty: an empty pair leaves no gate_* keys on disk.
		raw, err := os.ReadFile(filepath.Join(dir, "rebase-receipt.json"))
		if err != nil {
			t.Fatalf("reading receipt file: %v", err)
		}
		if strings.Contains(string(raw), "gate_drive_id") || strings.Contains(string(raw), "gate_owner_generation") {
			t.Errorf("empty pair serialized gate keys: %s", raw)
		}
	})

	t.Run("half-set-refused-on-write", func(t *testing.T) {
		for name, mut := range map[string]func(*RebaseReceipt){
			"drive-only": func(r *RebaseReceipt) { r.GateDriveID = "drive-01" },
			"gen-only":   func(r *RebaseReceipt) { r.GateOwnerGeneration = "gen-01" },
		} {
			t.Run(name, func(t *testing.T) {
				dir := testsupport.TempDir(t)
				r := sampleReceipt()
				mut(&r)
				if err := svc.WriteRebaseReceipt(ctx, dir, r); err == nil {
					t.Errorf("half-set gate pair written without refusal")
				}
				if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err != nil || found {
					t.Errorf("receipt present after refused write: found=%v err=%v", found, err)
				}
			})
		}
	})

	t.Run("half-set-refused-on-read", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		r := sampleReceipt()
		if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
			t.Fatalf("WriteRebaseReceipt: %v", err)
		}
		// Corrupt on disk: inject a lone gate_drive_id key.
		p := filepath.Join(dir, "rebase-receipt.json")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		mutated := strings.Replace(string(raw), "\"attempt\":", "\"gate_drive_id\": \"drive-01\",\n  \"attempt\":", 1)
		if mutated == string(raw) {
			t.Fatalf("fixture mutation did not apply")
		}
		if err := os.WriteFile(p, []byte(mutated), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err == nil || found {
			t.Errorf("half-set pair read back as valid: found=%v err=%v; want error", found, err)
		}
	})
}

// checkpointReceipt is sampleReceipt carrying a fully-set publish checkpoint:
// the completed-gate evidence for a rebased head, recorded so a denied publish
// can resume without re-running the suite (change 0408).
func checkpointReceipt() RebaseReceipt {
	r := sampleReceipt()
	r.PublishCheckpointHead = strings.Repeat("d", 40)
	r.PublishCheckpointBaseHead = strings.Repeat("c", 40)
	r.PublishCheckpointCommand = "go test ./..."
	r.PublishCheckpointGate = "local"
	r.PublishCheckpointPRNumber = "7"
	r.PublishCheckpointEvidence = "<!-- evidence -->\nresult: green\n"
	return r
}

// TestRebaseReceiptPublishCheckpoint proves the optional publish checkpoint:
// fully-set round-trips byte-identically, fully-empty serializes no
// publish_checkpoint_* keys, a partially-set checkpoint is refused on write,
// a malformed member (bad head, non-positive PR number) is refused, and a
// checkpoint coexisting with a live gate-continuation pair is refused — a
// completed terminal and a live drive are mutually exclusive states.
func TestRebaseReceiptPublishCheckpoint(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()

	t.Run("fully-set-round-trips", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		r := checkpointReceipt()
		if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
			t.Fatalf("WriteRebaseReceipt with checkpoint: %v", err)
		}
		got, found, err := svc.ReadRebaseReceipt(ctx, dir)
		if err != nil || !found {
			t.Fatalf("ReadRebaseReceipt: found=%v err=%v", found, err)
		}
		if got != r {
			t.Fatalf("round trip mutated the receipt:\n got %+v\nwant %+v", got, r)
		}
	})

	t.Run("empty-checkpoint-serializes-no-keys", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		if err := svc.WriteRebaseReceipt(ctx, dir, sampleReceipt()); err != nil {
			t.Fatalf("WriteRebaseReceipt: %v", err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "rebase-receipt.json"))
		if err != nil {
			t.Fatalf("reading receipt file: %v", err)
		}
		if strings.Contains(string(raw), "publish_checkpoint") {
			t.Errorf("empty checkpoint serialized publish_checkpoint keys: %s", raw)
		}
	})

	t.Run("partial-and-malformed-refused-on-write", func(t *testing.T) {
		for name, mut := range map[string]func(*RebaseReceipt){
			"head-only":        func(r *RebaseReceipt) { *r = sampleReceipt(); r.PublishCheckpointHead = strings.Repeat("d", 40) },
			"missing-evidence": func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointEvidence = "" },
			"bad-head":         func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointHead = "not-a-sha" },
			"bad-base-head":    func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointBaseHead = "nope" },
			"zero-pr-number":   func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointPRNumber = "0" },
			"non-numeric-pr":   func(r *RebaseReceipt) { *r = checkpointReceipt(); r.PublishCheckpointPRNumber = "seven" },
			"live-drive-coexists": func(r *RebaseReceipt) {
				*r = checkpointReceipt()
				r.GateDriveID, r.GateOwnerGeneration = "drive-01", "gen-01"
			},
		} {
			t.Run(name, func(t *testing.T) {
				dir := testsupport.TempDir(t)
				var r RebaseReceipt
				mut(&r)
				if err := svc.WriteRebaseReceipt(ctx, dir, r); err == nil {
					t.Errorf("invalid checkpoint written without refusal")
				}
				if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err != nil || found {
					t.Errorf("receipt present after refused write: found=%v err=%v", found, err)
				}
			})
		}
	})
}
