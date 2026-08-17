<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0314 — Native process supervisor and local gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-17-0314-native-process-supervisor-and-local-gate.md)**
<!-- docket:backlink:end -->

# Native process supervisor and local gate

**Change:** 0314 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-16 · **Status:**
Approved focused design

## Purpose and boundary

This change replaces the lossy process mechanics behind Bash `gate-run` with a native Go
supervisor that can outlive its launching foreground call on both Darwin and Linux, retain separate
durable output streams, distinguish an ordinary exit from signal termination exactly, and stop only
a process group whose Docket ownership is still provable.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are fixed upstream constraints.
This spec resolves only change 0314's local process-supervision package, one-shot gate operations,
durable run-state contract, native Darwin/Linux process boundary, and abandoned-supervisor
recovery. It does not reopen the one-binary, one-shot JSON protocol, agent-first, no-daemon,
no-interpreter, repository-compatibility, or hard-cutover decisions.

The Go implementation does not modify or retire `scripts/gate-run.sh`, its Bash tests, the shipped
build skills, or the four-harness evidence. Those surfaces remain the active `v0.9.2` product until
change 0318 performs the hard cutover. Change 0264's harness measurement remains independent and
useful; this change neither runs that probe nor rewrites its verdict.

## Landed foundation and independently deliverable result

Change 0304 is complete on `main` and supplies the only predecessor contracts this change consumes:

- one `docket` executable with a single process exit site;
- the protocol-v1 `app.Envelope`, closed `app.Result` taxonomy, coarse `app.ExitCode` mapping, and
  one-document JSON presentation rule;
- Cobra-based command registration through `internal/cli`; and
- the baseline Go test/build conventions and Darwin/Linux amd64/arm64 target set.

No later migration package is a dependency. In particular, this change does not import or extend
configuration (0305), persistent documents (0306), domain policy (0307), Git (0308), metadata
transactions (0309), status (0310), installation or harness rendering (0311), planning mutations
(0312), or feature workspaces, pull requests, and build evidence (0313). An explicit run root and
working directory keep supervision repository-independent; later workflows supply those values.

Change 0314's independently reviewable deliverable is:

- a new `internal/process` package with typed launch, observe, stop, and recover operations;
- a public `docket gate` CLI group that exposes those operations through protocol-v1 result
  documents and human text;
- a package-private re-execution mode in the same `docket` binary that becomes the durable
  supervisor, establishes the Unix session, starts and waits for the gate command, and writes the
  exact terminal record;
- private, atomically updated per-run state with a random ownership token and kernel-backed live
  lock;
- identity-safe, bounded process-group termination and owned abandoned-run recovery; and
- real-process tests that prove the contract on Darwin and Linux.

No metadata mutation, feature-branch transition, PR publication, results attachment, agent
dispatch, repair decision, or workflow retry policy is delivered here. A human can launch, observe,
stop, and recover a standalone local gate; change 0315 later composes those mechanics into the
claim-to-implemented workflow.

## Chosen architecture

Re-execute the current `docket` binary as a narrow supervisor subprocess in a new Unix session:

```text
docket gate launch
  |
  | allocate private run dir + ownership token + held live lock
  | exec the same docket binary in internal supervisor mode, Setsid=true
  v
supervisor (session leader and process-group leader)
  |
  | publish addressable identity and complete launch handshake
  | open stdout.log / stderr.log; close child stdin
  | start command as argv, without a shell, inside the supervisor's group
  | wait and decode native WaitStatus
  v
atomic terminal record

docket gate observe / stop / recover
  |
  +--> validate manifest + token + live lock + live pid/pgid/sid facts
  +--> read or mutate only this run's owned state
```

This design preserves the approved one-binary distribution while giving the exact waiter a lifetime
independent of the CLI call that launched it. The supervisor is per-run, not a service: it has no
socket, discovery registry, shared memory, cache, or cross-repository routing, and it exits after its
one direct child has terminated and the terminal record is durable.

Three alternatives are rejected:

1. **Ship a second helper executable.** It could wait exactly, but it would contradict the approved
   one-binary artifact, create binary/helper version skew, and pull packaging work from 0317 into
   this change.
2. **Launch the gate directly from the public CLI and keep that process waiting.** It has exact
   status but remains bounded by the foreground call's lifetime, failing the core survival
   requirement.
