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
DOCKET_BASH_PATH="$(command -v bash)"
for candidate in "$DOCKET_BASH_PATH" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$candidate" ] || continue
  candidate_major="$(LC_ALL=C "$candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')"
  if [ -n "$candidate_major" ] && [ "$candidate_major" -ge 4 ]; then DOCKET_BASH_PATH="$candidate"; break; fi
done
export DOCKET_BASH_PATH
mkdir -p "$tmp/void/docket"
printf 'runtime:\n  bash: %s\n' "$DOCKET_BASH_PATH" > "$tmp/void/docket/config.yml"

# --- stub helper dir: each stub echoes its own basename + forwarded args, and can exit N -------
stub="$tmp/stub-scripts"; mkdir -p "$stub"
for h in docket-status board-refresh archive-change terminal-publish cleanup-feature-branch \
         github-mirror sync-integration-branch render-change-links render-adr-index \
         adr-checks board-checks reclaim-claims docket-config; do
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

# --- (D.5) render-learnings-index: pure renderer, facade dispatch to the script ---
LD="$tmp/learnings"; mkdir -p "$LD"
# Seed with a minimal valid finding file
cat > "$LD/example-finding.md" <<'FINDING'
---
slug: example-finding
hook: "A one-line rule."
topics: [testing]
changes: [1]
created: 2026-06-17
updated: 2026-07-16
promotion_state: retained
promoted_to:
---

## Apply
The rule.

## War story
- 2026-07-14 (#72, PR #79) — something happened.
FINDING
learning_out="$("$REPO/scripts/docket.sh" render-learnings-index --learnings-dir "$LD" 2>/dev/null)"
assert "facade dispatches render-learnings-index" 'printf "%s" "$learning_out" | grep -qF "# Learnings"'
rejection_out="$("$REPO/scripts/docket.sh" bogus-op 2>&1)"
assert "render-learnings-index is listed in the rejection help text" \
  'grep -qF "render-learnings-index" <<<"$rejection_out"'

# --- (E) preflight: side effects (worktree) THEN prints the env block -------
rm -rf "$work/.docket"
pf_out="$(cd "$work" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" preflight 2>/dev/null)"; pf_rc=$?
assert "preflight exits zero on a migrated repo" '[ "$pf_rc" -eq 0 ]'
assert "preflight created the metadata worktree" '[ -d "$work/.docket" ]'
assert "preflight prints the env block (BOOTSTRAP present)" 'printf "%s\n" "$pf_out" | grep -qxF "BOOTSTRAP=PROCEED"'

# --- (F) bootstrap verb: routes to docket-config.sh --bootstrap; the cell guard holds ---------
# CREATE_ORPHAN cell (fresh: no docket branch, no live planning surface on main) → creates the orphan.
fbare="$tmp/f.git"; fwork="$tmp/f"
git init --quiet --bare "$fbare"; git clone --quiet "$fbare" "$fwork" 2>/dev/null
git -C "$fwork" config user.email t@t.test; git -C "$fwork" config user.name Test
git -C "$fwork" checkout --quiet -b main; : > "$fwork/README.md"
git -C "$fwork" add README.md; git -C "$fwork" commit --quiet -m init; git -C "$fwork" push --quiet -u origin main
boot_out="$(cd "$fwork" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" bootstrap 2>/dev/null)"; boot_rc=$?
assert "bootstrap exits zero in the CREATE_ORPHAN cell" '[ "$boot_rc" -eq 0 ]'
assert "bootstrap emits BOOTSTRAP=PROCEED after the orphan create" 'printf "%s\n" "$boot_out" | grep -qxF -- "BOOTSTRAP=PROCEED"'
assert "bootstrap created + pushed the orphan docket branch" \
  'git -C "$fbare" rev-parse --verify --quiet refs/heads/docket >/dev/null'
assert "bootstrap seeded the managed .gitignore block" \
  'grep -qF -- "# docket:start" "$fwork/.gitignore"'
# a subsequent preflight now verdicts PROCEED (the repo is migrated)
pf_boot="$(cd "$fwork" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" preflight 2>/dev/null)"; pf_boot_rc=$?
assert "preflight after bootstrap exits zero" '[ "$pf_boot_rc" -eq 0 ]'
assert "preflight after bootstrap verdicts PROCEED" 'printf "%s\n" "$pf_boot" | grep -qxF -- "BOOTSTRAP=PROCEED"'

