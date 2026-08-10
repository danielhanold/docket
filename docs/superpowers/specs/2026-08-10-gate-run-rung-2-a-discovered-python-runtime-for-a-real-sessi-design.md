<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0285 — gate-run rung 2 — a discovered Python runtime for a real session and an exact child status](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0285-gate-run-rung-2-a-discovered-python-runtime-for-a-real-sessi.md)**
<!-- docket:backlink:end -->

# gate-run rung 2 — a discovered Python runtime — design

Change: 0285. Designed 2026-08-10 in an **interactive brainstorm with the human** — not auto-groomed,
so no adversarial critic gated it. The audit trail is `## Assumptions` below; every assumption
records a decision the human took in that session, not a default the designer chose.

Depends on change 0282 reaching `done`: `scripts/gate-run.sh` exists only on that branch.

## Problem

`scripts/gate-run.md` carries two *Named residuals*. They look unrelated and are the same limitation
twice: **a POSIX shell cannot reach two kernel facilities**, and both are reachable from any
non-shell process.

**Residual 1 — no session primitive on macOS.** `setsid(1)` is absent on darwin, so `gate-run.sh`'s
rung 1 is unreachable there and every macOS launch lands on rung 3 (`set -m`, own process group).
ADR-0081 narrowed the contract honestly rather than claiming a session it does not deliver.

The evidence that motivates the capability is stronger than the evidence for what macOS actually
runs, and in an asymmetric direction:

- `gate-execution-evidence.md`'s four harness verdicts were all measured **on macOS 26.6.1**, all
  with `fork` + `POSIX::setsid` — *"a `nohup`'d, fully-redirected, backgrounded helper that forks,
  calls `setsid(2)` in the child"*. **That shape is one `gate-run.sh` cannot use**, because it
  required an interpreter. Rung 1 is dead code on the only platform the evidence covers.
- The disambiguating arms removed `setsid` from a plain `fork`, which leaves the child in the
  launcher's **own** process group. So the measurements compare *new session* against *same group as
  the launcher* — they never exercised the middle rung.
- The middle rung's only support is **ADR-0080's synthetic two-arm test**: a launcher TERM'd its own
  process group; the `set -m` child survived, the other died. That reproduces Codex's stated
  mechanism (*"tears down that call's process group on return"*) faithfully, but it was not run
  inside a live harness turn, and ADR-0080 says so: *"session-scoped teardown was not tested and is
  not claimed."*

Net: **no harness has ever been measured against the rung macOS actually runs.**

**Residual 2 — the `129..192` exit-status floor.** `$?` renders `exit 143` and death-by-signal-15
identically. `gate-run.md` states the bias is deliberate and that *"no arrangement of shell builtins
puts it there"* — correct, and the fix it names is *"a non-shell helper to read the raw wait
status."*

Both close with one helper that forks, `setsid(2)`s the child, execs, waits, and reads the raw
16-bit wait status.

## Decision

**Add `runtime.python` to docket's existing discovered-runtime framework, and use it to fill
`gate-run.sh`'s vacant rung 2.** Rung 2 is currently a documented empty slot — `script(1)` was
measured and rejected there at change 0282's plan time (typescript framing, pty CRLF, and a pty
merges stdout with stderr, which the stdout-is-the-protocol rule forbids).

**This takes no new *kind* of dependency.** ADR-0062 already fixed the boundary and drew it narrow:
*"This is **not** a claim that docket has no external requirements at all — change 0132 established
that docket validates a configured GNU Bash 4+ runtime. The rule bans an external YAML parser,
nothing wider."* `runtime.python` is a second instance of the change-0132 pattern that ADR-0062
explicitly blesses, not a new category.

### Why Python and not perl

`/usr/bin/perl` is a real Mach-O binary on macOS today and `POSIX::setsid` works, so it is a genuine
candidate. It was rejected on the following grounds, in the order they decided it:

1. **Resolution determinism is the only axis perl wins, and pinning an absolute path erases it.**
   Perl's advantage was that `/usr/bin/perl` is one fixed path needing no `PATH` lookup, while
   `python3` means whatever the environment says. A discovered-and-persisted absolute path removes
   that difference entirely.
2. **Validation timing favors the configured runtime.** `/usr/bin/perl` would be *assumed*, never
   checked — a wrong assumption surfaces inside an unattended gate. A discovered runtime is probed
   during `install.sh`, on screen, with a human present and a remedy printed.
3. **Deprecation risk runs the other way.** Apple's Catalina-era notice deprecates Perl, Ruby, and
   Python *together*; Python 2.7 was already removed and `/usr/bin/python3` is a shim. Homebrew
   Python is not subject to that notice at all.