3. **Detach the gate directly and infer its result later from a PID, log, or conventional
   `128+signal` code.** No surviving authoritative waiter exists, so it recreates the ambiguity and
   PID-reuse hazards this change exists to remove.

Go code must not call `fork` and continue running the multithreaded Go runtime in the child. The
public launch process uses `os/exec` to start a fresh copy of the current executable with native
session attributes; only normal exec boundaries start new Go processes.

## Package and dependency boundaries

The intended direction is:

```text
internal/cli  -->  internal/app  -->  internal/process  -->  Go stdlib / OS syscalls
```

- `internal/process` owns run identities, files, locking, process launch/wait, liveness
  classification, signalling, and recovery. It imports no app, CLI, config, Git, repository,
  workspace, evidence, install, harness, or domain package.
- `internal/app` owns operation names, protocol DTOs, `app.Result` mapping, safe human text, and the
  thin orchestration calls into `internal/process`.
- `internal/cli` parses flags and the `-- <argv...>` boundary, constructs requests, and presents the
  complete application result. It contains no process-state policy.
- Darwin and Linux session, process-group, wait-status, and signal primitives sit behind a small
  package-private platform interface. Shared state machines and record codecs do not branch on
  `runtime.GOOS`.
- No generic exported subprocess runner is introduced. The process package supervises caller
  commands only through the gate operations defined here; Git and `gh` stay in their existing typed
  adapters.

The current executable path is resolved by the application at launch and passed into the process
service as an explicit dependency. Tests substitute a purpose-built helper process without
changing production command construction.

## Public gate operations

The public CLI surface is:

```text
docket gate launch --root <absolute-dir> --cwd <absolute-dir> -- <command> [args...]
docket gate observe <absolute-run-dir>
docket gate stop <absolute-run-dir> [--reason <text>]
docket gate recover --root <absolute-dir>
```

The operation names in protocol envelopes are `gate.launch`, `gate.observe`, `gate.stop`, and
`gate.recover`. `gate` by itself is a command group and reports missing-command input like the
existing `diagnostic` and `development` groups.

`--root` is required rather than defaulting to a global directory or silently discovering a
repository. The caller chooses the durable ownership boundary and is responsible for its retention
policy. `--cwd` is also required and used verbatim after absolute-path and directory validation;
this package never discovers or validates a Docket feature workspace. Both choices keep repository
and workflow behavior in 0313 and 0315.

The command after `--` is required, preserved as an argument array, and executed directly. No shell
string, interpolation, command-file parser, or environment mini-language is introduced. The child
inherits the launching process's environment, with only supervisor-private variables and file
descriptors removed before exec. Its stdin is `/dev/null`.

Every operation supports the root command's existing `--json` transport. JSON writes exactly one
protocol-v1 document to stdout; human mode writes stable readable text. Unexpected supervisor
diagnostics are durable in the run directory and never leak a second document onto stdout.

## Result and state vocabulary

Within `internal/app`, one gate result DTO carries the envelope plus the facts applicable to its
operation:

```go
type GateResult struct {
    Envelope
    RunID       string          `json:"run_id,omitempty"`
    RunDir      string          `json:"run_dir,omitempty"`
    State       GateState       `json:"state,omitempty"`
    ExitCode    *int            `json:"exit_code,omitempty"`
    Signal      *int            `json:"signal,omitempty"`
    Cause       string          `json:"cause,omitempty"`
    StdoutLog   string          `json:"stdout_log,omitempty"`
    StderrLog   string          `json:"stderr_log,omitempty"`
    Reason      string          `json:"reason,omitempty"`
    Recovery    []RecoveryEntry `json:"recovery,omitempty"`
}
```

The exact Go split may follow package conventions, but protocol meanings are fixed:

| State | Meaning |
|---|---|
| `running` | the supervisor lock is held and its recorded pid/pgid/sid identity is live and valid |
| `passed` | the direct child exited normally with code 0 |
| `failed` | the direct child exited normally with a non-zero code |
| `signaled` | the direct child was terminated by the exact recorded signal, without a Docket stop intent |
| `stopped` | Docket deliberately stopped the run and verified its recorded group gone |
| `vanished` | the supervisor is gone and no exact terminal record or completed stop exists |

