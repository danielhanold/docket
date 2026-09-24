<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0449 — Unrelated invalid change records must not block a named change's metadata writes or board](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0449-unrelated-invalid-change-records-must-not-block-a-named-chan.md)**
<!-- docket:backlink:end -->
# Unrelated Invalid Change Records Must Not Block a Named Change Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. (In this repo the build role is `docket-build`, which fulfills the subagent-driven contract.)

**Goal:** A named change B completes every metadata write on its path — claim, refresh/reconcile, attach, lifecycle, mark-implemented, halt, repair, required ADR writes, finalize block/clear-block, and archive/closeout — and the canonical board still updates, despite unrelated invalid records elsewhere in the corpus; while defects in B, in records B actually requires, any NEW or CHANGED error anywhere, illegal evolution, and shared-infrastructure failures all still refuse.

**Architecture:** One bounded in-memory extension at the existing transaction boundary: `transaction.Request` gains an optional `Scope *ValidationScope` carrying a caller-supplied subject resolver. A nil scope keeps today's strict whole-corpus gates byte-for-byte; a non-nil scope makes the engine's before/after gates refuse only errors *relevant* to the resolved subject set, grandfathering **exact** pre-existing error findings that are confined to unrelated records whose tree blob ids are unchanged. Relevance and equality live in a new pure module (`internal/repository/transaction/scope.go`): findings compare as canonicalized multisets (full struct — `Related`, `Detail`, severity, multiplicity), and every finding is mapped to record paths in *both* states via snapshot id/slug lookups; anything unresolvable is relevant (fail closed). An **empty resolved subject set falls back to strict whole-corpus validation** — absence of scope can never mean "ignore every error". The production loader starts retaining source bytes and tree blob ids for parse-failed records so "unchanged" is provable. Named operations in `internal/app` supply the resolver (structural closure per ADR-0093: root + depends_on + stack ancestors/descendants + operation-required ADR/artifact paths — never associative `related`/`discovered_from`/`adrs` citations); unscoped operations (create, groom, kill, reclaim, learning ops, evidence ops) pass no scope and stay strict. Separately: the canonical board renderer stops aborting on one bad record (per-record classify/readiness errors and caller-supplied unrenderable parse-failed records land in a "Needs repair" notice), and named live branch-fact probes bound to B's actual base/stack set instead of every stack branch in the corpus.

**Tech Stack:** Go (module `github.com/danielhanold/docket`); table-driven unit tests plus the existing isolated-git integration harnesses in `internal/app`; suite gate via the source Go runner: `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-23-unrelated-invalid-change-records-must-not-block-a-named-chan-design.md` (synchronized metadata tree: `.docket/docs/superpowers/specs/...` on branch `docket`; read it alongside this plan — its Grounding and Final-design-logic-check sections are normative).

## Global Constraints

- **No new commands, flags, config keys, persisted baselines, error-code allowlists, alternate renderers, lifecycle statuses, or permissive parsers.** The only new validation input is the in-memory subject scope on existing operations.
- **Never weaken the strict path:** a nil `Scope` must execute today's exact gate sequence; `TestEngineBeforeGateRefusesInvalidBase` (`internal/repository/transaction/engine_test.go`) must keep passing unmodified.
- **Empty scope never disables validation:** a resolver error, a nil resolver inside a non-nil scope, or an empty resolved subject set ⇒ strict whole-corpus behavior for that gate (fail closed).
- **Keep unchanged:** the complete corpus read, exact-version `Expected` checks, `ValidateEvolution` (frozen-record protection), declared-path/materialize/verify-delta enforcement, lease push + `maxAttempts` retry, idempotent replay, `AdmissionHook`. Scope and baseline are derived fresh inside each attempt (`runCandidate`), never cached across attempts.
- **ADR-0093 boundary:** structural references (`depends_on`, `stacked_on`, an ADR being recorded/superseded and its targets) create subjects; associative `related`/`discovered_from`/`adrs` citations never do.
- **Read-only health checks keep reporting the whole repository** — `status`, health checks, and their findings surfaces are untouched.
- **Point-in-time records are never edited** (`docs/changes/**`, `docs/adrs/**` bodies, existing specs/results); the departure from #0309 is recorded as a **new** ADR via the docket-adr workflow, not by rewriting Accepted text.
- Every new guard/test is **mutation-tested** with the cache defeated: `go test -count=1 …` (learnings: guards-are-code, cached-runner-serves-a-mutated-tree). A mutation that leaves an assert green is a defect until proven otherwise.
- Comparison plumbing: `domain.compareFindings` sorts only by Code/Entity/Field and ignores `Related`/`Detail` — it **cannot** serve as the equality test (spec Grounding). Use the canonical multiset key from Task 1 everywhere.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054). Line numbers in this plan are read-time anchors only — re-locate by symbol before editing.
- The BUILD gate runs the whole configured suite from source (`go run ./cmd/docket development test`); read the budget report even on green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines).

## File Structure

