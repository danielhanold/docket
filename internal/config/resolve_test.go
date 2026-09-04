package config

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// The three layers a caller may supply. The built-in layer is synthesized, so
// it never appears here.
func srcG(data string) Source {
	return Source{Layer: LayerGlobal, Name: "/tmp/xdg/docket/config.yml", Data: []byte(data)}
}

func srcR(data string) Source {
	return Source{Layer: LayerRepository, Name: ".docket.yml", Data: []byte(data)}
}

func srcL(data string) Source {
	return Source{Layer: LayerRepositoryLocal, Name: ".docket.local.yml", Data: []byte(data)}
}

var mainCtx = ResolveContext{DefaultBranch: "main"}

func mustResolve(t *testing.T, sources []Source, rctx ResolveContext) *resolution {
	t.Helper()
	res, err := resolve(sources, rctx)
	if err != nil {
		t.Fatalf("resolve: unexpected error %v; diagnostics %v", err, diagSummary(res))
	}
	return res
}

func diagSummary(res *resolution) []string {
	if res == nil {
		return nil
	}
	out := make([]string, 0, len(res.diags))
	for _, d := range res.diags {
		out = append(out, fmt.Sprintf("%s/%s/%s@%s", d.Severity, d.Code, d.Path, d.Provenance.Layer))
	}
	return out
}

func diagsWithCode(res *resolution, code string) []Diagnostic {
	var out []Diagnostic
	for _, d := range res.diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

// effectiveLeaf reads one Effective leaf by its dotted path so a table can
// assert against Effective without repeating the struct walk. Only the leaves
// Effective actually carries are addressable — asking for a deferred or inert
// path is a test bug, not a miss.
func effectiveLeaf(t *testing.T, eff Effective, path string) (any, Provenance, bool) {
	t.Helper()
	switch path {
	case "integration_branch":
		return eff.IntegrationBranch.Value, eff.IntegrationBranch.Provenance, eff.IntegrationBranch.Explicit
	case "changes_dir":
		return eff.ChangesDir.Value, eff.ChangesDir.Provenance, eff.ChangesDir.Explicit
	case "adrs_dir":
		return eff.ADRsDir.Value, eff.ADRsDir.Provenance, eff.ADRsDir.Explicit
	case "results_dir":
		return eff.ResultsDir.Value, eff.ResultsDir.Provenance, eff.ResultsDir.Explicit
	case "finalize.gate":
		return eff.Finalize.Gate.Value, eff.Finalize.Gate.Provenance, eff.Finalize.Gate.Explicit
	case "finalize.test_command":
		return eff.Finalize.TestCommand.Value, eff.Finalize.TestCommand.Provenance, eff.Finalize.TestCommand.Explicit
	case "finalize.require_pr_approval":
		return eff.Finalize.RequirePRApproval.Value, eff.Finalize.RequirePRApproval.Provenance, eff.Finalize.RequirePRApproval.Explicit
	case "learnings.enabled":
		return eff.Learnings.Enabled.Value, eff.Learnings.Enabled.Provenance, eff.Learnings.Enabled.Explicit
	case "reclaim.lease_ttl":
		return eff.Reclaim.LeaseTTL.Value, eff.Reclaim.LeaseTTL.Provenance, eff.Reclaim.LeaseTTL.Explicit
	case "reclaim.auto":
		return eff.Reclaim.Auto.Value, eff.Reclaim.Auto.Provenance, eff.Reclaim.Auto.Explicit
	case "review.min_fix_severity":
		return eff.Review.MinFixSeverity.Value, eff.Review.MinFixSeverity.Provenance, eff.Review.MinFixSeverity.Explicit
	case "review.max_fix_tasks":
		return eff.Review.MaxFixTasks.Value, eff.Review.MaxFixTasks.Provenance, eff.Review.MaxFixTasks.Explicit
	case "gate_observation_budget":
		return eff.GateObservation.Value, eff.GateObservation.Provenance, eff.GateObservation.Explicit
	case "board_surfaces":
		return eff.BoardSurfaces.Value, eff.BoardSurfaces.Provenance, eff.BoardSurfaces.Explicit
	case "change_types":
		return eff.ChangeTypes.Value, eff.ChangeTypes.Provenance, eff.ChangeTypes.Explicit
	}
	t.Fatalf("effectiveLeaf: %q is not an Effective leaf", path)
	return nil, Provenance{}, false
}

func leaseTTL(n int) string { return fmt.Sprintf("reclaim:\n  lease_ttl: %d\n", n) }

// TestResolveBoardDefaults pins the built-in board presentation: the canonical
// six-token permutation, and one updated/desc sort per section, all
// non-explicit built-in.
func TestResolveBoardDefaults(t *testing.T) {
	res := mustResolve(t, nil, mainCtx)
	b := res.effective.Board
	if got := b.SectionOrder.Value; !reflect.DeepEqual(got, BoardSectionTokens) {
		t.Fatalf("default section_order = %v, want %v", got, BoardSectionTokens)
	}
	if b.SectionOrder.Explicit || b.SectionOrder.Provenance.Layer != LayerBuiltIn {
		t.Fatalf("default section_order must be non-explicit built-in, got explicit=%v layer=%q",
			b.SectionOrder.Explicit, b.SectionOrder.Provenance.Layer)
	}
	for _, s := range BoardSectionTokens {
		srt, ok := b.Sorting[s]
		if !ok {
			t.Fatalf("missing default sorting for %s", s)
		}
		if srt.By.Value != "updated" || srt.Direction.Value != "desc" {
			t.Errorf("%s default sort = %s %s, want updated desc", s, srt.By.Value, srt.Direction.Value)
		}
		if srt.By.Explicit || srt.By.Provenance.Layer != LayerBuiltIn {
			t.Errorf("%s default by must be non-explicit built-in, got explicit=%v layer=%q",
				s, srt.By.Explicit, srt.By.Provenance.Layer)
		}
		if srt.Direction.Explicit || srt.Direction.Provenance.Layer != LayerBuiltIn {
			t.Errorf("%s default direction must be non-explicit built-in, got explicit=%v layer=%q",
				s, srt.Direction.Explicit, srt.Direction.Provenance.Layer)
		}
	}
}

// TestResolveBoardLayeringAndPerLeafInheritance: a global sets built {by: id,
// direction: asc}; a repository-local overrides ONLY built.direction. Each
// sort leaf resolves independently, and every sibling section keeps its
// built-in updated/desc.
func TestResolveBoardLayeringAndPerLeafInheritance(t *testing.T) {
	res := mustResolve(t, []Source{
		srcG("board:\n  sorting:\n    built:\n      by: id\n      direction: asc\n"),
		srcL("board:\n  sorting:\n    built:\n      direction: desc\n"),
	}, mainCtx)
	b := res.effective.Board

	built := b.Sorting["built"]
	if built.By.Value != "id" || built.By.Provenance.Layer != LayerGlobal || !built.By.Explicit {
		t.Errorf("built.by = %+v, want id from the global layer (explicit)", built.By)
	}
	if built.Direction.Value != "desc" || built.Direction.Provenance.Layer != LayerRepositoryLocal || !built.Direction.Explicit {
		t.Errorf("built.direction = %+v, want desc from the repository-local layer (explicit)", built.Direction)
	}

	for _, s := range BoardSectionTokens {
		if s == "built" {
			continue
		}
		srt := b.Sorting[s]
		if srt.By.Value != "updated" || srt.By.Explicit || srt.By.Provenance.Layer != LayerBuiltIn {
			t.Errorf("%s.by = %+v, want the untouched built-in updated", s, srt.By)
		}
		if srt.Direction.Value != "desc" || srt.Direction.Explicit || srt.Direction.Provenance.Layer != LayerBuiltIn {
			t.Errorf("%s.direction = %+v, want the untouched built-in desc", s, srt.Direction)
		}
	}
}

// TestResolveBoardSectionOrderWholeListReplacement: a global declares one full
// valid permutation, a repository declares a different full valid permutation;
// the repository wins wholesale with repository provenance.
func TestResolveBoardSectionOrderWholeListReplacement(t *testing.T) {
	repoOrder := []string{"built", "in-progress", "blocked", "groomed", "proposed", "deferred"}
	res := mustResolve(t, []Source{
		srcG("board:\n  section_order: [deferred, proposed, groomed, blocked, built, in-progress]\n"),
		srcR("board:\n  section_order: [built, in-progress, blocked, groomed, proposed, deferred]\n"),
	}, mainCtx)
	got := res.effective.Board.SectionOrder
	if !reflect.DeepEqual(got.Value, repoOrder) {
		t.Errorf("section_order = %v, want the repository permutation %v (a higher layer replaces the list whole)", got.Value, repoOrder)
	}
	if got.Provenance.Layer != LayerRepository || !got.Explicit {
		t.Errorf("section_order provenance = %+v (explicit %v), want the repository layer", got.Provenance, got.Explicit)
	}
}

// TestResolveBoardInvalidSectionOrderFallsBack: a section_order that fails
// shape validation, or whose token list is not a permutation of the six
// (missing, unknown, or duplicate token), is warned about and dropped as one
// value — so a lower valid layer's list wins and the snapshot stays valid.
func TestResolveBoardInvalidSectionOrderFallsBack(t *testing.T) {
	repoOrder := []string{"built", "in-progress", "blocked", "groomed", "proposed", "deferred"}
	cases := []struct{ name, yaml string }{
		{"missing token", "board:\n  section_order: [in-progress, built, blocked, groomed, proposed]\n"},
		{"unknown token", "board:\n  section_order: [in-progress, built, blocked, groomed, proposed, bogus]\n"},
		{"duplicate token", "board:\n  section_order: [in-progress, built, blocked, groomed, proposed, proposed]\n"},
		{"not a list", "board:\n  section_order: everything\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := resolve([]Source{
				srcR("board:\n  section_order: [built, in-progress, blocked, groomed, proposed, deferred]\n"),
				srcL(tc.yaml),
			}, mainCtx)
			if err != nil {
				t.Fatalf("resolve err = %v, want nil (warn-and-ignore keeps the snapshot valid); diags %v", err, diagSummary(res))
			}
			var warns []Diagnostic
			for _, d := range res.diags {
				if d.Path == "board.section_order" {
					if d.Severity != SeverityWarning {
						t.Errorf("board.section_order diag severity = %q, want warning: %+v", d.Severity, d)
					}
					warns = append(warns, d)
				}
			}
			if len(warns) != 1 {
				t.Fatalf("want exactly one board.section_order warning, got %v", diagSummary(res))
			}
			if warns[0].Provenance == nil || warns[0].Provenance.Layer != LayerRepositoryLocal || warns[0].Provenance.Source != ".docket.local.yml" {
				t.Errorf("warning provenance = %+v, want the repository-local layer", warns[0].Provenance)
			}
			got := res.effective.Board.SectionOrder
			if !reflect.DeepEqual(got.Value, repoOrder) {
				t.Errorf("section_order = %v, want the repository layer's valid list %v (the lower valid layer won)", got.Value, repoOrder)
			}
			if got.Provenance.Layer != LayerRepository || !got.Explicit {
				t.Errorf("section_order provenance = %+v (explicit %v), want the repository layer", got.Provenance, got.Explicit)
			}
		})
	}
}

