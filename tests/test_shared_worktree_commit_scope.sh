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
#                              metadata-writing skill body — and every reference file a skill body
#                              DELEGATES a shared-tree commit to — carries the marker. Task-5 scope.
# Both live in ONE file on purpose: a second file would split the exception lists for one invariant.
#
# CONTRACT BOUNDARY: this detects `commit` as an exact-token subcommand under an EXPLICIT driver
# set. It is not, and must not become, a general shell parser. A commit issued through a driver
# spelling outside the set is outside the contract (accepted limit — the set is small because the
# repo's metadata-writing drivers are). Introducing a new driver means extending DRIVERS in the
# same change: a review obligation, not something the guard infers.
#
# GROUP B's two accepted limits, stated rather than papered over:
# (a) Only two of the seven skills have a commit-bearing heading, so B2 is a FILE-LEVEL token check
#     for the other five: the marker could sit anywhere in the file and pass. Scoping to a heading
#     would silently skip five of seven, which is worse; the realistic drift — a marker deleted or
#     reflowed away — is still caught. B2c below inherits the same file-level check for the same
#     reason.
# (b) A file that grows a SECOND commit site is covered by that file's single marker.
# Both need contrived prose to exploit. A THIRD limit is NOT accepted and is closed by B2c: a skill
# body may DELEGATE its commit instruction to a reference file, and B2's `skills/*/SKILL.md` glob
# cannot reach one — so the dispatching body's own marker would sit on a different commit and pass
# while the instruction actually being followed goes unmarked.
# Run: bash tests/test_shared_worktree_commit_scope.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

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
  local seg="$1" tok driver_seen=0 subcmd="" glob_was_off=0
  # Word-split the segment WITHOUT pathname expansion: a `*` in a scanned line would otherwise be
  # globbed against the process CWD, making the token stream depend on the filesystem the guard
  # runs from. Restore the caller's setting rather than leaving -f set — this file has code after
  # these functions.
  case "$-" in *f*) glob_was_off=1 ;; esac
  set -f
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
  [ "$glob_was_off" -eq 1 ] || set +f
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
  local seg="$1" prev="" tok glob_was_off=0
  # Same no-glob word split as segment_is_commit_call, and the same restore.
  case "$-" in *f*) glob_was_off=1 ;; esac
  set -f
  for tok in $seg; do
    if [ "$prev" = "-C" ]; then
      tok="${tok#\"}"; tok="${tok%\"}"; tok="${tok#\$}"; tok="${tok#\{}"; tok="${tok%\}}"
      [ "$glob_was_off" -eq 1 ] || set +f
      printf '%s' "$tok"; return 0
    fi
    prev="$tok"
  done
  [ "$glob_was_off" -eq 1 ] || set +f
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

# --- the reflow-proof matcher --------------------------------------------------------------------
# Byte-identical to the three existing copies (test_docket_review.sh, test_gate_execution_posture.sh,
# test_loop_continuation.sh). #0253 is hoisting these into a sourced tests/lib/prose_guard.sh — when
# it merges, replace this definition with the source; it is the consolidation target for this copy.
# Without it, a pure re-flow of the guarded sentence across a line break reddens a policy assert
# about policy that never changed (learnings: phrase-grep-over-wrapped-prose).
#
# Always flattened ONCE into a variable and matched with a here-string, never `flatten < f | grep -q`:
# under this file's `set -o pipefail` that is a producer piped into an early-exiting consumer, `tr`
# takes SIGPIPE, and the assert goes intermittently red (AGENTS.md, Shell).
flatten(){ tr -s '[:space:]' ' '; }

MARKER='Stage by explicit path'

