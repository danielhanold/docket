# Guard the README config snippet against `.docket.yml.example` drift — results

Change: #107 · Branch: feat/guard-the-readme-config-snippet-against-docket-yml-example-d · PR: <url> · Plan: docs/superpowers/plans/2026-07-20-readme-snippet-drift-guard-plan.md · ADRs: none

## Verify (human)

Nothing interactive — the deliverable is a test section and the whole suite is green (52/52
`tests/test_*.sh`). One judgment call is worth your eye at the merge gate:

- [ ] The exact-count assert (`snippet flattened key count is exactly 5`) will redden the next time
      someone legitimately adds a key to the README snippet. That is intended — the spec chose an
      exact count over `>= 1` precisely so the snippet cannot creep back toward being an all-keys
      mirror. Confirm you want that friction; the remedy is inline in the assert's own failure
      message (bump the literal, add the key to `.docket.yml.example`, same commit).

## Findings

**No ADR written, deliberately.** The spec's `## No ADR` section already settled this: the
forward-only direction is a test-design decision scoped to one guard, recorded in the spec and in
the test's own comment, and ADR-0048 owns the standing rule it sits under. Nothing during the build
raised a decision above that bar — the one implementation-time judgment call (below) is a local awk
detail, documented inline.

**The whole-branch review found 2 Important + 8 Minor issues in the first-cut guard; all Important
and 6 Minor were fixed on the branch.** Recorded because the class is instructive — every one was a
way for a *green* guard to be proving less than it claimed:

- **Second fence silently unguarded (Important).** `readme_snippet()` took only the first ```` ```yaml ````
  fence in the section and nothing asserted there was only one. A reviewer verified that adding a
  second fence containing `metadata_branch: BOGUS` and `nonexistent_key: 1` left **all six asserts
  green**. Closed by consolidating both extractors onto one `snippet_section()` (so the heading
  literal appears once) plus an explicit fence-count assert. I re-proved this one myself after the
  fix: the mutation now reddens `(8) section has exactly one yaml fence ... got 2`.
- **Plan overclaimed a completion bar (Important).** The plan's self-review table said the spec's
  "add a nested key consistently to both files → stays green" bar was covered by Task 1 Step 4.
  It was not: that step only probes the flattener in isolation, and in reality a consistent snippet
  addition does *not* stay green — the exact-count floor fires. The spec's own two requirements
  genuinely conflict here; the exact count is binding. Plan text corrected rather than quietly left.
- **Section boundary satisfiable by a neighbor (Minor).** Bounding on `^### ` only meant a compound
  edit (this section's link removed *and* the next heading demoted to `##`) let a *later* section's
  link satisfy both pointer asserts — the "sentinel satisfied by a neighbor's content" hazard the
  comment claimed to defend against. Boundary widened to `^#{1,3} `.
- **Flattener could drop key-shaped lines invisibly (Minor).** Its key regex rejects anything
  outside `[A-Za-z_][A-Za-z0-9_]*`, and `sn_count` counts *post-filter* output — so `some-new-key:
  yes` in the snippet was dropped by the flattener and invisible to both the count floor and the
  loop. Added a raw-vs-flattened line-count cross-check as the structural safety net.
- Also fixed: indent measured in literal spaces while line shape used `[[:space:]]` (tab-indented
  key mis-nested); failure messages that misdescribed the grow case and carried no remedy into CI
  output; the pointer regex pinning link *text* rather than target; and a missing caveat that
  value-equality is sound only because this one fence shows shipped defaults.

**Left unfixed (2 Minor, judged defended-in-depth):** duplicate flattened paths resolve
first-match-wins (section `(1)`'s fidelity diff already covers a drifted duplicate in the example),
and the narrow key regex remains narrow by design — now with the cross-check above as its net.

**Notable plan deviation — the boundary fix needed a fix of its own.** Widening the section boundary
to `^#{1,3} ` collides with the YAML sample's *own* leading comment line (`# .docket.yml —
committed...`), which is simultaneously valid YAML-comment and valid markdown-H1 syntax; applied
naively it truncated the section and drove the key count to 0. Resolved by gating the heading-exit
check off while inside a fenced block. This is the repo's `specified-but-unreachable` lesson
recurring in a new place — a range expression closing on a *comment* rather than the content it was
aimed at.

**Plan/repo mismatch worth knowing:** the plan referenced `tests/run_all.sh` for the full-suite
step. No such script (and no CI config) exists in this repo — the suite is the `tests/test_*.sh`
loop, per AGENTS.md's "run the whole suite at the build gate". Plan corrected.

## Follow-ups

- **Change #108 — "Guard the README's remaining config fences against key drift"** (auto-captured
  from this build, `discovered_from: [107]`). This guard deliberately covers one fence. The
  README's other config fences (`auto_capture: true` ~264, `terminal_publish: true` ~407,
  `metadata_branch: main` ~433, the global-config and `.docket.local.yml` samples, the skills/runner
  fences) are unguarded, and they *deliberately show non-default values* — so they need an
  existence-only check, not this section's value equality. That design call is why it is its own
  change rather than an extension here.
