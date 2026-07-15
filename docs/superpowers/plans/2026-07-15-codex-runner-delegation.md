# Cross-Harness Runner Delegation (first runner: Codex) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an agent's whole run be delegated from the parent harness (Claude Code) to a child harness (OpenAI Codex) via an explicit `runner:` config key, a shim wrapper, and a deterministic dispatch facade + per-runner adapter.

**Architecture:** Three seams, each keyed by runner name: (1) config — `runner:` per agent entry + a `runners.<name>:` knob block; (2) generation — a runner registry in `sync-agents.sh` that swaps the generated wrapper *body* for a one-call shim; (3) dispatch — `scripts/runner-dispatch.sh` (facade, behind `docket.sh runner-dispatch`) hands off to `scripts/runners/codex.sh` (adapter wrapping `codex exec`). Spec: `docs/superpowers/specs/2026-07-15-codex-runner-delegation-design.md` (on the `docket` branch).

**Tech Stack:** bash 3.2-compatible shell (macOS + Linux), awk/sed (BSD+GNU portable), the existing tests/ assert conventions, `codex` CLI ≥ 0.144 (`exec`, `--output-last-message`, `login status` — all verified on the build machine, codex-cli 0.144.4).

## Global Constraints

- **ADR-0015 passthrough:** `model:` values are opaque strings, passed verbatim to the child (`codex exec -m`). Never validate or map model IDs.
- **ADR-0012 script-vs-model boundary:** all mechanics in scripts with co-located `.md` contracts; skills/wrappers only invoke.
- **ADR-0034 cwd-independence (change 0075):** never derive the repo root from `$PWD` semantics alone — use `scripts/lib/docket-root.sh`'s `docket_main_worktree`.
- **Abort-and-report:** every runtime failure (missing binary, no auth, child nonzero, unknown runner) exits nonzero with a stderr diagnostic. Never degrade a `runner:`-configured agent to a native run.
- **Unregistered `runner:` under `claude` is a loud generation-time ERROR** (sync-agents exits nonzero). `runner:` under any non-claude harness is **warned-and-ignored** (reserved).
- **Shell portability (LEARNINGS):** no `producer | grep -q`/`head` under pipefail — capture to a variable first; `grep -qF -- "$pat"` for patterns that may lead with `--`; `[^[:space:]]` never `[^ ]` in awk; `pwd -P` both sides before prefix-stripping mktemp paths.
- **Guards are code (LEARNINGS):** every new test assertion must be mutation-tested (strip the guarded clause, watch it redden). Mocks must mirror the real tool's output shape (the fake `codex` must accept the real flag grammar and write the `--output-last-message` file).
- **Do not restructure `sync-agents.sh`** beyond the additive registry + emission seam — change 0077 is concurrently reshaping the same file; keep the diff surface additive (new functions + small call-site edits), never a reflow.

---

## File structure

| File | Responsibility |
|---|---|
| `scripts/runners/codex.sh` (new) | Codex adapter: preflight, prompt assembly, flag mapping, foreground `codex exec`, final-message relay |
| `scripts/runners/codex.md` (new) | Adapter contract (incl. prerequisites + `runners.codex` keys) |
| `scripts/runner-dispatch.sh` (new) | Runner-neutral facade: arg validation, repo-root anchor, `runners.<name>:` config resolution, adapter handoff |
| `scripts/runner-dispatch.md` (new) | Facade contract |
| `scripts/docket.sh` (modify) | `runner-dispatch` joins `WRAPPED_OPS` + usage table |
| `scripts/docket.md` (modify) | Facade contract table gains the op row |
| `sync-agents.sh` (modify) | `runner` field resolution, runner registry, shim body emission, warnings/errors, `--check` leg (c) parity |
| `tests/test_runner_dispatch.sh` (new) | Facade + adapter hermetic tests (fake `codex` on PATH) |
| `tests/test_sync_agents.sh` (modify) | Shim-generation, error/warn, registry↔adapter parity, `--check` coverage |
| `tests/test_script_contracts_coverage.sh` (modify) | Extend the existence audit to `scripts/runners/` |
| `README.md` (modify) | `### Runner delegation` under `## Customization`; pointer in `## Tuning agent models & effort`; sample `.docket.yml` keys |
| `skills/docket-convention/references/agent-layer.md` (modify) | `runner:` documented beside `model:`/`effort:` |

---

### Task 1: Codex adapter — `scripts/runners/codex.sh`

**Files:**
- Create: `scripts/runners/codex.sh`
- Create: `scripts/runners/codex.md`
- Test: `tests/test_runner_dispatch.sh` (adapter section)

**Interfaces:**
- Consumes: `scripts/lib/docket-root.sh` (nothing — the *facade* anchors; adapter trusts env), the built-in wrapper sources `agents/docket-*.md`.
- Produces (later tasks rely on): CLI `codex.sh --agent <name> [--model <m>] [--effort <e>] [--] [<args…>]`; env in: `DOCKET_REPO_ROOT` (absolute, required), `DOCKET_RUNNER_CFG_SANDBOX` (default `workspace-write`), `DOCKET_RUNNER_CFG_NETWORK` (default `true`); mock seam `CODEX_BIN="${CODEX_BIN:-codex}"`. stdout = the child's final message ONLY; codex's own event stream goes to stderr; nonzero exit on any failure.

- [ ] **Step 1: Write the failing adapter tests**

Create `tests/test_runner_dispatch.sh` with the shared fixture helpers and the adapter cases. Follow the house style (`set -uo pipefail`, `fail=0`, `assert(){ if eval "$2"; …}`).

