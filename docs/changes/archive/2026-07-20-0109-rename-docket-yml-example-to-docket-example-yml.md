---
id: 109
slug: rename-docket-yml-example-to-docket-example-yml
title: Rename .docket.yml.example to .docket.example.yml so editors syntax-highlight it
status: done
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: [107]
related: [101, 107, 108]
discovered_from: [101]
adrs: [48]
spec:
plan: docs/superpowers/plans/2026-07-20-rename-docket-yml-example-to-docket-example-yml.md
results: docs/results/2026-07-20-rename-docket-yml-example-to-docket-example-yml-results.md
trivial: true
auto_groomable:
branch: feat/rename-docket-yml-example-to-docket-example-yml
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/112
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-07-20-rename-docket-yml-example-to-docket-example-yml.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-20-rename-docket-yml-example-to-docket-example-yml.md) |
| Results | [2026-07-20-rename-docket-yml-example-to-docket-example-yml-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-20-rename-docket-yml-example-to-docket-example-yml-results.md) |
| PR | [#112](https://github.com/danielhanold/docket/pull/112) |
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

### 2026-07-20 — reconciled against `origin/main` @ 2748ed9

Scope holds: this is still a rename plus a reference sweep, and nothing about ADR-0048's three
invariants or the file's contents changes. Five refinements, all additive:

1. **`depends_on: [107]` is discharged.** Change 0107 is `done` (archived
   `2026-07-20-0107-guard-the-readme-config-snippet-against-docket-yml-example-d.md`); PR #110
   landed. The sequencing rationale in *Open questions* is satisfied — this change now lands the
   rename *after* the drift guard, exactly as intended, and absorbs the guard's pinned filename as
   part of the sweep.

2. **0107's guard needs no new file — but it does enlarge one.** The README-snippet drift guard
   landed *inside* `tests/test_docket_yml_example.sh` as a new numbered section `(8)`, not as a
   separate test. Its old-name references (comments plus `assert` description strings, around
   lines 469–543) are swept along with the rest of that file. The *What changes* file list is
   therefore still complete as written.

3. **New reference site: the example file points at its own guard.** `.docket.yml.example` names
   `tests/test_docket_yml_example.sh` at lines 6 and 41. Because the test file is renamed too,
   those two in-file pointers must be updated in the same pass — they were not called out when
   this change was drafted.

4. **The grep-clean exclusion set has grown.** *Out of scope* names change 0101's artifacts and
   ADR-0048's body. Four of 0107's artifacts now exist on the integration branch and are equally
   historical, so they join the exclusion set:
   `docs/changes/archive/2026-07-20-0107-guard-the-readme-config-snippet-against-docket-yml-example-d.md`,
   `docs/superpowers/specs/2026-07-20-readme-snippet-drift-guard-design.md`,
   `docs/superpowers/plans/2026-07-20-readme-snippet-drift-guard-plan.md`, and
   `docs/results/2026-07-20-readme-snippet-drift-guard-results.md`.
   So does **`docs/adrs/README.md`** (line 51), which carries the old filename only because it is
   *rendered* from ADR-0048's immutable title — `render-adr-index.sh` is its sole writer, so there
   is nothing to hand-edit there and the stale-looking name is correct output.

5. **The ADR `## Update` note is a metadata write, not a feature-branch edit.** ADR-0048 lives on
   `docket` (and, under this repo's `terminal_publish: true`, is mirrored onto `main`). Feature
   branches never modify docket metadata, so the dated `## Update` note is recorded via the
   `docket-adr` dispatch in step 6 — which also owns republication onto the integration branch.

Also verified: test discovery is glob-based — no runner, test, or CI file enumerates
`tests/test_docket_yml_example.sh` by name — so renaming it cannot silently drop it from the suite.

Not folded in (deliberately): active stubs 0102, 0103, 0106, 0108 and two learnings files name the
old filename on the metadata branch. That is metadata, not the code line; each stub's own reconcile
pass picks up the new name at build time. 0106 is `in-progress` under another agent — untouched.