`unavailable` is not a fabricated run state. An unreadable run, malformed owned record,
unrecognized schema, unprovable live identity, or failed syscall is an operation failure with a
stable `reason`; it never guesses a terminal state.

Application result mapping is operation-sensitive:

- a successful launch, a running observation, or a `passed` observation is `applied`;
- a `failed` observation is `gate-failed`, with the exact `exit_code`;
- a `signaled`, `stopped`, or `vanished` observation is `interrupted`;
- a stop that sent a signal and verified the group gone is `applied`;
- a stop that finds an already-terminal run is `no-op` and preserves that terminal state;
- a valid run whose ownership or termination cannot be proved is `blocked`;
- malformed state is `invalid-state`, bad arguments are `invalid-input`, and filesystem or process
  failures are `external-failed`.

JSON consumers decide from `result`, `state`, and the typed numeric fields, never by parsing human
text or relying on the coarse process exit code. Nil collections normalize to `[]` on every path,
following the landed status-result convention.

## Durable run state and ownership

Each launch allocates 128 bits of cryptographic randomness and encodes it as a lowercase-hex run ID
and independent ownership token. The run directory is a direct child of the explicit root:

```text
<root>/
  registry.lock
  <run-id>/                     # 0700
    live.lock                   # 0600; held for the supervisor's lifetime
    manifest.json               # atomic; allocation/session/process identity and phase
    stdout.log                  # 0600; child stdout only
    stderr.log                  # 0600; child stderr only
    supervisor.log              # 0600; bounded internal diagnostics
    terminal.json               # atomic; written only by the exact waiter
    stop-intent.json            # atomic; written immediately before the first signal
    stopped.json                # atomic; only after group termination is verified
    abandoned.json              # atomic; recovery-only, never a child verdict
```

The root and every run directory are explicitly `chmod`ed to `0700`; files are explicitly
`chmod`ed to `0600`. Creation modes alone are insufficient because umask can mask a promised mode.
Every structured write uses a same-directory temporary file, file sync, atomic rename, and
directory sync. A reader sees a complete old or new record, never a partial JSON document.

The versioned manifest records at least:

- schema version, run ID, and random ownership token;
- canonical root and run-directory identity;
- supervisor PID, process-group ID, and session ID;
- phase (`allocated`, `established`, `running`, or `terminal`);
- the explicit working directory;
- creation and update timestamps for diagnostics; and
- a safe command descriptor that does not persist environment values or require reconstructing the
  argv for correctness.

The manifest is provenance, never proof by itself. A PID, pathname, timestamp, conventional
directory name, or manifest field alone authorizes no signal or removal. The ownership conjunction
for a live run is:

1. the requested run directory is an immediate, non-symlink child of the canonical explicit root;
2. its valid manifest's run ID and token agree with the directory's allocation record;
3. `live.lock` is held by the still-running supervisor;
4. the recorded PID is greater than 1 and still equals both its live process-group and session
   leader IDs; and
5. the recorded group is not the observer's own group.

The supervisor is the only process that retains the live-lock descriptor. It is not inherited by
the user command. Once the supervisor exits, the kernel releases the lock; a later process can reuse
its PID, but the now-free lock prevents that reused identity from passing the ownership conjunction.
The random token prevents one owned directory or handle from being mistaken for another.

The root registry lock covers only allocation/publication and recovery inventory. It is never held
for the duration of a gate. No manifest becomes visible before a live lock is already held, so
recovery cannot misclassify a half-published launch as abandoned.

## Launch and establishment

Launch is an ordered state machine:

1. Validate all inputs before creating the root or run directory. Reject relative paths, a missing
   command, a non-directory working path, an unsafe root, symlinks at the run slot, and unsupported
   platform behavior as `invalid-input` or `external-failed` without starting a process.
2. Under the root registry lock, allocate the private run directory, acquire `live.lock`, and
   publish the `allocated` manifest. The lock descriptor is passed to the new supervisor process;
   the public launcher closes its copy only after spawn.
3. Start the same executable in package-private supervisor mode with stdin closed and all of its
   own streams detached from the foreground call. Native process attributes create a new Unix
   session; the supervisor must prove `pid == pgid == sid` before publishing `established`.
4. Publish the addressable session identity before starting the user's command. From this point a
   failed launch can safely target the recorded group if and only if the ownership conjunction
   still holds.
