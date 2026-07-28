---
id: 152
slug: consolidate-the-two-surviving-hand-rolled-gnu-bash-4-validat
title: Consolidate the two surviving hand-rolled GNU Bash 4+ validator copies
status: proposed
priority: medium
type: refactor
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: [133, 150, 153]
discovered_from: [133]
adrs: []
spec: docs/superpowers/specs/2026-07-28-consolidate-bash4-validator-copies-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-28-consolidate-bash4-validator-copies-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-consolidate-bash4-validator-copies-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0133 centralized docket's `runtime.bash` mechanics into `scripts/lib/docket-runtime.sh` and
routed three callers through it. Its whole-branch review found the GNU Bash 4+ **validator** still
has two more hand-rolled copies, deliberately out of 0133's scope: `scripts/docket.sh`'s POSIX
bootstrap prologue and `scripts/ensure-docket-env.sh`'s own `DOCKET_BASH_PATH` validation. Three
implementations, so a future correction to the version grammar still lands in only one.

## What changes

**`scripts/docket.sh`'s prologue stays — as a documented exception.** The stub's open question is
answered: not because the library fails to parse under `sh` (it does not — it is bootstrap-compatible
by requirement and runs under `dash`), but because `docket_runtime_validate_bash` splits its two-line
payload with `$'\n'`, which degrades to a literal under a non-bash `/bin/sh` and returns a **wrong
answer with exit 0**. The prologue also has no `BASH_SOURCE` and no `SELF_DIR` before its `exec`, so
it could not locate the library without inventing its own path resolution.

**`scripts/ensure-docket-env.sh` consolidates.** Its five validation lines map 1:1 onto the library's
five reason tokens, so every `die` string is preserved verbatim and no library token changes. Its
second duplicate — `validate_literal_path`, byte-identical to `docket_runtime_serializable` — folds
too, following `ensure-global-config.sh`'s precedent.

**Coverage is the real deliverable.** Routing through the library does not by itself deliver the
stub's mutation bullet: `ensure-docket-env.sh` and `ensure-global-config.sh` both lack any Bash-3 or
non-GNU fixture, so breaking the library's major check leaves them green. Both need negative
fixtures, and `ensure-docket-env.sh`'s five diagnostics must be pinned *before* the detection moves,
since none is asserted today.

Design settled in the linked spec.

## Out of scope

- Behavior changes to 0133's three existing callers, the library's token interface, and its Bash 3.2
  compatibility requirement. Adding their missing mutation coverage is explicitly in scope.
- `_docket_runtime_scan`'s leaf-match grammar — change 0153 owns it.
- Any change to which interpreter docket runs under.
