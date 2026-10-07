package reposetup

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

// repoOnlyForTest is the live registry list, so the tests follow the schema.
func repoOnlyForTest() []string { return config.RepoOnlyPaths() }

// mustResolve proves bytes resolve as the repository (and optional
// repository-local) layer the switch will hand to config.
func mustResolve(t *testing.T, committed, local []byte) {
	t.Helper()
	sources := []config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: committed}}
	if local != nil {
		sources = append(sources, config.Source{Layer: config.LayerRepositoryLocal, Name: ".docket.local.yml", Data: local})
	}
	if _, diags, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: "main"}); err != nil {
		t.Fatalf("config.Resolve refused the output: %v (diagnostics %v)\ncommitted:\n%s\nlocal:\n%s", err, diags, committed, local)
	}
}

func leafValues(t *testing.T, src []byte, paths ...string) map[string]string {
	t.Helper()
	got, err := ConfigLeafValues(src, paths)
	if err != nil {
		t.Fatalf("ConfigLeafValues: %v", err)
	}
	return got
}

func TestFoldPrivateConfigMergesLeaves(t *testing.T) {
	committed := []byte("# team settings\nfinalize:\n  gate: local\n  test_command: make\n")
	local := []byte("finalize:\n  test_command: make quick\n")
	folded, dropped, err := FoldPrivateConfig(committed, local, repoOnlyForTest())
	if err != nil {
		t.Fatalf("FoldPrivateConfig: %v", err)
	}
	if len(dropped) != 0 {
		t.Errorf("dropped = %v, want none", dropped)
	}
	want := map[string]string{"finalize.gate": "local", "finalize.test_command": "make quick", "visibility": "private"}
	if got := leafValues(t, folded, "finalize.gate", "finalize.test_command", "visibility"); !reflect.DeepEqual(got, want) {
		t.Errorf("folded leaves = %v, want %v\n%s", got, want, folded)
	}
	if !containsLine(folded, "# team settings") {
		t.Errorf("committed comment lost:\n%s", folded)
	}
	mustResolve(t, folded, nil)
}

func containsLine(b []byte, line string) bool { return hasLine(b, line) }

func TestFoldPrivateConfigDropsRepoOnlyLocalKeys(t *testing.T) {
	committed := []byte("integration_branch: main\n")
	local := []byte("integration_branch: develop\nreclaim:\n  auto: true\n")
	folded, dropped, err := FoldPrivateConfig(committed, local, repoOnlyForTest())
	if err != nil {
		t.Fatalf("FoldPrivateConfig: %v", err)
	}
	want := map[string]string{"integration_branch": "main", "reclaim.auto": "true", "visibility": "private"}
	if got := leafValues(t, folded, "integration_branch", "reclaim.auto", "visibility"); !reflect.DeepEqual(got, want) {
		t.Errorf("folded leaves = %v, want %v\n%s", got, want, folded)
	}
	if !slices.Equal(dropped, []string{"integration_branch"}) {
		t.Errorf("dropped = %v, want [integration_branch]", dropped)
	}
	mustResolve(t, folded, nil)
}

func TestFoldPrivateConfigDropsAgentPins(t *testing.T) {
	committed := []byte("agents:\n  claude:\n    build-standard:\n      model: opus\nreclaim:\n  auto: true\n")
	local := []byte("agents:\n  codex:\n    review-lean:\n      effort: high\n")
	folded, dropped, err := FoldPrivateConfig(committed, local, repoOnlyForTest())
	if err != nil {
		t.Fatalf("FoldPrivateConfig: %v", err)
	}
	wantDropped := []string{"agents.claude.build-standard.model", "agents.codex.review-lean.effort"}
	if !slices.Equal(dropped, wantDropped) {
		t.Errorf("dropped = %v, want %v", dropped, wantDropped)
	}
	if got := leafValues(t, folded, "agents"); len(got) != 0 {
		t.Errorf("empty agents mapping kept: %v\n%s", got, folded)
	}
	mustResolve(t, folded, nil)

	empty, dropped, err := FoldPrivateConfig(nil, nil, repoOnlyForTest())
	if err != nil || len(dropped) != 0 {
		t.Fatalf("FoldPrivateConfig(nil, nil) = %q, %v, %v", empty, dropped, err)
	}
	if string(empty) != "visibility: private\n" {
		t.Errorf("FoldPrivateConfig(nil, nil) = %q, want %q", empty, "visibility: private\n")
	}
	if _, _, err := FoldPrivateConfig([]byte("- a\n"), nil, repoOnlyForTest()); err == nil {
		t.Errorf("a sequence root folded without error")
	}
	if _, _, err := FoldPrivateConfig(nil, []byte("- a\n"), repoOnlyForTest()); err == nil {
		t.Errorf("a sequence local root folded without error")
	}
}