5. Open the two child log files, start the command in the supervisor's process group, and atomically
   publish `running`. The command is never started before the group that contains it is durably
   addressable.
6. The public launcher waits only for the bounded establishment handshake: `running`, an exact
   terminal record from a fast command, or a typed launch failure. It never waits for the gate's
   ordinary duration.

The production establishment bound remains ten seconds, preserving the retained safety posture
without adding a configuration key. Tests receive deterministic synchronization seams rather than
sleep-tuning against that bound.

If establishment fails, launch performs a bounded ownership-checked teardown. Where the full
ownership conjunction cannot be established, nothing group-directed is signalled; the run directory
and supervisor diagnostic remain, and the result loudly identifies the run as blocked or externally
failed. Launch never reports a usable handle while an unaddressable command might still start.

## Native supervisor and exact terminal status

The supervisor is the session and process-group leader. The user command remains in that group so a
single negative-PGID signal reaches the command and ordinary descendants that have not deliberately
escaped the group. The supervisor itself catches the graceful termination signal and remains alive
long enough to wait and record; the child begins with the default termination disposition so the
same group-directed signal still terminates it. Real-process tests, not source-shape assertions,
prove both halves.

The supervisor waits for the direct child and decodes the native wait status:

- normal exit writes `kind: "exit"` and the exact exit code;
- signal termination writes `kind: "signal"` and the exact signal number; and
- command-start failure writes a distinct supervisor failure record, never a fabricated terminal
  child record.

`terminal.json` is written only by this exact waiter, only after the child has terminated, and
before the supervisor releases its live lock. No public verb synthesizes a terminal record. A
genuine `exit 143` therefore remains `kind=exit, code=143`, while `SIGTERM` remains
`kind=signal, signal=15`; no `128+signal` heuristic appears anywhere.

The direct child's terminal status outranks later liveness observations. Deliberately escaped
descendants are outside the recorded group and cannot be safely signalled; this retained residual
is documented rather than widened into process-tree discovery or platform-specific global scans.

## Observation

Observe is short-lived, idempotent, and read-only. Its order is load-bearing:

1. Validate the run path, manifest, token, and record shapes.
2. Read `terminal.json` first. A valid terminal record decides `passed`, `failed`, `signaled`, or
   `stopped` according to exact status and the stop markers.
3. With no terminal record, probe the live lock and recorded process facts. A held lock plus the
   complete live identity conjunction decides `running`.
4. A cleanly free lock means the supervisor is gone. Re-read `terminal.json` after that probe so a
   terminal write racing the first read wins rather than being misreported as disappearance.
5. With the supervisor gone and still no terminal, a completed stop decides `stopped`; otherwise
   the run is `vanished`, with an `abandoned.json` marker supplying recovery detail when present.

Probe outcomes are three-way: live, cleanly gone, and unprovable. Only the operating system's clean
not-found answer is evidence of absence. Permission failures, malformed records, unexpected syscall
errors, or identity disagreement are unprovable and produce `blocked`, `invalid-state`, or
`external-failed`; they never become `running`, `vanished`, or permission to signal.

Logs are not a completion oracle. Observe may return their paths and bounded safe diagnostic detail,
but it never searches output for success text and never turns a partial log into a verdict.

## Stop and bounded group termination

Stop is idempotent and preserves a child's own verdict when one exists:

1. Validate owned state and read the terminal record first. An already-terminal run returns `no-op`
   with its existing state.
2. With no terminal record, require the full ownership conjunction immediately before signalling.
   A free lock or unprovable identity triggers a terminal re-read and then a safe refusal; neither
   authorizes a signal.
3. Atomically write `stop-intent.json`, flattening the optional human reason to a bounded safe
   string, then send `TERM` to the negative recorded PGID.
4. Wait up to ten seconds for an exact terminal record and for the group to disappear.
5. Before escalation, re-prove that the supervisor still holds the live lock and still leads the
   recorded session/group. If it does, send `KILL` to the group and spend up to five further seconds
   verifying absence. If ownership became unprovable, do not escalate; return `blocked` and retain
   every diagnostic.
6. Re-read the terminal record after verified teardown. A normal exit remains `passed` or `failed`;
   a signal record following the stop intent is `stopped`. If `KILL` also killed the supervisor and
   no terminal record can exist, atomically write `stopped.json` only after group absence was
   verified.

