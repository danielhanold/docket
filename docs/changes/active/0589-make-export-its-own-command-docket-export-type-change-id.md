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
spec:
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
<!-- docket:artifacts:end -->

## Why

Exporting an artifact is a different job from opening one, but the CLI spells it as a flag on open: `docket open <type> <change_id> --export`. That reads as "open this, and also export it", and several flags (`--output-path`, `--warnings`) only make sense together with `--export`, so `open` has to police combinations a dedicated command would not have. Now that the ticket integration planned by change 581 leans on the export for ticket bodies and attachments, export is a first-class job and should have a command named for it.

## What changes

- Add a top-level `docket export <type> [<change_id>]` command, `<type>` one of change, spec, plan, or results. It does what `docket open … --export` does today: the same clean copy, file name, destination default, `--output-path`, `--warnings`, and JSON result.
- Remove `--export`, and the flags that only work with it, from `docket open`, so `open` only opens.
- Give the new command its own capability-catalog entry, and point the `docket-export` skill (and any other skill or doc that names `open … --export`) at it.

Left to grooming: whether `docket export` opens the copy afterwards (today's `--export` opens it unless `--print` is given) or only writes it and prints its path; whether `docket open --export` is removed outright or kept briefly with a message naming the new command; and the catalog entry's id.

## Acceptance criteria

`docket export spec <id>` writes the same file `docket open spec <id> --export` writes today, with the same JSON result; `docket open` no longer accepts `--export`, `--output-path`, or `--warnings`; the capability catalog lists the export command, and the docket-export skill resolves its argv from that entry.

## Out of scope

Changing what the export writes (sections kept, link removal, warnings). Adding export for the board or pull requests. Changes to `docket open`'s other behavior.
