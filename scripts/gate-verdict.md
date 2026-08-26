# gate-verdict.sh — report the run-gate verdict (thin delegator)

## Purpose

`scripts/gate-verdict.sh` is a **pure delegator** over the Go binary. All behavior
lives in `docket run gate-verdict` (`internal/app/rungate_verdict.go`, change 0334):

- **Attributed mode** (`gate-verdict <key>`) — loads the durable gate record armed by
  `gate-before`, attributes exactly one new in-progress claim to the dispatched run,
  delegates the run predicate to `RunVerify`, and renders one line of the attributed
  vocabulary (`gate-done` / `gate-retry-once` / `gate-stop …`) with atomic one-retry
  accounting.
- **Observe-only mode** (`gate-verdict --unattributed [<id>...]`) — holds no key, reads
  and writes no record, verifies the supplied hint ids (or every current in-progress
  id) and prints one `gate-observe …` line each. There is no path from this mode to a
  retry grant.

The wrapper adds none of it. It exists so the run-gate payload's consumer-facing
spelling `docket.sh gate-verdict …` resolves in a consuming repo, where the Go binary
is on PATH but this helper facade is not.

## Usage

```
docket.sh gate-verdict <key>
docket.sh gate-verdict --unattributed [<id>...]
```

The wrapper forwards its whole argument vector to `docket run gate-verdict …` and
`exec`s the binary, so **stdout and the exit code pass through untouched**.

## Behavior and exit codes

The wrapper decides nothing. Per the Go operation every `gate-*` report line exits `0`
— the report line is the contract (learning: exit-code-encodes-a-non-failure); only a
mode/argument mismatch (a key alongside `--unattributed`, a missing key without it, or
a non-integer hint) is a non-zero usage error.

## Mock seam

`DOCKET_BIN` (default `docket`) — the Go binary the wrapper delegates to, the same seam
`scripts/verify-run.sh` uses. There is no other seam and no other state.
