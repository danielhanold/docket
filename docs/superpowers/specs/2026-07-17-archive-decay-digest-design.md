# Archive decay — a rolling one-line digest for the always-loaded board — design

**Date:** 2026-07-17
**Change:** #0093
**Status:** autonomously groomed (docket-auto-groom, 2026-07-17) — no human in the loop; every
decision below is a default-biased choice recorded in `## Assumptions` for deferred human review,
and the draft was gated by the `docket-auto-groom-critic` adversary.

## Problem

docket's archive only grows. This repo is already at **75 archived changes**, and `render-board.sh`
re-renders every one of them on **every** board pass in two always-loaded surfaces:

1. The **Archive `<details>` table** (`render-board.sh` lines ~224-243) emits one `| # | Title |
   Merged |` row per archived change — all 75 today, unboundedly more tomorrow.
2. The **Mermaid graph** (lines ~218-221) emits a `NNNN:::done` node for **every** done id in the
   archive, most of which carry no edge (no active change depends on them) — a growing list of
   floating green boxes.

`BOARD.md` is loaded into agent context routinely, so this cost is paid on every read and grows
without bound. The archive **change files** themselves are fine — they are read on demand, and they
are immutable historical records. The problem is confined to the **derived, always-loaded rendered
views**.

**Design guardrail (from capture, non-negotiable):** decay applies to **rendered views only**.
Archived change files, specs, and ADRs are never summarized-in-place, rewritten, deleted, or
collapsed on disk. This change edits `render-board.sh` output only; it touches no source record.

## Decision summary

Give `render-board.sh`'s archive rendering a **count-based recency window** over `done` entries plus
a **per-month digest** for older `done` entries; **`killed` entries are never collapsed** (they carry
abandonment signal worth keeping visible and are a rare, slow-growing minority). Separately, prune
the mermaid graph to only the done nodes an active change actually depends on. No config knob, no
caller change — `board-refresh.sh` and `docket-status.sh` call the renderer exactly as before.

**Two independent output changes with different blast radius** (the earlier draft wrongly claimed a
single "inert for small archives" property — corrected here, in Assumptions #4/#6, and in the test
plan):

- The **archive-table window** is *inert below the threshold*: a board whose `done` archive is at or
  below `ARCHIVE_RECENT` (so it has no collapsed rows) renders its **archive table** byte-identically
  to today.
- The **mermaid pruning** is *deliberate and universal*: it changes the mermaid block for **any**
  board that has a done archive id no active change depends on — essentially every real board,
  including the existing test golden (it drops `0012:::done`, keeps `0010:::done`). This is the
  intended improvement, not a regression.

Concretely, the Archive `<details>` block becomes:

- **Verbatim table:** all `killed` entries (any age) **plus** the `ARCHIVE_RECENT` most recent
  `done` entries, together sorted by the existing key (merged date desc, then id desc), rendered in
  the current `| # | Title | Merged |` table — unchanged shape (killed and recent done interleave by
  date). Bound: ≈ `ARCHIVE_RECENT` + (killed count), effectively flat since killed grow slowly.
- **Older done (collapsed):** `done` entries beyond the recent-`done` window rolled up into a
  **per-month digest**, one row per `YYYY-MM` bucket, newest first, e.g. `| [2026-06](archive/) | 29
  done |`, each row linking to the `archive/` directory (the browsable full detail — no new file).
  Buckets are derived from the archive filename's `YYYY-MM` prefix. Killed never appear here.

And the mermaid graph:

- A done id is styled `:::done` **only when it is referenced by an active change's `depends_on`**
  (i.e. it appears as a `PARENT` in an emitted edge). Unreferenced done ids — the ones with no edge
  — are dropped entirely. Referenced done nodes keep their green styling (id-sorted, as the code
  already sorts `DONE_IDS` numerically) so a reader still sees "this dependency is already merged."
  Emit the `classDef done` line only when at least one `:::done` node remains (avoids a dangling
  def; keeping it unconditionally is harmless if simpler).

