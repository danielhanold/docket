---
id: 214
slug: agents-md-s-promoted-frontmatter-rule-omits-the-whitespace-c
title: AGENTS.md's promoted frontmatter rule omits the whitespace-class half that corrupted two field writes
status: proposed
priority: medium
type: docs
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: []
discovered_from: [206]
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

The learnings finding `frontmatter-edit-anchor` is `promotion_state: promoted`, `promoted_to:
AGENTS.md` — promotion is what makes a rule fire *unprompted*, because AGENTS.md is always loaded
while a learnings finding is read only when its index line looks relevant.

Change 0206's close-out extended that finding with a second, independent failure mode, and the
promoted surface still carries only the first half. AGENTS.md §"Frontmatter and generated blocks"
says:

> Anchor a frontmatter-field edit to the first `---…---` block, never a bare column-0 line match.

It says nothing about the whitespace class. During 0206's build a `docket-implement-next` run wrote
`plan:` with `perl -pi -e 's/^plan:\s*$/plan: <path>/'` — **correctly anchored** — and silently
produced a welded two-field line, because in Perl `\s` includes `\n` and `$` matches *before* the
final newline rather than consuming it. On an **empty-valued** field, which is the shape every
docket field is born in, `\s*` eats the line terminator: `results: <path>trivial: false`. It hit
twice in one run (`plan:`, then `pr:` and `results:` together) and was caught only because the run
read the field back; the exit code is 0 either way.

So an agent following AGENTS.md to the letter still writes the corrupting form. The anchoring rule
was satisfied and the write was still wrong.

No committed script is affected — a repo-wide grep for the `s/^<field>:\s*$/` shape across
`scripts/` and `skills/` finds nothing. The hazard lives in agent-authored ad-hoc shell, which is
exactly the population AGENTS.md governs and no test can reach.

## What changes

- Add one bullet to AGENTS.md §"Frontmatter and generated blocks": match the trailing run with
  `[[:blank:]]*`, never `\s*`, and read the field back after writing it.
- Decide whether it belongs as its own bullet or as a clause on the existing anchoring bullet — the
  two are the same operation with independent failure modes, and the 0206 instance satisfied the
  first while violating the second.

## Out of scope

- Any change to `scripts/`: the grep above found no affected site, and the deterministic field
  writers are not the population at risk.
- Re-promoting or re-tiering the `frontmatter-edit-anchor` finding itself — it is already
  `promoted`, and the ledger side was updated at 0206's close-out.
