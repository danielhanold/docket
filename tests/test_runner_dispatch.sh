#!/usr/bin/env bash
# tests/test_runner_dispatch.sh — run: bash tests/test_runner_dispatch.sh
# Hermetic: a fake `codex` binary on PATH records argv and mimics the real CLI's
# flag grammar (login status / exec / --output-last-message) — LEARNINGS: a
# tool-output mock must mirror the real tool's shape. The real codex CLI may be
# installed on the build machine, so "binary missing" is simulated via the
# CODEX_BIN seam, never by stripping PATH.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

ADAPTER="$ROOT/scripts/runners/codex.sh"
FACADE="$ROOT/scripts/runner-dispatch.sh"

# --- fixture: sandbox repo + fake codex ---------------------------------------
make_fixture(){  # sets SBX (repo root), BIN (fake-bin dir), LOG (argv log), MSG (final message)
  SBX="$(mktemp -d)"; SBX="$(cd "$SBX" && pwd -P)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  ( cd "$SBX" && git commit --allow-empty -qm init )
  BIN="$SBX/fakebin"; LOG="$SBX/codex-argv.log"; MSG="relayed-final-message-$$"
  mkdir -p "$BIN"
  cat > "$BIN/codex" <<FAKE
#!/usr/bin/env bash
# fake codex: mirrors the real grammar. login status -> ok; exec -> record argv,
# write the --output-last-message file, emit event noise on stdout, exit 0.
if [ "\$1" = "login" ] && [ "\$2" = "status" ]; then
  [ -f "$SBX/no-auth" ] && { echo "Not logged in" >&2; exit 1; }
  echo "Logged in using ChatGPT"; exit 0
fi
if [ "\$1" = "exec" ]; then
  shift
  printf '%s\n' "\$@" >> "$LOG"
  out=""; prev=""
  for a in "\$@"; do [ "\$prev" = "--output-last-message" ] && out="\$a"; prev="\$a"; done
  echo "event: task_started (fake codex noise)"
  [ -n "\$out" ] && printf '%s\n' "$MSG" > "\$out"
  [ -f "$SBX/exec-fails" ] && exit 3
  exit 0
fi
echo "fake codex: unexpected argv: \$*" >&2; exit 9
FAKE
  chmod +x "$BIN/codex"
}

run_adapter(){  # $@ = adapter args; SANDBOX_OVERRIDE/NETWORK_OVERRIDE opt in the caller's env
  ( cd "$SBX" && PATH="$BIN:$PATH" DOCKET_REPO_ROOT="$SBX" \
      DOCKET_RUNNER_CFG_SANDBOX="${SANDBOX_OVERRIDE:-}" DOCKET_RUNNER_CFG_NETWORK="${NETWORK_OVERRIDE:-}" \
      bash "$ADAPTER" "$@" )
}

# ---- adapter: happy path ------------------------------------------------------
make_fixture
out="$(run_adapter --agent status --model gpt-5.1-codex --effort high 2>"$SBX/stderr.log")"; rc=$?
argv="$(cat "$LOG")"
assert "adapter exits 0 on success" '[ "$rc" = "0" ]'
assert "stdout is exactly the final message" '[ "$out" = "$MSG" ]'
assert "codex event noise is NOT on stdout" '! grep -qF "task_started" <<<"$out"'
assert "argv: -C flag present" 'grep -qxF -- "-C" <<<"$argv"'
assert "argv: -C repo root value present" 'grep -qxF -- "$SBX" <<<"$argv"'
assert "argv: model passthrough verbatim" 'grep -qxF -- "gpt-5.1-codex" <<<"$argv"'
assert "argv: default sandbox workspace-write" 'grep -qxF -- "workspace-write" <<<"$argv"'
assert "argv: network access -c override present by default" 'grep -qxF -- "sandbox_workspace_write.network_access=true" <<<"$argv"'
assert "argv: effort mapped to model_reasoning_effort" 'grep -qxF -- "model_reasoning_effort=high" <<<"$argv"'
assert "argv: --output-last-message present" 'grep -qxF -- "--output-last-message" <<<"$argv"'
assert "argv: exactly one exec call recorded" '[ "$(grep -cxF -- "--output-last-message" "$LOG")" = "1" ]'
# prompt content (the prompt is multiline — grep the whole recorded argv log; the
# strings below appear only in the prompt, never in a flag value)
assert "prompt names skill docket-status" 'grep -qF "docket-status" "$LOG"'
assert "prompt names skill docket-convention" 'grep -qF "docket-convention" "$LOG"'
assert "prompt carries the wrapper body (abort-and-report)" 'grep -qi "abort-and-report" "$LOG"'
rm -rf "$SBX"

# ---- adapter: effort mapping + omissions --------------------------------------
make_fixture
run_adapter --agent status >/dev/null 2>&1
argv="$(cat "$LOG")"
assert "no --model => no -m flag" '! grep -qxF -- "-m" <<<"$argv"'
assert "no --effort => no reasoning-effort override" '! grep -qF "model_reasoning_effort" <<<"$argv"'
: > "$LOG"
run_adapter --agent status --effort max >/dev/null 2>&1
argv="$(cat "$LOG")"
assert "effort max maps to xhigh" 'grep -qxF -- "model_reasoning_effort=xhigh" <<<"$argv"'
rm -rf "$SBX"

# ---- adapter: sandbox/network knobs -------------------------------------------
make_fixture
SANDBOX_OVERRIDE="danger-full-access" NETWORK_OVERRIDE="false" run_adapter --agent status >/dev/null 2>&1
argv="$(cat "$LOG")"
assert "sandbox knob honored" 'grep -qxF -- "danger-full-access" <<<"$argv"'
assert "network=false drops the network override" '! grep -qF "network_access" <<<"$argv"'
rm -rf "$SBX"

# ---- adapter: passthrough args land in the prompt ------------------------------
make_fixture
run_adapter --agent status -- "run the board-only pass" >/dev/null 2>&1
assert "passthrough args reach the prompt" 'grep -qF "run the board-only pass" "$LOG"'
rm -rf "$SBX"

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
# Line numbers captured variable-side, never `grep | head` — a producer piped into an early-exiting
# consumer takes SIGPIPE under pipefail (AGENTS.md).
alpha_hits="$(grep -nxF -- "argv-alpha" "$LOG")"; alpha_ln="$(head -n1 <<<"$alpha_hits")"; alpha_ln="${alpha_ln%%:*}"
beta_hits="$(grep -nxF -- "argv-beta" "$LOG")";  beta_ln="$(head -n1 <<<"$beta_hits")";   beta_ln="${beta_ln%%:*}"
assert "0277 codex: order is preserved (alpha before beta)" '[ "$alpha_ln" -lt "$beta_ln" ]'
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
# BYTES ≠ CONTENT. `-s` counts bytes while `$(cat …)` strips trailing newlines, so a newline-only
# brief passed every gate and yielded an EMPTY payload — the task-context block was suppressed and
# the child ran with NO TASK AT ALL, silently. Both ends must use one predicate.
printf '\n\n' > "$SBX/blank-brief"
err="$( run_adapter --agent status --brief-file "$SBX/blank-brief" 2>&1 >/dev/null )"; rc=$?
assert "0277 codex: a newline-only brief file is refused" '[ "$rc" != "0" ]'
assert "0277 codex: the no-content refusal names the file" 'grep -qF -- "blank-brief" <<<"$err"'
assert "0277 codex: the no-content refusal never reached codex exec" '[ ! -s "$LOG" ]'
# The same gap on the argv leg: arity is not content, so `-- ""` is arguments-present, payload-empty.
err="$( run_adapter --agent status -- "" 2>&1 >/dev/null )"; rc=$?
assert "0277 codex: an empty trailing argv payload is refused" '[ "$rc" != "0" ]'
assert "0277 codex: the empty-argv refusal never reached codex exec" '[ ! -s "$LOG" ]'
# A value-taking flag in FINAL position must not spin the parse loop (the `--observe` hazard).
err="$( run_adapter --agent status --brief-file 2>&1 >/dev/null )"; rc=$?
assert "0277 codex: --brief-file with no value exits instead of spinning" '[ "$rc" != "0" ]'
rm -rf "$SBX"

# ---- adapter: failure postures -------------------------------------------------
make_fixture
touch "$SBX/no-auth"
err="$( run_adapter --agent status 2>&1 >/dev/null )"; rc=$?
assert "unauthenticated codex aborts nonzero" '[ "$rc" != "0" ]'
assert "auth abort names the remedy" 'grep -qi "codex login" <<<"$err"'
assert "auth abort never reached exec" '[ ! -f "$LOG" ]'
rm -f "$SBX/no-auth"
touch "$SBX/exec-fails"
run_adapter --agent status >/dev/null 2>&1; rc=$?
assert "child nonzero exit propagates" '[ "$rc" = "3" ]'
rm -f "$SBX/exec-fails"
err="$( run_adapter --agent no-such-agent 2>&1 >/dev/null )"; rc=$?
assert "unknown agent aborts nonzero" '[ "$rc" != "0" ]'
assert "unknown agent abort names the missing source" 'grep -qF "no-such-agent" <<<"$err"'
err="$( cd "$SBX" && DOCKET_REPO_ROOT="$SBX" CODEX_BIN="definitely-missing-codex-xyz" bash "$ADAPTER" --agent status 2>&1 >/dev/null )"; rc=$?
assert "codex missing (CODEX_BIN seam) aborts nonzero" '[ "$rc" != "0" ]'
assert "missing-binary abort names the install remedy" 'grep -qi "install" <<<"$err"'
rm -rf "$SBX"

# ---- facade: validation ---------------------------------------------------------
make_fixture
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --agent status 2>&1 >/dev/null )"; rc=$?
assert "facade: missing --runner rejected" '[ "$rc" != "0" ]'
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex 2>&1 >/dev/null )"; rc=$?
assert "facade: missing --agent rejected" '[ "$rc" != "0" ]'
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner gemini-cli --agent status 2>&1 >/dev/null )"; rc=$?
assert "facade: unknown runner rejected nonzero" '[ "$rc" != "0" ]'
assert "facade: unknown-runner message names it" 'grep -qF "gemini-cli" <<<"$err"'
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner ../codex --agent status 2>&1 >/dev/null )"; rc=$?
assert "facade: path-traversal runner name rejected" '[ "$rc" != "0" ]'
assert "facade: traversal rejection says invalid" 'grep -qiF "invalid runner name" <<<"$err"'
rm -rf "$SBX"

