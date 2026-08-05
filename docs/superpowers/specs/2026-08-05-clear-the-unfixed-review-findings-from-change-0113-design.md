<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0202 — Clear the unfixed review findings from change 0113](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0202-clear-the-unfixed-review-findings-from-change-0113.md)**
<!-- docket:backlink:end -->

# Clear the unfixed review findings from change 0113

Design doc for change 0202.

## Problem

Change 0113 shipped the `aborted-run` health check and merged with five non-blocking review
findings unfixed. Two are load-bearing for abort detection:

- **`branch_only_artifact` C-quoting (finding 4).** `scripts/board-checks.sh:105` reads
  `git ls-tree -r --name-only --full-tree`, whose default output C-quotes any path containing a
  quote, a backslash, a control character, or — under the default `core.quotePath=true` — any
  non-ASCII byte. `git_has` then runs `cat-file -e "$INTEGRATION_BRANCH:\"docs/…caf\303\251.md\""`,
  which fails, and the function reports an inherited artifact as branch-only. That is a **false
  positive** in a check whose whole value is credibility.
- **Leg A's `results` arm is unpinned (finding 2).** Mutation D
  (`tests/test_board_checks.sh:1369`) unanchors only `fm_field "$f" plan`, and the only body-prose
  fixtures (205, 223) carry a body `plan:` line. Swapping `fm_field "$f" results` (line 393) to
  `field` therefore leaves the suite green while reintroducing the exact silent false negative
  ADR-0057 and 0113's own anchored-read rule exist to prevent.

Three are smaller but real:

- **Finding 1** — `tests/test_docket_status.sh` asserts every `board-checks.sh` argument
  `health_checks` passes except `--results-dir` (`scripts/docket-status.sh:734`). Deleting that
  argument reddens nothing, and the `${RESULTS_DIR:-docs/results}` fallback on the callee side
  (`board-checks.sh:54`) makes the regression silent: a repo with a non-default `results_dir` would
  scan a nonexistent directory forever, green.
