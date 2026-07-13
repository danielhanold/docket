---
id: 67
slug: learnings-promotion-destination
title: Give the learnings ledger a promotion destination — it has no way to shrink
status: proposed
priority: medium
created: 2026-07-13
updated: 2026-07-13
depends_on: []
related: [6, 65]
adrs: [5]
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
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

Design the ledger's missing exit path. The likely shape — to be settled at grooming, not here:

- **A promotion destination.** Create the repo's `CLAUDE.md` (or decide, deliberately, that a different
  file is the right home) and define what belongs there: the durable, always-true project conventions,
  as distinct from the dated, provenance-carrying build-loop lessons that stay in the ledger.
- **Promote the graduating entries.** Several current entries read as settled convention rather than
  war story — e.g. "never `producer | early-exiting-consumer` under `pipefail`", "anchor a
  frontmatter-field edit to the first `---…---` block", "grep for a `--flag` with `grep -E -e`". Those
  are rules, not memories; they belong where they are always in context.
- **Make the promotion lever real in the harvest procedure.** `docket-finalize-change`'s step 2.5 and
  the convention's *Learnings ledger* both name promotion as a distill lever. Whatever this change
  decides, those two prose sources must end up describing something that actually exists.
- **Possibly revisit the cap itself.** ~300 lines was picked before anyone had watched the ledger
  behave. If the real steady state with a working promotion valve is 200, or 400, say so.

`ADR-0005` (close-out-only harvest) owns the surrounding policy — one writer, one moment, ledger never
published to the integration branch. This change should not disturb any of that; if it does, that is an
ADR-worthy decision and should be recorded as one.

## Out of scope

- **Changing who writes the ledger or when.** ADR-0005's close-out-only harvest, the single-writer rule,
  and the `(#<id>` idempotency probe all stand. This change is about the *exit* path, not the entrance.
- **Publishing the ledger to the integration branch.** ADR-0005 decided it stays on `metadata_branch` as
  working memory. Promotion moves *distilled conventions* to a permanent home; it does not publish the
  ledger itself.
- **Automating the distill.** Deciding whether a lesson has graduated into a convention is a judgment
  call. A script that mechanically evicts entries at a line count is not what is being asked for.

## Open questions

- **Is `CLAUDE.md` actually the right destination?** The convention names it, but docket's own durable
  rules arguably belong in `docket-convention` (which the convention text also names as a promotion
  target). CLAUDE.md is read by any agent in the repo; the convention is read by docket skills. Those
  are different audiences and the split may matter.
- **What is the promotion test?** What distinguishes "a lesson with a date and a PR number" from "a rule
  that is simply true"? Without a crisp test, promotion becomes another judgment call that gets deferred
  every harvest — and the valve stays shut in practice even once it exists.
- **Who runs the promotion — the harvest, or a human?** The harvest is autonomous-capable and runs at
  close-out. Promotion rewrites a doc that governs every agent in the repo. That may want a human gate.
- **Does the ~300 cap survive?** And is a line count even the right trigger, versus something like
  "entries older than N changes that no longer earn their context."

## Reconcile log
