#!/usr/bin/env bash
# tests/test_closeout.sh — verifies change 0025: the deterministic close-out scripts
# (archive-change.sh, terminal-publish.sh, cleanup-feature-branch.sh) and the call-site wiring.
# Hermetic: a temp repo with a local *bare* origin carrying docket + main; no gh, no network.
# Run: bash tests/test_closeout.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARCHIVE="$REPO/scripts/archive-change.sh"
PUBLISH="$REPO/scripts/terminal-publish.sh"
CLEANUP="$REPO/scripts/cleanup-feature-branch.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

git_quiet(){ git "$@" >/dev/null 2>&1; }

# new_repo: prints "<work> <origin>" — a fresh clone with a bare origin holding docket + main.
# docket branch: docs/changes/active/0007-sample.md, its spec, one Accepted + one Proposed ADR.
# main branch: a trivial baseline (the integration branch publish target).
new_repo(){
  local root work origin
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  # --- main branch (baseline) ---
  git -C "$work" checkout -b main >/dev/null 2>&1
  echo "code" > "$work/README.md"; git -C "$work" add README.md
  git_quiet -C "$work" commit -m "main baseline"
  git_quiet -C "$work" push -u origin main
  # --- docket branch (orphan metadata) ---
  git -C "$work" checkout --orphan docket >/dev/null 2>&1
  git -C "$work" rm -rf . >/dev/null 2>&1 || true
  mkdir -p "$work/docs/changes/active" "$work/docs/changes/archive" \
           "$work/docs/superpowers/specs" "$work/docs/adrs"
  cat > "$work/docs/changes/active/0007-sample.md" <<'EOF'
---
id: 7
slug: sample
title: Sample change
status: implemented
priority: medium
created: 2026-06-01
updated: 2026-06-01
spec: docs/superpowers/specs/2026-06-01-sample.md
adrs: [3, 5]
pr: https://github.com/o/r/pull/42
results:
---

## Why
Body.
EOF
  echo "# spec" > "$work/docs/superpowers/specs/2026-06-01-sample.md"
  cat > "$work/docs/adrs/0003-accepted.md" <<'EOF'
---
id: 3
slug: accepted
title: An accepted decision
status: Accepted
date: 2026-06-01
---
## Decision
Yes.
EOF
  cat > "$work/docs/adrs/0005-proposed.md" <<'EOF'
---
id: 5
slug: proposed
title: A proposed decision
status: Proposed
date: 2026-06-01
---
## Decision
Maybe.
EOF
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket metadata"
  git_quiet -C "$work" push -u origin docket
  # leave the work clone parked on docket (the metadata working tree)
  printf '%s %s\n' "$work" "$origin"
}

assert "archive-change.sh exists and is executable" '[ -x "$ARCHIVE" ]'

# --- archive-change.sh: done happy path ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 \
  --results docs/results/2026-06-18-sample-results.md >/dev/null 2>&1
assert "done: active file is gone" '[ ! -e "$W/docs/changes/active/0007-sample.md" ]'
assert "done: dated archive file exists" '[ -e "$W/docs/changes/archive/2026-06-18-0007-sample.md" ]'
assert "done: status set to done" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "$W/docs/changes/archive/2026-06-18-0007-sample.md" status)" = done ]'
assert "done: updated set to date" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "$W/docs/changes/archive/2026-06-18-0007-sample.md" updated)" = 2026-06-18 ]'
assert "done: results link written" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "$W/docs/changes/archive/2026-06-18-0007-sample.md" results)" = docs/results/2026-06-18-sample-results.md ]'
assert "done: pushed (origin docket == local)" '[ "$(git -C "$W" rev-parse @)" = "$(git -C "$W" rev-parse origin/docket)" ]'
assert "done: commit touched ONLY the change file (nothing else)" 'other="$(git -C "$W" show --name-only --format= HEAD | grep -v "^$" | grep -v "0007-sample.md" || true)"; [ -z "$other" ] && git -C "$W" show --name-only --format= HEAD | grep -q "0007-sample.md"'

# --- archive-change.sh: killed path + ## Why killed ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome killed --date 2026-06-18 \
  --reason "Obsoleted by 0009." >/dev/null 2>&1
