# Auto-groom critic re-check foreground — never-yield rule Implementation Plan

> **Provenance / degrade notice:** This plan was authored **directly by `docket-implement-next`** running the convention's *Skill layer* **`auto` fallback** — the configured plan skill (`superpowers:writing-plans`) was not available as an invocable skill in this session. The plan is the artifact; method is the agent's choice. Same degrade applies to build (`superpowers:subagent-driven-development`) and review (`superpowers:requesting-code-review`) — the executed plan and a whole-branch review are the respective artifacts, and the degrade is flagged in the PR body per the Missing-skill rule.

**Goal:** Close the two prose defects that let a forked `docket-auto-groom` background its critic re-check and *yield*, returning a half-done run that read as `completed`. Three product-code edits on the feature branch — (1) qualify auto-groom §3's re-check foreground, (2) state the general **never-yield** rule + reciprocal caller-side reading in `docket-convention`'s *Composition* paragraph, (3) two positive-anchor guard sentinels in `tests/test_composition_wiring.sh` — plus one metadata-branch edit handled outside this feature branch: a dated `## Update` note on ADR-0024 (`docs/adrs/0024`, carried to `main` via the change's `adrs: [24]` at close-out).

**Architecture:** Wording-only. No script, schema, dispatch *mechanic*, verdict vocabulary, or one-round bound changes (all explicit non-goals in the spec/stub). The never-yield rule lives ONCE at the contract source (the convention's *Composition* paragraph every wrapper loads via `skills:`), so it binds auto-groom's re-check, both single-shot dispatchers, and any future multi-round dispatch with no per-skill duplication (spec D1). The auto-groom §3 edit is the one concrete under-qualified round; it points back at the convention rule rather than restating it.

**Tech Stack:** Markdown (two `SKILL.md` files), bash sentinel test (`tests/test_composition_wiring.sh`). The ADR-0024 `## Update` note is Markdown metadata on the `docket` branch — NOT a feature-branch file (metadata discipline: the feature branch never modifies ADRs).

## Global Constraints

- **Wording only.** The critic gate, its verdict vocabulary, the one-bounded-revision-round rule, `context: fork`, dispatch mechanics, and `sync-agents.sh`/wrapper generation are all out of scope. Touch only the prose that governs how a dispatch is *awaited*.
- **Positive-anchor sentinels, sampling not parsing** (LEARNINGS #5/#13): anchor each new test on the *meaningful framing*, and make it **non-vacuous** — deleting the guarded sentence must flip the test red. A bare count of "foreground" is rejected (the initial dispatch already contains one); pin the *second* qualifier by its distinctive phrase.
- **Run the WHOLE suite as the gate** (LEARNINGS #52/#54/#42): one foreground run of `tests/run_all.sh` (or the repo's runner), `timeout 600000`, never backgrounded. The two new sentinels are a floor, not the ceiling — a regression in an out-of-goal test is exactly what the rest of the suite exists to catch.
- **No cross-cutting literal-count edits** (LEARNINGS #56): the convention edit ADDS sentences to an existing paragraph; it changes no enumerated count (wrappers 9/no-skill 4, states 7, the 4/3 fork split). Confirm no count moved before committing.
- **Metadata/code split** (convention *Branch & metadata discipline*): the two `SKILL.md` edits + the test are product code on `feat/auto-groom-critic-recheck-foreground`. The ADR-0024 `## Update` note is a metadata commit on `docket` (via the `.docket/` worktree), carried to `main` by `adrs: [24]` at terminal close-out — it is NOT committed on the feature branch and is executed in Step 6 (ADRs), not here.

---

### Task 1: Guard sentinels first (TDD red) — `tests/test_composition_wiring.sh`

**Files:**
- Modify: `tests/test_composition_wiring.sh`

**Interfaces:**
- Consumes: `$CONV` (`skills/docket-convention/SKILL.md`), a new `$AUTOGROOM` (`skills/docket-auto-groom/SKILL.md`).
- Produces: two `assert` sentinels that are RED now (the guarded prose does not exist yet) and turn green after Tasks 2–3.

- [ ] **Step 1:** Add `AUTOGROOM="$REPO/skills/docket-auto-groom/SKILL.md"` alongside the existing `IMPL`/`CONV` path vars.
- [ ] **Step 2:** Add sentinel — *convention states the never-yield rule*. Anchor on the meaningful framing, e.g. the paragraph both forbids backgrounding-and-yielding to await a notification AND names the notification channel: `grep -qi "never .*background" "$CONV"` **and** `grep -qi "task-notification" "$CONV"` combined so deleting the sentence flips it red. Prefer a single `assert` whose expression ANDs the two greps (`grep -qi "never" ... && grep -qi "yield" ... && grep -qi "task-notification" ...`) to pin the *never-yield* framing specifically, not any stray "notification".
- [ ] **Step 3:** Add sentinel — *auto-groom's re-check is foreground*. Anchor on the distinctive phrase committed in Task 2's §3 edit (`re-check is dispatched foreground` / `foreground exactly like the first pass`), NOT a bare "foreground" count. e.g. `grep -qi "re-check is dispatched foreground" "$AUTOGROOM"`.
- [ ] **Step 4 (RED):** Run `bash tests/test_composition_wiring.sh`; confirm the two NEW sentinels report `NOT OK` (prose absent) while every pre-existing assert stays `ok`. This proves the sentinels are non-vacuous.

### Task 2: Close the wording gap — `skills/docket-auto-groom/SKILL.md` §3

**Files:**
- Modify: `skills/docket-auto-groom/SKILL.md` (§3 Critic pass, the revision-round clause, ~line 44)

**Interfaces:**
- Consumes: nothing.
- Produces: the re-check clause explicitly foreground, pointing at the convention's never-yield rule.

- [ ] **Step 1:** In the revision-round parenthetical `(designer revises; ONE bounded revision round; the critic re-checks only the revised items)`, extend the last clause to: `the critic re-checks only the revised items — this re-check is dispatched foreground exactly like the first pass: the designer blocks on the critic's return and never backgrounds it to await a notification, per the convention's *Composition* never-yield rule`.
- [ ] **Step 2:** Confirm the gate/verdict vocabulary and the "ONE bounded revision round" bound are otherwise byte-unchanged. Wording addition only.
- [ ] **Step 3:** Re-run `bash tests/test_composition_wiring.sh`; the auto-groom re-check sentinel now flips to `ok`; the convention sentinel is still `NOT OK` (Task 3 pending).

### Task 3: State the never-yield rule at the contract source — `skills/docket-convention/SKILL.md` *Composition*

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the *Composition (change 0017)* paragraph)

**Interfaces:**
- Consumes: nothing.
- Produces: the general never-yield rule + reciprocal caller-side reading appended to the existing *Composition* paragraph.

- [ ] **Step 1:** After the paragraph's existing "…re-read after a re-sync — never an in-context return." add: **Foreground means the parent *actively blocks* on the child's return — it may never background a dispatched or forked child and *yield* to await a task-notification.** A forked/subagent skill has no channel to receive one (the same no-channel fact ADR-0024 states for the human), so "wait for the notification" hands control back to the caller and returns a **half-done run that the caller reads as `completed`**. Reciprocally, a caller must **not** read a bare `completed` as proof the child finished: it verifies the child's git-state transition (re-read after a re-sync, above) and **never adopts or commits a child's uncommitted working-tree files** on its behalf.
- [ ] **Step 2:** Verify no enumerated count changed (LEARNINGS #56) and no model/effort tier literal was introduced (the suite's `pins no literal model/effort tier` guard). The addition names ADR-0024 and `task-notification`; it introduces no `opus/xhigh`-style token.
- [ ] **Step 3:** Re-run `bash tests/test_composition_wiring.sh`; both new sentinels now `ok`, all pre-existing asserts still `ok` (GREEN).

### Task 4: Whole-suite gate

**Files:** none (verification only).

- [ ] **Step 1:** Run the FULL test suite once, foreground, `timeout 600000` — `bash tests/run_all.sh` (or the repo's runner; discover it first). Zero failures required.
- [ ] **Step 2:** If any out-of-goal test reddens, treat it as a review prompt (LEARNINGS #54): the sentinel list is a floor. Fix the prose, not the test, unless the test genuinely guards moved content.

---

## Out of this plan (handled elsewhere)

- **ADR-0024 `## Update` note** — a metadata commit on `docket` (`.docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md`), extending its "no channel to the human" to the task-notification corollary. Authored in Step 6 (ADRs) of `docket-implement-next`, NOT on this feature branch; `adrs: [24]` carries it to `main` at close-out. The ADR index (`docs/adrs/README.md`) regenerates via the script — note-only, so expected no-op.
- **Change frontmatter** (`adrs: [24]` already set; `plan:`, `pr:`, `status:` writes) — metadata commits on `docket` per the field-write rule.