// TestResolveBoardInvalidSortLeafInheritsOnlyThatLeaf: an out-of-enum sort leaf
// is warned about and dropped, so ONLY that leaf inherits the next valid
// layer; its valid sibling leaf and every other section are untouched.
func TestResolveBoardInvalidSortLeafInheritsOnlyThatLeaf(t *testing.T) {
	res, err := resolve([]Source{
		srcG("board:\n  sorting:\n    built:\n      by: id\n      direction: asc\n"),
		srcL("board:\n  sorting:\n    built:\n      by: priority\n      direction: desc\n"),
	}, mainCtx)
	if err != nil {
		t.Fatalf("resolve err = %v, want nil; diags %v", err, diagSummary(res))
	}
	built := res.effective.Board.Sorting["built"]
	if built.By.Value != "id" || built.By.Provenance.Layer != LayerGlobal || !built.By.Explicit {
		t.Errorf("built.by = %+v, want id from the global layer (the invalid repo-local leaf fell through)", built.By)
	}
	if built.Direction.Value != "desc" || built.Direction.Provenance.Layer != LayerRepositoryLocal || !built.Direction.Explicit {
		t.Errorf("built.direction = %+v, want desc from the repository-local layer", built.Direction)
	}
	var warns []Diagnostic
	for _, d := range res.diags {
		if d.Path == "board.sorting.built.by" {
			if d.Severity != SeverityWarning {
				t.Errorf("built.by diag severity = %q, want warning", d.Severity)
			}
			warns = append(warns, d)
		}
	}
	if len(warns) != 1 {
		t.Fatalf("want exactly one board.sorting.built.by warning, got %v", diagSummary(res))
	}
	if warns[0].Provenance == nil || warns[0].Provenance.Layer != LayerRepositoryLocal {
		t.Errorf("warning provenance = %+v, want the repository-local layer", warns[0].Provenance)
	}
	for _, s := range BoardSectionTokens {
		if s == "built" {
			continue
		}
		srt := res.effective.Board.Sorting[s]
		if srt.By.Value != "updated" || srt.By.Explicit || srt.Direction.Value != "desc" || srt.Direction.Explicit {
			t.Errorf("%s sort = %+v/%+v, want the untouched built-in updated desc", s, srt.By, srt.Direction)
		}
	}
}

