#!/usr/bin/env bash
# scripts/mint-stub.sh — deterministic discovered-work stub mint (change 0091). The MECHANICAL half
# of auto-capture: the calling skill judges WHAT is material (and gates on AUTO_CAPTURE); this script
# does the mint — cheap active-slug dedup, id allocation (max+1 across active/ + archive/), stub write
# from the change template with discovered_from: populated, and the compare-and-swap push. Git-only
# (no gh, no network beyond the remote). ADR-0012: a deterministic script, never model prose.
# ADR-0021: authors its own mechanical commit.
#
# Usage: mint-stub.sh --changes-dir DIR --title TITLE --body-file FILE --discovered-from ID
#                     [--slug SLUG] [--minted N] [--cap N] [--remote R] [--template PATH]
#   Mints exactly ONE stub per invocation. --minted is how many stubs THIS skill invocation has
#   already minted; at --cap (default 3) the mint is refused so the caller can surface the overflow.
#   Report (exactly one line, stdout):
#     minted <id> <slug>
#     skipped duplicate <slug> (matches #<id>)
#     skipped cap-reached (cap <n>, minted <n>)
#   Exit codes: 0 minted | 3 duplicate | 4 cap reached | 1 error.
#   Mock seams: GIT="${GIT:-git}"; TODAY="${TODAY:-$(date -u +%Y-%m-%d)}".
set -uo pipefail
GIT="${GIT:-git}"
TODAY="${TODAY:-$(date -u +%Y-%m-%d)}"
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHANGES_DIR=""; TITLE=""; BODY_FILE=""; FROM=""; SLUG=""; MINTED=0; CAP=3; REMOTE="origin"
TEMPLATE="$SELF_DIR/../skills/docket-new-change/change-template.md"
die(){ printf '%s\n' "mint-stub: $*" >&2; exit 1; }
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --title) TITLE="$2"; shift ;;
    --body-file) BODY_FILE="$2"; shift ;;
    --discovered-from) FROM="$2"; shift ;;
    --slug) SLUG="$2"; shift ;;
    --minted) MINTED="$2"; shift ;;
    --cap) CAP="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    --template) TEMPLATE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"
[ -n "$TITLE" ]       || die "missing --title"
[ -n "$BODY_FILE" ]   || die "missing --body-file"
[ -f "$BODY_FILE" ]   || die "body file not found: $BODY_FILE"
[ -f "$TEMPLATE" ]    || die "change template not found: $TEMPLATE"
case "$FROM"   in ''|*[!0-9]*) die "missing/invalid --discovered-from (want a change id)" ;; esac
case "$MINTED" in ''|*[!0-9]*) die "invalid --minted" ;; esac
case "$CAP"    in ''|*[!0-9]*) die "invalid --cap" ;; esac
# The body is the model's prose; pin only its entry shape so a malformed stub can never land.
head1="$(head -n1 "$BODY_FILE")"
case "$head1" in "## Why"*) ;; *) die "body file must start with '## Why'" ;; esac

# shellcheck source=/dev/null
. "$SELF_DIR/lib/docket-frontmatter.sh"   # field / int_field

# slugify TEXT — lowercase, non-alphanumerics -> '-', squeeze, trim, cap at 60 chars, trim again.
# The second trim (B5) matters: `cut -c1-60` runs AFTER the first trim, so truncation can reopen a
# trailing '-' the first trim never saw (the pre-cut string ended mid-word further along). Without
# it, slugify is not idempotent — slugify(stored_slug) != slugify(cut of a longer want) — which is
# exactly the property dup_of's comparison depends on.
slugify(){
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//' | cut -c1-60 | sed -E 's/-+$//'
}
[ -n "$SLUG" ] || SLUG="$(slugify "$TITLE")"
[ -n "$SLUG" ] || die "could not derive a slug from --title"

# Cap first: it is the cheapest refusal and must not depend on repo state.
if [ "$MINTED" -ge "$CAP" ]; then
  printf 'skipped cap-reached (cap %s, minted %s)\n' "$CAP" "$MINTED"
  exit 4
fi

WT="$($GIT -C "$CHANGES_DIR" rev-parse --show-toplevel)" || die "not a git worktree: $CHANGES_DIR"
REL_ABS="$(cd "$CHANGES_DIR" && pwd -P)"; REL="${REL_ABS#"$WT"/}"
cur_branch="$($GIT -C "$WT" rev-parse --abbrev-ref HEAD)"