```bash
#!/usr/bin/env bash
# tests/test_runner_dispatch.sh — run: bash tests/test_runner_dispatch.sh
# Hermetic: a fake `codex` binary on PATH records argv and mimics the real CLI's
# flag grammar (login status / exec / --output-last-message) — LEARNINGS: a
# tool-output mock must mirror the real tool's shape.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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

run_adapter(){  # $@ = adapter args; env comes from caller
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
assert "argv: -C repo root" 'grep -qxF -- "$SBX" <<<"$argv" && grep -qxF -- "-C" <<<"$argv"'
assert "argv: model passthrough verbatim" 'grep -qxF -- "gpt-5.1-codex" <<<"$argv"'
assert "argv: default sandbox workspace-write" 'grep -qxF -- "workspace-write" <<<"$argv"'
assert "argv: network access -c override present by default" 'grep -qxF -- "sandbox_workspace_write.network_access=true" <<<"$argv"'
assert "argv: effort mapped to model_reasoning_effort" 'grep -qxF -- "model_reasoning_effort=high" <<<"$argv"'
assert "argv: --output-last-message present" 'grep -qxF -- "--output-last-message" <<<"$argv"'
assert "argv: exactly one exec call recorded" '[ "$(grep -cxF -- "--output-last-message" "$LOG")" = "1" ]'
# prompt content: last argv line is the prompt; it names the skill + carries the wrapper body
prompt="$(tail -n1 "$LOG")"
assert "prompt names skill docket-status" 'grep -qF "docket-status" <<<"$prompt"'
assert "prompt names skill docket-convention" 'grep -qF "docket-convention" <<<"$prompt"'
assert "prompt carries the wrapper body (abort-and-report)" 'grep -qi "abort-and-report" <<<"$prompt"'
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
prompt="$(tail -n1 "$LOG")"
assert "passthrough args reach the prompt" 'grep -qF "run the board-only pass" <<<"$prompt"'
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
err="$( cd "$SBX" && DOCKET_REPO_ROOT="$SBX" bash "$ADAPTER" --agent status 2>&1 >/dev/null )"; rc=$?
assert "codex missing from PATH aborts nonzero" '[ "$rc" != "0" ]'
rm -rf "$SBX"

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_runner_dispatch.sh`
Expected: FAIL (NOT OK lines / `scripts/runners/codex.sh: No such file`).

- [ ] **Step 3: Implement the adapter**

Create `scripts/runners/codex.sh`:

```bash
#!/usr/bin/env bash
# scripts/runners/codex.sh — the codex runner adapter (change 0079). Owns everything
# child-specific for delegating a whole agent run to OpenAI Codex CLI via `codex exec`:
# preflight (binary + auth), prompt assembly from the built-in wrapper source, flag
# mapping (model verbatim per ADR-0015; effort -> model_reasoning_effort; sandbox/network
# from the runners.codex config), foreground execution, final-message relay on stdout.
# Invoked by runner-dispatch.sh — not directly by skills. Contract: scripts/runners/codex.md.
# Mock seam: CODEX_BIN. Env in (from the facade): DOCKET_REPO_ROOT (absolute, required),
# DOCKET_RUNNER_CFG_SANDBOX (default workspace-write), DOCKET_RUNNER_CFG_NETWORK (default true).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SELF_DIR/../../agents"
CODEX_BIN="${CODEX_BIN:-codex}"

die(){ printf 'runners/codex: %s\n' "$*" >&2; exit 1; }

AGENT=""; MODEL=""; EFFORT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --agent)  AGENT="${2:-}"; shift 2 ;;
    --model)  MODEL="${2:-}"; shift 2 ;;
    --effort) EFFORT="${2:-}"; shift 2 ;;
    --) shift; break ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$AGENT" ] || die "--agent is required"
[ -n "${DOCKET_REPO_ROOT:-}" ] || die "DOCKET_REPO_ROOT is not set (invoke via docket.sh runner-dispatch)"

SRC="$AGENTS_SRC/docket-$AGENT.md"
[ -f "$SRC" ] || die "no built-in agent source for '$AGENT' (expected $SRC)"

# --- preflight: binary + auth (abort-and-report; never degrade to a native run) --
command -v "$CODEX_BIN" >/dev/null 2>&1 || die "codex CLI not on PATH — install Codex CLI (https://github.com/openai/codex) or unset runner: codex"
"$CODEX_BIN" login status >/dev/null 2>&1 || die "codex CLI is not authenticated — run: codex login"

# --- prompt assembly: skills to load + the wrapper body + passthrough args -------
# skills: [a, b] frontmatter line -> "a b"
skills_line="$(sed -n 's/^skills:[[:space:]]*\[\(.*\)\].*/\1/p' "$SRC" | head -n1 | tr ',' ' ')"
skills_line="$(echo $skills_line)"
# body = everything after the second frontmatter fence
body="$(awk '/^---[[:space:]]*$/{d++; next} d>=2{print}' "$SRC")"
prompt=""
if [ -n "$skills_line" ]; then
  prompt="First, load these skills by name, in this order:"
  for s in $skills_line; do prompt="$prompt
- invoke skill \`$s\`"; done
  prompt="$prompt

Then execute the following instructions exactly:

"
fi
prompt="$prompt$body"
if [ $# -gt 0 ]; then
  prompt="$prompt

Additional caller arguments / task context:
$*"
fi

# --- flag mapping -----------------------------------------------------------------
SANDBOX="${DOCKET_RUNNER_CFG_SANDBOX:-workspace-write}"
NETWORK="${DOCKET_RUNNER_CFG_NETWORK:-true}"
case "$EFFORT" in max) EFFORT="xhigh" ;; esac   # codex vocabulary tops out at xhigh

cmd=( "$CODEX_BIN" exec -C "$DOCKET_REPO_ROOT" --sandbox "$SANDBOX" --color never )
[ "$SANDBOX" = "workspace-write" ] && [ "$NETWORK" = "true" ] && \
  cmd+=( -c "sandbox_workspace_write.network_access=true" )
[ -n "$MODEL" ]  && cmd+=( -m "$MODEL" )
[ -n "$EFFORT" ] && cmd+=( -c "model_reasoning_effort=$EFFORT" )
last_msg="$(mktemp)"
cmd+=( --output-last-message "$last_msg" "$prompt" )

# --- foreground execution + final-message relay ------------------------------------
# codex's own event stream stays on stderr; THIS script's stdout is the child's final
# message only (the shim relays it verbatim).
"${cmd[@]}" 1>&2
rc=$?
if [ -s "$last_msg" ]; then cat "$last_msg"; fi
rm -f "$last_msg"
[ "$rc" = "0" ] || die "codex exec exited $rc"
exit 0
```

