<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0390 — maintenance sweep --scope full re-probes the remote per item, hanging the sweep](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0390-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h.md)**
<!-- docket:backlink:end -->

# Design — eliminate per-item remote re-probing in `maintenance sweep --scope full`

## Problem

`docket maintenance sweep --scope full` runs for many minutes with no streamed output and no
observed completion (killed after 1m25s during investigation). It is **not** a deadlock: bare
`git fetch`, `git ls-remote`, and `ssh git@github.com` each complete in ~0.5s against the same
remote. The pathology is redundant network **volume**, aggravated by an over-long per-op timeout.

### Confirmed mechanism

`maintenanceSweep` (`internal/app/maintenance.go`) pins one operational context at the top of the
run (the `reader.PinContext` call whose comment reads *"every mutation below reloads its own fresh
authority (and the dispatched op reloads once more)"*). It then iterates the worklist and, for
**every** item, calls `sweepRunCloseout` / `sweepRunCleanup` / `sweepRunReclaim`, each of which
calls `sweepReloadVersion` (via `sweepReloadPresent`) to "re-pin fresh authority." That reload is
another `reader.PinContext`, and the dispatched op (`FinalizeCloseout` / `FinalizeCleanup` /
`ChangeReclaim`) re-pins once more.

Each `PinContext` runs `loadOperationalContext` → `gatherRepoFacts`, which performs **three network
git round-trips to GitHub over SSH**:

- `FetchBranch(setupRemote(), <default-branch>)`
- `FetchBranch(setupRemote(), <integration-branch>)`
- `ProbeRemoteBranch(setupRemote(), <metadata-branch>)` — a `git ls-remote`

For a full-scope sweep over the current backlog (236 `done` + 83 `killed` terminal records feed the
historical-cleanup pass), this is O(items × network-round-trips) sequential GitHub calls. A
goroutine dump captured the process parked in one such `ProbeRemoteBranch` protocol read.

### Aggravating factor

`gitcli`'s `defaultNetworkTimeout` is **5 minutes** (`internal/gitcli/client.go`). The per-op
context deadline is therefore 5 minutes, so any single stalled probe adds up to 5 minutes of
silent hang before the sweep gets a `KindTimedOut`.

### Why the remote re-probe is pure waste

The per-item reload exists to protect against a **metadata race** — another agent moving a change's
state on `origin/docket` between the worklist snapshot and the mutation. That authority is the
change blob versions. The **remote branch tips** (default / integration / metadata) are not the
authority the reload protects, and they do not change over a single sweep run. Re-fetching them per
item recomputes a value that was already known from the sweep's initial pin.

## Chosen approach — reuse the sweep's pinned remote facts + cap the sweep network timeout

### 1. Per-item reload reads metadata only, taking remote facts as given

`gatherRepoFacts` already has the seam: when its input carries `defaultTip` / `integrationTip`, it
uses them instead of calling `FetchBranch` (the `in.defaultTip != ""` / `in.integrationTip != ""`
branches in `internal/app/repository_facts.go`). The metadata-branch probe (`ProbeRemoteBranch`)
is the remaining un-short-circuited network call and must gain the same treatment: a supplied
metadata tip is taken as given rather than re-probed.

The sweep captures the remote facts once from its initial `PinContext` and threads them into every
per-item reload, so `sweepReloadVersion`'s `PinContext` reloads only the metadata (re-reading change
blob versions from `origin/docket`, which genuinely can move) and performs **zero** remote network
probes. The dispatched op's own re-pin is fed the same pinned remote facts.

Preferred shape (to be settled in the plan against the actual `StatusReader` / operational-context
types): pass the already-resolved remote facts as an optional input to the reload path — either a
variant of `PinContext` that accepts pre-pinned remote facts, or a sweep-scoped reader that carries
them — rather than adding hidden per-reader mutable cache state. Whatever the shape, the observable
contract is: **a per-item reload issues no remote network call.**

Rejected alternatives:

- **Split a separate metadata-only re-pin path from the operational-context load.** Cleaner
  conceptual separation but a larger surface than reusing the existing short-circuit; deferred as
  unnecessary for the fix.
- **Cache remote facts inside the reader for the sweep lifetime.** Least call-site churn, but adds
  hidden mutable state to a type whose whole purpose is to be freshly re-pinnable; worse for
  reasoning and worse for the next reader of this code.

### 2. Cap the sweep's per-op network timeout

Independent of change 1, the sweep must run its `gitcli` client with a network timeout far below the
5-minute default (target ~20–30s — comfortably above the observed ~0.5s healthy probe, far below a
silent multi-minute hang), so a genuine network stall fails fast and visibly with a `KindTimedOut`
finding instead of hanging. Scope the shortened timeout to the sweep's client construction; do not
change the package default for unrelated callers unless the plan finds that cleaner and safe.

This is defense-in-depth: change 1 removes the redundant **volume**; change 2 bounds any **single**
stall — including a first, legitimate probe against a genuinely slow remote.

## Testing

- **Zero-network per-item reload (the core guard).** Drive a multi-item worklist with a `gitcli`
  seam whose network operations are counted (or hard-fail). After the initial pin, assert the
  per-item reload path issues **zero** remote network calls — strip the remote-facts reuse and the
  test must redden (mutation-tested guard, not decoration). This is the regression lock for the
  bug.
- **Sweep completes with a blocking network.** With a git seam whose network calls would block,
  assert the full-scope sweep processes its whole worklist and returns — proving no per-item network
  dependency remains.
- **Timeout cap honored.** Assert the sweep constructs its client with the shortened network
  timeout, and that a stalled probe surfaces a `KindTimedOut` finding within the cap rather than the
  5-minute default.
- **Metadata race still caught.** The reload must still detect a change that moved/vanished on
  `origin/docket` between snapshot and mutation (the reload's actual purpose) — a metadata mutation
  between pin and reload is still observed. Reusing remote facts must not weaken this.
- Run the whole suite at the build gate (`finalize.test_command` → `go run ./cmd/docket development
  test`), not only these files.

## Out of scope

- `--scope implementation` — ADR-0101 / change 389 already keep historical cleanup off the
  `docket-implement-next` startup hot path; this change does not alter scope semantics.
- The separate `internal/app` suite wall-clock / timeout work.
- The content or behavior of the sweep's health-check pass.
- Any change to **what** the sweep decides to close out, clean up, or reclaim. This is purely about
  **how** per-item fresh authority is reloaded and how long a single network probe may block.
