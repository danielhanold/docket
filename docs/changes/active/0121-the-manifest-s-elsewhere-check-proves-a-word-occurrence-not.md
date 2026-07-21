---
id: 121
slug: the-manifest-s-elsewhere-check-proves-a-word-occurrence-not
title: The manifest's elsewhere: check proves a word occurrence, not a real config read
status: proposed
priority: medium
created: 2026-07-21
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [102]
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

Change 0102 shipped a classification manifest in `tests/test_docket_example_yml.sh` binding every
config key documented in `.docket.example.yml` to either a real resolver export
(`resolved:<EXPORT>`) or a named non-resolver consumer (`elsewhere:<file>`).

The `elsewhere:` half proves less than it claims. The check is a word-boundary grep for the key
name in the named consumer file — it proves the key name **occurs** there, never that it is
genuinely **read** there. Change 0102's own review demonstrated the gap: a key classified
`elsewhere:sync-agents.sh` passed because `\btimeout\b` matched English prose inside an embedded
dispatch prompt ("Make exactly ONE foreground Bash call, with the maximum timeout (600000)").

0102 narrowed this — targets are now constrained to a declared allowlist of five real consumer
files rather than any path — but those five include a 400+ line SKILL.md and the resolver script,
which between them carry a large English vocabulary. The residual is documented honestly in
ADR-0052 and deliberately deferred there.

The manifest's own stated design rule is that an `elsewhere:` entry must be *anchored on consuming
code, never a bare allowlist* — this is the remaining distance to that rule.

## What changes

Tighten the `elsewhere:` check so it evidences a real read rather than a word occurrence. Options
to weigh:

- Require the match to be a code-shaped read (an assignment, a variable reference, a `yaml_get`/
  `field_of`-style call naming the key) rather than any occurrence.
- Require the match to fall outside comment/prose regions of the consumer.
- Have each consumer declare the config keys it reads in a machine-readable header, and check
  against that — moving the anchor onto the consumer where it belongs.

Pick one deliberately; each trades false-red risk against strength.

## Out of scope

- The `resolved:` half, which change 0102 already ties back to the key's own leaf name.
- Re-opening whether the manifest should exist.
