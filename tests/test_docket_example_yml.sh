#!/usr/bin/env bash
# tests/test_docket_example_yml.sh — run: bash tests/test_docket_example_yml.sh
# Guards .docket.example.yml, docket's canonical all-comprehensive config reference (change 0101).
# The example is PURE DOCUMENTATION — no docket tooling reads it — so these tests are the only
# thing keeping it honest. Replaces tests/test_config_example.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
EX="$REPO/.docket.example.yml"
CFGSCRIPT="$REPO/scripts/docket-config.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# Hermetic: never read OR WRITE the dev machine's real global config. See the
# config-layer-write-and-read-hazards learning — this suite reaches ensure-global-config.sh.
export XDG_CONFIG_HOME="$tmp/xdg-void"
mkdir -p "$XDG_CONFIG_HOME/docket"
printf '#!/bin/sh\nprintf "GNU bash, version 5.2.0(1)-release\\n"\n' >"$tmp/fake-bash"
chmod +x "$tmp/fake-bash"
printf 'runtime:\n  bash: %s\n' "$tmp/fake-bash" >"$XDG_CONFIG_HOME/docket/config.yml"

# fixture builder: a clone with a bare origin, one commit on main (origin/HEAD -> main).
# Mirrors tests/test_docket_config.sh's mkrepo.
mkrepo(){
  local dir="$1" bare="$1.origin.git"
  git init --quiet --bare "$bare"
  git clone --quiet "$bare" "$dir" 2>/dev/null
  git -C "$dir" config user.email t@t.test
  git -C "$dir" config user.name  Test
  git -C "$dir" checkout --quiet -b main
  : > "$dir/README.md"
  git -C "$dir" add README.md
  git -C "$dir" commit --quiet -m init
  git -C "$dir" push --quiet -u origin main
  git -C "$dir" remote set-head origin -a >/dev/null 2>&1
}

assert ".docket.example.yml exists at repo root" '[ -f "$EX" ]'

# --- (1) FIDELITY: example == shipped defaults -------------------------------
# Copy the example in as .docket.yml on a fixture's default branch; the resolver's export must be
# BYTE-IDENTICAL to the same fixture with no config file at all. This proves (a) every active value
# equals the shipped default, (b) both `auto` sentinels resolve to the unset behavior, and (c) no
# active key in the example collides with the resolver's FLAT leaf-key reader.
mkrepo "$tmp/none"
mkrepo "$tmp/full"
cp "$EX" "$tmp/full/.docket.yml"
git -C "$tmp/full" add .docket.yml
git -C "$tmp/full" commit --quiet -m cfg
git -C "$tmp/full" push --quiet origin main

# Guard against the fidelity asserts passing VACUOUSLY: the fixture setup above (cp/add/commit/
# push) is unchecked and this suite runs with no -e, so if any one of those four commands silently
# failed, $tmp/full's origin/main would carry no .docket.yml, both sides would resolve as
# no-config, and the byte-identity assert below would go green while proving nothing. Prove the
# example actually reached the fixture's origin/main BEFORE trusting the comparison.
assert "fidelity fixture: example reached the fixture's origin/main" \
  'git -C "$tmp/full" show origin/main:.docket.yml 2>/dev/null | grep -q "^metadata_branch:"'

# --repo-dir differs between the two fixtures, and plain format emits absolute REPO_ROOT /
# METADATA_WORKTREE paths — normalize those two lines out before diffing.
norm(){ grep -vE '^(REPO_ROOT|METADATA_WORKTREE)=' ; }
exp_none="$(bash "$CFGSCRIPT" --repo-dir "$tmp/none" --export --format plain 2>/dev/null | norm)"
exp_full="$(bash "$CFGSCRIPT" --repo-dir "$tmp/full" --export --format plain 2>/dev/null | norm)"

assert "fidelity: export is non-empty (guard against both sides failing silently)" \
  '[ -n "$exp_none" ] && [ "$(printf "%s\n" "$exp_none" | wc -l)" -ge 15 ]'
assert "fidelity: example resolves byte-identically to no config at all" \
  '[ "$exp_none" = "$exp_full" ]'
if [ "$exp_none" != "$exp_full" ]; then
  echo "--- diff (no-config vs example-as-.docket.yml) ---"
  diff <(printf '%s\n' "$exp_none") <(printf '%s\n' "$exp_full") || true
  echo "---"
fi

# --- (2) COMPLETENESS: every schema key appears in the example ---------------
# Two sources, because export keys alone UNDER-COVER the schema (change 0101 reconcile).
#
# (2a) Exported keys: every KEY= the resolver emits maps to a YAML path in the example.
# The mapping lives here on purpose — a new export key with no entry fails this test, forcing
# the example AND this mapping to be updated in the same PR. That is the must-update rule's
# enforcement; the header prose is only its statement.
#
# Format: "EXPORT_KEY:yaml_regex". A leading '#' in the regex matches the commented form.
# Export keys that are DERIVED (not settable config) are skipped here so the loop below never
# asserts they're "mapped" to an example line. REPO_ROOT and METADATA_WORKTREE are ALSO already
# stripped out of $exp_none by norm() (above) before this loop ever sees them; they're listed
# here too as belt-and-braces in case norm() ever changes, distinct from DOCKET_MODE /
# DEFAULT_BRANCH / BOOTSTRAP, which are the genuinely derived-and-skipped keys this list exists
# for.
exported_skip="DOCKET_MODE DEFAULT_BRANCH METADATA_WORKTREE REPO_ROOT BOOTSTRAP"
map_for(){ # map_for <EXPORT_KEY> -> ERE matching the example's line, or empty if unmapped
  case "$1" in
    METADATA_BRANCH)       echo '^metadata_branch:[[:space:]]*docket' ;;
    INTEGRATION_BRANCH)    echo '^integration_branch:[[:space:]]*auto' ;;
    DOCKET_BASH_PATH)      echo '^#[[:space:]]+bash:[[:space:]]*/[^[:space:]]+' ;;
    CHANGES_DIR)           echo '^changes_dir:[[:space:]]*docs/changes' ;;
    ADRS_DIR)              echo '^adrs_dir:[[:space:]]*docs/adrs' ;;
    RESULTS_DIR)           echo '^results_dir:[[:space:]]*docs/results' ;;
    FINALIZE_GATE)         echo '^[[:space:]]+gate:[[:space:]]*local' ;;
    FINALIZE_TEST_COMMAND) echo '^[[:space:]]+test_command:[[:space:]]*auto' ;;
    FINALIZE_REQUIRE_PR_APPROVAL) echo '^[[:space:]]+require_pr_approval:[[:space:]]*false[[:space:]]*$' ;;
    LEARNINGS_ENABLED)     echo '^[[:space:]]+enabled:[[:space:]]*true' ;;
    LEARNINGS_CAP)         echo '^[[:space:]]+cap:[[:space:]]*300' ;;
    BOARD_SURFACES)        echo '^board_surfaces:[[:space:]]*\[[[:space:]]*inline[[:space:]]*\]' ;;
    AUTO_GROOM)            echo '^auto_groom:[[:space:]]*false' ;;
    AUTO_CAPTURE)          echo '^auto_capture:[[:space:]]*false' ;;
    TERMINAL_PUBLISH)      echo '^terminal_publish:[[:space:]]*false' ;;
    RECLAIM_LEASE_TTL)     echo '^[[:space:]]+lease_ttl:[[:space:]]*72' ;;
    RECLAIM_AUTO)          echo '^[[:space:]]+auto:[[:space:]]*false' ;;
    SKILL_BRAINSTORM)      echo '^[[:space:]]+brainstorm:[[:space:]]*superpowers:brainstorming' ;;
    SKILL_PLAN)            echo '^[[:space:]]+plan:[[:space:]]*superpowers:writing-plans' ;;
    SKILL_BUILD)           echo '^[[:space:]]+build:[[:space:]]*superpowers:subagent-driven-development' ;;
    SKILL_REVIEW)          echo '^[[:space:]]+review:[[:space:]]*superpowers:requesting-code-review' ;;
    SKILL_FINISH)          echo '^[[:space:]]+finish:[[:space:]]*superpowers:finishing-a-development-branch' ;;
    *) echo '' ;;
  esac
}

# Drive the loop off the resolver's ACTUAL export surface, never a hand-copied list.
for k in $(printf '%s\n' "$exp_none" | sed -n 's/^\([A-Z_][A-Z_0-9]*\)=.*/\1/p'); do
  case " $exported_skip " in *" $k "*) continue ;; esac
  re="$(map_for "$k")"
  assert "completeness: export key $k is mapped" '[ -n "$re" ]'
  [ -n "$re" ] && assert "completeness: $k present in example" 'grep -Eq "$re" "$EX"'
done

