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

exit $fail
