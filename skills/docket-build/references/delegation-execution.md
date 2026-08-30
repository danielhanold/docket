# Delegation execution — required capabilities and per-harness evidence

Point-in-time evidence record for the retired Bash delegation facade's adapter launch shape. The
maintained dispatch surface is host-native (change 0371) and never invokes the facade; this page is
kept as the measurement record behind the frozen facade until change 0370 deletes it. The six
required capabilities are defined once, in [`gate-execution.md`](gate-execution.md) — this file does
not restate them. What it adds is the evidence for a *different launch shape*: the **adapter** launch
that starts a whole delegated agent run, rather than the **gate** launch that starts a test command.

Two things on this page are deliberately kept apart. The **mechanism** — docket's own detachment
act — was measured. The **per-harness verdicts** were not. Reading the first as evidence for the
second is the error this page exists to prevent.

## Why the gate verdicts do not transfer

`gate-execution.md`'s verdicts are explicitly **version- and scope-scoped**, and every one names the
shape it measured — a gate launch. A delegated adapter launch differs in duration, in what the child
does to the repo, and in which CLI subcommand is invoked. A verdict measured for one is **not**
evidence for the other, so these rows are re-derived rather than inherited. Never inherit a row on
faith.

Read a verdict exactly as `gate-execution.md` defines it: `supported` — measured, with evidence and
version recorded. `unverified` — not measured, or measured inconclusively; **treat as unknown, never
as working.** `incompatible` — measured and established as unable to meet a required capability.
A probe that changes two variables at once proves nothing about either, and a run in which the child
never started is **inconclusive**, not `incompatible`.

## The mechanism — measured hermetically

Independent of any harness, the facade's detachment mechanism was measured hermetically on **macOS
(darwin 25.6.0), GNU Bash 5.x**, 2026-08-09:

- `setsid` is **not present** on macOS, and docket takes no `perl` dependency. Detachment therefore
  uses Bash **job control**: under `set -m`, a background job becomes a **process-group leader**.
- One run, two arms, one variable changed: a launcher started two children, one under `set -m` and
  one not, then the launcher's whole **process group** received `TERM`. The `set -m` child (own
  PGID) **survived**; the non-`set -m` child (the launcher's PGID) was **killed**.
- That is capability 1's stronger reading — survival of the teardown of the initiating call's
  process group, not merely of its parent's exit. The frozen facade's own detach test pins it with a
  fake adapter, and that assert is mutation-tested by removing `set -m`.

This measures the **facade**, not any harness. It says nothing about whether a given child CLI
tolerates being started that way, which is what the next section is about.

## Per-harness verdicts — the adapter launch shape

Every row below is **`unverified`**: the adapter launch shape has not been measured on any harness.
Re-probing requires each CLI installed and authenticated, which the change that introduced this file
could not do. This is a recorded gap, not an implied pass — an `unverified` row is read as unknown,
so nothing on this page licenses treating a delegated run on these harnesses as known to work.

| Harness | Adapter launch shape | Verdict |
|---|---|---|
| claude | none shipped — `scripts/runners/` holds codex, cursor and opencode only; claude is the *parent* harness on the delegation path | `unverified` — no adapter exists to measure |
| cursor | `cursor … --print --force --sandbox disabled` | `unverified` — adapter launch shape unmeasured |
| codex | `codex exec --skip-git-repo-check --sandbox danger-full-access` | `unverified` — adapter launch shape unmeasured |
| opencode | `opencode run --dir <anchor>` (plus `--auto` under `permissions: auto-approve`) | `unverified` — adapter launch shape unmeasured |

The population is the shipped harness roster (`HD_SHIPPED_HARNESSES`), so every harness docket ships
defaults for is accounted for here, including the one that carries no adapter. Each shape named
above is read from that runner's own adapter (each runner's own adapter contract in the frozen
facade); it is what *would* be probed, not what has been.

## Probe recipe

For each harness that has a shipped adapter, with its CLI installed and authenticated, changing
**one** variable per run:

1. Launch a delegated agent through the frozen facade's launch verb (see the facade's own contract
   doc) with a task deliberately longer than the parent harness's foreground ceiling.
2. Confirm the call returns in seconds with a dispatch key, and that
   `<git-common-dir>/docket/dispatch/<key>/launch` records a `pgid` different from the launching
   shell's.
3. Tear down the launching call's **process group** (`kill -TERM -<launcher pgid>`), not just its
   pid. Isolate the launcher into its own group first, or the probe kills the harness running it.
4. Wait past the child's expected duration, then check for `<key>/done` and a non-empty
   `<key>/stdout.log`.
5. Sentinel present with a well-formed `exit_code` ⇒ capabilities 1–3 hold for this shape; record
   the measured verdict with the CLI version. Sentinel absent ⇒ **inconclusive**, not
   `incompatible`, unless the run establishes the child was killed by the teardown.

Record the version in the row. Re-probe when it moves.
