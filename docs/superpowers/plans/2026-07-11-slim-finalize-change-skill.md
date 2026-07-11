# Slim docket-finalize-change Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Behavior-neutral slim of `skills/docket-finalize-change/SKILL.md` (234 lines / 3529 words → ≤ ~140 lines / ≤ ~2200 words) by rewiring the close-out prose to the shared `docket-convention/references/terminal-close-out.md` reference (landed in #0053) and cutting narration — while every invariant survives in meaning, every sentinel test stays green, and no gate/selection/close-out semantics change.

**Architecture:** One skill markdown file is rewritten in place. The shared close-out sequence (archive → re-render → terminal-publish → cleanup → board) is already single-sourced in `references/terminal-close-out.md`; finalize's Step 3 + Terminal-publish section collapse to loud blocking pointers at it, keeping only finalize's own facts (UTC merge date, `--results`, abort-and-report posture). The rebase-retest merge gate stays **inline** (it is on the common path — LEARNINGS #20), compressed by cutting narration only. ADR-0002 gets a dated `## Update`. Seven sentinel test files are deliberately re-anchored where an invariant legitimately moved to the reference, kept where it is finalize-owned.

**Tech Stack:** Markdown (skill + ADR), bash sentinel tests under `tests/`. No scripts, no code.

## Global Constraints

- **Behavior-neutral.** No change to: gate semantics, the selection/classification matrix behavior, sign-off rules, close-out ordering, or any contract. Every deleted sentence must be (a) narration, (b) restated inline, (c) covered by `references/terminal-close-out.md`, or (d) covered by a script contract. No invariant simply vanishes.
- **Size target (asserted in the PR description):** ≤ ~140 lines / ≤ ~2200 words for `skills/docket-finalize-change/SKILL.md`. Baseline verified: 234 lines / 3529 words on origin/main.
- **No literal model/effort tier** in the two agent-dispatch clauses (existing #0017 guard): the finalize body must contain NO `opus`/`sonnet`/`haiku`/`fable` and NO `xhigh`; it must name the wrapper as the tier source (`model/effort its wrapper resolves` / `its wrapper resolves`).
- **The `## Terminal publish (docket-mode)` heading is a cross-ref anchor — keep it byte-for-byte** (test_closeout greps `grep -qF "## Terminal publish (docket-mode)"`).
- **The `2.5 **Harvest learnings.**` step stays finalize-owned and its heading byte-stable** (it is the single source cited by the convention and docket-status).
- **Step-0 preamble citation must use the landed heading verbatim:** `### Step-0 preamble (every operating skill)` in `skills/docket-convention/SKILL.md`.
- **The reference is the single source now:** delete finalize's single-source ownership claims (Overview + Terminal-publish) and the "identical — must not diverge" note.
- Convention loads first in the skill (keep `## Convention (load first — blocking)` shape); keep `name:`/`description:` frontmatter unchanged.

### Sentinel-preservation checklist (every one of these greps must still pass — the invariant they encode must survive, in `finalize` unless noted as legitimately moved to the reference)

**In `skills/docket-finalize-change/SKILL.md` (finalize-owned — MUST remain here):**
- `require_pr_approval: false` with an inline `# … default false` comment (regex `require_pr_approval: *false +#.*default false`)
- `validates` within 1-3 chars of `human sign-off`
- `reviewDecision != APPROVED`
- `exactly one eligible` … `no prompt`
- `more than one eligible` … `prompt`
- `not git-mergeable` … `surface, do not merge`
- a `require_pr_approval`/`reviewDecision != APPROVED` clause … `surface, do not merge`
- `explicit id overrides` … `require_pr_approval`
- `finalize.gate` or `finalize:`
- all four gate values present as words: `local`, `ci`, `both`, `off`
- `off` … one of `today|no rebase|no re-test|trust`
- `docket-rebase-resolver` (and near the word `conflict`)
- `docket-integration-repair` (and near `red`/`fail`)
- `force-with-lease`
- the phrase `before any push` must appear on a line BEFORE the first `force-with-lease` line (ordering assertion)
- `sign-off`; `interactive` … `prompt`/`sign-off`; `autonomous` … `abort-and-report`
- `abort-and-report` appears multiple times (keep the full set — the test counts occurrences)
- abort paths: `ambiguous` … `conflict`; `no` … `suite`/`suite … not found`/`no … test_command`; `two attempts`/`<=2`/`cannot reach green`/`stuck`; `lease … reject`/`reject … lease`/`concurrent push`
- `model/effort its wrapper resolves` (or `its wrapper resolves`); and NO `opus|sonnet|haiku|fable`, NO `xhigh`
- `/archive-change.sh`, `/terminal-publish.sh`, `/cleanup-feature-branch.sh` (invocations kept)
- `## Terminal publish (docket-mode)` (exact heading)
- Accepted gate: `whose ADR is \`Accepted\`` (literal, with backticks — test_docket_metadata_branch) AND a form matching `whose ADR is .?Accepted|Accepted. gate|status: is .?Accepted|status.? is **Accepted` (test_closeout)
- `adr-<NN>` or `ADR-only`; `terminal-publish.sh --adr`
- NO `git mv .*active/`; NO `git worktree add -B .?pub-adr` (no leftover raw bash)
- `terminal publish`/`terminal-publish`; `checkout origin/docket`; `Skipped entirely in \`main\`-mode`
- `Harvest learnings`; `already cites`
- `is **never** published`
- `append interactive-verification`
- `SKILL_FINISH`

**Re-anchoring is allowed ONLY when an invariant genuinely moved to `references/terminal-close-out.md`.** If a sentinel fails because its clause was legitimately collapsed into the reference pointer, re-anchor THAT test's assertion to grep the reference file (`skills/docket-convention/references/terminal-close-out.md`) for the same invariant — preserving the assertion's intent, anchored on a unique phrase the reference owns. NEVER relocate a must-preserve substring in finalize just to pass a grep (the #0036/#0037 twin-sentinel lesson), and NEVER weaken an assertion to nothing. Most of the checklist above stays in finalize; expect only close-out-sequence mechanics (already owned by the reference) to be re-anchor candidates — and note that `checkout origin/docket`, the `## Terminal publish` heading, and the Accepted-gate phrasing are asserted specifically against finalize, so they MUST stay in finalize.

---

### Task 1: Append the dated `## Update` to ADR-0002

**Files:**
- Modify: `docs/adrs/0002-docket-mode-default-and-bootstrap.md`
- Test: `tests/test_adr_checks.sh` (must stay green)

**Interfaces:**
- Consumes: nothing.
- Produces: a new `## Update — 2026-07-11 (change 0054)` section appended after the existing `## Update — 2026-06-19 (change 0025)`, noting that finalize's "terminal-publish single-sourced in finalize" clause's documentation home moved to `skills/docket-convention/references/terminal-close-out.md` (via #0053/#0054), while the single-source *principle* is unchanged (Decision 3 still stands). Do NOT edit the original Decision/Context/Consequences — append only (ADR immutability).

- [ ] **Step 1: Read** `docs/adrs/0002-docket-mode-default-and-bootstrap.md` fully to match the existing `## Update` style.
- [ ] **Step 2: Append** the new dated `## Update` section (a short paragraph, per the style of the 2026-06-19 update): the terminal-publish close-out sequence is now documented in `docket-convention/references/terminal-close-out.md` (the single source for ordering + per-caller failure posture); `docket-finalize-change` still owns *when* to run each step, the Harvest-learnings step, and authors the commit messages; the single-source principle of Decision 3 is unchanged — only the doc home for the shared *sequence* moved. Keep `status: Accepted` unchanged.
- [ ] **Step 3: Verify** `bash tests/test_adr_checks.sh` → zero `NOT OK` (no numbering/index/status regressions).
- [ ] **Step 4: Commit**

```bash
git add docs/adrs/0002-docket-mode-default-and-bootstrap.md
git commit -m "docs(0054): ADR-0002 update — close-out sequence doc home moved to the shared reference"
```

---

### Task 2: Slim `skills/docket-finalize-change/SKILL.md`

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`
- (Do NOT edit tests in this task — Task 3 co-evolves them.)

**Interfaces:**
- Consumes: `skills/docket-convention/references/terminal-close-out.md` (the close-out single source), `skills/docket-convention/SKILL.md` `### Step-0 preamble (every operating skill)`.
- Produces: a slimmed finalize skill body ≤ ~140 lines / ≤ ~2200 words that preserves every finalize-owned sentinel in the Global-Constraints checklist.

- [ ] **Step 1: Read** the current `skills/docket-finalize-change/SKILL.md` in full (234 lines) and `skills/docket-convention/references/terminal-close-out.md`. Map each of the ~7 sections to its plan target (see the spec's section-by-section table): Overview ~15→~6 (drop bookends + ownership claims), When-to-use keep, Convention/Step-0 ~9→~3 (cite the landed preamble heading), Selection ~35→~20 (keep matrix + eligible def + explicit-id-overrides rule; cut prompt-rationale narration), Per-change steps ~70 (keep numbered 1/2/2.5/3/4/5/6; Step 3 archive→re-render→publish collapses to a LOUD blocking pointer at `references/terminal-close-out.md` + finalize-only facts [UTC merge date via `gh mergedAt`, `--results`, abort-and-report]; Step 4 cleanup → ~2L invoke+trust-exit; delete the "identical … must not diverge" note), the rebase-retest gate ~95→~65 inline (keep config block, 6-step flow, two-agent boundary, sign-off rule, full abort-and-report set, PR-comment durable-reason rule; cut why-rebase paragraph + restated wrapper/model prose + rule-free narration), finishing-a-development-branch ~8→~5, Terminal publish (docket-mode) ~45→~8-10 (compact pointer; KEEP the `## Terminal publish (docket-mode)` heading, `checkout origin/docket`, the `whose ADR is \`Accepted\`` phrasing, `terminal-publish.sh --adr`, `adr-<NN>`/`ADR-only`, `Skipped entirely in \`main\`-mode`).
- [ ] **Step 2: Rewrite** the skill in place per that map. Cut change-archaeology provenance narration (0015/0029/0035) per #0053 decision 2 — keep bare `(ADR-NNNN)` where a why is load-bearing. Keep the convention-load-first block and frontmatter.
- [ ] **Step 3: Self-check the sentinel checklist** — for EACH finalize-owned grep target in Global Constraints, confirm it is present. Run this checklist as a script:

```bash
FIN=skills/docket-finalize-change/SKILL.md
# spot-run a few high-risk ones; full set verified by Task 3's tests
grep -qF "## Terminal publish (docket-mode)" "$FIN" && echo "ok heading"
grep -q "checkout origin/docket" "$FIN" && echo "ok checkout"
grep -qF 'whose ADR is `Accepted`' "$FIN" && echo "ok accepted"
grep -qF "Harvest learnings" "$FIN" && grep -qF "already cites" "$FIN" && echo "ok harvest"
grep -qF 'is **never** published' "$FIN" && echo "ok board-never"
grep -qF "append interactive-verification" "$FIN" && echo "ok results"
grep -qF "SKILL_FINISH" "$FIN" && echo "ok finish"
grep -q "force-with-lease" "$FIN" && grep -q "docket-rebase-resolver" "$FIN" && grep -q "docket-integration-repair" "$FIN" && echo "ok gate agents"
! grep -qiE '\b(opus|sonnet|haiku|fable)\b' "$FIN" && ! grep -qiE '\bxhigh\b' "$FIN" && echo "ok no tiers"
! grep -qE 'git mv .*active/' "$FIN" && ! grep -qE 'git worktree add -B .?pub-adr' "$FIN" && echo "ok no raw bash"
```
All echo lines must print. Fix any missing anchor by restoring the invariant's phrasing (in meaning) before proceeding.
- [ ] **Step 4: Check size** `wc -lw skills/docket-finalize-change/SKILL.md` → ≤ ~140 lines / ≤ ~2200 words. If over, cut more narration (never a sentinel).
- [ ] **Step 5: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md
git commit -m "refactor(0054): slim docket-finalize-change — rewire close-out to the shared reference"
```

---

### Task 3: Re-anchor the seven sentinel tests + behavior-neutrality review

**Files:**
- Modify (only where a sentinel legitimately moved): `tests/test_finalize_gate.sh`, `tests/test_closeout.sh`, `tests/test_docket_metadata_branch.sh`, `tests/test_learnings_ledger.sh`, `tests/test_board_refresh_on_transition.sh`, `tests/test_results_artifact.sh`, `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: the slimmed skill from Task 2.
- Produces: all seven tests green, with each re-anchored sentinel preserving its assertion's intent.

- [ ] **Step 1: Run all seven** and collect failures:

```bash
for t in test_finalize_gate test_closeout test_docket_metadata_branch test_learnings_ledger test_board_refresh_on_transition test_results_artifact test_docket_config; do echo "== $t =="; bash tests/$t.sh 2>&1 | grep "NOT OK"; done
```
- [ ] **Step 2: For each failing sentinel, decide and act.** Treat every failure as a review prompt: *does the compressed prose still carry the invariant?*
  - If the invariant is **finalize-owned** (see checklist) and merely got rephrased — restore the exact phrase/anchor in `skills/docket-finalize-change/SKILL.md` (go back to Task 2's file), do NOT touch the test.
  - If the invariant **legitimately moved** to `references/terminal-close-out.md` (a close-out-sequence mechanic the reference now owns) — re-anchor THAT assertion in the test to grep the reference file for the same invariant on a unique phrase it owns. Preserve intent; never weaken to a vacuous grep; never relocate a must-preserve substring just to pass.
  - Record each decision (which sentinels stayed, which re-anchored, and why) for the review.
- [ ] **Step 3: Behavior-neutrality diff review.** `git diff origin/main -- skills/docket-finalize-change/SKILL.md` and confirm every deleted sentence is narration / restated / covered-by-reference / covered-by-script-contract. Also **anchor grep-gate:** confirm no skill, agent wrapper, or script contract cites a finalize heading that no longer exists:

```bash
# every '## '/'### ' heading finalize still has:
grep -nE '^#{2,3} ' skills/docket-finalize-change/SKILL.md
# nothing outside finalize should reference a removed finalize heading — spot-check cross-refs:
grep -rnE 'docket-finalize-change/SKILL|Terminal publish|Harvest learnings' skills scripts --include=*.md | grep -v 'skills/docket-finalize-change/SKILL.md' | head
```
Confirm the `## Terminal publish (docket-mode)` and `2.5 **Harvest learnings.**` anchors survived.
- [ ] **Step 4: Re-run all seven** — zero `NOT OK`.
- [ ] **Step 5: Commit**

```bash
git add tests/
git commit -m "test(0054): re-anchor finalize sentinels after the slim (intent-preserving)"
```

---

### Task 4: Full-suite verification

**Files:** none (verification only).

**Interfaces:**
- Consumes: the whole change.
- Produces: evidence the whole suite is green and the size target is met.

- [ ] **Step 1: Run the FULL suite in ONE foreground call** (LEARNINGS: never background the ~10-min docket suite):

```bash
for t in tests/test_*.sh; do out="$(bash "$t" 2>&1)"; n=$(grep -c "^NOT OK" <<<"$out"); [ "$n" -gt 0 ] && echo "FAIL $(basename "$t") ($n)"; done; echo "suite done"
```
Expected: no `FAIL` lines. If any test outside the seven regressed, it means an invariant it asserts against finalize (or a moved reference) broke — fix in the skill (restore the invariant) or re-anchor that test with justification.
- [ ] **Step 2: Assert size** `wc -lw skills/docket-finalize-change/SKILL.md` → ≤ ~140 lines / ≤ ~2200 words. Record the exact numbers for the PR description.
- [ ] **Step 3: Commit** (only if any fix was needed; otherwise skip):

```bash
git add -A && git commit -m "test(0054): full-suite green after finalize slim"
```

---

## Notes for the implementer

- This is a **documentation** change; there is no runtime to smoke-test beyond the sentinel tests. The spec's "live smoke" (finalizing #0054's own merged PR) happens at merge time, not during the build — do not attempt to merge.
- The single highest risk is **silently dropping an invariant while compressing** (LEARNINGS #52: a goal-scoped rewrite passes its own audit while a dimension outside the goal set slips through). The sentinel checklist in Global Constraints IS the external audit — run it exhaustively; a green suite is the gate.
- Do NOT change gate/selection/close-out semantics. If a compression would alter meaning, keep the longer phrasing — size is a target, correctness is the constraint.
- Run the full suite in ONE foreground Bash call with `timeout 600000`.
