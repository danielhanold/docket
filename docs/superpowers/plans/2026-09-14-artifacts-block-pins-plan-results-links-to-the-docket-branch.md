<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0417 — Artifacts block pins plan/results links to the docket branch, where those files never live](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-15-0417-artifacts-block-pins-plan-results-links-to-the-docket-branch.md)**
<!-- docket:backlink:end -->
# Lifecycle-Pinned Plan/Results Artifact Links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the `## Artifacts` block's Plan and Results rows link to the branch where those files actually live — the change's feature branch while not yet merged, the integration branch once `done` — instead of the hardcoded `docket` metadata branch where they never live (change 0417).

**Architecture:** `render.LinkContext` gains an `IntegrationBranch` field and a `BlobURLOnBranch(path, branch)` builder; `internal/render/artifacts.go` selects the ref per row from the change's own `Status()`/`Branch()` (Spec and ADR rows stay on the metadata branch); the app layer's sole constructor `linkContextOf` plumbs the integration branch from the already-pinned `StatusPin`. No new re-render call sites: all 17 `ArtifactBlockContent` sites already receive their `LinkContext` from `linkContextOf`, so the change auto-propagates (learning: check-plumbing-auto-discovery) — the only call-site edit is `repository_check.go`'s partial pin.

**Tech Stack:** Go; table-driven unit tests; frozen goldens in `internal/render/testdata/artifacts/`; mutation-probed guards run with `go test -count=1` (learning: cached-runner-serves-a-mutated-tree).

