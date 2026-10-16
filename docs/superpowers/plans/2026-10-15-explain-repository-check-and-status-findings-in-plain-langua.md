<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0553 — Explain repository check and status findings in plain language](../../changes/archive/2026-10-16-0553-explain-repository-check-and-status-findings-in-plain-langua.md)**
<!-- docket:backlink:end -->

# Explain repository check and status findings in plain language — Implementation Plan

> **For agentic workers:** `docket-build` runs this plan: one tier worker per task under `docket-build-task`, then one full-suite gate.

**Goal:** Every record-level finding that `docket repository check` and `docket status` print says, in plain words, what is wrong, whether it matters, and what to do. The change adds four things:
- an `impact` part on each finding;
- one shared validation-text table;
- rewritten frontmatter-repair and generated-view text;
- a plain check header.

It also makes one code switch: check reports validator findings under their own code.

**Architecture:** Text stays at its producers, and the domain layer stays text-free.
- `reposetup.RecordLocator` names where a record really lives: `.docket/…`, the private checkout, or words where no host-absolute path may print. It builds the two hand-edit remedy shapes.
- `internal/app/validation_text.go` maps a validator `domain.Finding` to `{Message, Impact, Remedy}`. Status reads it through `validationFinding`, and check through `corpusFindings`.
- `PlanRepairs` and the app-layer derived-view producers write their own rewritten text.
- Four finding types gain `Impact`.
- A golden catalog file holds every message.

**Stack:** Go.
- Whole suite: `go run ./cmd/docket development test`.
- Focused tests: `go test -count=1 ./internal/<pkg>/ -run <Name>`. Always pass `-count=1`, because the cache can serve a stale PASS against a mutated tree (learning `cached-runner-serves-a-mutated-tree`).
- Undo a mutation probe by hand, never with `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).

**Spec:** `docs/superpowers/specs/2026-10-15-explain-repository-check-and-status-findings-in-plain-langua-design.md`, on the docket branch; the primary checkout has it at `.docket/…`. Read its §3 rules and §4 wording table before writing text.

## Global Constraints

- **Codes stay stable.** The one exception is that check reports a validator finding under its own code instead of `frontmatter-manual-review`. `RepairDigest`, the retired-vocabulary guard, and JSON consumers keep their codes.
- **Behavior stays put.** No new blocking gate and no `--explain` flag. Severities, exits (`CheckExit`), detection, and repair behavior are unchanged.
- **Message:** one sentence naming the record and value, with fields in backticks.
- **Impact:** one sentence on what goes wrong if left alone. If nothing does, say "It's harmless", and add how the problem usually arises when that is known. Every claim must be true of the code: soften to the truth and never invent a consequence.
- **Remedy:** must work as typed, in the exact state that produced it (learning `printed-remedy-state-validity`).
  - A hand edit names the file and field.
  - Never suggest hand-editing BOARD.md, the ADR index, `## Artifacts`, or `docket:backlink`. The one exception: a `*-malformed` remedy says how to restore the markers.
  - Name a docket command wherever one can do the work.
  - A hand-edit-only fix ends with "then run `docket repository repair`".
- **Locations come from the resolved layout.** `docket status` never prints a host-absolute path (`StatusResult.HumanText` doc comment), so status names a private checkout in words. Check prints the path, as the topology findings do.
- **No internal vocabulary** in user-facing strings: "canonical render", "roster", "scalar", "corpus", "lease", "snapshot", "postcondition", "final". Say "done or killed" instead of "final".
- **JSON fields:**
  - `Impact` is `json:"impact,omitempty"` on `reposetup.Finding` and `StatusFinding`.
  - `message` keeps only the what-is-wrong sentence.
  - The new `RepairFinding` fields are `json:"-"`, so the `repository repair --json` output is unchanged.
- **Out of scope:**
  - topology findings in `internal/reposetup/health.go`;
  - config diagnostics;
  - `artifact-missing` and `branch-malformed`;
  - evolution and transaction-scope codes;
  - change 539.
- **Comments** anchor on symbols, never on line numbers (ADR-0054).
- **No finding-code literals in package `app`** outside `finding_codes.go` (`TestNoInlineFindingCodeLiterals`). Use the `Code*` constants.

## Review Focus

1. **Private `docket status` never prints a host-absolute path.** Pinned by Task 5 `TestStatusValidationTextNeverPrintsPrivateCheckoutPath`.
2. **ADR-graph findings carry only an id, but their remedy still names the file.** Task 4 adds `Detail["path"]`, and Task 5 `TestExplainValidationADRGraphNamesFile` pins it.
3. **An unlisted code (an evolution or transaction-scope code arriving through `change.halt`) keeps today's text, never an empty message.** Pinned by Task 5 `TestExplainValidationFallbackKeepsCodeAndDetail`.
4. **A blank or unknown state never prints `repository check:  ()`.** Pinned by Task 3 `TestCheckHeaderUnknownAndBlankState`.
5. **A cycle finding (no path) never prints an empty file reference.** Pinned by Task 2 `TestRecordLocatorEmptyRelNamesNoFile` and Task 5 `TestExplainValidationCycleNamesMembersNotEmptyFile`.

## File Structure

New files:
- `internal/reposetup/findingtext.go`
- `internal/app/validation_text.go`
- `internal/app/validation_text_test.go`
- `internal/app/finding_catalog_test.go`
- `internal/app/testdata/finding-catalog.txt`

Each task's **Files** block lists its edits to existing files.

---

### Task 1: The `impact` part — fields and both renderers

**Tier:** economy

**Files:**
- Modify: `internal/reposetup/health.go` (`Finding`)
- Modify: `internal/app/status_result.go` (`StatusFinding`)
- Modify: `internal/app/config_diagnostics.go` (`appendFindingBlock`)
- Modify: `internal/app/status_human.go` (`writeFinding`)
- Tests: `internal/app/config_diagnostics_test.go`, `internal/app/status_human_test.go`, new `internal/app/finding_impact_schema_test.go`

**Produces:**
- `reposetup.Finding.Impact` and `StatusFinding.Impact`, each `string` with tag `json:"impact,omitempty"`, declared directly after `Message`.
- Both renderers print a non-empty impact on its own line, between the message and the remedy.

- [ ] **Step 1: Write failing tests**

- **`TestAppendFindingBlockPrintsImpactBetweenMessageAndRemedy`** (in `config_diagnostics_test.go`):
  - `appendFindingBlock` of `{Code: "drop-final-claimed-at", Severity: warning, Ref: "docs/x.md", Message: "what is wrong", Impact: "whether it matters", Remedy: "what to do"}` renders exactly `"\n- [warning] drop-final-claimed-at (docs/x.md)\n  what is wrong\n  whether it matters\n  remedy: what to do"`.
  - A finding with no impact still renders exactly `"\n- [error] c\n  m\n  remedy: r"`.
- **`TestWriteFindingPrintsImpactContinuation`** (in `status_human_test.go`): `writeFinding` of `{Code: "change-reference-dangling", Entity: "change", Identity: "0263", Field: "depends_on", Message: "msg", Impact: "imp", Remedy: "rem"}` renders exactly `"  change-reference-dangling change 0263 (depends_on) — msg\n    imp\n    remedy: rem\n"`.
- **`TestSchemaPublishesFindingImpact`** (new file `finding_impact_schema_test.go`): `schemaKeyPaths(t)`, from `schema_revision_test.go`, contains both `"status RES findings.impact"` and `"repository.check RES findings.impact"`.

If `schemaKeyPaths` spells array elements differently, print the keys that contain `findings.` once and use the real spelling. Do not loosen the check to a substring match.

- [ ] **Step 2: Run the tests**

Run: `go test -count=1 ./internal/app/ -run 'TestAppendFindingBlockPrintsImpact|TestWriteFindingPrintsImpact|TestSchemaPublishesFindingImpact'`

Expected: compile failure (`unknown field Impact`).

- [ ] **Step 3: Implement**

Add the two fields.

- **`appendFindingBlock`:** after the message clause, add `if f.Impact != "" { fmt.Fprintf(b, "\n  %s", f.Impact) }`. Update the doc comment to name the message, impact, and remedy lines.
- **`writeFinding`:** after the `" — %s\n"` row, print `strings.TrimSpace(f.Impact)`, when non-empty, as `"    %s\n"`. Add one sentence about it to the doc comment.

- [ ] **Step 4: Verify**

