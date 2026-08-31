<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0367 — Configurable rendered-board sections and sorting](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0367-configurable-rendered-board-sections-and-sorting.md)**
<!-- docket:backlink:end -->
# Configurable Rendered-Board Sections and Sorting — Implementation Plan

> **For agentic workers:** This plan is executed by the docket-build skill, which fans each task
> out to a profile worker under the docket-build-task contract (focused test → implement →
> verify → self-review → one commit per task). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render the inline board as six configurable presentation groups (In progress, Built,
Blocked, Groomed, Proposed, Deferred) with per-section, layered-config-driven sorting, defaulting
to `updated desc`, while leaving every other projection (digest, autonomous selection, Mermaid
graph, GitHub mirror, lifecycle data) byte-for-byte and semantics-for-semantics unchanged.

**Architecture:** Three distinct responsibilities. (1) `internal/config` owns the new `board`
block: schema rows in the single-owner registry, warn-and-ignore-with-inheritance validation,
layered resolution, provenance, and inspection. (2) `internal/render/board.go` gains a pure
classification function (lifecycle change → rendered section token) and a total, deterministic
per-section comparator, both driven by a typed `BoardPresentation` carried on `BoardInput`.
(3) `internal/app` builds the presentation from the resolved config in exactly one place
(`derived_views.go`) and threads it to the one renderer entry point; no transaction caller
reconstructs defaults.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), `go.yaml.in/yaml/v3`, table-driven
`go test`. Focused runs: `go test ./internal/<pkg>/ -run <Name> -count=1` (`-count=1` always —
the Go test cache will otherwise serve a green result against a tree you just mutated).

**Spec:** `docs/superpowers/specs/2026-08-29-configurable-rendered-board-sections-and-sorting-design.md`
(on the `docket` metadata branch; the change file is
`docs/changes/active/0367-configurable-rendered-board-sections-and-sorting.md`).

## Global Constraints

- The six section tokens are a closed vocabulary, exactly: `in-progress`, `built`, `blocked`,
  `groomed`, `proposed`, `deferred` — deliberately distinct from lifecycle statuses.
- `board.section_order` is a whole-list replacement; a valid list contains every token exactly
  once. A list with a missing, unknown, or duplicate token is **warned about and ignored as one
  value**, letting the next valid lower layer or the built-in order win. Never auto-append.
- `board.sorting.<section>.by` ∈ {`id`, `updated`, `created`}; `.direction` ∈ {`asc`, `desc`}.
  Each is an independent scalar leaf: an invalid leaf is warned about and only that leaf inherits.
- Built-in defaults: section order = the six tokens in the order above; every section sorts
  `updated desc`.
- Board settings are `scopeAny` (global-able) and never coordination-fenced.
- Equal date values tie-break on numeric ID in the **same direction** as the primary sort.
  Missing/empty/malformed dates sort **after all valid dates regardless of direction**; ties
  inside the unknown-date group use ID in the configured direction.
- Archive stays a fixed footer outside `section_order`/`sorting`: date descending, then numeric
  ID descending within a date. Unchanged labels, recent window (15), monthly collapse.
- Projection isolation is mechanical: digest, autonomous selection, Mermaid graph node/edge
  order, and GitHub-mirror semantics receive **no** board presentation input.
- A board config warning must not invalidate the snapshot, suppress unrelated valid board
  leaves, or disable the inline surface.
- Every guard added here is mutation-tested: strip the guarded thing, watch the focused test
  redden, restore. Restore from a saved copy of your edit (e.g. `cp file file.bak` first) —
  `git checkout -- file` restores to HEAD and destroys the uncommitted work under test. Defeat
  the test cache with `-count=1` on every mutation probe.
- No Bash renderer or Bash config work; no shell evaluation, network, or Git writes in rendering.
- Commit messages: conventional prefix + `(0367)`, e.g. `feat(0367): …` / `test(0367): …`.

## File Structure

- Modify `internal/config/schema.go` — registry rows for `board.section_order` and the 12
  `board.sorting.<section>.{by,direction}` leaves; exported canonical token slice.
- Modify `internal/config/config.go` — `Board`/`BoardSort` types on `Effective`.
- Modify `internal/config/defaults.go` — built-in board defaults.
- Modify `internal/config/resolve.go` — assemble the board leaves.
- Modify `internal/config/decode.go` — warn-and-drop validation for `board.*` (the
  `keepKnownSurfaces` precedent generalized).
- Modify `internal/config/resolve_test.go`, `internal/config/decode_test.go`,
  `internal/config/defaults_test.go`, `internal/config/schema_test.go` (whatever the exhaustive
  registry-walk tests require for new rows).
- Modify `internal/app/config.go` — `effectiveLines` board rows.
- Modify `.docket.example.yml` and `internal/assets/embedded/tree/.docket.example.yml` — the
  canonical example gains the `board` block (both copies; a drift assert ties them).
- Modify `internal/render/board.go` — presentation types, classification, comparator,
  presentation-driven emission, rewritten header contract comment.
- Modify `internal/render/board_test.go` + `internal/render/testdata/board/` — new golden,
  updated PROVENANCE.md, new unit/regression tests.
- Modify `internal/app/derived_views.go` — the one option-building path + threaded signatures.
- Modify the `includeBoard`/`renderCanonicalBoard` call sites in `internal/app` (change_attach,
  change_claim, change_create, change_kill, change_groom, change_implemented, change_reclaim,
  change_lifecycle, change_repair, change_reconcile, finalize_block, repository_check,
  repository_migrate_repair — derive the authoritative list by grep, never from this prose).
- Add isolation-guard tests beside the digest/selection owners (`internal/app`,
  `internal/domain/selection_test.go` vicinity, `internal/render/board_test.go`).

---

### Task 1: Config schema — `board` registry rows, typed Effective leaves, built-in defaults

**Files:**
- Modify: `internal/config/schema.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/defaults.go`
- Modify: `internal/config/resolve.go`
- Test: `internal/config/resolve_test.go`, `internal/config/defaults_test.go`

**Interfaces:**
- Consumes: the existing registry (`buildRegistry`, `pathSpec`, `listLeaf`, `enumLeaf`,
  `assign`, `builtinValue`, `builtinEffective`).
- Produces (later tasks rely on these exact names):
  - `config.BoardSectionTokens = []string{"in-progress", "built", "blocked", "groomed", "proposed", "deferred"}` (exported, canonical order).
  - `config.BoardSortFields = []string{"id", "updated", "created"}` and
    `config.BoardSortDirections = []string{"asc", "desc"}` (exported).
  - On `config.Effective`, after `BoardSurfaces`:
    ```go
    Board Board `json:"board"`
    ```
    with
    ```go
    // Board is the rendered-board presentation policy: a complete section-order
    // permutation and one sort per section. Sorting is keyed by
    // BoardSectionTokens and always carries all six entries.
    type Board struct {
        SectionOrder Value[[]string]      `json:"section_order"`
        Sorting      map[string]BoardSort `json:"sorting"`
    }
    type BoardSort struct {
        By        Value[string] `json:"by"`        // id | updated | created
        Direction Value[string] `json:"direction"` // asc | desc
    }
    ```

