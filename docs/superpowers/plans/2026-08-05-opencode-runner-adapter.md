<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0205 — opencode runner adapter — delegate build workers to OpenRouter models](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-05-0205-opencode-runner-adapter.md)**
<!-- docket:backlink:end -->

# opencode runner adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `scripts/runners/opencode.sh` so a Claude Code session can delegate individual docket agents (the four build profile workers, in the motivating case) to opencode running OpenRouter models, and make a model-less delegation a loud generation-time error on every runner.

**Architecture:** Three seams, already defined by change 0079's framework. (1) A new per-runner adapter script + contract under `scripts/runners/`, a ~100-line sibling of `codex.sh`/`cursor.sh`, plus its token in `sync-agents.sh`'s `REGISTERED_RUNNERS`. The `runner-dispatch.sh` facade is **not touched** — it already resolves an arbitrary `runners.<name>:` block per-key across the config layers and exports each key as `DOCKET_RUNNER_CFG_<KEY>`, so the new `permissions` knob needs no resolver plumbing. (2) A runner-wide guard in `sync-agents.sh`'s `emit_wrapper`, immediately after the existing change-0168 provenance gates, turning an empty user-model into `exit 1`. (3) The config/doc surface: `.docket.example.yml`, `README.md`, `docs/opencode/setup.md`, and the example-yml guard's key tables.

**Tech Stack:** POSIX-ish bash (the repo's `set -uo pipefail` house style), the existing `tests/test_*.sh` assert-harness convention, opencode CLI 1.18.11.

## Global Constraints

Every task's requirements implicitly include these. They are house rules from `AGENTS.md` and this repo's learnings ledger, plus values verified live during planning.

- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) under `set -o pipefail` — capture into a variable first, then `grep <<<"$var"`.
- **`grep` for a pattern leading with `--` must declare it**: `grep -qF -- "<pat>"` or `grep -E -e "<pat>"`. A bare leading `--` exits 2, and inside a negated assert (`! grep …`) that error inverts into a permanently green, vacuous guard.
- **awk indent classes are `[^[:space:]]`**, never `[^ ]`.
- **A guard is code: mutation-test it.** Strip the thing it guards, watch it redden, and **confirm the mutation actually landed** with a `grep -c` before and after — an in-place substitution that silently fails to match yields a green run with nothing mutated, which reads exactly like a robust guard.
- **Write the assert that DETECTS the state you removed**, not one that confirms the wording you introduced.
- **An absence assert needs a non-vacuity companion through the SAME extractor** — a bare `[ -z "$x" ]` is green when the extractor breaks.
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number** (`tests/test_comment_anchor_style.sh` enforces the filename-plus-line form).
- **`grep` on PATH is ugrep and is more permissive than BSD `grep`.** Any new regex in shipped code must also be checked under `/usr/bin/grep`.
- **Run the whole suite at the build gate**, never only the tests this plan enumerates. There is no GitHub Actions CI in this repo — the suite is the de-facto gate. Run every `tests/test_*.sh`.
- **No vendor allowlist (ADR-0015).** Model IDs and effort tokens pass through verbatim; docket never validates them.

**Verified live during planning against opencode 1.18.11** (`opencode run --help`) — use these exact spellings:

| Purpose | Flag | Notes |
|---|---|---|
| model | `-m, --model` | "model to use in the format of provider/model" |
| reasoning effort | `--variant` | "model variant (provider-specific reasoning effort, e.g., high, max, minimal)" — takes docket's `max` natively, so **no mapping table** (unlike codex, where `max` → `xhigh`) |
| working directory | `--dir` | the analogue of codex's `-C` |
| permission posture | `--auto` | "auto-approve permissions that are not explicitly denied (dangerous!)", boolean, default false |
| output shape | `--format` | `default` (formatted) \| `json` (raw JSON events) |

**Trap:** opencode's `-p` is `--password`, **not** print/non-interactive. Do not copy cursor.sh's `-p`. The prompt is a **positional** `message` argument.

---

### Task 1: The opencode runner adapter

**Files:**
- Create: `scripts/runners/opencode.sh`
- Create: `scripts/runners/opencode.md`
- Modify: `sync-agents.sh` — the `REGISTERED_RUNNERS` assignment
- Modify: `tests/test_sync_agents.sh` — add the reverse leg of the registry-parity guard (Step 6)
- Test: `tests/test_runner_opencode.sh` (create)

