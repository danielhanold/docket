# docket command facade — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one executable facade (`scripts/docket.sh`) exposing a finite table of named docket operations — no `eval`/`run`/`shell` escape hatch — with config flowing model-ward as raw `KEY=value` stdout, side effects behind a shared `preflight`, and the subcommand table as the permission inventory.

**Architecture:** A thin routing script (`scripts/docket.sh`) dispatches a fixed allowlist of operations to the existing helper scripts, forwarding args verbatim and passing exit codes/stderr through unmasked. Two named verbs are new: `env` (prints resolved config raw, read-only) and `preflight` (runs today's Step-0 side effects, then prints the env block). The Step-0 sync logic is extracted from `docket-status.sh` into a sourceable `scripts/lib/docket-preflight.sh` that both the facade and `docket-status.sh` share. `docket-config.sh` gains a `--format plain` presentation mode (raw, unquoted) alongside its existing `--format shell` (`%q`, the default, unchanged).

**Tech Stack:** POSIX-ish Bash (`set -uo pipefail`), hermetic Bash test scripts (temp repos + bare origins, no network, no CI), the existing docket helper-script family under `scripts/`.

## Global Constraints

- **Canonical invocation spelling** (byte-identical everywhere docket emits or documents it): `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <operation> [args...]` — the quote closes before `/docket.sh`.
- **No escape hatch, ever:** no `run`/`exec`/`shell`/`eval` operation name; the facade never evaluates, sources, or executes caller-supplied shell text. Arguments after the operation name are forwarded verbatim to the wrapped helper only.
- **Operation name = daily helper's script basename** (minus `.sh`), so the inventory stays grep-derivable — the two exceptions are the named verbs `preflight` (shared impl) and `env` (wraps `docket-config.sh`), documented as verbs in the contract.
- **Routing boundary, not a second implementation:** behavior stays in the helpers; the helper's exit code and stderr pass through unmasked; existing helper argument validation and provenance guards remain authoritative.
- **The subcommand table in `scripts/docket.md` IS the permission inventory.** Every dispatched op is a documented row; no undocumented op; no documented-but-unrouted op; the not-exposed scripts never appear as ops.
- **Nothing docket emits is ever `eval`'d or `source`d by an agent** — `env`/`preflight` print raw `KEY=value` for the model to read as literals.
- **Backward compatibility:** existing `eval "$(docket-config.sh --export)"` callers must be byte-for-byte unaffected — `--format shell` (`%q`) stays the default.
- **Tests are hermetic and standalone:** each is `bash tests/test_<name>.sh`, no network, temp repos + bare origins; the suite is the de-facto gate (no GitHub Actions). Mock seams: `GIT`, `CONFIG_EXPORT_CMD`, `SCRIPTS_DIR` (override the helper dir).
- **LEARNINGS #64 (2026-07-13):** derive gated/inventory call-site lists by grep, never by hand; **mutation-test the sentinel** (strip the feature, watch the test go red) before trusting it. **LEARNINGS #64b:** an aborting resolver emits nothing on stdout, so tests must clear the asserted variable before re-reading, and empty-output must be a distinct failure assertion — never `eval ""` on stale state.

---

### Task 1: `docket-config.sh --format plain` (raw, model-facing presentation)

**Files:**
- Modify: `scripts/docket-config.sh` (the `emit` helper + the `--emit` block at the end; the arg loop)
- Modify: `scripts/docket-config.md` (document `--format`)
- Test: `tests/test_docket_config.sh` (append a `--format plain` section)

**Interfaces:**
- Produces: `docket-config.sh [--export] [--format plain|shell] [--bootstrap] [--repo-dir DIR]`. `--format shell` (default) is today's `%q`-quoted, eval-able output — **unchanged**. `--format plain` emits raw `KEY=value` (no `%q`, no `export ` prefix) for the same ordered key set, with **`METADATA_WORKTREE` absolutized** (docket-mode → `<repo-abs>/.docket`, main-mode → `<repo-abs>`); every other key is byte-identical to its shell-mode value minus the quoting.
- Consumed by: Task 4 (`docket.sh env` / `preflight`).

**Why `METADATA_WORKTREE` only is absolutized:** the relative worktree root (`.` / `.docket`) is the one path value that is dangerous when the reader's cwd differs. The `*_DIR` keys (`CHANGES_DIR`/`ADRS_DIR`/`RESULTS_DIR`) stay repo-relative subpaths because their correct absolute root differs by consumer — `CHANGES_DIR`/`ADRS_DIR` are composed against the metadata worktree, but `RESULTS_DIR` is a feature-worktree subpath — so forcing one root would mislead at least one key. This narrows (does not drop) the spec's "path-valued keys are absolute" and is recorded as an ADR in Task 5/review.

- [ ] **Step 1: Write the failing tests** — append to `tests/test_docket_config.sh` just before the final `exit $fail` (reuse its `mkrepo`/`run`/`rung` fixtures; the file already sets `set -uo pipefail`, `assert`, and a hermetic `XDG_CONFIG_HOME`):

```bash
# ============================================================================
# (Z) --format plain — raw model-facing presentation (change 0068)
# ============================================================================
mkrepo "$tmp/fmt"
# docket branch so bootstrap verdict resolves to PROCEED (mkrepo leaves a live main surface,
# so create the orphan docket branch the way docket-config --bootstrap would).
git -C "$tmp/fmt" push --quiet origin "$(git -C "$tmp/fmt" commit-tree "$(git -C "$tmp/fmt" mktree </dev/null)" -m orphan):refs/heads/docket" 2>/dev/null
git -C "$tmp/fmt" fetch --quiet origin docket 2>/dev/null

# shell format (default) is UNCHANGED: %q-quoted, eval-able, empty => KEY=''
shell_out="$(run "$tmp/fmt" --export)"
assert "shell format still %q-quotes empty values" 'printf "%s" "$shell_out" | grep -qxF "FINALIZE_TEST_COMMAND='\'''\''"'
assert "shell format METADATA_WORKTREE stays relative .docket" 'printf "%s" "$shell_out" | grep -qxF "METADATA_WORKTREE=.docket"'

# plain format: raw KEY=value, no %q, no export prefix, empty => bare "KEY="
plain_out="$(run "$tmp/fmt" --export --format plain)"
assert "plain format emits raw empty value (no quotes)" 'printf "%s\n" "$plain_out" | grep -qxF "FINALIZE_TEST_COMMAND="'
assert "plain format has no export prefix" '! printf "%s\n" "$plain_out" | grep -q "^export "'
assert "plain format emits BOOTSTRAP" 'printf "%s\n" "$plain_out" | grep -qxF "BOOTSTRAP=PROCEED"'
assert "plain format emits raw enum values unquoted" 'printf "%s\n" "$plain_out" | grep -qxF "DOCKET_MODE=docket"'
# METADATA_WORKTREE absolutized in plain mode
fmt_abs="$(cd "$tmp/fmt" && pwd -P)"
assert "plain format absolutizes METADATA_WORKTREE (docket-mode)" 'printf "%s\n" "$plain_out" | grep -qxF "METADATA_WORKTREE=$fmt_abs/.docket"'
assert "plain format keeps CHANGES_DIR as repo-relative subpath" 'printf "%s\n" "$plain_out" | grep -qxF "CHANGES_DIR=docs/changes"'

# plain mode still fails closed on an aborting resolver: nothing on stdout, non-zero exit
# (#64b: clear the asserted capture first so a prior value can never masquerade as success).
plain_abort=""
plain_abort="$(bash "$SCRIPT" --repo-dir "$tmp/does-not-exist" --export --format plain 2>/dev/null)"; abort_rc=$?
assert "plain mode aborts non-zero on bad repo" '[ "$abort_rc" -ne 0 ]'
assert "plain mode emits NOTHING on abort" '[ -z "$plain_abort" ]'

# unknown --format value is a wiring error (exit 2)
run "$tmp/fmt" --export --format bogus >/dev/null 2>&1; fmt_rc=$?
assert "unknown --format exits 2" '[ "$fmt_rc" -eq 2 ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_config.sh; echo "exit=$?"`
Expected: the new `(Z)` assertions print `NOT OK` (— `--format` is an unknown argument today → exit 2 for the plain runs; `plain format …` assertions fail), `exit=1`.

- [ ] **Step 3: Add the `--format` flag and a format-aware emitter to `scripts/docket-config.sh`**

In the arg loop (after the `--export` case, before `-h|--help`), add:

```bash
    --format)    [ $# -ge 2 ] || { printf 'docket-config: --format requires an argument\n' >&2; exit 2; }
                 case "$2" in plain|shell) FORMAT="$2" ;; *) printf 'docket-config: --format must be plain or shell, got %s\n' "$2" >&2; exit 2 ;; esac
                 shift ;;
```

Add `FORMAT=shell` to the defaults block (next to `MODE=export`).

Replace the `emit(){ printf '%s=%q\n' "$1" "$2"; }` definition with a format-aware pair:

```bash
emit(){   # emit KEY VALUE — presentation keyed on $FORMAT
  case "$FORMAT" in
    plain) printf '%s=%s\n'  "$1" "$2" ;;   # raw, model-facing (never eval'd)
    *)     printf '%s=%q\n'  "$1" "$2" ;;   # shell-eval-able (default; unchanged)
  esac
}
```

In the `--emit` block, compute the plain-mode absolute worktree once and emit it in place of the relative value:

```bash
  # METADATA_WORKTREE: relative for shell (eval'd by code running at the repo root); absolute for
  # plain (the model reads it as a cwd-independent literal). REPO_DIR is the resolver's repo.
  MW_EMIT="$METADATA_WORKTREE"
  if [ "$FORMAT" = plain ]; then
    REPO_ABS="$(cd "$REPO_DIR" && pwd -P)"
    case "$METADATA_WORKTREE" in
      .)  MW_EMIT="$REPO_ABS" ;;
      *)  MW_EMIT="$REPO_ABS/$METADATA_WORKTREE" ;;
    esac
  fi
```

and change the `emit METADATA_WORKTREE "$METADATA_WORKTREE"` line to `emit METADATA_WORKTREE "$MW_EMIT"`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh; echo "exit=$?"`
Expected: all `ok`, `exit=0` (the new `(Z)` block and every pre-existing assertion).

- [ ] **Step 5: Document `--format` in the contract**

In `scripts/docket-config.md`, under Usage, add a `--format plain|shell` row: "shell (default) — `%q`-quoted, eval-able (the historical `eval "$(… --export)"` contract, unchanged); plain — raw `KEY=value`, no quoting, no `export ` prefix, `METADATA_WORKTREE` absolutized, for the docket facade's model-facing `env`/`preflight` output (never eval'd)." Note the abort posture is identical in both formats (nothing on stdout, non-zero exit).

- [ ] **Step 6: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0068): add docket-config.sh --format plain (raw model-facing output)"
```

---

### Task 2: shared preflight library `scripts/lib/docket-preflight.sh`

**Files:**
- Create: `scripts/lib/docket-preflight.sh`
- Test: `tests/test_docket_preflight.sh`

**Interfaces:**
- Produces: a sourceable function `docket_preflight <scripts_dir>` that (1) resolves config via `${CONFIG_EXPORT_CMD:-<scripts_dir>/docket-config.sh --export}` and evals it into the **caller's** scope (setting `DOCKET_MODE`, `METADATA_BRANCH`, `METADATA_WORKTREE`, `INTEGRATION_BRANCH`, `CHANGES_DIR`, `BOARD_SURFACES`, `BOOTSTRAP`, …); (2) fails closed on any non-`PROCEED` bootstrap verdict (`return 1` + a stderr diagnostic — `STOP_MIGRATE` names `migrate-to-docket.sh`); (3) ensures the metadata worktree (docket-mode: create from `$METADATA_BRANCH` then `origin/$METADATA_BRANCH`, idempotent), disables its hooks via `<scripts_dir>/disable-worktree-hooks.sh` (best-effort), and `fetch` + `pull --rebase` the metadata branch (main-mode: a plain `git pull --rebase`). Returns 0 on success. Honors the `GIT` seam for every git call. Prints nothing on stdout (side-effect + resolution only).
- Consumed by: Task 3 (`docket-status.sh`), Task 4 (`docket.sh preflight`).

- [ ] **Step 1: Write the failing tests** — create `tests/test_docket_preflight.sh`:

```bash
#!/usr/bin/env bash
# tests/test_docket_preflight.sh — hermetic tests for scripts/lib/docket-preflight.sh (change 0068).
# Sources the lib and drives docket_preflight against stubbed config exports + temp repos. No network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LIB="$REPO/scripts/lib/docket-preflight.sh"
SCRIPTS="$REPO/scripts"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# A fixture config-export command: prints the given lines. $1 = a file with KEY=value lines.
mkexport(){ printf '#!/usr/bin/env bash\ncat %q\n' "$1" > "$2"; chmod +x "$2"; }

# --- (A) non-PROCEED verdicts fail closed -----------------------------------
printf 'BOOTSTRAP=STOP_MIGRATE\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=.docket\n' > "$tmp/stop.env"
mkexport "$tmp/stop.env" "$tmp/stop-export.sh"
( . "$LIB"; CONFIG_EXPORT_CMD="bash $tmp/stop-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/stop.err"; rc=$?
assert "STOP_MIGRATE returns non-zero" '[ "$rc" -ne 0 ]'
assert "STOP_MIGRATE names migrate-to-docket" 'grep -qi "migrate" "$tmp/stop.err"'

printf 'BOOTSTRAP=CREATE_ORPHAN\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=.docket\n' > "$tmp/orphan.env"
mkexport "$tmp/orphan.env" "$tmp/orphan-export.sh"
( . "$LIB"; CONFIG_EXPORT_CMD="bash $tmp/orphan-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/orphan.err"; rc=$?
assert "CREATE_ORPHAN returns non-zero" '[ "$rc" -ne 0 ]'

printf 'BOOTSTRAP=WAT\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=.docket\n' > "$tmp/wat.env"
mkexport "$tmp/wat.env" "$tmp/wat-export.sh"
( . "$LIB"; CONFIG_EXPORT_CMD="bash $tmp/wat-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/wat.err"; rc=$?
assert "unknown verdict returns non-zero" '[ "$rc" -ne 0 ]'

# --- (B) docket-mode PROCEED creates + syncs the metadata worktree ----------
# Build a repo with a real `docket` branch on a bare origin.
bare="$tmp/dk.git"; work="$tmp/dk"
git init --quiet --bare "$bare"
git clone --quiet "$bare" "$work" 2>/dev/null
git -C "$work" config user.email t@t.test; git -C "$work" config user.name Test
git -C "$work" checkout --quiet -b main; : > "$work/README.md"
git -C "$work" add README.md; git -C "$work" commit --quiet -m init; git -C "$work" push --quiet -u origin main
git -C "$work" push --quiet origin "$(git -C "$work" commit-tree "$(git -C "$work" mktree </dev/null)" -m orphan):refs/heads/docket"
git -C "$work" fetch --quiet origin docket
printf 'BOOTSTRAP=PROCEED\nDOCKET_MODE=docket\nMETADATA_BRANCH=docket\nMETADATA_WORKTREE=.docket\nINTEGRATION_BRANCH=main\nCHANGES_DIR=docs/changes\n' > "$tmp/ok.env"
mkexport "$tmp/ok.env" "$tmp/ok-export.sh"
assert "metadata worktree absent before preflight" '[ ! -d "$work/.docket" ]'
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" ) >/dev/null 2>"$tmp/ok.err"; rc=$?
assert "docket-mode PROCEED returns zero" '[ "$rc" -eq 0 ]'
assert "docket-mode PROCEED created the metadata worktree" '[ -d "$work/.docket" ]'

# --- (C) PROCEED sets config vars in the caller's scope ---------------------
DOCKET_MODE=""; METADATA_WORKTREE=""
( cd "$work" && . "$LIB" && CONFIG_EXPORT_CMD="bash $tmp/ok-export.sh" docket_preflight "$SCRIPTS" >/dev/null 2>&1 \
  && [ "$DOCKET_MODE" = docket ] && [ "$METADATA_WORKTREE" = .docket ] ); rc=$?
assert "PROCEED exposes resolved config vars to the caller" '[ "$rc" -eq 0 ]'

exit $fail
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_preflight.sh; echo "exit=$?"`
Expected: FAIL — `scripts/lib/docket-preflight.sh` does not exist yet (source errors / all assertions `NOT OK`), `exit=1`.

- [ ] **Step 3: Create `scripts/lib/docket-preflight.sh`**

```bash
#!/usr/bin/env bash
# scripts/lib/docket-preflight.sh — the shared Step-0 preflight (change 0068). Sourced by
# scripts/docket.sh and scripts/docket-status.sh; extracts the metadata-worktree sync that was
# docket-status.sh's private ensure_and_sync_worktree so there is ONE sync implementation.
#
# docket_preflight <scripts_dir>
#   1. resolve config: eval "$(${CONFIG_EXPORT_CMD:-<scripts_dir>/docket-config.sh --export})"
#      into the CALLER's scope (DOCKET_MODE, METADATA_BRANCH, METADATA_WORKTREE, BOOTSTRAP, …).
#   2. enforce the bootstrap verdict fail-closed (non-PROCEED => return 1 + stderr diagnostic).
#   3. ensure + sync the metadata worktree (docket-mode) or the primary tree (main-mode);
#      disable the metadata worktree's shared git hooks (best-effort, change 0063).
#   Returns 0 on success. Prints nothing on stdout. Honors the GIT and CONFIG_EXPORT_CMD seams.
# This file is a sourced helper: it is documented within its callers' contracts (docket.md,
# docket-status.md), not by a co-located .md (test_script_contracts_coverage.sh scopes lib/ out).

docket_preflight(){
  local scripts_dir="$1"
  local git="${GIT:-git}"
  local cfg
  cfg="$(${CONFIG_EXPORT_CMD:-"$scripts_dir"/docket-config.sh --export})" \
    || { echo "docket-preflight: config export failed" >&2; return 1; }
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE)  echo "docket-preflight: repo not migrated — run migrate-to-docket.sh" >&2; return 1 ;;
    CREATE_ORPHAN) echo "docket-preflight: fresh repo — bootstrap is opt-in; run docket-config.sh --bootstrap (or a docket skill) to create the docket branch" >&2; return 1 ;;
    *) echo "docket-preflight: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; return 1 ;;
  esac

  if [ "${DOCKET_MODE:-}" = docket ]; then
    local wt="${METADATA_WORKTREE:-.docket}"
    if [ ! -d "$wt" ]; then
      "$git" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$git" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-preflight: cannot create metadata worktree $wt" >&2; return 1; }
    fi
    # change 0063: skip the repo's shared git hooks on the metadata worktree (idempotent;
    # self-heals existing installs). Best-effort — a failure here must not block preflight.
    "$scripts_dir"/disable-worktree-hooks.sh --worktree "$wt" >&2 \
      || echo "docket-preflight: warning — could not disable hooks on $wt (continuing)" >&2
    "$git" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
      && "$git" -C "$wt" pull --rebase origin "$METADATA_BRANCH" >&2 \
      || { echo "docket-preflight: metadata worktree sync failed" >&2; return 1; }
  else
    "$git" pull --rebase >&2 || { echo "docket-preflight: metadata sync failed" >&2; return 1; }
  fi
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_docket_preflight.sh; echo "exit=$?"`
Expected: all `ok`, `exit=0`.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/docket-preflight.sh tests/test_docket_preflight.sh
git commit -m "feat(0068): extract shared docket_preflight into scripts/lib/docket-preflight.sh"
```

---

### Task 3: rewire `docket-status.sh` onto the shared preflight

**Files:**
- Modify: `scripts/docket-status.sh` (source the lib; replace `main()`'s config+bootstrap+sync block; delete the private `ensure_and_sync_worktree` and `config_export`)
- Modify: `scripts/docket-status.md` (note the shared preflight)
- Test: `tests/test_docket_status.sh` (add a "no private sync" structural assertion; existing assertions must stay green)

**Interfaces:**
- Consumes: `docket_preflight` from Task 2 (via `. "$SELF_DIR"/lib/docket-preflight.sh`).
- Preserves: the `CONFIG_EXPORT_CMD` and `GIT` seams (the lib honors both), and the existing `BOARD_SURFACES`/`METADATA_WORKTREE`/`DOCKET_MODE` variables in `main()` scope (the lib evals them into scope, exactly as the old inline `eval` did).

- [ ] **Step 1: Write the failing structural test** — append to `tests/test_docket_status.sh` before its final `exit`:

```bash
# --- (0068) docket-status shares the preflight impl; no private sync copy -----
assert "docket-status sources the shared preflight lib" \
  'grep -q "lib/docket-preflight.sh" "$SCRIPT"'
assert "docket-status calls docket_preflight" \
  'grep -q "docket_preflight" "$SCRIPT"'
assert "docket-status no longer defines a private ensure_and_sync_worktree" \
  '! grep -qE "^ensure_and_sync_worktree\(\)" "$SCRIPT"'
```

(`$SCRIPT` is already defined at the top of `tests/test_docket_status.sh` as the path to `scripts/docket-status.sh`. Confirm the variable name before running; if it differs, use the file's existing script-path variable.)

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_status.sh; echo "exit=$?"`
Expected: the three new assertions `NOT OK` (lib not sourced yet, private fn still present), `exit=1`.

- [ ] **Step 3: Rewire `scripts/docket-status.sh`**

Add the source line after the existing frontmatter source (`. "$SELF_DIR"/lib/docket-frontmatter.sh`, ~line 23):

```bash
# shellcheck source=lib/docket-preflight.sh
. "$SELF_DIR"/lib/docket-preflight.sh
```

Delete the private `config_export(){ … }` (line ~40) and the entire `ensure_and_sync_worktree(){ … }` function (lines ~42–60).

In `main()`, replace lines ~404–412 (the `cfg="$(config_export)"` … `eval` … bootstrap `case` … `ensure_and_sync_worktree`) with a single call:

```bash
main(){
  docket_preflight "$SCRIPTS_DIR" || exit 1
  board_pass
  ...
```

Everything below `board_pass` is unchanged (it consumes `BOARD_SURFACES`, `DOCKET_MODE`, `METADATA_WORKTREE`, etc., which `docket_preflight` set in scope).

- [ ] **Step 4: Run the full docket-status suite to verify green**

Run: `bash tests/test_docket_status.sh; echo "exit=$?"`
Expected: all `ok` including the pre-existing bootstrap-gate (STOP_MIGRATE prints "migrate"; CREATE_ORPHAN non-zero), sync-fixture (docket-mode creates the worktree), board, and sweep assertions — the seams and messages are preserved by the lib — plus the three new `(0068)` structural assertions, `exit=0`.

- [ ] **Step 5: Update the contract + commit**

In `scripts/docket-status.md`, replace any description of the private worktree sync with: "Step-0 sync is delegated to the shared `scripts/lib/docket-preflight.sh` (`docket_preflight`), the single sync implementation shared with the `docket.sh` facade." Then:

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "refactor(0068): docket-status.sh reuses shared docket_preflight (no private sync copy)"
```

---

### Task 4: the facade `scripts/docket.sh` (dispatch + `env` + `preflight`)

**Files:**
- Create: `scripts/docket.sh`
- Test: `tests/test_docket_facade.sh` (dispatch, forwarding, exit-code, rejection, `env`, `preflight`; the inventory sentinel comes in Task 5)

**Interfaces:**
- Produces: `docket.sh <operation> [args...]`. Operations: `preflight`, `env` (verbs), and the wrapped helpers `docket-status`, `board-refresh`, `archive-change`, `terminal-publish`, `cleanup-feature-branch`, `github-mirror`, `sync-integration-branch`, `render-change-links`, `render-adr-index`, `adr-checks`, `board-checks`. Wrapped ops `exec` `"$SCRIPTS_DIR"/<basename>.sh "$@"` (args verbatim, exit/stderr unmasked). `env` = `exec "$SCRIPTS_DIR"/docket-config.sh --export --format plain`. `preflight` = `docket_preflight "$SELF_DIR"` then print the plain env block. Unknown/missing op → exit 2, listing the supported operations on stderr. Mock seam: `SCRIPTS_DIR` (default `$SELF_DIR`) overrides the helper dir; `GIT`, `CONFIG_EXPORT_CMD` honored via the lib.
- Consumes: Task 1 (`--format plain`), Task 2 (`docket_preflight`).

- [ ] **Step 1: Write the failing tests** — create `tests/test_docket_facade.sh`:

```bash
#!/usr/bin/env bash
# tests/test_docket_facade.sh — hermetic tests for scripts/docket.sh (change 0068): dispatch,
# argument forwarding, exit-code passthrough, operation rejection, env, preflight, and the
# inventory sentinel (Task 5). No network; stub helpers via the SCRIPTS_DIR seam.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
FACADE="$REPO/scripts/docket.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- stub helper dir: each stub echoes its own basename + forwarded args, and can exit N -------
stub="$tmp/stub-scripts"; mkdir -p "$stub"
for h in docket-status board-refresh archive-change terminal-publish cleanup-feature-branch \
         github-mirror sync-integration-branch render-change-links render-adr-index \
         adr-checks board-checks docket-config; do
  printf '#!/usr/bin/env bash\necho "CALLED %s $*"\nexit 0\n' "$h" > "$stub/$h.sh"; chmod +x "$stub/$h.sh"
done
# a helper that exits with a chosen code to prove exit-code passthrough
printf '#!/usr/bin/env bash\necho "CALLED board-checks $*" >&2\nexit 7\n' > "$stub/board-checks.sh"; chmod +x "$stub/board-checks.sh"

runf(){ SCRIPTS_DIR="$stub" bash "$FACADE" "$@"; }

# --- (A) dispatch + verbatim argument forwarding ----------------------------
out="$(SCRIPTS_DIR="$stub" bash "$FACADE" board-refresh --changes-dir /x --surfaces "inline github" 2>/dev/null)"
assert "board-refresh routes to board-refresh.sh with args verbatim" \
  '[ "$out" = "CALLED board-refresh --changes-dir /x --surfaces inline github" ]'
out="$(SCRIPTS_DIR="$stub" bash "$FACADE" archive-change --id 7 --slug foo 2>/dev/null)"
assert "archive-change routes with args" '[ "$out" = "CALLED archive-change --id 7 --slug foo" ]'

# --- (B) exit-code passthrough (unmasked) -----------------------------------
SCRIPTS_DIR="$stub" bash "$FACADE" board-checks >/dev/null 2>&1; assert "helper exit code passes through" '[ $? -eq 7 ]'

# --- (C) reject unknown + not-exposed operations ----------------------------
SCRIPTS_DIR="$stub" bash "$FACADE" definitely-not-an-op >/dev/null 2>"$tmp/unk.err"; rc=$?
assert "unknown op exits 2" '[ "$rc" -eq 2 ]'
assert "unknown op lists supported operations" 'grep -q "board-refresh" "$tmp/unk.err"'
for forbidden in docket-config disable-worktree-hooks render-board install migrate-to-docket sync-agents run exec shell eval bash; do
  SCRIPTS_DIR="$stub" bash "$FACADE" "$forbidden" >/dev/null 2>&1
  assert "not-exposed/escape op '$forbidden' is rejected (exit 2)" '[ $? -eq 2 ]'
done
# missing operation name
SCRIPTS_DIR="$stub" bash "$FACADE" >/dev/null 2>&1; assert "missing op exits 2" '[ $? -eq 2 ]'

# --- (D) env: raw plain KEY=value from a real repo fixture ------------------
bare="$tmp/e.git"; work="$tmp/e"
git init --quiet --bare "$bare"; git clone --quiet "$bare" "$work" 2>/dev/null
git -C "$work" config user.email t@t.test; git -C "$work" config user.name Test
git -C "$work" checkout --quiet -b main; : > "$work/README.md"
git -C "$work" add README.md; git -C "$work" commit --quiet -m init; git -C "$work" push --quiet -u origin main
git -C "$work" push --quiet origin "$(git -C "$work" commit-tree "$(git -C "$work" mktree </dev/null)" -m orphan):refs/heads/docket"
git -C "$work" fetch --quiet origin docket
env_out="$(cd "$work" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" env 2>/dev/null)"; env_rc=$?
assert "env exits zero on a migrated repo" '[ "$env_rc" -eq 0 ]'
assert "env emits raw BOOTSTRAP line" 'printf "%s\n" "$env_out" | grep -qxF "BOOTSTRAP=PROCEED"'
assert "env emits no export prefix / no %q quotes" '! printf "%s\n" "$env_out" | grep -qE "^export |=.\x27.*\x27$"'
work_abs="$(cd "$work" && pwd -P)"
assert "env absolutizes METADATA_WORKTREE" 'printf "%s\n" "$env_out" | grep -qxF "METADATA_WORKTREE=$work_abs/.docket"'

