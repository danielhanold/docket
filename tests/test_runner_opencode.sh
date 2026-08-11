#!/usr/bin/env bash
# tests/test_runner_opencode.sh — the opencode runner adapter (change 0205). Mirrors
# runners/cursor.sh: binary-only preflight, prompt assembly from the built-in wrapper source,
# verbatim model passthrough, effort -> --variant, foreground exec, verbatim stdout relay.
# The `permissions` knob is the one shape with no sibling: `ask` (and unset) is a REFUSAL that
# must abort BEFORE any child process is invoked, because a delegated run cannot answer
# opencode's approval prompt and would hang until it timed out.
# run: bash tests/test_runner_opencode.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADAPTER="$REPO/scripts/runners/opencode.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "adapter exists" '[ -f "$ADAPTER" ]'
assert "contract doc exists (test_script_contracts_coverage parity)" '[ -f "$REPO/scripts/runners/opencode.md" ]'
assert "registered in sync-agents REGISTERED_RUNNERS" 'grep -qE "^REGISTERED_RUNNERS=\"[^\"]*\bopencode\b" "$REPO/sync-agents.sh"'

# A mock opencode that records its argv + counts invocations and prints a final message.
MOCK_DIR="$(mktemp -d)"
cat > "$MOCK_DIR/opencode" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$MOCK_ARGV"
# The prompt is MULTI-LINE, so $MOCK_ARGV's line count is not the arg count and line positions are
# not arg positions. Record the arg count and the index of `--` separately, so a positional assert
# (is `--` the second-to-last arg?) is possible at all.
printf '%s\n' "$#" > "${MOCK_ARGC:-/dev/null}"
i=1; for a in "$@"; do [ "$a" = "--" ] && printf '%s\n' "$i" > "${MOCK_DASHPOS:-/dev/null}"; i=$((i+1)); done
printf 'CALL\n' >> "${MOCK_CALLS:-/dev/null}"
printf 'MOCK-FINAL-MESSAGE\n'
exit "${MOCK_RC:-0}"
MOCK
chmod +x "$MOCK_DIR/opencode"

run_adapter(){  # $@ = adapter args ; sets OUT / RC / ARGV / CALLS / ARGC / DASHPOS
  MOCK_ARGV="$MOCK_DIR/argv.txt"
  MOCK_CALLS="$MOCK_DIR/calls.txt"
  MOCK_ARGC="$MOCK_DIR/argc.txt"
  MOCK_DASHPOS="$MOCK_DIR/dashpos.txt"
  : > "$MOCK_CALLS"; : > "$MOCK_ARGV"; : > "$MOCK_ARGC"; : > "$MOCK_DASHPOS"
  OUT="$( MOCK_ARGV="$MOCK_ARGV" MOCK_CALLS="$MOCK_CALLS" MOCK_RC="${MOCK_RC:-0}" \
          MOCK_ARGC="$MOCK_ARGC" MOCK_DASHPOS="$MOCK_DASHPOS" \
          OPENCODE_BIN="$MOCK_DIR/opencode" \
          DOCKET_RUNNER_CFG_PERMISSIONS="${PERM:-auto-approve}" \
          DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" "$@" 2>/dev/null )"
  RC=$?
  ARGV="$(cat "$MOCK_ARGV" 2>/dev/null)"
  CALLS="$(grep -c CALL "$MOCK_CALLS" 2>/dev/null)"
  ARGC="$(cat "$MOCK_ARGC" 2>/dev/null)"
  DASHPOS="$(cat "$MOCK_DASHPOS" 2>/dev/null)"
}

# --- happy path: foreground exec, stdout relayed verbatim ----------------------------------------
run_adapter --agent status --model openrouter/deepseek/deepseek-v4-flash-0731 --effort high
assert "happy: exits 0"                        '[ "$RC" = "0" ]'
assert "happy: relays the child final message" 'grep -qF "MOCK-FINAL-MESSAGE" <<<"$OUT"'
assert "happy: invokes the run subcommand"     'grep -qxF -- "run" <<<"$ARGV"'
assert "happy: model is passed via --model"    'grep -qxF -- "--model" <<<"$ARGV"'
assert "happy: model value is verbatim (ADR-0015)" \
  'grep -qxF -- "openrouter/deepseek/deepseek-v4-flash-0731" <<<"$ARGV"'
