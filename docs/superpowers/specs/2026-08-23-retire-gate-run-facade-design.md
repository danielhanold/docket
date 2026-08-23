<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0339 — Retire the gate-run.sh launch/liveness/stop facade now that the native Go-v1 gate is canonical (collapse the shared docket-liveness.sh seam with runner-dispatch.sh)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0339-retire-the-gate-run-sh-launch-liveness-stop-facade-now-that.md)**
<!-- docket:backlink:end -->

# Retire the gate-run.sh launch/liveness/stop facade — design

**Change:** 0339 · **Date:** 2026-08-23 · **Type:** refactor

## Problem

Docket carries two implementations of "launch a long-running child detached, check its liveness,
stop it": the native Go-v1 gate (`docket gate launch|observe|stop|recover`, change 0314 —
`internal/app/gate.go` over the `internal/process` supervisor) and the shell facade
`scripts/gate-run.sh`. Change 0338 retired the facade's `--observe` verb (it now refuses with a
pointer to `docket gate observe --json`), leaving `--launch` and `--stop` as a second
launch/liveness/stop path held in agreement with the native gate only by convention. The shared
liveness predicate `docket_group_alive_and_ours` in `scripts/lib/docket-liveness.sh` (extracted by
change 0284 after real drift) ties `gate-run.sh` to its other consumer, `runner-dispatch.sh`.

`docket-finalize-change` and `docket-implement-next` already run the native gate end-to-end.
`docket-build`'s SKILL.md is the last skill-level caller of `gate-run --launch`/`--stop`.
`runner-dispatch.sh` never calls `gate-run.sh` — it only shares the liveness lib.

## Decisions (settled during grooming)

1. **Full retirement, no wrapper.** `scripts/gate-run.sh` and `scripts/gate-run.md` are deleted;
   the `gate-run` entry leaves `WRAPPED_OPS` in `scripts/docket.sh` (and its header comment line),
   so `docket.sh gate-run` fails like any unknown op. No passthrough shim and no deprecation
   window — the same posture 0338 took for `--observe`. A shell entry point kept "for environments
   without the Go binary" was rejected: it preserves exactly the two-spellings drift class 0338
   killed, and the native gate is proven in production.
2. **`runner-dispatch.sh` stays on its shell liveness path.** Migrating the runner-delegation
   subsystem (change 0079; ~1,600 lines with its own launch records, adapters, and detach tests)
   onto the native supervisor is a separate future change, not part of 0339. The stub's boundary
   statement already draws this line.
3. **The liveness lib is kept, single-consumer** (option A). `scripts/lib/docket-liveness.sh`
   remains with `runner-dispatch.sh` as its sole consumer; only its ownership prose changes. The
   fold-back-inline alternative was rejected as churn with no drift left to prevent — if a future
   change migrates runner-dispatch natively, the lib dies then, with 0339 having spent nothing on
   it.
4. **Orphaned caller guidance moves to `skills/docket-build/references/gate-execution.md`**
   (option A). Minting a new contract doc for the native command (e.g. `scripts/docket-gate.md`)
   was rejected: `scripts/*.md` files document `.sh` siblings, and a native-command doc family is
   a bigger structural decision than this cleanup should smuggle in. `gate-execution.md` is
   already the shared gate-posture reference both `docket-build` and `docket-finalize-change`
   point at.

## What changes

### Retirement

- Delete `scripts/gate-run.sh` and `scripts/gate-run.md`.
- Remove `gate-run` from `WRAPPED_OPS` and the facade header comment in `scripts/docket.sh`.
- Call sites and cross-references are derived by whole-repo grep at build time (house rule: never
  hand-list gated sites), sorted into executable vs prose vs frozen. Archived changes, results
  files, and merged plans mentioning `gate-run` are point-in-time records and stay untouched.

### Caller migration — docket-build

Rewrite the "shipped implementation of clauses 1–3" passage of `skills/docket-build/SKILL.md` to
the native verbs:

- Launch: `docket gate launch --root <dir> --cwd <dir> -- <command…>`.
- Stop: `docket gate stop <run-dir>`.
- The shell token vocabulary dies with the script: the `launch-failed` stdout token becomes
  reading the launch's protocol-v1 JSON envelope with jq (jq is already the documented caller-loop
  dependency since 0338); the `--stop` token set (`stopped` / `already-terminal` / `unavailable`)
  becomes the native stop's JSON result. Exact field spellings are read from the protocol-v1
  envelope at implementation time, never restated from memory.
- The died-state posture (stop, then at most one bounded relaunch; `died` is never a red suite)
  keeps its meaning, re-keyed on the native vocabulary.
- The "reuse the canonical loop" pointer retargets from `gate-run.md` to `gate-execution.md`.

Finalize and implement-next are already native; they only get the doc-pointer retarget below.

### New home for orphaned guidance — gate-execution.md

`skills/docket-build/references/gate-execution.md` gains, moved from `gate-run.md`:

- **The canonical caller's loop** — already parses `docket gate observe` JSON since 0338, so this
  is a move plus native-spelling retarget, not a redesign.
- **The state vocabulary and retryability rule** — the observed states and "only `running` is
  retryable."
- **The per-platform capability note** — what detachment delivers per platform, verbatim intent
  preserved.
- **An evidence-carryover note:** the per-harness verdicts were measured against `gate-run.sh`'s
  launch shape; the native launcher performs the same Setsid session-leader detachment and
  establishment handshake (`internal/process/launch.go`), so the capability verdicts carry over
  without re-probing — and that carryover is recorded on the page rather than left silent. The
  "shipped mitigation" paragraph now names `docket gate launch`.

Every cross-reference to `gate-run.md` (docket-build SKILL, docket-finalize-change SKILL and its
`references/gate-failure.md`, docket-convention, `check-test-source-hygiene` if the grep finds it)
retargets to the new home.

### Liveness lib ownership prose

- `scripts/lib/docket-liveness.sh` header: rewrite "shared with gate-run.sh" to name
  `runner-dispatch.sh` as sole consumer (the *shared-resource-keeps-first-owner-assumptions*
  learning is exactly this edit).
- Same rewrite in `scripts/runner-dispatch.sh` comments and `scripts/runner-dispatch.md` prose.

### Tests

- `tests/test_docket_liveness.sh` — unchanged.
- `tests/test_gate_run.sh`, `tests/test_gate_run_stop.sh` — deleted, **after** sorting what each
  block guards (the *test-premise-deleted-not-regated* learning): subject-mechanics guards die
  with the script (native equivalents live in `internal/process` / `internal/cli` Go tests); any
  guard on caller-loop shape or posture prose moves with the loop, `test_gate_execution_posture.sh`
  being the natural carrier.
- `tests/test_gate_execution_posture.sh` — retargeted to the moved guidance and the native
  spellings.
- The embedded-assets tree (`internal/assets/embedded/`) regenerates at build; the suite runs per
  `finalize.test_command` at the build gate.

## Out of scope

- Migrating `runner-dispatch.sh` onto the native supervisor — a future change.
- Any change to the protocol-v1 document schema.
- Re-probing the per-harness gate-execution verdicts (the carryover note covers them).
- The forked-mode launch question (change 0264) and ADR-0024 — explicit non-adjacencies, as in
  0338.

## Risks

- **Prose drift sweep is the real surface.** The mechanical deletion is small; the risk is a
  missed `gate-run` reference in maintained source. Mitigation: derive the site list from a
  whole-repo grep and sort every hit explicitly (executable / maintained prose / frozen record).
- **Deleted-test guard loss.** Mitigated by the sort-before-delete step above.
