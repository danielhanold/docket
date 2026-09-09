<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0154 — Remove stale Bash instructions and duplicated runtime contracts from Docket skills](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-09-0154-audit-skill-bodies-for-the-stale-restatement-class-change-01.md)**
<!-- docket:backlink:end -->
# Remove Stale Bash Instructions and Duplicated Runtime Contracts from Docket Skills — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One documentation PR that audits every tracked Markdown file under `skills/`, removes or corrects stale runtime instructions (deleted Bash owners, the retired line-oriented status report, the retired GitHub mirror, retired main-mode/config behavior) and duplicated runtime contracts, replaces them with usable references to current Go owners, reconciles every dependent guard without weakening surviving invariants, regenerates the embedded assets bundle, and records a complete reviewable inventory.

**Architecture:** Audit-and-edit over prose, driven by the spec's ownership rule (delete-and-point → compress-to-judgment-plus-pointer → keep-and-prove-correspondence). Every deletion is preceded by a whole-test-surface search for the prose being removed; dependent asserts are relocated or retired with recorded premises, never re-greened by restoring the copy. Runtime truth is verified against named Go owners (`internal/app/status_result.go`, `status_human.go`, `planning.go`, `derived_views.go`, `internal/config/schema.go`, `capability.go`, `.docket.example.yml`) and the capability/schema discovery channels — never against sibling prose.

**Tech Stack:** Markdown under `skills/`; Go guard tests in `internal/repoguard/` (`prose_contracts_test.go`, `capability_surface_test.go`, `config_read_channel_test.go`, absence/asset guards); embedded bundle via `go generate ./internal/assets` (`cmd/genassets`); full gate via `go run ./cmd/docket development test` (`internal/suiterunner`).

