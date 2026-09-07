<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0408 — Finalize publish is denied by the auto-mode classifier whenever the gate rebases](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0408-finalize-publish-is-denied-by-the-auto-mode-classifier-whene.md)**
<!-- docket:backlink:end -->

# Change 0408 — Diagnose finalize publish denials before selecting a remedy

## Decision and scope

The first implementation gate is a controlled comparison of publication through Docket Go and direct Git, across Claude Code versions 2.1.259, 2.1.260, and the installed current version (2.1.263 when groomed). The human approved this investigation-first direction and explicitly asked that a possible 2.1.260 behavior change be tested.

No permission grant, split publisher, or new recovery subsystem is selected in advance. Complete the bounded investigation and present its evidence and proposed remedy to the human before implementing a policy change or runtime redesign. An autonomous implementer must stop at that decision gate with an evidence-backed report using its existing halt contract; the presence of this spec does not authorize it to guess the unselected solution.

The eventual product objective remains unattended finalize where the host permits it, plus honest recovery after denial that preserves a still-valid green gate. Completing the experiment alone does not establish that the product defect is fixed.

## Corrected problem statement

During change 0404 on 2026-09-06, Claude Code 2.1.260 denied the Go finalize publisher in both the finalize child and the parent session. The child retried the same command and was denied again. A human-typed command recovered the run.

There is no evidence that every real rebase triggers this denial:

- Archived change 0100 records a plain Git force-with-lease denial on 2026-07-19, before the Go migration. This is a contemporaneous written record; the raw July transcript was not recovered during grooming.
- Commit 4c70044c56c4ec203c0d8e8441369f90e043afb9 introduced the Go finalize publisher on 2026-08-19 under change 0316.
- Change 0403 successfully rebased and published through Go on 2026-09-04 under Claude Code 2.1.259. Its parent transcript records auto mode. The finalize child reported rebased with a passed gate and then rewrite published.
- The pre-rebase head for 0403 was e04a07ec0f37c4a9a082d3b0f1e298745f632a77; the published head was 337b48b32c1bd4a113826895bbe95ae3bf57d35f. A local Git ancestry check returned false: this was an actual history rewrite, not a fast-forward or already-published no-op.
- Additional recovered Go results report rewrite published for 0384 on 2.1.252 and 0364 on 2.1.258.

Thus command presentation, harness version, loaded policy, model, and session context are hypotheses. The generic 0404 denial does not identify a particular classifier rule. The original change title describes the initial report; this corrected body and spec govern implementation.

## Changelog evidence

