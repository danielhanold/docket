<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0315 — Claim-to-implemented agent workflow](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-18-0315-claim-to-implemented-workflow.md)**
<!-- docket:backlink:end -->

# Claim-to-implemented agent workflow

**Change:** 0315 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-17 · **Status:**
Approved focused design

## Purpose and boundary

This change composes the already-landed Go foundations into the essential implementation half of
the agent-first lifecycle. A Claude-hosted run can obtain authoritative candidate context, claim
one build-ready change, reconcile it against current reality, attach a verified plan, build and
review in the owned feature workspace, record a passed local gate at the exact feature head,
publish a pull request, and move the change to `implemented` without directly editing metadata or
delegating model judgment to Go.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are fixed upstream constraints.
This design neither reopens their one-binary, agent-first, one-shot JSON, transaction, external
effect, configuration, packaging, or hard-cutover decisions nor repeats behavior delivered by
changes 0305 through 0314.

Change 0315 stops at an open, verified PR and an `implemented` metadata record. Change 0316 owns
rebase, retest, conflict repair, merge, archive, stack closeout, reclaim, cleanup, maintenance
sweeps, and persistent halted recovery. Changes 0317 and 0318 own release acceptance and the
self-hosting cutover respectively.

## Landed foundation and independently deliverable result

Changes 0312, 0313, and 0314 are complete on `main` and are consumed as dependencies rather than
redesigned here.

- `internal/domain` already owns readiness, claim eligibility, claim and lease refresh, effective
  stack-base resolution, and the `in-progress` to `implemented` transition.
- `internal/repository/transaction` already owns version-checked, loss-preserving metadata
  transactions, semantic retry, exact-path commits, request receipts, validation, derived views,
  and lease pushes.
- `internal/document` and the 0312 mutation layer already own change/spec/plan/results parsing,
  managed artifact and backlink rendering, inline-board rendering, and repository validation.
- `internal/workspace` already owns feature-workspace identity, preparation, inspection,
  non-forcing head publication, and cleanup mechanics.
- `internal/githubcli` already owns typed, probe/act/verify pull-request publication.
- `internal/evidence` already owns the strict build-evidence record and exact-head verification.
- `internal/process` plus the landed `gate launch`, `gate observe`, and `gate stop` operations
  already own durable local-gate execution and exact terminal outcomes.
- Change 0324 and ADR-0094 already establish the pinned `docket-plan-writer` role and its
  single-artifact return contract.

Change 0315's independently reviewable deliverable is:

- an authoritative, revision-pinned implementation context for one selected change;
- narrow application and CLI operations for claim, refresh, reconciliation, artifact attachment,
  feature-workspace preparation/publication, evidence recording, PR publication, transition to
  `implemented`, and read-only run verification;
- revised Go-v1 `docket-implement-next`, `docket-plan-writer`, and build-gate skill surfaces that
  sequence those operations while retaining model-authored reconciliation, planning, build,
  review, and PR prose;
- regenerated embedded assets corresponding to those revised source assets; and
- hermetic end-to-end tests proving a supported repository can progress from a build-ready change
  to a verified PR and `implemented` record without a direct metadata write or shell-owned Git,
  GitHub, gate, or backlink operation.

The slice is independently useful even though it is not yet Docket's self-hosting cutover: it is
exercised through supported configuration fixtures and direct Go-v1 asset invocation. A repository
that actively requests a deferred capability still receives `unsupported-config` before mutation.
Disabling Docket's own deferred settings and making these assets the only production path remain
0318 work.

## Chosen architecture

Claude remains the workflow controller; Go exposes narrow checkpoint operations:

```text
Claude-hosted docket-implement-next
  |
  +--> context implementation     authoritative candidate bundle
  +--> change claim               exact-version metadata transaction
  +--> change reconcile           authored patch + reconcile log transaction
  +--> workspace prepare          owned checkout at resolved effective base
  +--> pinned plan writer         authors and commits exactly one plan
  +--> change attach-plan         verifies commit and links plan transactionally
  +--> docket-build agents        TDD, commits, and bounded review/fix decisions
  +--> gate launch/observe        exact local-gate terminal result
  +--> evidence record/verify     passed terminal record at feature HEAD
  +--> workspace publish          non-forcing exact-head publication
  +--> pr publish                 probe/act/verify one PR
  +--> change mark-implemented    verified PR/head + atomic metadata transition
  `--> run verify --id            read-only postcondition report
