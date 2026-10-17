<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0559 — Let a change deliver a results artifact instead of a PR](../../changes/archive/2026-10-17-0559-close-a-change-to-done-without-a-pr.md)**
<!-- docket:backlink:end -->

# Let a change deliver a results artifact instead of a PR — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, one plan task per build-tier worker under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every change a `deliverable` (`pr` or `results-artifact`), add `docket change complete` to close a results-artifact change to `done` without a PR, and teach groom, mark-implemented, the Artifacts links, the board, `finalize cleanup`, the sweep, `run verify`, the skills, and the docs about the new ending — plus rename every bare `already` result to `already-<what happened>`.

**Architecture:** `deliverable` is a new closed domain vocabulary decoded like `priority` (absent ⇒ `pr`). `change.complete` is a single-target archive transaction modelled on `change.kill` (domain legality in a new `domain.Complete`, scoped validation via `changeScope`, results verified on the transaction's own tree through a helper shared with mark-implemented). `finalize.cleanup` gains a no-PR branch for done results-artifact changes; `run verify` gains a results-artifact postcondition branch; implement-next gains a separate ending in a new reference file.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), cobra CLI, Markdown skills embedded via `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-17-close-a-change-to-done-without-a-pr-design.md` (metadata branch). Read it with this plan; section numbers below (§N) refer to it.

## Global Constraints

- Frontmatter key is `deliverable`; values exactly `pr` and `results-artifact`. Never rename a frontmatter key (ADR-0129).
- An absent or empty `deliverable:` reads as `pr`; no existing record is rewritten.
- `change.create` writes `deliverable:` explicitly on every new record, directly after `type:`.
- New vocabularies registered in `docket schema`: `deliverables` and `complete_dispositions` (`completed`, `already-completed`, `contended`, `failed`).
- Catalog id `change.complete`, CLI `docket change complete --request <file> [--repo-dir <dir>]`, effect `metadata-write`. Commit subject `change NNNN completed (done, archived)`.
- The capability catalog must stay ≤ 16384 bytes (`TestCapabilitiesPayloadWithinByteBudget`); it is ~15.8 KB today. Do not raise the cap.
- Every finding code is minted from a constant in `internal/app/finding_codes.go` and listed in `AllFindingCodes` (`TestNoInlineFindingCodeLiterals`).
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers.
- Point-in-time records (archived changes, specs, plans, results, Accepted ADRs, `docs/changes/**`, `docs/superpowers/**`, `docs/results/**`) keep their old wording — never edit them.
- Every skill edit re-baselines its row in `internal/repoguard/budgets_test.go` (`skillBudgets`) to the exact new line/word counts with a leading `0559: <what>` note; a new skill file gets a new row. Run `go generate ./internal/assets/` after any `skills/**` edit and commit the regenerated assets with it.
- Unit tests: `go test -count=1 ./internal/<pkg>/ -run '<Regex>'`. Integration tests carry `//go:build integration`, must be named under an existing shard prefix (`TestIntegrationFinalizeCleanup…`, `TestIntegrationSweep…`, `TestIntegrationRunVerdict…`, `TestIntegrationChangeRuntime…`), and run with `go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/` — never an unfiltered `./internal/app/` integration run.
- Shell in test scripts follows AGENTS.md (no `grep -q` on a pipe under pipefail, templated `mktemp`).

## Review Focus

1. A groom that turns a change into `results-artifact` without touching `stacked_on`, while the record already carries `stacked_on`, or while an active change stacks on it — a person expects a refusal, not a silently unstackable branch. Tests in Task 3.
2. `finalize cleanup` on a completed change whose branch is checked out somewhere other than the owned worktree (for example the primary checkout) — expect `pending` / `branch-checked-out`, branch kept. Test in Task 7.
3. `finalize cleanup` on a completed change when fetching `origin/<integration_branch>` fails — expect `pending` / `ancestry-probe-failed`, branch kept (an errored probe is never clean absence). Test in Task 7.
4. A record carrying an unknown `deliverable:` value (hand typo) — expect a warning finding naming it, and both endings (`change complete`, `change mark-implemented`) to refuse it rather than guess. Tests in Tasks 1, 4, 5.
5. A completed change whose results file is later deleted or broken — `run verify` must report `run-incomplete` with the results reason, never `run-complete`. Test in Task 9.

## Decisions this plan fixes (not stated in the spec)

- An unknown `deliverable:` is a **warning** finding `change-deliverable-unknown`, exactly like `change-priority-unknown`. Safety comes from both endings refusing anything but their own value: `change complete` requires `results-artifact`, `mark-implemented` requires `pr`.
- `change complete` refusals return an empty `disposition` with the refusal reason as the first finding's code (the four-member vocabulary stays closed). Pre-transaction work is only the replay check; every precondition, including the results verification, is decided inside the transaction's `Plan` on the tree it commits to.
- The create digest carries `deliverable` with `omitempty`, normalised so `""` and `"pr"` both digest as absent: an explicit `pr` and an omitted field replay each other, and pre-0559 retries still replay.
- Groom checks stacking on the **resulting** state, and only when the request sets `deliverable` or `stacked_on`, so an unrelated revise of an old record is never blocked.
- Refusal codes: `unknown-deliverable`, `invalid-deliverable` (abstain), `invalid-type` (abstain), `results-artifact-not-stackable`; domain reasons `not-results-artifact`, `pr-recorded`, `not-pr-deliverable`; existing `results-missing`, `results-identity-broken`, `results-content-invalid`, `illegal-source-status`, `not-ready-<kind>`, `invalid-branch_prefix`, `unknown-type`.

---

### Task 1: `deliverable` in the domain, decode, validation, schema vocabulary, and record writer

**Files:**
- Modify: `internal/domain/types.go`, `internal/domain/entities.go`, `internal/domain/actions.go` (`newChangeBuilder`)
- Modify: `internal/repository/decode.go` (`changeWire`, `decodeChange`), `internal/repository/validate.go` (`changeIdentity`, new code const)
- Modify: `internal/app/validation_text.go` (`validationTexts`), `internal/app/testdata/finding-catalog.txt` (regenerate), `internal/app/schema_vocab.go`, `internal/app/schema_vocab_test.go` (spec row)
- Modify: `internal/render/record.go` (`NewChangeRecord`, `ChangeRecord`)
- Test: `internal/domain/types_test.go`, `internal/repository/decode_test.go` (or the existing decode test file), `internal/repository/validate_test.go`, `internal/render/record_test.go`, `internal/app/finding_catalog_test.go` (case added)

**Interfaces:**
- Produces:
  - `type Deliverable string`; `DeliverablePR Deliverable = "pr"`, `DeliverableResultsArtifact Deliverable = "results-artifact"`; `var AllDeliverables = []Deliverable{DeliverablePR, DeliverableResultsArtifact}`; `func ParseDeliverable(s string) (Deliverable, bool)` — exact membership, `""` is **not** a member (like `ParsePriority`).
  - `ChangeSpec.Deliverable Deliverable`, `ChangeSpec.RawDeliverable string` (placed after `Type`); `func (c Change) Deliverable() Deliverable` — effective value: `DeliverablePR` for an absent/empty key, `""` for an unknown stored value; `func (c Change) RawDeliverable() string` — stored text.
  - `repository.CodeChangeDeliverableUnknown = "change-deliverable-unknown"` (warning, field `deliverable`, detail `raw`, `status`).
  - `render.NewChangeRecord.Deliverable string` — `""` renders as `pr`.
  - Schema vocabulary `"deliverables"`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/domain/types_test.go
func TestParseDeliverable(t *testing.T) {
	for _, s := range []string{"pr", "results-artifact"} {
		if d, ok := ParseDeliverable(s); !ok || string(d) != s { t.Errorf("ParseDeliverable(%q) = %q,%v", s, d, ok) }
	}
	for _, s := range []string{"", "PR", "results_artifact", " pr"} {
		if _, ok := ParseDeliverable(s); ok { t.Errorf("ParseDeliverable(%q) accepted", s) }
	}
}

// decode test (repository): three records through decodeChange / BuildSnapshot
// absent key       -> Deliverable() == DeliverablePR, RawDeliverable() == ""
// deliverable: ''  -> Deliverable() == DeliverablePR
// results-artifact -> Deliverable() == DeliverableResultsArtifact
// deliverable: research -> Deliverable() == "", RawDeliverable() == "research"
func TestDecodeChangeDeliverable(t *testing.T) { /* table over the four */ }

// validate test: the unknown record yields exactly one finding
// Code CodeChangeDeliverableUnknown, Severity warning, Field "deliverable",
// Detail["raw"] == "research"; the absent/empty/known records yield none.
func TestChangeIdentityDeliverableUnknown(t *testing.T) { … }

// render/record_test.go: ChangeRecord with Deliverable "" and "results-artifact"
// emits "deliverable: pr" / "deliverable: results-artifact" on the line directly
// after the "type:" line (assert line order by index).
func TestChangeRecordWritesDeliverableAfterType(t *testing.T) { … }
```

Also add `{vocab: "deliverables", byType: "Deliverable"}` to the `specs` list in `TestVocabularyConstCompleteness`, and a builder-carried `change-deliverable-unknown` case (unknown, and proposed) to the finding-catalog test inputs in `finding_catalog_test.go`, mirroring the `change-priority-unknown` cases.

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/domain/ ./internal/repository/ ./internal/render/ -run 'Deliverable' ; go test -count=1 ./internal/app/ -run 'TestVocabularyConstCompleteness|TestFindingCatalog'`
Expected: FAIL — undefined `ParseDeliverable` / `Deliverable`, vocabulary `deliverables` not emitted.

- [ ] **Step 3: Implement**

- `types.go`: the type, constants, `AllDeliverables`, `ParseDeliverable` beside the priority block, doc comments in the same style.
- `entities.go`: the two spec fields and two accessors; `actions.go` `newChangeBuilder` copies both (a builder that drops them would turn every lifecycle write into an unknown-deliverable record in memory).
- `decode.go`: `Deliverable scalar \`yaml:"deliverable"\`` in `changeWire`; in `decodeChange` after the type block: `spec.RawDeliverable = d.text("deliverable", wire.Deliverable)`; empty ⇒ `DeliverablePR`, else `ParseDeliverable` (unknown ⇒ `""`).
- `validate.go`: in `changeIdentity`, `if c.Deliverable() == ""` append the warning, mirroring the priority branch; doc comment says it is a warning because both endings refuse an unknown value.
- `validation_text.go`: entry for the new code — Message "%s has `deliverable: <raw>`, which isn't a deliverable."; Impact "docket can't tell how this change ends, so both `docket change complete` and `docket change mark-implemented` refuse it until it is fixed."; Remedy like `priorityRemedy` (groom revise first for a proposed change, hand edit `deliverable:` to `pr` or `results-artifact` otherwise). Regenerate the catalog with `go test ./internal/app/ -run TestFindingCatalog -update` and review the diff.
- `schema_vocab.go`: `v["deliverables"]` built from `domain.AllDeliverables` like `priorities`.
- `record.go`: `Deliverable string` on `NewChangeRecord` (comment: `""` ⇒ `pr`); a `{Name: "deliverable", …}` field directly after `type`.

- [ ] **Step 4: Run to verify they pass**

Run the Step 2 commands, plus `go test -count=1 ./internal/render/ ./internal/repository/ ./internal/domain/`.
Expected: PASS. Fix any golden that now carries the `deliverable:` line (render record goldens) by regenerating through its own `-update` flag where one exists.

- [ ] **Step 5: Commit**

```bash
git add internal/domain internal/repository internal/render internal/app/validation_text.go internal/app/testdata/finding-catalog.txt internal/app/schema_vocab.go internal/app/schema_vocab_test.go internal/app/finding_catalog_test.go
git commit -m "feat(domain): add the deliverable field with pr as the default"
```

---

### Task 2: `change.create` takes `deliverable`; status and implementation context report it

**Files:**
- Modify: `internal/app/change_create.go`, `internal/app/finding_codes.go`, `internal/app/status_result.go` (`StatusChange`), `internal/app/status.go` (row fill), `internal/app/implementation_context.go` (`ContextEntitySummary` and its fill)
- Test: `internal/app/change_create_test.go`, the status JSON test file that asserts `StatusChange` fields, the implementation-context test file

**Interfaces:**
- Consumes: Task 1's `domain.Deliverable`, `ParseDeliverable`, `Change.Deliverable()`, `render.NewChangeRecord.Deliverable`.
- Produces:
  - `ChangeCreateRequest.Deliverable string \`json:"deliverable,omitempty" docket:"enum=deliverables"\``.
  - `FCUnknownDeliverable FindingCode = "unknown-deliverable"`, `FCResultsArtifactNotStackable FindingCode = "results-artifact-not-stackable"` (both in `AllFindingCodes`).
  - `func deliverableText(c domain.Change) string` — `string(c.Deliverable())` when non-empty, else `c.RawDeliverable()`; used by status and context.
  - `StatusChange.Deliverable string \`json:"deliverable"\``; `ContextEntitySummary.Deliverable string \`json:"deliverable,omitempty"\``.

- [ ] **Step 1: Write the failing tests** (in `change_create_test.go`, following its plan-closure helpers)

```go
func TestChangeCreateWritesDeliverable(t *testing.T) {
	// omitted -> record frontmatter has deliverable: pr
	// "results-artifact" -> deliverable: results-artifact
}
func TestChangeCreateRejectsUnknownDeliverable(t *testing.T) {
	// req.Deliverable = "research" -> ResultInvalidInput, finding "unknown-deliverable",
	// message "unknown deliverable \"research\"; valid values are pr, results-artifact", engine not called
}
func TestChangeCreateRefusesStackedResultsArtifact(t *testing.T) {
	// Deliverable results-artifact + StackedOn set -> invalid-input "results-artifact-not-stackable", no engine call
}
func TestChangeCreateRefusesStackingOnResultsArtifactParent(t *testing.T) {
	// snapshot holds change 2 with deliverable: results-artifact; request StackedOn=2
	// -> Plan refuses with code "results-artifact-not-stackable" (field stacked_on)
}
func TestChangeCreateDigestTreatsPRAsOmitted(t *testing.T) {
	// canonicalDigest of payloads for Deliverable "" and "pr" are equal; "results-artifact" differs;
	// and the "" payload's JSON carries no "deliverable" key
}
```

Status/context: one assertion each that a change created with `results-artifact` reports `"deliverable":"results-artifact"`, an old record without the key reports `"pr"`, and an unknown one reports its raw text.

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestChangeCreate.*Deliverable|TestChangeCreate.*Stack|Deliverable'`
Expected: FAIL — unknown field `Deliverable`.

- [ ] **Step 3: Implement**

- Shape stage (`validateChangeCreateShape`): unknown deliverable ⇒ `FCUnknownDeliverable`; `results-artifact` with non-nil `StackedOn` ⇒ `FCResultsArtifactNotStackable` "a results-artifact change takes no part in stacking; order it with depends_on".
- `changeCreatePayload.Deliverable string \`json:"deliverable,omitempty"\``, set to `""` when the request says `""` or `"pr"`.
- `Plan`: after `validateChangeReferences`, when `StackedOn` resolves to a change whose `Deliverable()` is `results-artifact`, refuse with a `domain.Finding{Code: string(FCResultsArtifactNotStackable), Field: "stacked_on", …}` (same shape as the dangling-reference findings).
- Pass `Deliverable: o.req.Deliverable` to `render.ChangeRecord`.
- Status row and context summary use `deliverableText`.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestChangeCreate|TestStatus|TestImplementationContext|TestNoInlineFindingCodeLiterals|TestVocabularyConstCompleteness'`
Expected: PASS. If a `docket schema` golden or a published-schema test reddens on the new request/result fields, regenerate it through its own update flag and review the diff.

- [ ] **Step 5: Commit**

```bash
git add internal/app
git commit -m "feat(change): create takes a deliverable and status reports it"
```

---

### Task 3: `change.groom` edits `deliverable`, `type`, and `branch_prefix`

**Files:**
- Modify: `internal/app/change_groom.go`, `internal/app/change_create.go` (extract two shared message helpers), `internal/app/finding_codes.go`
- Test: `internal/app/change_groom_test.go`

**Interfaces:**
- Consumes: Task 1 domain API; Task 2 `FCUnknownDeliverable`, `FCResultsArtifactNotStackable`.
- Produces:
  - `ChangeGroomRequest` fields: `Deliverable string \`json:"deliverable,omitempty" docket:"enum=deliverables"\``, `Type string \`json:"type,omitempty" docket:"enum=change_types"\``, `BranchPrefix *string \`json:"branch_prefix,omitempty"\`` (nil = unchanged, `""` = clear, value = set after `domain.NormalizeBranchPrefix`).
  - `FCInvalidDeliverable FindingCode = "invalid-deliverable"`, `FCInvalidType FindingCode = "invalid-type"` (abstain refusals).
  - `func branchPrefixProblem(raw string) string` and `func unknownTypeMessage(eff config.Effective, t string) string` in `change_create.go`, used by both create and groom so the messages stay identical.

- [ ] **Step 1: Write the failing tests**

```go
func TestChangeGroomEditsDeliverableTypeAndPrefixOnEveryAcceptingOutcome(t *testing.T) {
	// for outcome in spec, trivial, revise, re-enable: each field lands in the
	// record frontmatter (deliverable: results-artifact, type: chore, branch_prefix: spike)
}
func TestChangeGroomEachNewFieldAloneIsAValidRevise(t *testing.T) {
	// revise with only Deliverable / only Type / only BranchPrefix=ptr("x") / only BranchPrefix=ptr("")
	// -> no empty-revise finding; empty-revise message names deliverable, type, and branch_prefix
}
func TestChangeGroomAbstainRefusesNewFields(t *testing.T) {
	// codes: invalid-deliverable, invalid-type, invalid-branch_prefix
}
func TestChangeGroomBranchPrefixThreeStates(t *testing.T) {
	// nil -> existing branch_prefix byte-identical; ptr("") -> branch_prefix: null;
	// ptr(" Spike/ ") -> branch_prefix: spike; ptr("a/b") -> invalid-branch_prefix (same message as create)
}
func TestChangeGroomRefusesUnconfiguredType(t *testing.T) {
	// Type "nope" not in change_types -> invalid-input "unknown-type", no engine call
}
func TestChangeGroomRefusesUnknownDeliverable(t *testing.T) { /* "unknown-deliverable" */ }
func TestChangeGroomStackingRefusals(t *testing.T) {
	// (a) Deliverable results-artifact + StackedOn in the same request -> shape refusal
	// (b) record already stacked_on: 2, request sets Deliverable results-artifact only -> Plan refusal
	// (c) StackedOn names a results-artifact parent -> Plan refusal
	// (d) request sets Deliverable results-artifact on change 2 while active change 5 has stacked_on: 2 -> Plan refusal
	// all with code results-artifact-not-stackable; a revise touching neither field on (b)'s record still applies
}
func TestChangeGroomTypeEditChangesNextClaimBranch(t *testing.T) {
	// groom type: chore (or branch_prefix: spike) on a proposed change, decode the
	// planned record bytes, run domain.Claim on it -> Branch().Value == "chore/<slug>" ("spike/<slug>")
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestChangeGroom'`
Expected: FAIL — unknown fields.

- [ ] **Step 3: Implement**

- `validateChangeGroomShape`: extend the revise minimum-input condition and the `FCEmptyRevise` message ("…a title, a priority, a deliverable, a type, or a branch_prefix"); abstain refusals for the three fields; non-abstain checks for an unknown deliverable and a malformed non-empty `branch_prefix` (`branchPrefixProblem`); same-request `results-artifact` + `StackedOn` refusal.
- New config stage in `ChangeGroom` right after `PinContext`: `validateChangeGroomConfig(eff, req)` refuses a non-empty, non-abstain `Type` not in `eff.ChangeTypes.Value` with `FCUnknownType` / `unknownTypeMessage`, before any engine call.
- `Plan` first patch pass: `upsertField` for `deliverable` and `type`; for `branch_prefix`, `upsertField` with the normalised value or `document.Null()` when it normalises to `""`.
- `Plan`, after `gc` is resolved and only when the request set `Deliverable` or `StackedOn`: the three resulting-state checks (b)–(d) via `refuseGroom(string(FCResultsArtifactNotStackable), …)`; (d) scans `candidate.Changes()` for a non-final change whose `StackedOn()` is present and equals this id.
- Update the `ChangeGroomRequest` doc comments to name the three fields beside `Title`/`Priority`.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestChangeGroom|TestChangeCreate|TestNoInlineFindingCodeLiterals'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app
git commit -m "feat(groom): edit deliverable, type, and branch_prefix on a proposed change"
```

---

### Task 4: Share the results verification; `mark-implemented` accepts only `pr` changes

**Files:**
- Modify: `internal/app/change_implemented.go`, `internal/domain/actions.go` (`MarkImplemented`), `internal/app/finding_codes.go`
- Test: `internal/domain/actions_test.go`, `internal/app/change_implemented_test.go` (or the existing mark-implemented unit test file)

**Interfaces:**
- Produces:
  - `func resultsArtifactProblem(resultsPath string, data []byte, found bool, ch domain.Change) (reason, message string)` — pure; returns `ReasonImplementedResultsIdentity` when not found or when `backlinkTargets` errors / is false, `ReasonImplementedResultsInvalid` with the first `ValidateResultsContent(data, ResultsPhaseFinal)` finding, `"", ""` when valid. `verifyImplementedResults` keeps its signature and becomes `ReadArtifact` + this helper.
  - Domain reason `"not-pr-deliverable"` (`FailInvalidState`) from `MarkImplemented` when `c.Deliverable() != DeliverablePR`; app constant `ReasonImplementedNotPRDeliverable = "not-pr-deliverable"`, registered in `AllFindingCodes`.

- [ ] **Step 1: Write the failing tests**

```go
// domain
func TestMarkImplementedRefusesNonPRDeliverable(t *testing.T) {
	// in-progress, reconciled, results-artifact -> FailInvalidState, Reason "not-pr-deliverable";
	// unknown deliverable ("") -> same refusal; pr -> succeeds
}
// app
func TestChangeMarkImplementedRefusesResultsArtifactChange(t *testing.T) {
	// valid evidence; record deliverable: results-artifact ->
	// ResultInvalidState, finding "not-pr-deliverable", engine not called, no GitHub call
}
func TestResultsArtifactProblem(t *testing.T) {
	// not found -> results-identity-broken; foreign backlink -> results-identity-broken;
	// final-contract failure -> results-content-invalid; valid -> "", ""
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/domain/ -run TestMarkImplemented ; go test -count=1 ./internal/app/ -run 'TestChangeMarkImplemented|TestResultsArtifactProblem'`
Expected: FAIL.

- [ ] **Step 3: Implement** — domain check first in `MarkImplemented` after `requireStatus`; app pre-check right after the already-implemented replay branch in `ChangeMarkImplemented` (before workspace/GitHub probes); extract `resultsArtifactProblem` with no behaviour change to the existing results refusals.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -count=1 ./internal/domain/ ; go test -count=1 ./internal/app/ -run 'TestChangeMarkImplemented|TestResultsArtifactProblem|TestNoInlineFindingCodeLiterals'`
Expected: PASS, existing mark-implemented tests unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/domain internal/app
git commit -m "feat(change): mark-implemented closes only pr changes; share the results check"
```

---

### Task 5: `domain.Complete` and the `change.complete` operation

**Files:**
- Modify: `internal/domain/actions.go` (new `Complete`)
- Create: `internal/app/change_complete.go`
- Modify: `internal/app/finding_codes.go`, `internal/app/schema_vocab.go`, `internal/app/schema_vocab_test.go`, `internal/app/schema_registry.go` (`operationBindings`, sorted), `internal/cli/change.go`, `internal/cli/install.go` (`assetIndependent`), any cli test lists that enumerate change subcommands (`internal/cli/change_test.go`), `docs/reference/glossary.md` (the operation-dispositions table row)
- Test: `internal/domain/actions_test.go`, `internal/app/change_complete_test.go` (new, modelled on `change_kill_test.go`), `internal/app/schema_vocab_test.go`

**Interfaces:**
- Consumes: Task 1 `Deliverable()`; Task 4 `resultsArtifactProblem`; existing `buildKillCandidate`, `retargetArtifactBacklinks`, `applyClaimRemovals`, `lifecycleFieldValue`, `refuseLifecycle`, `refuseLifecyclePolicy`, `validateLifecycleShape`, `readTreeBlob`, `changeScope`, `includeBoard`, `domain.ClaimEligibility`, `domain.NewBranchFacts`.
- Produces:
  - `func Complete(c Change) (ActionResult, *PolicyFailure)` — `requireStatus(c, "complete", StatusProposed, StatusInProgress)`; `Deliverable() != DeliverableResultsArtifact` ⇒ `FailInvalidState "not-results-artifact"`; PR present ⇒ `FailInvalidState "pr-recorded"`; empty `Results()` ⇒ `FailBlocked "results-missing"`. Result: status `done`, `clearClaimedAt()`, branch kept, `own(RunHaltedSection)` when `HasRunHalted()`. It does not touch `updated:` (the app upserts it, as kill does).
  - `const OperationChangeComplete = "change.complete"`.
  - `CompleteDispCompleted = "completed"`, `CompleteDispAlreadyCompleted = "already-completed"`, `CompleteDispContended = "contended"`, `CompleteDispFailed = "failed"`; vocabulary `complete_dispositions`.
  - `type ChangeCompleteRequest struct { ChangeID int \`json:"change_id" docket:"required"\`; Path string \`json:"path" docket:"required"\`; Revision string \`json:"revision" docket:"required"\` }`.
  - `type ChangeCompleteResult struct { Envelope; ID int \`json:"id,omitempty"\`; Disposition string \`json:"disposition,omitempty" docket:"enum=complete_dispositions"\`; ArchivePath string \`json:"archive_path,omitempty"\`; Revision string \`json:"committed_revision,omitempty"\`; Findings []StatusFinding \`json:"findings"\` }` with `HumanText()`: `change NNNN completed (archived <path>) — <rev>` / `change NNNN already completed (<path>)` / `change complete: <result>`.
  - `func ChangeComplete(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeCompleteRequest) ChangeCompleteResult`.
  - CLI `docket change complete --request <file> [--repo-dir <dir>]` via `changeSubcommand(…, EffectMetadataWrite)`.

- [ ] **Step 1: Write the failing tests**

```go
// domain/actions_test.go
func TestCompleteMatrix(t *testing.T) {
	// proposed+results-artifact+results set -> done, claimed_at cleared (from in-progress), branch kept,
	//   OwnedRemovals == [RunHaltedSection] only when HasRunHalted
	// blocked/deferred/implemented/done/killed -> illegal-source-status
	// pr deliverable -> not-results-artifact; unknown deliverable -> not-results-artifact
	// PR set -> pr-recorded; no results -> results-missing
}

// app/change_complete_test.go (helpers mirror killPlanFor/baseKillOp)
func TestChangeCompleteRejectsBadShapeWithoutEngineCall(t *testing.T)   // invalid-change_id, empty-path, empty-revision
func TestChangeCompletePlanFromProposed(t *testing.T) {
	// build-ready proposed change with a valid results file on the tree:
	// files = archive create at docs/changes/archive/2026-08-16-NNNN-<slug>.md + active delete
	//   (+ results/spec backlink replaces + board); archived bytes: status: done, updated: 2026-08-16,
	//   no claimed_at value, artifacts block re-rendered; CommitSubject "change NNNN completed (done, archived)"
}
func TestChangeCompletePlanFromInProgressWithAndWithoutRunHalted(t *testing.T) {
	// claimed_at cleared, branch: kept, "## Run halted" section absent from the archived bytes
}
func TestChangeCompletePlanRefusals(t *testing.T) {
	// each refused, no files: pr deliverable (not-results-artifact); blocked, deferred, implemented
	// (illegal-source-status); needs-grooming proposed (not-ready-needs-grooming); open depends_on
	// (not-ready-waiting-dependency); pr: set (pr-recorded); no results (results-missing);
	// results file absent from tree / foreign backlink (results-identity-broken);
	// results failing the final contract (results-content-invalid)
}
func TestChangeCompleteReplaysAlreadyCompleted(t *testing.T) {
	// corpus holds the change archived as done + results-artifact -> ResultNoOp,
	// Disposition already-completed, ArchivePath = archived path, engine not called
}
func TestChangeCompleteRevisionMismatchContends(t *testing.T) {
	// engine pin mismatch -> ResultContended, Disposition contended, nothing written
}
func TestChangeCompleteUsesScopedValidation(t *testing.T) {
	// the transaction.Request the recording engine receives carries a non-nil Scope
}
```

Add `{vocab: "complete_dispositions", byPrefix: "CompleteDisp"}` to `TestVocabularyConstCompleteness`, and extend whatever cli test enumerates change subcommands / catalog ids with `complete` / `change.complete`.

**Mutation check (spec test 10):** delete the deliverable check from `domain.Complete`, run `TestCompleteMatrix` and `TestChangeCompletePlanRefusals` with `-count=1`, confirm the `pr deliverable` cases go red, restore from a backup copy (not `git checkout`).

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/domain/ -run TestComplete ; go test -count=1 ./internal/app/ -run 'TestChangeComplete|TestVocabularyConstCompleteness'`
Expected: FAIL — undefined `Complete` / `ChangeComplete`.

- [ ] **Step 3: Implement**

- `ChangeComplete`: shape via `validateLifecycleShape("change_id", …)`; `PinContext`; `resolveBoardSurface`; `Discover`; one corpus read + `BuildSnapshot`; when the id resolves to a change that is `done` with `results-artifact`, return `ResultNoOp` / `already-completed` / `ArchivePath: c.Path()` without an engine call. Otherwise `Engine.Execute` with the exact-blob `Expected` pin on `req.Path`/`req.Revision`, `newPlanningLoader(eff)`, `Scope: changeScope(req.ChangeID, req.Path, false)`, and `changeCompleteOp`.
- `changeCompleteOp.Plan` (re-runs per attempt, reading the clock each time): look up the change (`FCNotFound`); `domain.Complete` (refuse via `refuseLifecyclePolicy`); for `proposed`, `domain.ClaimEligibility(snap, c, domain.NewBranchFacts(nil))`; read the results blob with `readTreeBlob` and refuse on any `resultsArtifactProblem` reason (`refuseLifecycle(FindingCode(reason), msg)`); `applyClaimRemovals(src, result.OwnedRemovals)`; patch `result.Changed` with `lifecycleFieldValue`, `upsertField` `updated`; archive path from the clock's UTC date; `buildKillCandidate`; `ArtifactBlockContent` + `ReplaceBlock`; files = archive create + active delete; `retargetArtifactBacklinks`; `includeBoard`; receipt `{archive_path, id, op}` (sorted keys).
- Outcome mapping: `mapOutcome(res, execErr, ResultInvalidState)`; disposition applied ⇒ `completed`, contended ⇒ `contended`, failed ⇒ `failed`, refused ⇒ empty (the reason is the first finding's code).
- Register every new token in `finding_codes.go` / `AllFindingCodes` (`FindingCode("not-results-artifact")`, `FindingCode("pr-recorded")`, plus `results-missing` if not already registered); the `complete_dispositions` vocabulary; the `change.complete` binding (`Request: ChangeCompleteRequest{}`, `Result: ChangeCompleteResult{}`, comment `// ChangeComplete`); the cobra subcommand with Short "Complete a results-artifact change, archiving it as done without a PR, from a JSON request"; `"change complete": true` in `assetIndependent`.
- Glossary: add the `complete_dispositions` row to the operation-dispositions table (`change complete`).

- [ ] **Step 4: Run to verify they pass**

Run: `go test -count=1 ./internal/domain/ ; go test -count=1 ./internal/app/ -run 'TestChangeComplete|TestChangeKill|TestVocabularyConstCompleteness|TestNoInlineFindingCodeLiterals|TestOperationBindings' ; go test -count=1 ./internal/cli/`
Expected: PASS, including `TestCapabilitiesPayloadWithinByteBudget` (log shows < 16384 bytes) and `TestPublishedRequestIsTheDecodedJSONFile`.

- [ ] **Step 5: Commit**

```bash
git add internal/domain internal/app internal/cli docs/reference/glossary.md
git commit -m "feat(change): complete a results-artifact change to done without a PR"
```

---

### Task 6: Artifacts links for completed changes and the archive header

**Files:**
- Modify: `internal/render/artifacts.go` (`legacyArtifactBranch` and the `ArtifactBlockContent` doc comment), `internal/render/board.go` (archive table header and the comment naming it), `internal/render/testdata/board/board.golden`
- Test: `internal/render/artifacts_test.go`, `internal/render/board_test.go`

**Interfaces:**
- Consumes: Task 1 `Change.Deliverable()`.
- Produces: `legacyArtifactBranch` returns `("", false)` for any `results-artifact` change.

- [ ] **Step 1: Write the failing tests**

```go
func TestArtifactBlockCompletedResultsArtifactLinksMetadataBranch(t *testing.T) {
	// done, results-artifact, no ## Build evidence, Results + Plan set, archived path:
	// shared LinkContext (web URL, integration "main", metadata "docket") and private
	// LinkContext (no web URL, metadata "dckt"): the Results row is relativeRow output
	// (relative link from the record), never a URL containing "/blob/main/" nor a backtick path;
	// no "Spec (merged)" row
}
func TestArtifactBlockLegacyDonePRUnchanged(t *testing.T) {
	// done, deliverable absent (pr), no build evidence -> Results row still absolute on the integration branch
}
func TestBoardArchiveHeaderSaysClosed(t *testing.T) {
	// rendered board contains "| # | Title | Closed |" and not "| # | Title | Merged |"
}
```

**Mutation check (spec test 10):** restore the legacy rule for completed changes (drop the new deliverable clause) and confirm `TestArtifactBlockCompletedResultsArtifactLinksMetadataBranch` reddens; restore from a backup copy.

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/render/ -run 'TestArtifactBlock|TestBoardArchiveHeader'`
Expected: FAIL.

- [ ] **Step 3: Implement** — one early return in `legacyArtifactBranch` for `DeliverableResultsArtifact` with a comment that a results artifact never lives on the integration branch (refines ADR-0141's legacy-row rule); header string `| # | Title | Closed |` and the comment `"# | Title | Closed"`; regenerate `board.golden` via the board test's update flag (or edit the one header line) and confirm that is the only golden diff.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -count=1 ./internal/render/ ; go test -count=1 ./internal/app/ -run 'Board'`
Expected: PASS. Any other test asserting `Merged` in the archive header is updated to `Closed` (find with `grep -rn "Title | Merged" internal tests`).

- [ ] **Step 5: Commit**

```bash
git add internal/render
git commit -m "feat(render): completed changes link results on the metadata branch; archive header says Closed"
```

---

### Task 7: `finalize cleanup` no-PR path, the sweep assessment, and the private end-to-end proof

**Files:**
- Modify: `internal/app/finalize_cleanup.go` (`FinalizeCleanup`, `finalizeCleanupWorkspace` signature, new functions), `internal/app/maintenance_assess.go` (`sweepAssessHistorical`)
- Test: `internal/app/finalize_cleanup_test.go`, `internal/app/maintenance_assess_test.go`, `internal/app/finalize_cleanup_integration_test.go`

**Interfaces:**
- Consumes: Task 1 `Deliverable()`; Task 5 `ChangeComplete` (integration test only).
- Produces:
  - `finalizeCleanupWorkspace(ctx, deps, cc) (clean, removed bool, finding, note *StatusFinding)` — `removed` is `res.Disposition == workspace.CleanupCleaned`; the existing caller ignores it.
  - `func finalizeCleanupResultsArtifact(ctx context.Context, deps FinalizeDeps, cc *closeoutContext) CleanupOpResult`.
  - `func finalizeCleanupLocalRefNoPR(ctx context.Context, git FinalizeCleanupGit, cc *closeoutContext, featureRef gitcli.RefName, featureBranch string) (done bool, removedRef string, finding *StatusFinding)`.

Behaviour of `finalizeCleanupResultsArtifact` (dispatched from `FinalizeCleanup`'s `StatusDone` arm when `Deliverable() == results-artifact`; it never calls `deps.GitHub` and never touches a remote ref):
1. `recordedBranch` returns `errBranchMissing` ⇒ `ResultNoOp`, `already-clean`, message "the completed change has no recorded feature branch; nothing to clean". A malformed branch ⇒ the existing pending refusal.
2. Workspace leg via `finalizeCleanupWorkspace`; not clean ⇒ pending (branch retained), exactly as the PR path.
3. Local leg `finalizeCleanupLocalRefNoPR`: `ResolveRef` — `KindRefUnavailable` ⇒ done/absent, other error ⇒ `ref-probe-failed`; `FetchBranch(origin, refs/heads/<integration>)` error ⇒ `ancestry-probe-failed`; `IsAncestor(tip, integrationTip)` error ⇒ `ancestry-probe-failed`, false ⇒ `tip-not-in-merge-chain`; `ListWorktrees` error ⇒ `worktree-list-probe-failed`, branch checked out anywhere ⇒ `branch-checked-out`; `DeleteLocalBranchChecked(featureRef, tip)` error ⇒ `local-ref-delete-failed`; success ⇒ `"local:" + branch`.
4. Disposition: any finding ⇒ pending (`finalizeCleanupResult`); else a removed workspace or ref ⇒ `cleaned` (`ResultApplied`); else ⇒ `already-clean` (`ResultNoOp`). Workspace-remnant notes ride along as today.

Sweep: in `sweepAssessHistorical`, a done `results-artifact` record skips the PR-reference check and the PR-backlink and remote-ref legs; no recorded branch ⇒ `SweepDispSkipped` / `ReasonSweepSnapshotNoWork` ("completed without a feature branch; nothing to clean"); otherwise the base/target checks and the workspace + local-ref legs run as for any done record.

- [ ] **Step 1: Write the failing tests** (unit tests use the existing cleanup fakes; inject a GitHub fake whose every method fails the test)

```go
func TestFinalizeCleanupResultsArtifactRemovesCleanWorkspaceAndOwnlessBranch(t *testing.T)
	// workspace cleaned; tip is an ancestor of origin/main -> cleaned, RemovedRefs ["local:<branch>"], zero GitHub calls
func TestFinalizeCleanupResultsArtifactKeepsDirtyWorkspace(t *testing.T)          // workspace blocked -> pending, no ref delete
func TestFinalizeCleanupResultsArtifactKeepsBranchWithOwnCommit(t *testing.T)     // IsAncestor false -> pending, reason tip-not-in-merge-chain, branch kept
func TestFinalizeCleanupResultsArtifactNeverTouchesRemote(t *testing.T)           // zero ProbeRemoteBranch / DeleteRemoteRefLease calls
func TestFinalizeCleanupResultsArtifactAlreadyClean(t *testing.T)                 // no branch: -> already-clean; workspace already clean + ref absent -> already-clean
func TestFinalizeCleanupResultsArtifactBranchCheckedOutElsewhere(t *testing.T)    // Review Focus 2: branch-checked-out, kept
func TestFinalizeCleanupResultsArtifactFetchFailureRetainsBranch(t *testing.T)    // Review Focus 3: ancestry-probe-failed, kept
func TestSweepAssessCompletedResultsArtifact(t *testing.T) {
	// live owned workspace -> actionable; no branch -> entry snapshot-no-work;
	// never snapshot-invalid for lacking a PR; zero PR-body numbers gathered for it
}
```

Integration (`//go:build integration`, cleanup shard):

```go
func TestIntegrationFinalizeCleanupResultsArtifactRealGit(t *testing.T)
	// real repo: claimed results-artifact change, prepared worktree, attach final results,
	// ChangeComplete -> completed; FinalizeCleanup -> cleaned; worktree dir gone, local branch gone,
	// a same-named branch pushed to origin beforehand is still there
func TestIntegrationFinalizeCleanupResultsArtifactPrivate(t *testing.T)
	// spec test 11: newPrivateLeakWorkspaceFor(t, create with Deliverable results-artifact, <spec flag the
	// helper needs for a claimable change>);
	// attach results, ChangeComplete, FinalizeCleanup; metadata commit on dckt; the shared remote
	// has no new branch or ref; worktree and local branch removed
```

**Mutation check (spec test 10):** remove the `IsAncestor` containment check from `finalizeCleanupLocalRefNoPR`, confirm `…KeepsBranchWithOwnCommit` reddens (the branch is deleted), restore from a backup copy.

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestFinalizeCleanupResultsArtifact|TestSweepAssessCompletedResultsArtifact'`
Expected: FAIL.

- [ ] **Step 3: Implement** per the behaviour list above; update the file-header comment of `finalize_cleanup.go` to describe the no-PR path in one sentence.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestFinalizeCleanup|TestSweepAssess|TestGatherSweepSharedFacts' ; go test -tags integration -count=1 -run '^TestIntegrationFinalizeCleanup' ./internal/app/`
Expected: PASS. Read the shard's budget line in `tests/runtime-budgets.tsv` terms when the full suite runs; the two new integration tests are the cost.

- [ ] **Step 5: Commit**

```bash
git add internal/app
git commit -m "feat(finalize): clean up a completed results-artifact change without a PR"
```

---

### Task 8: `run verify` results-artifact postconditions

**Files:**
- Modify: `internal/app/run_verify.go`
- Test: `internal/app/run_verify_test.go`, `internal/app/runtracker_verdict_integration_test.go`

**Interfaces:**
- Consumes: Task 1 `Deliverable()`.
- Produces: `ReasonRunNotCompleted = "not-completed"`; `func runVerifyResultsArtifact(ctx context.Context, deps PlanningDeps, pin StatusPin, c domain.Change, id int) RunVerifyResult`.

Order inside `RunVerify`: `proposed` ⇒ `run-unclaimed`; `HasRunHalted` ⇒ `run-halted` (both unchanged); then a `results-artifact` change goes to `runVerifyResultsArtifact`, which: adds `not-completed` (observed = raw status) unless status is `done`; adds `results-unlinked` / `results-identity-broken` / `results-content-invalid` exactly as the PR path computes them (an artifact read error stays operational `artifact-read-failed`); no workspace inspection, remote probe, PR probe, or evidence check. No unmet ⇒ `run-complete` with empty head and PR; else `run-incomplete`. The file-header comment names the results-artifact branch.

- [ ] **Step 1: Write the failing tests**

```go
func TestRunVerifyResultsArtifact(t *testing.T) {
	// GitHub and workspace fakes fail the test on any call.
	// done + valid results -> run-complete, Unmet empty
	// in-progress + valid results -> run-incomplete, Unmet [{not-completed, "in-progress"}]
	// in-progress, no results -> not-completed + results-unlinked
	// proposed -> run-unclaimed; ## Run halted -> run-halted
	// done, results file absent (Review Focus 5) -> run-incomplete, results-identity-broken
	// pr change paths unchanged (existing tests stay green)
}
```

Integration (`TestIntegrationRunVerdict…` shard): `TestIntegrationRunVerdictCompletedResultsArtifactReadsDone` — follow the existing keyed run-complete case: start a run, claim a results-artifact change with the run context, attach results, `ChangeComplete`, then `run.verdict` with the key reads the run-done report line and drives the successful-run closeout with no gate drive. While writing it, read every `VerdictRunComplete` arm in `runtracker_verdict.go` and `runtracker_complete.go` and note in the commit message that none assumes a PR or a gate drive (or fix the one that does).

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/app/ -run TestRunVerifyResultsArtifact`
Expected: FAIL.

- [ ] **Step 3: Implement** the branch and the reason constant.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestRunVerify' ; go test -tags integration -count=1 -run '^TestIntegrationRunVerdict' ./internal/app/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app
git commit -m "feat(run): verify a results-artifact run by completion and results"
```

---

### Task 9: Every "already" result names what already happened

**Files:**
- Modify: `internal/app/finalize_closeout.go` (`CloseoutDispAlready`), `internal/app/finalize_block.go` (`BlockDispAlready`), `internal/app/finalize_retarget.go` (`childOutcomeAlready`), `internal/app/pr_backlink_repoint.go` (`prBacklinkAlready`), `internal/githubcli/bodyedit.go` (`BodyAlready`), `internal/githubcli/comment.go` (`CommentAlready`), `internal/githubcli/retarget.go` (`RetargetAlready`)
- Modify readers: `skills/docket-finalize-change/SKILL.md`, `skills/docket-convention/references/stacked-changes.md`, `docs/reference/glossary.md` (`block_dispositions`, `closeout_dispositions` rows and any prose naming the bare value), every test asserting the old value; budgets rows for the two skill files; `go generate ./internal/assets/`
- Test: `internal/app/finalize_cleanup_test.go` (beside `TestFinalLifecycleCodeSpellings`), `internal/app/schema_vocab_test.go`

**Interfaces:**
- Produces (values only; Go constant names stay): `already-closed-out`, `already-recorded`, `already-retargeted` (child outcome), `already-repointed`, `already-edited`, `already-posted`, `already-retargeted` (githubcli).

- [ ] **Step 1: Write the failing tests**

```go
func TestAlreadyOutcomeSpellings(t *testing.T) {
	pins := []struct{ got, want string }{
		{CompleteDispAlreadyCompleted, "already-completed"},
		{CloseoutDispAlready, "already-closed-out"}, {BlockDispAlready, "already-recorded"},
		{childOutcomeAlready, "already-retargeted"}, {prBacklinkAlready, "already-repointed"},
		{string(githubcli.BodyAlready), "already-edited"}, {string(githubcli.CommentAlready), "already-posted"},
		{string(githubcli.RetargetAlready), "already-retargeted"},
	}
	for _, p := range pins { if p.got != p.want { t.Errorf("%q, want %q", p.got, p.want) } }
}
func TestNoSchemaVocabularyHasBareAlready(t *testing.T) {
	for name, v := range SchemaVocabularies([]string{"noop"}) {
		for _, m := range v.Members { if m == "already" { t.Errorf("vocabulary %s carries a bare already", name) } }
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestAlreadyOutcomeSpellings|TestNoSchemaVocabularyHasBareAlready'`
Expected: FAIL on the seven old values.

- [ ] **Step 3: Implement** — change the seven string values. Then find every reader with a whole-repo grep, never this list alone: `grep -rn -e '"already"' -e '`already`' -e "'already'" internal skills docs scripts tests cmd --include='*.go' --include='*.md' --include='*.sh'`, excluding point-in-time records (`docs/changes`, `docs/superpowers`, `docs/results`, `docs/adrs`). Update skill routing prose (finalize closeout routing; stacked-changes root-archived replay), glossary rows and prose, and test assertions. Re-baseline the two skill budget rows and run `go generate ./internal/assets/`.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestAlreadyOutcomeSpellings|TestNoSchemaVocabularyHasBareAlready|TestFinalizeCloseout|TestFinalizeBlock|TestFinalizeRetarget|TestPRBacklink' ; go test -count=1 ./internal/githubcli/ ./internal/repoguard/ ./internal/assets/`
Expected: PASS. Mutation check: set `BlockDispAlready` back to `"already"` and confirm both new tests redden; restore from a backup copy.

- [ ] **Step 5: Commit**

```bash
git add internal skills docs/reference/glossary.md
git commit -m "refactor: name what already happened in every no-op replay result"
```

---

### Task 10: implement-next's results-artifact ending and the review skill's results-artifact mode

**Files:**
- Create: `skills/docket-implement-next/references/results-artifact.md`
- Modify: `skills/docket-implement-next/SKILL.md` (pointer paragraph in Step 4, Step postconditions, Terminal disposition table and the "Step 7's postcondition" sentences), `skills/docket-review/SKILL.md` (new `## Results-artifact mode` section)
- Modify: `internal/repoguard/budgets_test.go` (re-baseline two rows, add a row for the new reference), regenerated assets under `internal/assets/`
- Test: `go test ./internal/repoguard/ ./internal/assets/` (budgets, feature-dispatch markers, prose contracts)

**Content requirements:**

`SKILL.md` pointer (one paragraph, directly after the `workspace.prepare` paragraph in Step 4, before *Preparation (parent)*): when the `context.implementation` bundle reports `deliverable: results-artifact`, read `references/results-artifact.md` now (blocking) and follow it instead of the rest of Step 4 and Steps 5–7 — including skipping `workspace.commit-spec`; a `deliverable` that is neither `pr` nor `results-artifact` is a `halted` run. Selection, claim, reconcile, and `workspace.prepare` are unchanged.

`SKILL.md` disposition and postconditions: the `advanced` row reads "Built a change → PR opened (Step 7 reached), or completed a results-artifact change (done and archived, with a valid results artifact attached)". The postconditions table gains a row for the results-artifact ending: the change `done` and archived on the metadata branch, `results:` set, and the final results file with its backlink on the metadata branch. The sentences saying Step 7's postcondition is the only one that completes the run, and that `advanced` is claimable only when Step 7's postcondition holds, name the results-artifact ending's row too.

`references/results-artifact.md` — spec §5 items 1–7 in skill voice, plus:
- the work happens in the feature worktree; never commit to or publish the branch; never modify the primary checkout; keep findings, numbers, and the commands or short scripts that produced them inline in the results artifact; delete everything else from the worktree before cleanup; a needed code change or a must-keep file the results artifact cannot carry ⇒ `change.halt` saying so;
- human steps: do the work it can, halt at the step (not earlier) naming it in the halt report; a person finishes with `docket change complete`;
- results under Step 6.5's existing rules (template, `**Human action:**`, checkpoints, backlog match);
- review: dispatch `docket-review-standard` inside a `<!-- docket:feature-dispatch:start targets=docket-review-standard -->` … `<!-- docket:feature-dispatch:end -->` block whose payload carries the exact line `Feature worktree: <absolute canonical feature-worktree root>` and, inline, the spec body (or trivial change body), the results artifact's text, the change title and scope, and the relevant learnings hooks — copy the block shape from `SKILL.md` Step 6;
- fix pass: the run revises the results artifact itself for findings at or above `review.min_fix_severity` (read through `diagnostic.config`; blockers always), capped by `review.max_fix_tasks`, then re-attaches; no fix workers, no suite gate, no re-review; below-threshold findings go into Known issues;
- complete then clean up: `change.complete` (request `{change_id, path, revision}` pinned from `status --json`), then `finalize.cleanup --id <id>`; a `pending` cleanup goes into the run report and never halts or undoes the completion; end with `advanced`;
- refresh the claim at phase boundaries as in the `pr` path;
- the final report names the results path and the cleanup disposition.

`docket-review/SKILL.md` `## Results-artifact mode` (short): when the payload carries a spec body and a results artifact instead of a branch and an evidence record, skip evidence verification and the branch diff; check that the results answer what the spec asked, that claims and numbers trace to cited sources or commands a reader could re-run, that the `**Human action:**` line is accurate, and that follow-ups are named; same return schema and severities.

- [ ] **Step 1:** Write the reference file and the two skill edits.
- [ ] **Step 2:** Run `go generate ./internal/assets/`, then `go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/harness/` — expect budget failures naming the exact new counts and the missing row.
- [ ] **Step 3:** Re-baseline the two rows and add `{"docket-implement-next/references/results-artifact.md", <lines>, <words>}` at the exact counts, each with a `0559:` note.
- [ ] **Step 4:** Re-run Step 2's command — expect PASS, including the feature-dispatch repoguard (`feature_worktree_dispatch_test.go`) over the new block. Mutation check: delete the payload line from the new block and confirm the repoguard reddens; restore.
- [ ] **Step 5: Commit**

```bash
git add skills/docket-implement-next skills/docket-review internal/repoguard/budgets_test.go internal/assets
git commit -m "feat(skills): implement-next completes a results-artifact change; review checks results"
```

---

### Task 11: Convention, grooming, and capture skills; attach-results help text

**Files:**
- Modify: `skills/docket-convention/SKILL.md`, `skills/docket-convention/references/close-out.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-new-change/SKILL.md`, `internal/cli/change.go` (attach-results Short), `internal/repoguard/budgets_test.go`, regenerated `internal/assets/`
- Test: `go test ./internal/repoguard/ ./internal/assets/ ./internal/cli/`

**Content requirements (spec §8):**
- `docket-convention/SKILL.md`: the manifest lists `deliverable:` (`pr` default, `results-artifact`); the lifecycle diagram and status table say `done` means a merged PR **or** a completed results artifact; the archive-date sentence becomes "the merge date for a merged change, and the transaction date for a completed results artifact and for a killed change"; name `change.complete`.
- `close-out.md`: `change.complete` as the third done driver, owning its own transaction as kill does; `finalize.cleanup`'s no-PR path (removes a clean worktree and a local branch with no commits of its own; never a remote ref); fix kill's flag to `--request` (not `--input`) and say kill is not idempotent (a retry after success is refused).
- `docket-groom-next/SKILL.md`: one short paragraph beside Retitle and Reprioritize covering `deliverable`, `type`, and `branch_prefix` on `change.groom` (branch_prefix: omit to keep, `""` to clear).
- `docket-new-change/SKILL.md`: one clause — when the work ends in findings, not code, the create request carries `deliverable: results-artifact`.
- `internal/cli/change.go`: attach-results Short drops "an in-progress change" ("Write a results record on the metadata branch and link it to a change").
- Find every other skill sentence that equates `done` with a merged PR or `advanced` with a PR: `grep -rn -i -e 'merged PR' -e 'merged pull request' -e 'advanced' -e 'done means' skills` and update the ones that state the equivalence as a rule.

- [ ] **Step 1:** Make the edits.
- [ ] **Step 2:** `go generate ./internal/assets/` then `go test -count=1 ./internal/repoguard/ ./internal/assets/ ./internal/cli/` — expect budget failures with exact counts.
- [ ] **Step 3:** Re-baseline each touched skill row at the exact counts with a `0559:` note.
- [ ] **Step 4:** Re-run — expect PASS.
- [ ] **Step 5: Commit**

```bash
git add skills internal/cli/change.go internal/repoguard/budgets_test.go internal/assets
git commit -m "docs(skills): the convention, grooming, and capture skills know the results-artifact ending"
```

---

### Task 12: User docs

**Files:**
- Modify: `docs/reference/glossary.md` (define `deliverable` and results artifact; lifecycle table: `done` is reached by finalize close-out, the sweep, or `change complete`; `deliverables` vocabulary if the field-values table lists vocabularies), `docs/concepts/change-lifecycle.md`, `docs/guide/building-without-supervision.md` (the `advanced` bullet), and every other user doc the greps below find
- Test: `go test -count=1 ./internal/repoguard/` (prose contracts and doc guards)

- [ ] **Step 1:** Find the sites by grep, not from the list: `grep -rn -i -e 'advanced' -e 'merged pull request' -e 'merged PR' -e 'reaches done' -e 'to done' docs README.md --include='*.md' | grep -v -e '^docs/changes/' -e '^docs/superpowers/' -e '^docs/results/' -e '^docs/adrs/'`. Sort hits into "states that done implies a merge / advanced implies a PR" (edit) and incidental (leave).
- [ ] **Step 2:** Edit: describe only current behaviour, no change or PR citations (ADR citations are fine). `advanced` (implement-next) = "built a change and opened a pull request, or completed a results-artifact change". Do not touch finalize's `advanced` (it still means a change closed out).
- [ ] **Step 3:** Run `go test -count=1 ./internal/repoguard/` — expect PASS; fix any prose-contract sentinel the edits moved (relocate the assert to the owning text, never re-add removed text).
- [ ] **Step 4: Commit**

```bash
git add docs README.md
git commit -m "docs: done no longer implies a merge; describe change complete"
```

---

## ADR to record (implement-next Step 6, through `docket-adr` — not a build task)

One ADR, related to ADR-0129, ADR-0141, ADR-0148: a change's deliverable is a PR or a results artifact; `done` no longer implies a merge (finalize close-out, the sweep, or `change complete` reach it); a completed change's Plan/Results rows are never legacy (refines ADR-0141's legacy-row rule); a no-op replay result names what already happened (`already-<what happened>`), never a bare `already`. The controller records its number in `adrs:` through `change.reconcile`.

## Self-review notes

- Spec coverage: §1 → Tasks 1–3; §2 → Task 3; §3 → Tasks 4–5; §4 → Tasks 6–7; §5 → Task 10; §6 → Task 8; §7 → Tasks 5, 7, 11; §8 → Tasks 10–12 + ADR; §9 → Task 9. Spec tests 1–12 map to Tasks 1–3 (1–2), 5 (3–4), 4 (5), 6 (6, 8), 7 (7, 11), 8 (9), 5/6/7 mutation checks (10), 9 (12).
- Buildable at every task boundary: Task 9 updates skill/doc readers in the same task as the value change; Tasks 10–11 re-baseline budgets in the same commit as the skill edits.