The [2.1.260 changelog](https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md#21260), reviewed 2026-09-07 UTC, lists Bash permission fixes, a Read-rule rollback, corrected managed-policy loading, and changed sandbox treatment of human-typed commands. It does not announce a force-push classifier rule. The managed-policy change warrants checking effective settings; human-shell success is not a classifier control. The following release also fixes version-gated feature flags reaching older clients, so a binary-version pin alone cannot prove identical server-side behavior.

These are investigation leads, not a diagnosis. Preserve a source snapshot or immutable source revision in the experiment report, and distinguish documented changes from inferred relevance.

## Baseline matrix and bounded execution

Use the same Docket source revision and binary digest throughout the comparison; varying Docket and Claude together would confound the result.

| Axis | Required baseline |
| --- | --- |
| Claude version | 2.1.259; 2.1.260; current installed version, recorded exactly |
| Publication form | Current Go finalize publisher; visible direct Git with the equivalent exact old-value lease |
| Launch context | Finalize child in an auto-mode parent; reproduce any differential in an attended auto-mode parent separately |
| Git state | Equivalent actual non-fast-forward updates of disposable feature refs on the same approved remote |
| Policy | Same declared effective settings, managed policy, and supported model; no custom rewrite exception in the baseline |

Start with two independent matched pairs per version in the finalize-child context: twelve primary attempts. Alternate arm order between pairs. If a differential appears, repeat that version's pair twice in the attended-parent context. Do not run an unbounded search for a desired classifier result; mixed results remain mixed and any larger study requires a separately justified scope.

Resolve historical binaries through a supported distribution, isolated from the user's installed executable. Verify the selected binary version on every launch. Do not silently substitute current for an unavailable historical version. If historical auto mode or the required model is unavailable, record that limitation and report the affected cells as unavailable.

Do not change global permissions, shared project policy, branch protection, or default-branch history to manufacture a result. The test target must be explicitly approved and contain only disposable test data. Define the entire pair, including both effects, before executing either arm. An independently authorized comparison is not an alternate-tool retry of a denied production command. Stop denied cells according to the host's permission rules.

Use local bare-remote fixtures first to verify test setup and lease behavior. Such fixtures establish Git correctness only; they do not establish the classifier's behavior for GitHub. A live arm needs an actual approved GitHub test target. If that target or authorization is unavailable, return the complete experiment preparation and the exact missing prerequisite without performing external writes.

Each paired arm starts from equivalent fresh refs, PR state, owned workspace/receipt state, and green evidence. The remote old commit must not be an ancestor of the intended new commit. Record this precondition and independently verify the resulting remote ref after each allowed attempt. A no-op, fast-forward, dry-run, help invocation, early evidence rejection, or unrelated command failure cannot count as an allowed rewrite.

The direct Git arm must preserve Docket's explicit ref-and-old-OID lease. A bare force or implicit tracking-ref lease is not equivalent. Track the PR-evidence update separately: Go composes a ref update and PR-body convergence, while a visible Git command exposes only the former. A Go-only denial does not prove binary opacity unless compound-effect and PR-update differences have been accounted for. Any supplementary phase-isolation probe must be declared and bounded before it runs.

## Measurements and interpretation

Capture, without credentials or unrelated transcript content:

- UTC timestamp, Claude version and binary identity, Docket revision/digest, model, launch path, permission mode, and parent/child context.
- Effective permission and auto-mode configuration, managed-policy load state, sandbox settings, working directory, and prompt/consent wording. Store redacted snapshots and stable digests; note anything that cannot be observed.
- Exact tool command, whether the host allowed execution, the original denial text, whether the process started, and separate ref-push and PR-update outcomes.
- Old/new refs and OIDs, current base OID, lease expectation, evidence head/command, and verified final remote state.
- Session and tool-event identifiers sufficient to locate original evidence.

Do not infer a historical configuration from the current machine. Do not label a child's mode as directly observed when only its parent records it. Do not infer that an error means permission denial, or that exit zero means a rewrite occurred.

| Result pattern | Permitted conclusion and next proposal |
| --- | --- |
| Both forms succeed on 2.1.259 and fail on 2.1.260 under otherwise matched conditions | Evidence supports a version-associated change; investigate policy-loading or classifier changes and test current before proposing a Docket refactor. |
| Go repeatedly fails while equivalent visible Git succeeds within one version/context | Evidence supports a presentation/composition difference; evaluate a transparent prepare/execute/verify interface while preserving Docket's authority checks. |
| Both forms fail across versions | A general authorization or context limitation is more plausible; present supported scoped permission configuration and attended recovery as options. |
| Current succeeds while historical failing version still fails | Propose a version-scoped compatibility recommendation, with the recovery requirement retained. |
| Mixed, unavailable, or confounded cells | Report the uncertainty and the smallest remaining discriminator; no causal claim, blanket allow rule, or speculative runtime redesign. |

The small sample is a bounded diagnostic, not a statistical guarantee. Server-side policy or feature state that cannot be held constant must remain an explicit limitation.

## Recovery requirements for the subsequently selected repair

Retain the existing exact lease and response-loss behavior in PublishRewrite: already at the intended head is a no-op; a changed remote is contention; an unobservable remote provides no permission to force or repeat an uncertain write. FinalizePublish must continue to preserve authored PR-body bytes outside the managed evidence block.

A persistent publish checkpoint is a candidate, not a chosen design. The current RebaseReceipt stores a live gate continuation but no completed evidence. processFinalizeGate.mapDriveOutcome mints evidence into the command response and removes the raw run root; the skill writes a temporary evidence file. Re-entering composeLocalGate after a completed real rebase may therefore start another suite.

Any chosen recovery must retain verified evidence before an external publication can be denied, supply a durable concrete resume path, and reuse that evidence only while repo/change/PR identity, local head, effective base, gate policy, and resolved test command remain valid. It must recheck current authority and the receipt's lease. A moved head/base or changed command invalidates reuse; “never rerun a green gate” applies to still-valid evidence, not stale evidence. Repair sign-off and independent build/finalize policies remain in force.

A host denial often prevents the binary from running at all. The skill must distinguish that from a Go operation result, use only the permitted exact retry, and report the specific denied action. Persistent failure follows the existing Finalize blocked mechanism with a valid resume instruction. If recording the block is also denied, report that separately; do not pretend the marker exists.

## Deliverables, acceptance, and stopping rule

The first gate delivers an evidence report, the exact reproducible setup/run instructions, a populated matrix with explicit unavailable cells, and a proposed remedy tied to the observations. Keep transcript excerpts limited to the relevant events. The report may be a research artifact; it must not claim the defect fixed.

Before any further code or permission change, present the measured conclusion and concrete repair design for the human's selection. The earlier suggestion of an autoMode.allow rule was not accepted as a prerequisite. Do not silently treat an inconclusive probe as that acceptance.

For an eventual runtime repair, use meaningful behavioral tests for denied publication, durable restart recovery, unchanged evidence reuse, stale head/base/command refusal, competing remote updates, and push-success/response-loss recovery. Assert that the suite is not invoked on valid resume, and mutation-test the guard. Run the full build/finalize suite from resolved configuration at the corresponding build gates, and handle authoritative runtime-budget breaches per AGENTS.md. The live classifier comparison is an attended acceptance activity, not a network-writing default test-suite case.

## Related work and exclusions

Related: 0100 (pre-Go denial), 0260 (denial posture), 0316 (Go publisher), 0360 (evidence/coordination concerns), 0396 (durable running-gate continuation), 0403 (successful counterexample), and 0404 (observed failure). None is an unmet implementation dependency. ADR-0043 records the merge-side policy decision; ADR-0105 records the existing continuation design. Neither decides this publish-side policy question.

No change to Claude Code itself, no bot approval, no branch-protection change, no merge-method redesign, no generic permission-bypass mechanism, and no automatic expansion of trusted infrastructure. No production Git rewrite, permission edit, or implementation was performed during grooming.
