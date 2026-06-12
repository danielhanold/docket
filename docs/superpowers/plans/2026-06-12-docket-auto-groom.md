# docket-auto-groom Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the `docket-auto-groom` skill — an autonomous drain that grooms auto-groomable needs-brainstorm stubs to build-ready (spec or trivial) or abstains to the human queue — plus the convention vocabulary, sibling-skill amendments, and tests.

**Architecture:** docket is a skill suite: each deliverable is a markdown `SKILL.md` under `skills/`, guarded by bash sentinel tests under `tests/`. This change adds one new skill directory, amends four existing skills (`docket-convention`, `docket-groom-next`, `docket-new-change` + its template, `docket-status`), and updates the README skill table. Spec: `docs/superpowers/specs/2026-06-12-docket-auto-groom-design.md` on the `docket` branch (read it from `.docket/` if more context is needed).

**Tech Stack:** Markdown skills, bash tests (`set -uo pipefail`, `assert` helper, fixed-string greps + `grep -n` order assertions). `link-skills.sh` globs `skills/*/` — it needs NO edit for a new skill (learning 2026-06-12 #12).

**Key learnings to honor (from `LEARNINGS.md`):** assert order with `grep -n` line-number compares, not just presence; prove assertions non-vacuous (deleting the guarded clause must flip the test); sentinel greps are sampling — the whole-branch review reads for meaning; repo tests can only see the integration branch (never assert metadata-branch files exist).

---

### Task 1: Convention vocabulary — knob, field, autonomous-grooming definitions

**Files:**
- Modify: `skills/docket-convention/SKILL.md`
- Create: `tests/test_auto_groom.sh`

- [ ] **Step 1: Write the failing test**

Create `tests/test_auto_groom.sh`:

```bash
#!/usr/bin/env bash
# tests/test_auto_groom.sh — guards change 0014 (docket-auto-groom):
#   - convention defines the auto_groom knob, the tri-state auto_groomable field,
#     effective resolution, the autonomous-eligible queue, and the abstain rule
#   - the docket-auto-groom skill drains (loops), designer+critic gate every
#     build-ready exit, kill/defer are never autonomous, abstain flips the flag
#   - groom-next selection bands prefer stubs that need a human
#   - new-change can set the flag at create time; template documents it
#   - the board renders abstained stubs distinctly
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"

# --- convention: knob + field + shared definitions ---
assert "convention: .docket.yml example carries the auto_groom knob (default false)" \
  'grep -qE "^auto_groom: false" "$CONV"'
assert "convention: manifest carries tri-state auto_groomable" \
  'grep -qF "auto_groomable:" "$CONV"'
assert "convention: unset means inherit the repo default" \
  'grep -qF "unset ⇒ inherit" "$CONV"'
assert "convention: effective auto-groomable is defined" \
  'grep -qF "**effective auto-groomable**" "$CONV"'
assert "convention: autonomous-eligible queue is defined" \
  'grep -qF "**autonomous-eligible**" "$CONV"'
assert "convention: abstain rule is defined (flag flip + blocked section)" \
  'grep -qF "## Auto-groom blocked" "$CONV"'
assert "convention: body sections list the Auto-groom blocked section" \
  'grep -qF "\`## Auto-groom blocked\`" "$CONV"'
assert "convention: groom-next selection bands defined" \
  'grep -qF "selection bands" "$CONV"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_auto_groom.sh`
Expected: all `NOT OK` lines, final `FAIL`, exit 1.

- [ ] **Step 3: Edit `skills/docket-convention/SKILL.md` — four insertions**

**(a)** In the `.docket.yml` fenced example (the block listing `metadata_branch:` … `results_dir:`), add a final line:

```yaml
auto_groom: false            # repo default for autonomous grooming; per-change auto_groomable overrides
```

**(b)** In the change-manifest fenced YAML, insert directly under the `trivial:` line:

```yaml
auto_groomable:           # tri-state: unset ⇒ inherit the repo's auto_groom; true/false ⇒ explicit override
```

**(c)** In `### Change body sections`, add after the `## Reconcile log` bullet:

```markdown
- `## Auto-groom blocked` — dated entries appended by `docket-auto-groom` when it abstains: the decision(s) it could not default, what context is missing, and any recommendation. Its presence distinguishes "the agent tried and bailed" from "a human opted out" of auto-grooming.
```

**(d)** Add a new subsection immediately after `### Build-readiness & selection (shared definition)` (before `### Learnings ledger`):

```markdown
### Autonomous grooming (shared definition)

A change's **effective auto-groomable** value is its `auto_groomable:` override when explicitly set, else the repo's `auto_groom` knob (default `false`). The field is human input with one exception: `docket-auto-groom`'s abstain is the single agent write (it flips the override to `false`).

A stub is **autonomous-eligible** — selectable by `docket-auto-groom` — when it is needs-brainstorm (`proposed`, no `spec:`, not `trivial: true`) AND effective auto-groomable. Unsatisfied `depends_on` does NOT exclude it (the same design-ahead rule as interactive grooming; the implementer's reconcile re-validates at build time). Ranking is the same deterministic selection order as build-ready selection.

**Abstain rule.** When autonomous grooming cannot safely default a decision, it emits NO spec; it flips `auto_groomable: false` and appends a dated `## Auto-groom blocked` body section. The stub stays needs-brainstorm — out of the autonomous queue, still in the interactive one. Re-arm = a human supplies the missing context and flips the flag back to `true`. Kill and defer are never autonomous: they surface inside the blocked section as recommendations.

**Interactive selection bands.** `docket-groom-next` still sees every needs-brainstorm stub, but its default order prefers stubs that need a human: (1) abstained (`## Auto-groom blocked` present), (2) effective `auto_groomable: false`, (3) effective auto-groomable — flagged "docket-auto-groom will handle it unless you want it now." Within each band, the deterministic selection order applies. The board renders abstained stubs as **auto-groom blocked — needs you**, distinct from plain needs-brainstorm.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_auto_groom.sh`
Expected: all `ok`, final `PASS`, exit 0. Also run `bash tests/test_convention_extraction.sh` — must still PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/SKILL.md tests/test_auto_groom.sh
git commit -m "feat(0014): convention — auto_groom knob, auto_groomable field, autonomous-grooming definitions"
```

---

### Task 2: The `docket-auto-groom` skill

**Files:**
- Create: `skills/docket-auto-groom/SKILL.md`
- Modify: `tests/test_auto_groom.sh` (append assertions before the final `if` block)

- [ ] **Step 1: Append failing tests**

Append to `tests/test_auto_groom.sh` (immediately before the `if [ "$fail" = 0 ]` line):

```bash
AG="$REPO/skills/docket-auto-groom/SKILL.md"

# --- the skill itself ---
assert "auto-groom: skill file exists" '[ -f "$AG" ]'
assert "auto-groom: drains until the queue is empty" \
  'grep -qF "until no autonomous-eligible stub remains" "$AG"'
assert "auto-groom: loads the convention first" \
  'grep -qF "docket-convention" "$AG"'
assert "auto-groom: rejects the simulated-human auto-answerer" \
  'grep -qiF "not invoke \`superpowers:brainstorming\`" "$AG"'
assert "auto-groom: designer records an Assumptions block" \
  'grep -qF "## Assumptions" "$AG"'
assert "auto-groom: designer reads the learnings ledger" \
  'grep -qF "LEARNINGS.md" "$AG"'
assert "auto-groom: critic is a fresh subagent, not the designer" \
  'grep -qF "fresh subagent" "$AG"'
assert "auto-groom: critic gates trivial verdicts too" \
  'grep -qF "trivial verdicts alike" "$AG"'
assert "auto-groom: kill and defer are never autonomous" \
  'grep -qF "Kill and defer are NEVER autonomous" "$AG"'
assert "auto-groom: abstain flips the flag and appends the blocked section" \
  'grep -qF "auto_groomable: false" "$AG" && grep -qF "## Auto-groom blocked" "$AG"'
assert "auto-groom: takes no claim, cites ADR-0004" \
  'grep -qF "ADR-0004" "$AG"'
assert "auto-groom: never implements (markdown only)" \
  'grep -qF "never branches, worktrees, or code" "$AG"'

# order: designer pass precedes critic pass precedes exits
designer_line="$(grep -nF "### Step 2 — Designer pass" "$AG" | head -1 | cut -d: -f1)"
critic_line="$(grep -nF "### Step 3 — Critic pass" "$AG" | head -1 | cut -d: -f1)"
exit_line="$(grep -nF "### Step 4 — Exit" "$AG" | head -1 | cut -d: -f1)"
assert "auto-groom: designer → critic → exit, in that order" \
  '[ -n "$designer_line" ] && [ -n "$critic_line" ] && [ -n "$exit_line" ] && [ "$designer_line" -lt "$critic_line" ] && [ "$critic_line" -lt "$exit_line" ]'
```

- [ ] **Step 2: Run test to verify the new assertions fail**

Run: `bash tests/test_auto_groom.sh`
Expected: Task-1 assertions `ok`, all `auto-groom:` assertions `NOT OK`, final `FAIL`.

- [ ] **Step 3: Create `skills/docket-auto-groom/SKILL.md`**

Full content:

````markdown
---
name: docket-auto-groom
description: Use when a repo (or individual stubs) opted into autonomous grooming and you want the auto-groomable needs-brainstorm queue drained with no human — selecting each autonomous-eligible stub deterministically and designing it via a default-biased self-brainstorm gated by an adversarial critic, exiting each stub with a linked spec, a trivial verdict, or an abstain back to the human queue. Kill and defer are never autonomous. Writes markdown only — never branches, worktrees, or code.
---

# docket-auto-groom — the autonomous groomer (drain)

## Overview

`docket-auto-groom` is `docket-groom-next`'s autonomous sibling. Same queue vocabulary, same exits where safe — but no human, and **drain semantics**: nobody is waiting between stubs, so one invocation loops until no autonomous-eligible stub remains, then reports. It keeps superpowers' brainstorming *reasoning* — enumerate the decision points, weigh approaches, commit to the conservative default — and replaces the *waiting-for-a-human protocol* with an audit trail (the spec's `## Assumptions` block) plus an adversarial critic that gates every build-ready exit. It writes markdown only: change files, specs, `BOARD.md` — never branches, worktrees, or code.

## When to use

- The repo sets `auto_groom: true` (or stubs carry `auto_groomable: true`) and needs-brainstorm stubs are piling up.
- You want the backlog groomed to build-ready overnight / from a routine, with abstains waiting for you in the morning.
- Do NOT use for interactive design — that is `docket-groom-next`; the human there is the point.
- Do NOT use to capture new ideas (`docket-new-change` mints ids) or to re-groom a change that already has a spec (build-time reconcile owns drift).

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (needs-brainstorm, **effective auto-groomable**, **autonomous-eligible**, the abstain rule, metadata working tree, …) without redefinition; no step below is executable without the convention loaded.

## Where everything is read and written

All reads and writes happen in the **metadata working tree** on `metadata_branch`, pushed to its remote immediately. In `docket`-mode that tree is the persistent `.docket/` worktree parked on `docket` — ensure it (state-specific create per the convention's Branch model, idempotent) and **sync it to `origin/docket` before any read**. In single-branch/`main`-mode this degrades to the primary working tree on the integration branch. The steps below say "`.docket/`" / "`origin/docket`" for the common case; read those as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

## Procedure — the drain loop

Repeat steps 1–5 until no autonomous-eligible stub remains; then step 6.

### Step 1 — Select

Sync the metadata working tree. Rank every **autonomous-eligible** stub (per the convention: needs-brainstorm AND effective auto-groomable; unsatisfied `depends_on` does NOT exclude — design ahead, note the dependency state in the assumptions) by the deterministic selection order. Pick the top. None left → step 6.

### Step 2 — Designer pass

Read the stub body, its `related`/`depends_on` neighbours (active + recently archived), the ADR index, `<changes_dir>/LEARNINGS.md`, and the relevant code. Enumerate the decision points an interactive brainstorm would raise. For each, weigh 2–3 approaches and COMMIT to the conservative / recommended default — do NOT invoke `superpowers:brainstorming` with a simulated human answerer (a subagent picking "the recommended option" is the model agreeing with itself while faking an approval gate; rejected at design time). Draft the spec to `.docket/docs/superpowers/specs/<UTC date>-<slug>-design.md` with an `## Assumptions` block: every decision, the chosen default, the rejected alternatives, and why — the human's deferred audit trail. If the stub is genuinely mechanical (no real design questions), the draft verdict is *trivial* instead of a spec, with the reasoning written for the critic.

### Step 3 — Critic pass

Dispatch a **fresh subagent** (never the designer reviewing itself) to adversarially attack the draft — specs and trivial verdicts alike. Per assumption, one verdict: **sound** (stands) · **wrong but fixable from available context** (designer revises; ONE bounded revision round; the critic re-checks only the revised items) · **needs human context** (⇒ the whole groom abstains — a spec must only be emitted when every decision in it is safe to auto-commit, because emission = build-ready = the autonomous builder will build it).

### Step 4 — Exit (one of three)

1. **Spec** — every assumption survived: set `spec:`, refresh the body to the settled design (proposal altitude), resolve `## Open questions`, set `updated: <UTC today>`. Build-ready.
2. **Trivial** — the critic confirmed no hidden design decisions: set `trivial: true`, tighten the body, log the reasoning in the body, set `updated:`. Build-ready, no spec.
3. **Abstain** — any needs-human-context verdict: emit NO spec; flip `auto_groomable: false` and append a dated `## Auto-groom blocked` section (the undecidable decision(s), what context is missing, what a human should supply, and any recommendation — including "this should probably be killed/deferred because …"). The stub stays needs-brainstorm, first in `docket-groom-next`'s queue.

**Kill and defer are NEVER autonomous.** Verdict authority over the backlog's composition stays human; the strongest the drain may say is an abstain-with-recommendation.

### Step 5 — Commit, push, board

Commit the stub's outcome (change-file edit + spec when emitted) in the metadata working tree; push `origin/docket`. On a non-fast-forward rejection: `pull --rebase`, and if the rebase brought in commits touching this stub's file, RE-READ it — no longer autonomous-eligible (groomed, killed, claimed, or opted out elsewhere) ⇒ DISCARD this iteration's writes for it and loop. Then refresh `BOARD.md` via `docket-status`'s Board pass as a separate, must-land commit. Loop to step 1.

### Step 6 — Report

Summarize the drain: groomed N (specs), trivial M, abstained K — each abstain with its one-line reason — plus anything skipped to a lost race. STOP. Grooming never implements; the build-ready output is `docket-implement-next`'s queue.

## Termination & concurrency

Every exit shrinks the queue (spec/trivial ⇒ no longer needs-brainstorm; abstain ⇒ no longer effective auto-groomable), so the drain visits each stub at most once and provably terminates. No claim is taken — ADR-0004's final-push CAS stance, adopted for the autonomous case: its human-attended rationale does not apply here, but the load-bearing half does — each stub's writes land in a single final commit, so a late collision wastes minutes, not hours, and the post-rebase re-read is the arbiter.
````

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_auto_groom.sh`
Expected: all `ok`, `PASS`. Non-vacuous spot-check: temporarily delete the `### Step 3 — Critic pass` heading, re-run (expect order assertion `NOT OK`), restore.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-auto-groom/SKILL.md tests/test_auto_groom.sh
git commit -m "feat(0014): docket-auto-groom skill — autonomous grooming drain"
```

---

### Task 3: `docket-groom-next` — selection bands

**Files:**
- Modify: `skills/docket-groom-next/SKILL.md`
- Modify: `tests/test_auto_groom.sh` (append before the final `if`)

- [ ] **Step 1: Append failing tests**

```bash
GN="$REPO/skills/docket-groom-next/SKILL.md"

# --- groom-next: auto-groom-aware bands ---
assert "groom-next: selection bands present" \
  'grep -qF "selection bands" "$GN"'
assert "groom-next: abstained stubs first" \
  'grep -qF "## Auto-groom blocked" "$GN"'
assert "groom-next: auto-groomable stubs flagged, not hidden" \
  'grep -qF "docket-auto-groom will handle it unless you want it now" "$GN"'
band1_line="$(grep -nF "abstained" "$GN" | head -1 | cut -d: -f1)"
band3_line="$(grep -nF "will handle it unless you want it now" "$GN" | head -1 | cut -d: -f1)"
assert "groom-next: abstained band stated before auto-groomable band" \
  '[ -n "$band1_line" ] && [ -n "$band3_line" ] && [ "$band1_line" -lt "$band3_line" ]'
```

- [ ] **Step 2: Run test — new assertions fail**

Run: `bash tests/test_auto_groom.sh` — expect the four `groom-next:` assertions `NOT OK`.

- [ ] **Step 3: Edit `skills/docket-groom-next/SKILL.md` Step 1**

In `### Step 1 — Select`, after the sentence ending "Empty queue → report that nothing needs grooming and stop.", insert this paragraph:

```markdown
When autonomous grooming is in play (see the convention's *Autonomous grooming* shared definition), rank in **selection bands** — the human's attention goes first to stubs that need a human: (1) abstained stubs (a `## Auto-groom blocked` section is present — they are literally waiting on you), then (2) effective `auto_groomable: false` stubs, then (3) effective auto-groomable stubs, each flagged "#NNNN is auto-groomable — docket-auto-groom will handle it unless you want it now." Within each band, the deterministic order applies unchanged. Every needs-brainstorm stub stays selectable — bands reorder, they never exclude; an explicit id still overrides everything.
```

- [ ] **Step 4: Run tests**

Run: `bash tests/test_auto_groom.sh` (PASS) and `bash tests/test_groom_recap.sh` (must still PASS — Step 3 untouched).

- [ ] **Step 5: Commit**

```bash
git add skills/docket-groom-next/SKILL.md tests/test_auto_groom.sh
git commit -m "feat(0014): groom-next selection bands — humans get the stubs that need a human"
```

---

### Task 4: `docket-new-change` — create-time flag + template

**Files:**
- Modify: `skills/docket-new-change/SKILL.md`
- Modify: `skills/docket-new-change/change-template.md`
- Modify: `tests/test_auto_groom.sh` (append before the final `if`)

- [ ] **Step 1: Append failing tests**

```bash
NC="$REPO/skills/docket-new-change/SKILL.md"
TPL="$REPO/skills/docket-new-change/change-template.md"

# --- new-change: create-time flag ---
assert "new-change: create-time auto_groomable mention" \
  'grep -qF "auto_groomable: true" "$NC"'
assert "new-change: scan stubs leave the field unset (inherit)" \
  'grep -qF "leave \`auto_groomable\` unset" "$NC"'
assert "template: documents tri-state auto_groomable" \
  'grep -qF "auto_groomable:" "$TPL"'
```

- [ ] **Step 2: Run test — new assertions fail**

Run: `bash tests/test_auto_groom.sh` — expect the three new assertions `NOT OK`.

- [ ] **Step 3: Edit the two files**

**(a)** `skills/docket-new-change/SKILL.md`, Brainstorm-mode step **4 (Draft the change)** — append to the step's text:

```markdown
When the human provided rich initial context and says the change may be designed without them, set `auto_groomable: true` at draft time — `docket-auto-groom` will carry it to build-ready. Otherwise leave the field unset (it inherits the repo's `auto_groom` default).
```

**(b)** Same file, **Scan mode** paragraph — after the sentence introducing `docket-groom-next` as the later brainstorm pass, add:

```markdown
Scan-stubs leave `auto_groomable` unset — they inherit the repo default; in an `auto_groom: true` repo that makes the whole scan harvest autonomously groomable, which is the point.
```

**(c)** `skills/docket-new-change/change-template.md` — insert directly under the `trivial: false` line:

```yaml
auto_groomable:           # tri-state: unset ⇒ inherit repo auto_groom; true/false ⇒ explicit override
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_auto_groom.sh` — PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-new-change/SKILL.md skills/docket-new-change/change-template.md tests/test_auto_groom.sh
git commit -m "feat(0014): new-change sets auto_groomable at create time; template documents it"
```

---

### Task 5: `docket-status` — board treatment for abstained stubs

**Files:**
- Modify: `skills/docket-status/SKILL.md`
- Modify: `tests/test_auto_groom.sh` (append before the final `if`)

- [ ] **Step 1: Append failing test**

```bash
ST="$REPO/skills/docket-status/SKILL.md"

# --- status: board renders abstained stubs distinctly ---
assert "status: abstained readiness cell defined" \
  'grep -qF "auto-groom blocked — needs you" "$ST"'
```

- [ ] **Step 2: Run test — fails**

Run: `bash tests/test_auto_groom.sh` — expect the `status:` assertion `NOT OK`.

- [ ] **Step 3: Edit `skills/docket-status/SKILL.md`**

In the Board section's **Readiness rules** list (item 3, "Per-group tables…"), the current second bullet reads:

```markdown
   - A `proposed` change with no spec and not `trivial: true` renders **needs-brainstorm**.
```

Replace it with:

```markdown
   - A `proposed` change with no spec and not `trivial: true` renders **needs-brainstorm** — unless its body carries an `## Auto-groom blocked` section, in which case it renders **auto-groom blocked — needs you** (the autonomous groomer abstained; a human must resolve or re-arm it).
```

- [ ] **Step 4: Run tests**

Run: `bash tests/test_auto_groom.sh` (PASS) and `bash tests/test_board_refresh_on_transition.sh` (must still PASS).

- [ ] **Step 5: Commit**

```bash
git add skills/docket-status/SKILL.md tests/test_auto_groom.sh
git commit -m "feat(0014): board renders abstained stubs as auto-groom blocked — needs you"
```

---

### Task 6: README — skill table (fixes 0012's drift too)

**Files:**
- Modify: `README.md`

The README still says "six skills" and its table lacks `docket-groom-next` (pre-existing drift from change 0012). With `docket-auto-groom` the suite is **eight** skills. No test file guards the README; the whole-branch review covers it.

- [ ] **Step 1: Update the counts and enumerations**

- Line 3: "provides six skills to create changes, work the next change to a PR, …" → "provides eight skills to create changes, groom stubs to build-ready (interactively or autonomously), work the next change to a PR, finalize a merged change, report the board, record architecture decisions (ADRs), and define the shared convention they all load".
- Line 13: "six skills, no CLI" → "eight skills, no CLI".
- Line 15: "The six skills cover the full loop: create, implement, finalize, report, decide" → "The eight skills cover the full loop: create, groom, implement, finalize, report, decide".
- Heading `## The six skills` → `## The eight skills`.

- [ ] **Step 2: Add the two missing table rows**

Insert after the `docket-new-change` row:

```markdown
| `docket-groom-next` | Interactive groomer — selects the next needs-brainstorm stub deterministically and designs it to build-ready with the human; abstained auto-groom stubs come first. |
| `docket-auto-groom` | Autonomous groomer — drains the auto-groomable needs-brainstorm queue with no human: default-biased self-brainstorm gated by an adversarial critic; emits specs/trivial verdicts or abstains back to the human queue; never kills or defers. |
```

- [ ] **Step 3: Verify and commit**

Run: `grep -c "six skills" README.md` — expect `0`. Run the full suite: `for t in tests/test_*.sh; do bash "$t" >/dev/null && echo "PASS $t" || echo "FAIL $t"; done` — all PASS.

```bash
git add README.md
git commit -m "docs(0014): README — eight skills; add groom-next (0012 drift) and auto-groom rows"
```

---

### Task 7: Full-suite verification

**Files:** none (verification only)

- [ ] **Step 1: Run every test**

Run: `for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 && echo "PASS $t" || echo "FAIL $t"; done`
Expected: `PASS` for all 8 files (7 existing + `test_auto_groom.sh`).

- [ ] **Step 2: Non-vacuity spot checks**

Temporarily (a) delete the convention's `### Autonomous grooming` subsection → `test_auto_groom.sh` must FAIL; restore. (b) Swap the groom-next bands paragraph's band order mentally — confirm the `grep -n` order assertion would catch a reorder (band1 line vs band3 line). No commit; restore everything (`git checkout -- .` if needed, then re-run suite → all PASS).
