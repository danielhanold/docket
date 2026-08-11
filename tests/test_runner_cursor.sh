#!/usr/bin/env bash
# tests/test_runner_cursor.sh — the cursor runner adapter (change 0135). Mirrors runners/codex.sh:
# preflight, prompt assembly from the built-in wrapper source, verbatim model passthrough with
# effort ridden inside the model value, foreground exec, final-message relay on stdout.
# Failure posture is LOUD abort-and-report — never a silent inline fall-back, which would
# reproduce change 0135's own root cause in a new location.
# run: bash tests/test_runner_cursor.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADAPTER="$REPO/scripts/runners/cursor.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "adapter exists" '[ -f "$ADAPTER" ]'
assert "contract doc exists (test_script_contracts_coverage parity)" '[ -f "$REPO/scripts/runners/cursor.md" ]'
assert "registered in sync-agents REGISTERED_RUNNERS" 'grep -qE "^REGISTERED_RUNNERS=\"[^\"]*\bcursor\b" "$REPO/sync-agents.sh"'

# A mock cursor-agent that records its argv + counts invocations and prints a final message.
MOCK_DIR="$(mktemp -d)"
cat > "$MOCK_DIR/cursor-agent" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$MOCK_ARGV"
printf 'CALL\n' >> "${MOCK_CALLS:-/dev/null}"
printf 'MOCK-FINAL-MESSAGE\n'
exit "${MOCK_RC:-0}"
MOCK
chmod +x "$MOCK_DIR/cursor-agent"

run_adapter(){  # $@ = adapter args ; sets OUT / RC / ARGV / CALLS
  MOCK_ARGV="$MOCK_DIR/argv.txt"
  MOCK_CALLS="$MOCK_DIR/calls.txt"
  # Both logs are truncated per run: the mock writes argv only when it actually RUNS, so a run that
  # aborts in the adapter would otherwise leave the previous run's argv in place and turn any
  # "the prompt contains X" assert green on stale evidence (mirrors test_runner_opencode.sh).
  : > "$MOCK_CALLS"; : > "$MOCK_ARGV"
  OUT="$( MOCK_ARGV="$MOCK_ARGV" MOCK_CALLS="$MOCK_CALLS" MOCK_RC="${MOCK_RC:-0}" \
          CURSOR_BIN="$MOCK_DIR/cursor-agent" \
          DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" "$@" 2>/dev/null )"
  RC=$?
  ARGV="$(cat "$MOCK_ARGV" 2>/dev/null)"
  CALLS="$(grep -c CALL "$MOCK_CALLS" 2>/dev/null)"
}

# --- happy path: foreground exec, final message relayed on stdout --------------------------------
run_adapter --agent status --model gpt-5.5-medium-fast --effort high
assert "happy: exits 0"                       '[ "$RC" = "0" ]'
assert "happy: relays the child final message" 'grep -qF "MOCK-FINAL-MESSAGE" <<<"$OUT"'
assert "happy: passes -p (non-interactive print mode)" 'grep -qxF -- "-p" <<<"$ARGV"'
assert "happy: passes --output-format text"   'grep -qxF -- "--output-format" <<<"$ARGV" && grep -qxF -- "text" <<<"$ARGV"'
assert "happy: model is passed via --model"   'grep -qxF -- "--model" <<<"$ARGV"'
assert "happy: effort rides INSIDE the model value" 'grep -qF -- "gpt-5.5-medium-fast[effort=high]" <<<"$ARGV"'
assert "happy: no separate --effort flag exists on cursor-agent" '! grep -qxF -- "--effort" <<<"$ARGV"'
assert "happy: prompt carries the skills to load"  'grep -qF "docket-convention" <<<"$ARGV"'
assert "happy: prompt carries the wrapper body"    'grep -qi "refresh docket state" <<<"$ARGV"'
assert "happy: exactly one cursor-agent invocation" '[ "$CALLS" = "1" ]'