**Spec:** `docs/superpowers/specs/2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary checkout). The spec's "Ownership and reference rule", "Inventory and implementation evidence", and "Guard and validation strategy" sections bind every task below.

## Global Constraints

- **Population is discovered, never hard-coded:** `git ls-files 'skills/**/*.md' 'skills/*.md'` in this checkout is the in-scope set (29 files at baseline `d7363492`; recheck). Every file gets a recorded disposition — hits AND "no hit".
- **Docs-only intent:** runtime, CLI/schema/config behavior, defaults, permissions, and feature support are unchanged. Outside-`skills/` edits are limited to dependent Go tests, small corrections to a reference chosen as an owner, and regenerated `internal/assets` output — and each must name the in-scope edit that requires it.
- **Never modernize history:** historical specs, archived changes, merged plans/results, Accepted ADR bodies, and frozen fixture corpora are evidence, not targets.
- **Ownership rule order:** (1) delete and point at the current owner; (2) compress to caller judgment + pointer, keeping prerequisites, authority checks, failure posture, and follow-up at the decision site; (3) keep-and-prove only with a named, current, non-vacuous Go guard — no exemption by deleted Bash test name. The 0170 wrapper-count exemption is withdrawn.
- **Reference channels for consuming repos:** argv/effects → capability catalog (`docket capabilities`); payload shapes/vocabularies → the schema operation; live config values/classifications → configuration/context operations. A repo-local `docs/` or `internal/` path is never the sole operational reference for a consuming repo. Verify every proposed owner before pointing at it.
- **Guard discipline (binds every task):** before removing prose, search the entire test surface for it, including a whitespace-collapsed match for wrapped phrases; mutation-test every new/changed guard (strip the guarded state, watch it redden) and confirm each mutation landed with `/usr/bin/grep -cF` before/after counts taken through a whitespace-flattened copy; defeat Go's test cache with `-count=1`; PATH `grep` is ugrep — use `/usr/bin/grep` for portability-sensitive counts. Prefer absence asserts scoped to where the state lives, each with a non-vacuity companion through the same extractor.
- **Preserve caller variance:** diff restatements against each other before consolidating; a maintenance pass's log-and-continue and finalize's abort-and-report must not be flattened into one rule.
- **Build gate:** the full configured suite via `go run ./cmd/docket development test` (resolved from `build.test_command`), run from this feature checkout. Read the budget report even on green.
- **No new generic prose-lint framework** and no new closed-token ban.

## File Structure

Files this plan touches (population is re-discovered in Task 1; this is the expected shape):

- Modify: `skills/docket-status/SKILL.md` (seeds 1–3: report grammar, sweep posture, deleted script owners, board/mirror claims)
- Modify: `skills/docket-convention/SKILL.md` (seeds 3–5: mirror section, config-layers copy, `board_surfaces`/sketch, derived-view script family, main-mode advice, nine/eight counts)
- Retire or rewrite: `skills/docket-convention/github-board-mirror.md`
- Modify as hits dictate: `skills/docket-convention/references/terminal-close-out.md` (main-mode degradation section) and any other population file with a verified hit
- Modify: `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/config_read_channel_test.go`, `internal/repoguard/capability_surface_test.go`, and any other test file the prose-search in each task surfaces
- Regenerate: `internal/assets/embedded/tree/**` (generated — never hand-edit)
- Create: `docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md` (the audit inventory, committed on this branch; feeds the results file)

---

### Task 1: Baseline, population discovery, and mechanical seed scan

**Files:**
- Create: `docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md`

**Interfaces:**
- Produces: the inventory file every later task appends dispositions to. Columns per file: `path | disposition (hit/no-hit/pending) | hits (section/quoted clause) | owner verified against | notes`. Tasks 2–7 update rows they resolve; Task 9 verifies no `pending` remains.

- [ ] **Step 1: Record the baseline and discover the population**

```bash
cd "$(git rev-parse --show-toplevel)"
git rev-parse HEAD                      # record in the inventory header
git ls-files 'skills/**/*.md' 'skills/*.md' | sort   # the in-scope set — record every path
```

- [ ] **Step 2: Run the mechanical seed scans and record raw hits**

Derive token populations from their owners, not a hand-kept registry:

```bash
# Removed script-tree references (the retired Bash owners):
/usr/bin/grep -rn -E '(docket-status|board-checks|board-refresh|render-board|github-mirror|render-change-links|render-artifact-backlink)\.(md|sh)' skills/ || true
# Copied count words and enumerations:
/usr/bin/grep -rn -E '\b(nine|eight|seven|six|five)\b' skills/ || true
# Old report grammar tokens:
/usr/bin/grep -rn -E 'pass ok|harvest <id>|backlog <status>|swept <id>|issue-minted|project-minted' skills/ || true
# Retired surfaces:
/usr/bin/grep -rn -E 'main-mode|main mode|github-mirror|Projects v2|write-back' skills/ || true
# Operation names / vocabularies to cross-check against the catalog and schema:
docket capabilities | awk '{print $1}' | sort > /tmp/cap-ops.txt   # reference population for later verification
```

Also perform whitespace-collapsed variants of the phrase scans for prose that wraps (pipe each file through `tr -s '[:space:]' ' '` before matching).

- [ ] **Step 3: Write the inventory skeleton**

One row per discovered file, `disposition: pending`, raw mechanical hits attached. Header records baseline SHA, discovery command, and file count. Note explicitly that mechanical scans are seeds — every file still gets a full manual read (Task 7).

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md
git commit -m "docs(0154): audit inventory skeleton — baseline, population, mechanical seeds"
```

### Task 2: Status skill — report contract and sweep posture (spec seeds 1–2)

**Files:**
- Modify: `skills/docket-status/SKILL.md`
- Modify: dependent tests the Step-1 search surfaces (expect `internal/repoguard/prose_contracts_test.go` sentinels anchored on this file, e.g. `change_0389_sweep_scope`, `change_0397_preflight_op`)
- Modify: `docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md` (record dispositions)

**Interfaces:**
- Consumes: inventory rows from Task 1.
- Produces: a status skill whose report instructions are grounded in `StatusResult` (`internal/app/status_result.go`) / `StatusResult.HumanText` (`internal/app/status_human.go`) and the sweep operation's typed dispositions; no reference to `scripts/docket-status.md`, `scripts/board-checks.md`, `scripts/board-refresh.md`, `board-refresh.sh`, `render-board.sh`.

- [ ] **Step 1: Search the test surface for every clause you will touch**

For each candidate deletion (the `backlog <status>`/`change <id> …`/`ready [<id> …]` line grammar at lines 12 and ~80, `pass ok`, the `harvest <id> <path>` bullet, the "Sweep posture" paragraph at ~line 123, every `scripts/*.md` pointer):

```bash
PHRASE='pass ok'   # repeat per clause
/usr/bin/grep -rnF "$PHRASE" internal/ tests/ | /usr/bin/grep -v 'internal/assets/embedded' || true
# and whitespace-collapsed for wrapped clauses:
for f in internal/repoguard/*_test.go tests/*.sh; do tr -s '[:space:]' ' ' < "$f" | /usr/bin/grep -cF "$PHRASE" >/dev/null && echo "$f"; done
```

Record every dependent in the inventory before editing. (The embedded tree under `internal/assets/embedded/` is generated — it will be regenerated in Task 8, never hand-edited.)

- [ ] **Step 2: Verify the current owners before writing replacements**

Read `internal/app/status_result.go` (`StatusResult` and its report/problem entry types) and `internal/app/status_human.go` (`HumanText`). Run the real thing — `docket status --json` against this repo and `docket schema` for the status/sweep result shapes — and confirm what the payload actually carries (digest, per-change entries, health-check findings, sweep envelope, problem entries). Do not encode any field the running code does not emit.

- [ ] **Step 3: Rewrite the report-contract prose (seed 1)**

In `skills/docket-status/SKILL.md`: replace the old line-grammar teaching ("Overview" line 12, "Read the report" block ~lines 79–93, "Judgment follow-ups", "Final summary") with instructions to (a) request the structured report (`--json`, shapes discoverable via the schema operation), (b) validate/interpret the actual payload, and (c) summarize for the human from it. Keep only tokens an explicit caller decision branches on (e.g. the board-off configuration case, disposition tokens the skill keys on). Delete the `scripts/docket-status.md` / `scripts/board-checks.md` pointers — those files do not exist; the closed check-id set and report vocabulary are owned by the schema operation and the checker itself. Do NOT "repair" the historical `health checks failed <exit>` line from absorbed 0159 — the report model replacement covers it.

- [ ] **Step 4: Rewrite the sweep-posture prose (seed 2)**

Replace the Bash-era stage-by-stage retry narrative (~line 123: `sync pull-failed`, `archive script-error`, `commit-failed`/`push-failed` step 6a, the `swept`/`harvest` cross-check) with the surviving obligations, stated against the current operation: distinguish the read (`status`) from explicit maintenance (`maintenance.sweep`); observe a sweep's terminal envelope; inspect individual problem entries even when the overall result is applied; preserve unknown state; surface findings without unsolicited repairs; keep sweep recovery distinct from finalize's merge authorization. Derive any retry advice from the current operation's reason vocabulary (schema-owned), never the former line cross-check. Preserve the deliberate posture divergence sentence (sweep log-and-continue vs finalize abort-and-report) — that is caller variance, not duplication.

- [ ] **Step 5: Reconcile dependent guards**

For each dependent found in Step 1: if it guards a surviving property, relocate its anchor to the rewritten clause or the owning artifact; if its only subject is the retired copy, delete it and record the premise in the inventory. Never restore deleted prose to green a grep. Keep still-valid sentinels (e.g. the 0389/0397 sentinels' surviving claims) anchored on the new wording, bound phrase-to-claim with a bounded gap, not whole-file presence.

- [ ] **Step 6: Mutation-test the reconciled guards and run the package**

```bash
go test ./internal/repoguard/ -run TestProseContracts -count=1
# For each changed sentinel: mutate the guarded clause back toward the retired state,
# confirm the mutation landed (/usr/bin/grep -cF before/after on a whitespace-flattened copy),
# confirm the test reddens, restore from your edited copy (keep a backup — git checkout restores HEAD, not your edit).
```

- [ ] **Step 7: Commit**

```bash
git add skills/docket-status/SKILL.md internal/repoguard/ docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md
git commit -m "docs(0154): status skill — structured report contract, current sweep posture"
```

### Task 3: Board and GitHub-mirror claims (spec seed 3)

**Files:**
- Modify: `skills/docket-status/SKILL.md` (board bullets ~lines 87, 107–115, the `issue-minted`/`project-minted` write-back bullet with its `docket:config-read-channel: write-back` marker)
- Modify: `skills/docket-convention/SKILL.md` (~line 288 "Board refresh on status writes" stays the owner; ~line 369 mirror section, ~line 371 "Derived-view script family")
- Retire or rewrite: `skills/docket-convention/github-board-mirror.md`
- Modify: `internal/repoguard/config_read_channel_test.go` and any other dependents the prose search surfaces
- Modify: inventory

**Interfaces:**
- Consumes: Task 2's rewritten status skill.
- Produces: skills that state the current property — disabling rendering does not authorize deleting a pre-existing board; status stays read-only; no active mirror/write-back recipe anywhere under `skills/`.

- [ ] **Step 1: Verify the current owners**

Read `internal/app/planning.go` (`fenceBoardSurface`) and `internal/app/derived_views.go` (`includeBoard`) for the board-inclusion gate, and `internal/config/capability.go` for the classification of a repository `github` board-surface request (unsupported, mutation-blocking). Confirm with `docket schema`/`docket capabilities` output where the classification vocabulary is discoverable.

- [ ] **Step 2: Search the test surface for every clause to be removed**

Same procedure as Task 2 Step 1, for: "must not exist", "`board-refresh.sh` is its only writer", the whole mirror paragraph of `skills/docket-status/SKILL.md:115`, the `config-read-channel: write-back` marker, and the body of `github-board-mirror.md`. Expect `internal/repoguard/config_read_channel_test.go` to key on the marker — read that guard's population/floor logic before touching the marker so removal doesn't silently shrink a guarded population below its floor.

- [ ] **Step 3: Fix the board-absence contradiction**

In `skills/docket-status/SKILL.md`: replace "With the board off it must not exist" (lines 79 and 87) with the current property from the convention's own "Board refresh on status writes": a repo with `board_surfaces: []` renders and commits nothing and a pre-existing `BOARD.md` is left untouched; disabled rendering never authorizes deleting a board; the structured report remains the summary source and the skill never reads or writes `BOARD.md`. Delete the `board-refresh.sh`/`render-board.sh` writer narrative (lines 107–113) — the convention's board-refresh paragraph is the owner; leave a pointer to it.

- [ ] **Step 4: Remove active mirror/write-back instructions**

Delete the mirror bullet at `skills/docket-status/SKILL.md:115` (including its `issue-minted`/`project-minted` write-back and the `write-back` marker) and rewrite `skills/docket-convention/SKILL.md:369` to one concise supported/deferred/obsolete sentence grounded in the resolver: the `github` board surface is classified unsupported and mutation-blocking by `internal/config/capability.go`; historical `issue:` frontmatter data is preserved, never acted on. Then either delete `skills/docket-convention/github-board-mirror.md` (preferred — its content is an active recipe for a retired feature) or reduce it to the same compatibility statement; if deleted, fix the convention's link and verify no maintained incoming links remain:

```bash
/usr/bin/grep -rn 'github-board-mirror' skills/ internal/ tests/ docs/ --exclude-dir=embedded | /usr/bin/grep -v 'docs/superpowers\|docs/changes\|docs/adrs' || true
```

Do not restore the feature; do not remove historical `issue:` data or fixtures.

- [ ] **Step 5: Reconcile guards, mutation-test, run**

Update `config_read_channel_test.go` for the removed marker per its own floor rules (assert the marker population you now expect; keep the guard's non-vacuity). Reconcile any mirror-phrase sentinels by the Task 2 Step 5 rule. Consider (and prefer, per the spec) one narrow negative guard: no file under `skills/` teaches an active mirror write-back — bound to subject + obligation (e.g. `issue-minted|project-minted` as instructions to record back), mutation-tested by planting the retired recipe and watching it redden, with a non-vacuity companion.

```bash
go test ./internal/repoguard/ -count=1
```

- [ ] **Step 6: Commit**

```bash
git add skills/ internal/repoguard/ docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md
git commit -m "docs(0154): board/mirror claims — current gate semantics, mirror recipe retired"
```

### Task 4: Convention configuration copies (spec seed 4, incl. the 0257 item)

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the `.docket.yml` sketch ~lines 25–45, "Config layers" ~line 57, `board_surfaces` paragraph ~line 65)
- Modify: dependent tests per search
- Modify: inventory

**Interfaces:**
- Consumes: nothing beyond Task 1.
- Produces: convention config prose that keeps ownership concepts and the supported board model, with copied key inventories/defaults removed where a current owner is readable.

- [ ] **Step 1: Verify against the four named owners**

Read `internal/config/schema.go` (key set, per-key classification), `internal/config/capability.go`, the shipped `.docket.example.yml`, and the resolved `diagnostic.config` result (`docket diagnostic config --json` or as the catalog spells it — resolve argv from `docket capabilities`). List, in the inventory, each config claim in the convention with its verdict: true/false/owner-readable.

- [ ] **Step 2: Remove the copied coordination-key inventory**

In "Config layers" (~line 57): keep the layer model, per-field precedence concept, and the coordination-key *fence* concept with its posture (warned-and-ignored, never fatal, ADR-0019) — that is judgment the caller needs. Delete the copied parenthetical key list (`metadata_branch`, `integration_branch`, …) — the per-key classification is authoritative in `internal/config` and discoverable via `diagnostic.config`/schema; the prose already says so, so let the pointer stand alone. Search the test surface for the enumerated keys in this sentence first (Task 2 Step 1 procedure) and relocate dependents.

- [ ] **Step 3: Audit the configuration sketch and `board_surfaces` paragraph**

For every line of the `.docket.yml` sketch: verify the key, default, and comment against `internal/config/schema.go` and `.docket.example.yml`. Correct or delete anything retired (the spec names the sketch's `board_surfaces` comment describing `github` as a renderable surface, and the stale "superpowers default shown" comment that deferred 0257 names — fix or remove that comment and record the disposition explicitly as the one absorbed 0257 item). Keep a minimal example only where it matches the current supported configuration; do not describe every parseable key as an active feature. In the `board_surfaces` paragraph (~line 65), align with `capability.go`: `github` is not a supported render surface; remove the mint-and-write-back clause.

- [ ] **Step 4: Reconcile guards, mutation-test, run, commit**

Same guard procedure as prior tasks, then:

```bash
go test ./internal/repoguard/ -count=1
git add skills/docket-convention/SKILL.md internal/repoguard/ docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md
git commit -m "docs(0154): convention config prose — owners pointed at, sketch verified, 0257 item resolved"
```

### Task 5: Convention legacy explanations and counts (spec seed 5)

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (~line 113 nine/eight wrapper counts; ~line 371 "Derived-view script family"; ~line 385 main-mode clauses; any other hits)
- Modify: `skills/docket-convention/references/terminal-close-out.md` (the `## main-mode degradation` section)
- Modify: dependent tests per search
- Modify: inventory

**Interfaces:**
- Consumes: Task 1's count-word and retired-surface scans.
- Produces: convention prose with ADR-0099's single metadata topology, no advertised main-mode, no Bash script family, and no redundant cardinalities.

- [ ] **Step 1: Verify current reality**

ADR-0099 (one metadata topology) and 0363 (main-mode removed) govern: confirm in the current tree that no main-mode path survives (`/usr/bin/grep -rn 'main-mode\|DOCKET_MODE' internal/ | head`), and that the derived-view renderers named at line 371 (`board-refresh.sh`, `render-board.sh`, `github-mirror.sh`, `render-change-links.sh`, `render-artifact-backlink.sh`) do not exist (`ls scripts/`). Identify the current Go owners: board rendering inside the metadata transaction (`derived_views.go`), `artifact.backlink` for backlink blocks, the app-layer link-block renderer.

- [ ] **Step 2: Rewrite the Derived-view family and main-mode passages**

Replace the "Derived-view script family" paragraph with the current statement: derived views (board, `## Artifacts` blocks, `docket:backlink` blocks) are rendered by the Go app inside the owning metadata transaction / via the `artifact.backlink` operation; each generated block still has a sole writer and is never hand-edited (the ADR-0012 boundary as evolved). Remove "surviving for frozen / main-mode paths". At ~line 385, drop "(migrated or main-mode)" and "The guard is a no-op in `main`-mode" — the resolver has no such mode. In `references/terminal-close-out.md`, delete the `## main-mode degradation` section and its TOC entry; per the extraction rule leave no dangling anchors — verify incoming references:

```bash
/usr/bin/grep -rn 'main-mode' skills/ | /usr/bin/grep -v 'docs/' || true
```

- [ ] **Step 3: Remove redundant cardinalities (nine/eight, and analogues)**

At line 113, the "nine exceptions … Those eight" passage: prefer removing the counts entirely, keeping the rule (which wrappers omit the convention injection and why — the no-metadata-operations boundary each worker contract's Scope states). If an enumeration must survive, it must cite a current non-vacuous Go guard over that population (name it in the inventory); the 0170 exemption is withdrawn. Sweep the Task 1 count-word scan's other hits ("Lifecycle — eight states", "all eight states" in the mirror text Task 3 already handles, "Seven skills", etc.) with the same rule: a count that merely restates an adjacent list is deleted; a count a guard genuinely floors stays with its guard named. Check each count's test dependents first — several suite guards key on cardinalities.

- [ ] **Step 4: Reconcile guards, mutation-test, run, commit**

```bash
go test ./internal/repoguard/ -count=1
git add skills/ internal/repoguard/ docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md
git commit -m "docs(0154): convention — one topology, script family retired, counts owned or removed"
```

### Task 6: Learnings/harvest wording and remaining cross-file seed echoes

**Files:**
- Modify: files the Task 1 scans hit outside Tasks 2–5's scope (baseline expectation: `skills/docket-status/SKILL.md` harvest bullet is handled in Task 2; verify `skills/docket-convention/SKILL.md:358` and `skills/docket-convention/references/learnings.md:48` "harvest is deferred from Go v1" wording against current reality; `skills/docket-new-change/SKILL.md:51` "scan harvest" is judgment vocabulary, likely no-hit)
- Modify: inventory

**Interfaces:**
- Consumes: Task 1's raw scan hits minus rows resolved by Tasks 2–5.
- Produces: every mechanical hit outside the five seeds resolved with a recorded disposition.

- [ ] **Step 1: Triage each remaining mechanical hit**

For each: is the claim true against the running binary and current owners? A true statement about a deferred feature ("automated learnings harvest is deferred from Go v1 — edit `learnings/` files directly") is current guidance, not staleness — record `no hit: verified current` with the evidence. A false or dead-owner claim gets the ownership-rule treatment as in prior tasks.

- [ ] **Step 2: Apply fixes with the standard guard procedure, run, commit**

```bash
go test ./internal/repoguard/ -count=1
git add skills/ internal/repoguard/ docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md
git commit -m "docs(0154): resolve remaining mechanical hits across skills tree"
```

### Task 7: Full manual read of every remaining population file

**Files:**
- Modify: any population file where the read finds an analogous defect
- Modify: inventory (every `pending` row resolved)

**Interfaces:**
- Consumes: the inventory with all mechanical hits resolved.
- Produces: a complete audit — 29/29 (or the discovered count) rows with `hit: fixed`, `hit: reported (out of scope, change id if known)`, or `no hit`.

- [ ] **Step 1: Read every file not yet fully dispositioned**

The mechanical scans are sampling, not parsing. Read each remaining file end-to-end as a worker in an unknown consuming repo, asking per sentence: does this name a dead owner, restate a runtime-owned list/count/shape, describe retired behavior, or claim something only true in docket's own checkout? The build/build-task/review/implement-next/finalize/adr/groom/brainstorm/auto-groom skill bodies and their references are the bulk of this set.

- [ ] **Step 2: Fix in-scope defects; report adjacent ones**

Apply the ownership rule + guard procedure to genuine hits of this change's class. Findings outside the class (runtime gaps, unrelated doc rewrites) are recorded in the inventory with their existing change id when known — not fixed here, not silently dropped.

- [ ] **Step 3: Verify no pending rows, run, commit**

```bash
/usr/bin/grep -c 'pending' docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md   # expect 0
go test ./internal/repoguard/ -count=1
git add skills/ internal/repoguard/ docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md
git commit -m "docs(0154): complete whole-population manual audit"
```

### Task 8: Regenerate the embedded assets bundle

**Files:**
- Regenerate: `internal/assets/embedded/tree/**`

**Interfaces:**
- Consumes: the final authored `skills/` tree.
- Produces: an embedded bundle byte-matched to the authored tree, ready for the suite's correspondence guards.

- [ ] **Step 1: Regenerate through the owner, never by hand**

```bash
go generate ./internal/assets
```

- [ ] **Step 2: Verify correspondence both ways**

```bash
go test ./internal/assets/ -count=1
diff -r <(git ls-files 'skills/**/*.md' 'skills/*.md' | sort) <(cd internal/assets/embedded/tree && git ls-files 'skills/**/*.md' 'skills/*.md' | sort) || true
git status --porcelain internal/assets/embedded/   # every delta must correspond to an authored edit; a deleted authored file must be gone here too
```

If `github-board-mirror.md` was deleted in Task 3, confirm its embedded copy is deleted by the regeneration and that installed-layout absence is what the guards expect.

- [ ] **Step 3: Commit**

```bash
git add internal/assets/embedded/
git commit -m "docs(0154): regenerate embedded skills bundle"
```

### Task 9: Full suite gate and evidence closure

**Files:**
- Modify: inventory (final evidence appendix)

**Interfaces:**
- Consumes: everything above.
- Produces: a green full-suite run recorded with budget findings; the evidence the results file will cite.

- [ ] **Step 1: Run the full configured suite from this checkout**

Run `go run ./cmd/docket development test` (the resolved `build.test_command`) through the docket-build gate procedure (`docket gate drive advance` slices per the build skill — inline blocking, never backgrounded-and-yielded).

Expected: SUITE pass. Trust the suite summary, not a piped exit code.

- [ ] **Step 2: Read the budget report**

Record any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line as a screening finding and any `SERIAL CONFIRMED OVER BUDGET:` line as an authoritative breach to act on, in the inventory's evidence appendix.

- [ ] **Step 3: Close the evidence appendix**

Append: actual suite result; the guard-mutation matrix (each new/changed guard, the mutation applied, landing proof, red observed); asset-correspondence proof; the 0257 item's disposition; out-of-scope findings reported with ids. This appendix is the source for the results file's inventory obligation.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/plans/2026-09-08-audit-skill-bodies-inventory.md
git commit -m "docs(0154): suite gate evidence and audit close-out"
```

---

## Self-review notes

- Spec coverage: seeds 1–5 → Tasks 2–5; whole-population inventory with no-hit records → Tasks 1, 6, 7; ownership/reference rule and caller-variance preservation → Global Constraints + per-task steps; guard strategy (reuse, narrow additions, mutation proof, no generic prose-lint) → Tasks 2–7 Step patterns and Task 3 Step 5; asset regeneration → Task 8; full configured suite + budget reading → Task 9; 0257 single-item absorption → Task 4 Step 3; out-of-scope honesty → Task 7 Step 2 and Task 9 Step 3.
- Line numbers cited above are reading aids against baseline `d7363492` for the implementer arriving cold; every edit step anchors on the quoted clause or symbol, per ADR-0054, and the inventory records quoted clauses, not line numbers.
- Seeds may already be partially fixed at build time: in that case record the replacement and its evidence in the inventory instead of reapplying the edit (spec, "Inventory and implementation evidence").