// TestResolveBoardUnknownSortingSectionWarns: an unknown section under
// board.sorting is the deliberate warn-and-ignore surface (unknown-key,
// warning severity — the same family as an unknown skills role); the snapshot
// stays valid and effective policy is unchanged.
func TestResolveBoardUnknownSortingSectionWarns(t *testing.T) {
	res, err := resolve([]Source{srcR("board:\n  sorting:\n    bogus:\n      by: id\n")}, mainCtx)
	if err != nil {
		t.Fatalf("resolve err = %v, want nil; diags %v", err, diagSummary(res))
	}
	warns := 0
	for _, d := range res.diags {
		if d.Path == "board.sorting.bogus" {
			if d.Code != CodeUnknownKey || d.Severity != SeverityWarning {
				t.Errorf("bogus section diag = %s/%s, want unknown-key warning", d.Code, d.Severity)
			}
			warns++
		}
	}
	if warns != 1 {
		t.Fatalf("want one warning on board.sorting.bogus, got %v", diagSummary(res))
	}
	if got := res.effective.Board.Sorting["built"]; got.By.Value != "updated" || got.By.Explicit {
		t.Errorf("built sort = %+v; an unknown section must not alter effective policy", got)
	}
}

// TestResolveBoardUnknownBoardKeyIsError: any other unknown key under board.
// stays on the strict typo path — an error that invalidates the snapshot.
func TestResolveBoardUnknownBoardKeyIsError(t *testing.T) {
	res, err := resolve([]Source{srcR("board:\n  foo: 1\n")}, mainCtx)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("resolve err = %v, want ErrInvalidConfig", err)
	}
	bad := diagsWithCode(res, CodeUnknownKey)
	if len(bad) != 1 || bad[0].Path != "board.foo" || bad[0].Severity != SeverityError {
		t.Errorf("want one error unknown-key on board.foo, got %v", diagSummary(res))
	}
}

func TestPrecedencePerLeaf(t *testing.T) {
	cases := []struct {
		name     string
		sources  []Source
		want     int
		layer    LayerKind
		explicit bool
	}{
		{"nothing declared wins the built-in default", nil, 72, LayerBuiltIn, false},
		{"global alone", []Source{srcG(leaseTTL(10))}, 10, LayerGlobal, true},
		{"repository beats global", []Source{srcG(leaseTTL(10)), srcR(leaseTTL(20))}, 20, LayerRepository, true},
		{"repository-local beats everything", []Source{srcG(leaseTTL(10)), srcR(leaseTTL(20)), srcL(leaseTTL(30))}, 30, LayerRepositoryLocal, true},
		{"repository-local beats global with no repository layer", []Source{srcG(leaseTTL(10)), srcL(leaseTTL(30))}, 30, LayerRepositoryLocal, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := mustResolve(t, tc.sources, mainCtx)
			got := res.effective.Reclaim.LeaseTTL
			if got.Value != tc.want {
				t.Errorf("lease_ttl = %d, want %d", got.Value, tc.want)
			}
			if got.Provenance.Layer != tc.layer {
				t.Errorf("lease_ttl provenance layer = %q, want %q", got.Provenance.Layer, tc.layer)
			}
			if got.Explicit != tc.explicit {
				t.Errorf("lease_ttl explicit = %v, want %v", got.Explicit, tc.explicit)
			}
		})
	}
}

// A leaf outside Effective (learnings.cap is inert) still resolves through the
// same precedence, because the classifier reads `declared`, not Effective.
func TestPrecedenceForNonEffectiveLeaf(t *testing.T) {
	capYAML := func(n int) string { return fmt.Sprintf("learnings:\n  cap: %d\n", n) }
	res := mustResolve(t, []Source{srcG(capYAML(100)), srcR(capYAML(200)), srcL(capYAML(250))}, mainCtx)
	d, ok := res.declared["learnings.cap"]
	if !ok {
		t.Fatalf("learnings.cap missing from declared; have %v", res.declared)
	}
	if d.value != 250 {
		t.Errorf("learnings.cap = %v, want 250", d.value)
	}
	if d.prov.Layer != LayerRepositoryLocal {
		t.Errorf("learnings.cap layer = %q, want %q", d.prov.Layer, LayerRepositoryLocal)
	}
	if n := len(res.allDecls); n != 3 {
		t.Errorf("allDecls = %d declarations, want 3 (every layer's, not just the winner)", n)
	}
}

func TestExplicitDefaultProvenance(t *testing.T) {
	res := mustResolve(t, []Source{srcR("reclaim:\n  auto: false\n")}, mainCtx)
	got := res.effective.Reclaim.Auto
	if got.Value != false {
		t.Errorf("reclaim.auto = %v, want false", got.Value)
	}
	if !got.Explicit {
		t.Error("reclaim.auto explicit = false, want true: a declaration that repeats the default is still a declaration")
	}
	if got.Provenance.Layer != LayerRepository || got.Provenance.Source != ".docket.yml" {
		t.Errorf("reclaim.auto provenance = %+v, want the repository layer", got.Provenance)
	}
	if got.Provenance.Line != 2 {
		t.Errorf("reclaim.auto provenance line = %d, want 2", got.Provenance.Line)
	}
}

func TestListReplacesWhole(t *testing.T) {
	res := mustResolve(t, []Source{
		srcG("change_types: [feat]\n"),
		srcR("change_types: [chore, docs]\n"),
	}, mainCtx)
	got := res.effective.ChangeTypes
	want := []string{"chore", "docs"}
	if !reflect.DeepEqual(got.Value, want) {
		t.Errorf("change_types = %v, want %v (a higher layer replaces the list whole)", got.Value, want)
	}
	if got.Provenance.Layer != LayerRepository {
		t.Errorf("change_types layer = %q, want %q", got.Provenance.Layer, LayerRepository)
	}
}