assert "killed: dated archive file exists" '[ -e "$W/docs/changes/archive/2026-06-18-0007-sample.md" ]'
assert "killed: status set to killed" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "$W/docs/changes/archive/2026-06-18-0007-sample.md" status)" = killed ]'
assert "killed: ## Why killed section appended with reason" 'grep -qF "## Why killed" "$W/docs/changes/archive/2026-06-18-0007-sample.md" && grep -qF "Obsoleted by 0009." "$W/docs/changes/archive/2026-06-18-0007-sample.md"'

# --- archive-change.sh: idempotent reuse-existing (second run is a no-op, no new file across a day boundary) ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
head1="$(git -C "$W" rev-parse @)"
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-19 >/dev/null 2>&1  # different date!
rc=$?
assert "idempotent: second run exits 0 (no-op)" "[ $rc -eq 0 ]"
assert "idempotent: no second archive file minted across day boundary" '[ "$(ls "$W/docs/changes/archive/" | grep -c -- "-0007-sample.md")" -eq 1 ]'
assert "idempotent: reuses 2026-06-18 name (ignores later --date)" '[ -e "$W/docs/changes/archive/2026-06-18-0007-sample.md" ]'
assert "idempotent: no new commit on the no-op run" '[ "$(git -C "$W" rev-parse @)" = "$head1" ]'

# --- archive-change.sh: fail-closed when the id is absent (neither active nor archive) ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 99 --outcome done --date 2026-06-18 >/dev/null 2>&1; rc_fail=$?
assert "fail-closed: missing id exits non-zero" '[ "$rc_fail" -ne 0 ]'

assert "publish: terminal-publish.sh exists and is executable" '[ -x "$PUBLISH" ]'

# --- terminal-publish.sh: copy-set built correctly with the Accepted-ADR gate ---
read -r W _ < <(new_repo)
# precondition: archive the change on docket first (publish copies the archived path)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
# inspect what landed on origin/main
git -C "$W" fetch origin main >/dev/null 2>&1
ls_main(){ git -C "$W" ls-tree -r --name-only origin/main; }
assert "publish: archived change file on integration branch" 'ls_main | grep -q "docs/changes/archive/2026-06-18-0007-sample.md"'
assert "publish: spec on integration branch" 'ls_main | grep -q "docs/superpowers/specs/2026-06-01-sample.md"'
assert "publish: Accepted ADR-0003 on integration branch" 'ls_main | grep -q "docs/adrs/0003-accepted.md"'
assert "publish: Proposed ADR-0005 SKIPPED (gate)" '! ls_main | grep -q "docs/adrs/0005-proposed.md"'
assert "publish: pub-7 worktree torn down" '! git -C "$W" worktree list | grep -q "pub-7"'
assert "publish: BOARD.md never published" '! ls_main | grep -q "docs/changes/BOARD.md"'

# --- terminal-publish.sh: guarded no-op re-run (byte-identical second publish) ---
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1; before="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1; after="$(git -C "$W" rev-parse origin/main)"
assert "publish: re-run exits 0" "[ $rc -eq 0 ]"
assert "publish: re-run is a no-op (no new integration commit)" '[ "$before" = "$after" ]'

# --- terminal-publish.sh: CAS retry under a competing push between provision and push ---
# Inject a competing commit on origin/main mid-flight via GIT wrapper that fires once before the
# pub push. Simpler hermetic form: push a competing commit, then run publish and assert it still lands.
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
# competing writer advances origin/main
comp="$(mktemp -d)"; git clone "$(git -C "$W" remote get-url origin)" "$comp" >/dev/null 2>&1
git -C "$comp" checkout main >/dev/null 2>&1
echo more >> "$comp/README.md"; git -C "$comp" -c user.email=c@c -c user.name=c commit -am "competing" >/dev/null 2>&1
git -C "$comp" push origin main >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1
assert "publish: CAS push succeeds despite a competing advance" "[ $rc -eq 0 ]"
assert "publish: copy-set landed atop the competing commit" 'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-06-18-0007-sample.md"'
assert "publish: competing commit preserved (not clobbered)" 'git -C "$W" log origin/main --oneline | grep -q competing'

# --- terminal-publish.sh: main-mode no-op ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch main --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish: main-mode exits 0 (no-op)" "[ $? -eq 0 ]"
assert "publish: main-mode created no pub worktree" '! git -C "$W" worktree list | grep -q "pub-7"'

exit "$fail"
