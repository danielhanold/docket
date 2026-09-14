<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0425 — Restore native Codex dispatch for Multi-Agent V2 Docket coordinators](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0425-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor.md)**
<!-- docket:backlink:end -->

# Change 0425: native Codex coordination with explicit feature-worktree binding

## Goal

Make Codex native named-agent dispatch the production route for Docket coordinators and their planner, build, and review children when the selected model satisfies change 0424's Multi-Agent capability policy.

A Codex feature child may start with the coordinator's primary-checkout cwd because native child dispatch has no startup-directory control. Before task repository work, the child validates the assigned registered feature worktree and then explicitly targets task repository operations to it. This design adds no substitute runner, app-server root, host relocation mechanism, relay, or custom agent wrapper.

## Evidence and scope correction

Change 0423 corrected two premises in the proposal. First, a native Codex child on a Multi-Agent V2-capable model can dispatch registered children. Second, safety does not require the feature child to start in the feature worktree: the successful option is to validate an assigned registered feature root first and bind subsequent feature work to it.

The focused POC worker regression established a Terra/low controller dispatching one registered `docket-build-standard` child resolved as Terra/medium; a real baseline/RED/GREEN sequence; a committed implementation; durable scope acknowledgement; a configured full-suite pass at the exact implementation commit; a clean feature tree; and an unchanged final primary snapshot. The POC did not establish the complete ImplementNext-to-planner-to-worker chain, hard filesystem isolation, parallel safety, or production completion. Those claims require the continuous acceptance below.

The demonstrated assignments are Terra/low coordinator, Sol/medium planner, and Terra/medium standard worker. They are observed fixture assignments, not claims about current shipped defaults or family-level capability rules. Change 0424's exact-model registry and typed capability policy remain authoritative.

## Selected architecture

### Native coordinator entry

The caller arms the existing implement-next gate, retains its key privately, and launches the registered `docket-implement-next` role through ordinary Codex native named-agent dispatch. It passes the gate dispatch-context unchanged. If the existing gate workflow also supplies a run epoch, the caller passes that epoch through every operation whose live schema accepts it; change 0425 neither assumes every `gate-before` response contains an epoch nor manufactures one.

Missing registration, an invalid coordinator model, failed capability validation, or failed/uncertain native dispatch halts visibly. There is no fallback to `agent.enter`, `codex exec`, another harness, a generic agent, inline reconstruction, relay, or a second launch. The coordinator likewise uses native registered dispatch for planner, build, and review children.

### Capability policy

The shared role inventory declares whether a role owns downstream dispatch. Codex generation and installation consume change 0424's exact-model registry:

- dispatch-owning roles require Multi-Agent V2 or newer;
- a model known to be V1 is rejected;
- unknown-model and machine-local assertion behavior remains owned by 0424;
- family names and effort labels do not establish capability;
- routing never silently changes a configured model or effort.

### Static assignment and private operational payload

Each feature child receives a static immutable assignment containing non-secret values only: change, phase, task and role identity; canonical primary and feature roots; branch and pre-dispatch feature HEAD; synchronized change/spec paths; resolved plan path when applicable; exact configured test argv; entry-checker argv; and a digest over the assignment. This file may be constructed and validated before a gate scope exists. It never contains gate context, scope capability, owner generation, handoff token, or any other credential.

Dynamic operational values are separate and private. The coordinator obtains the real scope and child capability, then constructs and validates the final native message containing the static assignment's identity plus the exact scope, child capability, relevant predecessor/owner values, and unchanged outer gate context. The final message is dispatched only after the real scope exists. The coordinator's parent capability never enters the child message, assignment, evidence, terminal record, or searchable fixture material.

Protected fields in the dispatched message must equal the validated final payload. A missing, altered, duplicated, or substituted field halts before task work.

### Feature-worktree binding

The coordinator resolves an absolute feature root from registered Docket workspace state. It must be canonical, existing, non-primary, in the same repository, and match the assigned change and branch.

The child's first task action is the supplied interpreter-bearing entry argv. The checker reloads and hashes the immutable assignment and validates the registered feature root. Startup-cwd equality is not required. After binding:

- feature shell commands execute with the canonical feature root as their working directory;
- feature file reads and writes resolve beneath that root;
- feature Git and Docket operations receive the explicit applicable root;
- skills that operate on feature artifacts receive the root and exact artifact paths;
- every test invocation reloads the immutable assignment and its exact test argv;
- the child does not infer the feature root from cwd, prose, role name, or a sibling fixture.

This rule does not prohibit role-authorized reads of synchronized metadata, the child's own live capability catalog, prescribed gate state, or other explicitly named control inputs. Such reads are identified separately from feature-artifact access. The worker does not run `repository.prepare`; repository preparation belongs to the coordinator.

### Planner handoff

After feature-worktree creation, the coordinator dispatches the registered plan writer with its static assignment, private dynamic payload, and a complete immutable snapshot of the resolved planning-skill contract. The snapshot includes the exact usable content or packaged payload, digest, logical skill identity, provenance/version, and directed inputs. The planner does not depend on a parent-only path, a stale global installation, or omitted conversational context.

The planner binds to the feature root, invokes the supplied skill contract, writes only its authorized plan artifact there, stamps the backlink, commits only that artifact, and returns the repo-relative path and commit. The coordinator verifies containment, tracking, single-artifact delta, trailer, backlink, descendant relationship, and clean tree before attachment. Missing or mismatched skill input, digest, artifact, commit, or backlink halts; the coordinator does not recreate or repair the planner's output.

### Build-worker handoff and drive ownership

For each plan task, the build controller prepares one normal scope, retains the parent capability, and dispatches one registered profile worker with the exact validated child payload. The worker binds first, then drives baseline, RED, GREEN, and focused verification in order using the same scope. It commits successful task work before terminal acknowledgement and acknowledges with the child capability and current owner generation.

A worker `WAITING` return is valid only when it carries the drive id and single-use handoff token produced by `gate.drive.handoff`. The nearest live controller claims that handoff with `gate.drive.claim`, captures the fresh owner generation, and advances the same drive through short driver calls. The worker does not keep polling after it has handed off. When agent judgment is needed again, the controller may dispatch a fresh worker for the same task only through the existing explicit continuation contract. This is not a replacement drive and consumes neither repair nor escalation allowance.

If a worker returns without a valid handoff while its scope still owns a nonterminal or unconsumed drive, the controller uses the existing parent-capability takeover contract based on the observed return event, captures the new owner generation, and settles that same drive. It never uses a timer as takeover authority and never launches a replacement test. Invalid ownership, predecessor, generation, handoff, or acknowledgement halts.

### Review

Production implementation of 0425 includes normal review of the shipped change through the registered review role, bound to the exact feature root and commit. The reviewer validates its assignment before read-only repository inspection, and the coordinator verifies that the result names the expected branch and commit.

The POC 0423 continuous certification stops with typed halt before review, PR, merge, or production mutation. Its purpose is to certify the native coordinator/planner/worker chain safely; it does not impersonate 0425's production review stage.

### Catalog and schema use

Each agent resolves the live Docket capability catalog for operations it owns. One agent's catalog is not executable authority for another agent and is not reused as a cross-agent cache. Static input may carry resolved stable data such as paths and test argv, while each child independently validates the operations it invokes.

## Plan artifact ownership

The plan file intentionally exists on the feature branch until merge. Its `plan:` metadata field exists on the metadata branch. Diagnostics must therefore become ownership-aware:

- for an in-progress change with a registered owning feature worktree, validate the attached plan at its recorded feature commit and path;
- report a finding if the workspace is missing or foreign, or the commit/path/blob disagrees;
- after integration, validate the plan on the integration branch under existing done-state rules;
- never copy the plan into the primary checkout to silence a finding;
- never suppress all missing-plan findings for in-progress changes.

Feature artifacts belong in the feature worktree. Metadata transactions in `.docket`, Git administration in `.git`, live gate state, and external evidence are separate bookkeeping surfaces whose expected access is declared and audited.

## Gate, results, and terminal sequencing

The caller-side implement-next gate remains authoritative. The outer dispatch context reaches claim and every nested gate operation whose schema accepts it. Any supplied run epoch follows the same rule. Continuations retain their existing key, change, continuation, phase, and epoch identities.

