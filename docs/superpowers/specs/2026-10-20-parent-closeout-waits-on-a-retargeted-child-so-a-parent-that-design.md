<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0571 — Close out a landed stack parent without waiting on children that never merged into it](../../changes/archive/2026-10-20-0571-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that.md)**
<!-- docket:backlink:end -->

# A landed stack parent closes out without waiting on children that never merged into it — design

Change #571, groomed 2026-10-19. Found while building #567 (PR #433).

## The problem

When a stacked parent's PR merges to the integration branch, `finalize closeout` takes the
integration-destination path (`closeoutIntegrationDestination`). That path asks
`domain.DeriveRootCloseoutSet` which descendants the parent carries. `DeriveRootCloseoutSet` walks
**every** change stacked on the parent, at any depth, active and archived alike
(`StackDescendantsParentFirst` over a corpus that includes `archive/`), and `proveCarry` refuses any
descendant whose status is not `stacked-merged` with the token `not-stacked-merged`.
`closeoutUnprovenDisposition` maps that token to the `children-retarget-required` disposition, and
the parent stays `implemented`.

So a parent that has landed on main never reaches `done` while any change stacked on it is in any
other state:

- a child whose PR finalize retargeted to main before the parent merged (the case #567's results
  reported);
- a child that is only groomed (`proposed`), being built (`in-progress`), `blocked`, or `deferred`;
- a child that was `killed`;
- a child that already landed on main by itself and is `done` (this answers #567's open question:
  the parent's closeout still refuses after the child lands).

The sweep retries the parent every run and gets the same refusal every time.

The rest of finalize already disagrees with closeout here:

- `finalize merge` deliberately lets a retargeted child through: `probeUnretargetedOpenChildren`
  counts only open child PRs that still target the parent's branch ("A retargeted child (its PR now
  targets some other base) does not block").
- The carry proof that rebase, publish, and merge run (`proveCarriedOnHead` over
  `domain.DeriveCarriedSet`) counts only children connected to the parent by merged
  `stacked-merged` links. An open or final child "claims no carry for this parent branch".
- The original stacks design said root closeout carries "every `stacked-merged` descendant", not
  every descendant.

The end-to-end test for the parent-landed-first flow (`TestE2EStackParentLandedFirst` in
`internal/app/finalize_e2e_test.go`) works around this by writing the parent's done record by hand
(`archiveAsDoneOnOrigin`).

## The rule

**A landed parent's closeout takes to `done` exactly the children whose PRs merged into it, down
the chain, once each one's work is proven on the integration branch. No other child can block it.**

"Merged into it" is read from each child's live PR facts, which closeout already probes for every
descendant (`probeDescendantFacts`): the child's PR is merged and its base is the parent's recorded
`branch:`. That is the same link `walkCarryChain` and `DeriveCarriedSet` already check for
`stacked-merged` children.