- Create: `internal/repository/transaction/scope.go` + `scope_test.go` — canonical finding key, finding→paths resolution, baseline capture, scoped-gate decision (pure functions).
- Modify: `internal/repository/transaction/state.go` — `LoadedState.Blobs map[string]gitcli.ObjectID`; `ValidationScope`/`SubjectResolver` types.
- Modify: `internal/repository/transaction/engine.go` — `Request.Scope`; scoped before-gate, post-plan recheck, scoped after-gate in `runCandidate`.
- Modify: `internal/app/planning.go` — `planningLoader.Load` retains parse-failed sources and fills `Blobs`.
- Create: `internal/app/subject_scope.go` + `subject_scope_test.go` — the production `SubjectResolver` for change-rooted and ADR-rooted operations.
- Modify (thread scope): `internal/app/change_claim.go`, `change_reconcile.go`, `change_attach.go`, `change_lifecycle.go`, `change_implemented.go`, `change_halt.go`, `change_repair.go`, `finalize_block.go`, `finalize_closeout.go`, `adr_ops.go`.
- Modify: `internal/render/board.go` — per-record tolerance + `BoardInput.Unrenderable` + "Needs repair" notice; `internal/app/derived_views.go` + every `includeBoard` caller — thread unrenderable info.
- Modify: `internal/app/status.go` (`stackBranchesFor`), plus the named branch-fact callers: `change_claim.go`, `workspace_ops.go`, `implementation_context.go`, `finalize_merge.go`, `finalize_block.go`, `finalize_retarget.go`, `change_repair.go`.
- Create: `internal/app/named_isolation_integration_test.go` — the end-to-end named-workflow acceptance tests (spec test 2).
- ADR: one new record via the docket-adr workflow (Task 10).

## Acceptance-test mapping (spec §Acceptance tests)

1. *Metadata isolation and safety* → Tasks 1–7 (engine/scope unit + per-operation integration tests, including the paired refusals, finding-volatility cases, empty-scope, frozen-record, lease-retry, replay).
2. *Complete named workflows end to end* → Task 10 (with Task 9's bounded branch facts and Task 8's board notice).
3. *Mutation checks* → explicit mutation steps inside Tasks 1, 3, 5–9, plus the sweep step in Task 10.
4. *Build gate* → Task 10 final step (`go run ./cmd/docket development test`).

**Worker note on code sketches:** snippets below are unverified plan-supplied code (learnings: plan-supplied-test-code-is-unverified) — adapt names/fields to the real APIs where they diverge, prove every new test CAN fail, and keep the *stated semantics* (fail-closed rules, multiset equality, blob-id proof) which are normative.

---

### Task 1: Canonical finding identity, finding→path resolution, and the scoped-gate decision (pure module)

**Files:**
- Create: `internal/repository/transaction/scope.go`
- Create: `internal/repository/transaction/scope_test.go`
- Modify: `internal/repository/transaction/state.go` (add `Blobs` field + scope types)

**Interfaces (later tasks rely on these exact names):**

```go
// state.go additions
type LoadedState struct {
    Snapshot  domain.Snapshot
    Report    domain.ValidationReport
    Documents map[string]document.Document
    Sources   map[string][]byte
    // Blobs maps every corpus path to its exact tree blob object id, INCLUDING
    // records that failed to parse. Overlay-touched paths carry "" (overlayTree
    // clears ids), which the comparator reads as "changed" — fail closed.
    Blobs map[string]gitcli.ObjectID
}

// SubjectResolver resolves the operation's required record paths from a loaded
// state. It is called for the before state and again for the candidate state.
type SubjectResolver func(st LoadedState) (map[gitcli.RepoPath]bool, error)

// ValidationScope is the bounded in-memory subject contract (change 0449).
type ValidationScope struct{ Subjects SubjectResolver }
```

```go
// scope.go — all pure; no I/O
// canonicalFindingKey serializes one finding completely and deterministically:
// Code, Severity, Field, Entity (all four fields), Related sorted by a total
// EntityRef key, Detail as sorted key=value pairs. Two findings are "the same"
// iff their keys are equal. domain.compareFindings is NOT sufficient (it drops
// Related/Detail) and must not be used here.
func canonicalFindingKey(f domain.Finding) string

// findingPaths resolves every EntityRef a finding carries (Entity + all
// Related) to record paths in st: a non-empty Path is itself; a change ID
// resolves through st.Snapshot.Change, an ADR ID through st.Snapshot.ADR, a
// learning slug through st.Snapshot.Learning. ok=false when ANY ref fails to
// resolve — the caller must then treat the finding as relevant (fail closed).
func findingPaths(f domain.Finding, st LoadedState) (paths []gitcli.RepoPath, ok bool)

// findingRelevant reports whether an error finding touches the scope in EITHER
// state: any resolved path (Entity or Related, resolved in before AND in
// candidate where given) is a subject, or resolution fails anywhere.
func findingRelevant(f domain.Finding, scope map[gitcli.RepoPath]bool, before, after *LoadedState) bool

// baselineErrors captures the before state's grandfather candidates: the
// multiset (key → count) of canonical keys of error-severity findings that are
// NOT relevant to scope in the before state, alongside the union of paths each
// key's findings resolved to.
type errorBaseline struct {
    counts map[string]int
    paths  map[string][]gitcli.RepoPath
}
func baselineErrors(before LoadedState, scope map[gitcli.RepoPath]bool) errorBaseline

// scopedAfterErrors decides the after gate: it returns the error findings that
// must refuse. An after-state error finding survives (is grandfathered) ONLY
// when all of: (1) not relevant to scope in either state; (2) its canonical key
// has remaining count in the baseline (each match decrements — multiset
// semantics, so a count change refuses the surplus); (3) every path it resolves
// to (in both states) has equal, non-empty Blobs ids in before and after.
// Everything else — new keys, changed Detail/Related (a different key), a
// touched path, an unresolvable ref — is returned as a refusal finding.
func scopedAfterErrors(before, after LoadedState, scope map[gitcli.RepoPath]bool) []domain.Finding
```