# ---- 0208 leg (c): a valueless trailing flag must die, never hang ---------------------
# Every value-taking flag whose arm ends in `shift 2` hangs when the flag is the FINAL argument:
# bash's `shift` FAILS rather than truncating at `$# = 1`, the loop has no trailing shift, and the
# facade runs under `set -uo pipefail` with no `-e`. Measured before the fix:
# `timeout 3 bash scripts/runner-dispatch.sh --runner` returned 124.
#
# The bound is a background job plus a SENTINEL FILE, and both halves are load-bearing:
#   * The stop must be INDEPENDENT of the guard under test, or deleting the guard deletes the stop
#     and the mutation hangs instead of reddening (LEARNINGS: mutation-target-needs-a-forced-exit).
#   * Completion is the sentinel FILE, never `kill -0` on the pid: a finished-but-unwaited child is
#     a zombie whose pid still answers `kill -0`, so a liveness poll would report HUNG for every
#     healthy run — the assert would pass for the wrong reason and go vacuous the moment it is fixed.
#   * `set -m` makes the job a process-group LEADER so the give-up path can signal the whole tree.
#     Without it the subshell dies and the spinning facade is orphaned into the rest of the suite.
# `timeout(1)` is deliberately not used: stock macOS ships none and no existing test depends on one.
#
# The stderr path is derived by `bounded_err_path` rather than assigned inside `run_bounded`,
# because the caller reads the helper's exit code through `$( )` — a SUBSHELL, whose variable
# assignments cannot reach the caller. A `BOUND_ERR` set only inside the helper would still be
# empty at the assert, and `grep -qF -- "…" ""` fails for a missing operand rather than for a
# missing diagnostic: an assert that can never go green, i.e. one that stays red after the fix.
bounded_err_path(){ printf '%s' "$SBX/bounded.err"; }
BOUND_ERR=""
run_bounded(){  # $1 = seconds to wait; $2... = args to the facade -> prints the exit code, or HUNG
  local secs="$1"; shift
  local rcf="$SBX/bounded.rc"
  BOUND_ERR="$(bounded_err_path)"
  rm -f "$rcf" "$rcf.partial"; : > "$BOUND_ERR"
  set -m
  ( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" "$@" >/dev/null 2>"$BOUND_ERR"
    printf '%s' "$?" > "$rcf.partial"; mv -f "$rcf.partial" "$rcf" ) &
  local p=$! i=0
  set +m
  while [ "$i" -lt $(( secs * 10 )) ] && [ ! -f "$rcf" ]; do sleep 0.1; i=$(( i + 1 )); done
  if [ ! -f "$rcf" ]; then
    kill -TERM "-$p" 2>/dev/null || kill -TERM "$p" 2>/dev/null
    wait "$p" 2>/dev/null
    printf 'HUNG'
    return 0
  fi
  wait "$p" 2>/dev/null
  cat "$rcf"
}

make_fixture
BOUND_ERR="$(bounded_err_path)"   # the caller's own copy — see bounded_err_path above
for f in runner agent model effort worktree; do
  rc="$(run_bounded 3 --"$f")"
  # Pinned on the MECHANISM, not merely on "it failed" (LEARNINGS: assert-pins-outcome-not-mechanism):
  # `HUNG` and a non-zero code are different outcomes, and only one of them is this leg's subject.
  assert "0208(c): trailing --$f exits rather than hanging" '[ "$rc" != "HUNG" ]'
  assert "0208(c): trailing --$f exits nonzero" '[ "$rc" != "0" ]'
  assert "0208(c): trailing --$f says it requires a value" \
    'grep -qF -- "--'"$f"' requires a value" "$BOUND_ERR"'
done
rm -rf "$SBX"

# ---- facade: repo-root anchor + adapter handoff -----------------------------------
make_fixture
out="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status --model m1 2>/dev/null )"
argv="$(cat "$LOG")"
assert "facade: handoff reaches codex exec" 'grep -qxF -- "m1" <<<"$argv"'
assert "facade: repo root anchored to the main worktree" 'grep -qxF -- "$SBX" <<<"$argv"'
assert "facade: relays the adapter's stdout" '[ "$out" = "$MSG" ]'
# cwd-independence (ADR-0034): invoke from a subdir; -C must still be the repo root
: > "$LOG"
mkdir -p "$SBX/sub/dir"
( cd "$SBX/sub/dir" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "facade: -C is the main worktree even from a subdir" 'grep -qxF -- "$SBX" <<<"$argv"'
rm -rf "$SBX"

# ---- change 0206: --worktree, the explicit run anchor -------------------------------
# The facade's anchor is an ARGUMENT defaulting to the main worktree. ADR-0034 is unamended:
# nothing resolves the anchor from the caller's CWD, so a RELATIVE --worktree joins to the main
# worktree, not to $PWD. That relative-from-a-subdir assert is the one that distinguishes this
# design from the rejected resolve-the-caller's-CWD option — do not drop it.
make_fixture
# REAL linked worktrees, not `mkdir -p` directories: since 0208 the anchor gate is a MEMBERSHIP
# test, so a bare subdirectory of the main worktree is exactly the value it now rejects. A `mkdir`
# fixture here would make legs (b) and (c) assert that a rejected value is accepted.
git -C "$SBX" worktree add -q -b featslug "$SBX/.worktrees/featslug" >/dev/null 2>&1
mkdir -p "$SBX/sub/dir"
assert "0206: fixture sanity — .worktrees/featslug is a REAL linked worktree" \
  '[ -f "$SBX/.worktrees/featslug/.git" ]'

# (a) flag absent => main worktree (regression fence on today's behavior)
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "0206: no --worktree => anchor is the main worktree" 'grep -qxF -- "$SBX" <<<"$argv"'

# (b) absolute --worktree => that path verbatim
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/.worktrees/featslug" >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "0206: absolute --worktree becomes the anchor" \
  'grep -qxF -- "$SBX/.worktrees/featslug" <<<"$argv"'
assert "0206: absolute --worktree displaces the main worktree" '! grep -qxF -- "$SBX" <<<"$argv"'

# (c) relative --worktree from a FOREIGN cwd inside the repo => joins to the MAIN worktree.
# This is the ADR-0034 discriminator: CWD-resolution would yield $SBX/sub/dir/.worktrees/featslug.
: > "$LOG"
( cd "$SBX/sub/dir" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree ".worktrees/featslug" >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "0206: relative --worktree joins to the main worktree, not the cwd" \
  'grep -qxF -- "$SBX/.worktrees/featslug" <<<"$argv"'
assert "0206: relative --worktree did NOT resolve against the caller cwd" \
  '! grep -qF -- "$SBX/sub/dir/.worktrees" <<<"$argv"'

# (d) build-* agent with no --worktree => loud nonzero naming the flag
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy 2>&1 >/dev/null )"; rc=$?
assert "0206: build-* without --worktree is rejected" '[ "$rc" != "0" ]'
assert "0206: build-* rejection names --worktree" 'grep -qF -- "--worktree" <<<"$err"'

# (e) the gate is SCOPED to build-* — a metadata-scoped agent still needs no flag
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 ); rc=$?
assert "0206: non-build agent without --worktree still succeeds" '[ "$rc" = "0" ]'

# (f) resolved anchor is not a directory => nonzero
printf 'x\n' > "$SBX/notadir"
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/notadir" 2>&1 >/dev/null )"; rc=$?
assert "0206: --worktree pointing at a non-directory is rejected" '[ "$rc" != "0" ]'
assert "0206: non-directory rejection says directory" 'grep -qiF "not a directory" <<<"$err"'

# (g) resolved anchor outside this repo's worktree set => nonzero
OUTSIDE="$(mktemp -d)"; OUTSIDE="$(cd "$OUTSIDE" && pwd -P)"
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$OUTSIDE" 2>&1 >/dev/null )"; rc=$?
assert "0206: --worktree outside the repo worktree set is rejected" '[ "$rc" != "0" ]'
assert "0206: outside-repo rejection says worktree of this repository" \
  'grep -qiF "not a worktree of this repository" <<<"$err"'
rm -rf "$OUTSIDE"

# (h) 0208: an ordinary subdirectory of the main worktree is CONTAINED but not a MEMBER.
# This is the value the pre-0208 gate wrongly admitted: docket_main_worktree("$SBX/sub/dir")
# returns $SBX, so containment passed and a delegated run anchored inside the primary checkout.
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/sub/dir" 2>&1 >/dev/null )"; rc=$?
assert "0208(a): an ordinary subdirectory is rejected" '[ "$rc" != "0" ]'
assert "0208(a): the subdirectory rejection names worktree top-level" \
  'grep -qiF "worktree top-level" <<<"$err"'

# (i) 0208: a real linked worktree is still accepted — the positive half of the membership pair.
# Without it every assert above is satisfied by a gate that rejects everything.
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/.worktrees/featslug" >/dev/null 2>&1 ); rc=$?
assert "0208(a): a real linked worktree is accepted" '[ "$rc" = "0" ]'
assert "0208(a): and the accepted worktree is the anchor handed to the adapter" \
  'grep -qxF -- "$SBX/.worktrees/featslug" "$LOG"'

# (j) 0208: a SYMLINKED alias of a real member is accepted — the pwd -P normalization leg.
# On macOS /tmp is a symlink to /private/tmp, so an un-normalized exact-line match would reject
# valid worktrees the old containment check accepted. This leg reproduces that shape locally.
ln -s "$SBX/.worktrees/featslug" "$SBX/featlink"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX/featlink" >/dev/null 2>&1 ); rc=$?
assert "0208(a): a symlinked alias of a real worktree is accepted" '[ "$rc" = "0" ]'
rm -rf "$SBX"