# STOP_MIGRATE cell (live planning surface on main, no docket branch) → cell guard: NO write, exits 0.
sbare="$tmp/s.git"; swork="$tmp/s"
git init --quiet --bare "$sbare"; git clone --quiet "$sbare" "$swork" 2>/dev/null
git -C "$swork" config user.email t@t.test; git -C "$swork" config user.name Test
git -C "$swork" checkout --quiet -b main; : > "$swork/README.md"
mkdir -p "$swork/docs/changes/active"; echo x > "$swork/docs/changes/active/0001-x.md"
git -C "$swork" add README.md docs/changes/active/0001-x.md
git -C "$swork" commit --quiet -m init; git -C "$swork" push --quiet -u origin main
smig_out="$(cd "$swork" && XDG_CONFIG_HOME="$tmp/void" bash "$FACADE" bootstrap 2>/dev/null)"; smig_rc=$?
# NOTE (spec §5 correction, verified 2026-07-14): the resolver reports the verdict and exits 0;
# fail-closed is preflight's job. The cell guard is about the WRITE, not the exit code.
assert "bootstrap in STOP_MIGRATE cell exits zero (resolver reports the verdict)" '[ "$smig_rc" -eq 0 ]'
assert "bootstrap in STOP_MIGRATE cell emits BOOTSTRAP=STOP_MIGRATE" \
  'printf "%s\n" "$smig_out" | grep -qxF -- "BOOTSTRAP=STOP_MIGRATE"'
assert "bootstrap in STOP_MIGRATE cell writes NO docket branch" \
  '! git -C "$sbare" rev-parse --verify --quiet refs/heads/docket >/dev/null'
assert "bootstrap in STOP_MIGRATE cell writes NO .gitignore" '[ ! -f "$swork/.gitignore" ]'

# ============================================================================
# Inventory sentinel (change 0068) — derive both sides by grep; never hand-list.
# ============================================================================
FSH="$REPO/scripts/docket.sh"; FMD="$REPO/scripts/docket.md"

# ops declared in docket.sh: the three verbs + the WRAPPED_OPS array value, tokenized.
sh_wrapped="$(sed -n 's/^WRAPPED_OPS="\(.*\)"/\1/p' "$FSH")"
sh_ops="$(printf 'preflight\nenv\nbootstrap\n%s\n' "$sh_wrapped" | tr ' ' '\n' | sed '/^$/d' | sort -u)"

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

# no escape-hatch op name (including pipe-combined case labels); never calls eval at all
for hatch in run exec shell eval; do
  assert "docket.sh dispatch has no '$hatch' operation arm" \
    '! grep -qE "(^|\|)[[:space:]]*'"$hatch"'[[:space:]]*(\)|\|)" "$FSH"'
done

# docket.sh must never invoke the eval builtin, anywhere — not just on caller args, and not
# laundered through an intermediate variable. Strip comments first (the word "eval" appears in
# docket.sh's own header comments) before scanning for an actual eval invocation.
code_no_comments="$(sed 's/#.*//' "$FSH")"
assert "docket.sh never calls the eval builtin" \
  '! printf "%s" "$code_no_comments" | grep -qE "(^|[;&|[:space:]])eval([[:space:]]|$)"'

# The dispatch `case` itself is the routable surface — the op set the other assertions derive from
# `WRAPPED_OPS` proves only "WRAPPED_OPS matches the doc", NOT "the dispatch matches the doc". A
# `case` arm hand-added OUTSIDE the WRAPPED_OPS loop (e.g. `deploy-secret) exec ...`) would route a
# name that is in neither `sh_ops` nor `md_ops`, so set-equality would still hold. Close that hole:
# assert the `case "$op"` block contains ONLY the known dispatch arms (the three verbs + the three
# meta-arms). Any hand-added arm reddens this. Derived by grep from the actual case block.
case_labels="$(sed -n '/^case "\$op" in/,/^esac/p' "$FSH" \
  | grep -oE '^  [^)]+\)' | sed -E 's/\)$//; s/^  //; s/[[:space:]]+$//' | sort -u)"
expected_labels="$(printf '%s\n' '-h|--help' '""' 'env' 'preflight' 'bootstrap' '*' | sort -u)"
assert "docket.sh dispatch case has ONLY the known arms (a hand-added op arm reddens)" \
  '[ "$case_labels" = "$expected_labels" ] || { echo "case=[$case_labels] expected=[$expected_labels]" >&2; false; }'

exit $fail