# --- group B1: the convention states the rule at the grant ---------------------------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
assert "B1: convention SKILL.md exists" '[ -f "$CONV" ]'
# Scope to the Step-0 preamble section — the same awk-range idiom test_skill_handoff_precedence.sh
# uses — so a stray mention elsewhere in a 6000-word file cannot satisfy these.
PREAMBLE="$(awk '/^### Step-0 preamble/{f=1;next} f&&/^### /{exit} f' "$CONV" | flatten)"
assert "B1: the Step-0 preamble section is non-empty (extractor floor)" '[ -n "$PREAMBLE" ]'
assert "B1: the preamble still grants direct git plumbing (the sentence the rule attaches to)" \
  'grep -qF -- "stays direct" <<<"$PREAMBLE"'
assert "B1: the preamble carries the marker" 'grep -qF -- "$MARKER" <<<"$PREAMBLE"'
# BIND the rule to its subject with a BOUNDED gap: a guard that only proves the words are present
# survives a rewrite that keeps them and severs them from the shared tree they are about
# (learnings: prose-guard-binds-phrase-to-claim). The gap is `[^.]{0,120}` — sentence-local and
# short — never `.*`: an unbounded gap re-binds across paragraphs and re-admits the drift this
# assert exists to catch. 120 also stays under BSD grep's 255-repetition ceiling.
assert "B1: the pathspec rule is bound to the SHARED tree, not floating" \
  'grep -qiE "shared[^.]{0,120}(stage|explicit path|add -A)|((stage|explicit path|add -A)[^.]{0,120}shared)" <<<"$PREAMBLE"'
# `-e` is not decoration: a pattern that could ever lead with `--` is parsed as an option (exit 2),
# and inside a negated assert that error inverts into a permanently green, vacuous guard.
assert "B1: the preamble names the bare-add spelling it forbids" \
  'grep -qE -e "add -A" <<<"$PREAMBLE"'

# --- group B2: coverage over every metadata-writing skill ----------------------------------------
# Sites are DERIVED, never hand-listed (AGENTS.md: enumerated floor). A skill is in scope iff its
# body INVOKES `docket.sh preflight` — the convention's Step-0 preamble, which is what MAKES a skill
# an operating skill that reads and writes on metadata_branch. docket-convention is excluded as the
# rule's home.
#
# Keyed on the COMMAND STRING, not on prose describing it. The obvious predicate — "the body names
# the metadata working tree" — yields the same set today but is keyed on a SPELLING, which AGENTS.md
# forbids for exactly the reason visible in docket-adr: it already uses the variant "metadata tree"
# more often than the canonical phrase, so an ordinary slim normalizing to its own house idiom would
# silently drop it from coverage — a false green in the one channel this group exists to guard.
# `docket.sh preflight` / `docket repository prepare` is a literal invoked command, immune to both
# reflow and rewording. change 0377 Task 9 migrated docket-status and docket-adr from the frozen
# `docket.sh preflight` facade to the typed `docket repository prepare` Step-0 command; the derivation
# matches EITHER Step-0 command so all seven operating skills stay in scope through the migration window.
IN_SCOPE="$(grep -lE 'docket\.sh preflight|docket repository prepare' "$REPO"/skills/*/SKILL.md 2>/dev/null \
            | grep -v '/docket-convention/' | sort)"
assert "B2: the derivation found in-scope skills (extractor floor)" '[ -n "$IN_SCOPE" ]'
assert "B2: the derivation yields exactly 7 metadata-writing skills (found $(grep -c . <<<"$IN_SCOPE"))" \
  '[ "$(grep -c . <<<"$IN_SCOPE")" -eq 7 ]'