# ---- 0208: same-repo is the FIRST `worktree` line, never an anywhere-in-list match ----
# `git worktree list` keeps the record of a worktree whose directory was deleted without a prune, so
# a FOREIGN repo's list can carry a `worktree <this repo's root>` line for a path that is no longer
# its worktree. An anywhere-in-list match would read that stale record as proof of same-repo and
# then hand the delegated run — a child harness that may execute under an auto-approve permission
# grant — a tree docket does not own, regressing the very guarantee gate 3 exists to provide.
# This is the mutation the gate's OWN comment names, so it is fixtured rather than reasoned about:
# the block builds the shape that produces such a record — FGN once had this repo's PATH as one of
# its worktrees, that directory was deleted unpruned, and the path was then re-created as a repo of
# its own. It carries its own two repos rather than reusing make_fixture, because the stale record
# must be written BEFORE the anchor repo exists at that path.
STALE="$(mktemp -d "${TMPDIR:-/tmp}/docket-stale-wt.XXXXXX")"; STALE="$(cd "$STALE" && pwd -P)"
HERE="$STALE/here"; FGN="$STALE/foreign"
git init -q "$FGN"; git -C "$FGN" config user.email t@t.test; git -C "$FGN" config user.name Test
( cd "$FGN" && git commit --allow-empty -qm init )
git -C "$FGN" worktree add -q -b gone "$HERE" >/dev/null 2>&1
rm -rf "$HERE"
git init -q "$HERE"; git -C "$HERE" config user.email t@t.test; git -C "$HERE" config user.name Test
( cd "$HERE" && git commit --allow-empty -qm init )
# Fixture sanity, so a future git that prunes this record on read fails the leg LOUDLY rather than
# leaving the rejection asserts green for the ordinary foreign-repo reason.
stale_list="$(git -C "$FGN" worktree list --porcelain)"
assert "0208(a): fixture sanity — the foreign repo's list carries a STALE record for this repo's root" \
  'grep -qxF -- "worktree $HERE" <<<"$stale_list"'
mkdir -p "$HERE/runners"
cat > "$HERE/runners/ad.sh" <<AD
#!/usr/bin/env bash
printf '%s\n' "\${DOCKET_REPO_ROOT:-}" > "$STALE/anchor.log"
AD
chmod +x "$HERE/runners/ad.sh"
: > "$STALE/anchor.log"
err="$( cd "$HERE" && RUNNERS_DIR="$HERE/runners" bash "$FACADE" --runner ad --agent status \
    --worktree "$FGN" 2>&1 >/dev/null )"; rc=$?
assert "0208(a): a foreign repo holding a stale record for this repo's root is still rejected" \
  '[ "$rc" != "0" ]'
assert "0208(a): and that refusal is the worktree-of-this-repository gate" \
  'grep -qiF "not a worktree of this repository" <<<"$err"'
assert "0208(a): the foreign tree never reached the adapter" '[ ! -s "$STALE/anchor.log" ]'
rm -rf "$STALE"

# ---- 0208 leg (b): the --worktree requirement keys on DECLARED scope ------------------
# The pre-0208 gate matched `build-*` only, leaving `rebase-resolver`, `integration-repair` and the
# three `review-*` rungs — two of which COMMIT — able to anchor silently in the main tree on the
# integration branch. The facade reads `worktree-scope:` from the agent source, so this section
# drives REAL agent names against the REAL agents/ directory; a fabricated name would test the
# tolerant fallback instead of the gate.
make_fixture
git -C "$SBX" worktree add -q -b featslug "$SBX/.worktrees/featslug" >/dev/null 2>&1
WT="$SBX/.worktrees/featslug"
assert "0208(b): fixture sanity — the scope fixture is a REAL linked worktree" '[ -f "$WT/.git" ]'
# Fixture sanity on the DECLARATION side too: every assert below reads the real agents/ tree through
# the facade's DOCKET_AGENTS_SRC default, so a source that lost its `worktree-scope:` line would turn the
# refusal legs green-for-the-wrong-reason (no declared scope => the tolerant metadata fallback).
assert "0208(b): fixture sanity — review-lean really declares feature scope" \
  'grep -qx "worktree-scope: feature" "$ROOT/agents/docket-review-lean.md"'
assert "0208(b): fixture sanity — status really declares metadata scope" \
  'grep -qx "worktree-scope: metadata" "$ROOT/agents/docket-status.md"'

# Pinned on the MECHANISM, not merely on "it failed" and not on a bare `--worktree` mention: with
# no --worktree the anchor defaults to the main worktree, so gate 3b's own main-tree refusal fires
# on exactly these agents too, and ITS diagnostic contains the literal `--worktree` as well. A
# rejection assert keyed on either of those stays green with gate 1 reverted to `build-*` — measured,
# not reasoned about. The exit-code leg is kept as the floor (both gates gone => rc 0), but the
# diagnostic clause is what separates gate 1 from the gate that shadows it.
for a in rebase-resolver review-lean integration-repair; do
  err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent "$a" 2>&1 >/dev/null )"; rc=$?
  assert "0208(b): feature-scoped $a without --worktree is rejected" '[ "$rc" != "0" ]'
  assert "0208(b): the $a rejection is gate 1's, naming the declared scope" \
    'grep -qF -- "--worktree is required for feature-scoped agents" <<<"$err" &&
     grep -qF -- "worktree-scope: feature" <<<"$err"'
done

for a in status adr; do
  ( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent "$a" >/dev/null 2>&1 ); rc=$?
  assert "0208(b): metadata-scoped $a without --worktree still succeeds" '[ "$rc" = "0" ]'
done

# A feature-scoped agent WITH a worktree reaches the adapter — the non-vacuity floor for the
# refusals above, which are otherwise satisfied by a gate that rejects every dispatch.
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent review-lean \
    --worktree "$WT" >/dev/null 2>&1 ); rc=$?
assert "0208(b): feature-scoped review-lean WITH --worktree succeeds" '[ "$rc" = "0" ]'
assert "0208(b): and its anchor is the feature worktree" 'grep -qxF -- "$WT" "$LOG"'

# The MAIN-TREE rejection: membership alone still admits the repo root, and the repo root is the
# one value the whole gate exists to reject — a feature-scoped worker anchored in the primary
# checkout on the integration branch.
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent review-lean \
    --worktree "$SBX" 2>&1 >/dev/null )"; rc=$?
assert "0208(a): a feature-scoped agent anchored at the main worktree is rejected" '[ "$rc" != "0" ]'
assert "0208(a): the main-tree rejection names the integration branch hazard" \
  'grep -qiF "integration branch" <<<"$err"'

# ...and it is SCOPED: a metadata-scoped agent may legitimately anchor at the main worktree, which
# is the default anchor for every one of them. Without this leg the rejection could be widened to
# every agent and nothing would redden.
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --worktree "$SBX" >/dev/null 2>&1 ); rc=$?
assert "0208(a): a metadata-scoped agent at the main worktree is still accepted" '[ "$rc" = "0" ]'

# The tolerant fallback: an agent with no source file keeps the ADAPTER's more specific
# unknown-agent diagnostic instead of dying at the facade's scope probe. Generation is the loud
# seam for absence; the facade must not shadow the better message.
# EMITTER-PINNED, not exit-code-pinned: the dispatch fails either way (the adapter refuses an
# unknown agent), so `rc != 0` would stay green with the tolerance removed. What is asserted is
# WHICH refusal it takes — the adapter's, naming the missing source, and not the facade's.
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent no-such-agent 2>&1 >/dev/null )"
assert "0208(b): an agent with no source file keeps the ADAPTER's unknown-agent diagnostic" \
  'grep -qF "no built-in agent source for" <<<"$err" && ! grep -qF "runner-dispatch:" <<<"$err"'
# An OFF-SHAPE name is held to the same tolerance for the same reason ($AGENT becomes a path
# component, so the probe skips it rather than dying): the refusal must still be the adapter's.
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent "../evil" 2>&1 >/dev/null )"
assert "0208(b): an off-shape agent name is not rejected by the scope probe either" \
  '! grep -qF "runner-dispatch:" <<<"$err"'

# ---- 0208(d): the sources DIRECTORY is the probe's precondition, and it is LOUD -------
# The per-file tolerance asserted just above is deliberate and it does NOT extend to the directory
# holding those files. A missing or misdirected sources directory resolves EVERY agent to metadata
# scope in one stroke, which disarms both delegation gates together — `--worktree` stops being
# required and the main-tree rejection stops firing — so a feature-scoped worker would anchor in
# the primary checkout on the integration branch with nothing said. That is a gate green because it
# never ran, which is the shape AGENTS.md's guard rule forbids.
#
# Every leg carries a MECHANISM assert beside its exit code: `$LOG` stays EMPTY, i.e. the adapter
# was never reached. Without it a leg would stay green on a facade that ADMITTED the dispatch and
# merely failed further downstream for an unrelated reason — which is precisely the silent
# admission being tested against.
#
# Two bogus shapes, not one: the absent directory, and a directory that EXISTS but holds no
# `docket-*.md` sources. The second is the leg a bare `[ -d ]` precondition would wave through
# while every scope read inside it still came back empty — misdirection, not absence.
mkdir -p "$SBX/empty-agents-dir"
printf 'not an agent source\n' > "$SBX/empty-agents-dir/README.md"
for bogus_label in absent misdirected; do
  case "$bogus_label" in
    absent)      BOGUS="$SBX/no-such-agents-dir" ;;
    misdirected) BOGUS="$SBX/empty-agents-dir" ;;
  esac
  : > "$LOG"
  err="$( cd "$SBX" && PATH="$BIN:$PATH" DOCKET_AGENTS_SRC="$BOGUS" \
      bash "$FACADE" --runner codex --agent review-lean 2>&1 >/dev/null )"; rc=$?
  assert "0208(d): sources directory ($bogus_label) refuses a --worktree-less feature-scoped dispatch" \
    '[ "$rc" != "0" ]'
  assert "0208(d): the refusal ($bogus_label) names the sources seam and the scope it could not read" \
    'grep -qF -- "DOCKET_AGENTS_SRC=$BOGUS" <<<"$err" && grep -qF -- "worktree-scope" <<<"$err"'
  assert "0208(d): and the dispatch ($bogus_label) never reached the adapter" '[ ! -s "$LOG" ]'
done