Run: `go test -count=1 ./internal/app/ -run 'TestAppendFindingBlock|TestAppendConfigFindingBlock|TestRefusalAndCheckShareFindingRenderer|TestWriteFinding|TestSchema'`

Expected: PASS.

Mutation probe: delete the impact clause in `appendFindingBlock`. The new test must go red. Restore the clause.

- [ ] **Step 5: Commit**

Message: `feat(findings): carry an impact sentence on check and status findings (change 0553)`. Stage the touched files explicitly.

---

### Task 2: `RecordLocator` — real record locations and the two remedy shapes

**Tier:** standard

**Files:**
- Create: `internal/reposetup/findingtext.go`
- Create: `internal/reposetup/findingtext_test.go`

**Produces** (used by Tasks 5–9):
- `type RecordLocator struct{ Checkout, Label string }`
- `(RecordLocator) HandEditThenRepair(instruction, rel string) string`
- `(RecordLocator) RepairOrHandEdit(action, instruction, rel string) string`
- `PlainDocumentError(err error) string`

Parameters:
- `instruction` is a lowercase imperative clause, for example "remove `172` from `depends_on`".
- `rel` is the record's metadata-branch path. An empty `rel` means the edit spans several records, so no file is named.

- [ ] **Step 1: Write failing tests**

Let `tail` be ", then run `docket repository repair` so the board and link blocks catch up." Use `instruction` = "remove `172` from `depends_on`" and `rel` = `docs/changes/active/0263-x.md`.

**`TestRecordLocatorHandEditThenRepairPerLayout`** pins `HandEditThenRepair(instruction, rel)` exactly, for each locator:

| Locator | Expected result |
|---|---|
| `{Checkout: ".docket"}` | "Remove `172` from `depends_on` in `.docket/<rel>`, commit and push it from `.docket`" + tail |
| `{Checkout: P}`, where P = `/home/u/.local/share/dckt/o/r/checkouts/r-1` | "… in `P/<rel>`, commit and push it from `P`" + tail |
| `{Label: "your private metadata checkout"}` | "… in `<rel>` in your private metadata checkout, commit and push it from your private metadata checkout" + tail |
| `{}` | "… in `<rel>` on the metadata branch, commit and push it" + tail |

**`TestRecordLocatorEmptyRelNamesNoFile`:** with an empty `rel`, the result contains neither "``" nor " in `", and does contain "commit and push it from `.docket`".

**`TestRecordLocatorRepairOrHandEdit`:** `{Checkout: ".docket"}.RepairOrHandEdit("remove it", "delete the `claimed_at:` line", rel)` returns exactly:

"Run `docket repository repair` to remove it (it shows the edit first), or delete the `claimed_at:` line in `.docket/<rel>` by hand and commit and push it from `.docket`."

**`TestPlainDocumentError`:**
- `&document.Error{Kind: document.KindMarkerImbalance, Name: "artifacts", Line: 12, Msg: "start marker has no matching end"}` renders as "start marker has no matching end (`artifacts`) on line 12".
- `errors.New("boom")` renders as "boom".

- [ ] **Step 2: Run the tests**

Run: `go test -count=1 ./internal/reposetup/ -run 'TestRecordLocator|TestPlainDocumentError'`

Expected: compile failure.

- [ ] **Step 3: Implement `internal/reposetup/findingtext.go`**

Start with a file comment explaining why the locator exists. A path on the metadata branch is not where the record gets edited (that is `.docket` in a shared repo, or outside the clone in a private one), and status must not print host-absolute paths.

`type RecordLocator struct { Checkout string; Label string }`
- `Checkout` is `.docket`, or the private checkout path.
- `Label` names the checkout in words. It is used only when `Checkout` is empty.
- The zero value names the path "on the metadata branch".

Unexported helpers:
- **`where(rel)`**, tried in order:
  - "" when rel is empty
  - "`" + `filepath.Join(Checkout, filepath.FromSlash(rel))` + "`"
  - "`rel` in " + Label
  - "`rel` on the metadata branch"
- **`commitClause()`**, tried in order:
  - "commit and push it from `" + Checkout + "`"
  - "commit and push it from " + Label
  - "commit and push it"

  Comment why: check and status read the pushed branch, so an uncommitted edit fixes nothing.

Exported:
- **`HandEditThenRepair(instr, rel)`** returns `capitalizeFirst(instr)`, then " in " + where when where is non-empty, then ", " + commitClause + ", then run `docket repository repair` so the board and link blocks catch up."
- **`RepairOrHandEdit(action, instr, rel)`** returns "Run `docket repository repair` to " + action + " (it shows the edit first), or " + instr, then " in " + where when non-empty, then " by hand and " + commitClause + "."
- **`PlainDocumentError(err)`**: for a `*document.Error`, found with `errors.As`, return `Msg`, then " (`Name`)" when Name is set, then " on line N" when Line > 0. Any other error returns `err.Error()`.

Give each exported symbol a doc comment saying when to use it.

- [ ] **Step 4: Verify**

Run the same command as Step 2.

Expected: PASS.

- [ ] **Step 5: Commit**

Message: `feat(reposetup): name a record's real location in hand-edit remedies (change 0553)`.

---

### Task 3: The check header names the setup state and counts findings

**Tier:** economy

**Files:**
- Modify: `internal/app/repository_check.go` (`HumanText`, plus new `checkHeader`, `checkStatePhrases`, and `findingCounts`)
- Modify: `internal/app/config_diagnostics_test.go`
- Modify: `internal/app/repository_check_test.go`
- Modify: `internal/bashupgrade/registry_test.go`
- Create: `internal/app/repository_check_header_test.go`

**Produces:**
- `checkHeader(state string, findings []reposetup.Finding) string`
- `var checkStatePhrases map[reposetup.State]string`
- `stateConstants(t) []string`, a test helper

**Unchanged:**
- the refusal header from `checkGatherFailure`, `repository check: <result>: <error>`
- the JSON envelope

- [ ] **Step 1: Write failing tests**

`stateConstants` uses `go/parser` to read `../reposetup/classify.go` (resolved through the existing `appPackageDir(t)`). It returns the value of every const whose type is `State`, and fails if it finds fewer than 7. That makes it a shape-keyed guard, not a hand-written list.

Tests:

- **`TestCheckStatePhrasesCoverEveryState`:**
  - Every `stateConstants` value has a non-empty `checkStatePhrases` entry.
  - Every map key is one of those values. This is the reverse direction (learning `correspondence-guard-runs-one-way`).
- **`TestCheckHeaderForms`** pins these headers exactly:

  | State | Findings | Header |
  |---|---|---|
  | `healthy` | none | `repository check: setup healthy, no problems found` |
  | `healthy` | one warning | `repository check: setup healthy, 1 warning (read-only, nothing was changed)` |
  | `legacy` | warning, error, warning | `repository check: old single-branch layout, 1 error, 2 warnings (read-only, nothing was changed)` |
  | `fresh` | none | `repository check: not set up yet (read-only, nothing was changed)` |

- **`TestCheckHeaderUnknownAndBlankState`:** for each of `""`, `"bogus"`, and `"unknown"` with one error, the header is exactly `repository check: setup state unknown, 1 error (read-only, nothing was changed)`.

- [ ] **Step 2: Run the tests**

Run: `go test -count=1 ./internal/app/ -run 'TestCheckStatePhrases|TestCheckHeader'`

Expected: compile failure.

- [ ] **Step 3: Implement**

`HumanText` now writes `b.WriteString(checkHeader(r.RepositoryState, r.Findings))` in place of the old `fmt.Fprintf` header. Update its doc comment to match.

Then add:

- **`checkStatePhrases`** maps:

  | State | Phrase |
  |---|---|
  | healthy | setup healthy |
  | fresh | not set up yet |
  | legacy | old single-branch layout |
  | partial | migration interrupted |
  | needs-review | setup waiting for your review |
  | conflict | setup conflict |
  | unknown | setup state unknown |

  Give it a doc comment naming its guard.
- **`checkHeader(state, findings)`** looks up the phrase. A missing state falls back to the unknown phrase. With `c := findingCounts(findings)`:

  | Case | Header |
  |---|---|
  | `c` is empty and the state is healthy | "repository check: <phrase>, no problems found" |
  | `c` is empty, any other state | "repository check: <phrase> (read-only, nothing was changed)" |
  | otherwise | "repository check: <phrase>, <c> (read-only, nothing was changed)" |