- [ ] **Step 1: Write the failing tests** in `scope_test.go`. Cover, minimum (each is a spec-named case):
  - `canonicalFindingKey`: two findings equal except `Detail["count"]` ("2" vs "3") get different keys; equal except one `Related` member get different keys; `Related` in different input order get the SAME key; `Detail` map iteration order does not change the key.
  - `findingPaths`: entity with Path only; entity with Kind=change/ID resolving through a snapshot fixture; a Related member that fails lookup ⇒ ok=false; an id-only cycle finding (Entity ID=A, Related contains B) resolves to both paths.
  - `findingRelevant`: id-only cycle attributed to unrelated A whose `Related` contains subject B ⇒ relevant; a finding on A with no connection to scope ⇒ not relevant; unresolvable ⇒ relevant; a dependency removed in the candidate (resolves in before only) still resolves against the *before* state ⇒ before-state obligations survive.
  - `scopedAfterErrors`: identical unrelated finding with equal blob ids ⇒ grandfathered (empty result); same key but blob id changed for its path ⇒ refused; same key but path's after blob id is "" (overlay-touched) ⇒ refused; count 2→3 of an identical key ⇒ exactly the surplus refused; a dangling-reference finding whose `Detail["lookup"]` flips `absent`→`ambiguous` ⇒ different key ⇒ refused; a disappearance of one error plus appearance of a *different* error ⇒ the new one refused (a disappearance is not a license); two before-findings that `domain.compareFindings` ties on (same Code/Entity/Field, different `Related`) are matched by full key, not input position.
  Build tiny snapshots via `repository.BuildSnapshot` fixtures or hand-built `domain` constructors — follow the existing pattern in `internal/repository/transaction/loader_test.go` / `engine_test.go` fakes.
- [ ] **Step 2: Run to verify failure:** `go test ./internal/repository/transaction/ -run 'TestCanonicalFindingKey|TestFindingPaths|TestFindingRelevant|TestScopedAfterErrors' -count=1` — expect FAIL (undefined symbols).
- [ ] **Step 3: Implement** `scope.go` and the `state.go` additions exactly per the interface block. `Blobs` is additive — no existing test may need edits; `nil` Blobs maps make `scopedAfterErrors` refuse any grandfather attempt (missing id ≠ equal id — fail closed), assert that too.
- [ ] **Step 4: Run to verify pass:** same command; then `go test ./internal/repository/transaction/ -count=1` (whole package still green).
- [ ] **Step 5: Mutation-test the two sharpest guards:** (a) make `canonicalFindingKey` ignore `Detail` — the Detail-count test must redden; (b) make `findingPaths` return ok=true on a failed lookup — the fail-closed tests must redden. Restore. Use `-count=1` on every probe.
- [ ] **Step 6: Commit:** `git add internal/repository/transaction/scope.go internal/repository/transaction/scope_test.go internal/repository/transaction/state.go && git commit -m "feat(transaction): canonical finding identity and scoped-gate decision module (change 0449)"`

### Task 2: Production loader retains parse-failed sources and blob ids

**Files:**
- Modify: `internal/app/planning.go` (`planningLoader.Load`)
- Test: `internal/app/planning_test.go` (or the file holding `TestNewPlanningLoaderParseFailureIsFinding` — locate by that symbol)

**Interfaces:** Consumes `LoadedState.Blobs` from Task 1. Produces: every found corpus blob appears in `Sources` (exact bytes) and `Blobs` (ListTree object id), parse-failed included; parse failures remain error findings, never Go errors.

- [ ] **Step 1: Write failing tests:** extend the existing parse-failure loader test: a corpus with one unparseable change record must yield (a) the parse error finding (unchanged assertion), (b) `Sources[badPath]` == the exact bad bytes, (c) `Blobs[badPath]` == the tree entry's object id, (d) `Blobs[goodPath]` non-empty for a healthy record. Note the entries come from `t.ListTree` in `Load` — the `classified`/`paths` loop already walks them; capture `e.ObjectID` alongside.
- [ ] **Step 2: Verify failure:** `go test ./internal/app/ -run TestNewPlanningLoader -count=1` — FAIL.
- [ ] **Step 3: Implement:** in `Load`, record `blobIDs[string(e.Path)] = e.ObjectID` while classifying entries; in the parse-failure branch, ALSO do `sources[rel] = append([]byte(nil), b.Blob.Bytes...)` before `continue`; return `Blobs: blobIDs`. Do NOT add parse-failed docs to `Documents` or `in.Documents`.
- [ ] **Step 4: Check the evolution consumer:** `repository.ValidateEvolution` now sees parse-failed bytes in `BeforeSources`/`AfterSources`. Read `internal/repository/evolution.go` for any assumption that every Sources path has a parsed Document; run `go test ./internal/repository/ ./internal/app/ -count=1`. If an evolution rule trips on source-without-document, constrain the rule to documented paths and add a regression test proving an unparseable A's unchanged bytes produce no evolution finding.
- [ ] **Step 5: Commit:** `git commit -am "feat(app): planning loader retains parse-failed sources and tree blob ids (change 0449)"`