assert "happy: effort maps to --variant"       'grep -qxF -- "--variant" <<<"$ARGV" && grep -qxF -- "high" <<<"$ARGV"'
assert "happy: repo root maps to --dir"        'grep -qxF -- "--dir" <<<"$ARGV" && grep -qxF -- "$REPO" <<<"$ARGV"'
assert "happy: prompt carries the skills to load" 'grep -qF "docket-convention" <<<"$ARGV"'
assert "happy: prompt carries the wrapper body"   'grep -qi "refresh docket state" <<<"$ARGV"'
assert "happy: exactly one opencode invocation"   '[ "$CALLS" = "1" ]'
# opencode's -p is --password, NOT print mode. Copying cursor.sh's -p would silently hand the
# prompt to a basic-auth flag. This assert detects that specific mis-port.
assert "happy: never passes -p (opencode's -p is --password, not print)" '! grep -qxF -- "-p" <<<"$ARGV"'
# `--` must end option parsing, so a prompt that happens to open with a flag is still taken as the
# message. Positional, not a presence grep: `--` must be the SECOND-TO-LAST arg, i.e. immediately
# before the prompt. A bare presence check would pass with the sentinel emitted anywhere.
# (Verified against opencode 1.18.11: `opencode run --version` prints the version, while
# `opencode run -- --version` sends `--version` as the message.)
assert "happy: -- is the arg immediately before the positional prompt" \
  '[ -n "$DASHPOS" ] && [ -n "$ARGC" ] && [ "$DASHPOS" = "$(( ARGC - 1 ))" ]'

# --- docket's `max` passes through unmapped (unlike codex, which maps max -> xhigh) ---------------
run_adapter --agent status --model openrouter/moonshotai/kimi-k3 --effort max
assert "max effort: passed to --variant unmapped" 'grep -qxF -- "max" <<<"$ARGV"'
assert "max effort: never rewritten to codex's xhigh" '! grep -qxF -- "xhigh" <<<"$ARGV"'

# --- passthrough args land in the prompt ---------------------------------------------------------
run_adapter --agent status --model m/x/y -- please-do-0205
assert "passthrough: -- args reach the prompt" 'grep -qF "please-do-0205" <<<"$ARGV"'

# --- 0277: the brief-file channel + a non-lossy argv join -----------------------------------------
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
run_adapter --agent status --model m/x/y --brief-file "$BF"
assert "0277 opencode: a brief-file dispatch exits 0" '[ "$RC" = "0" ]'
assert "0277 opencode: the payload heading is present" \
  'grep -qxF -- "Additional caller arguments / task context:" <<<"$ARGV"'
assert "0277 opencode: brief line 1 lands on its own line" 'grep -qxF -- "line-one: build change 0277" <<<"$ARGV"'
assert "0277 opencode: brief line 2 lands VERBATIM on its own line" \
  'grep -qxF -- "line-two: it'"'"'s got a quote, a backslash \n, a percent %s, and a \`backtick\`" <<<"$ARGV"'
assert "0277 opencode: a --flag-shaped brief line survives intact" 'grep -qxF -- "-- --flag-shaped-line" <<<"$ARGV"'
assert "0277 opencode: the brief is NOT flattened onto one line" \
  '! grep -qF -- "line-one: build change 0277 line-two:" <<<"$ARGV"'

# The surviving argv path is non-lossy too — joined on NEWLINE, in order, not on a space.
run_adapter --agent status --model m/x/y -- "argv-alpha" "argv-beta"
assert '0277 opencode: multiple post-`--` args each land on their own line' \
  'grep -qxF -- "argv-alpha" <<<"$ARGV" && grep -qxF -- "argv-beta" <<<"$ARGV"'
assert '0277 opencode: post-`--` args are NOT space-joined' '! grep -qF -- "argv-alpha argv-beta" <<<"$ARGV"'

