---
id: 262
slug: ban-the-single-backslash-word-boundary-form-too-not-just-its
title: 'Ban the single-backslash word-boundary form too, not just its escaped spelling'
status: killed
priority: medium
type: fix
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [246]
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

**Trigger** — surfaced by change 0246's whole-branch review (finding 1), while hardening
`tests/test_grep_portability.sh` with a banned class for the escaped word-boundary source form.
The class 0246 shipped matches the **two-backslash** spelling only. In bash, `"\\b"` and `"\b"`
deliver the identical byte pair `\b` to grep, and the scanner is a pure byte-pattern with no quote
awareness — so a double-quoted **single-backslash** `"\b"` reaches grep exactly the same way and is
unguarded. 0246 corrected its own header to state that limitation honestly and asserted it, rather
than expanding scope mid-fix-loop.

**Opportunity** — extend the portability guard to the surviving spelling, so the ban covers the
defect class rather than one way of writing it. Today a worker who writes `grep -qE "\b$key\b"`
instead of `grep -qE "\\b$key\\b"` reintroduces exactly the BSD-portability defect 0246 removed,
with the guard green.

**Independent value** — holds with 0246 reverted. The hazard is a property of `/usr/bin/grep` on
BSD/macOS versus PATH `grep` (ugrep 7.5.0 here, which accepts constructs BSD grep rejects), not of
0246's diff. Two tracked files already carry the defect in the surviving spelling and pass 0246's
class clean: `tests/test_docket_metadata_branch.sh:112` and `tests/test_cursor_dispatch_rule.sh`
(two sites, :38 and :93).

**Boundary** — the work is: decide the policy for the single-backslash spelling (convert, or bless
with an asserted-exact list in the manner of 0246's `elsewhere_shape_exempt`), then implement it in
`tests/test_grep_portability.sh` and whichever tracked files must change. It deliberately leaves
alone the escaped two-backslash class 0246 already ships, and the toolchain pin/report question,
which is change #0150.

**Reason for deferral** — the surviving-spelling population is large and pre-existing: 0246's
review estimated ~42 sites, and the computed figure the change now prints is 48. Converting or
blessing 48 sites across many test files is a change's worth of work in its own right, and folding
it into 0246 — a change whose scope was three named guard defects — would have expanded that branch
well past what was groomed and specified.

## Why killed

Consolidated into #0263 at the 2026-08-09 backlog triage: both changes extend the same guard
surface with the same shape of work — settle a convert-or-bless policy for a grep-derived
population, sweep it, and extend a shape-keyed guard (`tests/test_grep_portability.sh` territory)
with population floors and mutation tests. One brainstorm settles both. 0263 carries this stub's
scope verbatim as its fourth leg, including the known live carriers and the
`elsewhere_shape_exempt` blessing option.
