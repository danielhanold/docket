<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0312 — Planning mutations, inline board, and ADRs](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0312-planning-mutations-board-and-adrs.md)**
<!-- docket:backlink:end -->

# Planning Mutations, Inline Board, and ADRs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first complete planning-state write slice (change 0312): ten typed metadata operations — `change create/groom/block/defer/kill`, `learning record/update`, `adr record/supersede/reverse` — each landing its source mutation and every affected v1-owned derived view (inline board, change artifact block, spec backlink, ADR index) as one validated atomic transaction commit.

**Architecture:** A new pure package `internal/render` owns canonical new-record serialization, source-preserving authored-section edits, and the four derived-view renderers. `internal/app` gains one coarse operation per mutation: it validates the typed request up front, pins authoritative context via the 0310 read seams, then hands a `transaction.SemanticOperation` closure to the 0309 engine, which reloads fresh state per attempt, applies the domain transition, renders every affected view, validates the complete candidate, and commits with an exact-lease push. `internal/cli` adds thin commands that read a closed JSON request from stdin or a file and present the versioned result.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), Cobra, go-yaml (via `internal/document` only), the repo's bash test runner (`scripts/run-tests.sh`), real-git temp fixtures with bare remotes, golden fixtures frozen from the Bash renderers.

**Spec:** `docs/superpowers/specs/2026-08-16-planning-mutations-board-and-adrs-design.md` (synchronized metadata-tree copy; the change file is `docs/changes/active/0312-planning-mutations-board-and-adrs.md` on the `docket` branch).

## Global Constraints

