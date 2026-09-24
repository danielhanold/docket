<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0449 — Unrelated invalid change records must not block a named change's metadata writes or board](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-24-0449-unrelated-invalid-change-records-must-not-block-a-named-chan.md)**
<!-- docket:backlink:end -->

# Change 0449 — Unrelated invalid change records must not block a named change's metadata writes or board

## Origin and ordering

Split out of change 0446 on 2026-09-23 (its former §8, the metadata-related grounding, and acceptance tests 10–12). Built last, in succession after 0446 (gate history and run bookkeeping) and 0448 (named start skips unrelated maintenance), which `depends_on` enforces. Because it lands last, it also owns the end-to-end named-workflow acceptance test (test 2 below), which exercises all three changes together.

## Required outcome

An explicitly named change B must be able to complete its metadata writes—claim, refresh/reconcile, plan/results attachment, lifecycle writes, required ADR writes, finalize block/clear-block, and final archive/closeout—despite unrelated invalid records in another change A. The canonical board must still update for B. Named reads must not fail on live facts B does not need.

This must not accept new damage. Defects in B, in records B actually requires, new or changed errors anywhere, illegal evolution, unapproved source changes, shared configuration failures, and failed corpus reads remain refusals. The guarantee is for explicit change IDs; operations without an explicit subject contract keep strict whole-corpus validation.

## Grounding

Inspected main `442770e11bd05baf6200bec53e9c4757622811a6` on 2026-09-23 (read-only).

- **#0309, #0310, and #0312:** the transaction design intentionally requires error-free complete before/after reports. `planningLoader.Load` reads the full corpus and reports malformed documents as error findings (not Go errors), and sets `Sources` only after a successful parse. `Engine.runCandidate` refuses when `before.Report.HasErrors()` before consulting the operation. `change.claim` (both claim and refresh-claim) and `runCloseoutArchiveTransaction` (and `closeoutStacked`) use that loader. Thus an invalid independent A blocks B's claim and final archive. `TestEngineBeforeGateRefusesInvalidBase` asserts the refusal before planning; `TestNewPlanningLoaderParseFailureIsFinding` proves only that a parse failure becomes an error finding excluded from the snapshot, with no Go error. The contract must be deliberately narrowed, not bypassed at a single caller.
- **Existing inputs:** `LoadedState`, exact tree bytes/blob ids, `domain.Finding{Code, Severity, Entity, Field, Related, Detail}`, `domain.ValidationReport.HasErrors`, the request's `Expected []EntityExpectation` (known before `Plan`), the plan's declared `MutationPlan.Files` (known only after `Plan`), and existing before/after validation. There is no existing "operation subject" concept; this change adds one only as a bounded in-memory input.
- **Precedent:** change 0337's `backlinkArtifactLoader` (`finalize_backlink_loader.go`) is already a scoped `StateLoader` whose stated rule is that "a pre-existing error in a record the mutation cannot touch must not refuse the patch." It reads only its targeted paths and has a no-op `ValidateEvolution`, so it cannot serve a full-corpus mutation as is. This change generalizes its rule while keeping the complete corpus read.
- **ADR-0093:** structural references gate authority; associative `related`/`discovered_from`/`adrs` links do not. Preserve that distinction when determining required subjects. Do not turn a cited historical ADR into a new execution dependency merely because its id appears in `adrs`.
- **Board:** `includeBoard` invokes the one canonical board renderer. `boardClassify` errors on an active-location change with a non-active or unknown status, and `boardReadinessCell` errors on unexpected readiness (duplicate/negative id, unusable slug). Either error aborts the whole render, and the claim's `Plan` surfaces it as an external plan-stage failure rather than a refusal. Merely relaxing the transaction gate would leave that blocker. Conversely, an **unparseable** record never errors the board today: it is silently absent from `Documents`, so its row, its counts, and dependents' readiness cells change without notice.
- **Finding volatility (from `internal/repository/validate.go`):** `cycleFinding` attributes to its first member by id only (no path), with `Related` and `Detail["members"]` listing every member; `graphNodes` drops duplicate-id nodes, so a collision can make cycles appear or vanish. Duplicate findings carry `Detail["count"]`. Dangling references carry `Detail{"lookup":"absent"|"ambiguous","target":id}`, and `depends_on`/`stacked_on` dangling references are errors. If B depends on an unparseable A, B itself carries an error-severity dangling-reference finding. Parse findings have a path-only entity and no detail. `compareFindings` sorts only by code, entity, and field, ignores `Related`/`Detail`, and keeps input order for ties.
- **Live branch facts:** `ContextImplementation` calls `BranchFacts(stackBranches(snap))` before selecting B; `WorkspaceInspect` (via `loadWorkspaceContext`) resolves B first but still fetches every stack branch in the corpus. `gitStatusReader.BranchFacts` records an unavailable ref as `false`, but any other fetch failure for any branch fails the whole call.

