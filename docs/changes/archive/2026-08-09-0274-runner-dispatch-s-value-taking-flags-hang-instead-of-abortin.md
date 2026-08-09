---
id: 274
slug: runner-dispatch-s-value-taking-flags-hang-instead-of-abortin
title: 'runner-dispatch''s value-taking flags hang instead of aborting when given with no value'
status: killed
priority: medium
type: fix
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: []
discovered_from: [271]
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

**Trigger** — surfaced while building change 0271 (detached delegation posture). Adding the
`--observe <key>` flag to `scripts/runner-dispatch.sh` forced a close read of its argument loop,
which revealed that every pre-existing value-taking flag shares a latent hang.

**Opportunity** — `runner-dispatch.sh`'s parser uses `shift 2` for `--runner`, `--agent`,
`--model`, `--effort`, and `--worktree`. Bash's `shift` **fails rather than truncating** when the
argument count is smaller than the shift count, so a value-taking flag supplied in **final
position with no value** never advances the loop and the script spins forever. Confirmed by
measurement, not inference: `timeout 3 bash scripts/runner-dispatch.sh --runner` returns **124**.
The facade is the single chokepoint every delegated agent run passes through (ADR-0038), so a hang
here is a hang with no diagnostic and no exit code — the caller cannot tell it from a slow child.

0271 fixed this for the one flag it introduced (`--observe` uses
`shift; [ $# -gt 0 ] && shift`), which also made that flag's own "requires a dispatch key" refusal
reachable instead of decoration. The other five were left exactly as they were, deliberately: they
are outside that change's boundary.

**Independent value** — holds with 0271 fully reverted. The five flags predate it, the hang is
reachable on the pre-0271 synchronous path, and a hand invocation or a malformed generated wrapper
reaches it today. Fixing it also converts five unreachable validation branches into reachable ones.

**Boundary** — normalize the shift handling of the five value-taking flags in
`scripts/runner-dispatch.sh` uniformly, so a missing value produces the loud abort the facade's
posture already promises rather than a hang; add a guard that drives each flag in final position
and asserts a bounded non-zero exit rather than a timeout. Stops there. It does **not** touch flag
semantics, value resolution, the run gate, or the adapters — and it does not restyle argument loops
in other scripts, though a follow-up audit of the same `shift 2` shape elsewhere would be
reasonable to scope separately.

**Reason for deferral** — 0271's branch is the delegation execution posture; its diff is already
large (20 files, ~2900 lines) and carries its own blocker fixes. Folding an unrelated
argument-parser repair for four flags it never touches would expand that branch's intended scope
and blur what its review covered.

## Why killed

Consolidated into #0208 at the 2026-08-09 backlog triage: 0208's flag-parse leg (c) — absorbed
from #0210, discovered by 0206's whole-branch review — already specifies the identical fix
(`[ $# -ge 2 ] || die "--flag requires a value"` at all five `shift 2` sites, list derived by
grep at build time) and the identical tests (one hang-regression leg per flag, bounded by a
background+poll+kill helper). This stub re-discovered the same defect from 0271's build without
knowing 0210 had already carried it into 0208. Its measured evidence (`timeout 3 … --runner`
→ 124) is recorded in 0208's consolidation note.
