<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0413 — Finalize rebase gate cannot clear derived embedded-asset manifest collisions](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0413-finalize-rebase-gate-cannot-clear-derived-embedded-asset-man.md)**
<!-- docket:backlink:end -->
# Finalize rebase gate cannot clear derived embedded-asset manifest collisions — Results

## Outcome

The finalize rebase controller now clears conflict stops whose only unmerged paths are Docket's own
generated embedded-asset bundle (`internal/assets/embedded/manifest.json` and everything under
`internal/assets/embedded/tree/`) by regenerating the bundle in-process, consuming **zero** resolver
reservations. Mixed authored+generated stops still charge exactly one reserved resolver dispatch for
the authored decisions; after the resolver resolves the authored inputs, the controller regenerates
the bundle and stages it in the same continue.

Delivered in `internal/app/finalize_generated.go` (new): the eligibility predicates
`pathsGeneratedOnly` and `bundleRepoEligible` (gated on the exact Docket module identity
`github.com/danielhanold/docket` plus the committed generator contract — no generic generated-file
detection), the in-process `regenerateEmbeddedBundle` (reusing `internal/assets` Generate/WriteTree
with the same staging+rename atomic-write mechanics as `cmd/genassets`, so a failed regeneration
never leaves a partial bundle), the `RegenerateBundle` test seam, and the bounded
`advanceGeneratedOnly` loop. The loop is wired at the three conflicted-stop sites in
`internal/app/finalize_rebase.go` (`mapBegunRebase`, `recoverFromReceipt`, and `mapContinuedRebase` —
placed **before** the resolver-budget exhaustion branch so generated-only stops clear even after the
last permitted authored resolution), and `finalizeRebaseContinueBudgeted` gained the mixed-conflict
branch. `StageAndContinueRebase`'s doc contract in `internal/gitcli/rebase.go` was widened to make
directory-pathspec staging of deletions load-bearing.

No new command, configuration key, generator registry, hooks framework, separate state store,
history squashing, or blanket resolver-budget increase — the generator is reused unchanged. Ownership
checks, the workspace operation lock, interruption/re-entry handling, and the post-rebase whole-suite
gate are all preserved; regeneration is conflict resolution only and never bypasses validation or
merge permission. No material departures from the spec. One in-flight adaptation: an integration
assertion the plan drafted against `FinalizeRebaseResult.ResolverUsed` was replaced with the
authoritative receipt check (`reloadReceipt(...).ResolverUsed`), because a completed continue flows
through `composeLocalGate`, whose result carries no resolver counts by protocol — the intent
("charged exactly one dispatch") is preserved.

## Verification performed

- Full configured suite (`go run ./cmd/docket development test`) green at the final build head
  (recorded build evidence, verified). Budget report showed only screening `BUDGET WATCH:` lines
  (parallel-overrun streak 1/5 on the usual slow shards); **no** `SERIAL CONFIRMED OVER BUDGET:`
  line — no authoritative breach, no action required.
- Real-Git integration coverage (behind the `integration` build tag): the core regression drives
  more generated-only stops than the configured resolver limit and completes with zero reservations,
  both sides' authored edits preserved, and a clean bundle drift check; plus mixed-conflict charging
  exactly one authored dispatch, generation-failure-without-continuation (reservation preserved, Git
  untouched, same reservation retriable), interrupted re-entry without duplicate continuation, and an
  ineligible (foreign-module) repo remaining on the normal resolver path.
- Two new integration shard runners (`tests/test_go_integration_app_generatedbundle.sh`,
  `tests/test_go_integration_app_mixedbundle.sh`) register the new `TestIntegrationGeneratedOnly*`
  and `TestIntegrationMixedConflict*` prefixes; the allowlist-free completeness contract
  (`tests/test_go_integration_contract.sh`) passes, so the new tests actually run in the suite.
- Revert guard: neutering `bundleRepoEligible` makes the core regression and mixed-conflict
  regression fail; restoring makes them pass again.
- Whole-branch deep review (rung: deep) returned 0 blockers, 1 important, 2 minor — all coverage/doc
  findings on this branch's own diff, all fixed in-branch (see Findings and limitations).
- The embedded-bundle drift check (`go run ./cmd/genassets -repo . -check`) matches the authored
  roots after the resolver/finalize doc edits were frozen into the bundle; harness golden wrappers
  for `docket-rebase-resolver` were regenerated across all four harnesses.

## Findings and limitations

### Review dispositions (all findings were on this branch's own diff)

- **[important] `advanceGeneratedOnly` non-advancing-stop guard was untested** — fixed
  (commit 0e4ccd7b): added a unit test that reddens when the `stopped == prev` /
  `errBundleNotAdvancing` guard is neutralized.
- **[minor] step-aside guard (outstanding reservation / started continuation) was untested** — fixed
  (commit ac38a3ec): added a unit test proving the fast path falls through without mutating Git when
  the resolver flow owns the stop.
- **[minor] empty-after-regeneration bundle commit** — a replayed bundle-only commit can become
  empty once regenerated onto the advanced base; `git rebase --continue` then reports "no changes"
  and the fast path blocks to the abort path (`ReasonRebaseGitFailed`) by design. Documented in-code
  (commit 7c4c6800). Low reachability; auto-clearing would require distinguishing an empty-commit
  continue from a real failure, which is out of scope for this fast path.

### Coverage scope

The mechanism is exercised end-to-end at the controller level over real Git repositories (real
`BeginRebase`/`StageAndContinueRebase`/receipt state), but not through a full `docket
finalize-change` run against a live GitHub PR. That outer path is unchanged by this fix (it reuses
the same rebase controller), so the controller-level integration tests are the load-bearing
evidence; a live end-to-end finalize was not performed.

## Human testing

### End-to-end finalize over a real embedded-bundle collision

Prerequisites: a Docket clone, a feature branch with several commits that each regenerate
`internal/assets/embedded` (e.g. successive `skills/`/`agents/` edits), and an independently
regenerated bundle on `main`, with an open PR.

1. Run `docket-finalize-change` for that PR so its rebase-onto-`main` gate replays the feature
   commits over the diverged bundle.
   Expected: the rebase completes without dispatching the resolver for the manifest collisions and
   without exhausting the resolver budget; `finalize.resolver_max_attempts` reservations stay unspent
   for the generated-only stops; both sides' authored edits survive; the post-rebase suite gate runs
   as usual.
