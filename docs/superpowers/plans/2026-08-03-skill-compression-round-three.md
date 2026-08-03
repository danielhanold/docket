<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0201 — Skill compression round three — targeted progressive disclosure on the Big 4 + regrowth-guard ratchet](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-03-0201-skill-compression-round-three.md)**
<!-- docket:backlink:end -->

# Skill Compression Round Three Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply targeted progressive disclosure to the Big 4 skill files (convention, finalize-change, implement-next, build) — three cold-path reference extractions plus in-place tightening — then ratchet the regrowth-guard budgets down to post-slim actuals and harden the raise procedure.

**Architecture:** Each extraction moves a cold path behind a loud blocking pointer at its trigger moment (the proven `docket-convention/references/` pattern); hot paths are tightened in place, wording only. Every task is a behavior-neutral prose relocation verified by the test suite's existing anchors (the anchor grep-gate), so each task's commit must leave the focused test set green. The budget test's completeness guard auto-requires a row for every new `skills/**/*.md`, so each extraction adds its reference's budget row in the same commit.

**Tech Stack:** Markdown skill files; bash test suite (`tests/test_*.sh`) run with `"$DOCKET_BASH_PATH"` (GNU bash from the preflight export — `/opt/homebrew/bin/bash`).

## Global Constraints

- **Behavior-neutrality outranks the size target** (learnings: size-target-is-direction). No rule added, dropped, or reweighted — only relocated or reworded. If review shows residual prose is load-bearing, accept the overshoot and stop trimming.
- **Prior inline-placement decisions are honored, not reversed** (spec §2): 0137's dispatch-capability rule + A/B/C tier table stays in convention's SKILL.md; 0167's nine halting dispositions stay in docket-build's `## Halting conditions`. Tighten wording only — same content, same location, same citation anchors.
- **Out of scope:** frontmatter `description:` lines; agent wrappers + `sync-agents.sh`; scripts and script contracts; templates; `github-board-mirror.md`; the three existing convention references; the seven skills ≤ 1.4k words.
- **Anchor harvest before every deletion** (learnings: restatement-accumulates-its-own-guards): before removing or moving any sentence, grep `tests/` for its distinctive phrases — asserts anchor to copies, not sources. A hit means keep the phrase, move the assert's target, or relocate the phrase into the extracted file *and repoint nothing silently*.
- **Diff restatements against each other before consolidating** (learnings: consolidation-flattens-caller-variance): where two litanies differ in posture (must-land vs best-effort, abort-and-report vs continue), the difference is behavior — keep both sentences.
- **Stub + pointer under the original heading** (learnings: skill-extraction-and-stub-pointer): extracted sections keep their heading in the parent as a stub with a blocking pointer, so name-based cross-refs (other skills cite these headings in italics) still resolve.
- **Pipefail discipline** (AGENTS.md): never `producer | grep -q`; capture into a variable first. `grep` patterns leading with `--` need `-e`/`--`.
- **Marker blocks:** validate order/balance before touching any marker-bounded block; the `configured-bash-finalize` block in finalize's SKILL.md and the `docket:build-evidence` block shape in docket-build's SKILL.md are cross-skill contracts and must survive byte-identical.
- **Word/line measurement:** `wc -l < file` / `wc -w < file`. Budget rounding rule (from the BUDGETS comment): lines up to the next multiple of 5, words to the next multiple of 50; if that lands within 25 words (or a proportional near-zero line margin) of the actual, take the multiple after.
- Full suite at the build gate: `for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test"; done` — never only the enumerated focused tests.

---

### Task 1: Extract `docket-finalize-change/references/gate-failure.md`

**Build profile:** standard