- [ ] **Step 1: Write the failing resolution tests** in `internal/config/resolve_test.go`,
  following the file's existing table style (build `[]Source` layers inline, call
  `config.Resolve`, assert on the snapshot). Cover:

```go
func TestResolveBoardDefaults(t *testing.T) {
    snap := mustResolve(t, nil) // however the file's helpers spell "no layers"
    b := snap.Effective.Board
    if got := b.SectionOrder.Value; !slices.Equal(got, config.BoardSectionTokens) {
        t.Fatalf("default section_order = %v", got)
    }
    if b.SectionOrder.Explicit || b.SectionOrder.Provenance.Layer != config.LayerBuiltIn {
        t.Fatalf("default section_order must be non-explicit built-in")
    }
    for _, s := range config.BoardSectionTokens {
        srt, ok := b.Sorting[s]
        if !ok {
            t.Fatalf("missing default sorting for %s", s)
        }
        if srt.By.Value != "updated" || srt.Direction.Value != "desc" {
            t.Fatalf("%s default sort = %s %s", s, srt.By.Value, srt.Direction.Value)
        }
    }
}

func TestResolveBoardLayeringAndPerLeafInheritance(t *testing.T) {
    // global sets built: {by: id, direction: asc}; repository-local overrides
    // ONLY built.direction: desc. Expect built.by=id (global provenance),
    // built.direction=desc (repository-local provenance), every sibling
    // section still updated/desc built-in, and Explicit=true only where a
    // layer supplied the leaf.
}

func TestResolveBoardSectionOrderWholeListReplacement(t *testing.T) {
    // global declares a full valid permutation; repository declares a
    // different full valid permutation → repository wins wholesale, with
    // repository provenance.
}
```

  Also extend the defaults mirror test in `defaults_test.go` (it proves registry `def` cells and
  `builtinEffective` cannot drift) so it walks the new rows.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run 'TestResolveBoard' -count=1`
Expected: FAIL (compile error: `Board` undefined / fields missing).

- [ ] **Step 3: Implement.**
  - `config.go`: add the `Board`/`BoardSort` types and the `Board` field on `Effective` exactly
    as in the Interfaces block.
  - `schema.go`: add the exported token slices, then in `buildRegistry` append after the
    `board_surfaces` row (row-numbering comments in the file: update the affected `// N:` labels
    consistently):

```go
// board.section_order — the rendered-board section permutation. Shape-only
// validation lives here (a list of non-empty strings); the permutation rule
// (every token exactly once) is the decode stage's warn-and-ignore surface,
// which a leafValidator cannot express (see boardSectionOrderPermutation).
{path: "board.section_order", kind: kindStringList,
    def:   append([]string(nil), BoardSectionTokens...),
    merge: mergeListReplace, scope: scopeAny, disp: dispSupported,
    validate: listLeaf(listOpts{})},
```

  and, in a loop over `BoardSectionTokens`, one `by` and one `direction` row per section:

```go
for _, s := range BoardSectionTokens {
    rows = append(rows,
        pathSpec{path: "board.sorting." + s + ".by", kind: kindString,
            enum: BoardSortFields, def: "updated",
            merge: mergeScalar, scope: scopeAny, disp: dispSupported,
            validate: enumLeaf(BoardSortFields...)},
        pathSpec{path: "board.sorting." + s + ".direction", kind: kindString,
            enum: BoardSortDirections, def: "desc",
            merge: mergeScalar, scope: scopeAny, disp: dispSupported,
            validate: enumLeaf(BoardSortDirections...)},
    )
}
```

  (Adapt to `buildRegistry`'s literal-slice style — build the slice in a variable first if
  needed. `enumLeaf` is variadic; `enumLeaf(BoardSortFields...)` works as-is.)
  - `defaults.go`: in `builtinEffective`, populate `Board`:

```go
Board: Board{
    SectionOrder: builtinValue(append([]string(nil), BoardSectionTokens...)),
    Sorting:      builtinBoardSorting(),
},
```

```go
func builtinBoardSorting() map[string]BoardSort {
    m := make(map[string]BoardSort, len(BoardSectionTokens))
    for _, s := range BoardSectionTokens {
        m[s] = BoardSort{By: builtinValue("updated"), Direction: builtinValue("desc")}
    }
    return m
}
```

  - `resolve.go`: in `assemble`, after the `board_surfaces` assign:

```go
set(assign(&eff.Board.SectionOrder, r.declared, "board.section_order"))
for _, s := range BoardSectionTokens {
    srt := eff.Board.Sorting[s]
    set(assign(&srt.By, r.declared, "board.sorting."+s+".by"))
    set(assign(&srt.Direction, r.declared, "board.sorting."+s+".direction"))
    eff.Board.Sorting[s] = srt
}
```

- [ ] **Step 4: Run the package tests.**

