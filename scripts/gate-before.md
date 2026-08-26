# gate-before.sh — arm the run gate (thin delegator)

## Purpose

`scripts/gate-before.sh` is a **pure delegator** over the Go binary. All behavior —
re-syncing the metadata worktree to fresh origin, reading the in-progress claim set,
capturing the dispatch epoch, minting the durable gate record, and printing
`gate-armed <key>` or `gate-unarmed <reason-token>` — lives in
`docket run gate-before` (`internal/app/rungate_before.go`, change 0334). The wrapper
adds none of it.

It exists so the run-gate payload's consumer-facing spelling
`docket.sh gate-before implement-next` resolves in a consuming repo, where the Go
binary is on PATH but this helper facade is not.

## Usage

```
docket.sh gate-before <target>
```

- `<target>` — the only accepted value is `implement-next`; anything else is a usage
  error (non-zero exit) the Go binary owns.

The wrapper forwards its whole argument vector to `docket run gate-before …` and
`exec`s the binary, so **stdout and the exit code pass through untouched**.

## Behavior and exit codes

The wrapper decides nothing. Per the Go operation: `gate-armed <key>` on a successful
arm and `gate-unarmed <reason-token>` on any arming failure both exit `0` — the report
line is the contract (learning: exit-code-encodes-a-non-failure). Only a bad target is
a non-zero usage error.

## Mock seam

`DOCKET_BIN` (default `docket`) — the Go binary the wrapper delegates to, the same seam
`scripts/verify-run.sh` uses. There is no other seam and no other state.
