<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0336 — Finalize selects the best merge method permitted by repository and branch policy](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-22-0336-finalize-s-go-merge-verb-hardcodes-merge-honor-the-repo-s-al.md)**
<!-- docket:backlink:end -->
# Finalize Effective Merge-Method Selection Implementation Plan

> **For agentic workers:** This plan is executed task-by-task by the resolved build skill (`docket-build`), one fresh worker per task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `finalize merge` selects the best merge method the repository and its base-branch rules actually permit (rebase → merge commit → squash), and every CLI command that advertises `--repo-dir` as "default: current directory" fulfils that promise at the CLI boundary.

**Architecture:** A shared `internal/cli` resolver turns an omitted `--repo-dir` into the process working directory before any application operation runs. `internal/githubcli` gains a closed `MergeMethod` vocabulary, two fail-closed capability probes (`gh api` repository settings and branch rules), a pure intersection/selection function, and a `MergeResult` value replacing `MergePullRequest`'s loose tuple. `internal/app/finalize_merge.go` maps the new `method-unavailable` outcome to a blocked refusal before any effect and surfaces the attempted method on the protocol document. Everything else in the finalize sequence is unchanged.

**Tech Stack:** Go 1.26, cobra, `gh` CLI (driven through the existing `runRequest` seam and the protocol-faithful fake `gh` test harnesses).

