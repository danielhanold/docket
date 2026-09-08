<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0410 — Require durable results artifacts with human testing and coordinator findings](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-08-0410-require-durable-results-artifacts-with-human-testing-and-coo.md)**
<!-- docket:backlink:end -->

# Required results artifacts and coordinator checkpoint capture

Date: 2026-09-07
Status: Approved design from the human brainstorm; implementation is separate.

## Problem

Docket already models a results artifact, but the maintained implement-next workflow still makes it optional. Its current three-section template leaves much of the coordinator's handoff in a transient final report. Useful human functional checks, findings, limitations, and out-of-scope work can disappear when that session ends.

Restore a dependable handoff for every implemented change, including trivial changes. Preserve meaningful discoveries during implementation without creating a transcript, a second backlog, or empty documentation.

## Assumptions

- This change specifies future Docket behavior; this grooming session creates the proposed change and spec only.
- The human approved the complete template below and the decisions in this spec: mandatory artifacts; omit empty sections; human functional checks only where automated tests do not cover the behavior; checkpoint capture; and follow-ups held for human triage without automatic change creation.
- The coordinator can preserve only information it has received or observed. Findings trapped in an abruptly terminated child or not yet captured before a crash may still be lost.
- Docket's existing feature-workspace ownership, metadata transactions, gate driver, evidence, and artifact backlink operations remain authoritative. Repository access is not permission to bypass these contracts.
- The four currently supported harnesses are Claude, Codex, Cursor, and OpenCode. The implementation must derive its coverage inventory from the supported adapter registry and include any additions present at reconciliation.

## Current behavior and related work

Source inspected at main revision 0d1a7e1b36af36c1f805952cd3836e84dbad5b6f.

- skills/docket-implement-next/SKILL.md, Step 6.5, conditionally writes and attaches results after build and review. Its Step 7 postcondition uses an optional results conjunction.
- skills/docket-implement-next/results-template.md contains Verify (human), Findings, and Follow-ups, directs callers to omit the file when no trigger fires, and keeps build receipt details in the PR.
- ChangeAttachResults in internal/app/change_attach.go verifies the owned workspace head, tracked regular file, configured results directory, backlink, and exact change version before attaching the path. It does not require a result at completion or validate the new content contract.
- ChangeMarkImplemented in internal/app/change_implemented.go and RunVerify in internal/app/run_verify.go check results identity only when a path is present. Missing results can therefore coexist with a successful completion.
- Change 0001 introduced the optional artifact. Changes 0170, 0190, and 0218 concern evidence, post-gate results commits, and review findings respectively.
- Change 0330 deliberately keeps already-known late verification outcomes and findings in terminal Closeout notes. Merged results remain frozen.
- Change 0360 contains a broader coordination/evidence umbrella, including the results-only delta problem. This change owns mandatory results and capture sequencing; it does not implement that umbrella or a new ancestor permit.
- Change 0374 and ADR-0102 define independent build/finalize commands and exact-head evidence reuse. Preserve those decisions.

The old edge-paths reference still describes an ancestor/results-only permission that current finalize prose explicitly does not support. Reconcile that contradictory guidance where it touches this workflow; do not activate a deferred capability.

## Chosen approach

Keep one human-readable Markdown results file per change in the existing configured results directory. Strengthen the shared coordinator instructions and the Go completion checks together. Reuse existing artifact attachment and Git durability primitives.

A template-only change would leave omission possible. A continuous observation log would add writes and recovery complexity while reducing readability. The chosen approach captures at meaningful checkpoints and consolidates the same file before completion.

The coordinator remains accountable for completeness and accuracy. Go verifies structural completion conditions and artifact identity; it cannot infer whether every real-world observation was included or whether prose is useful.

## Authoring template

This is the canonical authoring template, to replace results-template.md. Angle-bracket instructions are authoring guidance only. Remove them from actual artifacts. Omit an entire optional section, including any subsections, when there is no substantive content.

The existing artifact.backlink operation owns the generated backlink above the title. Do not hand-author its markers. The backlink provides the change identity and navigation to its spec, plan, and PR; do not add empty header fields for unavailable links.