# Defensive exclusion: opencode.md documents a direct hand invocation that bypasses the facade, so
# the refusal cannot live only at the facade.
: > "$MOCK_DIR/calls.txt"
ERR="$( MOCK_CALLS="$MOCK_DIR/calls.txt" OPENCODE_BIN="$MOCK_DIR/opencode" \
        DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --model m/x/y --brief-file "$BF" -- "also argv" 2>&1 >/dev/null )"; RC=$?
assert "0277 opencode: both channels together are refused" '[ "$RC" != "0" ]'
assert "0277 opencode: the refusal says never both" 'grep -qiF "never both" <<<"$ERR"'
assert "0277 opencode: NO child was invoked" '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'

# BYTES ≠ CONTENT. The `-s` validation counts bytes while `$(cat …)` strips trailing newlines, so a
# newline-only brief passed every gate and produced an EMPTY payload — the task-context block was
# suppressed and the child ran with no task at all, silently. Both ends must use one predicate.
: > "$MOCK_DIR/calls.txt"
printf '\n\n' > "$MOCK_DIR/blank-brief.txt"
ERR="$( MOCK_CALLS="$MOCK_DIR/calls.txt" OPENCODE_BIN="$MOCK_DIR/opencode" \
        DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --model m/x/y --brief-file "$MOCK_DIR/blank-brief.txt" 2>&1 >/dev/null )"; RC=$?
assert "0277 opencode: a newline-only brief file is refused" '[ "$RC" != "0" ]'
assert "0277 opencode: the no-content refusal names the file" 'grep -qF -- "blank-brief.txt" <<<"$ERR"'
assert "0277 opencode: NO child was invoked for a newline-only brief" \
  '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'
# The same gap on the argv leg: arity is not content, so `-- ""` is arguments-present, payload-empty.
: > "$MOCK_DIR/calls.txt"
ERR="$( MOCK_CALLS="$MOCK_DIR/calls.txt" OPENCODE_BIN="$MOCK_DIR/opencode" \
        DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --model m/x/y -- "" 2>&1 >/dev/null )"; RC=$?
assert "0277 opencode: an empty trailing argv payload is refused" '[ "$RC" != "0" ]'
assert "0277 opencode: NO child was invoked for an empty argv payload" \
  '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'

# --- no effort => no --variant flag ---------------------------------------------------------------
run_adapter --agent status --model openrouter/deepseek/deepseek-v4-flash-0731
assert "no effort: no --variant flag emitted" '! grep -qxF -- "--variant" <<<"$ARGV"'
assert "no effort: model still passed"        'grep -qxF -- "--model" <<<"$ARGV"'

# --- effort 'auto' => treated as no pin ------------------------------------------------------------
run_adapter --agent status --model openrouter/deepseek/deepseek-v4-flash-0731 --effort auto
assert "auto effort: no --variant flag emitted" '! grep -qxF -- "--variant" <<<"$ARGV"'

# --- no model + an effort => effort DROPPED with a warn -------------------------------------------
: > "$MOCK_DIR/argv.txt"
ERR="$( MOCK_ARGV="$MOCK_DIR/argv.txt" OPENCODE_BIN="$MOCK_DIR/opencode" \
        DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --effort high 2>&1 >/dev/null )"
assert "no model: effort dropped with a WARN" 'grep -qi "effort" <<<"$ERR" && grep -qi "dropped" <<<"$ERR"'
assert "no model: no --variant flag passed"  '! grep -qxF -- "--variant" "$MOCK_DIR/argv.txt"'
assert "no model: child still ran (drop is not an abort)" 'grep -qxF -- "run" "$MOCK_DIR/argv.txt"'

# --- `inherit` is docket's own NO-PIN sentinel, not a vendor model ID ------------------------------
: > "$MOCK_DIR/argv.txt"
ERR="$( MOCK_ARGV="$MOCK_DIR/argv.txt" OPENCODE_BIN="$MOCK_DIR/opencode" \
        DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --model inherit --effort high 2>&1 >/dev/null )"
