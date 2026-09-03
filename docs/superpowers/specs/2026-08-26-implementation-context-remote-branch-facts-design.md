<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0357 — Implementation context must load remote branch facts before judging stack base](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-27-0357-implementation-context-loads-remote-branch-facts.md)**
<!-- docket:backlink:end -->

# Implementation context loads remote branch facts

**Change:** 0357 · **Date:** 2026-08-26 · **Status:** Approved design

## Summary

`docket context implementation` is the read-only gate that tells `docket-implement-next` which change it may claim. It currently evaluates every candidate with an empty `BranchFacts` value. That makes a proposed child stacked on a live parent look unbuildable even when the parent's recorded branch exists on the remote: effective-base rule 4 cannot see the branch, readiness becomes `stack-base-unresolved`, and the workflow stops before its facts-backed claim transaction can run.

The operation will load remote branch facts through its existing `StatusReader`, using the same pinned context and the same `stackBranches(snapshot)` inventory already used by status, claim, and workspace preparation. One returned fact set will drive selection, eligibility, readiness, and effective-base reporting. This restores the motivating workflow: a child declared with `stacked_on: <parent>` can pass the implementation gate, be claimed, and build from the live parent's separate pushed branch.

## Motivating outcome

Given:

- a proposed, designed child carrying `stacked_on: <parent id>`;
- a live parent whose change record carries `branch: <parent branch>`; and
- that exact branch present on `origin`;

both automatic selection and `docket context implementation --id <child id>` must return an applied context bundle. The bundle must report `build-ready`, `claim_eligible: true`, and an effective base resolved to the parent's recorded branch. The normal claim and workspace operations can then prepare the child's workspace from that branch, preserving the parent's unmerged work.

This is specifically the downstream consumer-repository failure that created change 0357: child 0003 was stacked on parent 0004, the parent's feature branch existed on `origin`, status and stack-base resolution said ready, but implementation context refused before claim.

## Design

### Load facts once after snapshot construction

After `ContextImplementation` pins context, reads the corpus, and builds the domain snapshot, it will call:

```go
deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))
```

The call must use the existing `StatusPin` returned at the start of the operation. `stackBranches` derives the complete, deterministic set of recorded stack-ancestor branches that `ResolveEffectiveBase` may consult. No new branch-discovery rule or second Git seam is introduced.

The existing empty-facts construction and its comment will be removed. An empty result returned by the reader remains a valid fact set meaning none of the requested branches exists; skipping the read entirely is not equivalent.

### Thread one fact set through the whole decision

The returned facts will be passed to `selectContextChange`. The same value will then drive the bundle's `EvaluateReadiness` and `ResolveEffectiveBase` calls. Consequently:

- automatic selection and explicit-id inspection use the same remote evidence;
- the selected change's eligibility and reported readiness cannot disagree because of separate fact reads; and
- the reported effective base is the one whose resolution licensed the bundle.

No claim mutation occurs here. Claim continues to re-read the corpus and branch facts and re-prove eligibility inside its transaction, preserving the existing stale-observation safety boundary.

### Preserve existing stack semantics

This change supplies missing evidence to the existing domain resolver; it does not modify its rules:

- an unstacked change resolves to the integration branch;
- a child of a `done` parent resolves terminally to the integration branch;
- a live parent resolves to its recorded branch only when that branch is present in facts;
- a branchless `stacked-merged` parent follows ADR-0092's recursive rule; and
- a live parent's missing remote branch remains `stack-base-unresolved`.

The last case is intentional. Docket must not claim or build a child from an invented fallback when the parent branch has not been pushed or has disappeared.

## Failure handling

If `BranchFacts` returns an error, `ContextImplementation` will classify it through the existing `classifyStatusError` path and return no context bundle. A wrapped external-reader error therefore remains `external-failed`; cancellation remains interrupted; unexpected contract failures remain internal errors. The operation must not translate a failed lookup into an empty fact set, because that would misreport an observation failure as proven branch absence.

Artifact reads and all other typed context outcomes retain their current behavior.

## Regression coverage

### Focused orchestration tests

`internal/app/implementation_context_test.go` will cover:

1. A proposed designed child stacked on a live parent, with the parent's recorded branch in the fake reader's facts. Automatic selection and explicit-id variants both return applied bundles with `build-ready`, claim eligibility, and the parent branch as the effective base.
2. The reader is asked for the deterministic `stackBranches(snapshot)` set using the original pin.
3. The same child with the parent branch absent remains refused as `not-ready-stack-base-unresolved`.
4. A branch-facts reader failure returns the existing typed failure and no partial bundle.

The present unstacked tests continue to prove integration-branch behavior is unchanged. Stale test comments that say implementation context deliberately uses empty facts will be corrected.

### Real-Git workflow regression

The existing planning/claim Git fixtures will create both metadata modes with:

- a live parent record carrying a non-integration recorded branch;
- a proposed, designed child stacked on that parent; and
- the parent branch pushed to the bare test origin and advanced beyond the integration branch.

The regression will run production `ContextImplementation` through `NewGitStatusReader`, then continue through claim and workspace preparation. It must prove:

- implementation context applies rather than returning `stack-base-unresolved`;
- the bundle resolves the exact recorded parent branch;
- claim succeeds under its independent in-transaction re-proof; and
- workspace preparation uses the parent branch tip, not the integration-branch tip.

Removing or bypassing the new `BranchFacts` call must make this regression fail at the original pre-claim gate. This joins the existing downstream effective-base coverage rather than replacing it.

## Scope

Expected production work is confined to `internal/app/implementation_context.go`. Tests belong in `internal/app/implementation_context_test.go` and the existing real-Git planning/claim workflow test area. Comments whose premise changes are maintained with those tests.

Out of scope:

- changes to `ResolveEffectiveBase`, readiness precedence, ADR-0092, or `stack-base.sh`;
- other `NewBranchFacts(nil)` sites that do not gate this implementation-context path;
- stack close-out reachability and stale-parent-worktree protections tracked by change 0327;
- branch naming and recorded-branch identity behavior already handled by change 0347; and
- Cursor wrapper quoting tracked by change 0356.

## Acceptance criteria

- A proposed child stacked on a live parent with a pushed recorded branch is available through both automatic and explicit-id implementation context.
- Its context bundle is build-ready, claimable, and resolved to the parent's recorded branch.
- The child can proceed through claim and prepare a workspace based on that branch's tip.
- A genuinely absent parent branch still refuses as stack-base unresolved.
- A failed branch-facts lookup produces a typed operation failure, not a false absence.
- Focused tests and the repository's configured full suite pass.
- No stack-resolution policy or ADR change is introduced.