# The same disarm reached through the OTHER gate: with the sources unreadable, gate 3b's main-tree
# rejection is equally unarmed, so a feature-scoped agent handed the primary checkout would be
# admitted. Asserted separately because it is a distinct gate, and passing `--worktree` satisfies
# gate 1 outright — this leg is green-for-the-wrong-reason if only gate 1 is considered.
: > "$LOG"
err="$( cd "$SBX" && PATH="$BIN:$PATH" DOCKET_AGENTS_SRC="$SBX/no-such-agents-dir" \
    bash "$FACADE" --runner codex --agent review-lean --worktree "$SBX" 2>&1 >/dev/null )"; rc=$?
assert "0208(d): a bogus sources directory also refuses a feature-scoped dispatch AT THE MAIN TREE" \
  '[ "$rc" != "0" ]'
assert "0208(d): the main-tree leg never reached the adapter either" '[ ! -s "$LOG" ]'

# THE POSTURE, stated as an assert: the refusal is unconditional, not scoped to agents that happen
# to be feature-scoped. With no sources the facade cannot tell the two apart, so a scoped refusal
# would be exactly the silent admission above. A metadata dispatch pays a loud, one-line-diagnosable
# install failure for that; the alternative is a silent one.
: > "$LOG"
err="$( cd "$SBX" && PATH="$BIN:$PATH" DOCKET_AGENTS_SRC="$SBX/no-such-agents-dir" \
    bash "$FACADE" --runner codex --agent status 2>&1 >/dev/null )"; rc=$?
assert "0208(d): the refusal is unconditional — a metadata-scoped dispatch is refused too" \
  '[ "$rc" != "0" ] && [ ! -s "$LOG" ]'

# NON-VACUITY FLOOR. Everything above would stay green on a facade that refused any dispatch
# carrying the variable at all. Pointed at the REAL sources the seam is honored and behavior is
# byte-identical to the default: gate 1 fires with ITS own diagnostic, and a properly anchored
# feature-scoped dispatch still reaches the adapter.
err="$( cd "$SBX" && PATH="$BIN:$PATH" DOCKET_AGENTS_SRC="$ROOT/agents" \
    bash "$FACADE" --runner codex --agent review-lean 2>&1 >/dev/null )"
assert "0208(d): the seam pointed at the real sources still refuses via GATE 1, not the precondition" \
  'grep -qF -- "--worktree is required for feature-scoped agents" <<<"$err" &&
   ! grep -qF -- "DOCKET_AGENTS_SRC=" <<<"$err"'
: > "$LOG"
( cd "$SBX" && PATH="$BIN:$PATH" DOCKET_AGENTS_SRC="$ROOT/agents" \
    bash "$FACADE" --runner codex --agent review-lean --worktree "$WT" >/dev/null 2>&1 ); rc=$?
assert "0208(d): and a properly anchored feature-scoped dispatch still reaches the adapter" \
  '[ "$rc" = "0" ] && [ -s "$LOG" ]'

# THE NAMESPACE IS LIVE (ADR-0014). The seam is read under its `DOCKET_`-prefixed name only, so an
# un-namespaced `AGENTS_SRC` sitting in the caller's shell — a plausible name for an unrelated tool,
# and the spelling docket's own adapters and sync-agents.sh use for their internal variable — cannot
# reach the input the delegation gates key on. Reverting the rename turns both asserts red: the
# facade would read the bogus path and refuse at the precondition instead.
: > "$LOG"
err="$( cd "$SBX" && PATH="$BIN:$PATH" AGENTS_SRC="$SBX/no-such-agents-dir" \
    bash "$FACADE" --runner codex --agent review-lean 2>&1 >/dev/null )"
assert "0208(d): an un-namespaced AGENTS_SRC in the environment is not read as the seam" \
  'grep -qF -- "--worktree is required for feature-scoped agents" <<<"$err"'
( cd "$SBX" && PATH="$BIN:$PATH" AGENTS_SRC="$SBX/no-such-agents-dir" \
    bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 ); rc=$?
assert "0208(d): and a metadata dispatch is unaffected by it" '[ "$rc" = "0" ]'
rm -rf "$SBX"

# ---- 0270: config locality — a MAIN-worktree grant survives a --worktree dispatch ----
# Provenance (opencode): filed as "a machine-local `runners.opencode.permissions: auto-approve`
# grant is invisible to a build-* delegation". It never was. The facade resolves runners.<name>.*
# at docket_main_worktree() and anchors the RUN at --worktree; the two trees are deliberately
# DECOUPLED, and the decoupling is load-bearing because .docket.local.yml is gitignored — a
# feature worktree carries no copy of it, so an anchor-relative read would drop every
# machine-local grant on exactly the build-* dispatches that require --worktree.
# Tested here, at the FACADE, because the config loop is runner-agnostic (it knows no runner's key
# names); tests/test_runner_opencode.sh drives the adapter in isolation and never runs the facade.
#
# The fixture MUST be a real linked worktree. With a bare `mkdir -p` subdirectory,
# docket_main_worktree "$ANCHOR" trivially returns $SBX because the subdirectory IS part of the
# main worktree, and the resolution under test never happens — every assert below goes vacuous.
make_fixture
git -C "$SBX" worktree add -q -b featslug "$SBX/.worktrees/featslug" >/dev/null 2>&1
WT="$SBX/.worktrees/featslug"
assert "0270: fixture is a REAL linked worktree, not a subdirectory" '[ -f "$WT/.git" ]'

# Mirror production: the machine-local layer is gitignored, and the grant is written to the main
# worktree ONLY — after the worktree exists, so it can never be copied into it. The .gitignore
# line is documentation inside the fixture: it shows WHY a feature worktree lacks the file.
printf '.docket.local.yml\n' > "$SBX/.gitignore"
printf 'runners:\n  codex:\n    sandbox: danger-full-access\n' > "$SBX/.docket.local.yml"
assert "0270: the grant exists ONLY in the main worktree" '[ ! -e "$WT/.docket.local.yml" ]'

# cwd INSIDE the linked worktree is the production shape (a build worker dispatches from its own
# tree) and is the condition under which a cwd-derived config root would read the wrong tree.
# The agent is a REAL build-* one (agents/docket-build-economy.md), not `status`: the prose above
# names the build-* dispatches as the case that matters, so the dispatch fenced here must be one of
# them — otherwise "special-case the config read for build-*" (an ordinary-looking refactor in a
# facade that already branches on `case "$AGENT" in build-*)` three times) would leave this section
# green with the invariant gone. It is otherwise the same code path: the `build-*` requires-
# --worktree gate is satisfied by the --worktree already passed, and the run gate below the handoff
# is scoped to implement-next, so build-economy reaches the adapter exactly as `status` did.
# DOCKET_HARNESS_ROOT is pinned into the sandbox so the GLOBAL layer cannot satisfy the grant
# assert. This file unsets XDG_CONFIG_HOME, so an unpinned run resolves GLOBAL_CFG at the
# developer's real ~/.config/docket/config.yml — and on a machine that sets runners.codex.sandbox
# there (the documented knob, spelled exactly as this fixture spells it) the grant assert would
# pass with no main-worktree read at all, the mutation probe would stay green, and the fence would
# be decoration.
: > "$LOG"
# The trailing payload satisfies change 0277's build-* empty-payload gate, which refuses a
# task-less build-* dispatch before the adapter is ever reached. It is incidental to what this
# section fences (the config-locality read) — but without it this whole block would abort at that
# gate and its asserts would read an empty argv log.
( cd "$WT" && PATH="$BIN:$PATH" DOCKET_HARNESS_ROOT="$SBX" \
    bash "$FACADE" --runner codex --agent build-economy --worktree "$WT" -- "0270 fixture task" >/dev/null 2>&1 ); rc=$?
argv="$(cat "$LOG")"
# 0208: the SUCCESS-path conjunct the 0206 review asked for — a feature-scoped agent WITH a real
# --worktree exits 0. The argv asserts below are satisfied by any run that reached the adapter, so
# they do not by themselves distinguish "succeeded" from "succeeded then failed afterwards"; and
# without an exit-code leg the mutation where a feature-scoped dispatch aborts unconditionally
# would leave this block red only by accident. It rides HERE rather than in a second fixture
# because this block already builds the exact shape the leg needs.
assert "0208(a): a feature-scoped agent WITH a real --worktree exits 0" '[ "$rc" = "0" ]'
assert "0270: main-worktree grant reaches the child across a --worktree dispatch" \
  'grep -qxF -- "danger-full-access" <<<"$argv"'
# Anchor pair. The positive leg pins the anchor handed to the adapter to the linked worktree, so
# the grant assert above cannot be satisfied by a run that quietly anchored somewhere else. The
# negative leg fences a different case: the main worktree must not leak into the child's argv
# *alongside* the anchor. It is the weaker of the two — it is independently satisfied whenever
# "$SBX" never appears at all, including if the adapter never ran — so it stands beside the
# positive assert, never in place of it.
assert "0270: the anchor handed to the adapter IS the linked worktree" \
  'grep -qxF -- "$WT" <<<"$argv"'
assert "0270: the anchor is NOT the main worktree" '! grep -qxF -- "$SBX" <<<"$argv"'
rm -rf "$SBX"

# ---- facade: runners.<name> config resolution across layers ------------------------
make_fixture
printf 'runners:\n  codex:\n    sandbox: danger-full-access\n    network: false\n' > "$SBX/.docket.yml"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "facade: committed runners.codex.sandbox honored" 'grep -qxF -- "danger-full-access" <<<"$argv"'
assert "facade: committed runners.codex.network=false honored" '! grep -qF "network_access" <<<"$argv"'
: > "$LOG"
printf 'runners:\n  codex:\n    sandbox: workspace-write\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 )
argv="$(cat "$LOG")"
assert "facade: local layer beats committed per key" 'grep -qxF -- "workspace-write" <<<"$argv"'
assert "facade: unset-in-local key falls to committed (network still false)" '! grep -qF "network_access" <<<"$argv"'
rm -rf "$SBX"

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
# BYTES ≠ CONTENT: the facade's `-s` check counts bytes, the adapter's payload is `$(cat …)` with
# its trailing newlines stripped. A newline-only brief satisfied both `-s` checks and reached the
# child as no task at all. The facade must refuse it with the same content predicate.
printf '\n\n' > "$SBX/blank-brief"
# The happy-path dispatch above wrote the mock's log; truncate so "never reached the child" is
# evidence about THIS run rather than a stale hit.
: > "$LOG"
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status \
    --brief-file "$SBX/blank-brief" 2>&1 >/dev/null )"; rc=$?
