---
id: 233
slug: guard-against-stacked-gap-ere-patterns-that-hang-instead-of
title: Guard against stacked-gap ERE patterns that hang instead of failing
status: proposed
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [226]
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

**Trigger** — surfaced while fixing review findings on change 0226. A prose guard written as
`grep -qiE "at each mint site[^.]{0,160}read[^.]{0,60}auto-capture\.md"` did not redden against the
mutated file it existed to catch: under ugrep it backtracked catastrophically and hung for minutes.
Rewriting it with a single bounded gap fixed it. A second fix worker on the same branch was warned
and avoided the shape; a third confirmed the same hazard independently.

**Opportunity** — nothing in the repo detects this class. `tests/test_grep_portability.sh` guards
ERE bounds above 255 (the BSD-grep ceiling) but says nothing about *stacked* gaps, which are legal,
portable, and still pathological. A `/usr/bin/grep -rlE` sweep for two `[^x]{0,n}` gaps in one
pattern currently matches three test files — `tests/test_docket_review.sh`,
`tests/test_dispatch_capability.sh`, and `tests/test_docket_build.sh` — the latter two entirely
outside change 0226's diff. Each is a latent multi-minute hang that fires precisely when a guard is
doing its job, which is the worst possible time: the symptom looks like a wedged suite, not a caught
mutation.

**Independent value** — holds with change 0226 reverted. The hazard predates it, lives mostly in
files it never touched, and grows with every new proximity-shaped prose sentinel this project
writes — and it writes many, because keying guards on proximity rather than bare presence is
established house style here. The deliverable is a guard plus a documented single-gap rule, which
makes every future sentinel safe by default rather than by remembering.

**Boundary** — audit the existing `tests/*.sh` prose-grep patterns for stacked gaps, rewrite each to
a single bounded gap (or an equivalent non-backtracking shape), and add a guard that rejects the
stacked form the way `test_grep_portability.sh` rejects an over-255 bound. Stops there. It does not
touch what any assert means, does not restate the 255-bound rule, and does not change the reviewer,
build, or fix-loop contracts those files guard.

**Reason for deferral** — change 0226's scope is the auto-capture reframe. Its own patterns are
already fixed in-branch; what remains is a repo-wide sweep of unrelated test files plus a new
portability-style guard, which would expand an approved documentation change into a test-suite
audit. It also wants its own mutation proof — a pattern that hangs rather than fails needs a
timeout-shaped assert, which is a design question worth its own groom.