```

Each command is a one-shot request/response. There is no workflow daemon, hidden session, mutable
server-side cursor, or Go-owned phase machine. Durable metadata, workspace manifests, Git refs,
gate records, evidence, and the PR are the resume state. The skill inspects those checkpoints and
decides which valid next operation to invoke.

Two alternatives are rejected:

1. **A monolithic `implementation advance` command.** It would make Go choose phases, dispatch
   agents, interpret authored work, and own recovery policy, crossing the approved agent-first
   boundary and obscuring the individual external-effect postconditions.
2. **Retain the Bash/direct-Git workflow and add only a Go status check.** That would leave durable
   writes, worktree and push decisions, gate interpretation, and PR publication outside the typed
   engine, so the end-to-end deliverable would not satisfy the migration boundary.

## Package and command boundaries

Application orchestration stays thin and depends inward on the landed services:

```text
internal/cli  -->  internal/app
                       |
                       +--> repository snapshot / transaction
                       +--> domain
                       +--> document renderers
                       +--> workspace --> gitcli
                       +--> process
                       +--> evidence
                       `--> githubcli
```

- `internal/cli` parses typed flags or protocol-v1 request files and presents exactly one result.
  It contains no lifecycle, Git, document, gate, evidence, or GitHub policy.
- `internal/app` assembles authoritative inputs, verifies cross-package preconditions, calls one
  coarse operation, and maps closed outcomes into the landed result taxonomy. It does not dispatch
  agents or retain workflow state.
- Metadata mutations execute through `repository/transaction.Engine`. They submit expected entity
  versions, patch only operation-owned fields and sections, render every affected v1-owned derived
  view, validate the whole repository, and push the metadata ref with a lease.
- Workspace, Git, `gh`, gate, evidence, and document mechanics stay in their landed packages.
  Change 0315 may add only the named adapter methods needed to expose those mechanics through the
  application layer; it does not fork their policy or introduce generic command runners.
- The skills invoke `docket` operations. They do not edit change frontmatter, managed blocks, or
  board files, call Git/`gh` for owned effects, interpret gate process state from PIDs or logs, or
  invoke legacy helper-facade mutations.

The public operation spellings settled by this slice are:

```text
docket context implementation [--id <change-id>]
docket change claim --id <change-id> --version <entity-version>
docket change refresh-claim --id <change-id> --version <entity-version>
docket change reconcile --id <change-id> --version <entity-version> --input <request-file>
docket artifact backlink --artifact <path> --change <path>
docket change attach-plan --id <change-id> --version <entity-version> --path <path> --commit <oid>
docket change attach-results --id <change-id> --version <entity-version> --path <path> --commit <oid>
docket workspace prepare --id <change-id> --version <entity-version>
docket workspace inspect --id <change-id>
docket workspace publish --id <change-id> --head <oid>
docket evidence record --id <change-id> --run <absolute-run-dir> --head <oid>
docket evidence verify --record <request-file> --head <oid>
docket pr publish --id <change-id> --head <oid> --body <request-file> --evidence <request-file>
docket change mark-implemented --id <change-id> --version <entity-version> --head <oid> --pr <reference>
docket run verify --id <change-id>
```

JSON mode accepts the same semantics through the existing protocol-v1 request mechanism. Authored
Markdown is supplied through a request file or stdin-backed request field, never a shell-escaped
flag. Exact flag grouping may follow established Cobra conventions, but implementation must
preserve these operation names and request meanings so shipped skills do not infer behavior from
human text.

## Authoritative implementation context

`context implementation` is read-only. Without `--id` it applies the landed deterministic change
selection policy; with `--id` it inspects that exact change, which supports an attributed retry.
It loads one fresh repository snapshot and returns either a typed no-candidate outcome or a single
internally consistent bundle containing at least:

- metadata ref name and exact metadata commit;
- selected change ID, canonical metadata path, loss-preserving source bytes, parsed semantics, and
  opaque entity version;
