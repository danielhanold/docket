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
mkdir -p "$SBX/.worktrees/featslug" "$SBX/sub/dir"

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
      bash "$FACADE" "$@" )
}

# (a) a NON-implement-next agent never engages the gate — load-bearing, not an optimization:
#     a build-* delegation leaves its change in-progress BY DESIGN.
make_gate_fixture
printf '%s\n' "5 $OLD" > "$SNAP/current"; printf '%s\n' "5 $OLD" "7 $FUT" > "$SNAP/after.1"
run_gate --runner ad --agent status >/dev/null 2>&1; rc=$?
assert "0237 gate: a status delegation exits 0" '[ "$rc" = "0" ]'
assert "0237 gate: a status delegation never calls verify-run" '[ ! -s "$SBX/vr.log" ]'
mkdir -p "$SBX/.worktrees/w"
run_gate --runner ad --agent build-standard --worktree "$SBX/.worktrees/w" >/dev/null 2>&1
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

exit $fail
