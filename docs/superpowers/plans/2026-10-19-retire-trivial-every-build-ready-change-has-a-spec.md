<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0573 — Retire trivial: every build-ready change has a spec](../../changes/active/0573-retire-trivial-every-build-ready-change-has-a-spec.md)**
<!-- docket:backlink:end -->

# Retire `trivial`: every build-ready change has a spec — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` (one task per `docket-build-task`
> worker, one commit per task, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax.

**Goal:** Remove the `trivial` flag and groom outcome everywhere, so a change is build-ready only when
it has a spec, and every groom that makes a change build-ready writes one.

**Architecture:** First move every test that used `trivial: true` as a shortcut onto a real spec link
(green before and after the removal). Then retire the `change.groom` `trivial` outcome, stop new
records writing `trivial:`, and drop `Trivial` from the domain model and decoder in one compile unit,
together with the call sites that read it (groom gate, `docket open`, board). Finally rewrite skill,
agent, and doc prose, regenerate embedded assets and harness goldens, and lower the skill budgets.

**Tech Stack:** Go (`internal/domain`, `internal/repository`, `internal/app`, `internal/render`,
`internal/cli`, `internal/repoguard`), Markdown skills/agents/docs, `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-19-retire-trivial-every-build-ready-change-has-a-spec-design.md`
(metadata branch; the feature branch carries the copy at the same path).

## Global Constraints

- One rule: a change is build-ready only when it is `proposed`, has a non-empty `spec:`, its
  dependencies are `done`, its stack base resolves, and it is not held (held ⇒ `on-hold`).
- Legacy `trivial:` lines are inert data: no new check, finding, warning, or gate about them; every
  writer leaves them byte-identical; a `trivial: true` record with no spec reads needs-grooming.
- `outcome: trivial` refuses through the existing `invalid-outcome` finding and writes nothing — no
  special "retired" message.
- `workspace.commit-spec` keeps its `no-spec` no-op disposition (decision 11); only its comment changes.
- Docs describe only current behavior: delete `trivial` mentions outright, no "formerly" notes.
  "Required for every change, trivial included" becomes "required for every implemented change."
- Do not touch frozen or point-in-time material: `docs/release/**`, `internal/install/legacydata/**`,
  `internal/install/testdata/legacy/**`, `testdata/bash-upgrade/**`, `testdata/repositories/**`
  (including the status corpus, whose `trivial: true` lines now prove the key is inert),
  `internal/repository/testdata/corpus/**`, `internal/render/testdata/board/corpus/**`,
  `docs/results/**`, `docs/adrs/**`, `docs/superpowers/**`, `docs/changes/**`. ADR-0129 stays as is.
- Ordinary-English "trivial"/"trivially"/"non-trivial" stays (e.g. `cursor-rules/dispatch.head.md`
  "trivial dispatch", `skills/docket-convention/SKILL.md` and `skills/docket-finalize-change/SKILL.md`
  "one trivial attempt/dispatch", test comments like "trivially-passing"). The two flag uses spelled
  "non-trivial" DO change: implement-next's "(non-trivial changes)" and new-change's "The default path
  for any non-trivial new change".
- The ADR superseding ADR-0006 (decision 8) is recorded by the coordinator through `docket-adr`, not
  by any task here. Where prose must cite it, write "the new ADR superseding ADR-0006" with no number.
- Canonical test record builders (`lifecycleChange`, `groomableChange`, board fixtures) KEEP their
  `trivial: false` line: it is inert, and many `strings.Replace(…, "trivial: false\n", …)` anchors
  insert fields after it. Any `strings.Replace` anchored on `"trivial: true\n"` must be re-anchored —
  `strings.Replace` silently no-ops on a missing anchor and turns the test vacuous.
- Cross-references in maintained source anchor on symbols or quoted clauses, never line numbers.
- After any edit under `skills/`, `agents/`, or `cursor-rules/`, run `go generate ./internal/assets/`.
- Learnings in play: `restatement-accumulates-its-own-guards` (tests pin the "trivial included"
  copies — relocate the pins, never re-add the text), `test-premise-deleted-not-regated` (delete a
  test whose subject was trivial; narrow one whose guarded property survives),
  `assert-detects-removal-not-replacement` (pair every new present-phrase pin with an absent one),
  `intermediate-task-state-buildable` (each task below compiles and passes on its own),
  `cached-runner-serves-a-mutated-tree` (always `-count=1`).

## Review Focus

1. A legacy active record with `trivial: true` and no spec — it must read `needs-grooming` in
   `status` and on the board, be selectable by grooming, and be refused by claim as not build-ready
   (Task 5, `TestStatusLegacyTrivialRecordReadsNeedsGrooming`).
2. Grooming that legacy record (`spec` outcome, then a sections-only `revise`) must apply and leave
   the `trivial: true` line byte-identical (Task 5, `TestChangeGroomPlanLegacyTrivialRecordGroomsAsStub`).