**Files:**
- Create: `skills/docket-finalize-change/references/gate-failure.md`
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `tests/test_skill_size_budgets.sh` (add the new reference's budget row + comment entry)

**Interfaces:**
- Consumes: current finalize SKILL.md sections.
- Produces: `references/gate-failure.md` holding the merge-gate *failure* flows; SKILL.md keeps the gate sequence, green-path merge, selection, dispositions, and one blocking pointer at the failure branch. Task 2 tightens the remaining SKILL.md text; Task 8 ratchets its budget row.

- [ ] **Step 1: Anchor harvest.** For each section about to move, grep `tests/` for its distinctive phrases. Sections moving (bodies move; see step 3 for what each leaves behind):
  - `### The two agents (split at rebase-completion)`
  - `### Sign-off on auto-authored repairs`
  - `### abort-and-report points (the full set)` including **Where the reason surfaces**
  - `### ## Finalize blocked — marking a change that needs a human` (the whole marker-lifecycle section: write shape, bare-heading rule, re-mark-replaces rule, auto-detect skip + named-id override, CONFLICTING-not-marked rule, clearing rule, board cell)
  - The failure legs inside **Flow** items 2 and 5 (conflict → resolver dispatch; red → repair dispatch).

  Run at minimum (capture-then-grep per AGENTS.md; add more phrases as the harvest suggests):
  ```bash
  cd "$WORKTREE"  # .worktrees/skill-compression-round-three
  for p in "docket-rebase-resolver" "docket-integration-repair" "Sign-off" "sign-off" \
           "abort-and-report" "Finalize blocked" "finalize blocked — needs you" \
           "re-mark REPLACES" "gate-failure"; do
    echo "== $p"; grep -rlF -- "$p" tests/ || true
  done
  ```
  Record which tests anchor which phrase — those phrases must remain greppable at the location the test reads (most read SKILL.md; keep those phrases in the retained SKILL.md text or update nothing and verify the focused set stays green).

- [ ] **Step 2: Create `skills/docket-finalize-change/references/gate-failure.md`** with a title line, one-sentence scope ("The merge-gate failure flows — read when the gate does not pass clean. Loaded on demand from `docket-finalize-change/SKILL.md`; not auto-loaded."), then the moved bodies verbatim (wording untouched in this task): the two agents split, sign-off on auto-authored repairs, the abort-and-report point set + where the reason surfaces, and the full `## Finalize blocked` marker lifecycle.

- [ ] **Step 3: Slim SKILL.md.** Replace each moved section with its stub: keep the heading, one-to-two-line summary, and the blocking pointer — the house shape is the convention's existing "**read `references/….md` now (blocking)**" phrasing. Specifically:
  - Flow item 2 keeps: rebase + "On conflict → **read `references/gate-failure.md` now (blocking)** — it owns the resolver dispatch and the ambiguous-conflict abort."
  - Flow item 5 keeps: validate-per-gate + "On red → **read `references/gate-failure.md` now (blocking)** — it owns the repair dispatch, the sign-off rule, and every abort-and-report point."
  - `## Finalize blocked` keeps heading + "A gate failure is recorded as a `## Finalize blocked` body section on the change file; the write shape, auto-detect skip, named-id override, and clearing rule live in `references/gate-failure.md` (**read it before marking, skipping, or clearing**)." Keep any single sentence the anchor harvest proved a test greps from SKILL.md (e.g. Selection's skip line already carries "already carrying `## Finalize blocked`" — Selection is not moving).
  - The Selection section, disposition table, per-change steps 1–6, the gate's green path, the `configured-bash-finalize` marker block, and Terminal publish stay in SKILL.md untouched.

- [ ] **Step 4: Add the budget row + comment entry** for the new file in `tests/test_skill_size_budgets.sh`: measure `wc -l`/`wc -w` of `gate-failure.md`, apply the rounding rule, insert `skills/docket-finalize-change/references/gate-failure.md  <L> <W>` in the BUDGETS table (alphabetical position), and add a comment entry noting the row was set by change 0201 per the rounding rule from measured actuals.

- [ ] **Step 5: Run the focused test set** (every test that greps finalize's SKILL.md or references dirs):
  ```bash
  for t in test_board_refresh_on_transition test_closeout test_config_read_channel \
           test_configured_bash_finalize test_docket_config test_docket_example_yml \
           test_docket_metadata_branch test_docket_review test_finalize_disposition \
           test_finalize_gate test_learnings_ledger test_readme_finalize_docs \
           test_results_artifact test_skill_size_budgets; do
    "$DOCKET_BASH_PATH" "tests/$t.sh" || echo "RED: $t"
  done
  ```
  Expected: all PASS. A RED here means an assert anchored to a moved phrase — fix by keeping the phrase in SKILL.md's stub or (if the test's own comment says it guards the mechanics) repointing the assert at `references/gate-failure.md` in this same commit, per the relocation-not-restoration rule.

- [ ] **Step 6: Loud-pointer + moved-not-copied self-check.** Verify each moved body appears exactly once (in the reference), each stub heading survives in SKILL.md, and SKILL.md names `references/gate-failure.md` at every trigger moment (conflict, red, marker lifecycle).

- [ ] **Step 7: Commit**
  ```bash
  git add skills/docket-finalize-change/ tests/test_skill_size_budgets.sh
  git commit -m "refactor(0201): extract finalize gate-failure flows to references/gate-failure.md"
  ```

### Task 2: Tighten `docket-finalize-change/SKILL.md` in place

**Build profile:** standard

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`

**Interfaces:**
- Consumes: Task 1's slimmed SKILL.md.
- Produces: finalize SKILL.md at ≤ ~2,900 words (direction, not gate); all anchors intact. Task 8 measures it.

- [ ] **Step 1: Anchor harvest** (same command shape as Task 1 Step 1) over the phrases you intend to reword — especially in Selection, Ordering, the disposition table, the durable-root section, and step 2.5 (harvest). `test_learnings_ledger.sh`, `test_closeout.sh`, `test_finalize_disposition.sh`, and `test_readme_finalize_docs.sh` grep this file's prose.

- [ ] **Step 2: Tighten, wording only.** The moves (spec §In-place tightening):
  - Provenance narration → bare `(change NNNN)` / `(ADR-NNNN)` pointers (e.g. "Version-defense (change 0062's spike observed…)" compresses; "(change 0102)"-style parentheticals stay).
  - Duplicated litanies → single owner + citation: the durable-root rationale, repeated "abort-and-report" restatements now owned by `references/gate-failure.md`, repeated `preflight`-export reminders (state once).
  - Enumerable prose → tables where a list is already implicit (the Selection matrix and disposition table already exist — leave; candidates: the gate values, the ordering keys if prose can drop).
  - Do NOT touch: the `configured-bash-finalize` marker block; the disposition table's four words; Selection's skip-reason enumeration; step 2.5's harvest procedure (it is the single source the sweep invokes by reference — tighten narration around it only, not its rules).
- [ ] **Step 3: Verify posture variance survived.** Diff before/after and confirm no best-effort became must-land or vice versa (step 5 board is must-land; step 6 sync is best-effort; harvest is best-effort-with-surfaced-failure).
- [ ] **Step 4: Run the Task 1 focused test set.** Expected: all PASS.
- [ ] **Step 5: Measure** `wc -w` — target ≤ ~2,900; if materially over, re-check for un-cut narration; if the residual is load-bearing, accept and note the actual for Task 8.
- [ ] **Step 6: Commit**
  ```bash
  git add skills/docket-finalize-change/SKILL.md
  git commit -m "refactor(0201): tighten finalize SKILL.md in place"
  ```

### Task 3: Extract `docket-implement-next/references/edge-paths.md`

**Build profile:** standard

**Files:**
- Create: `skills/docket-implement-next/references/edge-paths.md`
- Modify: `skills/docket-implement-next/SKILL.md`
- Modify: `tests/test_skill_size_budgets.sh` (budget row + comment entry)

**Interfaces:**
- Consumes: current implement-next SKILL.md.
- Produces: `references/edge-paths.md` holding the rare edges; SKILL.md keeps the spine (pick → claim → reconcile → plan → build → review → PR) with one-line trigger + pointer per edge. **The four dispatch-site Tier A/C clauses and their *Dispatch-capability resolution* back-pointers stay inline, verbatim-in-meaning (0137 anchors).** Task 4 tightens; Task 8 ratchets.

- [ ] **Step 1: Anchor harvest.** Moving candidates:
  - Step 3's reconcile-kill escape hatch (the OBSOLETE → terminal close-out bullet).
  - Resume-with-explicit-id semantics (the `## The reconcile pass and the reconciled flag` resume-safety half; the selection-skips-in-progress consequence).
  - Step 7's PR-body assembly mechanics: the PR→issue reference rule, the PR-body back-link line (change 0136), the build-evidence block-writing detail (marker order/balance validation, stale-`head_sha`-expected note).
  - NOT moving: Step 6's rung selection + triage (0170 anchors, executed verbatim); the four Tier A/C dispatch clauses (0137); Step 2's claim CAS; the field-write rule; the disposition table.

  ```bash
  for p in "reconcile-kill" "OBSOLETE" "terminal-close-out.md" "back-link line" \
           "docket:build-evidence" "Closes #N" "↩ Change" "marker order and balance" \
           "edge-paths"; do
    echo "== $p"; grep -rlF -- "$p" tests/ || true
  done
  ```
  `test_dispatch_capability.sh` greps the Tier A/C site clauses — run it after every edit to this file. `test_composition_wiring.sh`, `test_closeout.sh`, `test_results_artifact.sh`, `test_artifact_backlink_coverage.sh`, `test_loop_continuation.sh` also anchor here.

- [ ] **Step 2: Create `skills/docket-implement-next/references/edge-paths.md`** — title, one-sentence scope ("The implementer's rare edges — read at the trigger moment named in SKILL.md; not auto-loaded."), then the moved bodies verbatim: reconcile-kill close-out procedure (with its board-pass-best-effort and loop-back-to-Step-1 rules), resume-with-id guidance (re-run reconcile when `reconciled: false` or base advanced; trust commits, not checkboxes; a bare resume claims a different change — pass the id), and PR-body assembly (issue reference / never `Closes #N`, the `↩ Change <padded-id> — <title>` back-link shape, evidence-block write mechanics + marker validation + expected-staleness note).

- [ ] **Step 3: Slim SKILL.md.** Each edge compresses to trigger + pointer at its site:
  - Step 3: "Change now **OBSOLETE** → kill it via the terminal close-out — **read `references/edge-paths.md` now (blocking)**; it owns the kill sequence, cleanup, board pass, and the loop back to Step 1." (The FUNDAMENTALLY-invalidated → `halted` hatch stays inline — it is the skill's stop signal, not an edge procedure.)
  - Reconcile-flag section keeps the audit-signal definition + "on any resume — **read `references/edge-paths.md` (blocking)** for the resume rules."
  - Step 7 keeps: invoke `$SKILL_FINISH` DIRECTED-to + "assembling the PR body (issue reference, back-link line, evidence block) → **read `references/edge-paths.md` now (blocking)**", plus the `status: implemented` + field-write + board-pass close. The evidence block's *existence* and finalize-reads-it contract stay named inline; the assembly mechanics move.
- [ ] **Step 4: Budget row + comment entry** for `skills/docket-implement-next/references/edge-paths.md`, measured + rounded as in Task 1 Step 4.
- [ ] **Step 5: Run the focused set:**
  ```bash
  for t in test_artifact_backlink_coverage test_board_refresh_on_transition test_closeout \
           test_composition_wiring test_dispatch_capability test_docket_config \
           test_docket_metadata_branch test_docket_review test_learnings_ledger \
           test_loop_continuation test_results_artifact test_skill_facade_wiring \
           test_skill_size_budgets; do
    "$DOCKET_BASH_PATH" "tests/$t.sh" || echo "RED: $t"
  done
  ```
  Expected: all PASS. Same relocation-not-restoration rule on any RED.
- [ ] **Step 6: Loud-pointer + moved-not-copied self-check** (as Task 1 Step 6).
- [ ] **Step 7: Commit**
  ```bash
  git add skills/docket-implement-next/ tests/test_skill_size_budgets.sh
  git commit -m "refactor(0201): extract implement-next edge paths to references/edge-paths.md"
  ```

### Task 4: Tighten `docket-implement-next/SKILL.md` in place

**Build profile:** standard

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md`

**Interfaces:**
- Consumes: Task 3's slimmed SKILL.md.
- Produces: implement-next SKILL.md at ≤ ~2,900 words (direction); 0137/0170 anchors byte-survivable. Task 8 measures.

- [ ] **Step 1: Anchor harvest** on phrases being reworded. This file has the densest anchor set: the Tier A/C clauses (tighten around them, never inside their site-noun + tier + back-pointer conjunctions), Step 6's rung map and triage rules, the `DIRECTED to:` markers (`test_skill_handoff_precedence.sh` greps these), the field-write rule.
- [ ] **Step 2: Tighten, wording only:** provenance narration → bare pointers ("(change 0136)", "(change 0170)" stay as pointers; sentences *about* what a change did compress to the rule + pointer); duplicated metadata-discipline litanies → the `### The field-write rule` single owner + short citations at use sites; Step 1's digest-acquisition narration → its rule core; Step 4's cross-tree narration → the invariant sentences.
- [ ] **Step 3: Run the Task 3 focused set.** Expected: all PASS.
- [ ] **Step 4: Measure** `wc -w` — target ≤ ~2,900 (direction; note actual for Task 8).
- [ ] **Step 5: Commit**
  ```bash
  git add skills/docket-implement-next/SKILL.md
  git commit -m "refactor(0201): tighten implement-next SKILL.md in place"
  ```

### Task 5: Extract `docket-convention/references/auto-capture.md`

**Build profile:** premium — the convention is the hot shared contract every docket run loads; 20 test files anchor into it (a consequential risk named by this plan).

**Files:**
- Create: `skills/docket-convention/references/auto-capture.md`
- Modify: `skills/docket-convention/SKILL.md`
- Modify: `tests/test_skill_size_budgets.sh` (budget row + comment entry)

**Interfaces:**
- Consumes: convention's `### Auto-capture (shared definition)` section.
- Produces: the full definition in `references/auto-capture.md`; SKILL.md keeps the stub heading + ~4-line summary + read trigger (mirroring the learnings.md precedent). Consumers (`docket-implement-next`, `docket-finalize-change`, `docket-status` prose citing "the convention's *Auto-capture* shared definition") resolve against the surviving heading.

- [ ] **Step 1: Anchor harvest:**
  ```bash
  for p in "Auto-capture" "auto-capture" "mint-stub" "policy-suppressed" "materiality bar" \
           "discovered_from" "exit 3" "cap (3)" "never both by default" "auto-capture.md"; do
    echo "== $p"; grep -rlF -- "$p" tests/ || true
  done
  ```
  Also confirm none of `test_convention_extraction.sh`'s ten anti-copy sentinels live inside the Auto-capture section (none should; verify before moving).
- [ ] **Step 2: Create `skills/docket-convention/references/auto-capture.md`** — title, scope sentence ("The full auto-capture shared definition — read before minting or suppressing a discovered stub; not auto-loaded."), then the moved body verbatim: the per-discovery classify → admit → suppress sequence, the materiality bar, the deterministic mint-stub invocation (flags, `--minted <n so far>` carry-forward rule, docket- vs main-mode `--changes-dir`), exit codes (3 duplicate / 4 cap / 1 real error), the surfaced-never-silent + best-effort-never-aborts posture, and the mint-is-metadata-only rule.
- [ ] **Step 3: Slim SKILL.md.** Under the surviving `### Auto-capture (shared definition)` heading keep exactly: (1) what it governs — what an autonomous skill does with follow-up work discovered mid-run — and the `auto_capture` map knobs (`enabled` default `false`, `types` default `all`; global-able); (2) disabled ⇒ report in prose, enabled ⇒ classify then mint an ordinary `proposed` needs-brainstorm stub with `discovered_from:`/`type:` — capture fidelity, not autonomy; (3) mint sites (`docket-implement-next` reconcile + review, the finalize/status harvest) and the never-mint rule for `docket-auto-groom` + interactive skills; (4) the read trigger: "Discovered follow-up work mid-run → **read `references/auto-capture.md` now (blocking)** before minting or suppressing." Everything else moves.
- [ ] **Step 4: Budget row + comment entry** for `skills/docket-convention/references/auto-capture.md` (measured + rounded).
- [ ] **Step 5: Run the focused set** (the convention's 20 anchoring tests):
  ```bash
  for t in test_artifact_backlink_coverage test_auto_groom test_board_refresh_on_transition \
           test_change_links_coverage test_composition_wiring test_config_read_channel \
           test_convention_extraction test_cursor_permissions_docs test_dispatch_capability \
           test_docket_config test_docket_metadata_branch test_finalize_disposition \
           test_finalize_gate test_learnings_ledger test_results_artifact \
           test_role_skill_self_description test_skill_facade_wiring \
           test_skill_handoff_precedence test_skill_size_budgets test_sync_agents; do
    "$DOCKET_BASH_PATH" "tests/$t.sh" || echo "RED: $t"
  done
  ```
  Expected: all PASS.
- [ ] **Step 6: Loud-pointer + moved-not-copied self-check;** also verify the operating skills' citations ("per the convention's *Auto-capture* shared definition") still resolve to the surviving heading.
- [ ] **Step 7: Commit**
  ```bash
  git add skills/docket-convention/ tests/test_skill_size_budgets.sh
  git commit -m "refactor(0201): extract convention auto-capture definition to references/auto-capture.md"
  ```

### Task 6: Tighten `docket-convention/SKILL.md` in place

**Build profile:** premium — same named risk as Task 5, and this file has a proven compressibility floor (learnings: size-target-is-direction, #85 entry).

**Files:**
- Modify: `skills/docket-convention/SKILL.md`

**Interfaces:**
- Consumes: Task 5's slimmed SKILL.md.
- Produces: convention SKILL.md at ≤ ~4,700 words (direction). The 0137 dispatch-capability rule + tier table, the Step-0 preamble, the bootstrap 2×2, build-readiness/selection, and the lifecycle table remain inline. Task 8 measures.

- [ ] **Step 1: Anchor harvest** on every phrase being reworded. Mandatory keeps beyond Task 5's list: `test_convention_extraction.sh`'s section headers (`### Configuration`, `### Directory layout`, `### Change manifest`, `### ADR file`, `### Lifecycle`, `### Build-readiness`, `### Bootstrap guard`, `### Branch model`) and its ten anti-copy sentinels ("never gitignored", "proposed ──claim──▶", "satisfied when it reaches", "immutable once Accepted", "live planning surface", "half-migrated", "only flow of metadata onto the code line", "zero-padded to 4 digits", "PM-altitude proposal", "must never trail the change files") — all must remain in this file. `test_dispatch_capability.sh` pins the tier table; `test_docket_config.sh` and `test_config_read_channel.sh` pin config prose; `test_role_skill_self_description.sh` pins the Skill-layer bullet.
- [ ] **Step 2: Tighten, wording only:**
  - Provenance narration → bare pointers ("(change 0084)", "(ADR-0019)" style survives; sentences narrating history compress to the rule).
  - Bootstrap-guard probe prose compresses toward the 2×2 verdict table (the table + probe definitions stay; the surrounding narration tightens).
  - Duplicated litanies → single owner + citation: the "commit and push on metadata_branch immediately" refrain (owned by *Branch model* / Step-0 preamble; use sites cite), the facade-invocation shape (state once at *Reaching the helper scripts*).
  - Enumerable prose → tables where enumerable (candidates: the `board_surfaces` token behaviors; the config-layer precedence chain if prose can drop).
  - Do NOT touch content/location of: dispatch-capability resolution + tier table (0137); the Step-0 preamble's numbered steps; the lifecycle diagram + table; build-readiness/selection definition; the Skill-layer role table + autonomy-precedence paragraph (its "a future slim must not keep it and drop the call-site directions" sentence is a direct constraint on this task); the Agent-layer wrapper counts (suite-asserted); the coordination-key fence list.
- [ ] **Step 3: Run the Task 5 focused set.** Expected: all PASS.
- [ ] **Step 4: Measure** `wc -w` — target ≤ ~4,700 (direction; the #85 floor precedent says accept a load-bearing overshoot; note actual for Task 8).
- [ ] **Step 5: Commit**
  ```bash
  git add skills/docket-convention/SKILL.md
  git commit -m "refactor(0201): tighten convention SKILL.md in place"
  ```

### Task 7: Tighten `docket-build/SKILL.md` in place

**Build profile:** standard

**Files:**
- Modify: `skills/docket-build/SKILL.md`

**Interfaces:**
- Consumes: current docket-build SKILL.md (no extraction for this file).
- Produces: docket-build SKILL.md at ≤ ~2,100 words (direction); the nine `## Halting conditions` and the build-evidence block shape intact in place. Task 8 measures.

- [ ] **Step 1: Anchor harvest:** `test_docket_build.sh` and `test_docket_review.sh` grep this file (routing rubric nouns, halting conditions, the evidence-record shape, the escalation ladder).
  ```bash
  for p in "Halting conditions" "docket:build-evidence" "NEEDS_ESCALATION" "escalation allowance" \
           "merge-base --is-ancestor" "stray commit" "configured-bash-finalize" "uncertainty sink"; do
    echo "== $p"; grep -rlF -- "$p" tests/ || true
  done
  ```
- [ ] **Step 2: Tighten, wording only:** provenance → pointers; the routing rubric's narration compresses around its four bullets (the organizing principle sentence and the four-conditions economy rule stay — 0184 anchors); the escalation section's narration tightens around the ladder block; `## Halting conditions` keeps all nine bullets in place, tightened in wording only (0167); the evidence-record fenced block survives byte-identical; checkpointing prose compresses around its two modes.
- [ ] **Step 3: Run focused tests:** `"$DOCKET_BASH_PATH" tests/test_docket_build.sh`, `tests/test_docket_review.sh`, `tests/test_skill_size_budgets.sh`. Expected: PASS.
- [ ] **Step 4: Measure** `wc -w` — target ≤ ~2,100 (direction; note actual).
- [ ] **Step 5: Commit**
  ```bash
  git add skills/docket-build/SKILL.md
  git commit -m "refactor(0201): tighten docket-build SKILL.md in place"
  ```

### Task 8: Ratchet the regrowth guard + harden the raise procedure

**Build profile:** standard

**Files:**
- Modify: `tests/test_skill_size_budgets.sh`

**Interfaces:**
- Consumes: the post-slim actuals of all four SKILL.md files (Tasks 1–7 landed) and the three new reference rows (already added by Tasks 1/3/5).
- Produces: Big-4 budget rows ratcheted down to post-slim actuals + margin; the BUDGETS comment procedure now requires a raise to name the reference file considered and why the prose cannot live there.

- [ ] **Step 1: Measure all four files:**
  ```bash
  for f in skills/docket-convention/SKILL.md skills/docket-finalize-change/SKILL.md \
           skills/docket-implement-next/SKILL.md skills/docket-build/SKILL.md; do
    echo "$f $(wc -l < "$f" | tr -d ' ') $(wc -w < "$f" | tr -d ' ')"; done
  ```
- [ ] **Step 2: Ratchet the four rows** to the measured actuals via the rounding rule (lines → next multiple of 5, words → next multiple of 50, within-25 → the multiple after; apply the established near-zero-line-margin judgment for short margins). Verify each new reference row (Tasks 1/3/5) still fits its measured actual.
- [ ] **Step 3: Harden the raise procedure.** In the BUDGETS comment block, immediately after the "To raise a budget, edit the number here in the same diff that grows the file." sentence, add: "A raise must additionally NAME the references/ file the new prose was considered for and STATE why it cannot live there (a rule that must intervene at the moment of action, a cross-skill contract quoted where it is produced, …) — 'no other home' is a claim argued in-diff, not asserted (change 0201)." Add one comment entry documenting this change's ratchet: the four old rows, the four new rows, and the three added reference rows.
- [ ] **Step 4: Mutation-test the ratchet** (guards-are-code): temporarily set one Big-4 row 50 words below its measured actual, run the test, confirm it reddens on that row; restore. Then run clean:
  ```bash
  "$DOCKET_BASH_PATH" tests/test_skill_size_budgets.sh
  ```
  Expected: PASS with the new lower rows.
- [ ] **Step 5: Verification sweep (spec §Verification, items the per-task loops cannot see):**
  - Loud-pointer check across all three extractions: each reference file reachable from exactly one blocking pointer *per trigger moment* in its parent; grep each parent for `references/<name>.md`.
  - Whole-branch behavior-neutrality read: `git diff origin/main...HEAD -- skills/` reviewed for any rule added/dropped/reweighted (postures, gates, dispositions unchanged).
  - `docket-status` smoke run: `"${DOCKET_SCRIPTS_DIR:?}"/docket.sh docket-status --digest-only` exits 0 with a `ready` line.
- [ ] **Step 6: Commit**
  ```bash
  git add tests/test_skill_size_budgets.sh
  git commit -m "test(0201): ratchet skill budgets to post-slim actuals; raises must argue the reference-file case"
  ```

---

## Self-review notes

- Spec coverage: three extractions (Tasks 1, 3, 5), in-place tightening of all four (Tasks 2, 4, 6, 7), ratchet + raise-rule (Task 8), verification recipe (per-task anchor harvests + focused sets; Task 8 Step 5 carries the loud-pointer check, neutrality read, and smoke run; the full suite runs at the build gate).
- The budget-test completeness guard forces each reference's row into its creating commit (Tasks 1/3/5 Step 4) — the suite is green after every task, not only at the end.
- Word targets are direction, not gate, in every measuring step, per learnings.
- No task touches `.docket/`, wrappers, scripts, or templates; Task 8 is the only non-`skills/` edit (the test file).
