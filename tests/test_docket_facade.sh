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
# assert the `case "$op"` block contains ONLY the known dispatch arms (the two verbs + the three
# meta-arms). Any hand-added arm reddens this. Derived by grep from the actual case block.
case_labels="$(sed -n '/^case "\$op" in/,/^esac/p' "$FSH" \
  | grep -oE '^  [^)]+\)' | sed -E 's/\)$//; s/^  //; s/[[:space:]]+$//' | sort -u)"
expected_labels="$(printf '%s\n' '-h|--help' '""' 'env' 'preflight' '*' | sort -u)"
assert "docket.sh dispatch case has ONLY the known arms (a hand-added op arm reddens)" \
  '[ "$case_labels" = "$expected_labels" ] || { echo "case=[$case_labels] expected=[$expected_labels]" >&2; false; }'

exit $fail