# --- passthrough args land in the prompt ---------------------------------------------------------
run_adapter --agent status -- please-do-0135
assert "passthrough: -- args reach the prompt" 'grep -qF "please-do-0135" <<<"$ARGV"'

# --- 0277: the brief-file channel + a non-lossy argv join ----------------------------------------
# The caller's brief is the child's ONLY input. It used to travel as `$*`, which joins the positional
# parameters on the first character of IFS — a multi-line brief passed as several arguments arrived
# as one line, silently. The fixture carries the characters a model-authored brief actually holds
# (single quotes, backslashes, `%`, backticks, a `--flag`-shaped line): the append must be VERBATIM.
BF="$MOCK_DIR/brief.txt"
cat > "$BF" <<'BRIEF'
line-one: build change 0277
line-two: it's got a quote, a backslash \n, a percent %s, and a `backtick`
-- --flag-shaped-line
BRIEF
run_adapter --agent status --brief-file "$BF"
assert "0277 cursor: a brief-file dispatch exits 0" '[ "$RC" = "0" ]'
assert "0277 cursor: the payload heading is present" \
  'grep -qxF -- "Additional caller arguments / task context:" <<<"$ARGV"'
assert "0277 cursor: brief line 1 lands on its own line" 'grep -qxF -- "line-one: build change 0277" <<<"$ARGV"'
assert "0277 cursor: brief line 2 lands VERBATIM on its own line" \
  'grep -qxF -- "line-two: it'"'"'s got a quote, a backslash \n, a percent %s, and a \`backtick\`" <<<"$ARGV"'
assert "0277 cursor: a --flag-shaped brief line survives intact" 'grep -qxF -- "-- --flag-shaped-line" <<<"$ARGV"'
assert "0277 cursor: the brief is NOT flattened onto one line" \
  '! grep -qF -- "line-one: build change 0277 line-two:" <<<"$ARGV"'

# The surviving argv path is non-lossy too — joined on NEWLINE, in order, not on a space.
run_adapter --agent status -- "argv-alpha" "argv-beta"
assert "0277 cursor: multiple post-\`--\` args each land on their own line" \
  'grep -qxF -- "argv-alpha" <<<"$ARGV" && grep -qxF -- "argv-beta" <<<"$ARGV"'
assert "0277 cursor: post-\`--\` args are NOT space-joined" '! grep -qF -- "argv-alpha argv-beta" <<<"$ARGV"'

# Defensive exclusion: cursor.md documents a direct hand invocation that bypasses the facade, so the
# refusal cannot live only at the facade.
: > "$MOCK_DIR/calls.txt"
ERR="$( MOCK_CALLS="$MOCK_DIR/calls.txt" CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --brief-file "$BF" -- "also argv" 2>&1 >/dev/null )"; RC=$?
assert "0277 cursor: both channels together are refused" '[ "$RC" != "0" ]'
assert "0277 cursor: the refusal says never both" 'grep -qiF "never both" <<<"$ERR"'
assert "0277 cursor: NO child was invoked" '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'

# BYTES ≠ CONTENT. The `-s` validation counts bytes while `$(cat …)` strips trailing newlines, so a
# newline-only brief passed every gate and produced an EMPTY payload — the task-context block was
# suppressed and the child ran with no task at all, silently. Both ends must use one predicate.
: > "$MOCK_DIR/calls.txt"
printf '\n\n' > "$MOCK_DIR/blank-brief.txt"
ERR="$( MOCK_CALLS="$MOCK_DIR/calls.txt" CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --brief-file "$MOCK_DIR/blank-brief.txt" 2>&1 >/dev/null )"; RC=$?
assert "0277 cursor: a newline-only brief file is refused" '[ "$RC" != "0" ]'
assert "0277 cursor: the no-content refusal names the file" 'grep -qF -- "blank-brief.txt" <<<"$ERR"'
assert "0277 cursor: NO child was invoked for a newline-only brief" \
  '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'