3. A request with `outcome: trivial` must refuse `invalid-input`/`invalid-outcome`, call no engine,
   and list exactly `spec, revise, abstain, re-enable` (Task 3).
4. A change claimed before the upgrade with `trivial: true` and no spec, still in progress — its
   `workspace.commit-spec` must still report `no-spec` as a no-op (Task 5, renamed integration test).
5. `docket open <id> spec` on a legacy trivial record must answer `artifact-unset` ("change N has no
   spec yet"); a malformed `trivial: maybe` must produce no decode finding (Task 5).

---

### Task 1: App test fixtures link a spec instead of using `trivial: true`

**Tier:** standard

**Files:**
- Create: `internal/app/spec_fixture_test.go` (no build tag)
- Modify: `internal/app/change_claim_test.go` (`claimableChange`, and the hold insertion anchored on
  `"trivial: true\n"`), `internal/app/claim_workflow_git_test.go` (`buildReadyChange`),
  `internal/app/workflow_e2e_test.go` (`specLinkedBuildReadyChange`, `workflowMetadataSpec`),
  `internal/app/planning_git_test.go` (`killableChangeWithSpec`),
  `internal/app/named_branch_facts_test.go` (`stackFixtureBlob`),
  `internal/app/finalize_cleanup_integration_test.go` (three `trivial: true` records),
  `internal/app/runtracker_verdict_integration_test.go` (`TestIntegrationRunVerdictCompletedResultsArtifactReadsDone`),
  `internal/app/workflow_integration_test.go` (the stacked child comment "trivial: true stands in for a spec"),
  `internal/app/leak_check_private_integration_test.go` (`newPrivateLeakWorkspaceFor`,
  `prepareLeakWorkspace`, `TestIntegrationPrivateModeLeakCheckScansBranchName`),
  `internal/app/change_implemented_test.go` (`miRecord`'s `trivial` parameter and the `trivial` knob),
  `internal/app/change_integration_test.go` (the "trivial change with no results artifact still refuses" case),
  plus every caller the steps below surface.

**Interfaces:**
- Produces (package `app`, test-only, untagged file):
  - `func fixtureSpecPath(slug string) string` → `"docs/superpowers/specs/2026-08-01-" + slug + "-design.md"`
  - `func withLinkedSpec(src, specPath string) string` — replaces the first `"\nspec:\n"` with
    `"\nspec: '" + specPath + "'\n"`; panics (message names the missing anchor) when `src` has no
    `"\nspec:\n"`, so a fixture can never silently stay spec-less.
  - `func fixtureSpecFile(id int, slug string) string` — the spec bytes as the metadata branch stores
    them (backlink block + `# Widget: design` body). Move `workflowMetadataSpec`'s body here and make
    `workflowMetadataSpec` and `specWithBacklink`/`killableChangeWithSpec`/`specLinkedBuildReadyChange`
    delegate to or be replaced by these helpers where the shapes match (keep any helper whose output
    differs in a way a test depends on).
- `claimableChange(id, slug)` and `buildReadyChange(id, slug)` keep their names and signatures and now
  return `withLinkedSpec(<proposed record>, fixtureSpecPath(slug))`.

- [ ] **Step 1: Write the helper test** in `spec_fixture_test.go`:
  `TestWithLinkedSpecLinksAndRefusesMissingAnchor` — `withLinkedSpec(lifecycleChange(3, "widget", "proposed"), fixtureSpecPath("widget"))`
  contains `"\nspec: 'docs/superpowers/specs/2026-08-01-widget-design.md'\n"` and no `"\nspec:\n"`;
  calling it on `"---\nid: 3\n---\n"` panics (recover and assert the panic happened).
- [ ] **Step 2: Run it** — `go test -count=1 -run TestWithLinkedSpec ./internal/app/` → FAIL (undefined).
- [ ] **Step 3: Implement the three helpers**, then switch `claimableChange` and `buildReadyChange`
  to `withLinkedSpec`. Re-anchor `change_claim_test.go`'s hold insertion (currently replacing
  `"trivial: true\n"`) on a line that is still present (e.g. `"trivial: false\n"`), and grep
  `internal/app` for any other `"trivial: true\n"` anchor and re-anchor it the same way.
- [ ] **Step 4: Convert the remaining shortcut fixtures.**
  - `stackFixtureBlob`: replace `trivial: true\n` with `spec: 'docs/superpowers/specs/x-design.md'\n`
    (snapshot-only fixture; no file read).
  - `finalize_cleanup_integration_test.go` (three sites) and the run-verdict results-artifact test:
    link the spec with `withLinkedSpec` AND add `fixtureSpecPath(slug): fixtureSpecFile(id, slug)` to
    the repo file map, since `context.implementation` refuses a linked spec that does not exist.
    These are `deliverable: results-artifact` changes, whose ending skips `workspace.commit-spec`.
    Change `prepareLeakWorkspace`'s `spec bool` to a `commitSpec bool` meaning "run
    `WorkspaceCommitSpec` and expect `committed`"; `false` now skips the call entirely (instead of
    asserting `no-spec`). Callers that passed `true` keep `true`.
  - `newPrivateLeakWorkspaceFor`: drop the `spec bool` parameter and the `GroomTrivial` branch; it
    always grooms with a spec. `TestIntegrationPrivateModeLeakCheckScansBranchName` keeps its point
    (only the branch name leaks): create the change with a clean title (e.g. "Fix the alias table",
    type `fix`) and `BranchPrefix: "dckt"`, so title, slug, spec path, and commit subjects are clean
    while the branch is `dckt/fix-the-alias-table`. Update the expected `featureRef` and keep
    `requireLeakRefusal`'s exactly-one-hit `LeakHit{Source: "branch-name", Text: "dckt", Rule: "alias"}`.
    If the normalizer or the alias rule does not yield exactly that one hit, report BLOCKED with the
    observed hits rather than loosening `requireLeakRefusal`.
  - `miRecord`: drop the `trivial bool` parameter and the `trivial` case knob; delete the
    `change_integration_test.go` case "trivial change with no results artifact still refuses" and its
    "(there is no trivial exemption to remove)" comment — the condition-5 rows for spec'd changes
    already pin the requirement.
  - Fix the comments that described the old shortcut (`claimableChange`, `buildReadyChange`, the
    stacked-child comment in `workflow_integration_test.go`, `specLinkedBuildReadyChange` "(not a
    trivial verdict)").