assert "0277 facade: a whitespace-only brief file is refused" '[ "$rc" != "0" ]'
assert "0277 facade: the whitespace refusal names the path" 'grep -qF -- "blank-brief" <<<"$err"'
# EMITTER-PINNED, or the assert would stay green on the adapter's own defensive refusal and the
# facade's guard would be decoration: the facade must refuse before it ever builds the invocation.
assert "0277 facade: the whitespace refusal comes from the FACADE, not the adapter" \
  'grep -qF -- "runner-dispatch:" <<<"$err" && ! grep -qF -- "runners/codex:" <<<"$err"'
assert "0277 facade: the whitespace refusal never reached the child" '[ ! -s "$LOG" ]'
rm -rf "$SBX"

# The build-* empty-payload gate, at the SAME pre-verb point as the --worktree gate, so it holds
# for the legacy foreground verb (the hand-invocation path, the one most likely to be typed
# task-less) exactly as it does for --launch. A build worker with no task is always the
# silent-improvise defect; a loud abort is strictly better than a successful-looking task-less run.
make_fixture
# A REAL linked worktree, not `mkdir -p`: since 0208 the anchor gate is a MEMBERSHIP test, so a
# bare subdirectory is refused at gate 3 — which would abort the two "WITH a payload runs" legs
# below AND satisfy the refusal legs above for the wrong reason.
git -C "$SBX" worktree add -q -b payloadslug "$SBX/.worktrees/w" >/dev/null 2>&1
assert "0277 gate: fixture sanity — the payload fixture is a REAL linked worktree" \
  '[ -f "$SBX/.worktrees/w/.git" ]'
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy \
    --worktree "$SBX/.worktrees/w" 2>&1 >/dev/null )"; rc=$?
assert "0277 gate: build-* with NO payload is refused" '[ "$rc" != "0" ]'
assert "0277 gate: the refusal names the improvise failure mode" 'grep -qiE "improvis|no task" <<<"$err"'
assert "0277 gate: the refusal never reached the child" '[ ! -s "$LOG" ]'
# ARITY IS NOT CONTENT: `[ $# -gt 0 ]` was satisfied by `-- ""`, which then produced an empty
# payload in the adapter — a build worker dispatched with no task at all while the dispatch looked
# successful. The gate measures the argv the same way it measures a brief file: by its content.
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy \
    --worktree "$SBX/.worktrees/w" -- "" 2>&1 >/dev/null )"; rc=$?
assert "0277 gate: build-* with an EMPTY argv payload is refused" '[ "$rc" != "0" ]'
assert "0277 gate: the empty-argv refusal names the improvise failure mode" 'grep -qiE "improvis|no task" <<<"$err"'
# Emitter-pinned for the same reason as the whitespace-brief case above.
assert "0277 gate: the empty-argv refusal comes from the FACADE, not the adapter" \
  'grep -qF -- "runner-dispatch:" <<<"$err" && ! grep -qF -- "runners/codex:" <<<"$err"'
assert "0277 gate: the empty-argv refusal never reached the child" '[ ! -s "$LOG" ]'
# ... and it is satisfied by EITHER channel.
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy \
    --worktree "$SBX/.worktrees/w" -- "do the task" >/dev/null 2>&1 ); rc=$?
assert "0277 gate: build-* WITH argv payload runs" '[ "$rc" = "0" ]'
: > "$LOG"
printf 'do the task\n' > "$SBX/brief.txt"
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent build-economy \
    --worktree "$SBX/.worktrees/w" --brief-file "$SBX/brief.txt" >/dev/null 2>&1 ); rc=$?
assert "0277 gate: build-* WITH a brief file runs" '[ "$rc" = "0" ]'
# SCOPED to the verbs that START a child: `--observe` reads a result the matching `--launch`
# already recorded, so it has no payload and the generated shim gives its observe line no brief
# slot. The gate must not refuse it. This dispatch still fails (there is no such dispatch key) —
# what is pinned is WHICH refusal it takes, so a gate that swallowed observe would redden here.
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --observe no-such-key --runner codex \
    --agent build-economy --worktree "$SBX/.worktrees/w" 2>&1 >/dev/null )"
assert "0277 gate: --observe is exempt from the build-* payload gate" \
  '! grep -qiE "improvis|carries no task" <<<"$err"'
# SCOPED to build-*: a metadata-scoped agent legitimately dispatches payload-free.
( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex --agent status >/dev/null 2>&1 ); rc=$?
assert "0277 gate: a non-build agent with no payload still runs" '[ "$rc" = "0" ]'
rm -rf "$SBX"

# ---- 0173: runners.<name> value class — block mapping, tolerant posture -------------
# The facade exports runners.<name>.* as DOCKET_RUNNER_CFG_*. These values are free-form and more
# likely to be paths or URLs than model IDs, so the class is "rest of line", not the flow-map class
# sync-agents.sh uses. Asserts read the exported value directly (via a throwaway adapter dropped
# into a RUNNERS_DIR of our own) rather than through the codex adapter, so they pin the READER and
# not the adapter's flag mapping. DOCKET_HARNESS_ROOT is pinned into the sandbox so the global
# layer cannot reach the developer's real ~/.config/docket/config.yml.
make_fixture
mkdir -p "$SBX/runners"
cat > "$SBX/runners/probe.sh" <<'PROBE'
#!/usr/bin/env bash
printf '%s' "${DOCKET_RUNNER_CFG_PROBEKEY-<unset>}"
PROBE
chmod +x "$SBX/runners/probe.sh"
probe(){  # $1 = yaml value text -> prints the resulting DOCKET_RUNNER_CFG_PROBEKEY
  printf 'runners:\n  probe:\n    probekey: %s\n' "$1" > "$SBX/.docket.yml"
  ( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" \
      bash "$FACADE" --runner probe --agent status 2>/dev/null )
}

assert "0173 rd: slash-bearing value arrives intact" \
  '[ "$(probe "/Users/x/some/path")" = "/Users/x/some/path" ]'
assert "0173 rd: colon-bearing URL value arrives intact" \
  '[ "$(probe "https://example.test/v1")" = "https://example.test/v1" ]'
assert "0173 rd: trailing comment stripped, whitespace trimmed" \
  '[ "$(probe "workspace-write   # why we chose it")" = "workspace-write" ]'
# Comment detection requires WHITESPACE before the `#`, per YAML — so a `#` inside the value
# (a URL fragment) is part of the value, not the start of a comment.
assert "0173 rd: a non-whitespace-preceded # stays in the value" \
  '[ "$(probe "https://example.test/v1#frag")" = "https://example.test/v1#frag" ]'
assert "0173 rd: a plain value is unchanged (non-regression)" \
  '[ "$(probe "danger-full-access")" = "danger-full-access" ]'
# A COMMENT-ONLY value must export NOTHING. The capture's `[[:space:]]*` is greedy and eats the
# space before the `#`, so the whitespace-preceded strip cannot fire here — without its own leg the
# comment TEXT becomes the value, and codex.sh would then run `--sandbox '# TODO decide later'`
# (or `die` on a commented-out `network:`), turning a cosmetic comment into a failed dispatch —
# exactly what the tolerant posture exists to prevent (change 0173 review).
assert "0173 rd: a comment-only value exports nothing" \
  '[ "$(probe "  # TODO decide later")" = "<unset>" ]'
rm -rf "$SBX"

# -- tolerant posture: an unparseable value skips WITHOUT dying, and still masks lower layers --
make_fixture
mkdir -p "$SBX/runners"
cat > "$SBX/runners/probe.sh" <<'PROBE'
#!/usr/bin/env bash
printf '%s' "${DOCKET_RUNNER_CFG_SANDBOX-<unset>}"
PROBE
chmod +x "$SBX/runners/probe.sh"
# High-precedence layer claims `sandbox` with an EMPTY value; the committed layer sets a real one.
printf 'runners:\n  probe:\n    sandbox:\n' > "$SBX/.docket.local.yml"
printf 'runners:\n  probe:\n    sandbox: danger-full-access\n' > "$SBX/.docket.yml"
tol_out="$( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" \
    bash "$FACADE" --runner probe --agent status 2>/dev/null )"; tol_rc=$?
assert "0173 rd: malformed high-precedence value does not kill the dispatch" '[ "$tol_rc" = "0" ]'
assert "0173 rd: and it still MASKS the lower layer (per-key precedence preserved)" \
  '[ "$tol_out" = "<unset>" ]'
rm -rf "$SBX"