Once stop has written its intent and sent the first signal, it completes this fixed bounded sequence
even if the invoking context is cancelled; abandoning halfway would leave an unobserved live gate.
Before the first signal, cancellation performs no process mutation and returns `interrupted`.

`stop` never writes `terminal.json`. An external signal that lands after the stop intent is recorded
is classified as the requested stop, preserving the retained conservative bias. A command that
deliberately creates a new session escapes the recorded group; stop reports only what it can verify
about the owned group and never broadens signalling to a process discovered by pathname or command
line.

## Abandoned-supervisor recovery

`gate.recover` is deliberately narrower than change 0316's workflow recovery and cleanup. It scans
one explicitly supplied run root and stabilizes only local supervisor state:

- take a registry snapshot of immediate, non-symlink candidate directories;
- validate each ownership manifest and token before treating it as Docket state;
- leave a held live lock untouched and report the run as live;
- for an unlocked owned run, re-read terminal and stop records;
- when no terminal/stop exists and the recorded group is cleanly absent, atomically write
  `abandoned.json` so future observations carry a stable disappearance cause; and
- when a group still exists or its identity is unprovable after the lock is free, retain the
  directory, signal nothing, and report it as needing inspection.

Foreign directories, unknown schemas, symlinks, unreadable candidates, token mismatches, and
ambiguous live process names are reported and left byte-untouched. Recovery never deletes a run
directory or its logs. Retention policy, terminal run cleanup, and composition into maintenance or
finalize belong to change 0316; this change supplies truthful owned inventory and stable abandoned
state without pulling that lifecycle forward.

Recovery results are deterministic and sorted by run ID. One or more newly written abandonment
markers produces `applied`; a clean scan with nothing to change produces `no-op`; per-run blocked or
external findings remain structured entries rather than causing recovery to mutate the unsafe
candidate.

## Platform boundary

Darwin and Linux share the state machine and must meet the same contract. Platform files own only:

- starting the supervisor as a new session leader;
- reading live PID, process-group, and session relationships;
- sending a signal to one process group;
- interpreting the native child wait status; and
- classifying clean absence separately from syscall failure.

No runtime ladder, `uname` branch, `setsid(1)` discovery, pty, Bash job control, Python, Perl, or
shell wrapper remains in the Go path. Unsupported operating systems fail explicitly rather than
silently falling back to weaker detachment.

The contract is a genuine new Unix session on both supported operating systems. A platform-specific
test that merely compiles does not prove this; each target's real-process suite inspects the live
session/group facts and exercises group teardown.

## Failure, privacy, and diagnostic rules

- Free-form command arguments, environment values, and output bytes are never copied into protocol
  error messages. Logs stay private in the run directory.
- Persisted command description is bounded and diagnostic only; correctness never depends on
  reconstructing argv from it.
- Filesystem and syscall errors carry a stable stage/reason and bounded safe detail. Callers never
  parse operating-system prose.
- A malformed terminal record is `invalid-state`, not a guessed exit or signal.
- A supervisor-start failure is `external-failed`, not `gate-failed`; no gate verdict exists.
- A vanished supervisor is `interrupted`, never a red gate and never automatic permission for repair
  work. Change 0315 owns the later workflow disposition.
- The process package performs no network access and reads no Docket configuration.

## Testing strategy

Tests follow the repository's TDD and mutation discipline and use real helper processes wherever the
kernel contract is the subject.

### Pure and filesystem tests

- request validation for absolute roots/cwds, argv presence, path containment, symlink refusal, and
  reason bounds;
- manifest/terminal/stop/abandoned codec round trips and unknown-schema refusal;
- hostile-umask tests proving `0700` directories and `0600` files;
- atomic-record tests that repeatedly read during replacement and never observe partial JSON;
- token mismatch, foreign directory, malformed record, and unreadable-state classifications;
- deterministic recovery ordering and all live/free/unprovable lock branches; and
- boundary tests proving `internal/process` does not import migration packages owned by 0305–0313.

### Real-process contract tests

- the establishment handshake returns only after a live `pid == pgid == sid` supervisor and an
  addressable command group exist;
- the gate survives exit and process-group teardown of the initiating launcher;
- stdout and stderr remain byte-exact, separate, durable files and stdin is closed;
- normal exits 0, 7, and 143 produce exact exit records;
- `SIGTERM` produces an exact signal-15 record distinct from exit 143;
- a graceful stop reaches the child while the supervisor remains able to record, and a TERM-ignoring
  child reaches bounded KILL escalation;