**Interfaces:**
- Consumes: `DOCKET_REPO_ROOT` (absolute, exported by `runner-dispatch.sh`), `DOCKET_RUNNER_CFG_PERMISSIONS` (from the `runners.opencode:` block, resolved by the facade's existing per-key loop — no facade change), and the adapter CLI `--agent <name> [--model <m>] [--effort <e>] [-- <args…>]` shared by every adapter.
- Produces: the `opencode` token in `REGISTERED_RUNNERS`, which `sync-agents.sh`'s `is_registered_runner` gates `runner: opencode` on; and the mock seam `OPENCODE_BIN` (default `opencode`), matching `CODEX_BIN` / `CURSOR_BIN`.

**Design decisions locked here** (do not re-litigate mid-task):

1. **Preflight is binary-only** — `command -v "$OPENCODE_BIN"`, matching `cursor.sh`. It does **not** probe auth. `codex.sh` can use `codex login status` because that subcommand's contract is a status check; opencode's nearest equivalent is `opencode auth list`, whose exit code on a machine with **zero** credentials could not be established during planning without destroying the developer's real credentials. Shipping a probe whose failure semantics are unverified would convert an authenticated-but-unusual setup into a hard abort. Auth is a **documented prerequisite** in the contract instead, and "confirm `opencode auth list` exit semantics with no credentials" is a named human verification item.
2. **`permissions` is a three-way gate, not a boolean.** `auto-approve` → append `--auto`. `ask` (and unset, which defaults to `ask`) → **die before any child process is invoked**, with a diagnostic naming the knob. Any other value → die as an unknown value. That last leg mirrors `codex.sh`'s posture on a non-boolean `network` ("explicit config is never silently ignored"); silently treating an unknown value as `ask` would make a typo look like a deliberate refusal.
3. **`inherit` is normalized to empty before mapping**, exactly as `cursor.sh` and `emit_opencode_md` do. `inherit` is docket's own no-pin sentinel, never a vendor ID; handing the literal string to `--model` would let opencode's own resolution silently substitute something.
4. **Effort with no model is dropped with a WARN**, matching `cursor.sh` and the opencode emitter. Task 2's required-model rule makes this near-unreachable through the shim, but the adapter is independently invocable and stays defensive.
5. **Relay is opencode's default formatted stdout, verbatim**, with the child's exit code propagated — the `cursor.sh` posture. opencode has no `--output-last-message` analogue, so the only alternative is `--format json`, which would require the adapter to parse an unversioned event schema; a wrong parse silently truncates the relay, which is strictly worse than decoration. The `json` alternative is recorded in the contract as the documented escape hatch, and relay legibility is a named human verification item.

- [ ] **Step 1: Write the failing test**

Create `tests/test_runner_opencode.sh`. It mirrors `tests/test_runner_cursor.sh`'s harness exactly (same `assert`, same mock-recording shape) so a reader of one can read the other.

```bash
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
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "adapter exists" '[ -f "$ADAPTER" ]'
assert "contract doc exists (test_script_contracts_coverage parity)" '[ -f "$REPO/scripts/runners/opencode.md" ]'
assert "registered in sync-agents REGISTERED_RUNNERS" 'grep -qE "^REGISTERED_RUNNERS=\"[^\"]*\bopencode\b" "$REPO/sync-agents.sh"'

# A mock opencode that records its argv + counts invocations and prints a final message.
MOCK_DIR="$(mktemp -d)"
cat > "$MOCK_DIR/opencode" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$MOCK_ARGV"
printf 'CALL\n' >> "${MOCK_CALLS:-/dev/null}"
printf 'MOCK-FINAL-MESSAGE\n'
exit "${MOCK_RC:-0}"
MOCK
chmod +x "$MOCK_DIR/opencode"

run_adapter(){  # $@ = adapter args ; sets OUT / RC / ARGV / CALLS
  MOCK_ARGV="$MOCK_DIR/argv.txt"
  MOCK_CALLS="$MOCK_DIR/calls.txt"
  : > "$MOCK_CALLS"; : > "$MOCK_ARGV"
  OUT="$( MOCK_ARGV="$MOCK_ARGV" MOCK_CALLS="$MOCK_CALLS" MOCK_RC="${MOCK_RC:-0}" \
          OPENCODE_BIN="$MOCK_DIR/opencode" \
          DOCKET_RUNNER_CFG_PERMISSIONS="${PERM:-auto-approve}" \
          DOCKET_REPO_ROOT="$REPO" bash "$ADAPTER" "$@" 2>/dev/null )"
  RC=$?
  ARGV="$(cat "$MOCK_ARGV" 2>/dev/null)"
  CALLS="$(grep -c CALL "$MOCK_CALLS" 2>/dev/null)"
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

# --- docket's `max` passes through unmapped (unlike codex, which maps max -> xhigh) ---------------
run_adapter --agent status --model openrouter/moonshotai/kimi-k3 --effort max
assert "max effort: passed to --variant unmapped" 'grep -qxF -- "max" <<<"$ARGV"'
assert "max effort: never rewritten to codex's xhigh" '! grep -qxF -- "xhigh" <<<"$ARGV"'

# --- passthrough args land in the prompt ---------------------------------------------------------
run_adapter --agent status --model m/x/y -- please-do-0205
assert "passthrough: -- args reach the prompt" 'grep -qF "please-do-0205" <<<"$ARGV"'

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

rm -rf "$MOCK_DIR"
echo "---"; [ "$fail" = "0" ] && echo "ALL PASS" || echo "FAILURES"; exit $fail
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_runner_opencode.sh`
Expected: FAIL — `NOT OK - adapter exists`, `NOT OK - contract doc exists`, `NOT OK - registered in sync-agents REGISTERED_RUNNERS`, and every behavioral assert failing because `$ADAPTER` does not exist. Confirm the last line is `FAILURES` and the exit code is nonzero.

- [ ] **Step 3: Write the adapter**

Create `scripts/runners/opencode.sh`:

```bash
#!/usr/bin/env bash
# scripts/runners/opencode.sh — the opencode runner adapter (change 0205). Owns everything
# child-specific for delegating a whole agent run to opencode via `opencode run`: preflight
# (binary), prompt assembly from the built-in wrapper source, flag mapping (model verbatim per
# ADR-0015; effort -> --variant; repo root -> --dir; permission posture -> --auto), foreground
# execution, stdout relay. Invoked by runner-dispatch.sh — not directly by skills.
# Contract: scripts/runners/opencode.md. Mock seam: OPENCODE_BIN. Env in (from the facade):
# DOCKET_REPO_ROOT (absolute, required), DOCKET_RUNNER_CFG_PERMISSIONS (default `ask`).
#
# WHY `ask` REFUSES: opencode prompts for approval before editing a file or running a command. A
# delegated run has no human channel to answer with, so without --auto it blocks on the first
# prompt until something times out. Refusing up front turns a silent hang into a diagnostic. The
# opposite posture — defaulting to --auto — would hand blanket auto-approval to anyone who merely
# typed `runner: opencode`, so the grant is an explicit, visible line in config instead.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SELF_DIR/../../agents"
OPENCODE_BIN="${OPENCODE_BIN:-opencode}"

die(){ printf 'runners/opencode: %s\n' "$*" >&2; exit 1; }
warn(){ printf 'runners/opencode: %s\n' "$*" >&2; }

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

# --- permission posture: resolved BEFORE preflight so the refusal never depends on the binary ---
# An enum, not a boolean, structurally parallel to runners.codex.sandbox — it leaves room for a
# future deny-list value without a boolean->enum migration. An unrecognized value DIES rather than
# falling back to `ask`: a typo must not be indistinguishable from a deliberate refusal.
PERMISSIONS="${DOCKET_RUNNER_CFG_PERMISSIONS:-ask}"
case "$PERMISSIONS" in
  auto-approve) ;;
  ask) die "runners.opencode.permissions is 'ask' (the default) — a delegated run cannot answer opencode's approval prompts and would hang. Set 'runners.opencode.permissions: auto-approve' to approve everything not explicitly denied by your own opencode deny rules, or drop 'runner: opencode' from this agent." ;;
  *)   die "runners.opencode.permissions must be 'ask' or 'auto-approve' (got '$PERMISSIONS')" ;;
esac

# --- preflight: binary (abort-and-report; never degrade to a native run) --------
# Auth is NOT probed. `opencode auth list`'s exit code on a machine with zero credentials is
# unverified, and a probe whose failure semantics are unknown would convert an unusual-but-working
# setup into a hard abort. Authentication is a documented prerequisite — see opencode.md.
command -v "$OPENCODE_BIN" >/dev/null 2>&1 || die "opencode CLI not on PATH — install opencode (https://opencode.ai) or unset runner: opencode"

# --- prompt assembly: skills to load + the wrapper body + passthrough args -------
# skills: [a, b] frontmatter line -> "a b" (sed emits at most one line per file shape;
# first-line capture kept variable-side to stay pipefail-safe — LEARNINGS)
skills_line="$(sed -n 's/^skills:[[:space:]]*\[\(.*\)\].*/\1/p' "$SRC")"
skills_line="$(head -n1 <<<"$skills_line" | tr ',' ' ')"
# collapse whitespace + trim WITHOUT word-splitting/globbing (a bare `echo $x` would glob-expand)
skills_line="$(printf '%s' "$skills_line" | tr -s '[:space:]' ' ' | sed 's/^ *//; s/ *$//')"
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
# ADR-0015: the model ID is passed VERBATIM and never validated. Unlike codex — whose vocabulary
# tops out at xhigh, forcing a max->xhigh mapping — opencode's --variant takes docket's `max`
# natively, so the effort vocabulary passes straight through with NO mapping table.
# `inherit` is DOCKET'S OWN "no pin" sentinel (never a vendor model ID), so it is normalized to
# empty here — the same normalization emit_opencode_md performs — BEFORE the mapping below.
# Without this, `--model inherit` would reach opencode as a literal provider/model string.
if [ "$MODEL" = "inherit" ]; then MODEL=""; fi
if [ -z "$MODEL" ] && [ -n "$EFFORT" ] && [ "$EFFORT" != "auto" ]; then
  warn "WARN effort '$EFFORT' dropped — --variant is a provider-specific model option and no model is resolved. Set an explicit model to pin effort on opencode."
  EFFORT=""
fi

cmd=( "$OPENCODE_BIN" run --dir "$DOCKET_REPO_ROOT" )
[ -n "$MODEL" ] && cmd+=( --model "$MODEL" )
[ -n "$EFFORT" ] && [ "$EFFORT" != "auto" ] && cmd+=( --variant "$EFFORT" )
[ "$PERMISSIONS" = "auto-approve" ] && cmd+=( --auto )
cmd+=( "$prompt" )

# --- foreground execution + relay ---------------------------------------------------
# opencode has no --output-last-message analogue, so the child's default formatted stdout IS the
# relay, verbatim — the cursor.sh posture. The alternative, --format json, would require parsing an
# unversioned event schema here, where a wrong parse silently TRUNCATES the relay; decoration in a
# faithful relay is the smaller failure. Any nonzero exit propagates as-is — abort-and-report,
# never a retry or a degrade.
"${cmd[@]}"
rc=$?
if [ "$rc" != "0" ]; then
  printf 'runners/opencode: opencode run exited %s\n' "$rc" >&2
  exit "$rc"
fi
exit 0
```

- [ ] **Step 4: Register the runner**

Modify `sync-agents.sh` — the `REGISTERED_RUNNERS` assignment:

```bash
REGISTERED_RUNNERS="codex cursor opencode"
```

- [ ] **Step 5: Write the contract**

Create `scripts/runners/opencode.md`:

````markdown
# runners/opencode.sh — the opencode runner adapter

## Purpose

The third per-runner adapter of the cross-harness runner delegation framework (change 0079):
delegates one docket agent's **whole run** to opencode via its non-interactive `opencode run`.
Owns everything child-specific — permission gating, preflight, prompt assembly, flag mapping,
foreground execution, relay. Invoked only by `runner-dispatch.sh` (behind
`docket.sh runner-dispatch`), never directly by skills or shims.

The motivating use is cost asymmetry: opencode reaches OpenRouter models, so docket's four
build profile workers can be delegated to cheap models while the review rungs stay native on the
parent's own subscription. Because build and review are already separate wrappered agents
(ADR-0063), that split needs no new mechanism — just `runner:` on the rows you want to leave.

## Usage

```
bash scripts/runners/opencode.sh --agent <name> [--model <m>] [--effort <e>] [--] [<args…>]
```

- `--agent <name>` (required) — the built-in agent to delegate; its wrapper source
  `agents/docket-<name>.md` supplies the skills list and body for the prompt. That source is
  behavior-only — model and effort arrive as the flags below, resolved by the caller from the
  user's config layers. A **shipped** default is never forwarded: only a user-configured value
  becomes a flag, and since change 0205 a `runner:`-bearing agent with no user-configured model
  is a generation-time error rather than a model-less dispatch.
- `--model <m>` (optional here, required in practice by the generation-time rule) — passed to
  `opencode run --model` **verbatim** (ADR-0015 opaque passthrough). OpenRouter IDs are
  double-prefixed (`openrouter/<vendor>/<model>`); opencode splits that itself. Docket's own
  `inherit` no-pin sentinel is normalized to "no flag" and never reaches opencode.
- `--effort <e>` (optional) — mapped to `opencode run --variant`, opencode's provider-specific
  reasoning-effort knob. Values pass through **verbatim, with no mapping table**: `--variant`
  accepts docket's `max` natively, unlike codex where `max` becomes `xhigh`. `auto` and an unset
  value both emit no flag (the provider's own default applies). With no model resolved the effort
  has nothing to attach to and is dropped with a WARN.
- `-- <args…>` — appended to the prompt as caller task context.

Environment (set by the facade):

| Var | Meaning | Default |
|---|---|---|
| `DOCKET_REPO_ROOT` | absolute main-worktree path; becomes `opencode run --dir` | required |
| `DOCKET_RUNNER_CFG_PERMISSIONS` | `runners.opencode.permissions` — `ask` \| `auto-approve` | `ask` |

Mock seam: `OPENCODE_BIN` (default `opencode`).

## The `permissions` knob

opencode has no sandbox *levels*. Where codex takes `--sandbox workspace-write |
danger-full-access`, opencode has a permission system that prompts for approval before editing a
file or running a shell command; `--auto` auto-approves everything **not explicitly denied** in
opencode's own config, and its own help text marks it `(dangerous!)`.

- **`ask`** (the default) — names what actually happens with no flag. A delegated run under `ask`
  **fails at adapter preflight, before any child process is invoked**, because a delegated run has
  no human channel and would otherwise block on the first approval until something times out. The
  default value describes reality rather than serving as a placeholder.
- **`auto-approve`** — bakes `--auto`. Self-describing at the config site; a reader needs no
  knowledge of opencode's CLI. Pair it with opencode's own deny rules — `--auto` approves what is
  not explicitly denied, so the deny list is the real boundary.
- Any other value is a loud refusal, not a silent fall-back to `ask`: explicit config is never
  silently ignored, and a typo must not be indistinguishable from a deliberate refusal.

An enum rather than a boolean, structurally parallel to `runners.codex.sandbox`, leaving room for
a future `deny-list` value without a boolean→enum migration.

## Behavior

1. **Permission gate** — resolve `permissions`; refuse on `ask` (default) or an unknown value.
   Evaluated **before** preflight so the refusal is identical with or without the binary present.
2. **Preflight** — `opencode` (or `$OPENCODE_BIN`) resolvable on PATH. Failure is a loud
   abort-and-report — **never** a silent degrade to a native run, because `runner:` was explicit
   human config. Authentication is **not** probed; see *Prerequisites*.
3. **Prompt assembly** — from `agents/docket-<agent>.md`: "invoke skill `<s>`" for each entry of
   the wrapper's `skills:` frontmatter list (docket skills are linked into `~/.agents/skills` by
   `link-skills.sh`, which opencode reads), then the wrapper body verbatim (which carries the
   abort-and-report rule), then any passthrough args. The assembled prompt is opencode's
   **positional** `message` argument.
4. **Flag mapping** — `run --dir $DOCKET_REPO_ROOT`, `--model <model>` and `--variant <effort>`
   when supplied, `--auto` when `permissions` is `auto-approve`.
5. **Execution + relay** — runs `opencode run` **foreground**, blocking until exit; the child's
   stdout is the adapter's stdout, verbatim.

## Exit codes

- `0` — child ran and exited 0; stdout carries its output.
- `1` — precondition abort (bad args, missing agent source, missing binary, missing
  `DOCKET_REPO_ROOT`, or a `permissions` value that refuses).
- any other — the child's own nonzero exit, propagated.

## Invariants

- The `ask` refusal happens **before** any child process is invoked — never after.
- Model IDs and effort tokens are never validated or rewritten (ADR-0015); `max` is **not**
  remapped.
- Exactly one `opencode run` invocation per adapter run; always foreground, never backgrounded.
- Never degrades to running the agent natively.

## Relay shape — why default, not `--format json`

`opencode run` offers `--format default` (formatted) and `--format json` (raw JSON events). It has
no `--output-last-message` analogue, so there is no flag that yields the final message alone.
This adapter relays the **default** formatted stdout verbatim, matching `runners/cursor.sh`.
Parsing `--format json` would bind docket to an unversioned event schema where a wrong or drifted
parse silently **truncates** the relay; decoration inside a faithful relay is the smaller failure,
and it is visible rather than silent. If real-world output proves unusable, `--format json` plus a
documented extractor is the recorded escape hatch — a deliberate, reversible follow-up.

## Prerequisites (documented, not automated)

- opencode installed (`opencode` on PATH); verified against **1.18.11** (`run --model`,
  `--variant`, `--dir`, `--auto`).
- A provider authenticated — `opencode auth login` (alias `opencode providers`). OpenRouter for
  the double-prefixed IDs in the recipe. **Not probed by the adapter:** `opencode auth list`'s
  exit code on a machine with zero credentials is unverified, and a probe with unknown failure
  semantics would convert an unusual-but-working setup into a hard abort.
- Docket skills linked into `~/.agents/skills` (`link-skills.sh`, automatic on install).
- `runners.opencode.permissions: auto-approve` in a config layer — without it every delegated run
  refuses by design.
````

- [ ] **Step 6: Run the test to verify it passes**

Run: `bash tests/test_runner_opencode.sh`
Expected: PASS — final line `ALL PASS`, exit 0.

Then confirm the registry-parity assertion in the existing suite picked up the new adapter automatically (it derives its population from the `scripts/runners/*.sh` glob, so it must now cover three runners):

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -c "0079: adapter"`
Expected: `3`

**That guard runs one direction only** — it walks the adapter files and checks each is in the registry, so a registry token with no adapter file is unguarded. The spec calls for parity "in both directions", and a one-way correspondence guard is exactly the shape that misses its own target case. Add the reverse leg immediately after the existing loop in `tests/test_sync_agents.sh` (the one whose body asserts `0079: adapter '$tok' is in REGISTERED_RUNNERS`):

```bash
# Reverse direction (change 0205): every REGISTERED_RUNNERS token has a live adapter file. The
# forward loop above walks the FILES, so it is structurally incapable of catching a token whose
# adapter was renamed or deleted — sync-agents.sh would then accept `runner: <token>` at
# generation time and the failure would surface only at dispatch, inside the child handoff.
n_registry_tokens=0
for tok in $runners_from_registry; do
  assert "0205: registry token '$tok' has a live adapter at scripts/runners/$tok.sh" \
    '[ -f "$REPO/scripts/runners/'"$tok"'.sh" ]'
  n_registry_tokens=$((n_registry_tokens+1))
done
# NON-VACUITY: an extractor that returned the empty string would run the loop zero times and leave
# every assert above unexecuted, which reads exactly like parity holding.
assert "0205: the reverse loop actually enumerated the registry (got $n_registry_tokens)" \
  '[ "$n_registry_tokens" -ge 3 ]'
```

Note this reuses `$runners_from_registry` and `$REPO`, both already in scope at that point in the file. Confirm the new leg is non-vacuous by mutation:

```bash
cp sync-agents.sh /tmp/sync-agents.sh.bak
perl -0pi -e 's/^REGISTERED_RUNNERS="codex cursor opencode"$/REGISTERED_RUNNERS="codex cursor opencode ghostwriter"/m' sync-agents.sh
grep -c 'ghostwriter' sync-agents.sh                               # MUST be 1
bash tests/test_sync_agents.sh 2>&1 | grep -F "NOT OK - 0205: registry token 'ghostwriter'"   # MUST print
cp /tmp/sync-agents.sh.bak sync-agents.sh; rm -f /tmp/sync-agents.sh.bak
```

And the contract-coverage audit:

Run: `bash tests/test_script_contracts_coverage.sh`
Expected: exit 0, including `ok   - contract present for runners/opencode.sh`.

- [ ] **Step 7: Mutation-test the load-bearing guards**

Two asserts in this file are negative and would pass vacuously if miswired. Prove each goes red, and **confirm each mutation actually landed** with a `grep -c` before and after.

Mutation A — the `ask` refusal must abort *before* the child runs. Move the permission gate to after the `cmd` invocation:

```bash
cd /Users/homer/dev/docket/.worktrees/opencode-runner-adapter
cp scripts/runners/opencode.sh /tmp/opencode.sh.bak
grep -c 'a delegated run cannot answer' scripts/runners/opencode.sh   # expect 1
# Neuter the refusal: make `ask` fall through to auto-approve instead of dying. Replace the WHOLE
# line rather than splicing inside the diagnostic string — a partial splice leaves unbalanced
# quotes, and a syntax error reddens the suite for a reason that is not the guard.
perl -0pi -e 's/^  ask\) die .*$/  ask) PERMISSIONS=auto-approve ;;/m' scripts/runners/opencode.sh
grep -c 'ask) PERMISSIONS=auto-approve ;;' scripts/runners/opencode.sh   # MUST be 1 — if 0, the mutation did NOT land; fix the pattern and retry
bash -n scripts/runners/opencode.sh                                      # the mutant must still PARSE
bash tests/test_runner_opencode.sh; echo "rc=$?"                       # MUST be nonzero, naming the ask asserts
cp /tmp/opencode.sh.bak scripts/runners/opencode.sh
bash tests/test_runner_opencode.sh                                     # back to ALL PASS
```

Mutation B — the `max` passthrough must not be remapped. Add codex's mapping:

```bash
cp scripts/runners/opencode.sh /tmp/opencode.sh.bak
perl -0pi -e 's/if \[ "\$MODEL" = "inherit" \]; then MODEL=""; fi/if [ "\$MODEL" = "inherit" ]; then MODEL=""; fi\ncase "\$EFFORT" in max) EFFORT="xhigh" ;; esac/' scripts/runners/opencode.sh
grep -c 'max) EFFORT="xhigh"' scripts/runners/opencode.sh              # MUST be 1
bash tests/test_runner_opencode.sh; echo "rc=$?"                       # MUST be nonzero on the max asserts
cp /tmp/opencode.sh.bak scripts/runners/opencode.sh
bash tests/test_runner_opencode.sh                                     # ALL PASS
rm -f /tmp/opencode.sh.bak
```

- [ ] **Step 8: Check the new regexes under BSD grep**

PATH `grep` is ugrep and is more permissive than the BSD `grep` a portability-sensitive reader may use. Confirm the test file's own patterns behave the same:

```bash
PATH=/usr/bin:/bin:/usr/sbin:/sbin bash tests/test_runner_opencode.sh
```
Expected: `ALL PASS`.

- [ ] **Step 9: Commit**

```bash
git add scripts/runners/opencode.sh scripts/runners/opencode.md sync-agents.sh \
        tests/test_runner_opencode.sh tests/test_sync_agents.sh
git commit -m "feat(0205): opencode runner adapter + contract, registered"
```

---

### Task 2: The runner-wide required-model rule

**Files:**
- Modify: `sync-agents.sh` — `emit_wrapper()`, immediately after the existing change-0168 provenance flags
- Test: `tests/test_sync_agents.sh` — migrate the change-0168 runner-provenance block; add the new rule's tests

**Interfaces:**
- Consumes: `RES_MODEL_FROM_USER` (set by `resolve_agent_layers`, already read by `emit_wrapper` to compute `flag_model`) and `$4`/`$5`/`$6` (runner, harness, agent-name) from `emit_wrapper`'s existing signature.
- Produces: a nonzero exit from `sync-agents.sh` when a `runner:`-bearing `claude` agent resolves no user-configured model. No new function, no new field, no signature change.

**Why this is framework-wide and not opencode-only.** "Is a model required?" must not be an adapter-by-adapter fact a user learns twice. The framework already prefers loud failure over silent degradation (ADR-0037: *explicit config is never silently ignored and never silently degraded*). The guard is milder in value for a subscription-billed child but costs nothing there. This is a **behavior change** for any existing model-less codex or cursor configuration — it needs an ADR, minted at review time by `docket-adr`, not written by this task.

**Ordering constraint (do not get this wrong):** the new check goes **after** `is_registered_runner`. An existing test fixture uses `status: { runner: gemini-cli }` — no model *and* an unregistered runner — and asserts the diagnostic names `gemini-cli`. Checking the model first would change that diagnostic and break the test for the wrong reason.

**The `inherit` leg.** `model: inherit` is docket's own no-pin sentinel, and every adapter normalizes it to "no flag". A user writing `runner: opencode, model: inherit` is explicitly asking for no pin — precisely the pay-per-token-default-of-unknown-identity case this rule exists to prevent. It is therefore treated as no model. Without this leg the rule has a one-word bypass.

**This rule is loud, but it is not fail-*before*-write.** `emit_wrapper`'s three call sites redirect its stdout into the target wrapper path, so the shell creates and truncates that file before the function body runs. When the rule fires, the offending agent is left with a **zero-length** wrapper, and any agent later in the glob is not written at all. That is inert rather than dangerous — an empty wrapper gives the harness nothing to dispatch, and the next successful run overwrites it — but it is a real difference from the change-0168 harness-defaults gate, which validates before the write loop begins. Making this rule fail-before-write would mean resolving every (harness, agent) pair twice, once to validate and once to emit; that is a larger refactor than the rule warrants and is deliberately not attempted here. The diagnostic therefore tells the user to re-run, and the test asserts `! -s` (absent **or** empty), not `! -e`.

- [ ] **Step 1: Write the failing test — the new rule**

Append to `tests/test_sync_agents.sh`, after the existing change-0079 runner blocks (the `unregistered runner under claude` block is a good anchor — put this immediately after its `rm -rf "$SBX"`):

```bash
# ---- change 0205: a runner: with no USER-configured model is a generation-time ERROR ----
# Runner-wide, not opencode-only. Under change 0168 a shipped agents/harness-defaults.yml value is
# never forwarded to a child harness, so a model-less delegation silently ran on the CHILD's own
# default — of unknown identity and, on a pay-per-token backend like OpenRouter, unknown cost. The
# failure surfaced on the bill, not in the run. Raised at generation time, where the config was
# just written, rather than mid-dispatch. Asserted on ALL THREE runners because this is a framework
# rule, not an adapter behavior: a per-adapter implementation would be the defect it replaces.
for rnr in codex cursor opencode; do
  mkgitrepo
  mkdir -p "$SBX/.claude"
  printf 'agents:\n  claude:\n    status: { runner: %s }\n' "$rnr" > "$SBX/.docket.yml"
  err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
  assert "0205/$rnr: model-less runner fails generation nonzero" '[ "$rc" != "0" ]'
  assert "0205/$rnr: the diagnostic names the agent"  'grep -qF "docket-status" <<<"$err"'
  assert "0205/$rnr: the diagnostic names the runner" 'grep -qF "'"$rnr"'" <<<"$err"'
  assert "0205/$rnr: the diagnostic says a model is required" 'grep -qiE "model" <<<"$err"'
  # The error must not leave a USABLE shim behind. Note this is deliberately `! -s` (absent OR
  # empty), NOT `! -e`: emit_wrapper's call sites redirect into the target path, so the shell
  # creates and truncates the file BEFORE the function body runs and exits. The offending agent is
  # therefore left with a zero-length wrapper, which is inert — the harness has nothing to
  # dispatch — and is overwritten on the next successful run. Asserting `! -e` here would be
  # asserting a fail-before-write property this rule does not have.
  assert "0205/$rnr: no usable wrapper was written for the offending agent" \
    '[ ! -s "$SBX/.claude/agents/docket-status.md" ]'
  # NON-VACUITY COMPANION: the same fixture WITH a user model must succeed and emit a real shim.
  # Without this, every assert above stays green if sync-agents.sh broke for an unrelated reason.
  printf 'agents:\n  claude:\n    status: { runner: %s, model: some/model-id }\n' "$rnr" > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc2=$?
  assert "0205/$rnr: the SAME fixture with a user model succeeds (the guard above is not vacuous)" '[ "$rc2" = "0" ]'
  assert "0205/$rnr: and emits a real shim baking that model" \
    'grep -qF -- "--model some/model-id" "$SBX/.claude/agents/docket-status.md"'
  rm -rf "$SBX"
done

# `model: inherit` is docket's own NO-PIN sentinel — every adapter normalizes it to "no flag", so
# accepting it here would leave a one-word bypass around the rule: the child would run on its own
# default exactly as if no model had been written at all.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex, model: inherit }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0205: model: inherit does not satisfy the required-model rule" '[ "$rc" != "0" ]'
assert "0205: the inherit diagnostic still names the agent" 'grep -qF "docket-status" <<<"$err"'
rm -rf "$SBX"

# ORDERING FENCE: the registration check must still fire FIRST. This fixture is model-less AND
# unregistered; if the model check ran first, the diagnostic would stop naming the runner and the
# change-0079 unregistered-runner assert would break for the wrong reason.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: gemini-cli }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"
assert "0205: an unregistered AND model-less runner still reports the REGISTRATION failure first" \
  'grep -qF "gemini-cli" <<<"$err" && grep -qiE "not a registered runner" <<<"$err"'
rm -rf "$SBX"

# A non-claude harness carrying runner: is warned-and-ignored (reserved) and emits NATIVE — the
# required-model rule must not fire there, because no delegation happens.
mkgitrepo
mkdir -p "$SBX/.claude" "$SBX/.cursor"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  cursor:\n    status: { runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0205: a reserved (non-claude) runner does NOT trip the required-model rule" '[ "$rc" = "0" ]'
assert "0205: and its wrapper is still native" '! grep -qF "runner-dispatch" "$SBX/.cursor/agents/docket-status.md"'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E "NOT OK - 0205"`
Expected: the model-less legs fail (`0205/codex: model-less runner fails generation nonzero`, and the same for cursor/opencode, plus the `inherit` leg) — the rule does not exist yet, so `sync-agents.sh` exits 0 and writes a shim. The non-vacuity companions and the ordering fence should already pass.

- [ ] **Step 3: Write the rule**

Modify `sync-agents.sh` in `emit_wrapper()`. The existing provenance block reads:

```bash
  local flag_model="" flag_effort=""
  [ "${RES_MODEL_FROM_USER:-0}" = "1" ]  && flag_model="$2"
  [ "${RES_EFFORT_FROM_USER:-0}" = "1" ] && flag_effort="$3"
  emit_shim "$1" "$2" "$3" "$runner" "$6" "$flag_model" "$flag_effort"
```

Insert the rule between the flag computation and `emit_shim`:

```bash
  local flag_model="" flag_effort=""
  [ "${RES_MODEL_FROM_USER:-0}" = "1" ]  && flag_model="$2"
  [ "${RES_EFFORT_FROM_USER:-0}" = "1" ] && flag_effort="$3"
  # change 0205: a delegated agent MUST carry a user-configured model. Runner-wide, not per-adapter
  # — "is a model required?" must not be an adapter-by-adapter fact a user learns twice. Because a
  # shipped default is never forwarded (the provenance rule directly above), a model-less
  # delegation ran on the CHILD's own default: unknown identity, and on a pay-per-token backend
  # like OpenRouter unknown cost, with the failure surfacing on the bill rather than in the run.
  # `inherit` is docket's own no-pin sentinel — every adapter normalizes it to "no flag", so
  # accepting it here would leave a one-word bypass. Raised at generation time, where the config
  # was just written, and AFTER the registration check above so an unregistered runner still
  # reports its own (more specific) failure.
  if [ -z "$flag_model" ] || [ "$flag_model" = "inherit" ]; then
    log "ERROR docket-$6: runner '$runner' requires an explicit model — add a 'model:' to the agents.$5.$6 entry in a config layer, then re-run. docket never forwards its own shipped default to another harness (that ID means nothing to the child), so without one the run would silently use $runner's own default model, of unknown identity and cost."
    exit 1
  fi
  emit_shim "$1" "$2" "$3" "$runner" "$6" "$flag_model" "$flag_effort"
```

- [ ] **Step 4: Migrate the change-0168 provenance block**

The existing block whose fixture is `status: { runner: codex }` — no user model — is now an illegal configuration and would fail the run, taking seven asserts down with it. It must be **migrated, not deleted**: the provenance property it pins is still real, but only its **effort** half remains reachable.

Find the block introduced by the comment `change 0168: a shipped default never becomes a child-runner flag` in `tests/test_sync_agents.sh`. Replace its fixture and the four model-facing asserts, keeping everything else:

```bash
# ---- change 0168: a shipped default never becomes a child-runner flag -------
# The provenance boundary. `runner:` delegates this agent to a DIFFERENT harness's CLI, so the
# baked --model/--effort flags are read by that child, not by Claude. A shipped
# agents/harness-defaults.yml value is a CLAUDE default; baking it into a Codex dispatch sends a
# Claude model ID to a Codex child. Only a USER-configured value may cross that boundary — the
# resolved pair still pins the wrapper's own native frontmatter, which is bookkeeping for the
# Claude parent and never reaches the child.
#
# CHANGE 0205 NARROWED THIS TEST'S REACH, and the narrowing is a strengthening, not lost coverage.
# The fixture used to configure a bare `runner: codex` with no model; that is now a generation-time
# ERROR (the required-model rule), which makes the MODEL half of the provenance leak structurally
# impossible rather than merely guarded — a shipped model can no longer be the resolved-and-baked
# value under a runner, because a user model is mandatory. What remains reachable, and is therefore
# what this block now pins, is the EFFORT half: effort stays optional, so a user who configures only
# a model must still not have the shipped effort forwarded to the child.
mkgitrepo
mkdir -p "$SBX/.claude"
HROOT168F="$(mktemp -d)"; mkdir -p "$HROOT168F/.claude"
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex, model: user-picked-id }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168F" bash "$SYNC" >/dev/null 2>&1 )
S="$SBX/.claude/agents/docket-status.md"
# Fixture sanity FIRST: without a real shim the negative asserts below are vacuous.
assert "0168: a user-model runner config emits a shim" 'grep -qF "docket.sh runner-dispatch" "$S"'
assert "0168: the shim names the runner" 'grep -qF -- "--runner codex" "$S"'
assert "0168: the USER model is baked (the provenance rule's positive half)" \
  'grep -qF -- "--model user-picked-id" "$S"'
assert "0168: no user effort configured => NO --effort flag baked" '! grep -qF -- "--effort" "$S"'
# 0169 (re-pointed by 0205): the sidecar supplies an EFFORT for this very agent on both the parent
# and the child harness, so the negative assert above is non-vacuous in the direction that matters
# — a shipped effort could be baked into the flags if provenance were ignored. Pin the fixture's
# premise so a future emptying of either block cannot quietly re-vacuum it.
assert "0169: the claude sidecar really does supply an effort for this agent (the guard above is not vacuous)" \
  '[ -n "$(hd_field "$HD" claude status effort)" ]'
assert "0169: the codex sidecar really does supply an effort for this agent" \
  '[ -n "$(hd_field "$HD" codex status effort)" ]'
assert "0169: and neither shipped EFFORT leaked into the runner flags" \
  '! grep -qF -- "--effort $(hd_field "$HD" claude status effort)" "$S" && ! grep -qF -- "--effort $(hd_field "$HD" codex status effort)" "$S"'
assert "0169: nor did the shipped CODEX model" \
  '! grep -qF -- "$(hd_field "$HD" codex status model)" "$S"'
assert "0168: runner shim frontmatter still carries the resolved native effort (bookkeeping)" \
  '[ "$(fm "$S" effort)" = "$(hd_field "$HD" claude status effort)" ]'
```

Leave the block that immediately follows — the one whose fixture is `status: { runner: codex, model: gpt-5.5, effort: high }` and which asserts `an explicit override still passes through to the child` — **unchanged**. It already carries a user model and is unaffected.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_sync_agents.sh`
Expected: final line `ALL PASS`, exit 0. If any `0168`/`0169` assert is red, the migration in Step 4 is wrong — do not "fix" it by loosening the assert.

- [ ] **Step 6: Mutation-test the rule**

The rule is a negative guard on config that does not exist in the happy path, so it must be proven to redden. Confirm the mutation lands both times.

```bash
cd /Users/homer/dev/docket/.worktrees/opencode-runner-adapter
cp sync-agents.sh /tmp/sync-agents.sh.bak

# Mutation A — remove the rule entirely.
grep -c "requires an explicit model" sync-agents.sh   # expect 1
perl -0pi -e 's/  if \[ -z "\$flag_model" \] \|\| \[ "\$flag_model" = "inherit" \]; then/  if false; then/' sync-agents.sh
grep -c "if false; then" sync-agents.sh               # MUST be 1 — if 0, the mutation did NOT land
bash tests/test_sync_agents.sh 2>&1 | grep -c "NOT OK - 0205"   # MUST be > 0
cp /tmp/sync-agents.sh.bak sync-agents.sh

# Mutation B — keep the rule but drop the `inherit` leg (the one-word bypass).
perl -0pi -e 's/ \|\| \[ "\$flag_model" = "inherit" \]//' sync-agents.sh
grep -c 'flag_model" = "inherit"' sync-agents.sh      # MUST be 0 — the leg is gone
bash tests/test_sync_agents.sh 2>&1 | grep -F "NOT OK - 0205: model: inherit"   # MUST print a line
cp /tmp/sync-agents.sh.bak sync-agents.sh
bash tests/test_sync_agents.sh 2>&1 | tail -2          # back to ALL PASS
rm -f /tmp/sync-agents.sh.bak
```

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0205): require an explicit model on every runner delegation"
```

---

### Task 3: Config surface and documentation, end-to-end

**Files:**
- Modify: `.docket.example.yml` — the `runners:` block
- Modify: `tests/test_docket_example_yml.sh` — `where_documented` table, completeness asserts, population floor
- Modify: `README.md` — the *Runner delegation* section
- Modify: `docs/opencode/setup.md` — a new delegation-recipe section
- Modify: `scripts/runner-dispatch.md` — the framework prose that counts shipped pairs

**Interfaces:**
- Consumes: the `runners.opencode.permissions` knob shipped by Task 1 and the required-model rule shipped by Task 2. This task adds no behavior — it makes both discoverable.
- Produces: no code interface. The one machine-checked output is `.docket.example.yml`'s key set, which `tests/test_docket_example_yml.sh` audits exactly.

**Why this is a task and not a footnote.** A new config knob is not done when it merely works: ship the sample config, the README, and the now-stale prose in the same change. This repo has been bitten before — a change added the `skills:` map's resolution logic and skill-body wiring but never surfaced it in the sample config or README, and the human caught it at the merge gate.

**Two pieces of stale prose to fix while here** (both predate this change and are wrong *today*, before any of this lands): `.docket.example.yml`'s `runners:` comment says *"One pair ships today"* and names only codex, and `scripts/runner-dispatch.md` describes the registry seam without the third adapter. `README.md` correctly says "Two pairs" and becomes three.

**Model IDs are outside truth.** The recipe ships OpenRouter IDs that no in-repo test can validate — every mirror assertion compares generated output against the sidecar that generated it, so both sides move together. Do **not** write an assert for them. They are a named human verification item on the results file instead. Use the IDs already shipped in `agents/harness-defaults.yml`'s `opencode:` block, so the recipe re-points nothing new to the outside world.

- [ ] **Step 1: Write the failing test**

Modify `tests/test_docket_example_yml.sh` in three places.

(a) In the `where_documented` case table, extend the header arm and add the new leaf. The header arm currently reads `finalize|learnings|reclaim|build|skills|runners|runners.codex|auto_capture)`:

```bash
    finalize|learnings|reclaim|build|skills|runners|runners.codex|runners.opencode|auto_capture) echo 'elsewhere:HEADER' ;;