A child counts as carried when its status is `stacked-merged` **or** `implemented` and its live PR
merged into its parent's recorded branch. An `implemented` child whose PR is open, closed without
merging, or merged anywhere else (for example, retargeted to main and merged there before its own
closeout ran) is not carried and never blocks the parent; its own closeout handles it. An
`implemented` child with no usable PR reference is not carried either (grooming call: nothing docket can
see merged anywhere). The `implemented` case covers a child whose PR merged into
the parent's branch but whose own closeout never ran: it was merged by hand on GitHub, or its
closeout was interrupted. Today the parent refuses until that child's closeout runs. That closeout
needs the parent's branch to still exist (`requireStackedPreservation` fetches it), so in a
repository with GitHub's "automatically delete head branches" setting on, the child reports
`unknown` forever and the parent never reaches `done`. Under this rule the parent's closeout takes
that child straight from `implemented` to `done`, gated by the same Git content proof every carried
child already gets (`proveCarriedContent`: the child's merge result is reachable in the pinned
integration history, or exact-content at the parent's merge result). That proof reads the
integration branch, not the parent's branch, so a deleted parent branch doesn't matter.

The walk descends through carried children only. Grandchildren beneath a child that is not carried
are not carried by this parent; they wait for their own parent.

### Before and after

| Situation when parent A lands on main | Today | After |
|---|---|---|
| Child B's PR was retargeted to main and is still open | A refuses (`children-retarget-required`) and stays `implemented` forever | A is archived as `done`; B is untouched. B's effective base becomes main (ADR-0092) and B is finalized later on its own |
| Child B is `proposed`, `in-progress`, `blocked`, `deferred`, or `killed` | A refuses forever | A is archived as `done`; B is untouched |
| Child B already landed on main and is `done` | A refuses forever | A is archived as `done` |
| Child B is `stacked-merged` into A's branch | A and B archived together (`root-archived`) | Unchanged |
| Child B's PR merged into A's branch but B still reads `implemented` | A refuses until B's own closeout runs; stuck forever if A's branch was deleted | A and B archived together (`root-archived`) after B's content is proven on main |
| B's PR merged into A's branch after A had already landed, so B's work is not on main | A refuses until B's own closeout marks it `stacked-merged`, then refuses again because B's work isn't on main (`blocked`, `descendant-carry-unproven`) | A refuses, naming B (`blocked`, `descendant-carry-unproven`). The content proof fails either way |
| Child B's PR still targets A's branch and is open (A was merged outside docket) | A refuses forever | A is archived as `done`. Cleanup keeps A's branch and reports `children-retarget-required` because an open child PR still targets it (unchanged) |

### What else stays fail-closed

- A `stacked-merged` child is a claim that it merged into its parent. If its live PR facts show a
  different destination, or can't be established, it still refuses the parent's closeout with
  today's tokens (`destination-mismatch` → `blocked`, `pr-unknown` → `unknown`). This applies to
  `stacked-merged` children only: for an `implemented` child, merging into the parent's branch is
  the test for being carried, so any other PR state just means "not carried".
- A GitHub lookup that fails is `unknown` and retried, as today. That is an observation failure,
  not a child blocking the parent. To keep it from coming from children the rule ignores, look up
  only descendants whose status is `implemented` or `stacked-merged` (grooming call). Today
  `probeDescendantFacts` also looks up archived `done` and `killed` descendants, whose facts the
  walk never reads. Filtering by status keeps the facts map built before the walk, so the
  domain/app split stays as it is.
- A carried child whose work is not proven on main still refuses the parent (`blocked`,
  `descendant-carry-unproven`), with zero descendant writes. The all-or-nothing transaction is
  unchanged.
- A root whose own `stacked_on` ancestry cycles still refuses, as today (`rootAncestryCycles`).

## One definition of "what a parent carries"

Today two walks answer this question differently: `DeriveCarriedSet` (rebase, publish, merge) and
`DeriveRootCloseoutSet` (root closeout). After this change there is one rule, shared by all four
boundaries. Widening the shared walk to accept an `implemented`-but-merged child also means rebase,
publish, and merge prove that child's work survives in the head about to merge, so a rebase that
dropped it refuses before the irreversible merge instead of after it.

Builder's call: whether `DeriveRootCloseoutSet` is retired in favour of `DeriveCarriedSet`, kept as
a thin wrapper, or both are rebuilt on one shared walk. What matters is that the four boundaries
cannot drift apart. `walkCarryChain` and `ProveDoneCarry` (the upward walk cleanup uses on `done`
children, from #567) are unaffected: after the parent's closeout every carried child is `done` with
a PR merged into its parent's branch, which is what that walk already expects.

Fix the header comment of `internal/app/finalize_preservation.go` while there: it says
`proveCarriedOnHead` serves "FinalizeRebase, FinalizePublish, FinalizeMerge, and stacked closeout",
but its callers are rebase, publish, and merge only (root closeout uses its own derivation plus
`proveCarriedContent`; stacked closeout uses `requireStackedPreservation`). After this change it
should name what is actually shared.

The archive transaction (`closeoutArchiveOp`) needs no new shape: it already applies
`domain.MarkDone` and clears `## Finalize blocked` per target, and `MarkDone` already accepts
`implemented` (the parent itself takes that path).

## The `children-retarget-required` closeout disposition goes away

With the rule above nothing produces the `not-stacked-merged` token or the closeout disposition
`children-retarget-required`. Remove both, and every place that documents them:

- `CloseoutDispChildrenRetargetRequired` in `internal/app/finalize_closeout.go`,
  `closeoutUnprovenDisposition`'s `not-stacked-merged` arm, and the `closeout_dispositions` set in
  `internal/app/schema_vocab.go`;
- `docs/reference/glossary.md`'s `closeout_dispositions` row;
- `skills/docket-finalize-change/SKILL.md` step 9's `children-retarget-required` bullet, and
  `skills/docket-finalize-change/references/gate-failure.md`'s "or a `children-retarget-required`
  closeout" clause;
- `skills/docket-convention/references/stacked-changes.md`: the routing sentence in *The stack
  close-out is idempotent*, the carried-descendant definition there (a carried child may come from
  `implemented` as well as `stacked-merged`), and *The `stacked-merged` state*'s claim that root
  closeout is the only consumer taking a change out of `stacked-merged` (it now also takes an
  `implemented`-but-merged child straight to `done`);
- the embedded copies under `internal/assets/embedded/tree/` (regenerate with
  `go generate ./internal/assets/`).

Cleanup's own `children-retarget-required` disposition (`CleanupDispChildrenRetargetRequired`) is a
different thing and stays: there it means an open child PR still targets the branch, so the branch
is kept.

`skills/docket-implement-next/references/edge-paths.md` gives a reason that becomes false: "an
ancestor's closeout cannot prove the carry of a named change not yet `stacked-merged`
(`children-retarget-required`)". Keep the behaviour (a `not-ready-stack-base-unresolved` refusal is
still never a closeout trigger, and `TestProseContracts`'s `change_0448_named_preflight_skip`
contract in `internal/repoguard/prose_contracts_test.go` still pins that sentence), but replace the
reason with a true one: the
bounded closeout covers only the named change's own `depends_on` set, and an unresolved stack base
clears through the parent's own finalize or the next sweep. Update the matching explanatory comments
in `internal/repoguard/prose_contracts_test.go` and `internal/repoguard/budgets_test.go`. Re-check
any prose budget those guards hold after the edit.

## Tests

- **Domain** (`internal/domain/stackcloseout_test.go`): the existing "descendant still in-progress is
  not-stacked-merged" and "a gap mid-chain is chain-broken" cases now describe allowed situations.
  Rewrite them to assert the new outcomes: a parent with an open, retargeted, `proposed`,
  `in-progress`, `killed`, or `done` child has an empty carry set and is proven; a stacked-merged
  grandchild beneath an open child is not carried. Add: an `implemented` child whose PR merged into
  the parent's branch is carried; an `implemented` child whose PR merged into main (or is open, or
  has no PR) is not carried and does not block; a `stacked-merged` child with mismatched or missing
  facts still refuses; a three-level chain through an `implemented`-but-merged middle link is
  carried end to end.