```markdown
# <Change title> — Results

## Outcome

<What was delivered and how the behavior changed.
Explain any material departures from the spec and why.>

## Human testing

### <Functional scenario not covered by automated tests>

<Prerequisites and setup needed for this scenario.>

1. <Human action.>
   Expected: <Observable behavior.>
2. <Human action.>
   Expected: <Observable behavior.>

<Cleanup instructions, only when needed.>

## Verification performed

<Concise account of checks the agents actually performed,
their outcomes, and links to supporting evidence.
Identify skipped, failed, or incomplete verification explicitly.
Do not reproduce test logs or individual automated test cases.>

## Findings and limitations

### <Finding>

<What was observed, supporting evidence, and practical impact.
Distinguish confirmed problems from suspected issues.
Explain any workaround or remaining limitation.>

## Follow-ups

### <Actionable follow-up>

<Problem or opportunity, supporting evidence or reproduction
details, why it falls outside this change, and suggested next action.
Link an existing change when available.>
```

### Content rules

1. Outcome is required at finalization and describes the delivered behavior, even for a trivial change. Keep it proportional; a short factual sentence can be sufficient. Include material spec departures and their reasons. At an intermediate checkpoint, describe actual progress truthfully without claiming the change is complete.
2. All other sections are conditional. Omit empty headings, empty lists, placeholder instructions, and filler such as None, N/A, Not applicable, or No findings. This applies to nested headings and optional link/cleanup fields as well.
3. Human testing contains only functional scenarios needing human verification that automated tests do not cover. Give sufficient prerequisites, concrete human actions, and observable expected results. Include cleanup only when the scenario requires it.
4. Do not ask the human to rerun a unit/integration/full test suite, translate an automated test into manual steps, or recheck behavior already covered by automation. For partially covered workflows, specify only the uncovered functional assertion; shared setup can be repeated when needed to reach it.
5. Do not infer coverage from a green suite. Inspect relevant tests and returned evidence. If coverage is uncertain, report that uncertainty under Findings and limitations or Verification performed rather than presenting a speculative human testing checklist. A skipped automated check remains an automated-verification gap; it does not become a human task.
6. Verification performed is a concise record of actual agent checks, their outcomes, and useful evidence pointers. Identify skipped, failed, or incomplete checks truthfully when relevant. Do not copy full logs, enumerate every automated case, or turn this section into human instructions. Distinguish reported child results from independently verified evidence where that difference matters.
7. Findings and limitations preserve discoveries, unresolved issues, material risks, workarounds, and limitations. State evidence and impact; distinguish confirmed observations from hypotheses. Resolved findings need detail only when the discovery, fix, or remaining consequence matters for the handoff.
8. Follow-ups describe genuinely out-of-scope work with evidence or reproduction details, impact, the scope boundary, and a suggested next action. Link an existing change when known. Untracked entries remain for human triage. Never mint a change, issue, ADR, or learning automatically through this feature.
9. A defect in this branch's own promised behavior remains subject to the existing fix/block/halt rules. Writing it under Follow-ups does not authorize deferral or permit completion.
10. Avoid duplication between findings and follow-ups by referring to the relevant finding when it already carries the evidence. Final consolidation removes repeated or superseded wording while preserving unresolved information; Git retains checkpoint history.
11. Persist material implementation information that would otherwise appear only in the coordinator's final report: delivered outcome, spec departures, manual checks, verification gaps, findings, limitations, and follow-ups. Driver disposition and unrelated skipped-change bookkeeping retain their existing report channels.
12. Existing enabled dummy-mode presentation remains additive under its current surface rules. Any In plain terms block must contain substantive prose; technical content remains authoritative. This feature does not add a new presentation mode.

## Artifact location and ownership

Use the existing filename convention: <results_dir>/<YYYY-MM-DD>-<slug>-results.md. The UTC date is the first authoring date. Choose the path once per change; on resume reuse results: when present, or verify and recover the unique committed checkpoint for this change at the canonical results location. Never create a second file merely because the date changed.

The file belongs to the owned feature branch. The results: field and derived change/board links belong to the metadata branch and are written only by the catalog-resolved change.attach-results transaction. Keep the existing generated reciprocal backlink.

The coordinator authors and consolidates results. Workers and reviewers return findings through their existing contracts. Do not introduce a new agent, require a second review round, or permit concurrent writers to the results file. A controller executing an invoked build role may perform the checkpoint on the same coordinator's behalf under the explicit caller contract; task workers must not independently edit it.

