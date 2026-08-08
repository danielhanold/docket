---
id: 263
slug: guard-the-remaining-agents-md-shell-rules-across-scripts-tes
title: 'Guard the remaining AGENTS.md Shell rules across scripts, tests, and agent-executed markdown'
status: proposed
priority: medium
type: chore
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [254]
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

**Trigger** — surfaced during change 0254's whole-branch review. Review finding 3 found a bare `mv`
installing the learnings index inside `skills/docket-finalize-change/SKILL.md` — literal bash an
agent runs verbatim, carrying exactly the defect 0254 was sweeping out of `scripts/`. 0254 closed
that site and, to keep it closed, built a second walk over the **agent-executed markdown** surface
(`scripts/*.md`, `skills/*/SKILL.md`, `skills/*/references/*.md`). That walk exists now, and it is
enforcing exactly two rules.

**Opportunity** — the repo's remaining `AGENTS.md` `## Shell` rules have no repo-wide guard on any
surface. Verified 2026-08-08: `tests/test_grep_portability.sh` covers every tracked path minus
`docs/`, so the ERE-bound rule is enforced everywhere including markdown; but the
producer-piped-into-early-exiting-consumer rule (`grep -q`, `head`, `head -n1` under
`set -o pipefail`), the leading-`--` grep-pattern rule, and the awk indent-class rule are enforced
by nothing — not in `scripts/`, not in `tests/`, not in the markdown an agent executes. Each is a
shape-keyed, greppable property of the same kind 0254's guard already proves is enforceable.

**Independent value** — stands entirely with 0254 reverted. 0254's contribution here is
evidentiary, not structural: it established that agent-executed markdown is a live executable
surface where a shell defect behaves exactly as it would in a script, and it priced the walk. The
rules themselves predate it and are unguarded on their own terms.

**Boundary** — one new shape-keyed guard (or one extension of an existing one) covering the three
currently-unguarded `## Shell` rules across `scripts/`, `tests/`, and the agent-executed markdown
surface, each with a population floor and mutation tests, plus whatever sweep the guard's first
red run demands. It stops at the `## Shell` section: the `Frontmatter and generated blocks` and
`Guards and tests` rules are a separate question with different shapes, and `docs/` stays excluded
as immutable historical record (the exclusion `test_grep_portability.sh` already documents).

**Reason for deferral** — 0254's thesis is two specific BSD tool defaults, settled at groom time
across assumptions A1–A10 with a scope its spec fixes deliberately. Adding three unrelated shell
rules and their sweeps would roughly double a change already at 27 files, and the pipefail rule in
particular is likely to have live sites whose repair is real work rather than a mechanical flag
insertion — that belongs in a diff a human reads on its own terms.