```

and beside the two existing `runners.codex.*` entries:

```bash
    runners.opencode.permissions) echo 'elsewhere:scripts/runners/opencode.sh' ;;
```

(b) Beside the existing `completeness: runners.codex.*` asserts, add the new leaf's value anchor and its consumer anchor. Put the consumer anchor next to the existing `runners.codex.sandbox is still read by the codex adapter` assert:

```bash
assert "completeness: runners.opencode.permissions present" \
  'grep -Eq "^[[:space:]]+permissions:[[:space:]]*ask[[:space:]]*(#.*)?$" "$EX"'
```

```bash
assert "runners.opencode.permissions is still read by the opencode adapter" \
  'grep -q "DOCKET_RUNNER_CFG_PERMISSIONS" "$REPO/scripts/runners/opencode.sh"'
```

(c) Bump the exact population floor. The comment enumerating the 18 must be updated in the same edit — the assert's own diagnostic warns that bumping the number alone launders an untagged key back to green:

```bash
# EXACT, not >=. An at-least floor of 15 is satisfied by the PRE-0102 file and would tolerate a
# regression that silently drops both runners.codex leaves. The 20: 3 finalize.*, 2 learnings.*,
# 2 reclaim.*, 1 build.checkpoint, 2 auto_capture.*, runners.codex + its 2 leaves,
# runners.opencode + its 1 leaf, 5 skills.*.
expected_nested_key_count=20
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep "NOT OK"`
Expected: failures for `completeness: runners.opencode.permissions present` (the key is not in the example file yet), `runners.opencode.permissions is still read by the opencode adapter` **passes already** (Task 1 shipped the adapter), and the population-floor assert fails reporting `got 18` against the expected 20.

- [ ] **Step 3: Add the knob to the sample config**

Modify `.docket.example.yml`. Replace the `runners:` block's comment and body:

```yaml
# runners — per-runner knobs for RUNNER DELEGATION (change 0079): handing an agent's whole run to
# a child harness with its own subscription, models, and skills. Activated per agent by an explicit
# `runner:` key inside an `agents:` entry — never inferred from model IDs. Three pairs ship today,
# all with parent `claude` (Claude Code): children `codex` (OpenAI Codex CLI), `cursor` (Cursor
# CLI), and `opencode`. A delegated agent MUST carry an explicit `model:` in your own config — a
# shipped default is never forwarded to another harness, so a model-less delegation is a
# generation-time error rather than a silent run on the child's own default. With no agent carrying
# a `runner:` key, this block is inert.
# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
runners:
  codex:
    sandbox: workspace-write   # workspace-write (default) | danger-full-access
    network: true              # default true — git push and gh need it
  opencode:
    permissions: ask           # ask (default — REFUSES to delegate) | auto-approve
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_example_yml.sh`
Expected: exit 0. If the population floor now reports a number other than 20, do **not** simply set the expected value to whatever it printed — first confirm the new keys sit under the `runners:` header that already carries `scope: any layer`, then reconcile. Also confirm the adjacency-inheritance floor still reports exactly 2:

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -F "adjacency"`
Expected: `ok` — the count is unchanged, because `runners.opencode` inherits its scope from the tagged `runners:` header rather than from a same-depth sibling.