`findingCounts(findings) string` counts by severity and renders "1 error, 2 warnings". The order is errors, warnings, then any other severity as "N other finding(s)". It joins the parts with ", " and pluralizes with a plain "s". It returns "" when there are no findings.



Update the sites that pin the old header:
- `TestRefusalAndCheckShareFindingRenderer`: replace `const header = "repository check:  ()"` with `header := checkHeader("", findings)`, and fix its comment.
- In `repository_check_test.go`, the assert `strings.Contains(human, string(reposetup.StateFresh))` becomes `…checkStatePhrases[reposetup.StateFresh]`.
- In `internal/bashupgrade/registry_test.go`, `runRepoConfirm` expects `"repository check: setup healthy, no problems found"`.
- `grep -rn 'repository check: no-op\|(healthy)"' internal cmd tests` must return no executable hit.

- [ ] **Step 4: Verify**

Run: `go test -count=1 ./internal/app/ -run 'TestCheck|TestRefusalAndCheck|TestRepositoryCheck' && go test -count=1 ./internal/bashupgrade/`

Expected: PASS. If the bashupgrade guide test skips, note it.

Mutation probe: delete the `StatePartial` entry. `TestCheckStatePhrasesCoverEveryState` must go red. Restore the entry.

- [ ] **Step 5: Commit**

Message: `feat(check): name the setup state and count findings in the check header (change 0553)`.

---

### Task 4: ADR-graph findings carry their record's path

**Tier:** economy

**Files:**
- Modify: `internal/domain/adr.go`
- Test: `internal/domain/adr_test.go`

**Produces:**

`Detail["path"] = a.Path()` on every finding from these four producers:
- `validateADREdges` (mismatch)
- `validateADRStatusRef` (mismatch)
- `adrSelfReference`
- `adrDangling`

`a` is the referring ADR. The change is additive: the code, the Entity, and the other keys are unchanged. `adr-id-gap` gets no path, because no record exists for it. List this key in the results file (spec §2 "Missing data").

- [ ] **Step 1: Write a failing test**

```go
s := adrSnapshot(
	NewADR(ADRSpec{ID: 1, Path: "docs/adrs/0001-a.md", Status: ADRStatus{Kind: ADRReversedBy, Ref: 2}}),
	NewADR(ADRSpec{ID: 2, Path: "docs/adrs/0002-b.md", Status: ADRStatus{Kind: ADRAccepted}, Supersedes: []ADRID{1, 2, 9}}),
	NewADR(ADRSpec{ID: 3, Path: "docs/adrs/0003-c.md", Status: ADRStatus{Kind: ADRSupersededBy, Ref: 2}}),
)
```

`TestADRGraphFindingsCarryRecordPath` builds the snapshot above and runs `ValidateADRGraph(s)`. Then, using the existing `findingFor(got, code, id, field)` helper, it asserts these `Detail["path"]` values:

| Code | ADR | Field | `Detail["path"]` |
|---|---|---|---|
| `adr-backpointer-mismatch` | 2 | `supersedes` | `docs/adrs/0002-b.md` |
| `adr-self-reference` | 2 | `supersedes` | `docs/adrs/0002-b.md` |
| `adr-dangling-reference` | 2 | `supersedes` | `docs/adrs/0002-b.md` |
| `adr-backpointer-mismatch` | 3 | `status` | `docs/adrs/0003-c.md` |

If the fixture does not produce a row, fix the fixture, never the assert.

- [ ] **Step 2: Run the test**

Run: `go test -count=1 ./internal/domain/ -run TestADRGraphFindingsCarryRecordPath`

Expected: FAIL.

- [ ] **Step 3: Implement**

Add `"path": a.Path()` to the two mismatch `Detail` maps and to `adrDangling`'s map. Give `adrSelfReference` the map `Detail: map[string]string{"path": a.Path()}`. Above `adrRef`, add a one-line comment: graph findings name the ADR by id only, and its path is in `Detail["path"]`.

- [ ] **Step 4: Verify**

Run: `go test -count=1 ./internal/domain/ ./internal/repository/`

Expected: PASS. If a test compares a whole `Detail` map, add `path` to its expectation.

- [ ] **Step 5: Commit**

Message: `feat(domain): carry the referring ADR's path on graph findings (change 0553)`.

---

### Task 5: The shared validation-text table, its coverage guard, and status wiring

**Tier:** premium. This task carries most of the user-facing text.

**Files:**
- Create: `internal/app/validation_text.go`
- Create: `internal/app/validation_text_test.go`
- Modify: `internal/app/status.go` (`validationFinding`, `assembleFindings`, new `statusRecordLocator`)
- Modify: `internal/app/change_create.go` (`findingsToStatus`)

**Uses:** `RecordLocator` (Task 2), `StatusFinding.Impact` (Task 1), and the ADR `Detail["path"]` (Task 4).

**Provides to Tasks 6 and 9:**
- Types: `findingText{Message, Impact, Remedy}` and `validationExplainer func(domain.Finding, reposetup.RecordLocator) findingText`.
- Table: `validationTexts map[string]validationExplainer`.
- Functions: `explainValidation(f, loc)`, `validationRecordPath(f)`, `validationFinding(f, loc) StatusFinding`, and `statusRecordLocator(layout.Layout)`.
- Test helpers: `codeConstants`, `validationCodeSet`, `validationFixture{label, finding}`, and `validationFixtures() map[string][]validationFixture`.

- [ ] **Step 1: Write the coverage guard, fixtures, and behavior tests**

**`codeConstants(t, dir string, skip map[string]bool) map[string]string`.** Uses `os.ReadDir` plus `go/parser` over `dir`'s `.go` files. It skips `_test.go` files and `skip` names. It returns `value → "file:Name"` for every const whose name starts with `Code` and whose value is a string `BasicLit` (unquote it with `strconv.Unquote`).

**`validationCodeSet(t)`.** The spec's coverage set. It is `codeConstants` over `../domain` (no skip) plus `../repository` with `{"evolution.go": true}` skipped. Both paths are relative to `appPackageDir(t)`. The transaction subpackage is a separate directory, so it is never read.

**`TestValidationTextCoversEveryValidatorCode`.** Asserts:
1. `len(set) >= 37`.
2. The evolution exclusion removes at least one code. Compare `codeConstants` over `../repository` with and without the skip, so the skip is not vacuous.
3. Every code in the set has a `validationTexts` entry **and** a `validationFixtures()` entry. Each fixture renders non-empty `Message`, `Impact`, and `Remedy` under `RecordLocator{Checkout: ".docket"}`.
4. Every `validationTexts` key is in the set. This is the reverse direction.

**`validationFixtures()`.** One fixture per code, plus one per text branch. Mirror each producer's real shape: read `internal/repository/validate.go`, `decode.go`, `build.go`, and `internal/domain/adr.go`.

Use these fixture helpers:
- `fxChange(id, slug)`: path `docs/changes/active/%04d-<slug>.md`
- `fxArchived(id, slug)`: path `docs/changes/archive/2026-05-28-%04d-<slug>.md`
- `fxADR(id, slug)`: path `docs/adrs/%04d-<slug>.md`
- `fxLearning(slug)`: path `docs/changes/learnings/<slug>.md`

Every ADR-graph fixture's Entity is `{Kind: adr, ID: 12}` with `Detail["path"] = "docs/adrs/0012-x.md"`.

Each fixture sets exactly the Entity, Field, Severity, Related, and Detail keys its producer sets. The values are your choice, but keep them realistic.

Add one extra fixture for every branch the Step 3 text switches on:
- E vs W severity
- `depends_on`, `stacked_on`, and other fields
- `lookup=ambiguous`
- placement `active` vs `archive`
- raw empty
- edge vs status

Edge shapes to get right:
- `change-id-invalid` and `adr-id-invalid`: ID 0
- cycles: Entity `{change, ID 12}` with no Path, plus `members` and Related for every member
- `record-unaccounted`: Entity kind `repository`, no Detail
- `adr-id-gap`: Entity `{adr, ID 7}`, `highest`

**Behavior tests** (all in `validation_text_test.go`). `danglingDep()` is a change-reference-dangling finding: Entity `{change, ID 263, slug x, path docs/changes/active/0263-x.md}`, Field `depends_on`, Related `[{change, 172}]`, Detail `lookup=absent target=172`.