- [ ] **Step 5: Run every affected test.** Unit: `go test -count=1 ./internal/app/`. Integration, one
  prefix at a time for each file touched or each caller of `claimableChange`/`buildReadyChange`
  (`git grep -l -e 'claimableChange(' -e 'buildReadyChange(' -- internal/app`), e.g.
  `go test -tags integration -count=1 -run '^TestIntegrationClaim' ./internal/app/`,
  `-run '^TestIntegrationPrivateModeLeakCheck'`, `-run '^TestIntegrationRunVerdict'`,
  `-run '^TestIntegrationFinalize'`, `-run '^TestIntegrationWorkflow'`. For every caller that now fails
  with a missing spec (`missing-artifact`, `spec-missing`, an artifact-link status finding, or a
  closeout archive of a missing spec), add `fixtureSpecPath(slug): fixtureSpecFile(id, slug)` to that
  test's file map. Expected: all PASS; `git grep -n '"trivial: true' -- 'internal/app/*_test.go'`
  prints nothing except legacy-record fixtures introduced later (none yet).
- [ ] **Step 6: Commit** — `test(app): build-ready fixtures link a spec instead of trivial: true`.

### Task 2: Domain test fixtures link a spec instead of `Trivial: true`

**Tier:** economy

**Files:**
- Modify: `internal/domain/selection_test.go` (`selSpec.build`, `TestSelectQueueExcludesAWaitingDependency`,
  `TestSelectQueueExcludesAHeldChange`), `internal/domain/readiness_test.go` (the cases that use
  `trivial: true` only to get past the design check: the two `ReadyInvalid` id/slug cases,
  `TestEvaluateReadinessRejectsAnEmptySlug`, the stacked case built with `parent: parentEdge(4)`),
  `internal/domain/types_test.go` (`TestLifecycleActionCarriesHold`).

- [ ] **Step 1:** In `selSpec.build`, replace the `trivial` default with a spec default: when
  `!sp.noDesign && sp.spec.State != FieldPresent`, set `Spec: specRef("docs/specs/a.md")` (reuse
  `specRef` from `readiness_test.go`); drop `Trivial:` from the `ChangeSpec`. Update its doc comment
  ("defaulting to a build-ready shape (proposed, with a spec: reference)").
- [ ] **Step 2:** Replace every other `Trivial: true` / `trivial: true` that only supplies design with
  `Spec: specRef("docs/specs/a.md")` / `spec: specRef("docs/specs/a.md")`. Leave the cases whose
  subject IS trivial ("trivial with no spec is build-ready", "a held trivial change is on-hold",
  "a trivial change needs no design", the `readySpec.trivial` field, `entities_test.go`) for Task 5.
- [ ] **Step 3: Run** — `go test -count=1 ./internal/domain/` → PASS.
- [ ] **Step 4: Commit** — `test(domain): build-ready fixtures link a spec instead of Trivial`.

### Task 3: Retire the `change.groom` `trivial` outcome

**Tier:** standard

**Files:**
- Modify: `internal/app/change_groom.go`, `internal/app/finding_codes.go`, `internal/app/schema_vocab.go`,
  `internal/cli/change.go`
