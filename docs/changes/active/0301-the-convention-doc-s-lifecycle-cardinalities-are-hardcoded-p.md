---
id: 301
slug: the-convention-doc-s-lifecycle-cardinalities-are-hardcoded-p
title: 'The convention doc''s lifecycle cardinalities are hardcoded prose with no guard'
status: proposed
priority: medium
type: docs
created: 2026-08-12
updated: 2026-08-12
depends_on: []
related: []
discovered_from: [298]
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

**Trigger** — surfaced closing out #0298, whose review finding 7 was fixed and then reverted by the
merge gate. The fix is gone from merged main; the defect it named is not.

**Opportunity** — `skills/docket-convention/SKILL.md` still asserts the lifecycle has "eight states"
and `github-board-mirror.md` still says "(all eight)", in the same repo where #0298 stripped every
hardcoded lifecycle cardinality out of the scripts. A ninth status makes both documents lie, and
nothing gates them. The prose edit is three words per site; the work is the guard that keeps them
from re-arming.

**Independent value** — the convention doc is the shipped contract every consuming repo reads, and
the claim is wrong-by-construction the next time the lifecycle grows. Worth doing whether or not
stacked changes exist.

**Boundary** — the two cardinality assertions in `skills/docket-convention/SKILL.md` and
`skills/docket-convention/github-board-mirror.md`, plus one guard keying on syntactic shape that
reddens if either re-arms. It deliberately leaves alone the 56 single-backslash `\b` sites tracked
by #0300 — this change needs only that its own guard avoid that spelling, using an explicit
`[^[:alnum:]_]` class.

**Reason for deferral** — #0298's own attempt is what proved the guard spelling is the hard part:
its `\b` guard was vacuous on BSD, `tests/test_grep_portability.sh` reddened the suite gate, and the
gate's revert-and-record path removed the whole fix. Redoing it inside a merge close-out would mean
re-running the gate on an approved branch for work outside that branch's scope.

## Open questions

- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-target: the convention still reads `eight states` and `github-board-mirror.md` says `all eight`; the proposed shell-grep guard has no runner. Make it a Go test (e.g. in `internal/domain`, where the status set lives). `github-board-mirror.md` describes the sunset GitHub mirror, so that half may collapse to deleting the doc.

