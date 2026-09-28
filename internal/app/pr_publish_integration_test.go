//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_finalizeops.sh (prefix ^TestIntegrationFinalizeOps).

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
)

// TestIntegrationFinalizeOpsPRPublishAcceptsSkippedEvidenceAtExactHead: a build.gate: off repository's
// truthful skipped evidence certifying the exact feature head passes PRPublish's
// evidence conjunct — the operation proceeds PAST it (VerdictSkipped is accepted
// exactly as VerdictVerified). Any later refusal is not the evidence conjunct;
// reverting the green-or-skipped acceptance would refuse here with
// ReasonPREvidenceUnverified, so this pins the verify-site change. (PR-body
// weaving of a skipped block is a separate concern: evidence.Upsert is green-only
// today, so the operation still fails later at body assembly — see the change
// notes.)
func TestIntegrationFinalizeOpsPRPublishAcceptsSkippedEvidenceAtExactHead(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	reader := prReader(t)
	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	deps := workspaceDepsFor(t, reader)
	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Authored prose.\n", EvidenceRecord: prSkippedEvidenceBytes(t, prHead)})
	if res.Reason == ReasonPREvidenceUnverified {
		t.Fatalf("skipped evidence at the exact head was refused at the evidence conjunct: %q", res.Message)
	}
}

// TestIntegrationFinalizeOpsPRPublishPreEffectValidationIsScoped pins change 0449's pre-effect rule
// for PR publication (spec: "B must pass relevant validation before an external
// effect, while A's unrelated findings cannot veto it"): an error on B itself or
// on a structural subject of B (a depends_on target) refuses before
// EnsurePullRequest, while unrelated records carrying errors — one parseable but
// invalid, one unparseable — never veto the publication.
func TestIntegrationFinalizeOpsPRPublishPreEffectValidationIsScoped(t *testing.T) {
	requireRealGit(t)
	badType := func(src string) string {
		out := strings.Replace(src, "type: feat\n", "type: 'Not A Token'\n", 1)
		if out == src {
			t.Fatal("defect fixture did not rewrite the record's type; the fixture shape changed")
		}
		return out
	}
	b := inProgressChangeBlob(7, "widget", "v7", "")
	unrelatedInvalid := StatusBlob{
		Kind: repository.KindChange, Location: repository.LocationActive,
		Path: groomPath(30, "a-invalid"), Version: "v30",
		Data: []byte(badType(lifecycleChange(30, "a-invalid", "proposed"))),
	}
	unrelatedBroken := StatusBlob{
		Kind: repository.KindChange, Location: repository.LocationActive,
		Path: unrelatedBrokenPath, Version: "v99", Data: []byte(unrelatedBrokenBytes),
	}
	badB := b
	badB.Data = []byte(badType(string(b.Data)))
	depPath := "docs/changes/archive/2026-08-01-0008-dep.md"
	withDep := b
	withDep.Data = []byte(strings.Replace(string(b.Data), "depends_on: []\n", "depends_on: [8]\n", 1))
	if string(withDep.Data) == string(b.Data) {
		t.Fatal("dependency fixture did not rewrite depends_on; the fixture shape changed")
	}
	badDep := StatusBlob{
		Kind: repository.KindChange, Location: repository.LocationArchive,
		Path: depPath, Version: "v8", Data: []byte(badType(fixtureArchivedDone(8, "dep"))),
	}

	publish := func(t *testing.T, corpus []StatusBlob) (PRPublishResult, *fakeGitHub) {
		t.Helper()
		reader := &fakeReader{pin: mainPin(t), corpus: corpus}
		gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
		res := PRPublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
			newWorkingRepo(t, nil).invocation,
			PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Authored prose.\n", EvidenceRecord: prEvidenceBytes(t, prHead)})
		return res, gh
	}

	t.Run("unrelated-errors-do-not-veto", func(t *testing.T) {
		corpus := []StatusBlob{b, unrelatedInvalid, unrelatedBroken}
		// Non-vacuity: the unrelated parseable record genuinely carries an error
		// finding in the built report, so only the relevance rule lets B through.
		inputs, _ := parseCorpus(corpus)
		build, err := repository.BuildSnapshot(repository.BuildInput{Config: mainPin(t).Config.Effective, Documents: inputs})
		if err != nil {
			t.Fatalf("BuildSnapshot: %v", err)
		}
		poisoned := false
		for _, f := range build.Report.Findings() {
			if f.Severity == domain.SeverityError && f.Entity.Path == unrelatedInvalid.Path {
				poisoned = true
			}
		}
		if !poisoned {
			t.Fatalf("the unrelated record %s carries no error finding; the non-veto row would be vacuous", unrelatedInvalid.Path)
		}
		res, gh := publish(t, corpus)
		if res.Result != ResultApplied || len(gh.ensureCalls) != 1 {
			t.Fatalf("publish beside unrelated invalid records = %q (reason %q msg %q findings %+v, %d ensure calls), want applied with one ensure",
				res.Result, res.Reason, res.Message, res.Findings, len(gh.ensureCalls))
		}
	})

	for _, tc := range []struct {
		name   string
		corpus []StatusBlob
		names  string
	}{
		{"defective-B-refuses-before-effect", []StatusBlob{badB, unrelatedInvalid, unrelatedBroken}, b.Path},
		{"defective-dependency-refuses-before-effect", []StatusBlob{withDep, badDep, unrelatedInvalid, unrelatedBroken}, depPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, gh := publish(t, tc.corpus)
			if res.Result == ResultApplied || res.Result == ResultNoOp {
				t.Fatalf("publish applied despite a relevant defect (%s): %q", tc.names, res.Result)
			}
			if res.Reason != ReasonPRRecordInvalid {
				t.Fatalf("reason = %q (msg %q), want %q", res.Reason, res.Message, ReasonPRRecordInvalid)
			}
			if len(gh.ensureCalls) != 0 {
				t.Fatalf("EnsurePullRequest invoked %d time(s) despite a relevant defect; want 0", len(gh.ensureCalls))
			}
			named := false
			for _, f := range res.Findings {
				if f.Path == unrelatedInvalid.Path || f.Path == unrelatedBrokenPath {
					t.Errorf("refusal carries an unrelated record's finding %+v", f)
				}
				if f.Path == tc.names {
					named = true
				}
			}
			if !named {
				t.Errorf("refusal does not name the defective record %s: findings %+v", tc.names, res.Findings)
			}
		})
	}
}