# (2b) THE CLASSIFICATION MANIFEST (change 0102).
# Every key documented in the example is classified in exactly one of two ways:
#
#   resolved:<EXPORT_NAME>   the resolver reads it; the test asserts that export is ACTUALLY
#                            emitted, so a manifest entry cannot claim an export that does not
#                            exist (nor survive one being removed).
#   elsewhere:<consumer>     deliberately not resolver-read, with a consumer named that mentions
#                            the key; the test greps that named file for it. Naming the consumer
#                            is what keeps this from decaying into an allowlist — per the
#                            correspondence-guard-runs-one-way learning, an allowlist answers
#                            "is this expected?" and never "does this exist?", which is the
#                            enumerated floor that let require_pr_approval ship documented-but-
#                            unwired in the first place. One entry (github_project, below) is the
#                            documented exception: its named file only FENCES the key rather than
#                            reading it, so for that entry alone the anchor proves less than the
#                            others — see the inline note on that arm.
#
# An UNCLASSIFIED key fails, naming itself as documented-but-unclassified. That is the direction
# that catches this bug class: a key added to the example with no resolution and no named reader.
#
# The mapping is explicit rather than derived because key -> export name is not 1:1
# (gate -> FINALIZE_GATE, enabled -> LEARNINGS_ENABLED, auto -> RECLAIM_AUTO,
# brainstorm -> SKILL_BRAINSTORM); any derivation would need this same table, hidden inside a
# transform instead of stated plainly.
#
# CORRESPONDENCE EXEMPTIONS (change 0102 whole-branch review, IMPORTANT 1): resolved: proves the
# named export is emitted SOMEWHERE, never that it belongs to THIS key — see the correspondence
# check below, which closes that gap for every entry except the ones named here.
#   BOARD_SURFACES — its value is built entirely through intermediate variables (bs_raw / bs /
#   _filtered; docket-config.sh's board_surfaces layer-resolution block). No `BOARD_SURFACES=`
#   assignment line ever contains the literal leaf key "board_surfaces" — the mechanical same-line
#   check below would false-red it.
#   DOCKET_BASH_PATH — the manifest key is the `runtime:` block header while its value is assigned
#   through block-scoped intermediates, so no assignment line can carry that header literally.
correspondence_exempt="BOARD_SURFACES DOCKET_BASH_PATH"
classify_key(){ # classify_key <example-key-name> -> "resolved:EXPORT" | "elsewhere:path" | ""
  case "$1" in
    runtime)              echo 'resolved:DOCKET_BASH_PATH' ;;
    metadata_branch)      echo 'resolved:METADATA_BRANCH' ;;
    integration_branch)   echo 'resolved:INTEGRATION_BRANCH' ;;
    changes_dir)          echo 'resolved:CHANGES_DIR' ;;
    adrs_dir)             echo 'resolved:ADRS_DIR' ;;
    results_dir)          echo 'resolved:RESULTS_DIR' ;;
    gate)                 echo 'resolved:FINALIZE_GATE' ;;
    test_command)         echo 'resolved:FINALIZE_TEST_COMMAND' ;;
    require_pr_approval)  echo 'resolved:FINALIZE_REQUIRE_PR_APPROVAL' ;;
    enabled)              echo 'resolved:LEARNINGS_ENABLED' ;;
    cap)                  echo 'resolved:LEARNINGS_CAP' ;;
    board_surfaces)       echo 'resolved:BOARD_SURFACES' ;;
    auto_groom)           echo 'resolved:AUTO_GROOM' ;;
    auto_capture)         echo 'resolved:AUTO_CAPTURE' ;;
    terminal_publish)     echo 'resolved:TERMINAL_PUBLISH' ;;
    lease_ttl)            echo 'resolved:RECLAIM_LEASE_TTL' ;;
    auto)                 echo 'resolved:RECLAIM_AUTO' ;;
    brainstorm)           echo 'resolved:SKILL_BRAINSTORM' ;;
    plan)                 echo 'resolved:SKILL_PLAN' ;;
    build)                echo 'resolved:SKILL_BUILD' ;;
    review)               echo 'resolved:SKILL_REVIEW' ;;
    finish)               echo 'resolved:SKILL_FINISH' ;;
    # Block headers carry no value of their own; their children are classified above.
    finalize|learnings|reclaim|skills|runners|codex) echo 'elsewhere:HEADER' ;;
    # Genuinely non-resolver-read keys, each with its real consumer named.
    #
    # github_project is the one exception to "real consumer": .docket.example.yml itself says
    # NOT WIRED TODAY — no script reads this key. docket-config.sh's only match is its
    # coordination-key FENCE list (warns-and-ignores the key in machine-scoped layers), not a
    # reader. This entry is accurate (the key really is unread) but the anchor is
    # documentation-only, unlike every other elsewhere: entry below.
    github_project)       echo 'elsewhere:scripts/docket-config.sh' ;;
    agents)               echo 'elsewhere:sync-agents.sh' ;;
    agent_harnesses)      echo 'elsewhere:sync-agents.sh' ;;
    sandbox)              echo 'elsewhere:scripts/runners/codex.sh' ;;
    network)              echo 'elsewhere:scripts/runners/codex.sh' ;;
    *) echo '' ;;
  esac
}

# is_header_key <key> <file> -> prints "1" iff the file contains a bare "<key>:" line (no value)
# that is ITSELF followed by a more-indented line — i.e. a genuine YAML block opener, not merely
# a valueless line occurring somewhere in the file. Backs the elsewhere:HEADER arm below and
# closes two escapes found in review: (a) DECOY DEFEAT — a stray bare "<key>:" line elsewhere in
# the file (this file already reuses short nested names like auto/plan/build/review/finish
# across blocks, so a same-named valueless line elsewhere is plausible) used to satisfy a
# whole-file grep regardless of WHICH occurrence matched; now the matching occurrence itself must
# open a block. (b) CHILDLESS ESCAPE — a genuinely childless bare key (nothing nested under it)
# used to pass as a "header" with zero consumer anchor, invisible to (2c)'s orphan check since
# that only walks unindented keys. Blank lines between a header and its first real child are
# skipped rather than read as "no child" (no real header in this file has one, but the scan
# tolerates it); indent is measured with the same [^[:space:]] idiom as flatten_yaml (below), so
# tabs are handled identically. Mutation-tested (task-4-report.md): all six real headers
# (codex/finalize/learnings/reclaim/runners/skills) still pass; relabeling require_pr_approval to
# elsewhere:HEADER still reddens; a bare childless "newsub:" injected under finalize: reddens too.
is_header_key(){
  awk -v k="$1" '
    { line[NR] = $0 }
    END {
      pat = "^[[:space:]]*" k ":[[:space:]]*$"
      found = 0
      for (i = 1; i <= NR && !found; i++) {
        if (line[i] !~ pat) continue
        ind = match(line[i], /[^[:space:]]/) - 1
        for (j = i + 1; j <= NR; j++) {
          if (line[j] ~ /^[[:space:]]*$/) continue
          cind = match(line[j], /[^[:space:]]/) - 1
          if (cind > ind) found = 1
          break
        }
      }
      if (found) print "1"
    }
  ' "$2"
}

# PRESENCE-SENSITIVE pseudo-keys: keys that ship COMMENTED because merely uncommenting them
# (even at their default values) changes behavior — see (3) below, which asserts their marker
# comment count. Named ONCE, here, as the single source (3)'s exact-count assert reads, so a
# third such key shipping commented forces (3) to be updated in the same commit — a marker
# comment with no name here (or vice versa) leaves the two counts mismatched, which reddens (3)
# instead of silently passing.
presence_sensitive_keys="agents agent_harnesses"

# COMMENTED CONFIG KEYS (change 0102 whole-branch review, IMPORTANT 2): this file ships
# documented-but-disabled keys in commented form — agents:/agent_harnesses: today, potentially
# others tomorrow — and a hardcoded name list here is blind to a NEW one: nothing forces its name
# into a list, so it needs no classify arm, no count bump, and trips nothing. Generalize instead
# of enumerating: every real commented key in this file is the line IMMEDIATELY following its own
# "# scope: repo-only ..." / "# scope: any layer ..." tag — the SAME tag every ACTIVE key carries
# (the file's own standing rule: "every key carries one" [scope tag]). A commented PROSE line that
# happens to end in "word:" (e.g. "# exceptions:", a sentence wrapped mid-line, or "# generation:",
# likewise) is never preceded by a scope tag, so it is not a false positive; neither is a nested
# commented sub-key inside the agents: example block (e.g. "#   claude:", "#     status: {...}")
# — none of those sit directly under a scope tag either, only agents:/agent_harnesses: do.
# Verified against the whole real file: extracts the three intentionally commented top-level keys
# {agent_harnesses, agents, runtime}, nothing else.
commented_config_keys(){  # commented_config_keys <file> -> one key name per line on stdout
  awk '
    /^[[:space:]]*#[[:space:]]*scope:[[:space:]]*(repo-only|any layer|local-only)/ { prev_scope=1; next }
    {
      if (prev_scope && match($0, /^[[:space:]]*#[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:/)) {
        line = $0
        sub(/^[[:space:]]*#[[:space:]]*/, "", line)
        sub(/:.*/, "", line)
        print line
      }
      prev_scope = 0
    }
  ' "$1"
}

# Collect every key the example documents: active keys at any nesting depth, PLUS the commented
# keys the discriminator above finds. Captured ONCE, raw (undeduped) — change 0102 whole-branch
# review, MINOR 3: the manifest loop below and the duplicate-leaf check further down are two
# DIFFERENT consumers of this same extraction; a duplicated pipeline let one copy drift (or go
# empty) with nothing catching it, since the old floor guarded only the line-240 copy. Both are
# now derived from this one variable, and its own non-vacuity floor (below, by expected_key_count)
# guards them both at once.
manifest_unclassified=""
manifest_bad_export=""
manifest_bad_correspondence=""
manifest_bad_consumer=""
manifest_bad_header=""
example_keys_raw="$(
  { sed -nE 's/^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*):.*/\1/p' "$EX"
    commented_config_keys "$EX"
  }
)"
example_keys="$(printf '%s\n' "$example_keys_raw" | sort -u)"

# Declared consumer allowlist for elsewhere: targets (change 0102 whole-branch review, IMPORTANT
# 1). Reused verbatim by (2c)'s orphan-key check below — same list, defined once. Anchoring an
# elsewhere: entry on a NAMED file only proves that file mentions the key; without a floor on
# WHICH files are legal targets, that anchor is satisfiable by ANY file that happens to mention
# the key — including .docket.example.yml itself (which documents every key by definition),
# README.md, or this test file's own case arms — collapsing "elsewhere:" into exactly the bare
# allowlist it exists to forbid (per the correspondence-guard-runs-one-way learning: an allowlist
# answers "is this expected?", never "does this exist?").
consumers="$CFGSCRIPT $REPO/sync-agents.sh $REPO/scripts/runner-dispatch.sh"
consumers="$consumers $REPO/skills/docket-finalize-change/SKILL.md $REPO/scripts/runners/codex.sh"
for k in $example_keys; do
  cls="$(classify_key "$k")"
  case "$cls" in
    '')
      manifest_unclassified="$manifest_unclassified $k"
      ;;
    resolved:*)
      exp_name="${cls#resolved:}"
      # The export must ACTUALLY be emitted — a manifest entry cannot claim a phantom export.
      grep -q "^$exp_name=" <<<"$exp_none" \
        || manifest_bad_export="$manifest_bad_export $k($exp_name)"
      # CORRESPONDENCE: the check above proves $exp_name is emitted, but nothing yet ties it back
      # to THIS key — a manifest entry could claim a REAL but UNRELATED export (e.g.
      # `finalize.notify_slack` classified as `resolved:METADATA_BRANCH`) and stay green, which is
      # the require_pr_approval bug reproduced verbatim: rename a key, copy-paste a resolved: arm
      # pointing at an existing export, touch nothing in docket-config.sh. Close it by requiring
      # docket-config.sh to assign $exp_name on a line that ALSO names the leaf key $k — the shape
      # every real entry has (e.g. FINALIZE_REQUIRE_PR_APPROVAL="$(lcl require_pr_approval)").
      # Anchored on lines that are themselves an assignment TO $exp_name (`^$exp_name=`), never a
      # whole-file grep for $exp_name — that would be satisfiable by a comment or an unrelated
      # mention nowhere near the real assignment. See correspondence_exempt (above) for the one
      # entry this mechanical check cannot reach.
      case " $correspondence_exempt " in
        *" $exp_name "*) ;;
        *)
          grep -qE "^$exp_name=.*\\b$k\\b" "$CFGSCRIPT" \
            || manifest_bad_correspondence="$manifest_bad_correspondence $k($exp_name not tied to $k in docket-config.sh)"
          ;;
      esac
      ;;
    elsewhere:HEADER)
      # A mapping opener carries no value of its own; its children carry the real
      # classification. But nothing else here verifies the key IS actually a bare block
      # opener — the HEADER label is otherwise an unverified escape hatch: appending a new,
      # unwired key to the case arm above would silence "documented key is classified" for it
      # with zero further checking. So require the shape a real header has: a bare "<key>:"
      # occurrence that is itself followed by a more-indented child line (see is_header_key
      # above for why a bare line alone is not enough).
      [ "$(is_header_key "$k" "$EX")" = "1" ] \
        || manifest_bad_header="$manifest_bad_header $k"
      ;;
    elsewhere:*)
      consumer="${cls#elsewhere:}"
      # The target itself must be a DECLARED consumer — never an arbitrary path. Without this,
      # the mention-grep below is satisfiable by pointing at ANY file that happens to mention the
      # key (see the rationale on $consumers above); this is what actually forbids that escape.
      allowlisted=1
      case " $consumers " in
        *" $REPO/$consumer "*) ;;
        *) allowlisted=0
           manifest_bad_consumer="$manifest_bad_consumer $k(target $consumer is not a declared consumer)" ;;
      esac
      # The NAMED consumer must actually mention the key — this is what keeps the entry anchored
      # on consuming code instead of decaying into a bare allowlist. Skipped when the allowlist
      # check above already failed (change 0102 whole-branch review, MINOR 4): the target is then
      # often not even a real path, so grepping it here only prints an unsuppressed "No such file
      # or directory" and adds a second, redundant failure entry for the same root cause.
      if [ "$allowlisted" -eq 1 ]; then
        grep -qE "\\b$k\\b" "$REPO/$consumer" \
          || manifest_bad_consumer="$manifest_bad_consumer $k(not in $consumer)"
      fi
      ;;
  esac