- linked spec path, source bytes, and entity version;
- dependency, stack, and related-change summaries needed to understand the selected change;
- readiness and claim-eligibility outcomes;
- the resolved effective-base outcome, base branch, and source change when applicable;
- relevant accepted ADR index entries and enabled learning index entries or explicit capability
  warnings; and
- supported workflow configuration, repository mode, integration branch, test command, remote,
  and conventional feature branch.

The context does not summarize or rewrite authored Markdown. The skill receives exact bytes so the
model can reconcile the proposal and spec without a second ad hoc file read. All facts in one
bundle come from the same snapshot; a caller never selects from one revision and claims against a
newly loaded but unreported revision.

The result includes typed absence, malformed, ambiguous, not-ready, unsupported-configuration, and
effective-base outcomes. It does not silently omit a missing artifact or choose among duplicate
IDs. Read-only diagnostics may return context plus deferred-capability warnings, but a subsequent
mutation performs its own preflight and refuses an actively requested unsupported behavior before
opening a transaction.

## Claim and lease contract

`change claim` consumes the context's exact entity version. Inside the transaction it reloads fresh
origin state, proves the same change is still proposed, uniquely identified, build-ready, and has a
resolved effective base, then applies the landed `domain.Claim` action. The transaction updates
the change, artifact block, and inline board together. A contention or policy refusal creates no
feature workspace and performs no external effect.

A successful result returns the committed metadata revision, new entity version, feature branch,
claim timestamp, lease classification, and the context facts needed for the next inspection. Lost
response retry converges by re-reading authoritative state: an already-held matching claim may
return the applied receipt or a typed already-claimed outcome, while another claim or incompatible
change edit returns `contended`. The operation never adopts a branch or workspace as proof that the
metadata claim succeeded.

`change refresh-claim` is a small independent metadata transaction. It requires `in-progress`, an
exact entity version, and the existing claim identity implied by the change record, and only
re-stamps `claimed_at` plus affected derived views. The implementation skill refreshes immediately
before and after long model/worker phases. A contention stops the run from beginning another
effect; it is not permission to overwrite a newer record.

Lease expiry and reclaim are different responsibilities. This change evaluates and refreshes the
current claim but never clears or steals an expired claim. Reclaim remains 0316.

## Reconciliation transaction

Claude authors reconciliation after inspecting the revision-pinned context and current feature
reality. `change reconcile` accepts structured desired edits plus authored Markdown, not an entire
replacement repository record. Its request may:

- patch the owned proposal sections and the still-mutable linked spec content;
- update complete desired relationship values when current reality requires it;
- append one dated reconcile-log entry supplied by the agent; and
- set `reconciled: true` and refresh the claim.

The operation reloads fresh state, checks the expected version and legal `in-progress` status,
validates that changes stay within the operation-owned fields/sections and within the selected
change's scope, rerenders backlinks/artifact blocks/inline board, validates the whole repository,
and commits all outputs atomically. Unknown frontmatter, authored sections outside the patch, line
endings, and unrelated files stay byte-identical.

The binary validates shape and lifecycle invariants; it does not decide whether an implementation
plan is technically wise or whether a newly discovered fact changes the proposal. That judgment
and prose remain Claude-owned. If fresh authoritative state differs incompatibly from the context
used to author the request, the operation returns `contended` and writes nothing; it never
text-merges two authored decisions.

## Plan and results attachment

The pinned `docket-plan-writer` from ADR-0094 continues to author the plan in the feature workspace
and commit it as exactly one allowed new artifact plus its required backlink change. The writer
uses `artifact backlink` for deterministic managed-block rendering and returns only its canonical
repository-relative plan path. It does not attach the plan or edit change metadata.

`change attach-plan` receives the canonical path and the exact feature commit reported by the
writer. Before opening a metadata transaction it verifies from Git, not from the child return
alone, that:

- the commit is the current feature head and descends from the prepared base;
- the plan path is canonical, repository-relative, inside the allowed planning directory, and is a
  regular tracked file;
- the writer commit has the ADR-0094 single-artifact delta and required commit trailer;
- the managed backlink is balanced and targets this change; and
- the plan contains no unresolved placeholder token required by the planning contract.

The transaction then rechecks the exact change version, stores the plan path, renders the change
artifact block and inline board, validates the repository, commits, and pushes. A retry verifies
the same plan identity and returns its prior applied outcome; it never attaches whatever happens
to occupy the path after the verified commit.