- [ ] **Step 5: Update the README**

Modify `README.md`, in the *Runner delegation — running docket agents on another harness* section.

Replace the opening paragraph's pair count:

```markdown
Docket agents normally run on the harness hosting your session. **Runner delegation** hands an
agent's *whole run* to a child harness with its own subscription, models, and skills — activated
per agent by an explicit `runner:` key, never inferred from model IDs. Three pairs ship today, all
with parent `claude` (Claude Code): children `codex` (OpenAI Codex CLI), `cursor` (Cursor CLI), and
`opencode`.
```

Replace the YAML example so it shows the motivating split rather than three unrelated one-liners:

````markdown
```yaml
# .docket.yml (or the global ~/.config/docket/config.yml — runner is a machine preference)
agents:
  claude:                       # the PARENT harness: when Claude Code hosts the session…
    status: { model: gpt-5.1-codex, effort: medium, runner: codex }   # …run docket-status on Codex
    adr:    { model: gpt-5.1, effort: high, runner: cursor }          # …or run docket-adr on Cursor
    # Delegate the four build workers to cheap OpenRouter models; leave review native.
    build-economy:  { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731 }
    build-standard: { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731 }
runners:
  codex:
    sandbox: workspace-write    # workspace-write (default) | danger-full-access
    network: true               # default true — git push and gh need it
  opencode:
    permissions: auto-approve   # ask (default) REFUSES to delegate — see below
```
````

