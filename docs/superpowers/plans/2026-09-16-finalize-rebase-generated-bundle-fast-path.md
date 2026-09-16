<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0413 — Finalize rebase gate cannot clear derived embedded-asset manifest collisions](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-16-0413-finalize-rebase-gate-cannot-clear-derived-embedded-asset-man.md)**
<!-- docket:backlink:end -->
# Finalize Rebase Generated-Bundle Fast Path — Implementation Plan (change 0413)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The finalize rebase controller clears conflict stops whose only unmerged paths are Docket's own generated embedded-asset bundle (`internal/assets/embedded/{manifest.json,tree/}`) by regenerating the bundle in-process — consuming zero resolver reservations — while mixed authored+generated stops still charge exactly one reserved resolver dispatch for the authored decisions and the controller regenerates the bundle after the resolver resolves them.

**Architecture:** A new `internal/app/finalize_generated.go` holds the eligibility predicates (module identity + generator contract), the in-process regeneration (reusing `internal/assets.Generate`/`WriteTree` exactly as `cmd/genassets` does), and a bounded generated-only advance loop (`advanceGeneratedOnly`) that runs under the workspace operation lock at every conflicted-stop site in `internal/app/finalize_rebase.go` (`mapBegunRebase`, `recoverFromReceipt`, `mapContinuedRebase`). The budgeted continue path (`finalizeRebaseContinueBudgeted`) gains a mixed-conflict branch: when the resolver's report covers the authored paths and every leftover live unmerged path is an eligible bundle output, the controller regenerates the bundle (before the continuation-started marker is written) and stages the reported paths plus the bundle directory in one continue.

**Tech Stack:** Go; `internal/assets` (Generate, WriteTree, DiffTree, DefaultAllowedRoots); `internal/gitcli` rebase primitives; real-Git integration tests behind the existing `//go:build integration` tag.

**Spec:** `docs/superpowers/specs/2026-09-16-finalize-rebase-gate-cannot-clear-derived-embedded-asset-man-design.md` (on the `docket` metadata branch; also readable at `.docket/docs/superpowers/specs/…` from the primary tree). The change file is `docs/changes/active/0413-finalize-rebase-gate-cannot-clear-derived-embedded-asset-man.md` on the same branch.

## Global Constraints

- Eligibility requires the feature workspace's module identity to be exactly `github.com/danielhanold/docket`; a similarly named file in another repository is never eligible. No generic generated-file detection.
- Recognize only `internal/assets/embedded/manifest.json` and paths under `internal/assets/embedded/tree/`.
- Generated-only stops allocate no resolver reservation and consume no resolver budget, including after the last permitted authored resolution. Real reservations remain charged exactly once and are never refunded.
- Reuse the existing generator unchanged: no new command, no new configuration key, no generator registry or hooks framework, no separate state store, no squashing, no blanket budget increase.
- Preserve ownership checks, the workspace operation lock, interruption/re-entry handling, and the post-rebase whole-suite gate. Never fabricate resolver reports or reservation tokens for deterministic work. Never generate from unresolved authored inputs. Do not loop on an unchanged stopped commit.
- The suite command is `go run ./cmd/docket development test` run from the feature-workspace source tree.
- Comment cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054; `TestCommentAnchorStyle`).
- Skills/agents are authored roots frozen into the embedded bundle: after editing `skills/` or `agents/`, regenerate with `go generate ./internal/assets/` in the same commit.

## File Structure

- Create: `internal/app/finalize_generated.go` — bundle constants, `pathsGeneratedOnly`, `bundleRepoEligible`, `regenerateEmbeddedBundle`, `regenBundle` seam accessor, `advanceGeneratedOnly`.
- Create: `internal/app/finalize_generated_test.go` — pure unit tests for the predicates and the production regeneration.
- Create: `internal/app/finalize_generated_integration_test.go` (`//go:build integration`) — the real-Git fixture (`beginBundleConflicts`) and all end-to-end tests.
- Modify: `internal/app/finalize_context.go` — `FinalizeDeps` gains the `RegenerateBundle` seam.
- Modify: `internal/app/finalize_rebase.go` — hook `advanceGeneratedOnly` into the three conflicted-stop sites; add the mixed-conflict branch to `finalizeRebaseContinueBudgeted`.
- Modify: `internal/gitcli/rebase.go` — `StageAndContinueRebase` doc contract admits a directory pathspec; test in `internal/gitcli/rebase_test.go` (or the file that already tests `StageAndContinueRebase`).
- Modify: `skills/docket-finalize-change/SKILL.md`, `skills/docket-finalize-change/references/gate-failure.md`, `agents/docket-rebase-resolver.md` — the mixed-conflict handoff; then `go generate ./internal/assets/`.

---

### Task 1: Bundle predicates, production regeneration, and the `RegenerateBundle` seam

**Files:**
- Create: `internal/app/finalize_generated.go`
- Create: `internal/app/finalize_generated_test.go`
- Modify: `internal/app/finalize_context.go` (add one field to `FinalizeDeps`, after `ContinueGit`)

**Interfaces:**
- Produces: `const embeddedBundleDir = "internal/assets/embedded"`, `const docketModulePath = "github.com/danielhanold/docket"`, `func pathsGeneratedOnly(paths []string) bool`, `func bundleRepoEligible(wsDir string) bool`, `func regenerateEmbeddedBundle(wsDir string) error`, `func regenBundle(deps FinalizeDeps) func(string) error`, and the `FinalizeDeps.RegenerateBundle func(wsDir string) error` field. Tasks 3 and 4 consume all of these.

- [ ] **Step 1: Write the failing unit tests**

In `internal/app/finalize_generated_test.go`:

