//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// Reproduce acceptance 431 with the production reader and workspace service:
// another change advances the shared metadata branch during a plan task.
func TestIntegrationAgentMetadataAllowsUnrelatedAdvance(t *testing.T) {
	f := setupRebaseFixtureStatus(t, planRepoModes()[0], "in-progress")
	ctx := context.Background()
	pin, err := f.deps.Reader.PinContext(ctx, f.repo.invocation)
	if err != nil {
		t.Fatal(err)
	}
	a, deps, root := reviewRealAssignment(t, false)
	template, err := os.ReadFile(a.Resources[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	templateRelative := ".agents/skills/docket-implement-next/results-template.md"
	writeRepoFile(t, f.wp, templateRelative, string(template))
	runGit(t, f.wp, "add", templateRelative)
	runGit(t, f.wp, "commit", "-m", "fixture: install planner template")
	a.Resources[0].Path = filepath.Join(f.wp, templateRelative)
	worktree, err := f.deps.Client.DiscoverWorktree(ctx, gitcli.DiscoverOptions{InvocationPath: f.wp})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := codexcontract.ObserveRootIdentity(worktree.Root, worktree.GitDir)
	if err != nil {
		t.Fatal(err)
	}
	a.ChangeID, a.ChangePath, a.MetadataRevision = f.id, groomPath(f.id, f.slug), pin.MetadataRevision
	a.Primary, a.Feature, a.CommonDir = f.gitrepo.PrimaryWorktree, worktree.Root, f.gitrepo.CommonDir
	a.ReadRoots = append(a.ReadRoots, worktree.Root)
	a.Branch, a.EntryHEAD, a.RootIdentity = f.target.FeatureBranch(), runGit(t, f.wp, "rev-parse", "HEAD"), &identity
	v := NewAgentWorkspaceValidator(f.deps, WorkspaceDeps{Service: f.svc})
	deps.Workspace = v
	for _, stage := range []string{"dispatch", "entry"} {
		if result := reviewCheck(t, a, deps, root, stage); result.Result != ResultApplied {
			t.Fatalf("%s: %s", stage, result.Reason)
		}
	}
	f.repo.writerAdvance(t, "docket", map[string]string{groomPath(413, "other"): lifecycleChange(413, "other", "in-progress"), "docs/changes/BOARD.md": "another change claimed\n"})
	writeRepoFile(t, f.wp, "docs/plan.md", "# Plan\n")
	runGit(t, f.wp, "add", "docs/plan.md")
	runGit(t, f.wp, "commit", "-m", "planner artifact")
	if result := reviewCheck(t, a, deps, root, "active"); result.Result != ResultApplied {
		t.Fatalf("post-commit active validation rejected unrelated metadata: %s", result.Reason)
	}
}

func TestIntegrationAgentMetadataRejectsAssignedDrift(t *testing.T) {
	f := setupRebaseFixtureStatus(t, planRepoModes()[0], "in-progress")
	ctx := context.Background()
	path := groomPath(f.id, f.slug)
	record := strings.Replace(lifecycleChange(f.id, f.slug, "in-progress"), "depends_on: []", "depends_on: [6]", 1)
	record = strings.Replace(record, "stacked_on:\n", "stacked_on: 7\n", 1)
	record = strings.Replace(record, "spec:\n", "spec: docs/spec.md\n", 1)
	record = strings.Replace(record, "adrs: []", "adrs: [8]", 1)
	base := map[string]string{path: record, "docs/spec.md": "# Assigned design\n", groomPath(6, "dependency"): lifecycleChange(6, "dependency", "done"), groomPath(7, "parent"): lifecycleChange(7, "parent", "done"), "docs/adrs/0008-decision.md": fixtureADR(8, "decision")}
	base[groomPath(6, "dependency")] = strings.Replace(base[groomPath(6, "dependency")], "depends_on: []", "depends_on: [9]", 1)
	ancestor := strings.Replace(lifecycleChange(9, "ancestor", "done"), "spec:\n", "spec: docs/ancestor-spec.md\n", 1)
	base[groomPath(9, "ancestor")] = strings.Replace(ancestor, "adrs: []", "adrs: [10]", 1)
	base["docs/ancestor-spec.md"] = "# Transitive design\n"
	base["docs/adrs/0010-ancestor.md"] = fixtureADR(10, "ancestor")
	f.repo.writerAdvance(t, "docket", base)
	for _, changed := range []string{path, "docs/spec.md", groomPath(6, "dependency"), groomPath(7, "parent"), "docs/adrs/0008-decision.md", groomPath(9, "ancestor"), "docs/ancestor-spec.md", "docs/adrs/0010-ancestor.md"} {
		t.Run(changed, func(t *testing.T) {
			pin, err := f.deps.Reader.PinContext(ctx, f.repo.invocation)
			if err != nil {
				t.Fatal(err)
			}
			a := codexcontract.Assignment{ChangeID: f.id, ChangePath: path, MetadataRevision: pin.MetadataRevision, Feature: f.wp, Branch: f.target.FeatureBranch()}
			v := NewAgentWorkspaceValidator(f.deps, WorkspaceDeps{Service: f.svc})
			if err := v.ValidateAgentWorkspace(ctx, a, f.repo.invocation); err != nil {
				t.Fatalf("positive control: %v", err)
			}
			f.repo.writerAdvance(t, "docket", map[string]string{changed: base[changed] + "\nChanged input.\n"})
			if err := v.ValidateAgentWorkspace(ctx, a, f.repo.invocation); err == nil || err.Error() != "metadata-inputs-mismatch" {
				t.Fatalf("changed assigned input must refuse: %v", err)
			}
			f.repo.writerAdvance(t, "docket", base)
		})
	}
	// An identical tree on unrelated history must not launder the assignment pin.
	pin, err := f.deps.Reader.PinContext(ctx, f.repo.invocation)
	if err != nil {
		t.Fatal(err)
	}
	tree := runGit(t, f.repo.writer, "rev-parse", "docket^{tree}")
	orphan := runGit(t, f.repo.writer, "commit-tree", tree, "-m", "unrelated metadata history")
	a := codexcontract.Assignment{ChangeID: f.id, ChangePath: path, MetadataRevision: orphan, Feature: f.wp, Branch: f.target.FeatureBranch()}
	// Make the unrelated commit available in the invocation clone without moving docket.
	runGit(t, f.repo.invocation, "fetch", f.repo.writer, orphan)
	v := NewAgentWorkspaceValidator(f.deps, WorkspaceDeps{Service: f.svc})
	if err := v.ValidateAgentWorkspace(ctx, a, f.repo.invocation); err == nil || err.Error() != "metadata-history-diverged" {
		t.Fatalf("unrelated history at %s accepted: %v", pin.MetadataRevision, err)
	}
}

type advancingAgentWorkspace struct {
	WorkspaceService
	advance func()
}

func (w advancingAgentWorkspace) Inspect(ctx context.Context, req workspace.InspectRequest) (workspace.Inspection, error) {
	out, err := w.WorkspaceService.Inspect(ctx, req)
	if err == nil {
		w.advance()
	}
	return out, err
}

func TestIntegrationAgentMetadataRechecksConcurrentAdvance(t *testing.T) {
	f := setupRebaseFixtureStatus(t, planRepoModes()[0], "in-progress")
	ctx := context.Background()
	for _, related := range []bool{false, true} {
		pin, err := f.deps.Reader.PinContext(ctx, f.repo.invocation)
		if err != nil {
			t.Fatal(err)
		}
		a := codexcontract.Assignment{ChangeID: f.id, ChangePath: groomPath(f.id, f.slug), MetadataRevision: pin.MetadataRevision, Feature: f.wp, Branch: f.target.FeatureBranch()}
		service := advancingAgentWorkspace{WorkspaceService: f.svc, advance: func() {
			path, body := groomPath(413, "other"), fixtureChange(413, "other")
			if related {
				path, body = a.ChangePath, lifecycleChange(f.id, f.slug, "in-progress")+"\nAssignment changed during inspection.\n"
			}
			f.repo.writerAdvance(t, "docket", map[string]string{path: body})
		}}
		err = NewAgentWorkspaceValidator(f.deps, WorkspaceDeps{Service: service}).ValidateAgentWorkspace(ctx, a, f.repo.invocation)
		if related && (err == nil || err.Error() != "metadata-inputs-mismatch") {
			t.Fatalf("concurrent assigned edit accepted: %v", err)
		}
		if !related && err != nil {
			t.Fatalf("concurrent unrelated edit refused: %v", err)
		}
	}
}
