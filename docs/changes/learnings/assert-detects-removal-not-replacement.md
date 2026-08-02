---
slug: assert-detects-removal-not-replacement
hook: "A guard written to CONFIRM the wording you just introduced detects nothing — write the assert that DETECTS the state you just removed, and prove the mutation actually landed before believing it passed."
topics: [testing, guards, mutation]
changes: [135, 167, 193]
created: 2026-07-28
updated: 2026-08-02
promotion_state: candidate
promoted_to:
---

## Apply
When a change replaces prose, a shape, or a code arm, the reflexive guard greps for the **new**
text. That assert is green the moment the edit lands and stays green forever after — including in
every future state where the defect has been reintroduced alongside the replacement. It pins that
the replacement is *present*, never that the defect is *gone*.

Write the guard the other way round:

1. **Assert the negative first** — that the removed state is absent — then mutate the original
   defect back in and watch it go red. A guard that never saw red against the real defect is
   untested code (see [[guards-are-code]]).
2. **Scope the negative assert to where the state lives.** A whole-file OR-grep for a concept
   matches body prose, headings, and examples, so downgrading the very heading it guards leaves it
   green. Anchor on the line, the block, or the producing code.
3. **Beware strings the environment supplies.** An assert that a diagnostic "names X" can be
   satisfied by a worktree path, a branch name, or a fixture directory that happens to contain `X`
   — it passes with the feature entirely absent.
4. **A reverse-direction guard must not hardcode the forward population.** Extracting the names to
   check with a pattern listing the very names the forward loop already asserts makes the reverse
   loop structurally incapable of finding the new one — the case it exists for.
5. **An absence assert needs a non-vacuity companion through the SAME extractor.** Inverting a
   presence assert into `[ -z "$extracted" ]` makes every extraction failure — wrong path, renamed
   file, broken parser — read as the property holding. Keep one live assert that extracts a value
   that must still be there, so a dead extractor reddens something.

**Corollary — a mutation that "passes" is evidence only if the mutation landed.** An in-place
substitution that silently fails to match yields a green run with nothing mutated, which reads
exactly like a robust guard. Confirm the edit with a count before and after (`grep -c`), every time.

Related: [[plan-supplied-test-code-is-unverified]] (prove the assert *can* pass at all),
[[correspondence-guard-runs-one-way]] (the direction a guard iterates is the only one it proves),
[[specified-but-unreachable]] and [[marker-scoped-guard-needs-a-population-floor]] (what a prose
sentinel does and does not pin).

## War story
- 2026-07-28 (#135, PR #127) — **Five false greens in one change**, none caught by the suite; every
  one surfaced by a reviewer mutating the code by hand. (1) A `perl -0pi` substitution silently did
  not match, so a mutation test "passed" without the mutation ever landing. (2) `preflight:
  diagnostic names cursor-agent` was satisfied by the **worktree path** — it contained the literal
  `cursor-agent` — and passed during the red run with no adapter on disk at all. (3) Two asserts
  claiming to prove a reverse-direction derivation both grepped the same file for the same table
  row. (4) A reverse-direction emitter guard hardcoded `(claude|cursor|codex)` in its extraction
  pattern, so a planted `windsurf) emit_windsurf_md` arm — its exact target failure — left the suite
  green. (5) A guard over a "certifying tier" heading stayed green when that heading was downgraded
  to "optional, best-effort", because its whole-file OR-grep still matched the phrase in body prose.
  The unifying shape: **each assert confirmed the wording just introduced instead of detecting the
  state just removed.** Both surviving implementers ended up asserting `grep -c` before and after
  every mutation as standing practice.
- 2026-07-30 (#167, PR #139) — Guard **anchoring** was the dominant defect class across four fix
  rounds, and it failed in both directions. Too loose: a presence-anywhere grep survives deletion of
  the very rule it guards, because the phrase also occurs in the frontmatter `description:`, a
  summary line, or a template block — deleting all three `## Outcomes` bullets once left all three
  outcome asserts green. An unanchored `no` alternative matched inside the word `Nothing`, so a full
  inversion of the change's no-review rule passed. And the `agents.default` guard was **vacuous**:
  its extraction returned the empty string against the real file, so the negative assert could not
  fail under any mutation. Every one was found by mutation, none by reading. The generalization the
  round produced: anchor on a **stable syntactic feature** (a line, a marker block, the producing
  code), never on where prose happens to sit — the too-loose presence grep and the reflow-brittle
  line-scoped assert are one root cause seen from opposite sides (tracked as #0171).
- 2026-08-02 (#193, PR #152) — Removing a config block turned two presence asserts into absence
  asserts, and the two files diverged on the one detail that matters. `test_docket_build.sh` kept a
  live companion reading a surviving key through the same `.docket.yml` extractor, so a broken
  extractor reddens it. `test_docket_review.sh` kept only `[ -z "$dy_skills" ]` — green against a
  renamed file, a wrong path, or an `awk` that stopped matching, i.e. green for reasons that have
  nothing to do with the block being gone. Same edit, same session, same shape; only one of them
  can fail. **The inversion is the moment to add the companion, not a later hardening pass.**