- Test: `internal/app/change_groom_test.go`, `internal/app/finding_codes_test.go`,
  `internal/app/schema_vocab_test.go`

**Interfaces:**
- Produces: `GroomOutcome` members `GroomSpec`, `GroomRevise`, `GroomAbstain`, `GroomReEnable` only;
  `GroomTrivial` and `FCMissingRationale` no longer exist. The groom gate's `c.Trivial()` check stays
  until Task 5.

- [ ] **Step 1: Write the failing tests.**
  - Add a row to `TestChangeGroomRejectsBadShapeWithoutEngineCall`:
    `{"retired trivial outcome", func(r *ChangeGroomRequest) { r.Outcome = "trivial"; r.SpecMarkdown = "" }, "invalid-outcome"}`
    (asserts invalid-input, zero engine calls).
  - `TestChangeGroomRetiredTrivialOutcomeRefused`, modelled on `TestChangeGroomRetiredRearmOutcomeRefused`:
    `validateChangeGroomShape` with `Outcome: GroomOutcome("trivial")` yields `invalid-outcome` whose
    message ends with `"must be one of spec, revise, abstain, re-enable"`; update the rearm test's
    suffix to the same string.
  - `schema_vocab_test.go`: `assertVocabMembers(t, v, "groom_outcomes", []string{"spec", "revise", "abstain", "re-enable"})`.
- [ ] **Step 2: Run** — `go test -count=1 -run 'TestChangeGroom|TestSchemaVocab' ./internal/app/` → the new rows FAIL.
- [ ] **Step 3: Remove the outcome.** In `change_groom.go`: delete `GroomTrivial`, the `case GroomTrivial`
  in `validateChangeGroomShape` and in `fieldEdits`, `hasAuthoredRationale`, and the
  `groomed (trivial)` fallback in `HumanText` (a non-revise/abstain/re-enable applied result now
  always carries a `SpecPath`; render `change %04d groomed — %s` if it is ever empty). Change the
  `invalid-outcome` message to `outcome %q must be one of spec, revise, abstain, re-enable`. Rewrite
  every doc comment that names the trivial outcome or the `trivial:` field write (file header,
  `GroomRevise`, `GroomAbstain`, the `Title`/`Priority`/`Deliverable`/`Hold` field docs "spec, trivial,
  revise, and re-enable" → "spec, revise, and re-enable", the receipt comment, the sections-only
  revise receipt comment). `finding_codes.go`: delete `FCMissingRationale` from the const block, the
  registry list, and the comment listing it. `schema_vocab.go`: drop `GroomTrivial` from
  `groom_outcomes`. `internal/cli/change.go`: help becomes
  `"Groom a proposed change to build-ready with a spec, record or clear an auto-groom abstain (abstain, re-enable), or revise any proposed change without changing its groom state, from a JSON request"`.
- [ ] **Step 4: Delete or narrow the tests whose subject was the outcome** (ask what each guards):
  delete `TestChangeGroomTrivialRequiresRationale`, `TestChangeGroomPlanTrivialOutcomeFileSet`,
  `titledTrivialRequest` and `TestChangeGroomPlanTitleOnTrivialOutcome`; drop the `trivial` row/entry
  from every outcome table (`"trivial accepts title"`, `"trivial accepts priority"`, and the
  `trivial := ChangeGroomRequest{…}` tables in the priority, deliverable/type/branch-prefix, hold, and
  section-heading tests). In `finding_codes_test.go` drop the three `GroomTrivial` emitters and
  `FCMissingRationale` from the expected list. Leave tests built on `trivialChange` (legacy records)
  for Task 5.
- [ ] **Step 5: Run** — `go build ./... && go test -count=1 ./internal/app/ ./internal/cli/` → PASS;
  `git grep -n -e GroomTrivial -e FCMissingRationale -e missing-rationale -- '*.go'` → no output.
- [ ] **Step 6: Commit** — `refactor(groom): retire the trivial outcome`.

### Task 4: New records carry no `trivial:` line

**Tier:** economy

**Files:**
- Modify: `internal/render/record.go` (`ChangeRecord`), `internal/render/testdata/records/change.golden.md`
- Test: `internal/render/record_test.go`

- [ ] **Step 1: Write the failing test** `TestChangeRecordHasNoTrivialKey` in `record_test.go`: render
  a `ChangeRecord` with the fixture the golden test uses and assert the output has no line starting
  with `trivial:` (check every frontmatter line, not a substring of the whole file).
- [ ] **Step 2: Run** — `go test -count=1 -run TestChangeRecord ./internal/render/` → FAIL.
- [ ] **Step 3:** Delete the `{Name: "trivial", …}` field from `ChangeRecord`; delete the
  `trivial: false` line from `change.golden.md` (no other byte changes).
