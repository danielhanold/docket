#!/usr/bin/env bash
# tests/test_shared_worktree_commit_scope.sh — change 0247.
# ONE invariant, two channels: no commit into the SHARED .docket metadata worktree may stage
# anything it did not write. The tree is dirty for another agent's whole multi-tool-call
# edit->commit window, so a pathspec-less commit lands that agent's staged work under this run's
# message (observed live 2026-08-09: an interactive groom's three staged files were swallowed by
# two concurrent autonomous commits, and the groom's own commit reported "nothing to commit").
#
#   Group A (script channel) — every `git … commit` in scripts/**/*.sh carries a `--` pathspec.
#                              Default-deny, with a keyed exception list for exclusive-worktree
#                              sites. Task-3 scope.
#   Group B (agent channel)  — the convention states the rule at the direct-git grant, and every
#                              metadata-writing skill body carries the marker. Task-5 scope.
# Both live in ONE file on purpose: a second file would split the exception lists for one invariant.
#
# CONTRACT BOUNDARY: this detects `commit` as an exact-token subcommand under an EXPLICIT driver
# set. It is not, and must not become, a general shell parser. A commit issued through a driver
# spelling outside the set is outside the contract (accepted limit — the set is small because the
# repo's metadata-writing drivers are). Introducing a new driver means extending DRIVERS in the
# same change: a review obligation, not something the guard infers.
# Run: bash tests/test_shared_worktree_commit_scope.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# The explicit driver set. `g` is docket-config.sh's local wrapper `g(){ "$GIT" -C "$REPO_DIR" "$@"; }`
# — it writes the metadata branch, so it is a metadata-writing driver even though it is one letter.
DRIVERS='git $GIT ${GIT} "$GIT" "${GIT}" g'

# segments_of — read logical_lines records ("<lineno><TAB><text>") on stdin; emit one record per
# shell segment, as "<lineno><TAB><masked segment><US><raw segment>" (US = 0x1f). It takes the whole
# file's stream rather than one line at a time so the scan costs ONE awk per file instead of one per
# logical line — the difference between a ~1s guard and a ~27s one across scripts/.
# THREE stages in one pass, each independently mutable and each load-bearing:
#
#   (1) MASK — replace the CONTENTS of quoted runs with X, preserving quotes and LENGTH. Detection
#       must run on masked text because commit MESSAGES routinely carry the very characters stages
#       (2) and (3) key on: reclaim-claims.sh's message contains a `;`, mint-stub.sh's contains a
#       space-hash. Raw-text detection reports both as violations — they are this guard's live
#       positive controls for masking (#0119, settled).
#   (2) COMMENT STRIP — truncate at an unquoted comment introducer. Comments are prose, and only
#       executable text can violate the gate (AGENTS.md: "sort them into prose vs executable").
#       Ordering after (1) is what makes it safe: a hash inside a quoted run is already an X, so a
#       surviving hash is a real introducer.
#   (3) SPLIT on `;` `|` `&&` `||`. Per-SEGMENT, never whole-line: a whole-line predicate has a
#       demonstrated false NEGATIVE on the #0083 mark path, where an unscoped commit shares a line
#       with a scoped neighbour whose `--` satisfies the line-level check (#0119, settled).
#
# Both tracks are emitted because they answer different questions. The masked track is what the
# predicate may safely read; the raw track is where the exception KEY lives, since masking would
# turn `-C "$mw"` into `-C "XXXX"` and collapse every key in the repo to one meaningless string.
# They stay aligned because stage (1) is length-preserving, so a cut or split offset computed on
# the masked string indexes the same character of the raw one.
#
# Implemented in awk rather than `sed 's/&&/\n/g'`: BSD sed is not required to expand \n in a
# replacement, and a split that silently does nothing turns the per-segment predicate back into the
# whole-line predicate whose false negative is the reason segments exist. (macOS's sed happens to
# expand it — a property of one sed, not a portability guarantee.)
segments_of(){
  awk '{
    tab = index($0, "\t")
    if (tab == 0) next
    lno = substr($0, 1, tab - 1)
    raw = substr($0, tab + 1)
    masked = ""; inq = "";
    for (i = 1; i <= length(raw); i++) {
      ch = substr(raw, i, 1)
      if (inq == "") { masked = masked ch; if (ch == "\"" || ch == "'"'"'") inq = ch }
      else if (ch == inq) { masked = masked ch; inq = "" }
      else { masked = masked "X" }
    }
    cut = 0
    for (i = 1; i <= length(masked); i++) {
      if (substr(masked, i, 1) != "#") continue
      p = (i == 1) ? " " : substr(masked, i - 1, 1)
      if (p == " " || p == "\t") { cut = i; break }
    }
    if (cut > 0) { masked = substr(masked, 1, cut - 1); raw = substr(raw, 1, cut - 1) }
    n = length(masked); start = 1; i = 1
    while (i <= n + 1) {
      seplen = 0
      if (i <= n) {
        two = substr(masked, i, 2); ch = substr(masked, i, 1)
        if (two == "&&" || two == "||") seplen = 2
        else if (ch == ";" || ch == "|") seplen = 1
      }
      if (i > n || seplen > 0) {
        printf "%s\t%s\037%s\n", lno, substr(masked, start, i - start), substr(raw, start, i - start)
        i += (seplen > 0 ? seplen : 1); start = i
      } else i++
    }
  }'
}

# logical_lines FILE — join backslash-continued lines into one logical line, emitting
# "<first-line-number><TAB><joined text>". Every multi-line commit in scripts/ (mint-stub.sh,
# reclaim-claims.sh, terminal-publish.sh's marker-clearing commit) puts its `--` pathspec on a
# CONTINUATION line, so a per-physical-line scan reports every one of them as unscoped.
logical_lines(){
  awk '{
    if (buf == "") { start = NR; buf = $0 } else { buf = buf " " $0 }
    if (buf ~ /\\$/) { sub(/\\$/, "", buf); next }
    printf "%d\t%s\n", start, buf; buf = ""
  }
  END { if (buf != "") printf "%d\t%s\n", start, buf }' "$1"
}