- **Compose, never duplicate.** This slice is a client of `internal/gitcli`, `config`, `document`, `domain`, `repository`, `repository/transaction`, `app`, `cli`. Lifecycle rules come from `domain` (`Block`, `Defer`, `Kill`, `Supersede`, `Reverse`, `NextADRID`, `ValidateADRGraph`); parsing/patching from `document`; snapshot building and evolution from `repository.BuildSnapshot` / `repository.ValidateEvolution`; transactions from `transaction.Engine`. No new YAML parser, no new Git porcelain, no reimplemented readiness/selection.
- **`internal/render` is pure.** It receives typed values, source bytes, and an explicit `LinkContext`; it returns bytes or an error. It never reads the filesystem, invokes Git, reads the clock, parses flags, or commits. Equal input ⇒ byte-identical output.
- **Existing records are never regenerated.** Owned frontmatter fields and managed blocks change only through `document.PatchSet` (loss-preserving, marker order/balance-checked); authored bodies change only through the fence-aware section editor (Task 1). The canonical new-record serializer (Task 2) is for *new* files only. Unknown fields, unknown headings, comments, quoting, blank lines, and line endings survive byte-identically.
- **New records are canonical by construction.** All frontmatter is emitted through `document.New` (`internal/document/builder.go`), which quotes free-text scalars unconditionally (ADR-0071). Flow collections (`depends_on: [3]`) stay unquoted sequences.
- **Board surfaces.** `board_surfaces: [inline]` ⇒ every change operation's plan includes the board bytes (even when unchanged). `board_surfaces: []` ⇒ the board renderer is not invoked and any historical `docs/changes/BOARD.md` is left untouched. `board_surfaces` containing `github` ⇒ `unsupported-config` at preflight, before any transaction.
- **Atomicity.** Every operation submits exactly one `transaction.MutationPlan` covering the closed file set in the spec's "Atomic file plans" table. Renderer or validation refusal ⇒ nothing commits. Allocation, section patching, and rendering re-run from fresh state on every attempt — never replay stale bytes.
- **Idempotency.** Creation operations (`change create`, `change groom`'s spec file creation rides its change edit, `learning record`, `adr record/supersede/reverse`) carry a caller-supplied `request_id` (shape `^[A-Za-z0-9][A-Za-z0-9._-]*$`, 8–128 ASCII) and a canonical-JSON `sha256:` digest over all semantic inputs, including authored section bytes. Replay returns the original receipt (`Replayed: true`); same id + different digest ⇒ `invalid-input`. Non-allocating operations require exact submitted entity versions instead (full blob object id from the 0310 read context).
- **Result protocol v1.** Every result struct embeds `app.Envelope`, is registered in `TestEnvelopeNotShadowed` (`internal/app/shadow_test.go`), marshals collections as `[]` never `null`, and never exposes transaction-worktree paths. Outcomes map onto the closed `app.Result` taxonomy only.
- **TDD, and every mutation probe runs `go test -count=1`.** Go's cache serves stale passes against a mutated tree otherwise (learnings: `cached-runner-serves-a-mutated-tree`). Negative mutation tests are required for the section editor's guards and the frozen-golden drift asserts — strip the guard, watch the named test redden, restore.
- **Golden fixtures are historical snapshots with provenance.** Board/artifact-block/backlink/ADR-index goldens are generated once from the live Bash renderers (`scripts/render-board.sh`, `render-change-links.sh`, `render-artifact-backlink.sh`, `render-adr-index.sh`) and frozen with a `PROVENANCE.md` naming the generating command and commit — they must NOT track the scripts afterward (the scripts die in the 0316+ cutover; learnings: `frozen-copy-needs-a-drift-assert`). Fixture corpora are data: exclude them from repo-wide scans with a bounded path, and mutation-test the exclusion (learnings: `frozen-fixture-corpus-trips-repo-wide-scans`).
- **Kill is a rename, not a reuse.** The archive move is one `MutationCreate` (archive path) + one `MutationDelete` (active path) in the same plan; identity checks compare by record content identity, not path (learnings: `relocation-reads-as-identity-reuse` — `repository.ValidateEvolution` already models this; do not add a path-keyed check).
- **Build gate = the whole suite** via `scripts/run-tests.sh` (that is what `.docket.yml` `finalize.test_command` resolves to), never only this plan's tests. A trailing `OVER BUDGET:` line is a finding to act on (Task 16).
- **Commit discipline.** Stage only the files each task names — never `git add -A`.

## File Structure

- `internal/render/link.go` — `LinkContext` and blob-URL derivation.
- `internal/render/section.go` + `section_test.go` — fence-aware, source-preserving authored-section editor.
- `internal/render/record.go` + `record_test.go` + `testdata/records/` — canonical new change/learning/ADR serializers.
- `internal/render/artifacts.go` + `artifacts_test.go` — change artifact-block content and spec backlink content.
- `internal/render/board.go` + `board_test.go` + `testdata/board/` — inline board renderer.
- `internal/render/adrindex.go` + `adrindex_test.go` + `testdata/adrindex/` — ADR index renderer.
- `internal/app/planning.go` + `planning_test.go` — shared deps, production `transaction.StateLoader`, request-digest canonicalization, board-surface fence, disposition→result mapping, planning result findings.
- `internal/app/change_create.go`, `change_groom.go`, `change_lifecycle.go` (block+defer), `change_kill.go` + `_test.go` each — the five change operations.
- `internal/app/learning_ops.go` + `learning_ops_test.go` — `learning record` / `learning update`.
- `internal/app/adr_ops.go` + `adr_ops_test.go` — `adr record` / `adr supersede` / `adr reverse`.
- `internal/app/planning_git_test.go` — real-git integration + concurrency matrix (both repository modes, bare remotes).
- `internal/cli/change.go`, `internal/cli/learning.go`, `internal/cli/adr.go` + `_test.go` each — command surface.
- `internal/app/shadow_test.go` — result-struct registration (modified per operation task).
- `tests/runtime-budgets.tsv` — budget rows for any new/expanded shell-visible test files (Task 16).

---

### Task 1: `internal/render` scaffold, LinkContext, and the authored-section editor

**Files:**
- Create: `internal/render/link.go`, `internal/render/section.go`
- Test: `internal/render/section_test.go`, `internal/render/link_test.go`

**Interfaces (later tasks rely on these exact names):**

```go
package render

// LinkContext carries everything link rendering needs; render never derives it.
type LinkContext struct {
	// RepoWebURL is the https base of the repository, no trailing slash,
	// e.g. "https://github.com/danielhanold/docket". Empty means "render
	// repo-relative links only" (callers without a resolvable web remote).
	RepoWebURL string
	// MetadataBranch is the branch blob links point at, e.g. "docket" or "main".
	MetadataBranch string
}

// BlobURL returns RepoWebURL + "/blob/" + MetadataBranch + "/" + repoRelPath,
// or "" when RepoWebURL is empty.
func (l LinkContext) BlobURL(repoRelPath string) string

type SectionIntent string

const (
	SectionPreserve SectionIntent = "preserve"
	SectionReplace  SectionIntent = "replace"
	SectionRemove   SectionIntent = "remove"
)

// SectionEdit names one operation-owned top-level section by its exact heading
// line (e.g. "## Why killed") and what to do with it. Markdown is the section
// body WITHOUT the heading line; it must be empty unless Intent is replace.
type SectionEdit struct {
	Heading  string
	Intent   SectionIntent
	Markdown string
}

// ApplySectionEdits splices edits into src, touching only owned sections.
//   - owned is the closed set of headings this operation may touch; an edit
//     naming a heading outside owned is an error.
//   - Headings are matched as exact full lines at column 0, outside fenced
//     code blocks (``` or ~~~ fences, any info string, fences may be indented
//     up to 3 spaces per CommonMark — reuse the fence rules in
//     internal/document/markers.go as the reference behavior, but implement
//     locally: markers.go's helpers are private and marker-specific).
//   - A section spans from its heading line to the line before the next
//     top-level "## " heading outside fences, or EOF.
//   - Every owned heading present in src must be unique; a duplicate refuses
//     the whole edit set. All edits are validated before any splice.
//   - replace: substitute the section body (insert heading + body before the
//     document's trailing newline when absent — appended at EOF, preceded by
//     exactly one blank line). remove: delete heading and body (error when
//     absent). preserve: assert untouched (no-op, valid whether present or not).
//   - Splices apply from the last edit position toward the first, then the
//     candidate is reparsed with document.Parse; a parse failure refuses.
//   - All other bytes — unknown headings, prose, line endings — are identical.
func ApplySectionEdits(src []byte, owned []string, edits []SectionEdit) ([]byte, error)

// Owned heading sets, exported for app operations:
var ChangeOwnedHeadings = []string{
	"## Why", "## What changes", "## Out of scope", "## Open questions",
	"## Why deferred", "## Why killed", "## Auto-groom blocked",
}
var LearningOwnedHeadings = []string{"## Apply", "## War story"}
```

- [ ] **Step 1: Write failing tests for the editor's contract.** Cover at minimum, each as its own test function:
  - replace of an existing section changes only that section's bytes (assert byte-identical prefix/suffix around the splice, CRLF fixture included);
  - replace inserts at EOF when the section is absent (single preceding blank line, trailing newline preserved);
  - remove deletes heading + body; remove of an absent section errors;
  - a `## Why` line inside a fenced block is not a heading (fixture: a change file whose body contains a fenced example holding `## Why killed`);
  - duplicate owned heading ⇒ error, src returned unchanged;
  - edit naming a heading outside `owned` ⇒ error;
  - two edits in one call splice correctly (end-toward-beginning order observable via offsets);
  - unknown headings/sections between owned ones survive byte-identically;
  - candidate reparse failure refuses (construct via a Markdown body that document.Parse rejects, e.g. breaking the frontmatter close — drive the editor over a file whose only `---` close would be consumed by a malicious section body; if unconstructable, cover with the validation-order test instead and say so in a comment).
  - `BlobURL` with and without `RepoWebURL`.

```go
func TestApplySectionEditsReplacesOnlyOwnedBytes(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nold why\n\n## Custom notes\n\nkeep me\n")
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionReplace, Markdown: "new why\n"},
	})
	if err != nil { t.Fatal(err) }
	if !bytes.Contains(out, []byte("## Custom notes\n\nkeep me\n")) {
		t.Fatalf("unowned section mutated:\n%s", out)
	}
	if bytes.Contains(out, []byte("old why")) { t.Fatalf("owned body not replaced") }
}
```

- [ ] **Step 2: Run `go test ./internal/render/ -count=1` — verify every test fails** (package does not exist yet).
- [ ] **Step 3: Implement `link.go` and `section.go`** per the contract above. Section scanning: split into lines preserving terminators, track fence state, collect top-level headings with byte offsets, validate, then splice.
- [ ] **Step 4: Run `go test ./internal/render/ -count=1` — all pass.**
- [ ] **Step 5: Mutation-probe the two load-bearing guards.** Temporarily delete (a) the fence-state check and (b) the duplicate-heading refusal; run `go test ./internal/render/ -count=1` after each; the fenced-heading and duplicate tests must fail. Restore. Record the probe in the task commit message body.
- [ ] **Step 6: Commit** `feat(0312): render section editor and link context (task 1)` staging exactly the four files.

---

### Task 2: Canonical new-record serializers (change, learning, ADR)

**Files:**
- Create: `internal/render/record.go`, `internal/render/testdata/records/` (goldens + `PROVENANCE.md`)
- Test: `internal/render/record_test.go`

**Interfaces:**
- Consumes: `document.New`, `document.FieldSpec` (`internal/document/builder.go`), Task 1's nothing.
- Produces:

```go
// NewChangeRecord describes one canonical proposed change record.
type NewChangeRecord struct {
	ID             domain.ChangeID
	Slug, Title    string
	Type           string // validated against config change_types by the app layer
	Priority       string // stored spelling: critical|high|medium|low
	Created        time.Time // date part rendered YYYY-MM-DD UTC
	DependsOn      []domain.ChangeID
	StackedOn      *domain.ChangeID
	Related        []domain.ChangeID
	DiscoveredFrom []domain.ChangeID
	ADRs           []domain.ADRID
	Why, WhatChanges, OutOfScope string // markdown bodies, no heading lines
}
func ChangeRecord(r NewChangeRecord) ([]byte, error)

type NewLearningRecord struct {
	Slug, Hook string
	Topics     []string
	Changes    []domain.ChangeID
	Created    time.Time
	Apply, WarStory string
}
func LearningRecord(r NewLearningRecord) ([]byte, error)

type NewADRRecord struct {
	ID          domain.ADRID
	Slug, Title string
	Date        time.Time
	Change      *domain.ChangeID
	RelatesTo   []domain.ADRID
	Supersedes  []domain.ADRID // populated by adr supersede; empty for plain record
	Reverses    []domain.ADRID
	Context, Decision, Consequences, Alternatives string
}
func ADRRecord(r NewADRRecord) ([]byte, error)
```

Behavioral requirements:
- Field names, order, and defaults follow the live templates exactly: change records mirror `skills/docket-new-change/change-template.md` (id, slug, title, status: proposed, priority, type, created, updated, depends_on, stacked_on, related, discovered_from, adrs, spec, plan, results, trivial: false, auto_groomable, branch, pr, blocked_by, reconciled: false) but WITHOUT the template's trailing comments (canonical Go records are comment-free; the template's comments are authoring hints). `updated` = `created`. Body: `## Artifacts` with an empty `docket:artifacts` managed block (exact marker spelling from `internal/document/markers.go` — `<!-- docket:artifacts:start (generated — do not hand-edit) -->`), then `## Why`, `## What changes`, `## Out of scope` with the supplied bodies.
- Learning records mirror the live findings (see any file under `docs/changes/learnings/`): slug, hook (quoted by construction), topics (flow seq), changes (flow seq), created, updated, `promotion_state: retained`, empty `promoted_to:`; body `## Apply` + `## War story`.
- ADR records mirror `docs/adrs/*.md`: id, slug, title, `status: Accepted`, date, `supersedes: []`/`reverses: []` flow seqs, `relates_to: []`, `change:` (empty when nil); body `## Context`, `## Decision`, `## Consequences`, `## Alternatives considered`.
- All frontmatter goes through `document.New`; every output ends with exactly one trailing newline and reparses cleanly via `document.Parse` (assert in tests — round-trip, learnings: `validator-must-match-the-reader-it-feeds`).

