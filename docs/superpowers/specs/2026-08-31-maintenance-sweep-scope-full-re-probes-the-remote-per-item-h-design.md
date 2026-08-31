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
Historical cleanup candidates are `done` and `stacked-merged`, not `killed` records. These details
must be reflected in both measurements and test fixtures.

## Assumptions and consistency contract

- Change 0389 has landed. ADR-0101's full/implementation worklists, ordering, counts, and cleanup
  suffix rules are unchanged. This optimization applies to the shared sweep path in both scopes.
- Default, integration, and metadata branches can all advance during a sweep. No immobility is
  assumed. This design deliberately makes repository identity, default/integration branch names,
  initial source revisions, resolved configuration, and initial topology admission a snapshot for
  one invocation. Configuration or default-branch changes take effect on the next invocation.
- Metadata is **not** part of the reusable snapshot. Every reload fetches the current remote
  metadata branch. The sweep's own preceding mutations are visible in the same way as another
  writer's mutations. A missing branch or failed fetch never falls back to a cached revision.
- Existing per-operation GitHub, merge/reachability, branch-absence, ownership, workspace, and
  transaction proofs remain live. In particular, their direct integration-branch fetches must not
  be replaced with the initial source revision. Exact-version and exact-lease transaction checks
  still handle races after the reload.
- A fresh invocation performs the full operational admission again. There is no process-global
  cache and no reuse across repositories or invocations.

## Design

### Explicit reader for one sweep

After the initial full pin succeeds and the existing capability preflight admits the sweep,
derive an explicit immutable sweep session from the Git reader's discovered repository and that
pin. It supplies a `StatusReader` for the remainder of this invocation. Keep ordinary
`gitStatusReader.PinContext` semantics unchanged for standalone commands.

The session's `PinContext` performs one fresh `FetchBranch` of `refs/heads/docket`, then returns a
new pin combining the captured setup/configuration with that fetch's exact metadata commit.
`ReadCorpus` continues to read through that new pin. Do not rerun default-branch discovery,
default/integration setup fetches, or the topology-only metadata `ls-remote`. A supplied metadata
tip must never stand in for this fetch. The session is bound to its original repository; it must
not silently reuse facts if asked to operate on a different repository.

The observable successful-reload contract is **one metadata fetch and zero repeated setup
probes**, not zero network calls. Preserve `FetchBranch`'s own bounded failure-classification
behavior; a failing fetch may perform its existing diagnostic probe within the same network
budget. Do not add another metadata probe before the fetch.

Thread the session into both the sweep helpers and the `Planning.Reader` used by every production
operation closure in `MaintenanceSweep`. The closures are currently created outside
`maintenanceSweep`; wrapping only the local `reader` would leave their full pins unchanged.
Refactor that construction boundary so the closures and reload helpers receive the same session.
Retain the injected `sweepOps` orchestration test seam, but also exercise production wiring.
Concrete helper/type names are implementation details; the explicit session, lifetime, and
freshness contract are settled by this spec.

Only reader plumbing changes in dispatched operations. Their mutation clients, services, and
operation-specific proofs remain untouched. Keep both existing metadata reloads rather than
removing one or substituting a transaction-only freshness check in this change. The initial pin's
configuration is used consistently by both snapshot building and dispatched operations; there
must not be a mixture of freshly resolved and captured configuration within this sweep.

### Bound context reads without changing mutation deadlines

Use a **separate read-only Git client** for the sweep's initial reader and its derived session,
constructed with `gitcli.WithNetworkTimeout(30 * time.Second)`. This is a fixed internal policy
for sweep context reads, not a new flag or configuration key. Its local-operation timeout remains
the package default. The constructor may expose an internal test seam for shorter durations.

The CLI's sweep dependency construction must leave `Planning.Client`, its transaction engine,
`CleanupGit`, the workspace service, and the GitHub client on their existing timeout policies.
Only `Planning.Reader` and the session's metadata-fetch path use the short-timeout client. Do not
replace the shared mutation client, change either package default, or carry a short context
deadline from a read into the subsequent mutation. Standalone status/finalize/reclaim commands
and repository setup commands keep their existing policies.

Thirty seconds bounds each Git network adapter operation on the read client, not the full pin,
item, or sweep. Keep `FetchBranch`'s shared budget for fetch and failure classification and allow
the adapter's existing process-reaping overhead. A lower caller deadline still takes precedence.
GitHub calls and mutation/proof traffic can still take longer; this change makes no promise that
all stalls or the whole sweep are capped at thirty seconds.

