# Consuming-repo script resolution (`DOCKET_SCRIPTS_DIR`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docket's deterministic helper scripts reachable from any consuming repo by resolving every call through an absolute `DOCKET_SCRIPTS_DIR` env var that `install.sh` injects, with a loud failure when it is missing.

**Architecture:** A new idempotent injector script (`scripts/ensure-docket-env.sh`) writes `DOCKET_SCRIPTS_DIR` (the absolute path to the docket clone's own `scripts/`) into the user's shell profile (primary, re-sourced on every Bash-tool call) and Claude Code's user-level `settings.json` `env` (reinforcement). `install.sh` runs it (so re-running back-fills already-migrated clones); `migrate-to-docket.sh` points at it. Every skill body switches its bare `scripts/<name>.sh` calls to `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh`, and a test-suite static audit (the repo has no GitHub Actions CI) guards against regressions.

**Tech Stack:** POSIX/bash shell scripts, `jq` (already a dependency, per `ensure-claude-settings.sh`), `awk`/`sed`/`grep`, the repo's hand-rolled `tests/test_*.sh` harness (`assert` + `ok`/`no` helpers; run individually with `bash tests/<file>`).

## Global Constraints

- **Resolved call-site form (verbatim):** `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh` — the `:?` fails loud with the remedy when the var is unset.
- **The injected value** is the absolute path to the docket clone's `scripts/` directory (the live clone the skills are already symlinked from → zero drift). It is `scripts/ensure-docket-env.sh`'s own directory.
- **`DOCKET_` namespacing constraint:** every env var docket introduces is `DOCKET_`-namespaced (joins `DOCKET_MODE` / `DOCKET_INTEGRATION_BRANCH` / `DOCKET_HARNESS_ROOT`).
- **Shell floor:** zsh (`~/.zshenv`, always-sourced), bash (`~/.bashrc`), fish (`~/.config/fish/config.fish`, `set -gx`), POSIX `export` fallback (`~/.profile`) for any other/unknown shell.
- **Settings reinforcement target:** user-level `~/.claude/settings.json` `.env.DOCKET_SCRIPTS_DIR`, idempotent `jq` (the `ensure-claude-settings.sh` precedent). Claude Code only (the open per-harness question resolves to Claude-only — the harness-agnostic profile `export` is the actual guarantee).
- **Idempotency:** every write is a no-op on re-run; the profile write uses a marker block so a moved clone updates the value instead of duplicating it.
- **Prose vs. runnable rule (makes the drift-guard a clean grep):** a *runnable* invocation an agent executes → resolved form above; a *prose* mention naming a script → **bare basename** (e.g. `render-board.sh`), never `scripts/<name>.sh`. After this change no skill body contains a literal `scripts/<concrete-name>.sh` substring (placeholders like `scripts/<name>.sh` are exempt — they contain no `[a-z]` after the slash).
- **Test seams:** `HOME` (profile target dir), `DOCKET_HARNESS_ROOT` (settings.json root; default `$HOME`), `DOCKET_TARGET_SHELL` (force the profile flavor; default `basename "$SHELL"`). Tests MUST sandbox `HOME` so they never touch the developer's real profile.
- **No CI infra exists** (`.github/workflows/` is empty): the drift-guard ships as a `tests/` file, the de-facto gate — mirroring `tests/test_change_links_coverage.sh` and how `tests/test_sync_agents.sh` exercises `sync-agents.sh --check`.

---

### Task 1: `scripts/ensure-docket-env.sh` — the injector

**Files:**
- Create: `scripts/ensure-docket-env.sh`
- Test: `tests/test_ensure_docket_env.sh`

**Interfaces:**
- Consumes: nothing (foundational).
- Produces: an executable script that, when run, writes `DOCKET_SCRIPTS_DIR=<its own dir>` to the shell profile (selected by `DOCKET_TARGET_SHELL`/`$SHELL`) and to `${DOCKET_HARNESS_ROOT:-$HOME}/.claude/settings.json` `.env.DOCKET_SCRIPTS_DIR`. Honors seams `HOME`, `DOCKET_HARNESS_ROOT`, `DOCKET_TARGET_SHELL`. Idempotent.

- [ ] **Step 1: Write the failing test**

Create `tests/test_ensure_docket_env.sh`:

```bash
#!/usr/bin/env bash
# tests/test_ensure_docket_env.sh — run: bash tests/test_ensure_docket_env.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/ensure-docket-env.sh"
EXPECTED="$REPO/scripts"               # the script exports its own dir
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Each case runs in a sandbox HOME so the real profile is never touched.
run(){ # run <target_shell>  -> sets $H to the sandbox home
  H="$(mktemp -d)"
  HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL="$1" bash "$SCRIPT" >/dev/null 2>&1
}

# zsh -> ~/.zshenv export
run zsh
assert "zsh: writes ~/.zshenv"            '[ -f "$H/.zshenv" ]'
assert "zsh: export line present"         'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.zshenv"'
assert "zsh: marker block present"        'grep -qF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv"'

# bash -> ~/.bashrc export
run bash
assert "bash: writes ~/.bashrc export"    'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.bashrc"'

# fish -> ~/.config/fish/config.fish set -gx
run fish
assert "fish: writes config.fish set -gx" 'grep -qF "set -gx DOCKET_SCRIPTS_DIR \"$EXPECTED\"" "$H/.config/fish/config.fish"'

# unknown shell -> ~/.profile POSIX export fallback
run ksh
assert "other: POSIX export to ~/.profile" 'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.profile"'

# settings.json env (jq), preserving an existing key
H="$(mktemp -d)"; mkdir -p "$H/.claude"
printf '{"permissions":{"allow":["keep"]}}\n' > "$H/.claude/settings.json"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
assert "settings: env.DOCKET_SCRIPTS_DIR set" 'jq -e --arg v "$EXPECTED" ".env.DOCKET_SCRIPTS_DIR == \$v" "$H/.claude/settings.json" >/dev/null'
assert "settings: pre-existing key preserved" 'jq -e ".permissions.allow | index(\"keep\")" "$H/.claude/settings.json" >/dev/null'
assert "settings: still valid JSON"           'jq empty "$H/.claude/settings.json"'

# idempotent: a second run leaves exactly one marker block + unchanged settings
H="$(mktemp -d)"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
assert "idempotent: exactly one marker block" '[ "$(grep -cF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv")" = "1" ]'

# stale block (clone moved) is replaced, not duplicated
H="$(mktemp -d)"
printf '# >>> docket (DOCKET_SCRIPTS_DIR) >>>\nexport DOCKET_SCRIPTS_DIR="/old/path/scripts"\n# <<< docket (DOCKET_SCRIPTS_DIR) <<<\n' > "$H/.zshenv"
HOME="$H" DOCKET_HARNESS_ROOT="$H" DOCKET_TARGET_SHELL=zsh bash "$SCRIPT" >/dev/null 2>&1
assert "stale path replaced"               'grep -qF "export DOCKET_SCRIPTS_DIR=\"$EXPECTED\"" "$H/.zshenv"'
assert "stale path: old value gone"        '! grep -qF "/old/path/scripts" "$H/.zshenv"'
assert "stale path: still one block"       '[ "$(grep -cF ">>> docket (DOCKET_SCRIPTS_DIR) >>>" "$H/.zshenv")" = "1" ]'

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_ensure_docket_env.sh`
Expected: FAIL (script does not exist yet — every assert errors / NOT OK).

- [ ] **Step 3: Write the script**

Create `scripts/ensure-docket-env.sh`:

```bash
#!/usr/bin/env bash
# scripts/ensure-docket-env.sh — make docket's helper scripts reachable from any consuming
# repo by exporting DOCKET_SCRIPTS_DIR (absolute path to THIS scripts/ dir) into the user's
# shell profile (primary, re-sourced on every Bash-tool call) and Claude Code's user-level
# settings.json env (reinforcement, read at session start). Idempotent + standalone:
# install.sh runs it; re-running back-fills already-migrated clones (change 0034).
#
# DOCKET_SCRIPTS_DIR points at the live docket clone the skills are symlinked from -> zero
# drift. Skills resolve every helper as "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh,
# so a missing/incomplete install fails loud at the first call instead of silently degrading.
#
# Usage: bash scripts/ensure-docket-env.sh
# Seams (tests): HOME (profile target), DOCKET_HARNESS_ROOT (settings.json root; default $HOME),
#   DOCKET_TARGET_SHELL (force the profile flavor; default = basename "$SHELL").
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"   # this dir IS the value we export
VALUE="$HERE"
NAME="DOCKET_SCRIPTS_DIR"
MARK_OPEN="# >>> docket (DOCKET_SCRIPTS_DIR) >>>"
MARK_CLOSE="# <<< docket (DOCKET_SCRIPTS_DIR) <<<"
say(){ printf 'ensure-docket-env: %s\n' "$*"; }

# --- 1. shell-profile export (primary) ---------------------------------------
shell="${DOCKET_TARGET_SHELL:-$(basename "${SHELL:-sh}")}"
case "$shell" in
  zsh)  prof="$HOME/.zshenv";                  line="export $NAME=\"$VALUE\"" ;;
  bash) prof="$HOME/.bashrc";                  line="export $NAME=\"$VALUE\"" ;;
  fish) prof="$HOME/.config/fish/config.fish"; line="set -gx $NAME \"$VALUE\"" ;;
  *)    prof="$HOME/.profile";                 line="export $NAME=\"$VALUE\"" ;;   # POSIX fallback
esac
mkdir -p "$(dirname "$prof")"; touch "$prof"
# Idempotent marker block: strip any existing docket block, then append a fresh one
# (a moved clone updates the exported path instead of duplicating the block).
tmp="$(mktemp)"
awk -v o="$MARK_OPEN" -v c="$MARK_CLOSE" '
  $0==o {skip=1; next} $0==c {skip=0; next} !skip {print}
' "$prof" > "$tmp"
printf '%s\n%s\n%s\n' "$MARK_OPEN" "$line" "$MARK_CLOSE" >> "$tmp"
mv "$tmp" "$prof"
say "wrote $NAME -> $prof ($shell)"

# --- 2. Claude Code user-level settings.json env (reinforcement) --------------
HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
settings="$HARNESS_ROOT/.claude/settings.json"
if command -v jq >/dev/null 2>&1; then
  mkdir -p "$(dirname "$settings")"
  [ -f "$settings" ] || printf '{}\n' > "$settings"
  if jq empty "$settings" 2>/dev/null; then
    t="$(mktemp)"
    if jq --arg v "$VALUE" '.env //= {} | .env.DOCKET_SCRIPTS_DIR = $v' "$settings" > "$t"; then
      mv "$t" "$settings"; say "set env.$NAME -> ${settings#"$HARNESS_ROOT"/}"
    else rm -f "$t"; say "warning: could not update $settings"; fi
  else say "warning: $settings is not valid JSON — left unchanged"; fi
else
  say "warning: jq not found — wrote profile export only (settings.json env skipped)"
fi
```

Then make it executable:

```bash
chmod +x scripts/ensure-docket-env.sh
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_ensure_docket_env.sh`
Expected: every line `ok - ...`; exit 0.

- [ ] **Step 5: Commit**

```bash
git add scripts/ensure-docket-env.sh tests/test_ensure_docket_env.sh
git commit -m "feat(0034): ensure-docket-env.sh — inject DOCKET_SCRIPTS_DIR into shell profile + settings.json"
```

---

### Task 2: Wire `install.sh` to inject `DOCKET_SCRIPTS_DIR` (+ back-fill)

**Files:**
- Modify: `install.sh` (after the `sync-agents.sh` step, ~line 23)
- Test: `tests/test_install.sh` (add assertions; sandbox `HOME`)

**Interfaces:**
- Consumes: `scripts/ensure-docket-env.sh` from Task 1.
- Produces: an `install.sh` that injects `DOCKET_SCRIPTS_DIR` as its third primitive; re-running back-fills.

- [ ] **Step 1: Write the failing test**

Edit `tests/test_install.sh`. The existing test creates `mkdir -p "$tmp/.claude/skills"` and runs install with `DOCKET_HARNESS_ROOT="$tmp"`. Add `HOME="$tmp"` and `DOCKET_TARGET_SHELL=zsh` to BOTH `install.sh` invocations (so the profile write is sandboxed), then add assertions after the first run. Concretely, change the first run line from:

```bash
out="$(cd "$tmp" && DOCKET_HARNESS_ROOT="$tmp" bash "$REPO/install.sh" 2>&1)"; rc=$?
```

to:

```bash
out="$(cd "$tmp" && HOME="$tmp" DOCKET_HARNESS_ROOT="$tmp" DOCKET_TARGET_SHELL=zsh bash "$REPO/install.sh" 2>&1)"; rc=$?
```

and the idempotent second run likewise (add `HOME="$tmp" ... DOCKET_TARGET_SHELL=zsh`). Then add these assertions after the existing `install.sh ran sync-agents.sh` assertion:

```bash
assert "install.sh injected DOCKET_SCRIPTS_DIR into the shell profile" \
  'grep -qF "export DOCKET_SCRIPTS_DIR=\"$REPO/scripts\"" "$tmp/.zshenv"'
assert "install.sh injected env.DOCKET_SCRIPTS_DIR into settings.json" \
  'jq -e --arg v "$REPO/scripts" ".env.DOCKET_SCRIPTS_DIR == \$v" "$tmp/.claude/settings.json" >/dev/null'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_install.sh`
Expected: the two new assertions are NOT OK (install.sh does not run the injector yet); the existing assertions still pass.

- [ ] **Step 3: Add the injector step to `install.sh`**

In `install.sh`, after the `sync-agents.sh` block (the two lines that echo + run sync-agents), insert:

```bash
echo "==> ensure-docket-env.sh (export DOCKET_SCRIPTS_DIR)"
bash "$SCRIPT_DIR/scripts/ensure-docket-env.sh"
```

(Place it before the final `echo "docket: install complete"`.) Also extend the header comment's numbered list with a third primitive:

```
#   3. ensure-docket-env.sh — export DOCKET_SCRIPTS_DIR so the skills can reach scripts/ from any
#                             consuming repo (re-run back-fills already-migrated clones)
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_install.sh`
Expected: all `ok - ...`; exit 0.

- [ ] **Step 5: Commit**

```bash
git add install.sh tests/test_install.sh
git commit -m "feat(0034): install.sh runs ensure-docket-env.sh (inject + back-fill DOCKET_SCRIPTS_DIR)"
```

---

### Task 3: Point `migrate-to-docket.sh` at `install.sh` for script reachability

**Files:**
- Modify: `migrate-to-docket.sh` (the "Next steps" `cat <<EOF` block, ~lines 385-403)
- Test: `tests/test_ensure_docket_env.sh` (append a migrate-prose sentinel)

**Rationale:** The injection is machine-level (shell profile + user settings), owned by `install.sh`. Per the spec's "ensure install.sh has run (**or point at it**)", migrate keeps its side effects repo-local and *points* at `install.sh` rather than editing the user's shell profile itself. A freshly-migrated clone becomes script-reachable on the next `install.sh` run (which also back-fills).

**Interfaces:**
- Consumes: the `install.sh` behavior from Task 2 (the thing it points at).
- Produces: a migrate "Next steps" section that tells the user to run `install.sh` for script reachability.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_ensure_docket_env.sh`, before the final `exit $fail`:

```bash
# migrate-to-docket.sh points the user at install.sh for script reachability (DOCKET_SCRIPTS_DIR)
MIG="$REPO/migrate-to-docket.sh"
assert "migrate next-steps names DOCKET_SCRIPTS_DIR"  'grep -qF "DOCKET_SCRIPTS_DIR" "$MIG"'
assert "migrate next-steps points at install.sh"      'grep -qE "install\.sh" "$MIG"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_ensure_docket_env.sh`
Expected: the two new asserts are NOT OK (migrate does not mention `DOCKET_SCRIPTS_DIR`/`install.sh` yet).

- [ ] **Step 3: Add the pointer to migrate's "Next steps"**

In `migrate-to-docket.sh`, inside the final `cat <<EOF` "Next steps" block, add a bullet (after the existing `.docket.yml is OPTIONAL` bullet):

```
    - Make docket's helper scripts reachable from THIS repo: run docket's installer once on this
      machine —
          bash $MIGRATE_DIR/install.sh
      It exports DOCKET_SCRIPTS_DIR (the path to docket's scripts/) into your shell profile and
      Claude Code settings, so the skills can run their deterministic helpers here. Re-running it
      is safe and back-fills any already-migrated clone.
```

(`$MIGRATE_DIR` is already in scope in that heredoc — it interpolates to the docket clone path.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_ensure_docket_env.sh`
Expected: all `ok - ...`; exit 0.

- [ ] **Step 5: Commit**

```bash
git add migrate-to-docket.sh tests/test_ensure_docket_env.sh
git commit -m "feat(0034): migrate-to-docket.sh points at install.sh for DOCKET_SCRIPTS_DIR reachability"
```

---

### Task 4: Convention — authoritative `DOCKET_SCRIPTS_DIR` definition + rewrite convention call sites/prose

**Files:**
- Modify: `skills/docket-convention/SKILL.md`
- Modify: `skills/docket-convention/github-board-mirror.md`
- Test: `tests/test_docket_config.sh` (update the two convention/skill needles + add convention-def asserts)

**Scope of `scripts/` occurrences in these two files** (from the reconcile inventory): SKILL.md lines 36 (two: a bold descriptive + a runnable `eval`), 125, 211, 213 (three in the script-family list), 227 (two), 233; github-board-mirror.md line 7. Apply the prose-vs-runnable rule to each.

**Interfaces:**
- Consumes: the resolved-call-site form + namespacing constraint (Global Constraints).
- Produces: the authoritative convention text every other skill's rewrite (Task 5) refers to; a convention body with zero `scripts/<concrete-name>.sh` substrings.

- [ ] **Step 1: Write the failing test**

Edit `tests/test_docket_config.sh`. Change the convention needle (line ~215) and the per-skill needle (line ~219) from `scripts/docket-config.sh` to `/docket-config.sh`:

```bash
assert "convention names docket-config.sh" 'grep -qF "/docket-config.sh" "$CONV"'
for s in docket-implement-next docket-status docket-new-change docket-groom-next \
         docket-finalize-change docket-adr docket-auto-groom; do
  f="$REPO/skills/$s/SKILL.md"
  assert "$s Step 0 invokes docket-config.sh" 'grep -qF "/docket-config.sh" "$f"'
done
```

Then add, immediately after the `convention names docket-config.sh` assert, two convention-definition asserts (each anchored to a unique phrase it owns, per learnings #15/#2):

```bash
assert "convention defines the DOCKET_SCRIPTS_DIR resolved form" \
  'grep -qF "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}" "$CONV"'
assert "convention documents DOCKET_ namespacing" \
  'grep -qiF "DOCKET_-namespaced" "$CONV"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_config.sh`
Expected: `convention names docket-config.sh` and the two new asserts are NOT OK (convention not yet rewritten); the per-skill loop is still satisfied by the OLD `scripts/docket-config.sh` substring **only if** the substring `/docket-config.sh` is present — note the old form `scripts/docket-config.sh` *does* contain `/docket-config.sh`, so the per-skill asserts stay green here and flip meaning in Task 5. (That is fine: Task 4 only needs the three convention asserts to drive its work.)

- [ ] **Step 3: Add the authoritative definition to the convention**

In `skills/docket-convention/SKILL.md`, in the Configuration section, immediately AFTER the paragraph that ends `...the script is the single implementation the skills run instead of re-deriving it each session.` (the paragraph currently containing `eval "$(scripts/docket-config.sh --export)"`), insert a new paragraph:

```markdown
**Reaching the helper scripts (`DOCKET_SCRIPTS_DIR`).** Every helper script this convention names lives in the docket clone's `scripts/` directory, NOT in the consuming repo. A skill resolves each as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh` — `DOCKET_SCRIPTS_DIR` is the absolute path to that directory, injected into the shell profile (re-sourced on every Bash-tool call, so it reaches dispatched subagents) and Claude Code's user-level `settings.json` `env` by `install.sh`; re-running `install.sh` back-fills any already-migrated clone. Pointing at the live clone the skills are symlinked from keeps scripts and skills version-matched (zero drift). The `:?` makes a missing/incomplete install **fail loud** at the first call instead of silently degrading to hand-worked operations. Every env var docket introduces is **`DOCKET_`-namespaced** (joining `DOCKET_MODE` / `DOCKET_INTEGRATION_BRANCH` / `DOCKET_HARNESS_ROOT`) to avoid collisions in the user's shared shell. In prose this convention names a script by basename (e.g. `render-board.sh`); a runnable invocation always uses the resolved form.
```

- [ ] **Step 4: Rewrite the convention's own `scripts/` occurrences**

Apply the prose-vs-runnable rule throughout `skills/docket-convention/SKILL.md`:

- Line 36 — the runnable example `eval "$(scripts/docket-config.sh --export)"` → ``eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"``; the bold descriptive `**`scripts/docket-config.sh --export`**` → `**`docket-config.sh --export`**` (basename).
- Line 125 — `rendered by `scripts/render-change-links.sh`` → `rendered by `render-change-links.sh`` (basename prose).
- Line 211 — `owned by the deterministic `scripts/github-mirror.sh`` → ``github-mirror.sh`` (basename).
- Line 213 (script-family list) — `scripts/render-board.sh` → `render-board.sh`, `scripts/github-mirror.sh` → `github-mirror.sh`, `scripts/render-change-links.sh` → `render-change-links.sh` (all basenames; this is a prose list).
- Line 227 — `evaluated by the same `scripts/docket-config.sh`` → ``docket-config.sh`` (basename); the runnable `(`scripts/docket-config.sh --bootstrap`...)` → ``("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --bootstrap`...)``.
- Line 233 — `the best-effort, FF-only `scripts/sync-integration-branch.sh`` → ``sync-integration-branch.sh`` (basename).

In `skills/docket-convention/github-board-mirror.md`:

- Line 7 — `owned by the deterministic `scripts/github-mirror.sh`` → ``github-mirror.sh`` (basename).

Verify no concrete `scripts/<name>.sh` remains in either file:

```bash
grep -nE 'scripts/[a-z][a-z0-9-]*\.sh' skills/docket-convention/SKILL.md skills/docket-convention/github-board-mirror.md
```
Expected: no output.

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: all `ok`; exit 0. (The `grep -nE` from Step 4 prints nothing.)

- [ ] **Step 6: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-convention/github-board-mirror.md tests/test_docket_config.sh
git commit -m "feat(0034): convention defines DOCKET_SCRIPTS_DIR resolution + namespacing; basename prose"
```

---

### Task 5: Rewrite the 7 operating-skill bodies + add the drift-guard + update broken wiring sentinels

**Files:**
- Modify (skill bodies): `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-adr/SKILL.md`
- Create (drift-guard): `tests/test_consuming_repo_scripts.sh`
- Modify (broken wiring sentinels, in lockstep): `tests/test_render_board.sh`, `tests/test_board_checks.sh`, `tests/test_adr_checks.sh`, `tests/test_render_adr_index.sh`, `tests/test_change_links_coverage.sh`, `tests/test_closeout.sh`

**Interfaces:**
- Consumes: the convention's resolved-call-site form (Task 4).
- Produces: every skill body resolving helper scripts via `DOCKET_SCRIPTS_DIR`; a repo-wide audit guarding it.

- [ ] **Step 1: Write the drift-guard test (failing) + update the broken wiring sentinels**

Create `tests/test_consuming_repo_scripts.sh`:

```bash
#!/usr/bin/env bash
# tests/test_consuming_repo_scripts.sh — the DOCKET_SCRIPTS_DIR drift-guard (change 0034).
# The repo has no GitHub Actions CI; this test-suite file is the de-facto gate, mirroring
# test_change_links_coverage.sh and how test_sync_agents.sh exercises sync-agents.sh --check.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }

# (1) STATIC AUDIT: no skill body invokes a bare, CWD-relative scripts/<concrete-name>.sh.
#     A concrete name has a lowercase letter right after the slash; the placeholder
#     scripts/<name>.sh (literal "<") is intentionally exempt.
audit_fail=0
while IFS= read -r f; do
  hits="$(grep -nE 'scripts/[a-z][a-z0-9-]*\.sh' "$f" || true)"
  if [ -n "$hits" ]; then
    no "bare scripts/<name>.sh in ${f#"$REPO"/}"; printf '%s\n' "$hits"; audit_fail=1
  fi
done < <(find "$REPO/skills" -name '*.md' | sort)
[ "$audit_fail" = 0 ] && ok "no skill body uses a bare scripts/<name>.sh path"

# (2) RESOLUTION: from a foreign CWD with DOCKET_SCRIPTS_DIR set, the resolved form locates a helper.
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
got="$(cd "$tmp" && DOCKET_SCRIPTS_DIR="$REPO/scripts" bash -c 'echo "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh')"
[ -x "$got" ] && ok "DOCKET_SCRIPTS_DIR resolves docket-config.sh from a foreign CWD" \
              || no "DOCKET_SCRIPTS_DIR resolves docket-config.sh from a foreign CWD ($got)"

# (3) FAIL-LOUD: unset DOCKET_SCRIPTS_DIR -> the :? form exits non-zero with the remedy.
err="$(cd "$tmp" && bash -c 'echo "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh' 2>&1)"; rc=$?
[ "$rc" -ne 0 ] && ok "unset DOCKET_SCRIPTS_DIR fails loud (non-zero exit)" \
               || no "unset DOCKET_SCRIPTS_DIR fails loud (non-zero exit)"
printf '%s' "$err" | grep -qF "run docket/install.sh" && ok "fail-loud message carries the remedy" \
                                                       || no "fail-loud message carries the remedy"

exit $fail
```

Then update the broken wiring sentinels (each changes its grep needle from `scripts/<name>.sh` to `/<name>.sh` — the resolved invocation `}"/<name>.sh` contains `/<name>.sh`; a bare-basename prose mention does not, so the assertion still verifies a real invocation and stays non-vacuous). Make exactly these edits:

- `tests/test_render_board.sh` (~line 240): `"scripts/render-board.sh"` → `"/render-board.sh"`.
- `tests/test_board_checks.sh` (~line 321): `"scripts/board-checks.sh"` → `"/board-checks.sh"`.
- `tests/test_adr_checks.sh` (~line 138): `"scripts/adr-checks.sh"` → `"/adr-checks.sh"`.
- `tests/test_render_adr_index.sh` (~line 178): `"scripts/render-adr-index.sh"` → `"/render-adr-index.sh"`.
- `tests/test_change_links_coverage.sh` (~line 17): `'scripts/render-change-links.sh'` → `'/render-change-links.sh'`.
- `tests/test_closeout.sh` (~lines 265-285): in the eight sentinels that grep skill bodies, change `"scripts/archive-change.sh"` → `"/archive-change.sh"`, `"scripts/terminal-publish.sh"` → `"/terminal-publish.sh"`, `"scripts/cleanup-feature-branch.sh"` → `"/cleanup-feature-branch.sh"`. Leave the assertions that already match `terminal-publish\.sh --adr`, `## Terminal publish (docket-mode)`, `git mv .*active/`, and `git worktree add -B .?pub-adr` UNCHANGED (those needles do not include the `scripts/` prefix).

Do NOT touch `tests/test_ensure_claude_settings.sh:109` (`scripts/ensure-claude-settings.sh` there greps README.md, a doc path into the docket clone — it correctly stays).

