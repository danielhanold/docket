# Claude Code `context: fork` dispatch parity — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a directly-invoked Claude Code docket skill honor its wrapper's model/effort pin by adding native `context: fork` + `agent: docket-<name>` frontmatter to the four headless-safe autonomous skills, plus correct the stale "only cursor" assumption in code comment and README.

**Architecture:** Claude Code runs a directly-invoked skill inline at the session model, defeating the pin the generated wrapper carries. Claude Code's native fix is per-skill `context: fork` + `agent:` frontmatter, which forks the invocation into the existing pinned wrapper subagent. The frontmatter is committed once in each shared `SKILL.md` (symlinked into every harness by `link-skills.sh`) and is inert in Cursor/Codex/others (unknown YAML keys are ignored). Only human-non-interactive skills are forked, because a forked subagent has no channel to the human. No new generation, hooks, or routing — a minimal parity fix mirroring the Cursor dispatch-rule mechanism.

**Tech Stack:** Bash test scripts (repo convention: standalone `tests/test_*.sh`, `assert` helper, PASS/FAIL + exit code), markdown skill files with YAML frontmatter.

## Global Constraints

- Fork exactly these **4** skills: `docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom`. Each gets `context: fork` and `agent: docket-<its-own-name>`.
- Do **not** fork these **3**: `docket-finalize-change`, `docket-new-change`, `docket-groom-next` — they must carry neither `context:` nor `agent:` frontmatter.
- Frontmatter values contain no `": "` (colon-space) — safe as unquoted YAML scalars (per LEARNINGS 2026-06-10).
- Skill files live at `skills/<name>/SKILL.md`, committed on the integration branch — the test reads them directly (no metadata-branch gotcha).
- Tests are run individually with `bash tests/test_<name>.sh`; there is no central runner and no CI workflow. Each test is self-contained, prints `PASS`/`FAIL`, and exits nonzero on any failure.
- `HARNESS_HAS_DISPATCH_RULES` stays **cursor-only** — Claude Code does not get a generated dispatch *rule*; it uses the frontmatter mechanism instead.

---

### Task 1: Fork-invariant test + `context: fork` frontmatter on the 4 skills

**Files:**
- Create: `tests/test_skill_fork_dispatch.sh`
- Modify: `skills/docket-status/SKILL.md` (frontmatter block, lines 1-4)
- Modify: `skills/docket-adr/SKILL.md` (frontmatter block, lines 1-4)
- Modify: `skills/docket-implement-next/SKILL.md` (frontmatter block, lines 1-4)
- Modify: `skills/docket-auto-groom/SKILL.md` (frontmatter block, lines 1-4)

**Interfaces:**
- Consumes: nothing.
- Produces: the fork-dispatch invariant asserted by `tests/test_skill_fork_dispatch.sh`; the four forked `SKILL.md` files now carry `context: fork` + `agent: docket-<name>`.

- [ ] **Step 1: Write the failing test**

Create `tests/test_skill_fork_dispatch.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_skill_fork_dispatch.sh — change 0061.
# Asserts the fork-dispatch invariant: the four headless-safe autonomous skills carry
# `context: fork` + a matching `agent: docket-<name>` in their SKILL.md frontmatter (so a
# direct Claude Code invocation forks into the pinned wrapper), and the three interactive/
# excluded skills carry neither. Run: bash tests/test_skill_fork_dispatch.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILLS="$REPO/skills"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Print the YAML frontmatter block (between the first two --- fences) of a markdown file.
frontmatter(){ awk 'NR==1 && $0=="---"{f=1;next} f && $0=="---"{exit} f{print}' "$1"; }
# Extract a single-line frontmatter scalar value ("" if absent). Scoped to the frontmatter
# block so a body line like "agent:" can never satisfy the assertion.
fmval(){ frontmatter "$1" | sed -n "s/^$2:[[:space:]]*//p" | head -n1 | sed 's/[[:space:]]*$//'; }

# The four headless-safe autonomous skills that MUST fork into their pinned wrapper.
FORKED="docket-status docket-adr docket-implement-next docket-auto-groom"
# The three interactive/excluded skills that MUST NOT fork (no channel to the human, or a
# merge blocked by the auto-mode classifier — see change 0062).
EXCLUDED="docket-finalize-change docket-new-change docket-groom-next"

for s in $FORKED; do
  f="$SKILLS/$s/SKILL.md"
  assert "$s: SKILL.md exists" '[ -f "$f" ]'
  assert "$s: context is fork" '[ "$(fmval "$f" context)" = "fork" ]'
  assert "$s: agent routes to its own wrapper" '[ "$(fmval "$f" agent)" = "$s" ]'
done

for s in $EXCLUDED; do
  f="$SKILLS/$s/SKILL.md"
  assert "$s: SKILL.md exists" '[ -f "$f" ]'
  assert "$s: no context field (not forked)" '[ -z "$(fmval "$f" context)" ]'
  assert "$s: no agent field (not forked)" '[ -z "$(fmval "$f" agent)" ]'
done

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_skill_fork_dispatch.sh`
Expected: **FAIL** — the eight `context`/`agent` assertions for the four FORKED skills print `NOT OK` (their frontmatter lacks the fields); the EXCLUDED-skill assertions already print `ok`. Final line `FAIL`, exit 1. (This RED run proves the positive assertions are non-vacuous — they flip to `ok` only once the frontmatter is added in Step 3.)