- [ ] **Step 1: Freeze goldens.** Copy one real change record (a freshly minted Bash-era stub shape), one real learning record, and one real ADR into `internal/render/testdata/records/` is NOT the approach — the serializer emits *canonical* records, not copies. Instead hand-author the three expected golden outputs from the templates above, and write `PROVENANCE.md` naming the source templates and stating these goldens are the canonical v1 shapes (historical-snapshot contract, must not silently track the templates).
- [ ] **Step 2: Write failing golden tests** — one per record kind: build the struct, assert byte-equality with the golden; plus property tests: output reparses via `document.Parse` with zero findings-relevant errors; a title containing `: ` and a trailing colon round-trips (quoting by construction); empty vs populated relationship collections render `[]` vs `[3, 5]`.
- [ ] **Step 3: Run `go test ./internal/render/ -run TestChangeRecord -count=1`** (and siblings) — FAIL (functions undefined).
- [ ] **Step 4: Implement `record.go`.**
- [ ] **Step 5: Run `go test ./internal/render/ -count=1` — all pass.**
- [ ] **Step 6: Commit** `feat(0312): canonical change, learning, and ADR record serializers (task 2)`.

---

### Task 3: Change artifact-block and spec-backlink renderers

**Files:**
- Create: `internal/render/artifacts.go`
- Test: `internal/render/artifacts_test.go`, goldens under `internal/render/testdata/artifacts/` + `PROVENANCE.md`

**Interfaces:**
- Consumes: `LinkContext` (Task 1), `domain.Change`, `domain.Snapshot`.
- Produces:

```go
// ArtifactBlockContent renders the body of the managed docket:artifacts block
// (the table between the markers, no marker lines) for one change, from its
// typed fields: Spec, Plan, Results paths and ADRs list. Rows appear in the
// fixed order Spec, Plan, Results, ADRs; a row is omitted when its field is
// unset/empty. Empty content ("") means "no artifacts" — the caller still
// writes the (empty) block.
func ArtifactBlockContent(c domain.Change, link LinkContext) (string, error)

// BacklinkContent renders the full backlink block for a spec/plan/results file:
// marker lines + the "> ↩ **[Change NNNN — Title](url-or-relpath)**" line,
// targeting the change's CURRENT canonical metadata path.
func BacklinkContent(c domain.Change, link LinkContext) (string, error)
```

- [ ] **Step 1: Freeze goldens from the Bash renderers.** Run `scripts/render-change-links.sh` / `scripts/render-artifact-backlink.sh` against two fixture changes (one with spec+ADRs, one with spec+plan+results) in a scratch tree; freeze the emitted block/backlink bytes under `testdata/artifacts/` with a `PROVENANCE.md` naming the exact commands and this commit. Compare with the live examples at the top of `docs/changes/active/0312-...md` and this plan's own backlink for shape.
- [ ] **Step 2: Write failing tests**: golden byte-equality for both renderers; ADR list renders comma-separated `[ADR-0001](...)` links; with empty `LinkContext.RepoWebURL` links are repo-relative; determinism (call twice, `bytes.Equal`).
- [ ] **Step 3: Run `go test ./internal/render/ -run 'Artifact|Backlink' -count=1` — FAIL.**
- [ ] **Step 4: Implement. Step 5: tests pass (`-count=1`). Step 6: Commit** `feat(0312): artifact block and backlink renderers (task 3)`.

---

### Task 4: Inline board renderer

**Files:**
- Create: `internal/render/board.go`
- Test: `internal/render/board_test.go`, golden under `internal/render/testdata/board/` + `PROVENANCE.md`