assert "inherit sentinel: no --model flag passed" '! grep -qxF -- "--model" "$MOCK_DIR/argv.txt"'
assert "inherit sentinel: the literal sentinel never reaches opencode" \
  '! grep -qxF -- "inherit" "$MOCK_DIR/argv.txt"'
assert "inherit sentinel: effort dropped with a WARN (the -z MODEL branch is reached)" \
  'grep -qi "dropped" <<<"$ERR"'
assert "inherit sentinel: child still ran (drop is not an abort)" 'grep -qxF -- "run" "$MOCK_DIR/argv.txt"'

# --- permissions: auto-approve => --auto -----------------------------------------------------------
PERM=auto-approve run_adapter --agent status --model m/x/y
assert "permissions auto-approve: passes --auto" 'grep -qxF -- "--auto" <<<"$ARGV"'

# --- permissions: ask (explicit) => REFUSAL, and NO child process ----------------------------------
# The load-bearing assert of this file. Under `ask` the child would block on an approval prompt no
# delegated run can answer, so the refusal must happen BEFORE the invocation, not after it.
: > "$MOCK_DIR/argv.txt"; : > "$MOCK_DIR/calls.txt"
OUT="$( MOCK_ARGV="$MOCK_DIR/argv.txt" MOCK_CALLS="$MOCK_DIR/calls.txt" \
        OPENCODE_BIN="$MOCK_DIR/opencode" DOCKET_RUNNER_CFG_PERMISSIONS=ask \
        DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" --agent status --model m/x/y 2>&1 )"; RC=$?
assert "permissions ask: exits nonzero"       '[ "$RC" != "0" ]'
assert "permissions ask: NO child was invoked" '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'
assert "permissions ask: diagnostic names the knob" 'grep -qF "runners.opencode.permissions" <<<"$OUT"'
assert "permissions ask: diagnostic names the working value" 'grep -qF "auto-approve" <<<"$OUT"'
assert "permissions ask: never suggests running inline instead" '! grep -qi "inline" <<<"$OUT"'

# --- permissions: UNSET defaults to ask => same refusal --------------------------------------------
: > "$MOCK_DIR/calls.txt"
OUT="$( MOCK_CALLS="$MOCK_DIR/calls.txt" OPENCODE_BIN="$MOCK_DIR/opencode" \
        DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" --agent status --model m/x/y 2>&1 )"; RC=$?
assert "permissions unset: defaults to ask (nonzero)" '[ "$RC" != "0" ]'
assert "permissions unset: NO child was invoked" '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'

# --- permissions: unknown value => loud refusal, never a silent fall-back to ask -------------------
: > "$MOCK_DIR/calls.txt"
OUT="$( MOCK_CALLS="$MOCK_DIR/calls.txt" OPENCODE_BIN="$MOCK_DIR/opencode" \
        DOCKET_RUNNER_CFG_PERMISSIONS=yolo DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --model m/x/y 2>&1 )"; RC=$?
assert "permissions unknown: exits nonzero" '[ "$RC" != "0" ]'
assert "permissions unknown: echoes the offending value" 'grep -qF "yolo" <<<"$OUT"'
assert "permissions unknown: NO child was invoked" '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'
assert "permissions unknown: remedy names the unquoted-value rule (ADR-0065)" 'grep -qF "unquoted" <<<"$OUT"'

# --- a QUOTED value is the realistic way to reach that leg -----------------------------------------
# runner-dispatch.sh's block-mapping reader does not strip quotes, so `permissions: "auto-approve"`
# arrives here as the literal `"auto-approve"` — a correct-looking config that fails. Without the
# quoting remedy the diagnostic just echoes the value back with its quotes, which reads as noise.
: > "$MOCK_DIR/calls.txt"
OUT="$( MOCK_CALLS="$MOCK_DIR/calls.txt" OPENCODE_BIN="$MOCK_DIR/opencode" \
        DOCKET_RUNNER_CFG_PERMISSIONS='"auto-approve"' DOCKET_REPO_ROOT="$REPO" \
        bash "$ADAPTER" --agent status --model m/x/y 2>&1 )"; RC=$?
