<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0423 — Certify native Multi-Agent V2 orchestration through Docket ImplementNext](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0423-certify-native-multi-agent-v2-orchestration-through-docket-i.md)**
<!-- docket:backlink:end -->

# Certify native Multi-Agent V2 orchestration through Docket ImplementNext

## Decision and purpose

Certify the real registered Docket ImplementNext workflow through native Codex nested dispatch in an isolated disposable repository. The positive execution must include the actual docket-plan-writer and one actual Docket build-profile worker as native descendants of the ImplementNext coordinator. No invocation of the catalog operation agent.enter, regardless of executable spelling, is permitted anywhere in either experimental run or its setup/launch helpers. Separate root sessions, direct app-server role entry, generic substitutes, parent relays, and subprocess agent runners cannot satisfy an edge.

This is a feasibility and certification change, not a production routing change. It tests the narrower hypothesis behind changes 0424–0426: a coordinator's observed multi-agent capability may explain the earlier nesting failure. Neither the hypothesis nor the V1 negative result is assumed true in advance. Certification is scoped to the observed model IDs, host mode, version, configuration, and launch path.

Human steering during grooming explicitly requires the actual plan writer to be a native nested child and excludes every use of docket agent enter from the POC.

## Existing context

The Codex adapter's codexTargetRouting currently chooses root entry for root-coordinator markers and agent.enter with an explicit worktree for feature-scoped roles. featureWorktreeStartupGuard requires the child's actual canonical process cwd to equal the supplied feature root before repository work. ADR-0114 protects that worktree boundary following a real cross-worktree failure.

Earlier records in docs/reference/harness/fixtures/nested-launch and docs/codex/fixtures/root-entry establish useful historical probes, but do not establish this acceptance case. A sentinel from an unrelated role, a status workflow, or a plan writer launched in a separate root session is insufficient. Historical evidence stays immutable; the new certification states exactly which previous claims it supplements or contradicts.

Related changes 0323 and 0412 concern failures that must not be misclassified as a lack of nesting. Changes 0384 and 0393 are completed historical inputs. Change 0423 has no prerequisite dependency. Changes 0424–0426 remain separately owned follow-up work; this POC does not change their records or make their proposed policy an established fact.

## Deliverables and isolation

Deliver a repeatable fixture and operator runbook under docs/codex/fixtures/native-implement-next/, deterministic evidence-validation coverage, and a durable certification report linked from this change's results. Include exact setup and rerun instructions, evidence format, verdict rules, and cleanup instructions. Any executable helper has an explicit contract and is exercised by the repository's Go test runner.

Use two independent disposable repositories initialized from the same fixture baseline, each with its own local bare origin, Docket metadata tree, and exactly one nontrivial build-ready candidate. The candidate is a small deterministic file transformation or equivalent tiny feature with one real plan task and a meaningful automated assertion. It must require a plan and a build worker but fit the bounded checkpoint. Use the normal Docket metadata operations and real Git commits. No production backlog, production remote, persistent user configuration, or production agent installation may be mutated by the experiment.

Record the installed Docket full commit/build identity and hashes of the loaded role definitions and skills. Use fresh host sessions after preparing isolated registrations. Preserve the real installed role names, bodies, skill preload, receipts, and worktree startup guard. Permit a documented, fixture-local routing override solely to select native dispatch for the coordinator and feature children, plus an explicit request limiting execution to the checkpoint. Record exact baseline/override diffs and prove the loaded definitions match those files. Do not claim byte-identical unmodified installation when routing instructions differ.

Model assignments are isolated fixture configuration. Discover exact V2 and V1 candidates from the live Codex model catalog and record the source field and raw observation. Do not select by family nickname or infer capability from reasoning effort. Keep host version/mode, feature flags, permissions, task, loaded role bodies, child pins, and coordinator reasoning effort equal across arms wherever supported. The coordinator model assignment is the intended independent variable; record unavoidable differences and qualify causal conclusions.

## Native topology and worktree feasibility

The positive topology is a fresh host parent, a natively spawned registered docket-implement-next coordinator, then the actual registered docket-plan-writer and one actual registered build-profile worker dispatched by the workflow. Invoke the resolved build skill normally; docket-build is a skill, not an agent name. Preserve its profile-selection contract for the one task.

Every required agent edge must have a host-recorded native spawn, actual registered role identity, parent identity, terminal child receipt, and evidence that its owning coordinator consumed the receipt. A helper metadata child cannot replace the plan-writer edge. No external actor may write the plan or perform the worker's implementation to manufacture success.

Worktree feasibility is a first-class result. Before the feature children perform repository work, establish through the supported native host interface either an explicit target cwd or a verified inherited cwd that is exactly the workflow's registered feature-worktree root. The runbook must identify the concrete supported mechanism from current host evidence before using it; this spec assumes no nonexistent spawn argument. Merely setting a shell command's workdir, changing directory inside one command, or putting a path in a prompt does not establish the child's startup cwd.

Keep the Feature worktree input and canonical startup equality check. Machine evidence must bind the intended root, Git worktree registration, child host cwd, and initial child process cwd. Do not relabel the primary checkout as a feature worktree. If supported native dispatch cannot meet the existing boundary, halt before feature work and report native-worktree-unavailable. Do not remove the guard, let the parent relay the work, promote the child to a root session, or invoke agent.enter. A guard failure is a substantive POC finding, not a reason to weaken acceptance.

## Positive execution and bounded checkpoint

