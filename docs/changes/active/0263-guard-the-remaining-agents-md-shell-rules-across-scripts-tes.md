---
id: 263
slug: guard-the-remaining-agents-md-shell-rules-across-scripts-tes
title: 'Guard the remaining AGENTS.md Shell rules across scripts, tests, and agent-executed markdown'
status: proposed
priority: medium
type: chore
created: 2026-08-08
updated: 2026-08-09
depends_on: []
related: [262, 253]
discovered_from: [254]
adrs: []
spec: docs/superpowers/specs/2026-08-09-guard-the-remaining-agents-md-shell-rules-across-scripts-tes-design.md
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
| Spec | [2026-08-09-guard-the-remaining-agents-md-shell-rules-across-scripts-tes-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-guard-the-remaining-agents-md-shell-rules-across-scripts-tes-design.md) |
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

Two scope rulings recorded at the 2026-08-09 triage:

- **Producer-pipe rule split with #0172.** Groomed change 0172 owns that rule's `scripts/`/`tests/`
  `*.sh` population (400+ sites, its own canonical forms, its own guard
  `tests/test_pipefail_shape.sh`). This change's remit for the producer-pipe rule is the
  **agent-executed markdown surface only**; for `*.sh` it adds nothing on top of 0172. Grooming
  should decide whether the markdown leg extends 0172's guard or 0254's markdown walk.
- **Fourth leg (absorbed from #0262, killed pointing here): the single-backslash word-boundary
  spelling.** 0246's banned class in `tests/test_grep_portability.sh` matches only the
  two-backslash source form; in bash `"\b"` delivers the identical byte pair to grep, so the
  surviving spelling reintroduces the BSD-portability defect with the guard green — same
  guard-the-class-not-the-spelling shape as the three rules above, same file, same
  policy fork (convert the ~48 sites, or bless with an asserted-exact list per 0246's
  `elsewhere_shape_exempt` precedent). Known live carriers in the surviving spelling:
  `tests/test_docket_metadata_branch.sh:112`, `tests/test_cursor_dispatch_rule.sh:38,:93`.
  The toolchain pin/report question stays #0150's.

**Reason for deferral** — 0254's thesis is two specific BSD tool defaults, settled at groom time
across assumptions A1–A10 with a scope its spec fixes deliberately. Adding three unrelated shell
rules and their sweeps would roughly double a change already at 27 files, and the pipefail rule in
particular is likely to have live sites whose repair is real work rather than a mechanical flag
insertion — that belongs in a diff a human reads on its own terms.

## What changes

Groomed 2026-08-09 (auto-groom; design in the linked spec, critic-gated, all assumptions sound).
Three guard homes, one per rule class:

- **Producer-pipe, agent-executed markdown only** — extends 0172's `tests/test_pipefail_shape.sh`
  with the three-glob markdown surface as a separately-floored population, reusing 0172's taxonomy
  and `pipefail-ok:` token verbatim. Creates the real dependency `depends_on: [172]` (the guard
  file must exist first); the build's reconcile re-reads 0172's *built* guard, not its spec.
- **Leading-`--` grep pattern + awk literal-space class** — one new guard,
  `tests/test_shell_shape_rules.sh`, over the tracked-minus-docs walk. The leading-`--` leg is
  guard-only (tree verified clean; compliant-population floor ~117 lines). The awk leg is an owned
  widening (any `[^ ]` in awk program text, bless token `# awk-space-ok:`), detected with a
  bounded in-program quote tracker; one live site converts.
- **Single-backslash word boundary (leg absorbed from killed 0262)** — policy settled:
  **convert-by-default, per-site `# word-boundary-ok:` bless token** for deliberate PATH-grep
  idiom; widen the class in `tests/test_grep_portability.sh`, extend its assembled-spelling
  discipline to its own header comments, add a whole-line-comment drop, and convert the derived
  site population to explicit `[^[:alnum:]_]` classes with per-site inspection where conversion is
  not verdict-trivial.

All guards `/usr/bin/grep`-pinned, floored, mutation-tested, budgets-registered.

## Out of scope

- The producer-pipe rule on `*.sh` (0172's remit — the recorded split).
- The `Frontmatter and generated blocks` and `Guards and tests` AGENTS.md sections.
- `docs/` in every walk (immutable point-in-time records).
- The toolchain pin/report (#0150); the prose-anchor house pattern (#0253 — file collision on
  `test_docket_build.sh`/`test_docket_review.sh`, orderable either way, hence `related:`).

## Open questions
- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Narrow to two legs: the leading-`--` grep pattern and the awk `[^ ]` literal-space class, as additions to `internal/repoguard/shellshape_test.go`. The pipefail-markdown and word-boundary legs already landed there (TestPipeShapes, TestGrepPortability). `depends_on: [172]` cleared — 0172 was killed as already fixed in Go.

