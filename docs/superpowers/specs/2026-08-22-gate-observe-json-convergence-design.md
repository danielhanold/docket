<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0338 — Gate observe ships two serializations (shell state=name vs native protocol-v1 JSON) reconciled only by prose — converge on JSON, migrate the caller loop, retire the text contract](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-23-0338-gate-execution-terminal-sentinel-has-no-format-contract-poll.md)**
<!-- docket:backlink:end -->

# Gate observe — converge on protocol-v1 JSON, retire the text contract (change 0338)

## Problem

The gate's "done yet?" observation ships in two serializations for one operation:

- `"${DOCKET_SCRIPTS_DIR}"/docket.sh gate-run --observe` prints a plain-text `state=<name>` line
  (contract: `scripts/gate-run.md`), which the canonical caller loop parses today.
- The native Go-v1 gate `docket gate observe <run-dir>` (`internal/app/gate.go`) emits a
  protocol-v1 JSON document with `"state"` (and `"cause"`, omitempty) fields.

Only a prose sentence in `skills/docket-build/SKILL.md` keeps callers on the text format, and it
demonstrably failed during the change 0337 incident: a worker pointed the `state=<name>` loop at
the JSON emitter, every answer was unrecognized, and the poll spun until a human resumed it.

Human decision (Daniel, 2026-08-22): **protocol-v1 JSON is canonical; the text observe contract
retires.** This change closes the serialization seam only; retiring the rest of the `gate-run.sh`
facade is change 0339 (`depends_on: [338]`).

## Design decisions (settled during grooming)

1. **Observe path** — the caller loop invokes the native gate directly:
   `docket gate observe <run-dir>`. No facade passthrough shim.
2. **Facade fate** — `gate-run.sh --observe` is hard-retired in this change: it exits non-zero
   with a one-line stderr pointer to `docket gate observe`. The launch/liveness/stop verbs are
   untouched (0339's scope). The text serialization ceases to exist the moment this lands.
3. **Parse method** — the loop parses the JSON with **jq**, which becomes a documented required
   dependency of the caller loop. Rationale: jq is already a working docket dependency
   (`docket-status.sh`, `ensure-docket-env.sh`, `ensure-claude-settings.sh`); an exit-code-only
   vocabulary was rejected because it can carry nothing beyond a single number; a hand-rolled
   grep extraction re-creates the drift class this change kills; a Go-side `--state-only` flag
   would mint a second serialization of the same operation.

## The new canonical caller loop (`scripts/gate-run.md` § "The caller's loop")

Shape (normative; exact prose lands in `gate-run.md`):

- Each iteration runs `doc="$(docket gate observe "$run_dir")"` — capture into a variable first,
  never `producer | early-exiting-consumer` (pipefail house rule).
- Extract `state="$(jq -r '.state // empty' <<<"$doc")"` and, on `died`,
  `cause="$(jq -r '.cause // empty' <<<"$doc")"`.
- State vocabulary and arm semantics are unchanged from the 0286 loop:
  - `running` — the only retryable state; poll again within the observation budget.
  - `passed` / `failed` / `died` / `stopped` / `unavailable` — terminal; break with that verdict.
  - **Anything else is the fail-closed arm**: empty state, an unparseable document, a jq
    invocation failure, or jq absent from `PATH` all resolve to `unavailable` and break with a
    loud diagnostic — never a poll-again condition. A missing jq must surface as a named
    diagnostic ("jq not found — the gate observe loop requires it"), not as a silent spin.

## File-by-file scope

- `internal/app/gate.go` / `internal/cli/gate_test.go` — no shape change expected; the existing
  JSON (`state`, `cause`) already carries what the loop needs. Go-side coverage of the document
  shape stays where it is.
- `scripts/gate-run.sh` — `--observe` refuses: non-zero exit, stderr pointer to
  `docket gate observe`. All other verbs byte-identical in behavior.
- `scripts/gate-run.md` — the `--observe` row of the exit table and the `state=<state>` output
  contract are removed; "The caller's loop" section rewritten per above; jq documented as a
  required dependency of the loop; the `--observe` refusal documented.
- `skills/docket-build/SKILL.md` — the reconciling prose sentence ("key on `state=<name>`, not
  the native JSON") is deleted; every `state=<name>` reference updates to the JSON vocabulary and
  the native-gate invocation.
- `tests/test_gate_run.sh` — retargeted: (a) assert `--observe` refuses with the pointer;
  (b) a loop-shaped regression drives the documented jq extraction against real native-gate JSON
  through each terminal state; (c) mutation-test the fail-closed arm — feed an unknown/garbled
  document and a simulated jq-absent PATH, assert the loop breaks `unavailable` rather than
  polling. Guards must redden when the arm is stripped (guards-are-code rule).

## Out of scope

- Retiring `gate-run.sh`'s launch/liveness/stop machinery and the
  `scripts/lib/docket-liveness.sh` seam shared with `runner-dispatch.sh` — change 0339.
- Any change to the protocol-v1 document schema.
- Change 0264 (forked-mode launch shape) and ADR-0024 — explicit non-adjacencies.

## Testing summary

`tests/test_gate_run.sh` (retargeted as above) plus the untouched `internal/cli/gate_test.go`;
full suite at the build gate via `scripts/run-tests.sh` per `finalize.test_command`.
