# Markhaus → docket Migration Plan (first dogfood)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **This plan DEPENDS ON** the docket skill set being built and installed first — see `docs/superpowers/plans/2026-05-30-docket-skill-set.md` (Tasks 1–9, then `bash ~/dev/docket/link-skills.sh`). It exercises the five `docket-*` skills against a real repo; it does not build them.

> **✅ STATUS — IMPLEMENTED 2026-05-31** (subagent-driven; each task ran implementer → spec-compliance review → code-quality review, plus a final holistic review). Executed in `/Users/homer/dev/macmd` on `main`: 6 commits (`a39a0c4..575f37d`), pushed to `origin/main`. Tasks 0–5 complete and verified (clean `docket-status` health checks; 28 ADRs + index, 11 changes, generated BOARD). **Task 6 deliberately skipped** — no build-ready change exists yet (its two steps stay unchecked; see the Task 6 note).

**Goal:** Migrate the Markhaus repo (`/Users/homer/dev/macmd`) onto docket — convert its 28 prose-header ADRs in `docs/decisions/` to docket's frontmatter ledger in `docs/adrs/`, and turn its `docs/plans/` + `*-results.md` (and the newer `docs/superpowers/{specs,plans}/` generation) into tracked `docs/changes/` — proving docket's lifecycle, ADR index, and health checks end-to-end on real history.

**Architecture:** Operate **in `/Users/homer/dev/macmd`** on default `metadata_branch: main`. Three migrations run in order: (1) bootstrap docket config + dirs; (2) ADR ledger (`docs/decisions/` → `docs/adrs/`, mechanical prose→frontmatter conversion preserving every ADR number and the existing supersede/reverse graph, then `docket-adr` regenerates + validates the index); (3) changes (`docs/plans/` + results → `docs/changes/active|archive/`, completed work folded into `done` change bodies, open work classified by reading). Then `docket-status` regenerates `BOARD.md` and its health checks are the verification gate.

**Tech Stack:** Markdown, `git`, the installed `docket-*` skills, `superpowers:*` (only if a live build is run in Phase D). No code changes to Markhaus's app source.

---

## Scope & Decisions This Plan Locks In

1. **Target repo:** `/Users/homer/dev/macmd` (Markhaus). All paths below are relative to it unless they start with `~/dev/docket`. **Do not** modify the docket repo here.

2. **Prerequisite:** the five `docket-*` skills are built and installed (invocable by flat name in this harness). Verify with the check in Task 0. If absent, run the docket-skill-set plan first.

3. **Default `main` mode.** Markhaus has no hard-protected `main`, so use `metadata_branch: main`. Metadata commits land on `main`. (`docket` mode is a v1 rough edge — not used here.)

4. **Preserve ADR numbers.** The 28 existing ADRs keep ids 1–28 and their kebab slugs. Migration **hand-writes** their frontmatter (it is *not* a `docket-adr` "Create" — that would mint new numbers); only `docket-adr`'s **Index/validate** action is invoked, to regenerate `docs/adrs/README.md` and validate links. This is the real test of the index renderer against a 28-ADR ledger with reversals + supersessions.

5. **Classify changes by reading, not guessing.** A plan with a matching `*-results.md` **and** whose governing ADR is `Accepted`/`Reversed` (i.e. the work shipped) becomes a `done` change with the results folded into its body. A plan with no results and no shipped evidence is `proposed` or `in-progress` — decided by reading the plan, the code, and `git log`. The inventory tables below give the deterministic starting classification; the executor confirms each by reading. Per docket's ethos, treat this like a reconcile pass: record *why* each status was chosen.

6. **The spec's §12 example is stale.** It assumed an open `feat/quicklook-interactions` branch → change `0007`. There is **no such branch now** (quicklook plan #4 shipped — `plan-4-results.md`, ADR-0024 `Accepted`). The open quicklook work is `2026-05-20-plan-4-followup-quicklook-interactions.md`; classify it by reading, not by the spec's stale example.

7. **Commit on `main`, do not push unless asked.** Each task commits locally. Pushing to Markhaus's remote is a human decision — ask before `git push`.

---

## Source Inventory (real contents of `/Users/homer/dev/macmd` as of 2026-05-30)

### ADRs — `docs/decisions/*.md` (28) → `docs/adrs/`