# segment_is_commit_call RAWSEGMENT — true (0) when the segment invokes a driver from DRIVERS with
# `commit` as its subcommand. Tokenized on the RAW track, not the masked one: half the drivers in
# DRIVERS are QUOTED spellings (`"$GIT"` is docket-status.sh's house idiom), and masking turns every
# one of them into `"XXXX"` — a predicate reading masked text is blind to exactly the file this
# change was written for. Message text cannot reach this predicate: the subcommand is the FIRST
# non-flag token after the driver, which is always the real subcommand, never argument text.
#
# ONE predicate for both outcomes, deliberately: scan_commits reports `scoped` and `unscoped` off
# this single call, so the two cannot drift into disagreeing about what a commit call is.
segment_is_commit_call(){
  local seg="$1" tok driver_seen=0 subcmd=""
  for tok in $seg; do
    if [ "$driver_seen" -eq 0 ]; then
      case " $DRIVERS " in *" $tok "*) driver_seen=1 ;; esac
      continue
    fi
    # First non-flag, non-"-C <arg>" token after the driver is the subcommand.
    case "$tok" in
      -C) subcmd="__skip__"; continue ;;
      -*) continue ;;
    esac
    if [ "$subcmd" = "__skip__" ]; then subcmd=""; continue; fi
    subcmd="$tok"; break
  done
  [ "$driver_seen" -eq 1 ] || return 1
  # EXACT-TOKEN match. `commit-tree` (docket-config.sh's orphan bootstrap, via the `g` wrapper) is
  # NOT a commit: it writes a commit object from a tree with no index and no pathspec concept, so a
  # substring match would report a permanent false positive there.
  [ "$subcmd" = commit ]
}

# segment_is_scoped MASKEDSEGMENT — true (0) when a `--` pathspec mark is present. Reads the MASKED
# track: a `--` occurring inside a commit MESSAGE is not a pathspec, and taking it for one is a
# false negative that exempts the very site it appears on.
segment_is_scoped(){
  grep -qE '(^| )-- ' <<<"$1"
}

