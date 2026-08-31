<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0390 — maintenance sweep --scope full re-probes the remote per item, hanging the sweep](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0390-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h.md)**
<!-- docket:backlink:end -->

# Design — reuse sweep setup while fetching fresh metadata

## Problem and corrected diagnosis

Full maintenance repeatedly resolves repository setup for its worklist, so historical growth
multiplies network latency. The reported investigation stopped a sweep after 1m25s without a
terminal result. Healthy standalone remote commands took about 0.5s, and a goroutine dump showed
`ProbeRemoteBranch` in flight. This establishes a plausible amplification path; it does not
measure the full sweep or establish that network setup is its only cost.

Against source revision `c95e5189febeecffa2487d3303069bce2b07b92f`, the successful ordinary
`PinContext` path is `gitStatusReader.PinContext` → `loadOperationalContext`:

1. `RemoteDefaultBranch` performs a live `ls-remote --symref`.
2. `fetchPinnedRevision` fetches the default branch and supplies the configuration source.
3. A distinct integration branch is fetched; when it equals the default branch, its revision is
   reused and there is no second fetch.
4. `gatherRepoFacts` probes remote metadata presence. Its default/integration short-circuits are
   **already used** by this caller; changing those branches alone cannot fix the repeated work.
5. `fetchPinnedRevision` fetches `refs/heads/docket`, producing `StatusPin.MetadataRevision`.

Thus a healthy pin normally makes four Git network invocations, or five with distinct default and
integration branches. The metadata-presence probe and metadata fetch are separate operations.
Failure-classification traffic can add calls. `ReadCorpus` reads an immutable commit identified by
`MetadataRevision`; reading it again cannot discover a newer remote commit.

`maintenanceSweep` pins the inventory once; `sweepReloadVersion` re-pins before dispatch; and
`FinalizeCloseout`, `FinalizeCleanup`, or `ChangeReclaim` pins again. A successful closeout also
has a separately reloaded cleanup suffix. Reclaims with `reclaim.auto` disabled do not reload.
`ChangeReclaim` additionally reaches another `PinContext` through `WorkspaceInspect`; counting
only the three top-level operations misses that nested reload.
Historical cleanup candidates are `done` and `stacked-merged`, not `killed` records. These details
must be reflected in both measurements and test fixtures.

## Assumptions and consistency contract

- Change 0389 has landed. ADR-0101's full/implementation worklists, ordering, counts, and cleanup
  suffix rules are unchanged. This optimization applies to the shared sweep path in both scopes.
- Default, integration, and metadata branches can all advance during a sweep. No immobility is
  assumed. This design deliberately makes repository identity, default/integration branch names,
  initial source revisions, resolved configuration, and initial topology admission a snapshot for
  one invocation. Configuration or default-branch changes take effect on the next invocation.
- Metadata is **not** reusable across operation attempts. Fetch it once immediately before an
  attempt, then share that exact pin, corpus, and blob versions with the operation and its nested
  readers. The next attempt fetches anew, including cleanup after closeout. A missing branch or
  failed fetch never falls back to a prior revision.
- Existing per-operation GitHub, merge/reachability, branch-absence, ownership, workspace, and
  transaction proofs remain live. In particular, their direct integration-branch fetches must not
  be replaced with the initial source revision. Exact-version and exact-lease transaction checks
  still handle races after the reload.
- A fresh invocation performs the full operational admission again. There is no process-global
  cache and no reuse across repositories or invocations.

## Design

### One shared metadata observation per operation attempt

After the initial full pin succeeds and the existing capability preflight admits the sweep,
derive an explicit sweep session from the Git reader's discovered repository and that pin. Its
captured setup is immutable. Keep ordinary `gitStatusReader.PinContext` semantics unchanged for
standalone commands.

The session explicitly prepares each operation attempt with one fresh `FetchBranch` of
`refs/heads/docket`, combining the captured setup/configuration with that fetch's exact metadata
commit. It reads the corpus once at that revision and supplies an immutable observation containing
the pin, corpus, and blob versions. Do not rerun default-branch discovery,
default/integration setup fetches, or the topology-only metadata `ls-remote`. A supplied metadata
tip from a previous attempt must never stand in for this fetch. The session is bound to its
original repository; it must not silently reuse facts if asked to operate on a different repository.