Stage only the results path for a standalone checkpoint. Never adopt a child's uncommitted changes or commit unrelated staged content. Avoid index interference by requiring a safe, quiescent workspace and the existing ownership checks before authoring or committing.

## Checkpoint lifecycle

A checkpoint is an evaluated phase boundary, not an obligation to emit an empty file or an unchanged commit. Capture new material information when it exists; an unchanged substantive record is a no-op.

### Build findings

Capture coordinator-known build findings after build/task returns at a safe boundary. In the default docket-build binding, consolidate available build findings before the existing end-of-build gate where possible, so the gate tests the checkpoint-containing head. The outer implement-next coordinator reevaluates on return and saves any additional material information.

Do not add a commit after every observation or require capture after every task when no new useful information exists. Do not interrupt a live worker to change its checkout.

### Review and fixes

Capture the returned review findings before a long fix loop when the workspace is safe, then update their actual dispositions after fixes return. Retain unresolved findings and relevant fix consequences. This supports recovery if the run stops during remediation, while final consolidation gives the human the current state.

Checkpoint writes do not independently launch tests. Existing review-entry and fix-loop evidence obligations still apply; see the sequencing contract below.

### Planned pause, halt, or handoff

Before relinquishing control, persist any additional coordinator-known information if the existing ownership and gate-drive contracts permit safe workspace writes. A checkpoint does not authorize stopping at an intermediate successful step or change the existing terminal dispositions.

Never commit or change HEAD beneath a live/nonterminal gate, running worker, or transferred drive. If a pause/halt occurs before workspace preparation or a safe write is impossible, use the existing halt/continuation reporting channel to state the capture limitation and preserve available context. Do not fabricate an artifact path, steal a lease, or delay required gate handoff to satisfy a file write.

### Final consolidation

Before the implemented transition, read the current artifact and all available build/review/fix outcomes. Reconcile the content with the delivered change, remove empty sections and duplicates, and verify that no material coordinator handoff remains only in chat.

A final results artifact is mandatory even if every checkpoint so far was a no-op. A trivial change still supplies its actual Outcome. Finish and commit authored results before the last gate run needed to certify the published head.

### Persistence and recovery

At each safe checkpoint containing changed content:

1. Verify the owned change, feature workspace, and that no child or gate still owns execution against its head.
2. Read the existing results file and preserve prior material content.
3. Write the substantive update, stamp or validate its backlink, and commit only the results artifact.
4. Publish the checkpoint through the existing authorized feature-branch transport without a force overwrite. This is branch publication, not a PR, merge, status transition, or gate pass.
5. On first creation, attach the verified committed file through change.attach-results using the fresh entity version. Later updates retain the same path; revalidate or reattach as required by the existing attachment identity contract and always before completion.
6. Treat local commit success, remote publication, and metadata attachment as distinct facts. A denied/failed push must not be described as a remotely durable checkpoint. Preserve the local commit and original diagnostic and follow the existing halt/recovery posture; do not bypass a permission denial.

On resume, load the committed results before new work. Recover a commit-before-attach interruption only when the owned branch contains exactly one safe, correctly backlinked candidate for this change; otherwise surface the ambiguity. Reuse existing published/attached checkpoints, avoid duplicate paragraphs and no-change commits, and never overwrite newer remote work.

An abrupt crash can lose observations since the last checkpoint. No background autosave daemon, cross-agent journal, or guarantee of recovering unreturned child output is introduced.

## Test-evidence sequencing

Results prose is never gate evidence. Preserve the exact-head, observed-gate and resolved-command contracts of the Go runtime and ADR-0102. Never relabel an older passed run with a newer head, fabricate green evidence, assume docs are invisible to tests, or activate a results-only ancestor skip.