### Task 3: Engine executes the scoped gate sequence

**Files:**
- Modify: `internal/repository/transaction/engine.go` (`Request`, `runCandidate`)
- Test: `internal/repository/transaction/engine_scope_test.go` (new; reuse `engine_test.go` fakes/harness)

**Interfaces:** Consumes Task 1's module. Produces: `Request.Scope *ValidationScope`; scoped semantics at exactly three points of `runCandidate`, everything else byte-identical.

The scoped sequence (spec Design §1, steps 1–4), replacing the three `HasErrors` sites *only when `req.Scope != nil`*:

1. **Before-gate** (after `req.Loader.Load` of `before`): resolve `subjects, err := req.Scope.Subjects(before)`. On `err`, nil resolver, or `len(subjects)==0` ⇒ **strict**: refuse on `before.Report.HasErrors()` exactly as today. Otherwise refuse (via `refusedOutcome(acc, StageLoadBefore, …)`) on every error finding where `findingRelevant(f, subjects, &before, nil)`; unrelated errors do not refuse but note them: capture `base := baselineErrors(before, subjects)`.
2. **Post-plan recheck** (after `validatePlan(plan)` succeeds, before `newOverlayTree`): union every `plan.Files[i].Path` into `subjects`, re-resolve `req.Scope.Subjects(before)` is NOT rerun — the union is `subjects ∪ planPaths(plan)` — then recheck the before report: any error finding now relevant under the widened set refuses at `StagePlan`. Recompute `base := baselineErrors(before, widened)` so a plan-touched record's old error can never be grandfathered.
3. **After-gate** (replacing `after.Report.HasErrors()`): `bad := scopedAfterErrors(before, after, widened)`; refuse on `len(bad) > 0`. `ValidateEvolution` runs unchanged, unconditionally, after it.

All of this lives inside `runCandidate`, so a lease-loss retry re-derives scope and baseline from the fresh fetch automatically — assert it, don't re-engineer it.

