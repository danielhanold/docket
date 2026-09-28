//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"strings"
	"testing"
)

func TestIntegrationRecordOpsADRUnrelatedInvalidRecordProgress(t *testing.T) {
	requireRealGit(t)
	producerPath := groomPath(3, adrProducerSlug)
	targetPath := adrPath("0001", "one")
	rows := []struct {
		name string
		run  func(t *testing.T, repo *gitRepo, node realNode) ADRResult
	}{
		{name: "record", run: func(t *testing.T, repo *gitRepo, node realNode) ADRResult {
			return ADRRecordOp(context.Background(), node.deps, node.dir, validADRRecordRequest())
		}},
		{name: "record with producing change", run: func(t *testing.T, repo *gitRepo, node realNode) ADRResult {
			req := validADRRecordRequest()
			req.Change = &ADRProducingChange{ID: 3, Path: producerPath, Version: blobVersionAt(t, repo.origin, "docket", producerPath)}
			return ADRRecordOp(context.Background(), node.deps, node.dir, req)
		}},
		{name: "supersede", run: func(t *testing.T, repo *gitRepo, node realNode) ADRResult {
			req := validADRReplaceRequest()
			req.Target.Version = blobVersionAt(t, repo.origin, "docket", targetPath)
			return ADRSupersede(context.Background(), node.deps, node.dir, req)
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			repo := newWorkingRepo(t, map[string]string{
				producerPath:        lifecycleChange(3, adrProducerSlug, "in-progress"),
				targetPath:          fixtureADR(1, "one"),
				unrelatedBrokenPath: unrelatedBrokenBytes,
			})
			res := r.run(t, repo, planningDepsFor(t, repo.invocation))
			if res.Result != ResultApplied {
				t.Fatalf("%s beside an unrelated unparseable record = %q (findings %v), want applied", r.name, res.Result, res.Findings)
			}
			assertUnrelatedBrokenIntact(t, repo)
		})
	}
}

// TestIntegrationRecordOpsADRRecordIndexSurfacesUnparseableUnrelatedADR: an ADR record beside an
// unrelated unparseable ADR applies, leaves that ADR's bytes untouched, and
// publishes an index whose repair notice names it — the index used to drop it
// silently (change 0449).
func TestIntegrationRecordOpsADRRecordIndexSurfacesUnparseableUnrelatedADR(t *testing.T) {
	requireRealGit(t)
	const brokenADR, brokenADRBytes = "docs/adrs/0009-broken.md", "---\nid: 9\nslug: broken\n"
	repo := newWorkingRepo(t, map[string]string{
		adrPath("0001", "one"): fixtureADR(1, "one"),
		brokenADR:              brokenADRBytes,
	})
	node := planningDepsFor(t, repo.invocation)
	res := ADRRecordOp(context.Background(), node.deps, node.dir, validADRRecordRequest())
	if res.Result != ResultApplied {
		t.Fatalf("adr record beside an unrelated unparseable ADR = %q (findings %v), want applied", res.Result, res.Findings)
	}
	if got, ok := originFile(t, repo.origin, "docket", brokenADR); !ok || got != brokenADRBytes {
		t.Errorf("unrelated unparseable ADR on origin = %q (present %v), want its exact seeded bytes", got, ok)
	}
	index, ok := originFile(t, repo.origin, "docket", adrsIndexPath)
	if !ok || !strings.Contains(index, "| `"+brokenADR+"` | unclosed-frontmatter |") {
		t.Fatalf("published ADR index lacks the repair notice naming %s:\n%s", brokenADR, index)
	}
}

func TestIntegrationRecordOpsADRUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	producerPath := groomPath(3, adrProducerSlug)
	cases := unrelatedRefusalCases(t, 3, producerPath,
		lifecycleChange(3, adrProducerSlug, "in-progress"), lifecycleChange(3, "dupe", "in-progress"))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			node := planningDepsFor(t, repo.invocation)
			tip := originTip(t, repo.origin, "docket")

			req := validADRRecordRequest()
			req.Change = &ADRProducingChange{ID: 3, Path: producerPath, Version: blobVersionAt(t, repo.origin, "docket", producerPath)}
			res := ADRRecordOp(context.Background(), node.deps, node.dir, req)
			if res.Result == ResultApplied {
				t.Fatalf("adr record applied despite %s on its producing change; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, "", res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused adr record moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}