**Spec:** `docs/superpowers/specs/2026-08-21-finalize-effective-merge-method-design.md` (synchronized copy readable at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-08-21-finalize-effective-merge-method-design.md` while this change is active).

## Global Constraints

- **No config knob.** No `.docket.yml` key, resolver export, sample-config line, or README option for merge method. The preference order **rebase → merge commit → squash** is product policy, fixed in code.
- **Attempt exactly one method.** A GitHub rejection of the selected method stays `denied`; never retry a lower-priority method.
- **Fail closed.** Missing/malformed repository capability booleans, an unparseable rules payload, an empty `allowed_merge_methods` array, an unknown method token, or a failed probe is `unknown` (retain) — never a permissive default. A cleanly observed empty effective set is `blocked / merge-method-unavailable`, issued **before** any merge command.
- **Preserved conjuncts.** Exact `--match-head-commit`, the attended explicit-id `--admin` gate, no `--delete-branch`, authoritative post-attempt reprobe, and destination reachability verification are untouched. No test or implementation path may require the merge-result commit to have two parents or equal the original PR head.
- **Three-outcome discipline** (repo convention): value outcomes carry nil errors; `unknown` alone carries a typed `*Failure`, and never authorizes an effect.
- **Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers** (repo AGENTS.md rule; `tests/test_comment_anchor_style.sh` enforces part of it).
- **Suite:** the build gate runs the whole suite via `finalize.test_command` → `scripts/run-tests.sh` (read it from `.docket.yml`, never a second copy). Go mutation re-checks must defeat the test cache: `go test -count=1 ./internal/...`.
- **Guards are code:** every new assert added below gets its mutation check (strip the guarded thing, watch it redden) before it is trusted.

---

### Task 1: Shared `--repo-dir` current-directory resolver at the CLI boundary

**Files:**
- Create: `internal/cli/repodir.go`
- Create: `internal/cli/repodir_test.go`
- Modify: `internal/cli/artifact.go`, `internal/cli/change.go`, `internal/cli/context.go`, `internal/cli/evidence.go`, `internal/cli/finalize.go`, `internal/cli/pr.go`, `internal/cli/maintenance.go`, `internal/cli/root.go`, `internal/cli/workspace.go`, `internal/cli/run.go`

**Interfaces:**
- Consumes: nothing new; the existing cobra command tree and `runCLI(t, args...)` test helper (see `internal/cli/context_test.go` for its call shape: `out, errS, code := runCLI(t, "context", "implementation", "--repo-dir", dir, "--json")`).
- Produces: `func resolveRepoDir(c *cobra.Command) (string, error)` — used by every handler whose `--repo-dir` help text reads `(default: current directory)`.

Background: every handler currently does `repoDir, _ := c.Flags().GetString("repo-dir")` and passes the empty string into application operations, where GitHub/Git discovery rejects it (`invalid-request: invocation path is empty`, and `context finalize` degrades a real PR to `pr-unknown`). The one deliberate exception is `docket config` in `internal/cli/root.go`, whose help says `(required; used verbatim, no Git discovery)` and which is `MarkFlagRequired` — leave it untouched.

- [ ] **Step 1: Enumerate the sites mechanically (never hand-list)**

Run: `grep -n 'GetString("repo-dir")' internal/cli/*.go | grep -v _test` and `grep -n '"repo-dir"' internal/cli/*.go | grep 'default: current directory'`. Every `GetString` site whose command's flag help advertises `(default: current directory)` is in scope. The `configCmd` site in `root.go` (help: `required; used verbatim`) is the only exclusion. As of the branch tip the in-scope files are the ten listed above (`change.go` has 6 sites, `finalize.go` has 10, `workspace.go` 3, `context.go` 2, the rest 1 each) — but trust the grep, not this sentence.

- [ ] **Step 2: Write the failing tests**

`internal/cli/repodir_test.go`:

```go
package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newRepoDirProbe builds a minimal command carrying the defaulted flag, mirroring
// how the real subcommands declare it.
func newRepoDirProbe() *cobra.Command {
	c := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
	c.Flags().String("repo-dir", "", "repository directory to operate on (default: current directory)")
	return c
}

// TestResolveRepoDirExplicit: a non-empty explicit value is returned verbatim,
// with no cleaning, discovery, or substitution.
func TestResolveRepoDirExplicit(t *testing.T) {
	c := newRepoDirProbe()
	if err := c.Flags().Set("repo-dir", "/some/explicit/dir"); err != nil {
		t.Fatal(err)
	}
	got, err := resolveRepoDir(c)
	if err != nil || got != "/some/explicit/dir" {
		t.Fatalf("explicit value not returned verbatim: got=%q err=%v", got, err)
	}
}

// TestResolveRepoDirDefaultsToCwd: an omitted flag resolves through the process
// working directory — the invocation directory, not a Git-discovered root.
func TestResolveRepoDirDefaultsToCwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	got, err := resolveRepoDir(newRepoDirProbe())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wd, _ := os.Getwd() // compare against the same canonicalization Getwd applies
	if got != wd {
		t.Fatalf("omitted flag did not resolve to the working directory: got=%q want=%q", got, wd)
	}
}

// TestResolveRepoDirCwdFailure: when the current directory cannot be determined,
// the resolver returns an argument error before any operation runs.
func TestResolveRepoDirCwdFailure(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Skipf("cannot remove cwd on this platform: %v", err)
	}
	if _, err := resolveRepoDir(newRepoDirProbe()); err == nil {
		t.Fatal("expected an error when the working directory is unresolvable")
	}
}

// repoDirDefaultMatchesExplicit runs one command twice — once with an explicit
// --repo-dir and once with the flag omitted from that same directory — and
// requires byte-identical documents. Deleting the resolver reverts the no-flag
// run to the empty-path failure, so this assert pins the RESOLVED non-default
// value, not merely "some output appeared".
func repoDirDefaultMatchesExplicit(t *testing.T, args ...string) {
	t.Helper()
	dir := t.TempDir()
	explicit, _, _ := runCLI(t, append(append([]string{}, args...), "--repo-dir", dir, "--json")...)
	t.Chdir(dir)
	defaulted, _, _ := runCLI(t, append(append([]string{}, args...), "--json")...)
	if defaulted != explicit {
		t.Fatalf("omitted --repo-dir diverges from explicit:\nexplicit:  %q\ndefaulted: %q", explicit, defaulted)
	}
	if strings.Contains(defaulted, "invocation path is empty") {
		t.Fatalf("empty repo-dir leaked into the operation: %q", defaulted)
	}
}

// TestContextFinalizeRepoDirDefault: the read-only finalize context reaches
// repository discovery from the working directory (no pr-unknown degrade from an
// empty path).
func TestContextFinalizeRepoDirDefault(t *testing.T) {
	repoDirDefaultMatchesExplicit(t, "context", "finalize")
}

// TestFinalizeMergeRepoDirDefault: the mutating merge verb resolves the same way.
func TestFinalizeMergeRepoDirDefault(t *testing.T) {
	repoDirDefaultMatchesExplicit(t, "finalize", "merge", "--id", "1", "--version", "v", "--head", strings.Repeat("a", 40))
}

// TestStatusRepoDirDefault: at least one NON-finalize command family shares the
// contract, so the resolver cannot regress into a finalize-only special case.
func TestStatusRepoDirDefault(t *testing.T) {
	repoDirDefaultMatchesExplicit(t, "status")
}
```

Note: against an empty temp directory these operations fail (not a repository) — that is fine; the assert is that the defaulted and explicit runs fail **identically**, proving the empty string never reached the operation. If a document embeds an absolute path that `Getwd` canonicalizes differently than `t.TempDir()` returns on macOS (`/private/tmp` vs `/tmp`), pass `dir` through `filepath.EvalSymlinks` before the explicit run so both runs name the same spelling.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'RepoDir' -count=1 -v`
Expected: FAIL — `resolveRepoDir` undefined; the wiring tests fail with divergent documents.

- [ ] **Step 4: Implement the resolver**

`internal/cli/repodir.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// resolveRepoDir fulfils the advertised "--repo-dir ... (default: current
// directory)" contract at the CLI boundary. A non-empty explicit value is
// returned verbatim; an omitted value resolves through the process working
// directory; a failure to determine the working directory is an argument error
// returned before dependencies are constructed or an operation runs. The
// application and adapter layers keep requiring a concrete directory — empty
// input stays invalid at those boundaries. The resolved directory is the
// invocation working directory; it is never replaced with a primary worktree or
// a Git-discovered root.
func resolveRepoDir(c *cobra.Command) (string, error) {
	dir, _ := c.Flags().GetString("repo-dir")
	if dir != "" {
		return dir, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("--repo-dir omitted and the current directory could not be determined: %w", err)
	}
	return wd, nil
}
```

Use the same cobra import path the package already uses (see `internal/cli/root.go`).

- [ ] **Step 5: Route every in-scope handler through it**

In each site from Step 1, replace

```go
repoDir, _ := c.Flags().GetString("repo-dir")
```

with

```go
repoDir, err := resolveRepoDir(c)
if err != nil {
	return err
}
```

Where the handler already declares `err` later (`deps, err := newFinalizeDeps()` etc.), the new `err` declaration comes first, so change the later ones to `=` as needed to keep the compiler happy. Do not touch `configCmd`. Re-run the Step 1 grep afterward and confirm zero remaining `GetString("repo-dir")` sites outside `repodir.go` itself, `configCmd`, and test files.

- [ ] **Step 6: Run the package tests**

Run: `go test ./internal/cli/ -count=1`
Expected: PASS (including all pre-existing tests — they pass `--repo-dir` explicitly and must be unaffected).

- [ ] **Step 7: Mutation-check the wiring assert**

Temporarily revert one routed handler (`context finalize` in `internal/cli/context.go`) to the bare `GetString` form and re-run `go test ./internal/cli/ -run 'ContextFinalizeRepoDirDefault' -count=1`. Expected: FAIL. Restore the resolver call (re-apply your edit — do **not** `git checkout` the file, which would destroy the whole task; keep a copy of the edited file first). Re-run: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/repodir.go internal/cli/repodir_test.go internal/cli/artifact.go internal/cli/change.go internal/cli/context.go internal/cli/evidence.go internal/cli/finalize.go internal/cli/pr.go internal/cli/maintenance.go internal/cli/root.go internal/cli/workspace.go internal/cli/run.go
git commit -m "fix(0336): fulfil the advertised --repo-dir current-directory default at the CLI boundary"
```

---

### Task 2: Merge-method vocabulary and pure selection policy

**Files:**
- Create: `internal/githubcli/mergemethod.go`
- Create: `internal/githubcli/mergemethod_test.go`

**Interfaces:**
- Consumes: nothing outside the package.
- Produces (package-internal except the exported type):
  - `type MergeMethod string` with `MethodRebase MergeMethod = "rebase"`, `MethodMerge MergeMethod = "merge"`, `MethodSquash MergeMethod = "squash"`
  - `func (m MergeMethod) mergeFlag() string` → `"--rebase"` / `"--merge"` / `"--squash"`, `""` for anything else
  - `type methodSet struct { rebase, merge, squash bool }`
  - `func (s methodSet) intersect(o methodSet) methodSet`
  - `func (s methodSet) list() []MergeMethod` (preference order, for diagnostics)
  - `func selectMergeMethod(eff methodSet) (MergeMethod, bool)`

- [ ] **Step 1: Write the failing tests**

`internal/githubcli/mergemethod_test.go`:

```go
package githubcli

import (
	"reflect"
	"testing"
)

// TestSelectMergeMethodPriority covers every effective-set combination: the
// selection is the fixed preference order rebase → merge → squash, else
// unavailable. Mutation discipline: each row that removes the previously
// selected method and requires the next one IS the mutation check — a selection
// that ignores the removal reddens the row.
func TestSelectMergeMethodPriority(t *testing.T) {
	cases := []struct {
		name string
		eff  methodSet
		want MergeMethod
		ok   bool
	}{
		{"all enabled -> rebase", methodSet{rebase: true, merge: true, squash: true}, MethodRebase, true},
		{"rebase only", methodSet{rebase: true}, MethodRebase, true},
		{"rebase+squash -> rebase", methodSet{rebase: true, squash: true}, MethodRebase, true},
		{"merge+squash -> merge", methodSet{merge: true, squash: true}, MethodMerge, true},
		{"merge only", methodSet{merge: true}, MethodMerge, true},
		{"squash only -> squash", methodSet{squash: true}, MethodSquash, true},
		{"empty -> unavailable", methodSet{}, "", false},
	}
	for _, c := range cases {
		got, ok := selectMergeMethod(c.eff)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestMethodSetIntersect: repository permissions and branch rules compose by
// intersection, per element.
func TestMethodSetIntersect(t *testing.T) {
	a := methodSet{rebase: true, merge: true}
	b := methodSet{merge: true, squash: true}
	if got := a.intersect(b); got != (methodSet{merge: true}) {
		t.Fatalf("intersect: got %+v", got)
	}
	if got := a.intersect(methodSet{}); got != (methodSet{}) {
		t.Fatalf("intersect with empty must be empty: got %+v", got)
	}
}

// TestMergeFlag: the closed vocabulary renders exactly one gh flag per method;
// anything outside the vocabulary renders NOTHING (the act path guards on it).
func TestMergeFlag(t *testing.T) {
	for m, want := range map[MergeMethod]string{
		MethodRebase: "--rebase", MethodMerge: "--merge", MethodSquash: "--squash", MergeMethod("bogus"): "",
	} {
		if got := m.mergeFlag(); got != want {
			t.Errorf("mergeFlag(%q) = %q, want %q", m, got, want)
		}
	}
}

// TestMethodSetList: the diagnostic rendering names permitted methods in
// preference order.
func TestMethodSetList(t *testing.T) {
	got := methodSet{rebase: true, squash: true}.list()
	if !reflect.DeepEqual(got, []MergeMethod{MethodRebase, MethodSquash}) {
		t.Fatalf("list: got %v", got)
	}
	if l := (methodSet{}).list(); len(l) != 0 {
		t.Fatalf("empty set must list nothing, got %v", l)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/githubcli/ -run 'SelectMergeMethod|MethodSet|MergeFlag' -count=1 -v`
Expected: FAIL — types undefined.

- [ ] **Step 3: Implement**

`internal/githubcli/mergemethod.go` (probes are added in Task 3; this task creates the file with the vocabulary and policy):

```go
package githubcli

// This file owns the merge-method vocabulary and policy: the closed MergeMethod
// set, the capability probes over GitHub's repository settings and active
// branch rules, and the pure fixed-priority selection "rebase, else merge, else
// squash, else unavailable". There is deliberately NO configuration surface —
// the preference order is product policy. Repository permissions and branch
// rules compose by intersection; unobservable or malformed policy fails closed
// (three-outcome discipline; learnings: probe-error-is-not-clean-absence).

// MergeMethod is the closed vocabulary of merge methods Docket can select.
type MergeMethod string

const (
	// MethodRebase → `gh pr merge --rebase`.
	MethodRebase MergeMethod = "rebase"
	// MethodMerge → `gh pr merge --merge` (a merge commit).
	MethodMerge MergeMethod = "merge"
	// MethodSquash → `gh pr merge --squash`.
	MethodSquash MergeMethod = "squash"
)

// mergeFlag renders the gh `pr merge` flag for a vocabulary member, and nothing
// for anything else — the act path guards on a non-empty flag rather than
// defaulting permissively.
func (m MergeMethod) mergeFlag() string {
	switch m {
	case MethodRebase:
		return "--rebase"
	case MethodMerge:
		return "--merge"
	case MethodSquash:
		return "--squash"
	default:
		return ""
	}
}

// methodSet is a capability set over the three methods.
type methodSet struct {
	rebase, merge, squash bool
}

// intersect composes two capability sets; multiple applicable restrictions
// always narrow, never widen.
func (s methodSet) intersect(o methodSet) methodSet {
	return methodSet{
		rebase: s.rebase && o.rebase,
		merge:  s.merge && o.merge,
		squash: s.squash && o.squash,
	}
}

// list renders the permitted methods in preference order, for diagnostics.
func (s methodSet) list() []MergeMethod {
	out := []MergeMethod{}
	if s.rebase {
		out = append(out, MethodRebase)
	}
	if s.merge {
		out = append(out, MethodMerge)
	}
	if s.squash {
		out = append(out, MethodSquash)
	}
	return out
}

// selectMergeMethod is the pure ordered selection over the effective set:
// rebase, else merge, else squash, else unavailable. `--admin` never widens the
// set or reorders it.
func selectMergeMethod(eff methodSet) (MergeMethod, bool) {
	switch {
	case eff.rebase:
		return MethodRebase, true
	case eff.merge:
		return MethodMerge, true
	case eff.squash:
		return MethodSquash, true
	default:
		return "", false
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/githubcli/ -run 'SelectMergeMethod|MethodSet|MergeFlag' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/githubcli/mergemethod.go internal/githubcli/mergemethod_test.go
git commit -m "feat(0336): closed merge-method vocabulary and fixed-priority selection"
```

---

### Task 3: Repository and branch-policy capability probes (fail closed)

**Files:**
- Modify: `internal/githubcli/mergemethod.go`
- Modify: `internal/githubcli/mergemethod_test.go`

**Interfaces:**
- Consumes: `c.run(ctx, runRequest{...}) (runResult, *Failure)` (see `internal/githubcli/client.go`, `type runRequest struct { op, dir string; args []string; stdin []byte; network bool }`), `newFailure(op, stage, kind, msg, err)`, `stderrExcerpt`, the fake-gh scenario harness in `internal/githubcli/fakegh_test.go` (`fakeScenario`/`fakeArm`, first-match-on-`ArgvPrefix`, unmatched invocations exit 64 — find an existing probe test such as those in `internal/githubcli/repo_test.go` or `merge_test.go` for how a test writes the scenario file and constructs the client).
- Produces:
  - `func (c *Client) probeRepoMergeMethods(ctx context.Context, repo Repository) (methodSet, *Failure)`
  - `func (c *Client) probeBranchMergeRules(ctx context.Context, repo Repository, baseBranch string) (methodSet, *Failure)`
  - `const mergeMethodOp = "merge-method-capability"`

GitHub read surfaces (both via `gh api`, which prints the JSON response body and exits non-zero on HTTP failure):
- Repository settings: `gh api --hostname <host> repos/<owner>/<name>` → object carrying `allow_rebase_merge`, `allow_merge_commit`, `allow_squash_merge` booleans.
- Active branch rules: `gh api --hostname <host> repos/<owner>/<name>/rules/branches/<url-path-escaped branch>` → JSON **array** of rule objects `{"type": ..., "parameters": {...}}`. A `"type":"pull_request"` rule may carry `parameters.allowed_merge_methods` (array of `"merge"`/`"squash"`/`"rebase"` tokens); a `"type":"required_linear_history"` rule excludes merge commits. The exact live token casing is outside-repo truth — the named human verification item in Task 8 observes real payloads; in code, exactly these lowercase tokens are recognized and anything else fails closed.

- [ ] **Step 1: Write the failing tests**

Append to `internal/githubcli/mergemethod_test.go` (reuse the package's existing scenario-writing helper pattern; the snippets below spell the arms — adapt the client/scenario construction lines to the exact helper the neighboring tests use):

```go
// TestProbeRepoMergeMethods: the three booleans decode explicitly from gh's
// repository endpoint; the argv is exact (api, --hostname, repos/o/n).
func TestProbeRepoMergeMethods(t *testing.T) {
	c := newFakeGHClient(t, fakeScenario{Invocations: []fakeArm{{
		ArgvPrefix: []string{"api", "--hostname", "github.com", "repos/octo/widgets"},
		Stdout:     `{"allow_rebase_merge":false,"allow_merge_commit":true,"allow_squash_merge":true}`,
	}}})
	set, f := c.probeRepoMergeMethods(context.Background(), Repository{Host: "github.com", Owner: "octo", Name: "widgets"})
	if f != nil {
		t.Fatalf("unexpected failure: %v", f)
	}
	if set != (methodSet{merge: true, squash: true}) {
		t.Fatalf("decoded set: %+v", set)
	}
}

// TestProbeRepoMergeMethodsFailsClosed: a missing boolean, malformed JSON, or a
// non-zero gh exit is a typed Failure — never a permissive default set.
func TestProbeRepoMergeMethodsFailsClosed(t *testing.T) {
	cases := []struct{ name string; arm fakeArm }{
		{"missing field", fakeArm{ArgvPrefix: []string{"api"}, Stdout: `{"allow_rebase_merge":true,"allow_squash_merge":true}`}},
		{"malformed json", fakeArm{ArgvPrefix: []string{"api"}, Stdout: `{"allow_rebase`}},
		{"http failure", fakeArm{ArgvPrefix: []string{"api"}, Exit: 1, Stderr: "gh: Not Found (HTTP 404)"}},
	}
	for _, cse := range cases {
		c := newFakeGHClient(t, fakeScenario{Invocations: []fakeArm{cse.arm}})
		set, f := c.probeRepoMergeMethods(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"})
		if f == nil || set != (methodSet{}) {
			t.Errorf("%s: must fail closed, got set=%+v f=%v", cse.name, set, f)
		}
	}
}

// TestProbeBranchMergeRules: rules restrict by intersection; linear history
// removes merge; no method-specific rule contributes no restriction; the base
// branch is path-escaped so "feat/parent" is ONE endpoint segment.
func TestProbeBranchMergeRules(t *testing.T) {
	rules := `[
		{"type":"pull_request","parameters":{"allowed_merge_methods":["merge","squash"]}},
		{"type":"pull_request","parameters":{"allowed_merge_methods":["squash","rebase"]}},
		{"type":"required_linear_history","parameters":{}}
	]`
	c := newFakeGHClient(t, fakeScenario{Invocations: []fakeArm{{
		ArgvPrefix: []string{"api", "--hostname", "github.com", "repos/octo/widgets/rules/branches/feat%2Fparent"},
		Stdout:     rules,
	}}})
	set, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "octo", Name: "widgets"}, "feat/parent")
	if f != nil {
		t.Fatalf("unexpected failure: %v", f)
	}
	// intersection of the two allowed_merge_methods rules is {squash}; linear
	// history removes merge (already absent). rebase excluded by the first rule.
	if set != (methodSet{squash: true}) {
		t.Fatalf("composed branch set: %+v", set)
	}
}

// TestProbeBranchMergeRulesNoRestriction: an empty rules array (or rules of
// unrelated types, or a pull_request rule without allowed_merge_methods)
// contributes no restriction — all three methods stay permitted.
func TestProbeBranchMergeRulesNoRestriction(t *testing.T) {
	for name, body := range map[string]string{
		"empty array":     `[]`,
		"unrelated rule":  `[{"type":"deletion","parameters":{}}]`,
		"pr rule no key":  `[{"type":"pull_request","parameters":{"required_approving_review_count":0}}]`,
	} {
		c := newFakeGHClient(t, fakeScenario{Invocations: []fakeArm{{ArgvPrefix: []string{"api"}, Stdout: body}}})
		set, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main")
		if f != nil || set != (methodSet{rebase: true, merge: true, squash: true}) {
			t.Errorf("%s: want unrestricted set, got %+v f=%v", name, set, f)
		}
	}
}

// TestProbeBranchMergeRulesFailsClosed: malformed payloads, an EMPTY
// allowed_merge_methods array, an unknown token, and a failed request are all
// unobservable/invalid policy — a typed Failure, never a guess.
func TestProbeBranchMergeRulesFailsClosed(t *testing.T) {
	cases := map[string]fakeArm{
		"malformed json": {ArgvPrefix: []string{"api"}, Stdout: `{"not":"an array"}`},
		"empty methods":  {ArgvPrefix: []string{"api"}, Stdout: `[{"type":"pull_request","parameters":{"allowed_merge_methods":[]}}]`},
		"unknown token":  {ArgvPrefix: []string{"api"}, Stdout: `[{"type":"pull_request","parameters":{"allowed_merge_methods":["MERGE"]}}]`},
		"http failure":   {ArgvPrefix: []string{"api"}, Exit: 1, Stderr: "gh: Server Error (HTTP 500)"},
	}
	for name, arm := range cases {
		c := newFakeGHClient(t, fakeScenario{Invocations: []fakeArm{arm}})
		if _, f := c.probeBranchMergeRules(context.Background(), Repository{Host: "github.com", Owner: "o", Name: "n"}, "main"); f == nil {
			t.Errorf("%s: must fail closed", name)
		}
	}
}
```

If the package has no shared `newFakeGHClient` helper, follow exactly how `internal/githubcli/repo_test.go` or `merge_test.go` builds a client over a scenario file (re-exec of the test binary with `GO_WANT_FAKE_GH`), and name your helper accordingly; do not invent a second harness.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/githubcli/ -run 'ProbeRepoMergeMethods|ProbeBranchMergeRules' -count=1 -v`
Expected: FAIL — probes undefined.

- [ ] **Step 3: Implement the probes**

Append to `internal/githubcli/mergemethod.go` (add `context`, `encoding/json`, `net/url` imports):

```go
// mergeMethodOp labels every Failure raised while observing merge-method policy.
const mergeMethodOp = "merge-method-capability"

// repoMethodsJSON decodes the repository merge-method settings. Pointer fields
// force explicit presence: a missing, malformed, or unrecognized boolean is
// invalid external output, never a permissive default.
type repoMethodsJSON struct {
	AllowRebase *bool `json:"allow_rebase_merge"`
	AllowMerge  *bool `json:"allow_merge_commit"`
	AllowSquash *bool `json:"allow_squash_merge"`
}

// probeRepoMergeMethods reads the repository-enabled merge methods from
// GitHub's repository endpoint via `gh api`, addressed by the validated host
// and owner/name — never by CWD inference.
func (c *Client) probeRepoMergeMethods(ctx context.Context, repo Repository) (methodSet, *Failure) {
	res, f := c.run(ctx, runRequest{
		op:      mergeMethodOp,
		args:    []string{"api", "--hostname", repo.Host, "repos/" + repo.Owner + "/" + repo.Name},
		network: true,
	})
	if f != nil {
		return methodSet{}, f
	}
	if res.exitCode != 0 {
		return methodSet{}, newFailure(mergeMethodOp, StageInvoke, KindExternal,
			"gh api repository settings failed: "+stderrExcerpt(res.stderr), nil)
	}
	var raw repoMethodsJSON
	if err := json.Unmarshal(res.stdout, &raw); err != nil {
		return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput, "repository settings are not valid JSON", err)
	}
	if raw.AllowRebase == nil || raw.AllowMerge == nil || raw.AllowSquash == nil {
		return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput,
			"repository settings omit a merge-method capability field", nil)
	}
	return methodSet{rebase: *raw.AllowRebase, merge: *raw.AllowMerge, squash: *raw.AllowSquash}, nil
}

// branchRuleJSON is one active rule from GitHub's branch-rules endpoint.
type branchRuleJSON struct {
	Type       string          `json:"type"`
	Parameters json.RawMessage `json:"parameters"`
}

// prRuleParamsJSON extracts the merge-method restriction from a pull_request
// rule. The pointer distinguishes "key absent — no restriction" from "key
// present and empty — invalid policy that fails closed".
type prRuleParamsJSON struct {
	AllowedMergeMethods *[]string `json:"allowed_merge_methods"`
}

// probeBranchMergeRules reads the active rules for the exact PR base branch and
// composes the branch-permitted method set: every applicable
// allowed_merge_methods restriction intersects, and required_linear_history
// removes merge-commit semantics. No method-specific rule means no additional
// restriction. The full branch name is path-escaped so a stacked destination
// like "feat/parent" is one endpoint segment. Constraints GitHub does not
// expose through this read surface can still reject the later merge command —
// that remains an ordinary denial and never triggers fallback.
func (c *Client) probeBranchMergeRules(ctx context.Context, repo Repository, baseBranch string) (methodSet, *Failure) {
	if baseBranch == "" {
		return methodSet{}, newFailure(mergeMethodOp, StageValidate, KindInvalidInput, "base branch is empty", nil)
	}
	path := "repos/" + repo.Owner + "/" + repo.Name + "/rules/branches/" + url.PathEscape(baseBranch)
	res, f := c.run(ctx, runRequest{
		op:      mergeMethodOp,
		args:    []string{"api", "--hostname", repo.Host, path},
		network: true,
	})
	if f != nil {
		return methodSet{}, f
	}
	if res.exitCode != 0 {
		return methodSet{}, newFailure(mergeMethodOp, StageInvoke, KindExternal,
			"gh api branch rules failed: "+stderrExcerpt(res.stderr), nil)
	}
	var rules []branchRuleJSON
	if err := json.Unmarshal(res.stdout, &rules); err != nil {
		return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput, "branch rules are not a valid JSON array", err)
	}
	permitted := methodSet{rebase: true, merge: true, squash: true}
	for _, r := range rules {
		switch r.Type {
		case "pull_request":
			if len(r.Parameters) == 0 {
				continue
			}
			var p prRuleParamsJSON
			if err := json.Unmarshal(r.Parameters, &p); err != nil {
				return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput, "pull_request rule parameters undecodable", err)
			}
			if p.AllowedMergeMethods == nil {
				continue // rule present, no merge-method restriction
			}
			if len(*p.AllowedMergeMethods) == 0 {
				return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput,
					"allowed_merge_methods is present but empty", nil)
			}
			var s methodSet
			for _, tok := range *p.AllowedMergeMethods {
				switch tok {
				case "rebase":
					s.rebase = true
				case "merge":
					s.merge = true
				case "squash":
					s.squash = true
				default:
					return methodSet{}, newFailure(mergeMethodOp, StageDecode, KindInvalidOutput,
						"unknown merge-method token "+strconv.Quote(tok)+" in branch rules", nil)
				}
			}
			permitted = permitted.intersect(s)
		case "required_linear_history":
			permitted.merge = false
		}
	}
	return permitted, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/githubcli/ -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-check the escaping and the fail-closed empty array**

(a) Temporarily replace `url.PathEscape(baseBranch)` with `baseBranch`; `TestProbeBranchMergeRules` must FAIL (the fake's argv prefix no longer matches → exit 64). Restore. (b) Temporarily make the empty `allowed_merge_methods` case `continue` instead of failing; `TestProbeBranchMergeRulesFailsClosed/empty methods` must FAIL. Restore. Re-run `go test ./internal/githubcli/ -count=1`: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/githubcli/mergemethod.go internal/githubcli/mergemethod_test.go
git commit -m "feat(0336): fail-closed repository and branch-rule merge-method probes"
```

---

### Task 4: Wire selection into `MergePullRequest` behind a `MergeResult` value

**Files:**
- Modify: `internal/githubcli/merge.go`
- Modify: `internal/githubcli/merge_test.go`
- Modify: `internal/app/finalize_context.go` (the `FinalizeGitHub` interface)
- Modify: `internal/app/finalize_merge.go` (call-site adaptation only)
- Modify: `internal/app/finalize_block_test.go`, `internal/app/finalize_cleanup_test.go`, `internal/app/finalize_publish_test.go`, `internal/app/finalize_closeout_test.go`, `internal/app/finalize_merge_test.go` (mechanical fake updates)

**Interfaces:**
- Consumes: Task 2's `selectMergeMethod`/`methodSet`/`mergeFlag`, Task 3's two probes, the existing `mergeSnapshot`/`verifyMerge`/`probeMergeSnapshot` machinery in `internal/githubcli/merge.go`.
- Produces:
  - `MergeMethodUnavailable MergeOutcome = "method-unavailable"` (new member of the closed outcome set in `merge.go`)
  - `type MergeResult struct { Outcome MergeOutcome; Method MergeMethod; Facts MergedFacts; RepoMethods, BranchMethods []MergeMethod }`
  - New signature: `func (c *Client) MergePullRequest(ctx context.Context, repo Repository, number int, expectedHead ObjectRef, admin bool) (MergeResult, error)` — `Method` is set exactly when a merge command was issued (success, authoritative denial, or lost-response recovery); empty for validation failures, pre-effect refusals, and already-merged recovery. `RepoMethods`/`BranchMethods` are populated only on `MergeMethodUnavailable`.
  - `ProbeMerged` keeps its existing `(MergeOutcome, MergedFacts, error)` signature — read-only, unchanged.
  - `FinalizeGitHub.MergePullRequest` in `internal/app/finalize_context.go` mirrors the new signature.

This task changes a cross-package signature, so githubcli, app, and every fake move together — the tree must build and the whole `go test ./internal/...` run must be green at the task boundary. App-side **behavior** stays as close to unchanged as possible here: the new `MergeMethodUnavailable` outcome falls through `FinalizeMerge`'s existing `default:` arm (fail-closed `internal-error`) until Task 5 maps it properly.

- [ ] **Step 1: Update `internal/githubcli/merge_test.go` expectations first**

The existing merge tests script `pr view → pr merge --merge → pr view` arms. Update them for the new sequence and signature; add the new coverage:

1. Every existing act-path test gains two arms between the pre-decision `pr view` and the `pr merge`: a repository-settings arm (`ArgvPrefix: ["api", "--hostname", <host>, "repos/<o>/<n>"]`) and a branch-rules arm for the snapshot's `baseRefName` (`.../rules/branches/<escaped-base>`). Where a test's scenario is `Sequential: true`, keep the arm order probe → repo settings → branch rules → act → reprobe.
2. All-methods-enabled settings (`{"allow_rebase_merge":true,"allow_merge_commit":true,"allow_squash_merge":true}`, rules `[]`) now select **rebase**: the act arm's `ArgvPrefix` asserts `--rebase` (and retains explicit `--repo`, exact `--match-head-commit`, optional `--admin`, and the absence of `--delete-branch` — keep every existing argv assertion, updating only the method flag).
3. Add `TestMergeSelectsMergeWhenRebaseDisabled` (settings merge+squash → act arm asserts `--merge`) and `TestMergeSelectsSquashOnly` (squash only → `--squash`).
4. Add `TestMergeMethodUnavailableIssuesNoMerge`: settings all-false, rules `[]` → outcome `MergeMethodUnavailable`, nil error, `Method == ""`, `RepoMethods` empty, `BranchMethods` carrying all three — and the scenario contains **no** `pr merge` arm, so any issued merge exits 64 and reddens the test. Assert via the witness log (the package's existing invocation-recording mechanism) that zero `pr merge` invocations occurred.
5. Add `TestMergeProbeFailureIssuesNoMerge`: the repo-settings arm exits 1 → outcome `MergeUnknown` with a non-nil error, zero `pr merge` invocations.
6. Add `TestMergeDeniedNeverRetriesAnotherMethod`: settings all-true, act arm (`--rebase`) exits 1, reprobe arm returns the PR cleanly open/mergeable/same-head → outcome `MergeDenied`, `Method == MethodRebase`, and exactly **one** `pr merge` invocation in the witness log (no `--merge`/`--squash` fallback attempt).
7. Already-merged pre-decision tests: outcome `MergeAlreadyMerged` with `Method == ""` and **no** capability-probe arms consumed (no `api` invocations in the witness log — already-merged recovery performs no probes).
8. Signature: everywhere, `outcome, facts, err := c.MergePullRequest(...)` becomes `res, err := c.MergePullRequest(...)` with `res.Outcome` / `res.Facts` / `res.Method`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/githubcli/ -run 'Merge' -count=1`
Expected: FAIL (compile errors on `MergeResult`, then behavioral failures).

- [ ] **Step 3: Implement in `internal/githubcli/merge.go`**

(a) Add to the outcome block:

```go
	// MergeMethodUnavailable: repository settings and branch rules leave no
	// permitted merge method; the incompatible policy was observed cleanly and
	// NO merge was issued. Distinct from denied (nothing was attempted) and
	// from unknown (the policy WAS observable).
	MergeMethodUnavailable MergeOutcome = "method-unavailable"
```

(b) Add the result value:

```go
// MergeResult is the outcome of one MergePullRequest call. Method is attempt
// metadata, not a merged fact: it is set exactly when Docket issued the merge
// command — success, authoritative denial, or a lost response later recovered —
// and empty for validation failures, pre-effect refusals, and already-merged
// recovery (Docket did not choose the historical merge's method). RepoMethods
// and BranchMethods are populated only on MergeMethodUnavailable, naming the
// two observed permitted sets so a human can correct the conflicting setting.
type MergeResult struct {
	Outcome MergeOutcome
	Method  MergeMethod
	Facts   MergedFacts
	RepoMethods, BranchMethods []MergeMethod
}
```

(c) Rewrite `MergePullRequest` to return `(MergeResult, error)`. The validation and pre-decision sections keep their logic, each return wrapped, e.g. `return MergeResult{Outcome: MergeUnknown}, newFailure(...)`, `return MergeResult{Outcome: MergeAlreadyMerged, Facts: snap.facts()}, nil`. Between the mergeable authorization and the act, insert:

```go
	// Policy: read repository-enabled methods and the active rules for the PR's
	// ACTUAL base branch, intersect, and select the fixed priority. An
	// unobservable policy is unknown; a cleanly observed empty set is
	// method-unavailable. Neither issues a merge.
	repoSet, pf := c.probeRepoMergeMethods(ctx, repo)
	if pf != nil {
		return MergeResult{Outcome: MergeUnknown}, pf
	}
	branchSet, pf := c.probeBranchMergeRules(ctx, repo, snap.pr.BaseBranch)
	if pf != nil {
		return MergeResult{Outcome: MergeUnknown}, pf
	}
	method, ok := selectMergeMethod(repoSet.intersect(branchSet))
	if !ok {
		return MergeResult{
			Outcome:       MergeMethodUnavailable,
			RepoMethods:   repoSet.list(),
			BranchMethods: branchSet.list(),
		}, nil
	}
```

The act uses the selected flag (guarding the closed vocabulary):

```go
	flag := method.mergeFlag()
	if flag == "" {
		return MergeResult{Outcome: MergeUnknown}, newFailure(mergeOp, StageValidate, KindInvalidInput,
			"selected merge method outside the closed vocabulary", nil)
	}
	args := []string{
		"pr", "merge", strconv.Itoa(number),
		"--repo", repo.Spec(),
		flag,
		"--match-head-commit", string(expectedHead),
	}
```

(d) `verifyMerge` gains the method: `func (c *Client) verifyMerge(ctx context.Context, repo Repository, number int, expectedHead ObjectRef, mergeRejected bool, method MergeMethod) (MergeResult, error)` — every return carries `Method: method` (the merge command WAS issued on every path that reaches it from the act; the launch-failure path in `MergePullRequest` still returns `MergeUnknown` with an empty Method directly, since gh never started). Its outcome logic is unchanged.

(e) `ProbeMerged` and `probeMergeSnapshot` are untouched. Update `merge.go`'s file-header comment to describe method selection (anchor prose on symbol names, never line numbers).

- [ ] **Step 4: Mechanically update the app layer to the new signature**

- `internal/app/finalize_context.go`: `MergePullRequest(ctx context.Context, repo githubcli.Repository, number int, expectedHead githubcli.ObjectRef, admin bool) (githubcli.MergeResult, error)` in the `FinalizeGitHub` interface.
- `internal/app/finalize_merge.go`, in `FinalizeMerge`: replace `mo, facts, merr := deps.GitHub.MergePullRequest(...)` with `mres, merr := deps.GitHub.MergePullRequest(...)`; switch on `mres.Outcome`; use `mres.Facts` where `facts` was used. Add **no** new case yet — `MergeMethodUnavailable` reaches the existing `default:` arm (fail-closed `internal-error`) until Task 5.
- The four panic-body fakes (`finalize_block_test.go`, `finalize_cleanup_test.go`, `finalize_publish_test.go`, `finalize_closeout_test.go`): change the return type to `(githubcli.MergeResult, error)`; bodies stay `panic(...)`.
- The recording fake in `finalize_merge_test.go`: return `githubcli.MergeResult{Outcome: ..., Facts: ...}` from its scripted state; keep its call recording. Do **not** hand-author a second selection policy inside any fake — the fakes return whatever the test scripts, and only the githubcli tests exercise selection.

- [ ] **Step 5: Run the affected packages**

Run: `go build ./... && go test ./internal/githubcli/ ./internal/app/ -count=1`
Expected: PASS (app behavior unchanged for every previously reachable outcome; `finalize_e2e_test.go` still passes because the e2e fake gh's default repo settings do not exist yet — if the e2e suite fails here because `gh api` is unmatched (fake exits 64 → `MergeUnknown`), extend the e2e fake **now** with the minimal all-true `api` handler described in Task 6 Step 2 and defer everything else about Task 6; note it in the commit message).

- [ ] **Step 6: Commit**

```bash
git add internal/githubcli/merge.go internal/githubcli/merge_test.go internal/app/finalize_context.go internal/app/finalize_merge.go internal/app/finalize_block_test.go internal/app/finalize_cleanup_test.go internal/app/finalize_publish_test.go internal/app/finalize_closeout_test.go internal/app/finalize_merge_test.go
git commit -m "feat(0336): MergePullRequest selects the effective merge method behind a MergeResult value"
```

(Include `internal/app/finalize_e2e_test.go` in the add list if Step 5's contingency fired.)

---

### Task 5: App mapping — `merge-method-unavailable` blocked reason and the attempted-method field

**Files:**
- Modify: `internal/app/finalize_merge.go`
- Modify: `internal/app/finalize_merge_test.go`

**Interfaces:**
- Consumes: Task 4's `githubcli.MergeResult` / `githubcli.MergeMethodUnavailable`, the existing `mergeRefusal(result Result, disposition, reason, message string, id int)`, `newMergeResult`, `verifyMerge` (app-level), and the constants `MergeDispBlocked`, `ReasonMergeDenied` (the new reason constant goes beside `ReasonMergeDenied` in `finalize_merge.go`'s reason block).
- Produces:
  - `ReasonMergeMethodUnavailable = "merge-method-unavailable"`
  - `FinalizeMergeResult.Method string` with tag `json:"method,omitempty"` — the method Docket attempted; absent when no merge command was issued (validation failures, pre-effect blocks, already-merged recovery).

- [ ] **Step 1: Write the failing tests**

Extend `internal/app/finalize_merge_test.go`, following its existing scenario/fixture style (a recording fake GitHub scripted per test):

1. `TestFinalizeMergeMethodUnavailableBlocks`: the fake's `MergePullRequest` returns `githubcli.MergeResult{Outcome: githubcli.MergeMethodUnavailable, RepoMethods: []githubcli.MergeMethod{"squash"}, BranchMethods: []githubcli.MergeMethod{"rebase", "merge"}}`. Assert the document: `Result == ResultBlocked`, `Disposition == MergeDispBlocked`, `Reason == "merge-method-unavailable"` (assert the literal token, not the constant — the constant equaling itself proves nothing), `Merge == nil`, `Method == ""`, and the `Message` contains both `"squash"` (repo-enabled) and `"rebase"` (branch-permitted) so the human can see the conflicting sets. Also assert it is **not** reason `merge-denied` and **not** disposition `unknown`.
2. `TestFinalizeMergeReportsAttemptedMethod`: a successful scripted merge with `Method: githubcli.MethodRebase` → the applied document carries `Method == "rebase"`.
3. `TestFinalizeMergeDeniedCarriesMethod`: outcome `MergeDenied`, `Method: githubcli.MethodSquash` → result `external-failed` / disposition `denied` / reason `merge-denied` (unchanged), and `Method == "squash"`.
4. `TestFinalizeMergeAlreadyMergedOmitsMethod`: the already-merged short circuit (via the `ProbeMerged` script) → `Method == ""` and the JSON document contains no `"method"` key (marshal the result and assert `!strings.Contains(doc, "\"method\"")` — `omitempty` is the guard under test).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'FinalizeMergeMethod|AttemptedMethod|DeniedCarriesMethod|AlreadyMergedOmitsMethod' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implement**

(a) In the reason block of `finalize_merge.go`, beside `ReasonMergeDenied`:

```go
	// ReasonMergeMethodUnavailable: repository settings and branch rules leave
	// no permitted merge method for this PR's base; observed cleanly, blocked
	// BEFORE any merge command. Not merge-denied (nothing was attempted) and
	// not unknown (the incompatible configuration was observed successfully).
	ReasonMergeMethodUnavailable = "merge-method-unavailable"
```

(b) In `FinalizeMergeResult` (beside `Reason`):

```go
	// Method is the merge method Docket attempted — evidence of Docket's
	// choice, never an inference about how another actor historically merged.
	// Absent when no merge command was issued (already-merged recovery,
	// validation failures, pre-effect blocks).
	Method string `json:"method,omitempty"`
```

(c) In `FinalizeMerge`'s outcome switch, add before the `default:` arm:

```go
	case githubcli.MergeMethodUnavailable:
		return mergeRefusal(ResultBlocked, MergeDispBlocked, ReasonMergeMethodUnavailable,
			fmt.Sprintf("no merge method is permitted for this PR's base: repository enables %v, branch rules permit %v; align the repository merge settings with the branch rules", mres.RepoMethods, mres.BranchMethods), id)
```

(d) Thread the method: give the app-level `verifyMerge` a `method string` parameter and set `Method: method` on its success result (the `newMergeResult(success, FinalizeMergeResult{... Merge: &VerifiedMerge{...}})` construction); pass `string(mres.Method)` at the two post-attempt call sites (`MergeMerged`, post-attempt `MergeAlreadyMerged` recovery — `mres.Method` is already empty on the pre-attempt `ProbeMerged` short circuit, so passing it through uniformly implements the spec; the pre-attempt call site passes `""` explicitly since it has no `mres`). Set `Method: string(mres.Method)` on the `MergeDenied` refusal by adding the field after `mergeRefusal` returns, e.g.:

```go
	case githubcli.MergeDenied:
		r := mergeRefusal(ResultExternalFailed, MergeDispDenied, ReasonMergeDenied,
			"the merge was authoritatively rejected; it is never retried with admin", id)
		r.Method = string(mres.Method)
		return r
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-check the mapping**

Temporarily change the new case's reason to `ReasonMergeDenied`; test 1 must FAIL on the literal token assert. Restore. Temporarily drop `omitempty` from the `Method` tag; test 4 must FAIL. Restore. Re-run `go test ./internal/app/ -count=1`: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_merge.go internal/app/finalize_merge_test.go
git commit -m "feat(0336): finalize merge maps method-unavailable to blocked and reports the attempted method"
```

---

### Task 5a: Graph-shape-independent local-ref cleanup containment (spec amendment 2026-08-21)

**Added mid-build.** Task 6's e2e coverage surfaced a spec-internal contradiction: once rebase is the
effective default, `finalizeCleanupLocalRef` blocked local-ref deletion because it proved merge-chain
containment against the *original PR head*, which is never an ancestor of the integration tip after a
rebase or squash merge (`tip-not-in-merge-chain`). The human amended the spec (see its 2026-08-21
amendment) to bring exactly this predicate in scope. Two pre-existing e2e tests
(`TestE2EOrdinaryFinalize`, `TestE2ENoPathDocketDependency`) plus every new shape test depend on this.

**Files:**
- Modify: `internal/app/finalize_cleanup.go`

**Interfaces:**
- Consumes: `githubcli.MergedFacts.MergeCommit`, `validFullObjectID` (`finalize_rebase.go`), the
  existing `FinalizeCleanupGit.IsAncestor`/`FetchBranch`, and the `ReasonCleanupRefProbe`/
  `ReasonCleanupAncestryProbe`/`ReasonCleanupUnreachable` constants. Mirrors the identical
  merge-commit-keyed containment proof already in `verifyMerge` and `closeoutIntegrationDestination`.

- [x] **Step 1: The failing tests already exist** — Task 6's `TestE2EMergeSelectsRebaseShape` et al.,
  and the two pre-existing ordinary-finalize e2e tests, are red at `finalize cleanup = "blocked" /
  tip-not-in-merge-chain` once rebase is the default. No new test needed; this predicate is what makes
  them green.
- [x] **Step 2: Key the containment proof on the merge-result commit** — in `finalizeCleanupLocalRef`,
  validate `facts.MergeCommit` with `validFullObjectID` (retain on absence, `ReasonCleanupRefProbe`)
  and prove `IsAncestor(mergeCommit, integrationTip)` instead of `IsAncestor(expectedTip, …)`. The
  live-tip identity check (`tip != expectedTip`) and the `DeleteLocalBranchChecked(expectedTip)` lease
  keep using the original head unchanged.
- [x] **Step 3: Run the e2e suite** — `go test -tags e2e ./internal/app/` green, including the two
  formerly-red ordinary-finalize tests and all four shape tests.
- [x] **Step 4: Committed with Task 6.**

---

### Task 6: End-to-end coverage — method-aware fake gh and all three merge graph shapes

**Files:**
- Modify: `internal/app/finalize_e2e_test.go`

**Interfaces:**
- Consumes: the embedded `finalizeFakeGHSource` fake gh (stateful, performs REAL git commits in the bare origin via its `git(...)`/`doMerge` helpers), the `reachImplemented`/`e2eState`/`s.dk(...)` harness, `runGit`, and the env plumbing in `newImplEnv` (the fake reads `FAKE_GH_STATE`, `FAKE_GH_ORIGIN`, `FAKE_GH_OWNER`, `FAKE_GH_NAME`, `FAKE_GH_REPO_URL`, `FAKE_GH_FAULT{,_VERB}`).
- Produces: two new fake-gh env knobs — `FAKE_GH_REPO_SETTINGS` (JSON object; absent → all three methods enabled) and `FAKE_GH_BRANCH_RULES` (JSON array; absent → `[]`) — and a method-aware `doMerge`.

- [x] **Step 1: Extend the fake gh inside `finalizeFakeGHSource`**

(a) Handle `gh api` before the `pr` dispatch in `main()`:

```go
	if len(args) >= 1 && args[0] == "api" {
		owner, name := os.Getenv("FAKE_GH_OWNER"), os.Getenv("FAKE_GH_NAME")
		path := args[len(args)-1] // last arg is the endpoint path; --hostname rides earlier
		if path == "repos/"+owner+"/"+name {
			body := os.Getenv("FAKE_GH_REPO_SETTINGS")
			if body == "" {
				body = "{\"allow_rebase_merge\":true,\"allow_merge_commit\":true,\"allow_squash_merge\":true}"
			}
			fmt.Fprintln(os.Stdout, body)
			os.Exit(0)
		}
		if strings.HasPrefix(path, "repos/"+owner+"/"+name+"/rules/branches/") {
			body := os.Getenv("FAKE_GH_BRANCH_RULES")
			if body == "" {
				body = "[]"
			}
			fmt.Fprintln(os.Stdout, body)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "fake gh: unmatched api path %q\n", path)
		os.Exit(64)
	}
```

(b) Make `pr merge` method-aware and strict about the flag (exactly one of the three must be present — a merge issued with no method flag is a protocol error, exit 64):

```go
	case "merge":
		if faultMode("merge") == "denied" { ... unchanged ... }
		method := ""
		for _, f := range []string{"--rebase", "--merge", "--squash"} {
			if hasFlag(args, f) {
				if method != "" { os.Exit(64) }
				method = f
			}
		}
		if method == "" {
			fmt.Fprintln(os.Stderr, "fake gh: pr merge without a method flag")
			os.Exit(64)
		}
		doMerge(argNumber(args), flagVal(args, "--match-head-commit"), method)
		... loss fault + exit 0 unchanged ...
```

(c) Extend `doMerge(number int, matchHead, method string)` to produce the real graph shape per method, after the existing head/tree/baseTip resolution:

```go
	var mc string
	switch method {
	case "--merge":
		mc, err = git("commit-tree", tree, "-p", baseTip, "-p", head, "-m", "Merge pull request #"+strconv.Itoa(number))
	case "--squash":
		mc, err = git("commit-tree", tree, "-p", baseTip, "-m", "squash merge PR #"+strconv.Itoa(number))
	case "--rebase":
		// One rebased commit per feature commit not on base; single-parent chain.
		revs, rerr := git("rev-list", "--reverse", baseTip+".."+head)
		if rerr != nil { os.Exit(1) }
		tip := baseTip
		for _, rev := range strings.Fields(revs) {
			rtree, terr := git("rev-parse", rev+"^{tree}")
			if terr != nil { os.Exit(1) }
			tip, terr = git("commit-tree", rtree, "-p", tip, "-m", "rebased "+rev)
			if terr != nil { os.Exit(1) }
		}
		mc = tip
		if mc == baseTip { os.Exit(1) } // nothing to rebase: refuse rather than fake
	default:
		os.Exit(64)
	}
	if err != nil { os.Exit(1) }
```

`humanMergePR` (out-of-band human merge) already passes `--merge` explicitly and keeps working. Keep the existing `mergedAt`/`MergeCommit`/state bookkeeping on `mc`.

- [x] **Step 2: Update existing e2e expectations that assumed a two-parent merge**

With all methods enabled by default, finalize now selects **rebase**. Grep the e2e file for `rev-list", "--merges` and any assert equating the destination tip's parent count or the merge commit with the PR head; rewrite each to the graph-shape-independent proof: the document's `merge.merge_commit` is reachable from the freshly-read origin base tip (`runGit(t, origin, "merge-base", "--is-ancestor", mergeCommit, baseTip)` exits 0), the PR reports `MERGED`, and head/base facts match. Where a test exists specifically to count merge commits from the out-of-band human `--merge` path, leave it — that path still produces a merge commit.

- [x] **Step 3: Write the new failing e2e tests**

Add four tests following the existing pattern (`sharedBinaries` → `reachImplemented` → drive `finalize rebase`/`merge` via `s.dk`; set the env knobs through the state the harness passes to both the in-process seams and the argv subprocess — extend `s.env`/`env.env` the same way `withFault` injects `FAKE_GH_FAULT`):

1. `TestE2EMergeSelectsRebaseShape` (may fold into an existing merge-path test): default knobs → document `result: applied`, `"method":"rebase"` in the raw JSON, merge commit reachable from origin base, and `runGit(t, origin, "rev-list", "--merges", "--count", base)` reports `0` new merge commits (single-parent chain) — asserting the shape POSITIVELY without requiring it for verification.
2. `TestE2EMergeCommitShape`: `FAKE_GH_REPO_SETTINGS='{"allow_rebase_merge":false,"allow_merge_commit":true,"allow_squash_merge":true}'` → `"method":"merge"`, merge-commit count 1, reachable.
3. `TestE2ESquashOnlyShape`: settings squash-only → `"method":"squash"`, tip is single-parent, not equal to the original PR head, reachable.
4. `TestE2EMergeMethodUnavailable`: settings all-false → document `result: blocked`, `reason: merge-method-unavailable`, no `"method"` key, the PR still `OPEN` in the fake state file, and the origin base tip unmoved (capture it before, compare after — zero merge commands is proven by state, not absence of logging).

- [x] **Step 4: Run the e2e tests**

Run: `go test ./internal/app/ -run 'E2E' -count=1 -v` (respect the suite's actual e2e test-name pattern — reuse the file's existing naming so `scripts/run-tests.sh` picks them up unchanged).
Expected: PASS, including all pre-existing finalize e2e tests.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_e2e_test.go
git commit -m "test(0336): e2e merge-method selection across rebase, merge-commit, and squash graph shapes"
```

---

### Task 7: Protocol prose — the finalize skill documents method selection

**Files:**
- Modify: `internal/assets/embedded/tree/skills/docket-finalize-change/SKILL.md`

**Interfaces:**
- Consumes: Task 5's tokens (`merge-method-unavailable`, the `method` field). The skill body is the protocol document `FinalizeMergeResult` is exposed through.

- [ ] **Step 1: Edit the merge step's prose**

In the section describing `docket finalize merge --id <id> --version <version> --head <head>` (the paragraph beginning "It reloads fresh authority and rechecks every merge conjunct"), add two sentences after the sentence ending "issuing **no** merge call when any fails":

> Before the effect it selects the best merge method the repository settings and the base branch's active rules permit, in the fixed order rebase → merge commit → squash, and attempts exactly that one; the document's `method` field reports the attempted method (absent on already-merged recovery). A cleanly observed empty permitted set refuses `blocked` with reason `merge-method-unavailable` before any merge — fix the repository or branch-rule merge settings; it is not `merge-denied` and is never retried with another method.

Do not touch the `--admin` paragraph or any other step.

- [ ] **Step 2: Run the whole suite and obey any drift guard**

Run: `scripts/run-tests.sh`
The embedded asset tree may be mirrored by drift/bundle guards (e.g. `tests/test_asset_bundle_drift.sh`). If one reddens, follow **the guard's own printed remedy** (regenerate/re-derive the mirrored artifact via the command it names) — never hand-patch a frozen fixture and never weaken the guard. Also watch for a trailing `OVER BUDGET:` line and treat it as a finding.
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/assets/embedded/tree/skills/docket-finalize-change/SKILL.md
git commit -m "docs(0336): finalize skill documents merge-method selection and merge-method-unavailable"
```

(Add any guard-regenerated artifacts the remedy produced to the same commit.)

---

### Task 8: Whole-suite gate and the named human verification items

**Files:**
- None created; this task is verification and the record of what only a human can verify.

- [ ] **Step 1: Uncached Go verification**

Run: `go test -count=1 ./...`
Expected: PASS. `-count=1` defeats the Go test cache so every mutation-probed package re-executes against the final tree (a cached PASS against a mutated tree is meaningless).

- [ ] **Step 2: Full suite via the configured command**

Read `finalize.test_command` from `.docket.yml` (currently `scripts/run-tests.sh`) and run exactly that from the worktree root.
Expected: PASS, no new `OVER BUDGET:` rows attributable to this change's test files (if one appears, split or speed the offending file before calling the task done).

- [ ] **Step 3: Record the human verification items**

GitHub's live repository fields, active-rule payloads, and merge behavior are outside-repo truth — no in-repo test can be their oracle. The eventual results file for change 0336 must carry these **named human verification items** (copy them verbatim; do not silently drop them at results time):

1. Observe the real repository and effective base-branch capability payloads on a live GitHub repository (`gh api repos/<o>/<n>` and `gh api repos/<o>/<n>/rules/branches/<branch>`) and confirm the decoded fields and token spellings match what `probeRepoMergeMethods` / `probeBranchMergeRules` expect.
2. Finalize change 0336 itself through this repository's rebase-first path (rebase and squash enabled, merge commits disabled) and confirm the selected method is `rebase` and the document reports it.
3. Finalize a scratch squash-only repository to certify the last-priority fallback and its reachability proof.

- [ ] **Step 4: Final self-review against the spec's out-of-scope list**

Confirm by grep that the branch adds: no `.docket.yml` key or resolver export for merge method (`grep -rn "merge_method\|merge-method" internal/repository internal/app/config.go .docket.yml docs/README* 2>/dev/null` — hits only in the new reason token, prose, and this plan), no retry-after-denial path (exactly one `pr merge` composition site in `internal/githubcli/merge.go`), no historical-method inference on already-merged recovery, and no change to `--admin` semantics, approval requirements, branch cleanup, or changes 0327/0330/0331 surfaces.

---

## Self-Review (performed while writing)

- **Spec coverage:** CLI default → Task 1; vocabulary/probes/intersection/selection → Tasks 2–3; single-attempt flow, closed outcomes, exact arguments, no-delete-branch → Task 4; `blocked / merge-method-unavailable` mapping, method-as-attempt-metadata, protocol field → Task 5; three graph shapes, reachability without two-parent assumptions, zero-merge-on-empty-policy e2e → Task 6; protocol document → Task 7; whole suite, uncached verification, three named human items → Task 8. Repository/branch composition-by-intersection and required-linear-history are tested in Task 3 (`TestProbeBranchMergeRules`) and selected in Task 4.
- **Type consistency:** `MergeMethod`/`methodSet`/`selectMergeMethod` (Task 2) are consumed by Tasks 3–4 under the same names; `MergeResult{Outcome, Method, Facts, RepoMethods, BranchMethods}` defined in Task 4 is what Task 5's fakes return; `ReasonMergeMethodUnavailable` literal `"merge-method-unavailable"` matches Tasks 5–7.
- **Known judgment calls for the builder:** (1) exact fake-scenario helper names in Task 3 follow the package's existing harness, not this plan's placeholder `newFakeGHClient`; (2) Task 4 Step 5 carries the sanctioned contingency for the e2e fake ordering; (3) live GitHub token casing is deliberately delegated to the named human verification item, with unknown tokens failing closed in the meantime.