# ---- 0140: `inherit` is DOCKET'S OWN no-pin sentinel, owned by the FACADE ------------
# The facade normalizes `inherit` -> empty right after argument parsing, so no adapter re-decides
# it. These asserts route through a THROWAWAY PROBE ADAPTER (the same RUNNERS_DIR seam the 0173
# value-class asserts use), never through codex.sh: codex.sh carries its own defensive twin, so an
# assert dispatched through it would pass on the strength of either layer and would stay green with
# the facade's line deleted — an outcome assert, not a mechanism one. The probe records the argv the
# FACADE handed the adapter, which is the only place the facade's own decision is observable.
# DOCKET_HARNESS_ROOT is pinned into the sandbox so the global config layer cannot reach the
# developer's real ~/.config/docket/config.yml.
make_fixture
mkdir -p "$SBX/runners"
cat > "$SBX/runners/probe.sh" <<'PROBE'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$MOCK_ARGV"
PROBE
chmod +x "$SBX/runners/probe.sh"
PARGV="$SBX/probe-argv.txt"
dispatch_probe(){  # $@ = extra facade args -> fills PARGV with the adapter's argv, one entry per line
  : > "$PARGV"
  ( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" MOCK_ARGV="$PARGV" \
      bash "$FACADE" --runner probe --agent status "$@" >/dev/null 2>&1 )
}

dispatch_probe --model inherit --effort high
pargv="$(cat "$PARGV")"
assert "0140 rd: inherit sentinel => no --model flag reaches the adapter" \
  '! grep -qxF -- "--model" <<<"$pargv"'
assert "0140 rd: the literal sentinel never reaches the adapter" \
  '! grep -qxF -- "inherit" <<<"$pargv"'
# Effort is a SEPARATE knob: normalizing the model must not disturb it. Without this, dropping the
# whole flag pair would satisfy both asserts above.
assert "0140 rd: --effort survives model normalization" \
  'grep -qxF -- "--effort" <<<"$pargv" && grep -qxF -- "high" <<<"$pargv"'
assert "0140 rd: the adapter still ran (the negated asserts are not vacuous)" \
  'grep -qxF -- "--agent" <<<"$pargv"'

# Non-regression control (ADR-0015): a REAL model ID is not a sentinel and still passes verbatim.
# Without this leg, deleting the `[ -n "$MODEL" ]` guard outright — i.e. never forwarding a model at
# all — would keep every assert above green.
dispatch_probe --model gpt-5.1-codex
pargv="$(cat "$PARGV")"
assert "0140 rd: a real model ID still passes verbatim (ADR-0015)" \
  'grep -qxF -- "gpt-5.1-codex" <<<"$pargv"'
assert "0140 rd: ... carried by its own --model flag" \
  'grep -qxF -- "--model" <<<"$pargv"'
rm -rf "$SBX"

# ---- 0140: codex adapter DEFENSIVE TWIN ---------------------------------------------
# runner-dispatch.sh owns the sentinel (asserted above, through the probe seam). The adapter keeps
# its own one-line normalization because codex.md documents direct hand invocation that bypasses
# the facade, and this file exercises the adapter directly — exactly that path. Mirrors the
# existing inherit groups in tests/test_runner_cursor.sh and tests/test_runner_opencode.sh, with
# one deliberate difference: on codex, effort SURVIVES the model-less case, because codex carries
# reasoning effort on a separate `-c model_reasoning_effort=` flag rather than encoding it inside
# the model value. That asymmetry is correct, not a bug, and pinning it is what stops a later
# "make the adapters consistent" edit from silently deleting a working effort pin.
make_fixture
run_adapter --agent status --model inherit --effort high >/dev/null 2>&1
argv="$(cat "$LOG")"
assert "0140 codex: inherit sentinel => no -m flag" '! grep -qxF -- "-m" <<<"$argv"'
assert "0140 codex: the literal sentinel never reaches codex exec" \
  '! grep -qxF -- "inherit" <<<"$argv"'
assert "0140 codex: effort SURVIVES the model-less case (separate -c flag)" \
  'grep -qxF -- "model_reasoning_effort=high" <<<"$argv"'
assert "0140 codex: child still ran (normalization is not an abort)" \
  'grep -qxF -- "--output-last-message" <<<"$argv"'
# Non-regression control (ADR-0015): a real model ID still reaches codex exec verbatim.
: > "$LOG"
run_adapter --agent status --model gpt-5.1-codex >/dev/null 2>&1
argv="$(cat "$LOG")"
assert "0140 codex: a real model ID still passes verbatim (ADR-0015)" \
  'grep -qxF -- "gpt-5.1-codex" <<<"$argv"'
assert "0140 codex: ... carried by its own -m flag" 'grep -qxF -- "-m" <<<"$argv"'
rm -rf "$SBX"

# ---- 0237: exec -> call-and-return, exit code preserved verbatim -------------------
# The facade must regain control after the adapter (that is the whole seam the run gate hangs on),
# and every path where the gate takes no action must be byte-identical to the pre-0237 exec.
make_fixture
mkdir -p "$SBX/runners"
cat > "$SBX/runners/rc.sh" <<'RCA'
#!/usr/bin/env bash
# echoes a marker, then exits with the code named in $RC_WANTED
printf 'adapter-ran\n'
printf 'adapter-stderr\n' >&2
exit "$(cat "${RC_WANTED:?}")"
RCA
chmod +x "$SBX/runners/rc.sh"
export RC_WANTED="$SBX/rc-wanted"

for want in 0 3 7; do
  printf '%s\n' "$want" > "$RC_WANTED"
  out="$( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" \
      bash "$FACADE" --runner rc --agent status 2>"$SBX/e.log" )"; rc=$?
  assert "0237: adapter exit code $want is preserved verbatim" '[ "$rc" = "$want" ]'
  assert "0237: adapter stdout still relayed (rc=$want)" '[ "$out" = "adapter-ran" ]'
  assert "0237: adapter stderr still relayed (rc=$want)" 'grep -qF "adapter-stderr" "$SBX/e.log"'
done

# The facade must no longer exec its adapter. Two independent guards, because neither alone is
# sufficient:
#
# (1) A STATIC guard keyed on syntactic SHAPE — an `exec` in command position — not on the one
#     spelling of the handoff line. The obvious spelling-anchored form
#       ! grep -qE "…exec…\"\$DOCKET_BASH_PATH\"…" "$FACADE"
#     is a trap: `assert` evals its expression, so bash's double-quote processing collapses `\$`
#     to a bare `$`, the ERE engine reads that as an end-anchor, the pattern can never match, and
#     the negated assert is permanently green — a vacuous guard (AGENTS.md: a bare leading `--`
#     and a mis-escaped metacharacter both invert a negated assert into decoration). The pattern
#     below carries no `$` at all.
assert "0237: the facade no longer execs in command position" \
  '! grep -qE "^[[:space:]]*exec[[:space:]]" "$FACADE"'