covered=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  sk="$(basename "$(dirname "$f")")"
  hay="$(flatten < "$f")"
  covered=$((covered+1))
  # RE-KEYED (0316, category (a)): docket-finalize-change reads the shared tree via `docket.sh
  # preflight` (so it stays in scope AND in B2b's worktree-scope:metadata cross-check) but no longer
  # STAGES OR COMMITS by hand — every metadata write is a Go transaction (`docket finalize <verb>`)
  # that stages by explicit path inside the binary. The 'Stage by explicit path' SKILL marker guards
  # a hand-authored commit and does not apply to it; assert instead that the skill delegates
  # committing to the Go verbs and writes no metadata by hand. Authority #2: Go transactions own
  # committing (the skill stages nothing).
  if [ "$sk" = docket-finalize-change ]; then
    assert "B2: $sk commits via Go transactions, not a hand commit (skill writes no metadata by hand)" \
      'grep -qF -- "writes metadata by hand" <<<"$hay"'
    continue
  fi
  # RE-KEYED (0369, category (a), same class as docket-finalize-change above): docket-adr's
  # Create / Supersede / Reverse — the ADR-ledger writes — migrated to the
  # `docket adr record|supersede|reverse --request -` Go transactions, which stage and commit by
  # explicit path INSIDE the binary; the hand `git add`/commit those steps used to carry (and with
  # it the 'Stage by explicit path' marker) was deleted. The marker guarded that hand-authored
  # commit and no longer applies to the migrated record path, so — like docket-finalize-change —
  # assert instead that the record write delegates committing to the Go verb, keyed on the invoked
  # COMMAND STRING (immune to reflow/rewording, per this group's rationale above). docket-adr is
  # NOT asserted with the identical "writes no metadata by hand" phrase because it truthfully
  # retains two rare hand-metadata paths outside the record path — the `## Update` note append and
  # the frozen stale-index render — so the blanket claim would be a false guard.
  if [ "$sk" = docket-adr ]; then
    assert "B2: $sk commits the ADR record via a Go transaction, not a hand commit" \
      'grep -qF -- "docket adr record --request" <<<"$hay"'
    continue
  fi
  assert "B2: $sk carries the marker at its commit instruction" \
    'grep -qF -- "$MARKER" <<<"$hay"'
done <<<"$IN_SCOPE"
assert "B2: every in-scope skill was actually checked (covered=$covered = 7)" '[ "$covered" -eq 7 ]'

# Skills that must NOT be in scope: their commits are feature-branch, in a per-change worktree that
# is not shared. Including them would imply the shared-tree hazard applies there and dilute the
# rule's reason (spec Assumption 13). Each is floored by an existence check first: a negative assert
# on a skill that has since been renamed passes for the wrong reason.
for out in docket-build docket-build-task docket-review docket-brainstorm; do
  assert "B2: out-of-scope control '$out' is a real skill (the negative below is not vacuous)" \
    '[ -f "$REPO/skills/$out/SKILL.md" ]'
  assert "B2: $out is correctly OUT of scope (feature worktree, not the shared tree)" \
    '! grep -q "/$out/SKILL.md$" <<<"$IN_SCOPE"'
done

