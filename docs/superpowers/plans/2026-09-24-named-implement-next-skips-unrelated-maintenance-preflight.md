<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0448 — Named implement-next skips unrelated maintenance preflight](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0448-named-implement-next-skips-unrelated-maintenance-preflight.md)**
<!-- docket:backlink:end -->
# Named Implement-Next Skips Unrelated Maintenance Preflight Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. (In this repo the build role is `docket-build`, which fulfills the subagent-driven contract.)

**Goal:** An implement-next run invoked with exactly one explicit change id (initial named call, attributed retry, or named resume) goes from repository preparation directly to `context.implementation --id` and the existing claim/resume path — no maintenance preflight, no status sweep — so another change's failing closeout, cleanup, or reclaim can never halt an unrelated named change before its own claim/gate, while the named change's own merged-but-unclosed dependencies get a bounded closeout of only that dependency set.

**Architecture:** This is a **workflow-prose change with prose-contract guards** — zero Go behavior changes. The maintenance preflight is invoked by `skills/docket-implement-next/SKILL.md` Step 0 (agent-executed prose), not by any Go call site, so the fix is: (1) a named-invocation branch in Step 0 that bypasses the preflight and the Step-1 status-digest certification, (2) a bounded dependency-closeout paragraph that drives the **existing** `finalize.closeout` operation for only the named change's own `implemented` dependencies (the operation itself re-proves the merged PR — `pr-not-merged` refuses `blocked`), (3) a matching one-clause amendment to the convention's *Composition* paragraph, (4) new `proseContracts` rows in `internal/repoguard/prose_contracts_test.go` (sentinel `change_0448_named_preflight_skip`) that redden under the two spec-named mutations, and (5) regeneration of the embedded asset tree (`go run ./cmd/genassets -repo .`), which `TestEmbeddedMatchesAuthored` enforces. `maintenance.preflight`, `maintenance.sweep`, the no-ID selection path, and every Go operation stay byte-identical.