Run: `go test ./internal/config/ -count=1`
Expected: the three new tests PASS. Existing exhaustive registry-walk tests (`schema_test.go`,
`decode_test.go`, `capability_test.go`, `fixtures_test.go`) may now redden because they
enumerate registry rows or classify every path — extend their expectations for the 13 new
supported rows (that is in-scope for this task; the rows are `dispSupported`, so classification
expectations are the same as e.g. `change_types`). If `example_correspondence_test.go` reddens
because the example file lacks the new keys, leave it red only if Task 3 is running next in
sequence is impossible — otherwise fold the minimal example-file addition forward into Task 3 by
adding a temporary `t.Skip` **never**; instead do the example edit here minimally (uncommented
or commented per that test's rule) and let Task 3 finish the documentation. The task's suite
exit state must be green.

- [ ] **Step 5: Mutation probe (guard-of-record: the defaults mirror).** Temporarily change one
  registry `def` (e.g. `board.sorting.built.direction` to `"asc"`) and run
  `go test ./internal/config/ -run Defaults -count=1` — it must redden. Restore.

- [ ] **Step 6: Commit** — `feat(0367): add the board presentation block to the config schema`

---

### Task 2: Config decode — warn-and-ignore with layer inheritance for `board.*`

**Files:**
- Modify: `internal/config/decode.go`
- Test: `internal/config/decode_test.go`, `internal/config/resolve_test.go`

**Interfaces:**
- Consumes: Task 1's registry rows and `BoardSectionTokens`; decode's existing warn machinery
  (`keepKnownSurfaces`, `pathMatch{warn: true}`, warning-severity diags with
  `CodeInvalidValue`/`CodeUnknownKey`).
- Produces: the resolution-visible behavior later tasks rely on — an invalid `board.*`
  declaration yields a **warning** diagnostic and **no** `leafDecl` for that path from that
  layer, so `resolution.declared` keeps the highest *valid* layer's declaration and the snapshot
  stays valid.

Semantics to implement (the spec's failure contract, all warn-and-drop, never error):
1. `board.section_order` whose value fails shape validation, or whose token list is not a
   permutation of `BoardSectionTokens` (missing, unknown, or duplicate token) → one warning
   naming the offense; the whole list is dropped as one value.
2. `board.sorting.<section>.by` / `.direction` with a wrong type or a value outside the enum →
   warning; only that leaf dropped.
3. `board.sorting.<unknown-section>` → warning (`CodeUnknownKey`, warning severity — the same
   deliberate warn-and-ignore family as an unknown skills role / unknown board-surface token);
   do not descend.
4. Any other unknown key under `board.` (e.g. `board.foo`) → the standard strict unknown-key
   **error** (typo policy unchanged outside the two leaf families).
5. A warning must not suppress sibling valid board leaves (drop granularity: exactly the leaf,
   or exactly the one list value).

- [ ] **Step 1: Write the failing tests.** In `decode_test.go` / `resolve_test.go` (follow where
  the existing warn-surface tests for `board_surfaces` unknown tokens live), table-driven:

```go
func TestResolveBoardInvalidSectionOrderFallsBack(t *testing.T) {
    cases := []struct{ name, yaml string }{
        {"missing token", "board:\n  section_order: [in-progress, built, blocked, groomed, proposed]\n"},
        {"unknown token", "board:\n  section_order: [in-progress, built, blocked, groomed, proposed, bogus]\n"},
        {"duplicate token", "board:\n  section_order: [in-progress, built, blocked, groomed, proposed, proposed]\n"},
        {"not a list", "board:\n  section_order: everything\n"},
    }
    // For each: repository layer declares a VALID full permutation, the
    // repository-local layer declares the invalid value. Assert:
    //  - Resolve returns a valid snapshot (err == nil),
    //  - exactly one warning-severity diagnostic with Path "board.section_order"
    //    attributed to .docket.local.yml,
    //  - Effective.Board.SectionOrder equals the repository layer's valid list
    //    with repository provenance (the lower valid layer won).
}

func TestResolveBoardInvalidSortLeafInheritsOnlyThatLeaf(t *testing.T) {
    // global: board.sorting.built: {by: id, direction: asc}
    // repo-local: board.sorting.built: {by: priority, direction: desc}
    // Assert: by == "id" (global provenance, the invalid higher leaf fell
    // through), direction == "desc" (repo-local), one warning for
    // board.sorting.built.by, snapshot valid, and sibling sections untouched.
}

func TestResolveBoardUnknownSortingSectionWarns(t *testing.T) {
    // board.sorting.bogus: {by: id} → warning severity CodeUnknownKey,
    // snapshot valid, no effective change.
}

func TestResolveBoardUnknownBoardKeyIsError(t *testing.T) {
    // board.foo: 1 → standard error path, ErrInvalidConfig.
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run 'TestResolveBoard(Invalid|Unknown)' -count=1`
Expected: FAIL — today an out-of-enum value is an error-severity `CodeInvalidValue` and
invalidates the snapshot.

- [ ] **Step 3: Implement in `decode.go`.** Anchor on the file's existing special cases (the
  `case "board_surfaces":` branch and `keepKnownSurfaces`). Mechanism:
  - In the leaf-decode path, when the matched row's path is `board.section_order` or matches
    `board.sorting.<known-section>.{by,direction}`, run the row's validator as usual; if it
    returns diagnostics, downgrade each to `SeverityWarning` and **return no leaf** instead of
    recording the diagnostics as errors.
  - For `board.section_order`, after a successful shape validation, run the permutation check:

```go
// boardSectionOrderPermutation reports whether tokens is a complete
// permutation of BoardSectionTokens, and names the first offense when not.
func boardSectionOrderPermutation(tokens []string) (offense string, ok bool) {
    seen := make(map[string]bool, len(BoardSectionTokens))
    for _, tok := range tokens {
        if !inList(BoardSectionTokens, tok) {
            return fmt.Sprintf("names unknown section %q", tok), false
        }
        if seen[tok] {
            return fmt.Sprintf("lists %q more than once", tok), false
        }
        seen[tok] = true
    }
    for _, want := range BoardSectionTokens {
        if !seen[want] {
            return fmt.Sprintf("is missing section %q", want), false
        }
    }
    return "", true
}
```

    On failure, emit one warning (`CodeInvalidValue`, warning severity, Path
    `board.section_order`, message ending with the spec's contract, e.g. `"…; the whole list is
    ignored and a lower layer or the built-in order applies"`) and drop the decl.
  - In `matchPath` (or its equivalent), make `board.sorting.<not-a-known-section>` return the
    warn-shaped match (`pathMatch{warn: true}`) the way an unknown skills role does; leave
    every other unknown `board.*` key on the strict error path.
- Watch the drop granularity: a dropped `board.sorting.built.by` must not take
  `board.sorting.built.direction` with it (decode leaves are per-leaf already; keep it so).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/config/ -count=1`
Expected: PASS, including Task 1's tests (unchanged behavior for valid values).

- [ ] **Step 5: Mutation probes.**
  (a) **Section-order completeness check** (a spec-named mutation): comment out the
  missing-token loop inside `boardSectionOrderPermutation` (the `for _, want := range …` block),
  run `go test ./internal/config/ -run TestResolveBoardInvalidSectionOrderFallsBack -count=1` —
  the "missing token" case must redden. Restore.
  (b) Change the downgrade so the dropped leaf is still recorded — the inheritance asserts in
  `TestResolveBoardInvalidSortLeafInheritsOnlyThatLeaf` must redden. Restore.

- [ ] **Step 6: Commit** — `feat(0367): warn-and-inherit validation for board presentation config`

---

### Task 3: Config inspection, canonical example, and correspondence guards

**Files:**
- Modify: `internal/app/config.go` (`effectiveLines`)
- Modify: `.docket.example.yml`
- Modify: `internal/assets/embedded/tree/.docket.example.yml`
- Test: `internal/app/config_test.go` (or wherever `effectiveLines`/`HumanText` is tested),
  `internal/config/example_correspondence_test.go`

**Interfaces:**
- Consumes: Task 1's `Effective.Board`, `config.BoardSectionTokens`.
- Produces: `docket config` inspection output containing one line per new leaf, in canonical
  order, each with layer attribution — e.g.
  `board.section_order = [in-progress, built, blocked, groomed, proposed, deferred] (built-in)`
  and `board.sorting.in-progress.by = updated (built-in)` … (12 sorting lines).

- [ ] **Step 1: Write/extend the failing tests.** Extend the existing inspection test that
  asserts `effectiveLines` output to expect the 13 new lines, positioned right after
  `board_surfaces` and iterating sections in `config.BoardSectionTokens` order (the `Sorting`
  map must never be ranged directly — map order is random).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'Config' -count=1`
Expected: FAIL (missing lines).

- [ ] **Step 3: Implement `effectiveLines`.** After the `board_surfaces` leafLine:

```go
lines = append(lines, leafLine("board.section_order",
    listValue(eff.Board.SectionOrder.Value), eff.Board.SectionOrder.Provenance))
for _, s := range config.BoardSectionTokens {
    srt := eff.Board.Sorting[s]
    lines = append(lines,
        leafLine("board.sorting."+s+".by", textValue(srt.By.Value), srt.By.Provenance),
        leafLine("board.sorting."+s+".direction", textValue(srt.Direction.Value), srt.Direction.Provenance))
}
```

  (Adapt to the function's actual list-building shape — it returns a literal slice today; convert
  minimally.)

- [ ] **Step 4: Update the canonical example** in `.docket.example.yml`, near the existing
  `board_surfaces` block, written for a reader who has never heard of change 0367:

```yaml
# board — how the rendered inline board presents the backlog. Presentation only:
# lifecycle data, the digest, autonomous selection, and the GitHub mirror never
# read these. Settable at every layer (global config.yml, .docket.yml,
# .docket.local.yml); higher layers override per leaf, and section_order
# replaces as one whole list.
#
# section_order must list every section exactly once: a list with a missing,
# unknown, or duplicate token is warned about and ignored as one value, letting
# a lower layer or the built-in order apply.
# sorting.<section>.by is one of id | updated | created; .direction is asc | desc.
board:
  section_order: [in-progress, built, blocked, groomed, proposed, deferred]
  sorting:
    in-progress: {by: updated, direction: desc}
    built: {by: updated, direction: desc}
    blocked: {by: updated, direction: desc}
    groomed: {by: updated, direction: desc}
    proposed: {by: updated, direction: desc}
    deferred: {by: updated, direction: desc}
```

  Follow the example file's commented-vs-uncommented house convention for defaults (read
  `example_correspondence_test.go` first — it is the authority on what shape it requires; note a
  commented-out default documents but does not exercise the parser, and the correspondence test
  decides which is required). Mirror the exact same edit into
  `internal/assets/embedded/tree/.docket.example.yml`.

- [ ] **Step 5: Run the guards**

Run: `go test ./internal/config/ ./internal/app/ -count=1`
Expected: PASS, including `TestExampleSchemaCorrespondence` and any embedded-tree drift assert.
If a frozen-fixture drift guard reddens (e.g. under `testdata/repositories/`), obey **the
guard's own remedy message** (typically: mint the new versioned fixture tree and re-derive),
never an ad-hoc patch.

- [ ] **Step 6: Mutation probe.** Delete the `board.section_order` line from
  `.docket.example.yml` only; the correspondence guard must redden
  (`go test ./internal/config/ -run Example -count=1`). Restore.

- [ ] **Step 7: Commit** — `feat(0367): expose board presentation in inspection and the canonical example`

---

### Task 4: Renderer — presentation types and section classification

**Files:**
- Modify: `internal/render/board.go`
- Test: `internal/render/board_test.go`

**Interfaces:**
- Consumes: `domain.EvaluateReadiness(snap, c, facts)`, `Change.HasFinalizeBlocked()`,
  `Change.Status()`, `Change.Spec()`, `Change.Trivial()`; the test file's existing fixture
  helpers (`boardFromFacts`, the corpus loaders).
- Produces (later tasks depend on these exact names):

```go
type BoardSection string

const (
    BoardSectionInProgress BoardSection = "in-progress"
    BoardSectionBuilt      BoardSection = "built"
    BoardSectionBlocked    BoardSection = "blocked"
    BoardSectionGroomed    BoardSection = "groomed"
    BoardSectionProposed   BoardSection = "proposed"
    BoardSectionDeferred   BoardSection = "deferred"
)

// boardSectionOrderDefault is the built-in permutation, in display order.
var boardSectionOrderDefault = []BoardSection{
    BoardSectionInProgress, BoardSectionBuilt, BoardSectionBlocked,
    BoardSectionGroomed, BoardSectionProposed, BoardSectionDeferred,
}

type BoardSortKey string       // "id" | "updated" | "created"
type BoardDirection string     // "asc" | "desc"

type BoardSort struct {
    By        BoardSortKey
    Direction BoardDirection
}

// BoardPresentation is the typed presentation policy the resolved config
// produces. Board() refuses an incomplete or invalid presentation — the
// renderer never fills defaults, so a caller that forgot to build options
// fails loudly instead of silently rendering the built-in view.
type BoardPresentation struct {
    SectionOrder []BoardSection
    Sorting      map[BoardSection]BoardSort
}

// DefaultBoardPresentation returns the built-in presentation: the default
// permutation, updated desc everywhere.
func DefaultBoardPresentation() BoardPresentation

// boardClassify maps one ACTIVE change to exactly one rendered section, per
// the spec's precedence: Blocked → In progress → Built → Groomed → Proposed →
// Deferred. Pure; mutates nothing.
func boardClassify(in BoardInput, c domain.Change) (BoardSection, error)
```

  `BoardInput` gains `Presentation BoardPresentation` (do NOT rewire `Board()` yet — that is
  Task 6; this task adds types + classification + tests that call `boardClassify` directly from
  the same package's test file, or via exported behavior if the test file is external —
  **check the test file's package clause first** (`package render` vs `render_test`); if it is
  external, either add a tiny `export_test.go` with `var BoardClassifyForTest = boardClassify`
  or write the tests in a new internal-package test file).

Classification rules (implement exactly; each is a spec conjunct):
1. `StatusBlocked`, or `StatusImplemented && c.HasFinalizeBlocked()` → `BoardSectionBlocked`.
2. `StatusInProgress` → `BoardSectionInProgress`.
3. remaining `StatusImplemented`, plus `StatusStackedMerged` → `BoardSectionBuilt`.
4. `StatusProposed` with `c.Spec().Value != ""` **and**
   `domain.EvaluateReadiness(in.Snapshot, c, in.Facts).Kind == domain.ReadyBuildReady` →
   `BoardSectionGroomed`.
5. every other `StatusProposed` → `BoardSectionProposed` (needs-brainstorm, auto-groom-blocked,
   waiting-dependency, stack-base-unresolved, spec-bearing-not-ready, and trivial build-ready
   with an empty spec — the groomed label means build-ready **and spec-backed**).
6. `StatusDeferred` → `BoardSectionDeferred`.
7. any other status → error (`"render: board: change %04d has non-active status %q"`), the
   existing fail-closed posture.

- [ ] **Step 1: Write the failing tests** — one focused test per bucket plus the two precedence
  edges, using the file's existing change-fixture builders:

```go
func TestBoardClassifyEveryBucket(t *testing.T) {
    // table: (fixture change, facts) → expected BoardSection, covering:
    //  blocked lifecycle → blocked
    //  implemented + "## Finalize blocked" section → blocked  (precedence over built)
    //  implemented healthy → built
    //  stacked-merged → built
    //  in-progress → in-progress
    //  proposed + spec + all readiness conjuncts met → groomed
    //  proposed + spec + unmet dependency → proposed
    //  proposed + spec + unresolved stack base (zero Facts) → proposed
    //  proposed + trivial, no spec (build-ready) → proposed
    //  proposed needs-brainstorm → proposed
    //  proposed auto-groom-blocked → proposed
    //  deferred → deferred
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/render/ -run TestBoardClassify -count=1`
Expected: FAIL (compile: `boardClassify` undefined).

- [ ] **Step 3: Implement** the types and `boardClassify` in `board.go` (a plain switch on
  `c.Status()` with the finalize-blocked check before the implemented→built arm and the
  spec+readiness conjunction for groomed).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/render/ -run TestBoardClassify -count=1` → PASS.

- [ ] **Step 5: Mutation probes (both spec-named).**
  (a) Remove the `c.Spec().Value != ""` conjunct from the groomed arm → the
  trivial-build-ready-no-spec case must redden. Restore.
  (b) Invert the finalize-blocked precedence (check `built` before the finalize-blocked test) →
  the implemented+finalize-blocked case must redden. Restore. Run with `-count=1` both times.

- [ ] **Step 6: Commit** — `feat(0367): board presentation types and section classification`

---

### Task 5: Renderer — the per-section comparator

**Files:**
- Modify: `internal/render/board.go`
- Test: `internal/render/board_test.go`

**Interfaces:**
- Consumes: Task 4's `BoardSort`, `BoardSortKey`, `BoardDirection`; `Change.ID()`,
  `Change.Updated()`, `Change.Created()` (`domain.OptionalTime` — a date is usable iff
  `.State == domain.FieldPresent`; compare the parsed time value).
- Produces:

```go
// sortBoardSection orders rows in place per s. The comparator is total and
// deterministic: primary key per s.By/s.Direction; equal dates tie-break on
// numeric ID in the SAME direction; rows with a missing/empty/malformed date
// sort after every valid date regardless of direction, ordered among
// themselves by ID in the configured direction. Arrival order never decides.
func sortBoardSection(rows []domain.Change, s BoardSort)
```

- [ ] **Step 1: Write the failing tests** — a table over all three fields × both directions,
  plus the tie and unknown-date bands:

```go
func TestSortBoardSectionAllFieldsBothDirections(t *testing.T) {
    // Fixtures: ids 1..5 with updated/created dates chosen so id-order,
    // updated-order and created-order are three DIFFERENT permutations
    // (otherwise a wrong key silently passes). Assert exact id sequences for
    // {id,updated,created} × {asc,desc}.
}

func TestSortBoardSectionSameDateTiesFollowDirection(t *testing.T) {
    // Two rows share updated: 2026-08-30, a third differs.
    // desc → tie renders higher id first; asc → lower id first.
}

func TestSortBoardSectionUnknownDatesSortLast(t *testing.T) {
    // Rows with absent and malformed updated: land after all dated rows in
    // BOTH directions; among themselves ordered by id in the configured
    // direction. (A malformed date parses to State != FieldPresent — build the
    // fixture through the same frontmatter path the corpus uses.)
}

func TestSortBoardSectionIgnoresArrivalOrder(t *testing.T) {
    // Shuffle the input slice deterministically (e.g. reversed and rotated
    // copies); output identical for each input ordering.
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./internal/render/ -run TestSortBoardSection -count=1` → FAIL.

- [ ] **Step 3: Implement.** Shape:

```go
func sortBoardSection(rows []domain.Change, s BoardSort) {
    desc := s.Direction == BoardDirectionDesc // add the two direction consts
    idLess := func(a, b domain.Change) bool {
        if desc {
            return a.ID() > b.ID()
        }
        return a.ID() < b.ID()
    }
    sort.SliceStable(rows, func(i, j int) bool {
        a, b := rows[i], rows[j]
        if s.By == BoardSortKeyID {
            return idLess(a, b)
        }
        at, aok := boardSortDate(a, s.By)
        bt, bok := boardSortDate(b, s.By)
        switch {
        case aok && !bok:
            return true // valid dates before unknown, regardless of direction
        case !aok && bok:
            return false
        case !aok && !bok:
            return idLess(a, b)
        case at.Equal(bt):
            return idLess(a, b)
        case desc:
            return at.After(bt)
        default:
            return at.Before(bt)
        }
    })
}

// boardSortDate extracts the change's date for key (updated or created);
// ok is false for an absent/unparsed field.
func boardSortDate(c domain.Change, key BoardSortKey) (time.Time, bool)
```

  (Adjust to `OptionalTime`'s actual field names — read `internal/domain/entities.go` around the
  `OptionalTime` declaration.)

- [ ] **Step 4: Run to verify pass** — `go test ./internal/render/ -run TestSortBoardSection -count=1` → PASS.

- [ ] **Step 5: Mutation probe (spec-named: the date tie-breaker).** Replace the
  `at.Equal(bt)` arm with `return false` and rerun `-count=1`: the tie test must redden.
  Restore. Also flip the unknown-date arm (`aok && !bok → false`): the unknown-last test must
  redden. Restore.

- [ ] **Step 6: Commit** — `feat(0367): total deterministic board section comparator`

---

### Task 6: Renderer — presentation-driven Board(): sections, counts, unified tables, golden

This is the pivot task: `Board()` stops iterating lifecycle statuses and starts iterating the
configured section permutation. The default output deliberately changes, so the frozen golden is
re-derived and its provenance note rewritten.

**Files:**
- Modify: `internal/render/board.go` (emission + the header contract comment)
- Modify: `internal/render/board_test.go`
- Modify: `internal/render/testdata/board/board.golden`, `internal/render/testdata/board/PROVENANCE.md`

**Interfaces:**
- Consumes: Tasks 4–5 (`boardClassify`, `sortBoardSection`, `BoardPresentation`,
  `DefaultBoardPresentation`); the untouched helpers (`boardPRCell`, `boardSpecLink`,
  `boardTypeCell`, `boardReadinessCell`, `boardDepReason`, `boardStackCell`, archive emission,
  mermaid emission).
- Produces: `Board(in BoardInput) ([]byte, error)` where `in.Presentation` is required:

```go
// Board refuses an invalid presentation: SectionOrder must be a complete
// permutation of the six sections and Sorting must carry a valid sort for all
// six. Config owns user-facing fallback; an invalid presentation HERE is a
// docket wiring bug, reported as an error.
```

Rendering contract to implement (and to rewrite the file-header comment to — the old header
describes the retired script byte-authority and the lifecycle-status section order; replace that
prose with this contract, keeping the archive/mermaid clauses):

1. `# Backlog\n\n` unchanged.
2. Counts line: `**<total> changes** — <seg>` where `<seg>` now iterates the **configured
   section order** over the six rendered groups (count = classified membership), then the
   terminal `done`/`killed` archive counts, skipping zero counts. Labels/emoji:
   - in-progress → `🟢 <n> in progress`
   - built → `🔵 <n> built`
   - blocked → `🔴 <n> blocked`
   - groomed → `🟣 <n> groomed`
   - proposed → `🟡 <n> proposed`
   - deferred → `⚪ <n> deferred`
   - done → `✅ <n> done`, killed → `🗑️ <n> killed` (unchanged).
3. Active sections in configured order, each emitted only when non-empty, heading
   `\n## <emoji> <Title> (<n>)\n\n` with titles `In progress`, `Built`, `Blocked`, `Groomed`,
   `Proposed`, `Deferred` and the emoji above. No `— awaiting merge` heading suffix anywhere —
   the Built rows' State column carries it. Rows sorted by `sortBoardSection` with that
   section's `BoardSort`.
4. Tables:
   - In progress: `| # | Title | Priority | Type | Spec | Branch |` (existing row shape).
   - Built: `| # | Title | Priority | Type | PR | State |` — implemented row State =
     `awaiting merge`; stacked-merged row State = `merged into #NNNN` (zero-padded parent, `—`
     when no usable parent edge; reuse `boardStackCell`).
   - Blocked: `| # | Title | Priority | Type | PR | Reason |` — lifecycle-blocked row: PR cell
     from `boardPRCell` (empty when no pr:), Reason = the stored `blocked_by:` text;
     finalize-blocked implemented row: PR cell + `finalize blocked — needs you`.
   - Groomed: `| # | Title | Priority | Type | Spec |` — Spec cell `[spec](<boardSpecLink>)`
     like In progress.
   - Proposed: `| # | Title | Priority | Type | Readiness |` — existing readiness cells, except
     a build-ready row (necessarily trivial-without-spec here, since spec-backed build-ready is
     Groomed) renders `build-ready (trivial)`.
   - Deferred: `| # | Title | Priority | Type |` (unchanged shape).
5. Mermaid block and archive footer: byte-identical logic to today, emitted after the active
   tables, outside section order/sorting.

- [ ] **Step 1: Write the failing tests** (before touching `Board()`):

```go
func TestBoardRefusesInvalidPresentation(t *testing.T) {
    // zero Presentation, 5-token order, duplicate token, missing sorting
    // entry, bad sort key → error mentioning "presentation".
}

func TestBoardConfiguredSectionOrderAndOmission(t *testing.T) {
    // Fixture with members in only 3 of 6 groups; a reversed section_order.
    // Assert headings appear in the configured order, absent groups emit no
    // heading, and the counts line lists the same groups in the same order.
}

func TestBoardCountSummaryParity(t *testing.T) {
    // For a fixture covering all six groups + archive: every counts-line
    // segment's n equals the row count of the matching section table
    // (parse the rendered output; done/killed match archive fixtures).
}

func TestBoardBuiltAndBlockedUnifiedCells(t *testing.T) {
    // implemented healthy → Built row "awaiting merge" with its PR link;
    // stacked-merged → Built row "merged into #0NNN";
    // lifecycle-blocked (with and without pr:) → Blocked row reason text,
    // empty PR cell when none; implemented finalize-blocked → Blocked row
    // "finalize blocked — needs you".
}

func TestBoardProposedTrivialBuildReadyCell(t *testing.T) {
    // trivial build-ready row renders "build-ready (trivial)" under Proposed.
}
```

  Update every existing `render.Board(render.BoardInput{…})` call in the test file to pass
  `Presentation: render.DefaultBoardPresentation()` (the `boardFromFacts` helper centralizes
  most of them).

- [ ] **Step 2: Run to verify failure** — `go test ./internal/render/ -run TestBoard -count=1` → FAIL.

- [ ] **Step 3: Implement.** Restructure `Board()`:
  - validate `in.Presentation` first (permutation + complete valid sorting map; a small
    `func (p BoardPresentation) validate() error`);
  - classify every active change via `boardClassify` into `map[BoardSection][]domain.Change`;
  - counts line from the classified groups in `in.Presentation.SectionOrder`, then terminal
    archive counts (keep `boardTerminalStatuses`);
  - per-section emission: `sortBoardSection(rows, in.Presentation.Sorting[s])`, new
    section-keyed helpers `boardSectionEmoji(BoardSection)`, `boardSectionHeading(BoardSection)`,
    `boardSectionTableHeader(BoardSection)`, and a section-keyed `boardSectionRow(in, s, c)`
    that reuses the existing cell helpers (delete or absorb the now-dead status-keyed
    `boardTableHeader`/`boardRow`/`boardSectionTitle`/`boardEmoji` variants for ACTIVE statuses
    — keep whatever the counts/archive still need for done/killed);
  - keep `boardReadinessCell` for Proposed rows, adding the trivial decoration:

```go
if r.Kind == domain.ReadyBuildReady && c.Trivial() {
    return "build-ready (trivial)", nil
}
```

  - mermaid + archive emission stay as-is (do not thread presentation into them).
  - Rewrite the file-header contract comment to the contract above (the old script-authority
    prose is now false; cite change 0367 and keep the golden's role as drift guard).

- [ ] **Step 4: Re-derive the golden.** `TestBoardGolden` now fails because the default
  presentation changed by design. Regenerate:
  1. Add (or reuse) a temporary regeneration path — simplest: run the renderer over the corpus
     from a throwaway test that writes the bytes, e.g.
     `go test ./internal/render/ -run TestBoardGolden -count=1` after temporarily making the
     test write `got` to the golden path when `-update`-style env `BOARD_GOLDEN_UPDATE=1` is
     set; remove the hook after use, or write a small `go run` snippet — either way the
     committed state must contain **no** regeneration backdoor.
  2. **Review the diff by hand before accepting it** (`git diff internal/render/testdata/board/board.golden`):
     verify section order, headings, sorted row order (updated desc with id-desc ties), unified
     Built/Blocked tables, counts line, untouched mermaid + archive bytes. The golden is being
     re-founded, not mechanically refreshed — a renderer bug accepted into the golden is
     invisible forever after.
  3. Update `internal/render/testdata/board/PROVENANCE.md`: the golden is no longer the frozen
     Bash-script bytes; as of change 0367 it is the reviewed canonical render of the corpus
     under the DEFAULT presentation, and its role remains the byte-drift guard.

- [ ] **Step 5: Run the package** — `go test ./internal/render/ -count=1` → PASS, including
  `TestBoardGolden` and `TestBoardDeterministic` against the new bytes. Fix any test that
  asserted the old lifecycle sections by re-deriving its expectation from the spec (ask what
  each old test GUARDS: id-sort asserts become configured-sort asserts; section-title asserts
  become classification asserts — do not delete coverage, re-target it).

- [ ] **Step 6: Mutation probes.**
  (a) Make `Board()` ignore `in.Presentation.SectionOrder` and iterate
  `boardSectionOrderDefault` → `TestBoardConfiguredSectionOrderAndOmission` must redden.
  (b) Skip the `validate()` call → `TestBoardRefusesInvalidPresentation` must redden.
  Restore both; `-count=1` on every probe.

- [ ] **Step 7: Commit** — `feat(0367): presentation-driven board sections, counts, and unified tables`

---

### Task 7: Renderer — archive same-day regression, byte-stable repeat render, fixed footer

**Files:**
- Modify: `internal/render/board_test.go` (tests only; `board.go` only if a test exposes a defect)

**Interfaces:**
- Consumes: Task 6's `Board()`; the archive fixture builders in the test file.

- [ ] **Step 1: Write the tests** (these guard behavior that exists but was never pinned — the
  spec demands an explicit fixture):

```go
func TestBoardArchiveSameDayRowsSortIDDescending(t *testing.T) {
    // Three archived done changes sharing one YYYY-MM-DD filename date, plus
    // one on an earlier date. Assert the rendered archive rows appear
    // date-descending and, within the shared day, id-descending — and that
    // this holds under a NON-default presentation (e.g. everything id asc):
    // the archive is outside board.sorting.
}

func TestBoardArchiveIsAFixedFooter(t *testing.T) {
    // Render with a reversed section_order; assert the "<details>" archive
    // block still appears after the mermaid fence, i.e. last.
}

func TestBoardRepeatRenderByteStable(t *testing.T) {
    // Render the full corpus fixture twice with the same non-default
    // presentation; require bytes.Equal. (TestBoardDeterministic may already
    // cover the default path — this adds the non-default presentation band.)
}
```

- [ ] **Step 2: Run** — `go test ./internal/render/ -run 'TestBoardArchive|TestBoardRepeat' -count=1`.
Expected: PASS if Task 6 is correct; investigate any failure as a Task 6 defect (fix in
`board.go`, not by weakening the assert).

- [ ] **Step 3: Mutation probe.** In the archive comparator (`sort.SliceStable` over `arcRow`),
  drop the `rows[i].id > rows[j].id` tie-break (return `false`) → the same-day test must
  redden. Then thread the section sort into the archive (call `sortBoardSection` on the archive
  rows) → the fixed-footer/same-day asserts must redden. Restore both.

- [ ] **Step 4: Commit** — `test(0367): archive same-day ordering, fixed footer, and byte-stable render guards`

---

### Task 8: App wiring — one option-building path, threaded to every board render

**Files:**
- Modify: `internal/app/derived_views.go`
- Modify: every `includeBoard`/`renderCanonicalBoard` call site — derive the list with
  `grep -rn "includeBoard(\|renderCanonicalBoard(" internal/app --include='*.go'` (at writing:
  change_attach, change_claim, change_create, change_kill, change_groom, change_implemented,
  change_reclaim, change_lifecycle, change_repair, change_reconcile, finalize_block,
  repository_check `derivedViewFindings`, repository_migrate_repair
  `composeDerivedRepairBytes`); every site already holds a `config.Effective` (`eff`,
  `pin.Config.Effective`, `cfg`, or `sc.cfg`).
- Test: `internal/app/derived_views_test.go` (plus whichever op test exercises a board write).

**Interfaces:**
- Consumes: Task 1's `config.Effective.Board`; Task 6's `render.BoardPresentation` /
  `render.Board`.
- Produces:

```go
// boardPresentation is the ONE path from resolved configuration to renderer
// options. Config has already validated and defaulted every leaf, so this is
// a pure type lift — it invents no defaults and can only mistranslate, which
// the correspondence test below pins.
func boardPresentation(eff config.Effective) render.BoardPresentation

func renderCanonicalBoard(snap domain.Snapshot, pres render.BoardPresentation) ([]byte, error)

func includeBoard(ctx context.Context, tree transaction.Tree, boardPath string,
    candidate domain.Snapshot, pres render.BoardPresentation,
    files *[]transaction.FileMutation) error
```

- [ ] **Step 1: Write the failing tests:**

```go
func TestBoardPresentationLiftsResolvedConfig(t *testing.T) {
    // Resolve a config whose repository layer sets a NON-default value on
    // every kind of leaf: a permuted section_order and e.g.
    // board.sorting.proposed: {by: id, direction: asc}. Assert the returned
    // render.BoardPresentation carries the permuted order and id/asc for
    // proposed, updated/desc for the untouched sections. Asserting the
    // RESOLVED NON-DEFAULT value is load-bearing: a defaulted-through value
    // would stay green if the wiring were deleted.
}

func TestBoardRenderHonorsConfiguredPresentation(t *testing.T) {
    // Through renderCanonicalBoard with the lifted presentation above (a
    // small fixture snapshot): assert the rendered bytes DIFFER from the
    // default-presentation render and show the permuted first heading.
}
```

- [ ] **Step 2: Run** — `go test ./internal/app/ -run TestBoardPresentation -count=1` → FAIL.

- [ ] **Step 3: Implement.**

```go
func boardPresentation(eff config.Effective) render.BoardPresentation {
    order := make([]render.BoardSection, 0, len(eff.Board.SectionOrder.Value))
    for _, s := range eff.Board.SectionOrder.Value {
        order = append(order, render.BoardSection(s))
    }
    sorting := make(map[render.BoardSection]render.BoardSort, len(eff.Board.Sorting))
    for s, srt := range eff.Board.Sorting {
        sorting[render.BoardSection(s)] = render.BoardSort{
            By:        render.BoardSortKey(srt.By.Value),
            Direction: render.BoardDirection(srt.Direction.Value),
        }
    }
    return render.BoardPresentation{SectionOrder: order, Sorting: sorting}
}
```

  Thread `pres` through `renderCanonicalBoard` and `includeBoard`, and at each call site build
  it via `boardPresentation(<the site's effective config>)` — never inline
  `render.DefaultBoardPresentation()` in app code (the renderer's refusal plus the tests keep
  this honest). Keep the "only call site of render.Board in internal/app" comment truthful.

- [ ] **Step 4: Run the package** — `go test ./internal/app/ -count=1` → PASS (existing op tests
  render through the default-config effective and keep passing; any op-test golden that pinned
  old board bytes gets re-derived exactly as in Task 6 Step 5's re-target rule).

- [ ] **Step 5: Mutation probe (spec-named: one per-section sort leaf).** In
  `boardPresentation`, hard-code `By: "updated"` for every section (dropping `srt.By.Value`) →
  `TestBoardPresentationLiftsResolvedConfig` must redden (it asserted `id` for proposed).
  Restore. Then make one `includeBoard` call site pass
  `render.DefaultBoardPresentation()` → the renderer stays green there (same values), so
  confirm the guard for THAT is Step 3's grep-derived call-site sweep plus
  `TestBoardRenderHonorsConfiguredPresentation` on the shared path; note the residual only if a
  probe genuinely cannot redden (a residual is for the undetectable, not the unprobed).

- [ ] **Step 6: Commit** — `feat(0367): thread resolved board presentation through every board render`

---

### Task 9: Projection isolation — mechanical guards for digest, selection, mermaid, mirror

**Files:**
- Test: `internal/app/status_result_test.go` (or beside the digest owner — locate the
  machine-readable digest/status assembly in `internal/app/status*.go` by reading, not
  guessing), `internal/domain/selection_test.go` vicinity, `internal/render/board_test.go`.

**Interfaces:**
- Consumes: everything landed above; `domain` selection (`internal/domain/selection.go`) and the
  status digest assembly.

The isolation claim is: **no board presentation leaf flows into** the digest, the autonomous
ready-queue/selection, the Mermaid graph's node/edge order, or the GitHub-mirror semantics.
Guard it from both directions:

- [ ] **Step 1: Write the before/after comparison tests:**

```go
func TestStatusDigestUnchangedByBoardPresentation(t *testing.T) {
    // Build one fixture corpus; produce the machine-readable status/digest
    // result twice — once under default config, once under a config with a
    // permuted section_order and id/asc sorting everywhere. Assert the two
    // digest outputs are DEEPLY EQUAL (lifecycle-status counts, active change
    // lines, readiness tokens, ready queue, ordering — the whole structure).
}

func TestSelectionUnchangedByBoardPresentation(t *testing.T) {
    // domain selection takes no config — assert that mechanically: the
    // selection entry points' signatures accept Snapshot/facts only. Express
    // it as a compile-anchored test: call the selection function on a fixture
    // where board config COULD have changed order (priority ties broken by
    // creation age then lowest id) and assert the exact queue; then assert at
    // the app layer that the path building the autonomous queue never calls
    // boardPresentation (see the grep guard below).
}

func TestBoardMermaidBytesIdenticalAcrossPresentations(t *testing.T) {
    // Render the same fixture under default and permuted presentations;
    // slice each output from "```mermaid" through the closing fence; require
    // bytes.Equal.
}
```

  Plus one shape-derived static guard (never an enumerated call-site list — derive by grep at
  test runtime): a test that walks the non-render, non-board Go sources which implement the
  digest/selection/mirror paths and fails if any references `Board.SectionOrder`,
  `Board.Sorting`, `boardPresentation(` or `BoardPresentation` — anchored on the CONSUMING
  packages/files discovered via `go list`/filepath walk, with the allowlist being exactly the
  board-render path (`internal/render/board.go`, `internal/app/derived_views.go`, the
  op files' includeBoard argument expressions) and the guard asserting its own population floor
  (it must find `boardPresentation(` at least once in the allowed set, so an empty walk cannot
  go vacuously green).

- [ ] **Step 2: Run** — `go test ./internal/app/ ./internal/domain/ ./internal/render/ -run 'Presentation|Selection|Mermaid|Digest' -count=1`.
Expected: PASS (isolation should already hold); treat any failure as a wiring leak to fix in
the producing task's files.

- [ ] **Step 3: Mutation probe (spec-named: the isolation boundary).** Temporarily thread the
  presentation into a digest-visible ordering (e.g. sort the digest's active change lines with
  `sortBoardSection`, or reorder the ready queue by the configured direction) → the digest/
  selection comparison tests must redden. Separately, feed `in.Presentation` into the mermaid
  loop's iteration order → the mermaid byte test must redden. Restore all; `-count=1`.

- [ ] **Step 4: Run the two packages fully** — `go test ./internal/app/ ./internal/render/ ./internal/domain/ ./internal/config/ -count=1` → PASS.

- [ ] **Step 5: Commit** — `test(0367): mechanical projection-isolation guards for board presentation`

---

## Verification map (spec → task)

- built-in defaults + complete default order → Task 1
- per-layer resolution, whole-list replacement, per-leaf inheritance → Tasks 1–2
- missing/duplicate/unknown section token warn + lower-layer fallback → Task 2
- invalid sort field/direction warn + single-leaf inheritance → Task 2
- three sort fields × both directions, same-date ties, unknown-date-last → Task 5
- every classification bucket (spec-backed readiness, trivial placement, unresolved deps/stack
  bases, stacked-merged, finalize-blocked precedence) → Task 4 (+ table cells Task 6)
- count-summary parity → Task 6
- unified Built/Blocked cells → Task 6
- configured ordering + empty-section omission → Task 6
- same-day archive regression + fixed footer → Task 7
- byte-stable repeat render → Task 7 (and golden/determinism in Task 6)
- digest/ready-queue/Mermaid/GitHub-mirror unchanged → Task 9
- config inspection/example/documentation drift guards → Task 3
- spec-named mutations: groomed spec conjunct + finalize precedence (Task 4), section-order
  completeness (Task 2), per-section sort leaf (Task 8), date tie-breaker (Task 5),
  projection-isolation boundary (Task 9)

## Notes for the builder

- **Facts at app render sites:** `renderCanonicalBoard` passes zero `domain.BranchFacts` today,
  so a stacked proposal classifies via stack-base-unresolved → Proposed at those sites. That
  matches the current proposed-readiness cells; do not add Git lookups to rendering (spec's
  out-of-scope list).
- **Shared-board churn is per spec:** board presentation is global-able and not fenced, so two
  machines with different personal layers will each re-render the committed BOARD.md their own
  way (repository check/repair included). That is the spec's deliberate trade — presentation of
  a derived view, last writer wins — not a defect to engineer around here.
- **Full suite at the end:** docket-build's gate runs whatever `finalize.test_command` resolves
  to (`go run ./cmd/docket development test`) over the whole tree; task-level runs above are
  focused only.
- **Header comment + skill prose:** the `board.go` header rewrite is in Task 6. The stale
  `render-board.sh` prose in embedded skill docs is a separately-captured docs change (per the
  change file's reconcile log) — do not chase it here, EXCEPT any guard that reddens on this
  branch, which must be reconciled within the task that reddened it.
