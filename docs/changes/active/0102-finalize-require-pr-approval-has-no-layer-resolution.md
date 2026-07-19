---
id: 102
slug: finalize-require-pr-approval-has-no-layer-resolution
title: finalize.require_pr_approval has no layer resolution
status: proposed
priority: medium
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: []
discovered_from: [101]
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

Pick one of two coherent end-states and make the docs match:

1. **Wire it through the resolver** — add the layer-resolution chain in `scripts/docket-config.sh`,
   export it (e.g. `FINALIZE_REQUIRE_PR_APPROVAL`), add the `scripts/docket-config.md` table row,
   and change `docket-finalize-change` to read the exported value instead of parsing `.docket.yml`.
   This delivers the `any layer` scope the docs already promise.
2. **Fence it as per-repo-only** — add it to the coordination-key fence loop so machine-scoped
   values are loudly warned-and-ignored, and retag it `repo-only` in `.docket.yml.example`,
   `scripts/docket-config.md`, and the README.

Option 1 matches what is already documented; option 2 is the smaller change. Either way the
`.docket.yml.example` annotation added by change 0101 collapses back to a standard scope tag.

## Out of scope

- Changing what `require_pr_approval` *does* at the merge gate.
- Any other unexported/model-read key (`agents:`, `agent_harnesses:`, `github_project`,
  `runners.*`) — those have working consumers; this one has none.

## Open questions

- Which end-state does Daniel want — resolver-wired (`any layer`) or fenced (`repo-only`)?
- If wired: does the finalize skill read the export directly, or keep its own read with the
  resolver as the fallback?