- **Finding 3** — two mutation comments state claims the fixtures cannot reach. Mutation A's
  "the healthy-field fixture 221 (plan: SET) starts misfiring. Both directions" is false: 221's
  branch is `feat/arm-results`, which carries no plan file, so the misfire conjunct is unreachable.
  Mutation E's "stale-in-progress must stay unaffected" is asserted nowhere, and no ARM fixture
  produces a `stale-in-progress` finding at all (222's claim is 13h old against a 72h lease).
- **Finding 5** — the 0113 budget-rationale comment in `tests/test_skill_size_budgets.sh` was said
  to omit the measured actual and margin. **It no longer does**: lines 236-240 record
  `3728 words -> … 3800 (72 words of margin)` and `139 actual, 145 budget`, rewritten when 0201's
  slim and 0113's riders were re-measured post-rebase.

## Approach

Five independent fixes, one commit each is fine; nothing here is sequenced.

### 1. `branch_only_artifact` — NUL-delimited listing

```sh
branch_only_artifact(){
  local boa_ref="$1" boa_dir="$2" boa_p
  while IFS= read -r -d '' boa_p; do
    [ -n "$boa_p" ] || continue
    git_has "$INTEGRATION_BRANCH" "$boa_p" || { printf '%s' "$boa_p"; return 0; }
  done < <("$GIT" -C "$CHANGES_DIR" ls-tree -r -z --name-only --full-tree "$boa_ref" -- "$boa_dir" 2>/dev/null)
  return 1
}
```

`-z` suppresses C-quoting entirely and delimits with NUL. The capture-then-here-string shape the
current comment asks to preserve **cannot** be kept: `$(…)` strips NUL bytes, so the captured
string would lose every delimiter and read as one concatenated path. Process substitution is used
instead — the consumer is not a pipeline, so the early `return` that the existing comment guards
against (`set -uo pipefail` + a piped producer) is not reachable here; the producer simply gets
EPIPE and is reaped. The `[ -n "$boa_list" ] || return 1` early-out disappears with the capture and
is not needed: an empty listing yields zero loop iterations and falls through to `return 1`.

Replace the existing three-sentence comment (capture-then-here-string rationale) with the NUL
rationale, naming both the C-quoting failure mode and the command-substitution NUL constraint —
the next editor's likely "simplify" is to restore the capture.

Two further notes for the comment. The hazard the old comment called a "race" is really the
subshell of `producer | while … done`: the in-loop `return 0` exits only the subshell, the function
falls through to `return 1`, and the caller's `if` fails even though the path was printed. State it
correctly rather than copy it forward. And on the early `return`, the process-substituted producer
is orphaned and reaped with its remaining output discarded — fine for a pure reader like `ls-tree`,
and the reason never to swap in a producer with side effects.

**Test.** Two fixtures in `tests/test_board_checks.sh`, and only one of them is the
mutation-detecting one:

- **Sanity fixture (baseline only).** A branch carrying a plan file with a non-ASCII name
  (e.g. `docs/superpowers/plans/2026-06-01-café-plan.md`), `plan:` unset, `status: in-progress`,
  fresh claim: leg A fires. This proves the NUL plumbing reads a real path at all. It does **not**
  discriminate the mutation — a branch-only path fails `git_has` whether or not it was C-quoted.
- **Inherited fixture (the discriminating one).** A branch whose *only* plan file is a non-ASCII
  path that also exists on the integration branch, `plan:` unset, fresh claim. Fixed script:
  SILENT. Mutated: FIRES — the false positive. The "only" is load-bearing: `branch_only_artifact`
  returns the first non-inherited path it finds, so any stray branch-only plan in that fixture repo
  masks the assert.

The mutation must revert **both halves together** — `-z` back to plain `--name-only` *and* the
process-substituted `read -r -d ''` back to the capture/here-string form — asserted landed by a
`grep -c` before/after per the file's house rule. Reverting `-z` alone is not a usable mutation:
`read -d ''` would hit EOF on newline-delimited input, the loop body would never run, and the
function would return 1 for every input, making both fixtures green for the wrong reason.

### 2. Pin the `results` anchored read

Add ARM fixture **224**: `results:` absent from frontmatter, a body `results:` prose line, branch
`feat/arm-results` (which carries an unrecorded results file), `plan:` set, fresh claim. Baseline
fires leg A on 224.

Add **mutation D2**, a separate mutation from D (one mutation breaks exactly one predicate, which
is this file's existing shape): `awk` swaps `fm_field "$f" results` to `field`, asserted landed by
the `grep -cF 'fm_field "$f" results'` 1 → 0 transition, then:

- 224 goes GREEN (proves the anchoring);
- 221, which has no body `results:` line, still fires (the arm itself survives).

### 3. Pin the `--results-dir` caller wiring

In `tests/test_docket_status.sh`'s `health_checks` block, add a third mock invocation with
`RESULTS_DIR=docs/custom-results` exported alongside the existing env, logging to its own file, and
assert `--results-dir docs/custom-results` appears. Pinning the **resolved** value, not the
fallback: an assert on `--results-dir docs/results` would pass identically if the caller hardcoded
the default or if the callee's own fallback were the only thing supplying it.

### 4. Make the two unreachable mutation claims assertable

- **Mutation A.** Add ARM fixture **225**: branch `feat/arm-plan` (carries a branch-only plan
  file), `plan:` SET, `results:` set, fresh claim. Baseline: silent. Under mutation A (`-z` → `-n`):
  fires. Assert both, and rewrite A's comment to name 225 rather than 221.
- **Mutation E.** Add ARM fixture **226** (with a new ~100h claim constant beside `AR_STALE_CLAIM`
  / `AR_FRESH_CLAIM` at `test_board_checks.sh:1098`): no `branch:`, `claimed_at` ~100h old (past the 72h lease
  default, so `stale-in-progress` fires) — baseline emits both `stale-in-progress` and `aborted-run`
  leg B on 226; under mutation E only `stale-in-progress` survives. Assert both.

Fixture-based over comment-deletion because 0202 exists to close exactly the
"claimed-in-prose, asserted-nowhere" class; deleting the claim would resolve the finding by
lowering the bar the change is here to raise.

### 5. Finding 5 — verify, then close as already-satisfied

At build time, re-read `tests/test_skill_size_budgets.sh`'s 0113 rationale. If it still records the
measured actual and margin (it does as of 2026-08-05, lines 236-240), make **no edit**; record the
verification in the results file and in the PR body. Do not re-add a second measurement paragraph.

## Verification

`bash tests/test_board_checks.sh` and `bash tests/test_docket_status.sh` green, plus the full suite.
Every new predicate is mutation-tested with a landed-mutation `grep -c` assert, per 0113's own rule.
Run the added greps under `/usr/bin/grep` as well as the PATH `grep`, which is ugrep on this machine
and accepts constructs BSD grep rejects.

## Out of scope

Unchanged from the change file: `aborted-run`'s predicates and its 12h window (0211 owns the new
leg), and the `docket-implement-next` §5 git-state postcondition prose (change 0203).

## Assumptions

Every decision below was defaulted autonomously; this is the audit trail.

1. **NUL-delimited `ls-tree` over `core.quotePath=false`.** Chosen: `-z` + `read -r -d ''`.
   Rejected: `git -c core.quotePath=false ls-tree --name-only`, which only fixes the non-ASCII
   subset and still C-quotes quotes, backslashes, and control bytes — the review finding names all
   four. Rejected: `mapfile -d '' < <(…)`, because `-d` requires bash 4.4 and this repo's
   **shipped-script** floor is bash 4.0 (`ensure-docket-env.sh`; `docket-status.sh:720`). Test-side
   code does use `mapfile -d` (`tests/test_grep_portability.sh:102`). That is an unexplained
   pre-existing inconsistency, not a sanctioned carve-out — `ensure-docket-env.sh` validates only
   major ≥ 4, so nothing guarantees 4.4 there either. Worth capturing separately; it does not
   license `mapfile -d` in a shipped script.
2. **Process substitution replaces the capture-then-here-string shape.** The stub's suggested fix
   says to keep that shape; it is not possible, because command substitution strips NUL bytes. The
   original hazard the shape defended against — really the subshell of `producer | while … done`,
   where the in-loop `return 0` exits only the subshell — does not apply to a process-substituted
   redirect. Flagged rather than silently
   diverged from: the change file's own text is superseded here.
3. **Separate mutation D2 rather than widening D.** One mutation per predicate keeps attribution
   readable and matches every existing mutation in the file. A widened D would leave a single
   green/red signal covering two independent reads.
4. **New fixtures over deleted claims (finding 3).** The finding offers "either make each claim
   assertable or drop it". Making them assertable costs two small fixtures and directly serves the
   change's stated thesis. Dropping is the fallback only if a fixture would be contrived, which
   neither is.
5. **Finding 5 is verify-then-no-op.** The evidence says a later merge already supplied the missing
   figures. Treating it as a live edit would add a duplicate rationale; treating it as silently
   closed would lose the check. The verify-and-record middle is the conservative read. The change
   file's own bullet still asserts the omission and cites the stale pre-rebase figures
   (`4013 -> 4050`, `147 for 143`); **this spec supersedes it** — a reconcile pass must not restore
   the claim.
6. **Test-side `--results-dir` pin uses a non-default value.** Asserting the default string would be
   satisfied by the callee's own fallback, which is precisely the silent-regression path the finding
   describes.
7. **Non-ASCII test fixture name.** Chosen for the quoting test because it is the failure mode
   `core.quotePath=true` makes universal. A quote or backslash in a filename is legal in git but
   awkward to author portably in the fixture heredocs. macOS filename normalization is not a
   confounder: both sides of the comparison are git tree entries, compared as recorded bytes.
8. **Coupling with 0211 is ordering, not a dependency.** 0211 adds a third leg to the same
   `aborted-run` block and extends the same test file. This groom WRITES `related: [113, 211]` into
   0202's frontmatter (forward link only — the reciprocal on 0211 is skipped) and leaves
   `depends_on:` empty — nothing in 0202 needs 0211 merged, and landing 0202 first means
   0211 builds on hardened predicates. If 0211 lands first, the two touch adjacent regions of
   `board-checks.sh` and `test_board_checks.sh` and reconcile by composition, not choice.
9. **Fixture ids 224-226 are free.** 220-223 are the existing ARM fixtures; the build's reconcile
   re-checks this if 0211 has landed in the interim and claimed numbers of its own.