- [ ] **Step 2: Run to verify the drift-guard fails (and the rewrite is needed)**

Run: `bash tests/test_consuming_repo_scripts.sh`
Expected: the static audit is NOT OK — it lists bare `scripts/<name>.sh` hits in the 7 operating skills (docket-convention is already clean from Task 4). Resolution + fail-loud asserts pass.

- [ ] **Step 3: Rewrite the 7 operating-skill bodies**

For each skill body, replace every **runnable** `scripts/<name>.sh` invocation with `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh`, and every **prose** mention with the bare basename. The concrete occurrences (from the reconcile inventory):

- `skills/docket-auto-groom/SKILL.md`: line 25 (`eval "$(scripts/docket-config.sh --export)"` → resolved; the second `scripts/docket-config.sh --bootstrap` → resolved), line 48 (`scripts/render-change-links.sh ...` → resolved).
- `skills/docket-new-change/SKILL.md`: line 29 (eval + bootstrap → resolved), line 39 (render-change-links → resolved), line 61 (archive-change → resolved), line 62 (terminal-publish → resolved), line 64 (two: `scripts/archive-change.sh --outcome killed` → resolved; `scripts/terminal-publish.sh` prose "is a no-op" → basename `terminal-publish.sh`).
- `skills/docket-groom-next/SKILL.md`: line 29 (eval + bootstrap → resolved), line 66 (render-change-links → resolved).
- `skills/docket-adr/SKILL.md`: line 26 (eval + bootstrap → resolved), lines 57 & 65 (terminal-publish in fenced blocks → resolved), line 77 (render-adr-index runnable → resolved), line 80 (two: runnable `scripts/render-adr-index.sh ... > ...` → resolved), line 85 (adr-checks runnable → resolved).
- `skills/docket-finalize-change/SKILL.md`: line 25 (eval + bootstrap → resolved), line 77 (archive-change fenced → resolved), line 83 (render-change-links → resolved), line 88 (cleanup-feature-branch inline invocation → resolved), line 92 (sync-integration-branch inline → resolved), line 218 (prose "executed by `scripts/terminal-publish.sh`" → basename), line 219 (two: prose "handled by `scripts/terminal-publish.sh --adr <NN>`" → resolved form, since it shows a runnable `--adr` shape; keep `terminal-publish.sh --adr` matchable), line 235 (prose "`scripts/archive-change.sh` (the same invocation...)" → basename `archive-change.sh`), line 236 (runnable `scripts/terminal-publish.sh --id ...` → resolved).
- `skills/docket-status/SKILL.md`: line 39 (eval + bootstrap → resolved), line 48 (render-board runnable → resolved), line 57 (prose "re-run `scripts/render-board.sh`" → resolved, it is an instruction to run), line 61 (github-mirror "Invoke the deterministic `scripts/github-mirror.sh`" → resolved), line 67 (prose "`scripts/render-board.sh` is the executable source" → basename `render-board.sh`), line 143 (render-change-links → resolved), line 149 (archive-change → resolved), line 150 (terminal-publish → resolved), line 154 (cleanup-feature-branch → resolved), line 158 (sync-integration-branch → resolved), line 168 (prose "Mechanical checks → `scripts/board-checks.sh`" → basename `board-checks.sh`), line 171 (runnable `scripts/board-checks.sh --changes-dir ...` → resolved).
- `skills/docket-implement-next/SKILL.md`: line 27 (eval + bootstrap → resolved), line 52 (archive-change → resolved), line 53 (terminal-publish → resolved), line 54 (cleanup-feature-branch → resolved), line 56 (three: `scripts/archive-change.sh --outcome killed` → resolved; `scripts/terminal-publish.sh` prose "is a no-op" → basename; `scripts/cleanup-feature-branch.sh --slug` runnable → resolved), line 68 (render-change-links → resolved), line 76 (render-change-links → resolved), line 90 (render-change-links → resolved).

