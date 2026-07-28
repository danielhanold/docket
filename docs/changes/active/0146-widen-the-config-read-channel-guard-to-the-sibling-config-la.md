---
id: 146
slug: widen-the-config-read-channel-guard-to-the-sibling-config-la
title: Widen the config read-channel guard to the sibling config layers it does not match
status: proposed
priority: medium
type: fix
created: 2026-07-27
updated: 2026-07-28
depends_on: [120]
related: [147]
discovered_from: [120]
adrs: [52]
spec: docs/superpowers/specs/2026-07-28-config-read-channel-guard-widening-design.md
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
| Spec | [2026-07-28-config-read-channel-guard-widening-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-config-read-channel-guard-widening-design.md) |
| ADRs | [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0120 shipped `tests/test_config_read_channel.sh`, the prose-side enforcer for ADR-0052: every
occurrence of `.docket.yml` in `skills/**/*.md` must carry an in-line class marker or the suite
fails. The whole-branch review found the guard honest against the drift it was built for, and named
two bounded limitations in **what it matches** — both out of scope for 0120, both reachable by the
same mistake the ADR exists to prevent.

**1. Sibling config layers are invisible.** The scanned token is exactly `.docket.yml`. docket
documents two more config layers a skill could just as wrongly be told to read — the machine-local
`<repo>/.docket.local.yml` and the user-level `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`.
Verified during review: appending a line instructing an agent to read `.docket.local.yml` to
`skills/docket-status/SKILL.md` leaves the guard PASSing. ADR-0052's rule is about the config file,
not about one of its three filenames, so the guard is currently narrower than the decision it enforces.

**2. The occurrence test is a substring match.** `case "$line" in *"$TOKEN"*)` counts
`myconfig.docket.yml.bak` as an occurrence. Harmless today (no such string exists in skill prose)
and it errs toward over-reporting rather than fail-open, but it means the occurrence count the
equal-count rule depends on is not quite the count of real references.

## What changes

Widen `tests/test_config_read_channel.sh`'s scanned token from the single `.docket.yml` to the set
`{.docket.yml, .docket.local.yml, config.yml}`, closing a reproduced fail-open: today an unmarked
instruction to read `.docket.local.yml` or the user-level `config.yml` leaves the suite PASSing.

- **Both match sites widen** — the per-line prefilter and the counting `grep -oF`. Widening only the
  counter silently preserves the bug.
- **The third token is bare `config.yml`**, not path-qualified: `agent-layer.md` refers to the layer
  bare twice, so the bare form is docket's in-house phrasing and the likeliest evasion. Bare also
  keeps the token set overlap-free, so per-token counts sum exactly.
- **One shared class vocabulary** (`write-back` / `negative`) for all three filenames — the classes
  describe what a line says, not which layer it names.
- **The substring occurrence test is deliberately NOT tightened.** It over-reports (fail-safe, never
  admits an unmarked occurrence), no superstring occurrence exists in the tree, and the obvious
  boundary-anchored tightening consumes its boundary character and undercounts adjacent occurrences —
  re-opening the per-line fail-open change 0120 closed.
- **Audit result: the widening reclassifies nothing.** All thirteen sibling-filename occurrences in
  `skills/**` sit inside the two exclusions 0120 already declared; zero new markers are needed. That
  zero cost is load-bearing on those exclusions, which this change does not own.
- **ADR-0052 gains a new dated `## Update`** (never an edit — it is Accepted): its 2026-07-27 note
  describes the enforcer as covering "every `.docket.yml` occurrence", which goes stale here.

Fixtures extend the existing (a)-(g) set, including ground truth that a single `.docket.local.yml`
counts once rather than twice, and a token-set-membership floor — necessary because every real-tree
occurrence of the new tokens is excluded, so the real-tree scan exercises them zero times.

Design, rejected alternatives, and the full audit table are in the linked spec.

## Out of scope

- Re-litigating ADR-0052 itself, or the two `docket-convention` exclusions 0120 declared.
- The marker syntax, the admissible class set, or the equal-count rule — all settled by 0120.
- `tests/test_docket_example_yml.sh`, ADR-0052's key-side enforcer (see `related: [147]`).

## Open questions

Both resolved at grooming (see the spec's Assumptions 2 and 3):

- *Is `config.yml` specific enough to match on at all?* **Yes, bare.** Path context was proposed and
  rejected: docket's own convention reference uses the bare form, so a qualified token would miss the
  likeliest spelling. The generic-filename cost is bounded by the `skills/**` population and measured
  at zero occurrences outside the exclusions.
- *Should the three filenames share one class-marker vocabulary?* **Yes.** The classes are
  layer-independent; a machine-local-only class would force marker renames on layer changes.