Run the actual ImplementNext claim/reconcile/workspace/plan/build path against the disposable candidate. Bracket the coordinator with the existing run-gate facade and preserve its dispatch-context, change identity, attribution, and ownership rules. The fixture request explicitly authorizes native routing and a deliberate bounded halt after the single build task completes, before review or PR publication. Do not modify the production workflow to add a general checkpoint facility.

The plan writer must create a real plan commit in the correct feature worktree, return its normal PLAN_PATH receipt, and provide the required Docket-Plan-Path trailer and generated backlink. ImplementNext must verify and attach that plan through the normal operation. The worker must consume that plan, complete its actual one-task change and applicable validation, and return a real terminal receipt. Record the attachment and subsequent worker dispatch to establish consumption rather than relying on a coordinator sentence.

Complete applicable fixture build-gate obligations through the existing Docket gate driver. All live workers and test drives must be terminal or resolved through their existing ownership contract before the intentional halt is recorded. Retain and drain any yielded harness task/session identity; empty interim output is not a terminal result and never authorizes a second launch.

Capture results at the safe checkpoint and use the existing halt contract to leave the disposable candidate visibly in progress with the deliberate stop reason. The parent collects the terminal coordinator result and its keyed gate verdict. A run-halted verdict is the expected workflow outcome of this experiment, not ordinary ImplementNext completion, and never authorizes a retry. The certification validator may report its separate checkpoint result only from the required evidence. No review, PR creation, merge, or production routing mutation occurs.

## V1 control and verdicts

Run the same registered coordinator in a fresh matched fixture with its observed V1 assignment. Attempt the same workflow without substitutes or fallback. To establish the expected negative control, retain authoritative evidence of the coordinator's own effective top-level tool surface or an explicit host policy rejection after capability resolution. A nested tool inventory, coordinator claim, timeout, missing transcript item, or unavailable credentials cannot prove that collaboration is absent.

Record model mismatch, missing authoritative tool-surface evidence, authentication/rate failures, truncated lineage, unsupported cwd, and incomplete terminal collection as distinct limitations. If the V1 coordinator can dispatch, report that observation and reject the proposed V1/V2 discrimination for this configuration; do not force the expected outcome. If no suitable V1 model remains available, the negative control is not run, and paired certification is incomplete.

Use report-level verdicts, not new Docket lifecycle states:
- Certified: every positive predicate holds, the matched V1 capability-negative control is established, and no forbidden entry occurred.
- Hypothesis contradicted: authoritative completed observations disagree with the claimed capability distinction.
- Incomplete: an environmental, worktree, instrumentation, or lifecycle limitation prevents adjudication.

An incomplete or contradicted POC is valuable evidence but cannot be presented as the successful certification required by the dependent routing changes. Deterministic fixture tests alone do not certify a live host.

## Evidence and verification

Capture a manifest with run IDs; timestamps; host application/mode and CLI version; Docket identity; exact model IDs and catalog multi-agent fields; resolved effort/feature flags; role/skill hashes and override diffs; native session lineage and depths; tool calls with receipts; intended and actual worktree identities; Git commits, refs, trailers, plan path and attachment; worker delta and validation; drive terminal records; safe checkpoint and keyed gate verdict. Retain sufficient raw host events to re-derive each predicate. Hash referenced evidence and use fixture-relative paths so another reviewer can replay validation.

Bind the evidence to each fresh run and its actual commits. Coordinator prose, exit zero, arbitrary sentinels, and a valid-looking plan without matching child lineage are insufficient. A lack of agent.enter text in a partial transcript is not proof of absence: collect complete relevant launch/tool records and verify no root-entry route, app-server role launch, or substituted agent process appears. Unobserved execution gaps make certification incomplete.

Publish a concise sanitized evidence bundle preserving semantic fields and checksums for the published bytes. Exclude credentials and unrelated session contents. Retain raw evidence privately only as needed for audit, with a manifest describing redactions; missing required evidence must remain visible.

Deterministic tests cover valid and invalid bundles, unrelated or fabricated child receipts, missing coordinator consumption, plan/commit/attachment mismatch, incorrect cwd, role substitution, incomplete logs, unresolved live sessions, forbidden entry, an unexpectedly capable V1 control, and incomplete host evidence. Derive any structural route guard from executable sites and typed route identities rather than a hand-written list of strings. Mutation-test guards and the evidence validator: remove or alter each load-bearing fact and demonstrate rejection, then restore and demonstrate acceptance.

At implementation's build gate run the full configured build.test_command from the source checkout through the Go suite runner; inspect budget findings as well as test status. Live acceptance is separately required on the target Codex host. The results clearly separate deterministic checks, actual live observations, and any unrun acceptance case.

## Alternatives and boundaries

Using an unrelated native metadata child while keeping feature roles on root entry was rejected during grooming because it would not prove the real plan-writer edge. Dropping the worktree guard was rejected because it would recreate the failure ADR-0114 addresses. Changing production routing first was rejected because this change exists to obtain evidence before that decision.

Production model policy and its versioned registry belong to 0424. Production coordinator routing belongs to 0425, and the future scope of agent.enter belongs to 0426. This POC preserves existing production routing, shipped pins, feature guards, run-gate semantics, and historical ADRs. It does not implement the autonomous supervisor proposed in 0412 or automatically resume production changes.

The change is complete only when the repeatable fixture, validated evidence/report, and successful live paired certification are delivered. If the required host cannot certify it, retain a precise incomplete or contradicted result and surface that blocker for human disposition; do not mark the certification delivered merely because its documentation exists.
