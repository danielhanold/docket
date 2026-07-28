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
related: []
discovered_from: [133]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

Change 0133 centralized docket's `runtime.bash` mechanics into `scripts/lib/docket-runtime.sh` and
routed `install.sh`, `scripts/ensure-global-config.sh`, and `scripts/docket-config.sh` through it.
Its whole-branch review then found that the GNU Bash 4+ **validator** still has two more
independent hand-rolled copies the change deliberately did not touch, because 0133's spec scoped
the work to those three callers:

- `scripts/docket.sh` — a POSIX-syntax version check in its bootstrap prologue, which runs before
  an interpreter has been chosen.
- `scripts/ensure-docket-env.sh` — its own `DOCKET_BASH_PATH` validation.

So the repo now has three implementations of "is this an absolute executable GNU Bash 4+", not one.
A future correction to the version grammar (a new banner format, a different major-version parse)
still lands in only one of the three. 0133 narrowed its own prose so nothing claims otherwise —
the false claim is fixed; the duplication is not.

The two survivors are not a straight copy-paste job to remove. `scripts/docket.sh`'s check is a
bootstrap prologue constraint of its own kind, and whether it can source a library at that point —
and whether doing so is even desirable, given the prologue exists to run before anything is
resolved — is the actual design question this change has to answer. `ensure-docket-env.sh` is the
easier of the two.

## What changes

- Decide, per site, whether the check can and should route through `docket_runtime_validate_bash`,
  or whether one of them is a deliberate bootstrap exception that should be documented as such
  rather than removed.
- Route the sites where consolidation wins; document the exception where it does not.
- Extend the mutation coverage so removing the Bash-major check reddens a test through every
  surviving caller, not only through the three 0133 already covers.

## Out of scope

- Re-opening 0133's three callers, the library's interface, or its Bash 3.2 compatibility
  requirement.

## Open questions

- Can `scripts/docket.sh`'s prologue source a library at all, given it deliberately runs before the
  configured runtime is resolved — and if it can, does that weaken the prologue's purpose?