Add two bullets to the *Rules and limits* list:

```markdown
- **A delegated agent must carry an explicit `model:` in your config.** Docket never forwards its
  own shipped default to another harness — that ID means nothing to the child — so a model-less
  `runner:` is a loud generation-time error rather than a silent run on the child's own default,
  which on a pay-per-token backend surfaces on the bill instead of in the run.
- **Delegate leaves, not orchestrators.** A delegated run's own sub-dispatches run child-natively,
  so delegating `docket-implement-next` drags its review dispatch into the child too. Delegating
  the four `build-*` profile workers rather than the `docket-build` controller is the same rule:
  delegating the controller would move the routing decision into the child as well.
```

Add a prerequisites paragraph after the cursor one:

```markdown
**Prerequisites (opencode):** `opencode` installed (verified against 1.18.11) with a provider
authenticated (`opencode auth login`), and docket skills linked into `~/.agents/skills`
(`link-skills.sh`, automatic on install). `opencode` delegates to `opencode run`. **You must set
`runners.opencode.permissions: auto-approve`** — opencode prompts for approval before editing a
file or running a command, a delegated run has nothing to answer with, and the adapter therefore
refuses up front rather than hanging. That grant is deliberately a visible line in config, not
something you get by typing `runner: opencode`; pair it with opencode's own deny rules. Effort maps
to `--variant` and passes through unmapped, including docket's `max`. Full adapter contract:
`scripts/runners/opencode.md`; the config recipe is in
[docs/opencode/setup.md](docs/opencode/setup.md).
```

