---
id: 172
slug: normalize-the-banned-producer-pipe-shape-across-tests-and-he
title: Normalize the banned producer-pipe shape across tests and helpers
status: proposed
priority: medium
type: chore
created: 2026-07-30
updated: 2026-07-30
depends_on: []
related: []
discovered_from: [167]
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

`AGENTS.md` bans `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`) under
`set -o pipefail`: the producer takes SIGPIPE and the 141 becomes an intermittent failure. Several
existing test helpers still use exactly that shape, and change 0167's reviews found three more while
declining to fix them piecemeal — each time correctly, because normalizing one arm of a family while its
siblings keep the old shape makes the inconsistency worse and invites a later "fix" that reintroduces it.

Known instances: `fmv()` in `tests/test_docket_build.sh` (`awk | sed | head -n1 | sed`), `ex_field()` in
`tests/test_docket_example_yml.sh` (the sibling it was copied from), the non-vacuity floors using
`printf … | grep -c .`, and the `echo "$out" | grep -qxF` asserts in `tests/test_docket_config.sh`'s
`BLD-a`/`BLD-b` — which match the surrounding `RCL-a`/`AC-a` family, so the whole band moves together or
not at all.

Every instance found so far is benign in practice — exit status discarded inside `$( )`, inputs far under
the 64K pipe buffer, `grep -c` does not exit early. The risk is not today's flake; it is that the banned
shape reads as sanctioned house style, so the next helper copies it into a place where the producer is
large or the status is load-bearing.

## What changes

Sweep `tests/` and the `scripts/` helpers for the banned producer-pipe shape, derive the true site list
from a whole-repo grep rather than the instances listed above, and convert each to the capture-then-match
form (`var="$(producer)"; grep <<<"$var"`) or another form that is safe under `pipefail`. Where a site is
deliberately exempt, say so in a comment at the site so the next reader does not re-open it.

## Out of scope

- Changing what any assert checks. This is a mechanical shape normalization; every test's verdict before
  and after must be identical.
- The prose-anchor reflow-fragility cleanup, which is its own separate change.