// fenceCase declares one coordination-fenced path: what the committed layer
// says (which must win) and what a machine layer says (which must be warned
// about and dropped).
type fenceCase struct {
	path    string
	repo    string
	machine string
	want    any
}

func fenceCases() []fenceCase {
	return []fenceCase{
		// metadata_branch is no longer fenced — it is an obsolete tombstone
		// (change 0363), excluded at decode. Its tombstone behavior is covered by
		// TestMetadataBranchIsObsoleteTombstone.
		{"integration_branch", "integration_branch: develop\n", "integration_branch: trunk\n", "develop"},
		{"changes_dir", "changes_dir: docs/a\n", "changes_dir: docs/b\n", "docs/a"},
		{"adrs_dir", "adrs_dir: docs/a\n", "adrs_dir: docs/b\n", "docs/a"},
		{"results_dir", "results_dir: docs/a\n", "results_dir: docs/b\n", "docs/a"},
		{"finalize.skip_results_only_delta",
			"finalize:\n  skip_results_only_delta: true\n",
			"finalize:\n  skip_results_only_delta: false\n", true},
		{"github_project", "github_project: {owner: acme, number: 7}\n", "github_project: auto\n",
			githubProject{Owner: "acme", Number: 7}},
		{"terminal_publish", "terminal_publish: true\n", "terminal_publish: false\n", true},
	}
}

// TestEveryFence asserts BOTH halves for every repo-fenced row in both machine
// layers: the machine declaration is warned about at ITS OWN provenance, and
// the committed layer's value is the one that resolves. A warning alone would
// be a false alarm if the fenced value still won.
func TestEveryFence(t *testing.T) {
	// Sources are always supplied low-to-high, so which side the committed
	// layer sits on depends on the machine layer under test.
	machines := []struct {
		name    string
		layer   LayerKind
		ordered func(machine, repo string) []Source
	}{
		{"global", LayerGlobal, func(machine, repo string) []Source {
			return []Source{srcG(machine), srcR(repo)}
		}},
		{"repository-local", LayerRepositoryLocal, func(machine, repo string) []Source {
			return []Source{srcR(repo), srcL(machine)}
		}},
	}
	for _, tc := range fenceCases() {
		for _, m := range machines {
			t.Run(tc.path+"/"+m.name, func(t *testing.T) {
				res := mustResolve(t, m.ordered(tc.machine, tc.repo), mainCtx)

				fenced := diagsWithCode(res, CodeFencedIgnored)
				if len(fenced) != 1 {
					t.Fatalf("got %d fenced-setting-ignored diagnostics, want 1: %v", len(fenced), diagSummary(res))
				}
				d := fenced[0]
				if d.Severity != SeverityWarning {
					t.Errorf("fence severity = %q, want %q", d.Severity, SeverityWarning)
				}
				if d.Path != tc.path {
					t.Errorf("fence path = %q, want %q", d.Path, tc.path)
				}
				if d.Provenance == nil || d.Provenance.Layer != m.layer {
					t.Errorf("fence provenance = %+v, want the declaring layer %q", d.Provenance, m.layer)
				}

				got, ok := res.declared[tc.path]
				if !ok {
					t.Fatalf("%s did not resolve at all; declared = %v", tc.path, res.declared)
				}
				if !reflect.DeepEqual(got.value, tc.want) {
					t.Errorf("%s = %#v, want %#v (the committed layer's value)", tc.path, got.value, tc.want)
				}
				if got.prov.Layer != LayerRepository {
					t.Errorf("%s provenance layer = %q, want %q", tc.path, got.prov.Layer, LayerRepository)
				}
			})
		}
	}
}

// The fence table is derived from the registry in both directions, so a new
// repo-fenced row cannot ship untested and a row that loses its fence cannot
// keep a stale case.
func TestEveryFenceCoversTheRegistry(t *testing.T) {
	covered := make(map[string]bool)
	for _, c := range fenceCases() {
		covered[c.path] = true
	}
	declared := make(map[string]bool)
	for _, spec := range registry() {
		if spec.scope == scopeRepoFenced {
			declared[spec.path] = true
			if !covered[spec.path] {
				t.Errorf("registry row %q is repo-fenced but no fence case covers it", spec.path)
			}
		}
	}
	for path := range covered {
		if !declared[path] {
			t.Errorf("fence case %q is not a repo-fenced registry row", path)
		}
	}
}

func TestFencedPathStillReachesEffective(t *testing.T) {
	res := mustResolve(t, []Source{srcR("changes_dir: docs/committed\n"), srcL("changes_dir: docs/machine\n")}, mainCtx)
	value, prov, explicit := effectiveLeaf(t, res.effective, "changes_dir")
	if value != "docs/committed" || prov.Layer != LayerRepository || !explicit {
		t.Errorf("changes_dir = %v from %+v (explicit %v), want docs/committed from the repository layer", value, prov, explicit)
	}
}

// runtime.bash is obsolete in EVERY layer, not fenced into one: docket no
// longer has a Bash runtime for any layer to select. Decode reports it and
// drops it, so resolution must neither honor it nor add a fence warning on
// top of the obsolescence notice.
func TestRuntimeBashCommittedFence(t *testing.T) {
	for _, build := range []func(string) Source{srcG, srcR, srcL} {
		src := build("runtime:\n  bash: /bin/bash\n")
		t.Run(string(src.Layer), func(t *testing.T) {
			res := mustResolve(t, []Source{src}, mainCtx)
			if _, ok := res.declared["runtime.bash"]; ok {
				t.Error("runtime.bash resolved; it must be excluded in every layer")
			}
			obsolete := diagsWithCode(res, CodeObsoleteSetting)
			if len(obsolete) != 1 || obsolete[0].Path != "runtime.bash" {
				t.Errorf("want one obsolete-setting diagnostic on runtime.bash, got %v", diagSummary(res))
			}
			if fenced := diagsWithCode(res, CodeFencedIgnored); len(fenced) != 0 {
				t.Errorf("obsolete settings are not fenced settings; got %v", diagSummary(res))
			}
		})
	}
}

