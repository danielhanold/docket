# Gate driver contract — the caller-side contract for driving the native gate

This reference is the **caller-side contract for driving the native gate**: the typed driver
operations a caller invokes, the disposition vocabulary those operations return, and the ownership
handoff a departing caller must perform.

Its direct callers are the build controller's full-suite gate (every counted attempt),
implement-next's evidence re-mint and re-gates, and finalize's re-gate of a repaired head.
Finalize's post-rebase gate reaches the same driver only through the `finalize.rebase` operation,
never a direct `gate.drive` call. A build-task worker is never a caller: it
runs its focused tests directly under a fixed time limit. A caller makes **short, slice-bounded,
synchronous** calls to the native gate **driver**, which composes the raw supervisor,
persists one deadline and one execution identity, and returns one of four typed dispositions per
call. No caller runs a shell polling script, backgrounds the suite, subscribes to a notification, or
authors its own liveness check — the driver owns all of that.

## The driver's operations

The high-level surface is the `gate.drive` operation group: `gate.drive.start`,
`gate.drive.advance`, `gate.drive.handoff`, and `gate.drive.claim` (resolve each argv from the
capability catalog). Each op is
one short call that advances the same durable drive by **at most one slice** and returns the shared
protocol-v1 outcome document (the same document the in-process app seam returns, never a re-flattened
copy):

| Operation | What it does |
|---|---|
| `start` | Fingerprint the execution context, launch the first raw run through the supervisor, advance one slice, and return the drive id, owner generation, and disposition. A build or implement-next caller passes `--repo-dir <worktree> --owner build --change-id <id> --run-root <dir> --json` (`--change-id` charges `build.max_attempts` and lets the run tracker match the drive), plus `--run-context <token>` when its prompt carried it; `--owner build` resolves the build-owned suite command from config, so the caller passes no suite argv. Finalize's repaired-head re-gate passes `--owner finalize` instead, which resolves `finalize.test_command` and charges no build attempt. |
| `advance` | Resume the current attempt of a drive (by opaque drive id + owner generation) through one more slice. |
| `handoff` | Prove current ownership, revalidate repository + process identity, invalidate the current owner, and mint a **single-use** handoff token — the only way a departing owner transfers a live drive. |
| `claim` | Recompute identity, consume a handoff token (conflict-checked), and return a **fresh** owner generation the claimant advances with. |

Every op takes **opaque** drive and claim identifiers — never a PID, PGID, raw run-directory state,
or deadline. `--json` emits the shared document; human text names identity and disposition only. An
invocation that cannot parse its arguments or read a recognized drive record is a **command
failure**, distinct from a recognized workflow disposition; a recognized `FAILED` or `HALTED` drive
is a workflow result, not an excuse to omit the document.

## JSON capture — the required transport for consumed results

Any invocation whose result a workflow consumes MUST pass `--json` and **capture** the emitted
protocol-v1 document from that same first response, then **validate** that the fields the caller
needs are present before acting on any of them:

| Operation | Required from the captured first response |
|---|---|
| `start` | the drive identifier **and** the ownership generation |
| `handoff` | the **single-use** handoff token |
| `claim` | the **fresh** owner generation |

Human-readable output can never substitute for the captured document: it names identity and
disposition only, deliberately omitting generations and tokens. Each token keeps its existing
meaning — nothing here widens handoff or claim authorization.

**Shell-safe capture names.** Capture the emitted document into an explicitly named,
non-reserved variable — `gate_reply` — and capture the invocation's exit code, when the
caller needs it, into `gate_rc` on the very next line. Both names behave identically in
zsh and bash. Never assign to a zsh read-only special parameter — `status` and
`pipestatus` are the two a capture site reaches for — because under zsh that assignment
aborts the caller before the captured response is parsed, stranding a live drive.

A missing, malformed, or incomplete required response is a **caller-contract failure**, not
permission to rerun `start` (or any sibling op) to recover credentials. The caller maps it to its
**existing** halt posture — the build controller and implement-next halt with the missing-response
reason — and never invents credentials, infers them from a drive id, or mints a
new recovery path.

## The disposition vocabulary and what each earns

Every successful `start` or `advance` returns exactly one of four dispositions. The caller keys on
`.outcome`, never on process exit status:

| Disposition | Meaning | Permitted caller action |
|---|---|---|
| `WAITING` | The same drive is live and safe to continue, but this slice ended. | The current owner `advance`s again, or `handoff`s before it returns. |
| `PASSED` | The suite completed green against the recorded execution identity. | Consume the raw run dir the document exposes for evidence. |
| `FAILED` | The suite itself completed red and produced a trustworthy terminal record. | Enter the existing repair policy, bounded by the build phase's configured suite-attempt budget (`build.max_attempts`), or for finalize's re-gate by `finalize.repair_max_attempts`. |
| `HALTED` | Safe automatic continuation is impossible — a changed worktree, uncertain ownership, deadline expiry, malformed state, or an unadmitted death. | Stop automation, retain diagnostics, surface the typed cause. |

- **`WAITING` is the only nonterminal disposition, and it is not permission to replace an agent.** A
  plain `WAITING` leaves the current owner generation valid; another agent cannot claim a drive
  merely because it can see the suite running. The current owner either `advance`s again or performs
  an explicit `handoff`.
- **`FAILED` is the ONLY disposition that feeds repair.** Process death, malformed state, deadline
  expiry, identity uncertainty, and handoff mismatch are HALTED — never converted into `FAILED`, so
  an unfinished or ambiguous run never manufactures repair work.
- **Only `PASSED` exposes the raw run dir**, so only a trusted pass can feed the evidence operation.
- **A `HALTED` document's `finding`, when present, is copied verbatim into the halt report** with
  the glossary's `tree-survives` remedy (wait until `pgrep -lg <pgid>` prints nothing before
  re-running); it is information only — it never changes the halt, never adds one, and never
  feeds repair.

## Worktree admission — one live gate per worktree, and what `worktree-busy` means

A canonical feature worktree carries **at most one** running gate at a time. Before
`gate.drive.start` launches anything, the driver takes that worktree's lock; the gate's
supervisor holds it for the gate's whole life, and the kernel releases it when the supervisor
exits — so a finished or killed gate frees the worktree on its own, with no recovery step. A start
against a worktree whose lock is held is **refused** `worktree-busy`: never queued, never silently
joined to the running gate, and it never stops that gate. The refusal names the holder (its drive
and change, or a raw run dir) only after confirming that gate is still running; otherwise it says
the holder is unknown.

`worktree-busy` is a **command failure**, distinct from the four dispositions above: the response
carries the bounded reason token and a next-action message, and exposes no drive document to
advance. A caller treats it as a **blocking diagnostic — not a retry trigger and not a `FAILED`
result** — that reserves no suite attempt and feeds no repair attempt. Map it to the caller's own halt
posture (the build controller halts per its *Halting conditions*). Freeing the worktree is the
operator's act — wait for the holding gate to finish, or stop it (`run.cancel` for a tracked run,
`gate.stop <run-dir>` for a raw launch) — never polling on `start`.

## Handoff — the only ownership transfer

Before an owner returns control while a drive is still live, it MUST call `handoff` and then perform
no further work on that drive. `handoff` recomputes the repository fingerprint, records the workflow
phase, invalidates the old owner token, and writes a single-use receipt. A fresh owner calls `claim`
with the drive id and the handoff token; exact-fingerprint validation and conflict-checked
consumption make **only one** claimant authoritative. A claimant that loses the race or no longer
fingerprint-matches acquires **no** partial authority. Staged, unstaged, untracked, mode, rename,
deletion, and symlink differences are all part of the identity and must match exactly at claim time;
**no** WIP commit is created to move ownership.

A departing owner's structured report therefore names the drive id, the workflow phase, and the
opaque **handoff token** — the continuation the next owner claims. A bare "still waiting" with no
handoff token is not a valid departure: the drive would be stranded with a live owner generation
nobody holds.

## The raw verbs are primitive/operator APIs, not caller verbs

The raw verbs — `gate.launch`, `gate.observe`, `gate.stop`,
`gate.recover`, and `gate.cleanup` — retain their narrow primitive meanings and remain
callable by the **driver implementation, primitive-level tests, diagnostics, recovery, cleanup, and
operator workflows**. They are **not** high-level workflow APIs. A workflow caller never composes
them directly and never recreates shell observe/sleep polling — the build controller's
full-suite gate, implement-next's evidence re-mint and re-gates, and finalize's re-gate of a
repaired head drive the gate through the
`gate.drive` operations above instead, and finalize's local gate goes through `finalize.rebase`. The raw verbs are
documented as primitives in the operator-facing gate documentation, not here.