**Interfaces:**
- Consumes: `domain.Snapshot`, `domain.EvaluateReadiness`, `domain.SelectQueue`, `domain.BranchFacts` — the same policies `internal/app/status.go` composes (read `statusChange`/`readinessReason` there and reuse the domain calls, not the app DTOs).
- Produces:

```go
type BoardInput struct {
	Snapshot domain.Snapshot
	Facts    domain.BranchFacts // readiness inputs; zero value when unknown
}
// Board renders docs/changes/BOARD.md byte-for-byte in the established inline
// surface shape (see scripts/render-board.sh and the live BOARD.md): the
// "# Backlog" header, the counts line, then the sections in fixed order —
// In progress, Proposed, Blocked, Deferred, Implemented, Done, Killed (a
// section renders only when non-empty; match render-board.sh's exact set,
// order, headers, emoji, and column layouts — enumerate them from the script,
// do not guess). Links are repo-relative from docs/changes/.
func Board(in BoardInput) ([]byte, error)
```

- [ ] **Step 1: Inventory the reference surface.** Read `scripts/render-board.sh` end-to-end and write the section/column/wording contract into `board.go`'s package comment (section order, counts-line format, readiness-cell wording including `build-ready`, `needs-brainstorm`, `⏳ waiting on #N — not yet built`, `auto-groom blocked — needs you`, sort orders within sections). The script is the authority; the plan deliberately does not restate all 657 lines.
- [ ] **Step 2: Freeze a golden.** Build a small fixture corpus (6–10 hand-built change files covering: in-progress with spec+branch, proposed build-ready, proposed needs-brainstorm, proposed waiting-on-dep, deferred, done archived, killed archived), run `scripts/render-board.sh` over it, freeze output + `PROVENANCE.md` (command + commit + the note that the script dies at cutover). Inventory what the fixture actually covers and write it down (learnings: `frozen-corpus-covers-what-it-contains`); readiness bands the fixture lacks get direct Go unit tests instead.
- [ ] **Step 3: Failing tests**: golden byte-equality over the fixture snapshot (build `domain.Snapshot` from the same fixture files via `document.Parse` + `repository.BuildSnapshot`); determinism; unset-priority/unset-type records sort deliberately, not by accident of empty-string collation (learnings: `unset-sort-key-check-your-own-template` — pin the rule the script implements and test it).
- [ ] **Step 4–5: Implement; `go test ./internal/render/ -count=1` green.**
- [ ] **Step 6: Commit** `feat(0312): inline board renderer (task 4)`.

---

### Task 5: ADR index renderer

**Files:**
- Create: `internal/render/adrindex.go`
- Test: `internal/render/adrindex_test.go`, golden under `internal/render/testdata/adrindex/` + `PROVENANCE.md`

**Interfaces:**
- Consumes: `domain.Snapshot` (its ADR set — `domain.ADR` getters for ID, Slug, Title, RawStatus, Change, Supersedes, Reverses, RelatesTo; read `internal/domain/entities.go` for exact names).
- Produces:

```go
// ADRIndex renders docs/adrs/README.md from the complete candidate ADR set:
// header + fixed prose line, then groups "Active", "Superseded / Reversed",
// "Deprecated" (statuses beginning "Superseded by"/"Reversed by" go to the
// second group; "Deprecated" to the third; everything else Active), each
// sorted numerically, rows formatted exactly as scripts/render-adr-index.sh's
// row() — "- [ADR-0007](file.md) — Title (Status) ← change #11 → supersedes
// ADR-0001, ... · relates to ADR-0002"; an empty group renders "_None._".
func ADRIndex(snap domain.Snapshot) ([]byte, error)
```

- [ ] **Step 1: Freeze golden** by running `scripts/render-adr-index.sh --adrs-dir` over a fixture of 5 hand-built ADRs covering all three groups and every annotation; `PROVENANCE.md` as before.
- [ ] **Step 2: Failing tests**: golden equality; empty-group `_None._`; determinism.
- [ ] **Step 3–4: Implement; green with `-count=1`.**
- [ ] **Step 5: Commit** `feat(0312): ADR index renderer (task 5)`.

---

### Task 6: `internal/app` planning plumbing — loader, digest, fence, result mapping

**Files:**
- Create: `internal/app/planning.go`
- Test: `internal/app/planning_test.go`

**Interfaces:**
- Consumes: `transaction.StateLoader/LoadedState/Tree/Result/Failure`, `repository.BuildSnapshot/ValidateEvolution/InputDocument/BuildInput/EvolutionInput`, `document.Parse`, `config.Effective`, the corpus classification helpers in `internal/app/status_git.go` (`corpusPrefixes`, `classifyCorpusPath` — reuse; export or call within the package, they are already package-level).
- Produces:

```go
// PlanningDeps is every seam a planning operation needs; tests inject fakes.
type PlanningDeps struct {
	Client *gitcli.Client
	Engine interface {
		Execute(ctx context.Context, req transaction.Request) (transaction.Result, error)
	}
	Reader StatusReader          // 0310 pin/read seams for preflight + entity versions
	Clock  transaction.Clock     // sole time source; operations never call time.Now
}

// newPlanningLoader builds the production StateLoader: ListTree over the
// config-derived corpus prefixes, ReadBlobs, document.Parse per record,
// repository.BuildSnapshot; ValidateEvolution delegates to
// repository.ValidateEvolution. A parse failure of one record surfaces as a
// Report finding, not a Go error (mirror parseCorpus in status.go).
func newPlanningLoader(eff config.Effective) transaction.StateLoader

// canonicalDigest computes "sha256:<hex>" over the canonical compact JSON of
// payload (a closed request struct; json.Marshal with sorted struct fields is
// canonical enough since the struct is closed — no maps allowed in payloads).
func canonicalDigest(operation string, payload any) (transaction.RequestDigest, error)

// boardEnabled / fenceBoardSurface: []string board_surfaces →
//   (inline present → true), (contains "github" → *cliError with
//   app.ResultUnsupportedConfig, before any transaction).
func fenceBoardSurface(eff config.Effective) (inline bool, err error)

// mapOutcome folds a transaction outcome into the v1 taxonomy:
//   applied → ResultApplied; already-applied → ResultApplied (Replayed=true);
//   no-op → ResultNoOp; contended → ResultContended; refused → ResultInvalidState
//   (or ResultInvalidInput when the refusal findings are request-shaped — the
//   operation passes which); failed → by Failure.Kind: invalid-input→
//   ResultInvalidInput, invalid-state→ResultInvalidState, validation→
//   ResultInvalidState, external/unknown-result→ResultExternalFailed,
//   cancelled→ResultInterrupted; anything else → ResultInternalError.
func mapOutcome(res transaction.Result, err error, refusalKind app.Result) (app.Result, bool /*replayed*/)
```

