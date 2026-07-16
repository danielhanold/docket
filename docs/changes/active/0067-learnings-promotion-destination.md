---
id: 67
slug: learnings-promotion-destination
title: Give the learnings ledger a promotion destination — it has no way to shrink
status: in-progress
priority: medium
created: 2026-07-13
updated: 2026-07-16
depends_on: []
related: [6, 65]
adrs: [5]
spec: docs/superpowers/specs/2026-07-16-learnings-promotion-destination-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/learnings-promotion-destination
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-16-learnings-promotion-destination-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-16-learnings-promotion-destination-design.md) |
| ADRs | [ADR-0005](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0005-close-out-only-harvest.md) |
<!-- docket:artifacts:end -->

## Why

The learnings ledger has a growth valve that is welded shut, and change 0065's close-out is where it
finally showed.

The convention's *Learnings ledger* section says the ledger is append-only until ~300 lines, and that
"the next harvest past the cap also distills — merge near-duplicates, **drop entries promoted to
CLAUDE.md or this convention**." So distillation has exactly two levers: merge duplicates, and
promote durable conventions out to a permanent home.

**The second lever does not exist in this repo.** There is no `CLAUDE.md` on `main`. There never has
been. So every harvest can only ever merge near-duplicates — and near-duplicates are a finite,
shrinking resource. Once the obvious families are merged, the ledger has no mechanism left to get
smaller, and it grows monotonically with every change that ships a lesson.

That is exactly what happened at 0065's harvest (2026-07-13). The ledger was **382 lines** — well past
the ~300 cap. The distill merged five genuine near-duplicate families (sentinel anchoring; green-suite
-never-exercised-the-branch; goal-scoped review; enumerated counts; sibling design) and folded #11's
two mirror bugs together, preserving every distinct rule. Net result after *also* adding 0065's two new
entries: **367 lines**. About 31 lines of real compression — and the ledger is still 67 lines over cap
with the cheap merges now spent. The next harvest will be worse off, not better: fewer duplicates left,
same inflow.

The remaining ~43 entries are, as far as the harvest could tell, genuinely distinct lessons. Cutting
them to hit 300 would be destruction, not distillation — precisely the line the convention draws
("compression, not destruction"). So the harvest correctly stopped, and correctly escalated instead of
quietly blowing the cap.

There is a second-order cost. The ledger is read at three hot moments — `docket-groom-next` before a
brainstorm, `docket-implement-next` at plan time and at review. Every line is context those agents pay
for on every run. A ledger that can only grow is a context tax that can only rise, and the lessons most
worth loading (the stable, always-true conventions) are exactly the ones that should have graduated out
of it into a doc the agent reads anyway.

## What changes

The exit path is now **designed** — the spec (groomed 2026-07-16) settles the shape, and it went
further than "add a promotion destination": the single capped file becomes an **index + detail**
structure with a real shrink valve. PM-altitude summary; mechanics live in the spec:

- **The ledger becomes a findings directory.** `<changes_dir>/learnings/` — one curated finding file
  per lesson/family (`<slug>.md`, bare slug, no ordinal), plus a **generated** `README.md` index
  rendered by a new `render-learnings-index.sh` joining the derived-view family.
- **Reads become pay-per-relevance.** The three hot readers load the small index and pull only the
  findings that bear on the change at hand, instead of paying for 490 lines of history every run.
- **Promotion becomes real, and human-gated.** The harvest marks a must-fire-unprompted rule
  `candidate`; a human graduates it to the integration-branch agent-instructions file and flips
  `promoted`. Promoted findings leave the paid surface and the cap's view — the actual shrink valve.
  Target is `AGENTS.md`/`CLAUDE.md`; neither exists on `main` today, so the design recommends
  creating `AGENTS.md`.
- **A wholesale off switch + a cap that flags.** New `learnings:` config block — `enabled` (default
  `true`; a read/write gate, never a purge) and `cap` (default 300, now counting **active findings**,
  not raw lines). Past the cap the loop flags `needs you`; it never auto-merges its own memory.
- **Migration is the acceptance proof.** The current 490-line ledger is converted into finding files
  + index; `LEARNINGS.md` is left as a pointer stub.