done

assert "manifest: every documented key is classified (${manifest_unclassified:-none unclassified})" \
  '[ -z "$manifest_unclassified" ]'
assert "manifest: every resolved: entry names a REAL export (${manifest_bad_export:-none bad})" \
  '[ -z "$manifest_bad_export" ]'
assert "manifest: every resolved: entry's export is tied back to its key (${manifest_bad_correspondence:-none bad})" \
  '[ -z "$manifest_bad_correspondence" ]'
assert "manifest: every elsewhere: entry's named consumer mentions the key (${manifest_bad_consumer:-none bad})" \
  '[ -z "$manifest_bad_consumer" ]'
assert "manifest: every elsewhere:HEADER entry is a real bare block opener (${manifest_bad_header:-none bad})" \
  '[ -z "$manifest_bad_header" ]'
# NON-VACUITY, EXACT COUNT not a floor: the loop above must actually iterate, AND classify_key
# carries exactly expected_key_count key TOKENS — not "expected_key_count arms": it is 28 case
# arms carrying 33 key tokens, because the header arm alone
# (finalize|learnings|reclaim|skills|runners|codex) carries six — so an extraction that drops
# keys must redden too. A loose floor (formerly -ge 20) does not do this: the dominant breakage
# (dropping the [[:space:]]* from the first sed, so nested keys vanish) yields 15 and IS caught,
# but losing the commented-key extraction entirely — the one picking up agents:/agent_harnesses:
# — yields 30 and passed the old floor SILENTLY. Those two keys are precisely the ones whose
# consumer anchor (elsewhere:sync-agents.sh) is otherwise untested, so that silent pass was a real
# hole, not a hypothetical one (mutation-tested: dropping commented_config_keys from the pipeline
# reddens this exact assert, via the raw floor directly below it). If you add a new documented
# key, bump expected_key_count in the same commit as classify_key's new arm — that is the
# intentional-growth remedy this count is guarding. This is the single source for that count: the
# condition and the failure message below both read it, so bumping it in one place updates both
# instead of leaving one stale.
expected_key_count=33
# RAW FLOOR (change 0102 whole-branch review, MINOR 3): example_keys_raw feeds BOTH this section's
# manifest loop (via example_keys, deduped) and the duplicate-leaf check directly below (also
# fed from example_keys_raw, undeduped). Without this assert, an edit that makes the raw pipeline
# emit nothing would silently starve the duplicate-leaf check (`uniq -d` on empty input is empty,
# which reads as "no duplicates" — green forever) even though mf_count's OWN assert below would
# already have caught the manifest-loop side. Asserted against the same expected_key_count rather
# than a second magic number: the raw list is always >= the deduped one.
raw_count="$(printf '%s\n' "$example_keys_raw" | grep -c .)"
assert "manifest: raw key extraction is non-vacuous (>= $expected_key_count; got $raw_count)" \
  '[ "$raw_count" -ge "$expected_key_count" ]'
mf_count="$(printf '%s\n' "$example_keys" | grep -c .)"
assert "manifest: key extraction count is exactly $expected_key_count (got $mf_count; if intentional, bump expected_key_count and add the key's classify_key arm in the same commit)" \
  '[ "$mf_count" = "$expected_key_count" ]'
# DUPLICATE LEAF NAMES: derived from the SAME example_keys_raw captured above (change 0102
# whole-branch review, MINOR 3 — previously a second, independently-maintained copy of the same
# two extraction commands, guarded by nothing of its own). sort -u (in example_keys, above) dedups
# by leaf name across the WHOLE file, so a newly documented key whose leaf name COLLIDES with an
# already-classified key is invisible to this entire (2b) section — classify_key answers for the
# OTHER key, mf_count never moves, nothing fires. Plausible drift, not contrived: `enabled` is the
# obvious name for any future subsystem toggle, and learnings.enabled already set that precedent.
# This also independently protects yaml_get's flat, leaf-name-only reader — finalize.gate /
# learnings.enabled / reclaim.auto are all read as bare leaf keys, never scoped within their
# block, so a genuine name collision here is a real MIS-RESOLUTION hazard, not just a
# documentation one: yaml_get's `head -n1` would pick whichever line happens to appear first in
# the file.
dup_leaf_keys="$(printf '%s\n' "$example_keys_raw" | sort | uniq -d)"
assert "no duplicate leaf key names in the example (${dup_leaf_keys:-none}; a new key must not reuse an existing leaf name — classify_key and yaml_get cannot tell them apart)" \
  '[ -z "$dup_leaf_keys" ]'

# The value-anchored asserts for the non-exported keys are retained from the pre-0102 (2b): the
# fidelity check in (1) is structurally blind to keys the resolver never emits, so without these
# a typo'd value that merely has the right value as a PREFIX ("auto" matching "automanaged",
# "true" matching "truthy") would pass silently. sandbox/network carry a trailing inline comment
# in the example, so their anchors allow one optionally.
assert "completeness: github_project present (auto sentinel)" \
  'grep -Eq "^github_project:[[:space:]]*auto[[:space:]]*$" "$EX"'
assert "completeness: agent_harnesses present (commented)" \
  'grep -Eq "^#[[:space:]]*agent_harnesses:[[:space:]]*\[[[:space:]]*claude[[:space:]]*\][[:space:]]*$" "$EX"'
assert "completeness: agents present (commented)" \
  'grep -Eq "^#[[:space:]]*agents:[[:space:]]*$" "$EX"'
runtime_example="$(sed -n '/^# runtime:$/,/^$/p' "$EX")"
assert "completeness: runtime.bash uses the nested commented shape" \
  'grep -qxE "#[[:space:]]+bash:[[:space:]]*/[^[:space:]]+" <<<"$runtime_example"'
assert "completeness: runners.codex.sandbox present" \
  'grep -Eq "^[[:space:]]+sandbox:[[:space:]]*workspace-write[[:space:]]*(#.*)?$" "$EX"'
assert "completeness: runners.codex.network present" \
  'grep -Eq "^[[:space:]]+network:[[:space:]]*true[[:space:]]*(#.*)?$" "$EX"'
assert "completeness: runners block header present" 'grep -Eq "^runners:" "$EX"'

# change 0102: require_pr_approval is now RESOLVER-read and global-able, so it carries the
# standard any-layer tag like its two finalize siblings. The pre-0102 example carried a bespoke
# three-line note asserting the opposite (repo-committed only, silently ignored elsewhere) —
# that text described a state that no longer exists, and this pair keeps it from coming back.
assert "0102: require_pr_approval carries the any-layer scope tag" \
  'awk "/^  # require_pr_approval/,/^  require_pr_approval:/" "$EX" | grep -qF "scope: any layer"'
assert "0102: the stale repo-committed-only note is gone" \
  '! grep -qF "read by the finalize SKILL BODY, not by the config" "$EX"'

# (2c) The INVERSE direction. (2a)/(2b) prove every key the code reads is documented; neither
# proves the converse, so without this the example can accrete keys NOTHING reads — a phantom key
# passes (2a) (the loop iterates export keys, not example keys), passes the fidelity diff (the
# resolver simply ignores it), and passes the scope-tag awk (satisfied by a neighbor's comment
# window). A key REMOVED from the resolver would likewise keep its documentation forever.
# Anchored on the CONSUMERS, not a hand-maintained allowlist, so it cannot drift on its own: every
# active top-level key in the example must appear in the resolver or one of the four non-resolver
# consumers. (Word-boundary grep — it proves the key name is KNOWN to a consumer, not that the read
# is correctly wired; github_project is the live proof of that gap and is annotated as such in the
# example itself.) $consumers is declared once, above in (2b) — the elsewhere: allowlist check
# there needs the identical list, so it is defined there and reused here verbatim.

# GUARD THE GUARD: a wrong path in $consumers below doesn't fail loudly — grep -qlE on a
# nonexistent file just errors into 2>/dev/null and the loop leans on whichever remaining
# files still happen to mention the key, so the orphan-keys assert can stay green while one
# whole consumer is silently absent from the check. Demonstrated: reverting one path to a
# known-wrong value left the suite green; a previous fix corrected exactly such a typo'd path,
# found by hand, not by this suite. Assert every listed path is a real file BEFORE trusting the
# loop below.
consumers_missing=""
for c in $consumers; do
  [ -f "$c" ] || consumers_missing="$consumers_missing $c"
done
assert "(2c) every consumer path exists (${consumers_missing:-none missing})" \
  '[ -z "$consumers_missing" ]'

orphan_keys=""
for k in $(sed -nE 's/^([A-Za-z_][A-Za-z0-9_]*):.*/\1/p' "$EX"); do
  # shellcheck disable=SC2086
  grep -qlE "\\b$k\\b" $consumers >/dev/null 2>&1 || orphan_keys="$orphan_keys $k"
done
assert "no orphan keys: every active top-level key is read by a consumer (${orphan_keys:-none})" \
  '[ -z "$orphan_keys" ]'

# runners.* is consumed by the runner-dispatch script family, not the resolver. Anchor on the
# PRODUCER so the example and its consumer cannot silently diverge (same shape as the
# require_pr_approval producer assert above).
assert "runners.codex.sandbox is still read by the codex adapter" \
  'grep -q "DOCKET_RUNNER_CFG_SANDBOX" "$REPO/scripts/runners/codex.sh"'

