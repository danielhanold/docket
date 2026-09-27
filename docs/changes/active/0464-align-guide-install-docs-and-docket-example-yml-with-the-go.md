---
id: 464
slug: 'align-guide-install-docs-and-docket-example-yml-with-the-go'
title: 'Align guide, install docs, and .docket.example.yml with the Go v1 config and CLI'
status: 'proposed'
priority: 'medium'
type: 'docs'
created: '2026-09-27'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [363, 371]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

While building the docs glossary (PR #344) we checked every documented config key and CLI example against the Go v1 binary (`internal/config/schema.go` and `capability.go`, `docket <verb> --help`). Several guide and install pages, the shipped `.docket.example.yml`, and parts of the `docket-convention` skill still describe the pre-Go-v1 behaviour. A reader who follows them writes config that the binary treats as obsolete or inert, or that **blocks every repository mutation**. They also run commands that don't exist. The glossary already states the binary's behaviour, so the long-form pages now contradict it.

Verified drift:

- **`metadata_branch` is obsolete** (change 0363), so there is no single-branch/main-mode opt-out. Still described as live in `docs/guide/where-the-metadata-lives.md` ("Single-branch mode: the opt-out"), `README.md` ("Status"), and `.docket.example.yml`. The convention still says `metadata_branch` resolves where PM commits land.
- **Any explicit `skills.*` value blocks mutation** (`dispDeferredActive`), including `auto` and values that repeat the default. `docs/install/workflow-roles.md`, the convention's *Skill layer* / config sample, and Tier C's "explicit `auto` authorizes inline" still teach rebinding as supported.
- **`agents.<h>.<a>.model/effort` in a repository layer blocks** (`dispAgentsLeaf`); only the global config may pin. `docs/install/models-and-effort.md` documents repo-committed and repo-local pins.
- **`agents…runner` blocks, and every `runners.*` key is inert.** Delegation was retired in change 0371, but `docs/install/delegating-across-harnesses.md` and the `runners:` block in `.docket.example.yml` present it as working.
- **`terminal_publish: true` blocks.** `docs/guide/landing-changes.md` ("Selective publish on close-out") still describes it copying records.
- **`finalize.gate: ci`/`both` block**; only `local`/`off` are honoured. The convention's `finalize` paragraph still lists all four as supported.
- **The `github` board surface blocks, and `github_project` is read by nothing.** `.docket.example.yml` still documents the GitHub mirror.
- **`runtime.bash` is obsolete in every layer**, yet `.docket.example.yml` shows it as live.
- **`docs/guide/capturing-work.md`, "Migrating to typed changes"**:
  - tells readers to run `docket status --digest-only --type untyped`, but `--digest-only` doesn't exist and `--type untyped` is refused as `invalid-input`;
  - claims a bare `docket status` commits and pushes, but it is read-only;
  - lists priorities without `critical`.
- **Learnings writers:** the convention says learnings writes are human curation only, but the binary exposes the typed `docket learning record` / `learning update` operations.

## What changes

- Rewrite each listed passage to match the Go v1 binary. A blocked or obsolete option should be stated as such, with the remedy the binary's diagnostic names; don't just delete the section. The reader who has the old config needs to learn why docket refuses.
- Update `.docket.example.yml` so no key is shown as live when the schema marks it obsolete, deferred, blocking, or inert. The example-file test must keep passing.
- Update the stale `docket-convention` passages: the config sample, the `metadata_branch` wording, the *Skill layer* rebinding, the Tier C "explicit auto" clause, the four `finalize.gate` values, and the learnings writers. Grooming should decide whether any other skill text depends on them.
- Replace the broken `capturing-work.md` recipe with one that works against the current CLI.
- Re-check every repoguard prose-contract row that pins a phrase on these pages, and update each row in the same commit as the phrase it guards.

## Out of scope

- Changing binary behaviour, for example re-enabling `skills.*` rebinding, delegation, or terminal publish. This change aligns the prose to the code, not the reverse.
- The glossary itself (PR #344), which already reflects the binary.
- Point-in-time records: archived changes, results, Accepted ADRs, and specs. They keep their historical wording.
- A guard that keeps doc snippets in sync with the capability catalog. That is worth its own change.