# The same gap on the argv leg: arity is not content, so `-- ""` is arguments-present, payload-empty.
: > "$MOCK_DIR/calls.txt"
ERR="$( MOCK_CALLS="$MOCK_DIR/calls.txt" CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status -- "" 2>&1 >/dev/null )"; RC=$?
assert "0277 cursor: an empty trailing argv payload is refused" '[ "$RC" != "0" ]'
assert "0277 cursor: NO child was invoked for an empty argv payload" \
  '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'

# --- no effort => BARE model, no bracket ---------------------------------------------------------
run_adapter --agent status --model gpt-5.5-medium-fast
assert "no effort: model passed bare" 'grep -qxF -- "gpt-5.5-medium-fast" <<<"$ARGV"'
assert "no effort: no bracket encoding" '! grep -qF -- "[effort=" <<<"$ARGV"'

# --- effort 'auto' => treated as no pin, bare model ----------------------------------------------
run_adapter --agent status --model gpt-5.5-medium-fast --effort auto
assert "auto effort: model passed bare" 'grep -qxF -- "gpt-5.5-medium-fast" <<<"$ARGV"'

# --- no model + an effort => effort DROPPED with a warn (mirrors the emitter's edge case) --------
: > "$MOCK_DIR/argv.txt"
ERR="$( MOCK_ARGV="$MOCK_DIR/argv.txt" CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --effort high 2>&1 >/dev/null )"
assert "no model: effort dropped with a WARN" 'grep -qi "effort" <<<"$ERR" && grep -qi "dropped" <<<"$ERR"'
assert "no model: no --model flag passed" '! grep -qxF -- "--model" "$MOCK_DIR/argv.txt"'
assert "no model: child still ran (drop is not an abort)" 'grep -qxF -- "-p" "$MOCK_DIR/argv.txt"'

# --- `inherit` is docket's own NO-PIN sentinel, not a vendor model ID -----------------------------
# An explicit `--model inherit` must behave EXACTLY like no model at all: no --model flag reaches
# cursor-agent, and the effort-dropped WARN fires. Handing the literal `inherit[effort=xhigh]` to
# cursor-agent would hit its compatible-model fallback — a silently-substituted model AND a
# silently-destroyed effort pin, i.e. this change's own root cause reproduced in the adapter.
: > "$MOCK_DIR/argv.txt"
ERR="$( MOCK_ARGV="$MOCK_DIR/argv.txt" CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --model inherit --effort xhigh 2>&1 >/dev/null )"
assert "inherit sentinel: no --model flag passed" '! grep -qxF -- "--model" "$MOCK_DIR/argv.txt"'
assert "inherit sentinel: the literal sentinel never reaches cursor-agent" \
  '! grep -qF -- "inherit[effort=" "$MOCK_DIR/argv.txt"'
assert "inherit sentinel: effort dropped with a WARN (the -z MODEL branch is reached)" \
  'grep -qi "effort" <<<"$ERR" && grep -qi "dropped" <<<"$ERR"'
assert "inherit sentinel: child still ran (drop is not an abort)" 'grep -qxF -- "-p" "$MOCK_DIR/argv.txt"'

# --- preflight: binary missing => loud abort, NEVER a degrade ------------------------------------
OUT="$( CURSOR_BIN="$MOCK_DIR/definitely-not-here" DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status 2>&1 )"; RC=$?
assert "preflight: missing binary exits nonzero" '[ "$RC" != "0" ]'
# NOTE: this worktree's own path contains the string "cursor-agent", so a bare
# `grep -qi cursor-agent` would pass off any shell error mentioning the path. Anchor on the
# adapter's own diagnostic prefix instead.
assert "preflight: diagnostic is the adapter's own, and names cursor-agent" 'grep -qF "runners/cursor:" <<<"$OUT" && grep -qF "cursor-agent CLI" <<<"$OUT"'
assert "preflight: never suggests running inline instead" '! grep -qi "inline" <<<"$OUT"'
assert "preflight: never suggests a fall-back" '! grep -qi "fall.back\|fallback\|instead run\|natively" <<<"$OUT"'