The sweep checks presence/ambiguity and obtains reclaim's expected version from this observation.
Pass the same observation into the dispatched operation through an explicit reader bound to that
attempt: its `PinContext` returns the supplied pin without fetching, and its `ReadCorpus` returns
the same immutable corpus without rereading the remote or resolving mutable tracking refs. This
also applies to nested readers such as `WorkspaceInspect` inside reclaim. The operation still runs
its own capability and domain validations against these bytes. Do not add an implicit reader
cache, a time-to-live, or a user-authored shortcut that can skip the preparation boundary.

The successful preparation contract is **one metadata fetch for the whole operation attempt and
zero repeated setup probes or nested-reader fetches**, not one call for each invocation of
`PinContext` and not one total remote call per change. Preserve `FetchBranch`'s bounded
failure-classification behavior; a failing fetch may perform its existing diagnostic probe within the same network
budget. Do not add another metadata probe before the fetch.

Thread the prepared observation into both the sweep helpers and the `Planning.Reader` used by
every production operation closure in `MaintenanceSweep`. The closures are currently created
outside `maintenanceSweep`; wrapping only the local `reader` would leave their pins unchanged.
Refactor that construction boundary so each closure receives the observation for its attempt.
Retain the injected `sweepOps` orchestration test seam, but also exercise production wiring.
Concrete helper/type names are implementation details; the preparation boundary, immutable
observation, lifetime, and freshness contract are settled by this spec.

Discard the observation after its operation returns. A cleanup suffix must prepare again because
closeout may have changed paths, statuses, and stack relationships. Never share an observation
across two operations or changes. The initial pin's configuration is used consistently throughout
the invocation; do not mix freshly resolved and captured configuration.

Preserve the existing metadata-race and skip decisions at the single preparation boundary. A
later concurrent edit is handled by the existing transaction's fresh-base and exact-version checks
where those apply. Cleanup's workspace/ref deletion is not one metadata transaction, so a fresh
pre-operation observation and its live ownership/merge/ref proofs remain necessary under the
current safety policy. This design removes the second observation immediately after the first;
it does not claim atomic protection against every concurrent metadata edit throughout cleanup.

### Call budget and what one fetch buys

The initial metadata fetch supplies **all changes** for inventory and selection. No per-record
metadata fetch is necessary to enumerate or parse that inventory. After setup:

| Work | Metadata preparation fetches | Other remote traffic |
|---|---:|---|
| Record outside this invocation's worklist; disabled reclaim | 0 | None caused by metadata enumeration; existing PR selection queries are separate |
| One cleanup or enabled reclaim attempt | 1 | Existing operation-specific proofs and any transactions/writes |
| Closeout followed by its allowed cleanup suffix | 2 | Existing proofs and any transactions/writes for both operations |
| Nested readers within an already prepared operation | 0 additional | Their existing non-metadata safety probes, if any |

A vanished/ambiguous record or failed preparation skips dispatch after that preparation attempt;
it must not retry the fetch through the operation. Transaction fetches and push reconciliation are
additional intentional calls; they are not nested reader reloads and must not be removed to meet
this budget. In particular, do not advertise this as one total GitHub request per change.

Full scope still schedules a cleanup attempt for every `done`/`stacked-merged` candidate. It
therefore still performs one metadata preparation fetch per historical cleanup attempt, even if
the operation ultimately has nothing to change. Eliminating that remaining archive-dependent
traffic needs a separate bulk no-work detector that accounts for remote refs, workspaces, and
backlink repairs. Merely having fetched all records once does not prove those resources clean.

### Sweep network deadlines

Configure the sweep's Git/GitHub adapters with explicit budgets by operation purpose:

- **Remote reads: 30 seconds.** Includes default-branch discovery, metadata/source fetches,
  transaction base fetches, branch-presence/ancestry preparation, GitHub repository/PR discovery,
  merged-state checks, and open-child queries.
- **Remote writes: 60 seconds.** Includes metadata/integration pushes and lease-protected remote
  branch deletion. A timeout is an uncertain result until the existing reconciliation establishes
  otherwise; never assume the remote write did not happen.

These are sweep-only constructor policies. Cover `Planning.Reader`, `Planning.Client`, its
transaction engine, `CleanupGit`, workspace services, `PRProber`, and the GitHub client; leaving
any reachable network path at the five-minute default fails the requirement. Add an explicit
read/write distinction to adapter timeout selection where needed rather than setting one shared
thirty-second timeout for every operation. Existing package defaults and standalone command
policies remain unchanged, as do local-operation budgets. Internal duration injection is allowed
for tests; no new user flag or configuration setting is required.