- [ ] **Step 6: Add the config recipe to the opencode setup doc**

Modify `docs/opencode/setup.md`. Insert a new section between *Pinning models and effort* and *Verifying it works*:

````markdown
## Delegating Claude Code agents to opencode (runner delegation)

Everything above configures opencode as the harness **hosting** your session. Runner delegation is
the other direction: your session stays in Claude Code, and individual docket agents are handed to
opencode — with its models and its bill — for their whole run. The motivating use is cost
asymmetry: through OpenRouter, opencode reaches DeepSeek-tier models at a fraction of a
frontier-model task, and because docket's build and review roles are already separate agents
(ADR-0063), you can send build work to cheap models while review stays on your Claude
subscription.

```yaml
# .docket.local.yml (this machine only) or the global ~/.config/docket/config.yml
agents:
  claude:                       # the PARENT harness: when Claude Code hosts the session…
    build-economy:  { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731 }
    build-standard: { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731 }
    build-premium:  { runner: opencode, model: openrouter/moonshotai/kimi-k3 }
    build-max:      { runner: opencode, model: openrouter/moonshotai/kimi-k3 }
    # review-lean / review-standard / review-deep: no runner: → native Claude Code
runners:
  opencode:
    permissions: auto-approve   # REQUIRED — see below. Default `ask` refuses to delegate.
```

