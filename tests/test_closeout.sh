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

# Joins backslash-line-continuations so a multi-line shell invocation collapses to one logical
# line (matches real shell continuation semantics) — used by the 0064 --enabled gate sentinel
# below so a wrapped invocation is inspected as a whole, not line-by-line. Plain awk (not a
# GNU-sed N/branch idiom) so it behaves identically under BSD sed (macOS) and GNU sed (Linux CI).
join_continuations(){
  awk '
    {
      line = $0
      if (sub(/\\[[:space:]]*$/, "", line)) {
        buf = buf line " "
        next
      }
      print buf line
      buf = ""
    }
    END { if (buf != "") print buf }
  ' "$1"
}

# Every REAL terminal-publish.sh call site (skills/ prose + shell code) that supplies --id or
# --adr must also supply --enabled, regardless of flag order or line-wrapping. Scans skills/
# (all files), scripts/*.sh, and the repo-root *.sh entry points (migrate-to-docket.sh,
# install.sh — no call sites today, but a future one must not slip through un-gated). Excludes
# scripts/*.md contracts (they document the CLI generically), excludes terminal-publish.sh's own
# usage header/--help text (not a call site, and its own [--enabled true|false] sits on a
# separate non-continued comment line), and excludes tests/ (which deliberately exercises the
# back-compat default-omitted-enabled path).
#
# Evaluated PER INVOCATION, not per line: a joined logical line can carry more than one
# `terminal-publish.sh …` mention (e.g. a gated `--id` shape and an ungated `--adr` shape
# documented side by side in one sentence). Splitting on each `terminal-publish.sh` occurrence
# before filtering means an `--enabled` on one invocation can never hide a bare invocation on
# the same line — a bug the earlier per-line grep had (whitewashing, change 0064 finding 2).
find_ungated_terminal_publish_call_sites(){
  local f
  while IFS= read -r f; do
    join_continuations "$f" \
      | grep -n "terminal-publish\.sh" \
      | awk '
          {
            n = split($0, segs, "terminal-publish\\.sh")
            for (i = 2; i <= n; i++) print segs[1] "terminal-publish.sh" segs[i]
          }
        ' \
      | grep -E -- '--(id|adr)([[:space:]"]|$)' \
      | grep -v -- '--enabled' \
      | sed "s#^#$f:#"
  done < <(find "$REPO/skills" -type f
           find "$REPO/scripts" -type f -name '*.sh' ! -name 'terminal-publish.sh'
           find "$REPO" -maxdepth 1 -type f -name '*.sh')
}

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

# --- archive-change.sh: set_field does NOT rewrite a body-level status: line ---
read -r W _ < <(new_repo)
printf '\nstatus: this-is-body-prose-not-frontmatter\n' >> "$W/docs/changes/active/0007-sample.md"
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
af="$W/docs/changes/archive/2026-06-18-0007-sample.md"
assert "set_field: frontmatter status set to done" '[ "$(. "$REPO/scripts/lib/docket-frontmatter.sh"; field "'"$af"'" status)" = done ]'
assert "set_field: body status: line untouched (not rewritten to done)" 'grep -qF "status: this-is-body-prose-not-frontmatter" "'"$af"'"'

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
assert "publish: competing commit preserved (not clobbered)" 'git -C "$W" log origin/main --oneline --grep=competing | grep -q .'

# --- terminal-publish.sh: CAS conflict ELSE-branch (competing writer DIVERGES a copy-set path) ---
# The test above advanced README (a different path => clean rebase, the if-branch). Here the
# competing writer rewrites the SAME archived change file with divergent bytes. publish provisions
# pub on the work clone's STALE origin/main (it only fetched docket), so its push is non-FF; the
# `pull --rebase` then replays our "add archived file (docket bytes)" onto the competing
# "add archived file (other bytes)" => add/add CONFLICT => the loop takes its else-branch
# (re-checkout origin/docket's authoritative bytes -> rebase --continue). docket's bytes must win
# and no conflict markers may leak.
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
comp="$(mktemp -d)"; git clone "$(git -C "$W" remote get-url origin)" "$comp" >/dev/null 2>&1
git -C "$comp" checkout main >/dev/null 2>&1
mkdir -p "$comp/docs/changes/archive"
printf 'COMPETING-DIVERGENT-BYTES must be overwritten by docket authoritative content\n' \
  > "$comp/docs/changes/archive/2026-06-18-0007-sample.md"
git -C "$comp" add -A
git -C "$comp" -c user.email=c@c -c user.name=c commit -m "competing divergence on a copy-set path" >/dev/null 2>&1
git -C "$comp" push origin main >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1
landed="$(git -C "$W" show origin/main:docs/changes/archive/2026-06-18-0007-sample.md 2>/dev/null)"
assert "publish(conflict): exits 0 after resolving the same-file rebase conflict" "[ $rc -eq 0 ]"
assert "publish(conflict): docket authoritative bytes win (status: done present)" 'printf "%s\n" "$landed" | grep -q "^status: done"'
assert "publish(conflict): competing divergent bytes overwritten" '! printf "%s\n" "$landed" | grep -q "COMPETING-DIVERGENT-BYTES"'
assert "publish(conflict): no conflict markers leaked into the landed file" '! printf "%s\n" "$landed" | grep -q "^<<<<<<<"'
# change 0040: the copy-set (--id 7) includes Accepted ADR-0003, so the retry path must ALSO
# regenerate the ADR index (A7 — regenerate-don't-3-way-merge); same-path divergence exercises it.
cidx="$(git -C "$W" show origin/main:docs/adrs/README.md 2>/dev/null)"
assert "publish(conflict): ADR index regenerated in the CAS retry path (lists ADR-0003)" 'printf "%s\n" "$cidx" | grep -q "ADR-0003"'
assert "publish(conflict): no conflict markers leaked into the retry-path index" '! printf "%s\n" "$cidx" | grep -q "^<<<<<<<"'

# --- terminal-publish.sh: main-mode no-op ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch main --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish: main-mode exits 0 (no-op)" "[ $? -eq 0 ]"
assert "publish: main-mode created no pub worktree" '! git -C "$W" worktree list | grep -q "pub-7"'
# change 0040: main-mode early-exits before the copy/render region, so it writes no ADR index.
assert "publish(index): main-mode writes no ADR index" '! git -C "$W" ls-tree -r --name-only origin/main 2>/dev/null | grep -q "docs/adrs/README.md"'

# --- terminal-publish.sh --adr: standalone Accepted ADR publishes to the integration branch ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1
ls_main(){ git -C "$W" ls-tree -r --name-only origin/main; }
assert "publish --adr: exits 0" "[ $rc -eq 0 ]"
assert "publish --adr: ADR-0003 file landed on integration branch" 'ls_main | grep -q "docs/adrs/0003-accepted.md"'
assert "publish --adr: no change file published (archive skipped)" '! ls_main | grep -q "docs/changes/"'
assert "publish --adr: pub-adr-3 worktree torn down" '! git -C "$W" worktree list | grep -q "pub-adr-3"'

# --- terminal-publish.sh --adr: NO Accepted gate (a non-Accepted ADR still publishes) ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 5 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1
assert "publish --adr: Proposed ADR-0005 still published (no gate in adr mode)" \
  'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/0005-proposed.md"'

# --- terminal-publish.sh --adr: idempotent re-run is a no-op ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1; before="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1; after="$(git -C "$W" rev-parse origin/main)"
assert "publish --adr: re-run exits 0" "[ $rc -eq 0 ]"
assert "publish --adr: re-run is a no-op (no new integration commit)" '[ "$before" = "$after" ]'

# --- terminal-publish.sh --adr: main-mode no-op ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch main --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish --adr: main-mode exits 0 (no-op)" "[ $? -eq 0 ]"
assert "publish --adr: main-mode created no pub-adr worktree" '! git -C "$W" worktree list | grep -q "pub-adr-3"'

# --- change 0064: --enabled false suppresses the publish (change shape) ---
read -r W _ < <(new_repo)
before="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled false ) >/dev/null 2>&1
rc=$?
git -C "$W" fetch origin main >/dev/null 2>&1
after="$(git -C "$W" rev-parse origin/main)"
assert "0064 publish --enabled false: exits 0 (suppressed publish is success)" '[ "$rc" -eq 0 ]'
assert "0064 publish --enabled false: integration branch untouched" '[ "$before" = "$after" ]'
assert "0064 publish --enabled false: no pub worktree provisioned" '! git -C "$W" worktree list | grep -q "pub-7"'

# --- change 0064: --enabled false suppresses the ADR shape too (the docket-adr path) ---
read -r W _ < <(new_repo)
before="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled false ) >/dev/null 2>&1
rc=$?
git -C "$W" fetch origin main >/dev/null 2>&1
after="$(git -C "$W" rev-parse origin/main)"
assert "0064 publish --adr --enabled false: exits 0" '[ "$rc" -eq 0 ]'
assert "0064 publish --adr --enabled false: no ADR file on integration branch" \
  '! git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/0003-accepted.md"'
assert "0064 publish --adr --enabled false: no ADR index on integration branch" \
  '! git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/README.md"'
assert "0064 publish --adr --enabled false: integration branch untouched" '[ "$before" = "$after" ]'

# --- change 0064: back-compat — omitting --enabled still publishes (default true) ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?
git -C "$W" fetch origin main >/dev/null 2>&1
assert "0064 publish: omitting --enabled defaults to true (publishes)" '[ "$rc" -eq 0 ]'
assert "0064 publish: default-true actually landed the ADR" \
  'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/0003-accepted.md"'

# --- change 0064: an explicit --enabled true publishes exactly as today ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled true ) >/dev/null 2>&1
rc=$?
git -C "$W" fetch origin main >/dev/null 2>&1
assert "0064 publish --enabled true: exits 0" '[ "$rc" -eq 0 ]'
assert "0064 publish --enabled true: ADR landed on integration branch" \
  'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/0003-accepted.md"'

# --- change 0064: fail-closed — an unparseable --enabled value aborts ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled maybe ) >/dev/null 2>&1
rc=$?
assert "0064 publish: unparseable --enabled exits non-zero (never coerced to true)" '[ "$rc" -ne 0 ]'

# --- change 0064: argument validation still fires when publishing is disabled ---
# A disabled publish must not mask a broken call site.
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled false ) >/dev/null 2>&1
rc=$?
assert "0064 publish: missing --outcome still aborts even with --enabled false" '[ "$rc" -ne 0 ]'

# --- terminal-publish.sh: --id and --adr are mutually exclusive; exactly one required ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --adr 3 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish: --id + --adr together is rejected (non-zero)" '[ "$?" -ne 0 ]'
( cd "$W" && "$PUBLISH" --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish: neither --id nor --adr is rejected (non-zero)" '[ "$?" -ne 0 ]'

# =====================================================================================
# change 0040 — terminal-publish refreshes the integration-branch ADR index when it
# publishes an ADR, rendered from the integration branch's OWN ADR set (no dangling links),
# in the SAME publish commit, only when an ADR is actually published, a no-op in main-mode.
# Helper: extract the ADR-file links the index emits (rows render `(NNNN-slug.md)` relative paths).
idx_links(){ printf '%s\n' "$1" | grep -oE '\(([0-9]{4}-[^)]+\.md)\)' | tr -d '()'; }

# (1) change-publish (--id) with an Accepted ADR → index lists it, every link resolves, same commit
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1
idx="$(git -C "$W" show origin/main:docs/adrs/README.md 2>/dev/null)"
assert "publish(index): README.md present on integration branch after --id publish" '[ -n "$idx" ]'
assert "publish(index): index lists the published Accepted ADR-0003" 'printf "%s\n" "$idx" | grep -q "ADR-0003"'
landed_adrs="$(git -C "$W" ls-tree -r --name-only origin/main -- docs/adrs)"
dangle=0; for l in $(idx_links "$idx"); do printf '%s\n' "$landed_adrs" | grep -qx "docs/adrs/$l" || dangle=1; done
assert "publish(index): every index link resolves to a file on the branch (no dangling row)" '[ "$dangle" -eq 0 ]'
tip_files="$(git -C "$W" show --name-only --format= origin/main)"
assert "publish(index): ADR file and index land in the SAME publish commit" \
  'printf "%s\n" "$tip_files" | grep -qx "docs/adrs/0003-accepted.md" && printf "%s\n" "$tip_files" | grep -qx "docs/adrs/README.md"'

# (2) ADR-only publish (--adr) → index includes the published ADR; every link resolves
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1
idx="$(git -C "$W" show origin/main:docs/adrs/README.md 2>/dev/null)"
assert "publish --adr(index): index lists the published ADR-0003" 'printf "%s\n" "$idx" | grep -q "ADR-0003"'
landed_adrs="$(git -C "$W" ls-tree -r --name-only origin/main -- docs/adrs)"
dangle=0; for l in $(idx_links "$idx"); do printf '%s\n' "$landed_adrs" | grep -qx "docs/adrs/$l" || dangle=1; done
assert "publish --adr(index): every index link resolves (no dangling row)" '[ "$dangle" -eq 0 ]'

# (3) renders from the BRANCH set, not the metadata superset — dangling-link guard (A4).
# ADR-0003 is Accepted on docket but its change is not terminal, so its FILE is not on main.
# Publishing a DIFFERENT ADR (0005) must yield an index that lists 0005 and NOT 0003.
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 5 --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1
idx="$(git -C "$W" show origin/main:docs/adrs/README.md 2>/dev/null)"
assert "publish(index, branch-set): lists the just-published ADR-0005" 'printf "%s\n" "$idx" | grep -q "ADR-0005"'
assert "publish(index, branch-set): does NOT list ADR-0003 whose file is not on the branch (no dangling row)" \
  '! printf "%s\n" "$idx" | grep -q "ADR-0003"'

# (4) no-ADR change-publish → no spurious ADR index back-fill commit
read -r W _ < <(new_repo)
sed -i.bak 's/^adrs: \[3, 5\]/adrs:/' "$W/docs/changes/active/0007-sample.md" && rm -f "$W/docs/changes/active/0007-sample.md.bak"
git -C "$W" commit -aqm "test: change with no adrs" >/dev/null 2>&1
git_quiet -C "$W" push origin docket
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1
assert "publish(index): no-ADR change-publish writes NO ADR index (no spurious back-fill)" \
  '! git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/README.md"'
assert "publish(index): no-ADR change-publish still lands the change file" \
  'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-06-18-0007-sample.md"'

# (6) idempotent re-run → the index is byte-stable (no new integration commit, no churn)
read -r W _ < <(new_repo)
"$ARCHIVE" --changes-dir "$W/docs/changes" --id 7 --outcome done --date 2026-06-18 >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1; idx1="$(git -C "$W" show origin/main:docs/adrs/README.md 2>/dev/null)"; rev1="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --id 7 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1; idx2="$(git -C "$W" show origin/main:docs/adrs/README.md 2>/dev/null)"; rev2="$(git -C "$W" rev-parse origin/main)"
assert "publish(index): idempotent re-run keeps the index byte-stable" '[ "$idx1" = "$idx2" ] && [ -n "$idx1" ]'
assert "publish(index): idempotent re-run makes no new integration commit" '[ "$rev1" = "$rev2" ]'

# (7) CAS retry where the competing writer DIVERGES the index PATH itself (A7, LEARNINGS #25).
# A concurrent push lands divergent docs/adrs/README.md on main; our --adr 3 publish also ADDs a
# rendered README, so the rebase hits an add/add CONFLICT on the index path. The retry-path
# refresh_adr_index must REGENERATE (not 3-way-merge): docket's freshly-rendered index wins, the
# competing bytes are overwritten, and no conflict markers leak. (Provisioning pub on the work
# clone's STALE origin/main is what forces the non-FF push and the rebase.)
read -r W _ < <(new_repo)
comp="$(mktemp -d)"; git clone "$(git -C "$W" remote get-url origin)" "$comp" >/dev/null 2>&1
git -C "$comp" checkout main >/dev/null 2>&1
mkdir -p "$comp/docs/adrs"
printf 'COMPETING-INDEX-BYTES must be overwritten by the regenerated ADR index\n' > "$comp/docs/adrs/README.md"
git -C "$comp" add -A
git -C "$comp" -c user.email=c@c -c user.name=c commit -m "competing divergence on the index path" >/dev/null 2>&1
git -C "$comp" push origin main >/dev/null 2>&1
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1
idx="$(git -C "$W" show origin/main:docs/adrs/README.md 2>/dev/null)"
assert "publish(index-conflict): exits 0 after resolving an add/add conflict on the index path" "[ $rc -eq 0 ]"
assert "publish(index-conflict): regenerated index wins (lists ADR-0003)" 'printf "%s\n" "$idx" | grep -q "ADR-0003"'
assert "publish(index-conflict): competing index bytes overwritten" '! printf "%s\n" "$idx" | grep -q "COMPETING-INDEX-BYTES"'
assert "publish(index-conflict): no conflict markers leaked into the landed index" '! printf "%s\n" "$idx" | grep -q "^<<<<<<<"'

# --- cleanup-feature-branch.sh: removes a worktree under .worktrees/<slug> + its branch ---
read -r W _ < <(new_repo)
git -C "$W" worktree add "$W/.worktrees/sample" -b feat/sample main >/dev/null 2>&1
git -C "$W" push -u origin feat/sample >/dev/null 2>&1
( cd "$W" && "$CLEANUP" --slug sample ) >/dev/null 2>&1
assert "cleanup: worktree removed" '[ ! -e "$W/.worktrees/sample" ]'
assert "cleanup: local branch deleted" '! git -C "$W" rev-parse --verify -q feat/sample >/dev/null'
assert "cleanup: remote branch deleted" '! git -C "$W" ls-remote --exit-code origin feat/sample >/dev/null 2>&1'

# --- cleanup-feature-branch.sh: provenance guard refuses an out-of-.worktrees path ---
read -r W _ < <(new_repo)
out_base="$(mktemp -d)"
git -C "$W" worktree add "$out_base/evil" -b feat/evil main >/dev/null 2>&1
( cd "$W" && "$CLEANUP" --slug evil --worktrees-dir "$out_base" ) >/dev/null 2>&1; rc_guard=$?
assert "cleanup: refuses a worktree outside .worktrees/ (non-zero)" '[ "$rc_guard" -ne 0 ]'
assert "cleanup: out-of-tree worktree survives the refusal" '[ -e "$out_base/evil" ]'
assert "cleanup: refused branch feat/evil still present (guard fired before delete)" 'git -C "$W" rev-parse --verify -q feat/evil >/dev/null'

# --- finalize wiring sentinels: docket-finalize-change invokes the scripts (single source) ---
FINALIZE="$REPO/skills/docket-finalize-change/SKILL.md"
assert "wiring(finalize): invokes archive-change (via the docket.sh facade)" 'grep -q "docket.sh archive-change" "$FINALIZE"'
assert "wiring(finalize): invokes terminal-publish (via the docket.sh facade)" 'grep -q "docket.sh terminal-publish" "$FINALIZE"'
assert "wiring(finalize): invokes cleanup-feature-branch (via the docket.sh facade)" 'grep -q "docket.sh cleanup-feature-branch" "$FINALIZE"'
assert "wiring(finalize): Terminal publish section heading preserved (cross-ref anchor)" 'grep -qF "## Terminal publish (docket-mode)" "$FINALIZE"'
assert "wiring(finalize): Accepted gate still documented" 'grep -qiE "whose ADR is .?Accepted|Accepted. gate|status: is .?Accepted|status.? is \*\*Accepted" "$FINALIZE"'
assert "wiring(finalize): ADR-only publish path preserved" 'grep -qiE "adr-<NN>|ADR-only" "$FINALIZE"'
assert "wiring(finalize): no leftover raw archive bash (git mv active/)" '! grep -qE "git mv .*active/" "$FINALIZE"'
assert "wiring(finalize): ADR-only publish names docket.sh terminal-publish --adr" 'grep -qE "docket\.sh terminal-publish --adr" "$FINALIZE"'
assert "wiring(finalize): no leftover by-hand pub-adr git block" '! grep -qE "git worktree add -B .?pub-adr" "$FINALIZE"'

# --- call-site wiring sentinels: status sweep + two kill paths invoke the scripts ---
STATUS="$REPO/skills/docket-status/SKILL.md"
TCO="$REPO/skills/docket-convention/references/terminal-close-out.md"
assert "wiring(status): sweep points at the terminal-close-out reference" 'grep -qF "terminal-close-out.md" "$STATUS"'
NEWCHG="$REPO/skills/docket-new-change/SKILL.md"
IMPL="$REPO/skills/docket-implement-next/SKILL.md"
assert "wiring(close-out ref): sweep invokes archive-change.sh"   'grep -q "/archive-change.sh" "$TCO"'
assert "wiring(close-out ref): sweep invokes terminal-publish.sh" 'grep -q "/terminal-publish.sh" "$TCO"'
assert "wiring(close-out ref): sweep invokes cleanup-feature-branch.sh" 'grep -q "/cleanup-feature-branch.sh" "$TCO"'
# --- change 0036: the sweep delegates archiving to archive-change.sh (no manual double-archive) ---
# The sweep's per-change archive must NOT hand-roll the move any more (mirrors the finalize sentinel).
assert "wiring(status): sweep has no leftover raw archive bash (git mv active/)" \
  '! grep -qE "git mv .*active/" "$STATUS"'
# The renderer re-render must be ordered AFTER archive-change.sh and BEFORE terminal-publish
# (LEARNINGS #0035 — anchor to the unique "before … terminal-publish" phrasing, assert order not presence).
assert "wiring(close-out ref): sweep re-renders the Artifacts block before terminal-publish" \
  'awk "/render-change-links\\.sh/{r=NR} /terminal-publish\\.sh/{if(r && r<NR){print \"ok\"; exit}}" "$TCO" | grep -q ok'
assert "wiring(status): sweep names render-change-links (via the docket.sh facade) in the delegated archive flow" \
  'grep -q "docket.sh render-change-links" "$STATUS"'
# The sweep's failure posture is log-and-continue (its own unique phrasing), NOT abort-and-report.
assert "wiring(status): sweep failure posture is log-and-continue (abandon the remainder of this change)" \
  'grep -qiE "abandon the remainder of (this|THIS) change" "$STATUS"'
assert "wiring(status): sweep documents abort-and-report as a deliberate divergence, not its own posture" \
  'grep -qiE "deliberately divergent from .?docket-finalize-change" "$STATUS"'
assert "wiring(close-out ref): proposed-kill invokes archive-change.sh"   'grep -q "/archive-change.sh" "$TCO"'
assert "wiring(close-out ref): proposed-kill invokes terminal-publish.sh" 'grep -q "/terminal-publish.sh" "$TCO"'
assert "wiring(close-out ref): reconcile-kill invokes archive-change.sh"     'grep -q "/archive-change.sh" "$TCO"'
assert "wiring(close-out ref): reconcile-kill invokes cleanup-feature-branch.sh" 'grep -q "/cleanup-feature-branch.sh" "$TCO"'
assert "wiring(close-out ref): reconcile-kill invokes terminal-publish.sh" 'grep -q "/terminal-publish.sh" "$TCO"'

# --- change 0064: every documented terminal-publish call site passes --enabled ---
ADRSKILL="$REPO/skills/docket-adr/SKILL.md"
assert "0064 wiring: close-out step 3 passes --enabled" \
  'grep -q -- "--enabled" "$TCO"'
assert "0064 wiring: close-out step 3 passes the terminal_publish placeholder" \
  'grep -q -- "--enabled <terminal_publish>" "$TCO"'
assert "0064 wiring: docket-adr passes --enabled on BOTH --adr call sites" \
  '[ "$(grep -c -- "--enabled <terminal_publish>" "$ADRSKILL")" -eq 2 ]'
# The invariant: no REAL terminal-publish.sh call site may omit the gate — checked across BOTH
# skills/ prose AND scripts/*.sh code (the earlier version only grepped skills/, and only matched
# --id/--adr as the literal next token on the same physical line, so it was blind to
# scripts/docket-status.sh's real sweep call site and to ordinary flag reordering).
assert "0064 wiring: no terminal-publish.sh call site (skills/ prose or scripts/*.sh) omits --enabled" \
  '[ -z "$(find_ungated_terminal_publish_call_sites)" ]'

# The close-out contract: a SUPPRESSED publish is success, so it must not trip the skip-publish
# guard — cleanup (step 4) and the board refresh (step 5) still run. This is the spec's
# "close-out integration" requirement; the sequence is skill-driven prose, so it is asserted here
# as a contract sentinel rather than an executable close-out fixture.
assert "0064 wiring: close-out states a suppressed publish does not skip steps 4-5" \
  'grep -qi "does NOT trip the\|not trip the skip-publish" "$TCO"'

exit "$fail"