`ADR-0005` (close-out-only harvest) owns the surrounding policy — one writer, one moment, ledger never
published to the integration branch. The design preserves all of it in substance: only ADR-0005's
founding *consequence* ("short enough to actually be read") is what failed. The restructure is recorded
as one new ADR that `relates_to` ADR-0005 rather than superseding it.

## Out of scope

- **Changing who writes the ledger or when.** ADR-0005's close-out-only harvest, the single-writer rule,
  and the `(#<id>` idempotency probe all stand. This change is about the *exit* path, not the entrance.
- **Publishing the ledger to the integration branch.** ADR-0005 decided it stays on `metadata_branch` as
  working memory. Promotion moves *distilled conventions* to a permanent home; it does not publish the
  ledger itself.
- **Automating the distill.** Deciding whether a lesson has graduated into a convention is a judgment
  call. A script that mechanically evicts entries at a line count is not what is being asked for.

## Open questions

All four are **resolved by the spec** (2026-07-16); recorded here as the answers, not the questions:

- **Is `CLAUDE.md` the right destination?** → The target is the repo's always-in-context
  agent-instructions file at the integration-branch root — `AGENTS.md` **or** `CLAUDE.md`,
  harness-agnostic and symlink-aware. Neither exists on `main`, so the design recommends creating
  `AGENTS.md` (the neutral spelling) — a human decision the loop surfaces, never takes.
- **What is the promotion test?** → *"Will the agent know to search for this?"* A rule that must fire
  **unprompted** graduates; a war story stays in retrieval.
- **Who runs the promotion?** → **Human-gated.** The harvest proposes (`promotion_state: candidate`)
  and never touches the integration branch; a human lands the edit and flips `promoted`.
- **Does the ~300 cap survive?** → Yes, but re-based: it now counts **active findings**
  (`retained` + `candidate`), not raw lines, and is configurable via `learnings.cap`. Promoted
  findings stop counting, which is precisely what makes the valve work.

Remaining unknowns are tracked as risks in the spec's §7, not here.

## Reconcile log

- **2026-07-16** — Reconciled at claim time by `docket-implement-next`. The spec was groomed the same
  day, so this pass is a verification rather than a refresh; every load-bearing factual claim was
  re-checked against current reality and holds:
  - Ledger is **490 lines** across **33** top-level entries (spec says 491 — a trailing-newline
    off-by-one, not drift). The cap-breach premise stands.
  - **Neither `AGENTS.md` nor `CLAUDE.md` exists on `origin/main`** (verified via `git ls-tree`) — the
    "promotion destination does not exist" premise, which is the whole motivation, is still true.
  - Cited ADRs **0005, 0012, 0019, 0028, 0030, 0031, 0032 all exist and are `Accepted`**. ADR-0005 is
    unchanged, so the `relates_to`-not-supersedes decision holds. Highest ADR id is **0040** ⇒ the new
    ADR lands at **0041** (assigned by `docket-adr`, not hard-coded).
  - The stated analogs exist: `scripts/render-adr-index.sh` (+ contract) for the renderer,
    `scripts/lib/docket-frontmatter.sh` for the no-YAML-loader parse, `board-refresh.sh`/`render-board.sh`
    for the gated-writer-wraps-pure-renderer split. Skills live in-repo under `skills/`, so the prose
    edits are integration-branch product code as §4.8 states.
  - **Body refreshed**: `## What changes` still read as pre-grooming ("to be settled at grooming, not
    here") and understated the change — grooming reshaped it from "add a promotion destination" into an
    index+detail restructure. Rewritten to the settled shape at PM altitude. `## Open questions` folded
    to their resolved answers.
  - **In-flight scan** — two active changes mention learnings, neither conflicts:
    - **#0018** (yq YAML parsing) is `proposed`, `low`, no spec — an *evaluation* stub. 0067
      deliberately parses via the existing frontmatter lib, so it is unblocked; if 0018 ever adopts yq,
      the finding-file frontmatter simply becomes another consumer. Noted, no action.
    - **#0084** (`terminal_publish` opt-in default) is `implemented`, PR open, unmerged. It is a
      separate concern; the only interaction is possible textual overlap if both edit convention/README
      prose. Kept additive to stay rebase-resolvable (the 2026-07-16 #79 learning).
  - **Verdict: build as specced.** Not obsolete, not invalidated; scope unchanged.
