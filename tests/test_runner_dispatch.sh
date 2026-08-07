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

exit $fail