# env fails closed (#64b: clear capture first) — aborting resolver emits nothing, non-zero
env_abort=""
env_abort="$(cd "$tmp" && bash "$FACADE" env 2>/dev/null)"; ea_rc=$?   # $tmp is not a git repo
assert "env aborts non-zero outside a repo" '[ "$ea_rc" -ne 0 ]'
assert "env emits nothing on abort" '[ -z "$env_abort" ]'

# --- (E) preflight: side effects (worktree) THEN prints the env block -------
rm -rf "$work/.docket"
pf_out="$(cd "$work" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" preflight 2>/dev/null)"; pf_rc=$?
assert "preflight exits zero on a migrated repo" '[ "$pf_rc" -eq 0 ]'
assert "preflight created the metadata worktree" '[ -d "$work/.docket" ]'
assert "preflight prints the env block (BOOTSTRAP present)" 'printf "%s\n" "$pf_out" | grep -qxF "BOOTSTRAP=PROCEED"'

exit $fail
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_facade.sh; echo "exit=$?"`
Expected: FAIL — `scripts/docket.sh` does not exist, `exit=1`.

- [ ] **Step 3: Create `scripts/docket.sh`**

```bash
#!/usr/bin/env bash
# scripts/docket.sh — the one executable docket facade (change 0068). A finite table of named
# operations; NO run/exec/shell/eval escape hatch; NEVER evaluates, sources, or executes
# caller-supplied shell text. Config flows model-ward: `env`/`preflight` print raw KEY=value on
# stdout for the model to read as literals — nothing here is meant to be eval'd or sourced by an
# agent. The subcommand table below (and in scripts/docket.md) IS the permission inventory.
#
# Usage: docket.sh <operation> [args...]
#   preflight                 Step-0 side effects (sync the metadata worktree), then print env
#   env                       print resolved KEY=value config (read-only)
#   docket-status [args]      the docket-status orchestrator
#   board-refresh [args]      gated BOARD.md writer
#   archive-change [args]     move a change to archive/
#   terminal-publish [args]   publish terminal records onto the integration branch
#   cleanup-feature-branch    delete a merged feature branch + worktree
#   github-mirror [args]      GitHub Issues/Projects mirror
#   sync-integration-branch   fast-forward the local integration branch
#   render-change-links       per-change Artifacts link block (pure renderer)
#   render-adr-index          ADR index (pure renderer)
#   adr-checks [args]         ADR consistency checks
#   board-checks [args]       board consistency checks
#
# Contract: scripts/docket.md. Mock seams: SCRIPTS_DIR (helper dir), GIT, CONFIG_EXPORT_CMD.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"
GIT="${GIT:-git}"
# shellcheck source=lib/docket-preflight.sh
. "$SELF_DIR"/lib/docket-preflight.sh