func TestBoardGithubTokenMachineFence(t *testing.T) {
	cases := []struct {
		name       string
		src        Source
		want       []string
		wantFenced bool
	}{
		{"machine local drops github, keeps the rest", srcL("board_surfaces: [inline, github]\n"), []string{"inline"}, true},
		{"machine global drops github down to an empty list", srcG("board_surfaces: [github]\n"), []string{}, true},
		{"committed layer keeps github", srcR("board_surfaces: [inline, github]\n"), []string{"inline", "github"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := mustResolve(t, []Source{tc.src}, mainCtx)
			got := res.effective.BoardSurfaces
			if !reflect.DeepEqual(got.Value, tc.want) {
				t.Errorf("board_surfaces = %#v, want %#v", got.Value, tc.want)
			}
			if !got.Explicit || got.Provenance.Layer != tc.src.Layer {
				t.Errorf("board_surfaces provenance = %+v (explicit %v), want the declaring layer %q",
					got.Provenance, got.Explicit, tc.src.Layer)
			}
			fenced := diagsWithCode(res, CodeFencedIgnored)
			if tc.wantFenced != (len(fenced) == 1) {
				t.Fatalf("wantFenced=%v but got %v", tc.wantFenced, diagSummary(res))
			}
			if tc.wantFenced {
				if fenced[0].Path != "board_surfaces" || fenced[0].Severity != SeverityWarning {
					t.Errorf("fence diagnostic = %+v, want a board_surfaces warning", fenced[0])
				}
				if fenced[0].Provenance == nil || fenced[0].Provenance.Layer != tc.src.Layer {
					t.Errorf("fence provenance = %+v, want %q", fenced[0].Provenance, tc.src.Layer)
				}
			}
		})
	}
}

// TestAgentHarnessesResolution pins the typed, provenance-carrying repository
// input: it resolves with the list-replace precedence, records which layer
// supplied it, and distinguishes the deliberate empty-list retire-everything
// state from the untouched-because-absent state.
func TestAgentHarnessesResolution(t *testing.T) {
	cases := []struct {
		name         string
		sources      []Source
		wantValue    []string
		wantExplicit bool
		wantLayer    LayerKind
	}{
		{
			name:         "repository layer supplies the list",
			sources:      []Source{srcR("agent_harnesses: [claude, codex]\n")},
			wantValue:    []string{"claude", "codex"},
			wantExplicit: true,
			wantLayer:    LayerRepository,
		},
		{
			name: "repository-local empty list replaces, not appends",
			sources: []Source{
				srcR("agent_harnesses: [claude, codex]\n"),
				srcL("agent_harnesses: []\n"),
			},
			wantValue:    []string{},
			wantExplicit: true,
			wantLayer:    LayerRepositoryLocal,
		},
		{
			name:         "global-only declaration resolves but carries the global provenance",
			sources:      []Source{srcG("agent_harnesses: [claude]\n")},
			wantValue:    []string{"claude"},
			wantExplicit: true,
			wantLayer:    LayerGlobal,
		},
		{
			name:         "absent everywhere is the touch-nothing state",
			sources:      nil,
			wantExplicit: false,
			wantLayer:    LayerBuiltIn,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := mustResolve(t, tc.sources, mainCtx)
			got := res.effective.AgentHarnesses
			if got.Explicit != tc.wantExplicit {
				t.Errorf("agent_harnesses explicit = %v, want %v", got.Explicit, tc.wantExplicit)
			}
			if got.Provenance.Layer != tc.wantLayer {
				t.Errorf("agent_harnesses provenance layer = %q, want %q", got.Provenance.Layer, tc.wantLayer)
			}
			if tc.wantExplicit && !reflect.DeepEqual(got.Value, tc.wantValue) {
				t.Errorf("agent_harnesses value = %#v, want %#v", got.Value, tc.wantValue)
			}
		})
	}
}

// TestAgentHarnessesInvalidTokens pins that a duplicate or an out-of-set token
// is a CodeInvalidValue diagnostic that invalidates the whole snapshot.
func TestAgentHarnessesInvalidTokens(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"duplicate token", "agent_harnesses: [claude, claude]\n"},
		{"out-of-set token", "agent_harnesses: [emacs]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := resolve([]Source{srcR(tc.doc)}, mainCtx)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("resolve error = %v, want ErrInvalidConfig", err)
			}
			bad := diagsWithCode(res, CodeInvalidValue)
			if len(bad) != 1 || bad[0].Path != "agent_harnesses" {
				t.Errorf("want one invalid-value diagnostic on agent_harnesses, got %v", diagSummary(res))
			}
		})
	}
}

// Model and effort resolve independently, and inside one layer a harness-
// specific pin falls back to `default` before the next layer is consulted.
func TestAgentsHarnessFirstFallback(t *testing.T) {
	res := mustResolve(t, []Source{srcG(
		"agents:\n" +
			"  default:\n" +
			"    adr:\n" +
			"      model: shared-model\n" +
			"  claude:\n" +
			"    adr:\n" +
			"      effort: high\n")}, mainCtx)

	claude := res.effective.Agents["claude"]["adr"]
	if claude.Model.Value != "shared-model" || claude.Model.Provenance.Layer != LayerGlobal || !claude.Model.Explicit {
		t.Errorf("claude/adr model = %+v, want shared-model from the global layer", claude.Model)
	}
	if claude.Effort.Value != "high" || claude.Effort.Provenance.Layer != LayerGlobal || !claude.Effort.Explicit {
		t.Errorf("claude/adr effort = %+v, want high from the global layer", claude.Effort)
	}
	if claude.Model.Provenance.Line == claude.Effort.Provenance.Line {
		t.Errorf("model and effort provenance must be independent, both report line %d", claude.Model.Provenance.Line)
	}

	codex := res.effective.Agents["codex"]["adr"]
	if codex.Model.Value != "shared-model" || codex.Model.Provenance.Layer != LayerGlobal {
		t.Errorf("codex/adr model = %+v, want the global agents.default fallback", codex.Model)
	}
	if codex.Effort.Value != "xhigh" || codex.Effort.Provenance.Layer != LayerBuiltIn || codex.Effort.Explicit {
		t.Errorf("codex/adr effort = %+v, want the built-in xhigh", codex.Effort)
	}

	cursor := res.effective.Agents["cursor"]["adr"]
	if cursor.Effort.Value != "" || cursor.Effort.Provenance.Layer != LayerBuiltIn {
		t.Errorf("cursor/adr effort = %+v, want the suppressed built-in pin", cursor.Effort)
	}
}