Build evidence for the implementation gate remains attached to the implementation HEAD it tested. Review fixes may move HEAD and require refreshed build evidence under the existing policy. The coordinator then authors, attaches, and commits the durable results artifact; because that commit moves HEAD, the earlier implementation gate is preserved as stage evidence rather than misrepresented as final-head evidence. After the final results content is committed, the coordinator obtains the normal required checkpoint/certification gate for that exact final HEAD. Any later material commit invalidates that final evidence and requires re-establishment.

Only after the exact final head is clean, published as required, reviewed, supported by attached results and exact-head evidence, and otherwise satisfies the normal ImplementNext postconditions may 0425 mark its production implementation implemented. The parent then obtains and obeys the keyed outer verdict. Child prose, a tool exit, or an earlier green gate cannot replace it.

The POC 0423 packages stage evidence and terminates through a typed halt before production closeout. It does not create review, PR, merge, or implemented state.

## Failure posture

The run halts without fallback for invalid model capability, missing registration, uncertain dispatch ownership, failed worktree binding, invalid immutable input or skill snapshot, payload mismatch, leaked parent capability, missing catalog operation, broken handoff/takeover/predecessor/owner identity, unverifiable planner or worker commit, dirty or wrong-head gate execution, unsafe results checkpoint, or unattributable outer verdict.

A halt preserves completed work and emits credential-free diagnostics. It does not weaken checks, repair child output in the parent, switch launch mechanisms, or infer authority from nearby files or recent runs.

## Progressive disclosure and affected surfaces

The neutral inventory retains typed worktree scope and downstream-dispatch capability. Codex mechanics appear only where needed:

- the Codex adapter and generated dispatch contract select native dispatch;
- coordinator instructions disclose gate context, optional epoch, scope creation, and private payload duties;
- feature-role instructions disclose binding and explicit feature targeting;
- planner instructions disclose the complete skill-snapshot contract;
- build instructions disclose static-versus-dynamic input, handoff/claim/takeover, commit, and acknowledgement;
- review instructions disclose exact-commit read-only binding.

Other harness behavior and shared role semantics remain unchanged. Generated assets, embedded copies, goldens, installer ownership checks, guards, and documentation are regenerated from their existing sources.

A successor ADR supersedes the conflicting Codex routing and startup requirements of ADR-0114 without editing that accepted historical record. It retains the safety invariant that the verified owning worktree is authoritative, while replacing actual-cwd equality and automatic feature `agent.enter` with native dispatch plus explicit binding.

Change 0426 owns broader legacy `agent.enter` deprecation and retirement. Change 0425 makes native option 2 standard for this workflow and removes its automatic route here; it does not implement 0426.

## Acceptance criteria

### Hermetic and mutation coverage

1. Derive coordinator roles from typed metadata and enforce 0424's exact-model capability policy. Mutations admitting V1, using family inference, or rewriting pins fail.
2. Prove Codex coordinator, planner, build, and review routes use registered native dispatch with no reachable automatic `agent.enter`, runner, relay, generic, inline, or cross-harness fallback.
3. Launch a child from primary cwd with a valid feature assignment and prove explicit feature targeting succeeds. Reject primary, foreign, nested, nonexistent, symlink-escaped, wrong-branch/change, unregistered, and replaced roots before task work.
4. Derive executable feature-operation sites from repository syntax, not an enumerated spelling list. Mutations dropping explicit binding from representative shell, file, Git, Docket, and skill operations fail.
5. Prove the static assignment is credential-free and independently hash-valid before scope creation. Prove dynamic scope, child capability, owner data, handoff data, and outer context are absent from it.
6. Prove the final native message is constructed after real scope creation, equals the validated protected payload, contains the child capability, and excludes the parent capability.
7. Prove exact interpreter entry. Direct invocation of a non-executable checker, wrong interpreter/path, or wrong digest fails before task work.
8. Remove or alter the planner skill snapshot and require pre-write halt. Verify normal planner output is one committed, backlink-valid feature artifact.
9. Prove every agent loads its own live catalog; parent-cached executable authority in a child fails.
10. Prove the worker cannot run repository preparation, create a second scope, skip predecessors, reuse process-memory test values, acknowledge before commit, or acknowledge stale ownership.
11. Exercise valid `WAITING` handoff, controller claim with fresh generation, advancement of the same drive, explicit same-task continuation when judgment returns, and return-without-handoff takeover. Mutations that poll after handoff or launch replacement tests fail.
12. Exercise baseline pass, genuine assertion RED, GREEN, commit, acknowledgement, and build-owned configured full suite at the exact clean implementation commit.
13. Exercise production review at the exact shipped commit and reject stale or dirty review input.
14. Exercise implementation-stage evidence, a results commit that moves HEAD, and a separate exact-final-head checkpoint. Mutations reusing stale pre-results evidence or claiming a blanket single suite run fail.
15. Exercise keyed outer verdict behavior for completion, continuation, retry, halt, cancellation, and optional epoch presence/absence without manufacturing an epoch.
16. Exercise ownership-aware feature-only plan diagnostics, including valid feature plan, missing workspace, mismatched commit/blob, and post-integration resolution.
17. Mutation-test integrity auditing with a transient restored write. A final snapshot alone cannot support a hard-isolation claim; results must name any host visibility limit.
18. Run the configured complete repository suite through the maintained Go runner and inspect budget reports.

