---
id: 408
slug: 'finalize-publish-is-denied-by-the-auto-mode-classifier-whene'
title: 'Finalize publish is denied by the auto-mode classifier whenever the gate rebases'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-09-06'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [100, 260, 316, 360, 396, 403, 404]
discovered_from: [404]
adrs: [43, 105]
spec: 'docs/superpowers/specs/2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/finalize-publish-is-denied-by-the-auto-mode-classifier-whene'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-07T01:47:59Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md) |
| ADRs | [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md), [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md) |
<!-- docket:artifacts:end -->

## Why

Change 0404's finalize on 2026-09-06 passed its rebase/test gate, then Claude Code 2.1.260 denied publication through the Go binary in the finalize child, its prescribed retry, and the parent auto-mode session. The run needed a human command to proceed. This interrupts unattended close-out and can waste a completed green gate.

The original report overstates the cause and frequency. Archived change 0100 already recorded a plain Git force-with-lease denial on 2026-07-19. Conversely, change 0403 successfully rebased and published through the Go binary on 2026-09-04 under Claude Code 2.1.259; its old head is not an ancestor of the published head, so it required a real rewrite. Go opacity, a Claude version change, effective policy, and session context remain competing explanations.

The human specifically identified the 2.1.260 changelog as a lead. The linked spec records its permission-related changes and their limits. The original title is the initial failure hypothesis, not an established universal behavior. A controlled comparison must precede selection of a permission rule or runtime refactor.

## What changes

Make the first gate a bounded controlled comparison of equivalent direct-Git and Go publication across Claude Code 2.1.259, 2.1.260, and the installed current version. Prove each trial requires an actual history rewrite, preserve the exact old-value lease, and record effective policy, model, session context, and observed external effects.

Deliver a reproducible evidence report and a concrete remedy recommendation, then return to the human before changing permissions or redesigning the publisher. No autoMode.allow prerequisite or split publisher is preselected. An inconclusive or unavailable trial is reported honestly and does not certify a fix.

The selected repair must preserve still-valid green gate evidence across a denied publish, provide a concrete durable resume path, and retain remote lease checks, current identity/base checks, PR evidence convergence, and repair sign-off. Detailed experimental controls, interpretation rules, and the explicit post-investigation decision gate live in the spec.

## Out of scope

Changing Claude Code, branch protection, merge methods, bot approvals, or unrelated gate scheduling. Adding broad permissions or changing user settings as part of the baseline. Treating a renamed command, alternate tool after denial, no-op push, or human shell execution as proof that auto-mode publication works. Building a new publisher or recovery subsystem before the controlled comparison supports and the human selects that design.

## Run halted

### 2026-09-07

2026-09-07 — Autonomous implement-next halted at the spec's investigation decision gate.

## Why this run halts (not a defect, not an invalidated design)

Change 0408 is an investigation-first change. Its spec (`docs/superpowers/specs/2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md`) makes the first implementation gate a *controlled comparison* of finalize publication and does not authorize an autonomous implementer to guess or build the remedy:

- "An autonomous implementer must stop at that decision gate with an evidence-backed report using its existing halt contract; the presence of this spec does not authorize it to guess the unselected solution."
- "The live classifier comparison is an attended acceptance activity, not a network-writing default test-suite case."
- "If historical auto mode or the required model is unavailable, record that limitation and report the affected cells as unavailable." / "If that target or authorization is unavailable, return the complete experiment preparation and the exact missing prerequisite without performing external writes."

The design is sound and current (see reconcile assessment below); it simply cannot be executed by an unattended, single-fixed-Claude-version agent. Per the spec's own instruction, the honest outcome is this halt with the experiment preparation and the exact missing prerequisites, and no external writes.

## Reconcile assessment (current reality, 2026-09-07)

- The spec's factual scaffolding still holds on inspection: ADR-0043 and ADR-0105 are the cited decisions; related changes 0100/0260/0316/0360/0396/0403/0404 are all real and none is an unmet dependency (`depends_on: []`). The current binary is `v0.9.3-780-geffc9a6d`; the finalize/build gate is `local` with `go run ./cmd/docket development test`.
- No scope adjustment is warranted. The design's investigation-first posture and its explicit prohibition on preselecting an `autoMode.allow` rule, a split publisher, or a new recovery subsystem remain correct.
- No code was changed, no permission was edited, no Git rewrite or external write was performed by this run.

## Exact missing prerequisites (what a human/attended run must supply)

1. **Isolated historical Claude Code binaries** — 2.1.259 and 2.1.260 resolved through a supported distribution, isolated from the installed executable, with the version verified on every launch. The installed current version must be recorded exactly (the spec noted 2.1.263 at grooming time). This autonomous run is a single fixed Claude Code version and cannot install or launch alternate historical versions in isolation.
2. **An approved live GitHub test target** — an explicitly approved repository/remote containing only disposable test data, authorized for actual non-fast-forward force-with-lease pushes. No such target or authorization is provided to this run, and the spec forbids using the user's production remote, global permissions, shared policy, branch protection, or default-branch history to manufacture a result.
3. **Attended auto-mode launch capability** — the ability to launch a finalize child inside an auto-mode parent, and separately an attended auto-mode parent, with host permission rules governing denied cells. The spec classifies the live comparison as an *attended acceptance activity*, which an unattended drain cannot perform.
4. **Effective-policy/model observability** — the ability to record effective permission and auto-mode configuration, managed-policy load state, sandbox settings, and the supported model per launch, including for the historical versions (which cannot be inferred from the current machine).

