//go:build integration

package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/testsupport"
)

// doneCorpusChangeRecord is a `done` archived change carrying Spec, Plan, and
// Results rows. Under change 0417 a done change's Plan/Results rows pin the
// INTEGRATION branch while its Spec row stays on the metadata branch — the
// distinction the corpus link context (readCheckCorpus) must carry.
const doneCorpusChangeRecord = "---\n" +
	"id: 1\n" +
	"slug: example\n" +
	"title: Example change\n" +
	"status: done\n" +
	"priority: medium\n" +
	"type: feature\n" +
	"created: 2026-08-30\n" +
	"updated: 2026-08-30\n" +
	"spec: docs/superpowers/specs/2026-08-30-example-design.md\n" +
	"plan: docs/superpowers/plans/2026-08-30-example.md\n" +
	"results: docs/results/2026-08-30-example-results.md\n" +
	"branch: feat/example\n" +
	"pr: https://github.com/acme/widgets/pull/7\n" +
	"reconciled: true\n" +
	"---\n\n## Why\n\nbody\n"

// TestIntegrationRepoCheckCorpusPinsDoneChangeToIntegrationBranch guards the
// readCheckCorpus link-context wiring against a destructive regression (change
// 0417). readCheckCorpus builds corpus.link from the setup context's integration
// branch; that context feeds render.ArtifactBlockContent, which the derived-view
// comparison (artifactLinkFindings) uses to detect artifact-links drift and which
// migrate would then rewrite. If the corpus pin dropped IntegrationBranch, a done
// change's Plan/Results rows would resolve onto the metadata branch (blob/docket),
// disagree with the persisted body, report false stale drift, and migrate would
// rewrite them back to the metadata branch — reintroducing exactly the defect
// 0417 fixes.
//
// The existing hermetic derived-view unit tests build corpus.link inline with a
// metadata-only LinkContext (no RepoWebURL, no IntegrationBranch), so none of them
// exercise this wiring. This test drives readCheckCorpus over a real repository
// whose origin resolves to a GitHub web URL (RepoWebURL non-empty) so the rendered
// rows resolve to blob URLs, then asserts the branch each row lands on.
func TestIntegrationRepoCheckCorpusPinsDoneChangeToIntegrationBranch(t *testing.T) {
	dir := testsupport.TempDir(t)
	runGit(t, dir, "init", "-q")
	gitIdentity(t, dir)
	// The origin URL only needs to PARSE as a GitHub remote (githubWebURL); the
	// corpus is read from local objects, so the remote is never contacted.
	runGit(t, dir, "remote", "add", "origin", "https://github.com/acme/widgets.git")

	writeRepoFile(t, dir, "docs/changes/archive/2026-08-30-0001-example.md", doneCorpusChangeRecord)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "corpus with a done change")
	tip := runGit(t, dir, "rev-parse", "HEAD")

	client := newGitClient(t)
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: dir})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	cfg := derivedTestConfig()
	sc := setupContext{
		repo:              repo,
		cfg:               cfg,
		integrationBranch: "main",
		metadataTip:       tip,
	}

	corpus, err := readCheckCorpus(context.Background(), client, sc)
	if err != nil {
		t.Fatalf("readCheckCorpus: %v", err)
	}
	// The link context must have derived a non-empty RepoWebURL from the GitHub
	// origin, or the rendered rows would fall back to bare backtick paths and the
	// branch assertions below would be vacuous.
	if corpus.link.RepoWebURL == "" {
		t.Fatal("corpus link RepoWebURL is empty; the GitHub origin was not resolved, so branch assertions would be vacuous")
	}

	snap, ok := buildCorpusSnapshot(cfg, corpus.records)
	if !ok {
		t.Fatal("buildCorpusSnapshot failed for the done-change corpus")
	}
	c, outcome := snap.Change(1)
	if outcome != 0 { // domain.LookupFound == 0
		t.Fatalf("done change absent from snapshot (lookup outcome %d)", outcome)
	}

	body, err := render.ArtifactBlockContent(c, snap, corpus.link)
	if err != nil {
		t.Fatalf("render artifact block: %v", err)
	}

	// The done change's Plan and Results rows must resolve onto the integration
	// branch (main), NOT the metadata branch (docket). Dropping IntegrationBranch
	// from the corpus pin sends both back to blob/docket — the destructive
	// regression this guard exists to catch.
	planURL := "/blob/main/docs/superpowers/plans/2026-08-30-example.md"
	resultsURL := "/blob/main/docs/results/2026-08-30-example-results.md"
	if !strings.Contains(body, planURL) {
		t.Errorf("Plan row does not resolve onto the integration branch (want %q); rendered block:\n%s", planURL, body)
	}
	if !strings.Contains(body, resultsURL) {
		t.Errorf("Results row does not resolve onto the integration branch (want %q); rendered block:\n%s", resultsURL, body)
	}

	// The Spec row stays on the metadata branch, where the record lives.
	specURL := "/blob/" + reposetup.MetadataBranchName + "/docs/superpowers/specs/2026-08-30-example-design.md"
	if !strings.Contains(body, specURL) {
		t.Errorf("Spec row does not resolve onto the metadata branch (want %q); rendered block:\n%s", specURL, body)
	}
	// Guard the assertion itself: Plan/Results must not sit on the metadata branch.
	if strings.Contains(body, "/blob/"+reposetup.MetadataBranchName+"/docs/superpowers/plans/") ||
		strings.Contains(body, "/blob/"+reposetup.MetadataBranchName+"/docs/results/") {
		t.Errorf("Plan/Results resolved onto the metadata branch; rendered block:\n%s", body)
	}
}
