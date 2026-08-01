<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0186 — Bare mv prompts on a tty — backfill-change-types hangs the suite and can exit 0 without installing](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-01-0186-bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui.md)**
<!-- docket:backlink:end -->

# Bare `mv` prompts on a tty — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `backfill-change-types.sh`'s install non-interactive so the suite cannot hang on a maintainer's terminal, and so the tty path where `mv` exits 0 without installing anything cannot return.

**Architecture:** One-line fix (`mv` → `mv -f`) at the single install call site, pinned by two independent guards in the existing test file — a `script(1)` pty re-run of the rollback scenario (behavioral, skippable behind an exit-status-fidelity probe) and a call-site-anchored source sentinel (textual, always runs). Plus two purely additive discoverability fixes to the profilers that failed to surface this hang.

**Tech Stack:** POSIX shell / Bash 3.2+ and 5.x, BSD and GNU coreutils, `script(1)` (BSD and util-linux flavors), awk. No new dependencies.

## Global Constraints

Copied from the spec and from `AGENTS.md` (always-in-context repo rules). Every task's requirements implicitly include this section.

- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden, or it is decoration. **A mutation that "passes" is evidence only if the mutation landed**: confirm with `grep -c` before and after, every time.
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`) under `set -o pipefail` — the producer takes SIGPIPE and the 141 becomes an intermittent failure. Capture into a variable first, then `grep <<<"$var"`.
- **`grep` for a pattern that leads with `--` must declare it**: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`.
- **awk indent classes are `[^[:space:]]`, never `[^ ]`.**
- **Cross-references in maintained source anchor on a symbol name or a verbatim-quoted clause — NEVER a line number.** `tests/test_comment_anchor_style.sh` enforces the filename-plus-line-number form. This plan cites line numbers for navigation; **no line number may appear in any committed comment.**
- **Run the whole suite at the build gate**, never only the tests this plan enumerates: `for t in tests/test_*.sh; do bash "$t"; done`.
- **No blanket `</dev/null` at any runner level** (spec A8) — the pty redirect is a single call site only. Re-hiding this defect class is the one fix explicitly rejected.
- The test file runs under `set -uo pipefail` (**not** `-e`) and is **hermetic**: plain temp trees, no git, no network.
- `tests/test_backfill_change_types.sh` defines `assert(){ if eval "$2"; …}` — the second argument is **`eval`'d**, so keep it single-quoted at the source and let it expand shell variables at eval time.

## Verified probes (measured on this machine, 2026-08-01)

These are the facts the plan rests on. They were run, not assumed. An implementer who doubts one should re-run it rather than reason about it.

| Probe | Result |
|---|---|
| `mv src dst </dev/null` with `chflags uchg dst` | `mv: rename src to dst: Operation not permitted`, **rc=1** |
| `script -q /dev/null /bin/sh -c 'mv src dst' </dev/null` | `override … for dst? (y/n [n]) not overwritten`, **rc=0**, `src` still present — **the silent-success path** |
| `script -q /dev/null /bin/sh -c 'mv -f src dst' </dev/null` | `mv: rename src to dst: Operation not permitted`, **rc=1** |
| `script -q /dev/null /bin/sh -c 'exit 7'` (BSD form) | **rc=7** — status propagates |
| `script -q -e -c 'exit 7' /dev/null` (util-linux form, on this BSD host) | `script: illegal option -- -`, **rc=1** — correctly rejected by the probe |
| `script -q /dev/null env TMPDIR=… /bin/sh -c 'echo …; exit 3'` | **rc=3**, output on **stdout** even with the child's stderr redirected |

The last row is why the pty capture must read **stdout**: under `script` both child streams land on the pty and are re-emitted on `script`'s stdout, which the existing block's `2>&1 >/dev/null` idiom discards.

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `scripts/backfill-change-types.sh` | The fix: the install call site becomes non-interactive. Modify the `if ! mv "$out" "$CHANGES_DIR/active/$base"` line and extend the comment block above it. | 1 |
| `tests/test_backfill_change_types.sh` | Both guards. Adds a source sentinel (Task 1); widens the cleanup trap and adds the pty re-run plus its `script(1)` resolver (Task 2). | 1, 2 |
| `scripts/backfill-change-types.md` | Contract note that the install is non-interactive by construction — the "Nothing was installed" exit-code promise depends on it. | 1 |
| `scripts/profile-one-test.sh` | Emit the trace and stdout paths **before** launching the child. | 3 |
| `scripts/profile-asserts.sh` | Emit the TSV path before the loop and a `running <t>` line inside it. | 3 |
| `scripts/profile-one-test.md`, `scripts/profile-asserts.md` | Their `**Output:**` sentences describe the stream shape; both gain the pre-launch emission. | 3 |