### Failure and reporting behavior

- A network timeout returned by the initial full pin refuses the sweep before dispatch, using
  its existing typed external-failure result. The diagnostic must retain the adapter's timeout
  kind/message. Other initial failures retain their existing classifications.
- A metadata fetch failure in a sweep helper produces the existing skipped `reload-failed` entry
  and dispatches no operation for that item. A failure in the operation's own pin uses that
  operation's existing refusal mapping; an unresolved closeout withholds cleanup.
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
- Each successful helper/operation pin after setup performs exactly one metadata fetch and reads
  the returned revision. Disabled reclaim performs no reload.
- The production closeout/cleanup/reclaim closures use the session, not the original full reader.
- Required operation-specific network calls still occur and use their existing safety gates.

Mutation-test bypassing the session separately in a helper and in a production operation closure;
both must make the traffic guard fail. Also cover a second invocation and another repository so
reuse cannot escape its declared lifetime.

### Selective blocked-probe regression

Replace the old all-network-blocked completion test with a phase-aware fixture: allow initial
setup, fresh metadata fetches, and required operation traffic to succeed; after setup, make only
redundant setup probes fail or block under a short independent watchdog. Assert terminal return,
the expected successful entries, and zero attempts at those forbidden probes. Merely returning
after skipping the whole worklist is not a pass. Injecting a redundant probe must fail promptly,
not hang the test.

Separately block an allowed metadata fetch and assert the bounded failure/skip contract. A sweep
with blocked required network access is not expected to complete its mutations successfully.

### Metadata and source movement

Advance the local origin from an independent writer without refreshing the sweep clone's tracking
refs. Verify changed blob versions, vanished/moved records, and ambiguity are observed on the next
reload. Cover movement between the helper pin and the operation pin, and verify the cleanup suffix
sees a record archived by its preceding closeout. Race the subsequent transaction too, retaining
the existing contention proof. Reusing the initial metadata revision must redden these tests.

Advance default/integration source and configuration after setup: the invocation keeps its
captured setup, the next invocation adopts the new configuration, and direct mutation/proof
fetches still observe live remote state. Include metadata-fetch failure and deletion to prove no
stale-ref fallback or inferred absence is introduced.

### Timeout policy and safety

Assert the resolved production reader policy is thirty seconds and is installed by the sweep
command's real dependency builder. With a short injected duration, block initial and later
metadata reads through a controlled Git executable; assert adapter timeout, bounded return with
scheduling/reaping allowance, the operation result/entry mapping, and no forbidden mutation.
Do not make tests wait thirty seconds or derive an exact wall-clock equality from one machine.

Prove isolation as well: the transaction, cleanup, workspace, and GitHub services retain their
original clients/deadlines, standalone commands remain unchanged, and a mutation following a
successful read does not inherit that read's cancelled deadline context. Where a network operation
is intentionally left on the original policy, assert that boundary through resolved dependency
state or a controlled executable rather than
incurring the five-minute delay. Mutation-test removal of the short reader option and accidental
use of that client for mutation work.

### Performance evidence and build gate

Record before/after medians and categorized call counts on the same isolated representative
multi-item workload, with equal synthetic network latency and identical operation outcomes. The
candidate must reduce measured time on that controlled workload and reduce repeated pin traffic
from the baseline four/five network calls to one successful metadata fetch. Compare the same
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
- Remove the sweep's reload and rely only on transaction contention: potentially less work, but
  changes observation/skip behavior and needs a separate safety design. Retain current boundaries.
- Keep full pins while adding only a timeout: bounds some stalls but leaves the repeated setup
  traffic. Changing `gatherRepoFacts`'s existing tip short-circuits alone also misses the caller's
  earlier fetches and default-branch discovery.
- Shorten the shared sweep client or package defaults: rejected because it changes mutation and
  proof deadlines. A separate reader client makes the selected boundary explicit and testable.
- Hide a cache inside the ordinary reader: rejected in favor of an explicit session whose lifetime
  and invocation-level configuration snapshot are visible to callers.

## Out of scope

Scope vocabulary/membership and ADR-0101; closeout/cleanup/reclaim policy or proof changes;
eliminating freshness/CAS checks; GitHub or mutation timeout tuning; global defaults; retry policy,
total deadlines, circuit breakers, streaming output, scheduling, or parallel execution; corpus
parsing and unrelated suite runtime optimization; the subsequent health-check pass. This grooming
authors metadata only and does not implement code or run live maintenance.