### Continuous native Codex acceptance

In a fresh disposable repository and fresh Codex parent session, one continuous POC run must record exact Codex/Docket versions, source identity, role/model/effort resolution, initial checkout identities, and credential-free receipts, and demonstrate:

1. native Terra/low ImplementNext coordination;
2. unchanged outer gate context, plus run epoch only if supplied, reaching applicable claim/scope/drive operations;
3. registered feature creation and validation while coordinator starts in primary;
4. one native Sol/medium planner receiving the complete real skill snapshot and committing a verified plan;
5. one native Terra/medium standard worker receiving the exact post-scope child payload, binding from inherited primary cwd, running baseline/RED/GREEN, committing, and acknowledging;
6. correct WAITING handoff/claim behavior if WAITING occurs, with no replacement drive;
7. build-owned configured suite evidence at the exact implementation commit;
8. durable POC evidence packaging followed by typed halt before review, PR, merge, or production changes;
9. clean feature state, unchanged final primary snapshot, and separate audit of feature access, synchronized metadata reads, live catalog/gate reads, bookkeeping writes, broad reads, and transient writes;
10. no automatic `agent.enter`, runner, wrapper, host relocation, generic substitute, or duplicate launch.

The report distinguishes independently observed facts, child receipts, and opaque host behavior. It makes no hard-isolation or parallel-certification claim without direct evidence. The focused worker run may be cited but does not replace this continuous chain.

Production 0425 acceptance separately includes review, durable results, exact-final-head certification, publication/implemented transition under the normal workflow, and the keyed outer verdict.

## Assumptions

- Change 0424 lands first and supplies exact-model capability authority.
- Change 0423 preserves POC evidence and stops through typed halt rather than acting as production implementation.
- Native Codex dispatch preserves registered role identity, model, effort, and preload, but offers no supported nested-child startup-cwd selector.
- The coordinator can resolve a canonical registered feature root and issue scoped child capability without disclosing its parent capability.
- Run epoch is propagated only when the existing gate workflow actually supplies one.
- A complete planning-skill snapshot can be passed as bounded immutable content without a new harness.
- Ownership-aware diagnostics may inspect the registered feature worktree for an in-progress attached plan.
- Host visibility may limit proof of broad reads and transient writes; results state those limits.
- Parallel operation on distinct worktrees is a desired consequence, not certified by this single-run acceptance.
- Change 426 remains responsible for broader legacy retirement.

## Out of scope

Implementing 426; changing other harness launch behavior; live model probing per dispatch; family capability inference; automatic model substitution; adding native startup-directory support; app-server root promotion; host relocation; custom wrapper, runner, relay, generic agent, or cross-harness fallback; redesigning planning, review severity, merge approval, or retry policy; claiming hard isolation or parallel safety; embedding POC paths, task ids, capabilities, commits, or fixture names as production constants.
