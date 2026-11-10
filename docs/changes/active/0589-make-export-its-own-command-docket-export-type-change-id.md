---
id: 589
slug: 'make-export-its-own-command-docket-export-type-change-id'
title: 'Make export its own command: docket export <type> <change_id>'
status: 'proposed'
priority: 'medium'
type: 'refactor'
deliverable: 'pr'
created: '2026-11-10'
updated: '2026-11-10'
depends_on: []
stacked_on:
related: [561, 578, 582]
discovered_from: [581]
adrs: []
spec: 'docs/superpowers/specs/2026-11-10-make-export-its-own-command-docket-export-type-change-id-design.md'
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
ticket_provider:
ticket_id:
ticket_url:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-10-make-export-its-own-command-docket-export-type-change-id-design.md](../../superpowers/specs/2026-11-10-make-export-its-own-command-docket-export-type-change-id-design.md) |
<!-- docket:artifacts:end -->

## Why

Exporting an artifact is a different job from opening one, but the CLI spells it as a flag on open: `docket open <type> <change_id> --export`. That reads as "open this, and also export it", and several flags (`--output-path`, `--warnings`) only make sense together with `--export`, so `open` has to police combinations a dedicated command would not have. Now that the ticket integration planned by change 581 leans on the export for ticket bodies and attachments, export is a first-class job and should have a command named for it.

## What changes

- Add a top-level `docket export <type> [<change_id>]` command, `<type>` one of change, spec, plan, or results, with its own capability-catalog entry (`export`). It writes the same clean copy `docket open … --export` writes today, with the same file name, destination default, `--output-path`, `--warnings`, and export fields in its JSON result.
- Opening: with no `--output-path` it writes to the default destination and opens the copy; with `--output-path` it writes there, prints the path, and opens nothing; `--print` writes and prints without opening, whatever the destination.
- Remove `--export`, `--output-path`, and `--warnings` from `docket open` outright, with no alias, so `open` only opens.
- Point the `docket-export` skill and the docs that name `open … --export` at the new command.

## Acceptance criteria

- `docket export spec <id> --print --json` writes the same file, with the same bytes, that `docket open spec <id> --export --print --json` writes today, and reports the same `exported_path`, `replaced`, `leftovers`, and `mentions`.
- `docket export spec <id>` opens the copy; with `--output-path` or `--print` it opens nothing and prints the path.
- `docket open` rejects `--export`, `--output-path`, and `--warnings` as unknown flags.
- The capability catalog lists `export`, and the `docket-export` skill resolves its argv from that entry.

## Out of scope

Changing what the export writes (sections kept, link removal, warnings). Adding export for the board or pull requests. Changes to `docket open`'s other behavior.