All are prose-header (`# ADR-NNNN: <title>`, `**Date:**`, `**Status:**`, `## Context` / `## Decision` / `## Consequences`). 23 are plain `Accepted`; the 5 non-trivial statuses below are the migration's link graph:

| ADR | New `status:` | Link frontmatter to set |
|---|---|---|
| 0001 disabled-code-signing-for-local-dev | `Reversed by ADR-0022` | — (target of 0022.reverses) |
| 0007 app-sandbox-disabled-in-dev | `Reversed by ADR-0022` | — (target of 0022.reverses) |
| 0019 omit-application-groups-in-dev | `Reversed by ADR-0022` | — (target of 0022.reverses) |
| 0022 post-enrollment-signing-posture | `Accepted` | `reverses: [1, 7, 19]` |
| 0015 mdash-name-and-positioning | `Superseded by ADR-0021` | — (target of 0021.supersedes); **partial** — see note |
| 0021 rename-to-markhaus | `Accepted` | `supersedes: [15]` |
| 0025 pdf-page-size-via-webview-frame | `Superseded by ADR-0027` | — (target of 0027.supersedes) |
| 0027 page-size-and-margins-via-pagedjs | `Accepted` | `supersedes: [25]` |

All others (0002–0006, 0008–0014, 0016–0018, 0020, 0023, 0024, 0026, 0028) → `status: Accepted`, empty `supersedes`/`reverses`/`relates_to` unless their body cross-references another ADR (capture those as `relates_to`).

> **Partial-supersede note (0015):** the original status reads *"Name superseded by ADR-0021. Pricing & positioning still in force."* docket's enum can't express "partial", so set `status: Superseded by ADR-0021` **and** append a dated `## Update` note to 0015's body preserving the nuance ("only the *name* is superseded; pricing & positioning remain in force"). Do **not** delete the still-in-force content.

### Plans + results → `docs/changes/`