- [ ] **Step 1: Write failing engine tests** (fake loader/operation pattern from `engine_test.go`):
  - *Strict unchanged:* nil `Scope` + before-state error ⇒ refused (this is `TestEngineBeforeGateRefusesInvalidBase`'s territory — leave that test untouched, add none).
  - *Scoped progress:* before-state error on unrelated path, scope={B}, plan touches only B, after keeps the identical unrelated finding with equal Blobs ids ⇒ applied, and the unrelated finding does NOT appear in `Result.Findings` as a refusal (spec step 5: typed disposition wins; findings surface stays whatever the operation returns).
  - *Scoped refusals:* (a) error ON B ⇒ refused before Plan (operation's Plan must be provably un-invoked — flag on the fake); (b) error on a subject dependency path ⇒ refused; (c) empty resolved set with corpus errors ⇒ refused (strict fallback); (d) resolver error ⇒ refused (strict fallback), never applied; (e) plan declares a path whose record carried a pre-existing error ⇒ post-plan recheck refuses at StagePlan.
  - *After-gate volatility:* identical-key unrelated finding but after Blobs id differs ⇒ refused; a NEW unrelated error appearing in after ⇒ refused; count increase of an existing key ⇒ refused.
  - *Retry recomputation:* first attempt loses the lease (fake push), second attempt's fetched base carries a DIFFERENT unrelated-error set ⇒ outcome decided by the second base (assert the fake loader was consulted per attempt and the second attempt's new relevant error refuses).
  - *Replay:* a keyed request with `Scope` set replays already-applied without invoking loader validation gates differently (idempotency scan short-circuits before Load — assert unchanged behavior).
- [ ] **Step 2: Verify failure:** `go test ./internal/repository/transaction/ -run TestEngineScope -count=1` — FAIL (no `Scope` field).
- [ ] **Step 3: Implement** per the sequence above. Keep the strict branches as the *same code path* (one small helper `gateBeforeErrors(scope, before) ([]domain.Finding, errorBaseline, widenedScope)` is fine); do not duplicate the whole predicate divergently (learnings: duplicated-gate-copies-the-whole-predicate).
- [ ] **Step 4: Verify pass + package green:** `go test ./internal/repository/transaction/ -count=1`.
- [ ] **Step 5: Mutation-test:** (a) delete the empty-set strict fallback ⇒ case (c) reddens; (b) skip the post-plan recheck ⇒ case (e) reddens; (c) make the after-gate compare only finding Codes ⇒ the volatility cases redden. Restore each; `-count=1` throughout.
- [ ] **Step 6: Commit:** `git commit -am "feat(transaction): optional validation scope on the engine's before/after gates (change 0449)"`

### Task 4: Production subject resolver for change- and ADR-rooted operations

**Files:**
- Create: `internal/app/subject_scope.go`
- Create: `internal/app/subject_scope_test.go`

**Interfaces (Tasks 5–7 consume exactly these):**

```go
// changeScope builds the ValidationScope for an operation rooted at change id
// whose pinned record path is recPath. Resolution runs against EACH loaded
// state the engine hands it (before and candidate), by IDENTITY, not path —
// closeout legally moves B active/→archive/ (learnings:
// relocation-reads-as-identity-reuse). The resolved set is:
//   - recPath itself, plus the path snapshot resolves for id (they differ
//     mid-closeout);
//   - every direct depends_on target's path (LookupFound or not — a failed
//     lookup contributes nothing here; the DANGLING error finding on B itself
//     is already relevant via B, which is what makes "B depends on unparseable
//     A" refuse);
//   - every stack ancestor's path (domain.StackAncestors);
//   - with withDescendants, every stack descendant's path
//     (domain.StackDescendantsParentFirst) — closeout carries them;
//   - every extra path passed (ADR being written, artifact paths, the ADR
//     index — the caller knows its plan surface).
// Duplicate/collision findings need no enumeration: a duplicate or cycle
// finding NAMES the root id in Entity/Related, so findingRelevant catches it.
func changeScope(id int, recPath string, withDescendants bool, extra ...string) *transaction.ValidationScope

// adrScope is the same shape rooted at an ADR write: the ADR's own path, its
// structural supersede/reverse targets, and the ADR index path.
func adrScope(adrPath string, targetPaths []string, indexPath string) *transaction.ValidationScope
```

- [ ] **Step 1: Write failing tests:** snapshot fixtures with a root B (id 20), dependency 10, stack ancestor 5, descendant 30, unrelated A (id 99), and an associative `related: [99]` on B. Assert: A's path is NOT in the resolved set (ADR-0093 associative exclusion — the sharpest assert in this task); dependency/ancestor paths are; descendants only under `withDescendants`; the set resolves B by id when the state's snapshot holds B at its archive path while recPath still names active (identity, not path); `extra` paths pass through; an empty snapshot yields at least `{recPath}` ∪ extras (never empty — recPath is always known).
- [ ] **Step 2: Verify failure:** `go test ./internal/app/ -run TestChangeScope -count=1`.
- [ ] **Step 3: Implement** as a closure over the args returning `map[gitcli.RepoPath]bool` per state.
- [ ] **Step 4: Verify pass**, then mutation-test the ADR-0093 assert: make the resolver follow `Related()`/`related:` links and watch it redden. Restore.
- [ ] **Step 5: Commit:** `git commit -am "feat(app): production subject resolver for scoped metadata validation (change 0449)"`

### Task 5: Thread scope — claim, refresh-claim, reconcile, attach

**Files:**
- Modify: `internal/app/change_claim.go` (both `transaction.Request` literals, near `Loader: newPlanningLoader(eff)`), `internal/app/change_reconcile.go`, `internal/app/change_attach.go`
- Test: extend `internal/app/change_claim_test.go`, `change_reconcile_test.go`, `change_attach_test.go`

**Interfaces:** Consumes `changeScope` (Task 4). Pattern for every site in Tasks 5–7 — add one field to the existing Request literal:

```go
    Loader:    newPlanningLoader(eff),
    Scope:     changeScope(id, recPath, false),
    Operation: op,
```

(`withDescendants=false` everywhere except closeout; `extra` carries the paths that op's plan writes beyond the record itself when the caller already knows them — the post-plan union covers plan-declared paths like BOARD.md automatically, so extras are only for *validation-required* records, e.g. an ADR path the op reads for authority. For these three ops: none.)

- [ ] **Step 1: Write failing tests.** Use each op's existing test harness (fake engine or isolated git — follow the file's own idiom). The shared scenario pair, per operation:
  - *Progress:* corpus seeds an unparseable unrelated change A (raw bytes that fail `document.Parse`) plus healthy B ⇒ claim (and refresh), reconcile, attach each APPLY; A's bytes on the target ref are byte-identical after; the parse finding is still visible on a subsequent status read.
  - *Refusal:* (a) B itself carries the defect ⇒ refused; (b) B `depends_on` the unparseable A ⇒ refused (the dangling-ref error sits on B); (c) duplicate of B's id in another record ⇒ refused.
- [ ] **Step 2: Verify the progress tests FAIL** on the current strict gate (that failure *is* the reproduction of the bug) and the refusal tests PASS already: `go test ./internal/app/ -run 'TestChangeClaim.*Unrelated|TestChangeReconcile.*Unrelated|TestChangeAttach.*Unrelated' -count=1`.
- [ ] **Step 3: Add the `Scope:` field at each Request literal** (claim has two — claim and refresh-claim).
- [ ] **Step 4: Verify all new tests pass; package green:** `go test ./internal/app/ -count=1` (this package is partitioned behind build tags per change 0333 — if the bare command skips these tests, use the suite runner's package invocation from `tests/README.md`).
- [ ] **Step 5: Mutation-test once for the group:** remove the `Scope:` field from claim ⇒ its progress test reddens with the refusal shaped exactly like the old bug. Restore.
- [ ] **Step 6: Commit:** `git commit -am "feat(app): scoped validation for claim, reconcile, and attach (change 0449)"`

### Task 6: Thread scope — lifecycle, mark-implemented, halt, repair

**Files:**
- Modify: `internal/app/change_lifecycle.go` (`executeChangeLifecycle`'s Request), `change_implemented.go`, `change_halt.go` (both Request literals), `change_repair.go`
- Test: extend each op's test file

Same recipe as Task 5, same scenario pair per operation (progress with unparseable unrelated A; refusal with the defect on B). `withDescendants=false` for all four. For halt (two transactions) and repair, scope both Request literals.

- [ ] **Step 1: Write the failing progress tests + passing refusal tests** (per Task 5's scenario pair).
- [ ] **Step 2: Verify expected red/green:** `go test ./internal/app/ -run 'Unrelated' -count=1`.
- [ ] **Step 3: Add `Scope: changeScope(id, recPath, false)`** at each literal (use each function's in-scope id/path variables — in `executeChangeLifecycle` they are `id, recPath`).
- [ ] **Step 4: Package green** (`-count=1`), spot mutation-probe one op (drop lifecycle's scope ⇒ redden, restore).
- [ ] **Step 5: Commit:** `git commit -am "feat(app): scoped validation for lifecycle, implemented, halt, and repair (change 0449)"`

### Task 7: Thread scope — finalize block/clear-block, closeout (stacked + archive), required ADR writes

**Files:**
- Modify: `internal/app/finalize_block.go` (both Requests), `internal/app/finalize_closeout.go` (the stacked op near `closeoutStackedOp` and the archive op in `runCloseoutArchiveTransaction`), `internal/app/adr_ops.go` (both Requests)
- Test: extend `finalize_block_test.go`, the closeout tests (`finalize_closeout_*test.go` — locate by `runCloseoutArchiveTransaction` references), `adr_ops_test.go`

Specifics beyond the Task 5 recipe:

- **Closeout archive** is the multi-record path: `Scope: changeScope(int(cc.change.ID()), cc.change.Path(), true)` — descendants carried; ALSO union every `targets[i].activePath` and `targets[i].archivePath` via `extra` so a carried record is a subject even if stack traversal misses it in a degraded snapshot. The before-state obligation matters here: a required closeout descendant that is corrupt must refuse even though the candidate removes its active path (Task 1's before-state resolution rule carries this; add the integration assertion here).
- **Closeout stacked** (`closeoutStackedOp`): `changeScope(id, cc.change.Path(), true)`.
- **finalize block/clear-block:** `changeScope(id, recPath, false)`.
- **ADR ops** (`adr_ops.go`): `adrScope(adrPath, targetPaths, indexPath)` with the op's actual variables — the ADR being recorded/superseded, its structural supersede/reverse target paths, and the ADR index path the plan rewrites. The unrelated-invalid-A progress test here also proves the ADR-index render tolerates A (the index renders from the snapshot, where unparseable A simply isn't; the board task handles visibility).
- The **backlink legs keep `backlinkArtifactLoader`** untouched — assert nothing changed there (its own tests already pin the scoped-loader behavior).

- [ ] **Step 1: Failing progress tests** (unparseable unrelated A present): block, clear-block, stacked closeout, archive closeout, adr record — all apply; A's bytes unchanged. **Plus the two spec-named closeout refusals:** corrupt required descendant ⇒ refused; and *A's defect introduced between merge and closeout* still lets B's closeout apply (seed A's corruption into the metadata ref after the harness's merge step, before invoking closeout).
- [ ] **Step 2: Red/green check** (`-count=1`), **Step 3: add the Scope fields**, **Step 4: package green + mutation-probe the archive-closeout scope** (drop it ⇒ progress test reddens; restore).
- [ ] **Step 5: Commit:** `git commit -am "feat(app): scoped validation for finalize block, closeout, and ADR writes (change 0449)"`

### Task 8: Board renders usable records and a repair notice; unrenderable records surface instead of vanishing

**Files:**
- Modify: `internal/render/board.go`
- Modify: `internal/app/derived_views.go` (`includeBoard` signature) and every caller: `change_claim.go`, `change_attach.go`, `change_groom.go`, `change_create.go`, `change_implemented.go`, `change_kill.go`, `change_reclaim.go`, `change_lifecycle.go`, `change_reconcile.go`, `change_repair.go`, `finalize_block.go`, `finalize_closeout.go` (derive the full list by grep for `includeBoard(` — never hand-trust this enumeration)
- Test: `internal/render/board_test.go`, `internal/app/derived_views_test.go`; golden `internal/render/testdata/board/board.golden` must NOT change (healthy corpus renders byte-identically)

**Interfaces:**

```go
// render/board.go
// BoardUnrenderable names one record the board cannot render, by path.
type BoardUnrenderable struct {
    Path   string // repo-relative record path
    Reason string // one line: the finding code or classify error text
}
type BoardInput struct {
    Snapshot     domain.Snapshot
    Facts        domain.BranchFacts
    Presentation BoardPresentation
    // Unrenderable: records invisible to Snapshot (parse-failed) that the
    // caller wants surfaced in the repair notice.
    Unrenderable []BoardUnrenderable
}
```

Behavior: `Board()` no longer returns on a `boardClassify` or `boardReadinessCell`/`boardSectionRow` error for ONE record — it collects `BoardUnrenderable{Path: c.Path(), Reason: err.Error()}` and skips that row/count. Caller-supplied `Unrenderable` entries are merged (dedupe by path, sort by path). When the merged list is non-empty, emit after the terminal `<details>` block:

```markdown

## 🛠 Needs repair (N)

These records could not be rendered; counts above cover rendered records only.

| Record | Problem |
|---|---|
| `docs/changes/active/0099-….md` | frontmatter-parse-failed |
```

`Presentation.validate()` failure and the final `fmt` errors for *unknown section* remain hard errors (renderer/config faults stay real failures).

```go
// derived_views.go
func includeBoard(ctx context.Context, tree transaction.Tree, boardPath string,
    candidate domain.Snapshot, unrenderable []render.BoardUnrenderable,
    pres render.BoardPresentation, files *[]transaction.FileMutation) error

// boardUnrenderable derives the caller-supplied entries from a loaded state:
// every Sources path under the changes dir (active or archive) with no
// corresponding change in st.Snapshot (shape-keyed: absent-from-snapshot,
// never an enumerated finding-code list), Reason taken from the matching
// error finding's Code when one names that path, else "unreadable".
func boardUnrenderable(st transaction.LoadedState, changesDir string) []render.BoardUnrenderable
```

Every `includeBoard` caller passes `boardUnrenderable(st.State, <op>.changesDir)` (each op's Plan receives `st transaction.AttemptState`; the changes dir is already a field on each op struct or reachable from its `eff`).

- [ ] **Step 1: Failing render tests:** (a) snapshot whose active change has a non-active status ⇒ board renders, that record in the repair table, others intact (today: hard error — this is the flipped `boardClassify` abort); (b) `Unrenderable` input surfaces in the table sorted/deduped; (c) empty list ⇒ output byte-identical to today (golden untouched); (d) counts line excludes unrenderable records.
- [ ] **Step 2: Red check** (`go test ./internal/render/ -count=1`), **Step 3: implement board.go**, **Step 4: green + golden intact** (`git diff --exit-code internal/render/testdata/`).
- [ ] **Step 5: `boardUnrenderable` + signature threading:** failing test in `derived_views_test.go` (LoadedState with a parse-failed change source ⇒ one entry with the finding's code; healthy state ⇒ nil), then implement, then mechanically update every caller (grep-derived). The board-projection guard (`board_projection_isolation_guard_test.go`) and mutation-shape guard (`derived_views_guard_test.go`) must stay green — read their failure text before touching their subjects.
- [ ] **Step 6: End-to-end assertion (spec):** extend one Task-5 progress test (claim over unparseable A): the applied commit's BOARD.md contains B's row AND the repair-notice line naming A's path — atomic with the mutation.
- [ ] **Step 7: Mutation-probe:** silently drop unrenderable entries in `Board()` (render rows only) ⇒ tests (b)/(d) and Step 6 redden. Restore.
- [ ] **Step 8: Commit:** `git commit -am "feat(render,app): board renders usable records with a repair notice for unrenderable ones (change 0449)"`

### Task 9: Bound named live branch-fact probes to B's base/stack

**Files:**
- Modify: `internal/app/status.go` (add `stackBranchesFor` beside `stackBranches`)
- Modify the named callers (derive the real list by grep for `stackBranches(snap)` — spec: derive, don't trust this enumeration): `change_claim.go` (`resolveClaimTarget`), `workspace_ops.go`, `implementation_context.go` (named-id path only), `finalize_merge.go`, `finalize_block.go`, `finalize_retarget.go`, `change_repair.go`
- Test: each caller's test file + a unit test for `stackBranchesFor` in `status_test.go`

**Interfaces:**

```go
// stackBranchesFor bounds the live branch-fact probe to what named decisions
// about c actually consult: the recorded branches of c's stack ancestors, of c
// itself, and of the stack ancestors of c's direct depends_on targets (their
// readiness feeds EvaluateReadiness for c). Sorted, deduped.
func stackBranchesFor(snap domain.Snapshot, c domain.Change) []string
```

`implementation_context.go` fetches facts BEFORE selecting (`BranchFacts(ctx, pin, stackBranches(snap))` then `selectContextChange`): when the request carries an explicit id, resolve that change from the snapshot FIRST, probe `stackBranchesFor(snap, c)`, and keep the whole-corpus probe for the selection (no-id) path — verify against `selectContextChange`'s actual consumption of facts before wiring (learnings: verify-the-claim). Selection-path behavior must be byte-identical.

- [ ] **Step 1: Failing tests:** (a) unit: `stackBranchesFor` returns exactly the bounded set for a fixture with B (ancestor chain + dependency) and an unrelated stacked A; (b) per named caller: seed an unrelated change whose recorded branch makes the fake/live reader FAIL when probed (fake `BranchFacts` erroring on that name, or an invalid ref name in the git harness) ⇒ the named operation for B succeeds; (c) negative: a broken branch B actually requires (its own stack ancestor's) still refuses.
- [ ] **Step 2: Red check** (`-count=1`) — (b) fails today because the whole-corpus set includes A's branch.
- [ ] **Step 3: Implement + rewire the named call sites.**
- [ ] **Step 4: Green; mutation-probe:** revert one caller (claim) to `stackBranches(snap)` ⇒ its (b) test reddens. Restore.
- [ ] **Step 5: Commit:** `git commit -am "feat(app): named operations probe only their own base/stack branch facts (change 0449)"`

### Task 10: End-to-end named-workflow tests, the departure ADR, mutation sweep, suite gate

**Files:**
- Create: `internal/app/named_isolation_integration_test.go` (follow the harness idiom of `claim_workflow_git_test.go` / `change_integration_test.go` / `finalize_cleanup_integration_test.go` — isolated git repos, fake external/process seams, no live PRs)
- ADR: new record via the docket-adr workflow
- Results of the mutation sweep go into the build's evidence notes (docket-build owns the results file)

- [ ] **Step 1: Implementation-flow e2e test (spec test 2a):** one isolated repo; seed unparseable A + a mixed old runtime record + healthy named B; drive by explicit id through the production ops in workflow order: `context.implementation` (named) → claim → attach (plan) → mark-implemented, asserting after EVERY step: applied disposition, A's blob id unchanged on the metadata ref, B's board row + repair notice present. Include a named branch-fact read with A carrying an invalid branch name (Task 9's surface, now end-to-end).
- [ ] **Step 2: Finalize-flow e2e test (spec test 2b):** separately, named finalize path with the merge seam faked: block → clear-block → closeout archive, covering merge-already-landed recovery per the existing finalize harness's idiom. Introduce A's corruption BETWEEN the (faked) merge and closeout — closeout still applies. Then the paired refusal: corrupt B's own record before closeout ⇒ refused before any archive effect.
- [ ] **Step 3: Run both:** suite-runner invocation per `tests/README.md` (the internal/app partitions are behind build tags), `-count=1`.
- [ ] **Step 4: Mutation sweep (spec test 3 — run each, observe red, restore, `-count=1`):**
  1. Engine before-gate back to unconditional `before.Report.HasErrors()` refusal ⇒ Tasks 5–7 progress tests + e2e redden.
  2. After-gate back to unconditional ⇒ same reddening class.
  3. `Board()` back to abort-on-first-classify-error and drop of Unrenderable ⇒ Task 8 tests redden.
  4. One named caller back to whole-corpus `stackBranches` ⇒ Task 9 test reddens.
  5. Relevance check deleted (every finding "unrelated") ⇒ Task 3 scoped-refusal + Task 5 refusal tests redden.
  6. Equality weakened to code+count ⇒ Task 1/3 volatility tests redden.
  7. Empty scope treated as "no subjects to check" ⇒ Task 3 case (c) reddens.
  Record each probe's red assert name; any probe that stays green is a defect to fix before proceeding (learnings: assert-detects-removal-not-replacement, residual-is-for-undetectable-not-unprobed).
- [ ] **Step 5: Record the departure ADR** via the repo's ADR workflow (dispatch the registered `docket-adr` agent per repo convention; it owns numbering, the index render, and listing the new id in change 0449's `adrs:` — learnings: adr-update-delivery). Content: *Scoped metadata validation for named operations* — decision: named metadata operations validate a bounded in-memory subject set and grandfather exact unchanged unrelated pre-existing errors, departing from #0309's error-free-complete-corpus mutation contract; grounded in #0310/#0312, change 0337's scoped backlink loader as precedent, and the canonical generated-view design; consequences: strict unscoped operations and whole-repository health reporting remain, an empty scope fails closed, no persisted baseline exists. Do not rewrite #0309's Accepted text.
- [ ] **Step 6: Full suite gate:** `go run ./cmd/docket development test` — green, AND read the budget report; treat `SERIAL CONFIRMED OVER BUDGET:` as an action item and note any `BUDGET WATCH:` in the build evidence (the new integration tests are the likely watch candidates).
- [ ] **Step 7: Commit:** `git add -A && git commit -m "test(app): end-to-end named-workflow isolation coverage and mutation sweep (change 0449)"`

---

## Self-review (performed at authoring)

- Spec §1 (scoped transaction sequence steps 1–5) → Tasks 1–3 (bytes-for-parse-failed: Task 2; per-attempt freshness + replay: Task 3; findings-surface non-failure: Task 3 scoped-progress assert). §2 (workflow population) → Tasks 5–7. §3 (branch facts) → Task 9. §4 (generated views) → Task 8 (+ ADR-index consumer covered in Task 7's adr-ops progress test). Acceptance 1/2/3/4 → mapped above. Architecture-and-scope limits → Global Constraints + Task 10 Step 5.
- Known intermediate-state seams (learnings: intermediate-task-state-buildable): Task 3 lands `Scope` with zero producers — nil everywhere, suite green; Task 8's `includeBoard` signature change updates all callers within the same task; no cross-task dangling references.
- Type consistency: `ValidationScope`/`SubjectResolver`/`Blobs` (Task 1) ↔ engine (Task 3) ↔ `changeScope`/`adrScope` (Task 4) ↔ call sites (5–7); `BoardUnrenderable`/`boardUnrenderable` (Task 8) consistent throughout.
