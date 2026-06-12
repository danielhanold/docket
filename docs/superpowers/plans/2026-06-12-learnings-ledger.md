# Learnings Ledger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the learnings ledger — `<changes_dir>/LEARNINGS.md` on the metadata branch, harvested at close-out, read at groom/plan/review time — plus the convention contract, skill touch-ups, tests, and the retro-seed.

**Architecture:** The contract (file, format, harvest/read/distill rules) is single-sourced in `docket-convention` (ADR-0003 pattern). The harvest *procedure* is single-sourced in `docket-finalize-change` as step 2.5; `docket-status`'s sweep invokes it by reference. `docket-implement-next` (plan + review) and `docket-groom-next` (scan-context) gain read lines. A new test file guards the structure. The ledger itself is seeded on the `docket` branch from the five existing results files — a metadata write done by the ORCHESTRATOR in `.docket/` (never by a task subagent, per the 0005 invariant), not on this feature branch.

**Tech Stack:** Markdown skill files; bash test scripts.

**Spec:** `.docket/docs/superpowers/specs/2026-06-12-learnings-ledger-design.md` (docket branch; read-only input).

**Spec deviation (documented):** Spec §8 asks a test to "assert `LEARNINGS.md` is created with its header contract" — impossible as a repo test: the ledger lives only on the `docket` branch, and the suite runs against the integration-branch checkout. Instead Task 5 (retro-seed) is orchestrator-verified and recorded in the results file; the test file (Task 1) covers everything skill-text-side.

**Sentinel discipline:** Two new convention-only sentinel phrases are introduced: `build-loop memory` and `compression, not destruction`. They appear ONLY in `docket-convention`'s new subsection (and in the ledger file itself, which is not a skill); no operating skill may use either phrase. The texts below are written to respect this — do not paraphrase them into violations.

---

### Task 1: New test file (red)

**Files:**
- Create: `tests/test_learnings_ledger.sh`

- [ ] **Step 1: Create `tests/test_learnings_ledger.sh` with EXACTLY this content**

```bash
#!/usr/bin/env bash
# tests/test_learnings_ledger.sh — guards change 0006 (the learnings ledger):
#   - the convention carries the Learnings ledger contract (single source)
#   - the harvest procedure lives in docket-finalize-change; docket-status references it
#   - the readers (implement-next, groom-next) carry their read lines
#   - no operating skill restates the contract (sentinel scan)
# The ledger FILE lives on the docket branch only and is not testable here (see plan/results).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"
OPERATING=(docket-new-change docket-groom-next docket-implement-next docket-status docket-finalize-change docket-adr)

# (a) the convention contract
assert "convention has the Learnings ledger section" 'grep -qF "### Learnings ledger" "$CONV"'
assert "convention names the ledger path" 'grep -qF "LEARNINGS.md" "$CONV"'
assert "convention states the ~300-line soft cap" 'grep -qF "~300 lines" "$CONV"'
assert "directory layout lists LEARNINGS.md" 'grep -qF "LEARNINGS.md            # curated" "$CONV"'

# (b) the harvest procedure: single-sourced in finalize, referenced by status
assert "finalize carries the harvest step" \
  'grep -qF "Harvest learnings" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "finalize has the idempotency probe" \
  'grep -qF "already cites" "$REPO/skills/docket-finalize-change/SKILL.md"'
assert "status sweep invokes the harvest by reference" \
  'grep -qF "Harvest learnings" "$REPO/skills/docket-status/SKILL.md" && grep -qF "docket-finalize-change" "$REPO/skills/docket-status/SKILL.md"'

# (c) the readers
assert "implement-next reads the ledger at plan time and review" \
  '[ "$(grep -cF "LEARNINGS.md" "$REPO/skills/docket-implement-next/SKILL.md")" -ge 2 ]'
assert "groom-next reads the ledger in scan-context" \
  'grep -qF "LEARNINGS.md" "$REPO/skills/docket-groom-next/SKILL.md"'

# (d) anti-restatement sentinels — contract phrases live ONLY in the convention
for s in "build-loop memory" "compression, not destruction"; do
  assert "convention contains sentinel: $s" 'grep -qF "$s" "$CONV"'
  for sk in "${OPERATING[@]}"; do
    f="$REPO/skills/$sk/SKILL.md"
    assert "$sk does not restate: $s" '[ -f "$f" ] && ! grep -qF "$s" "$f"'
  done
done

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_learnings_ledger.sh; echo "exit=$?"`
Expected: NOT OK for (a), (b), (c), and the two "convention contains sentinel" lines; the per-skill "does not restate" lines pass (nothing restates yet); final FAIL, exit=1.

- [ ] **Step 3: Commit**

```bash
git add tests/test_learnings_ledger.sh
git commit -m "test(0006): learnings-ledger structural guards (red)"
```

---

### Task 2: Convention contract

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (directory-layout block + new subsection after "### Build-readiness & selection")

- [ ] **Step 1: Add the ledger to the directory layout**

In the directory-layout code block, after the line:

```
  README.md               # small static blurb linking to BOARD.md (NOT generated)
```

insert:

```
  LEARNINGS.md            # curated build-loop lessons; harvested at close-out (see "Learnings ledger")
```

- [ ] **Step 2: Insert the new subsection**

Immediately after the "### Build-readiness & selection (shared definition)" section (before "### Bootstrap guard"), insert EXACTLY:

```markdown
### Learnings ledger

`<changes_dir>/LEARNINGS.md` — the project's **build-loop memory**: a curated, hand-edited file of lessons the build loop taught, living on `metadata_branch` only (like `BOARD.md`, it is never published to the integration branch — but unlike the board it is curated prose, never regenerated). Flat dated entries, **newest first**, one to three lines each, with provenance and an actionable phrasing — e.g. `- 2026-06-12 (#12, PR #7) — <what happened, one clause>. Apply: <the rule to follow next time>.`

**Writing.** Entries are added only by the **harvest** at close-out (its procedural single source is the *Harvest learnings* step in `docket-finalize-change`; `docket-status`'s sweep invokes it by reference). Zero entries for a change is normal. Kills are not harvested — `## Why killed` already records the rationale.

**Reading.** `docket-implement-next` reads the ledger at plan time and again at its review step; `docket-groom-next` reads it before a brainstorm. No other skill reads it.

**Distilling.** Append-only until the file exceeds **~300 lines**; the next harvest past the cap also distills — merge near-duplicates and drop entries since promoted to CLAUDE.md or this convention. Distillation is **compression, not destruction**: git history keeps everything dropped. Boundary: the ledger holds lessons for the build loop; durable project conventions belong in CLAUDE.md — promotion removes the entry here.
```

- [ ] **Step 3: Verify and run tests**

Run: `bash tests/test_learnings_ledger.sh 2>&1 | grep -c "NOT OK"; bash tests/test_convention_extraction.sh >/dev/null 2>&1; echo "extraction=$?"`
Expected: NOT OK count drops to 5 (finalize ×2, status ×1, readers ×2 remain red); `extraction=0` (no existing sentinel broken).

- [ ] **Step 4: Commit**

```bash
git add skills/docket-convention/SKILL.md
git commit -m "docs(0006): convention — Learnings ledger contract (single source)"
```

---

### Task 3: Harvest procedure (finalize) + sweep reference (status)

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (new step 2.5 in "Per-change steps")
- Modify: `skills/docket-status/SKILL.md` (new sub-step h in the merge sweep)

- [ ] **Step 1: Insert step 2.5 in docket-finalize-change**

In "## Per-change steps", AFTER the step-2 block (which ends with the "> **Close-out (optional).** …" note) and BEFORE "3. **Archive (idempotent)**", insert EXACTLY:

```markdown
2.5 **Harvest learnings.** Distill this change's close-out signals — PR review comments (`gh pr view <pr> --comments`), merge-gate feedback, and the `results:` file's findings — into **zero or more** entries at the top of `<changes_dir>/LEARNINGS.md` in the metadata working tree (format per the convention's *Learnings ledger*; provenance `(#<id>, PR #<n>)`). Zero entries is normal — most changes teach nothing new; harvest only what generalizes beyond this change. **Idempotency probe:** skip the change entirely if the ledger already cites `(#<id>` — this is what makes a sweep racing finalize a no-op. If the file exceeds the convention's soft cap, this harvest also distills per the convention's rules. Commit the ledger as its **own commit** on `metadata_branch` (never bundled with the archive commit, which must stay byte-identical across concurrent archivers) and push. Kills are not harvested. This step is the harvest procedure's **single source**; `docket-status`'s sweep invokes it by reference.
```

Also update the sentence in the skill's Overview paragraph that lists what finalize does. Replace:

```
merging the approved PR into the integration branch, then driving the **`done`** terminal transition — archiving the change on `metadata_branch`, publishing its terminal records onto the integration branch (`docket`-mode), cleaning up the branch and worktree, and refreshing the board.
```

with:

```
merging the approved PR into the integration branch, then driving the **`done`** terminal transition — harvesting learnings, archiving the change on `metadata_branch`, publishing its terminal records onto the integration branch (`docket`-mode), cleaning up the branch and worktree, and refreshing the board.
```

And update the per-change framing line. Replace:

```
**Steps 1–4 run per selected change** (check → verify → archive → clean up), exactly mirroring `docket-status`'s per-change archive loop.
```

with:

```
**Steps 1–4 run per selected change** (check → verify → harvest → archive → clean up), exactly mirroring `docket-status`'s per-change archive loop (which invokes the same harvest by reference).
```

- [ ] **Step 2: Insert sub-step h in docket-status's merge sweep**

In "## Merge sweep", step 3, AFTER sub-step "g. **Remove the merged feature branch + worktree** …" and BEFORE the "**Determinism invariant.**" paragraph, insert EXACTLY:

```markdown
   h. **Harvest learnings (best-effort)** — invoke the harvest procedure (the *Harvest learnings* step in `docket-finalize-change`, its single source) for the swept change. Its idempotency probe makes a sweep racing `docket-finalize-change` a safe no-op. Best-effort like the board: log and continue on failure — never abort the sweep for it.
```

- [ ] **Step 3: Run tests**

Run: `bash tests/test_learnings_ledger.sh 2>&1 | grep -c "NOT OK"; bash tests/test_docket_metadata_branch.sh >/dev/null 2>&1; echo "meta=$?"`
Expected: NOT OK count drops to 2 (only the reader asserts remain); `meta=0`.

- [ ] **Step 4: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md
git commit -m "feat(0006): harvest procedure in finalize (single source) + sweep reference"
```

---

### Task 4: Reader lines (implement-next, groom-next)

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (step 4 and step 6)
- Modify: `skills/docket-groom-next/SKILL.md` (step 2)

- [ ] **Step 1: implement-next, step 4 — plan-time read**

In "### Step 4 — Worktree + plan", find the sentence beginning `Run \`superpowers:writing-plans\`:` and insert IMMEDIATELY BEFORE it:

```
Alongside the spec, read `<changes_dir>/LEARNINGS.md` from the same metadata working tree — past lessons inform the plan.
```

- [ ] **Step 2: implement-next, step 6 — review-time read**

In "### Step 6 — Review + ADRs", replace the opening sentence:

```
`superpowers:requesting-code-review` (whole-branch).
```

with:

```
`superpowers:requesting-code-review` (whole-branch); re-read `<changes_dir>/LEARNINGS.md` first so past lessons feed the review.
```

- [ ] **Step 3: groom-next, step 2 — scan-context read**

In "### Step 2 — Scan related context", replace:

```
Read the neighbouring `active/` changes, recently archived changes, and the ADR index BEFORE the brainstorm, so the conversation is informed by adjacent work.
```

with:

```
Read the neighbouring `active/` changes, recently archived changes, the ADR index, and `<changes_dir>/LEARNINGS.md` BEFORE the brainstorm, so the conversation is informed by adjacent work and past lessons.
```

- [ ] **Step 4: Run the full suite**

Run: `for t in tests/test_*.sh; do echo "== $t"; bash "$t" >/dev/null 2>&1 && echo PASS || echo "FAIL($?)"; done`
Expected: PASS for all six test files (including the new one).

- [ ] **Step 5: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-groom-next/SKILL.md
git commit -m "feat(0006): ledger read lines — implement-next (plan, review) + groom-next (scan)"
```

---

### Task 5: Retro-seed the ledger (ORCHESTRATOR ONLY — metadata tree, not this branch)

**Files:**
- Create: `.docket/docs/changes/LEARNINGS.md` (on the `docket` branch via the metadata working tree — NOT in this worktree, NOT on `feat/learnings-ledger`)

> Docket bookkeeping stays in the orchestrator context, never in task subagents. This task is executed directly by the orchestrating session in `/Users/homer/dev/docket/.docket`.

- [ ] **Step 1: Sync the metadata tree, then create `docs/changes/LEARNINGS.md` with EXACTLY this content**

```markdown
<!-- LEARNINGS.md — the learnings ledger (contract: docket-convention, "Learnings ledger").
     Build-loop memory: lessons harvested at change close-out, read at groom, plan, and review
     time. Durable project conventions belong in CLAUDE.md — promotion during a distill removes
     the entry here. Newest first. Soft cap ~300 lines; the first harvest past the cap also
     distills (compression, not destruction — git history keeps whatever is dropped). -->

- 2026-06-12 (#12, PR #7) — A code-review finding cited a sentence that did not exist in the
  reviewed file. Apply: verify review claims against the artifact (byte-diff against canonical
  content) before implementing fixes; reject false positives with evidence.
- 2026-06-12 (#12, PR #7) — link-skills.sh needed no edit for a new skill — it globs skills/*/.
  Apply: at reconcile, check whether plumbing auto-discovers before planning an edit to it.
- 2026-06-10 (#5, PR #6) — A full convention restatement hid in paraphrase ("satisfied = done")
  where fixed-string sentinels could not see it. Apply: sentinel greps are sampling, not parsing;
  pair them with a whole-branch review that reads for meaning.
- 2026-06-10 (#5, PR #6) — YAML frontmatter: an unquoted scalar value cannot contain ": "
  (colon-space). Apply: reword with an em-dash or quote the scalar in skill descriptions.
- 2026-06-04 (#2) — A backward-compat test assertion was vacuous (any "main-mode" mention
  satisfied it). Apply: prove each assertion non-vacuous — deleting the clause it guards must
  flip the test to NOT OK.
- 2026-06-02 (#1) — Fragmenting a tightly-coupled single-artifact edit across subagents risks
  inconsistent edits to shared content. Apply: build inline when tasks share one artifact; fan
  out only for genuinely independent tasks.
```

- [ ] **Step 2: Commit and push on the metadata branch**

```bash
cd /Users/homer/dev/docket/.docket
git pull --rebase origin docket
git add docs/changes/LEARNINGS.md
git commit -m "docket(0006): seed LEARNINGS.md from archived results files (0001-0012)"
git push origin docket
```

- [ ] **Step 3: Verify**

Run: `git -C /Users/homer/dev/docket/.docket show origin/docket:docs/changes/LEARNINGS.md | head -8`
Expected: the header comment, after a fetch confirms the push landed.
