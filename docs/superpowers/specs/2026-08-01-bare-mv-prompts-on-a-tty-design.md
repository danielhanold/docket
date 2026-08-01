<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0186 — Bare mv prompts on a tty — backfill-change-types hangs the suite and can exit 0 without installing](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-01-0186-bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui.md)**
<!-- docket:backlink:end -->

# Bare `mv` prompts on a tty — design

## Problem

`scripts/backfill-change-types.sh:162` installs each staged file with a bare `mv` (the stub says 161;
the verified line is **162**, and the rollback-restore `cp -p` is at **164**, not 163). BSD `mv` prompts
when the destination is unwritable **and** stdin is a terminal. `tests/test_backfill_change_types.sh`
(line ~201) forces exactly that condition with `chflags uchg` to exercise the install-phase rollback.

Two consequences, both verified 2026-08-01:

1. **The hang.** Run from an interactive terminal, `mv` blocks forever on
   `override rw-r--r-- … for 0002-b.md? (y/n [n])`. The suite is unfinishable by hand — the exact
   environment a maintainer runs the merge gate in. Every non-tty context (agent shells, the
   finalize gate) skips the prompt, fails `EPERM`, and reports green.
2. **The silent success the hang hides.** Under a pty with stdin at EOF, `mv` answers its own
   prompt `n`, prints `not overwritten`, and exits **0**. The staged file is never installed,
   `if ! mv` never fires, no rollback runs, and the script reports success — a half-migrated
   backlog with a zero exit, the precise outcome the install's undo exists to make impossible.

The guard is sound only because the environments that run it never have a tty.

## Decision summary

Replace the bare `mv` with `mv -f`, audit the `cp -p` twin, pin the property so the silent-success
path cannot return, and fix the two `profile-one-test.sh` / `profile-asserts.sh` ergonomics defects
that turned this into a six-minute mystery.

## Design

### 1. `mv -f` at the install call site

`scripts/backfill-change-types.sh:162`:

```sh
if ! mv -f "$out" "$CHANGES_DIR/active/$base"; then
```

`-f` suppresses the prompt unconditionally, on BSD and GNU alike, and returns non-zero when the
rename genuinely cannot happen. Probed on this machine (2026-08-01):

```
$ chflags uchg dst; mv -f src dst; echo $?
mv: rename src to dst: Operation not permitted
1
```

No prompt, non-zero exit, destination untouched — exactly what the rollback branch is written
against. This is the only edit that unblocks the suite.

### 2. Audit of the sibling install/restore calls

- **`cp -p` (line 164, the rollback restore) and `cp -p` (line 155, the backup stage).** `cp`
  prompts only under `-i`; neither call passes it, and scripts do not inherit interactive aliases.
  **No prompt exposure — no change.** The restore already degrades to a printed
  `rollback failed for …` warning on failure, which is the intended posture for a best-effort undo.
- **`mkdir -p`, `rm -rf`, the glob loops.** No interactive modes reachable. Nothing else in the
  script takes a path that could become unwritable mid-run.

The audit's conclusion is recorded here rather than encoded as a change, so the change's diff stays
the one line plus its guards.

### 3. Pinning the property in both environments

The existing rollback assertions are honest only without a tty. Two layers, both in
`tests/test_backfill_change_types.sh`:

- **A pty run of the rollback block.** A small local helper resolves a `script(1)` invocation for
  the host flavor and **probes it once for exit-status fidelity** — `script … /bin/sh -c 'exit 7'`
  must return 7 — before it is used:
  - BSD: `script -q /dev/null <cmd> <args…>` (propagates the child status; verified).
  - GNU/util-linux: `script -q -e -c '<cmd>' /dev/null`. **`-e` is mandatory** — without it
    util-linux `script` exits with its *own* status (0), so the non-zero-exit assertion would pass
    vacuously or fail spuriously on every GNU host. The exit-status probe is what makes this
    verified rather than assumed, on any flavor present or future.

  When no flavor passes the probe, the block prints a `skip - …` line, matching the idiom the
  surrounding `chflags` guard already uses.

  Four mechanical constraints the pty re-run must honor, all of which the existing non-pty block
  gets for free and would silently break under a pty:
  0. **Redirect the pty run's stdin from `/dev/null`.** `script` forwards its own stdin to the child
     pty. Without the redirect, a regression back to bare `mv`, run from a maintainer's terminal,
     blocks on the `override … (y/n [n])` read forever — the guard would reintroduce, in committed
     test code, the exact hang it exists to prevent. With `</dev/null` the pty gets EOF, `mv`
     self-answers `n` and exits 0, and the non-zero-exit assertion fails loudly and deterministically
     in **both** environments — which is the invariant this layer is buying. This is a **call-site**
     redirect on one scenario, not the blanket runner-level `</dev/null` that A8 rules out.
  1. **Capture stdout, not `2>&1 >/dev/null`.** The existing block captures the diagnostic with
     `rberr="$(… 2>&1 >/dev/null)"`. Under `script` both child streams land on the pty and are
     re-emitted on `script`'s **stdout**, which that idiom discards. The pty re-run captures plain
     stdout.
  2. **Normalize the pty's line discipline.** `script` emits CR line endings and injects a stray
     `^D`; the diagnostic assertions strip CRs (`tr -d '\r'`) before matching.
  3. **Extend the cleanup trap.** The current trap is
     `trap 'chflags -R nouchg "$drb" 2>/dev/null; rm -rf "$tmp"' EXIT` — scoped to `$drb` only. A
     second fixture with its own `uchg` file makes `rm -rf "$tmp"` fail and leaks an undeletable
     tmpdir. The trap must clear flags across **both** fixtures (or across `$tmp`) before the `rm`.