# --- source posture: no backgrounding, no inline-degrade path in the adapter itself ---------------
assert "source: never backgrounds the child" '! grep -qE "\"\\$\{cmd\[@\]\}\"[[:space:]]*.*&[[:space:]]*$" "$ADAPTER"'
assert "source: records the unreliability risk" 'grep -qi "unreliable" "$ADAPTER"'

# --- child nonzero propagates (abort-and-report, no retry) ---------------------------------------
MOCK_RC=7 run_adapter --agent status
assert "child nonzero: adapter propagates it" '[ "$RC" = "7" ]'
assert "child nonzero: no retry (single invocation)" '[ "$CALLS" = "1" ]'
unset MOCK_RC

# --- missing DOCKET_REPO_ROOT => precondition abort ----------------------------------------------
OUT="$( CURSOR_BIN="$MOCK_DIR/cursor-agent" bash "$ADAPTER" --agent status 2>&1 )"; RC=$?
assert "precondition: unset DOCKET_REPO_ROOT aborts" '[ "$RC" != "0" ]'
assert "precondition: names runner-dispatch as the entry point" 'grep -qi "runner-dispatch" <<<"$OUT"'

# --- unknown agent => precondition abort ----------------------------------------------------------
OUT="$( CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" --agent nope 2>&1 )"; RC=$?
assert "precondition: unknown agent aborts" '[ "$RC" != "0" ]'
assert "precondition: names the expected source path" 'grep -qF "docket-nope.md" <<<"$OUT"'

# --- unknown argument => precondition abort -------------------------------------------------------
OUT="$( CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" --bogus x 2>&1 )"; RC=$?
assert "precondition: unknown argument aborts" '[ "$RC" != "0" ]'

# --- missing --agent => precondition abort --------------------------------------------------------
OUT="$( CURSOR_BIN="$MOCK_DIR/cursor-agent" DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" 2>&1 )"; RC=$?
assert "precondition: missing --agent aborts" '[ "$RC" != "0" ]'

# --- 0208 leg (c): a valueless trailing flag must die, never hang ---------------------------------
# `--agent`, `--model` and `--effort` end in `shift 2`; bash's `shift` FAILS rather than truncating
# at `$# = 1`, this loop has no trailing shift, and the adapter runs under `set -uo pipefail` with
# no `-e` — so a valueless flag in FINAL position spun the parse loop forever. Measured before the
# fix: all three arms returned HUNG under a 3s bound. This adapter is a DOCUMENTED direct
# hand-invocation entry point (scripts/runners/cursor.md), so "the facade never emits a valueless
# flag" does not close the path — the facade's own guard is the twin of this one, not a substitute.
# Probed with NO DOCKET_REPO_ROOT and no mock binary on purpose: the parse loop runs before every
# one of those checks, so a healthy adapter must still refuse inside the loop.
# shellcheck source=lib/bounded_arg_probe.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/bounded_arg_probe.sh"
BOUNDED_DIR="$MOCK_DIR"; BOUNDED_CWD="$MOCK_DIR"
BOUND_ERR="$(bounded_probe_err)"   # the caller's own copy — see bounded_probe_err in the lib
for f in agent model effort; do
  rc="$(run_bounded_cmd 3 bash "$ADAPTER" --"$f")"
  # Pinned on the MECHANISM, not merely on "it failed" (LEARNINGS: assert-pins-outcome-not-mechanism):
  # `HUNG` and a non-zero code are different outcomes, and only one of them is this leg's subject.
  assert "0208(c): trailing --$f exits rather than hanging" '[ "$rc" != "HUNG" ]'
  assert "0208(c): trailing --$f exits nonzero" '[ "$rc" != "0" ]'
  assert "0208(c): trailing --$f says it requires a value" \
    'grep -qF -- "--'"$f"' requires a value" "$BOUND_ERR"'
done

rm -rf "$MOCK_DIR"
echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