assert "quoted value: refuses rather than silently accepting it" '[ "$RC" != "0" ]'
assert "quoted value: NO child was invoked" '[ "$(grep -c CALL "$MOCK_DIR/calls.txt")" = "0" ]'
assert "quoted value: remedy names the unquoted-value rule" 'grep -qF "unquoted" <<<"$OUT"'

# --- preflight: binary missing => loud abort, NEVER a degrade --------------------------------------
OUT="$( OPENCODE_BIN="$MOCK_DIR/definitely-not-here" DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve \
        DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" --agent status --model m/x/y 2>&1 )"; RC=$?
assert "preflight: missing binary exits nonzero" '[ "$RC" != "0" ]'
# Anchor on the adapter's OWN diagnostic prefix: a bare grep for "opencode" would be satisfied by
# any shell error mentioning the mock path, which contains the word.
assert "preflight: diagnostic is the adapter's own, and names the CLI" \
  'grep -qF "runners/opencode:" <<<"$OUT" && grep -qF "opencode CLI" <<<"$OUT"'
assert "preflight: never suggests a fall-back" '! grep -qiE "fall.back|fallback|instead run|natively" <<<"$OUT"'

# --- source posture: no backgrounding ---------------------------------------------------------------
assert "source: never backgrounds the child" '! grep -qE "\"\\$\{cmd\[@\]\}\"[[:space:]]*.*&[[:space:]]*$" "$ADAPTER"'

# --- child nonzero propagates (abort-and-report, no retry) ------------------------------------------
MOCK_RC=7 run_adapter --agent status --model m/x/y
assert "child nonzero: adapter propagates it" '[ "$RC" = "7" ]'
assert "child nonzero: no retry (single invocation)" '[ "$CALLS" = "1" ]'
unset MOCK_RC

# --- missing DOCKET_REPO_ROOT => precondition abort --------------------------------------------------
OUT="$( OPENCODE_BIN="$MOCK_DIR/opencode" DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve \
        bash "$ADAPTER" --agent status 2>&1 )"; RC=$?
assert "precondition: unset DOCKET_REPO_ROOT aborts" '[ "$RC" != "0" ]'
assert "precondition: names runner-dispatch as the entry point" 'grep -qi "runner-dispatch" <<<"$OUT"'

# --- unknown agent / unknown argument / missing --agent => precondition aborts -------------------------
OUT="$( OPENCODE_BIN="$MOCK_DIR/opencode" DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve \
        DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" --agent nope 2>&1 )"; RC=$?
assert "precondition: unknown agent aborts" '[ "$RC" != "0" ]'
assert "precondition: names the expected source path" 'grep -qF "docket-nope.md" <<<"$OUT"'
OUT="$( OPENCODE_BIN="$MOCK_DIR/opencode" DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve \
        DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" --bogus x 2>&1 )"; RC=$?
assert "precondition: unknown argument aborts" '[ "$RC" != "0" ]'
OUT="$( OPENCODE_BIN="$MOCK_DIR/opencode" DOCKET_RUNNER_CFG_PERMISSIONS=auto-approve \
        DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" 2>&1 )"; RC=$?
assert "precondition: missing --agent aborts" '[ "$RC" != "0" ]'

# --- 0208 leg (c): a valueless trailing flag must die, never hang ---------------------------------
# `--agent`, `--model` and `--effort` end in `shift 2`; bash's `shift` FAILS rather than truncating
# at `$# = 1`, this loop has no trailing shift, and the adapter runs under `set -uo pipefail` with
# no `-e` — so a valueless flag in FINAL position spun the parse loop forever. Measured before the
# fix: all three arms returned HUNG under a 3s bound. This adapter is a DOCUMENTED direct
# hand-invocation entry point (scripts/runners/opencode.md), so "the facade never emits a valueless
# flag" does not close the path — the facade's own guard is the twin of this one, not a substitute.
# Probed with NO DOCKET_REPO_ROOT and no mock binary on purpose: the parse loop runs before every
# one of those checks — including the `permissions` refusal — so a healthy adapter must still
# refuse inside the loop.
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
