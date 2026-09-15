package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/workspace"
)

type driftingStatusInspector struct {
	inner statusWorkspaceInspector
	drift func()
	calls int
}

// The status consumer must accept both formats emitted by the real renderer,
// including archive paths, without requiring metadata records in the code tree.
func TestStatusArtifactAcceptsRenderedBacklinkFormats(t *testing.T) {
	files := map[string]string{}
	type scenario struct {
		path  string
		kind  string
		valid bool
	}
	var scenarios []scenario
	for mode, link := range map[string]render.LinkContext{
		"local":  {},
		"github": {RepoWebURL: "https://github.com/example/fixture", MetadataBranch: "docket"},
	} {
		for location, prefix := range map[string]string{"active": "", "archive": "2026-09-15-"} {
			for _, kind := range []string{"plan", "results"} {
				for _, slug := range []string{"native-dispatch", "native-dispatch-other"} {
					p := "docs/" + mode + "-" + location + "-" + kind + "-" + slug + ".md"
					change := domain.NewChange(domain.ChangeSpec{ID: 425, Slug: slug, Title: "Native dispatch", Path: "docs/changes/" + location + "/" + prefix + "0425-" + slug + ".md"})
					block, err := render.BacklinkContent(change, link)
					if err != nil {
						t.Fatal(err)
					}
					files[p] = block + "# Artifact\n"
					scenarios = append(scenarios, scenario{p, kind, slug == "native-dispatch"})
				}
			}
		}
	}
	r := newWorkingRepo(t, files)
	reader := NewGitStatusReader(newGitClient(t)).(*gitStatusReader)
	ctx := context.Background()
	pin, err := reader.PinContext(ctx, r.invocation)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range scenarios {
		t.Run(s.path, func(t *testing.T) {
			obs, err := reader.ReadChangeArtifact(ctx, pin, ChangeArtifactTarget{ChangeID: 425, Slug: "native-dispatch", Status: string(domain.StatusDone), Location: string(domain.LocationArchive), Kind: s.kind, Path: s.path})
			if err != nil {
				t.Fatal(err)
			}
			if !obs.Found || !obs.Regular || obs.BacklinkValid != s.valid {
				t.Fatalf("rendered backlink validity want %t: %+v", s.valid, obs)
			}
		})
	}
}

func (d *driftingStatusInspector) Inspect(ctx context.Context, req workspace.InspectRequest) (workspace.Inspection, error) {
	insp, err := d.inner.Inspect(ctx, req)
	d.calls++
	if d.calls == 1 && err == nil {
		d.drift()
	}
	return insp, err
}

