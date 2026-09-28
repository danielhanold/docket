//go:build integration

package transaction

import (
	"context"
	"sort"
	"strings"
	"testing"
)

// TestIntegrationTxnRecoveryKeyedCommitCarriesFiveTrailers proves a keyed apply writes exactly the five
// engine trailers and an unkeyed apply writes exactly the three always-present
// ones — never one of the request pair alone.
func TestIntegrationTxnRecoveryKeyedCommitCarriesFiveTrailers(t *testing.T) {
	t.Run("keyed", func(t *testing.T) {
		r := newMainModeRepos(t)
		client, repo := r.discover(t)
		eng := newEngine(t, client)

		res, err := eng.Execute(context.Background(), Request{
			Repository: repo, Remote: "origin", TargetRef: r.Target,
			Idempotency: keyReq(), Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if res.Disposition != DispositionApplied {
			t.Fatalf("disposition = %q, want applied (findings %v)", res.Disposition, res.Findings)
		}
		got := trailerKeys(t, r.Origin, res.AppliedCommit)
		want := []string{"Docket-Operation", "Docket-Request-Digest", "Docket-Request-ID", "Docket-Result", "Docket-Transaction-ID"}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("keyed trailer keys = %v, want %v", got, want)
		}
		if res.RequestID != keyReq().RequestID {
			t.Errorf("result request id = %q, want %q", res.RequestID, keyReq().RequestID)
		}
	})

	t.Run("unkeyed", func(t *testing.T) {
		r := newMainModeRepos(t)
		client, repo := r.discover(t)
		eng := newEngine(t, client)

		res, err := eng.Execute(context.Background(), Request{
			Repository: repo, Remote: "origin", TargetRef: r.Target,
			Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		got := trailerKeys(t, r.Origin, res.AppliedCommit)
		want := []string{"Docket-Operation", "Docket-Result", "Docket-Transaction-ID"}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("unkeyed trailer keys = %v, want %v", got, want)
		}
		for _, k := range got {
			if k == "Docket-Request-ID" || k == "Docket-Request-Digest" {
				t.Errorf("unkeyed commit carries request trailer %q", k)
			}
		}
	})
}

// TestIntegrationTxnRecoveryKeyedReplayReturnsOriginalReceipt proves a repeated keyed request after a
// successful apply returns the ORIGINAL receipt bytes and the authoritative commit
// with no new commit, even after the corpus has moved on.
func TestIntegrationTxnRecoveryKeyedReplayReturnsOriginalReceipt(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	// A distinctive canonical receipt carrying the allocated id.
	planted := []byte(`{"id":"0003"}`)
	op := createOp(thirdChangePath, thirdChange())
	op.receipt = planted

	res1, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	if res1.Disposition != DispositionApplied {
		t.Fatalf("first disposition = %q, want applied", res1.Disposition)
	}
	if string(res1.Receipt) != string(planted) {
		t.Fatalf("first receipt = %q, want %q", res1.Receipt, planted)
	}
	original := res1.AppliedCommit

	// Move the corpus on with an unrelated unkeyed apply on top of the keyed commit.
	res2, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: createOp("docs/changes/active/0004-fourth-change.md", corpusChange(4, "fourth-change", "proposed")),
	})
	if err != nil || res2.Disposition != DispositionApplied {
		t.Fatalf("moving-on Execute: disposition %q err %v", res2.Disposition, err)
	}
	movedTip := r.originTip(t)

	// Replay the first key. It must return the ORIGINAL receipt and commit, add no
	// commit, and never re-run the operation.
	replayOp := createOp(thirdChangePath, thirdChange())
	replayOp.receipt = []byte(`{"id":"9999"}`) // a fresh reconstruction would surface this
	res3, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: replayOp,
	})
	if err != nil {
		t.Fatalf("replay Execute: %v", err)
	}
	if res3.Disposition != DispositionAlreadyApplied {
		t.Fatalf("replay disposition = %q, want already-applied", res3.Disposition)
	}
	if res3.AppliedCommit != original {
		t.Errorf("replay applied commit = %q, want original %q", res3.AppliedCommit, original)
	}
	if string(res3.Receipt) != string(planted) {
		t.Errorf("replay receipt = %q, want ORIGINAL %q", res3.Receipt, planted)
	}
	if replayOp.calls != 0 {
		t.Errorf("replay consulted the operation %d times; a replay must not replan", replayOp.calls)
	}
	if r.originTip(t) != movedTip {
		t.Error("replay added a commit to origin")
	}
	if !transactionsEmpty(t, repo) {
		t.Error("transactions root not empty after a replay")
	}
}