Re-run `sync-agents.sh` after editing, and restart the parent session.

**Delegate leaves, not orchestrators.** A delegated run's own sub-dispatches run child-natively, so
delegating an orchestrator drags everything beneath it into the child. Delegate
`docket-implement-next` and its review dispatch goes to opencode too. Delegating the four profile
workers rather than the `docket-build` controller is the same rule applied one level down:
delegating the controller would move the routing decision into the child as well.

**Model selection is explicit, by design.** The `opencode:` block in `agents/harness-defaults.yml`
is *not* consulted here, and that is deliberate. That block answers "if opencode ran this whole
project, what should each role cost?"; delegation asks a different question — "which rows do I want
to leave my Claude subscription, and which do I deliberately keep?" — and the build-delegated /
review-native split above is exactly that asymmetry. Cross-indexing the two would also mean
retuning the native table silently changed what your delegated Claude Code builds run on, with the
coupling invisible at the config site. So you write the models yourself, where you can see, grep,
review, and revert them.

Relatedly: **a delegated agent must carry a `model:`.** Docket never forwards its own shipped
default to another harness, so without one the run would fall through to opencode's own default —
pay-per-token, of unknown identity — and the mistake would surface on your bill rather than in the
run. `sync-agents.sh` refuses to generate a model-less delegation.