**Tech Stack:** Markdown skill bodies; Go test-only edits (`internal/repoguard`); `cmd/genassets` embedded-tree regeneration; suite via `internal/suiterunner` (docket-build's end gate).

**Spec:** `docs/superpowers/specs/2026-09-23-named-implement-next-skips-unrelated-maintenance-preflight-design.md` (synchronized metadata tree: `.docket/docs/superpowers/specs/...` on branch `docket`; read it alongside this plan).

## Global Constraints

- **No new commands, flags, preflight scopes, verdict policies, background cleanup, or deferred queues.** No Go source file outside `internal/repoguard/prose_contracts_test.go` changes; `internal/assets/embedded/tree/**` changes only by regeneration.
- **Keep the no-ID preflight, explicit `maintenance.preflight`, and `maintenance.sweep` unchanged** — the selection-path Step-0 paragraph keeps its 0397 guard phrases verbatim: `maintenance.preflight`, ``the envelope `result` and the Go-computed `preflight` verdict``, `as its own Bash call`, and the convention keeps ``runs the `maintenance.preflight` operation inline`` (all pinned by sentinel `change_0397_preflight_op`; breaking them is a task failure).
- **A named request never falls back to selecting another change.** Every local check for the named change is kept: readiness, entity version, dependencies, effective base, claim eligibility, gate context, resume/cancellation.
- **The bounded closeout touches only the named change's own dependency set** — its unmet `depends_on` ids at status `implemented` (plus `implemented` stack ancestors on a `not-ready-stack-base-unresolved` refusal), one `finalize.closeout --id` each, no cleanup suffix, no reclaim, no recursion.
- **Named finalize acquires no new maintenance prerequisite** — this plan touches no finalize skill prose.
- Every new guard is **mutation-tested**: strip the guarded prose, watch the row redden, restore (repo AGENTS.md rule; use `go test -count=1` so the cached runner never serves a stale green — learnings: cached-runner-serves-a-mutated-tree).
- Skill-body sentences are load-bearing: before rewording any existing sentence not named in this plan, grep `internal/repoguard/` and `tests/` for it (learnings: restatement-accumulates-its-own-guards). The only pre-existing dependents of the edited region are the `change_0397_preflight_op` rows, which this plan preserves.
- The BUILD gate runs the whole configured `build.test_command` from source through the Go suite runner (`internal/suiterunner`); read the budget report even on green.

## File Structure

- Modify: `skills/docket-implement-next/SKILL.md` — Step 0 named-invocation branch + bounded dependency closeout; Step 1 wording scoped to the selection path; the overview bullet and Scope paragraph redefine a single id as a named invocation (today it is "the degenerate case" of the allowlist).
- Modify: `skills/docket-convention/SKILL.md` — one clause in the *Composition* paragraph (the only other maintained named-entry caller; derived by whole-repo grep for `maintenance.preflight`, which finds no other executable prose surface — `skills/docket-status/SKILL.md` describes the operation itself and stays untouched).
- Modify: `internal/repoguard/prose_contracts_test.go` — new sentinel rows `change_0448_named_preflight_skip`.
- Regenerate: `internal/assets/embedded/tree/skills/docket-implement-next/SKILL.md`, `internal/assets/embedded/tree/skills/docket-convention/SKILL.md` (+ `internal/assets/embedded/manifest.json`) via `go run ./cmd/genassets -repo .` — never hand-edit.

## Acceptance-test mapping (spec §Acceptance tests)

No executable Go path changes, so the spec's behavioral tests are discharged by the prose contract plus existing suites: (1) named startup / no maintenance invocation → the named-branch guard rows (the preflight is only ever invoked by this prose); (2) local refusals → unchanged Go (`selectContextChange`, `ClaimEligibility`) plus the "never falls back" guard; (3) B's merged dependency → the bounded-closeout guard rows; (4) unchanged no-ID/maintenance behavior → untouched Go code, existing `internal/app` maintenance/preflight tests, and the preserved 0397/0389 sentinel rows; (5) mutation checks → explicit steps in Tasks 1–2; (6) build gate → docket-build's end-of-build full-suite run.

---

### Task 1: Named-invocation branch in `docket-implement-next` + guard rows + embedded regen

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (Overview bullet ~line 16; Step 0 ~line 29; Step 1 acquisition ~line 35; Step 1 Scope ~line 39)
- Modify: `internal/repoguard/prose_contracts_test.go` (append rows to the `proseContracts` table, before the closing `}`)
- Regenerate: `internal/assets/embedded/tree/**`, `internal/assets/embedded/manifest.json`

**Interfaces:**
- Produces: the sentinel name `change_0448_named_preflight_skip` and the exact guarded phrases below; Task 2 adds one more row under the same sentinel and must not duplicate these.

- [ ] **Step 1: Write the failing guard rows**

Append to the `proseContracts` table in `internal/repoguard/prose_contracts_test.go` (keep the table's comment style; these rows follow learnings prose-guard-binds-phrase-to-claim — each phrase is a full clause bound to its claim inside one sentence, and assert-detects-removal — the absent phrase is the retired unconditional opener, so restoring mandatory maintenance on a named request reddens):

```go
	// change 0448 — a single-explicit-id (named) invocation skips the
	// maintenance preflight and never falls back to selection; its own merged
	// dependencies get a BOUNDED finalize.closeout limited to that dependency
	// set. Present phrases bind each claim inside one sentence; the absent
	// phrase is the retired UNCONDITIONAL preflight opener, so restoring
	// mandatory maintenance on a named request reddens this row
	// (assert-detects-removal). Mutation-tested at introduction.
	{sentinel: "change_0448_named_preflight_skip", file: "skills/docket-implement-next/SKILL.md",
		present: []string{
			"**Named invocation — no maintenance preflight.**",
			"The named path never runs `maintenance.preflight` or any maintenance sweep first",
			"a named request **never falls back to selecting another change**",
			"For **each** id in that set — and never any other change — run one `finalize.closeout` operation",
			"An unrelated change's closeout is never attempted here",
		},
		absent: []string{
			"Then, before selection, run the **implementation preflight** inline",
			"a single id is the degenerate case",
			"a single id `90` is the degenerate case",
		}},
```

- [ ] **Step 2: Run the guard to verify it fails**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: FAIL — every `present` phrase reported missing, and both "degenerate case" `absent` phrases reported present-forbidden. (The retired-opener absent phrase also fails now; all clear together after Step 3.)

- [ ] **Step 3: Edit `skills/docket-implement-next/SKILL.md`**

Four edits. Copy replacement text exactly — the guard phrases above must match byte-for-byte.

**(a) Overview bullet** (in *When to use*). Replace:

> `- You want the backlog drained autonomously — pick the highest-priority build-ready change and ship it to a PR without human steering, or hand it an id set (`90,92,94`; a single id is the degenerate case) to scope the run to those changes.`

with:

> `- You want the backlog drained autonomously — pick the highest-priority build-ready change and ship it to a PR without human steering, or hand it an id set (`90,92,94`) to scope the run to those changes. A **single explicit id** is a **named invocation**: it skips the maintenance preflight and goes straight to that change's own checks (Step 0, *Named invocation*).`

**(b) Step 0.** The paragraph beginning `Then, before selection, run the **implementation preflight** inline:` is replaced by THREE paragraphs. The first two are new; the third is the existing paragraph with ONLY its opening clause reworded (everything from `resolve the` onward stays byte-identical — it carries the 0397 guard phrases).

New paragraph 1:

> `**Named invocation — no maintenance preflight.** When this run was invoked with exactly **one explicit change id** — an initial named call, an attributed retry carrying the id, or a named resume of an in-progress change — skip the implementation preflight and Step 1's status-digest certification entirely: after the `repository.prepare` operation succeeds, go **directly** to the authoritative `context.implementation` operation with `--id <id>` and the existing claim/resume path. The named path never runs `maintenance.preflight` or any maintenance sweep first — another change's failing closeout, cleanup, or reclaim can never halt this change before its own claim/gate; anyone who wants those maintenance outcomes runs the explicit `maintenance.preflight` or `maintenance.sweep` operation deliberately, where every problem stays visible. Every local check for the named change is kept — readiness, entity version, dependencies, effective base, claim eligibility, gate context, and resume/cancellation — so a malformed, absent, not-ready, or improperly resumed id returns its existing typed refusal, reported with its reason exactly as a scoped-id skip is (ending `drained` when nothing else is in scope); a named request **never falls back to selecting another change**, and shared repository/configuration/transport failures keep the `halted` posture. (Change 0448, narrowing ADR-0101/ADR-0106 for explicit ids.)`

New paragraph 2:

> `**The named change's own merged dependencies (bounded closeout).** Skipping the sweep never declares a dependency satisfied. When the named path's `context.implementation` read refuses `not-ready-waiting-dependency` (or `not-ready-stack-base-unresolved` for a stacked change), read the named change's own unmet set — run the `status` operation with `--json` (a write-free read) and take, from `changes[]`, the named change's `depends_on` ids whose `status` is `implemented`, plus, for the stacked refusal only, its stack ancestors whose `status` is `implemented`. For **each** id in that set — and never any other change — run one `finalize.closeout` operation with `--id <that id>` (resolve argv from the capability catalog): the operation itself re-proves the merged PR and applies the one verified terminal shape, so run no cleanup suffix and no reclaim, and never widen or recurse beyond this set. Map each disposition locally: `applied`/`no-op` — after the whole set, re-run the `repository.prepare` operation and re-run `context.implementation --id` once; `blocked` with reason `pr-not-merged` — the dependency is genuinely unmerged, so surface the ordinary waiting-dependency refusal naming that id (remedy: merge its PR, then `docket-finalize-change` or the explicit `finalize.closeout` operation); any other non-applied outcome (`contended`, `failed`, an unknown/external-failed probe, or another `blocked` reason) is a failure of the named change's **own** dependency — halt before claiming through the pre-claim run-reporting path, naming that dependency id and the same remedy. An unrelated change's closeout is never attempted here, and a dependency unmet for any non-`implemented` reason keeps its ordinary local refusal unchanged.`

Reworded opening of the existing paragraph — replace exactly:

> `Then, before selection, run the **implementation preflight** inline: resolve the`

with:

> `**Selection path only.** Otherwise — no argument, or an id set — run, before selection, the **implementation preflight** inline: resolve the`

(The remainder of that paragraph, through `…Step 1 takes its own fresh read.`, is untouched.)

**(c) Step 1 acquisition.** Replace:

> `Run the `status` operation with `--json` — a write-free read, and only AFTER Step 0's `maintenance.preflight` run and metadata re-sync (the snapshot is taken pre-sweep otherwise and would list already-merged changes).`

with:

> `Run the `status` operation with `--json` — a write-free read; on the selection path, only AFTER Step 0's `maintenance.preflight` run and metadata re-sync (the snapshot is taken pre-sweep otherwise and would list already-merged changes). A named invocation takes no ready-queue digest — it arrives here already holding Step 0's directive to read the explicit-id bundle below.`

**(d) Step 1 Scope paragraph.** Replace:

> `A caller may pass an **id allowlist** — `docket-implement-next 90,92,94` (a single id `90` is the degenerate case) — and selection is then **restricted to that set**, preserving the queue's order *within* it.`

with:

> `A caller may pass an **id allowlist** of two or more ids — `docket-implement-next 90,92,94` — and selection is then **restricted to that set**, preserving the queue's order *within* it. (A **single explicit id** is not an allowlist: it is a named invocation and takes Step 0's named branch, never this filter.)`

- [ ] **Step 4: Run the guard to verify it passes**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: PASS (both the new `change_0448_named_preflight_skip` rows' phrases and the preserved `change_0397_preflight_op` rows).

- [ ] **Step 5: Regenerate the embedded tree and verify**

Run: `go run ./cmd/genassets -repo .`
Then: `go test ./internal/assets/ -count=1`
Expected: PASS (`TestEmbeddedMatchesAuthored` proves the embedded copies now match the authored skills). `git status` shows only `skills/docket-implement-next/SKILL.md`, `internal/repoguard/prose_contracts_test.go`, and regenerated files under `internal/assets/embedded/`.

- [ ] **Step 6: Mutation-test the guard (spec acceptance test 5, first mutation)**

1. In `skills/docket-implement-next/SKILL.md`, temporarily delete the whole `**Named invocation — no maintenance preflight.**` paragraph and restore the selection-path opener to `Then, before selection, run the **implementation preflight** inline: resolve the` (this IS "restore mandatory maintenance on a named request").
2. Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1` — Expected: FAIL (missing present phrases AND the forbidden retired opener present).
3. Second mutation — "let the bounded dependency closeout touch a non-dependency": restore the file, then delete only the two clauses `For **each** id in that set — and never any other change — run one `finalize.closeout` operation` and `An unrelated change's closeout is never attempted here` (reword them to unbounded, e.g. "run finalize.closeout for every implemented change"). Run the same test — Expected: FAIL.
4. Restore the intended text exactly (re-apply Step 3's replacements — never `git checkout --` over uncommitted intended edits; learnings: mutation-restore-needs-a-backup-copy). Re-run the test — Expected: PASS. Re-run `go run ./cmd/genassets -repo .` if any mutation was regenerated, and confirm `go test ./internal/assets/ -count=1` passes.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-implement-next/SKILL.md internal/repoguard/prose_contracts_test.go internal/assets/embedded/
git commit -m "feat(skills): named implement-next invocation skips the maintenance preflight, with bounded own-dependency closeout (change 0448)"
```

---

### Task 2: Convention *Composition* amendment + guard row + embedded regen

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the `**Composition (change 0017; step 0 amended by change 0397).**` paragraph)
- Modify: `internal/repoguard/prose_contracts_test.go` (one more row under sentinel `change_0448_named_preflight_skip`, appended after Task 1's row)
- Regenerate: `internal/assets/embedded/tree/**`, `internal/assets/embedded/manifest.json`

**Interfaces:**
- Consumes: Task 1's sentinel name `change_0448_named_preflight_skip` (same sentinel, new file row — the table allows several rows per sentinel).

- [ ] **Step 1: Write the failing guard row**

Append to the `proseContracts` table, directly after Task 1's row:

```go
	// change 0448 — the convention's Composition paragraph carries the same
	// exemption: the step-0 preflight is selection-path only, and a single
	// explicit id goes directly to the authoritative explicit-id read. The
	// phrase is one bound clause, not floating vocabulary; the 0397 row above
	// still pins the surviving inline-operation sentence.
	{sentinel: "change_0448_named_preflight_skip", file: "skills/docket-convention/SKILL.md",
		present: []string{
			"an invocation naming exactly one explicit change id skips the preflight and goes directly to `context.implementation --id`",
		}},
```

- [ ] **Step 2: Run the guard to verify it fails**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: FAIL with exactly the one new missing-required phrase in `skills/docket-convention/SKILL.md`.

- [ ] **Step 3: Edit `skills/docket-convention/SKILL.md`**

In the *Composition* paragraph, replace exactly:

> `one protocol-v1 envelope keyed on its `preflight` verdict (change 0397, amending change 0017 for step 0 only; decision recorded in the change-0397 ADR).`

with:

> `one protocol-v1 envelope keyed on its `preflight` verdict (change 0397, amending change 0017 for step 0 only; decision recorded in the change-0397 ADR) — and it is **selection-path only**: an invocation naming exactly one explicit change id skips the preflight and goes directly to `context.implementation --id` and the claim/resume path, with a bounded closeout for only that change's own merged-but-unclosed dependencies (change 0448, narrowing ADR-0101/ADR-0106 for explicit ids).`

Touch nothing else in the paragraph — ``runs the `maintenance.preflight` operation inline`` (guard `change_0397_preflight_op`) appears earlier in the same sentence chain and must survive byte-identical.

- [ ] **Step 4: Run the guards to verify they pass**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: PASS.

- [ ] **Step 5: Regenerate the embedded tree and verify**

Run: `go run ./cmd/genassets -repo .`
Then: `go test ./internal/assets/ -count=1`
Expected: PASS.

- [ ] **Step 6: Mutation-test the guard**

1. Temporarily reword the inserted clause to sever the claim (e.g. drop `skips the preflight and` so the words "explicit change id" survive but the exemption is gone).
2. Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1` — Expected: FAIL.
3. Restore the exact Step-3 text; re-run — Expected: PASS. Re-run `go run ./cmd/genassets -repo .` and `go test ./internal/assets/ -count=1` — Expected: PASS.

- [ ] **Step 7: Regression check on the untouched surfaces (spec acceptance test 4)**

Run: `go test ./internal/app/ -run 'Preflight|MaintenanceSweep' -count=1` and `go test ./internal/repoguard/ -count=1`
Expected: PASS — proves the no-ID preflight, explicit `maintenance.preflight`, and `maintenance.sweep` behavior (all untouched Go) and every pre-existing prose sentinel still hold.

- [ ] **Step 8: Commit**

```bash
git add skills/docket-convention/SKILL.md internal/repoguard/prose_contracts_test.go internal/assets/embedded/
git commit -m "docs(convention): step-0 preflight is selection-path only; named invocations exempt (change 0448)"
```

---

## Coordinator follow-through (NOT build tasks — no worker executes these)

- **ADR (implement-next Step 6, `docket-adr` dispatch).** Record one ADR narrowing ADR-0101 and ADR-0106 (Accepted text is never rewritten in place — the new ADR narrows them; list its number in the change's `adrs:`). Decision content: (1) the mandatory implementation-startup preflight of ADR-0101/ADR-0106 now binds only selection-path invocations (no argument, or an id set of two or more); an invocation naming exactly one explicit change id — initial call, attributed retry, named resume — goes directly to `context.implementation --id` and the claim/resume path, because an unrelated change's failing closeout/cleanup/reclaim must not halt a named change before its own claim/gate, while deliberate `maintenance.preflight` / `maintenance.sweep` keep every honest outcome. (2) **§3 choice — preferred handling implemented via existing operations:** the named path runs one bounded `finalize.closeout --id` per own-dependency at status `implemented` (plus `implemented` stack ancestors on a stack-base-unresolved refusal); `finalize.closeout` already re-proves the merged PR, so `pr-not-merged` surfaces as the genuine waiting-dependency refusal, and any other non-applied disposition halts locally naming that dependency — no new operation, flag, or queue was needed, and the spec's fallback (pure refusal with a printed remedy) survives only as the disposition mapping's refusal arms. (3) Boundary: a multi-id allowlist remains selection behavior (preflight included) — the exemption is exactly the single-explicit-id request.
- **Suite gate (docket-build end gate).** One full-suite run via the resolved `build.test_command` through `internal/suiterunner`; read the budget report even on green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines are findings).

## Self-review notes

- Spec §1 (named start straight to B) → Task 1 (b)/(c)/(d); §2 (deliberate maintenance unchanged) → Global Constraints + Task 2 Step 7 (no Go edits; 0397/0389 sentinels preserved); §3 (own merged dependencies, chosen behavior + ADR note) → Task 1 paragraph 2 + Coordinator ADR item; §Architecture (ADR narrowing) → Coordinator item. Acceptance tests 1–6 → the mapping section above.
- Named-entry callers derived by search (reconcile log + this plan's grep): `skills/docket-implement-next/SKILL.md`, `skills/docket-convention/SKILL.md` Composition, their embedded copies (regenerated, never hand-edited), and the repoguard prose rows — all covered; no other executable prose surface invokes the preflight.
- Type/name consistency: one sentinel `change_0448_named_preflight_skip` across both tasks; all guarded phrases appear verbatim in the replacement prose they pin.