- [ ] **Step 4: Run** — `go test -count=1 ./internal/render/ ./internal/app/` → PASS (any app test
  comparing created-record bytes against a literal containing `trivial: false` is updated to match).
- [ ] **Step 5: Commit** — `refactor(record): new change records carry no trivial line`.

### Task 5: `trivial` leaves the domain model

**Tier:** premium (readiness is the build-selection licence; a mistake builds the wrong changes)

**Files:**
- Modify: `internal/domain/entities.go` (`ChangeSpec.Trivial`, `Change.Trivial`),
  `internal/domain/actions.go` (`newChangeBuilder`), `internal/domain/readiness.go` (`hasDesign`,
  `NeedsDesign`, `EvaluateReadiness` doc), `internal/repository/decode.go` (`changeWire.Trivial`,
  the `spec.Trivial = …` decode), `internal/app/change_groom.go` (groom gate in `changeGroomOp.Plan`),
  `internal/app/open.go` (`ReasonOpenTrivialNoSpec`, `changeArtifactPath`),
  `internal/render/board.go` (`boardClassify`, `boardReadinessCell`, the section doc comment),
  `internal/render/testdata/board/PROVENANCE.md`, comments in `internal/app/change_implemented.go`,
  `internal/app/implementation_context.go`, `internal/app/workspace_spec.go` (`SpecCopyNoSpec`).
- Test: `internal/domain/entities_test.go`, `readiness_test.go`, `internal/repository/decode_test.go`,
  `internal/render/board_test.go`, `internal/app/change_groom_test.go`, `internal/app/open_test.go`,
  `internal/app/status_test.go` (or the file holding `changeBlob` readiness tests),
  `internal/app/workflow_integration_test.go`.