# --- group B2b: cross-check against change 0208's DECLARED worktree-scope (ADR-0083) -------------
# Do not mint a second notion of scope. 0208 already established `worktree-scope:` as a declared
# frontmatter fact on agents/docket-*.md, with exactly two values. This is the reverse direction the
# forward loop above is structurally blind to (learnings: correspondence-guard-runs-one-way): every
# agent source declaring metadata scope whose skills: list names a docket operating skill must
# appear in the derived set. Five of the seven have wrappers; the other two are interactive and
# wrapper-less by construction, which is why this is a floor and not an equality.
#
# The wrapped skill is found by SHAPE — the first entry in `skills:` that is not docket-convention
# and that has a real SKILL.md — never by list position: docket-status declares
# `skills: [docket-status, docket-convention]` while docket-auto-groom-critic declares
# `skills: [docket-convention]` alone, so a position-keyed read is one reordering away from either
# a false green or a false red. An agent with no operating skill (docket-auto-groom-critic wraps
# only the convention; docket-brainstorm-consultant declares no `skills:` at all) is out of the
# population by construction, not by exception.
checked_scope=0
for src in "$REPO"/agents/docket-*.md; do
  [ -f "$src" ] || continue
  scope="$(sed -n '/^worktree-scope:/{s/^worktree-scope:[[:space:]]*//;p;q;}' "$src")"
  [ "$scope" = metadata ] || continue
  list="$(sed -n '/^skills:/{s/^skills:[[:space:]]*\[//;s/\].*//;p;q;}' "$src" | tr -d ' ')"
  wrapped=""
  for cand in ${list//,/ }; do
    [ -n "$cand" ] || continue
    [ "$cand" = docket-convention ] && continue
    [ -f "$REPO/skills/$cand/SKILL.md" ] || continue
    wrapped="$cand"; break
  done
  [ -n "$wrapped" ] || continue
  checked_scope=$((checked_scope+1))
  assert "B2b: '$wrapped' declares worktree-scope: metadata, so it must be in the derived set" \
    'grep -q "/$wrapped/SKILL.md$" <<<"$IN_SCOPE"'
done
assert "B2b: the 0208 cross-check reached its population (checked_scope=$checked_scope = 5)" \
  '[ "$checked_scope" -eq 5 ]'

# --- group B2c: coverage over the reference files a skill body delegates a commit TO --------------
# B2 above is structurally confined to `skills/*/SKILL.md` and cannot see a reference file. But
# `skills/docket-convention/references/terminal-close-out.md` is the DECLARED single source for two
# agent-authored commits into the shared tree — `docket-finalize-change` step 3 and `docket-status`'s
# Tier-A inline sweep are both sent there ("follow it exactly") — and its step 2 instructs a
# follow-on commit on `metadata_branch`. Half 3's whole evidential basis is that a standing rule
# loses to the SPECIFIC INSTRUCTION at the moment of action; for those commits that moment lives in
# the reference, so the dispatching body's marker sits on a DIFFERENT commit (finalize's step-2.5
# harvest) and satisfies B2 without covering the instruction being followed. That is a false green
# in the exact channel group B exists to guard.
#
# Population DERIVED, never hand-listed (AGENTS.md: enumerated floor), over every `references/*.md`
# under `skills/**`. In scope iff the file BINDS a git write verb to `metadata_branch`. Both halves
# are shape rather than phrasing: `metadata_branch` is the config KEY naming the shared branch — a
# literal identifier, as immune to reflow and house-idiom drift as B2's `docket.sh preflight` — and
# commit/push are the git operations themselves, not a way of describing them.
#
# BOUND, with the same sentence-local `[^.]{0,120}` window B1 uses, in both directions. Never `.*`:
# an unbounded gap admits every file that merely mentions the branch somewhere, which would sweep in
# `learnings.md` and `edge-paths.md` (controlled below) and dilute the rule exactly as the
# feature-branch skills would. 120 also stays under BSD grep's 255-repetition ceiling.
REF_BIND='metadata_branch[^.]{0,120}(commit|push)|(commit|push)[^.]{0,120}metadata_branch'

# select_commit_refs [ROOT] — emit the in-scope reference paths under ROOT, one per line. ROOT
# defaults to the repo's skills/ tree; the parameter exists so the synthetic controls below run
# through the SAME selector, exactly as scan_commits takes one for group A's fixture.
select_commit_refs(){
  local root="${1:-$REPO/skills}" f files hay
  files="$(find "$root" -path '*/references/*.md' -type f | sort)"
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    hay="$(flatten < "$f")"
    # `-e` is not decoration here either: see the B1 note above.
    grep -qiE -e "$REF_BIND" <<<"$hay" || continue
    printf '%s\n' "$f"
  done <<<"$files"
}

REFS="$(find "$REPO"/skills -path '*/references/*.md' -type f | sort)"
assert "B2c: reference files were found at all (extractor floor)" '[ -n "$REFS" ]'
REF_SCOPE="$(select_commit_refs)"
# COUNT FLOOR — without it the selector may silently degrade to matching nothing and the loop below
# becomes a vacuous zero-iteration guard, which is this whole file's named failure mode.
assert "B2c: the derivation selected reference files (selector floor)" '[ -n "$REF_SCOPE" ]'
# RE-BASELINED (0316, category (a)): pre-0316 TWO references bound a shared-tree commit —
# terminal-close-out.md (docket-status's sweep) and docket-finalize-change/references/gate-failure.md
# (finalize's marker write). The Go-sequencer rewrite made finalize's marker write a `docket finalize
# block` transaction, so gate-failure.md no longer instructs a hand commit on metadata_branch and
# drops out of scope. One commit-instructing reference remains (terminal-close-out.md, for
# docket-status's still-Bash sweep). Authority #2: Go transactions own committing.
assert "B2c: the derivation yields exactly 1 commit-instructing reference (found $(grep -c . <<<"$REF_SCOPE"))" \
  '[ "$(grep -c . <<<"$REF_SCOPE")" -eq 1 ]'

covered_refs=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  rel="${f#$REPO/}"
  hay="$(flatten < "$f")"
  covered_refs=$((covered_refs+1))
  assert "B2c: $rel carries the marker at its commit instruction" \
    'grep -qF -- "$MARKER" <<<"$hay"'
done <<<"$REF_SCOPE"
assert "B2c: every in-scope reference was actually checked (covered_refs=$covered_refs = 1)" \
  '[ "$covered_refs" -eq 1 ]'

# References that NAME `metadata_branch` but instruct no commit there must stay OUT: `learnings.md`
# states where `promotion_state:` lives, `edge-paths.md` states what a PR back-link points AT.
# Marking them would assert a shared-tree commit obligation at a site that performs none. Existence
# is floored first — a negative assert on a renamed file passes for the wrong reason.
for outref in docket-convention/references/learnings.md docket-implement-next/references/edge-paths.md; do
  assert "B2c: out-of-scope control '$outref' is a real file (the negative below is not vacuous)" \
    '[ -f "$REPO/skills/$outref" ]'
  assert "B2c: '$outref' names the branch but instructs no commit there, so it is correctly OUT" \
    '! grep -q "/$outref$" <<<"$REF_SCOPE"'
done

# POSITIVE CONTROLS, SYNTHETIC — the selector's two arms and its bound have no complete live
# witness, and an undemonstrable design decision is indistinguishable from decoration (same
# reasoning, and the same fixture directory, as group A's synthetic controls). Mutation-checked:
# deleting either arm reddens that arm's own assert, and widening `[^.]{0,120}` to `.*` reddens the
# bound's. The live pair does NOT supply this — BOTH repo files happen to satisfy
# the forward arm, so the reverse arm survives deletion against live text alone while still being
# the arm that catches `docket-finalize-change`'s own house phrasing ("commit … on
# `metadata_branch`"), which a reference could adopt at any time.
REFDIR="$FIXDIR/skills/fixture-skill/references"
mkdir -p "$REFDIR"
printf '%s\n' 'Write the field on `metadata_branch`, then commit and push it.' > "$REFDIR/forward.md"
printf '%s\n' 'Commit the finding files together on `metadata_branch`.' > "$REFDIR/reverse.md"
printf '%s\n' 'The finding lives on `metadata_branch` and is read by the sweep, by the board renderer, by the mirror, and by the health checks, none of which write, so nothing here is a commit.' > "$REFDIR/farapart.md"
printf '%s\n' 'The change file lives on `metadata_branch`. Nothing is written here.' > "$REFDIR/mention.md"
REF_FIXTURE="$(select_commit_refs "$FIXDIR")"
assert "B2c: 'metadata_branch … commit' IS selected (forward arm is live)" \
  'grep -q "/forward.md$" <<<"$REF_FIXTURE"'
assert "B2c: 'commit … metadata_branch' IS selected (reverse arm is live)" \
  'grep -q "/reverse.md$" <<<"$REF_FIXTURE"'
assert "B2c: a commit-word beyond the sentence-local window is NOT selected (the bound is live)" \
  '! grep -q "/farapart.md$" <<<"$REF_FIXTURE"'
assert "B2c: a bare mention across a sentence boundary is NOT selected" \
  '! grep -q "/mention.md$" <<<"$REF_FIXTURE"'

exit $fail