func TestSplitPrivateConfigRoutesSavedLocalLeaves(t *testing.T) {
	private := []byte("visibility: private\nbuild:\n  test_command: make\nreclaim:\n  auto: true\nintegration_branch: main\n")
	localKeys := []byte("reclaim:\n  auto: false\nintegration_branch: x\n")
	committed, local, err := SplitPrivateConfig(private, localKeys, repoOnlyForTest())
	if err != nil {
		t.Fatalf("SplitPrivateConfig: %v", err)
	}
	wantC := map[string]string{"build.test_command": "make", "integration_branch": "main", "visibility": "shared"}
	if got := leafValues(t, committed, "build.test_command", "integration_branch", "visibility", "reclaim.auto", "reclaim"); !reflect.DeepEqual(got, wantC) {
		t.Errorf("committed leaves = %v, want %v\n%s", got, wantC, committed)
	}
	if string(local) != "reclaim:\n  auto: true\n" {
		t.Errorf("local = %q, want %q", local, "reclaim:\n  auto: true\n")
	}
	mustResolve(t, committed, local)
}

func TestSplitPrivateConfigBornPrivate(t *testing.T) {
	committed, local, err := SplitPrivateConfig([]byte("visibility: private\nreclaim:\n  auto: true\n"), nil, repoOnlyForTest())
	if err != nil {
		t.Fatalf("SplitPrivateConfig: %v", err)
	}
	if local != nil {
		t.Errorf("born-private local = %q, want nil", local)
	}
	want := map[string]string{"reclaim.auto": "true", "visibility": "shared"}
	if got := leafValues(t, committed, "reclaim.auto", "visibility"); !reflect.DeepEqual(got, want) {
		t.Errorf("committed leaves = %v, want %v", got, want)
	}
	mustResolve(t, committed, nil)
}

func TestFoldSplitRoundTrip(t *testing.T) {
	committed := []byte("integration_branch: main\nfinalize:\n  gate: local\n  test_command: make\n")
	local := []byte("finalize:\n  test_command: make quick\nreclaim:\n  auto: true\n")
	folded, _, err := FoldPrivateConfig(committed, local, repoOnlyForTest())
	if err != nil {
		t.Fatalf("FoldPrivateConfig: %v", err)
	}
	c2, l2, err := SplitPrivateConfig(folded, local, repoOnlyForTest())
	if err != nil {
		t.Fatalf("SplitPrivateConfig: %v", err)
	}
	paths := []string{"integration_branch", "finalize.gate", "finalize.test_command", "reclaim.auto", "visibility"}
	wantC := map[string]string{"integration_branch": "main", "finalize.gate": "local", "visibility": "shared"}
	if got := leafValues(t, c2, paths...); !reflect.DeepEqual(got, wantC) {
		t.Errorf("committed after round trip = %v, want %v\n%s", got, wantC, c2)
	}
	wantL := map[string]string{"finalize.test_command": "make quick", "reclaim.auto": "true"}
	if got := leafValues(t, l2, paths...); !reflect.DeepEqual(got, wantL) {
		t.Errorf("local after round trip = %v, want %v\n%s", got, wantL, l2)
	}
	mustResolve(t, c2, l2)
}

func TestConfigLeafValues(t *testing.T) {
	src := []byte("finalize:\n  gate: [local, ci] # note\n  test_command: \"make x\"\n")
	got := leafValues(t, src, "finalize.gate", "finalize.test_command", "absent", "finalize.absent")
	want := map[string]string{"finalize.gate": "[local, ci]", "finalize.test_command": "make x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConfigLeafValues = %v, want %v", got, want)
	}
}

