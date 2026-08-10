<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0277 — Delegated task briefs travel through shell argv, a lossy model-performed transformation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0277-delegated-task-briefs-travel-through-shell-argv-a-lossy-mode.md)**
<!-- docket:backlink:end -->

# Delegated brief-file channel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `runner-dispatch` a `--brief-file` channel so a delegated child's task brief reaches it verbatim instead of through a model-quoted, `$*`-flattened argv string.

**Architecture:** Two mutually exclusive payload channels. The facade (`scripts/runner-dispatch.sh`) gains `--brief-file <path>`, validates it, refuses when both channels are present, refuses a `build-*` dispatch carrying no payload at all, spools the brief into `$DDIR/brief` under `--launch`, and composes a single-channel adapter invocation at every one of its three handoff sites. The three runner adapters gain the same flag, enforce the same exclusion defensively, and replace the lossy `$*` interpolation with an order- and newline-preserving join over `"$@"`. `sync-agents.sh`'s `emit_shim` teaches exactly one path: write a heredoc brief, then launch with `--brief-file`.

**Tech Stack:** POSIX-ish bash (GNU bash 4+ at `DOCKET_BASH_PATH`), the repo's hand-rolled `assert` test harness under `tests/`, `scripts/run-tests.sh` as the suite gate.

## Global Constraints

Copied verbatim from the spec (`.docket/docs/superpowers/specs/2026-08-09-delegated-brief-file-channel-design.md`) and from `AGENTS.md`:

- Both channels present ⇒ **refuse**, at the facade AND defensively in the adapters. Never prefer one, never concatenate.
- A `build-*` dispatch with no payload at all dies at the **same pre-verb validation point** as the existing `build-*` `--worktree` gate — verb-neutral, covering `--launch` and the legacy foreground verb alike. Non-build agents stay legal payload-free.
- All three adapters switch `$*` to a newline-preserving `"$@"` join **regardless** of whether a brief file was passed.
- The brief's contents are appended to the prompt **verbatim** — via `cat` / command substitution only. Never `eval`, never `printf %b`, never a format string. A model-authored brief is untrusted input containing single quotes, backslashes, `%`, backticks, and `--flag`-shaped lines.
- The shim teaches ONE path (heredoc write, then `--brief-file`), rendered **unbracketed** and emphatic. The bracketed `[-- <caller args>]` spelling must not reappear in any spelling; the single-quoting gymnastics paragraph is deleted with the argv teaching.
- `mktemp` always takes an explicit template: `"${TMPDIR:-/tmp}/<name>.XXXXXX"` (AGENTS.md — bare `mktemp` ignores `TMPDIR` on macOS).
- `grep` for a pattern that leads with `--` must declare it: `grep -qF -- "<pat>"` / `grep -qE -- "<pat>"`. A bare leading `--` is parsed as an option (exit 2), and inside a negated assert that error inverts into a permanently green, vacuous guard.
- Never `producer | early-exiting-consumer` under `pipefail` — capture into a variable, then `grep <<<"$var"`.
- A value-taking flag parsed with `shift 2` **spins forever** when it is the last argument (bash's `shift` fails rather than truncating, and these parse loops have no trailing shift). Every new value-taking flag uses the facade's house safe form: `shift; [ $# -gt 0 ] && shift`.
- Brief retention rides the dispatch dir's existing prune (`docket_dispatch_prune`). No new lifecycle, no separate cleanup.
- Scope fence: change 0208 is queued immediately after this one on `scripts/runner-dispatch.sh` and `tests/test_runner_dispatch.sh`. Touch nothing outside this plan. In particular the `--observe` poll-loop `state=` prefix-strip defect class belongs to change 0284 — do **not** fix it here.

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/runners/opencode.sh`, `codex.sh`, `cursor.sh` | `--brief-file` parse + validation, defensive XOR, newline-preserving argv join, verbatim brief append (Task 1) |
| `scripts/runners/opencode.md`, `codex.md`, `cursor.md` | document the new flag and the exclusion (Task 1) |
| `scripts/runner-dispatch.sh` | `--brief-file` parse/validate, XOR refuse, `build-*` empty-payload gate, single-channel handoff (Task 2); `$DDIR/brief` spool (Task 3); run-gate re-dispatch combined brief (Task 4) |
| `scripts/runner-dispatch.md` | Usage, Behavior, Exit codes, Invariants for all of the above (Tasks 2–4) |
| `sync-agents.sh` (`emit_shim`) | the two-step shim template (Task 5) |
| `tests/test_runner_dispatch.sh` | adapter-level brief/join/XOR cases + facade validation cases (Tasks 1, 2, 4) |
| `tests/test_runner_opencode.sh`, `tests/test_runner_cursor.sh` | the twin adapters' join + brief cases (Task 1) |
| `tests/lib/runner_dispatch_detach_common.sh`, `tests/test_runner_dispatch_detach.sh` | the `--launch` spool cases (Task 3) |
| `tests/test_sync_agents_runners.sh` | shim sentinels (Task 5) |
| `tests/runtime-budgets.tsv` | re-measured budget row for `tests/test_runner_dispatch.sh` (Task 6) |

---

### Task 1: Adapters — brief-file channel, defensive XOR, newline-preserving join

All three adapters are copy-paste twins. They are changed **together** in one task: a twin left on `$*` is exactly the `fix-reintroduces-its-own-defect-class` shape this change exists to close.

**Files:**
- Modify: `scripts/runners/codex.sh` (parse loop ~lines 19–32; prompt assembly ~lines 58–64)
- Modify: `scripts/runners/opencode.sh` (parse loop ~lines 24–32; prompt assembly ~lines 79–85)
- Modify: `scripts/runners/cursor.sh` (parse loop; prompt assembly ~lines 63–69)
- Modify: `scripts/runners/codex.md`, `scripts/runners/opencode.md`, `scripts/runners/cursor.md` (Usage + Behavior)
- Test: `tests/test_runner_dispatch.sh` (codex adapter section), `tests/test_runner_opencode.sh`, `tests/test_runner_cursor.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the adapter CLI contract every later task depends on — `--brief-file <path>` accepted by all three adapters; a brief file and trailing argv together ⇒ exit non-zero with a message containing the literal `never both`; the prompt's payload section keeps the existing heading `Additional caller arguments / task context:`.

- [ ] **Step 1: Write the failing adapter tests (codex, in `tests/test_runner_dispatch.sh`)**

Insert a new section immediately after the existing `# ---- adapter: passthrough args land in the prompt ------` section, before `# ---- adapter: failure postures ----`:

```bash
# ---- 0277: the brief-file channel + a non-lossy argv join ---------------------
# The caller's brief is the child's ONLY input. It used to travel as `$*`, which joins the
# positional parameters on the first character of IFS — a multi-line brief passed as several
# arguments arrived as one line, silently. The fixtures below carry the characters a
# model-authored brief actually contains (single quotes, backslashes, `%`, backticks, and a
# `-- --flag`-shaped line), because the append must be VERBATIM: no eval, no printf format.
make_fixture
BF="$SBX/brief.txt"
cat > "$BF" <<'BRIEF'
line-one: build change 0277
line-two: it's got a quote, a backslash \n, a percent %s, and a `backtick`
-- --flag-shaped-line
BRIEF
run_adapter --agent status --brief-file "$BF" >/dev/null 2>&1; rc=$?
assert "0277 codex: a brief-file dispatch exits 0" '[ "$rc" = "0" ]'
assert "0277 codex: the payload heading is present" \
  'grep -qxF -- "Additional caller arguments / task context:" "$LOG"'
assert "0277 codex: brief line 1 lands on its own line" 'grep -qxF -- "line-one: build change 0277" "$LOG"'
assert "0277 codex: brief line 2 lands VERBATIM on its own line" \
  'grep -qxF -- "line-two: it'"'"'s got a quote, a backslash \n, a percent %s, and a \`backtick\`" "$LOG"'
assert "0277 codex: a --flag-shaped brief line survives intact" 'grep -qxF -- "-- --flag-shaped-line" "$LOG"'
# THE LOSSINESS ASSERT: with `$*` the three lines would be joined onto one.
assert "0277 codex: the brief is NOT flattened onto one line" \
  '! grep -qF -- "line-one: build change 0277 line-two:" "$LOG"'
rm -rf "$SBX"

# The surviving argv path is non-lossy too — order preserved, joined on NEWLINE, not on a space.
make_fixture
run_adapter --agent status -- "argv-alpha" "argv-beta" >/dev/null 2>&1
assert "0277 codex: multiple post-\`--\` args each land on their own line" \
  'grep -qxF -- "argv-alpha" "$LOG" && grep -qxF -- "argv-beta" "$LOG"'
assert "0277 codex: post-\`--\` args are NOT space-joined" '! grep -qF -- "argv-alpha argv-beta" "$LOG"'
assert "0277 codex: order is preserved (alpha before beta)" \
  '[ "$(grep -nxF -- "argv-alpha" "$LOG" | head -n1 | cut -d: -f1)" -lt "$(grep -nxF -- "argv-beta" "$LOG" | head -n1 | cut -d: -f1)" ]'
rm -rf "$SBX"

# Defensive exclusion: the adapter contracts document a direct hand invocation that bypasses the
# facade, so the refusal cannot live only at the facade.
make_fixture
printf 'a brief\n' > "$SBX/brief.txt"
err="$( run_adapter --agent status --brief-file "$SBX/brief.txt" -- "also argv" 2>&1 >/dev/null )"; rc=$?
assert "0277 codex: both channels together are refused" '[ "$rc" != "0" ]'
assert "0277 codex: the refusal says never both" 'grep -qiF "never both" <<<"$err"'
assert "0277 codex: the refusal never reached codex exec" '[ ! -s "$LOG" ]'
# An unreadable or empty brief is a usage error, not an empty prompt.
err="$( run_adapter --agent status --brief-file "$SBX/no-such-brief" 2>&1 >/dev/null )"; rc=$?
assert "0277 codex: a missing brief file is refused" '[ "$rc" != "0" ]'
: > "$SBX/empty-brief"
err="$( run_adapter --agent status --brief-file "$SBX/empty-brief" 2>&1 >/dev/null )"; rc=$?
assert "0277 codex: an empty brief file is refused" '[ "$rc" != "0" ]'
assert "0277 codex: the empty-brief refusal says empty" 'grep -qiF "empty" <<<"$err"'
# A value-taking flag in FINAL position must not spin the parse loop (the `--observe` hazard).
err="$( run_adapter --agent status --brief-file 2>&1 >/dev/null )"; rc=$?
assert "0277 codex: --brief-file with no value exits instead of spinning" '[ "$rc" != "0" ]'
rm -rf "$SBX"
```

Add the equivalent block to `tests/test_runner_opencode.sh` and `tests/test_runner_cursor.sh`, using each file's own fixture/runner helper names and its own argv-log variable (read the top of each file and mirror its existing style; the assert *names* change prefix `0277 codex:` to `0277 opencode:` / `0277 cursor:`). Each twin needs at minimum: the multi-line-brief-verbatim assert, the not-flattened assert, the newline-join assert, and the both-channels refusal.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep -c "NOT OK"`
Expected: a non-zero count; the failures name the `0277 codex:` asserts (`--brief-file` is an unknown argument today, so the dispatches abort).

Run: `bash tests/test_runner_opencode.sh 2>&1 | grep "NOT OK"` and `bash tests/test_runner_cursor.sh 2>&1 | grep "NOT OK"`
Expected: the new `0277` asserts fail in each.

- [ ] **Step 3: Implement in all three adapters**

In each adapter's variable initialization line, add `BRIEF_FILE=""` (e.g. `AGENT=""; MODEL=""; EFFORT=""; BRIEF_FILE=""`).

In each adapter's parse loop, add this arm above `--)`:

```bash
    # `shift 2` is this loop's house form, but bash's `shift` FAILS rather than truncating when the
    # flag is the last argument and this loop has no trailing shift — so a value-taking flag in
    # final position would spin here forever, making the "requires a path" refusal below
    # unreachable. Shift the flag, then the value only if a value is actually there.
    --brief-file) BRIEF_FILE="${2:-}"; shift; [ $# -gt 0 ] && shift ;;
```

Immediately after the existing `[ -n "$AGENT" ] || die "--agent is required"` line, add:

```bash
# --- change 0277: the brief-file channel, and its exclusion with trailing argv -------
# The caller's brief is the child's ONLY input, and argv is a lossy way to carry it: a model must
# quote it correctly in one shot, and `$*` then joins the arguments on the first character of IFS.
# `--brief-file` removes both hazards — the caller writes a quoted heredoc and passes a path.
# BOTH CHANNELS AT ONCE IS REFUSED, never merged: preferring either silently drops or duplicates
# the child's whole input, and concatenation has no defensible ordering. Refusal is the only shape
# with no silent-wrong-answer mode. runner-dispatch.sh refuses it first; this is the DEFENSIVE TWIN
# for the direct hand invocation this contract documents, which bypasses the facade.
if [ -n "$BRIEF_FILE" ]; then
  [ $# -eq 0 ] || die "both --brief-file and trailing arguments were given — pass the brief in the file OR after '--', never both"
  [ -f "$BRIEF_FILE" ] && [ -r "$BRIEF_FILE" ] || die "--brief-file '$BRIEF_FILE' is not a readable file"
  [ -s "$BRIEF_FILE" ] || die "--brief-file '$BRIEF_FILE' is empty — a child launched with no task does not error, it improvises"
fi
```

Replace each adapter's prompt-payload block — the

```bash
if [ $# -gt 0 ]; then
  prompt="$prompt

Additional caller arguments / task context:
$*"
fi
```

block — with:

```bash
# The payload, from whichever channel carries it. `$*` joined the positional parameters on the
# first character of IFS, so a multi-line brief passed as several arguments was flattened to one
# line and its plan-task structure, code blocks, and file lists all lost their boundaries —
# silently. The loop below preserves both order and line structure, so the surviving argv path
# stops being lossy even though the shim no longer teaches it.
# The brief file is appended VERBATIM via command substitution: a model-authored brief is untrusted
# input holding single quotes, backslashes, `%`, and backticks, so it must never pass through
# `eval` or a `printf` format string.
payload=""
if [ -n "$BRIEF_FILE" ]; then
  payload="$(cat "$BRIEF_FILE")"
elif [ $# -gt 0 ]; then
  payload="$1"; shift
  for a in "$@"; do payload="$payload
$a"; done
fi
if [ -n "$payload" ]; then
  prompt="$prompt

Additional caller arguments / task context:
$payload"
fi
```

- [ ] **Step 4: Run the adapter tests to verify they pass**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep "NOT OK"`
Expected: no output from the `0277 codex:` asserts (facade-level `0277` asserts do not exist yet).

Run: `bash tests/test_runner_opencode.sh` and `bash tests/test_runner_cursor.sh`
Expected: exit 0, no `NOT OK` lines.

- [ ] **Step 5: Mutation-test the two new guards**

1. Restore-safe copy first (`git checkout --` would discard this task's uncommitted work — LEARNINGS `mutation-restore-needs-a-backup-copy`): `cp scripts/runners/codex.sh /tmp/codex.sh.bak`.
2. Revert the join to the lossy form: replace the `payload="$1"; shift` loop body with `payload="$*"`. Run `bash tests/test_runner_dispatch.sh 2>&1 | grep "NOT OK"` — the "NOT space-joined" assert must appear. Restore: `cp /tmp/codex.sh.bak scripts/runners/codex.sh`.
3. Delete the `[ $# -eq 0 ] || die …` line. Run again — the "both channels together are refused" assert must appear. Restore from the backup.
4. Record both mutation readings in the commit message body.

- [ ] **Step 6: Document the flag in the three adapter contracts**

In each of `scripts/runners/{codex,opencode,cursor}.md`, update the `## Usage` synopsis line to:

```
bash scripts/runners/<name>.sh --agent <name> [--model <m>] [--effort <e>] [--brief-file <path>] [--] [<args…>]
```

and add a bullet under it (adapt the adapter name):

```markdown
- `--brief-file <path>` (optional, change 0277) — the caller's task brief, read from a file and
  appended to the prompt **verbatim** under the `Additional caller arguments / task context:`
  heading. Preferred over trailing argv: the caller writes the file with a quoted-delimiter
  heredoc, so nothing about the brief is shell-quoted by a model and nothing is joined or
  reflowed. The file must exist, be readable, and be non-empty. **A brief file and trailing
  arguments together are refused** — passing both would silently drop or duplicate the child's
  only input, so this adapter dies rather than picking one. `runner-dispatch.sh` refuses the same
  shape first; this is the defensive twin for the hand invocation documented here.
- `-- <args…>` — the legacy payload channel, still supported and no longer lossy: the arguments
  are joined on a **newline** in order (they were previously interpolated with `$*`, which joins
  on the first character of `IFS` and flattened a multi-line brief onto one line).
```

- [ ] **Step 7: Commit**

```bash
git add scripts/runners/codex.sh scripts/runners/opencode.sh scripts/runners/cursor.sh \
        scripts/runners/codex.md scripts/runners/opencode.md scripts/runners/cursor.md \
        tests/test_runner_dispatch.sh tests/test_runner_opencode.sh tests/test_runner_cursor.sh
git commit -m "feat(0277): adapters take a brief file and stop flattening argv with \$*"
```

---

### Task 2: Facade — `--brief-file`, the XOR refusal, and the `build-*` empty-payload gate

**Files:**
- Modify: `scripts/runner-dispatch.sh` (parse loop ~lines 65–87; `build-*` gate ~lines 118–126; synchronous handoff ~line 949)
- Modify: `scripts/runner-dispatch.md` (`## Usage`, `## Behavior`, `## Invariants`)
- Test: `tests/test_runner_dispatch.sh` (facade sections)

**Interfaces:**
- Consumes: Task 1's adapter CLI — `--brief-file <path>` and the both-channels refusal.
- Produces: `BRIEF_FILE` (the caller's validated path, empty when absent) and `BRIEF_PATH` (the path actually handed to the adapter — the caller's file on the synchronous verb, reassigned to the durable spooled copy by Task 3). Tasks 3 and 4 read `BRIEF_PATH`.

- [ ] **Step 1: Write the failing facade tests**

Append to `tests/test_runner_dispatch.sh`, immediately before the final `exit $fail` line's preceding gate section (place it after the `# ---- facade: runners.<name> config resolution across layers ----` section so the fixtures stay grouped):

```bash
# ---- 0277: the facade's brief-file channel ------------------------------------------
# Same two channels as the adapters, refused in the same shape, but refused HERE FIRST so the
# facade can never construct the invocation its own adapters would reject.
make_fixture
BF="$SBX/brief.txt"
printf 'facade-brief-line-one\nfacade-brief-line-two\n' > "$BF"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status --brief-file "$BF" >/dev/null 2>&1 )
assert "0277 facade: the brief reaches the child's prompt" 'grep -qxF -- "facade-brief-line-one" "$LOG"'
assert "0277 facade: the brief keeps its line structure" 'grep -qxF -- "facade-brief-line-two" "$LOG"'

err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --brief-file "$BF" -- "and argv too" 2>&1 >/dev/null )"; rc=$?
assert "0277 facade: both channels together are refused" '[ "$rc" != "0" ]'
assert "0277 facade: the refusal says never both" 'grep -qiF "never both" <<<"$err"'

err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --brief-file "$SBX/no-such-brief" 2>&1 >/dev/null )"; rc=$?
assert "0277 facade: a missing brief file is refused" '[ "$rc" != "0" ]'
assert "0277 facade: the missing-file refusal names the path" 'grep -qF -- "no-such-brief" <<<"$err"'
: > "$SBX/empty-brief"
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --brief-file "$SBX/empty-brief" 2>&1 >/dev/null )"; rc=$?
assert "0277 facade: an empty brief file is refused" '[ "$rc" != "0" ]'
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status --brief-file 2>&1 >/dev/null )"; rc=$?
assert "0277 facade: --brief-file with no value exits instead of spinning" '[ "$rc" != "0" ]'
rm -rf "$SBX"

# The build-* empty-payload gate, at the SAME pre-verb point as the --worktree gate, so it holds
# for the legacy foreground verb (the hand-invocation path, the one most likely to be typed
# task-less) exactly as it does for --launch. A build worker with no task is always the
# silent-improvise defect; a loud abort is strictly better than a successful-looking task-less run.
make_fixture
mkdir -p "$SBX/.worktrees/w"
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy \
    --worktree "$SBX/.worktrees/w" 2>&1 >/dev/null )"; rc=$?
assert "0277 gate: build-* with NO payload is refused" '[ "$rc" != "0" ]'
assert "0277 gate: the refusal names the improvise failure mode" 'grep -qiE "improvis|no task" <<<"$err"'
assert "0277 gate: the refusal never reached the child" '[ ! -s "$LOG" ]'
# ... and it is satisfied by EITHER channel.
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy \
    --worktree "$SBX/.worktrees/w" -- "do the task" >/dev/null 2>&1 ); rc=$?
assert "0277 gate: build-* WITH argv payload runs" '[ "$rc" = "0" ]'
: > "$LOG"
printf 'do the task\n' > "$SBX/brief.txt"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy \
    --worktree "$SBX/.worktrees/w" --brief-file "$SBX/brief.txt" >/dev/null 2>&1 ); rc=$?
assert "0277 gate: build-* WITH a brief file runs" '[ "$rc" = "0" ]'
# SCOPED to build-*: a metadata-scoped agent legitimately dispatches payload-free.
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 ); rc=$?
assert "0277 gate: a non-build agent with no payload still runs" '[ "$rc" = "0" ]'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep "NOT OK"`
Expected: the `0277 facade:` and `0277 gate:` asserts fail (`--brief-file` is an unknown argument at the facade; a payload-free `build-economy` currently succeeds).

- [ ] **Step 3: Implement the facade parsing, validation, and gate**

In `scripts/runner-dispatch.sh`, extend the initialization line (currently `RUNNER=""; AGENT=""; MODEL=""; EFFORT=""; WORKTREE=""`):

```bash
RUNNER=""; AGENT=""; MODEL=""; EFFORT=""; WORKTREE=""; BRIEF_FILE=""
```

Add this arm to the parse loop, above `--)`:

```bash
    # Same last-argument hazard as `--observe` above: shift the flag, then the value only if a
    # value is actually there, so the "requires a path" refusal below stays reachable.
    --brief-file) BRIEF_FILE="${2:-}"; shift; [ $# -gt 0 ] && shift ;;
```

Extend the `build-*` gate block (currently a one-line `case`) to:

```bash
# --- change 0277: payload validation, at the SAME pre-verb point as gate 1 ------------
# Both payload channels are known here — `--brief-file` from the parse loop, the trailing argv as
# the surviving positional parameters — which is why these gates sit here rather than inside a
# verb: scoping them to `--launch` would leave the legacy foreground verb, the hand-invocation
# path, free to dispatch a task-less build worker silently.
if [ -n "$BRIEF_FILE" ]; then
  # BOTH CHANNELS ⇒ REFUSE. Preferring either silently drops or duplicates the child's entire
  # input, and concatenating invents an ordering; refusal is the only shape with no
  # silent-wrong-answer mode. The adapters carry the same refusal defensively.
  [ $# -eq 0 ] || die "both --brief-file and trailing arguments after '--' were given — pass the brief in the file OR after '--', never both"
  [ -f "$BRIEF_FILE" ] && [ -r "$BRIEF_FILE" ] || die "--brief-file '$BRIEF_FILE' is not a readable file"
  [ -s "$BRIEF_FILE" ] || die "--brief-file '$BRIEF_FILE' is empty — a child launched with no task does not error, it improvises"
fi
case "$AGENT" in
  build-*)
    [ -n "$WORKTREE" ] || die "--worktree is required for build-* agents (a build worker must run in its feature worktree, not the main tree)"
    # A build worker with NO task at all is always the improvise defect change 0271 documented:
    # it does not error, it invents work from whatever it can see in the worktree, and the
    # dispatch still looks successful. Loud here; non-build agents (status, adr, …) legitimately
    # dispatch payload-free and stay silent.
    [ -n "$BRIEF_FILE" ] || [ $# -gt 0 ] || die "a build-* dispatch carries no task: pass the brief with --brief-file <path> (preferred) or after '--'. A build worker launched with no task does not error — it improvises from whatever it finds in the worktree and the dispatch still looks successful" ;;
esac
# The path actually handed to the adapter. It is the caller's file on the legacy synchronous verb
# (nothing detaches, so there is no temp-file lifetime hazard); `--launch` reassigns it to the
# durable spooled copy in the dispatch dir.
BRIEF_PATH="$BRIEF_FILE"
```

Note: the `case` arm's original single-line body moves into the multi-line form above; keep the original `--worktree` diagnostic string byte-identical so its existing assert (`0206: build-* rejection names --worktree`) stays green.

Replace the synchronous handoff (the `"$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"` line above `rc=$?`) with:

```bash
# ONE channel, always: the facade never hands an adapter the both-channels shape its own
# defensive gate refuses. With a brief file the argv channel is empty by construction (the gate
# above refused any trailing argument), so the `--` terminator is passed with nothing after it.
if [ -n "$BRIEF_PATH" ]; then
  "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" --brief-file "$BRIEF_PATH" --
else
  "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
fi
rc=$?
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch.sh`
Expected: exit 0, no `NOT OK` lines (Task 1's adapter asserts and Task 2's facade asserts all green).

- [ ] **Step 5: Mutation-test the gate**

`cp scripts/runner-dispatch.sh /tmp/rd.sh.bak`. Delete the `[ -n "$BRIEF_FILE" ] || [ $# -gt 0 ] || die …` line, run `bash tests/test_runner_dispatch.sh 2>&1 | grep "NOT OK"`, and confirm `0277 gate: build-* with NO payload is refused` reddens. Restore with `cp /tmp/rd.sh.bak scripts/runner-dispatch.sh`. Record the reading in the commit body.

- [ ] **Step 6: Document in `scripts/runner-dispatch.md`**

Update the `## Usage` synopsis to:

```
docket.sh runner-dispatch [--launch] --runner <name> --agent <agent> [--model <m>] [--effort <e>] [--worktree <path>] [--brief-file <path>] [--] [<args…>]
docket.sh runner-dispatch --observe <key> --runner <name> --agent <agent> [--worktree <path>]
```

Add a bullet after the `--worktree` bullet:

```markdown
- `--brief-file <path>` (optional, change 0277) — the caller's task brief, read from a file
  instead of shell argv. The caller writes it with a quoted-delimiter heredoc, so no part of the
  brief is quoted by a model and none of it is joined or reflowed on the way to the child. The
  file must exist, be readable, and be non-empty. **Mutually exclusive with trailing `--`
  arguments**: passing both is a loud refusal, because preferring either channel silently drops
  or duplicates the child's entire input and concatenating them invents an ordering. Under
  `--launch` the brief is spooled into the per-dispatch directory as `brief` and the adapter is
  handed that durable copy; on the legacy foreground verb the caller's file is passed through.
- **`build-*` agents require a payload.** A `build-*` dispatch carrying neither a brief file nor
  trailing arguments is refused at the same pre-verb validation point as the `--worktree` gate —
  so the rule holds for `--launch` and the legacy verb alike. A build worker with no task does not
  error; it improvises from whatever is in the worktree and the dispatch still looks successful.
  Non-`build-*` agents (status, adr, …) legitimately dispatch payload-free and are unaffected.
```

Add to `## Invariants`:

```markdown
- **One payload channel per dispatch.** A brief file and trailing argv are never both forwarded to
  an adapter — the facade refuses the shape up front, and every handoff site (synchronous,
  `--launch`, and the run gate's re-dispatch) constructs a single-channel invocation.
```

- [ ] **Step 7: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_runner_dispatch.sh
git commit -m "feat(0277): facade takes --brief-file, refuses both channels, gates payload-less build dispatches"
```

---

### Task 3: `--launch` spools the brief into `$DDIR/brief`

**Files:**
- Modify: `scripts/runner-dispatch.sh` (the `if [ "$VERB" = "launch" ]` block, after `DDIR="$DROOT/$KEY"`; the detached handoff at `"$ADAPTER" "${args[@]}" -- "$@"` inside the backgrounded block)
- Modify: `scripts/runner-dispatch.md` (`### Launch (change 0271)` section)
- Modify: `tests/lib/runner_dispatch_detach_common.sh` (fake adapter records argv; `launch()` forwards extra args)
- Test: `tests/test_runner_dispatch_detach.sh`

**Interfaces:**
- Consumes: Task 2's `BRIEF_FILE` / `BRIEF_PATH`.
- Produces: `$DDIR/brief` — a byte-identical copy of the caller's brief inside the per-dispatch directory, and `BRIEF_PATH` reassigned to it before the adapter is started.

- [ ] **Step 1: Extend the detach fixture so argv is observable**

In `tests/lib/runner_dispatch_detach_common.sh`, change the fake adapter heredoc body to record its argv, adding this as the FIRST line after the shebang inside the `FAKE` heredoc:

```bash
printf '%s\n' "$@" >> "${FAKE_ARGV_LOG:-/dev/null}"
```

and change the `launch()` helper to forward extra arguments:

```bash
launch(){ local agent="${1:-status}"; shift 2>/dev/null || true
  ( cd "$SBX" && RUNNERS_DIR="$RDIR" FAKE_MARKER="$SBX/marker" \
    FAKE_ARGV_LOG="${FAKE_ARGV_LOG:-/dev/null}" \
    FAKE_SLEEP="${FAKE_SLEEP:-0}" FAKE_TAIL="${FAKE_TAIL:-0}" FAKE_RC="${FAKE_RC:-0}" \
    bash "$FACADE" --launch --runner fake --agent "$agent" "$@" ); }
```

- [ ] **Step 2: Write the failing spool test**

Append to `tests/test_runner_dispatch_detach.sh`, before its final `exit $fail`:

```bash
# ---- 0277: --launch spools the brief into the dispatch dir ---------------------------
# The brief becomes part of the dispatch's audit record, beside `launch`, `stdout.log`, and
# `done` — and the adapter is handed the DURABLE copy, so a detached child no longer depends on
# the caller's temp file outliving the call that started it.
make_fixture
FAKE_SLEEP=0
BF="$SBX/caller-brief.txt"
printf 'spooled-line-one\nspooled-line-two\n' > "$BF"
FAKE_ARGV_LOG="$SBX/argv.log"
KEY="$(launch status --brief-file "$BF")"; rc=$?
assert "0277 launch: a brief-file launch exits 0" '[ "$rc" = "0" ]'
DDIR="$(ddir_for "$KEY")"
# Wait for the (instant) child so its argv log is complete before reading it.
for _ in 1 2 3 4 5 6 7 8 9 10; do [ -f "$DDIR/done" ] && break; sleep 0.3; done
assert "0277 launch: the brief was spooled into the dispatch dir" '[ -f "$DDIR/brief" ]'
assert "0277 launch: the spooled brief is byte-identical to the caller's" 'cmp -s "$BF" "$DDIR/brief"'
assert "0277 launch: no partial file is left behind" '[ ! -e "$DDIR/brief.partial" ]'
argv="$(cat "$SBX/argv.log" 2>/dev/null)"
assert "0277 launch: the adapter was handed the DURABLE copy" 'grep -qxF -- "$DDIR/brief" <<<"$argv"'
assert "0277 launch: the adapter was NOT handed the caller's path" '! grep -qxF -- "$BF" <<<"$argv"'
rm -rf "$SBX"

# The exclusion and the build-* payload gate are pre-verb, so they refuse BEFORE anything is
# minted — a refused dispatch leaves no dispatch dir behind.
make_fixture
printf 'a brief\n' > "$SBX/b.txt"
before="$(ls "$(ddir_for "" )" 2>/dev/null | wc -l | tr -d ' ')"
err="$( launch status --brief-file "$SBX/b.txt" -- "argv too" 2>&1 >/dev/null )"; rc=$?
after="$(ls "$(ddir_for "" )" 2>/dev/null | wc -l | tr -d ' ')"
assert "0277 launch: both channels are refused" '[ "$rc" != "0" ]'
assert "0277 launch: the refusal minted no dispatch dir" '[ "$before" = "$after" ]'
rm -rf "$SBX"
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `bash tests/test_runner_dispatch_detach.sh 2>&1 | grep "NOT OK"`
Expected: `0277 launch: the brief was spooled into the dispatch dir` and the durable-copy asserts fail (nothing spools today).

- [ ] **Step 4: Implement the spool**

In `scripts/runner-dispatch.sh`, inside the `if [ "$VERB" = "launch" ]; then` block, immediately after `DDIR="$DROOT/$KEY"`, add:

```bash
  # THE BRIEF, SPOOLED (change 0277). Written atomically — a temp file BESIDE its destination then
  # `mv -f`, the same shape as `launch`, `done`, and `gate-before` — so a reader never sees a
  # half-written brief. Two things are bought: the detached child no longer depends on the
  # caller's temp file outliving this call, and the dispatch record gains its INPUT alongside its
  # output. Retention rides `docket_dispatch_prune`, which already bounds this directory; no new
  # lifecycle is introduced. A spool that cannot be written is a hard failure, not a degrade: the
  # brief is the child's only input, so dispatching without it is the improvise defect.
  if [ -n "$BRIEF_FILE" ]; then
    cat "$BRIEF_FILE" > "$DDIR/brief.partial" || die "cannot spool the brief into $DDIR"
    mv -f "$DDIR/brief.partial" "$DDIR/brief" || die "cannot spool the brief into $DDIR"
    BRIEF_PATH="$DDIR/brief"
  fi
```

Replace the detached handoff line inside the backgrounded block:

```bash
    if [ -n "$BRIEF_PATH" ]; then
      "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" --brief-file "$BRIEF_PATH" --
    else
      "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
    fi
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch_detach.sh`
Expected: exit 0, no `NOT OK` lines.

Run: `bash tests/test_runner_dispatch.sh` and `bash tests/test_runner_dispatch_observe.sh` and `bash tests/test_runner_dispatch_build_gate.sh`
Expected: exit 0 each — the shared fixture edit must not disturb the sibling shards.

- [ ] **Step 6: Document the spool**

In `scripts/runner-dispatch.md`, in the `### Launch (change 0271)` section's description of the per-dispatch directory contents, add `brief` to the file list with:

```markdown
- `brief` — the caller's task brief, spooled here at launch when `--brief-file` was passed
  (change 0277). Written atomically (`brief.partial` + `mv -f`) and handed to the adapter in place
  of the caller's own path, so a detached run never depends on a caller temp file outliving the
  call that started it. Absent when the dispatch carried its payload as trailing argv. It is part
  of the dispatch's audit record and is pruned with the rest of the directory.
```

- [ ] **Step 7: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/runner-dispatch.md \
        tests/lib/runner_dispatch_detach_common.sh tests/test_runner_dispatch_detach.sh
git commit -m "feat(0277): --launch spools the brief into the dispatch dir and hands over the durable copy"
```

---

### Task 4: The run gate's re-dispatch keeps one channel

The synchronous verb's run gate (change 0237) makes one bounded re-dispatch and appends a retry context as an **extra trailing argument**. With a brief file in play that is the both-channels shape the adapters now refuse — the facade would defeat its own gate on a path no caller can see. This task is the reconcile addendum's requirement.

**Files:**
- Modify: `scripts/runner-dispatch.sh` (the re-dispatch line `"$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@" "$retry_ctx"`)
- Modify: `scripts/runner-dispatch.md` (`## Behavior`, the run-gate re-dispatch description)
- Test: `tests/test_runner_dispatch.sh` (the `0237` gate section — `make_gate_fixture`)

**Interfaces:**
- Consumes: Task 2's `BRIEF_PATH`, Task 3's spooled copy.
- Produces: no new interface; the re-dispatch's argument shape becomes single-channel.

- [ ] **Step 1: Write the failing test**

Append a new case to the `0237` gate section in `tests/test_runner_dispatch.sh`, after case (h):

```bash
# (i) 0277: the re-dispatch must not open a SECOND payload channel. The gate appends its retry
#     context as trailing argv; with a brief file in play that is exactly the both-channels shape
#     the adapters refuse, so the facade would defeat its own gate on a path no caller can see.
#     The retry context rides INSIDE a combined brief instead — never dropped, never a second
#     channel.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "9 $FUT" > "$SNAP/after.1"; printf '%s\n' "9 $FUT" > "$SNAP/after.2"
printf 'run-incomplete 9 pr\n' > "$SNAP/verdict.9"
BF="$SBX/gate-brief.txt"
printf 'original-brief-line\n' > "$BF"
run_gate --runner ad --agent implement-next --brief-file "$BF" >/dev/null 2>&1
# ad.sh logs "$*" per invocation, one line per run: line 1 = first dispatch, line 2 = re-dispatch.
first="$(sed -n 1p "$SBX/ad.log")"
second="$(sed -n 2p "$SBX/ad.log")"
assert "0277 redispatch: the gate re-dispatched exactly once" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "2" ]'
assert "0277 redispatch: the first dispatch used the brief channel" 'grep -qF -- "--brief-file" <<<"$first"'
assert "0277 redispatch: the re-dispatch also used the brief channel" 'grep -qF -- "--brief-file" <<<"$second"'
# THE DEFECT ASSERT: no trailing argv rides alongside the brief file.
assert "0277 redispatch: the re-dispatch appended NO trailing argv" \
  '! grep -qF -- "Step 7 unmet" <<<"$second"'
# ... and the retry context is not lost — it is inside the brief the adapter was handed.
retry_brief="$(sed -n 's/.*--brief-file \([^ ]*\).*/\1/p' <<<"$second")"
assert "0277 redispatch: the re-dispatch brief still carries the original brief" \
  '[ -f "$SBX/redispatch-brief-copy" ] && grep -qxF -- "original-brief-line" "$SBX/redispatch-brief-copy"'
assert "0277 redispatch: the re-dispatch brief carries the retry context" \
  'grep -qF -- "Step 7 unmet" "$SBX/redispatch-brief-copy"'
rm -rf "$SBX"
```

The combined brief is a temp file the facade deletes after the re-dispatch returns, so the fake adapter must snapshot it while it exists. Extend `make_gate_fixture`'s `ad.sh` heredoc with these lines before its `exit`:

```bash
# 0277: snapshot any brief handed to us, so a case can assert on a file the facade deletes after
# the call returns.
prev=""; for a in "$@"; do [ "$prev" = "--brief-file" ] && cp "$a" "${SBX_COPY:?}" 2>/dev/null; prev="$a"; done
```

and export `SBX_COPY="$SBX/redispatch-brief-copy"` in `run_gate`'s environment (read `run_gate`'s definition and add it beside the existing `AD_LOG` / `SNAP_DIR` exports). Because the first dispatch also copies, the second copy overwrites it — which is what the asserts read.

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | grep "NOT OK"`
Expected: `0277 redispatch: the re-dispatch appended NO trailing argv` fails (today the retry context is appended as argv beside `--brief-file`).

- [ ] **Step 3: Implement the combined-brief re-dispatch**

Replace the re-dispatch line in `scripts/runner-dispatch.sh`:

```bash
  # ONE CHANNEL ON THE RETRY TOO (change 0277). The retry context used to ride as an extra
  # trailing argument, which — with a brief file in play — is exactly the both-channels shape the
  # adapters refuse, so the facade would kill its own re-dispatch. Instead the context is appended
  # to a COMBINED brief: the original brief's bytes verbatim, a blank line, then the retry
  # context. Never dropped, never a second channel. Templated into TMPDIR per the repo's mktemp
  # rule; removed when the re-dispatch returns.
  if [ -n "$BRIEF_PATH" ]; then
    RETRY_BRIEF="$(mktemp "${TMPDIR:-/tmp}/docket-retry-brief.XXXXXX")" || die "cannot create the re-dispatch brief"
    { cat "$BRIEF_PATH"; printf '\n%s\n' "$retry_ctx"; } > "$RETRY_BRIEF" \
      || die "cannot write the re-dispatch brief $RETRY_BRIEF"
    "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" --brief-file "$RETRY_BRIEF" --
    rm -f "$RETRY_BRIEF"
  else
    "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@" "$retry_ctx"
  fi
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch.sh`
Expected: exit 0, no `NOT OK` lines — including every pre-existing `0237` case, which exercises the `else` branch unchanged.

- [ ] **Step 5: Mutation-test**

`cp scripts/runner-dispatch.sh /tmp/rd4.sh.bak`. Change the brief branch's invocation to the old shape (`… --brief-file "$RETRY_BRIEF" -- "$retry_ctx"`), run `bash tests/test_runner_dispatch.sh 2>&1 | grep "NOT OK"`, and confirm the `appended NO trailing argv` assert reddens. Restore from the backup and record the reading in the commit body.

- [ ] **Step 6: Document**

In `scripts/runner-dispatch.md`'s run-gate re-dispatch description, add:

```markdown
When the dispatch carried its payload as a **brief file**, the re-dispatch does not append the
retry context as an extra argument — that would present both payload channels at once, the shape
the adapters refuse. It composes a **combined brief** instead (the original brief verbatim, a
blank line, then the retry context) in a temporary file, re-dispatches with `--brief-file`
pointing at it, and removes it when the call returns. With no brief file the re-dispatch keeps its
original trailing-argv shape.
```

- [ ] **Step 7: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_runner_dispatch.sh
git commit -m "fix(0277): the run gate's re-dispatch carries its retry context inside the brief"
```

---

### Task 5: The shim teaches one path — heredoc brief, then `--brief-file`

**Files:**
- Modify: `sync-agents.sh` (`emit_shim`, its preceding comment block, and STEP 1 of the `cat <<SHIM` heredoc)
- Test: `tests/test_sync_agents_runners.sh` (the `0271` shim-sentinel block)

**Interfaces:**
- Consumes: Task 2's facade CLI (`--brief-file`).
- Produces: generated shim text containing a `mktemp` + quoted-delimiter heredoc write step and an **unbracketed** `--brief-file <…>` slot on the launch line; the argv slot and the single-quoting paragraph are gone.

**Note:** `emit_shim` writes through an **unquoted** heredoc delimiter (`cat <<SHIM`), so `$`, backticks, and `\` in the new text must be escaped exactly as the surrounding lines escape them (`\$`, `` \` ``). The delimiter line `DOCKET_BRIEF_EOF` inside the emitted text needs no escaping.

- [ ] **Step 1: Rewrite the shim sentinels (failing first)**

In `tests/test_sync_agents_runners.sh`, replace the `launch_line` assert block. Delete these three asserts, which pin the argv teaching this change removes:

- `"0271: launch line ends with an unbracketed single-quoted task-text slot"`
- `"0271: shim requires the task text as ONE argument"`
- (keep `"0271: shim no longer renders the task slot as optional brackets"` — the bracketed spelling must still never reappear)

and add:

```bash
# change 0277: the task brief no longer travels as shell argv. The shim teaches ONE path — write
# the brief with a quoted-delimiter heredoc, then launch with --brief-file — because two taught
# paths let the model pick the lossy one. The slot stays UNBRACKETED for 0271's reason: a
# bracketed rendering reads as optional and was in fact dropped on live dispatches.
assert "0277: shim teaches an explicitly templated mktemp for the brief" \
  'grep -qF -- "TMPDIR:-/tmp" "$G"'
assert "0277: shim teaches a QUOTED-delimiter heredoc (every character literal)" \
  'grep -qF -- "<<'\''DOCKET_BRIEF_EOF'\''" "$G"'
assert "0277: shim closes the heredoc" 'grep -qxF -- "DOCKET_BRIEF_EOF" "$G"'
launch_line="$(grep -F -- "--launch" "$G")"
assert "0277: launch line ends with an unbracketed --brief-file slot" \
  'grep -qE -- "--brief-file <[^>]+>[[:space:]]*$" <<<"$launch_line"'
# DETECTS THE REMOVAL (LEARNINGS: assert-detects-removal-not-replacement): the argv payload slot
# and its quoting instructions are the defect, so neither may survive anywhere in the shim.
assert "0277: shim no longer renders a trailing single-quoted argv task slot" \
  '! grep -qE -- "-- '\''<[^>]+>'\''" "$G"'
assert "0277: shim no longer teaches the ONE-single-quoted-argument workaround" \
  '! grep -qiE "one single-quoted argument|as ONE .*argument" "$G"'
assert "0277: shim no longer teaches quote-escape gymnastics" '! grep -qF -- "'\''\\\\'\'''\''" "$G"'
# MIRROR correspondence: observe takes a KEY, never the brief.
observe_line="$(grep -F -- "--observe" "$G")"
assert "0277: observe line carries no brief slot" '! grep -qF -- "--brief-file" <<<"$observe_line"'
```

Keep the surviving `0271` asserts untouched: `--launch`/`--observe` baked, exit-4 pairing, no-inline-fallback, no-silent-retry, exactly two `docket.sh runner-dispatch` invocations, `! grep -qF -- "[--" "$G"`, and `"0271: shim names the omission failure as silent"`.

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_sync_agents_runners.sh 2>&1 | grep "NOT OK"`
Expected: the `0277:` asserts fail (the shim still teaches argv).

- [ ] **Step 3: Rewrite `emit_shim`'s STEP 1**

Replace the comment block above `emit_shim()` (the paragraph beginning "The task-text slot after `--` is baked UNBRACKETED") with:

```bash
# The brief slot is baked UNBRACKETED and carries its own emphatic rule, exactly like the
# `<feature worktree>` slot below. Both are required inputs the model must fill from its caller,
# and both fail SILENTLY when omitted — a task-less child improvises from the worktree and the
# dispatch still looks successful. The earlier `[-- <caller args>]` spelling read as optional and
# was in fact dropped on live dispatches, so an optional-looking rendering of a required slot is a
# defect here, not a style choice (change 0271).
# Change 0277: the brief travels as a FILE, not as shell argv. The old form asked the model to
# perform a lossy, unverified transformation — quote a multi-line brief into one shell argument,
# every time, correctly — and the adapters then joined multiple arguments on whitespace. A
# quoted-delimiter heredoc removes the entire quoting burden, and the facade refuses a payload-less
# `build-*` dispatch outright, so the omission mode is now partly mechanical rather than only
# narrated. ONE path is taught: two would let the model pick the lossy one.
```

Replace STEP 1 in the `cat <<SHIM` heredoc (from `STEP 1 — launch. Make a single foreground Bash call:` through the paragraph ending `confirm \`--\` is present and the text after it is complete.`) with:

```
STEP 1 — write the brief, then launch. Two foreground Bash calls, in this order.

1a. WRITE THE BRIEF. Your caller's task text goes into a file, never onto the command line:

    BRIEF="\$(mktemp "\${TMPDIR:-/tmp}/docket-brief.XXXXXX")"
    cat > "\$BRIEF" <<'DOCKET_BRIEF_EOF'
<THE TASK TEXT YOUR CALLER GAVE YOU>
DOCKET_BRIEF_EOF

The quoted delimiter makes every character between the two DOCKET_BRIEF_EOF lines literal:
nothing is expanded, nothing needs escaping, and no quote inside the text needs any handling
at all. Paste your caller's task text IN FULL — every plan task, change id, path, and resume
note — copied verbatim, never summarized, never trimmed, and never reworded to make quoting
easier. If the text itself contains a line reading exactly DOCKET_BRIEF_EOF, pick a different
delimiter; never trim the text to avoid it.

1b. LAUNCH with that file:

    "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch --launch $flags$wt_slot --brief-file <the path you just wrote>

The brief file is the ONLY way the child learns what to do. It inherits no conversation, no
plan, and no task from you: what is in that file is all it will ever see. This command is
INCOMPLETE until you replace the placeholder (drop the angle brackets) with the path from step
1a. Omit \`--brief-file\` ONLY when your caller handed you no task text at all — and for a
build worker the facade will refuse that dispatch outright. Getting this wrong FAILS SILENTLY:
a child launched with no task does not error, it improvises from whatever it can see in the
worktree and the dispatch still looks successful. Before you send the call, re-read it and
confirm \`--brief-file\` is present and that the file holds your caller's full task text.
```

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_sync_agents_runners.sh`
Expected: exit 0, no `NOT OK` lines.

- [ ] **Step 5: Mutation-test the unbracketed slot**

`cp sync-agents.sh /tmp/sa.sh.bak`. Re-bracket the slot in the emitted launch line (`[--brief-file <the path you just wrote>]`), run `bash tests/test_sync_agents_runners.sh 2>&1 | grep "NOT OK"`, and confirm both `0277: launch line ends with an unbracketed --brief-file slot` and `0271: shim no longer renders the task slot as optional brackets` redden. Restore from the backup; record the reading in the commit body.

- [ ] **Step 6: Eyeball one generated shim**

Run (hermetic, in a throwaway dir):

```bash
d="$(mktemp -d "${TMPDIR:-/tmp}/shimcheck.XXXXXX")"; cd "$d" && git init -q && git commit -q --allow-empty -m init
mkdir -p .claude
printf 'agents:\n  claude:\n    build-standard: { model: gpt-5.1-codex, effort: high, runner: codex }\n' > .docket.yml
DOCKET_HARNESS_ROOT="$d" bash /Users/homer/dev/docket/.worktrees/delegated-task-briefs-travel-through-shell-argv-a-lossy-mode/sync-agents.sh >/dev/null 2>&1
cat .claude/agents/docket-build-standard.md
```

Confirm by reading: the heredoc block is syntactically valid shell, the `--brief-file` slot and the `--worktree` slot both appear unbracketed on the launch line, and no `$`-expansion leaked from the generator (no empty `BRIEF=""`, no missing `${TMPDIR:-/tmp}`). Then `rm -rf "$d"`.

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_runners.sh
git commit -m "feat(0277): the shim teaches a heredoc brief file, not a quoted argv payload"
```

---

### Task 6: Re-measure and re-budget `tests/test_runner_dispatch.sh`, then run the suite

`tests/test_runner_dispatch.sh` sat at **10s contended against its own 10s ceiling** before this change — zero headroom — and Tasks 1, 2 and 4 all add cases to it. A trailing `OVER BUDGET: test_runner_dispatch` line from the suite is **this change's finding**, not pre-existing noise (LEARNINGS `budget-headroom-is-spent-before-it-is-breached`).

Separately: `tests/test_sync_agents_runners.sh` is over its own budget for reasons that predate this drain and are tracked as change **#0280**. Do not touch its row.

**Files:**
- Modify: `tests/runtime-budgets.tsv` (the `tests/test_runner_dispatch.sh` row only)

**Interfaces:** none.

- [ ] **Step 1: Run the whole suite**

Run: `bash scripts/run-tests.sh 2>&1 | tail -40`
Expected: every file passes. Read the trailing `OVER BUDGET:` line if present and note which files it names.

- [ ] **Step 2: Measure the file**

Run: `bash scripts/profile-one-test.sh tests/test_runner_dispatch.sh` (read `scripts/profile-one-test.md` first for its interface). Record both the serial number and the contended number the suite reported. The **contended** number is the one the ceiling is compared against.

- [ ] **Step 3: Raise the row with the measured number**

Edit the `tests/test_runner_dispatch.sh` row in `tests/runtime-budgets.tsv` to a ceiling with real headroom above the measured contended number (round up to leave at least ~50% margin; the table's hard rule is that **no row may exceed 60 seconds** — if the measurement would need more, shard the file along its own seams instead, the way `tests/lib/runner_dispatch_detach_common.sh` records). Read the file's header comment first and follow whatever provenance convention it documents for a changed row.

- [ ] **Step 4: Re-run the suite and confirm the budget line is clean**

Run: `bash scripts/run-tests.sh 2>&1 | tail -40`
Expected: all files pass; no `OVER BUDGET:` line naming `test_runner_dispatch`.

- [ ] **Step 5: Commit**

```bash
git add tests/runtime-budgets.tsv
git commit -m "test(0277): re-budget test_runner_dispatch with a measured number"
```

---

## Self-Review

**Spec coverage.**

| Spec section | Task |
|---|---|
| §1 `--brief-file` on both verbs, validated at the facade | 2 (validation, legacy verb), 3 (`--launch`) |
| §1 `--launch` spools to `$DDIR/brief` atomically, adapter gets the durable copy | 3 |
| §2 XOR refuse at the facade AND defensively in the adapters | 2 (facade), 1 (adapters) |
| §3 `build-*` empty-payload gate, verb-neutral, at the pre-verb point | 2 |
| §4 adapters: `$*` → newline-preserving `"$@"` join; brief appended verbatim | 1 |
| §5 shim: heredoc write + unbracketed `--brief-file`, argv teaching and quoting paragraph deleted | 5 |
| §6 brief lifecycle = the dispatch dir's existing prune (no new mechanism) | 3 (documented; no code) |
| Testing: facade spool/XOR/missing/empty/build-gate | 2, 3 |
| Testing: adapters, fixture content with quotes/backslashes/`%`/backticks/`-- --flag` | 1 |
| Testing: shim sentinels + re-bracket mutation | 5 |
| Reconcile addendum: run-gate re-dispatch single channel | 4 |
| Reconcile addendum: re-measure and raise the budget row | 6 |
| Named human verification item (fresh-session live dispatch) | recorded in the results file at close-out — **not** claimable by this run: the harness loads agent definitions at process start, so a shim change cannot be validated by the session that made it (LEARNINGS `generated-artifact-loaded-at-process-start`; proven on 0271) |

**Placeholder scan:** every step names exact files, exact code, and an exact command with an expected result. No "add error handling", no "similar to Task N".

**Type consistency:** `BRIEF_FILE` (the caller's validated path) and `BRIEF_PATH` (the path handed to the adapter) are introduced in Task 2 and used under those exact names in Tasks 3 and 4. The adapters' variable is `BRIEF_FILE` and their local payload variable is `payload` in all three. The flag is spelled `--brief-file` everywhere — facade, adapters, shim, contracts, tests.
