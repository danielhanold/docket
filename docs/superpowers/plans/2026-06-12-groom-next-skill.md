# docket-groom-next Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the sixth operating skill, `docket-groom-next`, which selects the next needs-brainstorm stub and grooms it to build-ready through an interactive brainstorm, plus the enumeration touch-ups and test coverage.

**Architecture:** One new SKILL.md following the post-0005 reference-loading pattern (blocking Step-0 `docket-convention` load, convention vocabulary used but never restated). Three existing files get one-line-scale touch-ups: `docket-convention` (operating-skills enumeration 5→6, two body lines + frontmatter description), `docket-new-change` (scan mode names `docket-groom-next` as its "later brainstorm pass"). Two test scripts gain the new skill in their hardcoded arrays. `link-skills.sh` globs `skills/*/` and needs no change.

**Tech Stack:** Markdown skill files; bash test scripts (`tests/*.sh`, plain `assert` helpers, run directly with `bash`).

**Spec:** `.docket/docs/superpowers/specs/2026-06-12-groom-next-skill-design.md` (on the `docket` branch — read-only input to this plan; never edit it from this worktree).

**Hard constraints carried from change 0005 (enforced by `tests/test_convention_extraction.sh`):**
- The new SKILL.md MUST contain the exact heading `## Convention (load first — blocking)` and the string `docket-convention`.
- The new SKILL.md MUST NOT contain any anti-copy sentinel. Forbidden exact strings: `never gitignored`, `proposed ──claim──▶`, `satisfied when it reaches`, `immutable once Accepted`, `live planning surface`, `half-migrated`, `only flow of metadata onto the code line`, `zero-padded to 4 digits`, `PM-altitude proposal`, `must never trail the change files`, `<!-- docket:convention:begin -->`, `<!-- docket:convention:end -->`.
- Because the new skill joins `tests/test_docket_metadata_branch.sh`'s `SKILLS` array, its SKILL.md MUST contain the strings `integration_branch` and `metadata working tree` (assertions B and C). Both occur naturally in the shared "Where everything is read and written" paragraph.

---

### Task 1: Extend the two test arrays (failing tests first)

**Files:**
- Modify: `tests/test_convention_extraction.sh:16`
- Modify: `tests/test_docket_metadata_branch.sh:9`

- [ ] **Step 1: Add `docket-groom-next` to the OPERATING array**

In `tests/test_convention_extraction.sh` replace line 16:

```bash
OPERATING=(docket-new-change docket-implement-next docket-status docket-finalize-change docket-adr)
```

with:

```bash
OPERATING=(docket-new-change docket-implement-next docket-status docket-finalize-change docket-adr docket-groom-next)
```

- [ ] **Step 2: Add `docket-groom-next` to the SKILLS array**

In `tests/test_docket_metadata_branch.sh` replace line 9:

```bash
SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr)
```

with:

```bash
SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr docket-groom-next)
```

- [ ] **Step 3: Run both tests to verify they fail (skill does not exist yet)**

Run: `bash tests/test_convention_extraction.sh; echo "exit=$?"`
Expected: `NOT OK` lines for `docket-groom-next has the Step-0 load heading` and `docket-groom-next names docket-convention` (grep on a missing file), final `FAIL`, `exit=1`.

Run: `bash tests/test_docket_metadata_branch.sh; echo "exit=$?"`
Expected: `NOT OK` lines for `integration_branch knob present in docket-groom-next` and `metadata working tree wording in docket-groom-next`, `exit=1`.

- [ ] **Step 4: Commit**

```bash
git add tests/test_convention_extraction.sh tests/test_docket_metadata_branch.sh
git commit -m "test(0012): expect docket-groom-next in skill inventories (red)"
```

---

### Task 2: Create `skills/docket-groom-next/SKILL.md`

**Files:**
- Create: `skills/docket-groom-next/SKILL.md`

- [ ] **Step 1: Write the skill file with EXACTLY this content**

````markdown
---
name: docket-groom-next
description: Use when stubs are sitting at needs-brainstorm on the docket board and you want the next one designed — selecting the next needs-brainstorm change (proposed, no spec, not trivial) deterministically and grooming it to build-ready through an interactive brainstorm with the human, exiting with a linked spec, a trivial verdict, a kill, or a defer. Selection is autonomous; the design conversation is not. Writes markdown only — never branches, worktrees, or code.
---

# docket-groom-next — the groomer (interactive)

## Overview

`docket-groom-next` drains the needs-brainstorm queue. `docket-new-change`'s scan mode captures ideas on the go as lightweight stubs; this skill is the later brainstorm pass that turns them build-ready. It mirrors `docket-implement-next`'s shape — a "next" skill over a queue — but the queue is needs-brainstorm stubs, the work is an interactive design conversation with the human, and the exit is a build-ready `proposed` change, not an open PR. One stub per invocation; loop by re-invoking. It writes markdown only: the change file, a spec, and a refreshed `BOARD.md` — never branches, worktrees, or code.

