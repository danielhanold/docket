---
id: 204
slug: restore-rationale-dropped-by-round-three-compression-finaliz
title: Restore dropped doc rationale (compression losses) and complete AGENTS.md's frontmatter-edit rule
status: killed
priority: medium
type: docs
created: 2026-08-03
updated: 2026-08-07
depends_on: []
related: [214]
discovered_from: [201, 206]
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

Change 0201's deep-rung review left two minor findings unfixed at merge time, both the same
defect: the compression round preserved a **rule** in its SKILL.md stub while dropping the
rule's **rationale** instead of relocating it into the new reference file. The rules still
work; the "why" is now stated nowhere, so the next reader (human or agent) who wants to
weaken or route around either rule has no argument to weigh against.

1. `docket-finalize-change`'s `## Finalize blocked` stub kept "an explicitly named id
   overrides the auto-detect skip" but dropped the anti-deadlock reason it exists — without
   the override a marked change can never be finalized, so the clearing rule could never
   fire.
2. `docket-convention`'s auto-capture summary kept "`docket-auto-groom` is never a mint site"
   and the provable-termination invariant, but dropped its concrete consequence: minted stubs
   are themselves autonomous-eligible, making `auto_groom` × `auto_capture` a backlog-growth
   loop.

Both were left for merge-time judgment per the no-auto-fix triage rule and are now merged as-is.

**Absorbed from killed change 0214 — the same defect class on a different surface:** AGENTS.md
§"Frontmatter and generated blocks" carries only half of the promoted `frontmatter-edit-anchor`
rule. It says to anchor a frontmatter-field edit to the first `---…---` block, but nothing about
the whitespace class. During 0206's build a run wrote `plan:` with
`perl -pi -e 's/^plan:\s*$/plan: <path>/'` — **correctly anchored** — and silently welded two
fields into one line, because in Perl `\s` includes `\n` and on an **empty-valued** field (the
shape every docket field is born in) `\s*` eats the line terminator: `results: <path>trivial:
false`. It hit twice in one run and the exit code is 0 either way. An agent following AGENTS.md
to the letter still writes the corrupting form. No committed script is affected (a repo-wide grep
for the `s/^<field>:\s*$/` shape finds nothing); the hazard lives in agent-authored ad-hoc shell,
exactly the population AGENTS.md governs and no test can reach.

## What changes

- Append the anti-deadlock rationale to the marker section of
  `skills/docket-finalize-change/references/gate-failure.md`.
- Restore the backlog-growth-loop consequence to
  `skills/docket-convention/references/auto-capture.md`.
- Sweep the other three files 0201 touched for the same defect class (a rule whose rationale
  was dropped rather than relocated) — per the learnings finding
  `fix-reintroduces-its-own-defect-class`, the change's own additions are the likeliest place
  for its defect class to reappear.
- (From 0214) Add one bullet to AGENTS.md §"Frontmatter and generated blocks": match the trailing
  run with `[[:blank:]]*`, never `\s*`, and read the field back after writing it. Decide whether
  it belongs as its own bullet or as a clause on the existing anchoring bullet — the two are the
  same operation with independent failure modes, and the 0206 instance satisfied the first while
  violating the second.

## Out of scope

- Any further size reduction of the Big 4; budgets stay at 0201's ratcheted values (the
  restored sentences are small, but re-measure rather than assuming headroom).
- Re-litigating what 0201 chose to extract.
- (From 0214) Any change to `scripts/` — the grep found no affected site, and the deterministic
  field writers are not the population at risk; re-promoting or re-tiering the
  `frontmatter-edit-anchor` finding itself, already `promoted` and updated at 0206's close-out.

## Open questions

- Whether the sweep finds enough further instances to justify a grep-able guard, or whether
  two fixes plus a manual read is the whole of it.

## Consolidation note

2026-08-05: absorbed change 0214 (killed pointing here) — both restore a missing half of a rule
on an always-loaded doc surface.

## Why killed

Consolidated into #0257 at the 2026-08-07 backlog triage: item 2 (auto-capture mint-site consequence) verified already restored by 0226 and dropped; the two surviving items carry over.
