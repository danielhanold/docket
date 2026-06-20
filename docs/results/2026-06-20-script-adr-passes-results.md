# Script docket-adr's deterministic passes — results
Change: #30 · Branch: feat/script-adr-passes · PR: (opened from this branch — see change 0030 `pr:`) · Plan: docs/superpowers/plans/2026-06-20-script-adr-passes.md · ADRs: 13 (new), cites 2, 7, 12

## Findings

- **ADR-0013 authored** — "ADR-0012's script-vs-model boundary extends to the docket-adr surface" (`relates_to: [12, 7, 2]`, `change: 30`). Records that the two read-only mechanical passes became script-owned analogs (`render-adr-index.sh` ↔ `render-board.sh`; `adr-checks.sh` ↔ `board-checks.sh`) and the ADR-only terminal-publish was folded into the shared `terminal-publish.sh --adr` rather than duplicated per-caller. Change-tied, so it rides change 0030's terminal publish at `done`.

- **Real-data smoke test — the index drift was broader than the plan predicted.** Running `render-adr-index.sh` against the live `.docket/docs/adrs/` ledger produced a clean, idempotent 12-ADR index (exit 0); `adr-checks.sh` reported a clean ledger (exit 0). The diff vs the committed `docket` `README.md` is **3 rows**, all hand-drift the generator heals:
  - ADR-0001 — `` `docket` `` backticks (index-only markdown not in the frontmatter `title:`).
  - ADR-0008 — index carried "; two-layer native precedence, on-demand generation, abort-and-report" beyond the canonical `title:`.
  - ADR-0009 — index carried ", never the designer skill" beyond the canonical `title:`.
  The plan's smoke-test step only predicted the ADR-0001 backticks. The renderer faithfully emits the frontmatter `title:` verbatim (verified it is not truncating) — these embellishments were index-only and are dropped on regeneration. This is the intended self-healing (spec §1): the index "is generated — do not hand-edit," so richer index text must now live in the ADR's own `title:` field (the single source). No back-fill of the stale `main` index is done here (out of scope); the next normal index-render pass heals it.

- **Plan inconsistency caught at build (resolved).** The plan's golden fixture omitted `← change #N` on a Superseded row (ADR-0004, `change: 4`) while the plan's verbatim renderer emitted change-refs unconditionally. Resolved per design spec §2 (only `→ supersedes`/`→ reverses` are parenthetically "Active rows"; `← change #N` is gated solely on "`change:` is set"): the renderer emits each annotation iff its field is non-empty — **no group special-casing** — and the golden was aligned. This is simpler than gating change-ref to Active and is the spec-faithful behavior. (Fix commit on the branch: `render-adr-index — emit change back-ref in all groups; align golden`.)

## Follow-ups

- **`adr-checks.sh` arm (b) is verb-agnostic.** It flags an un-flipped supersede/reverse target by matching only the target id, not the verb — so ADR-2 `reverses: [1]` with ADR-1 status `Superseded by ADR-0002` (wrong verb, right target) stays silent. Faithful to the original prose ("status flipped to point back," which never specified verb-matching); a verb-aware check would be a separate small change, not a defect here.
- **Numeric-`id:` trust is a shared assumption.** Both new scripts trust `field id` to be an integer (a malformed `id:` would corrupt a `declare -A` key / `MAXID` arithmetic). Identical to `render-board.sh` / `board-checks.sh` — a codebase-wide latent assumption, not introduced here. Out of scope.
- **Stale `main` ADR index history is not back-filled** (out of scope per spec). It heals on the next normal index-render pass once this tooling is on `main`.