Guidance for "runnable vs prose": if the agent would copy the token to a shell and run it (inside `eval "$(...)"`, a fenced ```` ``` ```` block, or an inline "invoke/run `…`" instruction), it is **runnable** → resolved form. If the sentence merely names the script ("X is the executable source", "owned by X", "X is a no-op in main-mode", "handled by X"), it is **prose** → basename. When in doubt, prefer the **resolved** form for anything that looks like a command and **basename** for a noun reference — both eliminate the `scripts/<name>.sh` substring, so the audit passes either way; the distinction is only readability.

After editing, confirm zero concrete bare paths remain across all skills:

```bash
grep -rnE 'scripts/[a-z][a-z0-9-]*\.sh' skills/
```
Expected: no output.

- [ ] **Step 4: Run the drift-guard, the updated sentinels, and the full suite**

```bash
bash tests/test_consuming_repo_scripts.sh
bash tests/test_render_board.sh
bash tests/test_board_checks.sh
bash tests/test_adr_checks.sh
bash tests/test_render_adr_index.sh
bash tests/test_change_links_coverage.sh
bash tests/test_closeout.sh
bash tests/test_docket_config.sh
```
Expected: every file exits 0 / all `ok`. (`test_docket_config.sh`'s per-skill loop now genuinely asserts the resolved `/docket-config.sh` invocation.)

- [ ] **Step 5: Commit**

```bash
git add skills/ tests/test_consuming_repo_scripts.sh tests/test_render_board.sh \
        tests/test_board_checks.sh tests/test_adr_checks.sh tests/test_render_adr_index.sh \
        tests/test_change_links_coverage.sh tests/test_closeout.sh
git commit -m "feat(0034): skills resolve helpers via DOCKET_SCRIPTS_DIR; add drift-guard; update wiring sentinels"
```

---

### Task 6: Full-suite regression sweep

**Files:** none (verification only).

**Interfaces:** Consumes everything above.

- [ ] **Step 1: Run the entire test suite**

```bash
for t in tests/test_*.sh; do
  printf '\n=== %s ===\n' "$t"
  bash "$t" || echo "!!! FAILED: $t"
done
```

- [ ] **Step 2: Confirm green + clean stderr**

Expected: every test file ends `PASS` / all `ok` and no `!!! FAILED` line. Per learnings #19/#22, a green run should also leave clean stderr — scan the output for unexpected warnings. Investigate and fix any failure before proceeding (a sentinel that broke means a `scripts/<name>.sh` occurrence was missed or a needle update was wrong).

- [ ] **Step 3: Commit (only if a fix was needed)**

```bash
git add -A
git commit -m "fix(0034): address regression-sweep findings"
```

---

## Self-Review (performed during planning)

**Spec coverage** — every spec touch-point maps to a task:
- `install.sh` injects `DOCKET_SCRIPTS_DIR` (profile + settings env, idempotent, back-fill) → Tasks 1+2.
- Every skill body switches to the resolved form → Tasks 4 (convention) + 5 (7 skills).
- `migrate-to-docket.sh` ensures reachability → Task 3 (points at `install.sh`).
- Convention documents `DOCKET_SCRIPTS_DIR` + `DOCKET_` namespacing → Task 4.
- CI/test drift-guard (resolves + no bare path) → Task 5 (`tests/test_consuming_repo_scripts.sh`), realized as a test-suite gate (no GitHub Actions exists; per reconcile).
- Tests: consuming-repo resolution, multi-shell profile syntax, idempotent re-runs → Tasks 1 (script) + 5 (resolution/fail-loud).
- Open question (per-harness settings env) → resolved to Claude-Code-only (Global Constraints), documented; the harness-agnostic profile `export` is the guarantee.

**Deferred to follow-up #37 (NOT in scope here):** relocating the per-skill manual-fallback prose into sibling files. This change leaves that prose in place and only switches the call-site form.

**Placeholder scan:** every code/edit step shows concrete content or an exact needle change; no TBD/TODO.

**Type/name consistency:** `DOCKET_SCRIPTS_DIR`, the resolved form `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh`, the marker strings, and the seams (`HOME`, `DOCKET_HARNESS_ROOT`, `DOCKET_TARGET_SHELL`) are used identically across all tasks.

## Notes for the ADR (Step 6 of docket-implement-next)

Record one ADR for the **consuming-repo script-resolution contract**: `DOCKET_SCRIPTS_DIR` env var; profile-`export` primary + user-`settings.json` `env` reinforcement; fail-loud `:?`; `DOCKET_` namespacing; prose-basename / runnable-resolved convention. Relates to ADR-0012 (script-vs-model boundary — this restores that layer's reachability).
