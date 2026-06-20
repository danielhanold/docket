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
auto_groomable: false
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

## Auto-groom blocked

**2026-06-20 — docket-auto-groom abstained** (designer committed a default; the
`docket-auto-groom-critic` returned *needs human context* on the load-bearing
decision). No spec emitted; `auto_groomable` flipped to `false`. Returned to the
interactive `docket-groom-next` queue.

**The undecidable decision:** *(a) vs (b)* — should a generated ADR index live on
the integration branch at all? The autonomous designer committed to **(a)**
(`docket`-only derived view; delete the generated index from `main`, leave a static
pointer stub, and **author a new convention-setting ADR** cementing the rule). The
critic refused that default on two independent grounds:

1. **The decision turns on the owner's intent, not repo-derivable fact.** The
   stub's own framing ("the index is like `BOARD.md`, which is never published")
   relies on an analogy the critic found imperfect. The asymmetry is real and
   load-bearing: `BOARD.md`'s source (active change files) is *never* on `main`, so
   indexing it there would be incoherent; the ADR index's source (Accepted ADR
   *files*) **is** on `main`, so an index alongside them is coherent and arguably
   serves the browse-the-code-line audience — a legitimate reason to prefer **(b)**.
   The authoritative `main:README.md` states *"`BOARD.md` is the one artifact that
   never leaves `docket`"* — a **singular** rule. Choosing (a) therefore *extends* a
   one-artifact rule into a general "derived views are `docket`-only" class rule:
   that is a convention **change**, not an application, and which way it should go
   is the owner's call.

2. **An autonomous pipeline should not cement a meta-convention ADR.** Emitting the
   spec would make this build-ready, and the autonomous builder would then delete a
   file from `main` *and* author an immutable, convention-setting **ADR** about
   docket's own publish model (plus tighten `docket-convention` / `docket-adr`
   wording). An Accepted ADR is the hardest artifact to walk back; writing one to
   settle an open convention question about the tool itself, with no human in the
   loop, is exactly what the abstain rule exists to prevent. (Note: this makes (a)
   arguably *less* conservative than (b) — (a) is irreversible-by-convention, (b) is
   additive tooling.)

**Also surfaced (a defect in the autonomous draft, recorded so the next groom
doesn't repeat it):** the draft justified its "static pointer stub on `main`" by
claiming it mirrors the existing static `README.md` blurbs under
`changes_dir`/`adrs_dir`. False on the relevant surface — `docs/changes/README.md`
exists only on `docket`; on `main` the *only* `README.md` under those dirs is the
generated ADR index being deleted. There is no static-folder-landing-page precedent
on `main` to lean on.

**Verified facts a human can build on:**
- `terminal-publish.sh`'s copy-set excludes `README.md` in both `--id` and `--adr`
  modes — so option (a) needs **no** script change to stop the index re-publishing.
- **No hard dependency** on the `main` index: no script/test reads it (`adr-checks.sh`
  and `render-adr-index.sh` both exclude `README.md`); the top-level `README.md`
  only names "the ADR index" in a one-line role description (no link). The remaining
  dependency is the **soft human-browse** one — which is the intent question above.
- A naive **(b)** ("add `README.md` to the copy-set") is subtly buggy: docket's index
  is regenerated at ADR *accept* time but ADR files reach `main` only at their
  change's *terminal* publish, so copying docket's index verbatim onto `main` would
  emit rows linking to ADR files not yet published → dangling links. A correct (b)
  would **re-render** from main's own ADR set in a dedicated main-side pass.

**What a human should supply (`docket-groom-next`):**
1. Pick the model — **(a)** treat all derived views as `docket`-only (extending the
   singular `BOARD.md` rule into a class rule), or **(b)** keep the index browsable
   on `main` alongside the published ADR files (re-rendered main-side, per the
   dangling-link wrinkle above).
2. Confirm whether the resulting convention should be recorded in a new ADR
   (relates_to ADR-0001/0013) decided interactively.

**Recommendation:** not a kill or defer — this is real, worth-doing work; it only
needs the owner's intent on (a)-vs-(b). Re-arm by answering above, flipping
`auto_groomable` back to `true` (or grooming it interactively), and deleting this
section.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