Two generations exist. Suggested `id` = chronological order; **final status confirmed by reading** (decision #5).

**Generation 1 — `docs/plans/*.md` + `docs/*-results.md`:**

| Plan file | Results file | Governing ADRs | Starting classification |
|---|---|---|---|
| 2026-05-18-spike-render-pipeline.md | spike-results.md | 0009, 0010, 0017 | `done` |
| 2026-05-18-plan-2-theme-system.md | *(none)* | 0012, 0013 | read → likely `done` |
| 2026-05-18-plan-3-tabs-toc.md | plan-3-results.md | 0020 | `done` |
| 2026-05-18-plan-6-onboarding.md | *(none)* | — | read → `done`/`proposed` |
| 2026-05-20-plan-4-quicklook.md | plan-4-results.md | 0014, 0024 | `done` |
| 2026-05-20-plan-5-pdf-export.md | plan-5-results.md | 0016, 0025 | `done` |
| 2026-05-20-preferences-pane.md | preferences-pane-results.md | 0013 | `done` (but dedupe vs Gen-2 preferences) |
| 2026-05-20-rename-and-enrollment.md | *(none)* | 0021, 0006, 0022 | read → likely `done` |
| 2026-05-20-plan-4-followup-quicklook-interactions.md | *(none)* | 0024 | read → likely `proposed`/`in-progress` (open quicklook work) |
| 2026-05-21-plan-5-followup-pdf-pagination.md | *(none)* | 0026, 0027 | read → likely `done` (pagination ADRs Accepted) |

**Generation 2 — `docs/superpowers/plans/*.md` (2026-05-29) WITH design specs in `docs/superpowers/specs/`:**

| Plan file | Matching spec (`spec:` link) | Starting classification |
|---|---|---|
| 2026-05-29-export-preview-render.md | docs/superpowers/specs/2026-05-29-export-preview-render-design.md | read → recent; `proposed`/`in-progress` |
| 2026-05-29-pagedjs-pdf-export.md | docs/superpowers/specs/2026-05-29-pagedjs-pdf-export-design.md | read → recent |
| 2026-05-29-preferences-pane.md | docs/superpowers/specs/2026-05-29-preferences-pane-design.md | **dedupe** vs Gen-1 preferences-pane — is this a re-plan/superseder of the 2026-05-20 one? Read both. |

**Older design docs — `docs/specs/*.md`:** `2026-05-18-mdash-design.md`, `2026-05-18-viewer-design-notes.md` — historical design artifacts. Link as the `spec:` of the relevant early change(s) if one maps cleanly (mdash relates to the rename/positioning work; viewer to the render pipeline); otherwise leave in place and reference from the change body. **Never moved** (frozen historical artifacts, like ADRs).

> **Spec/plan links are historical, not relocated** (docket §10 "Archived files don't move their spec/plan"). For Gen-1 `done` changes, the `plan:` link points at the existing `docs/plans/<file>` (and `spec:` at a `docs/specs/<file>` where one applies); for Gen-2 changes, `spec:` points at the existing `docs/superpowers/specs/<file>`. Do not move these files into `archive/`.

---

## Task 0: Verify prerequisites

**Files:** none (read-only checks)

- [x] **Step 1: Confirm docket skills are installed and invocable**

```bash
for s in docket-new-change docket-implement-next docket-finalize-change docket-status docket-adr; do
  ls -d "$HOME/.claude/skills/$s" >/dev/null 2>&1 && echo "ok $s" || echo "MISSING $s — run docket-skill-set plan + link-skills.sh first"
done
```
Expected: five `ok` lines. If any `MISSING`, stop and complete the docket-skill-set plan.

- [x] **Step 2: Confirm the Markhaus working tree is clean and on `main`**

```bash
cd /Users/homer/dev/macmd
git status --porcelain | head && git rev-parse --abbrev-ref HEAD
```
Expected: empty porcelain output (clean tree), branch `main`. If dirty, stash or commit existing work before migrating.

---

## Phase A — Bootstrap docket in Markhaus

### Task 1: Create `.docket.yml` and the change/ADR directory skeleton

**Files (in `/Users/homer/dev/macmd`):**
- Create: `.docket.yml`
- Create: `docs/changes/active/` `docs/changes/archive/` (dirs)
- Create: `docs/changes/README.md`

- [x] **Step 1: Write `.docket.yml` (defaults are correct for Markhaus, but make it explicit)**

```yaml
# .docket.yml — committed; read by every docket skill at startup
metadata_branch: main
changes_dir: docs/changes
adrs_dir: docs/adrs
```

- [x] **Step 2: Create the change directories + static README**

```bash
cd /Users/homer/dev/macmd
mkdir -p docs/changes/active docs/changes/archive
```
Write `docs/changes/README.md` (small **static** blurb — not generated):

```markdown
# Changes

docket's backlog for Markhaus. Each change is one markdown file (≈ one PR) with a status lifecycle.

- `active/` — non-terminal changes (proposed, in-progress, blocked, deferred, implemented)
- `archive/` — terminal changes (done, killed), filename prefixed with the UTC merge/kill date
- **[BOARD.md](BOARD.md)** — the generated status board (never hand-edited)

Decisions live in [`../adrs/`](../adrs/). See the docket skills for the workflow.
```

- [x] **Step 3: Verify and commit**

```bash
cd /Users/homer/dev/macmd
test -f .docket.yml && test -d docs/changes/active && test -d docs/changes/archive && echo "ok skeleton"
git add .docket.yml docs/changes/README.md
git commit -m "chore: bootstrap docket (config + changes dir)"
```
Expected: `ok skeleton`. (The empty `active/`/`archive/` dirs are committed once they contain files in later tasks; git won't track empty dirs — that's fine.)

---

## Phase B — Migrate the ADR ledger (`docs/decisions/` → `docs/adrs/`)

### Task 2: Rename the directory and convert all 28 ADRs to frontmatter

**Files (in `/Users/homer/dev/macmd`):**
- Rename: `docs/decisions/` → `docs/adrs/` (preserve git history)
- Modify: each `docs/adrs/0001-….md … 0028-….md` (prose-header → frontmatter)
- Modify: handle the static `docs/decisions/README.md` (Task 3 regenerates it)

- [x] **Step 1: Rename the directory (history-preserving)**

```bash
cd /Users/homer/dev/macmd
git mv docs/decisions docs/adrs
git commit -m "refactor: rename docs/decisions -> docs/adrs (docket ledger)"
```

- [x] **Step 2: Apply the conversion recipe to every `00NN-*.md`**

For each ADR file, transform the prose header into docket frontmatter, **keeping the `## Context` / `## Decision` / `## Consequences` body verbatim**. The mechanical recipe:

```
Source (prose header):                       Target (frontmatter):
  # ADR-0024: <Title>            ─────────▶    title: <Title>          (text after the colon)
  **Date:** 2026-05-20           ─────────▶    date: 2026-05-20
  **Status:** Accepted           ─────────▶    status: Accepted        (see status map for the 5 non-plain ones)
  (filename 0024-quicklook-…)    ─────────▶    id: 24, slug: quicklook-v1-interaction-limitations
  (body cross-refs to ADR-NN)    ─────────▶    relates_to: [NN]        (only if the body references another ADR)
  (produced by a plan)           ─────────▶    change: <id>            (back-link; set in Task 5 after change ids exist, or leave empty)
```

Resulting frontmatter block prepended to each file (example for a plain one):

```yaml
---
id: 24
slug: quicklook-v1-interaction-limitations
title: Quick Look extension v1 ships with known interaction limitations
status: Accepted
date: 2026-05-20
supersedes: []
reverses: []
relates_to: []
change:
---
```

Then the original `## Context` / `## Decision` / `## Consequences` sections follow unchanged. Remove the now-redundant `# ADR-NNNN: …` H1 and the `**Date:**`/`**Status:**` lines (their content moved to frontmatter).

- [x] **Step 3: Apply the 5 non-plain statuses + the link graph (from the Source Inventory table)**

- `0022-post-enrollment-signing-posture.md`: `status: Accepted`, `reverses: [1, 7, 19]`.
- `0001-…`, `0007-…`, `0019-…`: `status: Reversed by ADR-0022`.
- `0021-rename-to-markhaus.md`: `status: Accepted`, `supersedes: [15]`.
- `0015-mdash-name-and-positioning.md`: `status: Superseded by ADR-0021`; **append** a dated `## Update` note preserving "only the name is superseded; pricing & positioning remain in force" (partial-supersede note).
- `0027-page-size-and-margins-via-pagedjs.md`: `status: Accepted`, `supersedes: [25]`.
- `0025-pdf-page-size-via-webview-frame.md`: `status: Superseded by ADR-0027`.

- [x] **Step 4: Verify every ADR converted correctly**

```bash
cd /Users/homer/dev/macmd
bad=0
for f in docs/adrs/00*.md; do
  head -1 "$f" | grep -qx -- '---' || { echo "NO FRONTMATTER: $f"; bad=1; }
  grep -q '^id: ' "$f" && grep -q '^status: ' "$f" && grep -q '^date: ' "$f" || { echo "MISSING FIELD: $f"; bad=1; }
  grep -qF '## Decision' "$f" || { echo "NO ## Decision: $f"; bad=1; }
done
# link-graph spot checks:
grep -q 'reverses: \[1, 7, 19\]' docs/adrs/0022-*.md && echo "ok 0022 reverses"
grep -q 'supersedes: \[15\]'      docs/adrs/0021-*.md && echo "ok 0021 supersedes"
grep -q 'supersedes: \[25\]'      docs/adrs/0027-*.md && echo "ok 0027 supersedes"
grep -q 'Superseded by ADR-0021'  docs/adrs/0015-*.md && grep -qF '## Update' docs/adrs/0015-*.md && echo "ok 0015 partial-supersede"
[ "$bad" = 0 ] && echo "ok all 28 ADRs have frontmatter + sections"
```
Expected: the four `ok …` link lines + `ok all 28 ADRs …`, no `NO/MISSING` lines.

- [x] **Step 5: Commit**

```bash
cd /Users/homer/dev/macmd
git add docs/adrs/00*.md
git commit -m "refactor: convert 28 ADRs to docket frontmatter (preserve numbers + supersede/reverse graph)"
```

### Task 3: Regenerate + validate the ADR index via `docket-adr`

**Files:** Modify `docs/adrs/README.md` (regenerated)

- [x] **Step 1: Invoke `docket-adr`'s Index/validate action**

Run the `docket-adr` skill's **Index / validate** action against `docs/adrs/`. It must:
- (Re)render `docs/adrs/README.md` grouped **Active / Superseded-Reversed / Deprecated**, each row like `- [ADR-0024](0024-quicklook-v1-interaction-limitations.md) — Quick Look extension v1 ships with known interaction limitations (Accepted)`, with the superseded/reversed group annotating both endpoints (e.g. `0001 (Reversed by ADR-0022)` and `0022 … → reverses 0001, 0007, 0019`).
- **Validate** and report: numbering gaps (expect none — 1..28 contiguous), dangling `supersedes`/`reverses`/`relates_to` links (expect none), and status inconsistencies (expect none — every "Reversed by ADR-0022" has 0022 listing it in `reverses`, etc.).

- [x] **Step 2: Verify the index**

```bash
cd /Users/homer/dev/macmd
R=docs/adrs/README.md
grep -qiE 'superseded|reversed' "$R" && echo "ok groups present"
grep -q '0028-' "$R" && grep -q '0001-' "$R" && echo "ok spans full range"
# All 28 ADRs referenced in the index:
n=$(grep -oE '\(00[0-9][0-9]-[a-z0-9-]+\.md\)' "$R" | sort -u | wc -l | tr -d ' ')
[ "$n" = 28 ] && echo "ok all 28 linked" || echo "ONLY $n linked"
```
Expected: `ok groups present`, `ok spans full range`, `ok all 28 linked`. Address any validation warnings the skill reported before committing.

- [x] **Step 3: Commit**

```bash
cd /Users/homer/dev/macmd
git add docs/adrs/README.md
git commit -m "docs: regenerate ADR index via docket-adr (Active/Superseded/Reversed groups)"
```

---

## Phase C — Migrate plans + results → changes

### Task 4: Classify each plan and draft its change file

**Files (in `/Users/homer/dev/macmd`):**
- Create: one `docs/changes/active/<id>-<slug>.md` (non-terminal) or `docs/changes/archive/<date>-<id>-<slug>.md` (terminal) per source plan
- Read-only: `docs/plans/*`, `docs/*-results.md`, `docs/superpowers/{specs,plans}/*`, `docs/specs/*`, app source, `git log`

Work the **Source Inventory** tables top to bottom. For each plan:

- [x] **Step 1: Classify by reading (the reconcile-style pass)**

Read the plan, its `*-results.md` (if any), the governing ADRs' status, and `git log --oneline -- <relevant paths>`. Assign:
- `status: done` if the work shipped (results file present, or code + Accepted ADR confirm it). Use the **completion date** as the archive prefix — the results file's date / the last relevant commit date in **UTC** (`TZ=UTC git log -1 --date=format-local:%Y-%m-%d --format=%ad -- <path>`), since there is no merge commit to read.
- `status: in-progress` if a branch/code exists but it's unfinished.
- `status: proposed` if planned but not started.
Record the chosen status **and the one-line reason** in the change's `## Reconcile log` (this migration *is* a reconcile against current reality).

- [x] **Step 2: Assign the id and write the change file**

Allocate ids sequentially by `created` date across **all** migrated changes (Gen-1 then Gen-2), lowest date = lowest id. Write from docket's `change-template.md`:
- `created:` = the plan file's date; `updated:` = today (UTC) or the completion date for `done`.
- `spec:` = the matching `docs/superpowers/specs/<file>` for Gen-2 changes, or a `docs/specs/<file>` design doc where one applies to a Gen-1 change; otherwise empty (set `trivial: true` only if it truly had no design — most did, so leave `spec:` empty rather than lying with `trivial`).
- `plan:` = the existing plan path (`docs/plans/<file>` or `docs/superpowers/plans/<file>`). These are historical — **not moved**.
- `adrs:` = the governing ADR ids from the inventory table.
- `depends_on:` / `related:` = cross-links you observe (e.g. the pdf-pagination followup `depends_on` the pdf-export change; the quicklook followup `related` to / `depends_on` the quicklook change).
- Body: `## Why` + `## What changes` + `## Out of scope` distilled from the plan at PM altitude. **For `done` changes, fold the `*-results.md` summary into the body** (a `## Outcome` subsection or into `## What changes`) so the result is captured in the change — the results file itself stays in place as the historical artifact, linked.

- [x] **Step 3: Place the file in the right directory**

- terminal (`done`/`killed`) → `docs/changes/archive/<completion-date>-<id>-<slug>.md`
- non-terminal → `docs/changes/active/<id>-<slug>.md`

- [x] **Step 4: Dedupe the two preferences-pane plans**

The Gen-1 `2026-05-20-preferences-pane.md` (+ results) and Gen-2 `2026-05-29-preferences-pane.md` (+ spec) may be the same feature re-planned. Read both: if Gen-2 supersedes Gen-1, make **one** change reflecting current reality (link both plans; the earlier as historical), not two. If they are genuinely distinct slices, make two changes with a `related:` link. Record the decision in the `## Reconcile log`.

- [x] **Step 5: Verify the change set**

```bash
cd /Users/homer/dev/macmd
# Every change file has the required frontmatter fields:
bad=0
for f in docs/changes/active/*.md docs/changes/archive/*.md; do
  [ -e "$f" ] || continue
  for k in id slug title status priority created updated; do
    grep -q "^$k:" "$f" || { echo "MISSING $k in $f"; bad=1; }
  done
done
# ids are unique:
ids=$(grep -h '^id:' docs/changes/active/*.md docs/changes/archive/*.md 2>/dev/null | awk '{print $2}' | sort)
[ "$(echo "$ids" | wc -l)" = "$(echo "$ids" | uniq | wc -l)" ] && echo "ok unique ids" || echo "DUPLICATE IDS"
# archive filenames are date-prefixed:
for f in docs/changes/archive/*.md; do [ -e "$f" ] || continue; basename "$f" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]+-' || echo "BAD ARCHIVE NAME: $f"; done
[ "$bad" = 0 ] && echo "ok change frontmatter complete"
```
Expected: `ok unique ids`, `ok change frontmatter complete`, no `MISSING`/`DUPLICATE`/`BAD` lines.

- [x] **Step 6: Backfill ADR `change:` back-links**

Now that change ids exist, set each ADR's `change:` field to the id of the change that produced it (from the inventory `Governing ADRs` column — e.g. ADR-0024 ← the quicklook change; ADR-0027/0026 ← the pdf-pagination change). Leave empty for foundational ADRs (0005, 0008) with no single producing change.

- [x] **Step 7: Commit**

```bash
cd /Users/homer/dev/macmd
git add docs/changes docs/adrs/00*.md
git commit -m "feat: migrate Markhaus plans+results into docket changes; backfill ADR change links"
```

---

## Phase D — Verify with `docket-status` + optional live-loop proof

### Task 5: Generate the board and run health checks (the verification gate)

**Files:** Create `docs/changes/BOARD.md` (generated)

- [x] **Step 1: Run `docket-status`**

Invoke the `docket-status` skill in `/Users/homer/dev/macmd`. It must:
- Regenerate `docs/changes/BOARD.md` wholesale: count summary, emoji-grouped status sections, per-group tables with priority chips + clickable `spec`/`plan`/`pr` links, the build-ready vs needs-brainstorm / waiting-on-#N split, a Mermaid `depends_on` graph, and a collapsible Done section spanning the archive.
- Run the **merge sweep** (likely a no-op — migrated `done` changes are already archived; any `implemented` change with a real merged PR would be swept).
- Run the **health checks** and report.

- [x] **Step 2: Confirm the board and triage health output**

```bash
cd /Users/homer/dev/macmd
test -f docs/changes/BOARD.md && echo "ok board exists"
grep -qiE 'done|in progress|proposed' docs/changes/BOARD.md && echo "ok status groups"
grep -q '```mermaid' docs/changes/BOARD.md && echo "ok dependency graph"
```
Expected: `ok board exists`, `ok status groups`, `ok dependency graph`.

Review the health-check output. **Expected/acceptable** for migrated history:
- A `plan:` that resolves on a `done` change (the historical plan still exists) → fine, no link rot.
- No stale `in-progress` unless a change was genuinely classified `in-progress`.
- No `depends_on` cycles; no dangling ADR links (Task 3 already validated those).

Investigate and fix any *unexpected* warning (e.g. a `spec:` link that doesn't resolve, a dependency pointing at a wrong id) before committing.

- [x] **Step 3: Commit**

```bash
cd /Users/homer/dev/macmd
git add docs/changes/BOARD.md
git commit -m "docs: generate docket BOARD.md for Markhaus backlog"
```

### Task 6 (optional but recommended): Prove the live loop on one open change

If Task 4 produced at least one genuinely open, build-ready change (e.g. the quicklook-interactions followup, if it has a `spec:` or is `trivial`), prove docket end-to-end on real work:

- [ ] **Step 1: Run `docket-implement-next`** — ⏭️ SKIPPED: migration produced no build-ready change (onboarding is `proposed` but needs-brainstorm: no `spec:`, `trivial: false`), so `docket-implement-next` has nothing to select. Per this task's own contingency, the live loop runs later once an open change is brainstormed via `docket-new-change`.

Invoke `docket-implement-next` in `/Users/homer/dev/macmd`. Confirm it: syncs+sweeps (step 0), selects that change by the deterministic order, claims it (`in-progress` + `branch:`), reconciles (appends a `## Reconcile log` entry, `reconciled: true`), creates a `feat/<slug>` worktree off `origin/main`, runs `superpowers:writing-plans` → `subagent-driven-development` → `requesting-code-review`, records any ADRs via `docket-adr`, opens a PR, sets `status: implemented` + `pr:` on `main`, and **stops** at the merge gate.

- [ ] **Step 2: Close the loop (human)** — ⏭️ SKIPPED: depends on Step 1 (no live loop was run).

After reviewing the PR, run `docket-finalize-change` (or merge on GitHub then run `docket-status`'s sweep) and confirm the change archives to `done` with the UTC merge-date prefix and the feature branch/worktree are cleaned up. This is the full §12 smoke path on real Markhaus work.

> If no change is cleanly build-ready, **skip Task 6** and note it — Tasks 1–5 already prove the migration; the live loop can run later when an open change is brainstormed via `docket-new-change`.

---

## Self-Review (run by the plan author against spec §12 + the real repo)

**1. Coverage of §12's stated dogfood:**
- "Migrate `docs/plans/` + `*-results.md` into `docs/changes/`; completed plans become `done` changes (results folded into the body)" → Task 4 (Steps 1–3, results-folding in Step 2). ✓
- "Open work … becomes a real `in-progress`/`implemented` change" → Task 4 Step 1 classification + Task 6 live loop. ✓ (The spec's specific `feat/quicklook-interactions`/`0007` example is corrected to current reality — decision #6.)
- "`docs/decisions/` renamed to `docs/adrs/`, ADRs carry over (prose headers converted to frontmatter)" → Task 2. ✓
- "Exercising `docket-adr`'s index render against a real multi-ADR ledger" → Task 3, against 28 ADRs with 3 reversals + 2 supersessions. ✓
- "Proves the lifecycle and the PR gate end-to-end" → Tasks 5 (board/health) + 6 (live loop). ✓

**2. Real-data accuracy:** the ADR status map (0001/0007/0019 reversed-by-0022; 0015 superseded-by-0021 partial; 0025 superseded-by-0027) is taken from the actual `**Status:**` lines in `/Users/homer/dev/macmd/docs/decisions/`. The plan/results pairing and the two-generation structure (`docs/plans` vs `docs/superpowers/plans`) reflect the actual `ls`. The preferences-pane duplication is real and explicitly handled (Task 4 Step 4).

**3. Placeholder scan:** the ADR conversion is a single mechanical recipe applied to an *enumerated* list of 28 files with the 5 non-uniform cases spelled out — not a placeholder. Change classification is necessarily read-driven (no one can assign `done`/`proposed` without reading the plan + code), so the plan gives the deterministic method + inputs + starting table rather than fabricating final statuses; each verification step asserts structural completeness. No "TBD"/"handle appropriately" steps.

**4. Dependency consistency:** every skill this plan invokes (`docket-adr` Index/validate, `docket-status`, `docket-implement-next`, `docket-finalize-change`) is built by the prerequisite docket-skill-set plan; Task 0 gates on their presence. ADR ids (1–28) and change ids (sequential from 1) live in separate namespaces, consistent with docket's convention.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-30-markhaus-docket-migration.md`.** It depends on the docket-skill-set plan (Tasks 1–9) being done first. Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks. Task 4 (classify + draft changes) benefits most from review, since it is read-and-judge work on real history.

**2. Inline Execution** — `superpowers:executing-plans`, batch with checkpoints (natural checkpoints after Phase B, after Phase C, and before the optional live loop).

**Which approach — and run it now, or after the docket skills are built?**