Until (1)–(3) exist, every primary matrix cell is `unavailable`; no causal claim, blanket allow rule, or runtime redesign may be certified.

## Experiment preparation (ready for the attended run to execute)

Hold Docket constant: use one Docket source revision and binary digest throughout (record both). Vary only the Claude version and the publication form.

Primary matrix — 2 matched pairs per version in the finalize-child context = 12 primary attempts; alternate arm order between pairs; if a differential appears, repeat that version's pair twice in the attended-parent context:

| Axis | Baseline to hold |
| --- | --- |
| Claude version | 2.1.259; 2.1.260; current installed (record exactly) |
| Publication form | current Go finalize publisher; visible direct Git with the *equivalent exact ref-and-old-OID lease* (a bare force or implicit tracking-ref lease is NOT equivalent) |
| Launch context | finalize child in an auto-mode parent; reproduce any differential in an attended auto-mode parent separately |
| Git state | equivalent *actual non-fast-forward* updates of disposable feature refs on the approved remote (verify the remote old commit is NOT an ancestor of the intended new commit before each attempt) |
| Policy | same declared effective settings, managed policy, and supported model; no custom rewrite exception in the baseline |

Procedure per arm: start from equivalent fresh refs, PR state, owned workspace/receipt state, and green evidence; define the whole pair (both effects) before executing either arm; verify the selected binary version on launch; record whether the host allowed execution, the original denial text, whether the process started, and *separately* the ref-push and PR-body-update outcomes; independently verify the resulting remote ref after each allowed attempt. A no-op, fast-forward, dry-run, help invocation, early evidence rejection, or unrelated failure does NOT count as an allowed rewrite. Track the PR-evidence update separately from the ref update: a Go-only denial does not prove binary opacity unless compound-effect and PR-update differences are accounted for.

Use local bare-remote fixtures FIRST to verify setup and lease behavior (establishes Git correctness only, never the classifier's GitHub behavior). Any supplementary phase-isolation probe must be declared and bounded before it runs.

Measurements to capture (no credentials, no unrelated transcript content): UTC timestamp; Claude version + binary identity; Docket revision/digest; model; launch path; permission mode; parent/child context; effective permission/auto-mode config; managed-policy load state; sandbox settings; working dir; prompt/consent wording (redacted snapshots + stable digests); exact tool command; host allow/deny + original denial text; process-start yes/no; separate ref-push and PR-update outcomes; old/new refs and OIDs; current base OID; lease expectation; evidence head/command; verified final remote state; session/tool-event identifiers. Do not infer historical config from the current machine; do not label a child's mode as directly observed when only the parent records it; do not read "error" as "permission denied" or "exit zero" as "a rewrite occurred".

Interpretation gate (permitted conclusions only) — per the spec's result-pattern table:
- both forms succeed on 2.1.259, fail on 2.1.260 under matched conditions -> version-associated change; investigate policy-loading/classifier and test current before proposing a Docket refactor.
- Go repeatedly fails while equivalent visible Git succeeds within one version/context -> presentation/composition difference; evaluate a transparent prepare/execute/verify interface preserving Docket's authority checks.
- both forms fail across versions -> general authorization/context limitation; present supported scoped permission config and attended recovery as options.
- current succeeds while historical failing version still fails -> version-scoped compatibility recommendation, recovery requirement retained.
- mixed/unavailable/confounded -> report uncertainty and the smallest remaining discriminator; no causal claim.

## Deliverable of the first gate (to be produced by the attended run, then returned to the human)

An evidence report + exact reproducible setup/run instructions + a populated matrix with explicit `unavailable` cells + a proposed remedy tied to the observations. It must NOT claim the defect fixed. Present the measured conclusion and a concrete repair design to the human for selection BEFORE any code/permission change. The earlier `autoMode.allow` suggestion is NOT a preselected prerequisite.

Recovery requirements the eventually-selected repair must honor (for the human's later design step, not built now): retain PublishRewrite's exact lease + response-loss behavior (already-at-head = no-op; changed remote = contention; unobservable remote = no permission to force/repeat); FinalizePublish preserves authored PR-body bytes outside the managed evidence block; retain verified green-gate evidence across a denied publish with a durable concrete resume path, reusing evidence only while repo/change/PR identity, local head, effective base, gate policy, and resolved test command stay valid (moved head/base or changed command invalidates reuse); recheck current authority and the receipt's lease; distinguish a host denial (binary never ran) from a Go operation result, using only the permitted exact retry and reporting the specific denied action; persistent failure follows the existing Finalize blocked mechanism with a valid resume instruction, and if recording the block is also denied, report that separately rather than pretend the marker exists.

## Resume instruction for the human

This change stays `in-progress` on branch `fix/finalize-publish-is-denied-by-the-auto-mode-classifier-whene` with `claimed_at` refreshed. When the three missing prerequisites above are available, resume by re-dispatching `docket-implement-next` with the explicit id `408` through the resume path (`change.resume-halted --id 408 --version <current> --acknowledge-quiescent`), then execute the prepared experiment in an attended auto-mode context and produce the first-gate evidence report. Do not have the autonomous drain build a publisher/permission remedy — that decision is reserved for the human after the evidence is presented.
