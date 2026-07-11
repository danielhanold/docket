# Slim docket-implement-next + small skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Behavior-neutral slim of `docket-implement-next` (137 → ~95–100 lines) and Step-0 compression of the four small skills (`docket-new-change` 70→~55, `docket-groom-next` 77→~65, `docket-adr` 88→~78, `docket-auto-groom` 64→~58), by naming the `render-change-links.sh` field-write rule once, rewiring both kill paths onto `references/terminal-close-out.md`, and cutting narration — with exactly ONE named behavior delta (the kill paths gain the reference's step-2 `## Artifacts` re-render; the reference gains a one-line no-diff-is-success clarifier).

**Architecture:** Five skill markdown files are slimmed in place; one clarifier line is added to the shared reference. The `render-change-links.sh` regeneration litany in implement-next (3 sites: steps 4/6/7) collapses to a single named **field-write rule** in *Branch & metadata discipline*, with the three sites becoming one-line pointers. Both kill paths (new-change proposed-kill, implement-next reconcile-kill) rewire to the reference (same pattern #0053 used for docket-status's sweep), keeping only caller posture skill-side. Sentinel tests follow content: the script-invocation asserts that move to the reference are re-pointed there; everything else stays anchored in place.

**Tech Stack:** Markdown (skills + one reference line), bash sentinel tests under `tests/`. No scripts, no code, no frontmatter `description:` edits, no agent wrappers.

## Global Constraints

- **Behavior-neutral EXCEPT decision 3's one named delta:** both kill paths adopt the reference's full sequence, gaining its step-2 `## Artifacts` re-render (a no-op for kills — a killed change has no `plan:`/`results:` to re-point); the reference gains a one-line clarifier that a **no-diff re-render is success** (commit only when the block changed; unchanged ⇒ proceed to publish — so an empty re-render never trips the skip-publish guard). No other semantics change to selection, claim CAS, reconcile, plan/build/review, PR flow, or kill outcomes.
- **Sizes (asserted in the PR description):** implement-next ≤ ~100 L; new-change ~55 L; groom-next ~65 L; adr ~78 L; auto-groom ~58 L. Baselines on origin/main: 137/70/77/88/64 lines.
- **The `## Convention (load first — blocking)` section stays verbatim in every skill** (the convention cannot instruct its own loading).
- **`test_docket_config.sh` per-skill loop requires the literal `/docket-config.sh`** in each skill body — the compressed Step-0 pointer KEEPS the literal one-line `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"` in every one of the five skills (decision 2; deliberately unlike #0053's docket-status compression).
- **#0017 guard:** no model/effort tier literal (`opus`/`sonnet`/`haiku`/`fable`/`xhigh`) in implement-next; the `docket-status` and `docket-adr` dispatch clauses name the wrapper-resolved tier (`model/effort its wrapper resolves`). `test_composition_wiring.sh` guards this.
- **No bare `scripts/<name>.sh`** paths in any skill body (`test_consuming_repo_scripts.sh`): use `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh`.
- **Run the FULL suite as the gate** (LEARNINGS #52 / the #0054 experience: the full suite caught an out-of-scope sentinel regression the anticipated list missed). Never background the ~10-min suite; ONE foreground call, `timeout 600000`.

### Sentinel-preservation & re-point disposition (from spec §3 — the external audit)

**MUST STAY in the named skill (do NOT re-point these tests):**
- `test_docket_config.sh`: each of the 5 skills contains `/docket-config.sh`.
- `test_change_links_coverage.sh`: `docket-implement-next` contains `/render-change-links.sh` **exactly where the field-write rule states it** (the one surviving full invocation). (This test checks impl + finalize + status; new-change/groom/adr/auto-groom are not checked by it, but keep any single existing render-change-links mention they have.)
- `test_board_refresh_on_transition.sh`: `docket-implement-next` keeps the `## Best-effort board refresh` heading AND ≥3 occurrences of `run the Board pass (best-effort`; `docket-new-change` keeps the literal `must-land Board pass`. **No edit.**
- `test_docket_metadata_branch.sh`: `integration_branch` present in every skill; `metadata working tree` present in every skill; new-change keeps `origin/<integration_branch>`; **H. kill-wiring** — `docket-new-change` body contains both `kill` and `terminal-publish` (word); `docket-implement-next` body contains both `kill` and `terminal-publish` (word). The kill-path rewire's summary line MUST name `terminal-publish` in prose so these pass. Also: `v1 rough edge` must NOT appear in implement-next. **No edit to the test.**
- `test_composition_wiring.sh`: implement-next names the `docket-status` dispatch + wrapper-resolved tiers, no model literals. **No edit.**
- `test_learnings_ledger.sh`, `test_groom_recap.sh`, `test_auto_groom.sh`, `test_results_artifact.sh`, `test_adr_checks.sh`, `test_render_adr_index.sh`, `test_convention_extraction.sh`: anchored content stays in place. **No edit expected** — but run them; if one fails, treat it as a review prompt (restore in skill unless the content genuinely moved to the reference).

**RE-POINT to the reference (`skills/docket-convention/references/terminal-close-out.md`, var `$TCO`) — because the kill-path rewire moves the literal script invocations OUT of the skill and INTO the reference (the #0053 Task-4 pattern):**
- `test_closeout.sh` — the NEWCHG/IMPL script-path asserts:
  - `wiring(new-change): proposed-kill invokes archive-change.sh` → grep `$TCO` for `/archive-change.sh`
  - `wiring(new-change): proposed-kill invokes terminal-publish.sh` → grep `$TCO` for `/terminal-publish.sh`
  - `wiring(implement-next): reconcile-kill invokes archive-change.sh` → grep `$TCO` for `/archive-change.sh`
  - `wiring(implement-next): reconcile-kill invokes cleanup-feature-branch.sh` → grep `$TCO` for `/cleanup-feature-branch.sh`
  - `wiring(implement-next): reconcile-kill invokes terminal-publish.sh` → grep `$TCO` for `/terminal-publish.sh`
  Rename each label from `wiring(new-change|implement-next)` to `wiring(close-out ref)` to reflect the moved home. The reference already asserts these paths for the sweep (lines ~380-385), so the re-pointed asserts will pass against the reference. **Keep any caller-POSTURE assert on the skill file (do not move posture).**
- `test_docket_metadata_branch.sh` K3/K4 (`the integration branch), performing the archive move` in new-change + implement-next `main`-mode prose): the clause moves to the reference's `main`-mode section ("the step-1 archive commit is itself the terminal record"). Re-point K3/K4 to the reference's phrasing AND add caller-pointer asserts (`grep -qF "terminal-close-out.md"` on both new-change and implement-next). Verify the exact current K3/K4 assertion text before editing; preserve intent.

**Re-point rules (LEARNINGS #36/#37):** never relocate a must-stay phrase just to pass; re-point ONLY where the guarded content genuinely moved to the reference; never weaken an assertion to a vacuous grep; a must-stay phrase keeps its grammatical location.

---

### Task 1: Add the no-diff-is-success clarifier to `references/terminal-close-out.md`

**Files:**
- Modify: `skills/docket-convention/references/terminal-close-out.md`
- Test: `tests/test_closeout.sh` (must stay green)

**Interfaces:**
- Consumes: nothing.
- Produces: a one-line clarifier in the reference's step-2 (re-render) description stating that a no-diff re-render is success — commit only when the `## Artifacts` block actually changed; an unchanged block (nothing to re-point) is NOT a failure and proceeds to publish (the skip-publish guard fires only on a *failed* step-2 commit/push, never on an empty diff).

- [ ] **Step 1: Read** `skills/docket-convention/references/terminal-close-out.md` fully — locate the step-2 (re-render `## Artifacts`) description and the skip-publish guard wording.
- [ ] **Step 2: Add the clarifier** as a single sentence at the step-2 description (or immediately after the skip-publish-guard sentence): e.g. "A **no-diff re-render is success**: commit the block only when it actually changed; an unchanged block (nothing to re-point) is not a failure and proceeds to publish — the skip-publish guard fires on a *failed* commit/push, never on an empty diff." Do not alter ordering, posture table, or any other step.
- [ ] **Step 3: Verify** `bash tests/test_closeout.sh` → zero `NOT OK` (the reference's existing sweep asserts still pass; nothing removed).
- [ ] **Step 4: Commit**

```bash
git add skills/docket-convention/references/terminal-close-out.md
git commit -m "docs(0055): terminal-close-out — clarify a no-diff re-render is success"
```

---

### Task 2: Slim `docket-implement-next` (field-write rule + Step-0 pointer + reconcile-kill rewire)

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md`
- (Do NOT edit tests in this task — Task 4 co-evolves them.)

**Interfaces:**
- Consumes: `references/terminal-close-out.md`, the convention's `### Step-0 preamble (every operating skill)`.
- Produces: implement-next at ≤ ~100 lines, every finalize... (implement-next) sentinel in Global Constraints preserved.

- [ ] **Step 1: Read** the current `skills/docket-implement-next/SKILL.md` (137 L) fully, the reference, and the convention's Step-0 preamble section. Read this plan's Global Constraints "Sentinel-preservation" list — it binds this task.
- [ ] **Step 2: Rewrite** per spec §1:
  - **Step 0** → ~4-line pointer: "run the convention's *Step-0 preamble*" KEEPING the literal `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"`, act on `BOOTSTRAP`, writes land on `origin/docket`. STAYS: the unconditional foreground `docket-status` dispatch before selection, its git-state-not-in-context-return contract, and the wrapper-resolved-tier phrasing (NO model/effort literals).
  - **Field-write rule** (NEW, inside *Branch & metadata discipline*): state once — every change-file field write (claim, reconcile, `status:`, `plan:`, `adrs:`, `pr:`, `results:`) is a metadata commit in the metadata working tree on `metadata_branch`, never in the feature worktree, pushed immediately; a write to a **link-bearing field** (`spec:`/`plan:`/`adrs:`/`pr:`/`results:`) additionally regenerates the `## Artifacts` block IN THE SAME COMMIT via the full `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-change-links.sh --change-file … --adrs-dir …` invocation (sole writer of the block). Scope note: a claim writes `status:`/`branch:` (metadata discipline applies; Artifacts regen does NOT). This is the ONE surviving full `render-change-links.sh` invocation (satisfies `test_change_links_coverage.sh`).
  - **Steps 4 / 6 / 7** field-write sites shrink to "…per the **field-write rule**" one-liners (drop the repeated render-change-links invocations at those 3 sites).
  - **Step 3 reconcile-kill** → rewire to `references/terminal-close-out.md` with `--outcome killed` (same shape as #0053's sweep rewire): the reference owns invocations, ordering, `main`-mode degradation; the skill keeps CALLER POSTURE only — trust each exit code, a failure aborts the kill and is surfaced, loop back to Step 1 with a best-effort board refresh. The rewire summary line MUST name `terminal-publish` (word) so `test_docket_metadata_branch.sh` H passes. The OBSOLETE vs FUNDAMENTALLY-INVALIDATED escape-hatch distinction STAYS verbatim.
  - **STAYS verbatim/in-meaning:** Step 1 selection, Step 2 CAS loop, Step 4 SHA-compare push confirmation + cross-tree spec-read explanation, the reconcile-pass + `reconciled`-flag section (trim narration only — flag semantics + resume-safety rule are load-bearing), `## Best-effort board refresh` (heading + ≥3 `run the Board pass (best-effort`), all four `SKILL_*` role refs, ≥2 `LEARNINGS.md` mentions, results-close-out section. Cut provenance narration; keep bare `(ADR-NNNN)` + cross-skill rationale one-liners. Remove any `v1 rough edge` text.
- [ ] **Step 3: Sentinel self-check** — every line must print ok:

```bash
F=skills/docket-implement-next/SKILL.md
grep -qF '/docket-config.sh' "$F" && echo ok-config
grep -qF '/render-change-links.sh' "$F" && echo ok-renderer
grep -qF '## Best-effort board refresh' "$F" && echo ok-board-heading
[ "$(grep -c 'run the Board pass (best-effort' "$F")" -ge 3 ] && echo ok-board-count
grep -qi 'kill' "$F" && grep -qiE 'terminal.publish|terminal-publish' "$F" && echo ok-kill-wiring
grep -qF 'terminal-close-out.md' "$F" && echo ok-closeout-ref
grep -q 'integration_branch' "$F" && grep -qi 'metadata working tree' "$F" && echo ok-vocab
grep -qF 'SKILL_PLAN' "$F" && grep -qF 'SKILL_BUILD' "$F" && grep -qF 'SKILL_REVIEW' "$F" && grep -qF 'SKILL_FINISH' "$F" && echo ok-roles
[ "$(grep -c 'LEARNINGS.md' "$F")" -ge 2 ] && echo ok-learnings
! grep -qiE '\b(opus|sonnet|haiku|fable)\b' "$F" && ! grep -qiE '\bxhigh\b' "$F" && echo ok-no-tiers
grep -Eqi 'model/effort its wrapper resolves|its wrapper resolves' "$F" && echo ok-wrapper
! grep -qi 'v1 rough edge' "$F" && echo ok-no-v1
! grep -qE 'scripts/[a-z][a-z0-9-]*\.sh' "$F" && echo ok-no-bare
wc -l "$F"
```
Missing ok ⇒ restore that invariant. `wc -l` ≤ ~100 (a little over OK only if cutting drops an invariant).
- [ ] **Step 4: Commit** `git add "$F" && git commit -m "refactor(0055): slim docket-implement-next — field-write rule, Step-0 pointer, reconcile-kill rewire"`.

---

### Task 3: Slim the four small skills

**Files:**
- Modify: `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-adr/SKILL.md`, `skills/docket-auto-groom/SKILL.md`
- (Do NOT edit tests.)

**Interfaces:**
- Consumes: the reference, the convention's Step-0 preamble.
- Produces: the four skills at ~55/~65/~78/~58 lines, sentinels preserved.

- [ ] **Step 1: Read** each of the four skills + the reference. Per spec §2:
  - **docket-new-change** (70→~55): Step-0 → preamble pointer (KEEP literal `/docket-config.sh`); proposed-kill → close-out reference, KEEPING: the two-kill-origins framing (one line), the nothing-to-clean-up note (a `proposed` change never had a branch — the reference's step 4 removes nothing), and the **must-land Board pass** posture (literal `must-land Board pass` — stricter than the reference's best-effort default, stated explicitly). The rewire summary names `terminal-publish` (word) for `test_docket_metadata_branch.sh` H. Keep `origin/<integration_branch>`.
  - **docket-groom-next** (77→~65): "Where everything is read and written" → preamble pointer; selection bands, recap contract, `LEARNINGS.md`, `SKILL_BRAINSTORM` all STAY (test-anchored).
  - **docket-adr** (88→~78): "Where ADRs are read and written" → preamble pointer; publish wiring UNTOUCHED; no model literals.
  - **docket-auto-groom** (64→~58): "Where everything is read and written" → preamble pointer; critic dispatch UNTOUCHED.
  Each keeps its single full `render-change-links.sh` statement if it has one; keep `## Convention (load first — blocking)` verbatim; keep the literal `/docket-config.sh`; cut provenance narration.
- [ ] **Step 2: Per-skill self-check:**

```bash
for F in skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md skills/docket-adr/SKILL.md skills/docket-auto-groom/SKILL.md; do
  echo "== $F =="
  grep -qF '/docket-config.sh' "$F" && echo ok-config || echo MISSING-config
  grep -qi 'metadata working tree' "$F" && grep -q 'integration_branch' "$F" && echo ok-vocab || echo MISSING-vocab
  grep -qF '## Convention (load first — blocking)' "$F" && echo ok-conv
  ! grep -qE 'scripts/[a-z][a-z0-9-]*\.sh' "$F" && echo ok-no-bare || echo BARE-SCRIPT
  ! grep -qiE '\b(opus|sonnet|haiku|fable)\b|\bxhigh\b' "$F" && echo ok-no-tiers || echo TIER-LEAK
  wc -l "$F"
done
# new-change specifics:
NF=skills/docket-new-change/SKILL.md
grep -qF 'must-land Board pass' "$NF" && echo ok-mustland
grep -qi 'kill' "$NF" && grep -qiE 'terminal.publish|terminal-publish' "$NF" && echo ok-newchg-kill
grep -qF 'terminal-close-out.md' "$NF" && echo ok-newchg-ref
grep -qF 'origin/<integration_branch>' "$NF" && echo ok-newchg-origin
# groom-next specifics:
GF=skills/docket-groom-next/SKILL.md
grep -qF 'SKILL_BRAINSTORM' "$GF" && grep -qF 'LEARNINGS.md' "$GF" && echo ok-groom
```
Fix any MISSING/leak before committing.
- [ ] **Step 3: Commit** `git add skills/docket-new-change skills/docket-groom-next skills/docket-adr skills/docket-auto-groom && git commit -m "refactor(0055): Step-0 preamble compression for the four small skills + new-change kill rewire"`.

---

### Task 4: Re-point moved sentinels + behavior-neutrality + full suite

**Files:**
- Modify (only the moved asserts): `tests/test_closeout.sh`, `tests/test_docket_metadata_branch.sh`

**Interfaces:**
- Consumes: the slimmed skills + reference.
- Produces: all tests green with each re-pointed sentinel preserving intent.

- [ ] **Step 1: Run the affected tests** and collect failures:

```bash
for t in test_closeout test_docket_metadata_branch test_change_links_coverage test_board_refresh_on_transition test_docket_config test_composition_wiring test_learnings_ledger test_groom_recap test_auto_groom test_results_artifact test_adr_checks test_render_adr_index test_convention_extraction; do echo "== $t =="; bash tests/$t.sh 2>&1 | grep "NOT OK"; done
```
- [ ] **Step 2: Re-point ONLY the moved asserts** per Global Constraints "RE-POINT" list:
  - `tests/test_closeout.sh`: change the 5 NEWCHG/IMPL kill script-path asserts to grep `$TCO` (the reference) instead of `$NEWCHG`/`$IMPL`, relabel to `wiring(close-out ref)`. Keep any caller-posture assert on the skill file.
  - `tests/test_docket_metadata_branch.sh`: re-point K3/K4 to the reference's `main`-mode phrasing; ADD `grep -qF "terminal-close-out.md"` caller-pointer asserts on new-change + implement-next. (Verify the exact current K3/K4 text first.)
  - For any OTHER failure: decide restore-in-skill (invariant didn't move — go back to Task 2/3) vs re-point (moved to reference). Record each decision.
- [ ] **Step 3: Behavior-neutrality diff review.** `git diff origin/main -- skills/` — every deleted sentence is (a) narration, (b) restated inline, (c) in the reference, or (d) covered by a script contract, WITH decision 3's kill re-render as the single recorded exception. **Anchor grep-gate:** no skill references a convention heading or reference file that does not exist on the base:

```bash
grep -rnoE 'references/[a-z-]+\.md|### Step-0 preamble' skills/docket-implement-next skills/docket-new-change skills/docket-groom-next skills/docket-adr skills/docket-auto-groom | sort -u
```
Confirm each cited target exists.
- [ ] **Step 4: Full suite (ONE foreground call, timeout 600000):**

```bash
for t in tests/test_*.sh; do out="$(bash "$t" 2>&1)"; n=$(grep -c "^NOT OK" <<<"$out"); [ "$n" -gt 0 ] && echo "FAIL $(basename "$t") ($n)"; done; echo "suite done"
```
Zero `FAIL`. A failure outside the anticipated set = a dropped invariant (LEARNINGS #52) — fix in the skill or re-point with justification.
- [ ] **Step 5: Record sizes** `wc -l skills/docket-implement-next/SKILL.md skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md skills/docket-adr/SKILL.md skills/docket-auto-groom/SKILL.md` for the PR body.
- [ ] **Step 6: Commit** `git add tests/ && git commit -m "test(0055): re-point kill-path sentinels to the close-out reference (intent-preserving)"`.

---

## Notes for the implementer

- **Documentation change** — no runtime smoke beyond the sentinel tests and a read-only Step-0 smoke (`eval "$(…/docket-config.sh --export)"` → `BOOTSTRAP` → metadata-tree sync) of one slimmed skill against this clone (spec verification step 4).
- **The full suite is the gate** — the #0054 build proved the anticipated sentinel list can miss a regression; run every test.
- **The one allowed behavior delta** is the kill-path step-2 re-render + the reference clarifier. Every other deleted sentence must have a home. If a compression would change meaning, keep the longer phrasing.
- Do NOT touch `docket-convention`/`docket-status`/`docket-finalize-change` beyond the single reference clarifier line; no scripts, no `description:` frontmatter, no agent wrappers.