# driver_target SEGMENT — the `-C` target variable name, normalized: "$mw" -> mw, "${pub}" -> pub.
# Takes the RAW track: masking would render every target as "XXXX". The exception KEY is
# <basename>:<this>, never a line number (ADR-0054): line numbers rot fastest in exactly the files
# that move most, and nothing can check them.
driver_target(){
  local seg="$1" prev="" tok
  for tok in $seg; do
    if [ "$prev" = "-C" ]; then
      tok="${tok#\"}"; tok="${tok%\"}"; tok="${tok#\$}"; tok="${tok#\{}"; tok="${tok%\}}"
      printf '%s' "$tok"; return 0
    fi
    prev="$tok"
  done
  printf '%s' "-"
}

# scan_commits [ROOT] — emit one record per commit call site:
# "<basename>:<target> <scoped|unscoped> <lineno>". ROOT defaults to the repo's scripts/ tree; the
# parameter exists so the synthetic controls below can be scanned by the SAME scanner.
#
# Every loop is fed by a HERESTRING over a captured variable, never `< <(process substitution)`:
# scan_commits is itself always called inside a command substitution, and three process
# substitutions nested inside one truncates to the first file's records on macOS's system
# /bin/bash 3.2 — silently, which is the vacuous-guard failure mode this whole file exists to
# avoid. (The suite runs on DOCKET_BASH_PATH, Bash 4.3+, where it does not; a guard that depends on
# the interpreter for its non-vacuity is not a guard.)
scan_commits(){
  local root="${1:-$REPO/scripts}" f base lno pair mseg rseg files segs
  files="$(find "$root" -name '*.sh' -type f | sort)"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    base="$(basename "$f")"
    segs="$(logical_lines "$f" | segments_of)"
    while IFS=$'\t' read -r lno pair; do
      [ -n "$lno" ] || continue
      mseg="${pair%%$'\037'*}"; rseg="${pair#*$'\037'}"
      [ -n "$mseg" ] || continue
      segment_is_commit_call "$rseg" || continue
      if segment_is_scoped "$mseg"; then
        printf '%s:%s scoped %s\n' "$base" "$(driver_target "$rseg")" "$lno"
      else
        printf '%s:%s unscoped %s\n' "$base" "$(driver_target "$rseg")" "$lno"
      fi
    done <<<"$segs"
  done <<<"$files"
}

# --- group A: the script channel -----------------------------------------------------------------
FINDINGS="$(scan_commits)"
assert "A: the scanner found commit call sites at all (non-vacuity floor)" \
  '[ "$(grep -c . <<<"$FINDINGS")" -ge 6 ]'

# EXCEPTIONS — keyed <basename>:<-C target var>, each with the reason it is exempt. An exception is
# an argued exemption, never a place to park a defect.
#   terminal-publish.sh:pub — an EXCLUSIVE `mktemp -d` worktree, and the commit is index-driven
#     (the caller has built the index deliberately). A pathspec there would CHANGE BEHAVIOUR rather
#     than harden it, and the shared-tree hazard does not exist in a worktree only this process
#     holds (#0119, settled; spec Assumption 6).
EXCEPTIONS='terminal-publish.sh:pub'

unscoped="$(grep ' unscoped ' <<<"$FINDINGS" | awk '{print $1}' | sort -u)"
for key in $unscoped; do
  case " $EXCEPTIONS " in
    *" $key "*) continue ;;
  esac
  assert "A: pathspec-less commit at $key — every commit in the shared metadata worktree must stage by explicit path" 'false'
done

# EXISTENCE FLOOR — a stale exception must redden, not sit forever. This is the direction a
# forward-only loop is structurally blind to (learnings: correspondence-guard-runs-one-way). It
# doubles as the proof that the `unscoped` branch fires at all on live text.
for key in $EXCEPTIONS; do
  assert "A: exception '$key' still matches a real unscoped site (stale exceptions must redden)" \
    'grep -q "^$key unscoped " <<<"$FINDINGS"'
done

# POSITIVE CONTROLS, LIVE — the two shapes in this repo that make masking load-bearing. Defeat
# segments_of's stage (1) and both of these turn into reported violations.
assert "A: a semicolon-bearing commit MESSAGE does not split into a false positive (masked text)" \
  '! grep -q "^reclaim-claims.sh:[^ ]* unscoped " <<<"$FINDINGS"'
