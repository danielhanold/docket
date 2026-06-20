#!/usr/bin/env bash
# tests/test_sync_integration_branch.sh — verifies change 0029: the best-effort, FF-only
# sync-integration-branch.sh helper. Hermetic: a temp clone with a local *bare* origin holding
# main; no gh, no network. Run: bash tests/test_sync_integration_branch.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HELPER="$REPO/scripts/sync-integration-branch.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

git_quiet(){ git "$@" >/dev/null 2>&1; }   # silences the empty-bare-clone warning in fixtures

# new_repo: prints "<work> <origin>" — a fresh clone with a bare origin holding main@C0.
# C0 carries skills/sentinel.txt so an FF can be observed in the working tree.
new_repo(){
  local root origin work
  root="$(mktemp -d)"; root="$(cd "$root" && pwd -P)"   # macOS /var vs /private/var
  origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git -C "$work" checkout -b main >/dev/null 2>&1
  mkdir -p "$work/skills"; echo "v0" > "$work/skills/sentinel.txt"
  git -C "$work" add skills/sentinel.txt; git_quiet -C "$work" commit -m "C0 baseline"
  git_quiet -C "$work" push -u origin main
  printf '%s %s' "$work" "$origin"
}

# advance_origin WORK: push a new commit (C1, sentinel=v1) to origin WITHOUT moving WORK's
# local main — emulates origin advancing under a stale primary checkout. Uses a throwaway clone.
advance_origin(){
  local work="$1" origin tmp
  origin="$(git -C "$work" remote get-url origin)"
  tmp="$(mktemp -d)"; git_quiet clone "$origin" "$tmp/c"
  git -C "$tmp/c" config user.email t@t; git -C "$tmp/c" config user.name t
  git -C "$tmp/c" checkout main >/dev/null 2>&1
  echo "v1" > "$tmp/c/skills/sentinel.txt"
  git -C "$tmp/c" add skills/sentinel.txt; git_quiet -C "$tmp/c" commit -m "C1 advance"
  git_quiet -C "$tmp/c" push origin main
}

# --- Case 1: FF case — origin advanced, clone on main & clean → FF to origin tip ---
read -r W O < <(new_repo)
advance_origin "$W"
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
remote_tip="$(git -C "$W" rev-parse origin/main 2>/dev/null)"
sentinel="$(cat "$W/skills/sentinel.txt")"
assert "FF: exit 0"                       "[ $rc -eq 0 ]"
assert "FF: local advanced past C0"       "[ '$after' != '$before' ]"
assert "FF: local now equals origin tip"  "[ '$after' = '$remote_tip' ]"
assert "FF: working tree updated to v1"   "[ '$sentinel' = 'v1' ]"

# --- Case 2: dirty tree — uncommitted change blocks the FF even though origin advanced ---
read -r W O < <(new_repo)
advance_origin "$W"
echo "dirty" >> "$W/skills/sentinel.txt"        # uncommitted edit
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
assert "dirty: exit 0"                     "[ $rc -eq 0 ]"
assert "dirty: tip unchanged (no FF)"      "[ '$after' = '$before' ]"
assert "dirty: note mentions clean/dirty"  "printf '%s' \"\$out\" | grep -qiE 'clean|dirty|uncommitted'"

# --- Case 3: wrong branch — clone on a feature branch → skip even though origin advanced ---
read -r W O < <(new_repo)
advance_origin "$W"
git -C "$W" checkout -b feat/x >/dev/null 2>&1
mainref_before="$(git -C "$W" rev-parse main)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
mainref_after="$(git -C "$W" rev-parse main)"
cur="$(git -C "$W" symbolic-ref --short -q HEAD)"
assert "wrong-branch: exit 0"              "[ $rc -eq 0 ]"
assert "wrong-branch: still on feat/x"     "[ '$cur' = 'feat/x' ]"
assert "wrong-branch: main ref untouched"  "[ '$mainref_after' = '$mainref_before' ]"
assert "wrong-branch: note mentions branch" "printf '%s' \"\$out\" | grep -qiE 'branch|not on'"

# --- Case 4: non-FF divergence — local has a commit origin doesn't → skip, no merge commit ---
read -r W O < <(new_repo)
echo "local-only" > "$W/skills/sentinel.txt"
git -C "$W" add skills/sentinel.txt; git_quiet -C "$W" commit -m "C1prime local"
advance_origin "$W"                              # origin gets a DIFFERENT C1
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
parents="$(git -C "$W" rev-list --parents -n1 HEAD | wc -w | tr -d ' ')"
assert "non-FF: exit 0"                    "[ $rc -eq 0 ]"
assert "non-FF: tip unchanged"             "[ '$after' = '$before' ]"
assert "non-FF: no merge commit (single parent)" "[ '$parents' = '2' ]"   # 'sha parent' = 2 words ⇒ single parent

# --- Case 5: already current — origin not advanced → no-op ---
read -r W O < <(new_repo)
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
assert "current: exit 0"                   "[ $rc -eq 0 ]"
assert "current: tip unchanged"            "[ '$after' = '$before' ]"
assert "current: note mentions current"    "printf '%s' \"\$out\" | grep -qiE 'current|up.to.date|already'"

# --- Case 6: fetch failure — origin advanced then made unreachable → skip with note ---
read -r W O < <(new_repo)
advance_origin "$W"                              # origin is now ahead (C1)...
git -C "$W" remote set-url origin /nonexistent/path.git   # ...but unreachable
before="$(git -C "$W" rev-parse HEAD)"
out="$("$HELPER" --clone-dir "$W" --integration-branch main 2>&1)"; rc=$?
after="$(git -C "$W" rev-parse HEAD)"
assert "fetch-fail: exit 0"                "[ $rc -eq 0 ]"
assert "fetch-fail: tip unchanged (no FF)" "[ '$after' = '$before' ]"
assert "fetch-fail: note mentions fetch"   "printf '%s' \"\$out\" | grep -qiE 'fetch'"

# --- Case 7: usage error — missing required --integration-branch → exit 2 ---
read -r W O < <(new_repo)
out="$("$HELPER" --clone-dir "$W" 2>&1)"; rc=$?
assert "usage: missing --integration-branch exits 2" "[ $rc -eq 2 ]"

if [ "$fail" -eq 0 ]; then echo "ALL PASS"; else echo "FAILURES"; fi
exit "$fail"