- stopped markers appear only after verified group absence;
- fast exit during launch, terminal-write/read races, stop/exit races, supervisor death, and caller
  cancellation converge on the specified state;
- a free live lock plus a reused or mismatched PID/PGID never authorizes signalling;
- recovery marks a cleanly abandoned owned run and retains live, foreign, and unprovable runs; and
- deliberately escaped descendants document the bounded residual without expanding the signal
  target.

Synchronization uses pipes, barriers, and observable kernel state rather than arbitrary sleeps.
Production test hooks are package-private or explicit injected dependencies and are inert in normal
builds. Timing assertions verify completion within generous outer bounds while correctness rests on
state transitions, not machine-calibrated millisecond thresholds.

The real-process suite runs on both Darwin and Linux before the change is accepted. Cross-compiling
the other target is a useful build check but is not evidence that session creation, wait-status
decoding, or group termination works there. This requirement does not add release packaging,
four-harness acceptance, or a release workflow; those remain 0317.

The build gate is the whole resolved suite from `finalize.test_command`, not only the process
package tests. Every new safety guard has a negative test that removes or violates the guarded
condition and proves the relevant test reddens with Go test caching disabled.

## ADR action

Implementation records one new Accepted ADR owned by change 0314 that **supersedes ADR-0081**. Its
decision is that Go's native per-run supervisor establishes a genuine new session on Darwin and
Linux and reads the raw wait status, eliminating both ADR-0081's platform narrowing and the
shell-level exit/signal ambiguity without a discovered interpreter.

The new ADR relates to ADR-0080 and ADR-0087 but does not supersede either:

- ADR-0080 governs the separate delegated-agent boundary, which change 0314 does not alter.
- ADR-0087's distinction between clean absence and unprovable liveness remains applicable and is
  preserved by observe, stop, and recovery.

The ADR is minted at build time through the ordinary `docket-adr` workflow; this spec does not
preallocate its ID. ADR-0081 remains listed on the change so the supersession and eventual status
transition are delivered atomically with the implementation's ADR work.

## Explicit scope allocation

Change 0314 owns only the native gate mechanics above.

- **0305–0313:** no configuration resolution, document/domain/Git/transaction/status/install,
  planning, workspace, PR, or build-evidence behavior is recreated or widened here.
- **0315:** candidate context, claim/reconcile, agent dispatch, observation budgets, gate relaunch or
  repair policy, results attachment, run verification, PR association, and the transition to
  `implemented` remain downstream.
- **0316:** finalize/rebase/retest composition, workflow recovery, reclaim, archive, stacks,
  maintenance sweeps, and physical run-directory retention cleanup remain downstream.
- **0317:** release archives, checksums, target packaging, CI/release workflows, and four-harness
  acceptance remain downstream.
- **0318:** configuration contraction, self-hosting, Bash deletion, active skill rewiring,
  documentation replacement, and hard cutover remain downstream.

Also out of scope are a global daemon, sockets, a public Go API, Windows, CI gate polling,
shell/interpreter discovery, process-tree scanning outside the owned group, live-harness re-probes,
and changing the existing build-gate skill contract before 0315 composes the Go operations.

## Acceptance criteria

Change 0314 is complete when:

1. `docket gate launch` returns a private durable run handle only after a genuine new session and
   addressable process group are established.
2. The supervisor writes separate durable streams and one atomic exact terminal record as the sole
   waiter.
3. Real tests prove `exit 143` and signal 15 are different outcomes on Darwin and Linux.
4. Observe never infers completion from logs and distinguishes running, normal exit, signal death,
   deliberate stop, vanished supervisor, and unprovable state through typed protocol results.
5. Stop signals only a token-, lock-, and live-identity-proven group, escalates within fixed bounds,
   and writes no false terminal verdict.
6. Recovery marks only valid unlocked owned runs as abandoned, signals and deletes nothing, and
   leaves foreign or ambiguous state untouched.
7. Private modes, atomic writes, crash windows, response races, and guard mutations are covered by
   the full green suite.
8. A new ADR supersedes ADR-0081 without changing ADR-0080, change 0264, or any behavior allocated
   to changes 0305–0313 or 0315–0318.
