---
id: 180
slug: apply-adr-0065-s-quote-leg-to-hd-validate-and-the-remaining
title: Apply ADR-0065's quote leg to hd_validate and the remaining flow-map truncation corners
status: killed
priority: medium
type: fix
created: 2026-07-31
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [173]
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

ADR-0065 (change 0173) established that a bare-scalar validator built only on a
`consumed != raw` comparison is incomplete: that comparison is precisely a test for
"the value contains internal whitespace", so a quoted **but space-free** value
(`{model: "claude-opus-5"}`) has `consumed == raw` and slips through with its quote
characters riding into the emitted pin verbatim.

Change 0173 applied the fix to `validate_user_agent_values` in `sync-agents.sh` only.
The identical gap is still live in `hd_validate` in `scripts/lib/harness-defaults.sh` —
deliberately out of 0173's scope (that file was fixed by change 0168 and the spec
excluded it), and recorded here rather than left as an unrecorded observation.

A second, related corner surfaced in the same whole-branch review: `harness_agent_line`
in `sync-agents.sh` strips comments (`sed 's/#.*//'`) **before** either reader runs, so a
flow-map value containing a `#` (`{model: c#5}`) yields `raw == consumed == "c"` and passes
the gate — the same silent-truncation class this family of changes exists to close,
surviving in a corner the validator structurally cannot see. Unlikely for real model IDs,
but it is the same defect wearing different clothes.

## What changes

- Apply ADR-0065's quote leg to `hd_validate` in `scripts/lib/harness-defaults.sh`, so the
  shipped-defaults reader enforces the same rule its own diagnostic already prescribes.
- Decide and record what to do about the pre-reader comment strip in `harness_agent_line` —
  either make the readers see the raw line, or state why a `#`-bearing value is out of contract.
- Regression coverage at value level for both; the truncation is silent, so a test asserting
  only "generation succeeded" passes against the bug.

## Out of scope

- Any vendor model allowlist or availability lookup (ADR-0015 forbids it).
- Re-litigating ADR-0065 itself.

## Why killed

Consolidated into #0255 at the 2026-08-07 backlog triage: the hd_validate quote leg (correctness half) and #0181's rule documentation land together under ADR-0065.