- Prefer build checkpoint commits before the existing build gate. This avoids introducing a gate run merely to save already-known build findings.
- If a custom role returns certified evidence and saving new results advances HEAD, validate that evidence as historical evidence and satisfy the existing current-head requirement before entering any downstream stage that requires it. Do not review an uncertified branch by pretending its evidence is current.
- Batch review/fix checkpoint edits before the final certification run. A checkpoint itself does not call the suite; certification occurs at the existing evidence boundaries that need a current head.
- After final authored results are committed, obtain valid gate evidence for that exact head through the existing gate driver. Reuse existing evidence only when all current predicates hold. A stale record requires a real qualifying run; build.gate: off produces truthful skipped evidence.
- Avoid a self-referential results/test loop: Verification performed may cite actual earlier checks with their tested revision, and direct the reader through the change/PR link to the authoritative final evidence. Do not rewrite results solely to paste the final successful gate's timestamp, SHA, or pass message after that run.
- If the final gate reveals a new material problem or finding, preserve it, apply the existing fix/halt policy, and re-establish evidence after any commit. Never suppress a finding to keep the head unchanged.
- PR publication, mark-implemented, run.verify, and finalize continue checking the current head under their existing ownership and evidence rules. No extra reviewer or per-checkpoint full-suite requirement is introduced.

The common overhead is a few writes/commits and publication/attachment operations. A post-gate results edit can require another certification run; the implementation must document and test that fact rather than promise that checkpoint capture is always test-free.

## Required completion checks

Strengthen Go verification as well as skill prose. Reuse the existing Markdown/document machinery for a focused shared results validator; do not build a second general Markdown framework.

At minimum the final artifact validator proves:

- a nonempty, safe attached results path in the configured results root;
- a tracked regular file at the current feature head, with the correct balanced managed backlink;
- a nonempty title and substantive Outcome body;
- optional sections/subsections contain substantive authored content when present;
- no unfilled authoring placeholders or whole-section empty-value filler in the authored template structure.

Ignore headings inside fenced examples and generated managed blocks when recognizing the document structure. Do not reject legitimate quoted discussion merely because it mentions the words None or Not applicable. Structural checks do not certify correctness of manual instructions or completeness of findings; those remain explicit coordinator/reviewer obligations.

ChangeMarkImplemented must refuse a fresh transition when final results are missing, invalid, or not attached, including for trivial changes. Preserve existing exact-version, remote-head, PR and evidence checks.

RunVerify must report missing/invalid results as explicit unmet completion conjuncts for an active run; it must not return run-complete based only on the PR and green evidence. Preserve existing halted/waiting precedence and attribution contracts. Return stable, specific reasons for missing linkage, broken identity, or invalid content.

ChangeAttachResults must retain its ownership, path, backlink, commit and exact-version guarantees. At checkpoints allow a truthful in-progress artifact; enforce the complete final content contract at the implemented boundary. Where helper code is shared, pass the validation phase explicitly so checkpoint attachment cannot accidentally claim final completion.

No separate lifecycle state, user configuration opt-out, or mandatory follow-up item is added.

## Compatibility and post-merge behavior

This is prospective workflow enforcement. No migration, backfill, or rewrite of old results/specs/archived changes occurs. Existing records with an empty results: field remain parseable; repository-wide status/health reads must not require invented historical files.

Fresh mark-implemented transitions follow the new requirement. Active run.verify checks also apply the current requirement: a legacy active implemented change missing results cannot be reported as a newly verified complete run. This does not introduce a results gate into finalize or retroactively block the existing closeout of a previously implemented PR. Existing replay of an already-landed implemented transaction remains idempotent and does not rewrite its historical receipt.

Before merge, any authorized results changes must preserve the same branch, identity and evidence rules. After merge, authored results stay frozen. Already-known human verification outcomes and late findings supplied to finalize continue going into the terminal change's Closeout notes via the existing typed operation. Do not add a post-merge pause or edit frozen results.

## Harness and role coverage

The normative workflow is harness-neutral. Put authoring, checkpoints, recovery, and completion rules in shared skills/Go policy; adapter files express only their existing launch/install mechanics.

Cover Claude, Codex, Cursor, and OpenCode through the maintained harness registry and asset inventory. Exercise the shipped template and the implement-next/build/review instructions actually delivered by each adapter, including their embedded bundle copies. Do not mistake a hand-edited source skill for proof that installed harness assets changed.

Preserve pluggable skills.build and skills.review bindings, explicitly configured auto, current unavailable-capability fallback rules, all build profiles, and all review rungs. Rebound roles return their existing findings; the coordinator applies the common results contract and captures at the next safe boundary it controls. No hidden dependency on a tool namespace, a product-specific hook, a native message history, or one agent's private memory is allowed.

## Implementation surfaces

