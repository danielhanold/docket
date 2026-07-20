# Pin the `finalize.test_command` auto sentinel's cross-layer masking — results

Change: #106 · Branch: `feat/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma` · PR: (see manifest `pr:`) · Plan: `docs/superpowers/plans/2026-07-20-auto-sentinel-cross-layer-masking.md` · ADRs: none

Test-only change. `scripts/docket-config.sh` is **not** modified by this branch — it was mutated
temporarily during the mutation runs below and restored byte-clean each time (verified with
`git diff --quiet` after each restore, and by `git show --stat` on both task commits).

## Verify (human)

- [ ] Nothing interactive is required. The receipt is the suite: `bash tests/test_docket_config.sh`
      must report `PASS` with **221 ok / 0 NOT OK** on the rebased branch. (Baseline before this
      change was 216 ok.)

## Mutation evidence — the durable record

This section exists because the mutation runs are the *only* thing that proves these fixtures are
not decoration, and the raw run logs live in a gitignored scratch dir that is deleted with the
worktree. The whole-branch review flagged that as a Minor finding; this is its discharge.

The suite has 221 asserts after this change. Both mutations were applied to the **real**
`scripts/docket-config.sh`, never to a fixture copy.

| Run | Production code | ok | NOT OK | Which asserts redden |
|---|---|---|---|---|
| Baseline (branch base `3e26790`) | unmodified | 216 | 0 | — |
| After Task 1 (s4, s5) | unmodified | 220 | 0 | — |
| **Mutation 1** — collapse the sentinel **per-layer** instead of after the `:194` chain | mutated | 218 | 2 | `0106 s4`, `0106 s5` masking asserts. Both **control** asserts stayed green. |
| After Task 2 (s6) | unmodified | 221 | 0 | — |
| **Mutation 1 re-run, with s6 present** | mutated | 219 | 2 | `0106 s4`, `0106 s5` only — **`s6` stayed GREEN**. |
| **Mutation 2** — blanket "any layer says `auto` ⇒ unset" | mutated | 220 | 1 | **`0106 s6` alone** — all four `s4`/`s5` asserts stayed green. |
| Final | unmodified | 221 | 0 | — |

**Assert totals are conserved in both mutation runs** (219+2 = 221; 220+1 = 221). No assert vanished
rather than failed — the vacuity-by-disappearance mode the repo's `guards-are-code` rule names as
item (k).

**The 2×2 asymmetry is forced by precedence, not by fixture luck.** Resolution is
local > committed > global, first non-empty wins, with the sentinel collapsed once on the final
winner:

- Mutation 1 changes behavior only when the **winning** layer's own value is literally `auto` —
  true for `s4`/`s5`, structurally impossible for `s6` (whose sentinel sits *below* the winning
  rung, so `:-` short-circuits before it is ever consulted). `s6` therefore *cannot* redden here,
  which is precisely why it needs a mutation of its own.
- Mutation 2 clears on any rung holding `auto` regardless of precedence. `s4`/`s5` already resolve
  to empty, so the clear is a no-op (green); `s6`'s `make test` is wrongly wiped (red).

The re-run of Mutation 1 with `s6` present exists solely to demonstrate that negative result, which
the spec called out as load-bearing rather than incidental.

## Findings

None became ADRs — this change pins existing behavior and made no architectural decision. Three
Minor findings from the whole-branch review, all non-blocking:

1. **Reverse direction proven for only one of the two lower rungs.** `s6` covers
   *(lower = global `auto`, higher = committed real)*; the pair
   *(lower = committed `auto`, higher = local real)* is untested. Filed as its own change — see
   Follow-ups. Not folded in: it extends past the settled spec and would require re-running both
   mutations.
2. **Mutation evidence was not durable.** Discharged by this file.
3. **The new comment block anchors on line numbers** (`:194`, `:201`) — and this change exists
   *because* a comment about this exact property drifted, so hard line numbers reintroduce the rot
   vector one level up. **Considered and declined:** it matches house style throughout the repo, it
   is comment-only, and leaving `tests/test_docket_config.sh` byte-identical to its
   mutation-verified state was judged worth more than the marginal improvement. Worth revisiting
   repo-wide rather than in this one block.

Two observations recorded for context, neither a defect in this change:

- The repo has **no `.github/` directory**, so no CI runs any suite. The merge decision rests on the
  local runs recorded above. Cross-checked that no other test file reads or counts
  `test_docket_config.sh`'s contents, so a test-only append cannot break a sibling suite.
- Latent and unreachable: `rung()` passes `XDG_CONFIG_HOME="$x"`, and `${...:-...}` treats an *empty*
  value as unset — an empty `$x` would fall back to the real `$HOME/.config`. Not reachable from any
  fixture (`$tmp` is always non-empty) and pre-existing in the helper, not introduced here.

## Follow-ups

- **Change #112** — *Pin the reverse cross-layer masking for the committed-over-local rung pair*
  (auto-captured, `discovered_from: [106]`). Adds an `s7` fixture asserting a committed `auto` does
  not wipe a local `.docket.local.yml` real command. The stub records the specific refactor that
  slips past all five of this change's asserts while silently dropping a user's local test command.

## Notable plan deviations

One, corrected in-branch: the plan's verification checklist said the branch should carry "exactly
two commits, both `test(0106):`", which miscounted by omitting the plan-doc commit and used
`origin/main..HEAD` (a range that also reports commits landing on `main` after the branch was cut).
Corrected to the merge-base form in commit `2ce3cef`.

The branch was cut before change 0107's work merged, so it sits behind `origin/main`. Its own
contribution is exactly the plan, this results file, and `tests/test_docket_config.sh`; finalize's
rebase gate handles the staleness.
