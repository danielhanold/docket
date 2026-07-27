#!/usr/bin/env bash
# tests/test_config_read_channel.sh — run: bash tests/test_config_read_channel.sh
# Guards ADR-0052's read-channel rule AT THE SKILL-PROSE LAYER (change 0120): a documented config
# key resolves through docket-config.sh and skills read the EXPORTED value from the Step-0
# `preflight` block; a model-read of the config file is not a supported shape. ADR-0052's other
# enforcer, tests/test_docket_example_yml.sh, guards the KEY side (every documented key is wired);
# this one guards the PROSE side (no skill tells an agent to read the file).
#
# SHAPE, NOT SPELLING (AGENTS.md). The reject rule is "an unclassified occurrence of the config
# filename", not "no line says 'resolved from ...'" — an enumerated spelling misses the next
# phrasing, which is exactly how occurrence #2 shipped after ADR-0052 stated the rule. The
# admissible half is CLOSED and declared AT THE SITE:
#
#     <!-- docket:config-read-channel: write-back -->   the line describes a WRITE to the file
#     <!-- docket:config-read-channel: negative -->     the line says the file is NOT read that way
#
# The class is never inferred from wording: docket-status/SKILL.md's second occurrence ("record
# back into the change file / ...") names no key at all, so a "the line must name the written key"
# heuristic would redden on correct pre-existing prose.
#
# THE MARKER SITS ON THE SAME LINE AS THE OCCURRENCE. A position-sensitive attachment rule
# ("nearest preceding non-blank line") fails open the moment an edit inserts a blank line or moves
# the comment, and it reads green — attachment is failure mode 2 in the
# marker-scoped-guard-needs-a-population-floor learning. Same-line attachment moves the degree of
# freedom from POSITION to LINE CONTENT: a line can still gain a second, unmarked occurrence of the
# token while keeping its one existing marker. scan_tree closes that by counting occurrences and
# markers PER LINE and requiring them equal — a line short by even one marker is unclassified.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Built by parts so this file's own source is not an occurrence of the token it scans for.
TOKEN=".docket$(printf '')".yml
MARKER_RE='<!-- docket:config-read-channel: (write-back|negative) -->'

# EXCLUSIONS — declared, each with its reason. Both files ARE the contract that describes the
# config file itself, so a read-channel rule cannot apply to them as written:
#   skills/docket-convention/SKILL.md — defines the config file, its schema, and its layers.
#   skills/docket-convention/references/agent-layer.md — describes the config LAYERING itself.
# Re-examined at change 0120's reconcile, not waved through wholesale — three loose-provenance
# lines in docket-convention/SKILL.md were read, not just the one quoted below:
#   - "Read at startup by every docket skill." (the config section's opening line) and the
#     `# .docket.yml — ... read by every docket skill at startup` yaml comment inside its example
#     block both describe the file being read WITHOUT naming who performs the read.
#   - "`integration_branch` is a value *read from* the file, so the file cannot be located *by*
#     it" is a statement about WHERE the config file LIVES (default branch, not integration
#     branch), not about a read mechanism.
# All three stand unmarked because the same file closes the loop lower down: "performed
# deterministically by the config resolver (`docket-config.sh --export`)" — the only read channel
# this file ever attributes is the resolver, never an agent parsing the file directly. None of the
# three instructs an agent to parse anything itself.
EXCLUDE="
skills/docket-convention/SKILL.md
skills/docket-convention/references/agent-layer.md
"

# scan_tree <root> — the single classifier. The real run and EVERY mutation fixture go through this
# function; a fixture exercising a re-implementation would prove nothing about the rule that ships.
# It emits a record per scanned FILE as well as per occurrence, so the caller can assert on the
# POPULATION and not only on the verdicts (learning: backstop-must-compute-not-reenumerate —
# mutation-test the population, because a scan that reaches nothing yields zero findings and reads
# identical to a clean tree).
scan_tree(){
  local root="$1" f rel line n occ m cls
  local -a markers
  while IFS= read -r f; do
    rel="${f#"$root"/}"
    case "$EXCLUDE" in
      *"
$rel
"*) continue ;;
    esac
    printf 'file\t%s\n' "$rel"
    n=0
    while IFS= read -r line || [ -n "$line" ]; do
      n=$((n+1))
      case "$line" in *"$TOKEN"*) ;; *) continue ;; esac
      # Count OCCURRENCES of the token and MARKERS on this same line and require them equal — a
      # line with 2 occurrences and 1 marker must fail open no longer: it is reported unclassified,
      # not admitted on the strength of the one marker it happens to carry (Finding 1).
      occ="$(grep -oF -- "$TOKEN" <<<"$line" | wc -l | tr -d ' ')"
      markers=()
      while IFS= read -r m; do
        [ -n "$m" ] && markers+=("$m")
      done < <(grep -oE -- "$MARKER_RE" <<<"$line")
      if [ "${#markers[@]}" -eq "$occ" ]; then
        for m in "${markers[@]}"; do
          cls="${m#*: }"; cls="${cls% -->}"
          printf 'ok\t%s\t%s\t%s\n' "$rel" "$n" "$cls"
        done
      else
        printf 'unclassified\t%s\t%s\t%s\n' "$rel" "$n" "$line"
      fi
    done < "$f"
  done < <(find "$root/skills" -name '*.md' | sort)
}