Relevant learnings: verify the hypothesis against code; do not mistake missing evidence for a fact about an unrelated subject; test the producer/consumer path and its negative counterpart.

## Design

### 1. Isolate metadata validation without accepting new damage

Keep the transaction engine, complete corpus loader, semantic planners, exact-version expectations, evolution checks, declared-path enforcement, lease push, and retry/replay machinery. Do not weaken `ValidationReport.HasErrors`, discard unhealthy records, or globally convert errors to warnings.

The necessary extension is a bounded **in-memory validation subject set** on the existing metadata operation/transaction boundary. A named operation supplies its actual root identities from its existing request; the shared validation path resolves them from the freshly loaded state. The engine's before-gate runs **before** `Plan`, and the plan's declared `MutationPlan.Files` exist only after it. The initial scope must therefore come from the request's ids and its `Expected []EntityExpectation`, with plan paths unioned in at step 2. This is required because the present engine has no way to distinguish A's error from B's. No user-supplied ignore list, error-code allowlist, plugin policy API, persisted baseline, or second transaction engine is introduced. Operations without an explicit subject contract retain strict whole-corpus validation; an accidentally empty subject set must never mean "ignore every error."

Define relevance using existing typed identities, paths, findings, and domain relationships:

- Roots include the selected change and any other entities the operation explicitly acts on. Required records include the dependencies, stack ancestors/descendants, ADRs, artifacts, and PR/base/branch identities whose validity the particular operation uses for authority. An associative ADR citation alone is not such a dependency (ADR-0093); an ADR being recorded/superseded and its structural targets are. Reuse existing domain traversal and operation preconditions; do not follow informational `related`/`discovered_from` links into a repository-wide closure. Enumerating records to discover children or collisions is not making every enumerated record a dependency.
- Include every authoritative record the plan creates, edits, deletes, or carries through a multi-record closeout. Resolve required identities in both before and candidate states; a removed dependency must not erase a before-state obligation. Duplicate IDs/paths/slugs or shared identity claims affecting a root are relevant even when the conflicting finding is reported on another record. Check `Related` members as well as a finding's primary `Entity`; a cycle with B as a non-primary member still blocks B.
- Missing, malformed, or ambiguous required records remain local refusals. A B that depends on an unparseable A refuses, because A is a required record. A malformed sibling with no usable identity or required path is reported by its own path, not treated as a possible duplicate of every ID. Do not add a permissive alternate parser to mine identities from invalid YAML. Proven collisions and malformed files at B's required path remain blocking. Unattributable loader/invariant errors, failed corpus reads, invalid shared configuration, and actual repository-wide contradictions cannot be grandfathered as A's harmless error.

The transaction sequence for a scoped operation is:

1. Load the complete fresh base and all findings. Preserve raw source bytes for **every** corpus blob, including a document that fails parsing; today `planningLoader.Load` only populates `Sources` after a successful parse. Parsing failures remain findings; an I/O failure is still an error. Resolve the required base subjects and reject their errors before planning. Check all existing exact-version expectations.
2. Run the existing read-only semantic planner. It must tolerate unrelated invalid records while retaining its local preconditions. Union newly discovered required subjects and the plan's authoritative record paths into the scope, then recheck relevant base errors before any effect. Planning remains pure; it cannot publish, merge, or mutate workspace state to discover this scope.
3. Load and validate the complete candidate overlay with the existing validators. Reject all relevant errors and every new or changed error anywhere. The only errors allowed to remain are exact pre-existing findings confined to unrelated records whose source bytes and paths remain unchanged.
   - Compare the complete structured finding (including `Related`, `Detail`, severity, and multiplicity), not just a code or total count. Compare as multisets with canonicalized collection ordering; `compareFindings` cannot serve as the equality test.
   - Deciding that a finding is "confined to unrelated records" needs a mapping from the finding to record paths in both states. Resolve every id in `Entity` and `Related` to paths in the before and candidate snapshots. A finding whose ids cannot be resolved to unrelated, unchanged paths is relevant, not grandfathered.
   - "Unchanged source bytes" is proven from raw tree blob ids for every corpus path, since a parse-failed record's bytes are absent from `LoadedState.Sources` today (step 1).
   - Findings whose `Detail` counts or lookup outcome change because of B's edit are "changed" and refuse: a duplicate's `Detail["count"]`, a dangling reference flipping `absent`↔`ambiguous`, or a cycle gaining or losing members. That is correct, because B's edit is what changed them.
   - Match B's own findings by identity, not path, because closeout moves B from `active/` to `archive/`.
   - A disappearance is not a license to introduce a different error. Never persist this baseline; derive it again from each attempt's fresh base.
4. Apply all existing evolution and declared-write checks without exceptions, including frozen-record protection. Scoped validation must not allow a planner to edit or drop A merely because A was invalid already. Recompute scope and baseline after lease contention; reuse neither from a stale attempt. Preserve idempotent lost-response replay and exact request attribution.
5. Return unrelated health findings through existing findings surfaces without changing an otherwise applied/no-op operation into failure. Preserve their health severity; consumers must use the operation's typed disposition rather than a blanket "any error finding means stop." Read-only health checks continue reporting the whole repository's defects.

### 2. Thread the contract through the named workflow population

Apply this contract to the real named implementation/finalize mutation population, derived from production transaction callers and workflow consumers. Claim, refresh/reconcile, plan/results attachment, lifecycle writes, required ADR writes, finalize block/clear-block, and archive/closeout must not leave a strict global-validation chokepoint on B's path. This list is illustrative, not an exhaustive allowlist. Integration-side backlink transactions already load their exact artifact subjects; retain their local strictness. Check named contexts and pre-effect guards before PR publication/merge as well: B must pass relevant validation before an external effect, while A's unrelated findings cannot veto it. Reuse the same relevance rules rather than adding divergent per-command exceptions.

### 3. Bound named live branch-fact reads

Bound required live branch-fact probes in every named reader/mutator to B's actual base/stack needs, including context, claim, workspace, merge, block, repair, and retarget preflights. Derive the callers of `stackBranches`/`BranchFacts` rather than treating this list as exhaustive. An unrelated broken branch name is not a transport failure for B. A failed fetch of a branch B actually requires is still a refusal.

### 4. Generated views tolerate unrelated defects

Extend the existing canonical board renderer to report unrenderable records by path in a small repair notice while rendering usable records, including B, normally. Pass existing parse/validation findings so an unparseable A remains visible even though it has no decoded row; today such a record is silently dropped from the board, and the repair notice closes that gap too. Normal counts describe rendered records; the repair notice separately identifies unrenderable records, without claiming a complete healthy count. Do not invent a lifecycle status, configurable board group, alternate renderer, or silently omit A as if the repository were healthy. Do not feed a filtered snapshot into authoritative dependency/identity checks. Keep unrelated record bytes untouched and normal derived-view updates atomic with B's metadata mutation. Audit any required ADR-index/artifact render through the same consumer trace; locally invalid required artifacts still refuse. Renderer/programmer/configuration faults remain real failures, never silently skipped.

## Acceptance tests

1. **Metadata isolation and safety:**
   - Use real production loaders and transaction planning in isolated repositories.
   - Progress cases: seed unparseable A, invalid status/placement, duplicate identities confined to other changes, an unrelated dependency cycle, and invalid unrelated ADR/history. B's claim, reconcile/attachment, mark-implemented and final closeout succeed. A's source bytes stay identical, findings stay visible, and B's board row updates atomically with a repair notice for unrenderable A.
   - Paired refusals, each refusing before the relevant effect: corrupt B, a missing/corrupt actual dependency or artifact (including B depending on unparseable A), duplicate B identity, a cycle whose primary finding is A but includes B, a required closeout descendant, and a shared invariant failure.
   - A changed or new error must fail even when its code/count matches an old one. Include id-only findings (a cycle attributed to A by id whose `Related` contains B), a duplicate whose `Detail["count"]` changes because of B's edit, a dangling reference flipping `absent`↔`ambiguous`, and ties that `compareFindings` would order by input position.
   - Test omitted/empty scope, dynamically added subjects, frozen-record writes, unrelated source edits, lease retry with a changed base, and idempotent replay.