assert "A: a hash-bearing commit MESSAGE does not become a false positive (masked text)" \
  '! grep -q "^mint-stub.sh:[^ ]* unscoped " <<<"$FINDINGS"'
assert "A: both fixed docket-status.sh sites are reported scoped" \
  '[ "$(grep -c "^docket-status.sh:mw scoped " <<<"$FINDINGS")" -ge 3 ]'
assert "A: docket-config.sh's commit-tree bootstrap is not reported" \
  '! grep -q "^docket-config.sh:" <<<"$FINDINGS"'

# POSITIVE CONTROLS, SYNTHETIC — several of this guard's design decisions have no live witness in
# the repo, and one that looks live is not. The exact-token control cannot be carried by
# docket-config.sh: its `g commit-tree` sits inside a command substitution opened by an assignment
# quote, so segments_of's masking stage eats the whole construct and the assert above passes for the
# wrong reason — it would stay green with the subcommand match deleted outright. The remaining
# shapes (an unscoped commit sharing a line with a scoped one, across each of the splitter's two
# arms; a commented-out commit) do not occur in scripts/ at all, and an undemonstrable design
# decision is indistinguishable from decoration. These lines are scanned by the SAME scan_commits,
# so they exercise the real predicate rather than a restatement of it.
FIXDIR="$(mktemp -d "${TMPDIR:-/tmp}/docket-commit-scope.XXXXXX")"
trap 'rm -rf "$FIXDIR"' EXIT
cat > "$FIXDIR/fixture-controls.sh" <<'FIXTURE_EOF'
$GIT -C "$treeonly" commit-tree "$t" -m "writes an object, has no index"
$GIT -C "$naked" commit -q -m "no pathspec anywhere on this line"
$GIT -C "$fixed" commit -q -m "properly scoped" -- "$path"
$GIT -C "$sharedline" add -- "$f" && $GIT -C "$sharedline" commit -q -m "no pathspec of its own"
$GIT -C "$semiline" add -- "$f" ; $GIT -C "$semiline" commit -q -m "no pathspec of its own"
# $GIT -C "$commented" commit -q -m "prose about an unscoped commit, not an executable one"
FIXTURE_EOF
FIXTURE="$(scan_commits "$FIXDIR")"
assert "A: commit-tree is NOT reported as a commit (exact-token subcommand match)" \
  '! grep -q ":treeonly " <<<"$FIXTURE"'
assert "A: a pathspec-less commit IS reported unscoped (the detector is not inert)" \
  'grep -q "^fixture-controls.sh:naked unscoped " <<<"$FIXTURE"'
assert "A: a pathspec-bearing commit is reported scoped, not unscoped" \
  'grep -q "^fixture-controls.sh:fixed scoped " <<<"$FIXTURE"'
# The #0083 mark-path shape: an unscoped commit sharing a logical line with a scoped neighbour.
# A whole-line predicate finds the neighbour's `--` and clears the commit — the false NEGATIVE that
# per-segment evaluation exists to prevent, and the one it cannot be mutation-proved without.
assert "A: an unscoped commit sharing a line with a scoped neighbour is still caught (per-segment)" \
  'grep -q "^fixture-controls.sh:sharedline unscoped " <<<"$FIXTURE"'
# Same shape across a `;` rather than `&&`. Two separate controls because the splitter has two arms
# (two-character `&&`/`||` and one-character `;`/`|`), and one control leaves the other arm free to
# be deleted without reddening anything.
assert "A: the same shape across a semicolon is caught too (both splitter arms are live)" \
  'grep -q "^fixture-controls.sh:semiline unscoped " <<<"$FIXTURE"'
# Prose is not executable text, and a comment discussing an unscoped commit must not be reported —
# the guard would otherwise be unfixable by the only means available for a comment: rewording it.
assert "A: a commented-out unscoped commit is not reported (comments are prose)" \
  '! grep -q ":commented " <<<"$FIXTURE"'

exit $fail