func TestStatusReadsUnpublishedArtifactsFromRegisteredOwningWorkspace(t *testing.T) {
	const (
		id       = 425
		slug     = "native-dispatch"
		branch   = "codex/native-dispatch"
		planPath = "docs/superpowers/plans/native-dispatch.md"
	)
	recordPath := "docs/changes/active/0425-native-dispatch.md"
	record := strings.Replace(changeRecord(id, slug, "Native dispatch"), "status: proposed\n", "status: in-progress\nbranch: "+branch+"\nplan: "+planPath+"\n", 1)
	r := newWorkingRepo(t, map[string]string{recordPath: record})
	client := newGitClient(t)
	ctx := context.Background()
	repo, err := client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: r.invocation})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := workspace.NewService(client)
	if err != nil {
		t.Fatal(err)
	}
	target, err := workspace.NewTarget(id, slug, domain.EffectiveBase{Kind: domain.BaseResolved, Branch: "main"}, branch)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.Prepare(ctx, workspace.PrepareRequest{Repository: repo, Remote: originRemote, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	plan := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> [Change 0425](https://example.invalid/docs/changes/active/0425-native-dispatch.md)\n<!-- docket:backlink:end -->\n# Plan\n"
	writeRepoFile(t, prepared.Path, planPath, plan)
	runGit(t, prepared.Path, "add", planPath)
	runGit(t, prepared.Path, "commit", "-m", "add unpublished plan")

	result := Status(ctx, NewGitStatusReader(client), StatusOptions{RepoDir: r.invocation})
	if result.Result != ResultApplied {
		t.Fatalf("status result=%s reason=%s message=%s", result.Result, result.Reason, result.Message)
	}
	for _, finding := range result.Findings {
		if finding.Code == string(FCArtifactMissing) && finding.Path == planPath {
			t.Fatalf("unpublished owning-workspace plan reported missing: %+v", finding)
		}
	}

	runGit(t, r.invocation, "worktree", "remove", "--force", prepared.Path)
	missing := Status(ctx, NewGitStatusReader(client), StatusOptions{RepoDir: r.invocation})
	if missing.Result != ResultApplied {
		t.Fatalf("removed workspace status=%s: %s", missing.Result, missing.Message)
	}
	foundMissing := false
	for _, finding := range missing.Findings {
		if finding.Code == string(FCArtifactMissing) && finding.Path == planPath {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Fatal("removed owning workspace did not produce an artifact finding")
	}
}

func TestStatusArtifactReadRejectsWorkspaceRefDrift(t *testing.T) {
	const id, slug, branch = 425, "native-dispatch", "codex/native-dispatch"
	const planPath = "docs/superpowers/plans/native-dispatch.md"
	r := newWorkingRepo(t, map[string]string{"docs/changes/active/0425-native-dispatch.md": changeRecord(id, slug, "Native dispatch")})
	client := newGitClient(t)
	ctx := context.Background()
	repo, err := client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: r.invocation})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := workspace.NewService(client)
	if err != nil {
		t.Fatal(err)
	}
	target, err := workspace.NewTarget(id, slug, domain.EffectiveBase{Kind: domain.BaseResolved, Branch: "main"}, branch)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.Prepare(ctx, workspace.PrepareRequest{Repository: repo, Remote: originRemote, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	plan := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> [Change 0425](https://example.invalid/docs/changes/active/0425-native-dispatch.md)\n<!-- docket:backlink:end -->\n# Plan\n"
	writeRepoFile(t, prepared.Path, planPath, plan)
	runGit(t, prepared.Path, "add", planPath)
	runGit(t, prepared.Path, "commit", "-m", "plan")
	reader := NewGitStatusReader(client).(*gitStatusReader)
	pin, err := reader.PinContext(ctx, r.invocation)
	if err != nil {
		t.Fatal(err)
	}
	base := runGit(t, r.invocation, "rev-parse", "HEAD")
	reader.workspace = &driftingStatusInspector{inner: svc, drift: func() { runGit(t, r.invocation, "update-ref", "refs/heads/"+branch, base) }}
	_, err = reader.ReadChangeArtifact(ctx, pin, ChangeArtifactTarget{ChangeID: id, Slug: slug, Status: string(domain.StatusInProgress), Location: string(domain.LocationActive), Branch: branch, EffectiveBase: "main", Kind: "plan", Path: planPath})
	if err == nil || !strings.Contains(err.Error(), "changed during artifact read") {
		t.Fatalf("ref drift error=%v", err)
	}
}

func TestTerminalArtifactReadUsesIntegrationAfterFeatureBranchDeletion(t *testing.T) {
	const planPath = "docs/superpowers/plans/terminal.md"
	plan := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> [Change 0425](https://example.invalid/docs/changes/archive/2026-09-15-0425-terminal.md)\n<!-- docket:backlink:end -->\n# Plan\n"
	r := newWorkingRepo(t, map[string]string{planPath: plan})
	client := newGitClient(t)
	ctx := context.Background()
	reader := NewGitStatusReader(client).(*gitStatusReader)
	pin, err := reader.PinContext(ctx, r.invocation)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := reader.ReadChangeArtifact(ctx, pin, ChangeArtifactTarget{ChangeID: 425, Slug: "terminal", Status: string(domain.StatusDone), Location: string(domain.LocationArchive), Branch: "codex/deleted", EffectiveBase: "main", Kind: "plan", Path: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if !obs.Found || !obs.BacklinkValid || obs.SourceKind != sourceIntegration || obs.Revision != pin.IntegrationRevision {
		t.Fatalf("terminal artifact observation=%+v", obs)
	}
}

func TestActiveStackArtifactUsesEffectiveParentBase(t *testing.T) {
	const id, slug, branch = 425, "stack-child", "codex/stack-child"
	const planPath = "docs/superpowers/plans/stack-child.md"
	r := newWorkingRepo(t, map[string]string{})
	runGit(t, r.writer, "checkout", "-b", "codex/parent", "main")
	writeRepoFile(t, r.writer, "parent.txt", "parent\n")
	runGit(t, r.writer, "add", "parent.txt")
	runGit(t, r.writer, "commit", "-m", "parent")
	runGit(t, r.writer, "push", "origin", "codex/parent")
	runGit(t, r.writer, "checkout", "main")
	client := newGitClient(t)
	ctx := context.Background()
	repo, err := client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: r.invocation})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := workspace.NewService(client)
	if err != nil {
		t.Fatal(err)
	}
	target, err := workspace.NewTarget(id, slug, domain.EffectiveBase{Kind: domain.BaseResolved, Branch: "codex/parent"}, branch)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.Prepare(ctx, workspace.PrepareRequest{Repository: repo, Remote: originRemote, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	plan := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> [Change 0425](https://example.invalid/docs/changes/active/0425-stack-child.md)\n<!-- docket:backlink:end -->\n# Plan\n"
	writeRepoFile(t, prepared.Path, planPath, plan)
	runGit(t, prepared.Path, "add", planPath)
	runGit(t, prepared.Path, "commit", "-m", "stack plan")
	reader := NewGitStatusReader(client).(*gitStatusReader)
	pin, err := reader.PinContext(ctx, r.invocation)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := reader.ReadChangeArtifact(ctx, pin, ChangeArtifactTarget{ChangeID: id, Slug: slug, Status: string(domain.StatusInProgress), Location: string(domain.LocationActive), Branch: branch, EffectiveBase: "codex/parent", Kind: "plan", Path: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if !obs.Found || !obs.BacklinkValid || obs.SourceKind != "feature-workspace" {
		t.Fatalf("stack artifact=%+v", obs)
	}
}