- **A source sentinel that always runs.** Assert the install call site carries `mv -f`, anchored to
  the **call-site line** rather than written as a whole-file "no bare `mv `" grep. (Note: today such
  a grep would *not* false-positive — the script's comments write ``` `mv` ``` with a closing
  backtick, so `mv` + space appears on line 162 only. The anchoring is justified on its real ground:
  a whole-file negative assertion is one comment edit or one reformat away from a false failure, and
  a call-site-anchored positive assertion says what is actually meant.) Cheap,
  environment-independent, and the layer that still fires where the pty helper skipped.

Neither layer alone is sufficient: the pty run tests behavior but is skippable; the sentinel always
runs but is text over source. Together the property survives both a `script`-less host and a
refactor that reformats the call site.

### 4. Profiler ergonomics (`scripts/profile-one-test.sh`, `scripts/profile-asserts.sh`)

**Correcting the stub's diagnosis first.** The stub blames stdout buffering. That premise is false
and must not be built on: Bash flushes builtin output per command, and `scripts/profile-asserts.sh`
lines 13–17 record exactly that as *"Verified on this suite; it is why the reader can be a plain
`read` loop and not a pty."* If stdout were block-buffered, `profile-asserts.sh`'s whole timing
design could not work. Likewise, `profile-one-test.sh:77` already prints its `tracing …` line
**before** the child launches at 83–86. The observed "no output at all" is the *invoking harness*
withholding a command's output until the process exits — which affects stderr identically. So a
stream change fixes nothing, and §4 does **not** make one.

The real, fixable defect is **artifact discoverability during a hang**, and it is fixed by making
the artifact path knowable without waiting for the process:

- **`profile-one-test.sh`** — print the trace and stdout paths **before** launching the child (in
  addition to the existing end-of-run summary at lines 137–138, which stays byte-identical on
  stdout). A hung run can then be diagnosed by reading the growing trace file from another shell.
  The pre-launch emission goes to **stdout**, the same stream as the existing summary — no stream
  migration, so nothing that parses these scripts' output changes shape; a duplicated path line is
  harmless. Callers were not enumerated, so the design deliberately avoids any change that would
  require enumerating them.
- **`profile-asserts.sh`** — its only pre-launch line is `profiling %d test file(s)` at line 108,
  **outside** the per-test loop (112–123), so it never says *which* test is hanging; and its TSV
  path is printed only at line 150. Two additions: print the TSV path **before** the loop, and emit
  a per-test `running <t>` line **inside** the loop immediately before the run at line 116. That
  pair — a known artifact path plus the name of the test currently executing — is what would have
  identified this hang in seconds. Moving line 108's stream would not have.

### 5. Learnings