# (2) A RUNTIME guard, which is the fact that actually matters: the facade must still be alive to
#     run the gate after the adapter returns. Under `exec` the adapter REPLACES the facade's
#     process image, so the adapter's parent is whatever launched the facade; under
#     call-and-return the adapter's parent is the facade process itself. Comparing the two pids
#     discriminates by execution, so a future rewrite that reintroduces `exec` under any spelling
#     still reddens here.
cat > "$SBX/runners/ppid.sh" <<'PPA'
#!/usr/bin/env bash
printf '%s\n' "$PPID" > "${PPID_OUT:?}"
PPA
chmod +x "$SBX/runners/ppid.sh"
cat > "$SBX/wrap.sh" <<'WRAP'
#!/usr/bin/env bash
printf '%s\n' "$$" > "${WRAP_OUT:?}"
bash "${FACADE_PATH:?}" --runner ppid --agent status
st=$?        # keeps the facade off the last-command slot so bash cannot turn this
exit "$st"   # call into an implicit exec of its own and forge the result
WRAP
chmod +x "$SBX/wrap.sh"
( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" \
    PPID_OUT="$SBX/adapter.ppid" WRAP_OUT="$SBX/wrap.pid" FACADE_PATH="$FACADE" \
    bash "$SBX/wrap.sh" >/dev/null 2>&1 )
assert "0237: the adapter runs as a CHILD of the facade, not as its replacement image" \
  '[ -s "$SBX/adapter.ppid" ] && [ -s "$SBX/wrap.pid" ] &&
   [ "$(cat "$SBX/adapter.ppid")" != "$(cat "$SBX/wrap.pid")" ]'
unset RC_WANTED
rm -rf "$SBX"

# ---- 0237: the run gate — snapshot diff + agent gating ----------------------------
# A stub verify-run records its argv and replies from files the fixture controls, so these asserts
# pin the FACADE's diffing and gating, not verify-run's verdict logic (Task 1 owns that).
#
# Snapshot fixture files are `<id> <claimed_at-epoch>` lines, the shape verify-run's
# `--in-progress-ids --with-claimed-at` emits. The stub serves the bare `--in-progress-ids` form by
# projecting field 1, so a fixture describes ONE world and both reads agree about it. NOW/OLD/FUT
# are relative to the run, because the facade compares against the clock it reads at dispatch time:
# FUT is inside this run's claim window, OLD is a claim that predates it.
NOW="$(date -u +%s)"; OLD=$(( NOW - 100000 )); FUT=$(( NOW + 60 ))

make_gate_fixture(){
  make_fixture
  mkdir -p "$SBX/runners"
  cat > "$SBX/runners/ad.sh" <<'AD'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${AD_LOG:?}"
printf 'adapter\n' >> "${ORDER_LOG:?}"
# each invocation advances the "after" snapshot to the next staged file, if one exists
n=$(wc -l < "${AD_LOG:?}" | tr -d ' ')
[ -f "${SNAP_DIR:?}/after.$n" ] && cp "${SNAP_DIR}/after.$n" "${SNAP_DIR}/current"
# Per-invocation exit code: line $n of $AD_RC_FILE, defaulting to 0. Lets a case stage a FIRST
# adapter that exits non-zero (the asymmetry case (f) alone does not reach).
adrc=""
[ -n "${AD_RC_FILE:-}" ] && [ -f "$AD_RC_FILE" ] && adrc="$(sed -n "${n}p" "$AD_RC_FILE")"
# 0277: snapshot any brief handed to us, so a case can assert on a file the facade deletes as soon
# as the call returns. The last invocation to receive one wins, which is the re-dispatch's.
# With AD_RM_BRIEF set it also DELETES that brief, staging the caller's temp file vanishing while
# the delegated run is in flight — the run gate re-reads that path minutes later.
prev=""
for a in "$@"; do
  if [ "$prev" = "--brief-file" ]; then
    cp "$a" "${SBX_COPY:?}"
    [ -n "${AD_RM_BRIEF:-}" ] && rm -f "$a"
  fi
  prev="$a"
done
exit "${adrc:-0}"
AD
  chmod +x "$SBX/runners/ad.sh"
  SNAP="$SBX/snap"; mkdir -p "$SNAP"
  write_fake_vr   # default: verdicts come from $SNAP/verdict.<id>
  : > "$SBX/ad.log"; : > "$SBX/vr.log"; : > "$SBX/order.log"
  # The re-sync seam. It logs, so the ORDER of re-sync vs adapter is assertable — an
  # after-only re-sync (the pre-fix shape) leaves `adapter` as the first line.
  cat > "$SBX/fake-facade.sh" <<'FF'
#!/usr/bin/env bash
printf 'facade %s\n' "$*" >> "${ORDER_LOG:?}"
exit 0
FF
  chmod +x "$SBX/fake-facade.sh"
}
# write_fake_vr [VERDICT_BODY] — the stub reader. The snapshot half is shared by every case, so it
# lives here once; a case needing its own verdict logic passes a body rather than restating the
# snapshot half (and drifting from the real reader's two output shapes).
write_fake_vr(){
  { cat <<'VRHEAD'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${VR_LOG:?}"
withca=0
for a in "$@"; do [ "$a" = "--with-claimed-at" ] && withca=1; done
for a in "$@"; do
  [ "$a" = "--in-progress-ids" ] || continue
  if [ "$withca" = 1 ]; then cat "${SNAP_DIR:?}/current"
  else awk '{print $1}' "${SNAP_DIR:?}/current"; fi
  exit 0
done
VRHEAD
    if [ $# -gt 0 ]; then printf '%s\n' "$1"; else cat <<'VRTAIL'
id=""
for a in "$@"; do case "$a" in [0-9]*) id="$a" ;; esac; done
cat "${SNAP_DIR:?}/verdict.$id" 2>/dev/null || printf 'run-complete %s\n' "$id"
VRTAIL
    fi
  } > "$SBX/fake-verify-run.sh"
  chmod +x "$SBX/fake-verify-run.sh"
}
run_gate(){  # $@ = facade args
  ( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" \
      SNAP_DIR="$SNAP" AD_LOG="$SBX/ad.log" VR_LOG="$SBX/vr.log" ORDER_LOG="$SBX/order.log" \
      VERIFY_RUN="$SBX/fake-verify-run.sh" DOCKET_FACADE="$SBX/fake-facade.sh" \
      SBX_COPY="$SBX/redispatch-brief-copy" AD_RM_BRIEF="${AD_RM_BRIEF:-}" \
      bash "$FACADE" "$@" )
}

# (a) a NON-implement-next agent never engages the gate — load-bearing, not an optimization:
#     a build-* delegation leaves its change in-progress BY DESIGN.
make_gate_fixture
printf '%s\n' "5 $OLD" > "$SNAP/current"; printf '%s\n' "5 $OLD" "7 $FUT" > "$SNAP/after.1"
run_gate --runner ad --agent status >/dev/null 2>&1; rc=$?
assert "0237 gate: a status delegation exits 0" '[ "$rc" = "0" ]'
assert "0237 gate: a status delegation never calls verify-run" '[ ! -s "$SBX/vr.log" ]'
# A REAL linked worktree, for the same reason the payload below is passed: since 0208 the anchor
# gate is a MEMBERSHIP test, and a bare `mkdir -p` subdirectory is refused at gate 3 — before the
# adapter, before the run gate, leaving `vr.log` empty for the wrong reason.
git -C "$SBX" worktree add -q -b gateslug "$SBX/.worktrees/w" >/dev/null 2>&1
# The payload satisfies change 0277's build-* empty-payload gate. Without it the dispatch would
# abort BEFORE the run gate, leaving `vr.log` empty for the wrong reason and this assert vacuous.
run_gate --runner ad --agent build-standard --worktree "$SBX/.worktrees/w" -- "build task" >/dev/null 2>&1
# Non-vacuity floor for the assert below: the delegation must actually have REACHED the adapter,
# or "verify-run was never called" is true of a dispatch that was refused before either could run.
assert "0237 gate: fixture sanity — the build-* delegation reached the adapter" \
  'grep -qF "build-standard" "$SBX/ad.log"'
assert "0237 gate: a build-* delegation never calls verify-run" '[ ! -s "$SBX/vr.log" ]'
rm -rf "$SBX"

# (b) implement-next: an id that is new in AFTER *and* stamped inside this run's claim window is
#     this run's claim and is verified; an id already held at the handoff is not. The name is
#     deliberately narrow: what this case pins is that a claim held BEFORE the handoff is ignored,
#     which is strictly weaker than "concurrent runs never cross-fire" — cases (b2)/(b3)/(b4) carry
#     the attribution properties the set diff alone does not establish.
make_gate_fixture
printf '%s\n' "5 $OLD" > "$SNAP/current"; printf '%s\n' "5 $OLD" "7 $FUT" > "$SNAP/after.1"
printf 'run-complete 7\n' > "$SNAP/verdict.7"
run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
assert "0237 gate: implement-next exits 0 when the run completed" '[ "$rc" = "0" ]'
assert "0237 gate: verify-run was called on the NEW id" 'grep -qw 7 "$SBX/vr.log"'
assert "0237 gate: a claim HELD AT THE HANDOFF (5) is NOT verified" \
  '! grep -qE "(^| )5( |$)" "$SBX/vr.log"'
# Both snapshots must read FRESH ORIGIN state, so the re-sync is symmetric around the handoff.
# An after-only re-sync (the pre-fix shape) leaves `adapter` as order.log's first line.
assert "0237 gate: the BEFORE snapshot is preceded by a metadata re-sync" \
  '[ "$(sed -n 1p "$SBX/order.log")" = "facade preflight" ]'
assert "0237 gate: the adapter runs after that re-sync" \
  '[ "$(sed -n 2p "$SBX/order.log")" = "adapter" ]'
assert "0237 gate: the metadata is re-synced on BOTH sides of the handoff" \
  '[ "$(grep -c "^facade preflight$" "$SBX/order.log" | tr -d " ")" = "2" ]'
rm -rf "$SBX"

# (b2) THE ATTRIBUTION PROPERTY the set diff cannot supply. An abandoned in-progress change from an
#      earlier session — absent from the local tree, so absent from BEFORE even when the pre-handoff
#      re-sync fails — is new in AFTER but carries an OLD `claimed_at`. It cannot be this run's
#      claim, must not be verified, and must not spend an agent run being re-dispatched.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "7 $OLD" > "$SNAP/after.1"
printf 'run-incomplete 7 status pr branch\n' > "$SNAP/verdict.7"
err="$( run_gate --runner ad --agent implement-next 2>&1 >/dev/null )"; rc=$?
assert "0237 attribution: a claim stamped before this dispatch exits 0" '[ "$rc" = "0" ]'
assert "0237 attribution: it is never verified" '! grep -qE "(^| )7( |$)" "$SBX/vr.log"'
assert "0237 attribution: and never re-dispatched" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
assert "0237 attribution: the skip is announced, not silent" 'grep -qiF "run gate" <<<"$err"'
rm -rf "$SBX"

# (b3) an UNREADABLE claim stamp is no positive evidence of ownership — the gate acts on a positive
#      finding, never on a guess.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "7 -" > "$SNAP/after.1"
printf 'run-incomplete 7 status pr branch\n' > "$SNAP/verdict.7"
run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
assert "0237 attribution: an unreadable claimed_at exits 0" '[ "$rc" = "0" ]'
assert "0237 attribution: an unreadable claimed_at is never verified" \
  '! grep -qE "(^| )7( |$)" "$SBX/vr.log"'
assert "0237 attribution: an unreadable claimed_at is never re-dispatched" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
rm -rf "$SBX"

# (b4) TWO fresh claims inside the window — a concurrent loop claimed during our run. An
#      implement-next run claims at most one change, so neither can be attributed to us and the
#      gate must stand down rather than re-dispatch onto a change another agent is holding.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "7 $FUT" "9 $FUT" > "$SNAP/after.1"
printf 'run-incomplete 7 status pr branch\n' > "$SNAP/verdict.7"
printf 'run-incomplete 9 status pr branch\n' > "$SNAP/verdict.9"
err="$( run_gate --runner ad --agent implement-next 2>&1 >/dev/null )"; rc=$?
assert "0237 attribution: an ambiguous claim set exits 0" '[ "$rc" = "0" ]'
assert "0237 attribution: an ambiguous claim set verifies nothing" \
  '! grep -qE "^[0-9]" "$SBX/vr.log"'
assert "0237 attribution: an ambiguous claim set dispatches exactly once" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
assert "0237 attribution: and it says why" 'grep -qiF "run gate" <<<"$err"'
rm -rf "$SBX"

# (c) an EMPTY diff (drained / contended — the run claimed nothing) is a no-op.
make_gate_fixture
printf '%s\n' "5 $OLD" > "$SNAP/current"; printf '%s\n' "5 $OLD" > "$SNAP/after.1"
run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
assert "0237 gate: an empty diff exits 0" '[ "$rc" = "0" ]'
assert "0237 gate: an empty diff verifies nothing" '! grep -qE "^[0-9]" "$SBX/vr.log"'
assert "0237 gate: an empty diff dispatches exactly once" '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
rm -rf "$SBX"

# (d) run-halted NEVER re-dispatches — a halt means a human is needed — AND it is a TERMINAL
#     outcome at this seam: exit 3, its own code, distinct from the two-strikes abort's 1. Exiting
#     with the adapter's (healthy) 0 would tell a `/loop` driver to draw the next change on a
#     disposition the contract defines as stop + surface.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "9 $FUT" > "$SNAP/after.1"
printf 'run-halted 9\n' > "$SNAP/verdict.9"
err="$( run_gate --runner ad --agent implement-next 2>&1 >/dev/null )"; rc=$?
assert "0237 gate: run-halted exits 3 (its own terminal code), never the adapter's 0" '[ "$rc" = "3" ]'
assert "0237 gate: run-halted names the change on stderr" 'grep -qE "(^| )9( |$)" <<<"$err"'
assert "0237 gate: run-halted says a human is needed" 'grep -qiF "human" <<<"$err"'
assert "0237 gate: run-halted does NOT re-dispatch" '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
rm -rf "$SBX"

# (e) a broken snapshot disables the gate and warns — it never converts a healthy
#     dispatch into a failure (the facade's standing tolerant posture on this live path).
make_gate_fixture
cat > "$SBX/fake-verify-run.sh" <<'VRB'
#!/usr/bin/env bash
echo "verify-run: boom" >&2; exit 2
VRB
chmod +x "$SBX/fake-verify-run.sh"
err="$( run_gate --runner ad --agent implement-next 2>&1 >/dev/null )"; rc=$?
assert "0237 gate: an unusable snapshot does not fail the dispatch" '[ "$rc" = "0" ]'
assert "0237 gate: and it warns on stderr" 'grep -qiF "run gate" <<<"$err"'
rm -rf "$SBX"

# ---- 0237: one bounded re-dispatch, then abort-and-report -------------------------
# Mirrors docket-build's one-escalation-per-task rule: exactly one more chance, then stop.
# NOTE on `wc -l | tr -d " "`: BSD wc PADS its count with leading spaces, so a bare
# `[ "$(wc -l < f)" = "1" ]` is false even when the count is right. The `tr` is load-bearing.

# (f) run-incomplete -> ONE re-dispatch; a now-complete second verdict exits 0
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "4 $FUT" > "$SNAP/after.1"
printf 'run-incomplete 4 status pr\n' > "$SNAP/verdict.4"
write_fake_vr "# first verdict call is incomplete, second is complete
n=\$(grep -c '^4' \"\${VR_LOG:?}\" || true)
if [ \"\$n\" -le 1 ]; then printf 'run-incomplete 4 status pr\\n'; else printf 'run-complete 4\\n'; fi"
out="$( run_gate --runner ad --agent implement-next 2>"$SBX/e.log" )"; rc=$?
assert "0237 redispatch: exits 0 once the second verdict is complete" '[ "$rc" = "0" ]'
assert "0237 redispatch: the adapter ran exactly TWICE" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "2" ]'
assert "0237 redispatch: the retry carries the change id as task context" \
  'grep -qF " 4" "$SBX/ad.log"'
assert "0237 redispatch: the retry names the unmet conjuncts" \
  'grep -qE "status|pr" "$SBX/ad.log"'
assert "0237 redispatch: the retry keeps --agent implement-next" \
  'grep -qF -- "--agent implement-next" "$SBX/ad.log"'
rm -rf "$SBX"

# (f2) THE ASYMMETRY case (f) cannot reach: the FIRST adapter exits NON-ZERO (a common
#      accompaniment to a run that stopped short) and the re-dispatch drives the change to
#      run-complete. `$rc` is the first adapter's code and is stale by then — the gate's git-read
#      verdict is the stronger fact, so the facade must exit 0 rather than report a run it has just
#      proved complete as a failure. The override is scoped to the re-dispatch path; the
#      propagate-verbatim fence for a gate that took NO action is pinned by the 0/3/7 loop above
#      and by case (h).
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "4 $FUT" > "$SNAP/after.1"
export AD_RC_FILE="$SBX/ad.rc"; printf '%s\n' 7 0 > "$AD_RC_FILE"
write_fake_vr "# first verdict call is incomplete, second is complete
n=\$(grep -c '^4' \"\${VR_LOG:?}\" || true)
if [ \"\$n\" -le 1 ]; then printf 'run-incomplete 4 status pr\\n'; else printf 'run-complete 4\\n'; fi"
run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
assert "0237 redispatch: a non-zero FIRST adapter code does not survive a verified-complete retry" \
  '[ "$rc" = "0" ]'
assert "0237 redispatch: (f2) the adapter still ran exactly TWICE" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "2" ]'
unset AD_RC_FILE
rm -rf "$SBX"

# (f3) …and the override stops at SUCCESS. A halt discovered on the SECOND verdict is the same
#      terminal disposition as one discovered on the first — exit 3, never folded into the 0 above,
#      because a run that stopped deliberately is not a success.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "4 $FUT" > "$SNAP/after.1"
write_fake_vr "# first verdict call is incomplete, second is a halt
n=\$(grep -c '^4' \"\${VR_LOG:?}\" || true)
if [ \"\$n\" -le 1 ]; then printf 'run-incomplete 4 status pr\\n'; else printf 'run-halted 4\\n'; fi"
err="$( run_gate --runner ad --agent implement-next 2>&1 >/dev/null )"; rc=$?
assert "0237 redispatch: a halt on the SECOND verdict is terminal (exit 3), not a success" \
  '[ "$rc" = "3" ]'
assert "0237 redispatch: that halt says a human is needed" 'grep -qiF "human" <<<"$err"'
rm -rf "$SBX"

# (g) two strikes -> loud non-zero naming the change and the still-unmet conjuncts
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "6 $FUT" > "$SNAP/after.1"
write_fake_vr "printf 'run-incomplete 6 status pr branch\\n'"
err="$( run_gate --runner ad --agent implement-next 2>&1 >/dev/null )"; rc=$?
assert "0237 two-strikes: exits NON-ZERO" '[ "$rc" != "0" ]'
assert "0237 two-strikes: names the change id" 'grep -qE "(^| )6( |$)" <<<"$err"'
assert "0237 two-strikes: names the still-unmet conjuncts" 'grep -qF "branch" <<<"$err"'
assert "0237 two-strikes: caps the adapter at exactly TWO runs" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "2" ]'
rm -rf "$SBX"

# (h) the re-dispatch does NOT fire on run-complete / run-unclaimed, and both keep the adapter's
#     code. run-halted also never re-dispatches, but it is terminal rather than exit-0, so it has
#     its own case (d) instead of a leg here.
for v in run-complete run-unclaimed; do
  make_gate_fixture
  printf '\n' > "$SNAP/current"; printf '%s\n' "8 $FUT" > "$SNAP/after.1"
  printf '%s 8\n' "$v" > "$SNAP/verdict.8"
  run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
  assert "0237 redispatch: $v exits 0" '[ "$rc" = "0" ]'
  assert "0237 redispatch: $v dispatches exactly once" \
    '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
  rm -rf "$SBX"
done

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
assert "0277 redispatch: the re-dispatch brief still carries the original brief" \
  '[ -f "$SBX/redispatch-brief-copy" ] && grep -qxF -- "original-brief-line" "$SBX/redispatch-brief-copy"'
assert "0277 redispatch: the re-dispatch brief carries the retry context" \
  'grep -qF -- "Step 7 unmet" "$SBX/redispatch-brief-copy"'
# The combined brief is a temp file, and the facade owns its whole lifetime: it must not survive the
# re-dispatch. Read the path out of the logged argv rather than globbing TMPDIR, so a concurrent
# run's leftovers cannot make this assert pass or fail for someone else's reason.
retry_brief="$(sed -n 's/.*--brief-file \([^ ]*\).*/\1/p' <<<"$second")"
assert "0277 redispatch: the combined brief is removed once the re-dispatch returns" \
  '[ -n "$retry_brief" ] && [ ! -e "$retry_brief" ]'
rm -rf "$SBX"

# (i2) 0277: the combined-brief write is guarded HALF AND HALF, because a brace group's exit status
#      is its LAST command's — `{ cat …; printf …; } > f || die` cannot see a failed `cat`, and the
#      re-dispatch would then run on a brief holding ONLY the retry context, with the task itself
#      stripped out, while the facade reported a normal re-dispatch. The live failure mode: on the
#      synchronous verb the original brief is the CALLER's temp file, re-read only after a full
#      delegated run (minutes to tens of minutes), so TMPDIR reaping or the caller's own cleanup can
#      take it out from under the gate. The fixture stages exactly that — the fake adapter deletes
#      the brief it was handed. What is pinned is the MECHANISM, not merely "it failed": no adapter
#      may ever be handed a brief that carries the retry context without the original task.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "9 $FUT" > "$SNAP/after.1"; printf '%s\n' "9 $FUT" > "$SNAP/after.2"
printf 'run-incomplete 9 pr\n' > "$SNAP/verdict.9"
BF="$SBX/gate-brief.txt"
printf 'original-brief-line\n' > "$BF"
err="$(AD_RM_BRIEF=1 run_gate --runner ad --agent implement-next --brief-file "$BF" 2>&1 >/dev/null)"; rc=$?
assert "0277 redispatch: an unreadable original brief exits non-zero" '[ "$rc" != "0" ]'
# The refusal must be the FACADE's own, naming the brief it could not read — `cat`'s own
# "No such file" complaint also names that path, so an unqualified path match is satisfied by the
# very leak this case exists to close.
assert "0277 redispatch: the FACADE refuses, naming the unreadable original brief" \
  'grep -q -e "^runner-dispatch: .*$BF" <<<"$err"'
# THE MECHANISM ASSERT: the re-dispatch never happened, and the only brief any adapter saw is the
# intact original — never one holding the retry context alone.
assert "0277 redispatch: an unreadable original brief re-dispatches NOTHING" \
  '[ "$(wc -l < "$SBX/ad.log" | tr -d " ")" = "1" ]'
assert "0277 redispatch: no adapter is handed a brief stripped of the original task" \
  '[ -f "$SBX/redispatch-brief-copy" ] && grep -qxF -- "original-brief-line" "$SBX/redispatch-brief-copy" \
     && ! grep -qF -- "Step 7 unmet" "$SBX/redispatch-brief-copy"'
rm -rf "$SBX"

# (i3) 0277: the combined brief's separator is UNCONDITIONAL. A brief file need not end with a
#      newline, and `printf '\n%s\n'` alone then merely terminates the brief's last line instead of
#      leaving a blank one — gluing the retry context onto the final line of the task. That is the
#      boundary loss this change exists to remove, so the fixture's brief deliberately has NO
#      trailing newline and the asserts pin both halves: the brief's last line survives WHOLE, and
#      a blank line still separates it from the retry context.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '%s\n' "9 $FUT" > "$SNAP/after.1"; printf '%s\n' "9 $FUT" > "$SNAP/after.2"
printf 'run-incomplete 9 pr\n' > "$SNAP/verdict.9"
BF="$SBX/gate-brief.txt"
printf 'original-brief-line' > "$BF"   # no trailing newline — the whole point of this case
run_gate --runner ad --agent implement-next --brief-file "$BF" >/dev/null 2>&1
assert "0277 redispatch: an unterminated brief still leaves its last line whole" \
  '[ -f "$SBX/redispatch-brief-copy" ] && grep -qxF -- "original-brief-line" "$SBX/redispatch-brief-copy"'
# The blank line is the separator itself: line 1 is the brief, line 2 must be empty, and the retry
# context follows. Reading line 2 pins the SEPARATOR, not merely that both texts are present.
assert "0277 redispatch: a blank line separates the brief from the retry context" \
  '[ -z "$(sed -n 2p "$SBX/redispatch-brief-copy")" ] && grep -qF -- "Step 7 unmet" "$SBX/redispatch-brief-copy"'
rm -rf "$SBX"

exit $fail
