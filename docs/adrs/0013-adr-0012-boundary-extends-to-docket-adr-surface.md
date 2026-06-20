---
id: 13
slug: adr-0012-boundary-extends-to-docket-adr-surface
title: ADR-0012's script-vs-model boundary extends to the docket-adr surface
status: Accepted
date: 2026-06-20
supersedes: []
reverses: []
relates_to: [12, 7, 2]
change: 30
---

## Context

The scripting sweep (changes 0022/0023/0025/0026) moved every deterministic pass out of the docket-status / docket-finalize-change family into tested shell, governed by ADR-0012's rule: script a pass when it is mechanical (no reading intent from free text) and free of terminal-transition side effects shared across callers (or, when shared, owned by one jointly-owned script). docket-adr was the one operating skill the sweep never reached — three of its passes (ADR index render, ADR ledger validation, ADR-only terminal-publish) stayed model-prose even though each passes ADR-0012's test cleanly. The index render in particular drifted: docs/adrs/README.md carries "this index is generated — do not hand-edit" yet had no generator, so the published copy on main diverged from the authoritative docket copy (hand-embellished titles, stray backticks).

## Decision

Change 0030 extends ADR-0012's boundary to the docket-adr surface. The two read-only mechanical passes become script-owned analogs of existing siblings: render-adr-index.sh (↔ render-board.sh) and adr-checks.sh (↔ board-checks.sh). The ADR-only terminal-publish — a shared terminal-transition mechanic — is folded into the existing terminal-publish.sh as an --adr mode rather than duplicated per-caller, making one script the single executor of both publish shapes (--id change-publish, --adr ADR-only). This is the literal application of ADR-0012's "shared terminal-transition mechanics owned by one script, never duplicated per-caller." No ADR/board semantics change — faithful re-implementation.

## Consequences

- docket-adr now matches the other operating skills: deterministic passes are tested shell invoked from the skill body; the model no longer re-derives index/validation/publish by hand each run.
- The index becomes "same ADR files ⇒ byte-identical," so hand-drift self-heals on the next render. Regenerating drops index-only embellishments that were never in an ADR's frontmatter title — a deliberate cost of determinism; richer index text must now live in the ADR's title field (the single source).
- terminal-publish.sh is the single executor of both publish shapes; the by-hand ADR-only runbook in docket-finalize-change becomes documentation of what the script does.
- Next-id allocation stays in-model (welded to the CAS-push-rename retry) — explicitly out of scope; at most a future lib/ helper.