- [ ] **Step 3: Add the frontmatter to the four forked skills**

In each file below, insert the two lines `context: fork` and `agent: <name>` immediately before the closing `---` of the frontmatter block (after the `description:` line).

`skills/docket-status/SKILL.md` — change:

```yaml
---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by regenerating the BOARD.md board, sweeping merged changes to done, or running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
---
```

to:

```yaml
---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by regenerating the BOARD.md board, sweeping merged changes to done, or running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
context: fork
agent: docket-status
---
```

`skills/docket-adr/SKILL.md` — add the same two lines (`context: fork` / `agent: docket-adr`) before the closing `---`:

```yaml
context: fork
agent: docket-adr
```

`skills/docket-implement-next/SKILL.md` — add before the closing `---`:

```yaml
context: fork
agent: docket-implement-next
```

`skills/docket-auto-groom/SKILL.md` — add before the closing `---`:

```yaml
context: fork
agent: docket-auto-groom
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_skill_fork_dispatch.sh`
Expected: **PASS** — every assertion prints `ok`, final line `PASS`, exit 0.

- [ ] **Step 5: Run the neighboring suite to confirm no regression from the new frontmatter**

The new `context:`/`agent:` keys must not break any test that reads skill frontmatter or the skill/wrapper wiring.

Run:
```bash
for t in test_sync_agents test_composition_wiring test_convention_extraction test_link_skills; do
  echo "== $t =="; bash "tests/$t.sh" | tail -1
done
```
Expected: each prints `PASS`. If any prints `FAIL`, run that test verbatim, read the failing `NOT OK` line, and fix the cause (do **not** weaken the test).

- [ ] **Step 6: Commit**

```bash
git add tests/test_skill_fork_dispatch.sh \
        skills/docket-status/SKILL.md skills/docket-adr/SKILL.md \
        skills/docket-implement-next/SKILL.md skills/docket-auto-groom/SKILL.md
git commit -m "feat(0061): fork the 4 headless-safe skills into their pinned wrappers via context: fork"
```

---

### Task 2: Correct the stale "only cursor" comment in `sync-agents.sh`

**Files:**
- Modify: `sync-agents.sh:212` (the comment above `HARNESS_HAS_DISPATCH_RULES`)

**Interfaces:**
- Consumes: nothing.
- Produces: an accurate comment; no behavior change (`HARNESS_HAS_DISPATCH_RULES` stays cursor-only).

- [ ] **Step 1: Replace the comment**

Change line 212 from:

```bash
# Harnesses that get a generated Cursor-style dispatch rule (only cursor exhibits the inline quirk).
```

to:

```bash
# Harnesses that get a generated Cursor-style dispatch rule. Both Cursor and Claude Code exhibit
# the inline quirk (a directly-invoked skill runs at the session model, defeating the wrapper's
# model/effort pin), but they fix it differently: Cursor needs this generated alwaysApply dispatch
# rule, while Claude Code uses native per-skill `context: fork` frontmatter (see skills/docket-*/
# SKILL.md). So only Cursor belongs in this list.
```

- [ ] **Step 2: Verify the edit — new text present, stale text gone**

Run:
```bash
grep -q "Both Cursor and Claude Code exhibit" sync-agents.sh && echo "new-ok"
grep -q "only cursor exhibits the inline quirk" sync-agents.sh && echo "STALE-STILL-PRESENT" || echo "stale-gone"
```
Expected: `new-ok` then `stale-gone`.

