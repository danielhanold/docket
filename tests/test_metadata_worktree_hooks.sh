#!/usr/bin/env bash
# tests/test_metadata_worktree_hooks.sh — change 0063: disable-worktree-hooks.sh makes commits in a
# docket-owned worktree skip the repo's SHARED git hooks — worktree-scoped, idempotent, not global —
# without disabling hooks anywhere else. Hermetic: throwaway repo + an always-failing common
# pre-commit hook; ambient user/system git config ignored. Run: bash tests/test_metadata_worktree_hooks.sh
set -uo pipefail
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null   # no ambient core.hooksPath leaks in
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HELPER="$REPO/scripts/disable-worktree-hooks.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# setup: prints the main worktree path of a fresh repo that has (a) a second docket-owned worktree
# at .docket on branch `docket`, and (b) an always-failing pre-commit hook in the COMMON hooks dir
# (shared by every worktree). No helper applied yet.
setup(){
  local root work hooks
  root="$(mktemp -d)"; root="$(cd "$root" && pwd -P)"
  work="$root/work"
  git init -q "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git -C "$work" config commit.gpgsign false
  echo root > "$work/f.txt"; git -C "$work" add f.txt; git -C "$work" commit -qm C0
  hooks="$(cd "$work" && cd "$(git rev-parse --git-common-dir)" && pwd -P)/hooks"
  mkdir -p "$hooks"; printf '#!/bin/sh\nexit 1\n' > "$hooks/pre-commit"; chmod +x "$hooks/pre-commit"
  git -C "$work" worktree add -q "$work/.docket" -b docket
  printf '%s' "$work"
}

n=0
# try_commit DIR → sets $RC to the exit code of a commit attempt in worktree DIR (unique file each
# call). NOTE deviation from the brief: try_commit is called as a plain statement (not inside a
# $(...) command substitution) and its result read back via $RC, rather than `echo`d from within the
# substitution. `n=$((n+1))` inside a $(...) command substitution runs in a forked subshell, so the
# increment never survives back to the parent shell — every call would reuse n=1 and thus the same
# filename c1.txt, making later calls in the same worktree no-op commits (nothing changed) rather
# than genuine hook-block/hook-skip outcomes. This preserves the brief's intent (unique file each
# call, real exit-code assertion) without weakening any assertion.
try_commit(){
  n=$((n+1))
  ( cd "$1" && printf '%s\n' "$n" > "c$n.txt" && git add "c$n.txt" && git commit -qm "c$n" ) >/dev/null 2>&1
  RC=$?
}

# --- Case 1: the hook is real and active (main-worktree commit FAILS) ---
W="$(setup)"
try_commit "$W"
assert "hook active: main-worktree commit fails" "[ \"\$RC\" -ne 0 ]"

# --- Case 2: after the helper, a .docket commit SUCCEEDS (hook skipped) ---
"$HELPER" --worktree "$W/.docket" >/dev/null 2>&1
assert "helper exit 0"                            "[ $? -eq 0 ]"
try_commit "$W/.docket"
assert "skip: .docket commit succeeds"            "[ \"\$RC\" -eq 0 ]"

# --- Case 3: worktree-scoped, not global (main-worktree commit STILL fails) ---
try_commit "$W"
assert "scoped: main-worktree commit still fails" "[ \"\$RC\" -ne 0 ]"

# --- Case 4: idempotent — a second run is a clean no-op, single hooksPath entry ---
"$HELPER" --worktree "$W/.docket" >/dev/null 2>&1; rc=$?
count="$(git -C "$W/.docket" config --worktree --get-all core.hooksPath | wc -l | tr -d ' ')"
assert "idempotent: second run exit 0"            "[ $rc -eq 0 ]"
assert "idempotent: single core.hooksPath value"  "[ \"$count\" -eq 1 ]"
try_commit "$W/.docket"
assert "idempotent: .docket commit still skips"   "[ \"\$RC\" -eq 0 ]"

# --- Case 5: non-vacuous — WITHOUT the helper, a fresh .docket commit fails ---
W2="$(setup)"
try_commit "$W2/.docket"
assert "non-vacuous: unpatched .docket commit fails" "[ \"\$RC\" -ne 0 ]"

# --- Case 6: relocation success path — a deliberate non-default core.bare in COMMON config is
# relocated to the main worktree's per-worktree config, not left stranded, and the helper still
# succeeds end-to-end (hooks still skipped in .docket afterward).
W3="$(setup)"
git -C "$W3" config --local core.bare true
"$HELPER" --worktree "$W3/.docket" >/dev/null 2>&1; rc=$?
assert "relocate: helper exits 0"                    "[ $rc -eq 0 ]"
common_bare="$(git -C "$W3" config --local --get core.bare 2>/dev/null || true)"
assert "relocate: common core.bare no longer set"    "[ -z \"$common_bare\" ]"
worktree_bare="$(git -C "$W3" config --worktree --get core.bare 2>/dev/null || true)"
assert "relocate: per-worktree core.bare == true"    "[ \"$worktree_bare\" = true ]"
try_commit "$W3/.docket"
assert "relocate: .docket commit still succeeds"     "[ \"\$RC\" -eq 0 ]"

if [ "$fail" -eq 0 ]; then echo "ALL PASS"; else echo "FAILURES"; fi
exit "$fail"