```go
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
)

func TestPathsGeneratedOnly(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  bool
	}{
		{"manifest only", []string{"internal/assets/embedded/manifest.json"}, true},
		{"tree paths", []string{"internal/assets/embedded/tree/skills/x/SKILL.md", "internal/assets/embedded/manifest.json"}, true},
		{"mixed authored", []string{"internal/assets/embedded/manifest.json", "skills/x/SKILL.md"}, false},
		{"authored only", []string{"feature.txt"}, false},
		{"empty set", nil, false},
		{"bundle dir itself is not an output path", []string{"internal/assets/embedded"}, false},
		{"lookalike sibling", []string{"internal/assets/embedded-other/manifest.json"}, false},
	}
	for _, c := range cases {
		if got := pathsGeneratedOnly(c.paths); got != c.want {
			t.Errorf("%s: pathsGeneratedOnly(%v) = %v, want %v", c.name, c.paths, got, c.want)
		}
	}
}

// writeBundleFixtureRepo lays down a minimal eligible workspace: go.mod with the
// Docket module identity, all four DefaultAllowedRoots, and a committed-shape
// embedded bundle generated by the real generator. Returns the dir.
func writeBundleFixtureRepo(t *testing.T, module string) string {
	t.Helper()
	dir := t.TempDir()
	writeFileT(t, dir, "go.mod", "module "+module+"\n\ngo 1.22\n")
	writeFileT(t, dir, "skills/demo/SKILL.md", "demo skill v1\n")
	writeFileT(t, dir, "agents/demo.md", "demo agent\n")
	writeFileT(t, dir, "cursor-rules/demo.mdc", "demo rule\n")
	writeFileT(t, dir, ".docket.example.yml", "version: 1\n")
	m, payload, err := assets.Generate(dir, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("generate fixture bundle: %v", err)
	}
	if err := assets.WriteTree(filepath.Join(dir, "internal", "assets", "embedded"), m, payload); err != nil {
		t.Fatalf("write fixture bundle: %v", err)
	}
	return dir
}

func writeFileT(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBundleRepoEligible(t *testing.T) {
	eligible := writeBundleFixtureRepo(t, docketModulePath)
	if !bundleRepoEligible(eligible) {
		t.Fatalf("a Docket-module workspace with all roots and a committed bundle must be eligible")
	}
	foreign := writeBundleFixtureRepo(t, "github.com/someone/else")
	if bundleRepoEligible(foreign) {
		t.Fatalf("a foreign module identity must never be eligible")
	}
	noBundle := t.TempDir()
	writeFileT(t, noBundle, "go.mod", "module "+docketModulePath+"\n")
	if bundleRepoEligible(noBundle) {
		t.Fatalf("a workspace without the committed bundle and roots must not be eligible")
	}
	noGoMod := writeBundleFixtureRepo(t, docketModulePath)
	if err := os.Remove(filepath.Join(noGoMod, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if bundleRepoEligible(noGoMod) {
		t.Fatalf("an unreadable module identity must not be eligible (probe error is not clean absence)")
	}
}

func TestRegenerateEmbeddedBundle(t *testing.T) {
	dir := writeBundleFixtureRepo(t, docketModulePath)
	// Drift the authored root: add one file, delete another's bundle copy, and
	// corrupt the manifest the way a conflicted stop leaves it.
	writeFileT(t, dir, "skills/demo/SKILL.md", "demo skill v2\n")
	writeFileT(t, dir, "skills/extra/SKILL.md", "brand new skill\n")
	writeFileT(t, dir, "internal/assets/embedded/manifest.json", "<<<<<<< conflict markers\n")
	writeFileT(t, dir, "internal/assets/embedded/tree/skills/stale/SKILL.md", "stale payload the regen must delete\n")

	if err := regenerateEmbeddedBundle(dir); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	m, payload, err := assets.Generate(dir, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("re-generate for diff: %v", err)
	}
	diffs, err := assets.DiffTree(filepath.Join(dir, "internal", "assets", "embedded"), m, payload)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("the regenerated bundle drifts from the authored roots: %v", diffs)
	}
	if _, err := os.Stat(filepath.Join(dir, "internal/assets/embedded/tree/skills/stale/SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("regeneration did not delete the stale payload file (err=%v)", err)
	}
}

func TestRegenerateEmbeddedBundleFailsOnBrokenRoots(t *testing.T) {
	dir := writeBundleFixtureRepo(t, docketModulePath)
	if err := os.RemoveAll(filepath.Join(dir, "skills")); err != nil {
		t.Fatal(err)
	}
	if err := regenerateEmbeddedBundle(dir); err == nil {
		t.Fatalf("regeneration over a missing allowed root must fail, not write a partial bundle")
	}
	// The failure must leave the previous committed bundle in place (staging+rename,
	// never an in-place truncate).
	if _, err := os.Stat(filepath.Join(dir, "internal/assets/embedded/manifest.json")); err != nil {
		t.Fatalf("a failed regeneration destroyed the previous bundle: %v", err)
	}
}
```

Note: if a helper named `writeFileT` (or an equivalent) already exists in package `app`'s tests, reuse it instead of redefining.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestPathsGeneratedOnly|TestBundleRepoEligible|TestRegenerateEmbeddedBundle' -count=1`
Expected: FAIL — `pathsGeneratedOnly`, `bundleRepoEligible`, `regenerateEmbeddedBundle`, `docketModulePath` undefined.

- [ ] **Step 3: Implement `internal/app/finalize_generated.go`**

```go
package app

// This file is the generated-bundle fast path (change 0413) for the finalize
// rebase controller: recognizing conflict stops whose only unmerged paths are
// Docket's own embedded asset bundle, regenerating that bundle in-process with
// the existing generator (internal/assets — the same entrypoints cmd/genassets
// drives), and continuing the owned rebase without spending a resolver
// reservation. Eligibility is deliberately narrow: the exact Docket module
// identity and the exact bundle location — never generic generated-file
// detection. Regeneration is conflict resolution only; it is never permission
// to merge or to bypass the post-rebase gate.

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/gitcli"
)

// embeddedBundleDir is the repo-relative home of the generated bundle — the
// same location cmd/genassets freezes (its embeddedRel constant).
const embeddedBundleDir = "internal/assets/embedded"

// docketModulePath is the module identity that gates eligibility: only
// Docket's own repository carries this bundle contract.
const docketModulePath = "github.com/danielhanold/docket"

// pathsGeneratedOnly reports whether every path is a bundle OUTPUT: the
// manifest or a tree/ payload strictly under embeddedBundleDir. An empty set
// is not generated-only (there is nothing to clear), and the directory name
// itself is not an output path.
func pathsGeneratedOnly(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	prefix := embeddedBundleDir + "/"
	for _, p := range paths {
		if !strings.HasPrefix(p, prefix) {
			return false
		}
	}
	return true
}

// bundleRepoEligible reports whether the feature workspace is Docket itself
// with the generator contract present: go.mod declares docketModulePath, every
// DefaultAllowedRoots root exists, and the committed bundle location exists. A
// probe error is never a clean "eligible" — any failure answers false and the
// stop falls through to the normal resolver path.
func bundleRepoEligible(wsDir string) bool {
	if goModModule(filepath.Join(wsDir, "go.mod")) != docketModulePath {
		return false
	}
	for _, root := range assets.DefaultAllowedRoots() {
		if _, err := os.Lstat(filepath.Join(wsDir, filepath.FromSlash(root.Root))); err != nil {
			return false
		}
	}
	if _, err := os.Lstat(filepath.Join(wsDir, filepath.FromSlash(embeddedBundleDir))); err != nil {
		return false
	}
	return true
}