`ARCHIVE_RECENT` is a single named constant at the top of `render-board.sh`, defaulted to **15**
(matching the stub's own `e.g. last 15`), trivially tunable in one place.

## Design details

### Archive section rewrite (`render-board.sh` ~L224-243)

The existing block already collects `ARCFILES`, `ARC_COUNT[done]`, `ARC_COUNT[killed]`, and sorts
rows by `date desc, id desc`. The rewrite:

1. Keep the `<details><summary>… Archive — <lbl> (total)</summary>` header exactly as today
   (emoji/label/total logic unchanged).
2. Partition the sorted rows into (a) the **verbatim set** — every `killed` row plus the first
   `ARCHIVE_RECENT` `done` rows in sort order — and (b) the **collapsed set** — the remaining
   (older) `done` rows. Render the verbatim set into the existing `| # | Title | Merged |` table in
   the existing sort order (byte-identical shape; killed and recent done interleave by date).
3. Collapsed `done` rows are **not** listed individually. Tally them into per-`YYYY-MM` counts using
   the filename's first 7 characters (`YYYY-MM`) as the bucket key.
4. After the verbatim table, if any collapsed rows exist, emit an "Older done (collapsed)" sub-block:
   a `| Month | Done |` table, newest bucket first, each row reading e.g. `| [2026-06](archive/) | 29
   done |`. The month cell links to the relative `archive/` directory — offline-safe, works on GitHub
   and locally, and requires no generated index file. Killed rows are never in this set.
5. When the archive's `done` count is `<= ARCHIVE_RECENT`, **no** "Older done (collapsed)" sub-block
   is emitted and the **archive table** is byte-identical to the pre-change renderer — the table
   window is inert until it is needed. (The mermaid pruning is separate and universal — see above.)

### Mermaid pruning (`render-board.sh` ~L218-221)

Replace "emit every done id as `:::done`" with "emit `:::done` only for done ids in the set of ids
referenced by some active change's `depends_on`." The referenced set is already computable from the
same active-file scan that emits the `PARENT --> CHILD` edges (lines ~205-217); collect those parent
ids, intersect with `DONE_IDS`, and emit `:::done` for the intersection only. Edge emission itself
(lines ~205-217) is **unchanged** — including the pre-existing behavior for a `depends_on` pointing
at a killed id, which is out of scope here.

### Determinism (load-bearing)

Every input is derived from the change files (sort order, filename date prefixes, `depends_on`
sets) — **no wall-clock**. Same change files → identical bytes, preserving `render-board.sh`'s
golden-byte invariant and the "commit only if bytes changed" board gate. This is *why* the recency
window is count-based, not "last 30 days" (see Assumptions #1).

## Assumptions

Each row is a decision an interactive brainstorm would have raised, the conservative default chosen,
the rejected alternatives, and why. All are rendering-only and fully reversible by a follow-up; none
requires private human context (business priorities, external constraints) — which is why this
groom emits a build-ready spec rather than abstaining.

1. **Recency window is count-based, not time-based.** Chosen: count-based (`last N`). Rejected:
   time-based (`last 30 days`). A time-based window reads wall-clock, so the *same* change files
   would render *different* bytes as time passes — breaking the renderer's determinism/idempotency
   invariant, the golden byte-compare test, and the board's "commit only if changed" gate. This is
   a correctness constraint, not a preference: time-based is effectively disqualified.

2. **Window size `ARCHIVE_RECENT = 15`.** Chosen: 15 (the stub's own example), as a single named
   constant. Rejected: 20/25 (equally valid, arbitrary). Pure rendering aesthetic; one-line tunable.

3. **Digest granularity is per-month.** Chosen: per-`YYYY-MM` (matches the stub's
   `2026-06: 31 changes done, 2 killed` example; the archive filename already carries the date, so
   `YYYY-MM` is a substring — no date math). Rejected: per-quarter (coarser, needs quarter
   arithmetic, no clear benefit).

4. **Only `done` collapses into the digest; `killed` is always listed verbatim.** Chosen: the
   recency window and per-month digest apply to `done` only; every `killed` entry renders verbatim
   regardless of age. Rejected: uniform windowing (collapse killed alongside done, digest line
   `29 done, 2 killed`). Rationale: the stub's `## Open questions` explicitly parks this AND records
   a human lean — "whether killed changes stay individually listed (they carry more signal than
   routine dones)." The conservative, human-lean-honoring default on an explicitly-open question is
   to keep killed visible; the cost is negligible (killed are a rare minority — this repo has ~1
   killed against ~75 done — so the "tightest bound" argument for collapsing them is dominated by the
   done rows, which collapse either way). The stub's illustrative digest line showed a killed count,
   but that example is superseded by the open-question lean; with killed verbatim the digest is
   naturally done-only. Fully reversible to uniform windowing by a follow-up if the human prefers it.

5. **Older-entries detail links to the `archive/` directory, no new generated file.** Chosen: each
   digest row links to the relative `archive/` path (browsable on GitHub and locally; date-prefixed
   filenames sort by month there). Rejected: generating an `ARCHIVE.md` full index — it is a *new*
   derived surface that itself grows unbounded, needs its own gated writer (per the
   `atomic-generated-write` learning: render→temp→`mv`, sentinel, board-pass wiring, tests), and is
   scope creep for a "keep the board cheap" change. Deferred as a possible follow-up if a per-month
   filtered view is ever wanted.

6. **Always-on, no config knob.** Chosen: a fixed constant. Rejected: a `.docket.yml` knob (e.g.
   `board_archive_window`). Grounds for the conclusion (independent of any inertness claim): a knob
   would be a coordination key — the board is a derived view identical for all clones, so it is
   per-repo-only (ADR-0019) — would demand the end-to-end knob treatment (sample config + README +
   prose, per the `config-knob-ship-end-to-end` learning), and there is no evidence different repos
   need different windows (YAGNI). Zero config still serves every repo: the mermaid pruning helps
   every board universally, and the archive-table window helps large repos while leaving small ones'
   archive table byte-identical. (The earlier draft over-stated this as a single universal "byte-
   identical" property — corrected in the Decision summary and the test plan.) A knob remains a
   clean, additive follow-up.

7. **The GitHub board surface is exempt — no change.** Chosen: leave `github-mirror.sh` untouched.
   Rationale: the cost problem is specific to the always-loaded inline `BOARD.md` + agent context;
   GitHub Issues are queried on demand, natively paginated/filterable, and the mirror already
   *closes* issues on done/killed (closed issues are hidden by default). The stub's own parenthetical
   ("Issues scale fine") is the steer. Rejected: mirroring the decay into Issues (disproportionate,
   no benefit).

8. **Learnings-ledger index decay is OUT of scope — recommendation only.** Chosen: this change
   touches the board only. Rationale: the board archive is a *chronological* record where age
   predicts low value (routine old dones), so recency-decay fits. The learnings index
   (`render-learnings-index.sh` → `learnings/README.md`) is a *relevance-indexed* hint surface
   grouped by topic/slug, **not** chronological — a finding from a year ago may be the exact one
   that bears on today's change, so recency-decay would actively harm it. The learnings surface
   already has its correct compaction lever: promotion (graduate to AGENTS.md → drop from the
   `learnings.cap` count) plus human-gated consolidation when the cap flags needs-curation
   (ADR-0041, change #0067). Recommendation for the human: if the learnings index ever grows
   uncomfortably, tune `learnings.cap` / consolidate — do not bolt recency-decay onto it. Rejected:
   folding a learnings-index decay into this change (wrong value model; would duplicate/undermine
   #0067's promotion valve).

## Code changes

Single script plus its tests and contract; no caller, no config, no new file.

1. **`scripts/render-board.sh`** — introduce the `ARCHIVE_RECENT` constant; rewrite the archive
   `<details>` block (verbatim window + per-month "Older (collapsed)" table); prune the mermaid
   `:::done` emission to depends_on-referenced done ids. No new flags; signature unchanged.
2. **`scripts/render-board.md`** — update the **Archive section** and **Dependency graph (Mermaid)**
   behavior paragraphs to describe the window, the per-month done digest, and the referenced-only
   done nodes. Note the **archive-table-only** inert / byte-identical property (mermaid pruning is
   universal, not inert) and the count-based determinism rationale.
3. **`tests/test_render_board.sh`** — the golden byte-compare is the regression guard.
   - **The existing golden fixture MUST be updated** (it is *not* unchanged): its archive has only
     3 entries, so its **archive table** stays byte-identical, but the **mermaid pruning changes it** —
     the referenced-only rule drops `0012:::done` (nothing depends on #12) and keeps `0010:::done`
     (#0002 `depends_on: [10]`). Update the fixture's expected mermaid block accordingly; do not
     assert "unchanged."
   - **Add a dedicated large-archive fixture** with `> ARCHIVE_RECENT` `done` entries spanning
     multiple months plus at least one killed, and assert: (a) every killed row and the recent
     `ARCHIVE_RECENT` done rows render verbatim; (b) older done entries appear only as per-month
     `Done` digest rows with correct counts, and no killed row is collapsed; (c) the mermaid graph
     styles `:::done` only for done ids an active change depends on, dropping the rest; (d) re-running
     the renderer twice yields identical bytes (determinism).

## Out of scope

- Deleting, rewriting, or summarizing-in-place any archived change file, spec, or ADR — decay is
  rendered-views-only. ADRs are never archived or decayed (guardrail).
- Throughput / cycle-time analytics over the archive — **#0010** owns that; the per-month
  date-bucketing here may be shared with it later, but this change ships no analytics.
- Learnings-ledger index decay (Assumptions #8) — recommendation only, not built here.
- The GitHub board surface (Assumptions #7) — exempt.
- A config knob and an `ARCHIVE.md` full index (Assumptions #5, #6) — deferred follow-ups.

## Dependency state

`depends_on: []` — nothing gates this. `related: [10, 67]` — #0010 (archive analytics, may share
date-bucketing) and #0067 (learnings promotion valve, the learnings-side compaction lever
referenced in Assumptions #8). Neither is a build prerequisite; the implementer's reconcile pass
re-validates against what has actually merged at build time.