- **App** (closeout and the carry proof): parent closeout with a retargeted open child returns
  `done-archived` and writes only the parent; parent closeout carries an `implemented`-but-merged
  child to `done` (`root-archived`, the child in `carried_ids`), including when the parent's remote
  branch no longer exists; the content-unproven case still refuses with zero descendant writes;
  rebase, publish, and merge refuse when an `implemented`-but-merged child's work is not preserved
  at the target head; a parent whose retargeted child already merged to main (child still
  `implemented`) still merges and closes out; archived `done` and `killed` descendants are not
  looked up on GitHub (adjust any test that counts lookups).
- **End to end** (`TestE2EStackParentLandedFirst`): replace `archiveAsDoneOnOrigin` with the real
  parent closeout (`stackCloseout(t, s, parentID, "done-archived")`), then run the rest of the story
  unchanged. Delete `archiveAsDoneOnOrigin` if nothing else uses it.
- Mutation-test each new guard: restoring the old "every descendant must be stacked-merged" rule
  must redden the end-to-end test and the new domain cases; dropping the `implemented`-but-merged
  arm must redden its cases; skipping the content proof for that arm must redden its refusal case.

## Decision record

This reverses a documented rule (root closeout required every descendant to be `stacked-merged`)
and widens what a parent carries. Record it as an ADR: a stack parent carries exactly the children
whose PRs merged into it, whether or not their own closeout ran, proven in Git before any write; no
other child blocks the parent's closeout.

## Out of scope

- How a child's PR is retargeted, including a child whose PR still targets a parent that already
  landed (cleanup already keeps that parent's branch and reports it).
- Cleanup of carried children's branches and folders (done in #567).
- Letting implement-next's bounded closeout close out a stack ancestor to unblock a named stacked
  child.
- The killed-parent policy.