// The fallback runs harness-FIRST: a pin naming the harness outranks that
// layer's `default` pin for the same agent and field.
func TestAgentsHarnessSpecificBeatsDefaultInTheSameLayer(t *testing.T) {
	res := mustResolve(t, []Source{srcG(
		"agents:\n" +
			"  default:\n" +
			"    adr:\n" +
			"      model: shared-model\n" +
			"  claude:\n" +
			"    adr:\n" +
			"      model: claude-model\n")}, mainCtx)

	if got := res.effective.Agents["claude"]["adr"].Model.Value; got != "claude-model" {
		t.Errorf("claude/adr model = %q, want claude-model", got)
	}
	if got := res.effective.Agents["codex"]["adr"].Model.Value; got != "shared-model" {
		t.Errorf("codex/adr model = %q, want the default pin shared-model", got)
	}
}

func TestAgentsEffortAuto(t *testing.T) {
	res := mustResolve(t, []Source{srcG("agents:\n  claude:\n    adr:\n      effort: auto\n")}, mainCtx)
	got := res.effective.Agents["claude"]["adr"].Effort
	if got.Value != "" {
		t.Errorf("effort = %q, want \"\" (auto suppresses the pin)", got.Value)
	}
	if !got.Explicit || got.Provenance.Layer != LayerGlobal {
		t.Errorf("effort provenance = %+v (explicit %v), want an explicit global pin", got.Provenance, got.Explicit)
	}
}

// The repository layers may not pin agents at all: such a declaration is a
// deferred-capability request the classifier reports, never effective policy.
func TestAgentsRepoLayerExcludedFromEffective(t *testing.T) {
	for _, build := range []func(string) Source{srcR, srcL} {
		src := build("agents:\n  claude:\n    adr:\n      model: repo-model\n")
		t.Run(string(src.Layer), func(t *testing.T) {
			res := mustResolve(t, []Source{src}, mainCtx)
			got := res.effective.Agents["claude"]["adr"].Model
			if got.Value != "claude-opus-5" || got.Provenance.Layer != LayerBuiltIn || got.Explicit {
				t.Errorf("claude/adr model = %+v, want the untouched built-in default", got)
			}
			d, ok := res.declared["agents.claude.adr.model"]
			if !ok || d.prov.Layer != src.Layer {
				t.Errorf("the declaration must still reach the classifier; declared = %v", res.declared)
			}
		})
	}
}

func TestIntegrationBranchAuto(t *testing.T) {
	res := mustResolve(t, nil, mainCtx)
	got := res.effective.IntegrationBranch
	if got.Value != "main" {
		t.Errorf("integration_branch = %q, want the context's default branch", got.Value)
	}
	if got.Explicit || got.Provenance.Layer != LayerBuiltIn {
		t.Errorf("integration_branch provenance = %+v (explicit %v), want the built-in auto sentinel's layer", got.Provenance, got.Explicit)
	}

	res = mustResolve(t, []Source{srcR("integration_branch: develop\n")}, ResolveContext{})
	if got := res.effective.IntegrationBranch; got.Value != "develop" || got.Provenance.Layer != LayerRepository {
		t.Errorf("integration_branch = %+v, want develop from the repository layer with no context needed", got)
	}
}

func TestMissingResolutionContext(t *testing.T) {
	_, _, err := Resolve(nil, ResolveContext{})
	if !errors.Is(err, ErrMissingResolutionContext) {
		t.Fatalf("err = %v, want ErrMissingResolutionContext", err)
	}

	// An explicit `auto` is the same question asked in a file.
	_, _, err = Resolve([]Source{srcR("integration_branch: auto\n")}, ResolveContext{})
	if !errors.Is(err, ErrMissingResolutionContext) {
		t.Fatalf("explicit auto: err = %v, want ErrMissingResolutionContext", err)
	}
}

func TestTestCommandAutoUnsets(t *testing.T) {
	res := mustResolve(t, nil, mainCtx)
	if got := res.effective.Finalize.TestCommand; got.Value != "" || got.Explicit {
		t.Errorf("built-in test_command = %+v, want the unset default", got)
	}
	res = mustResolve(t, []Source{srcR("finalize:\n  test_command: auto\n")}, mainCtx)
	got := res.effective.Finalize.TestCommand
	if got.Value != "" {
		t.Errorf("test_command = %q, want \"\" (auto is the unset sentinel)", got.Value)
	}
	if !got.Explicit || got.Provenance.Layer != LayerRepository {
		t.Errorf("test_command provenance = %+v (explicit %v), want the declaring layer", got.Provenance, got.Explicit)
	}
	res = mustResolve(t, []Source{srcR("finalize:\n  test_command: make check\n")}, mainCtx)
	if got := res.effective.Finalize.TestCommand.Value; got != "make check" {
		t.Errorf("test_command = %q, want %q", got, "make check")
	}

	// Build-side twin (change 0374): build.test_command spells UNSET with the
	// same `auto` sentinel and masks the same way, on its own leaf.
	res = mustResolve(t, []Source{srcR("build:\n  test_command: auto\n")}, mainCtx)
	if got := res.effective.Build.TestCommand; got.Value != "" {
		t.Errorf("build.test_command = %q, want \"\" (auto is the unset sentinel)", got.Value)
	}
}

// TestBuildGateAndCommandResolveIndependently is the discriminating fixture
// (change 0374): it sets BOTH pairs divergently, so a leaf reading the wrong
// key cannot pass. Neither command falls back to the other.
func TestBuildGateAndCommandResolveIndependently(t *testing.T) {
	res := mustResolve(t, []Source{srcR(
		"build:\n  gate: local\n  test_command: go test ./...\nfinalize:\n  test_command: make check\n")}, mainCtx)
	if got := res.effective.Build.TestCommand.Value; got != "go test ./..." {
		t.Errorf("build.test_command = %q, want %q", got, "go test ./...")
	}
	if got := res.effective.Finalize.TestCommand.Value; got != "make check" {
		t.Errorf("finalize.test_command = %q, want %q", got, "make check")
	}
	if got := res.effective.Build.Gate.Value; got != "local" {
		t.Errorf("build.gate = %q, want local", got)
	}
}

// TestBuildCommandLegacyAutoResolvesUnset: `auto` is legacy input for BOTH
// commands. It resolves to "" (unconfigured), keeps its declaring layer's
// provenance, and masks a lower layer's explicit command exactly as
// finalize.test_command's auto already does.
func TestBuildCommandLegacyAutoResolvesUnset(t *testing.T) {
	res := mustResolve(t, []Source{
		srcG("build:\n  test_command: make global-suite\n"),
		srcR("build:\n  test_command: auto\n"),
	}, mainCtx)
	got := res.effective.Build.TestCommand
	if got.Value != "" {
		t.Errorf("build.test_command = %q, want \"\" (auto is the unset sentinel)", got.Value)
	}
	if got.Provenance.Layer != LayerRepository {
		t.Errorf("provenance layer = %v, want the repository layer that declared auto", got.Provenance.Layer)
	}
}