# The exposed wrapped-helper operations (op name == helper basename). Single source of the
# dispatch allowlist; the sentinel test greps THIS array and the docket.md table.
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index adr-checks board-checks"

usage(){ sed -n '/^# Usage:/,/^# Contract:/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }
reject(){ printf 'docket: unknown operation: %s\n' "${1:-<none>}" >&2; printf 'supported operations: preflight env %s\n' "$WRAPPED_OPS" >&2; exit 2; }

op="${1:-}"; [ $# -gt 0 ] && shift
case "$op" in
  -h|--help) usage; exit 0 ;;
  "" ) reject "" ;;
  env)
    exec "$SCRIPTS_DIR"/docket-config.sh --export --format plain ;;
  preflight)
    docket_preflight "$SELF_DIR" || exit 1
    exec "$SCRIPTS_DIR"/docket-config.sh --export --format plain ;;
  *)
    for _o in $WRAPPED_OPS; do
      if [ "$op" = "$_o" ]; then exec "$SCRIPTS_DIR"/"$op".sh "$@"; fi
    done
    reject "$op" ;;
esac
```

Note: `preflight` runs `docket_preflight "$SELF_DIR"` (side effects use the real lib + real helpers under `$SELF_DIR`), then `exec`s the plain env print. In hermetic dispatch tests the wrapped ops are exercised through `$SCRIPTS_DIR`; `env`/`preflight` are exercised against a real repo fixture (so the real `docket-config.sh` runs) — matching the test file above.

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_docket_facade.sh; echo "exit=$?"`
Expected: all `ok`, `exit=0`.