4. **Legibility.** A maintainer three years out reads Python.

Perl also loses nothing by this decision: if the Python runtime is absent, the ladder degrades to
rung 3, which is exactly today's behavior.

## Design

### 1. Config key — `runtime.python`

A sibling of `runtime.bash`, with identical layer semantics:

- Absolute executable path to a Python 3 interpreter.
- **Scope: local-only** (`.docket.local.yml` or the global `config.yml`); never committed. A
  committed value is warned-and-ignored, matching `runtime.bash`.
- Resolved by `docket-config.sh --export`; documented in `.docket.example.yml` beside `runtime.bash`.

Exported as **`DOCKET_PYTHON_PATH`** by `ensure-docket-env.sh`, alongside `DOCKET_BASH_PATH`.

Unlike `DOCKET_BASH_PATH`, this variable is **optional by construction**. Read sites use a plain
default, never the `:?` fail-loud form:

```sh
PYTHON_BIN="${DOCKET_PYTHON_PATH:-}"
```

### 2. Discovery — extend `ensure-global-config.sh`

The `consider_candidate` / `validate_runtime` framework is already generic; it has exactly one
consumer. Add a second candidate list and a second validator. Candidate order:

1. `$(brew --prefix)/bin/python3`
2. `/opt/homebrew/bin/python3` (Apple silicon)
3. `/usr/local/bin/python3` (Intel)
4. `python3` as found on `PATH`

**`/usr/bin/python3` must be rejected, and rejecting it by omission is not sufficient.** It is
Apple's Xcode developer-tools shim, not an interpreter — verified 2026-08-10 on darwin 25.6.0:
`/usr/bin/python3` and `/usr/bin/cc` are the same file (inode `1152921500312571562`, link count 78),
while `/usr/bin/perl` has its own inode and one link. On a machine without Xcode or the Command Line
Tools, **executing it opens a GUI dialog and blocks** — so a validation probe that merely runs the
candidate would hang the install itself.

Candidate 4 can resolve to it. The validator therefore rejects any candidate whose **symlink-resolved
absolute path** is `/usr/bin/python3`, and that rejection is asserted by a test.

### 3. Validator — functional, never a version string

`docket_runtime_validate_python` mirrors `docket_runtime_validate_bash` in shape but not in method.
A major-version check is sufficient for Bash; it is not sufficient here, because a **pyenv shim for
an uninstalled version** or a **half-built virtualenv** is present on `PATH`, passes every presence
and version check, and fails only at exec — at gate time, unattended.

The validator therefore:

1. Reuses `validate_serializable_path` (absolute, executable, serializable).
2. Rejects the resolved `/usr/bin/python3` shim per §2.
3. Asserts the needed surface exists: `os.fork`, `os.setsid`, `os.execv`, `os.waitpid`,
   `os.WIFSIGNALED`, `os.WEXITSTATUS`, `os.WTERMSIG`.
4. Runs an **end-to-end smoke test with two arms**, because the two arms are the whole point:
   - a child that exits 7 → reports exit 7, signal 0
   - a child killed by signal 15 → reports signal 15, **not** exit 143

Arm 2 is the assert that proves the residual is actually closed. A validator that omits it would
pass on an interpreter that cannot do the one thing this change exists for.

### 4. Rung 2 — the Python wrapper

`gate-run.sh` keeps `--launch`, `--observe`, and `--stop` in Bash. **Only the wrapper moves**, and
only when rung 2 is selected. The wrapper must be the process that calls `wait`, because that is the
process the kernel hands the raw status to.

New helper (name settled at plan time; `scripts/gate-run-wrap.py` is the working name), with a
co-located `scripts/gate-run-wrap.md` contract per the repo's script-contract rule.

`detach_mode()` becomes a three-rung probe, still **probed at runtime, never by `uname`**:

| Rung | Condition | Delivers |
|---|---|---|
| 1 | `command -v setsid` succeeds | new session, Bash wrapper |
| 2 | `DOCKET_PYTHON_PATH` resolves and passes its read-time probe | **new session, Python wrapper, exact child status** |
| 3 | neither | own process group, Bash wrapper — today's behavior |

**The Python wrapper assumes the role currently held by `gate-run.sh --__wrap` and must preserve
every invariant that wrapper holds.** Enumerated so the build cannot miss one:

- **`TERM` ignored in the parent, default restored in the child before exec.** ADR-0081 records why:
  an untrapped wrapper dies alongside its own child on a group-directed `TERM`, which makes the
  `kind=signal` terminal record unreachable and degrades every signal death to "no record at all."
  **The ignore *disposition* only — no handler** — so that `wait` returning the command's status
  stays the single code path that writes the terminal record.