# change 0102: require_pr_approval is now RESOLVER-read. The skill still NAMES the policy (that is
# what the (2c) consumer grep anchors on), but it must obtain the VALUE from the Step-0 export
# block — never by parsing .docket.yml itself. The next two asserts are the sole-channel proof.
# Reviewed and replaced (task-3 review, finding 1): the original single "does not parse .docket.yml"
# assert required the key name and the framing string on the SAME line, which no line in this file
# has ever satisfied — it was vacuous on day one and stayed green under both a revert of the
# finalize SKILL's export-block sentence
# and a bolted-on fallback sentence (mutation-tested; see task-3-report.md). Replaced with two
# assertions anchored on the real positive/negative shape of the sole-channel contract:
assert "require_pr_approval is still named by the finalize skill body" \
  'grep -q "require_pr_approval" "$REPO/skills/docket-finalize-change/SKILL.md"'
# (finding 2) Anchored on the PROVENANCE clause in the finalize SKILL — the `require_pr_approval`
# sentence naming FINALIZE_REQUIRE_PR_APPROVAL as "the sole channel" — the sentence that actually
# tells
# the agent where the value comes from — not a bare "does FINALIZE_REQUIRE_PR_APPROVAL appear
# anywhere" check. FINALIZE_REQUIRE_PR_APPROVAL also appears in the SKILL's "Every value below is
# read from the Step-0 `preflight` export block" framing
# sentence, so an existence-anywhere grep stays green even if this provenance clause is deleted
# outright;
# this requires the full provenance phrase (mutation-tested against deleting the clause).
assert "0102: the finalize skill's provenance clause (the 'sole channel' sentence) ties FINALIZE_REQUIRE_PR_APPROVAL to the Step-0 export block" \
  'grep -Eq "reads its resolved value as.{0,60}FINALIZE_REQUIRE_PR_APPROVAL.{0,80}Step-0 export block.{0,60}sole channel" "$REPO/skills/docket-finalize-change/SKILL.md"'
# (finding 1a) Positive framing: the SKILL's export-block sentence states the sole-channel rule as
# "never by parsing
# .docket.yml", tied to the exported keys it names. Reverting the export-block sentence back to its
# pre-0102 framing
# ("Configured by `.docket.yml`:") removes this phrase entirely, reddening this assert.
assert "0102: the finalize skill states its sole channel positively (never by parsing .docket.yml)" \
  'grep -Eq "FINALIZE_REQUIRE_PR_APPROVAL.{0,20}never by parsing.{0,15}\.docket\.yml" "$REPO/skills/docket-finalize-change/SKILL.md"'
# (finding 1b) Negative guard: no bolted-on fallback sentence ("...fall back to reading
# require_pr_approval from .docket.yml") — the explicit no-fallback-by-design contract. The
# positive assert above cannot catch an ADDED fallback sentence (it would leave "never by parsing"
# untouched), so this is the second, independent mutation target.
assert "0102: the finalize skill documents no .docket.yml fallback for the key" \
  '! ( fb=$(grep -niE "fall(s|ing)?[ -]?back" "$REPO/skills/docket-finalize-change/SKILL.md"); grep -qiE "\.docket\.yml|require_pr_approval" <<<"$fb" )'

# The standing rule is STATED in the header (and enforced by the loop above).
assert "example header states the must-update rule" \
  'grep -Eqi "every new config flag lands in" "$EX"'
# Checks all four layers the header's numbered list names (repo-local, repo-committed, global,
# built-in), not just two of them — each anchor is the list marker itself ("N. <layer-name>"),
# unique to that one line, so dropping any single layer from the header flips this NOT OK.
assert "example documents the four layers" \
  'grep -qF "1. repo-local" "$EX" && grep -qF "2. repo-committed" "$EX" && grep -qF "3. global" "$EX" && grep -qF "4. built-in" "$EX"'