# set_field FILE KEY VALUE — replace a scalar in the first ---…--- block only (AGENTS.md
# frontmatter-anchor rule). VALUE is model-authored English prose (a title), never a script
# constant — it MUST NOT be interpolated into a sed/awk pattern or replacement string, where '|',
# '&', and '\' are all metacharacters that would corrupt or silently swallow the value (B1). It is
# passed through awk's ENVIRON instead: unlike `awk -v`, an ENVIRON read is never re-processed for
# escape sequences or replacement syntax, so the value is written back byte-for-byte. Returns
# non-zero (and removes its own temp file) on any awk/mv failure; every call site checks this so a
# failed field write can never reach the commit/push below.
set_field(){
  local f="$1" k="$2" t; t="$(mktemp)" || return 1
  if ! MINT_SF_VAL="$3" awk -v key="$k" '
        BEGIN { val = ENVIRON["MINT_SF_VAL"]; dash = 0 }
        /^---$/ { dash++; print; next }
        dash == 1 && $0 ~ ("^" key ":") { print key ": " val; next }
        { print }
      ' "$f" > "$t"; then
    rm -f "$t"; return 1
  fi
  mv "$t" "$f"
}

# dup_of SLUG — print the id of an ACTIVE change whose slug OR title slugifies to SLUG; empty if none.
# Active-only by spec §5: an archived near-name is history, not a live duplicate.
dup_of(){
  local want="$1" f fslug ftitle
  shopt -s nullglob
  for f in "$WT/$REL/active/"*.md; do
    fslug="$(field "$f" slug)"
    ftitle="$(field "$f" title)"
    if [ "$(slugify "$fslug")" = "$want" ] || [ "$(slugify "$ftitle")" = "$want" ]; then
      int_field "$f" id; shopt -u nullglob; return 0
    fi
  done
  shopt -u nullglob
  return 1
}

# next_id — max `id:` across active/ + archive/, plus one. 1 when the backlog is empty.
next_id(){
  local f v max=0
  shopt -s nullglob
  for f in "$WT/$REL/active/"*.md "$WT/$REL/archive/"*.md; do
    v="$(int_field "$f" id)"; [ -n "$v" ] || continue
    [ "$v" -gt "$max" ] && max="$v"
  done
  shopt -u nullglob
  printf '%s' "$(( max + 1 ))"
}

# write_stub ID — render the stub from the template and stage+commit it. Frontmatter scalars are
# rewritten ONLY inside the first ---…--- block (AGENTS.md frontmatter-anchor rule); the template's
# commented body scaffolding is replaced wholesale by the model's body, and the empty ## Artifacts
# marker block is emitted verbatim (render-change-links.sh remains its sole writer). mkdir -p's the
# target directory every call (B2): a prior CAS `reset --hard` can prune active/ entirely when it
# held only the stub this same run just wrote (git does not track empty directories). Every
# set_field call and the stub write itself are checked (B1/B2): a failure here must never reach the
# commit/push below.
write_stub(){
  local id="$1" pad file tmp tmp2
  pad="$(printf '%04d' "$id")"
  file="$WT/$REL/active/$pad-$SLUG.md"
  mkdir -p "$WT/$REL/active" || die "mkdir -p active dir failed for stub $id"
  tmp="$(mktemp)"; tmp2="$(mktemp)"
  # frontmatter: everything up to and including the SECOND '---'
  awk 'NR==1&&$0=="---"{print;next} /^---$/{print;exit} {print}' "$TEMPLATE" > "$tmp"
  # Strip the template's instructional "  # comment" scaffolding (e.g. `spec:   # path under ...`)
  # from every line in the block BEFORE any set_field call below writes real values — a value
  # written afterward is never mistaken for comment text even if it happens to contain '#'.
  if ! sed -E 's/[[:space:]]+#.*$//' "$tmp" > "$tmp2"; then
    rm -f "$tmp" "$tmp2"; die "template comment-strip failed for stub $id"
  fi
  mv "$tmp2" "$tmp"
  set_field "$tmp" id "$id"                  || die "set_field id failed for stub $id"
  set_field "$tmp" slug "$SLUG"               || die "set_field slug failed for stub $id"
  set_field "$tmp" title "$TITLE"             || die "set_field title failed for stub $id"
  set_field "$tmp" created "$TODAY"           || die "set_field created failed for stub $id"
  set_field "$tmp" updated "$TODAY"           || die "set_field updated failed for stub $id"
  set_field "$tmp" discovered_from "[$FROM]"  || die "set_field discovered_from failed for stub $id"
  {
    cat "$tmp"
    printf '\n## Artifacts\n\n'
    printf '<!-- docket:artifacts:start (generated — do not hand-edit) -->\n'
    printf '<!-- docket:artifacts:end -->\n\n'
    cat "$BODY_FILE"
  } > "$file" || die "write stub file failed for id $id"
  rm -f "$tmp"
  $GIT -C "$WT" add "$REL/active/$pad-$SLUG.md"                                  || die "git add failed for $id"
  $GIT -C "$WT" commit -q -m "docket($pad): auto-capture stub discovered from #$FROM" \
       -- "$REL/active/$pad-$SLUG.md"                                            || die "commit failed for $id"
  printf '%s' "$file"
}

