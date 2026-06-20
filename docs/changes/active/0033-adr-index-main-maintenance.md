---
id: 33
slug: adr-index-main-maintenance
title: Decide how the ADR index is maintained on the integration branch
status: proposed
priority: medium
created: 2026-06-20
updated: 2026-06-20
depends_on: [30]
related: [30, 22]
adrs: [1, 13]
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Why

The ADR index `docs/adrs/README.md` on the integration branch (`main`) is badly
stale — it lists only ADR-0001/0002 while the authoritative `docket` copy lists
all of them. Change 0030 shipped the deterministic generator
(`render-adr-index.sh`) and wired `docket-adr` to regenerate the index, but that
regeneration writes to **`docket`** only. `terminal-publish.sh` copies the change
file + spec + Accepted ADR *files* — it does **not** copy `README.md`, and nothing
else publishes the index to `main`. So the `main` copy will **not** self-heal: it
stays stale indefinitely under current tooling. (0030's spec assumed "the next
index-render pass heals it" — true for `docket`, false for `main`.)

This is a design decision, not a back-fill chore: the ADR index is a *derived
view*, like `BOARD.md` — and `BOARD.md` is deliberately **never** published to
`main`. So the real question is whether the ADR index should live on `main` at all.

## What changes

Decide and implement one of two coherent models:

- **(a) Treat the index like `BOARD.md`** — a `docket`-only derived view. Delete
  the stale `docs/adrs/README.md` from `main`; the ADR *files* remain on `main` as
  the durable ledger, the index is browsed on `docket`.
- **(b) Maintain it on `main`** — add `docs/adrs/README.md` to the
  terminal-publish copy-set (or a dedicated publish step) so the regenerated index
  is refreshed on `main` with each terminal publish.

Whichever is chosen, the stale `main` index is resolved as a side effect (removed,
or refreshed) — no manual history rewrite.

## Out of scope

- The generator/validator themselves (shipped in 0030) — unchanged.
- `BOARD.md`'s never-published-to-main rule — referenced as precedent, not changed.

## Open questions

- (a) vs (b) — which model? (a) is simpler and consistent with `BOARD.md`; (b)
  keeps the ledger index browsable alongside the code on `main`.
- If (b): does the index ride every terminal publish, or a dedicated pass? Does a
  kill-publish also refresh it?
- If (a): is anything (docs, links) relying on `docs/adrs/README.md` existing on
  `main`?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