- [ ] **Step 4: Run the adapter tests to verify they pass**

Run: `bash tests/test_runner_dispatch.sh`
Expected: every `adapter:` assert `ok`; facade asserts still fail (facade not built yet — acceptable mid-task; if noise bothers, comment the facade section in this task and uncomment in Task 2 — but prefer writing only the adapter section now and appending the facade section in Task 2).

- [ ] **Step 5: Write the contract**

Create `scripts/runners/codex.md` documenting: Purpose (whole-run delegation adapter); Usage (CLI + env contract as in the header comment); Behavior (preflight → prompt assembly → flag mapping → foreground exec → relay); the `runners.codex` keys (`sandbox`: `workspace-write` default | `danger-full-access`; `network`: default `true`; approvals are always never — exec is non-interactive by definition); effort mapping (verbatim, `max`→`xhigh`); Exit codes (0 success; 1 precondition/abort; child's own nonzero propagated); Invariants (stdout = final message only; never degrades to native; model verbatim per ADR-0015); Prerequisites (Codex CLI installed + `codex login`; superpowers installed in Codex; `[features] multi_agent = true` in `~/.codex/config.toml` for delegated *orchestrators*' child-native fan-out — not needed for leaf agents; docket skills linked by `link-skills.sh`).

- [ ] **Step 6: Commit**

```bash
git add scripts/runners/codex.sh scripts/runners/codex.md tests/test_runner_dispatch.sh
git commit -m "feat(0079): codex runner adapter — codex exec wrapper with preflight, prompt assembly, final-message relay"
```

---

### Task 2: Dispatch facade — `scripts/runner-dispatch.sh`

**Files:**
- Create: `scripts/runner-dispatch.sh`
- Create: `scripts/runner-dispatch.md`
- Test: `tests/test_runner_dispatch.sh` (facade section, appended)

