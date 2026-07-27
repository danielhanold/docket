---
id: 146
slug: widen-the-config-read-channel-guard-to-the-sibling-config-la
title: Widen the config read-channel guard to the sibling config layers it does not match
status: proposed
priority: medium
type: fix
created: 2026-07-27
updated: 2026-07-27
depends_on: []
related: []
discovered_from: [120]
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

To be settled at grooming. Sketch: widen the token to the set of config-file names ADR-0052's rule
actually covers, and tighten the occurrence test so a longer surrounding filename is not counted.
Both are edits to the single `scan_tree` function plus fixtures alongside the existing (a)-(g) set.

Note the widening is not free: `.docket.local.yml` and `config.yml` appear in the two declared
`docket-convention` exclusions and possibly elsewhere in legitimate prose, so the change must re-run
the audit and classify whatever the wider token surfaces — the same shape of work 0120 did, at the
wider scope. `config.yml` in particular is a generic name and will need a more careful match than a
bare substring.

## Out of scope

- Re-litigating ADR-0052 itself, or the two `docket-convention` exclusions 0120 declared.
- The marker syntax, the admissible class set, or the equal-count rule — all settled by 0120.

## Open questions

- Is `config.yml` specific enough to match on at all, or does it need path context to avoid false
  positives on unrelated prose?
- Should the three filenames share one class-marker vocabulary, or does a machine-local-only layer
  warrant its own class?