# --- (1) THE REAL TREE -------------------------------------------------------
out="$(scan_tree "$REPO")"
files="$(grep    -c "$(printf '^file\t')"         <<<"$out")"
oks="$(grep      -c "$(printf '^ok\t')"           <<<"$out")"
unclassified="$(grep "$(printf '^unclassified\t')" <<<"$out")"

# Population floors. A glob that matches nothing, an exclusion list that swallows the tree, or a
# reader that finds no occurrences must NOT read as green — each of those yields zero unclassified
# findings, which is byte-identical to a clean tree.
assert "population: at least 10 skill files scanned (got $files)" '[ "$files" -ge 10 ]'
assert "population: the finalize skill is in the scanned set" \
  'grep -q -- "$(printf "^file\tskills/docket-finalize-change/SKILL.md$")" <<<"$out"'
assert "population: excluded file skills/docket-convention/SKILL.md is NOT scanned" \
  '! grep -q -- "$(printf "^file\tskills/docket-convention/SKILL.md$")" <<<"$out"'
assert "population: excluded file skills/docket-convention/references/agent-layer.md is NOT scanned" \
  '! grep -q -- "$(printf "^file\tskills/docket-convention/references/agent-layer.md$")" <<<"$out"'
assert "population: at least 4 occurrences were reached and classified (got $oks)" '[ "$oks" -ge 4 ]'

# Coverage: BOTH admissible classes are actually exercised by the real tree, so neither arm of the
# classifier is dead code. "At least one occurrence is marked" pins a population, never coverage.
assert "coverage: at least one write-back occurrence exists" \
  'grep -q -- "$(printf "^ok\t")" <<<"$(grep -- "write-back$" <<<"$out")"'
assert "coverage: at least one negative occurrence exists" \
  'grep -q -- "$(printf "^ok\t")" <<<"$(grep -- "negative$" <<<"$out")"'

# THE RULE.
assert "every occurrence in a scanned skill file is classified
$unclassified" '[ -z "$unclassified" ]'

# --- (2) MUTATION FIXTURES ---------------------------------------------------
# Against tmpdir copies, never the real tree.
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkfix(){ mkdir -p "$tmp/$1/skills/x"; }

# (a) the bad clause, unmarked => REJECTED. Note this fixture is a REGRESSION SPECIMEN, not the
# rule: the guard rejects it because it is unclassified, not because of how it is worded.
mkfix a
printf 'merge it into `<integration_branch>` (resolved from `%s`; not hard-coded `main`).\n' \
  "$TOKEN" > "$tmp/a/skills/x/SKILL.md"
outa="$(scan_tree "$tmp/a")"
assert "mutation (a): an unmarked occurrence of the bad clause is REJECTED" \
  'grep -q -- "$(printf "^unclassified\tskills/x/SKILL.md\t1\t")" <<<"$outa"'

# (b) one MARKED occurrence of each admissible class => PASSES, non-vacuously.
# The positive control is a marked occurrence, NOT the corrected clause: the corrected clause
# contains no occurrence of the token at all, so it would pass while proving nothing.
mkfix b
{ printf 'writes it back into `%s` on the default branch <!-- docket:config-read-channel: write-back -->\n' "$TOKEN"
  printf 'read from the Step-0 block, never by parsing `%s` <!-- docket:config-read-channel: negative -->\n' "$TOKEN"
} > "$tmp/b/skills/x/SKILL.md"
outb="$(scan_tree "$tmp/b")"
assert "mutation (b): marked occurrences yield NO unclassified findings" \
  '[ -z "$(grep -- "$(printf "^unclassified\t")" <<<"$outb")" ]'