The budget belongs to one top-level adapter operation, including its internal failure diagnosis
or lost-response reconciliation. A fetch plus its classification probe shares the thirty-second
budget; a push/delete plus its reconciliation shares the sixty-second budget. Child probes may
use a smaller read budget but cannot extend the parent's remaining time. If time is exhausted,
return the existing unknown/external-failure outcome and preserve recovery evidence; do not start
fresh clocks or new write retries to manufacture a verdict.

Allow the adapter's existing process-reaping overhead; a shorter caller deadline takes precedence.
Scope each deadline to the adapter call and release it before the next independent operation.
This is not a thirty/sixty-second deadline for an entire item or sweep. Many independent calls can
still accumulate, and there is no new sweep-wide deadline or outage circuit breaker.

### Failure and reporting behavior

- A network timeout returned by the initial full pin refuses the sweep before dispatch, using
  its existing typed external-failure result. The diagnostic must retain the adapter's timeout
  kind/message. Other initial failures retain their existing classifications.
- A metadata preparation failure produces the existing skipped `reload-failed` entry and
  dispatches no operation for that attempt. The prepared operation's reader performs no further
  fetch that could fail independently. Other operation failures keep their existing mappings;
  an unresolved closeout withholds cleanup.
- A successful fetch followed by a missing or ambiguous record keeps the existing vanished/skip
  behavior. Do not confuse it with an unsuccessful fetch or an absent metadata branch.
- Retain per-item result aggregation and continuation behavior. A mixed or all-failed sweep must
  be assessed from its entries, as change 0389 requires; a returned top-level no-op does not prove
  that every item succeeded. Do not add a new top-level result or a `KindTimedOut` protocol enum.
- Preserve cancellation handling and transaction failure/recovery semantics. No automatic retry,
  circuit breaker, or streamed progress is introduced here.

## Verification and acceptance

### Network classification and production wiring

Trace all reachable network sites from the actual sweep path, classifying them by purpose rather
than banning an argv spelling globally: an integration fetch for a cleanup reachability proof is
required even though an integration fetch during a repeated setup pin is redundant. Use a local
origin and controlled Git/GitHub adapters or executables; tests must not call a live remote.

Drive multiple items, both scopes, equal and distinct default/integration branches, an enabled
reclaim, a disabled reclaim, and a successful closeout with its cleanup suffix. Assert that:

- Full setup runs once per invocation, irrespective of historical population.
- Each successful preparation performs exactly one metadata fetch. The helper, operation, and
  nested readers all receive that revision and its corpus; they perform no duplicate fetches.
  Disabled reclaim performs no preparation.
- The production closeout/cleanup/reclaim closures use the prepared observation, not the original
  full reader. Exercise reclaim's `WorkspaceInspect` path too; top-level fakes alone miss it.
- Required operation-specific network calls still occur and use their existing safety gates.

Mutation-test bypassing the session in a helper, restoring an operation's redundant pin, and
leaving a nested reader unbound; each must make the traffic guard fail. Also cover a second
invocation and another repository so reuse cannot escape its declared lifetime.

### Selective blocked-probe regression

Replace the old all-network-blocked completion test with a phase-aware fixture: allow initial
setup, one preparation fetch per operation, and required operation traffic to succeed; after setup,
make redundant setup probes and duplicate reader fetches fail or block under a short independent
watchdog. Assert terminal return, the expected successful entries, and zero attempts at those
forbidden probes. Merely returning
after skipping the whole worklist is not a pass. Injecting a redundant probe must fail promptly,
not hang the test.

Separately block an allowed metadata fetch and assert the bounded failure/skip contract. A sweep
with blocked required network access is not expected to complete its mutations successfully.

### Metadata and source movement

Advance the local origin from an independent writer without refreshing the sweep clone's tracking
refs. Verify changed blob versions, vanished/moved records, and ambiguity are observed on the next
preparation. Assert that the helper, operation, and nested readers use the same observation even
if a concurrent edit arrives afterward; the subsequent transaction must detect a conflicting
edit using its existing checks. Verify the cleanup suffix takes a new observation and sees the
record archived by its preceding closeout. Reusing a previous attempt's metadata revision must
redden these tests. Retain deletion/ownership/merge proof tests: a shared observation cannot
authorize cleanup after a moved ref, active workspace, or unknown probe.