// TestCommandsDefaultUnconfigured: both commands default to the empty
// unconfigured state; both gates to local.
func TestCommandsDefaultUnconfigured(t *testing.T) {
	res := mustResolve(t, nil, mainCtx)
	if v := res.effective.Build.TestCommand.Value; v != "" {
		t.Errorf("default build.test_command = %q, want \"\"", v)
	}
	if v := res.effective.Finalize.TestCommand.Value; v != "" {
		t.Errorf("default finalize.test_command = %q, want \"\"", v)
	}
	if v := res.effective.Build.Gate.Value; v != "local" {
		t.Errorf("default build.gate = %q, want local", v)
	}
}

// TestBuildGateRejectsUnknownValue: build.gate is local|off only — no ci/both
// (finalize keeps its wider enum).
func TestBuildGateRejectsUnknownValue(t *testing.T) {
	_, err := resolve([]Source{srcR("build:\n  gate: ci\n")}, mainCtx)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestAutoCaptureTypesSubset(t *testing.T) {
	t.Run("outside the effective change types", func(t *testing.T) {
		res, err := resolve([]Source{
			srcG("auto_capture:\n  types: [fix]\n"),
			srcR("change_types: [feat]\n"),
		}, mainCtx)
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("err = %v, want ErrInvalidConfig", err)
		}
		bad := diagsWithCode(res, CodeInvalidValue)
		if len(bad) != 1 || bad[0].Path != "auto_capture.types" {
			t.Fatalf("want one invalid-value on auto_capture.types, got %v", diagSummary(res))
		}
		if bad[0].Provenance == nil || bad[0].Provenance.Layer != LayerGlobal {
			t.Errorf("provenance = %+v, want the declaring global layer", bad[0].Provenance)
		}
	})

	t.Run("inside the effective change types", func(t *testing.T) {
		res := mustResolve(t, []Source{
			srcG("auto_capture:\n  types: [fix]\n"),
			srcR("change_types: [feat, fix]\n"),
		}, mainCtx)
		if d, ok := res.declared["auto_capture.types"]; !ok || !reflect.DeepEqual(d.value, []string{"fix"}) {
			t.Errorf("auto_capture.types = %#v, want [fix]", res.declared["auto_capture.types"])
		}
	})

	t.Run("the all sentinel is never a subset question", func(t *testing.T) {
		mustResolve(t, []Source{
			srcG("auto_capture:\n  types: all\n"),
			srcR("change_types: [feat]\n"),
		}, mainCtx)
	})
}

// Every layer is validated before any is honored: a caller must see all of a
// broken configuration, not the first broken layer.
func TestInvalidLayerFailsWhole(t *testing.T) {
	res, err := resolve([]Source{
		srcG("changes_dir: /etc/changes\n"),
		srcR("a: [unclosed\n"),
	}, mainCtx)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
	var haveGlobal, haveRepo bool
	for _, d := range res.diags {
		switch {
		case d.Code == CodeInvalidValue && d.Provenance.Layer == LayerGlobal:
			haveGlobal = true
		case d.Code == CodeInvalidYAML && d.Provenance.Layer == LayerRepository:
			haveRepo = true
		}
	}
	if !haveGlobal || !haveRepo {
		t.Errorf("diagnostics = %v, want findings from BOTH layers", diagSummary(res))
	}

	snap, diags, err := Resolve([]Source{srcR("changes_dir: /etc/changes\n")}, mainCtx)
	if snap != nil || !errors.Is(err, ErrInvalidConfig) || len(diags) == 0 {
		t.Errorf("Resolve on an invalid layer = (%v, %d diags, %v), want (nil, >0, ErrInvalidConfig)", snap, len(diags), err)
	}
}

// Validity is a question about the invalid-class CODES carried at error
// severity: the two deliberate v0.9.2 warn-and-ignore surfaces also carry
// unknown-key, and they must not fail the whole configuration.
func TestWarnOnlyUnknownKeysKeepTheSnapshotValid(t *testing.T) {
	snap, diags, err := Resolve([]Source{srcR("skills:\n  deploy: some-skill\nboard_surfaces: [inline, trello]\n")}, mainCtx)
	if err != nil {
		t.Fatalf("err = %v, want nil: warn-and-ignore surfaces do not invalidate", err)
	}
	if snap == nil {
		t.Fatal("snapshot = nil, want a resolved snapshot")
	}
	var warnings int
	for _, d := range diags {
		if d.Code == CodeUnknownKey && d.Severity == SeverityWarning {
			warnings++
		}
	}
	if warnings != 2 {
		t.Errorf("unknown-key warnings = %d, want 2: %+v", warnings, diags)
	}
	if !reflect.DeepEqual(snap.Effective.BoardSurfaces.Value, []string{"inline"}) {
		t.Errorf("board_surfaces = %#v, want [inline]", snap.Effective.BoardSurfaces.Value)
	}
}

func TestDiagnosticOrdering(t *testing.T) {
	in := []Diagnostic{
		{Code: CodeInertSetting, Severity: SeverityInfo, Path: "learnings.cap"},
		{Code: CodeFencedIgnored, Severity: SeverityWarning, Path: "terminal_publish"},
		{Code: CodeInvalidValue, Severity: SeverityError, Path: "metadata_branch"},
		{Code: CodeDeferredSetting, Severity: SeverityInfo, Path: "auto_groom"},
		{Code: CodeObsoleteSetting, Severity: SeverityWarning, Path: "runtime.bash"},
		{Code: CodeInvalidType, Severity: SeverityError, Path: "metadata_branch"},
		{Code: CodeFencedIgnored, Severity: SeverityWarning, Path: "runtime.bash"},
	}
	sortDiagnostics(in)
	want := []string{
		"error/invalid-type/metadata_branch",
		"error/invalid-value/metadata_branch",
		// path before code: both runtime.bash warnings precede terminal_publish.
		"warning/fenced-setting-ignored/runtime.bash",
		"warning/obsolete-setting/runtime.bash",
		"warning/fenced-setting-ignored/terminal_publish",
		"info/deferred-setting/auto_groom",
		"info/inert-setting/learnings.cap",
	}
	got := make([]string, 0, len(in))
	for _, d := range in {
		got = append(got, fmt.Sprintf("%s/%s/%s", d.Severity, d.Code, d.Path))
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order =\n%v\nwant\n%v", got, want)
	}
}

