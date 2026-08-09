---
id: 265
slug: branch-the-adr-0065-quote-leg-diagnostic-so-it-stops-claimin
title: 'Branch the ADR-0065 quote-leg diagnostic so it stops claiming a truncation that did not happen'
status: proposed
priority: low
type: fix
created: 2026-08-08
updated: 2026-08-09
depends_on: []
related: [256, 267]
discovered_from: [255]
adrs: []
spec: docs/superpowers/specs/2026-08-09-branch-the-adr-0065-quote-leg-diagnostic-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-branch-the-adr-0065-quote-leg-diagnostic-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-branch-the-adr-0065-quote-leg-diagnostic-design.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced at 0255's whole-branch review (finding 6, minor) and deliberately deferred at close-out: fixing it in-branch would have contradicted 0255's spec Assumption 7 (existing diagnostic strings stay byte-identical) and #0181's explicit out-of-scope on diagnostic wording, both settled at groom time.

**Opportunity** — when ADR-0065's *quote* leg fires, the diagnostic reads `value '"claude-opus-5"' is not a bare scalar — the reader consumes only '"claude-opus-5"'`: the same string on both sides of "consumes only". The wording was written for the *whitespace* leg, where the two genuinely differ and the truncation is the point. On the quote leg there is no truncation, so the sentence tells the user nothing and reads as a bug in the validator. Branch the message per firing leg.

**Independent value** — stands with 0255 reverted: the wording is inherited from the 0173 twin and is misleading wherever the quote leg exists at all. 0255 only raised the cost by propagating it to two more validators.

**Boundary** — diagnostic *wording* for the quote leg, in the validators that judge an `agents:` config value, plus whatever sentinels pin those strings. It does not touch the firing predicates, the strip order, the `#` leg, or which values are accepted — no behavior change beyond the text a user reads on refusal.

**Second leg (absorbed from #0267, killed pointing here, 2026-08-09 triage) — correct the stale
`field()` quote-handling claim in script contracts.** `scripts/render-learnings-index.md` still
states "`field()` returns the raw scalar with surrounding quotes intact" — wrong since change
0138: `field()` strips a matched quote pair; `field_raw()` is the accessor that preserves them.
It sits in the paragraph immediately after one #0244 rewrote (PR #184), so a reader trusting the
contract next to the corrected text gets the opposite of the truth. Sweep every `scripts/*.md`
contract for the same stale claim while the context is loaded (0138 changed the behavior
repo-wide). Docs only on that leg: no code, no accessor behavior, no new guard. Both legs are
corrections of text the 0138/0244/0255 quote-handling lineage left wrong; one small groom covers
them together.

**Reason for deferral** — it inverts an assumption 0255's spec settled at groom time (byte-identical diagnostics), so it needs a human's call rather than a build decision, and reversing it mid-branch would have invalidated 0255's own sentinels.

## What changes

Settled design in the linked spec (auto-groomed 2026-08-09; critic-gated, one revision round):

- **Leg 1** — at the three diagnostic sites sharing the "consumes only" clause (the awk diagnostic
  inside `validate_harness_defaults` and the bash `validate_user_agent_values`, both in
  `sync-agents.sh`, plus `hd_validate` in `scripts/lib/harness-defaults.sh`), branch the message on
  `consumed != raw`: truncation branch keeps today's message byte-for-byte; pure-quote branch
  (`consumed == raw`, leading quote) gets a new middle clause with no truncation claim. Shared
  prefix and remedy tail unchanged; firing predicates untouched; the runners-shim site untouched
  (it never claimed a truncation).
- **Leg 1 sentinels** — quote-leg fixtures gain paired asserts (quote-branch clause present,
  `consumes only` absent on that firing); truncation probes unchanged. Exact assert list derived by
  grep at plan time.
- **Leg 2 (absorbed from killed #0267)** — correct `scripts/render-learnings-index.md`'s
  "Dequoting" paragraph: `field_raw()` is the raw accessor; `field()`/`fm_field()` strip a matched
  quote pair (change 0138). Groom-time sweep found this the only stale `scripts/*.md` site; the
  build re-confirms the sweep. Docs only, no new guard.

## Out of scope

- Firing predicates, strip order, the `#` leg, which values are accepted.
- Shared-helper extraction (that is #0256 — forward-linked in `related:`; the two new branched
  copies are recorded there for the consolidator).
- Any ADR change — message text and a docs correction change no decision.