Advance default/integration source and configuration after setup: the invocation keeps its
captured setup, the next invocation adopts the new configuration, and direct mutation/proof
fetches still observe live remote state. Include metadata-fetch failure and deletion to prove no
stale-ref fallback or inferred absence is introduced.

### Timeout policy and safety

Assert resolved production policies of thirty seconds for remote reads and sixty seconds for
remote writes through the sweep command's real dependency builder. Derive the reachable network
sites from source and verify each receives the correct policy, including nested workspace,
transaction, cleanup, and GitHub paths. Standalone clients must retain their default policies.

Use short injected durations and controlled Git/GitHub executables to block initial and later
metadata reads, a PR/proof read, a transaction fetch, and a push/delete. Assert bounded return with
scheduling/reaping allowance, the existing result/entry mapping, and no forbidden follow-on
mutation. Verify that internal diagnostic/reconciliation calls share the parent budget rather
than multiplying it, and that a potentially applied timed-out write remains unknown/recoverable
unless existing reconciliation proves its outcome. A successful read's cancelled deadline context
must not leak into the following mutation.

Mutation-test losing policy propagation at a nested dependency, restoring a duplicate reader
fetch, and resetting the clock before a diagnostic probe. Test that read and write durations are
distinct; a blanket thirty-second policy must fail. Do not wait thirty/sixty seconds in tests or
assert wall-clock equality calibrated on one machine.

### Performance evidence and build gate

Record before/after medians and categorized call counts on the same isolated representative
multi-item workload, with equal synthetic network latency and identical operation outcomes. The
candidate must reduce measured time on that controlled workload and reach the call-budget table:
one metadata preparation fetch per operation attempt and no duplicate reader fetches, including
nested readers. The original implementation performs at least two full pins per dispatched
operation (eight/ten context-read network calls, plus any nested pins). Compare the same
population and record remaining metadata, corpus, GitHub, and operation-proof costs separately;
do not infer end-to-end improvement solely from the call-count assertion.

Whole-sweep cost remains dependent on item count: there are still fresh metadata fetches, corpus
reads, and legitimate remote operations. This change removes setup amplification, not all linear
work, and provides no archive-independent runtime guarantee. A live sweep is mutating and is not
a benchmark fixture; use an isolated repository. Report any unmeasured live-performance claim as
a limitation in the results.

Run the complete build gate from the resolved `finalize.test_command` (currently
`go run ./cmd/docket development test`) and handle budget findings under `tests/README.md`.

## Alternatives considered

- Reuse the initial metadata tip and perform no network reload: rejected because immutable Git
  blobs cannot reveal concurrent metadata changes or the sweep's own newly archived records.
- Keep a helper refresh and another operation/nested-reader refresh: rejected. One observation
  prepared immediately before dispatch serves all of those readers and removes duplicate calls
  while preserving the helper's presence/version checks.
- Use only the initial inventory plus transaction contention: rejected for this change. Cleanup
  performs destructive effects outside a metadata transaction, and its fresh metadata/ownership
  checks cannot be replaced by a transaction it does not execute.
- Keep full pins while adding only a timeout: bounds some stalls but leaves the repeated setup
  traffic. Changing `gatherRepoFacts`'s existing tip short-circuits alone also misses the caller's
  earlier fetches and default-branch discovery.
- Apply a blanket thirty-second timeout or change package defaults: rejected. Sweep reads and
  writes get distinct explicit budgets, with uncertain-write recovery preserved; unrelated
  commands keep their current policies.
- Hide a cache inside the ordinary reader: rejected in favor of an explicit session whose lifetime
  and invocation-level configuration snapshot are visible to callers.

## Out of scope

Scope vocabulary/membership and ADR-0101; closeout/cleanup/reclaim policy or proof changes;
eliminating the single preparation boundary or transaction CAS checks; global defaults; retry policy,
total deadlines, circuit breakers, streaming output, scheduling, or parallel execution; corpus
parsing beyond reuse of one operation's observation and unrelated suite runtime optimization;
bulk no-work detection or mutation batching; the subsequent health-check pass. This grooming
authors metadata only and does not implement code or run live maintenance.