`change attach-results` applies the same canonical-path, containment, tracked-file, backlink,
exact-head, version, and transaction rules to an optional authored results record. Results are not
required to reach `implemented`; if the workflow authors them, they must be attached before the
transition. The deferred results-only gate-skip feature is not restored: a results document is
never evidence that the current head passed the gate.

## Workspace, build, gate, and evidence sequence

After reconciliation, `workspace prepare` reloads the authoritative snapshot, resolves the
effective base through the landed domain policy, constructs the landed workspace target, and
returns the canonical owned workspace path, exact base commit, feature ref, and workspace
disposition. It does not accept a model-selected base or infer stack policy from prose. Existing or
resumed workspaces must pass the 0313 ownership and live-Git checks.

The implementation skill then follows this sequence:

1. Refresh the claim and dispatch the pinned plan writer in the owned workspace.
2. Verify and attach the plan.
3. Refresh the claim and dispatch `docket-build`'s existing TDD build roles against the plan.
4. Launch the full resolved `finalize.test_command` through `gate launch`, observe it to an exact
   terminal state, and treat any over-budget report under the landed suite contract as a finding.
5. Only from `passed`, create a build-evidence record for the exact feature `HEAD`; then verify its
   canonical request bytes through `evidence verify`.
6. Run the existing bounded read-only review and agent-owned fix loop. Any fix changes `HEAD`, so it
   invalidates prior evidence and requires the full gate and evidence steps again.
7. Optionally author and attach a results record after the final evidence is valid.

`evidence record` accepts a gate run directory and expected head rather than an agent-supplied
`passed` boolean. It uses the landed process service to verify a complete `passed` terminal record,
the landed workspace service to verify the current feature head, and the landed evidence codec to
return an immutable typed record and canonical block containing the exact command, commit,
timestamp, and outcome. It does not persist a second evidence store. `evidence verify` parses those
request bytes and verifies their exact head before PR publication; the PR body becomes the durable
record after `pr publish`. Failed, running, stopped, vanished, malformed, or mismatched runs produce
no evidence.

Build and review quality decisions remain in the skills and their named agents. Go checks
mechanical invariants and exact state only. This change does not add autonomous repair policy,
checkpoint ledgers, cross-harness dispatch, CI gates, or a results-only shortcut.

## Feature publication, PR publication, and implemented transition

`workspace publish` takes the expected feature `HEAD`, inspects the owned workspace, and delegates
to the landed non-forcing publication operation. It returns the exact verified remote head. A
divergent remote ref is `contended`; unknown post-effect state is `unknown` and must be reprobed on
retry. Neither outcome permits a force push.

Claude authors the PR title and body. `pr publish` receives that authored content and the canonical
evidence request through files, then mechanically inserts or replaces the Docket-owned change
backlink and exact verified build-evidence block. The operation reparses the evidence rather than
trusting a prior command result and verifies that local `HEAD`, published remote head, evidence
head, requested head, feature branch, effective-base branch, and repository identity agree before
calling the landed `githubcli.EnsurePullRequest` probe/act/verify adapter. It returns the canonical
PR reference, URL, number, head, base, and publication disposition.

The adapter's existing PR adoption rules remain authoritative. This slice neither creates a second
PR lookup policy nor uses a guessed URL or child-agent prose as evidence that a PR exists.

`change mark-implemented` is the final mutation in this scope. Before its metadata transaction it
reprobes Git and GitHub and proves:

- the change is still the exact expected `in-progress` version, reconciled, and linked to the
  verified plan;
- local and remote feature heads equal the supplied exact head;
- valid build evidence names that exact head and a passed local gate;
- exactly one verified PR for the feature branch exists, targets the resolved effective-base
  branch, and names that exact head; and
- any attached results path still satisfies its recorded artifact identity.

It then applies the landed `domain.MarkImplemented` action and atomically records the canonical PR
reference, `status: implemented`, updated date, artifact block, inline board, and audit receipt.
The transition does not clear the claim, delete a branch or workspace, merge the PR, archive the
change, or close stacked descendants. Those are 0316 effects.

## Run verification and resumption

`run verify --id` is read-only and returns one report line in human mode plus typed JSON facts. Its
closed verdicts are:

- `run-complete`: the authoritative change is `implemented` and its recorded plan, optional
  results, feature head, evidence, remote ref, and PR all satisfy this slice's postconditions;
- `run-unclaimed`: the change remains proposed and no claim attributable to this run exists;
- `run-incomplete`: the change is in progress or implemented but one or more required conjuncts
  are absent, stale, contended, malformed, or unknown.

The result enumerates every unmet conjunct with a stable reason and the authoritative identities it
observed. Human text may summarize them, but automation keys on the typed verdict and fields, never
the process exit code or prose.

There is deliberately no `run-halted` write or recovery transition in 0315. An interrupted skill
can invoke the context, inspect, evidence-verify, PR-publish, and run-verify operations again and
continue from durable checkpoints when their preconditions still hold. If safe continuation needs
a reclaim, cleanup, merge/finalize recovery, or persistent halted record, the run stops for 0316 or
human handling rather than inventing that behavior here.

Response-loss retry follows the same rule at every boundary: re-read the authority that owns the
promised postcondition, verify the exact identity, and either return the prior applied result,
continue with the next valid operation, or report `contended`/`unknown`. Filesystem existence,
branch names, PIDs, commit-message prose, child completion notifications, and cached context are
never substitutes for the authoritative check.

## Skill and asset revisions

This change revises only the assets needed for the Go-v1 implementation path:

- `docket-implement-next` becomes the Claude-owned sequencer for the operations and verification
  rules in this spec. It retains candidate judgment, reconciliation, dispatch, bounded repair, and
  stop/report decisions.
- `docket-plan-writer` replaces its Bash backlink call with `docket artifact backlink` while
  preserving ADR-0094's pinned role and compact return contract.
- The build-gate portion of `docket-build` launches and observes the native gate and records exact
  evidence. TDD implementation, named build-agent dispatch, and review judgment remain unchanged.
- Focused reference material, command manifests, agent templates, and embedded assets are updated
  and regenerated together so source and installed protocol versions agree.

Go v1 uses the fixed approved roles. Per-repository role/model routing, skill rebinding, automatic
role substitution, autonomous grooming, and cross-harness delegation remain deferred. This slice
does not change `docket-finalize-change`, the auto-groom agents, capture/harvest skills, terminal
publishing, or release installer policy except where generated asset manifests must enumerate the
newly revised retained files.

## Failure, concurrency, and security rules

- Every metadata mutation performs capability preflight before its transaction and submits the
  exact entity version returned by the last authoritative operation.
- A semantic retry reloads fresh origin and reapplies the same operation only while its
  preconditions and authored intent still hold. It never blindly reuses a stale patch.
- Claim happens before workspace creation. A losing claimant creates no workspace, branch, gate
  run, evidence record, or PR.
- A lease refresh failure stops dispatch of new long-running work. It does not cancel or erase a
  child whose outcome must still be inspected.
- Authored Markdown crosses the CLI through stdin or an explicit request file and is bounded in
  size. Paths are canonical repository-relative values; symlink escapes, absolute artifact paths,
  and paths outside their allowed roots are rejected.
- External effects retain probe/act/verify/record semantics. An unverified push or PR effect is
  `unknown`, not success and not permission for a compensating delete or duplicate create.
- Logs and protocol results redact credentials, environment values, remote URLs with embedded
  credentials, unbounded stderr, authored document bodies, and PR body bytes.
- Managed backlink, artifact, board, and evidence blocks are rewritten only after full marker
  order and balance validation. A malformed block fails without mutation.
- Child-agent returns are hints. Plan, build, review, result, and PR claims are verified from Git,
  documents, gate state, evidence, metadata, and GitHub before the next durable transition.

## Testing strategy

### Unit and contract tests

- Context bundles are internally revision-consistent, preserve exact source bytes, normalize nil
  collections, and report ambiguous/malformed/missing facts without guessing.
- Every command emits exactly one protocol-v1 JSON document in JSON mode and maps closed outcomes
  without prose parsing.
- Claim, refresh, reconcile, attach-plan, attach-results, and mark-implemented exercise all legal,
  illegal, no-op, contention, unsupported-config, renderer-failure, validation-failure, and
  response-loss paths.