The finding is a sharper instance of the existing `agent-shell-noop-reads-as-success` — same root
shape (an environment difference makes a no-op read as success), new face (a *tty* difference, in
committed test code rather than an agent's throwaway command). The close-out harvest should
**extend** that finding with this war story rather than mint a new one; the rule to add is: *a guard
that forces a failure via a filesystem flag is sound only if the tool under test does not prompt —
check the tool's interactive mode, and pin the non-interactive flag.*

## Assumptions

Every decision below was defaulted autonomously; the rejected alternatives and the reasoning are
the deferred audit trail.

**A1 — `mv -f` over the alternatives.** Chosen: add `-f`. Rejected: (a) redirect the script's stdin
from `/dev/null` — hides the class rather than fixing it, and the stub explicitly scopes that out;
(b) replace `mv` with `cp -p && rm` — changes atomicity within a filesystem and is a larger diff for
no gain; (c) `install(1)` — not POSIX-uniform and changes ownership/mode semantics. `-f` is the
minimal edit that both suppresses the prompt and preserves the test's intent (a genuine `EPERM`).
Verified by probe, not assumed.

**A2 — no change to the `cp -p` calls.** Chosen: audit, document, leave. Rejected: adding `-f`
defensively — `cp` cannot prompt without `-i`, so `-f` would be cargo, and on the *restore* path
`-f` would additionally unlink a destination the undo is trying to preserve. Risk if wrong: a
future `cp -i` alias-equivalent is missed; mitigated by the sentinel in §3 being extendable.

**A3 — both a pty test and a source sentinel, not one.** Chosen: both, with the pty layer skippable
behind an **exit-status-fidelity probe**. Rejected: sentinel-only (the stub's cheap option) — it
never proves the behavior and would have caught neither bug on its own; pty-only — `script(1)`
diverges between BSD and util-linux in both argument order *and* exit-status propagation (util-linux
needs `-e`), a class of divergence this repo has been burned by twice (#0130, #0178), so a pty-only
guard risks being green-because-skipped or green-because-vacuous with no backstop. The probe is what
converts "we believe `script` propagates the status" into a checked precondition. Revised after the
critic pass, which established concrete failure modes in the first draft's mechanics — an
unredirected pty stdin that would reintroduce the hang inside the guard itself, missing `-e` on
util-linux, the `2>&1 >/dev/null` capture idiom losing the pty stream, and the `uchg` cleanup trap
scoped to one fixture — all now specified in §3. One first-round claim (a whole-file `mv ` grep
false-failing on the script's comments) was **retracted as false** on re-check; the anchoring
decision survives on different reasoning.

**A4 — fix artifact discoverability; do NOT change streams.** Chosen: pre-launch emission of the
artifact paths on the existing stream, plus a per-test `running <t>` line in `profile-asserts.sh`'s
loop. Rejected: (a) the stub's and the first draft's "move progress/paths to stderr because stdout
is buffered" — the premise is **false**, contradicted by `profile-asserts.sh:13-17`'s verified
comment, and `profile-one-test.sh` already prints its progress line pre-launch, so the change would
have been motion without effect; (b) `stdbuf`/`unbuffer` — an extra dependency chasing the same
false premise, and coreutils' `stdbuf` is absent by default on macOS; (c) moving the end-of-run
summary — a behavior change for callers this design has not enumerated, and enumerating them is not
worth it for a fix that can be purely additive. The residual risk if A4 is wrong: the harness-level
whole-command buffering remains, and a hang is still opaque *in that harness* — but the trace file
is now readable out-of-band, which is the property that was actually missing.

**A5 — extend the existing learnings finding, do not mint a new one.** Chosen: recommend extending
`agent-shell-noop-reads-as-success`. Rejected: a new `tty-conceals-a-prompt` finding — the ledger's
rule is that the harvest creates or extends and never merges, so splitting a same-root lesson into
two files makes the later consolidation a human chore. **The fit is not clean, and the harvest should
weigh the tension:** that finding's closing sentence scopes itself out of exactly this case —
*"Neither failure was a defect in any committed script — both are traps for the agent's own throwaway
commands"* — whereas this lesson **is** a defect in committed test code. Extending it broadens an
intentionally narrow finding. The harvest at close-out is the sole writer; this is a recommendation
to it with the counter-argument attached, not a write. And the choice is **three-way, not binary** —
two existing findings may fit better than the incumbent: `shell-portability` (`promoted`; *"treat …
as suspect — and test each on both GNU and BSD"*) is a clean home for a BSD-vs-GNU tool-behavior
divergence in committed code, and `green-suite-untested-branch` (*"green tests are not proof the hard
branch was exercised"*) matches the fact that the rollback assertions are honest only without a tty.
The harvest picks among the three; minting a fourth file stays the last resort.

**A6 — couplings recorded as `related`, not `depends_on`.** #0134 (audit `field()` call sites) is a
whole-repo audit whose remit reaches `backfill-change-types.sh`, so a file collision is plausible
but neither change gates the other. #0150 (pin/report the resolved shell toolchain) and #0178 (BSD
grep parse error) are the same *class* — a suite that is green because the environment differs from
the target — and share no file with this change. All three are `related`; nothing is a dependency,
so this change is buildable today. Forward link only.

**A7 — the two profiler fixes ride along rather than splitting out.** Chosen: keep them in this
change. Rejected: a separate `chore` change — they are the instruments that failed to surface this
bug, discovered in the same investigation, and total roughly six lines across two scripts; a
separate change costs more bookkeeping than the diff. The stub already scoped them in.

**A8 — the suite invocation is untouched.** No blanket `</dev/null` at any runner level, per the
stub's explicit out-of-scope. Recorded here because it is the tempting fix and a future reader will
wonder why it was not taken: it would re-hide every future instance of this class.

## Out of scope

- A repo-wide BSD-vs-GNU prompting audit. This change fixes the one proven site, its immediate
  twin, and records the rule; a sweep is its own change if the finding justifies one.
- Changing how the suite is invoked.
- Re-designing the profilers beyond the two ergonomics defects.
