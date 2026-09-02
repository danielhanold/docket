# Gate driver contract — the caller-side contract for driving the native gate

This reference is the **caller-side contract for driving the native gate**: the typed driver
operations a caller invokes, the disposition vocabulary those operations return, and the ownership
handoff a departing caller must perform. It is a **caller contract, not a harness quarantine** —
that axis separates it from [`gate-execution.md`](gate-execution.md), which holds the measured
per-harness capability verdicts and mechanism detail read once, ahead of the act. Change 0271 drew
the same line when it created `references/delegation-execution.md` rather than folding caller-facing
content into `gate-execution.md`.

Change 0342 retired the executable Bash observe loop this file used to publish. A caller no longer
sleeps and re-parses raw observation documents by hand; it makes **short, slice-bounded,
synchronous** calls to the native gate **driver**, which composes the raw supervisor,
persists one deadline and one execution identity, and returns one of four typed dispositions per
call. No caller runs a shell poll loop, backgrounds the suite, subscribes to a notification, or
authors its own liveness check — the driver owns all of that.

## The driver's operations

The high-level surface is the `gate.drive` operation group: `gate.drive.start`,
`gate.drive.advance`, `gate.drive.handoff`, `gate.drive.claim`, `gate.drive.prepare-scope`, and
`gate.drive.takeover` (resolve each argv from the capability catalog). Each op is
one short call that advances the same durable drive by **at most one slice** and returns the shared
protocol-v1 outcome document (the same document the in-process app seam returns, never a re-flattened
copy):

| Operation | What it does |
|---|---|
| `start` | Fingerprint the execution context, launch the first raw run through the supervisor, advance one slice, and return the drive id, owner generation, and disposition. Optional `--scope-id <id> --child-cap <token> --gate-context <token>` bind the new drive into a recovery scope. |
| `advance` | Resume the current attempt of a drive (by opaque drive id + owner generation) through one more slice. |
| `handoff` | Prove current ownership, revalidate repository + process identity, invalidate the current owner, and mint a **single-use** handoff token — the only way a departing owner transfers a live drive. |
| `claim` | Recompute identity, consume a handoff token with a compare-and-swap, and return a **fresh** owner generation the claimant advances with. |
| `prepare-scope` | `--change-id <id> --task-id <id> --phase <name> --branch <name> --worktree <dir> [--gate-context <token>]`: mint a recovery scope for one parent/child dispatch boundary with **separated** parent and child capabilities. The preparing parent keeps the parent capability; the child receives only the scope id and child capability. Effects: local-write. |
| `takeover` | `--scope-id <id> --parent-cap <token> [--drive-id <id>]`: the event-authorized exceptional transfer — prove the parent capability and scope identity, atomically supersede the child's owner generation, and return a fresh generation. Effects: local-write. |

Every op takes **opaque** drive and claim identifiers — never a PID, PGID, raw run-directory state,
or deadline. `--json` emits the shared document; human text names identity and disposition only. An
invocation that cannot parse its arguments or read a recognized drive record is a **command
failure**, distinct from a recognized workflow disposition; a recognized `FAILED` or `HALTED` drive
is a workflow result, not an excuse to omit the document.

## The disposition vocabulary and what each earns

Every successful `start` or `advance` returns exactly one of four dispositions. The caller keys on
`.outcome`, never on process exit status:

| Disposition | Meaning | Permitted caller action |
|---|---|---|
| `WAITING` | The same drive is live and safe to continue, but this slice ended. | The current owner `advance`s again, or `handoff`s before it returns. |
| `PASSED` | The suite completed green against the recorded execution identity. | Consume the raw run dir the document exposes for evidence, or continue the task phase. |
| `FAILED` | The suite itself completed red and produced a trustworthy terminal record. | Enter the existing bounded repair policy. |
| `HALTED` | Safe automatic continuation is impossible — identity drift, uncertain ownership, deadline expiry, malformed state, or an unadmitted death. | Stop automation, retain diagnostics, surface the typed cause. |

- **`WAITING` is the only nonterminal disposition, and it is not permission to replace an agent.** A
  plain `WAITING` leaves the current owner generation valid; another agent cannot claim a drive
  merely because it can see the suite running. The current owner either `advance`s again or performs
  an explicit `handoff`.
- **`FAILED` is the ONLY disposition that feeds repair.** Process death, malformed state, deadline
  expiry, identity uncertainty, and handoff mismatch are HALTED — never converted into `FAILED`, so
  an unfinished or ambiguous run never manufactures repair work.
- **Only `PASSED` exposes the raw run dir**, so only a trusted pass can feed the evidence operation.

## Handoff — the only ownership transfer

Before an owner returns control while a drive is still live, it MUST call `handoff` and then perform
no further work on that drive. `handoff` recomputes the repository fingerprint, records the workflow
phase, invalidates the old owner token, and writes a single-use receipt. A fresh owner calls `claim`
with the drive id and the handoff token; exact-fingerprint validation and compare-and-swap
consumption make **only one** claimant authoritative. A claimant that loses the race or no longer
fingerprint-matches acquires **no** partial authority. Dirty pre-commit task work is a supported
handoff state — staged, unstaged, untracked, mode, rename, deletion, and symlink differences are all
part of the identity and must match exactly at claim time; **no** WIP commit is created to move
ownership.

A departing owner's structured report therefore names the drive id, the workflow phase, and the
opaque **handoff token** — the continuation the next owner claims. A bare "still waiting" with no
handoff token is not a valid departure: the drive would be stranded with a live owner generation
nobody holds.

## Parent takeover — the event-authorized exception

Normal `handoff` remains the **preferred** transfer. `takeover` exists only for a **direct child
that returned without handing off** while its scope still binds a nonterminal (or terminal-unconsumed)
drive. The parent proves its parent capability and the scope identity; the transition atomically
**supersedes** the child's owner generation and mints a fresh one the parent advances with, so a
stale child call thereafter fails **owner-superseded**. The authorization is the observed
dispatch-return event the caller just saw — never a timer, heartbeat, or quiet log. Any ambiguity —
two candidate drives, an outstanding unclaimed handoff (`claim` it instead), or identity drift —
fails closed to `HALTED`, never a partial transfer.

## The raw verbs are primitive/operator APIs, not caller-loop verbs

The five raw verbs — `gate.launch`, `gate.observe`, `gate.stop`,
`gate.recover`, and `gate.cleanup` — retain their narrow primitive meanings and remain
callable by the **driver implementation, primitive-level tests, diagnostics, recovery, cleanup, and
operator workflows**. They are **not** high-level workflow APIs. A workflow caller never composes
them directly and never recreates the retired observe/sleep loop — every build task worker, the
build controller's final gate, implement-next's evidence re-mint and re-gates, and finalize's local
gate drive the gate through the `gate.drive` operations above instead. The raw verbs are
documented as primitives in the operator-facing gate documentation, not here.