- `TestExplainValidationFallbackKeepsCodeAndDetail`: `explainValidation({Code: "evolution-something", Detail: {b:2, a:1}}, RecordLocator{})` returns Message exactly `"evolution-something (a=1, b=2)"`, with an empty Impact and an empty Remedy.
- `TestExplainValidationDanglingDependency` (the spec's worked example), under `{Checkout: ".docket"}`:
  - Message is exactly "Change 0263 lists change 0172 under `depends_on`, but no change 0172 exists."
  - Impact contains "never becomes ready".
  - Remedy contains "`.docket/docs/changes/active/0263-x.md`".
- `TestExplainValidationADRGraphNamesFile`: for an `adr-self-reference` with Entity `{adr, ID 12}` (no path) and `Detail["path"]="docs/adrs/0012-x.md"`, the Remedy contains "`.docket/docs/adrs/0012-x.md`".
- `TestExplainValidationCycleNamesMembersNotEmptyFile`: for a `change-dependency-cycle` with Entity `{change, ID 12}` (no path) and members `0012,0013`, the Remedy contains "0012" and "0013" and no "``".
- `TestStatusValidationTextNeverPrintsPrivateCheckoutPath`:
  - Setup: `lay := layout.PrivateLayout("/repo/.git", "/repo", "/home/u/.local/share", "o/r")`, then `sf := validationFinding(danglingDep(), statusRecordLocator(lay))`.
  - No part of `sf` contains "/home/u" or `lay.MetadataWorktree`.
  - The Remedy contains "private metadata checkout".
  - The shared layout (`layout.SharedLayout("/repo/.git", "/repo")`) gives a Remedy that contains "`.docket/docs/changes/active/0263-x.md`".

- [ ] **Step 2: Run the tests**

Run: `go test -count=1 ./internal/app/ -run 'TestValidationText|TestExplainValidation|TestStatusValidationText'`

Expected: compile failure.

- [ ] **Step 3: Implement `internal/app/validation_text.go`**

Write a file header comment that covers:
- the file is the one plain-language table for the validator codes that reach status and check (change 0553);
- the domain and repository layers stay text-free;
- status (`validationFinding`) and check (`corpusFindings`) both call `explainValidation`, so the two cannot word a code differently;
- `TestValidationTextCoversEveryValidatorCode` keeps the table in step with the Code constants, which it parses from source.

`explainValidation` returns `validationTexts[f.Code](f, loc)` when the code has an entry. Otherwise it returns `findingText{Message: findingMessage(f)}`, today's text, which is never empty. `validationRecordPath` returns `f.Entity.Path`, else `f.Detail["path"]`, because ADR-graph findings name the ADR by id only.

Helpers, each a few lines:
- **`subjectOf(e EntityRef)`:**
  - change: "Change %04d" (id > 0) or "This change record"
  - adr: "ADR %04d" or "This ADR record"
  - learning: "The learning `slug`" or "This learning record"
  - otherwise: "`path`" or "This record"
- **`lowerSubject(e)`:** the same text with its first letter lowercased.
- **`refName(e)`:** "ADR %04d" for kind adr, else "change %04d".
- **`relatedTarget(f)`:** `Related[0]`, or else a change ref with id `Atoi(Detail["target"])`.
- **`missingTarget(f, target)`:** "but more than one record claims <target>" when `Detail["lookup"] == "ambiguous"`, else "but no <target> exists".
- **`rawValue(raw)`:** "an empty value" when empty, else "`raw`".
- **`handEdit(f, loc, instr)`:** `loc.HandEditThenRepair(instr, validationRecordPath(f))`.
- **`countOthers(f)`:** `Atoi(Detail["count"]) - 1`, with a minimum of 1.
- **`cycleMembers(f)`:** renders `Detail["members"]` as "0012, 0013, and 0014" or "0012 and 0013".
- **`const harmlessLink`:** "It's harmless: the link just leads nowhere, and nothing waits on it."
- **`const slugRule`:** "lowercase letters and digits in words joined by single hyphens"

Then write `validationTexts`, keyed by the `repository.Code*` and `domain.Code*` constants. Each entry is a closure that returns `findingText`.

The table below gives the exact wording. In it:
- `S` = `subjectOf(f.Entity)`
- `F` = `f.Field`
- `D[k]` = `f.Detail[k]`
- `T` = `refName(relatedTarget(f))`
- **HE(x)** = `handEdit(f, loc, x)`, with *x* the lowercase instruction shown

An example entry:

```go
repository.CodeChangeFinalClaimStamp: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
	return findingText{
		Message: fmt.Sprintf("%s is %s but still has `claimed_at: %s` from when it was being built.", subjectOf(f.Entity), f.Detail["status"], f.Detail["raw"]),
		Impact:  "It's harmless: nothing reads it once a change is closed. Older docket versions left it behind.",
		Remedy:  loc.RepairOrHandEdit("remove it", "delete the `claimed_at:` line", validationRecordPath(f)),
	}
},
```

| Code | Message | Impact | Remedy |
|---|---|---|---|
| `record-undecodable` | "docket can't read the frontmatter of `<path>`: <D[error]>." | "docket skips this record, so it is missing from the board and from every other check." | HE("fix the frontmatter between the two `---` lines") |
| `field-malformed` | "S has `F: <raw>`, which isn't a valid value for that field." (raw empty: "S has a `F` value docket can't read.") | E: "docket ignores the value, so everything that reads `F` sees it as missing." W: "It's mostly harmless: docket treats `F` as unset." | HE("fix the `F:` value") |
| `list-item-malformed` | "S's `F` list has the entry <rawValue> (position <index+1>), which docket can't read." | "docket skips that entry and keeps the rest of the list." | HE("fix or remove that entry in `F:`") |
| `change-type-invalid` | "S has `type: <raw>`, which isn't <slugRule>." | "docket can't label or group the change by its type." | HE("set `type:` to a single word or hyphenated words, such as `feature` or `fix`") |
| `adr-status-unparseable` | "S has `status: <rawValue>`, which isn't an ADR status." | "docket can't tell whether the decision is in force, so the ADR index can't list it correctly." | HE("set `status:` to `Accepted`, `Deprecated`, `Superseded by ADR-NNNN`, or `Reversed by ADR-NNNN`") |
| `learning-promotion-unknown` | "S has `promotion_state: <rawValue>`, which isn't a promotion state." | "docket can't tell whether the learning was promoted, so the learnings index can't place it." | HE("set `promotion_state:` to `retained`, `candidate`, or `promoted`") |
| `change-id-invalid`, `adr-id-invalid` | "S has `id: <raw>`, which isn't a usable change number." (ADR: "…usable ADR number.") | change: "Nothing can refer to this change by number, so it can't be selected, depended on, or built." ADR: "Other ADRs and changes can't refer to this decision, and the ADR index can't list it." | HE("set `id:` to the number at the start of the file name") |
| `change-slug-invalid`, `adr-slug-invalid`, `learning-slug-invalid` | "S has `slug: <rawValue>`, which isn't <slugRule>." | change: "docket names the change's branch after its slug, so it refuses to claim this change for building." (verify against `domain.ValidSlugToken`'s claim-time use) ADR: "The ADR's file name and links can't be built from its slug." learning: "The learnings index can't link the learning by its slug." | HE("set `slug:` to <slugRule>, and rename the file to match") |
| `change-status-unknown` | "S has `status: <raw>`, which isn't a change status." (raw empty: "S has no `status:`.") | "docket can't place the change on the board or decide whether it is ready to build." | HE("set `status:` to one of proposed, in-progress, blocked, deferred, implemented, stacked-merged, done, or killed") |
| `change-priority-unknown` | "S has `priority: <rawValue>`, which isn't a priority." | "It's harmless: docket sorts the change as medium priority." | HE("set `priority:` to critical, high, medium, or low") |
| `change-filename-mismatch`, `adr-filename-mismatch` | "S's file is named `<base>`, but its frontmatter says `id: <id>` and `slug: <slug>`." | "Links and lookups that build the file name from the id and slug miss this record." | HE("make the file name and the `id:` and `slug:` fields agree") |
| `change-id-duplicate`, `adr-id-duplicate`, `learning-slug-duplicate` | "S's number (learning: slug) is also used by <n> other record(s)." | change: "docket can't tell these changes apart, so every command that names this number refuses." ADR/learning: "docket can't tell them apart, so references to this one are ambiguous." | HE("give one of them an unused `id:` (learning: `slug:`) and rename its file to match") |
| `change-slug-duplicate` | "S uses `slug: <Entity.Slug>`, and <n> other change(s) use(s) it too." | "Their branches would get the same name, so building one can collide with the other." | HE("give one of them a different `slug:` and rename its file to match") |
| `record-path-duplicate` | "docket was handed `<path>` <count> times." | "This is a docket bug, not a problem in the record: lookups of this record report it as ambiguous." | "Report it as a docket bug and include the path; the record itself doesn't need an edit." |
| `change-placement-invalid` | active: "S is `<status>` but its file is still in `active/`." archive: "S's file is in `archive/`, but its status is `<status>`, not done or killed." | active: "The board and the backlog keep showing a change that is closed." archive: "docket treats the change as closed even though its status says it is still open." | active: HE("move the file into `archive/` with today's date in front of its name (`YYYY-MM-DD-`), or set `status:` back to the state the change is really in"). archive: HE("set `status:` to `done` or `killed`, or move the file back into `active/` without its date prefix") |
| `change-archive-date-invalid` | "The archived file `<base>` doesn't start with a `YYYY-MM-DD-` date." | "docket can't tell when the change was archived, so it sorts out of order in the archive." | HE("rename the file to start with the date it was archived, such as `2026-05-28-`") |
| `change-final-claim-stamp` | see example above | see example | see example (`RepairOrHandEdit`: repair removes it) |
| `change-state-incoherent` | "S is `<status>` but has no `F`." | "Commands that rely on a `<status>` change having `F` can't work on this change." | HE("add the missing `F:` value, or set `status:` to the state the change is really in") |
| `change-reference-dangling` | "S lists T under `F`, <missingTarget>." | depends_on: "docket counts <lowerSubject> as waiting on T, so it never becomes ready to build." stacked_on: "docket can't find the change this one is built on, so it can't build or finish this change." other: harmlessLink | HE("remove `<id>` from `F:` or correct it to the right number") |
| `change-dependency-cycle` | "Changes <cycleMembers> depend on each other in a loop through `depends_on`." | "None of them can ever become ready to build, because each waits on another." | `loc.HandEditThenRepair("remove one `depends_on` link between changes <cycleMembers> in its change record", "")` |
| `change-stack-cycle` | "Changes <cycleMembers> are stacked on each other in a loop through `stacked_on`." | "None of them has a base branch to build on, so none can be built or finished." | `loc.HandEditThenRepair("remove or correct one `stacked_on` value among changes <cycleMembers> in its change record", "")` |
| `artifact-reference-kind-mismatch` | "S's `F: <D[path]>` points at a <D[supplied-as]> record, not a spec, plan, or results file." | "docket can't open or link it as the change's F." | HE("point `F:` at the right file, or clear it") |
| `learning-filename-mismatch` | "The learning file `<base>` doesn't match its `slug: <slug>`." | "Links that build the file name from the slug lead nowhere." | HE("rename the file to `<slug>.md` or fix `slug:` to match the file name") |
| `learning-topic-invalid` | "S lists the topic <rawValue>, which isn't <slugRule>." | "The learnings index can't group the learning under that topic." | HE("fix or remove that entry in `topics:`") |
| `learning-promotion-destination-invalid` | E: "S is marked promoted, but `promoted_to:` is empty." W: "S names `promoted_to: <raw>`, but its `promotion_state` is `<state>`, not promoted." | E: "Nobody can tell which instructions file the rule was promoted into." W: "It's harmless: docket ignores `promoted_to:` unless the learning is promoted." | E: HE("set `promoted_to:` to the file it was promoted into, or change `promotion_state:`"). W: HE("clear `promoted_to:`, or set `promotion_state: promoted` if it really was promoted") |
| `learning-reference-dangling` | "S lists T under `changes`, <missingTarget>." | harmlessLink | HE("remove `<id>` from `changes:` or correct it to the right number") |
| `record-unaccounted` | "docket read `<path>` but couldn't match it to any record." | "This is a docket bug: the file is skipped without a reason being reported." | "Report it as a docket bug and include the path; nothing in the file needs to change." |
| `adr-backpointer-mismatch` | edge: "S lists T under `F`, but T's status is `<actual>`, not `<expected>`." status: "S has `status: <status>`, but ADR <expected-on> doesn't list <id %04d> under `<missing-edge>`." | both: "The ADR index shows the two decisions inconsistently, so a reader can't tell which one is in force." | edge: HE("remove `<target id>` from `F:`, or set T's `status:` to `<expected>` in its own file"). status: HE("fix this ADR's `status:`, or add `<id>` to `<missing-edge>:` in ADR <expected-on>'s own file") |
| `adr-dangling-reference` | "S names T in `F`, <missingTarget>." | E: "The ADR index can't connect the two decisions, so the relationship is lost." W: harmlessLink | HE("remove `<id>` from `F:` or correct it to the right number") |
| `adr-id-gap` | "There is no ADR <id %04d>, though ADR numbers go up to <highest>." | "It's harmless: docket never reuses a missing number. This usually means an ADR was deleted or never written." | "Nothing to do. If ADR <id> was deleted by mistake, restore its file from the metadata branch's git history." |
| `adr-self-reference` | "S names itself in `F`." | "A decision can't supersede, reverse, or relate to itself, so the ADR index shows a broken relationship." | HE("remove its own number from `F:`") |

Plural helpers render "1 record", "2 records", and so on.

Before committing:
- Confirm that each impact sentence is true of the code. If it is not, soften it.
- Run `gofmt -w`.
- Check that no user-facing string uses a banned word. Note that "final" must not appear even inside "finally".

Wire status in `internal/app/status.go`:
- `validationFinding(f domain.Finding, loc reposetup.RecordLocator)` fills `Message`/`Impact`/`Remedy` from `explainValidation(f, loc)`. All other fields stay as they are. Update the doc comment to say the text is the shared table's, the same text check prints.
- `assembleFindings` gains a trailing `loc reposetup.RecordLocator`. `Status` passes `statusRecordLocator(pin.Layout)`.
- Add:

  ```go
  // statusRecordLocator names where status's hand-edit remedies point: `.docket`
  // in a shared repository; a private checkout path is host-absolute, which
  // status never prints, so it is named in words.
  func statusRecordLocator(l layout.Layout) reposetup.RecordLocator {
  	if l.Mode == layout.Private {
  		return reposetup.RecordLocator{Label: "your private metadata checkout"}
  	}
  	return reposetup.RecordLocator{Checkout: layout.SharedWorktreeDir}
  }
  ```

- `findingsToStatus` (`change_create.go`) passes `reposetup.RecordLocator{}`. Mutation operations have no resolved layout at that call site, and the zero locator names the path on the metadata branch. Say so in its doc comment.
- `findingMessage` stays as the fallback. Update its comment.

- [ ] **Step 4: Verify**

Run: `go test -count=1 ./internal/app/ -run 'TestValidationText|TestExplainValidation|TestStatus|TestChangeCreate|Finding'`

Expected: PASS. A test that compared a whole validator `StatusFinding` now also sees `Impact` and `Remedy`. Update its expectation; do not delete it.

Mutation probes (restore after each):
1. Delete the `CodeChangeStackCycle` entry. The coverage guard must name it.
2. Add a phantom `"no-such-code"` entry. The reverse loop must name it.

- [ ] **Step 5: Commit**

Message: `feat(status): explain validator findings from one shared plain-language table (change 0553)`.

---

### Task 6: `repository check` reports validator findings under their own code

**Tier:** standard

**Files:**
- Modify: `internal/reposetup/repair.go` (`RepairFinding`, `FindingCode`, `CodeFrontmatterManualReview`)
- Modify: `internal/reposetup/health.go` (`frontmatterFinding`)
- Modify: `internal/app/repository_check.go`
- Modify: `internal/app/repository_repair.go`
- Modify: `internal/app/repair_remedy_family_test.go`
- Update the `corpusFindings` call sites in `repository_check_test.go` and `repository_repair_test.go`
- Tests: new `internal/app/repository_check_codes_test.go`; `internal/reposetup/health_test.go`

**Produces:**
- `RepairFinding` gains `ReviewCode`, `Impact`, and `Remedy`, each `string` with tag `json:"-"`. These are display-only: `RepairDigest` and `ApplyRepairs` never read them.
- `const CodeFrontmatterManualReview = "frontmatter-manual-review"`.
- `(RepairFinding) FindingCode() string`, used by `frontmatterFinding` and `planRepositoryRepair`.
- `checkCorpus.locator`.
- `checkRecordLocator(l layout.Layout) reposetup.RecordLocator`, which returns `{Checkout: metadataWorktreeRef(l)}`.
- `corpusFindings(cfg, recs, loc reposetup.RecordLocator)`.

- [ ] **Step 1: Write failing tests**

`repository_check_codes_test.go` (package app):

**`danglingDependencyRecord()`**
- path: `docs/changes/active/0005-waits.md`
- bytes: `changeRecordBytes("id: 5\nslug: waits\ntitle: Waits\nstatus: proposed\npriority: medium\ntype: feature\ncreated: 2026-08-30\nupdated: 2026-08-30\ndepends_on: [172]\n", "")`
- kind: change; location: active

**`TestCheckReportsValidatorCodeNotManualReview`** (the spec's §5/§6 code-switch guard):
- Run `corpusFindings(derivedTestConfig(), []corpusRecord{danglingDependencyRecord()}, RecordLocator{Checkout: ".docket"})`.
- Lift the result with `reposetup.EvaluateHealth(Classification{State: StateHealthy}, Facts{}, fm)`.
- The finding whose `Ref` is the record path has:
  - Code `repository.CodeChangeReferenceDangling`
  - Severity error
  - a non-nil `Repairable` of false
  - a non-empty Impact
  - a Remedy containing "`.docket/docs/changes/active/0005-waits.md`"
  - no "metadata corpus finding" in its Message
- Mutation probe: drop `FindingCode`'s ReviewCode arm. This test must go red.

**`TestPlanRepositoryRepairManualListNamesValidatorCode`:**
- Call `planRepositoryRepair(setupContext{cfg: derivedTestConfig()}, checkCorpus{records: […dangling], link: repairLink()})`.
- `plan.manual` must contain `"[change-reference-dangling]"`.
- Confirm a `setupContext` holding only `cfg` is enough when no derived repair exists.

`repairLink()` lives in `repository_repair_test.go`.

In `internal/reposetup/health_test.go`, add `TestRepairFindingFindingCode`. It covers three `RepairFinding` inputs and checks that `FindingCode()` and the code of `frontmatterFinding(input)` agree on each:

| Input | Expected code |
|---|---|
| repairable, `RepairQuoteScalar` | `quote-unsafe-scalar` |
| `ReviewCode` set | that `ReviewCode` |
| empty | `CodeFrontmatterManualReview` |

It also checks that `frontmatterFinding` keeps the producer's `Impact` and `Remedy`.

In `repair_remedy_family_test.go`, make four changes:
- Add `danglingDependencyRecord()`.
- Add `repository.CodeChangeReferenceDangling` to the floor.
- Change `manual < 2` to `manual < 3`.
- Fix the doc comment's probe sentence. "Apply the previewed mechanical repair" is gone. The probe is now: restore "Run `docket repository migrate`" in `frontmatterFinding`'s default.

- [ ] **Step 2: Run the tests**

Run: `go test -count=1 ./internal/reposetup/ -run TestRepairFindingFindingCode; go test -count=1 ./internal/app/ -run 'TestCheckReportsValidatorCode|TestPlanRepositoryRepairManualList|TestRepairableFamily'`

Expected: compile failure.

- [ ] **Step 3: Implement**

- **`FindingCode()`:**
  - `Repairable` → `string(rf.Code)`
  - otherwise, a non-empty `ReviewCode` → `ReviewCode`
  - otherwise → `CodeFrontmatterManualReview`

  Its doc comment names all three cases.
- **`frontmatterFinding(rf)`:**
  - Code is `rf.FindingCode()`.
  - Severity is warning when repairable, error otherwise.
  - It copies `Message` and `Impact`, and uses `rf.Remedy` when non-empty.
  - Default remedies apply only to a finding built without text:
    - repairable: "Run `docket repository repair` to make this edit (it shows the edit first)."
    - otherwise: "Edit the record by hand, then run `docket repository repair` so the board and link blocks catch up."
  - `Repairable` stays a non-nil pointer.

In `repository_check.go`:
- Add `locator reposetup.RecordLocator` to `checkCorpus` with a one-line comment.
- Add `checkRecordLocator`. Its comment: it names the checkout as the topology findings already do.
- In `readCheckCorpus`, set `corpus.locator = checkRecordLocator(sc.layout)` right after `var corpus checkCorpus`.
- `checkCorpusOutcome` calls `corpusFindings(cfg, corpus.records, corpus.locator)`.

The validator loop in `corpusFindings` becomes the code below. Keep the per-path suppression and the error-only rule unchanged, and update the doc comment.

```go
		text := explainValidation(f, loc)
		out = append(out, reposetup.RepairFinding{Path: validationRecordPath(f), Field: f.Field, Repairable: false,
			ReviewCode: f.Code, Message: text.Message, Impact: text.Impact, Remedy: text.Remedy})
```

`planRepositoryRepair` iterates `corpusFindings(cfg, corpus.records, corpus.locator)`, and its manual line uses `manualReviewLine(f.FindingCode(), f.Path, f.Message)`.

The test call sites pass `reposetup.RecordLocator{}`.

- [ ] **Step 4: Verify**

Run: `go test -count=1 ./internal/reposetup/ && go test -count=1 ./internal/app/ -run 'Check|Repair|Corpus'`

Expected: PASS.

Mutation probe: delete `FindingCode`'s `ReviewCode` arm. Both new app tests must go red. Restore the arm.

- [ ] **Step 5: Commit**

Message: `fix(check): report validator findings under their own code with plain text (change 0553)`.

---

### Task 7: Frontmatter-repair findings in plain language

**Tier:** standard

**Files:**
- Modify: `internal/reposetup/repair.go`
- Modify: `internal/app/repository_check.go` (`corpusFindings` passes `loc`)
- Modify: `internal/app/repository_migrate.go`
- Modify: `internal/reposetup/repair_test.go` (`planOne`)
- Test: new `internal/reposetup/repair_text_test.go`

**Produces:**
- `PlanRepairs(path string, src []byte, archived bool, loc RecordLocator) ([]RepairFinding, error)`.
- Every finding carries `Message`, `Impact`, and `Remedy`.
- `Path`, `Field`, `Code`, `Repairable`, and `Patch` are byte-identical to today's output, so `RepairDigest` and `ApplyRepairs` are untouched.

- [ ] **Step 1: Write failing tests**

`repair_text_test.go` uses `rel := "docs/changes/archive/2026-05-28-0001-wisdom-ai-bastion.md"` and `loc := RecordLocator{Checkout: ".docket"}`. The base frontmatter is `fm(extra) = "---\nid: 1\nslug: wisdom-ai-bastion\nstatus: done\ntitle: T\ntype: feature\n" + extra + "---\n\nBody.\n"`.

**`TestRepairFindingsCarryThreeParts`** is table-driven over seven shapes:

| Case | Source | archived | Repairable | Message contains |
|---|---|---|---|---|
| drop-final-claimed-at | `fm("claimed_at: 2026-05-20T10:00:00Z\n")` | true | yes | "`claimed_at`" |
| quote-unsafe-scalar | `title: yes` in place of `title: T` | false | yes | "`title: yes`" |
| scalar-to-list | `fm("depends_on: 3, 4\n")` | false | yes | "`depends_on: 3, 4`" |
| ambiguous value | `title: true` | false | no | "`title: true`" |
| non-id list | `fm("related: foo\n")` | false | no | "`related: foo`" |
| claimed_at, not done | `claimed_at` with `status: proposed` | true | no | "`proposed`" |
| undecodable | `"---\nid: 1\nid: 2\n---\nbody\n"` | false | no | "frontmatter" |

For each case, assert:
- exactly one finding (select by field if a fixture yields more; never loosen the count);
- the expected `Repairable`;
- non-empty `Message`, `Impact`, and `Remedy`;
- the message substring from the table;
- the remedy contains "`.docket/" + rel + "`" and "docket repository repair";
- no part contains a banned word (Global Constraints), compared case-insensitively.

**`TestDropFinalClaimedAtText`** pins the spec's worked example byte for byte:
- Message: "This finished change still has a `claimed_at` timestamp from when it was being built."
- Impact: "It's harmless: nothing reads it once a change is closed. Older docket versions left it behind."
- Remedy: "Run `docket repository repair` to remove it (it shows the edit first), or delete the `claimed_at:` line in `.docket/<rel>` by hand and commit and push it from `.docket`."

- [ ] **Step 2: Run the tests**

Run: `go test -count=1 ./internal/reposetup/ -run 'TestRepairFindingsCarryThreeParts|TestDropFinalClaimedAtText'`

Expected: compile failure.

- [ ] **Step 3: Implement**

Thread `loc` from `PlanRepairs` into `planQuote`, `planList`, and `planClaimedAt`, and update `PlanRepairs`'s doc comment.

New text for each shape. `tok` is the located value token. `path` is the record path. The remedy column calls methods on `loc`:

| Shape (site) | Message | Impact | Remedy |
|---|---|---|---|
| undecodable (`PlanRepairs`) | "docket can't read this record's frontmatter: " + `PlainDocumentError(err)` + "." | "docket skips the record, so the change is missing from the board and from every other check." | `HandEditThenRepair("fix the frontmatter between the two `---` lines", path)` |
| ambiguous (`planQuote`, non-repairable) | "`F: tok` is unquoted, so YAML reads it as something other than the text `tok`." | "docket can't tell whether you meant the text or the YAML value, so it won't change it for you." | `HandEditThenRepair("write `F: 'tok'` if you meant the text, or correct the value", path)` |
| quote-unsafe-scalar (`planQuote`) | "`F: tok` is unquoted, and YAML can misread values shaped like this." | "docket reads it correctly today; quoting is safe and does not change the value." | `RepairOrHandEdit("add the quotes", "quote the `F:` value", path)` |
| non-id list (`planList`, non-repairable) | "`F: tok` is written as plain text, and it isn't a list of change or ADR numbers." | "docket can't read it as a list, so it ignores this field." | `HandEditThenRepair("rewrite it as a list of numbers, such as `F: [12, 40]`, or delete the line", path)` |
| scalar-to-list (`planList`) | "`F: tok` is written as plain text, not a list." | "docket can't read it as a list, so it ignores these ids." | `RepairOrHandEdit("rewrite it as `NEW`", "rewrite it as `F: NEW`", path)`, where `NEW := tokenOf(candidate, f.Name)` (stop discarding `candidate`) |
| claimed_at, archived but not done or killed (`planClaimedAt`) | "This archived change still has a `claimed_at` timestamp, but its status is `<rawStatus(doc)>`, not done or killed." | "An archived change should be done or killed; docket won't remove the timestamp until the status is settled." | `HandEditThenRepair("set `status:` to `done` or `killed` (or move the file back into `active/` if the change is still open)", path)` |
| drop-final-claimed-at (`planClaimedAt`) | spec example (above) | spec example | `RepairOrHandEdit("remove it", "delete the `claimed_at:` line", path)` |

`F` is the field name.

Add `rawStatus(doc document.Document) string`. It decodes `status` exactly as `finalStatus` does and returns the value as written, or `"missing"` when the status is empty or the frontmatter cannot be decoded. It is for wording only, never for a decision.

Callers:
- `corpusFindings` passes `loc`.
- `repository_migrate.go` passes `reposetup.RecordLocator{}`, with a short comment: a migration has no attached metadata checkout yet.
- `planOne` passes `RecordLocator{}`.

Sweep for pins of the old text the same way: grep for each replaced literal (from `git diff`) and repoint the executable pins. Comments may stay.

- [ ] **Step 4: Verify**

Run: `go test -count=1 ./internal/reposetup/ && go test -count=1 ./internal/app/ -run 'Check|Repair|Corpus|Migrate'`

Expected: PASS. The `RepairDigest` and `ApplyRepairs` tamper tests must pass unchanged.

- [ ] **Step 5: Commit**

Message: `fix(repair): explain frontmatter findings in plain language with real record paths (change 0553)`.

---

### Task 8: Generated-view findings — plain text, no hand-edit offer when repairable

**Tier:** standard

**Files:**
- Modify: `internal/reposetup/derived.go`
- Modify: `internal/reposetup/derived_test.go`
- Modify: `internal/app/repository_check.go` (the derived-view producers and `corpusUnreadableFinding`)
- Test: new `internal/app/repository_check_derived_text_test.go`

**Produces:**
- `DerivedFinding` gains `Impact` and `Remedy` (strings).
- `Finding()` copies both. An empty `Remedy` defaults to:
  - repairable: "Run `docket repository repair` to regenerate it (it shows the change first)."
  - otherwise: "Fix it by hand, then run `docket repository repair`."
- Test helpers: `derivedScenarios(t) []checkCorpus` and `derivedScenarioFindings(t) map[string]reposetup.Finding` (first finding per code, lifted).

- [ ] **Step 1: Write failing tests**

`derivedScenarios` builds one `checkCorpus` per producible code, each with `locator: RecordLocator{Checkout: ".docket"}` and `link: render.LinkContext{MetadataBranch: layout.SharedName}`. It reuses existing fixtures: `canonicalChangeRecord`, `changeRecordBytes`, and `derivedChangeFM` from `repository_check_derived_test.go`; `backlinkRecord`, `backlinkSpecPath`, and `absoluteBacklinkArtifact` from `repository_check_backlinks_test.go`.

| # | Corpus | Produces |
|---|---|---|
| 1 | canonical record + `board: {present, "# Backlog\n\nstale bytes\n"}` | board-stale |
| 2 | canonical record + `adrIndex: {present, "# ADRs\n\nstale bytes\n"}` | adr-index-stale |
| 3 | `changeRecordBytes(derivedChangeFM, "")` | artifact-links-stale |
| 4 | `derivedChangeFM` record with no block | artifact-links-missing |
| 5 | record `0004-malformed` with a start marker only (copy the fixture in `TestRepairableFamilyRemediesNameRepairNotMigrate`) | artifact-links-malformed |
| 6 | `backlinkRecord("07","t",false)` + `absoluteBacklinkArtifact` | artifact-backlink-stale |
| 7 | the same record + a start-marker-only backlink source (copy `TestArtifactBacklinkStartMarkerOnlyIsMalformed`) | artifact-backlink-malformed |
| 8 | `backlinkRecord` 07 and 08 + `absoluteBacklinkArtifact` | artifact-backlink-shared |

**`TestRepairableGeneratedViewRemedyOffersNoHandEdit`** is the spec §6 guard. For every finding with `*Repairable == true`, assert that the remedy starts with "Run `docket repository repair`" and that, lowercased, it contains neither " or " nor "hand". Set a population floor of 5 or more repairable findings. Also assert that `DerivedFinding{Repairable: true}.Finding().Remedy` does not contain "hand".

Mutation probe: append ", or edit the file by hand." to one producer's remedy. The test must go red.

**`TestGeneratedViewFindingsCarryThreeParts`** asserts:
- all eight codes are present, each with non-empty `Message`, `Impact`, and `Remedy`;
- no message contains "canonical";
- the `artifact-links-malformed` remedy contains "`.docket/docs/changes/active/0004-malformed.md`";
- the `artifact-backlink-shared` message contains "0007" and "0008".

**`TestCorpusUnreadableText`** pins the exact three strings from the table below for `corpusUnreadableFinding(errors.New("object missing"))`. With `err.Error()` = "object missing", the message is "docket couldn't read the records on the metadata branch: object missing."

- [ ] **Step 2: Run the tests**

Run: `go test -count=1 ./internal/app/ -run 'TestRepairableGeneratedView|TestGeneratedViewFindings|TestCorpusUnreadableText'`

Expected: FAIL.

- [ ] **Step 3: Implement**

In `derived.go`:
- Add the two fields.
- Rewrite `Finding()` so it copies `Impact` and uses `df.Remedy` or the defaults above. Severity is unchanged: warning when repairable, error otherwise.
- Update the doc comments. A repairable remedy names only `docket repository repair`, because generated files are never hand-edited.
- In `derived_test.go`, fix `TestDerivedFindingLiftRepairable`'s stale "remedy naming migrate" comment and add `!strings.Contains(f.Remedy, "hand")`.

In `repository_check.go`, rewrite each producer's text. `View`, `Code`, `Path`, and `Repairable` stay as they are. `loc` is `corpus.locator`. `p` is the artifact path, and `rec.path` the record path:

| Code | Message | Impact | Remedy |
|---|---|---|---|
| board-stale | "BOARD.md doesn't match the current change records." | "The board shows out-of-date statuses. This usually means a record was edited with plain git instead of a docket command." | "Run `docket repository repair` to regenerate it (it shows the change first)." |
| adr-index-stale | "The ADR index doesn't match the current ADR records." | "It lists out-of-date titles or statuses. This usually means an ADR was edited with plain git instead of a docket command." | same as board-stale |
| artifact-links-missing | "This change record links a spec, plan, results file, or ADR but has no `## Artifacts` block." | "Readers of the record don't see its links." | "Run `docket repository repair` to add the block (it shows the change first)." |
| artifact-links-stale | "The `## Artifacts` block in this change record is out of date." | "Its links don't match the record's spec, plan, results, or ADRs. This usually means the record was edited with plain git instead of a docket command." | "Run `docket repository repair` to rewrite the block (it shows the change first)." |
| artifact-links-malformed | "The `## Artifacts` block markers in this change record don't pair up: " + `PlainDocumentError(err)` + "." | "docket won't rewrite a block whose start and end it can't find, so the block stays out of date." | `loc.HandEditThenRepair("restore the `<!-- docket:artifacts:start … -->` and `<!-- docket:artifacts:end -->` lines around the block, or delete both lines and everything between them", rec.path)` (deleting is safe: repair then adds the block, as artifact-links-missing) |
| artifact-backlink-stale | "The link at the top of `p` back to its change points to the wrong place." | "Following it from the file leads to a missing page. This usually happens after the change was archived." | "Run `docket repository repair`." |
| artifact-backlink-malformed | "The back-link markers at the top of `p` don't pair up: " + `PlainDocumentError(err)` + "." | "docket won't rewrite a link whose start and end it can't find, so the link stays as it is." | `loc.HandEditThenRepair("restore the `<!-- docket:backlink:start … -->` and `<!-- docket:backlink:end -->` lines around the link", p)` |
| artifact-backlink-shared | "`p` is linked from more than one change (IDS)." | "docket can't tell which change its back-link should point to, so it leaves the link alone." | `loc.HandEditThenRepair("remove the `spec:`, `plan:`, or `results:` line naming `p` from every change except the one that owns it (changes IDS)", "")` |
| corpus-unreadable (`corpusUnreadableFinding`) | "docket couldn't read the records on the metadata branch: " + `err.Error()` + "." | "None of the record checks ran, so this result says nothing about the records." | "Fix the read error, then run `docket repository check` again." |

IDS is the sorted, comma-joined `%04d` ids of `owners[p]`, computed at the producer.

`board-malformed` and `adr-index-malformed` have no producer (`grep -rn 'CodeBoardMalformed\|CodeADRIndexMalformed' internal` finds only the declarations). Give them no text and invent no producer.

Sweep for pins of the old text: grep for each message literal this task replaced (`git diff` lists them), and repoint the executable pins at the code, `Repairable`, or the new text. Comments may stay.

- [ ] **Step 4: Verify**

Run: `go test -count=1 ./internal/reposetup/ && go test -count=1 ./internal/app/ -run 'Derived|Backlink|Check|Repair|RepairableFamily'`

Expected: PASS. Then run the mutation probe from Step 1.

- [ ] **Step 5: Commit**

Message: `fix(check): explain generated-view findings and stop offering hand edits of generated files (change 0553)`.

---

### Task 9: The finding catalog — every message in one reviewable file

**Tier:** standard

**Files:**
- Create: `internal/app/finding_catalog_test.go`
- Create (generated): `internal/app/testdata/finding-catalog.txt`

- [ ] **Step 1: Write the test**

Declare the flag `var updateCatalog = flag.Bool("update", false, "rewrite testdata/finding-catalog.txt from the current finding text")`. It follows the `internal/harness/*` golden pattern and is never set in CI.

`TestFindingCatalogGolden` renders `renderFindingCatalog(t)`. With `-update`, it writes `testdata/finding-catalog.txt` (path relative to the package, creating the directory). Without it, it compares byte for byte and, on a mismatch, prints the fresh text and the command that regenerates it.

`renderFindingCatalog` writes these sections in order, sorting every map's keys:

1. A `#` header: generated by `TestFindingCatalogGolden -update`; do not hand-edit.
2. `## Check header`: one `checkHeader(state, [error, warning])` line per sorted `stateConstants` value, plus `checkHeader("healthy", nil)`.
3. `## Validator findings`: for each sorted code in `validationCodeSet` and each of its fixtures:
   - a heading `### <code> — <label> (<severity>)`;
   - the message, impact, and `remedy:` lines under `{Checkout: ".docket"}`;
   - fail when a code has no fixture.
4. `## Remedy locations by layout`: the depends_on dangling remedy under four locators: `.docket`, `/home/u/.local/share/dckt/o/r/checkouts/r-1a2b3c4d`, the label "your private metadata checkout", and the zero locator.
5. `## Frontmatter-repair findings`: for each of the seven Task 7 shapes:
   - run `PlanRepairs(rel, src, archived, {Checkout: ".docket"})`;
   - lift the result through `EvaluateHealth(Classification{State: StateHealthy}, Facts{}, rfs)`;
   - render each finding with `Repairable != nil` through `appendFindingBlock`.
6. `## Generated-view findings`: each `derivedScenarioFindings` value, sorted by code, then `corpusUnreadableFinding(errors.New("object 4b825dc6 is missing"))`.

Before comparing, check the population floor:
- the rendered text contains `"] "+code` for each of:
  - the 3 `RepairCode`s;
  - `CodeFrontmatterManualReview`;
  - the 8 producible derived codes;
  - `CodeCorpusUnreadable`;
- it contains at least 4 `"] frontmatter-manual-review "` blocks.

The validator section needs no floor here, because `validationCodeSet` already computes its population from source. If a repoguard objects to the file write, follow its accepted shape.

- [ ] **Step 2: Generate the file and read all of it**

Run: `go test -count=1 ./internal/app/ -run TestFindingCatalogGolden -update && go test -count=1 ./internal/app/ -run TestFindingCatalogGolden`
Expected: PASS on the second run.

Read the file top to bottom, as Daniel will. For each finding, check that:
- the message names the record and its value;
- harmless findings say they are harmless;
- the remedy works when typed as written;
- no banned word appears;
- there are no doubled spaces, no doubled periods, and no empty backticks;
- each frontmatter section has exactly one block.

Fix any wording at its producer, then regenerate.

Mutation probe: delete the "non-id list" row, check that the four-shape floor goes red, then restore it.

- [ ] **Step 3: Commit**

Message: `test(findings): generate one catalog of every check and status finding message (change 0553)`

---

### Task 10: Remaining sites, the ADR decision, and the whole suite

**Tier:** economy

- [ ] **Step 1: Check the skill and sweep for old text**
  - **The skill.** Read the `### Findings` paragraph of `skills/docket-status/SKILL.md`. If it still quotes none of `frontmatter-manual-review`, "metadata corpus finding", or the old header, leave it alone and record "checked, no edit". If you do edit it, run `go generate ./internal/assets/`.
  - **The sweep.** Run these three searches:
    - `grep -rn 'metadata corpus finding' internal cmd tests skills`
    - `grep -rn 'repository check: no-op' internal cmd tests skills`
    - `grep -rn '"frontmatter-manual-review"' internal`

    The first two must find no executable hit. The third must find only `CodeFrontmatterManualReview` and test files.

- [ ] **Step 2: Settle the ADR question the spec left open**
  - **Default:** no new ADR. The single app-layer table is internal organization that a one-file change can reverse. It sets no protocol or policy, ADR-0136 already governs the repair wording, and `adrs: [136]` stands.
  - **Record:** write the decision and its reason down for the results file.
  - **Escalate:** if the build finds a concrete reason against the default, dispatch `docket-adr`.

- [ ] **Step 3: Run the whole suite**

Run: `go run ./cmd/docket development test`. Expect green, and read the budget report.

- **Red tests that compare whole finding values.** These are old-text pins. Update or repoint them; never delete them.
- **A `TestRetiredVocabularySeal` hit.** The seal scans Go literals. Reword the string.

- [ ] **Step 4: Notes for the results file (the parent writes it)**
  - **Added Detail key:** `path`, on the 3 ADR-graph codes.
  - **Check code switch:** `frontmatter-manual-review` is kept for the 4 frontmatter shapes.
  - **Private status:** it names the checkout in words, while check prints the path.
  - **Codes with no producer:** `board-malformed` and `adr-index-malformed` have none, so they get no text.
  - **Steps 1 and 2:** their outcomes.

- [ ] **Step 5: Commit any suite-driven fixes**

Stage files explicitly and use the message `test: repoint finding-text pins at the plain-language wording (change 0553)`. Skip this step if Step 3 needed no fixes.