assert "mutation (b) is non-vacuous: both marked occurrences were actually reached" \
  '[ "$(grep -c -- "$(printf "^ok\t")" <<<"$outb")" = 2 ]'
assert "mutation (b): the write-back class is read off the marker" \
  'grep -q -- "$(printf "^ok\tskills/x/SKILL.md\t1\twrite-back$")" <<<"$outb"'
assert "mutation (b): the negative class is read off the marker" \
  'grep -q -- "$(printf "^ok\tskills/x/SKILL.md\t2\tnegative$")" <<<"$outb"'

# (c) the SAME occurrences with their markers stripped => REJECTED. This is the load-bearing pair
# with (b): it proves the marker, not the sentence, is what admits the line.
mkfix c
sed 's/ <!-- docket:config-read-channel: [a-z-]* -->//' "$tmp/b/skills/x/SKILL.md" > "$tmp/c/skills/x/SKILL.md"
outc="$(scan_tree "$tmp/c")"
assert "mutation (c) is non-vacuous: the marker strip actually changed the fixture" \
  '! cmp -s "$tmp/b/skills/x/SKILL.md" "$tmp/c/skills/x/SKILL.md"'
assert "mutation (c): stripping the markers REJECTS both occurrences" \
  '[ "$(grep -c -- "$(printf "^unclassified\t")" <<<"$outc")" = 2 ]'

# (d) an UNKNOWN class => REJECTED. Keeps the admissible set closed: a future author cannot widen
# the guard by inventing a class name at the site it is meant to constrain.
mkfix d
printf 'some prose about `%s` <!-- docket:config-read-channel: because-i-said-so -->\n' \
  "$TOKEN" > "$tmp/d/skills/x/SKILL.md"
outd="$(scan_tree "$tmp/d")"
assert "mutation (d): an unknown marker class is REJECTED" \
  'grep -q -- "$(printf "^unclassified\tskills/x/SKILL.md\t1\t")" <<<"$outd"'

# (e) the exclusion list is not a blanket: a NON-excluded file under the same directory prefix is
# still scanned. Guards against an exclusion match that is accidentally a prefix match.
mkdir -p "$tmp/e/skills/docket-convention/references"
printf 'unmarked `%s`\n' "$TOKEN" > "$tmp/e/skills/docket-convention/SKILL.md"
printf 'unmarked `%s`\n' "$TOKEN" > "$tmp/e/skills/docket-convention/references/learnings.md"
oute="$(scan_tree "$tmp/e")"
assert "mutation (e): an excluded file is skipped" \
  '! grep -q -- "$(printf "^file\tskills/docket-convention/SKILL.md$")" <<<"$oute"'
assert "mutation (e): a NON-excluded sibling is still scanned and rejected" \
  'grep -q -- "$(printf "^unclassified\tskills/docket-convention/references/learnings.md\t1\t")" <<<"$oute"'

# (f) TWO occurrences on one already-marked line but only ONE marker => REJECTED. This is the
# reviewer's reproduction: a later edit adds a new violating occurrence into an already-marked
# paragraph and the whole-line admit rule stayed green. scan_tree must count occurrences vs
# markers PER LINE and reject on a shortfall.
mkfix f
printf 'resolved from `%s`, never from `%s` <!-- docket:config-read-channel: negative -->\n' \
  "$TOKEN" "$TOKEN" > "$tmp/f/skills/x/SKILL.md"
outf="$(scan_tree "$tmp/f")"
assert "mutation (f): two occurrences with only one marker on the line is REJECTED" \
  'grep -q -- "$(printf "^unclassified\tskills/x/SKILL.md\t1\t")" <<<"$outf"'

# (g) the SAME two occurrences, each carrying its own marker => ACCEPTED, non-vacuously. Load-
# bearing pair with (f): proves the count-and-require-equal rule, not merely "any marker present".
mkfix g
printf 'resolved from `%s` <!-- docket:config-read-channel: negative -->, never from `%s` <!-- docket:config-read-channel: negative -->\n' \
  "$TOKEN" "$TOKEN" > "$tmp/g/skills/x/SKILL.md"
outg="$(scan_tree "$tmp/g")"
assert "mutation (g): the same line with two markers is ACCEPTED" \
  '[ -z "$(grep -- "$(printf "^unclassified\t")" <<<"$outg")" ]'
assert "mutation (g) is non-vacuous: both occurrences on the line were actually classified" \
  '[ "$(grep -c -- "$(printf "^ok\t")" <<<"$outg")" = 2 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
