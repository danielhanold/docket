---
id: 403
slug: 'surface-config-diagnostics-with-file-line-when-a-command-ref'
title: 'Surface config diagnostics with file:line when a command refuses on invalid configuration'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-03'
updated: '2026-09-03'
depends_on: []
stacked_on:
related: [392]
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

Several repos fail `docket repository check` on an invalid `.docket.yml`, and the only thing docket prints is `repository check: unsupported-config: invalid configuration` — no file, no key path, no line. The same bare message comes out of `repository init`, `repository migrate`, `repository prepare`, and `status`. The config resolver already produces one diagnostic per defect, each carrying the source file, the key path, a message, an optional remedy, and a provenance line and column; both call sites that resolve config for these commands (`resolveSetupConfig` for the repository family and `loadOperationalContext` for every operating command) discard that slice and wrap only the sentinel error. `docket diagnostic config --repo-dir <dir>` does print the diagnostics, but nothing in the refusal points the user at it, and even its human renderer omits the line number its JSON provenance carries. A user with a broken config is left to bisect the file by hand.

## What changes

Carry the resolver's diagnostics on the error at both resolve sites, and lift them into each refusing operation's existing `findings` array — one finding per diagnostic, coded by the diagnostic's own code, with `ref` set to `<source>:<line>` and the diagnostic's remedy carried through — so `repository check`, `repository prepare`, `repository init`, `repository migrate`, and `status` all name the offending line(s) in both human and JSON output. Introduce one shared human line renderer for a config diagnostic that includes `<file>:<line>`, and switch `docket diagnostic config` to it so its human output gains the line number too. Design detail lives in the linked spec.

## Out of scope

Relaxing any diagnostic's severity or tolerating unknown keys on any path (change 0392 owns install-path tolerance). New diagnostic classes. Changing any exit code or result vocabulary. A separate `diagnostics` array on the result documents — the existing `findings` shape is reused. Pointing the refusal at `docket diagnostic config` as the remedy: the diagnostics are embedded instead.