- [ ] **Step 1: Failing tests.** Loader: drive `Load` over a fake `transaction.Tree` (in-memory map — model on the fakes in `internal/repository/transaction/loader_test.go`) holding two changes + one ADR + `.docket.yml`-derived Effective; assert snapshot counts, `Documents`/`Sources` keyed by path, a malformed record lands in `Report` not error. Digest: deterministic across calls, differs on any field change, `sha256:` + 64 hex shape. Fence: `[inline]`→true, `[]`→false, `[inline github]`→unsupported-config error. mapOutcome: table-driven over every disposition × failure kind.
- [ ] **Step 2: FAIL; Step 3: implement; Step 4: green `-count=1`.**
- [ ] **Step 5: Commit** `feat(0312): planning loader, digest, board fence, outcome mapping (task 6)`.

---

### Task 7: `change create`

**Files:**
- Create: `internal/app/change_create.go`
- Test: `internal/app/change_create_test.go`
- Modify: `internal/app/shadow_test.go` (register `ChangeCreateResult`)

**Interfaces:**
- Consumes: Tasks 1–4, 6 (`render.ChangeRecord`, `render.ArtifactBlockContent`, `render.Board`, `ApplySectionEdits` not needed here, `PlanningDeps`, `canonicalDigest`, `fenceBoardSurface`), `document.PatchSet` for writing the artifact block into the new record — the serializer emits the empty block; fill it via `document.Parse`+`ReplaceBlock` inside the plan closure.
- Produces:

```go
const OperationChangeCreate = "change.create"

type ChangeCreateRequest struct {
	RequestID string   `json:"request_id"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Priority  string   `json:"priority"`
	Why         string `json:"why"`
	WhatChanges string `json:"what_changes"`
	OutOfScope  string `json:"out_of_scope"`
	DependsOn      []int `json:"depends_on"`
	StackedOn      *int  `json:"stacked_on"`
	Related        []int `json:"related"`
	DiscoveredFrom []int `json:"discovered_from"`
	ADRs           []int `json:"adrs"`
}

type ChangeCreateResult struct {
	Envelope
	ID       int    `json:"id,omitempty"`
	Slug     string `json:"slug,omitempty"`
	Path     string `json:"path,omitempty"`
	Revision string `json:"committed_revision,omitempty"`
	Replayed bool   `json:"replayed,omitempty"`
	Findings []StatusFinding `json:"findings"`
}

func ChangeCreate(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeCreateRequest) ChangeCreateResult
```

Behavior (each bullet gets a test):
- Full request validation before any transaction: request-id shape, non-empty title/why/what/out-of-scope, type ∈ config `change_types`, priority ∈ {critical,high,medium,low}, duplicate ids within a collection, self-references — failures return `invalid-input` with findings and no engine call (assert via a fake engine that records calls).
- Preflight: pin context via `deps.Reader.PinContext`, resolve config, `fenceBoardSurface`.
- The `SemanticOperation.Plan` closure, per attempt: allocate `max(active ∪ archive change id) + 1` from `st.State.Snapshot` (never fill gaps — test with a gapped corpus); derive slug from the title exactly as the domain expects (lowercase, hyphenated, trimmed — mirror the slug rules `domain` validates; read `internal/domain` slug validation first and reuse if exported); refuse (`invalid-input` findings) on dangling `depends_on`/`related`/`discovered_from`/`adrs` references or dependency cycles against the candidate snapshot; build the record via `render.ChangeRecord` with `Created = deps.Clock.Now().UTC()`; fill the artifact block; plan `MutationCreate` for `docs/changes/active/NNNN-slug.md` plus the board file when inline is enabled.
- Idempotency: `transaction.IdempotencyKey{RequestID: req.RequestID, Digest: canonicalDigest(OperationChangeCreate, semanticPayload)}` where semanticPayload is the request minus RequestID. Receipt: canonical compact JSON `{"op":"change.create","id":N,"slug":"...","path":"..."}` (≤4096 bytes). Replay: result rebuilt from the decoded receipt with `Replayed: true`.
- Existing Bash-era records are not rewritten: the plan touches only the new path (+ board).
- Two allocation attempts under a moved base produce a fresh id (test by driving the closure twice with different snapshots — unit level; the real concurrent proof is Task 16).

- [ ] **Step 1: Failing tests** with a fake engine (records the `transaction.Request`, returns a scripted `Result`) and fake reader — cover every bullet above; assert the emitted `MutationPlan` file set matches the spec's table exactly (create record [+ board]); board bytes present when inline, absent when `[]`.
- [ ] **Step 2: FAIL. Step 3: Implement. Step 4: green `-count=1`. Step 5: register in shadow_test.**
- [ ] **Step 6: Commit** `feat(0312): change create operation (task 7)`.

---

### Task 8: `change groom` (authored-spec and trivial outcomes)

**Files:**
- Create: `internal/app/change_groom.go`
- Test: `internal/app/change_groom_test.go`
- Modify: `internal/app/shadow_test.go`

**Interfaces:**
- Consumes: Tasks 1, 3, 4, 6 (`ApplySectionEdits` with `ChangeOwnedHeadings`, `ArtifactBlockContent`, `BacklinkContent`, `Board`, plumbing), `document.PatchSet` (`SetField` for spec/trivial/updated fields).
- Produces:

```go
const OperationChangeGroom = "change.groom"

type GroomOutcome string
const (GroomSpec GroomOutcome = "spec"; GroomTrivial GroomOutcome = "trivial")

type ChangeGroomRequest struct {
	ChangeID int    `json:"change_id"`
	Path     string `json:"path"`     // current canonical record path
	Version  string `json:"version"`  // exact full blob object id
	Outcome  GroomOutcome `json:"outcome"`
	SpecMarkdown string   `json:"spec_markdown,omitempty"` // required for spec outcome
	Sections []SectionEditRequest `json:"sections"` // proposal-section edits
	DependsOn []int `json:"depends_on"` // complete desired values
	Related   []int `json:"related"`
	DiscoveredFrom []int `json:"discovered_from"`
	ADRs      []int `json:"adrs"`
	StackedOn *int  `json:"stacked_on"`
}
type SectionEditRequest struct {
	Heading string `json:"heading"`
	Intent  string `json:"intent"` // preserve|replace|remove
	Markdown string `json:"markdown,omitempty"`
}