dup_id="$(dup_of "$SLUG")" && {
  printf 'skipped duplicate %s (matches #%s)\n' "$SLUG" "$dup_id"
  exit 3
}

id="$(next_id)"
write_stub "$id" >/dev/null

# safe_reset_hard LABEL — reset --hard to the fresh remote tip, but REFUSE (and die) when the
# worktree carries uncommitted changes this run did not just create itself (B3). This script shares
# its metadata worktree with other autonomous agents; the tree is clean-by-construction immediately
# after write_stub's own commit (git add/commit above are pathspec-scoped to the one file), so ANY
# `git status --porcelain` output at this point can only belong to another writer. Resetting over
# that would silently discard someone else's work while this script still reports success. The
# guarantee this preserves: reset --hard only ever discards THIS run's own last commit — never
# anything it didn't write itself.
safe_reset_hard(){
  local label="$1" dirty
  dirty="$($GIT -C "$WT" status --porcelain)"
  [ -z "$dirty" ] \
    || die "refusing reset --hard ($label): worktree has uncommitted changes from another writer: $dirty"
  $GIT -C "$WT" reset --hard "$REMOTE/$cur_branch" >/dev/null 2>&1 \
    || die "reset --hard failed ($label; $REMOTE/$cur_branch unreachable or missing)"
}

# Bounded CAS retry. On EVERY lost race (push rejected as non-fast-forward): fetch + safe_reset_hard
# to the fresh remote tip, then RE-DERIVE both the dedup verdict and the next id from that origin
# state — never from the working tree we just wrote (which would read back our own stub and re-mint
# the same id forever). A push failure that is NOT a lost race (auth, network, remote gone, hook
# rejection, ...) is a real error (B4): captured stderr is matched against the non-fast-forward
# signature, and anything else dies immediately instead of being burned through as five pointless
# retries. Either way — an immediate non-race die, or exhausting all 5 retries without converging —
# the local branch is reset back to the fresh remote tip before dying, so no unpushed commit is ever
# left behind (an unpushed leftover would otherwise corrupt the NEXT invocation's dedup/id scan,
# which reads the working tree, not git log). If that cleanup reset itself is refused by the B3
# guard (a genuinely concurrent writer), the diagnostic says so instead.
result=exhausted
for attempt in 1 2 3 4 5; do
  push_err="$($GIT -C "$WT" push "$REMOTE" "$cur_branch" 2>&1 >/dev/null)"; push_rc=$?
  if [ "$push_rc" -eq 0 ]; then
    result=pushed; break
  fi
  case "$push_err" in
    *"[rejected]"*|*"non-fast-forward"*|*"fetch first"*|*"stale info"*) : ;;   # lost race — retry below
    *)
      safe_reset_hard "push failure cleanup"
      die "push failed (attempt $attempt, not a lost race): $push_err"
      ;;
  esac
  $GIT -C "$WT" fetch "$REMOTE" >/dev/null 2>&1 \
    || die "fetch during CAS failed (attempt $attempt)"
  safe_reset_hard "attempt $attempt"
  # A concurrent writer may have minted this very stub while we raced.
  dup_id="$(dup_of "$SLUG")" && {
    printf 'skipped duplicate %s (matches #%s)\n' "$SLUG" "$dup_id"
    exit 3
  }
  id="$(next_id)"
  write_stub "$id" >/dev/null
done
case "$result" in
  pushed) printf 'minted %s %s\n' "$id" "$SLUG" ;;
  *)
    safe_reset_hard "exhaustion cleanup"
    die "push did not converge after 5 attempts (local branch reset to $REMOTE/$cur_branch; no unpushed commit left)"
    ;;
esac
exit 0
