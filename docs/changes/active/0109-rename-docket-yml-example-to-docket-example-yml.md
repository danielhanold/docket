---
id: 109
slug: rename-docket-yml-example-to-docket-example-yml
title: Rename .docket.yml.example to .docket.example.yml so editors syntax-highlight it
status: in-progress
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: [107]
related: [101, 107, 108]
discovered_from: [101]
adrs: [48]
spec:
plan:
results:
trivial: true
auto_groomable:
branch: feat/rename-docket-yml-example-to-docket-example-yml
claimed_at: 2026-07-20T14:05:21Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0048](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0048-docket-yml-example-invariants.md) |
<!-- docket:artifacts:end -->

## Why

Change 0101 shipped `.docket.yml.example` as the canonical config reference, and it works — but
the `.example` suffix lands *after* `.yml`, so neither editors nor GitHub recognize the file as
YAML. Both render it as plain text: no syntax highlighting, no folding, no structural cues. The
file's entire job is to be read, and the extension is undermining it.

`.docket.example.yml` keeps the same "this is an example" signal while ending in `.yml`, so every
YAML-aware tool highlights it correctly.

## What changes

Rename the file with `git mv` and update every **live** reference to the new name:

- the file itself, `.docket.yml`, and `README.md`
- `scripts/docket-config.sh` / `.md`, `scripts/ensure-global-config.sh` / `.md`,
  `scripts/github-mirror.md`
- the test suite: `tests/test_docket_yml_example.sh` (the test file's own name should follow the
  rename too) plus the references in `test_ensure_global_config.sh`, `test_finalize_gate.sh`,
  `test_install.sh`, `test_learnings_ledger.sh`, `test_sync_agents.sh`

ADR-0048 names the old filename in its title and body. It is `Accepted` and therefore immutable
except its `status:` line — so record the rename as a dated `## Update` note appended to ADR-0048,
not as an edit to the decision text. The decision itself is unchanged; only the artifact's name is.

A final `grep -rn 'docket\.yml\.example'` over the working tree (excluding the historical artifacts
listed below) must come back empty.

## Out of scope

- **Historical records are not rewritten.** Change 0101's archived change file, its spec, plan, and
  results, and ADR-0048's existing body are records of what happened under the old name; they keep
  it. Only the appended ADR `## Update` note acknowledges the rename.
- No change to the file's *contents*, structure, or the three invariants ADR-0048 established — this
  is a rename and a reference sweep, nothing more.
- No backward-compatibility shim, symlink, or dual-name lookup. The file is documentation, not an
  input any script reads at runtime, so a hard rename is safe.

## Open questions

None. `depends_on: [107]` exists only to sequence this behind PR #110, which adds a README-snippet
drift guard pinned to the old filename — landing the rename first would collide with it. Change
0108 (stub) touches the same README fences; whichever lands second absorbs the new name.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
