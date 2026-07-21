---
id: 53
slug: readme-yaml-fences-guarded-by-default-opt-out-marker-grammar
title: README yaml fences are guarded by default, with an opt-out marker grammar
status: Accepted
date: 2026-07-21
supersedes: []
reverses: []
relates_to: [48]
change: 108
---

## Context

Change 0107 added section `(8) README SNIPPET CORRESPONDENCE` to
`tests/test_docket_example_yml.sh`, guarding **one** README fence — the per-repo-settings
snippet — against drift from `.docket.example.yml` by value equality. Its own review surfaced
that the README carried eight other config fences that nothing guarded at all: a key renamed in
the resolver, or a key that never existed, would sit in the README indefinitely.

Two forces shaped the design:

1. **A hand-written fence list rots into the gap it was written to close.** The stub that
   proposed change 0108 enumerated the unguarded fences by line number, and that list was
   *already wrong when it was filed* — it omitted the `reclaim:` fence and named two files that
   do not exist. This is the repo's `enumerated-floor` rule (promoted to `AGENTS.md`) arriving as
   direct evidence.
2. **Value equality is not universally sound.** Most README config fences deliberately show
   NON-default values to illustrate opting in (`auto_capture: true`, `terminal_publish: true`,
   `metadata_branch: main`, and the two layered-config samples). Extending 0107's value-equality
   assert across them would go spuriously RED against correct prose — which is precisely why 0107
   scoped itself to one fence rather than looping.

## Decision

Every ` ```yaml ` fence in `README.md` is a guarded config fence by default; a fence opts out or
upgrades its assert through an HTML-comment marker, and a malformed marker is a hard failure.

- **Fence discovery is derived, never enumerated.** Section `(9)` scans `README.md` for yaml
  fences and puts every one in scope **by default**, so a new config fence is guarded the day it
  is written. The discovery regex is whitespace-tolerant and matches the closer at the same
  indent, because one fence is a list-item continuation indented two spaces (a column-0 regex
  structurally cannot see it — the design's own first draft shipped that bug).
- **The assert is existence-only by default:** each fence key must exist in
  `.docket.example.yml`. That is what makes one check applicable to every fence regardless of the
  values it illustrates.
- **Two markers, attached as the nearest preceding non-blank line:**
  - `<!-- docket:config-fence: ignore -->` — not `.docket.yml` schema; skip this fence entirely.
  - `<!-- docket:config-fence: values -->` — additionally assert value equality against the
    example. Applied to the `reclaim:` fence, whose `lease_ttl: 72` / `auto: false` are shipped
    defaults that *should* redden if the defaults move.
  - Attachment is the nearest preceding non-blank line rather than strictly the line above, and
    the marker may carry leading whitespace, because a column-0 HTML comment above an indented
    list-continuation fence would terminate the enclosing list.
- **An unknown or malformed token is a hard fail, never warned-and-ignored,** because the two
  mistake directions are asymmetric: a typo'd `ignore` fails safe (the fence is still checked and
  reddens loudly), but a typo'd `values` or a bare `<!-- docket:config-fence -->` fails **open and
  silent** — value coverage evaporates with no signal, which is the exact drift class the change
  exists to end.
- **The guard is anchored on `.docket.example.yml`, one hop.** Sections `(2a)`/`(2b)`/`(2c)`
  already bind the example to the resolver in both directions, so the README inherits resolver
  coverage transitively rather than introducing a second competing anchor.
- **The correspondence runs one way** (fence ⊆ example). The reverse loop is deliberately absent
  — "every example key appears in the README" is the fourth all-keys surface that change 0101
  deleted.

## Consequences

- **Enables:** a new README config fence is guarded automatically, with no registration step; a
  documented key that never existed, or one renamed in the resolver, reddens the suite.
- **Costs:** authors of *non-config* yaml fences in the README must now mark them `ignore` — the
  default-in choice deliberately puts the burden on the rarer case. Fence-count changes require
  bumping one literal (a sanctioned non-vacuity floor, whose assert message inlines the remedy).
- **Given up:** universal value equality. Seven fences are existence-only, so a wrong *value* in
  them is not caught — accepted because those fences illustrate non-defaults by design.
- **Residual holes, documented rather than closed:** a future fence key whose name collides with
  a prose-comment word in the example would be silently accepted (no collision exists among
  today's keys; the only tight closure would be an explicit allowlist, i.e. the enumerated floor
  this design exists to avoid). A yaml fence nested inside a wider four-backtick fence is
  discovered as a config fence (latent — the count floor trips first).
- **Non-vacuity is the live risk** and carries explicit floors: exact fence count, per-fence
  non-empty flatten, raw-vs-flattened cross-check, a population floor that every fence was
  visited, a floor that at least one fence stays values-marked, a whole-file reconciliation that
  every marker line is attached to a fence, and a positive control pinning that the `reclaim:`
  fence's value coverage is live regardless of which fence carries the marker. Two rounds of
  whole-branch review were needed to reach that set — the first two attempts each left a
  fail-open path where deleting or relocating the marker kept the suite green while a real value
  drifted.

Relates to ADR-0048 (`.docket.example.yml` as the tested canonical config reference this design
anchors on).