- [ ] **Step 5: Commit**

```bash
git add scripts/docket.sh tests/test_docket_facade.sh
git commit -m "feat(0068): add scripts/docket.sh facade (dispatch + env + preflight)"
```

---

### Task 5: `scripts/docket.md` contract + the inventory sentinel

**Files:**
- Create: `scripts/docket.md`
- Test: `tests/test_docket_facade.sh` (append the inventory sentinel section)

**Interfaces:**
- Produces: `scripts/docket.md` — the contract whose **Subcommand inventory** table (one row per operation, with a `Wraps` column) is the permission inventory. Satisfies `test_script_contracts_coverage.sh` (docket.sh ↔ docket.md co-location).
- Sentinel invariants (all grep-derived from `scripts/docket.sh` + `scripts/docket.md`, per LEARNINGS #64 — no third hand-maintained copy in the test):
  1. The operation set in `docket.sh` (`preflight`, `env`, and every `WRAPPED_OPS` token) equals the operation set documented in `docket.md`'s inventory table.
  2. Each wrapped op maps to an existing `scripts/<op>.sh` (op name == basename); `preflight`/`env` are the documented verbs (exempt — they appear in the table with a non-`<op>.sh` `Wraps` value).
  3. None of the not-exposed scripts (`docket-config`, `disable-worktree-hooks`, `render-board`, `install`, `migrate-to-docket`, `sync-agents`, `ensure-docket-env`, `ensure-claude-settings`) appears as an exposed op in either `docket.sh` or the `docket.md` table.
  4. `docket.sh` contains no `run`/`exec`/`shell`/`eval` operation NAME in its dispatch and never evals `"$@"`/positional caller args.

- [ ] **Step 1: Write the failing sentinel tests** — append to `tests/test_docket_facade.sh` before `exit $fail`:

```bash
# ============================================================================
# Inventory sentinel (change 0068) — derive both sides by grep; never hand-list.
# ============================================================================
FSH="$REPO/scripts/docket.sh"; FMD="$REPO/scripts/docket.md"

# ops declared in docket.sh: the two verbs + the WRAPPED_OPS array value, tokenized.
sh_wrapped="$(sed -n 's/^WRAPPED_OPS="\(.*\)"/\1/p' "$FSH")"
sh_ops="$(printf 'preflight\nenv\n%s\n' "$sh_wrapped" | tr ' ' '\n' | sed '/^$/d' | sort -u)"

# ops documented in docket.md: the leading `| \`op\` |` cell of each inventory-table row.
md_ops="$(grep -oE '^\| `[a-z-]+` ' "$FMD" | tr -d '`|' | tr -d ' ' | sort -u)"

assert "docket.sh op set == docket.md documented op set" '[ "$sh_ops" = "$md_ops" ] || { echo "sh=[$sh_ops] md=[$md_ops]" >&2; false; }'

# each wrapped op has a live helper of the same basename
sentinel_ok=1
for o in $sh_wrapped; do [ -f "$REPO/scripts/$o.sh" ] || { echo "op $o has no scripts/$o.sh" >&2; sentinel_ok=0; }; done
assert "every wrapped op maps to scripts/<op>.sh" '[ "$sentinel_ok" -eq 1 ]'

# not-exposed scripts never appear as ops (dispatch table or contract table)
for ne in docket-config disable-worktree-hooks render-board install migrate-to-docket sync-agents ensure-docket-env ensure-claude-settings; do
  assert "not-exposed '$ne' is not a docket.sh op"  '! printf "%s\n" "$sh_ops" | grep -qxF "$ne"'
  assert "not-exposed '$ne' is not a docket.md op"  '! printf "%s\n" "$md_ops" | grep -qxF "$ne"'
done

# no escape-hatch op name; never evals positional args
for hatch in run exec-op shell eval; do
  assert "docket.sh dispatch has no '$hatch' operation arm" '! grep -qE "^\s*'"$hatch"'\)" "$FSH"'
done
assert "docket.sh never evals caller args" '! grep -qE "eval .*\"\$[@*]\"|eval .*\$op" "$FSH"'
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_facade.sh; echo "exit=$?"`
Expected: the sentinel block fails — `scripts/docket.md` does not exist so `md_ops` is empty and the set-equality assertion is `NOT OK`, `exit=1`.

- [ ] **Step 3: Create `scripts/docket.md`**

Author the contract in the house style (Purpose / Usage / **Subcommand inventory** / Behavior / `env` output / Exit codes / Invariants). The inventory table must have exactly one row per op with a `Wraps` column, and each op cell formatted as `` | `op` | ``. Include every op: `preflight` (Wraps: shared preflight impl), `env` (Wraps: `docket-config.sh`, read-only), and the 11 wrapped helpers (Wraps: `<op>.sh`). State explicitly:
- The table **is the permission inventory**; operation name = helper basename (except the `preflight`/`env` verbs).
- No `run`/`exec`/`shell`/`eval` op; the facade never executes caller-supplied shell text; args are forwarded verbatim and the helper's exit code + stderr pass through unmasked.
- **Not exposed** (facade rejects, exit 2): `docket-config.sh` (reached via `env`), `disable-worktree-hooks.sh`, `render-board.sh` (internal to `board-refresh`), and the human-initiated tier (`install.sh`, `migrate-to-docket.sh`, `sync-agents.sh`, `ensure-*.sh`).
- **`env` output:** raw `KEY=value`, one per line, no `export ` prefix, no shell quoting (consumer is the model); `BOOTSTRAP` always present; an aborting resolver emits nothing and exits non-zero. `METADATA_WORKTREE` is absolutized; the `*_DIR` keys stay repo-relative subpaths (composed by the caller against the appropriate tree — metadata worktree for changes/adrs, feature worktree for results). This is a deliberate, ADR-recorded narrowing of the spec's "all path-valued keys absolute" (the `*_DIR` root differs by consumer).
- **`preflight`:** the sanctioned Step-0 / mid-run re-sync verb; runs `docket_preflight` (shared with `docket-status.sh`) then prints the `env` block; non-`PROCEED` bootstrap verdicts fail closed.
- Exit codes: `0` success; `2` unknown/missing operation (lists supported ops); otherwise the wrapped helper's own exit code, verbatim.

- [ ] **Step 4: Run to verify the sentinel passes**

Run: `bash tests/test_docket_facade.sh; echo "exit=$?"`
Expected: all `ok` including the sentinel block, `exit=0`.

- [ ] **Step 5: Verify contract coverage stays green**

Run: `bash tests/test_script_contracts_coverage.sh; echo "exit=$?"`
Expected: all `ok` (docket.sh now has docket.md; no orphaned contract), `exit=0`.

- [ ] **Step 6: Mutation-test the sentinel (LEARNINGS #64 — do NOT skip)**

Perform each mutation, run `bash tests/test_docket_facade.sh`, confirm it **reddens**, then `git checkout -- ` the mutated file. Record the four mutations + their red results in the results file (Task 6):
1. Add a bogus token to `WRAPPED_OPS` in `docket.sh` (op not in the contract) → set-equality reddens.
2. Delete one op row from the `docket.md` table → set-equality reddens.
3. Rename one wrapped op in `docket.sh` to a name with no `scripts/<op>.sh` → "maps to scripts/<op>.sh" reddens.
4. Add a `render-board` row to the `docket.md` table → "not-exposed render-board is not a docket.md op" reddens.

```bash
# example for mutation 1:
sed -i.bak 's/WRAPPED_OPS="docket-status/WRAPPED_OPS="bogus-op docket-status/' scripts/docket.sh
bash tests/test_docket_facade.sh; echo "exit=$? (expect 1)"
git checkout -- scripts/docket.sh; rm -f scripts/docket.sh.bak
```

- [ ] **Step 7: Commit**

```bash
git add scripts/docket.md tests/test_docket_facade.sh
git commit -m "feat(0068): scripts/docket.md contract + grep-derived, mutation-tested inventory sentinel"
```

---

### Task 6: whole affected-suite green + results close-out

**Files:**
- (No source changes) — verification only; the results file is authored in the implementer's step 6.5.

- [ ] **Step 1: Run every test this change touched or could affect**

Run each and confirm `exit=0`:

```bash
for t in test_docket_config test_docket_preflight test_docket_status test_docket_facade \
         test_script_contracts_coverage test_render_board test_board_refresh \
         test_board_refresh_on_transition test_consuming_repo_scripts; do
  echo "=== $t ==="; bash "tests/$t.sh" >/tmp/$t.out 2>&1; echo "exit=$? ($(grep -c '^ok' /tmp/$t.out) ok, $(grep -c 'NOT OK' /tmp/$t.out) NOT OK)"
done
```

Expected: every `exit=0`, zero `NOT OK`. (`test_consuming_repo_scripts` and the board tests confirm the `docket-status.sh` rewire and the new lib did not regress the wider script family.)

- [ ] **Step 2: Confirm backward compatibility of the resolver**

Run: `bash -c 'cd <a-real-migrated-clone>; eval "$(scripts/docket-config.sh --export)"; echo "$DOCKET_MODE/$METADATA_WORKTREE"'` — the historical `eval` path must still work (shell/`%q` default). This is the regression guard for the Global Constraint "existing eval callers byte-unaffected." (Covered by `test_docket_config.sh`'s pre-existing eval assertions; this is a spot-check.)

- [ ] **Step 3: No commit** — verification only. Any failure loops back to the owning task.

---

## Self-Review

**Spec coverage:**
- Core decision 1 (one facade, no escape hatch) → Task 4 + sentinel invariant 4 (Task 5).
- Core decision 2 (config flows model-ward, raw stdout, never eval'd) → Task 1 (`--format plain`) + Task 4 (`env`).
- Core decision 3 (side effects as a plain op; `preflight` prints env; mid-run re-sync verb) → Task 2 + Task 4 (`preflight`).
- Core decision 4 (self-sufficiency; only `DOCKET_SCRIPTS_DIR`; pure renderers skip preflight) → Task 4: `preflight` is the sync verb; wrapped ops are pass-through taking explicit paths (routing boundary). **Interpretation recorded for ADR:** wrapped helper ops do NOT each run preflight internally — that would contradict "routing boundary, not a second implementation," double-sync after the agent's `preflight`, and misfire for primary-tree ops (`sync-integration-branch`, `cleanup-feature-branch`); the shared preflight is realized in `preflight` and in `docket-status` (spec point 6).
- Core decision 5 (exactly one canonical spelling) → Global Constraints + docket.md; the spelling is asserted where docket emits/documents it (contract prose).
- Core decision 6 (docket-status reuses shared preflight) → Task 3.
- Subcommand inventory (13 rows) → Task 4 dispatch + Task 5 table + sentinel invariant 1.
- Dispatch semantics (verbatim forwarding, exit/stderr passthrough, unknown→non-zero+list) → Task 4 tests (A)(B)(C).
- `env` output format (raw, no export, absolute paths, BOOTSTRAP present, abort→empty+non-zero) → Task 1 + Task 4 (D); **absolutization narrowed to `METADATA_WORKTREE`, disclosed + ADR** (the `*_DIR` root differs by consumer).
- Error handling (fail-closed, exit/stderr preserved) → Task 2 bootstrap cases + Task 4 passthrough.
- Verification: hermetic preflight tests → Task 2; dispatch tests → Task 4; env output tests → Task 1/4; inventory sentinel + mutation test → Task 5; docket-status shares preflight → Task 3; co-located contract → Task 5.

**Placeholder scan:** every code step contains complete Bash; no "TBD"/"add error handling"/"similar to Task N". The one prose-authored artifact (`scripts/docket.md`, Task 5 Step 3) is a documentation file whose required contents are fully enumerated, and whose structural correctness is locked by the sentinel + coverage tests.

**Type consistency:** `docket_preflight <scripts_dir>` is defined in Task 2 and consumed identically in Tasks 3 and 4. `--format plain|shell` is defined in Task 1 and consumed in Task 4 (`docket-config.sh --export --format plain`). `WRAPPED_OPS` is the single dispatch allowlist in Task 4 and the grep target in Task 5. Seam names (`GIT`, `CONFIG_EXPORT_CMD`, `SCRIPTS_DIR`) are consistent across the lib, the facade, and the tests.

**Known deviations to record as ADRs (Task 6 / review step 6):**
1. Facade dispatch is pure pass-through for wrapped helpers; the shared preflight is realized only in `preflight` + `docket-status` (not per-op) — the faithful reading of "routing boundary, not a second implementation."
2. `env`/`preflight` absolutize `METADATA_WORKTREE` only; `*_DIR` keys stay repo-relative subpaths (their absolute root differs by consumer) — a narrowing of "all path-valued keys absolute."
3. Raw presentation lives in `docket-config.sh --format plain` (single key list, DRY) rather than a facade-side re-emit — "the facade owns the presentation" realized as "the facade selects the plain presentation," avoiding a second, drift-prone key list.
