---
id: 265
slug: branch-the-adr-0065-quote-leg-diagnostic-so-it-stops-claimin
title: 'Branch the ADR-0065 quote-leg diagnostic so it stops claiming a truncation that did not happen'
status: proposed
priority: medium
type: fix
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [255]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced at 0255's whole-branch review (finding 6, minor) and deliberately deferred at close-out: fixing it in-branch would have contradicted 0255's spec Assumption 7 (existing diagnostic strings stay byte-identical) and #0181's explicit out-of-scope on diagnostic wording, both settled at groom time.

**Opportunity** — when ADR-0065's *quote* leg fires, the diagnostic reads `value '"claude-opus-5"' is not a bare scalar — the reader consumes only '"claude-opus-5"'`: the same string on both sides of "consumes only". The wording was written for the *whitespace* leg, where the two genuinely differ and the truncation is the point. On the quote leg there is no truncation, so the sentence tells the user nothing and reads as a bug in the validator. Branch the message per firing leg.

**Independent value** — stands with 0255 reverted: the wording is inherited from the 0173 twin and is misleading wherever the quote leg exists at all. 0255 only raised the cost by propagating it to two more validators.

**Boundary** — diagnostic *wording* for the quote leg, in the validators that judge an `agents:` config value, plus whatever sentinels pin those strings. It does not touch the firing predicates, the strip order, the `#` leg, or which values are accepted — no behavior change beyond the text a user reads on refusal.

**Reason for deferral** — it inverts an assumption 0255's spec settled at groom time (byte-identical diagnostics), so it needs a human's call rather than a build decision, and reversing it mid-branch would have invalidated 0255's own sentinels.