- Shared results-template.md and implement-next workflow: checkpoints, final content, required Step 6.5/Step 7 postconditions, resume/edge paths, and final-report persistence.
- docket-build and its worker-return/controller boundaries: explicit capture ownership and gate ordering; no concurrent results writers.
- docket-review and the implement-next fix-loop reference: returned evidence, meaningful findings/dispositions, and actionable out-of-scope work.
- docket-convention and maintained user guidance: required results, conditional sections, discovered-work persistence, branch split, and frozen merged artifacts.
- Existing Go attachment, implemented-transition, and run verification seams, with focused shared content validation and typed reasons. Update the capability/schema surfaces only if public payloads actually change.
- Existing assets regeneration and harness fixtures: generate embedded copies through the tracked mechanism; do not hand-edit generated copies.
- Relevant completion/evidence, document validation, workflow guards, and cross-harness tests.

Reconcile these symbols and paths against current main before implementation. Search maintained source for optional-results/skip-results instructions and update all active callers that contradict this contract. Historical point-in-time records remain untouched. Do not absorb unrelated documentation drift or change 0360's other work.

## Acceptance criteria

1. A trivial change and a nontrivial change each produce, commit, publish and attach a valid results artifact before a fresh implemented transition; missing results are refused and run verification cannot claim completion.
2. An Outcome-only artifact succeeds when no other substantive content exists. Empty optional headings/subheadings, placeholder instructions, and whole-section filler are refused at final validation; literal quoted examples and fenced headings are not false positives.
3. A functional scenario entirely covered by automated tests produces no Human testing entry. A partially covered scenario includes only the uncovered human assertion. A skipped automated suite appears as a verification gap, never a manual rerun instruction.
4. Human instructions give usable setup, actions and expected behavior for the uncovered scenario, and cleanup only where needed. No placeholder metadata appears in the rendered artifact.
5. Build, review-before-fixes, post-fix, planned-safe-stop, and final checkpoints preserve available material findings. Repeated checkpoints with unchanged content create no duplicate entries or empty commits.
6. A resume after commit-before-attach recovers the uniquely attributable artifact; resume after publication/attachment reuses it. A changed date preserves the original path. Ambiguous candidates, unsafe paths, symlinks, malformed/wrong backlinks, dirty ownership conflicts, or newer remote work cause a precise refusal without overwriting data.
7. No checkpoint changes HEAD while a worker or gate is active. A waiting/handoff or pre-workspace halt preserves its existing disposition and reports any capture limitation through the existing channel.
8. A failed checkpoint publication retains the local commit and reports that remote durability was not established. A lost successful publication response is resolved from Git state without a duplicate write.
9. Checkpoint edits introduce no automatic per-checkpoint test execution. Build capture can precede the existing gate; post-gate edits invalidate stale evidence; final publication uses genuine evidence for the exact committed head. An enabled build.gate: off remains skipped, never green.
10. Final consolidation plus a successful unchanged-head certification terminates without a self-referential artifact/evidence rewrite loop. New material findings from a final gate are persisted and cause appropriate revalidation.
11. In-scope defects remain fix/block/halt work. Out-of-scope follow-ups retain actionable context and existing links while producing zero automatic change/issue/ADR/learning creation.
12. The final report contains no otherwise-lost material implementation finding. It links the durable results, while the PR retains authoritative machine gate evidence.
13. Shared policy and generated assets cover every registered supported harness, including Claude, Codex, Cursor and OpenCode, and preserve default/custom/auto role bindings.
14. Historical optional artifacts remain readable, existing terminal records stay unchanged, fresh transitions enforce results, and active run verification reports any missing current requirement without adding a retroactive finalize gate. Merged results stay frozen; closeout notes keep their existing typed lifecycle.
15. Use meaningful Go integration tests with temporary repositories for attachment/transition/remote-state behavior, focused document fixtures for content rules, and mutation-proven workflow guards for obligations expressed in prose. Derive the harness test matrix from the registry. Run the repository's configured full suite at the implementation build gate; report unexercised live-harness behavior truthfully.

## Out of scope

Feature implementation during this grooming; automatic backlog creation or learning promotion; per-observation commits, transcripts, hidden journals, background autosave, new agents/review rounds, new lifecycle states or config opt-outs; retrospective artifact repair; post-merge authored results changes; a new results-only/ancestor evidence permit; changes to finalize gate policy, merge consent, branch ownership, or native dispatch mechanics.