**Spec:** `docs/superpowers/specs/2026-09-10-artifacts-block-pins-plan-results-links-to-the-docket-branch-design.md` (lives on the `docket` metadata branch, readable at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/...` during the build).

## Global Constraints

- Ref selection table (spec, verbatim intent): Spec/ADR rows → metadata branch always; Plan/Results rows → feature branch (`branch:`) while the change is not `done`, integration branch once `done`. `stacked-merged` and `killed` get no special handling: the same test (`done` ⇒ integration, else feature branch) applies — document the choice in code comments.
- Defensive fallback: a Plan/Results path present with no usable branch (empty `branch:`, or empty `IntegrationBranch` on a `done` change) falls back to the metadata branch (today's behavior) — never a malformed URL.
- Relative-link mode (empty `RepoWebURL`) embeds no branch: output must stay byte-identical to today.
- `linkContextOf` stays the sole field-carrying `render.LinkContext` constructor in `internal/app` (0341 guard, `link_context_guard_test.go`, floor 16 — unchanged by this plan: no call sites added or removed).
- The frozen goldens in `internal/render/testdata/artifacts/` are historical snapshots (see `PROVENANCE.md`); this plan changes no existing golden byte — the existing fixtures exercise exactly the fallback path, and the tasks assert that.
- Every mutation probe: back up the file with `cp` before mutating and restore from the backup, never `git checkout --` over uncommitted work (learning: mutation-restore-needs-a-backup-copy); run the probe with `-count=1`.
- The build gate runs the whole suite via the configured `build.test_command` (docket-build owns the gate); read the budget report even on green.

**Working directory for every command:** `/Users/homer/dev/docket/.worktrees/artifacts-block-pins-plan-results-links-to-the-docket-branch`

---

### Task 1: `LinkContext.IntegrationBranch` + `BlobURLOnBranch`

**Files:**
- Modify: `internal/render/link.go`
- Test: `internal/render/link_test.go`

**Interfaces:**
- Consumes: existing `render.LinkContext{RepoWebURL, MetadataBranch string}` and `BlobURL(repoRelPath string) string`.
- Produces: `LinkContext.IntegrationBranch string` (new field); `func (l LinkContext) BlobURLOnBranch(repoRelPath, branch string) string` — returns `""` when `RepoWebURL` is empty; an empty `branch` falls back to `MetadataBranch`; otherwise `RepoWebURL + "/blob/" + branch + "/" + repoRelPath`. `BlobURL` becomes a delegation to `BlobURLOnBranch(path, l.MetadataBranch)` with unchanged behavior. Tasks 2 and 3 rely on these exact names.

- [ ] **Step 1: Write the failing tests**

Append to `internal/render/link_test.go`:

```go
func TestBlobURLOnBranch(t *testing.T) {
	l := render.LinkContext{
		RepoWebURL:        "https://github.com/danielhanold/docket",
		MetadataBranch:    "docket",
		IntegrationBranch: "main",
	}
	cases := []struct{ name, branch, want string }{
		{"feature branch", "fix/some-change",
			"https://github.com/danielhanold/docket/blob/fix/some-change/docs/superpowers/plans/x.md"},
		{"integration branch", "main",
			"https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/x.md"},
		// Defensive fallback: an empty branch must yield the metadata-branch
		// URL, never a malformed "/blob//" URL.
		{"empty branch falls back to metadata", "",
			"https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/x.md"},
	}
	for _, c := range cases {
		if got := l.BlobURLOnBranch("docs/superpowers/plans/x.md", c.branch); got != c.want {
			t.Errorf("%s: BlobURLOnBranch = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestBlobURLOnBranchWithoutRepoWebURL(t *testing.T) {
	l := render.LinkContext{RepoWebURL: "", MetadataBranch: "docket", IntegrationBranch: "main"}
	if got := l.BlobURLOnBranch("docs/x.md", "fix/some-change"); got != "" {
		t.Fatalf("BlobURLOnBranch with empty RepoWebURL = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/render/ -run 'TestBlobURLOnBranch' -count=1`
Expected: FAIL to compile — `l.BlobURLOnBranch undefined` / unknown field `IntegrationBranch`.

- [ ] **Step 3: Implement**

In `internal/render/link.go`, add the field to `LinkContext` (after `MetadataBranch`):

```go
	// IntegrationBranch is the branch PR merges land on, e.g. "main". It is
	// consulted only for rows whose file reaches the integration branch (the
	// Plan/Results rows of a done change); empty falls back to MetadataBranch
	// at the BlobURLOnBranch boundary, so a malformed URL is unrepresentable.
	IntegrationBranch string
```

Replace `BlobURL`'s body with a delegation and add the new builder:

```go
// BlobURL returns the blob URL on the metadata branch — correct for records
// that live on that branch (change files, specs, ADRs) — or "" when
// RepoWebURL is empty.
func (l LinkContext) BlobURL(repoRelPath string) string {
	return l.BlobURLOnBranch(repoRelPath, l.MetadataBranch)
}

// BlobURLOnBranch returns RepoWebURL + "/blob/" + branch + "/" + repoRelPath,
// or "" when RepoWebURL is empty. An empty branch falls back to
// MetadataBranch: the defensive default for callers whose lifecycle ref is
// unresolvable (change 0417), never a malformed "/blob//" URL.
func (l LinkContext) BlobURLOnBranch(repoRelPath, branch string) string {
	if l.RepoWebURL == "" {
		return ""
	}
	if branch == "" {
		branch = l.MetadataBranch
	}
	return l.RepoWebURL + "/blob/" + branch + "/" + repoRelPath
}
```

Also update `link.go`'s `MetadataBranch` field comment from "the branch blob links point at" to "the metadata branch blob links for metadata-branch records point at, e.g. \"docket\"" — it is no longer the branch for every link.

- [ ] **Step 4: Run the render package tests**

Run: `go test ./internal/render/ -count=1`
Expected: PASS (new tests green; existing `TestBlobURLWithRepoWebURL`/`TestBlobURLWithoutRepoWebURL` and all golden tests untouched and green — the delegation is behavior-preserving).

- [ ] **Step 5: Commit**

```bash
git add internal/render/link.go internal/render/link_test.go
git commit -m "feat(0417): LinkContext gains IntegrationBranch and a per-branch blob URL builder"
```

---

### Task 2: Per-row lifecycle ref selection in the artifacts renderer

**Files:**
- Modify: `internal/render/artifacts.go`
- Test: `internal/render/artifacts_test.go`

**Interfaces:**
- Consumes: `LinkContext.IntegrationBranch` and `BlobURLOnBranch(path, branch)` from Task 1; `domain.Change.Status()`, `domain.Change.Branch()` (an `OptionalString`), `domain.StatusDone`, `domain.StatusImplemented`, `domain.StatusStackedMerged` (all existing).
- Produces: unexported `func lifecycleBranch(c domain.Change, link LinkContext) string` and `pathRow(label, repoRelPath, branch string, link LinkContext) string` (gains a `branch` parameter). External signature `ArtifactBlockContent(c, snap, link)` unchanged — nothing outside the package changes for Task 3.

- [ ] **Step 1: Write the failing tests**

Append to `internal/render/artifacts_test.go`. Note the two new fixtures deliberately reuse **beta's artifact paths** so the relative-mode assertion can byte-compare against the existing frozen `block-spec-plan-results.relative.golden` — proving lifecycle state is invisible in relative mode.

```go
// gammaChange is fixture C: beta's spec/plan/results paths, plus the in-flight
// lifecycle state 0417 pins on — status implemented, feature branch set.
func gammaChange() domain.Change {
	return domain.NewChange(domain.ChangeSpec{
		ID:       9,
		Slug:     "gamma-change",
		Title:    "Gamma change",
		Status:   domain.StatusImplemented,
		Branch:   optString("fix/gamma-change"),
		Spec:     optString("docs/superpowers/specs/2026-08-16-beta-change-design.md"),
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Results:  optString("docs/results/2026-08-16-beta-change-results.md"),
		Location: domain.LocationActive,
		Path:     "docs/changes/active/0009-gamma-change.md",
	})
}

// deltaChange is fixture D: gamma after merge — done, archived.
func deltaChange() domain.Change {
	return domain.NewChange(domain.ChangeSpec{
		ID:       10,
		Slug:     "delta-change",
		Title:    "Delta change",
		Status:   domain.StatusDone,
		Branch:   optString("fix/delta-change"),
		Spec:     optString("docs/superpowers/specs/2026-08-16-beta-change-design.md"),
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Results:  optString("docs/results/2026-08-16-beta-change-results.md"),
		Location: domain.LocationArchive,
		Path:     "docs/changes/archive/2026-09-14-0010-delta-change.md",
	})
}

// TestArtifactBlockLifecyclePinsPlanResults is 0417's core assert and its
// mutation-tested guard (guards-are-code): the Plan/Results rows of a
// not-yet-done change resolve onto the FEATURE branch, of a done change onto
// the INTEGRATION branch, while the Spec row stays on the metadata branch in
// every state. Asserts pin the exact produced URL, positive and negative
// (assert-pins-outcome-not-mechanism / assert-detects-removal-not-replacement):
// each case also proves the docket-pinned spelling is GONE for Plan/Results.
//
// Mutation probes (run with -count=1; cp-backup artifacts.go first):
//   (a) in ArtifactBlockContent, pass link.MetadataBranch instead of
//       lifecycleBranch(c, link) for the Plan and Results rows (the pre-0417
//       hardcoding) -> the implemented and done cases must redden;
//   (b) in lifecycleBranch, delete the StatusDone arm -> the done case must
//       redden;
//   (c) in lifecycleBranch, delete the empty-branch fallback -> the
//       fallback case must redden.
func TestArtifactBlockLifecyclePinsPlanResults(t *testing.T) {
	const base = "https://github.com/danielhanold/docket/blob/"
	cases := []struct {
		name       string
		c          domain.Change
		wantRef    string // branch the Plan/Results URLs must use
	}{
		{"implemented pins the feature branch", gammaChange(), "fix/gamma-change"},
		{"done pins the integration branch", deltaChange(), "main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := render.ArtifactBlockContent(tc.c, domain.Snapshot{}, githubLink)
			if err != nil {
				t.Fatalf("ArtifactBlockContent: %v", err)
			}
			wantPlan := "| Plan | [2026-08-16-beta-change.md](" + base + tc.wantRef + "/docs/superpowers/plans/2026-08-16-beta-change.md) |"
			wantResults := "| Results | [2026-08-16-beta-change-results.md](" + base + tc.wantRef + "/docs/results/2026-08-16-beta-change-results.md) |"
			wantSpec := "| Spec | [2026-08-16-beta-change-design.md](" + base + "docket/docs/superpowers/specs/2026-08-16-beta-change-design.md) |"
			for _, want := range []string{wantPlan, wantResults, wantSpec} {
				if !strings.Contains(got, want) {
					t.Errorf("block missing row %q\ngot:\n%s", want, got)
				}
			}
			for _, banned := range []string{
				base + "docket/docs/superpowers/plans/",
				base + "docket/docs/results/",
			} {
				if strings.Contains(got, banned) {
					t.Errorf("Plan/Results row still pinned to the metadata branch (%q present)\ngot:\n%s", banned, got)
				}
			}
		})
	}
}

// TestArtifactBlockStackedMergedUsesFeatureBranch documents the spec's
// explicit choice: stacked-merged is NOT done, so it takes the feature-branch
// leg of the same test — no special handling.
func TestArtifactBlockStackedMergedUsesFeatureBranch(t *testing.T) {
	spec := domain.ChangeSpec{
		ID: 11, Slug: "stacked-change", Title: "Stacked change",
		Status:   domain.StatusStackedMerged,
		Branch:   optString("fix/stacked-change"),
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Location: domain.LocationActive,
		Path:     "docs/changes/active/0011-stacked-change.md",
	}
	got, err := render.ArtifactBlockContent(domain.NewChange(spec), domain.Snapshot{}, githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := "https://github.com/danielhanold/docket/blob/fix/stacked-change/docs/superpowers/plans/2026-08-16-beta-change.md"
	if !strings.Contains(got, want) {
		t.Errorf("stacked-merged Plan row does not use the feature branch\ngot:\n%s", got)
	}
}

// TestArtifactBlockMissingBranchFallsBackToMetadata pins the defensive
// default: a Plan path present with no branch: renders today's metadata-branch
// URL, never a malformed one. (betaChange — no status, no branch — exercises
// the same path via the frozen golden; this case makes the implemented-state
// variant explicit.)
func TestArtifactBlockMissingBranchFallsBackToMetadata(t *testing.T) {
	spec := domain.ChangeSpec{
		ID: 12, Slug: "branchless-change", Title: "Branchless change",
		Status:   domain.StatusImplemented,
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Location: domain.LocationActive,
		Path:     "docs/changes/active/0012-branchless-change.md",
	}
	got, err := render.ArtifactBlockContent(domain.NewChange(spec), domain.Snapshot{}, githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := "https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-16-beta-change.md"
	if !strings.Contains(got, want) {
		t.Errorf("branchless Plan row did not fall back to the metadata branch\ngot:\n%s", got)
	}
}

// TestArtifactBlockRelativeModeIgnoresLifecycle: relative rendering embeds no
// branch, so an implemented change with a feature branch must produce the
// exact bytes of the frozen relative golden (which was generated from a
// lifecycle-free fixture with the same artifact paths).
func TestArtifactBlockRelativeModeIgnoresLifecycle(t *testing.T) {
	got, err := render.ArtifactBlockContent(gammaChange(), domain.Snapshot{}, relativeLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := readArtifactGolden(t, "block-spec-plan-results.relative.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Errorf("relative-mode output diverged from frozen golden\ngot:\n%s\nwant:\n%s", got, want)
	}
}
```

Note: `githubLink` must gain the integration branch for the done case. Change the existing var (this cannot alter alpha/beta golden output — those fixtures never take the integration leg):

```go
var githubLink = render.LinkContext{RepoWebURL: "https://github.com/danielhanold/docket", MetadataBranch: "docket", IntegrationBranch: "main"}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/render/ -run 'TestArtifactBlock' -count=1`
Expected: FAIL — the lifecycle, stacked-merged cases redden (URLs still say `blob/docket`); the fallback and relative cases pass (they pin current behavior; that is fine — their job is to survive the change).

- [ ] **Step 3: Implement the per-row selection**

In `internal/render/artifacts.go`:

Add the selector (near `pathRow`):

```go
// lifecycleBranch resolves the blob ref for a Plan/Results row (change 0417).
// Those files never live on the metadata branch: they live on the change's
// feature branch until the PR merges, and on the integration branch once the
// change is done. done => IntegrationBranch; every other status — including
// stacked-merged and killed, which get no special handling by design — uses
// the feature branch. An unresolvable ref (unset branch:, or an empty
// IntegrationBranch) returns "" and BlobURLOnBranch falls back to the
// metadata branch: today's behavior, never a malformed URL.
func lifecycleBranch(c domain.Change, link LinkContext) string {
	if c.Status() == domain.StatusDone {
		return link.IntegrationBranch
	}
	return c.Branch().Value
}
```

Give `pathRow` a branch parameter and route it through `BlobURLOnBranch`:

```go
// pathRow renders a Spec/Plan/Results row. In GitHub mode the link text is the
// path basename and the URL is the full repo-relative path on the given
// branch ("" falls back to the metadata branch); in repo-relative mode the
// cell is the backtick-quoted path.
func pathRow(label, repoRelPath, branch string, link LinkContext) string {
	if url := link.BlobURLOnBranch(repoRelPath, branch); url != "" {
		return fmt.Sprintf("| %s | [%s](%s) |", label, path.Base(repoRelPath), url)
	}
	return fmt.Sprintf("| %s | `%s` |", label, repoRelPath)
}
```

Update the three call sites in `ArtifactBlockContent`:

```go
	if p := c.Spec().Value; p != "" {
		rows = append(rows, pathRow("Spec", p, link.MetadataBranch, link))
	}
	if p := c.Plan().Value; p != "" {
		rows = append(rows, pathRow("Plan", p, lifecycleBranch(c, link), link))
	}
	if p := c.Results().Value; p != "" {
		rows = append(rows, pathRow("Results", p, lifecycleBranch(c, link), link))
	}
```

Update `ArtifactBlockContent`'s doc comment: replace the final sentence ("The v1 LinkContext carries a single MetadataBranch, so every link resolves onto that branch; the Bash renderer's lifecycle-pinned Plan/Results branch is a later concern.") with:

```
// Spec and ADR rows link the metadata branch, where those records live. Plan
// and Results rows are lifecycle-pinned (change 0417): the feature branch
// while the change is not yet done, the integration branch once it is —
// see lifecycleBranch.
```

`adrCell` and `BacklinkContent` are untouched — ADRs and change files live on the metadata branch.

- [ ] **Step 4: Run the render package tests**

Run: `go test ./internal/render/ -count=1`
Expected: PASS — including the untouched frozen-golden tests `TestArtifactBlockSpecPlanResultsGitHub`/`...Relative` (betaChange has no status and no branch, so both rows take the metadata fallback and the goldens stay byte-identical; no golden file is regenerated — the spec anticipated regenerating `block-spec-plan-results.github.golden`, but the frozen fixture exercises exactly the fallback path, so byte-identity is the correct, stronger outcome).

- [ ] **Step 5: Run the mutation probes**

For each probe in the `TestArtifactBlockLifecyclePinsPlanResults` comment:

```bash
cp internal/render/artifacts.go "${TMPDIR:-/tmp}/artifacts.go.bak"
# apply mutation (a): in ArtifactBlockContent change both lifecycleBranch(c, link) args to link.MetadataBranch
go test ./internal/render/ -run 'TestArtifactBlockLifecyclePinsPlanResults' -count=1  # must FAIL
cp "${TMPDIR:-/tmp}/artifacts.go.bak" internal/render/artifacts.go
# repeat for (b): delete the StatusDone arm of lifecycleBranch  -> done case must FAIL
# repeat for (c): in link.go's BlobURLOnBranch, delete the empty-branch fallback
#   (cp-backup link.go for this one) -> TestArtifactBlockMissingBranchFallsBackToMetadata
#   and TestBlobURLOnBranch must FAIL
```

Expected: each mutation reddens the named test; after each restore, `go test ./internal/render/ -count=1` is green. A probe that stays green is a defect — stop and investigate before proceeding (residual-is-for-undetectable-not-unprobed).

- [ ] **Step 6: Commit**

```bash
git add internal/render/artifacts.go internal/render/artifacts_test.go
git commit -m "feat(0417): lifecycle-pin Plan/Results artifact rows to feature/integration branch"
```

---

### Task 3: Plumb the integration branch through `linkContextOf` and the check corpus

**Files:**
- Modify: `internal/app/link_context.go`
- Modify: `internal/app/repository_check.go` (one line, ~line 515 — anchor on the `corpus.link = linkContextOf(...)` assignment, not the line number)
- Test: `internal/app/link_context_test.go`

**Interfaces:**
- Consumes: `render.LinkContext.IntegrationBranch` (Task 1); `StatusPin.IntegrationBranch` and `StatusPin.DefaultBranch` (existing, `internal/app/status.go`); `setupContext.integrationBranch` (existing, `internal/app/repository_facts.go`).
- Produces: `linkContextOf(pin StatusPin) render.LinkContext` now fills `IntegrationBranch` from `pin.IntegrationBranch`, falling back to `pin.DefaultBranch` when empty — the same fallback `finalize_closeout.go`'s `closeoutContext` computes for its git operations, folded into the constructor so every render site agrees (learning: duplicated-gate-copies-the-whole-predicate). All 21 existing `linkContextOf` call sites pick this up with no edits except `repository_check.go`'s hand-built partial pin.

- [ ] **Step 1: Write the failing tests**

In `internal/app/link_context_test.go`, extend `TestLinkContextOfCarriesBothFields` (the struct-equality `want` must now carry the new field, or the test reddens on its own — extend it deliberately) and add the fallback case:

```go
// TestLinkContextOfCarriesBothFields is the constructor half of the 0341
// regression guard, extended by 0417 with the integration branch. Mutation
// probes: drop RepoWebURL from linkContextOf — reddens; drop the
// IntegrationBranch assignment — reddens (defaulted-param-hides-caller-wiring:
// the assert pins the RESOLVED non-default value).
func TestLinkContextOfCarriesBothFields(t *testing.T) {
	pin := StatusPin{
		RepoWebURL:        "https://github.com/owner/repo",
		IntegrationBranch: "main",
	}
	got := linkContextOf(pin)
	want := render.LinkContext{
		RepoWebURL:        "https://github.com/owner/repo",
		MetadataBranch:    "docket",
		IntegrationBranch: "main",
	}
	if got != want {
		t.Fatalf("linkContextOf = %+v, want %+v", got, want)
	}
	if url := got.BlobURL("docs/x.md"); url != "https://github.com/owner/repo/blob/docket/docs/x.md" {
		t.Fatalf("BlobURL = %q", url)
	}
}

// TestLinkContextOfIntegrationFallsBackToDefaultBranch mirrors
// closeoutContext's integration-branch fallback: an unresolved
// IntegrationBranch on the pin falls back to DefaultBranch.
func TestLinkContextOfIntegrationFallsBackToDefaultBranch(t *testing.T) {
	got := linkContextOf(StatusPin{DefaultBranch: "trunk"})
	if got.IntegrationBranch != "trunk" {
		t.Fatalf("IntegrationBranch = %q, want fallback to DefaultBranch %q", got.IntegrationBranch, "trunk")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestLinkContextOf' -count=1`
Expected: FAIL — `got != want` (IntegrationBranch empty), and the fallback test fails.

- [ ] **Step 3: Implement**

In `internal/app/link_context.go`, replace `linkContextOf`:

```go
// linkContextOf is the sole constructor of the LinkContext app operations hand
// to render: the repository web URL and the branch names travel together, so
// no call site can silently omit a field again — the exact defect 0341 fixes.
// The metadata records always live on the fixed docket branch; the integration
// branch (0417: the ref for a done change's Plan/Results rows) comes from the
// pin, falling back to the default branch exactly as finalize's
// closeoutContext resolves it for git operations.
func linkContextOf(pin StatusPin) render.LinkContext {
	integration := pin.IntegrationBranch
	if integration == "" {
		integration = pin.DefaultBranch
	}
	return render.LinkContext{
		RepoWebURL:        pin.RepoWebURL,
		MetadataBranch:    reposetup.MetadataBranchName,
		IntegrationBranch: integration,
	}
}
```

In `internal/app/repository_check.go`, the corpus builder (`readCheckCorpus`) hand-builds a partial pin; without the integration branch the check would render different bytes than the authoritative writers and report false artifact-links drift (decide-and-act-on-the-same-copy). Extend it — `sc.integrationBranch` is already populated from resolved config in `repository_facts.go`:

```go
	corpus.link = linkContextOf(StatusPin{
		RepoWebURL:        githubWebURL(remoteURL),
		IntegrationBranch: sc.integrationBranch,
	})
```

(`repository_migrate_repair.go` reuses `corpus.link`, so the repair path stays consistent with the check for free.)

- [ ] **Step 4: Run the app package tests**

Run: `go test ./internal/app/ -count=1`
Expected: PASS — including `TestLinkContextSoleConstructor` (no literal added, call-site count unchanged at ≥16) and the workflow integration tests (their asserts target metadata-branch records — backlinks and Spec rows — which are unaffected). If any app-layer test reddens on a `blob/docket` Plan/Results expectation for an implemented/done fixture, that test was pinning the 0417 defect: update its expectation to the lifecycle ref and say so in the commit message (test-premise-deleted-not-regated — check what the assert guarded first).

- [ ] **Step 5: Run the mutation probes**

```bash
cp internal/app/link_context.go "${TMPDIR:-/tmp}/link_context.go.bak"
# mutation: delete the IntegrationBranch field assignment in linkContextOf
go test ./internal/app/ -run 'TestLinkContextOf' -count=1   # must FAIL
cp "${TMPDIR:-/tmp}/link_context.go.bak" internal/app/link_context.go
# mutation: in repository_check.go's corpus pin, drop the IntegrationBranch
# entry — if no existing check-corpus test reddens, that is a KNOWN residual
# (the hermetic check fixtures may not contain an implemented change with a
# plan row); do NOT write a vacuous assert — record the residual in the
# results file instead (residual-is-for-undetectable-not-unprobed: first
# actually probe it; only an unreddenable state may be recorded as residual).
cp "${TMPDIR:-/tmp}/link_context.go.bak" internal/app/link_context.go 2>/dev/null || true
git diff --stat  # confirm the tree is back to the committed-plus-intended state
```

- [ ] **Step 6: Commit**

```bash
git add internal/app/link_context.go internal/app/link_context_test.go internal/app/repository_check.go
git commit -m "feat(0417): plumb the integration branch into linkContextOf and the check corpus"
```

---

## Build-time verification (docket-build's gate, after all tasks)

- Full suite green via the configured `build.test_command` (the Go runner is the sole channel). Read the budget report even on green: this change adds ~6 small unit tests, so no `SERIAL CONFIRMED OVER BUDGET` line is expected — surface any that appears.
- Metadata-branch behavior is invisible to the hermetic suite (learning: metadata-branch-invisible-to-suite): at build time, verify against real state and record it in the results file — render change 0416's block with the built binary (or trace `lifecycleBranch` by inspection against 0416's frontmatter: `status: implemented`, `branch: fix/scoped-build-task-gate-starts-omit-prepared-scope-identity`) and confirm the Results URL targets that feature branch, where the file verifiably exists (`git ls-tree origin/fix/scoped-build-task-gate-starts-omit-prepared-scope-identity -- docs/results/` after a fetch).
- Record the expected one-time post-merge effect in the results file: existing records written before 0417 carry docket-pinned Plan/Results rows, so `repository check` will surface artifact-links drift for implemented/done changes with plan/results set; the remedy is the existing `docket repository migrate` (the repair path renders with the same corpus link this change fixed). This is expected behavior, not a defect.

## Out of scope (from the change file — do not build)

PR row in the Go renderer; terminal publication / copying results to the metadata branch; moving where plan/results files live; the reciprocal `docket:backlink` blocks; change 0405's gate-handshake work.

## Self-review notes

- Spec coverage: implementation-shape items 1–4 map to Tasks 1–3; item 5 (budget re-baseline) is conditional and covered in build-time verification — no exact-count guard changes here (the 0341 floor is a minimum, untouched). The spec's "regenerate the github golden / add a done-state golden" is deliberately replaced by byte-identity on the frozen goldens plus inline exact-URL asserts: the frozen fixtures take the fallback path unchanged, and PROVENANCE.md forbids regenerating historical snapshots.
- Type consistency: `lifecycleBranch(c domain.Change, link LinkContext) string`, `pathRow(label, repoRelPath, branch string, link LinkContext)`, `BlobURLOnBranch(repoRelPath, branch string) string`, `LinkContext.IntegrationBranch string` are spelled identically across tasks.