- [ ] **Step 3: Confirm the script still parses (comment-only change)**

Run: `bash -n sync-agents.sh && echo "syntax-ok"`
Expected: `syntax-ok`.

- [ ] **Step 4: Commit**

```bash
git add sync-agents.sh
git commit -m "docs(0061): correct sync-agents.sh comment — Claude Code exhibits the inline quirk too"
```

---

### Task 3: README two-mechanism story + fork-exclusion principle

**Files:**
- Modify: `README.md` (harness-dispatch section — insert a paragraph after the "Always the full set, plus a Cursor dispatch rule." paragraph, before "The clone-identical guarantee is retired.")

**Interfaces:**
- Consumes: nothing.
- Produces: README documents both dispatch mechanisms and the fork-exclusion principle.

- [ ] **Step 1: Insert the new paragraph**

Find the paragraph ending with (unique anchor):

```
... `sync-agents.sh --check` covers both the generated agents and the dispatch rule.
```

Immediately after that paragraph (and before the paragraph beginning `**The clone-identical guarantee is retired.**`), insert a blank line then:

```markdown
**Two mechanisms for one inline quirk.** Both Cursor and Claude Code run a *directly-invoked* skill — a human typing `/docket-status`, or the model auto-invoking it — inline at the session model, which silently defeats the wrapper's model/effort pin. They fix it differently: Cursor uses the generated `docket-dispatch.mdc` rule above; **Claude Code uses native `context: fork` + `agent: docket-<name>` frontmatter** committed in each forked skill's `SKILL.md`, which forks the invocation into the same pinned wrapper. That frontmatter is inert in every other harness (unknown keys are ignored), so one shared `SKILL.md` serves all of them, and it degrades to today's inline behavior on a Claude Code too old to know the field. **Fork-exclusion principle:** only skills that never need the human mid-run are forked — a forked subagent has no channel to the human (Claude Code withholds `AskUserQuestion`, `EnterPlanMode`, and similar from subagents). So the four headless-safe autonomous skills — `docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom` — carry the frontmatter; the two interactive brainstorm skills (`docket-new-change`, `docket-groom-next`) and `docket-finalize-change` (whose headless merge is blocked by Claude Code's Merge-Without-Review classifier, a separate decision tracked as change 0062) do not.
```

- [ ] **Step 2: Verify the edit**

Run:
```bash
grep -q "Two mechanisms for one inline quirk" README.md && echo "para-ok"
grep -q "Fork-exclusion principle" README.md && echo "principle-ok"
```
Expected: `para-ok` then `principle-ok`.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(0061): document Claude context: fork dispatch + fork-exclusion principle"
```

---

## Self-Review

**1. Spec coverage:**
- Spec "Approach" (add `context: fork` + `agent:` to the 4 SKILL.md) → Task 1, Step 3. ✅
- Spec "Selection" table (4 forked, 3 not) → Task 1 test asserts both sides. ✅
- Spec "Correct the wrong assumption" → `sync-agents.sh` comment (Task 2) + README two-mechanism story & fork-exclusion principle (Task 3). ✅
- Spec "Testing" — structural test asserting 4 carry / 3 don't → Task 1. ✅
- Spec "Testing" — optional `sync-agents.sh --check` advisory leg → **deferred by design** (documented decision; see below), not implemented.
- Spec "Expected ADR" → recorded by the implementer at the review step (docket-adr), out of this plan's scope.

**2. Placeholder scan:** no TBD/TODO/"handle edge cases"/"similar to Task N" — all content is literal. ✅

**3. Type consistency:** the skill-name lists (`FORKED`/`EXCLUDED`) in the test match the Global Constraints and the per-file edits exactly; `agent:` value equals the skill's own `name` in every case. ✅

## Deferred decision (open question #1 in the change)

The change asked whether `sync-agents.sh --check` should also enforce the fork invariant. **Deferred.** `--check` cannot derive "should be forked" purely from "is autonomous-wrapped": `docket-finalize-change` is autonomous-wrapped yet deliberately *not* forked, so a check leg would need an explicit fork-allowlist — more standing machinery than this minimal parity fix warrants, and a second place for the 4/3 split to drift. The dedicated `tests/test_skill_fork_dispatch.sh` already guards the current invariant. This is recorded in the ADR the implementer writes at review.