# Scope tags: both forms present, and every ACTIVE top-level key is individually tagged — a real
# per-key check, not just "the phrase occurs somewhere in the file" (which the two asserts below
# alone would only prove). The awk pass finds each active (uncommented) top-level key's own
# preceding comment "window" bounded by the nearest neighbor on either side among: a section
# banner (# ═══...), another active top-level key, or a commented pseudo-key (# agent_harnesses:
# / # agents:). A header key (a mapping-opener like `finalize:`, with nothing after the colon)
# extends its window forward through its nested body, since its scope lives on its children, not
# on the header line itself. A scalar key whose window comes up empty (no comment lines of its
# own, immediately adjacent to the previous active key with zero lines between) inherits that
# key's tag coverage — this is the changes_dir / adrs_dir / results_dir group, one shared comment
# block above all three.
assert "scope tag: repo-only form present"  'grep -qF "scope: repo-only (coordination-fenced, ADR-0019)" "$EX"'
assert "scope tag: any-layer form present"  'grep -qF "scope: any layer" "$EX"'
assert "scope tag: local-only form present" 'grep -qF "scope: local-only" "$EX"'
untagged_keys="$(awk '
  {
    content[NR] = $0
    is_active = ($0 ~ /^[A-Za-z_][A-Za-z0-9_]*:/)
    is_pseudo = ($0 ~ /^# (agent_harnesses|agents):/)
    is_banner = ($0 ~ /^#[[:space:]]*═══/)
    if (is_active || is_pseudo || is_banner) { nb++; bnd[nb] = NR }
    if (is_active) {
      nk++
      keyline[nk] = NR
      keytype[nk] = ($0 ~ /^[A-Za-z_][A-Za-z0-9_]*:[[:space:]]*$/) ? "H" : "S"
      bndidx[nk] = nb
    }
  }
  END {
    maxNR = NR
    for (k = 1; k <= nk; k++) {
      idx = bndidx[k]
      prevB = (idx > 1) ? bnd[idx-1] : 0
      winStart = prevB + 1
      if (keytype[k] == "H") {
        nextB = (idx < nb) ? bnd[idx+1] : (maxNR + 1)
        winEnd = nextB - 1
      } else {
        winEnd = keyline[k]
      }
      tagged = 0
      for (l = winStart; l <= winEnd; l++) {
        if (content[l] ~ /scope: repo-only \(coordination-fenced, ADR-0019\)/) tagged = 1
        if (content[l] ~ /scope: any layer/) tagged = 1
      }
      if (!tagged && k > 1 && winStart == keyline[k] && prevB == keyline[k-1]) {
        tagged = taggedEff[k-1]
      }
      taggedEff[k] = tagged
      if (!tagged) {
        name = content[keyline[k]]
        sub(/:.*/, "", name)
        print name
      }
    }
  }
' "$EX")"
assert "scope tag: every ACTIVE top-level key is individually tagged" '[ -z "$untagged_keys" ]'
if [ -n "$untagged_keys" ]; then
  echo "--- untagged top-level keys ---"
  printf '%s\n' "$untagged_keys"
  echo "---"
fi

# --- (3) PRESENCE-SENSITIVE keys ship COMMENTED ------------------------------
# Regression guard for a real break (change 0048): gating per-repo generation on file PRESENCE
# littered wrappers into change-tracking-only repos and flipped their --check from a no-op to
# failing. An ACTIVE agents:/agent_harnesses: header in this example would re-arm that hazard
# for anyone who copies the file wholesale. See the opt-in-signal-not-file-presence learning.
assert "no ACTIVE agents: header"          '! grep -Eq "^agents:[[:space:]]*$" "$EX"'
assert "no ACTIVE agent_harnesses: header" '! grep -Eq "^agent_harnesses:" "$EX"'
# Scoped to the commented agents: excerpt (through the real, ACTIVE runners: header that follows
# it): the whole-file pattern also matches runners.codex: (change 0079), which IS meant to ship
# active — a real false positive caught while writing this guard.
assert "no ACTIVE codex: header under agents:" \
  '! sed -n "/^# agents:$/,/^runners:$/p" "$EX" | grep -Eq "^[[:space:]]*codex:[[:space:]]*$"'
assert "no ACTIVE cursor: header under agents:" \
  '! sed -n "/^# agents:$/,/^runners:$/p" "$EX" | grep -Eq "^[[:space:]]*cursor:[[:space:]]*$"'
presence_sensitive_marker_count="$(grep -cF "PRESENCE-SENSITIVE: uncommenting this key changes behavior" "$EX")"
presence_sensitive_expected="$(printf '%s\n' $presence_sensitive_keys | grep -c .)"
assert "PRESENCE-SENSITIVE marker count is exactly $presence_sensitive_expected, matching presence_sensitive_keys ($presence_sensitive_keys; got $presence_sensitive_marker_count; a new commented PRESENCE-SENSITIVE key must add its name to presence_sensitive_keys near the top of (2b), in the same commit as its marker comment)" \
  '[ "$presence_sensitive_marker_count" = "$presence_sensitive_expected" ]'
# ...but the commented examples ARE present, so a user can find and enable them. codex/cursor
# sit under a DOUBLY-commented example block (disabled-within-disabled), so the pattern allows
# one optional extra '#' layer.
assert "commented codex example present"  'grep -Eq "^#[[:space:]]*#?[[:space:]]*codex:" "$EX"'
assert "commented cursor example present" 'grep -Eq "^#[[:space:]]*#?[[:space:]]*cursor:" "$EX"'

# --- (4) MIRROR EQUALITY: relocated ADR-0039 ---------------------------------
# The commented agents.claude block mirrors agents/docket-*.md wrapper frontmatter VALUE FOR
# VALUE. The wrappers LEAD; this file mirrors. Same field regex as sync-agents.sh's field_of(),
# so the test cannot accept a shape the real resolver would reject.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }
# The example's agent lines are COMMENTED, so strip a leading '# ' before matching.
ex_field(){ # $1=agent  $2=field(model|effort)
  local line
  line="$(sed -E 's/^[[:space:]]*#[[:space:]]?//' "$EX" | grep -E "^    $1:[[:space:]]" | head -n1)"
  printf '%s' "$line" | sed -nE "s/.*[{,[:space:]]$2[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p" | head -n1
}
for a in status adr brainstorm-consultant auto-groom auto-groom-critic \
         implement-next rebase-resolver integration-repair finalize-change; do
  w="$REPO/agents/docket-$a.md"
  assert "$a: wrapper exists" '[ -f "$w" ]'
  assert "$a: model mirrors wrapper" '[ -n "$(ex_field "$a" model)" ] && [ "$(ex_field "$a" model)" = "$(fm "$w" model)" ]'
  assert "$a: effort mirrors wrapper" '[ -n "$(ex_field "$a" effort)" ] && [ "$(ex_field "$a" effort)" = "$(fm "$w" effort)" ]'
done

# --- (5) RESOLVER ROUND-TRIP (retained from tests/test_config_example.sh) ----
# Uncomment the agents: block + the cursor block and enable cursor — the example IDs must resolve
# through the REAL resolver (sync-agents.sh) into a cursor wrapper. Proves the commented blocks
# are valid YAML, not decorative prose.
#
# The naive "strip a leading # from every line" approach corrupts the file: dozens of unrelated
# prose paragraphs elsewhere also start with "#" + indentation and would get uncommented into
# garbage right along with the agents: block. So this ISOLATES the exact commented region first
# (unique start/end anchors, verified against this file) and transforms ONLY that slice.
#
# Within the slice, codex:/cursor: are DOUBLY commented (disabled inside the disabled agents:
# block — an extra opt-in step past enabling agents: itself). Stage 1 strips one '# ' layer,
# which uncomments agents:/claude:/its nine children and demotes codex:/cursor: from doubly- to
# singly-commented (still inert). Stage 2 then strips cursor:'s own remaining layer — ONLY
# cursor's, leaving codex commented. The two stage-2 substitutions must run children-line-first:
# both are gated by the same `/^  # cursor:/,$` range address, and once the header substitution
# fires it consumes the '#' that the range address matches on — reordering it first would freeze
# the range before the children ever get touched (found by testing against the real file).
agents_block="$(sed -n '/^# agents:$/,/finalize-change:.*grok-4\.5-fast-high/p' "$EX")"
stage1="$(printf '%s\n' "$agents_block" | sed -E 's/^#[[:space:]]?//')"
stage2="$(printf '%s\n' "$stage1" | sed -E \
  -e '/^  # cursor:/,$ s/^  #   /    /' \
  -e '/^  # cursor:/,$ s/^  # ?(cursor:)/  \1/')"
# Derive the harness list from the REAL commented agent_harnesses: line (proving IT is valid too)
# rather than hand-writing an unrelated literal, then extend it to enable cursor.
harnesses_line="$(sed -n 's/^#[[:space:]]*\(agent_harnesses:.*\)/\1/p' "$EX" | head -n1)"
harnesses_line="$(printf '%s' "$harnesses_line" | sed -E 's/\[claude\]/[claude, cursor]/')"

SB="$(mktemp -d)"; _sbs="$SB"
mkdir -p "$SB/.claude/agents" "$SB/.cursor/agents" "$SB/.config/docket"
{
  printf '%s\n' "$harnesses_line"
  printf '%s\n' "$stage2"
} > "$SB/.config/docket/config.yml"
err="$(cd "$SB" && HOME="$SB" XDG_CONFIG_HOME="$SB/.config" DOCKET_HARNESS_ROOT="$SB" \
       bash "$REPO/sync-agents.sh" 2>&1 >/dev/null)"; rc=$?
assert "round-trip: sync-agents resolves the uncommented example (exit 0)" '[ "$rc" = "0" ]'
assert "round-trip: no unknown-harness-token warning" \
  '! printf "%s" "$err" | grep -qiE "unknown agent_harnesses token"'
assert "round-trip: a claude wrapper was generated" '[ -f "$SB/.claude/agents/docket-status.md" ]'
assert "round-trip: claude status model mirrors the built-in" \
  '[ "$(fm "$SB/.claude/agents/docket-status.md" model)" = "$(fm "$REPO/agents/docket-status.md" model)" ]'
assert "round-trip: a cursor wrapper was generated" '[ -f "$SB/.cursor/agents/docket-status.md" ]'
assert "round-trip: cursor status model came from the example block" \
  '[ "$(fm "$SB/.cursor/agents/docket-status.md" model)" = "grok-4.5-fast-medium" ]'
rm -rf "$_sbs"

# --- (6) SCAFFOLD SHAPE: install writes runtime + pointer, never policy values
# Why this guard exists: the old scaffold COPIED config.yml.example, so a user installed once and
# then carried a frozen snapshot of that day's defaults forever — every later default change was
# silently pinned by their stale copy. The scaffold must therefore write only the installer-owned,
# machine-local runtime block; every policy value remains commented/unset.
SC="$(mktemp -d)"; _scs="$SC"
out="$(HOME="$SC" DOCKET_HARNESS_ROOT="$SC" XDG_CONFIG_HOME="$SC/.config" \
       bash "$REPO/scripts/ensure-global-config.sh" 2>&1)"; scrc=$?
GC="$SC/.config/docket/config.yml"
assert "scaffold: exits 0"            '[ "$scrc" = "0" ]'
assert "scaffold: wrote the file"     '[ -f "$GC" ]'
# Validate the managed block before trusting a range extraction: exactly one ordered marker pair,
# then pin the only active YAML shape without enumerating the discovered path spelling.
runtime_open_line="$(grep -nF '# >>> docket (runtime.bash) >>>' "$GC" | cut -d: -f1)"
runtime_close_line="$(grep -nF '# <<< docket (runtime.bash) <<<' "$GC" | cut -d: -f1)"
assert "scaffold: has one ordered installer-managed runtime block" \
  '[ "$(grep -cF "# >>> docket (runtime.bash) >>>" "$GC")" = "1" ] &&
   [ "$(grep -cF "# <<< docket (runtime.bash) <<<" "$GC")" = "1" ] &&
   [ "$runtime_open_line" -lt "$runtime_close_line" ]'
active_scaffold="$(grep -vE '^[[:space:]]*(#.*)?$' "$GC" 2>/dev/null)"
assert "scaffold: only active keys are runtime.bash with an absolute path" \
  '[ "$(printf "%s\n" "$active_scaffold" | wc -l | tr -d " ")" = "2" ] &&
   printf "%s\n" "$active_scaffold" | sed -n "1p" | grep -Eq "^runtime:[[:space:]]*$" &&
   printf "%s\n" "$active_scaffold" | sed -n "2p" | grep -Eq "^[[:space:]]+bash:[[:space:]]+'\''/[^'\'']+'\''[[:space:]]*$"'
assert "scaffold: points at .docket.example.yml" 'grep -qF ".docket.example.yml" "$GC"'
assert "scaffold: names the layer precedence"    'grep -qiE "repo-local|precedence" "$GC"'
# Non-destructive: when the block must be inserted, unrelated user bytes remain an exact suffix,
# including a missing final newline.
user_config="$SC/user-config.expected"
printf '# user edited\nauto_capture: true' > "$user_config"
cp "$user_config" "$GC"
HOME="$SC" DOCKET_HARNESS_ROOT="$SC" XDG_CONFIG_HOME="$SC/.config" \
  bash "$REPO/scripts/ensure-global-config.sh" >/dev/null 2>&1
user_bytes="$(wc -c < "$user_config" | tr -d ' ')"
assert "scaffold: unrelated user config bytes survive runtime insertion" \
  'tail -c "$user_bytes" "$GC" | cmp -s - "$user_config"'
rm -rf "$_scs"

# The deleted surfaces stay deleted.
assert "config.yml.example is gone"          '[ ! -f "$REPO/config.yml.example" ]'
assert "tests/test_config_example.sh is gone" '[ ! -f "$REPO/tests/test_config_example.sh" ]'
assert "no stale config.yml.example reference in install.sh" \
  '! grep -qF "config.yml.example" "$REPO/install.sh"'
assert "no stale config.yml.example reference in ensure-global-config.sh" \
  '! grep -qF "config.yml.example" "$REPO/scripts/ensure-global-config.sh"'

# --- (7) README + dogfooding -------------------------------------------------
README="$REPO/README.md"
assert "README has the step-2 global-config heading" 'grep -qF "### 2. Set up your global config" "$README"'
assert "README step-2 names .docket.example.yml"     'grep -qF ".docket.example.yml" "$README"'
assert "README no longer names config.yml.example"   '! grep -qF "config.yml.example" "$README"'

# Dogfooding: this repo's own .docket.yml carries ONLY the values it actually sets, plus a
# pointer to the example. It is the copy-out workflow's worked demonstration, so it must not
# regress into a second all-keys surface — that drift is exactly what change 0101 ended.
DY="$REPO/.docket.yml"
assert "repo .docket.yml points at the example" 'grep -qF ".docket.example.yml" "$DY"'
assert "repo .docket.yml is slim (<= 40 lines)"  '[ "$(wc -l < "$DY")" -le 40 ]'
assert "repo .docket.yml keeps its set values" \
  'grep -Eq "^metadata_branch:[[:space:]]*docket" "$DY" && grep -Eq "^terminal_publish:[[:space:]]*true" "$DY"'

# --- (8) README SNIPPET CORRESPONDENCE ---------------------------------------
# The README carries a small illustrative .docket.yml snippet (change 0101 cut it down from a
# full all-keys sample). Nothing tested it against the canonical example, so its values could
# drift silently and its pointer could rot. This section closes that (change 0107).
#
# $README is already set by (7) above.

# Extract the section body ONCE. readme_snippet() (the fence filter, below) and the pointer
# check (at the bottom of this section) both consume this single function, so the heading
# literal lives in exactly one place in this file — a rename is a one-line fix, not a
# hunt-for-both-copies. Bounded by ANY heading level 1-3 (`^#{1,3} `), not just `### `: a
# following heading that gets promoted or demoted still stops the scan at the true next
# heading instead of reading past it into whatever comes after.
#
# The heading check is gated off while inside a fenced code block (toggled by any ``` line).
# Without that gate, `^#{1,3} ` also matches the yaml sample's own leading comment line
# ("# .docket.yml — committed..." — one `#` + space is both valid YAML-comment syntax and valid
# markdown-H1 syntax), which would truncate the section before the fence even closes.
snippet_section(){
  awk '
    /^### `\.docket\.yml` — per-repo settings$/ { inseg=1; next }
    inseg && /^```/ { fence = !fence }
    inseg && !fence && /^#{1,3} / { exit }
    inseg { print }
  ' "$README"
}

# Extract the FIRST fenced yaml block within the section. First-fence-only is a deliberate,
# narrow choice, not an oversight: this section's convention is exactly one worked example.
# The fence-count assert directly below is what makes that choice safe — without it, a second
# fence added later in the section would be silently invisible to readme_snippet() (and to
# every assert fed by it), which is exactly the half-guarded hole this pair closes.
readme_snippet(){
  snippet_section | awk '
    /^```yaml$/ && !s { f=1; s=1; next }
    f && /^```$/ { exit }
    f { print }
  '
}

sn_fence_count="$(snippet_section | grep -c '^```yaml$')"
assert "(8) section has exactly one yaml fence (readme_snippet reads only the first; a second would be silently unguarded; got $sn_fence_count)" \
  '[ "$sn_fence_count" = "1" ]'

# Flatten block-mapping YAML to "path<TAB>value" lines, dotting by INDENTATION rather than
# hardcoding the one nested path we happen to know about (finalize.gate). An indent stack, so
# depth is generic: it resolves the example's three-level runners.codex.sandbox correctly.
# Deliberately NOT a general YAML parser — it covers exactly the block-mapping subset these two
# files use (scalar and inline-list values, full-line and trailing comments). Do not grow it.
#
# ind is measured with the SAME character class ([^[:space:]]) as the key-shape test just below
# ([[:space:]]*): measuring indent in literal spaces only (as this used to) undercounts a
# tab-indented line — the tab isn't a space, so its indent reads as 0 and a tab-indented nested
# key gets flattened to top level instead of nested under its parent.
flatten_yaml(){
  awk '
    { line = $0
      if (line ~ /^[[:space:]]*$/) next
      if (line ~ /^[[:space:]]*#/) next
      sub(/[[:space:]]+#.*$/, "", line)
      sub(/[[:space:]]+$/, "", line)
      if (line !~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_-]*:/) next
      ind = match(line, /[^[:space:]]/) - 1
      key = line; sub(/^[[:space:]]*/, "", key); sub(/:.*$/, "", key)
      val = line; sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_-]*:[[:space:]]*/, "", val)
      while (depth > 0 && indents[depth] >= ind) depth--
      depth++; indents[depth] = ind; keys[depth] = key
      path = keys[1]
      for (i = 2; i <= depth; i++) path = path "." keys[i]
      printf "%s\t%s\n", path, val
    }'
}

sn_flat="$(readme_snippet | flatten_yaml)"
ex_flat="$(flatten_yaml < "$EX")"

# NON-VACUITY FLOOR / GROWTH CEILING. The forward loop below iterates the snippet's keys, so its
# real failure mode is iterating an EMPTY set: rename the heading, retitle the fence, or move the
# section, and extraction yields nothing while every assert sails through proving nothing. An
# EXACT count (not ">= 1") also covers the OPPOSITE direction: the snippet quietly growing back
# toward being the all-keys mirror change 0101 deleted. Both directions are real signal, and the
# remedy for a genuine, intentional addition is inline in the message below (not just in this
# comment), so it survives into CI output: bump the literal 5 AND add the key to
# .docket.example.yml, in the same commit.
sn_count="$(printf '%s\n' "$sn_flat" | grep -c .)"
assert "(8) snippet flattened key count is exactly 5 (floor against extraction going silently empty, ceiling against undocumented growth; if intentional, bump this literal 5 and add the key to .docket.example.yml; got $sn_count)" \
  '[ "$sn_count" = "5" ]'
ex_count="$(printf '%s\n' "$ex_flat" | grep -c .)"
assert "(8) example flattened non-empty (guard against a silently empty comparison side; got $ex_count)" \
  '[ "$ex_count" -ge 20 ]'

# SAFETY NET for the flattener's deliberately narrow key regex ([A-Za-z_][A-Za-z0-9_-]*:): a key
# spelled with any other character (e.g. `some-new-key: yes`) is silently REJECTED by
# flatten_yaml rather than flagged, and since sn_count above counts POST-filter output, a
# dropped line is invisible to both the count floor and the forward loop below — an undocumented
# snippet key would sail past this entire section undetected. Cross-check structurally instead:
# every non-blank, non-full-line-comment line inside the fence must survive flattening into
# exactly one output line; anything the flattener drops shows up as a mismatch here.
sn_raw_count="$(readme_snippet | grep -vE '^[[:space:]]*$' | grep -vcE '^[[:space:]]*#')"
assert "(8) snippet flattener drops no key-shaped line (raw content lines vs. flattened; got raw=$sn_raw_count flattened=$sn_count)" \
  '[ "$sn_raw_count" = "$sn_count" ]'

# DIRECTION: this loop iterates the SNIPPET's keys and proves snippet ⊆ example, values equal.
# It deliberately does NOT iterate the example's keys, and the missing reverse loop is NOT an
# oversight — do not "fix" it.
#
# The correspondence-guard-runs-one-way learning (harvested from change 0101) says: name the
# direction you iterate, then write the other one. That rule assumes the two sets stand in a
# CORRESPONDENCE. These two do not. The README snippet is a deliberate PROPER SUBSET — a small
# illustrative taste — while .docket.example.yml is the canonical all-keys reference. So the
# reverse loop here would assert "every key in the example appears in the README", which is a
# completeness check that re-creates the fourth all-keys surface change 0101 existed to delete.
# Writing it would undo the change that motivated this guard.
#
# The orphan direction that actually bit 0101 — a documented key no real surface carries — is
# still covered here: a snippet key absent from the example fails the existence assert below.
# The asymmetry is safe BECAUSE of the subset relation, which was not true of 0101's
# export-keys-vs-example guards.
#
# CAVEAT: value equality below is sound only because THIS ONE FENCE shows shipped defaults. The
# README's other config fences deliberately show NON-default values to illustrate opting in —
# e.g. `auto_capture: true` (~README:264), `terminal_publish: true` (~README:407),
# `metadata_branch: main` (~README:433) — so do not generalize this value-equality guard to
# another fence; against one of those it would go spuriously RED for correctly demonstrating a
# non-default setting.
#
# Fed by a HEREDOC, never a pipe: a pipe runs the loop in a subshell and both accumulator
# variables come back empty, so every mismatch would silently pass.
sn_missing=""
sn_mismatched=""
while IFS="$(printf '\t')" read -r sn_path sn_val; do
  [ -n "$sn_path" ] || continue
  ex_hit="$(printf '%s\n' "$ex_flat" | awk -F'\t' -v p="$sn_path" '$1==p{print "1"; exit}')"
  if [ -z "$ex_hit" ]; then
    sn_missing="$sn_missing $sn_path"
    continue
  fi
  ex_val="$(printf '%s\n' "$ex_flat" | awk -F'\t' -v p="$sn_path" '$1==p{print $2; exit}')"
  if [ "$ex_val" != "$sn_val" ]; then
    sn_mismatched="$sn_mismatched $sn_path(README='$sn_val'!=example='$ex_val')"
  fi
done <<SNIPPET_KEYS
$sn_flat
SNIPPET_KEYS

assert "(8) every README snippet key exists in the example (${sn_missing:-none missing})" \
  '[ -z "$sn_missing" ]'
assert "(8) every README snippet value equals the example's (${sn_mismatched:-none mismatched})" \
  '[ -z "$sn_mismatched" ]'

# POINTER: the section's link to the canonical reference must resolve to a real file. Scoped to
# this section's body via snippet_section() (defined above), NOT a whole-file grep — the README
# names .docket.example.yml in several other places (the tooling list, the layered-config prose),
# so an unscoped match would stay green even after THIS section's own link rotted.
#
# Matches on the link TARGET, not the link text: the target is what must resolve, so a correct,
# non-rotted link whose anchor text is reworded (e.g. `[the canonical reference]
# (.docket.example.yml)`) must stay green rather than reddening on wording alone.
sn_ptr="$(snippet_section | sed -nE 's/.*\[[^]]*\]\(([^)]*\.docket\.example\.yml)\).*/\1/p' | head -n1)"
assert "(8) the section links to the canonical reference" '[ -n "$sn_ptr" ]'
assert "(8) canonical-reference link target exists (${sn_ptr:-<no link>})" \
  '[ -n "$sn_ptr" ] && [ -f "$REPO/$sn_ptr" ]'

# --- (9) README CONFIG FENCE KEY CORRESPONDENCE ------------------------------
TAB9="$(printf '\t')"

# PREREQUISITE GUARD (change 0108, Task 1). flatten_yaml's key class must admit HYPHENS, because
# README fences 289/310 carry `implement-next:` under agents.default. The class appears TWICE in
# flatten_yaml — the shape test and the value strip — and widening only the shape test is a
# half-fix that NOTHING ELSE IN THIS FILE CATCHES: the path count is identical either way
# (3 paths on the fixture below, 11 on fences 289/310), so every count-based floor passes. The
# half-fix is visible ONLY in the extracted VALUE, which comes back as the whole raw line. That
# is why this asserts the value and not just the path.
hyph_fix="$(printf 'agents:\n  default:\n    implement-next: { model: x, effort: y }\n')"
hyph_out="$(printf '%s\n' "$hyph_fix" | flatten_yaml)"
hyph_paths="$(printf '%s\n' "$hyph_out" | grep -c .)"
hyph_val="$(printf '%s\n' "$hyph_out" | awk -F"$TAB9" '$1=="agents.default.implement-next"{print $2}')"
assert "(9) flatten_yaml keeps a HYPHENATED key as its own path (got $hyph_paths paths, want 3)" \
  '[ "$hyph_paths" = "3" ]'
assert "(9) flatten_yaml STRIPS a hyphenated key from its value — half-fix guard; widening only the shape test leaves the whole raw line here and no count-based floor can see it (got [$hyph_val])" \
  '[ "$hyph_val" = "{ model: x, effort: y }" ]'

# FENCE DISCOVERY — DERIVED, NEVER ENUMERATED. The stub that proposed this change listed the
# unguarded fences by line number and its list was ALREADY WRONG on arrival (it omitted the
# reclaim: fence). A hand-written fence list is an enumerated floor that ages directly into the
# gap it was written to close, so the set is scanned out of the README instead: every yaml fence
# is in scope BY DEFAULT, and a new config fence is guarded the day it is written.
#
# The opener regex is WHITESPACE-TOLERANT and the closer is matched at the SAME indent, because
# fence 576 (skills: / brainstorm:) is a list-item continuation indented two spaces. A
# column-0-anchored regex structurally cannot see it — that is not hypothetical, it is the bug
# this change's own design draft shipped, which is why mutation-testing it is the fence-count
# assert below (fence 576 is one of the 9 it counts; a regression back to column-0 anchoring
# drops the count to 8).
#
# LATENT HOLE: a ```yaml fence nested inside a wider (four-backtick) fence would be discovered as
# a config fence in its own right — opener/closer matching is purely syntactic and does not track
# an enclosing fence type. Not exercised today (the README has no such nesting), and the right
# failure order is preserved if one is added: the fence-count floor below trips FIRST (an extra
# discovered fence with no keys in the example), forcing a conscious update rather than a silent
# miscount.
fence_openers(){
  awk '
    /^[[:space:]]*```yaml[[:space:]]*$/ && !inf { inf=1; ind=match($0,/[^[:space:]]/)-1; start=NR; next }
    inf && /^[[:space:]]*```[[:space:]]*$/ && match($0,/[^[:space:]]/)-1==ind { printf "%d\t%d\n", start, ind; inf=0 }
  ' "$1"
}

# Body of the fence opening at line $2 with indent $3, with that base indent stripped so nested
# keys keep their RELATIVE indent (flatten_yaml dots by indentation). substr() rather than an
# awk interval expression: {0,n} is not portable across awk implementations.
fence_body(){
  awk -v s="$2" -v ind="$3" '
    NR <= s { next }
    $0 ~ /^[[:space:]]*```[[:space:]]*$/ && match($0,/[^[:space:]]/)-1==ind { exit }
    { print substr($0, ind+1) }
  ' "$1"
}

# MARKER GRAMMAR. Two markers attach to a fence:
#   <!-- docket:config-fence: ignore -->   not .docket.yml schema — skip this fence entirely
#   <!-- docket:config-fence: values -->   also assert value equality against the example
#
# ATTACHMENT is the NEAREST PRECEDING NON-BLANK line, not strictly the line above. Fence 576
# forces this: it is a list-item continuation preceded by a blank line, and a column-0 HTML
# comment there would terminate the enclosing list. So the marker may carry leading whitespace,
# and must sit at AT LEAST its fence's own indent.
#
# AN UNKNOWN OR MALFORMED TOKEN IS A HARD FAIL, never warned-and-ignored, because the two mistake
# directions are ASYMMETRIC: a typo'd `ignore` fails safe (the fence is still checked and
# reddens, loudly), but a typo'd `values`, a typo'd marker name, or a bare
# `<!-- docket:config-fence -->` fails OPEN AND SILENT — value coverage evaporates with no signal,
# which is precisely the drift class this change exists to end. Any line matching
# docket:config-fence that does not match the exact grammar reddens.
#
# TWO ADJACENT MARKERS REDDEN as a duplicate — duplicate detection compares the two nearest
# non-blank lines above the fence. A marker separated from the fence by other content (a prose
# line, say) is NOT a duplicate case at all: it is simply not attached (see NONE below), and that
# silent-orphan case is caught separately by the whole-file marker reconciliation assert.
fence_marker(){
  awk -v s="$2" -v find="$3" '
    NR >= s { exit }
    $0 !~ /^[[:space:]]*$/ { prev2 = prev1; prev1 = $0 }
    END {
      if (prev1 !~ /docket:config-fence/) { print "NONE"; exit }
      if (prev2 ~ /docket:config-fence/)  { print "BAD duplicate-marker"; exit }
      mind = match(prev1, /[^[:space:]]/) - 1
      if (mind < find) { print "BAD marker-indent-below-fence"; exit }
      if (prev1 ~ /^[[:space:]]*<!--[[:space:]]+docket:config-fence:[[:space:]]+(ignore|values)[[:space:]]+-->[[:space:]]*$/) {
        t = prev1
        sub(/^.*docket:config-fence:[[:space:]]*/, "", t)
        sub(/[[:space:]]*-->.*$/, "", t)
        print "TOKEN " t
      } else { print "BAD malformed-marker" }
    }
  ' "$1"
}

# NON-VACUITY FLOOR 1 — the population itself. (9) iterates a DISCOVERED set, so its real failure
# mode is discovering ZERO fences and sailing through green. An EXACT count also catches the
# opposite direction (an undocumented fence added without keys in the example). The remedy is
# inline in the message so it survives into CI output.
fence_count="$(fence_openers "$README" | grep -c .)"
assert "(9) README yaml fence count is exactly 9 — floor against discovery going silently empty, ceiling against an unguarded new fence; if you ADDED a config fence, bump this literal AND ensure its keys are in .docket.example.yml in the same commit; if this dropped to 8, check that the fence regex is still whitespace-tolerant (fence 576 is indented) before touching the literal (got $fence_count)" \
  '[ "$fence_count" = "9" ]'

# ANCHOR: .docket.example.yml, ONE HOP. Sections (2a)/(2b)/(2c) already bind the example to the
# resolver in BOTH directions and prove it a faithful superset of everything the code reads, so
# going through the example inherits resolver coverage transitively and keeps a single anchor
# per artifact instead of two competing ones.
#
# ASSERT: EXISTENCE-ONLY by default. This is what makes one check applicable to all nine fences.
# Most of them deliberately show NON-DEFAULT values to illustrate opting in (auto_capture: true,
# terminal_publish: true, metadata_branch: main, and the two layered-config samples), so a
# value-equality assert would go spuriously RED against correct prose. Value equality is opt-in
# per fence — see the `values` marker on the reclaim: fence at README:233.
#
# DIRECTION (correspondence-guard-runs-one-way): this iterates the README's fence keys and proves
# fence ⊆ example. The reverse loop is deliberately ABSENT and is NOT an oversight — "every
# example key appears in the README" is the fourth all-keys surface change 0101 deleted. Do not
# add it.
#
# KEY RESOLUTION — QUERY-BY-KEY, not build-a-set. Two README fences use agents: and
# agent_harnesses: actively, but section (3) requires those keys to ship COMMENTED in the
# example, so a naive `path IN flatten_yaml(example)` reddens against correct prose. Resolution:
#   - top-level segment ACTIVE in the example  => the FULL dotted path must match;
#   - top-level segment only a COMMENTED pseudo-key => acceptance stops at the top-level segment,
#     because a commented key has no nested body to match against.
# Building a pseudo-key SET by regex is rejected: `^#[[:space:]]*[A-Za-z_]+:` also matches the
# example's prose comments (`# exceptions:`, `# scope: any layer` in many places, `# line:`),
# which would silently accept anything.
#
# is_pseudo_key matches the key LITERALLY via index()==1 rather than interpolating it into an
# ERE. That is strictly stronger than escaping it (escape-ere-metacharacters-in-key): there is no
# regex for a metacharacter to leak into at all.
#
# RESIDUAL HOLE, documented rather than closed: a future fence key whose NAME collides with a
# prose-comment word would be silently accepted (`scope:` would match `# scope: any layer`, and
# anchoring is no defense since that comment starts at column 0). No collision exists among
# today's 12 top-level fence keys. It is not closed because the only tight closure is an explicit
# two-key allowlist, which is exactly the enumerated floor this change exists to avoid.
is_pseudo_key(){
  awk -v k="$1" '
    { line=$0
      if (line !~ /^#/) next
      sub(/^#[[:space:]]*/, "", line)
      if (index(line, k ":") == 1) { found=1; exit } }
    END { exit(found?0:1) }
  ' "$EX"
}

ex9_paths="$(printf '%s\n' "$ex_flat" | cut -f1)"

# scan_fences <markdown-path> — emits one finding per line; EMPTY OUTPUT MEANS CLEAN.
# Takes the path as an argument (not the $README global) so the marker tests in Task 5 can scan a
# temporary fixture instead of mutating the real README.
scan_fences(){
  local md="$1" line ind body flatout flat raw p pv top marker token seen_token exval
  while IFS="$TAB9" read -r line ind; do
    [ -n "$line" ] || continue
    marker="$(fence_marker "$md" "$line" "$ind")"
    # "seen" is emitted for EVERY fence this loop reaches, BEFORE any marker-driven `continue` —
    # including the malformed-marker and ignore-marked cases below. It is the true population
    # floor (see NON-VACUITY FLOOR 4): every other finding kind is produced deeper in this
    # function, so a bug that short-circuits the loop body (or a marker change that routes every
    # fence through a `continue`) would leave those floors green for a reason unrelated to the
    # property they claim. `seen_token` is "none" for an unmarked fence and "bad" for a malformed
    # one, so grepping for the exact token `values` (rather than merely non-empty) can never match
    # an unmarked or malformed fence.
    case "$marker" in
      NONE)      token=""; seen_token="none" ;;
      "TOKEN "*) token="${marker#TOKEN }"; seen_token="$token" ;;
      "BAD "*)   echo "seen $line bad"; echo "marker $line ${marker#BAD }"; continue ;;
      *)         echo "seen $line bad"; echo "marker $line unparseable"; continue ;;
    esac
    echo "seen $line $seen_token"
    [ "$token" = "ignore" ] && continue
    body="$(fence_body "$md" "$line" "$ind")"
    flatout="$(printf '%s\n' "$body" | flatten_yaml)"
    flat="$(printf '%s\n' "$flatout" | grep -c .)"
    if [ "$flat" -eq 0 ]; then echo "empty $line"; continue; fi
    raw="$(printf '%s\n' "$body" | grep -vE '^[[:space:]]*$' | grep -vcE '^[[:space:]]*#')"
    [ "$raw" = "$flat" ] || echo "drop $line raw=$raw flat=$flat"
    while IFS="$TAB9" read -r p pv; do
      [ -n "$p" ] || continue
      top="${p%%.*}"
      if grep -Fxq "$top" <<<"$ex9_paths"; then
        if ! grep -Fxq "$p" <<<"$ex9_paths"; then
          echo "miss $line $p"
        elif [ "$token" = "values" ]; then
          # NOT EXERCISED TODAY: this value check is reached only inside the "top is ACTIVE" arm
          # above. A future values-marked fence whose top-level key is merely a COMMENTED
          # pseudo-key falls into the `elif is_pseudo_key` arm below instead, which has no value
          # check at all — silently inert, since every values-marked fence so far has an active
          # top-level key. Not a bug to close now; a future author should not assume marking a
          # pseudo-keyed fence `values` opts it into anything.
          exval="$(awk -F"$TAB9" -v k="$p" '$1==k{print $2; exit}' <<<"$ex_flat")"
          [ "$pv" = "$exval" ] || echo "value $line $p readme=$pv example=$exval"
        fi
      elif is_pseudo_key "$top"; then :
      else echo "miss $line $p"; fi
    done <<FLAT
$flatout
FLAT
  done <<OPENERS
$(fence_openers "$md")
OPENERS
}

findings9="$(scan_fences "$README")"
f9_miss="$(printf '%s\n' "$findings9" | grep '^miss ' | sed 's/^miss //' | tr '\n' ' ' | sed 's/ *$//')"
assert "(9) every README config-fence key exists in .docket.example.yml (fence-line + key path shown; ${f9_miss:-none missing})" \
  '[ -z "$f9_miss" ]'

# NON-VACUITY FLOOR 2 — a fence that flattens to ZERO paths contributes nothing to the existence
# loop above, so it would be silently unguarded rather than reported.
f9_empty="$(printf '%s\n' "$findings9" | grep '^empty ' | sed 's/^empty //' | tr '\n' ' ' | sed 's/ *$//')"
assert "(9) every config fence flattens to at least one key (fence lines listed; ${f9_empty:-none empty})" \
  '[ -z "$f9_empty" ]'

# NON-VACUITY FLOOR 3 — SAFETY NET for flatten_yaml's deliberately narrow key class. A key spelled
# outside [A-Za-z_][A-Za-z0-9_-]* is silently REJECTED by the flattener rather than flagged, and
# because the existence loop iterates POST-filter output, a dropped line is invisible to it. Cross-
# check structurally: every non-blank, non-full-line-comment line in a fence must survive
# flattening into exactly one path. This is also why README config fences must use inline-list
# syntax (`board_surfaces: [inline]`), never a YAML block sequence: a fence with `board_surfaces:`
# followed by `  - inline` on its own line yields `drop <line> raw=2 flat=1` here — that is a
# syntax-convention violation in the fence, not a flattener defect, so don't go hunting for a key
# spelling problem when this floor names it.
f9_drop="$(printf '%s\n' "$findings9" | grep '^drop ' | sed 's/^drop //' | tr '\n' ' ' | sed 's/ *$//')"
assert "(9) the flattener drops no key-shaped line in any fence (raw content lines vs flattened, per fence; ${f9_drop:-none dropped})" \
  '[ -z "$f9_drop" ]'

f9_marker="$(printf '%s\n' "$findings9" | grep '^marker ' | sed 's/^marker //' | tr '\n' ' ' | sed 's/ *$//')"
assert "(9) every docket:config-fence marker parses (fence-line + reason; ${f9_marker:-none malformed})" \
  '[ -z "$f9_marker" ]'

# NON-VACUITY FLOOR 4 — SCAN_FENCES'S OWN POPULATION, and specifically the `values` marker's
# continued existence. The reviewer proved that deleting the `values` marker line from README, or
# moving it one non-blank line earlier (still well-formed, but no longer the nearest non-blank
# line above the reclaim: fence), leaves fence_marker returning NONE for that fence and f9_value
# (below) silently green — green for a reason OTHER than "no drift", which is the exact
# fail-open-and-silent mode this whole section exists to end. `seen` (added above, emitted for
# every fence BEFORE any skip) gives two floors against it: an exact count of fences reached, and
# a floor that at least one of them is values-marked.
f9_seen="$(printf '%s\n' "$findings9" | grep -c '^seen ')"
assert "(9) scan_fences visited all 9 fences — same literal as NON-VACUITY FLOOR 1's fence-count assert above; if you added a fence, see that assert's message for the remedy (got $f9_seen)" '[ "$f9_seen" = "9" ]'
f9_vmarked="$(printf '%s\n' "$findings9" | grep -c '^seen .* values$')"
assert "(9) at least one fence is values-marked — floor against the marker being deleted entirely; it does NOT prove the marker sits on the RIGHT fence (see the positive control immediately below for that) (got $f9_vmarked)" \
  '[ "$f9_vmarked" -ge 1 ]'

# NON-VACUITY FLOOR 4b — POSITIVE CONTROL FOR *WHICH* FENCE, not merely whether some fence has a
# values marker. f9_vmarked above is a count and cannot distinguish "marker present, right fence"
# from "marker present, wrong fence": relocating the `values` marker from the reclaim: fence to
# fence 209 (section (8)'s per-repo snippet, which ALSO documents shipped defaults) leaves
# f9_vmarked >= 1 (fence 209 absorbs it) and f9_value below fully green too (fence 209's own values
# still equal the example), while reclaim.lease_ttl drifting 72 -> 99 goes completely undetected —
# the whole suite stays green for a reason unrelated to the property it claims, the exact defect
# class this section exists to end. The only assert that reddens under that relocation is this one:
# drift reclaim.lease_ttl in a throwaway fixture copy (never the real README — see this section's
# constraint) and demand scan_fences report a "value" finding NAMING reclaim.lease_ttl specifically,
# regardless of which fence currently carries the marker.
fx9e="$tmp/README-value-control.md"
sed 's/^  lease_ttl: 72/  lease_ttl: 99/' "$README" > "$fx9e"
fx9e_findings="$(scan_fences "$fx9e")"
fx9e_value="$(printf '%s\n' "$fx9e_findings" | grep '^value .*reclaim\.lease_ttl ')"
assert "(9) the reclaim: fence's value coverage is live — pins that lease_ttl drift is caught even if the values marker moved elsewhere (got [${fx9e_value}])" '[ -n "$fx9e_value" ]'

# NON-VACUITY FLOOR 5 — ORPHAN MARKER DETECTION. fence_marker only ever looks at the two nearest
# non-blank lines above ONE GIVEN fence, so a well-formed docket:config-fence line that sits
# outside the attachment position for every fence (separated from its intended fence by a prose
# line, for instance) returns NONE there and is never examined by the grammar at all — a marker
# reduced to decoration with no signal. Reconcile the whole file against what the scan actually
# consumed: every docket:config-fence line in the README must be the attached marker of some
# fence (`seen`'s token is "none" only when no marker attached; anything else, including a
# malformed one, means a marker WAS attached and examined, just possibly badly — that half is
# already covered by the marker-parse floor above).
#
# TWO ADJACENT MARKERS are the one case "anything else … means a marker WAS attached" glosses
# over: a duplicate pair collapses into a SINGLE `seen … bad` record (the nearer marker, flagged via
# the duplicate-marker branch), so f9_marked only advances by one while md_markers counts BOTH
# literal lines. This assert therefore ALSO fires alongside the duplicate-marker finding above,
# with a mismatch that reads exactly like an orphan ("file has N, fences consumed N-1") even though
# the real cause is the duplicate, not an orphan. Harmless in practice — the marker-parse floor's
# own message already names "duplicate-marker" right next to this one, so a maintainer reading both
# together is not misled for long — but don't trust THIS message's wording to tell you which of the
# two it is.
#
# md_markers COUNTS THE LITERAL STRING, not marker semantics, so it also fires if the README ever
# gains PROSE that mentions "docket:config-fence" — documenting the marker convention itself in a
# sentence, say, rather than as an attached comment. That occurrence has no fence to attach to, so
# md_markers grows while f9_marked does not, and this floor reddens with no hint that the cause is
# documentation rather than a real orphaned marker. This is intentional — a shape-based count, not a
# cleverer one, is the right call here — and is not a bug to fix by teaching the grep to tell prose
# apart from a real marker. If you hit this after adding such prose: the redness is real signal that
# the literal-count invariant no longer holds, so either phrase the prose without the exact
# substring `docket:config-fence`, or document the marker grammar in this test file instead (see the
# MARKER GRAMMAR comment above, where it already lives) rather than in the README.
md_markers="$(grep -c 'docket:config-fence' "$README")"
f9_marked="$(printf '%s\n' "$findings9" | grep '^seen ' | grep -vc ' none$')"
assert "(9) every docket:config-fence line in the README is attached to a fence (file has $md_markers, fences consumed $f9_marked)" \
  '[ "$md_markers" = "$f9_marked" ]'

# VALUE EQUALITY IS OPT-IN, and it is not lost where it is SOUND. Section (8) keeps it on fence
# 209 (the per-repo snippet, which documents shipped defaults); this marker adds it to the
# reclaim: fence, whose lease_ttl: 72 / auto: false are also shipped defaults and SHOULD redden if
# the defaults move. The other seven fences stay existence-only, because they deliberately
# illustrate non-default values. Fence 209 is therefore double-covered by (8) (existence + values)
# and (9) (existence only); that overlap is accepted rather than special-cased — (8)'s fence is
# simply left unmarked, and since no unmarked fence gets a value assert, no special-casing exists.
f9_value="$(printf '%s\n' "$findings9" | grep '^value ' | sed 's/^value //' | tr '\n' ' ' | sed 's/ *$//')"
assert "(9) values-marked fences match the example exactly (${f9_value:-none mismatched})" \
  '[ -z "$f9_value" ]'

# The ignore path has ZERO exercise in the README — all nine fences today are config fences — so
# without a fixture it would ship with its only branch untested. Assert it POSITIVELY on a
# temporary fixture rather than by adding a real ignored fence to the README. This is why the
# helpers above take a markdown path as an ARGUMENT instead of reading $README.
fx9="$tmp/fence-fixture.md"
printf '# Fixture\n\n<!-- docket:config-fence: ignore -->\n```yaml\nnot_a_docket_key: true\n```\n' > "$fx9"
fx9_count="$(fence_openers "$fx9" | grep -c .)"
assert "(9) fixture scaffold is valid — one discoverable fence (got $fx9_count)" '[ "$fx9_count" = "1" ]'
fx9_marker="$(fence_marker "$fx9" 4 0)"
assert "(9) an ignore marker parses to its token (got $fx9_marker)" '[ "$fx9_marker" = "TOKEN ignore" ]'
fx9_findings="$(scan_fences "$fx9")"
# scan_fences now also emits a "seen" record for this fence (NON-VACUITY FLOOR 4 above) — filter
# it out before checking for CLEAN, or this assert would redden on the ignore path working
# correctly. Checked separately, and POSITIVELY, right below: the ignore fixture's own seen
# record must carry the ignore token, so the ignore branch is proven exercised rather than the
# fixture being invisible to the scanner in a different way than before.
fx9_non_seen="$(printf '%s\n' "$fx9_findings" | grep -v '^seen ')"
assert "(9) an ignore-marked fence is skipped entirely — its non-schema key raises nothing (got [${fx9_non_seen}])" \
  '[ -z "$fx9_non_seen" ]'
fx9_seen_ignore="$(printf '%s\n' "$fx9_findings" | grep '^seen .* ignore$')"
assert "(9) the ignore fixture's fence is recorded as ignore-marked in its seen record (got [${fx9_seen_ignore}])" \
  '[ -n "$fx9_seen_ignore" ]'

# ...and the SAME fixture without the marker must report the key, so the assert above is proven to
# be the marker working rather than the fixture being invisible to the scanner.
fx9b="$tmp/fence-fixture-unmarked.md"
printf '# Fixture\n\n```yaml\nnot_a_docket_key: true\n```\n' > "$fx9b"
fx9b_findings="$(scan_fences "$fx9b")"
fx9b_non_seen="$(printf '%s\n' "$fx9b_findings" | grep -v '^seen ')"
assert "(9) the same fence WITHOUT the ignore marker does report its key — proves the skip is the marker, not an invisible fixture (got [${fx9b_non_seen}])" \
  '[ "$fx9b_non_seen" = "miss 3 not_a_docket_key" ]'

# NON-VACUITY FLOOR 2's "empty" branch and the marker grammar's BAD path are both demonstrably
# reachable but shipped with no fixture, unlike the ignore branch above. A comment-only fence body
# exercises the former; a malformed (but attached) token exercises the latter.
fx9c="$tmp/fence-fixture-empty.md"
printf '# Fixture\n\n```yaml\n# just a comment, no keys here\n```\n' > "$fx9c"
fx9c_findings="$(scan_fences "$fx9c")"
fx9c_empty="$(printf '%s\n' "$fx9c_findings" | grep '^empty ')"
assert "(9) a comment-only fence body trips NON-VACUITY FLOOR 2 with the exact empty finding (got [${fx9c_empty}])" \
  '[ "$fx9c_empty" = "empty 3" ]'

fx9d="$tmp/fence-fixture-badmarker.md"
printf '# Fixture\n\n<!-- docket:config-fence: bogus -->\n```yaml\nsome_key: true\n```\n' > "$fx9d"
fx9d_findings="$(scan_fences "$fx9d")"
fx9d_marker="$(printf '%s\n' "$fx9d_findings" | grep '^marker ')"
assert "(9) an unknown docket:config-fence token hard-fails via the marker branch with the exact malformed-marker finding (got [${fx9d_marker}])" \
  '[ "$fx9d_marker" = "marker 4 malformed-marker" ]'

exit $fail