type ChangeGroomResult struct {
	Envelope
	ID int `json:"id,omitempty"`
	SpecPath string `json:"spec_path,omitempty"`
	Revision string `json:"committed_revision,omitempty"`
	Findings []StatusFinding `json:"findings"`
}
func ChangeGroom(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeGroomRequest) ChangeGroomResult
```

Behavior (spec §`change groom`; each bullet a test):
- Entity expectation: `{Path, VersionBlob(req.Version)}` — a moved/edited record returns `contended` from the engine untouched.
- Domain gate: change must be proposed, need design (spec empty, `trivial: false`), and be legal to groom; refusal → `invalid-state`. Grooming never sets a claim.
- Spec outcome: validate `SpecMarkdown` non-empty and parseable before the transaction; spec path `docs/superpowers/specs/<clock UTC date>-<change-slug>-design.md`; refuse if that path already exists in the candidate tree; plan = spec file (backlink block + submitted markdown) + change record (spec field set via `PatchSet.SetField`, updated date, section/relationship edits, artifact block) + board when inline — one plan, tested by asserting the file set.
- Trivial outcome: no spec file; `trivial: true` via SetField; require a non-empty authored rationale among the section edits (refuse otherwise).
- Section edits go through `ApplySectionEdits` with `ChangeOwnedHeadings`; removing `## Open questions` / `## Auto-groom blocked` happens only when the request says remove.
- Relationship collections are complete desired values written via `PatchSet.SetField` (flow-seq `document.Value` — read `internal/document/value.go` for the list value constructor).
- `updated` is set from `deps.Clock`.

- [ ] **Steps 1–5 (TDD as Task 7):** failing tests → implement → green `-count=1` → shadow registration. Include a source-preservation test: groom a fixture change carrying an unknown frontmatter field + unknown body section, assert both survive byte-identically in the planned record bytes.
- [ ] **Step 6: Commit** `feat(0312): change groom operation (task 8)`.

---

### Task 9: `change block` and `change defer`

**Files:**
- Create: `internal/app/change_lifecycle.go`
- Test: `internal/app/change_lifecycle_test.go`
- Modify: `internal/app/shadow_test.go`

**Interfaces:**
- Consumes: `domain.Block`, `domain.Defer` (their `ActionResult.Fields` name the owned frontmatter changes — read `internal/domain/actions.go`; apply each `FieldChange` via `PatchSet.SetField`), `ApplySectionEdits`, `ArtifactBlockContent`, `Board`.
- Produces:

```go
const (
	OperationChangeBlock = "change.block"
	OperationChangeDefer = "change.defer"
)
type ChangeBlockRequest struct {
	ChangeID int    `json:"change_id"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	Reason   string `json:"reason"` // non-empty
}
type ChangeDeferRequest struct {
	ChangeID int    `json:"change_id"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	WhyDeferred string `json:"why_deferred"` // authored section body, non-empty
}
type ChangeLifecycleResult struct {
	Envelope
	ID int `json:"id,omitempty"`
	Status string `json:"status,omitempty"` // resulting stored status
	Revision string `json:"committed_revision,omitempty"`
	Findings []StatusFinding `json:"findings"`
}
func ChangeBlock(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeBlockRequest) ChangeLifecycleResult
func ChangeDefer(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeDeferRequest) ChangeLifecycleResult
```

Behavior: exact-version expectation; domain `Block`/`Defer` decides legality per current status (illegal → `invalid-state` carrying the `PolicyFailure` reason); block writes only the owned lifecycle fields + updated + artifact block (+ board); defer additionally replaces/inserts `## Why deferred` via the section editor; no process inspection anywhere. File set per the spec table: change record (+ board).

- [ ] **Steps 1–5: TDD as before.** Tests: every domain-legal source status transitions; every illegal one refuses with `invalid-state`; empty reason/section → `invalid-input` pre-transaction; source-preservation over a fixture with unknown fields; plan file set exact.
- [ ] **Step 6: Commit** `feat(0312): change block and defer operations (task 9)`.

---

### Task 10: `change kill`

**Files:**
- Create: `internal/app/change_kill.go`
- Test: `internal/app/change_kill_test.go`
- Modify: `internal/app/shadow_test.go`

**Interfaces:**
- Consumes: `domain.Kill` (clears claim metadata; read `killedResult` in `internal/domain/actions.go`), `ApplySectionEdits`, `ArtifactBlockContent`, `BacklinkContent`, `Board`, `document.PatchSet`.
- Produces:

```go
const OperationChangeKill = "change.kill"
type ChangeKillRequest struct {
	ChangeID int    `json:"change_id"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	WhyKilled string `json:"why_killed"` // authored section body, non-empty
}
type ChangeKillResult struct {
	Envelope
	ID int `json:"id,omitempty"`
	ArchivePath string `json:"archive_path,omitempty"`
	Revision string `json:"committed_revision,omitempty"`
	Findings []StatusFinding `json:"findings"`
}
func ChangeKill(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeKillRequest) ChangeKillResult
```

Behavior, one plan (spec §`change kill`):
- Domain `Kill` gates the transition and yields the killed-state field changes including cleared claim fields; apply via PatchSet, splice `## Why killed`, rerender the artifact block.
- Archive move: `MutationCreate` at `docs/changes/archive/<clock UTC date>-NNNN-slug.md` with the final bytes + `MutationDelete` of the active path, same plan (learnings: `presence-encoded-state` — leaving the active file would keep the change visibly alive; the delete is part of the transition, assert it explicitly).
- When the change's `spec:` points at a metadata-resident file present in the candidate tree, replace that spec's backlink block (via `PatchSet.ReplaceBlock("backlink", ...)` with `BacklinkContent` targeting the archive path) in the same plan; a spec outside the metadata tree or absent ⇒ no spec mutation, no failure.
- Board included when inline. No external cleanup: no branch/worktree/PR fields consulted or asserted; the test proves the plan's file set is exactly {archive create, active delete, [spec], [board]}.

- [ ] **Steps 1–5: TDD.** Extra tests: killed record's artifact links target the archive location; archive filename date comes from the injected clock; evolution acceptance — feed the before/after states through `repository.ValidateEvolution` in the test and assert no identity-reuse finding for the move (learnings: `relocation-reads-as-identity-reuse`).
- [ ] **Step 6: Commit** `feat(0312): change kill operation with atomic archive move (task 10)`.

---

### Task 11: `learning record` and `learning update`

**Files:**
- Create: `internal/app/learning_ops.go`
- Test: `internal/app/learning_ops_test.go`
- Modify: `internal/app/shadow_test.go`

**Interfaces:**
- Consumes: `render.LearningRecord`, `ApplySectionEdits` with `LearningOwnedHeadings`, `document.PatchSet`, config `learnings.enabled`.
- Produces:

```go
const (
	OperationLearningRecord = "learning.record"
	OperationLearningUpdate = "learning.update"
)
type LearningRecordRequest struct {
	RequestID string `json:"request_id"`
	Slug string `json:"slug"`
	Hook string `json:"hook"`
	Topics []string `json:"topics"`
	Changes []int `json:"changes"`
	Apply string `json:"apply"`
	WarStory string `json:"war_story"`
}
type LearningUpdateRequest struct {
	Path string `json:"path"`
	Version string `json:"version"`
	Hook string `json:"hook"`
	Topics []string `json:"topics"`
	Changes []int `json:"changes"`
	Sections []SectionEditRequest `json:"sections"` // Apply / War story
}
type LearningResult struct {
	Envelope
	Slug string `json:"slug,omitempty"`
	Path string `json:"path,omitempty"`
	Revision string `json:"committed_revision,omitempty"`
	Replayed bool `json:"replayed,omitempty"`
	Findings []StatusFinding `json:"findings"`
}
func LearningRecordOp(ctx context.Context, deps PlanningDeps, repoDir string, req LearningRecordRequest) LearningResult
func LearningUpdate(ctx context.Context, deps PlanningDeps, repoDir string, req LearningUpdateRequest) LearningResult
```

Behavior:
- Both refuse with `unsupported-config` at preflight when `learnings.enabled` is not true.
- record: slug shape-validated (reuse the domain learning slug rule); duplicate canonical slug in the candidate corpus ⇒ refusal, never overwrite; new file at `<learnings dir>/<slug>.md` (derive the directory from config the way `status_git.go`'s `corpusPrefixes` does) with `promotion_state: retained`, empty `promoted_to`, created=updated=clock date; idempotency key + receipt as Task 7.
- update: exact path+blob expectation; complete desired hook/topics/changes via SetField; sections via the editor; identity, `created`, `promotion_state`, `promoted_to`, unknown fields, unknown sections preserved byte-identically; `updated` changes only when some semantic input differs from the current record (byte-compare the planned record against the source before setting it — otherwise return the engine's `no-op`).
- Neither operation's plan contains the learnings README/index path — assert the plan file set is exactly {learning record} (spec table; the index stays byte-untouched).

- [ ] **Steps 1–5: TDD, `-count=1`, shadow registration.**
- [ ] **Step 6: Commit** `feat(0312): manual learning record and update operations (task 11)`.

---

### Task 12: `adr record`

**Files:**
- Create: `internal/app/adr_ops.go`
- Test: `internal/app/adr_ops_test.go`
- Modify: `internal/app/shadow_test.go`

**Interfaces:**
- Consumes: `render.ADRRecord`, `render.ADRIndex`, `render.ArtifactBlockContent`, `domain.NextADRID`, `domain.ValidateADRGraph`, `document.PatchSet`.
- Produces:

```go
const OperationADRRecord = "adr.record"
type ADRRecordRequest struct {
	RequestID string `json:"request_id"`
	Title string `json:"title"`
	Context string `json:"context"`
	Decision string `json:"decision"`
	Consequences string `json:"consequences"`
	Alternatives string `json:"alternatives"`
	RelatesTo []int `json:"relates_to"`
	Change *ADRProducingChange `json:"change,omitempty"`
}
type ADRProducingChange struct {
	ID int `json:"id"`
	Path string `json:"path"`
	Version string `json:"version"` // exact blob id of the producing change
}
type ADRResult struct {
	Envelope
	ID int `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
	Revision string `json:"committed_revision,omitempty"`
	Replayed bool `json:"replayed,omitempty"`
	Findings []StatusFinding `json:"findings"`
}
func ADRRecordOp(ctx context.Context, deps PlanningDeps, repoDir string, req ADRRecordRequest) ADRResult
```

Behavior:
- Per attempt: id = `domain.NextADRID(snapshot)` (max+1, never gap-fill — test with a gapped ADR set); slug from title; new Accepted ADR at `docs/adrs/NNNN-slug.md` with clock date; `relates_to` validated against the candidate ADR set.
- Producing change supplied ⇒ entity expectation on its exact version; append the new ADR id to its typed `adrs` collection (complete new value via SetField), update its date, rerender its artifact block — same plan.
- Every plan includes `docs/adrs/README.md` rendered by `render.ADRIndex` from the candidate ADR snapshot (spec: every ADR operation, unconditionally).
- Idempotency key + receipt (`{"op":"adr.record","id":N,"path":"..."}`); replay/digest-reuse semantics as Task 7.

- [ ] **Steps 1–5: TDD.** Assert plan file sets: {ADR, index} and {ADR, index, change} with/without producing change.
- [ ] **Step 6: Commit** `feat(0312): adr record operation with atomic index (task 12)`.

---

### Task 13: `adr supersede` and `adr reverse`

**Files:**
- Modify: `internal/app/adr_ops.go`, `internal/app/adr_ops_test.go`, `internal/app/shadow_test.go`

**Interfaces:**
- Consumes: `domain.Supersede`, `domain.Reverse` (they gate the target's status and produce the flip — read `adrStatusFlip` in `internal/domain/adr.go` for what the new status strings are), Task 12's machinery.
- Produces:

```go
const (
	OperationADRSupersede = "adr.supersede"
	OperationADRReverse   = "adr.reverse"
)
type ADRReplaceRequest struct {
	RequestID string `json:"request_id"`
	Target ADRTarget `json:"target"`
	Successor ADRRecordRequest `json:"successor"` // RequestID inside ignored; outer one governs
}
type ADRTarget struct {
	ID int `json:"id"`
	Path string `json:"path"`
	Version string `json:"version"` // exact blob id of the Accepted target
}
func ADRSupersede(ctx context.Context, deps PlanningDeps, repoDir string, req ADRReplaceRequest) ADRResult
func ADRReverse(ctx context.Context, deps PlanningDeps, repoDir string, req ADRReplaceRequest) ADRResult
```

Behavior:
- Target must be Accepted (domain refusal → `invalid-state`); exact-version expectation on the target file.
- New Accepted ADR carries `supersedes: [target]` (or `reverses:`); the OLD ADR changes ONLY its `status:` field via `PatchSet.SetField` — the spec freezes the accepted body, and `repository.ValidateEvolution`'s frozen-ADR check enforces it; test by asserting the old file's planned bytes differ from source only in the status value.
- Status spellings must be what the evolution validator and `render-adr-index.sh` grouping expect: `Superseded by ADR-NNNN` / `Reversed by ADR-NNNN` — confirm against `statusFlipFindings` in `internal/repository/evolution.go` and the index's group classifier before hardcoding.
- Optional producing change updated exactly as Task 12. Plan file set: {new ADR, old ADR, index, [change]}.

- [ ] **Steps 1–5: TDD** including: refusal on a non-Accepted target; frozen-body preservation; index reflects both the flip and the new ADR in one plan.
- [ ] **Step 6: Commit** `feat(0312): adr supersede and reverse operations (task 13)`.

---

### Task 14: CLI — `docket change` command family

**Files:**
- Create: `internal/cli/change.go`
- Test: `internal/cli/change_test.go`
- Modify: `internal/cli/root.go` (register), `internal/cli/install.go` if command/asset gating requires it (mirror how `status` was registered — read the 0310 pattern in `root.go` first)

**Interfaces:**
- Consumes: Tasks 7–10 operations, the existing presenter/JSON-mode plumbing in `internal/cli/presenter.go` / `jsonmode.go`.
- Produces commands (exact spelling, settled here):
  - `docket change create --request <path|->`
  - `docket change groom --request <path|->`
  - `docket change block --request <path|->`
  - `docket change defer --request <path|->`
  - `docket change kill --request <path|->`

Behavior:
- `--request` names a JSON file or `-` for stdin; the payload decodes into the operation's request struct with `json.Decoder.DisallowUnknownFields()` — an unknown field is `invalid-input`. Authored Markdown rides inside the JSON strings and is never interpolated into any shell command.
- Global `--json`/human presentation reuse the 0310 presenter: JSON mode emits exactly one document on stdout, diagnostics to stderr, exit code from `app.ExitCode`.
- Human mode prints a short outcome line (operation, id/slug, result, committed revision) plus findings — model on `status_human.go`, keep minimal.
- Commands construct `PlanningDeps` from the real client/engine/reader/clock (system clock satisfies `transaction.Clock`).

- [ ] **Step 1: Failing CLI tests**: registration (command exists, correct flag set), unknown-JSON-field rejection, stdin path, JSON single-document output for a scripted fake operation (inject via the same seam pattern `root_test.go` uses — read it first; if commands call `app` directly, test at the boundary with a temp real repo minimally, and lean on Task 16 for full paths).
- [ ] **Steps 2–4: implement, green `-count=1`. Step 5: Commit** `feat(0312): change command family (task 14)`.

---

### Task 15: CLI — `docket learning` and `docket adr` command families

**Files:**
- Create: `internal/cli/learning.go`, `internal/cli/adr.go`
- Test: `internal/cli/learning_test.go`, `internal/cli/adr_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Consumes: Tasks 11–13; identical request-file conventions as Task 14.
- Produces: `docket learning record|update --request <path|->`, `docket adr record|supersede|reverse --request <path|->`.

- [ ] **Steps 1–5: TDD exactly as Task 14. Commit** `feat(0312): learning and adr command families (task 15)`.

---

### Task 16: Real-git concurrency, idempotency, and mode matrix; budgets; full-suite gate

**Files:**
- Create: `internal/app/planning_git_test.go`
- Modify: `tests/runtime-budgets.tsv` (rows for any new shell-visible test shard; check whether Go package tests are budgeted via an existing `tests/test_go_*.sh` wrapper — read `tests/README.md` and mirror the 0309/0310 arrangement)

**Interfaces:**
- Consumes: everything; the real-git harness patterns in `internal/repository/transaction/harness_test.go` and `internal/app/status_git_test.go` (temp repos + bare remotes, both `main` and `docket` metadata modes).

Cover the spec's "Concurrent repository tests" verbatim, in both repository modes:
- [ ] unrelated concurrent mutations (e.g. `change block` on A ∥ `change defer` on B) both land; both authored decisions survive; board reflects the final winner's candidate snapshot;
- [ ] two operations against the same submitted entity version: one `applied`, one `contended` (typed, with contended paths);
- [ ] concurrent `change create` ∥ `change create` and `adr record` ∥ `adr record`: no duplicate ids (run N=4 goroutines, assert distinct allocated ids and a valid final corpus);
- [ ] derived views never trail sources: after every winning commit, re-read the remote tree and assert board/index/artifact block match a fresh render of the committed snapshot;
- [ ] a renderer/validation refusal pushes nothing: inject a request whose candidate fails whole-repository validation (e.g. groom pointing `depends_on` at a missing id at the domain-validation layer) and assert the remote ref is unchanged;
- [ ] each success = one commit with explicit paths, and the transactions root is left clean (no candidate worktrees — mirror `cleanup_test.go`'s assertion);
- [ ] idempotent replay end-to-end: run `change create`, sever the response (drop the result), re-run the identical request, assert `Replayed`, same id, single commit; re-run with same request-id + different title ⇒ `invalid-input`;
- [ ] kill end-to-end: archive move lands, spec backlink retargeted, active path gone, board updated, no feature-branch state touched.

Then:
- [ ] **Budgets:** run the new Go tests under the suite runner; if any shell-visible test file approaches its wall-clock budget, split or re-budget with a ledger note (learnings: `budget-headroom-is-spent-before-it-is-breached` — a row AT its ceiling is already spent).
- [ ] **Full-suite gate:** run `scripts/run-tests.sh` (background to a log if it nears the 600s foreground ceiling; key on exit code). Fix regressions. Treat any `OVER BUDGET:` line as a finding to act on, not noise.
- [ ] **Mutation-probe sweep:** re-run the Task 1 and Task 2 probes plus one new one (delete the frozen-ADR body preservation in Task 13's planner; the frozen-body test must redden) — all with `-count=1`.
- [ ] **Commit** `test(0312): concurrency, idempotency, and mode matrix; budget accounting (task 16)`.

---

## Self-review notes (spec coverage)

- Ten operations → Tasks 7–13; CLI seams → Tasks 14–15 (acceptance 1).
- Owned-field/section-only edits, no whole-record replacement → Tasks 1, 8–11, 13 source-preservation tests (acceptance 2).
- Canonical new records, idempotent creation, fresh-state retries → Tasks 2, 7, 11, 12, 16 (acceptance 3).
- One validated transaction commit per mutation incl. derived views → every operation task's plan-file-set assert + Task 16 (acceptance 4).
- Learning index untouched → Task 11 (acceptance 5).
- Kill = metadata transition + archive move only → Task 10 (acceptance 6).
- No 0313–0318 behavior: no claims, PR, worktree, finalize, harvest, promotion, GitHub mirror anywhere above (acceptance 7).
- Both-mode compat/failure/idempotency/concurrency tests + full suite → Task 16 (acceptance 8).
- Spec sections with deliberate implementation latitude: exact CLI spelling (settled in Task 14 as `--request <path|->`), board/section internals delegated to the named reference scripts with inventory steps rather than restated line-by-line.