// TestIntegrationTxnRecoveryRequestIDReusedDifferentDigest proves the same request ID with a different
// digest is invalid-input "request-id-reused".
func TestIntegrationTxnRecoveryRequestIDReusedDifferentDigest(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	// A well-formed keyed commit for our request id but a DIFFERENT digest.
	plantCommit(t, r, engineBlockMessage("prior request", "deadbeefcafe", "other.op",
		keyReq().RequestID, string(otherDigest()), b64(validReceipt())))

	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
	})
	if err == nil {
		t.Fatal("reused request id: want error")
	}
	assertFailureKind(t, err, KindInvalidInput)
	f, _ := AsFailure(err)
	if f.Detail != "request-id-reused" {
		t.Errorf("detail = %q, want request-id-reused", f.Detail)
	}
	if res.Disposition != DispositionFailed {
		t.Errorf("disposition = %q, want failed", res.Disposition)
	}
}

// TestIntegrationTxnRecoveryDuplicateRequestIDIsInvalidState proves two history commits carrying the same
// request id are invalid-state — the engine never picks a winner by commit order.
func TestIntegrationTxnRecoveryDuplicateRequestIDIsInvalidState(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	plantCommit(t, r, engineBlockMessage("first", "aaaa1111", "test.op",
		keyReq().RequestID, string(validDigest()), b64(validReceipt())))
	plantCommit(t, r, engineBlockMessage("second", "bbbb2222", "test.op",
		keyReq().RequestID, string(validDigest()), b64(validReceipt())))

	_, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
	})
	if err == nil {
		t.Fatal("duplicate request id: want error")
	}
	assertFailureKind(t, err, KindInvalidState)
}

// TestIntegrationTxnRecoveryMalformedResultIsInvalidState proves a malformed Docket-Result on a commit
// bearing our request id is invalid-state, for each malformation class.
func TestIntegrationTxnRecoveryMalformedResultIsInvalidState(t *testing.T) {
	// >4096 decoded bytes, canonical JSON.
	big := `{"x":"` + strings.Repeat("y", 5000) + `"}`
	cases := []struct {
		name   string
		result string // the raw Docket-Result trailer value
	}{
		{"bad-base64", "!!!not-base64!!!"},
		{"non-canonical-json", b64([]byte(`{ "ok" : true }`))}, // insignificant whitespace
		{"oversize", b64([]byte(big))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newMainModeRepos(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)

			plantCommit(t, r, engineBlockMessage("bad receipt", "cccc3333", "test.op",
				keyReq().RequestID, string(validDigest()), c.result))

			_, err := eng.Execute(context.Background(), Request{
				Repository: repo, Remote: "origin", TargetRef: r.Target,
				Idempotency: keyReq(), Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
			})
			if err == nil {
				t.Fatalf("%s: want error", c.name)
			}
			assertFailureKind(t, err, KindInvalidState)
		})
	}
}

// TestIntegrationTxnRecoveryRequestIDInProseDoesNotMatch proves a Docket-Request-ID line living in body
// prose (outside the final trailer block) never matches: the engine proceeds and
// applies, it does not replay.
func TestIntegrationTxnRecoveryRequestIDInProseDoesNotMatch(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	// The request-id appears in an EARLIER paragraph with trailing prose, and the
	// final paragraph is ordinary prose — so git parses no trailers at all.
	message := "prose commit\n\n" +
		"Docket-Request-ID: " + keyReq().RequestID + " is only mentioned in prose here.\n\n" +
		"An ordinary closing paragraph with no trailers.\n"
	planted := plantCommit(t, r, message)
	if keys := trailerKeys(t, r.Origin, planted); len(keys) != 0 {
		t.Fatalf("planted prose commit parsed trailers %v, want none", keys)
	}

	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Disposition != DispositionApplied {
		t.Fatalf("disposition = %q, want applied (prose must not match)", res.Disposition)
	}
}

// TestIntegrationTxnRecoveryKeyedReplayFoundDeepInHistory proves the ancestry scan has no depth window:
// the key's commit buried under 30 later commits is still found.
func TestIntegrationTxnRecoveryKeyedReplayFoundDeepInHistory(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)

	planted := []byte(`{"id":"deep"}`)
	keyedCommit := plantCommit(t, r, engineBlockMessage("buried apply", "dddd4444", "test.op",
		keyReq().RequestID, string(validDigest()), b64(planted)))

	// Bury it under 30 later commits.
	branch := r.short()
	for i := 0; i < 30; i++ {
		hgitOut(t, r.Writer, "commit", "-q", "--allow-empty", "-m", "later")
	}
	hgitOut(t, r.Writer, "push", "-q", "origin", branch)

	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Disposition != DispositionAlreadyApplied {
		t.Fatalf("disposition = %q, want already-applied", res.Disposition)
	}
	if res.AppliedCommit != keyedCommit {
		t.Errorf("applied commit = %q, want buried %q", res.AppliedCommit, keyedCommit)
	}
	if string(res.Receipt) != string(planted) {
		t.Errorf("receipt = %q, want %q", res.Receipt, planted)
	}
}
