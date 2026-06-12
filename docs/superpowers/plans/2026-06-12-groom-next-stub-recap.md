# Groom-Next Stub Recap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docket-groom-next` opens its brainstorm with a zero-context recap of the selected stub, so a human on a phone or in a fresh session can participate immediately.

**Architecture:** Single-artifact edit to `skills/docket-groom-next/SKILL.md` (change 0013, trivial): Step 3 gains a required recap before `superpowers:brainstorming` is invoked, and Step 1's dependency-status statement is redirected into that recap. Guarded by a new per-change sentinel test, following the house pattern (one `tests/test_<change>.sh` of fixed-string greps; the whole-branch review reads for meaning — sentinels are sampling, per LEARNINGS).

**Tech Stack:** Markdown skill text + bash sentinel test (no runner; tests are standalone `set -uo pipefail` scripts with an `assert` helper).

---

### Task 1: Recap step in docket-groom-next + sentinel test

**Files:**
- Create: `tests/test_groom_recap.sh`
- Modify: `skills/docket-groom-next/SKILL.md` (Step 1 second paragraph, Step 3 heading + body)

Per LEARNINGS 2026-06-02 (#1), this is one tightly-coupled artifact — build inline as a single task; do not fan out.

- [ ] **Step 1: Write the failing sentinel test**

Create `tests/test_groom_recap.sh` (mode 755; siblings are 644 but the suite is run via `bash`, so the bit is cosmetic):

```bash
#!/usr/bin/env bash
# tests/test_groom_recap.sh — guards change 0013 (groom-next stub recap):
#   - Step 3 opens with a recap of the selected stub, written for a zero-context reader,
#     BEFORE superpowers:brainstorming is invoked (phone / fresh-session grooming)
#   - the recap covers: what was selected and why, a PM-altitude summary, dependency
#     statuses (folded in from Step 1), and the open questions framed as the agenda
#   - the recap is an introduction, not a confirmation gate
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

SKILL="$REPO/skills/docket-groom-next/SKILL.md"

assert "step 3 is recap-then-groom" \
  'grep -qF "### Step 3 — Recap, then groom with the human" "$SKILL"'
assert "recap is written for a zero-context reader" \
  'grep -qF "written for a reader with no prior context" "$SKILL"'
assert "recap states the selection: id, title, priority" \
  'grep -qF "id, title, priority" "$SKILL"'
assert "recap distills the stub at PM altitude" \
  'grep -qF "PM-altitude summary" "$SKILL"'
assert "step 1 folds dependency statuses into the recap" \
  'grep -qF "as part of the Step 3 recap" "$SKILL"'
assert "recap frames open questions as the agenda" \
  'grep -qF "the agenda the brainstorm will work through" "$SKILL"'
assert "recap is an introduction, not a confirmation gate" \
  'grep -qF "introduction, not a confirmation gate" "$SKILL"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

Non-vacuity (per LEARNINGS 2026-06-04 #2): every fixed string above is absent from the current `SKILL.md` except `id, title, priority`-adjacent fragments — verify in Step 2 that at least the heading, zero-context, fold-in, agenda, and gate assertions FAIL before the edit; any assertion already passing must be tightened before proceeding.

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_groom_recap.sh`
Expected: `NOT OK` on (at minimum) the heading, zero-context, fold-in, agenda, and confirmation-gate assertions; final line `FAIL`; exit 1.

- [ ] **Step 3: Edit Step 1 of the skill — redirect the dependency statement into the recap**

In `skills/docket-groom-next/SKILL.md`, replace the exact sentence:

```
Instead, open the session by stating each dependency and its current status, so the human designs with eyes open.
```

with:

```
Instead, state each dependency and its current status as part of the Step 3 recap, so the human designs with eyes open.
```

- [ ] **Step 4: Edit Step 3 of the skill — add the recap**

Replace the exact current Step 3 block:

```
### Step 3 — Groom with the human

Run `superpowers:brainstorming` WITH THE HUMAN, seeded with the stub's body and its `## Open questions` — the open questions are the session's starting agenda. STOP AT THE SPEC — do NOT continue to `superpowers:writing-plans` (planning is build-time, owned by `docket-implement-next`).
```

with:

```
### Step 3 — Recap, then groom with the human

Open with a **recap of the selected stub**, written for a reader with no prior context — grooming is routinely invoked from a phone or a fresh session, long after the stub was captured, and a cold-start human cannot answer design questions about a change they have not been reminded of. The recap covers:

- What was selected and why: id, title, priority — and whether it was the deterministic pick or an explicitly requested id.
- A PM-altitude summary of the stub: its `## Why` and `## What changes` distilled into a few sentences.
- Each `depends_on` entry and its current status (the statement Step 1 requires).
- The stub's `## Open questions`, framed as the agenda the brainstorm will work through.

The recap is an introduction, not a confirmation gate — flow directly into the brainstorm; the human redirects there, not at a pre-brainstorm prompt.

Then run `superpowers:brainstorming` WITH THE HUMAN, seeded with the stub's body and its `## Open questions` — the open questions are the session's starting agenda. STOP AT THE SPEC — do NOT continue to `superpowers:writing-plans` (planning is build-time, owned by `docket-implement-next`).
```

(The brainstorm paragraph is byte-identical to the current one; only the heading changes and the recap paragraphs are inserted above it.)

- [ ] **Step 5: Run the new test to verify it passes**

Run: `bash tests/test_groom_recap.sh`
Expected: 7 × `ok`, final line `PASS`, exit 0.

- [ ] **Step 6: Run the full suite (cross-reference guard)**

Run: `for t in tests/test_*.sh; do echo "== $t"; bash "$t" | tail -1; done`
Expected: every file ends `PASS`. In particular `test_learnings_ledger.sh` still passes — the edit must not move or reword Step 2's `LEARNINGS.md` read line — and the inventory arrays in `test_convention_extraction.sh` / `test_docket_metadata_branch.sh` are untouched (no new skill).

- [ ] **Step 7: Commit**

```bash
git add tests/test_groom_recap.sh skills/docket-groom-next/SKILL.md
git commit -m "feat(0013): groom-next recaps the selected stub before the brainstorm"
```