- Reconciliation mutation tests remove or corrupt each guarded field, owned section, and marker
  shape in turn and prove the guard reddens.
- Plan/results verification covers containment, symlink escape, untracked files, wrong commits,
  multi-artifact deltas, missing ADR-0094 trailers, malformed backlinks, and stale entity versions.
- Run verification proves each promised postcondition independently by mutating or removing it and
  expecting `run-incomplete` with the matching stable reason.

### Real-Git integration tests

Both `main` and `docket` metadata modes use disposable repositories and real bare remotes. Tests
cover exact-version claim races, transaction retry from fresh origin, lost command responses,
effective-base consumption, workspace resume, local and remote head divergence, non-forcing
publication, artifact commits, board/link atomicity, and independent writers that win between
context and mutation.

Tests make the decision and effect against the same verified checkout. They never use the primary
worktree as a writable metadata scratch space and never infer an operation's success from a stale
local branch.

### Gate, evidence, and GitHub integration tests

- Real supervised processes produce passed, failed, signaled, stopped, vanished, and malformed
  records; evidence creation succeeds only for `passed` at the exact feature head.
- Changing `HEAD` after a passed gate makes evidence verification fail until the full gate is run
  and recorded again.
- A protocol-faithful fake `gh` covers create, adopt, update, response loss, head/base mismatch,
  duplicate PRs, malformed output, timeout, and unknown post-effect state.
- PR bodies preserve authored prose while deterministically replacing only the Docket-owned
  backlink and evidence blocks.

### End-to-end workflow test

A supported disposable repository starts with one build-ready change and real metadata and feature
remotes. Test agents or deterministic fixtures provide reconciliation, plan, implementation,
review, and PR prose. The Go-v1 skill path claims the change, prepares the workspace, attaches the
verified plan, runs a real passing gate, records evidence, publishes through fake `gh`, marks the
change implemented, and receives `run-complete`. The test asserts no direct skill-owned metadata
write and no legacy Bash facade invocation occurred.

Live GitHub publication, released binaries, and direct Claude/Codex/Cursor/OpenCode acceptance are
0317 tests. Docket self-hosting, active-config contraction, Bash removal, and hard-cutover proof are
0318 tests.

## Explicit ownership exclusions

This change does not:

- reimplement configuration, documents, domain policy, Git, transactions, status, install,
  planning mutations, workspace/GitHub/evidence mechanics, or process supervision owned by
  changes 0305 through 0314;
- add finalize, rebase, retest-after-rebase, merge, archive, reclaim, cleanup, maintenance sweep,
  persistent `Run halted`/finalize-blocked state, branch deletion, or stack closeout from 0316;
- add release builds, checksums, downloader changes, upgrade tests, or live four-harness acceptance
  from 0317;
- contract Docket's active configuration, make Go self-hosting authoritative, remove Bash, or
  perform the hard cutover from 0318;
- add auto-groom, automatic capture/harvest, terminal publishing, CI gates, results-only gate
  skipping, cross-harness delegation, a Bash fallback, or a daemon; or
- introduce stack behavior beyond consuming the already-resolved effective base as the workspace
  start point and PR target.

## Acceptance criteria

1. One Claude-hosted Go-v1 run can take a retained build-ready change through claim, reconcile,
   verified plan, build/review, passed local gate, evidence, PR publication, and `implemented`
   without directly editing metadata.
2. Every metadata change uses an exact-version transaction and atomically updates all affected
   v1-owned links, board output, validation, commit, and push state.
3. Every workspace, gate, feature-ref, evidence, and PR promise is verified against its owning
   authority and exact commit; child prose and cached context are never accepted as proof.
4. Claude retains all judgment, authorship, dispatch, review, and bounded repair decisions; Go owns
   validation, state transitions, durable mechanics, and typed outcomes only.
5. Interrupted or response-lost operations converge by inspection and idempotent retry without
   force pushes, duplicate PRs, false evidence, duplicate metadata transitions, or hidden workflow
   session state.
6. Both repository modes pass the hermetic claim-to-implemented workflow test, while unsupported
   requested capabilities fail before mutation.
7. The implementation contains none of the behavior allocated to changes 0316 through 0318 and
   does not duplicate the mechanics already delivered by changes 0305 through 0314.