- **The process-group-leader self-check** before recording, refusing to record when the pid does not
  lead its own group.
- **The `launch` and `identity` records**, in the shape `--launch`'s handshake already polls for.
- **Detachment fully established before the launcher returns.** This is the measured, load-bearing
  precondition from `gate-execution-evidence.md` — a parent returning immediately after the fork is
  the arm on which *"the gate never started"* under Codex.
- **Streams reopened onto the durable log; stdin closed.**

The terminal record gains the distinction it exists for: `kind=exit` with the true code, or
`kind=signal` with the true signal number, taken from the raw wait status — **never** via the
`128+N` convention, which is the lossy step being removed.

### 5. Fallback, and the doc surfaces that become conditional

`DOCKET_PYTHON_PATH` unset, or failing its read-time probe → **rung 3, exactly as today**. Never a
hard failure: docket keeps working on a machine that never ran `install.sh`; it just does not get
the stronger guarantee.

`gate-run.md`'s *Per-platform capability note* and *Named residuals* stop being unconditional
statements and become **per-rung** statements. Residual 2's text is retained verbatim for the rung-3
case and marked as not applying under rungs 1–2.

## Named risk — two wrapper implementations

This is the real cost of graceful degradation, and it is not avoidable without making Python
mandatory: **the Bash wrapper and the Python wrapper must hold identical invariants, and the
fallback path is the less-exercised one.** A drift between them is exactly the kind of defect that
stays invisible until the rare platform hits it.

**Mitigation, which is a build requirement and not advice:** the existing `gate-run` contract
asserts are **parameterized over the resolved rung** and run against **both** wrappers, rather than
a second suite being written for the new one. Change 0282's review found twelve vacuous contract
asserts on this exact file — asserts that matched the document's own prose — so every assert added
or re-pointed here is mutation-tested per the repo rule: strip the thing it guards, watch it redden.

## Testing

- Discovery and validator: a new test file covering candidate ordering, the `/usr/bin/python3`
  rejection **including via the `PATH` candidate**, and both smoke-test arms.
- Wrapper parity: the `gate-run` contract asserts parameterized over rung, executed against both
  wrappers.
- Exit-status fidelity: an explicit pair asserting `exit 143` and signal-15 death produce different
  terminal records under rung 2, and the documented identical record under rung 3.
- Mutation-test every new or re-pointed assert.
- Full suite at the gate, per `finalize.test_command`.

## Out of scope

- **The delegation boundary.** `runner-dispatch`'s `set -m` (ADR-0080) would benefit from the same
  upgrade. Deliberately excluded: it doubles the blast radius, and change 0284 already touches
  `runner-dispatch`. Named as a follow-up, not scoped here.
- **Measuring the middle rung in a live harness.** Change 0264 owns that and stays valuable
  regardless of which rung ships.
- **Making Python mandatory.** The fallback is the point.
- **Any change to the four harness verdicts** in `gate-execution.md`. None is re-probed here.

## Assumptions

1. **Python, not perl** — settled by the human. Rationale in *Why Python and not perl*; the deciding
   factor was that pinning an absolute path erases perl's only advantage while install-time
   validation is strictly better than perl's assumed path.
2. **Opportunistic, not mandatory** — settled by the human. The ladder degrades to today's behavior
   rather than failing, which also bounds the deprecation risk to "loses an upgrade," never "breaks."
3. **The `/usr/bin/python3` exclusion is a resolved-path rejection, not an omission from a list.**
   The `PATH` candidate can reach it. Designer's call, from the inode measurement.
4. **The validator is functional, not version-based.** Designer's call; a pyenv shim passes every
   non-functional check.
5. **Only the wrapper moves to Python; the three verbs stay in Bash.** Forced by the requirement that
   the waiter reads the status — not a preference.
6. **Contract asserts are parameterized over the rung rather than duplicated.** Designer's call,
   driven by 0282's twelve-vacuous-assert finding.
7. **The new ADR supersedes ADR-0081.** ADR-0081 names this exact mechanism: *"superseding this ADR
   is the mechanism."* The ADR is minted at build time by `docket-adr`, not pre-allocated here.
8. **Close-out trap, carried forward deliberately.** At terminal close-out, `terminal-publish`'s
   *Accepted* gate silently skips an ADR whose status is no longer `Accepted` — which ADR-0081's will
   not be once superseded. The status flip must be published explicitly with
   `terminal-publish.sh --adr 81`, or `main` is left inconsistent. This is a known, previously-hit
   failure for exactly this change shape; it is recorded here because finalize will not catch it.
9. **`depends_on: [282]`, not `related`.** `gate-run.sh` does not exist on `main` until 0282 merges.