// goModModule extracts the module path from a go.mod file, or "" when the file
// is unreadable or declares none.
func goModModule(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// regenerateEmbeddedBundle regenerates the bundle from the workspace's
// authored roots, mirroring cmd/genassets' publish mechanics: generate in
// memory, write into a staging directory beside the destination, then swap via
// remove+rename so a generation failure never leaves a partial bundle.
func regenerateEmbeddedBundle(wsDir string) error {
	m, payload, err := assets.Generate(wsDir, assets.DefaultAllowedRoots())
	if err != nil {
		return err
	}
	outDir := filepath.Join(wsDir, filepath.FromSlash(embeddedBundleDir))
	staging, err := os.MkdirTemp(filepath.Dir(outDir), ".embedded-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	staged := filepath.Join(staging, "embedded")
	if err := assets.WriteTree(staged, m, payload); err != nil {
		return err
	}
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	return os.Rename(staged, outDir)
}

// errBundleNotAdvancing marks a fast-path continue that left the rebase
// stopped on the same commit — regeneration cannot clear this stop, so it
// blocks through the existing failure path rather than looping.
var errBundleNotAdvancing = errors.New("the generated-bundle continue did not advance past the stopped commit")

// regenBundle returns the injected regeneration seam, or the in-process
// production generator when none is wired (production leaves RegenerateBundle
// unset).
func regenBundle(deps FinalizeDeps) func(string) error {
	if deps.RegenerateBundle != nil {
		return deps.RegenerateBundle
	}
	return regenerateEmbeddedBundle
}

// advanceGeneratedOnly is added in a later task (see finalize_rebase.go
// wiring); it lives in this file.
var _ = context.Background // placeholder imports used from Task 3 on
var _ gitcli.RebaseStatus
```

(The two trailing placeholder lines exist only so `context` and `gitcli` imports compile before Task 3 adds `advanceGeneratedOnly`; Task 3 removes them. If you prefer, omit those imports now and add them in Task 3.)

- [ ] **Step 4: Add the seam field in `internal/app/finalize_context.go`**

Immediately after the `ContinueGit FinalizeContinueGit` field of `FinalizeDeps` (search for `ContinueGit` in that file), add:

```go
	// RegenerateBundle is the narrow regeneration seam the generated-bundle
	// fast path (change 0413) rebuilds internal/assets/embedded through inside
	// an eligible feature workspace. It is nil in production wiring; the fast
	// path falls back to the in-process generator (regenerateEmbeddedBundle)
	// via regenBundle. A unit test injects a fake that counts regenerations or
	// fails deterministically.
	RegenerateBundle func(wsDir string) error
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestPathsGeneratedOnly|TestBundleRepoEligible|TestRegenerateEmbeddedBundle' -count=1`
Expected: PASS. Also run `go vet ./internal/app/`.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_generated.go internal/app/finalize_generated_test.go internal/app/finalize_context.go
git commit -m "feat(0413): bundle eligibility predicates, in-process regeneration, RegenerateBundle seam"
```

---

### Task 2: `StageAndContinueRebase` admits a directory pathspec (stages deletions)

The fast path stages the whole regenerated bundle — including deletions — by passing the directory `internal/assets/embedded` as one pathspec. `git add -- <dir>` stages modifications AND removals inside the directory, and `validateRepoPath` already accepts a directory-shaped path; this task pins that behavior with a test and widens the method's doc contract so the property is load-bearing, not incidental.

**Files:**
- Modify: `internal/gitcli/rebase.go` (doc comment on `StageAndContinueRebase` only — no code change expected)
- Test: add to the file that already tests `StageAndContinueRebase` in `internal/gitcli/` (find it with `grep -rn "StageAndContinueRebase" internal/gitcli/*_test.go`; follow that file's fixture helpers for creating a repo and a conflicted rebase)

**Interfaces:**
- Consumes: `func (c *Client) StageAndContinueRebase(ctx context.Context, worktreeDir string, paths []string) (RebaseStatus, error)` (existing).
- Produces: the guarantee Task 3 relies on — a directory pathspec stages every change under it, including deletions, before continuing.

- [ ] **Step 1: Write the failing (or pinning) test**

Using the existing gitcli test fixture idioms (real git; mirror the neighboring `StageAndContinueRebase` tests' setup), build this scenario:

1. Base branch commit b1 adds `bundle/manifest.json` = "base manifest" and `bundle/tree/old.txt` = "old payload".
2. Feature branch from b0 (b1's parent) commits `bundle/manifest.json` = "feature manifest" and `bundle/tree/old.txt` = "feature payload".
3. `BeginRebase` feature onto b1 → conflicted; unmerged paths include `bundle/manifest.json` (and possibly `bundle/tree/old.txt`).
4. Simulate regeneration in the worktree: write `bundle/manifest.json` = "regenerated", delete `bundle/tree/old.txt`, add `bundle/tree/new.txt` = "new payload".
5. Call `StageAndContinueRebase(ctx, wt, []string{"bundle"})`.

Assert: the call returns `RebaseRebased` (no error), and at the resulting head `git ls-files` shows `bundle/tree/new.txt` present and `bundle/tree/old.txt` absent, with `bundle/manifest.json` = "regenerated".

- [ ] **Step 2: Run the test**

Run: `go test ./internal/gitcli/ -run TestStageAndContinueRebaseDirectoryPathspec -count=1` (name the test that; add `//go:build integration` / `requireRealGit`-style gating only if the neighboring gitcli rebase tests use it — match the existing file exactly).
Expected: PASS if git behaves as documented above (this is a pinning test); if it FAILS on the deletion, change the implementation to use `git add -A -- <path>...` for the add invocation (`-A` scoped to explicit pathspecs stages removals; it is not a bare `git add -A`) and update the comment accordingly.

- [ ] **Step 3: Widen the doc contract**

In `internal/gitcli/rebase.go`, replace the `StageAndContinueRebase` doc comment's sentence "stages EXACTLY the given repo-relative paths (the caller has already validated them against the live unmerged set)" so it reads:

```go
// StageAndContinueRebase stages EXACTLY the given repo-relative pathspecs and
// continues the in-progress rebase non-interactively. A pathspec may be a
// single file the caller validated against the live unmerged set, or a
// directory the caller owns wholesale (the finalize generated-bundle fast
// path stages internal/assets/embedded this way): a directory pathspec stages
// every modification, addition, AND deletion beneath it. The result is
// classified structurally: the next conflict (conflicted with its
// UnmergedPaths), a completed rewrite (rebased), or failed. It never runs a
// repo-wide `git add -A` and never resolves a path the caller did not name.
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/gitcli/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gitcli/rebase.go internal/gitcli/<test file>
git commit -m "test(0413): pin directory-pathspec staging (incl. deletions) in StageAndContinueRebase"
```

---

### Task 3: `advanceGeneratedOnly` loop, wired into the three conflicted-stop sites

**Files:**
- Modify: `internal/app/finalize_generated.go` (add `advanceGeneratedOnly`; remove Task 1's placeholder lines)
- Modify: `internal/app/finalize_rebase.go` (`mapBegunRebase`, `recoverFromReceipt`, `mapContinuedRebase`)
- Create: `internal/app/finalize_generated_integration_test.go` (`//go:build integration`)

**Interfaces:**
- Consumes: Task 1's predicates and `regenBundle`; `continueGit(deps) FinalizeContinueGit` (existing: `RebaseState`, `StoppedRebaseCommit`, `StageAndContinueRebase`); `deps.Workspace.AcquireOperationLock(metaDir)`, `ReadRebaseReceipt`; `rebaseRefusal`, `withResolverCounts`, `ReasonRebaseGitFailed`, `ReasonRebaseWorkspaceProbe` (all existing in package `app`).
- Produces: `func advanceGeneratedOnly(ctx context.Context, deps FinalizeDeps, op string, rc *rebaseContext, status gitcli.RebaseStatus) (gitcli.RebaseStatus, *FinalizeRebaseResult)` — given a conflicted status, clears successive generated-only stops and returns the first status it cannot clear (authored conflict, completed rewrite) or a refusal. Task 4's caller site and Task 5's tests rely on this exact signature.

- [ ] **Step 1: Write the failing integration test (regression: more generated-only stops than the budget, zero reservations)**

Create `internal/app/finalize_generated_integration_test.go`. Model the fixture on `beginSuccessiveConflicts` / `setupRebaseFixture` (in `finalize_rebase_integration_test.go` and `finalize_rebase_test.go` — read both before writing). The new fixture helper:

```go
//go:build integration

package app

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
)

// makeBundleWorkspace turns a rebase fixture's feature workspace into an
// ELIGIBLE Docket-shaped repo: go.mod with the Docket module identity, all
// four DefaultAllowedRoots, and a committed generated bundle. module lets the
// ineligible-repo test plant a foreign identity.
func makeBundleWorkspace(t *testing.T, f *rebaseFixture, module string) {
	t.Helper()
	writeRepoFile(t, f.wp, "go.mod", "module "+module+"\n\ngo 1.22\n")
	writeRepoFile(t, f.wp, "skills/demo/SKILL.md", "demo skill v1\n")
	writeRepoFile(t, f.wp, "agents/demo.md", "demo agent\n")
	writeRepoFile(t, f.wp, "cursor-rules/demo.mdc", "demo rule\n")
	writeRepoFile(t, f.wp, ".docket.example.yml", "version: 1\n")
	regenerateBundleInto(t, f.wp)
	runGit(t, f.wp, "add", "-A")
	runGit(t, f.wp, "commit", "-q", "-m", "docket-shaped workspace with committed bundle")
}

// regenerateBundleInto runs the REAL generator over the workspace and writes
// the bundle in place, exactly as go generate ./internal/assets/ would.
func regenerateBundleInto(t *testing.T, wsDir string) {
	t.Helper()
	if err := regenerateEmbeddedBundle(wsDir); err != nil {
		t.Fatalf("fixture bundle regeneration: %v", err)
	}
}

// bundleFeatureCommit edits one authored skill file and regenerates the
// bundle, committing both — the shape that produces a manifest collision on
// every replayed commit once the base regenerates too.
func bundleFeatureCommit(t *testing.T, f *rebaseFixture, step int) {
	t.Helper()
	writeRepoFile(t, f.wp, "skills/demo/SKILL.md", fmt.Sprintf("demo skill feature v%d\n", step))
	regenerateBundleInto(t, f.wp)
	runGit(t, f.wp, "add", "-A")
	runGit(t, f.wp, "commit", "-q", "-m", fmt.Sprintf("feature bundle step %d", step))
}

// beginBundleConflicts publishes featureCommits bundle-regenerating feature
// commits, advances the base with an INDEPENDENT authored edit plus its own
// regenerated bundle (via a temp clone of the base branch, mirroring
// writerAdvance's mechanics but regenerating with the real generator), sets
// the resolver limit, and begins the rebase — which must stop conflicted on
// bundle outputs only.
func beginBundleConflicts(t *testing.T, limit, featureCommits int, module string) (*rebaseFixture, FinalizeDeps, string, FinalizeRebaseResult) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	makeBundleWorkspace(t, f, module)
	for i := 1; i <= featureCommits; i++ {
		bundleFeatureCommit(t, f, i)
	}
	head := runGit(t, f.wp, "rev-parse", "HEAD")
	runGit(t, f.wp, "push", "-f", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
	advanceBaseWithBundle(t, f) // see step note below
	if limit > 0 {
		setResolverConfig(t, f, limit)
	}
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	begin := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	return f, deps, head, begin
}
```

`advanceBaseWithBundle` must land ONE base commit that (a) makes an authored edit to a DIFFERENT authored file than the feature commits touch — e.g. write `agents/demo.md` = "demo agent, base edition\n" — and (b) regenerates the bundle from the base's authored roots. Implement it the way `f.repo.writerAdvance` lands base commits (read `writerAdvance` in the fixture code and mirror it: clone or worktree the base branch to a temp dir, apply the edit, run `regenerateBundleInto` on that temp checkout, commit, push). The independent authored edit is what the "preserved authored edits" assertion checks after the rebase.

Important eligibility detail: `makeBundleWorkspace`'s commit happens BEFORE the base branches diverge only in the feature workspace — the base checkout must also contain the docket-shaped files for its regeneration to succeed. Simplest ordering: make the docket-shaped commit FIRST, push it as the shared ancestor into the base branch too (or land it on the base via `writerAdvance` with the same file contents before the feature commits), so both sides regenerate over the same roots. Follow whatever `setupRebaseFixture` makes easiest; the invariant that matters is: base and feature share an ancestor containing go.mod + roots + bundle, the feature adds `featureCommits` bundle-regenerating commits, the base adds one independent authored+bundle commit.

Then the regression test:

```go
// TestIntegrationGeneratedOnlyStopsBypassResolverBudget is change 0413's core
// regression: more generated-only stops than the configured resolver limit
// complete WITHOUT any reservation, authored edits from both sides survive,
// and the committed bundle matches the authored roots exactly. Reverting the
// fast path must make this test fail (the second stop would exhaust limit 2 —
// see the revert guard task).
func TestIntegrationGeneratedOnlyStopsBypassResolverBudget(t *testing.T) {
	requireRealGit(t)
	// limit 2, four feature commits -> four successive generated-only stops.
	f, deps, _, begin := beginBundleConflicts(t, 2, 4, docketModulePath)
	if begin.Result != ResultApplied || begin.Disposition != RebaseDispRebased {
		t.Fatalf("begin = (%q, %q) reason %q msg %q paths %v, want applied/rebased straight through",
			begin.Result, begin.Disposition, begin.Reason, begin.Message, begin.UnmergedPaths)
	}
	if begin.Gate == nil || begin.Gate.Evidence == "" {
		t.Errorf("the completed rebase did not compose the gate: %+v", begin.Gate)
	}
	// Zero reservations: the budget group is untouched.
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "0" || rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("fast path spent resolver state: used %q token %q cont %q, want 0 and empty", rec.ResolverUsed, rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
	// Both sides' authored edits survive at the rebased head.
	if got := readRepoFile(t, f.wp, "skills/demo/SKILL.md"); got != "demo skill feature v4\n" {
		t.Errorf("feature authored edit lost: %q", got)
	}
	if got := readRepoFile(t, f.wp, "agents/demo.md"); got != "demo agent, base edition\n" {
		t.Errorf("base authored edit lost: %q", got)
	}
	// Clean bundle drift check at the final head.
	m, payload, err := assets.Generate(f.wp, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("post-rebase generate: %v", err)
	}
	diffs, err := assets.DiffTree(filepath.Join(f.wp, "internal", "assets", "embedded"), m, payload)
	if err != nil || len(diffs) != 0 {
		t.Fatalf("post-rebase bundle drift: %v %v", diffs, err)
	}
	if st, _ := f.deps.Client.RebaseState(context.Background(), f.wp); st.Disposition != gitcli.RebaseUnchanged {
		t.Errorf("a rebase is still in progress: %q", st.Disposition)
	}
	_ = deps
}
```

(If `readRepoFile` does not exist in the fixtures, add it beside `writeRepoFile`.)

Also in this file, the ineligible-repo test:

```go
// TestIntegrationGeneratedOnlyIneligibleRepoStaysOnNormalPath proves a repo
// that is NOT Docket (foreign module identity) never enters the fast path: the
// same bundle-shaped collision surfaces as an ordinary conflicted stop.
func TestIntegrationGeneratedOnlyIneligibleRepoStaysOnNormalPath(t *testing.T) {
	requireRealGit(t)
	_, _, _, begin := beginBundleConflicts(t, 2, 1, "github.com/someone/else")
	if begin.Disposition != RebaseDispConflicted || begin.Reason != ReasonRebaseConflicted {
		t.Fatalf("begin = disp %q reason %q, want a plain conflicted stop", begin.Disposition, begin.Reason)
	}
	if !pathsGeneratedOnly(begin.UnmergedPaths) {
		t.Fatalf("fixture defect: the stop was not bundle-only: %v", begin.UnmergedPaths)
	}
}
```

And the generation-failure test (uses the seam):

```go
// TestIntegrationGeneratedOnlyGenerationFailureBlocks proves a failing
// regeneration blocks through the existing failure path, leaves the rebase
// stopped (abort available), and never spends resolver state.
func TestIntegrationGeneratedOnlyGenerationFailureBlocks(t *testing.T) {
	requireRealGit(t)
	f, deps, head, _ := prepareBundleConflictsWithoutBegin(t, 2, 1, docketModulePath)
	deps.RegenerateBundle = func(string) error { return fmt.Errorf("synthetic generation failure") }
	begin := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if begin.Result != ResultBlocked || begin.Reason != ReasonRebaseGitFailed {
		t.Fatalf("begin = (%q, %q), want blocked/%q", begin.Result, begin.Reason, ReasonRebaseGitFailed)
	}
	if st, _ := f.deps.Client.RebaseState(context.Background(), f.wp); st.Disposition != gitcli.RebaseConflicted {
		t.Errorf("the failed fast path did not retain the stopped rebase: %q", st.Disposition)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "0" || rec.ResolverReservationToken != "" {
		t.Errorf("generation failure spent resolver state: used %q token %q", rec.ResolverUsed, rec.ResolverReservationToken)
	}
	// Abort restores the recorded original head.
	abort := FinalizeRebaseAbort(context.Background(), deps, f.repo.invocation, f.id, rec.Attempt,
		ResolverReport{ChangeID: f.id, Attempt: rec.Attempt, Disposition: ResolverStuck})
	if abort.Result != ResultApplied {
		t.Fatalf("abort after generation failure = %q reason %q", abort.Result, abort.Reason)
	}
}
```

`prepareBundleConflictsWithoutBegin` is `beginBundleConflicts` minus the final `FinalizeRebase` call (returning `f, deps, head`); refactor `beginBundleConflicts` to call it.

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationGeneratedOnly' -count=1 -timeout 15m`
Expected: FAIL — the regression test sees `conflicted` (or `blocked resolver-budget-exhausted`) instead of `rebased`, because no fast path exists yet. (Compile errors first while `advanceGeneratedOnly` is missing — that is the same red.)

- [ ] **Step 3: Implement `advanceGeneratedOnly` in `internal/app/finalize_generated.go`**

Remove Task 1's placeholder lines and add:

```go
// advanceGeneratedOnly clears successive conflict stops whose live unmerged
// set is exclusively bundle outputs in an eligible Docket workspace. It runs
// under the per-workspace operation lock, and defers to the normal resolver
// flow whenever that flow already owns the stop (an outstanding reservation
// or a started continuation). Each cleared stop regenerates the bundle from
// the stopped commit's merged authored roots and stages ONLY the bundle
// directory; the stopped commit must advance every iteration (never a loop on
// an unchanged stop). Generated-only stops allocate no reservation and spend
// no resolver budget. It returns the first status it cannot clear — an
// authored/mixed conflict, a completed rewrite — for the caller's existing
// mapping, or a refusal on a probe error, a generation failure, or a
// non-advancing continue (retained; abort remains available).
func advanceGeneratedOnly(ctx context.Context, deps FinalizeDeps, op string, rc *rebaseContext, status gitcli.RebaseStatus) (gitcli.RebaseStatus, *FinalizeRebaseResult) {
	if status.Disposition != gitcli.RebaseConflicted ||
		!pathsGeneratedOnly(status.UnmergedPaths) || !bundleRepoEligible(rc.wsDir) {
		return status, nil
	}
	id := int(rc.change.ID())
	release, err := deps.Workspace.AcquireOperationLock(rc.metaDir)
	if err != nil {
		r := rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe,
			"could not acquire the workspace operation lock: "+err.Error(), id)
		return status, &r
	}
	defer release()

	// Reload the receipt under the lock: an outstanding reservation or a
	// started continuation means the resolver flow owns this stop — the fast
	// path steps aside rather than mutating Git out from under it.
	rec, present, rerr := deps.Workspace.ReadRebaseReceipt(ctx, rc.metaDir)
	if rerr != nil {
		r := rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseReceiptRead, rerr.Error(), id)
		return status, &r
	}
	if !present || rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted == "1" {
		return status, nil
	}

	git := continueGit(deps)
	regen := regenBundle(deps)
	var prev gitcli.ObjectID
	for status.Disposition == gitcli.RebaseConflicted && pathsGeneratedOnly(status.UnmergedPaths) {
		stopped, serr := git.StoppedRebaseCommit(ctx, rc.wsDir)
		if serr != nil {
			r := rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe, serr.Error(), id)
			return status, &r
		}
		if stopped == prev {
			r := withResolverCounts(rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseGitFailed,
				errBundleNotAdvancing.Error()+"; retained for abort", id), rec)
			return status, &r
		}
		prev = stopped
		if gerr := regen(rc.wsDir); gerr != nil {
			r := withResolverCounts(rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseGitFailed,
				"generated-bundle regeneration failed at the stopped commit: "+gerr.Error()+"; retained for abort", id), rec)
			return status, &r
		}
		next, cerr := git.StageAndContinueRebase(ctx, rc.wsDir, []string{embeddedBundleDir})
		if cerr != nil {
			r := withResolverCounts(rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseGitFailed, cerr.Error(), id), rec)
			return status, &r
		}
		status = next
	}
	return status, nil
}
```

- [ ] **Step 4: Wire the three call sites in `internal/app/finalize_rebase.go`**

(a) In `mapBegunRebase`, insert at the top of the function body, before the `switch status.Disposition`:

```go
	// Generated-only stops are cleared deterministically before any resolver
	// admission (change 0413): they allocate no reservation and spend no budget.
	var genRefusal *FinalizeRebaseResult
	status, genRefusal = advanceGeneratedOnly(ctx, deps, op, rc, status)
	if genRefusal != nil {
		return *genRefusal
	}
```

(b) In `recoverFromReceipt`, inside `case gitcli.RebaseConflicted:` — before building the conflicted result — run the fast path over the live state and, when it completes the rewrite, compose the gate:

```go
	case gitcli.RebaseConflicted:
		// Generated-only stops recover deterministically too (change 0413):
		// regeneration is idempotent, so a response-lost fast-path run simply
		// resumes here without any reservation.
		adv, genRefusal := advanceGeneratedOnly(ctx, deps, op, rc, state)
		if genRefusal != nil {
			return *genRefusal
		}
		if adv.Disposition == gitcli.RebaseUnchanged || adv.Disposition == gitcli.RebaseRebased {
			return composeLocalGate(ctx, deps, repoDir, op, rc, pr, rec, adv.HeadOID, false)
		}
		state = adv
		// The owned attempt is still mid-conflict; surface the live conflicts. …
		(existing conflicted-result construction, now reading state.UnmergedPaths / state.HeadOID as before)
```

(c) In `mapContinuedRebase`, insert at the top of the function body, before the `switch status.Disposition` — this MUST run before the `used >= limit` exhaustion branch so generated-only stops after the last permitted authored resolution still clear:

```go
	// Clear generated-only stops BEFORE the exhaustion check (change 0413):
	// the budget governs authored resolver dispatches only, "including after
	// the last permitted authored resolution".
	var genRefusal *FinalizeRebaseResult
	status, genRefusal = advanceGeneratedOnly(ctx, deps, op, rc, status)
	if genRefusal != nil {
		return *genRefusal
	}
```

- [ ] **Step 5: Run the integration tests and the unit suite**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationGeneratedOnly' -count=1 -timeout 15m`
Expected: PASS (regression, ineligible, generation-failure).
Run: `go test ./internal/app/ ./internal/gitcli/ -count=1` and `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebase|TestIntegrationResolverBudget' -count=1 -timeout 20m`
Expected: PASS — the existing conflicted/budget tests are unaffected because their fixtures are not bundle-eligible (`bundleRepoEligible` is false in every pre-existing fixture repo).

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_generated.go internal/app/finalize_rebase.go internal/app/finalize_generated_integration_test.go
git commit -m "feat(0413): generated-only fast path clears bundle stops without resolver budget"
```

---

### Task 4: Mixed authored+generated conflicts in the budgeted continue

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`finalizeRebaseContinueBudgeted`)
- Test: `internal/app/finalize_generated_integration_test.go`

**Interfaces:**
- Consumes: Task 1's `pathsGeneratedOnly`, `bundleRepoEligible`, `regenBundle`, `embeddedBundleDir`; Task 2's directory-pathspec staging guarantee.
- Produces: the mixed-conflict behavior the skill/agent docs (Task 6) describe: the resolver resolves authored paths only, the controller regenerates and stages the bundle in the same continue.

- [ ] **Step 1: Write the failing integration test**

Add to `internal/app/finalize_generated_integration_test.go`. Fixture: like `beginBundleConflicts` but the base's single commit ALSO conflictingly edits the SAME authored file a feature commit edits (e.g. base writes `skills/demo/SKILL.md` = "conflicting base skill\n" plus its regenerated bundle), producing a stop whose unmerged set holds both `skills/demo/SKILL.md` and bundle outputs. Name the fixture variant `beginMixedBundleConflict(t, limit)` (1 feature commit).

```go
// TestIntegrationMixedConflictChargesOnlyAuthoredDispatch proves a mixed
// authored+generated stop charges exactly ONE reservation for the authored
// decision: the resolver resolves the authored path only, the controller
// regenerates and stages the bundle in the same continue, and the rebase
// completes with used=1.
func TestIntegrationMixedConflictChargesOnlyAuthoredDispatch(t *testing.T) {
	requireRealGit(t)
	f, deps, _, begin := beginMixedBundleConflict(t, 2)
	if begin.Disposition != RebaseDispConflicted {
		t.Fatalf("begin = disp %q (paths %v), want a mixed conflicted stop", begin.Disposition, begin.UnmergedPaths)
	}
	if pathsGeneratedOnly(begin.UnmergedPaths) {
		t.Fatalf("fixture defect: the stop is generated-only, not mixed: %v", begin.UnmergedPaths)
	}
	attempt := begin.Attempt
	ctx := context.Background()

	reserve := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, attempt)
	if reserve.Disposition != ReserveReserved {
		t.Fatalf("reserve = %q reason %q", reserve.Disposition, reserve.Reason)
	}
	// The resolver resolves ONLY the authored path and reports only it —
	// leaving every bundle output for the controller.
	writeRepoFile(t, f.wp, "skills/demo/SKILL.md", "reconciled skill content\n")
	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"skills/demo/SKILL.md"}, ResolverReservation: reserve.Reservation}
	cont := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if cont.Result != ResultApplied || cont.Disposition != RebaseDispRebased {
		t.Fatalf("mixed continue = (%q, %q) reason %q msg %q, want applied/rebased", cont.Result, cont.Disposition, cont.Reason, cont.Message)
	}
	if cont.ResolverUsed != 1 {
		t.Errorf("mixed continue charged %d dispatches, want exactly 1", cont.ResolverUsed)
	}
	// The regenerated bundle at the final head is drift-clean and reflects the
	// RESOLVED authored content (never generated from unresolved inputs).
	m, payload, err := assets.Generate(f.wp, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("post-rebase generate: %v", err)
	}
	diffs, err := assets.DiffTree(filepath.Join(f.wp, "internal", "assets", "embedded"), m, payload)
	if err != nil || len(diffs) != 0 {
		t.Fatalf("post-rebase bundle drift: %v %v", diffs, err)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "1" || rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("receipt after mixed continue = used %q token %q cont %q, want 1/empty/empty", rec.ResolverUsed, rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
}

// TestIntegrationMixedConflictGenerationFailureNoContinuation proves a
// regeneration failure during a mixed continue refuses BEFORE the
// continuation-started marker is written: the reservation stays outstanding
// (retriable), Git is untouched, and no continuation is recorded.
func TestIntegrationMixedConflictGenerationFailureNoContinuation(t *testing.T) {
	requireRealGit(t)
	f, deps, _, begin := beginMixedBundleConflict(t, 2)
	attempt := begin.Attempt
	ctx := context.Background()
	reserve := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, attempt)
	if reserve.Disposition != ReserveReserved {
		t.Fatalf("reserve = %q", reserve.Disposition)
	}
	writeRepoFile(t, f.wp, "skills/demo/SKILL.md", "reconciled skill content\n")
	deps.RegenerateBundle = func(string) error { return fmt.Errorf("synthetic generation failure") }
	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"skills/demo/SKILL.md"}, ResolverReservation: reserve.Reservation}
	cont := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if cont.Result != ResultBlocked || cont.Reason != ReasonRebaseGitFailed {
		t.Fatalf("continue = (%q, %q), want blocked/%q", cont.Result, cont.Reason, ReasonRebaseGitFailed)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverContinuationStarted != "" {
		t.Errorf("a failed regeneration wrote the continuation-started marker: %q", rec.ResolverContinuationStarted)
	}
	if rec.ResolverReservationToken == "" {
		t.Errorf("the outstanding reservation was lost; a retried continue can no longer verify it")
	}
	if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseConflicted {
		t.Errorf("Git was mutated by the failed regeneration path: %q", st.Disposition)
	}
	// The SAME reservation retries successfully once regeneration works again.
	deps.RegenerateBundle = nil
	retry := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if retry.Result != ResultApplied || retry.Disposition != RebaseDispRebased {
		t.Fatalf("retried continue = (%q, %q) reason %q, want applied/rebased", retry.Result, retry.Disposition, retry.Reason)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationMixedConflict' -count=1 -timeout 15m`
Expected: FAIL — today the continue stages only the reported authored path, `git rebase --continue` refuses over the still-unmerged bundle paths, and the result is `blocked`/`git-rebase-failed` with the continuation marker already written.

- [ ] **Step 3: Implement the mixed branch in `finalizeRebaseContinueBudgeted`**

In `internal/app/finalize_rebase.go`, after the existing loop that validates every reported path against the live unmerged set (the block ending with the `ReasonRebaseReportPaths` refusal "a reported path is not in the live unmerged set") and BEFORE the "Durably mark the continuation started" write, insert:

```go
	// Mixed authored+generated stop (change 0413): the resolver resolves
	// authored inputs and leaves bundle outputs to the controller. When every
	// live unmerged path the report does NOT cover is an eligible bundle
	// output, regenerate the bundle from the now-resolved authored roots and
	// stage it alongside the reported paths in this same continue. This
	// charges nothing beyond the one reservation already spent for the
	// authored decision. Regeneration runs BEFORE the continuation-started
	// marker: a generation failure refuses with Git untouched and the
	// reservation still outstanding, so the same report can be retried.
	// Leftovers that are NOT eligible bundle outputs keep today's behavior
	// (the staged continue fails structurally and is retained).
	reported := make(map[string]bool, len(stage))
	for _, p := range stage {
		reported[p] = true
	}
	var leftover []string
	for _, p := range state.UnmergedPaths {
		if !reported[p] {
			leftover = append(leftover, p)
		}
	}
	if len(leftover) > 0 && pathsGeneratedOnly(leftover) && bundleRepoEligible(rc.wsDir) {
		if gerr := regenBundle(deps)(rc.wsDir); gerr != nil {
			return withResolverCounts(rebaseRefusal(op, ResultBlocked, RebaseDispBlocked, ReasonRebaseGitFailed,
				"generated-bundle regeneration failed before the continuation started: "+gerr.Error()+"; the reservation remains outstanding", id), rec)
		}
		stage = append(append([]string{}, stage...), embeddedBundleDir)
	}
```

(`state` and `stage` are the existing locals in that function; `id` and `rec` are in scope.)

- [ ] **Step 4: Run the tests**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationMixedConflict|TestIntegrationGeneratedOnly|TestIntegrationFinalizeRebaseContinueValidatesReport|TestIntegrationResolverBudget' -count=1 -timeout 20m`
Expected: PASS (new mixed tests green; existing continue/budget behavior unchanged — their fixtures are not bundle-eligible, so `bundleRepoEligible` short-circuits the new branch).
Run: `go test ./internal/app/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_generated_integration_test.go
git commit -m "feat(0413): mixed conflicts regenerate the bundle after authored resolution, charging one dispatch"
```

---

### Task 5: Interrupted re-entry without duplicate continuation

**Files:**
- Test: `internal/app/finalize_generated_integration_test.go`

**Interfaces:**
- Consumes: Task 3's fast path via `recoverFromReceipt` (the `advanceGeneratedOnly` call added there), the `RegenerateBundle` seam.

- [ ] **Step 1: Write the test**

Interrupt the fast path mid-run by making regeneration fail after clearing the first stop, then re-enter with the identical `finalize rebase` request:

```go
// TestIntegrationGeneratedOnlyInterruptedReentry proves a fast-path run
// interrupted between stops resumes idempotently on re-entry: the SAME
// finalize.rebase request recovers through the owned receipt, clears the
// remaining generated-only stops without any reservation, and never replays a
// completed continuation (the stopped commit advanced, so re-entry works on
// the NEXT stop, not the cleared one).
func TestIntegrationGeneratedOnlyInterruptedReentry(t *testing.T) {
	requireRealGit(t)
	f, deps, head, _ := prepareBundleConflictsWithoutBegin(t, 2, 3, docketModulePath)
	ctx := context.Background()

	// First entry: regeneration succeeds once, then fails — the run clears
	// stop 1 and blocks at stop 2, simulating an interruption mid-loop.
	calls := 0
	deps.RegenerateBundle = func(ws string) error {
		calls++
		if calls > 1 {
			return fmt.Errorf("synthetic interruption")
		}
		return regenerateEmbeddedBundle(ws)
	}
	first := FinalizeRebase(ctx, deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if first.Result != ResultBlocked || first.Reason != ReasonRebaseGitFailed {
		t.Fatalf("interrupted entry = (%q, %q), want blocked/%q", first.Result, first.Reason, ReasonRebaseGitFailed)
	}
	stoppedAfterFirst, err := f.deps.Client.StoppedRebaseCommit(ctx, f.wp)
	if err != nil {
		t.Fatalf("probe stopped commit: %v", err)
	}

	// Re-entry with the IDENTICAL request: recoverFromReceipt adopts the owned
	// attempt and the fast path resumes from the live stop.
	deps.RegenerateBundle = nil
	second := FinalizeRebase(ctx, deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
		t.Fatalf("re-entry = (%q, %q) reason %q msg %q, want applied/rebased", second.Result, second.Disposition, second.Reason, second.Message)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "0" || rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("re-entry spent resolver state: used %q token %q cont %q", rec.ResolverUsed, rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
	_ = stoppedAfterFirst // documents that re-entry resumed from the live stop
}
```

- [ ] **Step 2: Run it**

Run: `go test -tags integration ./internal/app/ -run TestIntegrationGeneratedOnlyInterruptedReentry -count=1 -timeout 15m`
Expected: PASS with Task 3 already in place (this test pins the recovery property; if it fails, the `recoverFromReceipt` wiring from Task 3 step 4(b) is wrong — fix that, not the test).

- [ ] **Step 3: Commit**

```bash
git add internal/app/finalize_generated_integration_test.go
git commit -m "test(0413): interrupted fast-path re-entry recovers without duplicate continuation"
```

---

### Task 6: Resolver/finalize instructions for the mixed-conflict handoff + bundle regen

**Files:**
- Modify: `agents/docket-rebase-resolver.md`
- Modify: `skills/docket-finalize-change/SKILL.md` (the "Resolver loop (Go-enforced budget)" block and the `conflicted` bullet in step "3. Rebase onto the effective base (resolver loop)")
- Modify: `skills/docket-finalize-change/references/gate-failure.md` (the ResolverReport / resolver-contract section, "The two agents")
- Regenerate: `internal/assets/embedded/` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: the behavior Tasks 3–4 shipped. No code interfaces.

- [ ] **Step 1: Edit the resolver agent contract**

In `agents/docket-rebase-resolver.md` (23 lines; read it first), add one sentence to the resolver's charter, in that file's own voice, stating: paths under `internal/assets/embedded/` (the generated bundle: `manifest.json` and `tree/`) are controller-owned derived outputs — the resolver never hand-merges them and never lists them in `conflicted_paths`; it resolves and reports the authored inputs only, and the finalize controller regenerates the bundle itself.

- [ ] **Step 2: Edit the finalize skill**

In `skills/docket-finalize-change/SKILL.md`, inside the resolver-loop block (between the `docket:feature-dispatch` markers — validate marker order before editing, per the managed-block rule): after the sentence describing what the resolver edits and returns ("It edits only the conflicted regions in the returned workspace and returns a versioned `ResolverReport` …"), add a sentence: in Docket's own repository, conflicts under `internal/assets/embedded/` are derived outputs the controller clears by regenerating the bundle — generated-only stops never reach the reserve step (the operation clears them itself, spending no budget), and on a mixed stop the resolver reports only the authored paths while `finalize.rebase-continue` regenerates and stages the bundle in the same continue.

- [ ] **Step 3: Edit the resolver report reference**

In `skills/docket-finalize-change/references/gate-failure.md`, in the resolver-report/"The two agents" section, add the same rule from the report's perspective: `conflicted_paths` lists authored paths only; bundle outputs under `internal/assets/embedded/` are omitted and controller-regenerated.

- [ ] **Step 4: Regenerate the embedded bundle and verify**

Run: `go generate ./internal/assets/`
Run: `go run ./cmd/genassets -repo . -check`
Expected: "matches the authored roots".

- [ ] **Step 5: Run the repo guard / docs tests**

Run: `go test ./internal/repoguard/ ./internal/assets/ -count=1`
Expected: PASS (sentinel and drift gates over the edited skill/agent files).

- [ ] **Step 6: Commit**

```bash
git add agents/docket-rebase-resolver.md skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md internal/assets/embedded
git commit -m "docs(0413): resolver leaves bundle outputs to the controller; regenerate embedded bundle"
```

---

### Task 7: Revert guard and the full suite gate

**Files:** none new — verification only.

- [ ] **Step 1: Mutation-test the fast path (revert guard)**

Temporarily neuter the fast path — in `internal/app/finalize_generated.go`, make `bundleRepoEligible` return `false` unconditionally as the first line (keep the edit in your working tree only; do NOT use `git checkout --` to restore later — undo the edit by hand or with `git diff`/`git apply -R`, per the mutation-restore rule):

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationGeneratedOnlyStopsBypassResolverBudget|TestIntegrationMixedConflictChargesOnlyAuthoredDispatch' -count=1 -timeout 15m`
Expected: FAIL — the regression sees a conflicted/exhausted stop instead of `rebased`, proving the tests detect a reverted fast path. Then undo the mutation exactly and re-run the same command: PASS. (`-count=1` defeats the test cache on both runs.)

- [ ] **Step 2: Run the whole configured suite from source**

From the feature workspace root:

Run: `go run ./cmd/docket development test`
Expected: green SUITE summary. Read the budget report even on green: act on any `SERIAL CONFIRMED OVER BUDGET:` line; note `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` findings in the build evidence.

- [ ] **Step 3: Commit any stragglers and finish**

```bash
git status --porcelain   # must be clean except intentional changes
```

No commit expected here unless steps above surfaced a fix; if one did, commit it with a `fix(0413): …` message.

---

## Self-Review (performed while writing)

- Spec coverage: eligibility/module identity → Task 1; regeneration mechanics (staging+rename, no partial bundle) → Task 1; generated-only stops with no reservation at begin/recover/continue, including after the last permitted authored resolution → Task 3 (call-site (c) placed before the exhaustion branch); staging "only the resulting bundle changes, including deletions" → Task 2 + `StageAndContinueRebase([]string{embeddedBundleDir})`; mixed conflicts, resolver leaves bundle outputs, regenerate after authored resolution, never from unresolved inputs → Task 4 (leftover-only regeneration; authored paths validated first); charged exactly once / never refunded → untouched reserve path + Task 4 assertions; ownership checks, operation lock, interruption/re-entry → lock inside `advanceGeneratedOnly`, reservation/continuation deference, Task 5; no loop on an unchanged stopped commit → `prev` guard; generation failure without continuation → Task 3 + Task 4 tests; ineligible repo on normal path → Task 3 test; post-rebase whole-suite gate preserved → fast path only returns statuses into the existing `composeLocalGate` flow; revert-makes-regression-fail → Task 7; skill/agent instructions + regenerated shipped assets → Task 6.
- The fixture-ordering caveat in Task 3 step 1 (shared docket-shaped ancestor) is called out explicitly because it is the one place the fixture can silently produce an authored conflict instead of a bundle-only one; both fixture-defect asserts (`pathsGeneratedOnly` checks on `begin.UnmergedPaths`) exist to catch that.
- Type consistency: `advanceGeneratedOnly(ctx, deps, op, rc, status) (gitcli.RebaseStatus, *FinalizeRebaseResult)` is used identically at all three call sites; `RegenerateBundle func(wsDir string) error` matches `regenBundle`'s return type; `embeddedBundleDir` (no trailing slash) is what both the stage call and `pathsGeneratedOnly`'s derived prefix use.