No files are created. No file is split.

---

### Task 1: `mv -f` at the install call site, pinned by a source sentinel

The one edit that unblocks the suite, plus the always-runs guard. The behavioral guard is Task 2 — this task's sentinel is textual by design and is not a substitute for it.

**Files:**
- Modify: `scripts/backfill-change-types.sh` (the install loop's `mv` call and the comment block above it)
- Modify: `scripts/backfill-change-types.md` (the "Apply — staged, then installed" paragraph)
- Test: `tests/test_backfill_change_types.sh` (new source-sentinel section)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the call site `if ! mv -f "$out" "$CHANGES_DIR/active/$base"; then` — Task 2's pty guard asserts the *behavior* this line creates, and the sentinel below asserts the line itself. Neither task renames anything.

- [ ] **Step 1: Write the failing test**

Append a new section to `tests/test_backfill_change_types.sh`, immediately **after** the `# --- install-phase rollback ---` block's closing `fi` and **before** the `# --- dry run ---` section:

```bash
# --- source sentinel: the install cannot prompt ------------------------------
# `mv` PROMPTS when the destination is unwritable and stdin is a terminal (BSD). Interactively that
# hangs forever; under a pty at EOF it self-answers `n`, prints `not overwritten`, and exits 0 — so
# the staged file silently never installs, `if ! mv` never fires, and the run reports success. `-f`
# is what makes the install non-interactive, and it is load-bearing, not style.
#
# Anchored on the CALL SITE (an `mv` whose destination is the active-dir target) rather than written
# as a whole-file "no bare `mv `" grep: a whole-file negative assertion is one comment edit or one
# reformat away from a false failure, and a call-site-anchored positive assertion says what is
# actually meant. The population floor below is what keeps it from passing vacuously if the call
# site is ever renamed out from under it.
install_call="$(awk '/[[:space:]]mv[[:space:]]/ && /CHANGES_DIR\/active\/\$base/' "$SCRIPT")"
n_install="$(awk '/[[:space:]]mv[[:space:]]/ && /CHANGES_DIR\/active\/\$base/{c++} END{print c+0}' "$SCRIPT")"
assert "sentinel: exactly one mv install call site exists to check" '[ "$n_install" -eq 1 ]'
assert "sentinel: the install call site passes mv -f (cannot prompt on a tty)" \
  'grep -q -F -- "mv -f " <<<"$install_call"'
```

- [ ] **Step 2: Run the test to verify the sentinel fails**

Run: `bash tests/test_backfill_change_types.sh 2>&1 | grep -E "sentinel|NOT OK"`

Expected: `ok - sentinel: exactly one mv install call site exists to check` (the population floor already holds — there is one bare `mv` call site today) and `NOT OK - sentinel: the install call site passes mv -f (cannot prompt on a tty)`.

If the population-floor assert is *also* red, stop: the awk anchor does not match the real call site and must be corrected before going further — a red floor means the second assert is unfalsifiable rather than failing honestly.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/backfill-change-types.sh`, change the install loop's move from `mv` to `mv -f`:

```sh
  if ! mv -f "$out" "$CHANGES_DIR/active/$base"; then
```

Then extend the existing comment block that ends `…a failed \`mv\` restores whatever already landed before dying.` by appending these lines to it (no line numbers — anchor on the clause):

```sh
# `mv -f` is not decoration. Bare `mv` PROMPTS when the destination is unwritable and stdin is a
# terminal, so an interactive run blocks forever; and at EOF on a pty it answers its own prompt `n`,
# prints `not overwritten`, and exits 0 — the staged file is never installed, `if ! mv` never fires,
# no rollback runs, and the run reports SUCCESS with a half-migrated backlog. `-f` suppresses the
# prompt on BSD and GNU alike and still returns non-zero on a genuine EPERM, which is exactly what
# the rollback branch below is written against.
```

Leave both `cp -p` calls (the backup stage and the rollback restore) **unchanged**: `cp` prompts only under `-i`, neither passes it, and `-f` on the restore path would unlink the very destination the undo exists to preserve (spec A2).

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_backfill_change_types.sh`

Expected: every assertion `ok -`, including both sentinel lines. The pre-existing rollback block must stay green — `mv -f` returns non-zero on the `chflags uchg` destination exactly as bare `mv` did without a tty, so its diagnostic and restore assertions are unaffected.

- [ ] **Step 5: Mutation-test the sentinel**

A guard that never saw red against the real defect is untested code. Confirm the mutation **landed** before believing the result:

```bash
grep -c 'mv -f "$out"' scripts/backfill-change-types.sh   # expect 1
sed -i '' 's/mv -f "\$out"/mv "$out"/' scripts/backfill-change-types.sh
grep -c 'mv -f "$out"' scripts/backfill-change-types.sh   # expect 0  <- proves the mutation landed
bash tests/test_backfill_change_types.sh 2>&1 | grep "sentinel"
```

Expected: `NOT OK - sentinel: the install call site passes mv -f (cannot prompt on a tty)`.

Restore the fix and re-confirm:

```bash
sed -i '' 's/mv "\$out"/mv -f "$out"/' scripts/backfill-change-types.sh
grep -c 'mv -f "$out"' scripts/backfill-change-types.sh   # expect 1 again
bash tests/test_backfill_change_types.sh 2>&1 | grep "sentinel"
```

Expected: both sentinel assertions `ok -`.

(`sed -i ''` is the BSD form. On GNU use `sed -i`. Either way, the `grep -c` before/after is the evidence — not the sed's exit status.)

- [ ] **Step 6: Update the script contract**

In `scripts/backfill-change-types.md`, in the paragraph beginning **`**4. Apply — staged, then installed.**`**, append this sentence to the end of that paragraph:

```markdown
The move is `mv -f`: a bare `mv` prompts when a destination is unwritable and stdin is a terminal,
and at EOF it declines the overwrite and exits **0** — which would install nothing while reporting
success, making the "Nothing was installed" promise below false in exactly the environment a
maintainer runs it in.
```

- [ ] **Step 7: Run the whole suite**

Run: `for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAILED: $t"; done`

Expected: no `FAILED:` lines.

- [ ] **Step 8: Commit**

```bash
git add scripts/backfill-change-types.sh scripts/backfill-change-types.md tests/test_backfill_change_types.sh
git commit -m "fix(0186): make the backfill install non-interactive with mv -f

A bare mv prompts when the destination is unwritable and stdin is a tty:
interactively it hangs the suite forever, and at EOF on a pty it declines
the overwrite and exits 0 — installing nothing while reporting success.
Pinned by a call-site-anchored source sentinel."
```

---

### Task 2: Pin the property behaviorally with a `script(1)` pty re-run

The sentinel proves the text; this proves the behavior. It is skippable by construction (no faithful `script(1)` on the host), which is precisely why Task 1's sentinel exists as the backstop.

**Files:**
- Modify: `tests/test_backfill_change_types.sh` (widen the cleanup trap; add the `script(1)` resolver and the pty rollback scenario)

**Interfaces:**
- Consumes: `mv -f` at the install call site (Task 1); the file's existing `mkfix`, `arc_hash`, `assert`, `$SCRIPT`, `$tmp` helpers.
- Produces: `pty_probe` (sets `PTY_FLAVOR` to `bsd`/`gnu`, returns non-zero when no flavor is faithful) and `pty_run <cmd> [args…]` (runs under a pty with stdin at `/dev/null`, stdout captured, child status propagated). Nothing later depends on these.

- [ ] **Step 1: Widen the cleanup trap**

The rollback block's trap currently clears the immutable flag on `$drb` only. A second fixture with its own `uchg` file would make `rm -rf "$tmp"` fail and leak an undeletable tmpdir. In `tests/test_backfill_change_types.sh`, inside the `if chflags uchg …` branch, replace:

```bash
  trap 'chflags -R nouchg "$drb" 2>/dev/null; rm -rf "$tmp"' EXIT
```

with:

```bash
  # Scoped to $tmp, not $drb: the pty scenario below adds a SECOND fixture with its own immutable
  # file, and a flag left set anywhere under $tmp makes the rm fail and leaks an undeletable dir.
  trap 'chflags -R nouchg "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT
```

- [ ] **Step 2: Write the failing test**

Append this section to `tests/test_backfill_change_types.sh` immediately **after** the source-sentinel section added in Task 1:

```bash
# --- install-phase rollback, UNDER A PTY -------------------------------------
# The rollback block above is honest only where stdin is NOT a terminal. Under a pty a bare `mv`
# self-answers its override prompt `n` and exits 0, so the script installs nothing, never rolls
# back, and reports success — the assertions above would all still pass on the pre-fix code. This
# block re-runs the same scenario with a terminal attached so that path cannot come back.

# Resolve a script(1) flavor by PROBING it for exit-status fidelity rather than sniffing uname:
# util-linux's `script` exits with its OWN status unless `-e` is passed, which would make every
# exit-status assertion under it pass vacuously. A flavor that cannot report `exit 7` as 7 is not
# used at all.
PTY_FLAVOR=""
pty_probe(){
  command -v script >/dev/null 2>&1 || return 1
  script -q /dev/null /bin/sh -c 'exit 7' </dev/null >/dev/null 2>&1
  if [ "$?" -eq 7 ]; then PTY_FLAVOR=bsd; return 0; fi
  script -q -e -c 'exit 7' /dev/null </dev/null >/dev/null 2>&1
  if [ "$?" -eq 7 ]; then PTY_FLAVOR=gnu; return 0; fi
  return 1
}
# pty_run <cmd> [args...] — CMD under a pseudo-terminal. stdin comes from /dev/null at THIS call
# site (never at the runner level): `script` forwards its own stdin to the child pty, so without the
# redirect a regression to bare `mv` would block on the override prompt forever — the guard would
# reintroduce, in committed test code, the exact hang it exists to prevent. With EOF the regression
# fails loudly and deterministically instead. Both child streams land on the pty and come back on
# script's STDOUT, so callers capture stdout and strip the CRs the line discipline adds.
pty_run(){
  case "$PTY_FLAVOR" in
    bsd) script -q /dev/null "$@" </dev/null 2>/dev/null ;;
    gnu) script -q -e -c "$(printf '%q ' "$@")" /dev/null </dev/null 2>/dev/null ;;
    *)   return 127 ;;
  esac
}

drb2="$tmp/rollback-pty"; mkfix "$drb2"; mkdir -p "$drb2/tmpdir"
pb1="$(cat "$drb2/active/0001-a.md")"; pb2="$(cat "$drb2/active/0002-b.md")"
pb4="$(cat "$drb2/active/0004-d.md")"; pbarc="$(arc_hash "$drb2")"
if pty_probe && chflags uchg "$drb2/active/0002-b.md" 2>/dev/null; then
  praw="$(pty_run env TMPDIR="$drb2/tmpdir" bash "$SCRIPT" \
            --changes-dir "$drb2" --map "1=fix,2=docs,4=chore")"
  pbrc=$?
  perr="$(tr -d '\r' <<<"$praw")"
  chflags nouchg "$drb2/active/0002-b.md" 2>/dev/null
  assert "pty: a mid-install failure exits non-zero WITH a terminal attached" '[ "$pbrc" -ne 0 ]'
  assert "pty: the install did not silently decline the overwrite" \
    '! grep -q "not overwritten" <<<"$perr"'
  assert "pty: the failure names the file and says it rolled back" \
    'grep -q "install failed for 0002-b.md" <<<"$perr" && grep -q "rolled back" <<<"$perr"'
  assert "pty: the file installed BEFORE the failure is restored to its pre-run bytes" \
    '[ "$(cat "$drb2/active/0001-a.md")" = "$pb1" ]'
  assert "pty: the file the install failed ON is unchanged" \
    '[ "$(cat "$drb2/active/0002-b.md")" = "$pb2" ]'
  assert "pty: the files AFTER the failure are unchanged" \
    '[ "$(cat "$drb2/active/0004-d.md")" = "$pb4" ]'
  assert "pty: the archive is byte-identical" '[ "$(arc_hash "$drb2")" = "$pbarc" ]'
else
  # Matches the idiom the surrounding chflags guard already uses.
  echo "skip - pty: no exit-status-faithful script(1), or cannot make a destination unwritable here"
fi
```

- [ ] **Step 3: Run the test and confirm the pty block actually RAN**

Run: `bash tests/test_backfill_change_types.sh 2>&1 | grep -E "^(ok|NOT OK|skip) - pty"`

Expected on this machine (BSD `script`, `chflags` available): seven `ok - pty: …` lines and **no** `skip -` line.

A `skip -` line here is not a pass — it means the guard is green-because-skipped and has proven nothing. If it appears, run `script -q /dev/null /bin/sh -c 'exit 7'; echo $?` and confirm it prints `7`; if it does, `pty_probe` is wrong and must be fixed before the task is done.

- [ ] **Step 4: Mutation-test the pty guard against the real defect**

This is the step that proves the block is load-bearing rather than an expensive way to re-assert Task 1. Confirm the mutation landed:

```bash
grep -c 'mv -f "$out"' scripts/backfill-change-types.sh   # expect 1
sed -i '' 's/mv -f "\$out"/mv "$out"/' scripts/backfill-change-types.sh
grep -c 'mv -f "$out"' scripts/backfill-change-types.sh   # expect 0  <- the mutation landed
bash tests/test_backfill_change_types.sh 2>&1 | grep -E "^(ok|NOT OK|skip) - pty"
```

Expected: `NOT OK - pty: a mid-install failure exits non-zero WITH a terminal attached` **and** `NOT OK - pty: the install did not silently decline the overwrite`. The run must **terminate** — if it hangs, the `</dev/null` redirect in `pty_run` is missing or misplaced, which is the one defect this guard must never itself contain.

Note that the **non**-pty rollback block stays fully green under this same mutation. That contrast is the whole point: it is the demonstration that the pre-fix suite could not see this bug.

Restore and re-confirm:

```bash
sed -i '' 's/mv "\$out"/mv -f "$out"/' scripts/backfill-change-types.sh
grep -c 'mv -f "$out"' scripts/backfill-change-types.sh   # expect 1 again
bash tests/test_backfill_change_types.sh 2>&1 | grep -E "^(ok|NOT OK|skip) - pty"
```

Expected: seven `ok - pty: …` lines.

- [ ] **Step 5: Confirm the widened trap leaves nothing behind — and record what it does NOT cover**

Run: `bash tests/test_backfill_change_types.sh >/dev/null 2>&1; ls /var/folders/*/*/T/tmp.* 2>/dev/null | wc -l`

**Corrected after review (2026-08-01) — the original expectation here was wrong and is retained as a caution, not a target.** It read *"the count does not grow across two consecutive runs"*. It does grow: **+1 per run on `origin/main`, +2 per run on this branch.** Do not treat a growing count as a failure of this task.

The widened trap does what it is for — it clears flags across `$tmp`, so neither fixture can leave an undeletable directory *inside the test's own tree*. The leak is somewhere the trap cannot reach: `scripts/backfill-change-types.sh` creates its scratch dir with `mktemp -d` and **no template**, and macOS `mktemp -d` ignores `TMPDIR` entirely (measured). So the `TMPDIR="$drb/tmpdir"` redirect that both fixtures pass is a no-op on macOS, the stage dir lands under `/var/folders/…` outside `$tmp`, `cp -p` preserves the `uchg` flag onto its `.backup/0002-b.md`, and the script's own `trap 'rm -rf "$stage"'` then fails with `Operation not permitted`.

This is **pre-existing** (it is why the count already grows on `origin/main`); this branch's second `uchg` fixture doubles the rate by the identical mechanism. It is a resource leak, not a correctness problem for any assertion. It is tracked as its own change — the fix is one line in `backfill-change-types.sh` (give `mktemp` a template) and belongs with its own coverage, not smuggled in here.

Accept this step when the suite passes; sweep accumulated debris with `chflags -R nouchg <dir>; rm -rf <dir>` if it bothers you.

- [ ] **Step 6: Run the whole suite**

Run: `for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAILED: $t"; done`

Expected: no `FAILED:` lines.

- [ ] **Step 7: Commit**

```bash
git add tests/test_backfill_change_types.sh
git commit -m "test(0186): pin the non-interactive install under a real pty

Re-runs the rollback scenario with a terminal attached, behind a
script(1) exit-status-fidelity probe (util-linux needs -e or the
assertion passes vacuously). Widens the uchg cleanup trap to \$tmp so
the second fixture cannot leak an undeletable dir."
```

---

### Task 3: Profiler artifact discoverability

The instruments that turned this into a six-minute mystery. Purely additive on the existing stdout stream — **no stream migration**: the stub's stdout-buffering diagnosis was disproved during grooming (`profile-asserts.sh` documents, as verified, that Bash flushes builtin output per command — which is why its reader can be a plain `read` loop), and `profile-one-test.sh` already prints its progress line before the child launches. The real defect is that during a hang you cannot learn the artifact path or the test name.

**Files:**
- Modify: `scripts/profile-one-test.sh` (pre-launch path emission)
- Modify: `scripts/profile-asserts.sh` (pre-loop TSV path; per-test `running` line)
- Modify: `scripts/profile-one-test.md`, `scripts/profile-asserts.md` (their `**Output:**` sentences)

**Interfaces:**
- Consumes: nothing from earlier tasks — independent of Tasks 1 and 2 and reviewable on its own.
- Produces: nothing later depends on it. No test parses either script's output (verified: no file under `tests/` references `profile-asserts` or `profile-one-test`; both are dev tooling that gates nothing), so added stdout lines break no consumer.

- [ ] **Step 1: Add the pre-launch emission to `profile-one-test.sh`**

Immediately after the existing `printf 'tracing %s under %s ...\n' "$TEST" "$BASH_BIN"` line, insert:

```sh
# Printed BEFORE the child launches, not only in the end-of-run summary: when a test HANGS the
# summary never arrives, and the growing trace file read from another shell is the only way to see
# where it stopped. Same stream as the summary — nothing parsing this output changes shape, and a
# duplicated path line is harmless where a missing one is a dead end.
printf 'trace:  %s\nstdout: %s\n' "$TRACE" "$out"
```

Leave the end-of-run `printf '\ntrace:  %s\n'` / `printf 'stdout: %s\n'` pair byte-identical.

- [ ] **Step 2: Verify it prints before the child runs**

Run: `bash scripts/profile-one-test.sh tests/test_change_types.sh 2>&1 | head -5`

Expected: `tracing …`, then `trace:  /…/trace.log` and `stdout: /…/stdout.log`, all before any timing table. Confirm the named trace file exists and is non-empty afterwards: `ls -l` the path it printed.

- [ ] **Step 3: Add the two emissions to `profile-asserts.sh`**

After the existing `printf 'profiling %d test file(s) under %s\n' "${#tests[@]}" "$BASH_BIN"` line, insert:

```sh
# The records path up front for the same reason the per-test line below exists: during a hang the
# end-of-run print never arrives. `$TSV` is already resolved by this point.
printf 'per-assertion records: %s\n' "$TSV"
```

Then, inside the per-test loop, immediately **before** the `{ "$BASH_BIN" "$t" …; } | stamp_stream …` run, insert:

```sh
  # The pre-loop line says how MANY files; only this says WHICH one is executing. Without it a hung
  # test is anonymous — the per-test rollup below prints only after the file completes.
  printf 'running %s\n' "$t"
```

Leave the end-of-run `printf 'per-assertion records: %s\n' "$TSV"` line in place — the duplication is intentional and harmless.

- [ ] **Step 4: Verify both emissions**

Run: `bash scripts/profile-asserts.sh tests/test_change_types.sh 2>&1 | head -6`

Expected: `profiling 1 test file(s) under …`, then `per-assertion records: /…/asserts.tsv`, then `running tests/test_change_types.sh`, all before the tables.

- [ ] **Step 5: Update both script contracts**

In `scripts/profile-one-test.md`, in the sentence beginning `**Output:**`, replace `and finally the trace and captured-stdout paths` with:

```markdown
and finally the trace and captured-stdout paths — which are ALSO printed up front, before the test
launches, so a hung run can be diagnosed by reading the growing trace file from another shell
```

In `scripts/profile-asserts.md`, in the sentence beginning `**Output:**`, replace `a progress line per test file` with:

```markdown
the records path, then a `running <test>` line and a completion line per test file
```

- [ ] **Step 6: Run the whole suite**

Run: `for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAILED: $t"; done`

Expected: no `FAILED:` lines.

- [ ] **Step 7: Commit**

```bash
git add scripts/profile-one-test.sh scripts/profile-asserts.sh scripts/profile-one-test.md scripts/profile-asserts.md
git commit -m "chore(0186): make profiler artifacts discoverable during a hang

Print the trace/stdout and TSV paths before launching, and name each
test as it starts. Additive on the existing stdout stream — the stub's
buffering diagnosis was disproved, so no stream is migrated."
```

---

## Verification at the gate

- [ ] Whole suite green: `for t in tests/test_*.sh; do bash "$t" >/dev/null 2>&1 || echo "FAILED: $t"; done` prints nothing.
- [ ] **The suite completes from an interactive terminal** — the property this whole change exists to restore. Run `bash tests/test_backfill_change_types.sh` in a real terminal (not redirected) and confirm it terminates.
- [ ] Neither guard is green-because-skipped: `bash tests/test_backfill_change_types.sh 2>&1 | grep "^skip -"` prints nothing on this host.
- [ ] `bash tests/test_comment_anchor_style.sh` passes — no committed comment added by this change cites a line number.
- [ ] `git diff origin/main --stat` touches exactly the six files in the File Structure table.