**Interfaces:**
- Consumes: `scripts/lib/docket-root.sh` (`docket_main_worktree`), `scripts/runners/<name>.sh` (Task 1's CLI).
- Produces (Task 4's shims call this через `docket.sh`): CLI `runner-dispatch.sh --runner <name> --agent <agent> [--model <m>] [--effort <e>] [--] [<args…>]`. Resolves `runners.<name>:` keys across `.docket.local.yml` > `.docket.yml` > `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml` and exports each as `DOCKET_RUNNER_CFG_<UPPERCASED-KEY>`; exports `DOCKET_REPO_ROOT`; then `exec`s the adapter. Mock seam: `RUNNERS_DIR`.

- [ ] **Step 1: Append the failing facade tests to `tests/test_runner_dispatch.sh`**

```bash
# ---- facade: validation ---------------------------------------------------------
make_fixture
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --agent status 2>&1 >/dev/null )"; rc=$?
assert "facade: missing --runner rejected" '[ "$rc" != "0" ]'
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner codex 2>&1 >/dev/null )"; rc=$?
assert "facade: missing --agent rejected" '[ "$rc" != "0" ]'
err="$( cd "$SBX" && PATH="$BIN:$PATH" bash "$FACADE" --runner gemini-cli --agent status 2>&1 >/dev/null )"; rc=$?
assert "facade: unknown runner rejected nonzero" '[ "$rc" != "0" ]'
assert "facade: unknown-runner message names it" 'grep -qF "gemini-cli" <<<"$err"'
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
```

- [ ] **Step 2: Run to verify the new section fails**

Run: `bash tests/test_runner_dispatch.sh`
Expected: adapter asserts `ok`, facade asserts NOT OK / no-such-file.

- [ ] **Step 3: Implement the facade**

Create `scripts/runner-dispatch.sh`:

```bash
#!/usr/bin/env bash
# scripts/runner-dispatch.sh — the runner-neutral delegation facade (change 0079), behind
# `docket.sh runner-dispatch`. Validates arguments, anchors the repo root (ADR-0034),
# resolves the runners.<name>: config block across layers (repo-local > repo-committed >
# global; per-key), exports it as DOCKET_RUNNER_CFG_<KEY>, and execs the named adapter
# scripts/runners/<name>.sh. Registration IS the adapter file's existence. Unknown runner
# => loud nonzero (abort-and-report). Contract: scripts/runner-dispatch.md.
# Mock seams: RUNNERS_DIR, GIT (via lib/docket-root.sh).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNERS_DIR="${RUNNERS_DIR:-$SELF_DIR/runners}"
# shellcheck source=lib/docket-root.sh
. "$SELF_DIR/lib/docket-root.sh"

die(){ printf 'runner-dispatch: %s\n' "$*" >&2; exit 1; }

RUNNER=""; AGENT=""; MODEL=""; EFFORT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --runner) RUNNER="${2:-}"; shift 2 ;;
    --agent)  AGENT="${2:-}";  shift 2 ;;
    --model)  MODEL="${2:-}";  shift 2 ;;
    --effort) EFFORT="${2:-}"; shift 2 ;;
    --) shift; break ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$RUNNER" ] || die "--runner is required"
[ -n "$AGENT" ]  || die "--agent is required"
ADAPTER="$RUNNERS_DIR/$RUNNER.sh"
[ -f "$ADAPTER" ] || die "unknown runner '$RUNNER' — no adapter at $ADAPTER (registered runners: $(ls "$RUNNERS_DIR" 2>/dev/null | sed 's/\.sh$//' | tr '\n' ' '))"

REPO_ROOT="$(docket_main_worktree)"
[ -n "$REPO_ROOT" ] || die "not inside a git repository"
export DOCKET_REPO_ROOT="$REPO_ROOT"

# --- runners.<name>: config, per-key across layers (local > committed > global) -----
# Same nested-section awk shape as sync-agents.sh's section_body/harness_agent_line
# (kept self-contained here; sync-agents.sh has the twin — see LEARNINGS on tracked twins).
runner_block(){  # $1=file -> the dedented body under runners.<RUNNER>., '' when absent
  [ -f "$1" ] || return 0
  awk -v key="runners" '
    function ind(s,   m){ m=match(s, /[^[:space:]]/); return (m==0 ? length(s) : m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    !inb { if (nc ~ ("^[[:space:]]*" key "[[:space:]]*:[[:space:]]*$")) { inb=1; kin=ind(nc) } next }
    nc ~ /[^[:space:]]/ && ind(nc) <= kin { exit }
    { if (!haveBase && nc ~ /[^[:space:]]/) { base=ind($0); haveBase=1 }
      if (haveBase) print substr($0, base+1); else print }
  ' "$1" | awk -v key="$RUNNER" '
    function ind(s,   m){ m=match(s, /[^[:space:]]/); return (m==0 ? length(s) : m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    !inb { if (nc ~ ("^[[:space:]]*" key "[[:space:]]*:[[:space:]]*$")) { inb=1; kin=ind(nc) } next }
    nc ~ /[^[:space:]]/ && ind(nc) <= kin { exit }
    { if (!haveBase && nc ~ /[^[:space:]]/) { base=ind($0); haveBase=1 }
      if (haveBase) print substr($0, base+1); else print }
  '
}

GLOBAL_CFG="${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml"
seen_keys=" "
for f in "$REPO_ROOT/.docket.local.yml" "$REPO_ROOT/.docket.yml" "$GLOBAL_CFG"; do
  blk="$(runner_block "$f")"
  [ -n "$blk" ] || continue
  while IFS= read -r line; do
    k="$(sed -nE 's/^[[:space:]]*([A-Za-z0-9._-]+)[[:space:]]*:.*/\1/p' <<<"$line")"
    [ -n "$k" ] || continue
    case "$seen_keys" in *" $k "*) continue ;; esac   # first (highest-precedence) layer wins per key
    v="$(sed -nE 's/^[[:space:]]*[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p' <<<"$line")"
    [ -n "$v" ] || continue
    seen_keys="$seen_keys$k "
    uk="$(tr '[:lower:]-' '[:upper:]_' <<<"$k")"
    export "DOCKET_RUNNER_CFG_$uk=$v"
  done <<<"$blk"
done

# --- handoff: foreground, adapter owns everything child-specific --------------------
args=( --agent "$AGENT" )
[ -n "$MODEL" ]  && args+=( --model "$MODEL" )
[ -n "$EFFORT" ] && args+=( --effort "$EFFORT" )
exec bash "$ADAPTER" "${args[@]}" -- "$@"
```

- [ ] **Step 4: Run the full test file to verify it passes**

Run: `bash tests/test_runner_dispatch.sh`
Expected: all asserts `ok`, exit 0.

- [ ] **Step 5: Write the contract**

Create `scripts/runner-dispatch.md`: Purpose (runner-neutral facade — the single entry point shims call via `docket.sh runner-dispatch`); Usage (CLI grammar, `--` passthrough); Behavior (validate → adapter-existence registration check → repo-root anchor via `docket_main_worktree` → per-key `runners.<name>:` resolution local>committed>global → `DOCKET_RUNNER_CFG_*` export → exec adapter foreground); Exit codes (1 validation/unknown runner/not-a-repo; otherwise the adapter's); Invariants (never runs a child itself; never degrades to native; `runners:` is NOT coordination-fenced — global-able); the framework rule that adding a runner = adding `scripts/runners/<name>.sh` + `.md` + a registry token in `sync-agents.sh`.

- [ ] **Step 6: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_runner_dispatch.sh
git commit -m "feat(0079): runner-dispatch facade — arg validation, runners.<name> config resolution, adapter handoff"
```

---

### Task 3: Facade wiring — `docket.sh runner-dispatch`

**Files:**
- Modify: `scripts/docket.sh` (WRAPPED_OPS at line ~35; usage block at lines ~10–24)
- Modify: `scripts/docket.md` (operation table)
- Modify: `tests/test_script_contracts_coverage.sh` (extend audit to `scripts/runners/`)
- Test: `tests/test_docket_facade.sh` (existing sentinels; read it before editing)

**Interfaces:**
- Produces: `"$DOCKET_SCRIPTS_DIR"/docket.sh runner-dispatch <args…>` routes to `runner-dispatch.sh` — the exact invocation Task 4's shims emit.

- [ ] **Step 1: Extend the contracts-coverage test (failing first)**

Append to `tests/test_script_contracts_coverage.sh` before the final `exit $fail` (a third + fourth leg mirroring legs 1–2):

```bash
# (3) every scripts/runners/<name>.sh has a co-located scripts/runners/<name>.md (change 0079)
for sh in "$ROOT"/scripts/runners/*.sh; do
  [ -e "$sh" ] || continue
  base="$(basename "$sh" .sh)"
  if [ -f "$ROOT/scripts/runners/$base.md" ]; then ok "contract present for runners/$base.sh"; else no "missing scripts/runners/$base.md for runners/$base.sh"; fi
done

# (4) every scripts/runners/<name>.md has a live scripts/runners/<name>.sh
for md in "$ROOT"/scripts/runners/*.md; do
  [ -e "$md" ] || continue
  base="$(basename "$md" .md)"
  if [ -f "$ROOT/scripts/runners/$base.sh" ]; then ok "script present for runners/$base.md"; else no "orphaned scripts/runners/$base.md (no runners/$base.sh)"; fi
done
```

Run: `bash tests/test_script_contracts_coverage.sh` — expected PASS already (Task 1 shipped both files); mutation-test it: `mv scripts/runners/codex.md /tmp/ && bash tests/test_script_contracts_coverage.sh` must go NOT OK, then restore. Record the mutation check in the commit message.

- [ ] **Step 2: Wire the op**

In `scripts/docket.sh`: add `runner-dispatch [args]` line to the `# Usage:` block (after `render-adr-index`), and append `runner-dispatch` to `WRAPPED_OPS`:

```bash
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index adr-checks board-checks runner-dispatch"
```

In `scripts/docket.md`: add the matching row to the operation table: `runner-dispatch` — delegate one agent run to a child harness via a registered runner adapter (change 0079).

- [ ] **Step 3: Run the facade sentinels**

Run: `bash tests/test_docket_facade.sh`
Expected: PASS. If any sentinel asserts an op *count* or enumerates ops, update it to include `runner-dispatch` — read the failure first; never weaken a sentinel, extend its expected set.

- [ ] **Step 4: Verify routing end-to-end**

Run: `bash scripts/docket.sh runner-dispatch 2>&1; echo rc=$?`
Expected: `runner-dispatch: --runner is required` and `rc=1` (proves the facade routes to the new script).

- [ ] **Step 5: Commit**

```bash
git add scripts/docket.sh scripts/docket.md tests/test_script_contracts_coverage.sh tests/test_docket_facade.sh
git commit -m "feat(0079): docket.sh gains the runner-dispatch op; contracts audit covers scripts/runners/"
```

---

### Task 4: Generation — runner registry + shim wrappers in `sync-agents.sh`

**Files:**
- Modify: `sync-agents.sh`
- Test: `tests/test_sync_agents.sh` (append a `Task 0079` section)

**Interfaces:**
- Consumes: `docket.sh runner-dispatch` invocation shape (Task 3), `resolve_agent_layers` / `emit` / both generation passes / `check_project_level` leg (c) (all existing).
- Produces: generated wrapper files whose body is the shim when `runner:` resolves; `REGISTERED_RUNNERS` token list.

- [ ] **Step 1: Write the failing generation tests**

Append to `tests/test_sync_agents.sh` (before any final exit):

```bash
# ---- change 0079: runner delegation shims -----------------------------------------
# registry <-> adapters parity, BOTH directions (consuming-surface guard, LEARNINGS (d))
REGISTRY_LINE="$(grep -E '^REGISTERED_RUNNERS=' "$SYNC" | head -n1)"
assert "0079: sync-agents declares REGISTERED_RUNNERS" '[ -n "$REGISTRY_LINE" ]'
runners_from_registry="$(sed -E 's/^REGISTERED_RUNNERS="([^"]*)".*/\1/' <<<"$REGISTRY_LINE")"
for r in $runners_from_registry; do
  assert "0079: registry token '$r' has an adapter script" '[ -f "$REPO/scripts/runners/'"$r"'.sh" ]'
done
for a in "$REPO"/scripts/runners/*.sh; do
  [ -e "$a" ] || continue
  tok="$(basename "$a" .sh)"
  assert "0079: adapter '$tok' is in REGISTERED_RUNNERS" 'case " $runners_from_registry " in *" '"$tok"' "*) true;; *) false;; esac'
done

# shim generation: agents.claude.<agent>.runner: codex swaps the BODY for the shim
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, effort: high, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
G="$SBX/.claude/agents/docket-status.md"
assert "0079: shim keeps frontmatter model (bookkeeping)" '[ "$(fm "$G" model)" = "gpt-5.1-codex" ]'
assert "0079: shim body invokes docket.sh runner-dispatch" 'grep -qF "docket.sh runner-dispatch" "$G"'
assert "0079: shim body pins --runner codex" 'grep -qF -- "--runner codex" "$G"'
assert "0079: shim body pins --agent status" 'grep -qF -- "--agent status" "$G"'
assert "0079: shim body bakes the resolved model" 'grep -qF -- "--model gpt-5.1-codex" "$G"'
assert "0079: shim body bakes the resolved effort" 'grep -qF -- "--effort high" "$G"'
assert "0079: shim body demands ONE foreground call" 'grep -qi "one foreground" "$G"'
assert "0079: shim body forbids the inline fallback" 'grep -qi "never.*inline" "$G"'
assert "0079: shim replaced the native body" '! grep -qF "Execute docket-status to refresh docket state" "$G"'
assert "0079: exactly one dispatch invocation in the shim" '[ "$(grep -cF "docket.sh runner-dispatch" "$G")" = "1" ]'
# unlisted agent stays native in the same repo
assert "0079: agent without runner: stays native" 'grep -qF "abort-and-report" "$SBX/.claude/agents/docket-adr.md" && ! grep -qF "runner-dispatch" "$SBX/.claude/agents/docket-adr.md"'
# effort auto + runner => no --effort flag in the shim
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, effort: auto, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0079: effort auto omits --effort from the shim" '! grep -qF -- "--effort" "$G"'
# --check leg (c): a hand-reverted shim is advisory drift (proves leg c shares emission)
printf '%s\n' '---' 'name: docket-status' '---' 'native body' > "$G"
chk="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 )"
assert "0079: --check flags a de-shimmed wrapper as drift" 'grep -qF "drift in .claude/agents/docket-status.md" <<<"$chk"'
rm -rf "$SBX"

# runner under a NON-claude harness key: warned-and-ignored (reserved), file stays native
mkgitrepo
mkdir -p "$SBX/.claude" "$SBX/.cursor"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  cursor:\n    status: { model: gpt-5.5-medium-fast, runner: codex }\n' > "$SBX/.docket.yml"
warn="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"
assert "0079: non-claude runner warns (reserved)" 'grep -qiE "runner.*(cursor|reserved|ignored)" <<<"$warn"'
assert "0079: non-claude wrapper stays native" '! grep -qF "runner-dispatch" "$SBX/.cursor/agents/docket-status.md"'
rm -rf "$SBX"

# unregistered runner under claude: loud generation-time ERROR (nonzero)
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: gemini-cli }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0079: unregistered runner fails generation nonzero" '[ "$rc" != "0" ]'
assert "0079: unregistered-runner error names it" 'grep -qF "gemini-cli" <<<"$err"'
rm -rf "$SBX"

# no runner config anywhere: byte-identical native output (regression fence)
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0079: no-runner repo output stays byte-identical to built-in" 'diff -q "$REPO/agents/docket-status.md" "$SBX/.claude/agents/docket-status.md" >/dev/null'
rm -rf "$SBX"
```

- [ ] **Step 2: Run to verify the new section fails**

Run: `bash tests/test_sync_agents.sh`
Expected: pre-existing asserts `ok`; every `0079:` assert NOT OK.

- [ ] **Step 3: Implement in `sync-agents.sh`** — four additive edits:

(3a) Registry constant, near `VALID_HARNESS_TOKENS` (line ~78):

```bash
# Registered runner names (change 0079) — a runner: value must name one of these; each token
# has a matching scripts/runners/<name>.sh adapter (tests assert parity in both directions).
REGISTERED_RUNNERS="codex"
is_registered_runner(){ case " $REGISTERED_RUNNERS " in *" $1 "*) return 0;; *) return 1;; esac; }
```

(3b) `resolve_agent_layers` grows a third resolved field. Add `RES_RUNNER=""` to the reset line, and inside the layer loop (mirroring the model block):

```bash
    hr="$(field_of "$hline" runner)"; dr="$(field_of "$dline" runner)"
    if [ -z "$RES_RUNNER" ]; then
      if   [ -n "$hr" ]; then RES_RUNNER="$hr"
      elif [ -n "$dr" ]; then RES_RUNNER="$dr"; fi
    fi
```

(declare `hr dr` in the `local` list; the early-`break` condition stays on model+effort — a missing runner is the common case and must not defeat the break; move the `break` to require all three only if simpler: `[ -n "$RES_MODEL" ] && [ -n "$RES_EFFORT" ] && [ -n "$RES_RUNNER" ] && break` is WRONG (runner usually never fills) — instead drop the break entirely or leave it as-is and accept that a lower layer's runner is missed... **Correct resolution:** remove the early `break` — the loop is over ≤3 small files; correctness beats the micro-optimization. Note this in the commit.)

(3c) One emission chokepoint. Add `emit_wrapper` and route BOTH passes and `--check` leg (c) through it (replace each of the three `emit "$src" "$RES_MODEL" "$RES_EFFORT" > …` call sites):

```bash
# Emit either the native wrapper (emit) or, when a runner resolved for the claude
# harness, the runner-delegation shim body under the native frontmatter (change 0079).
# Non-claude harness + runner => warn (reserved) and emit native. Unregistered runner
# under claude => loud generation-time error (explicit config is never silently ignored).
emit_wrapper(){  # $1=src $2=model $3=effort $4=runner $5=harness $6=agent-name  (stdout)
  local runner="$4"
  if [ -z "$runner" ]; then emit "$1" "$2" "$3"; return 0; fi
  if [ "$5" != "claude" ]; then
    log "WARN $5/docket-$6: runner: $runner is reserved for the claude parent — ignored (native dispatch)"
    emit "$1" "$2" "$3"; return 0
  fi
  if ! is_registered_runner "$runner"; then
    log "ERROR docket-$6: runner '$runner' is not a registered runner (registered: $REGISTERED_RUNNERS)"
    exit 1
  fi
  emit_shim "$1" "$2" "$3" "$runner" "$6"
}

# The shim: native frontmatter (model line kept for bookkeeping — the effective pin is
# the baked --model argument), body = one foreground facade call + relay + verify rules.
emit_shim(){  # $1=src $2=model $3=effort $4=runner $5=agent-name  (stdout)
  awk '/^---[[:space:]]*$/{d++} d<2' "$1" | emit /dev/stdin "$2" "$3" 2>/dev/null || {
    # portable fallback: print frontmatter via emit on the full file, truncated at the 2nd fence
    emit "$1" "$2" "$3" | awk '/^---[[:space:]]*$/{d++; print; next} d<2{print}'
  }
  local flags="--runner $4 --agent $5"
  [ -n "$2" ] || flags="$flags"   # model may legitimately be empty (inherit child default)
  [ -n "$2" ] && flags="$flags --model $2"
  [ -n "$3" ] && [ "$3" != "auto" ] && flags="$flags --effort $3"
  cat <<SHIM
This agent is DELEGATED to the \`$4\` runner (cross-harness runner delegation, change 0079).
Do NOT execute the skill inline and do NOT load its skills yourself.

Make exactly ONE foreground Bash call, with the maximum timeout (600000):

    "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch $flags [-- <caller args>]

appending any caller-supplied task arguments after \`--\`. Block until it completes — never
background it, never poll. Then relay its stdout (the child's final message) as your result,
and verify the child's contract exactly as a native caller would: git state on origin/docket
for state-contract agents (status, adr); the relayed report for in-context-report agents.
If the dispatch exits non-zero, abort-and-report its stderr diagnostic — never retry
silently, never fall back to running the skill inline on this harness.
SHIM
}
```

**Frontmatter emission note for the implementer:** the goal of the first lines of `emit_shim` is “the same frontmatter `emit` would produce, without the native body.” The simplest portable form is:

```bash
emit "$1" "$2" "$3" | awk '/^---[[:space:]]*$/{d++; print; next} d<=1{print}'
```

(print everything through the second `---` fence, drop the body). Use that single-pipe form — discard the twin-awk sketch above if it fights you; the test asserting “shim replaced the native body” plus the frontmatter `fm` asserts define done.

The three call sites become:

```bash
emit_wrapper "$src" "$RES_MODEL" "$RES_EFFORT" "$RES_RUNNER" "$harness" "$name" > "$dir/$(basename "$src")"   # user_level_pass
emit_wrapper "$src" "$RES_MODEL" "$RES_EFFORT" "$RES_RUNNER" "$harness" "$name" > "$dir/docket-$name.md"      # project_level_pass
emit_wrapper "$src" "$RES_MODEL" "$RES_EFFORT" "$RES_RUNNER" "$harness" "$name" > "$tmp/docket-$name.md"      # check_project_level leg (c)
```

(3d) Nothing else changes — gitignore block, pruning, dispatch rule, migrations untouched.

- [ ] **Step 4: Run the suite section to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: all asserts `ok` (pre-existing AND `0079:`), exit 0.

- [ ] **Step 5: Mutation-test the new guards**

Minimum set (each must redden exactly the named assert, then restore):
1. In `emit_shim`, drop the `--runner $4` words → “pins --runner codex” reddens.
2. Make `emit_wrapper` fall back to `emit` on unregistered runner instead of `exit 1` → “fails generation nonzero” reddens.
3. Remove the non-claude warn + shim anyway → “non-claude wrapper stays native” reddens.
4. Point leg (c) back at bare `emit` → “--check flags a de-shimmed wrapper” reddens.

- [ ] **Step 6: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0079): runner registry + shim wrapper emission in sync-agents.sh (mutation-tested)"
```

---

### Task 5: Documentation

**Files:**
- Modify: `README.md` (three sites: sample `.docket.yml` at ~line 186; `## Tuning agent models & effort` at ~line 361; `## Customization` at ~line 444)
- Modify: `skills/docket-convention/references/agent-layer.md` (the `Harness-first agents: blocks` sample + a short `runner:` paragraph)

(Anchor to the *headings/shapes* named below, never line numbers — siblings move them.)

- [ ] **Step 1: README sample `.docket.yml`**

In the commented sample block (the one containing `# agents:`), extend the `agents:` comment and add `runners:` beneath it:

```
# agents:                    # per-skill subagent model/effort — and runner: to delegate an agent's
#                            # whole run to another harness (see "Runner delegation" below)
# runners:                   # per-runner knobs for runner delegation (e.g. runners.codex.sandbox)
```

- [ ] **Step 2: README `## Tuning agent models & effort` pointer**

Append a short paragraph at the end of that section:

```markdown
The same `agents:` entries can also carry a `runner:` key, which delegates that agent's whole
run to a *different* harness (e.g. OpenAI Codex) with its own subscription and models — see
[Runner delegation](#runner-delegation--running-docket-agents-on-another-harness) under
Customization.
```

- [ ] **Step 3: README `### Runner delegation` subsection under `## Customization`**

Insert after the `### Consultant-authored brainstorm (opt-in)` subsection:

```markdown
### Runner delegation — running docket agents on another harness

Docket agents normally run on the harness hosting your session. **Runner delegation** hands an
agent's *whole run* to a child harness with its own subscription, models, and skills — activated
per agent by an explicit `runner:` key, never inferred from model IDs. One pair ships today:
parent `claude` (Claude Code) → child `codex` (OpenAI Codex CLI).

```yaml
# .docket.yml (or the global ~/.config/docket/config.yml — runner is a machine preference)
agents:
  claude:                       # the PARENT harness: when Claude Code hosts the session…
    status: { model: gpt-5.1-codex, effort: medium, runner: codex }   # …run docket-status on Codex
runners:
  codex:
    sandbox: workspace-write    # workspace-write (default) | danger-full-access
    network: true               # default true — git push and gh need it
```

How it works: `sync-agents.sh` generates that agent's wrapper with a **shim body** — one
foreground call to `docket.sh runner-dispatch`, which resolves the `runners.codex` knobs and
runs `codex exec` (blocking, sandboxed, `--output-last-message` relay). Every invocation path
(skill fork, `@docket-status`, composition from another skill) inherits the delegation
unchanged. `model:` is passed to the child verbatim (ADR-0015); `effort:` maps to Codex's
`model_reasoning_effort` (docket's `max` → codex `xhigh`).

Rules and limits:

- **Only autonomous wrappers are delegatable** (the nine generated agents). Interactive skills
  stay inline — an exec primitive has no human channel.
- A delegated *orchestrator*'s own sub-dispatches run child-natively (for Codex:
  `spawn_agent`, via superpowers' Codex support). Per-agent model pins do **not** carry into
  those child-side dispatches (accepted limitation).
- `runner:` under a non-`claude` harness key is reserved and warned-and-ignored; an
  unregistered runner name fails generation loudly.
- Delegation is never a policy bypass: do not delegate `docket-finalize-change` to sidestep
  merge-approval gates (see change 0062).

**Prerequisites (codex):** Codex CLI installed and authenticated (`codex login`); superpowers
installed in Codex; docket skills linked (`link-skills.sh`, automatic on install); and
`[features] multi_agent = true` in `~/.codex/config.toml` if you delegate an orchestrator
(SDD fan-out) rather than a leaf agent.
```

- [ ] **Step 4: agent-layer.md**

In the `Harness-first agents: blocks` YAML sample, add one line to the `cursor:`-style example under a `claude:` key — extend the sample with:

```yaml
  claude:                               # per-harness override; runner: delegates the whole run
    status: { model: gpt-5.1-codex, runner: codex }   # …to a child harness (change 0079)
```

And append a paragraph after the resolution rules:

```markdown
**`runner:` — cross-harness delegation (change 0079).** An agent entry may carry `runner: <name>`
naming a registered runner (shipped: `codex`); the generated wrapper body then becomes a shim that
makes one foreground `docket.sh runner-dispatch` call, delegating the whole run to that child
harness. `runner` resolves per-field through the same four layers and is global-able (a machine
preference, like `model`/`effort` — it writes no shared state). It is honored under the `claude`
harness key (or `default:` when generating claude's files); under any other harness key it is
reserved and warned-and-ignored. An unregistered name is a loud generation-time error. Per-runner
knobs live in a top-level `runners.<name>:` block (any layer); the codex knobs and prerequisites
are in `scripts/runners/codex.md`, and the user-facing walkthrough is README's *Runner delegation*.
```

- [ ] **Step 5: Run the docs-adjacent sentinels**

Run: `bash tests/test_sync_agents.sh && bash tests/test_cursor_permissions_docs.sh && bash tests/test_convention_extraction.sh`
Expected: PASS (these sentinels grep README/agent-layer prose; if one reddens, read whether the new prose legitimately violated it — extend the expected set, never delete the guard).

- [ ] **Step 6: Commit**

```bash
git add README.md skills/docket-convention/references/agent-layer.md
git commit -m "docs(0079): runner delegation — README subsection + sample keys + agent-layer reference"
```

---

### Task 6: Whole-suite run + live Codex smoke dispatch

**Files:** none new (results evidence only — recorded in the change's results file at close-out).

- [ ] **Step 1: Run the ENTIRE test suite** (LEARNINGS: never only the enumerated tests)

Run (single foreground call, timeout 600000):

```bash
fails=0; for t in tests/test_*.sh; do out="$(bash "$t" 2>&1)" || { fails=$((fails+1)); printf '=== FAIL %s ===\n%s\n' "$t" "$out"; }; done; echo "suites failed: $fails"
```

Expected: `suites failed: 0`. Any red suite: re-run the identical suite against unmodified `origin/main` before calling it a regression (LEARNINGS environment family).

- [ ] **Step 2: Live smoke — one real `codex exec` dispatch of the cheapest agent**

Run (foreground, timeout 600000):

```bash
bash scripts/docket.sh runner-dispatch --runner codex --agent status -- "Run the board-only pass (--board-only) and report the digest lines."
```

Expected: exit 0; stdout carries Codex's final message (a backlog summary). Verify the child actually acted: the message references docket state (digest/board lines), and `git -C .docket status` shows no wedged state. This exercises: preflight (real auth), skill discovery in `~/.codex/skills`, sandbox+network flags, final-message relay. If the Codex-side skill discovery fails, capture the exact failure text — that is a spec §4 verification finding, not necessarily a build bug; investigate the prompt's skill-invocation phrasing before touching code.

- [ ] **Step 3: Record smoke evidence**

Keep the smoke command + trimmed output for the results file (step 6.5 of the docket build loop): live pair verified claude→codex, which flags were exercised, and the multi_agent-not-enabled note (leaf agent unaffected).

---

## Notes for the build loop (not tasks)

- **Two ADRs at step 6** (per spec §7): (a) the runner-delegation framework — explicit `runner:` switch, parent/child seams, whole-run semantics, ADR-0015 relationship; (b) the shim-wrapper mechanism — all invocation paths inherit delegation through the generated wrapper.
- **Concurrent change 0077** is reshaping `sync-agents.sh` (per-harness emitter registry) on its own branch. Keep this change's `sync-agents.sh` edits additive (new functions + three call-site swaps); whichever merges second resolves hunks by intent (a same-file change that merged after divergence supersedes).
- **`runners:` / `runner:` are NOT coordination-fenced** — do not add them to the fenced-key table in `scripts/docket-config.md`; no `docket-config.sh` changes are needed (the facade resolves `runners.<name>:` itself; `runner:` is read by `sync-agents.sh`'s own agents-block parser).

## Self-review (done at authoring)

- Spec §1 config surface → Tasks 2 (runners block), 4 (runner key). §2 generation → Task 4. §3 facade+adapter → Tasks 1–3. §4 skill availability → Task 6 step 2 (verify) + Task 1 contract (prerequisites). §5 eligibility/composition → shim text (verify-as-native-caller) + README rules. §6 docs → Task 5. §7 failure posture/testing → every task's failure asserts + Task 6.
- No placeholders; every code step carries the code. Type/name consistency: `DOCKET_RUNNER_CFG_*`, `DOCKET_REPO_ROOT`, `REGISTERED_RUNNERS`, `emit_wrapper`, adapter/facade CLIs match across tasks.
- Known judgment point flagged inline: `emit_shim`'s frontmatter extraction (Task 4 step 3c) states the goal + fallback one-liner so the implementer isn't wedded to a brittle sketch.