2. **Complete named workflows (end to end across 0446, 0448, and this change):**
   - Keep the same broken A and mixed old runtime records present throughout.
   - Implementation: exercise B by ID through preparation, authoritative context, claim, required metadata mutations, build/gate, publication and mark-implemented.
   - Finalize: separately, run named finalize through its gate, merge, archive/closeout and cleanup. Cover merge-already-landed recovery and scratch removal.
   - Use production validation/rendering/admission paths with existing isolated Git repositories and fake external/process seams; do not touch live PRs.
   - Introduce A's unrelated defect between B's merge and closeout to prove it cannot strand B there. Verify B's own defective inputs refuse before merge. Include named branch-fact reads with an invalid unrelated branch.
   - Starting a finalize gate alone is not sufficient evidence.
3. **Mutation checks:**
   - Progress tests must fail if you restore either global before/after validation veto, renderer-wide abort on A, a strict global validator at an intermediate workflow write, or whole-corpus branch-fact reads on a named path.
   - Safety tests must fail if you remove relevant-subject validation, compare only error counts/codes, omit `Related` members, discard malformed source bytes, permit new errors, or let an empty scope disable validation.
4. **Build gate:** run the whole suite via the source Go runner using resolved `build.test_command`, and inspect budget findings. Use existing fixtures and process seams; no new test framework.

## Final design logic check

The following boundaries must all hold together once 0446, 0448, and this change have landed; passing only one change's tests is insufficient.

| Boundary for named B | Why unrelated A cannot stop it | What must still stop B | Owner |
|---|---|---|---|
| Startup and retry/resume | Named path does not dispatch incidental maintenance or select from a global queue. | Invalid B, wrong lifecycle, failed repository preparation, or unresolved B ownership. | 0448 |
| Context and live facts | Full metadata read retains findings; required validation and branch probes use B's actual subjects. | Missing/ambiguous required identity, invalid base, or a failed required fact read. | this change |
| Metadata writes | Each fresh attempt admits only unchanged unrelated findings; pure planning, scope expansion, full candidate validation and evolution checks remain. | Relevant before/candidate defects, new/changed errors, illegal evolution, source tampering, or version/lease conflict. | this change |
| Generated views | Canonical renderer updates B and reports unrenderable A by path instead of aborting the entire render. | Invalid required artifact or a real renderer/configuration failure. | this change |
| Execution and run closeout | Local slot/epoch authority, target-only accounting, deterministic replacement ownership and durable release exclude obsolete siblings. | Live/unproven incumbent, owned pending mutation/launch, corrupt required ownership evidence, or failed release persistence. | 0446 |
| Named finalization through archive | The same metadata contract applies after merge and during already-merged recovery, not just to the finalize gate. | B's unmet merge/stack/evidence/approval conditions or unsafe B cleanup. | this change |

Safety cross-checks: an empty scope does not disable validation; dynamic plan subjects are rechecked; `Related` findings and before-state dependencies cannot escape attribution; A's malformed bytes are preserved; generated presentation never becomes authority. Shared infrastructure failures are not reclassified as unrelated record findings.

## Architecture and scope limits

Through the ADR workflow, record the scoped-validation departure from #0309's complete-error-free mutation contract, grounded in #0310/#0312, change 0337's scoped backlink loader, and the canonical generated-view design. Keep full-corpus health reporting, strict unscoped operations, and existing maintenance commands. Accepted ADR text is not rewritten in place.

The only new validation input is the bounded in-memory subject set on existing operations; it is not a persistent schema or configurable policy. No new commands, flags, persisted baselines, alternate renderer, lifecycle status, or permissive parser. Gate admission and run bookkeeping belong to 0446; named-start maintenance belongs to 0448.