## When to use

- Stubs show as needs-brainstorm on the board and you want to design the next one.
- You want to groom a specific stub now (pass its id explicitly to skip selection).
- Do NOT use to capture a brand-new idea — that is `docket-new-change`'s job; this skill never mints ids.
- Do NOT use to re-groom a change that already has a spec — drift against current reality is the reconcile pass's job in `docket-implement-next`. A human who wants to redo a design can clear `spec:` by hand first.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (needs-brainstorm, build-ready, metadata working tree, the bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Where everything is read and written

All reads and writes happen in the **metadata working tree** on `metadata_branch`, pushed to its remote immediately so the backlog stays reviewable on GitHub and visible to the autonomous implementer. In `docket`-mode that tree is the persistent `.docket/` worktree parked on `docket` — ensure it (state-specific create per the convention's Branch model, idempotent) and **sync it to `origin/docket` before any read** (`git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`). In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (which equals `origin/<integration_branch>` there). The steps below say "`.docket/`" / "`origin/docket`" for the common (`docket`-mode) case; read those as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

## Procedure

### Step 1 — Select

Sync the metadata working tree, then rank every needs-brainstorm change in `active/` — `status: proposed`, no `spec:`, not `trivial: true` — by the convention's deterministic selection order (the same ranking `docket-implement-next` uses). Pick the top, or accept an explicit id from the caller; an explicit id that is not needs-brainstorm is an error to report, never a silent re-pick. Empty queue → report that nothing needs grooming and stop.

Unsatisfied `depends_on` does NOT exclude a stub — designing ahead of dependencies is expected (that is what specs are for, and the implementer's reconcile pass re-validates every spec against current reality at build time). Instead, open the session by stating each dependency and its current status, so the human designs with eyes open.

No claim is taken — see *Concurrency* below.

### Step 2 — Scan related context

Read the neighbouring `active/` changes, recently archived changes, and the ADR index BEFORE the brainstorm, so the conversation is informed by adjacent work. Record the resulting `related:`/`depends_on:`/`adrs:` updates after the design settles.

### Step 3 — Groom with the human

Run `superpowers:brainstorming` WITH THE HUMAN, seeded with the stub's body and its `## Open questions` — the open questions are the session's starting agenda. STOP AT THE SPEC — do NOT continue to `superpowers:writing-plans` (planning is build-time, owned by `docket-implement-next`).

### Step 4 — Exit (one of four; the human confirms which)

All four exits reuse existing transitions — this skill introduces no new lifecycle status:

1. **Spec** (the normal exit): write the design doc natively to `.docket/docs/superpowers/specs/<UTC date>-<slug>-design.md` (on `metadata_branch`); set `spec:`; refresh the change body to the settled design (keep it at proposal altitude — design detail lives in the spec); remove resolved `## Open questions` entries; set `updated: <UTC today>`. The change is now build-ready.
2. **Trivial verdict**: the brainstorm concludes there is no real design question — set `trivial: true`, tighten the body, no spec, set `updated:`. Also build-ready.
3. **Kill**: the stub is obsolete, a duplicate, or decided against — follow the proposed-kill sub-path in `docket-new-change` (it owns the kill mechanics; do not restate them here).
4. **Defer**: right idea, wrong time — set `status: deferred`, add `## Why deferred`, set `updated:`.

### Step 5 — Commit, push, board

Commit the change-file edit + spec together in the metadata working tree and push to `origin/docket`. On a non-fast-forward rejection: `pull --rebase` and retry; if the rebase brought in commits touching the groomed change's file, RE-READ it first — if it is no longer needs-brainstorm (someone else groomed, killed, or claimed it), STOP and report rather than overwrite. Then refresh `BOARD.md` via `docket-status`'s Board pass as a separate, must-land commit (same pattern as `docket-new-change`'s step 5) — the readiness cell flips from needs-brainstorm, or the row leaves the Proposed section on a kill or defer. STOP — grooming never implements.

## Concurrency — no claim

Grooming is human-attended and minutes-long, so concurrent-groomer collisions are improbable; the step-5 push discipline (rebase-retry plus the mandatory re-read when the groomed file was touched) is the compare-and-swap that protects the write. A `grooming:` marker field and a status-based claim were considered and rejected — both add machinery (new field or new status, plus stale-state cleanup) for a race that the final-push CAS already resolves safely.
````

- [ ] **Step 2: Run the extraction test to verify it passes**

Run: `bash tests/test_convention_extraction.sh; echo "exit=$?"`
Expected: all `ok` lines (including `docket-groom-next has the Step-0 load heading`, `docket-groom-next names docket-convention`, and every `docket-groom-next has no convention copy: …` sentinel line), final `PASS`, `exit=0`.

- [ ] **Step 3: Run the metadata-branch test to verify it passes**

Run: `bash tests/test_docket_metadata_branch.sh; echo "exit=$?"`
Expected: `ok - integration_branch knob present in docket-groom-next`, `ok - metadata working tree wording in docket-groom-next`, no `NOT OK` lines, `exit=0`.

- [ ] **Step 4: Commit**

```bash
git add skills/docket-groom-next/SKILL.md
git commit -m "feat(0012): docket-groom-next — the groomer skill (green)"
```

---

### Task 3: Enumeration touch-ups in docket-convention and docket-new-change

**Files:**
- Modify: `skills/docket-convention/SKILL.md:3` (frontmatter description), `:8` (overview), `:12` (convention intro)
- Modify: `skills/docket-new-change/SKILL.md:47` (scan mode)

- [ ] **Step 1: Update the convention frontmatter description (line 3)**

Replace the description's skill enumeration. Old text (one line; only the enumerated list changes):

```
description: Use when any docket skill runs — docket-new-change, docket-implement-next, docket-status, docket-finalize-change, and docket-adr load this first (their blocking Step 0) — or when you need to understand how docket tracks work. The shared contract — .docket.yml configuration, directory layout, the change manifest and lifecycle, ADR format, build-readiness and selection, the bootstrap guard, and the branch model. Pure reference — defines the convention; performs no reads, writes, or git operations.
```

New text:

```
description: Use when any docket skill runs — docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, and docket-adr load this first (their blocking Step 0) — or when you need to understand how docket tracks work. The shared contract — .docket.yml configuration, directory layout, the change manifest and lifecycle, ADR format, build-readiness and selection, the bootstrap guard, and the branch model. Pure reference — defines the convention; performs no reads, writes, or git operations.
```

- [ ] **Step 2: Update the overview count (line 8)**

Old:

```
This skill defines the docket convention and does nothing else: no procedure, no reads or writes, no git. The five operating skills load it as their blocking Step 0 and use its vocabulary without restating it.
```

New:

```
This skill defines the docket convention and does nothing else: no procedure, no reads or writes, no git. The six operating skills load it as their blocking Step 0 and use its vocabulary without restating it.
```

- [ ] **Step 3: Update the convention-intro enumeration (line 12)**

Old (only the parenthesized list changes):

```
…the operating skills (docket-new-change, docket-implement-next, docket-status, docket-finalize-change, docket-adr) load it at startup as their blocking Step 0 and never restate it.
```

New:

```
…the operating skills (docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, docket-adr) load it at startup as their blocking Step 0 and never restate it.
```

- [ ] **Step 4: Point scan mode at the new skill (docket-new-change line 47)**

Old sentence (within the Scan mode paragraph):

```
They form an "ideas to brainstorm" backlog a later brainstorm pass turns build-ready.
```

New sentence:

```
They form an "ideas to brainstorm" backlog that `docket-groom-next` — the later brainstorm pass — turns build-ready.
```

- [ ] **Step 5: Verify the edits**

Run: `grep -c "docket-groom-next" skills/docket-convention/SKILL.md skills/docket-new-change/SKILL.md`
Expected: `skills/docket-convention/SKILL.md:2` and `skills/docket-new-change/SKILL.md:1`.

Run: `grep -n "five operating" skills/docket-convention/SKILL.md; echo "exit=$?"`
Expected: no output, `exit=1` (the word "five" is gone).

- [ ] **Step 6: Run both touched tests again**

Run: `bash tests/test_convention_extraction.sh && bash tests/test_docket_metadata_branch.sh; echo "exit=$?"`
Expected: final `PASS` from the first, no `NOT OK` anywhere, `exit=0`.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-new-change/SKILL.md
git commit -m "docs(0012): convention + new-change name docket-groom-next (six operating skills)"
```

---

### Task 4: Full-suite verification

**Files:** none modified.

- [ ] **Step 1: Run every test in the suite**

Run: `for t in tests/test_*.sh; do echo "== $t"; bash "$t" >/dev/null 2>&1 && echo PASS || echo "FAIL($?)"; done`
Expected: `PASS` for all five test files. If any fails, re-run it without the redirect to see which assertion broke, fix, and re-run before proceeding.

- [ ] **Step 2: Sanity-check link-skills.sh picks up the new skill via its glob (no edit was made to it)**

Run: `tmp=$(mktemp -d); mkdir -p "$tmp/.claude/skills"; DOCKET_HARNESS_ROOT="$tmp" bash link-skills.sh | grep groom; rm -rf "$tmp"`
Expected: a `linked …/.claude/skills/docket-groom-next -> …/skills/docket-groom-next` line.

- [ ] **Step 3: Verify no stray sentinel strings slipped into the new skill**

Run: `grep -nF -e "never gitignored" -e "satisfied when it reaches" -e "immutable once Accepted" -e "live planning surface" -e "half-migrated" -e "zero-padded to 4 digits" -e "PM-altitude proposal" -e "must never trail the change files" skills/docket-groom-next/SKILL.md; echo "exit=$?"`
Expected: no output, `exit=1`.