func TestRenderVisibilityEdit(t *testing.T) {
	edited, changed, err := RenderVisibilityEdit([]byte("visibility: shared # note\nx: 1\n"), "private")
	if err != nil || !changed || string(edited) != "visibility: private # note\nx: 1\n" {
		t.Errorf("edit = %q, %v, %v; want the value rewritten with its comment", edited, changed, err)
	}
	edited, changed, err = RenderVisibilityEdit([]byte("x: 1\nvisibility: 'shared'\n"), "private")
	if err != nil || !changed || string(edited) != "x: 1\nvisibility: private\n" {
		t.Errorf("quoted edit = %q, %v, %v", edited, changed, err)
	}
	for name, in := range map[string]string{
		"absent": "x: 1\n",
		"equal":  "visibility: private # kept\n",
		"nested": "x:\n  visibility: shared\n",
		"empty":  "",
	} {
		edited, changed, err := RenderVisibilityEdit([]byte(in), "private")
		if err != nil || changed || string(edited) != in {
			t.Errorf("%s: edit = %q, %v, %v; want unchanged", name, edited, changed, err)
		}
	}
	if _, _, err := RenderVisibilityEdit([]byte("visibility: shared\n"), "public"); err == nil {
		t.Errorf("an invalid target value was accepted")
	}
	if _, _, err := RenderVisibilityEdit([]byte("visibility: shared\nvisibility: private\n"), "private"); err == nil {
		t.Errorf("a duplicated key was edited")
	}
}

func TestRemoveGitignoreBlock(t *testing.T) {
	block := string(GitignoreBlock())
	in := "node_modules/\n# keep\n\n" + block + "\n\n"
	out, changed, err := RemoveGitignoreBlock([]byte(in))
	if err != nil || !changed || string(out) != "node_modules/\n# keep\n" {
		t.Errorf("remove = %q, %v, %v", out, changed, err)
	}
	out, changed, err = RemoveGitignoreBlock([]byte("a\n" + block + "b\n"))
	if err != nil || !changed || string(out) != "a\nb\n" {
		t.Errorf("mid-file remove = %q, %v, %v", out, changed, err)
	}
	out, changed, err = RemoveGitignoreBlock([]byte(block))
	if err != nil || !changed || len(out) != 0 {
		t.Errorf("block-only remove = %q, %v, %v; want empty", out, changed, err)
	}
	out, changed, err = RemoveGitignoreBlock([]byte("a\n"))
	if err != nil || changed || string(out) != "a\n" {
		t.Errorf("no-block remove = %q, %v, %v; want unchanged", out, changed, err)
	}
	out, changed, err = RemoveGitignoreBlock([]byte("a\n" + GitignoreStart + "\n.docket/\n"))
	var mal *MalformedGitignoreError
	if !errors.As(err, &mal) || mal.Generation != "docket" || out != nil || changed {
		t.Errorf("dangling start = %q, %v, %v; want MalformedGitignoreError and nil out", out, changed, err)
	}
}

func TestRemoveExcludeBlock(t *testing.T) {
	block := string(canonicalExcludeBytes)
	out, changed, err := RemoveExcludeBlock([]byte("# git ls-files --others\n*.swp\n" + block))
	if err != nil || !changed || string(out) != "# git ls-files --others\n*.swp\n" {
		t.Errorf("remove = %q, %v, %v", out, changed, err)
	}
	out, changed, err = RemoveExcludeBlock([]byte(block))
	if err != nil || !changed || len(out) != 0 {
		t.Errorf("block-only remove = %q, %v, %v; want empty", out, changed, err)
	}
	out, changed, err = RemoveExcludeBlock([]byte("*.swp\n"))
	if err != nil || changed || string(out) != "*.swp\n" {
		t.Errorf("no-block remove = %q, %v, %v; want unchanged", out, changed, err)
	}
	out, changed, err = RemoveExcludeBlock([]byte(ExcludeStart + "\n.worktrees/\n"))
	var mal *MalformedExcludeError
	if !errors.As(err, &mal) || out != nil || changed {
		t.Errorf("dangling start = %q, %v, %v; want MalformedExcludeError and nil out", out, changed, err)
	}
}