### What `auto-approve` actually grants

opencode has no sandbox *levels*. It has a permission system that prompts before editing a file or
running a shell command, and `--auto` auto-approves everything **not explicitly denied** in
opencode's own config. Its own help text marks it `(dangerous!)`.

A delegated run cannot answer a prompt, so `runners.opencode.permissions` has two values and no
useful third:

| Value | Effect |
|---|---|
| `ask` (default) | The adapter **refuses to delegate**, with a diagnostic naming this knob. Without `--auto` the child would block on the first approval until something timed out; refusing turns a silent hang into a message. |
| `auto-approve` | Bakes `--auto`. A delegated build worker can then run any command in the repository unwatched, except what your opencode deny rules forbid. |

The default names what actually happens rather than serving as a placeholder, and nobody receives
blanket auto-approval as a side effect of typing `runner: opencode` — the risk is accepted at a
visible line in config. **Pair `auto-approve` with opencode's own deny rules**: `--auto` approves
what is not explicitly denied, so the deny list, not the flag, is the real boundary.

### Verifying a delegated run

Model IDs and entitlement live outside this repo, so no docket test can validate them — confirm
them yourself:

```sh
opencode models openrouter        # the IDs above must appear, spelled exactly
```

Catalog presence is not entitlement: an ID can be listed and still fail under your credentials.
Certify one real dispatch end to end before trusting the setup — ask for a `build-economy` task and
confirm the work happened in opencode, not Claude Code.
````

- [ ] **Step 7: Update the framework prose in the dispatch contract**

Modify `scripts/runner-dispatch.md`. In *Purpose*, the sentence describing the seam names the registry; leave it, but the surrounding framing should not imply a two-adapter world. Replace the *Purpose* paragraph's final sentence:

```markdown
Adding a future runner touches only the seams: a new adapter script
+ contract in `scripts/runners/`, and a registry token in `sync-agents.sh`'s
`REGISTERED_RUNNERS` (generation-time); the facade itself never changes — it has now absorbed
three adapters (`codex`, `cursor`, `opencode`) without a line of change.
```

In the `--model` / `--effort` bullet, extend the provenance sentence with the new rule:

```markdown
- `--model` / `--effort` — forwarded to the adapter verbatim (model is ADR-0015 opaque
  passthrough end-to-end). The generated shim bakes these only from **user-configured** values;
  a value that came from docket's shipped `agents/harness-defaults.yml` is never forwarded. Since
  change 0205 a `runner:`-bearing agent with **no** user-configured model is a generation-time
  error, so the model-less case reaches this facade only on a direct hand invocation.
```

- [ ] **Step 8: Run the full suite**

This is the build gate, and it is the only CI this repo has. Run every test, not just the three this task touched.

```bash
cd /Users/homer/dev/docket/.worktrees/opencode-runner-adapter
fails=0
for t in tests/test_*.sh; do
  if ! out="$(bash "$t" 2>&1)"; then
    fails=$((fails+1)); printf '=== FAIL %s\n%s\n' "$t" "$out"
  fi
done
echo "failing suites: $fails"
```

Expected: `failing suites: 0`.

Pay particular attention to `tests/test_docket_config.sh`, `tests/test_sync_agents_opencode.sh`, and `tests/test_consuming_repo_scripts.sh` — the first two touch the same config and opencode surfaces, and the third audits what a consuming repo can reach.

- [ ] **Step 9: Commit**

```bash
git add .docket.example.yml tests/test_docket_example_yml.sh README.md docs/opencode/setup.md scripts/runner-dispatch.md
git commit -m "docs(0205): ship the opencode delegation recipe, knob, and required-model rule"
```

---

## Notes for the implementer

**The ADR is not yours to write.** The runner-wide required-model rule is a behavior change for
existing model-less codex/cursor configurations and needs an ADR. `docket-implement-next` dispatches
`docket-adr` at review time, which assigns the number and commits it on the metadata branch. Do not
create an ADR file on this feature branch.

**Human verification items for the results file.** These cannot be settled by any in-repo test.
Carry them forward so they land in the results file and the PR body:

1. **Model IDs** — `openrouter/deepseek/deepseek-v4-flash-0731` and `openrouter/moonshotai/kimi-k3`
   must be confirmed against `opencode models openrouter` under the user's own credentials.
   Catalog presence is not entitlement. Docket keeps no vendor allowlist (ADR-0015) and every
   mirror assert compares generated output against the sidecar that generated it, so no test in
   this repo can ever detect a wrong ID.
2. **`--variant` omission** — confirm that omitting the flag yields the provider's default rather
   than an error or a silent substitution. Change 0192 explicitly flagged its own equivalent case
   as unprobed, so this is not assumed.
3. **`--auto` and the deny-list interaction** — `--auto` approving "everything not explicitly
   denied" is read from one line of help text; the deny-list behavior is inferred, not tested. If a
   workable deny-list spelling exists, evaluate whether the recipe should recommend a starter set —
   but do **not** let it grow into a third `permissions` value in this change.
4. **Relay legibility** — confirm opencode's default formatted stdout relays usefully through the
   shim. If decoration makes it unreadable, `--format json` plus a documented extractor is the
   recorded escape hatch (see the adapter contract), and a follow-up change.
5. **Auth preflight semantics** — confirm `opencode auth list`'s exit code on a machine with zero
   credentials. If it reliably exits nonzero, adding an auth probe to the adapter's preflight (as
   `codex.sh` does with `codex login status`) becomes a cheap follow-up.
6. **One live end-to-end delegated dispatch** — a real `build-economy` task on a real branch,
   confirmed to have executed in opencode. Change 0192 certified its rungs natively; this is the
   same bar for the runner path, and nothing in this change is proven by the suite alone.
