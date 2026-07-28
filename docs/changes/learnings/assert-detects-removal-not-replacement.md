---
slug: assert-detects-removal-not-replacement
hook: "A guard written to CONFIRM the wording you just introduced detects nothing — write the assert that DETECTS the state you just removed, and prove the mutation actually landed before believing it passed."
topics: [testing, guards, mutation]
changes: [135]
created: 2026-07-28
updated: 2026-07-28
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
