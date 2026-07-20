---
id: 102
slug: finalize-require-pr-approval-has-no-layer-resolution
title: finalize.require_pr_approval has no layer resolution
status: proposed
priority: medium
created: 2026-07-19
updated: 2026-07-20
depends_on: []
related: [101]
discovered_from: [101]
adrs: []
spec: docs/superpowers/specs/2026-07-20-require-pr-approval-layer-resolution-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-require-pr-approval-layer-resolution-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-require-pr-approval-layer-resolution-design.md) |
<!-- docket:artifacts:end -->

## Why

`finalize.require_pr_approval` is documented as a config key (README, and change 0101's new
`.docket.yml.example` tags it `scope: any layer`), but it has **no layer resolution at all**:

- `scripts/docket-config.sh` never reads it — no `lcl`/`yaml_get`/`gbl` chain, no export key.
- `scripts/docket-config.md`'s resolved-values table has no row for it.
- It is absent from the coordination-key fence list (`scripts/docket-config.sh`, the
  `for _fkey in metadata_branch integration_branch ... terminal_publish` loop).
- Its only consumer, `skills/docket-finalize-change/SKILL.md`, reads `.docket.yml` directly.

The consequence: a user who sets `finalize.require_pr_approval: true` in `.docket.local.yml` or in
the global `~/.config/docket/config.yml` gets **silence** — the value is neither honored nor
warned-and-ignored. It is the one documented key whose advertised scope the implementation does
not deliver, and the failure mode is a merge gate the user believes is armed but is not.

Discovered while building change 0101, which had to annotate the key's scope honestly in
`.docket.yml.example` rather than use either standard scope tag.

## What changes

**Wire the key through the resolver as a genuinely global-able key**, delivering the `any layer`
scope the docs already promise:

- Add the layer-resolution chain in `scripts/docket-config.sh` alongside its two `finalize.*`
  siblings, and export `FINALIZE_REQUIRE_PR_APPROVAL`. Fail closed on a non-boolean — the house
  rule for every key added since 0064, and the right posture here, since defaulting a typo to
  `false` disarms a gate the user believes is armed. Deliberately **not** coordination-fenced.
- Add the `scripts/docket-config.md` table row and export-list entry.
- Change `docket-finalize-change` to read the exported value as its **sole channel**; delete its
  direct `.docket.yml` read. No fallback — a fallback sees only `.docket.yml`, so it would honor a
  machine-scoped value on one path and ignore it on the other, making this bug intermittent.
- Collapse the bespoke `scope:` annotation change 0101 added to `.docket.example.yml` back to the
  standard `any layer` tag, and correct any README claim that the key is repo-only or skill-read.

**Plus a drift guard**, because the underlying gap is that nothing connects "documented in
`.docket.example.yml`" to "resolved by `docket-config.sh`". A manifest test classifies every key in
the example file as either `resolved:<EXPORT_NAME>` (asserted to actually be emitted) or
`elsewhere:<consumer>` (a named non-resolver reader). An unclassified key fails. This fits ADR-0048's
existing charter for the example file and turns a class of bug into a red test.

One ADR records the rule: a documented config key resolves through `docket-config.sh`; a model-read
of `.docket.yml` is not a supported shape, with the `elsewhere:` allowlist as the named exception.

## Out of scope

- Changing what `require_pr_approval` *does* at the merge gate — this is resolution wiring only.
- Converting any other non-resolver key (`agents:`, `agent_harnesses:`, `github_project`,
  `runners.*`) — those have working consumers; this change classifies them, it does not move them.
- Reworking `yaml_get` or the flat-scalar reader.