**Interfaces:**
- Consumes: Tasks 1–3 (no remaining `Trivial:`/`GroomTrivial` shortcuts outside this task's files).
- Produces: `domain.ChangeSpec` and `domain.Change` with no `Trivial` field/accessor;
  `hasDesign(c) == (c.Spec().State == FieldPresent && c.Spec().Value != "")`.

- [ ] **Step 1: Write the failing legacy tests.**
  - `decode_test.go` `TestDecodeLegacyTrivialKeyIsInert`: decode a proposed record with `trivial: true`
    and an empty `spec:` → zero findings and `domain.NeedsDesign(change)` is true; decode one with
    `trivial: maybe` → zero findings. Narrow `TestDecodeChangeBooleanMalformed` to `reconciled: maybe`
    (its guarded property, booleans reporting `field-malformed`, still holds for `reconciled`); drop
    the `Trivial()` assertion in the canonical decode test.
  - `status` test `TestStatusLegacyTrivialRecordReadsNeedsGrooming`: a corpus with
    `changeBlob(15, "legacy", "feat", "high", "trivial: true\n")` (no spec) reports that change's
    readiness as `needs-grooming` and it is not in the build-ready queue.
  - `change_groom_test.go`: rename `trivialChange` to `legacyTrivialChange` (doc: "a record carrying a
    legacy trivial: true line and no spec — a stub"). `TestChangeGroomPlanLegacyTrivialRecordGroomsAsStub`:
    (a) `validGroomSpecRequest()` against it applies, writes `spec:`, and the record still contains
    the exact line `"\ntrivial: true\n"` exactly once; (b) a sections-only revise applies and leaves
    that line byte-identical. Replace `TestChangeGroomPlanReviseTrivialRationale` and
    `TestChangeGroomPlanTitleOnlyReviseOfTrivialChange` with (b) (delete the duplicates). In
    `TestChangeGroomPlanReviseRefusals` keep the `spec-not-linked` row on `legacyTrivialChange`.
  - `open_test.go`: change 9 stays `trivial: true` with no spec (comment "9 legacy trivial"); its
    spec row now expects `ReasonOpenArtifactUnset`, `"change 9 has no spec yet"`.
- [ ] **Step 2: Run** — `go test -count=1 ./internal/repository/ ./internal/app/` → the new tests FAIL.
- [ ] **Step 3: Remove `Trivial` from the domain and decoder.** Delete the field, the accessor, and the
  builder copy; `hasDesign` keeps only the spec test; rewrite the `EvaluateReadiness`, `NeedsDesign`,
  `hasDesign` comments ("proposed, carrying no spec"). In `decode.go` delete the wire field and the
  decode line (the `changeWire` comment already says unknown keys stay in the document).
- [ ] **Step 4: Fix the dependent call sites.**
  - Groom gate: `c.Status() != domain.StatusProposed || c.Spec().Value != ""`, message
    `change %04d is not a proposed, needs-design change (status %q, spec %q)`; rewrite the gate
    comment ("revise never writes spec: or auto_groomable:").
  - `open.go`: delete `ReasonOpenTrivialNoSpec` and the `c.Trivial()` branch; the doc comment says an
    unset field (a stub's spec included) is refused as not there yet.
  - `board.go`: `boardClassify` drops the now-redundant `c.Spec().Value != ""` guard and its trivial
    comment (readiness build-ready/on-hold implies a spec); `boardReadinessCell` deletes the
    `ReadyBuildReady` and `ReadyOnHold` arms so they fall to the existing per-record fault; the section
    doc comment drops the `build-ready (trivial)` row, the `on-hold` Proposed row, and the
    "trivial-without-spec" paragraph.
  - Comments: `change_implemented.go` (three "trivial" clauses → "for every implemented change"),
    `implementation_context.go` (`ContextEntity`: "e.g. a change with no spec"),
    `workspace_spec.go` (`SpecCopyNoSpec`: "the change links no spec (a legacy record claimed with
    none), so there is no copy (no-op)").
- [ ] **Step 5: Update the remaining tests whose subject was the flag.**
  - Domain: delete the `Trivial` check in `entities_test.go`; delete `readySpec.trivial` and the cases
    "trivial with no spec is build-ready", "a held trivial change is on-hold", "a trivial change needs
    no design"; reword "not trivial" case names ("proposed, no spec is needs-grooming").
  - Board: delete `TestBoardProposedTrivialBuildReadyCell`, `TestBoardHeldTrivialStaysProposed`, the
    "proposed trivial no spec" classification row and its fixture, and the `trivial` leg of the
    stacked-on-killed-parent table (keep the `spec` leg, drop the loop variable); fix the comments
    naming trivial. Update `PROVENANCE.md`'s coverage list (needs-grooming "(no spec)", drop the held
    trivial and `build-ready (trivial)` bullets, drop "vs trivial-without-spec → Proposed"); the corpus
    files are untouched.
  - Groom: `TestChangeGroomPlanRefusesNonGroomable` drops "already trivial"; the abstain and re-enable
    refusal tables drop their `trivial` rows; fix assertions that read "trivial field changed under
    revise" so they still assert `trivial: false` is unchanged byte-for-byte (inert-line proof) with
    wording that does not call it a groom field.
  - `workflow_integration_test.go`: rename `TestIntegrationWorkflowLifecycleCommitSpecTrivialNoSpec`
    to `TestIntegrationWorkflowLifecycleCommitSpecNoSpec`; comment: a legacy change claimed with no
    spec still gets the `no-spec` no-op. Fix `commitSpecRecord`'s `"" … a trivial change` comment.
- [ ] **Step 6: Run** — `go build ./... && go vet ./internal/... && go test -count=1 ./internal/domain/ ./internal/repository/ ./internal/render/ ./internal/app/ ./internal/cli/`
  and `go test -tags integration -count=1 -run '^TestIntegrationWorkflowLifecycleCommitSpec' ./internal/app/` → PASS.
  `git grep -n -w -i trivial -- '*.go'` lists only legacy-record fixtures, the inert `trivial: false`
  fixture lines, generic document-layer tests (`internal/document/*_test.go`), and ordinary English.
- [ ] **Step 7: Mutation check** — copy `decode.go` aside, then temporarily re-add a `trivial` wire
  field and, when it decodes true with an empty spec, set `spec.Spec` to a present non-empty value
  (trivial-as-design reintroduced). Confirm `TestDecodeLegacyTrivialKeyIsInert` and
  `TestStatusLegacyTrivialRecordReadsNeedsGrooming` go RED (run with `-count=1`), then restore from
  the copy (never `git checkout --` over uncommitted work) and confirm GREEN.
- [ ] **Step 8: Commit** — `refactor(domain): build-ready requires a spec; trivial is inert data`.

### Task 6: Build-side skill prose — convention, implement-next, review, status

**Tier:** standard

**Files:**
- Modify: `skills/docket-convention/SKILL.md`, `skills/docket-implement-next/SKILL.md`,
  `skills/docket-implement-next/references/results-artifact.md`,
  `skills/docket-implement-next/results-template.md`, `skills/docket-review/SKILL.md`,
  `skills/docket-status/SKILL.md`, regenerated `internal/assets/embedded/tree/**`
- Test: `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`

- [ ] **Step 1: Update the pins first (red).** In `prose_contracts_test.go`:
  `change_0410_implement_next_results` present phrase → `"required for every implemented change"`;
  `change_0410_convention_results` phrases → `"required close-out artifacts (one per implemented change)"`
  and `"required close-out artifact for every implemented change."`; add `absent: []string{"trivial included"}`
  to both. Add sentinel `change_0573_convention_spec_required` on `skills/docket-convention/SKILL.md`:
  present `"has a \`spec:\`, and all \`depends_on\` are satisfied"`, absent `"trivial: true"`,
  `"trivial:"`. Run `go test -count=1 -run TestProseContracts ./internal/repoguard/` → FAIL.
- [ ] **Step 2: Edit the convention.** Directory map: "(one per implemented change)"; results
  paragraph: "for every implemented change."; manifest example: delete the `trivial:` line;
  build-readiness: "only when it is `proposed`, has a `spec:`, and all `depends_on` are satisfied
  (`done`). A `proposed` change with no spec is **needs-grooming** (not build-ready)."; hold sentence:
  "the auto-groomer's `hold: true` on a spec that needs a person's step"; auto-groomable:
  "(`proposed`, no `spec:`)". Leave the "one trivial dispatch" sentence alone.
- [ ] **Step 3: Edit implement-next and its references.** Delete "A trivial change has no spec —
  patch the body only."; drop "(a trivial change reports `no-spec`)"; results line → "**required for
  every implemented change**"; mark-implemented "(missing, invalid, or unattached results refuse the
  transition)"; phase table drops "(non-trivial changes)"; `reconciled` paragraph → "build-readiness
  is `spec:` plus satisfied `depends_on`". `results-artifact.md`: "The spec is the plan." and "the spec
  body, the results artifact's text, …". `results-template.md` header comment: "REQUIRED close-out
  artifact for every implemented change." `docket-review`: "When the payload carries a spec body and
  a results artifact's text". `docket-status`: "a `proposed` change with no spec and not waiting
  shows **needs-grooming**".
- [ ] **Step 4: Regenerate and re-baseline.** `go generate ./internal/assets/`. For each edited skill
  file, measure (`wc -l -w <file>`) and lower its `skillBudgets` row to the exact new counts, appending
  `0573: trivial retired (<old> -> <new>)` to its note; also fix the 0410 history comment's "trivial
  included" only if it describes current behavior (it is history — leave it).
- [ ] **Step 5: Run** — `go test -count=1 ./internal/repoguard/ ./internal/assets/` → PASS.
- [ ] **Step 6: Commit** — `docs(skills): build-readiness is a spec; results required for every implemented change`.

### Task 7: Grooming prose — groom-next, new-change, auto-groom, agents, cursor rule

**Tier:** standard

**Files:**
- Modify: `skills/docket-groom-next/SKILL.md`, `skills/docket-new-change/SKILL.md`,
  `skills/docket-auto-groom/SKILL.md`, `agents/docket-auto-groom.md`,
  `agents/docket-auto-groom-critic.md`, `cursor-rules/dispatch/docket-auto-groom-critic.md`,
  regenerated `internal/assets/embedded/tree/**`, regenerated
  `internal/harness/{claude,codex,cursor,opencode}/testdata/golden/docket-auto-groom*`
- Test: `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`

- [ ] **Step 1: Pins first (red).** Add sentinel `change_0573_spec_always` for each of
  `skills/docket-groom-next/SKILL.md` and `skills/docket-new-change/SKILL.md`: present
  `"always a written spec, sized to the change"`; absent `"outcome: trivial"`, `"trivial: true"`,
  `"Trivial path"`, `"Trivial verdict"`. For `skills/docket-auto-groom/SKILL.md`: present
  `"Exit (one of two)"`; absent `"outcome: trivial"`, `"trivial verdict"`, `"trivial M"`. Run
  `go test -count=1 -run TestProseContracts ./internal/repoguard/` → FAIL.
- [ ] **Step 2: Brainstorm direction (decision 6).** In groom-next Step 3 (right after invoking
  `superpowers:brainstorming` WITH THE HUMAN, seeded with …) and new-change Brainstorm step 2 (right
  after "invoke `superpowers:brainstorming` WITH THE HUMAN."), add one sentence, phrased by shape and
  citing no superpowers heading, e.g.: "Tell it the artifact is always a written spec, sized to the
  change: a small change gets a short spec, and it never concludes that a change needs no written
  design." Keep it to that sentence; these skills load on every groom.
- [ ] **Step 3: groom-next.** Description: "(proposed, no spec)" and "exiting with a linked spec, a
  kill, a defer, or a re-enable". When-to-use and Step 1: "(has `spec:`)", "no `spec:`" (drop the
  `trivial: true` clauses), recap "the linked spec's body and the owned sections". Step 4: "Exit (one
  of five …)", "All five exits", delete exit 2, renumber Kill 2, Defer 3, Revise 4, Re-enable 5, and
  fix every exit reference ("Step 4, exit 4" in When to use; Retitle "exits 1 (spec), 4 (revise), and
  5 (re-enable)"; "The same three exits" in Reprioritize, Deliverable, Hold, Stacking). Revise: "It
  never sets `spec:` or `auto_groomable:`". Re-enable: "A spec groom of an abstained stub needs no
  re-enable." Hold: "suggest a hold before the spec exit". Step 5: "a stub edit finds it groomed (`spec:`)".
- [ ] **Step 4: new-change.** Delete the trivial When-to-use bullet and the whole `## Trivial path`
  section; "The default path for a new change"; step 4 hold sentence: "or in the step-5 `change.groom`
  request"; Scan mode: "(no spec)".
- [ ] **Step 5: auto-groom, agents, cursor rule.** Skill + `agents/docket-auto-groom.md` descriptions
  (keep them identical): "exiting each stub with a linked spec or an abstain back to the human queue."
  Step 2: replace the "genuinely mechanical → trivial" sentence with "A mechanical stub gets a short
  spec with its `## Assumptions` block like any other." Step 3: "to adversarially attack the draft
  spec". Step 4: "Exit (one of two)", delete the Trivial exit, Abstain becomes 2. Hold: "When the
  emitted spec names a step …". Step 6: "groomed N, abstained K". Termination: "(spec ⇒ no longer
  needs-grooming; abstain ⇒ no longer auto-groomable)". Critic description (agent file) and the cursor
  rule's two mentions: "an auto-groom draft spec" / "the draft spec under review".
- [ ] **Step 6: Regenerate.** `go generate ./internal/assets/`, then
  `go test -count=1 ./internal/harness/claude/ ./internal/harness/codex/ ./internal/harness/cursor/ ./internal/harness/opencode/ -update`;
  confirm `git status` shows golden changes only in the `docket-auto-groom*` goldens. Lower the three
  skills' `skillBudgets` rows to their exact new counts with a `0573:` note (if groom-next or
  new-change grew, re-baseline upward at the exact count with the reason "spec-always brainstorm
  direction").
- [ ] **Step 7: Run** — `go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/harness/... ./internal/install/` → PASS.
- [ ] **Step 8: Commit** — `docs(skills): grooming always writes a spec; auto-groom exits spec or abstain`.

### Task 8: User docs, then the whole-repo audit

**Tier:** economy

**Files:**
- Modify: `README.md`, `docs/guide/capturing-work.md`, `docs/guide/designing-before-building.md`,
  `docs/guide/building-without-supervision.md`, `docs/concepts/change-lifecycle.md`,
  `docs/reference/glossary.md`, `docs/install/models-and-effort.md`,
  `docs/comparison/ai-native-sdlc-playbook.md`

- [ ] **Step 1: Edit, describing only current behavior.**
  - README design row: "The spec, linked from the change".
  - capturing-work: results "(one per implemented change)"; delete the `trivial` field bullet;
    needs-grooming "(no spec)" / "a proposed change with no spec"; build-ready "has a spec"; replace
    the "mark it `trivial`" paragraph with one sentence: small work still gets a spec, just a short one.
  - designing-before-building: needs-grooming/build-ready definitions without the trivial mark; "The
    output of grooming is a **spec**" (drop the trivial alternative); auto-groom sentence drops "(or
    that comes out genuinely trivial)".
  - change-lifecycle: diagram label "(has a spec, deps done)"; the two definitions drop the trivial mark.
  - building-without-supervision: build-ready "has a spec and whose dependencies are all `done`".
  - models-and-effort: "a quick stub or a small revise needs little".
  - glossary: drop `trivial` from the owned-fields list; results "for every implemented change"; Stub
    "`proposed`, no spec"; delete `### Trivial` and its index entry; Readiness definitions; rename
    "### Capture modes: designed / rough stub / scan" (drop the Trivial sentence, fix its index
    anchor); Groom exits and outcomes (`spec`, `revise`, `abstain`, `re-enable`); Revise "a spec'd
    change stays build-ready" and "never sets `spec:` or `auto_groomable:`"; auto-groom "drafts a spec
    with an `## Assumptions` block, then has the critic attack it. Each stub exits as spec or abstain.";
    groom-next exits "spec, kill, defer, revise, or re-enable"; new-change drops the trivial-path
    sentence; `groom_outcomes` row `spec` `revise` `abstain` `re-enable`.
  - playbook: "Build-ready = spec."; delete the "Trivial path" row; drop "; or `trivial: true`"; drop
    "trivial," from the capture list.
- [ ] **Step 2: Audit.** Run
  `git grep -n -i -w trivial -- ':!docs/release' ':!internal/install/legacydata' ':!internal/install/testdata' ':!testdata' ':!docs/results' ':!docs/adrs' ':!docs/superpowers' ':!docs/changes' ':!*/testdata/*'`
  and confirm every remaining hit is ordinary English, a legacy-record test fixture, an inert
  `trivial: false` fixture line, or a generic document-layer test. Any flag-meaning hit is fixed here.
- [ ] **Step 3: Run** — `go test -count=1 ./internal/repoguard/` → PASS.
- [ ] **Step 4: Commit** — `docs: every build-ready change has a spec`.

The build gate then runs the whole suite through `build.test_command`
(`go run ./cmd/docket development test`); read its budget report even when green.