func TestResolveOrdersDiagnosticsEndToEnd(t *testing.T) {
	_, diags, _ := Resolve([]Source{
		srcL("terminal_publish: true\nchanges_dir: docs/machine\n"),
	}, mainCtx)
	if len(diags) < 2 {
		t.Fatalf("want at least two diagnostics, got %+v", diags)
	}
	for i := 1; i < len(diags); i++ {
		if diags[i-1].Path > diags[i].Path && diags[i-1].Severity == diags[i].Severity {
			t.Errorf("diagnostics are not path-sorted within a severity: %q before %q", diags[i-1].Path, diags[i].Path)
		}
	}
}

// The built-in layer is synthesized and the caller's layers are ordered by
// construction; violating either is a programming error, not a user's invalid
// configuration, so it must not masquerade as ErrInvalidConfig.
func TestResolveRejectsMisorderedSources(t *testing.T) {
	cases := []struct {
		name    string
		sources []Source
	}{
		{"built-in supplied", []Source{{Layer: LayerBuiltIn, Name: "built-in"}}},
		{"repository before global", []Source{srcR(""), srcG("")}},
		{"layer repeated", []Source{srcR(""), srcR("")}},
		{"unknown layer", []Source{{Layer: LayerKind("staging"), Name: "x"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Resolve(tc.sources, mainCtx)
			if err == nil {
				t.Fatal("err = nil, want a programming error")
			}
			if errors.Is(err, ErrInvalidConfig) {
				t.Errorf("err = %v, want a plain error rather than ErrInvalidConfig", err)
			}
		})
	}
}

// TestTolerateUnknownKeysDegradesToWarning holds the install path's tolerance
// rule (change 0392): with the flag on, an unknown key — top-level or nested —
// is a WARNING carrying the shared remedy, the snapshot stays valid, and the
// unknown subtree resolves to defaults.
func TestTolerateUnknownKeysDegradesToWarning(t *testing.T) {
	rctx := ResolveContext{DefaultBranch: "main", TolerateUnknownKeys: true}
	for name, yml := range map[string]string{
		"top-level": "some_future_block:\n  enabled: true\n",
		"nested":    "finalize:\n  some_new_field: 7\n",
	} {
		t.Run(name, func(t *testing.T) {
			res := mustResolve(t, []Source{srcR(yml)}, rctx)
			warns := diagsWithCode(res, CodeUnknownKey)
			if len(warns) != 1 {
				t.Fatalf("unknown-key diagnostics = %v, want exactly 1", diagSummary(res))
			}
			d := warns[0]
			if d.Severity != SeverityWarning {
				t.Errorf("severity = %s, want warning", d.Severity)
			}
			if d.Remedy != ToleratedUnknownKeyRemedy {
				t.Errorf("remedy = %q, want the shared ToleratedUnknownKeyRemedy", d.Remedy)
			}
			if d.Message == "" {
				t.Errorf("message was dropped; the reclassifier must keep it")
			}
			// The unknown subtree contributed no leaves: a known sibling leaf
			// still resolves to its built-in default, non-explicit.
			if _, _, explicit := effectiveLeaf(t, res.effective, "finalize.gate"); name == "nested" && explicit {
				t.Errorf("finalize.gate became explicit; the unknown subtree must contribute nothing")
			}
		})
	}
}

// TestTolerateUnknownKeysLeavesOtherClassesFatal proves the option changes
// nothing for the other invalid classes.
func TestTolerateUnknownKeysLeavesOtherClassesFatal(t *testing.T) {
	rctx := ResolveContext{DefaultBranch: "main", TolerateUnknownKeys: true}
	for name, yml := range map[string]string{
		"invalid-yaml":  "board: [unclosed\n",
		"duplicate-key": "learnings:\n  cap: 1\n  cap: 2\n",
		"invalid-type":  "metadata_branch: [not, a, string]\n",
		"invalid-value": "learnings:\n  cap: -1\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := Resolve([]Source{srcR(yml)}, rctx)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err = %v, want ErrInvalidConfig — tolerance must not reach %s", err, name)
			}
		})
	}
}

// TestUnknownKeyStrictWithoutTolerance is the mutation control for the option:
// the reclassifier must be the ONLY thing that flips the verdict, so the zero
// value keeps today's hard failure.
func TestUnknownKeyStrictWithoutTolerance(t *testing.T) {
	_, diags, err := Resolve([]Source{srcR("some_future_block: true\n")}, mainCtx)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
	for _, d := range diags {
		if d.Code == CodeUnknownKey && d.Severity != SeverityError {
			t.Errorf("unknown-key severity = %s without tolerance, want error", d.Severity)
		}
		if d.Code == CodeUnknownKey && d.Remedy == ToleratedUnknownKeyRemedy {
			t.Errorf("the tolerated remedy leaked into the strict path")
		}
	}
}

// TestTolerateUnknownKeysLeavesFenceIntact: a fenced KNOWN key keeps its
// existing warn-and-ignore posture with the option on. Mirror the exact
// layer/fixture of TestBoardGithubTokenMachineFence (a machine-layer
// board_surfaces carrying the fenced github token), changing only the rctx.
func TestTolerateUnknownKeysLeavesFenceIntact(t *testing.T) {
	rctx := ResolveContext{DefaultBranch: "main", TolerateUnknownKeys: true}
	res := mustResolve(t, []Source{srcL("board_surfaces: [inline, github]\n")}, rctx)
	fenced := diagsWithCode(res, CodeFencedIgnored)
	if len(fenced) != 1 || fenced[0].Severity != SeverityWarning {
		t.Fatalf("fenced diagnostics = %v, want one fenced-setting-ignored warning", diagSummary(res))
	}
	if fenced[0].Remedy == ToleratedUnknownKeyRemedy {
		t.Errorf("the fence's own remedy was overwritten")
	}
}

// TestWarningsFilter pins the helper the install path reads its surfaced
// diagnostics through.
func TestWarningsFilter(t *testing.T) {
	in := []Diagnostic{
		{Code: CodeUnknownKey, Severity: SeverityWarning},
		{Code: CodeInvalidValue, Severity: SeverityError},
		{Code: "x", Severity: SeverityInfo},
	}
	out := Warnings(in)
	if len(out) != 1 || out[0].Code != CodeUnknownKey {
		t.Fatalf("Warnings = %v, want only the warning-severity diagnostic", out)
	}
}
