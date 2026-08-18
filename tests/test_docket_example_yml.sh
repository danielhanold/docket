#!/usr/bin/env bash
# tests/test_docket_example_yml.sh — run: bash tests/test_docket_example_yml.sh
# Guards .docket.example.yml, docket's canonical all-comprehensive config reference (change 0101).
# The example is PURE DOCUMENTATION — no docket tooling reads it — so these tests are the only
# thing keeping it honest. Replaces tests/test_config_example.sh.
set -uo pipefail
# --- BASH VERSION GATE (change 0246) -------------------------------------------------------------
# THIS MUST STAY AT THE TOP OF THE FILE, above every function and heredoc.
#
# WHY: bash 3.2's $(...) parser cannot see heredocs — it scans the heredoc BODY as shell. The
# scope_guard_awk assignment below (`scope_guard_awk="$(cat <<'SCOPE_GUARD_AWK'`) has a backtick
# inside a comment in its body, so under 3.2 the whole file from that point to EOF fails to parse.
# Observed directly: `PATH=/usr/bin:/bin bash tests/test_docket_example_yml.sh` ran 103 of this
# file's asserts, printed zero failures, then died with "unexpected EOF while looking for matching
# `" and exit 2. The 290 asserts that never ran include the ENTIRE mirror and round-trip family.
#
# scripts/run-tests.sh never hits this — it re-execs itself under a Bash 4.3+ runtime and runs every
# test file with $TEST_BASH. The exposed path is DIRECT invocation, which is this file's own
# documented run line (see the header above), on any machine whose PATH resolves bash to 3.2 first
# (stock macOS). Bash parses incrementally, so this gate executes before the line-684 construct is
# ever parsed — which is exactly why it must not be moved down or wrapped in a function.
#
# The floor here is 4, not run-tests.sh's 4.3: that file needs `wait -n`, this one does not.
if [ "${BASH_VERSINFO[0]:-0}" -lt 4 ]; then
  printf '%s\n' "test_docket_example_yml.sh requires bash >= 4 (running ${BASH_VERSION:-unknown} from ${BASH:-unknown}). Bash 3.2 cannot parse this file's heredoc-in-\$() constructs and silently skips most of its asserts. Re-run with a bash 4+ binary, or use scripts/run-tests.sh." >&2
  exit 2
fi
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
EX="$REPO/.docket.example.yml"
CFGSCRIPT="$REPO/scripts/docket-config.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

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
#
# CAPTURED FIRST, never `git show … | grep -q` (AGENTS.md § Shell). This assert was that pipeline
# and it was an intermittent RED under parallel suite load — the failure this repair exists for.
# The example is ~40KB, which is larger than a macOS pipe's 16KB default capacity; unloaded the
# kernel grows the pipe to 64KB, the whole blob fits, and `git show` exits 0 before `grep -q` ever
# closes the read end. Under the parallel runner the kernel declines to grow it, `git show` blocks
# with the blob half-written, `grep -q` matches on the `metadata_branch:` line and exits, and
# `git show` takes SIGPIPE — which `set -o pipefail` (line 6) promotes to the pipeline's 141.
# Measured on the change-0276 hardware: 70/70 runs returned 141 under pipe-buffer pressure and
# 0/70 unloaded, so file GROWTH was never the trigger — the pre-0276 example failed identically.
# The emptiness leg keeps the guard's teeth: an unpushed fixture makes `git show` error to empty,
# which reddens here rather than sailing into a vacuous byte-identity comparison.
pushed_example="$(git -C "$tmp/full" show origin/main:.docket.yml 2>/dev/null)"
assert "fidelity fixture: example reached the fixture's origin/main" \
  '[ -n "$pushed_example" ] && grep -q "^metadata_branch:" <<<"$pushed_example"'

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
    FINALIZE_SKIP_RESULTS_ONLY_DELTA) echo '^[[:space:]]+skip_results_only_delta:[[:space:]]*false[[:space:]]*$' ;;
    LEARNINGS_ENABLED)     echo '^[[:space:]]+enabled:[[:space:]]*true' ;;
    LEARNINGS_CAP)         echo '^[[:space:]]+cap:[[:space:]]*300' ;;
    BOARD_SURFACES)        echo '^board_surfaces:[[:space:]]*\[[[:space:]]*inline[[:space:]]*\]' ;;
    AUTO_GROOM)            echo '^auto_groom:[[:space:]]*false' ;;
    CHANGE_TYPES)          echo '^change_types:[[:space:]]*\[[[:space:]]*chore,[[:space:]]*docs,[[:space:]]*feat,[[:space:]]*fix,[[:space:]]*refactor,[[:space:]]*perf[[:space:]]*\]' ;;
    AUTO_CAPTURE_ENABLED)  echo '^[[:space:]]+enabled:[[:space:]]*false' ;;
    AUTO_CAPTURE_TYPES)    echo '^[[:space:]]+types:[[:space:]]*all' ;;
    TERMINAL_PUBLISH)      echo '^terminal_publish:[[:space:]]*false' ;;
    GATE_OBSERVATION_BUDGET) echo '^gate_observation_budget:[[:space:]]*30[[:space:]]*$' ;;
    DELEGATION_OBSERVATION_BUDGET) echo '^delegation_observation_budget:[[:space:]]*60[[:space:]]*$' ;;
    RECLAIM_LEASE_TTL)     echo '^[[:space:]]+lease_ttl:[[:space:]]*72' ;;
    RECLAIM_AUTO)          echo '^[[:space:]]+auto:[[:space:]]*false' ;;
    BUILD_CHECKPOINT)      echo '^[[:space:]]+checkpoint:[[:space:]]*false' ;;
    REVIEW_MIN_FIX_SEVERITY) echo '^[[:space:]]+min_fix_severity:[[:space:]]*minor[[:space:]]*$' ;;
    REVIEW_MAX_FIX_TASKS)  echo '^[[:space:]]+max_fix_tasks:[[:space:]]*10[[:space:]]*$' ;;
    SKILL_BRAINSTORM)      echo '^[[:space:]]+brainstorm:[[:space:]]*superpowers:brainstorming' ;;
    SKILL_PLAN)            echo '^[[:space:]]+plan:[[:space:]]*superpowers:writing-plans' ;;
    SKILL_BUILD)           echo '^[[:space:]]+build:[[:space:]]*docket-build[[:space:]]*$' ;;
    SKILL_REVIEW)          echo '^[[:space:]]+review:[[:space:]]*docket-review[[:space:]]*$' ;;
    SKILL_FINISH)          echo '^[[:space:]]+finish:[[:space:]]*superpowers:finishing-a-development-branch' ;;
    DUMMY_MODE_ENABLED)    echo '^[[:space:]]+enabled:[[:space:]]*false' ;;
    DUMMY_MODE_PERSONA)    echo '^[[:space:]]+persona:[[:space:]]*""[[:space:]]*$' ;;
    DUMMY_MODE_SURFACES)   echo '^[[:space:]]+surfaces:[[:space:]]*all[[:space:]]*$' ;;
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
#   CHANGE_TYPES — change 0127. Same shape as BOARD_SURFACES: the value is assembled through
#   intermediates (ct_raw / ct_body) and the built-in fallback reads a library array, so no
#   `CHANGE_TYPES=` assignment line ever carries the literal leaf key "change_types".
#   DUMMY_MODE_SURFACES — change 0276. Same shape as BOARD_SURFACES and CHANGE_TYPES: the leaf is
#   read into an intermediate (`dm_surfaces_raw="$(dm_key surfaces all)"`) and the export is then
#   assigned either the literal `all` or the accumulated `${dm_kept[*]-}`, so no
#   `DUMMY_MODE_SURFACES=` assignment line carries the literal leaf key "surfaces". Its two
#   siblings are NOT exempt — `DUMMY_MODE_ENABLED="$(dm_key enabled false)"` and
#   `DUMMY_MODE_PERSONA="$(dm_key persona '')"` each name their leaf on the assignment line, so
#   they are tied back mechanically like every other entry.
correspondence_exempt="BOARD_SURFACES DOCKET_BASH_PATH CHANGE_TYPES DUMMY_MODE_SURFACES"
# The one elsewhere: key whose consumer mention cannot be code-shaped, and why (change 0246).
# .docket.example.yml says github_project is NOT WIRED TODAY — no script reads it. Its only match in
# docket-config.sh is the coordination-key FENCE list (`for _fkey in … github_project …`), which
# warns-and-ignores the key in machine-scoped layers rather than reading it, so the mention is a
# bare space-delimited token with no code shape at all. The classifier's own comment on that arm
# already calls the anchor "documentation-only, unlike every other elsewhere: entry". Exempting the
# one honest outlier is right; widening the shape set until a fence-list token counts as code would
# re-admit the English-prose match the shapes exist to reject. Asserted to hold exactly this key.
elsewhere_shape_exempt="github_project"
classify_key(){ # classify_key <example-key-name> -> "resolved:EXPORT" | "elsewhere:path" | ""
  case "$1" in
    runtime)              echo 'resolved:DOCKET_BASH_PATH' ;;
    metadata_branch)      echo 'resolved:METADATA_BRANCH' ;;
    integration_branch)   echo 'resolved:INTEGRATION_BRANCH' ;;
    changes_dir)          echo 'resolved:CHANGES_DIR' ;;
    adrs_dir)             echo 'resolved:ADRS_DIR' ;;
    results_dir)          echo 'resolved:RESULTS_DIR' ;;
    finalize.gate)                echo 'resolved:FINALIZE_GATE' ;;
    finalize.test_command)        echo 'resolved:FINALIZE_TEST_COMMAND' ;;
    finalize.require_pr_approval) echo 'resolved:FINALIZE_REQUIRE_PR_APPROVAL' ;;
    finalize.skip_results_only_delta) echo 'resolved:FINALIZE_SKIP_RESULTS_ONLY_DELTA' ;;
    learnings.enabled)            echo 'resolved:LEARNINGS_ENABLED' ;;
    learnings.cap)                echo 'resolved:LEARNINGS_CAP' ;;
    board_surfaces)       echo 'resolved:BOARD_SURFACES' ;;
    auto_groom)           echo 'resolved:AUTO_GROOM' ;;
    change_types)         echo 'resolved:CHANGE_TYPES' ;;
    auto_capture.enabled) echo 'resolved:AUTO_CAPTURE_ENABLED' ;;
    auto_capture.types)   echo 'resolved:AUTO_CAPTURE_TYPES' ;;
    dummy_mode.enabled)   echo 'resolved:DUMMY_MODE_ENABLED' ;;
    dummy_mode.persona)   echo 'resolved:DUMMY_MODE_PERSONA' ;;
    dummy_mode.surfaces)  echo 'resolved:DUMMY_MODE_SURFACES' ;;
    terminal_publish)     echo 'resolved:TERMINAL_PUBLISH' ;;
    gate_observation_budget)      echo 'resolved:GATE_OBSERVATION_BUDGET' ;;
    delegation_observation_budget) echo 'resolved:DELEGATION_OBSERVATION_BUDGET' ;;
    reclaim.lease_ttl)            echo 'resolved:RECLAIM_LEASE_TTL' ;;
    reclaim.auto)                 echo 'resolved:RECLAIM_AUTO' ;;
    build.checkpoint)             echo 'resolved:BUILD_CHECKPOINT' ;;
    review.min_fix_severity)      echo 'resolved:REVIEW_MIN_FIX_SEVERITY' ;;
    review.max_fix_tasks)         echo 'resolved:REVIEW_MAX_FIX_TASKS' ;;
    skills.brainstorm)            echo 'resolved:SKILL_BRAINSTORM' ;;
    skills.plan)                  echo 'resolved:SKILL_PLAN' ;;
    skills.build)                 echo 'resolved:SKILL_BUILD' ;;
    skills.review)                echo 'resolved:SKILL_REVIEW' ;;
    skills.finish)                echo 'resolved:SKILL_FINISH' ;;
    # Block headers carry no value of their own; their children are classified above.
    finalize|learnings|reclaim|build|review|skills|runners|runners.codex|runners.opencode|auto_capture|dummy_mode) echo 'elsewhere:HEADER' ;;
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
    # Read by the GENERATOR, not by an adapter: these govern the shim wrapper's own frontmatter
    # pin (the parent-side relay agent), which is decided at generation time — change 0269.
    runners.codex.shim_model)  echo 'elsewhere:sync-agents.sh' ;;
    runners.codex.shim_effort) echo 'elsewhere:sync-agents.sh' ;;
    runners.codex.sandbox) echo 'elsewhere:scripts/runners/codex.sh' ;;
    runners.codex.network) echo 'elsewhere:scripts/runners/codex.sh' ;;
    runners.opencode.permissions) echo 'elsewhere:scripts/runners/opencode.sh' ;;
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

# code_shaped_mention <leaf-key> <file> -> exit 0 iff a NON-COMMENT line of the file mentions the
# key in a code-shaped context. Backs the elsewhere: arm, replacing a bare word-boundary grep that
# a sentence of English prose satisfied (change 0102's `timeout`-in-a-heredoc false positive).
#
# Two conditions: the line is not a comment (first non-space character is not `#`), and the key
# occurs in one of four shapes, each DERIVED from a real mention in the entries' named
# consumers rather than guessed:
#
#   1. `:`-adjacency   — `agents[[:space:]]*:` (sync-agents.sh, a quoted YAML-key regex) and
#                        `runners.opencode.permissions: auto-approve` (opencode.sh). The optional
#                        literal `[[:space:]]*` is matched because the real mentions are grep
#                        patterns that spell the gap that way.
#   2. dot-qualified   — `codex.network`, `opencode.permissions`: the key as the leaf of a config
#                        path, which is how the runner adapters name it in their die messages.
#   3. flag argument   — `--sandbox` (codex.sh, the `exec … --sandbox "$SANDBOX"` invocation).
#                        runners.codex.sandbox's QUALIFIED form appears nowhere in its consumer;
#                        the flag is the only real mention, which is why this shape is required
#                        and not decorative.
#   4. shell assignment — `shim_model="$(runner_key "$runner" shim_model)"` (sync-agents.sh, change
#                        0269). Added because runners.codex.shim_model / shim_effort are read by
#                        the GENERATOR, whose reader is a layered lookup assigned into a shell
#                        variable of the key's own name: the key never appears there as a literal
#                        YAML key (the awk pattern is built from a `$k` variable), never
#                        dot-qualified (the diagnostics interpolate `runners.$r.$k`), and never as
#                        a flag. Without this shape those two entries have no anchor at all — and
#                        the shape-exemption list is not the answer, since it exists for keys with
#                        NO reader, which is the opposite of these. Bounded on both sides — line
#                        start plus indentation on the left, a mandatory `=` on the right — so it
#                        matches an assignment STATEMENT rather than any occurrence of the name:
#                        neither `--sandbox "$SANDBOX"` nor a sentence of prose can satisfy it,
#                        which is what the negative controls below re-verify. The left boundary is
#                        line-start, NOT any whitespace, and that is load-bearing: a whitespace
#                        boundary also matched the reader's own fallback
#                        (`[ -n "$shim_model" ] || shim_model="inherit"`), so deleting the layered
#                        lookup left the anchor standing on the default it falls back to —
#                        mutation-tested, and red only once the boundary was tightened here.
#                        The assignment's LHS is matched in either casing (the reader's variable is
#                        a global today, `SHIM_MODEL=`); see shape_ere's header for why that is the
#                        same anchor rather than a widening, and the population floor below for
#                        what keeps shape 4 from becoming a general-purpose escape hatch.
#
# Every shape carries a LEFT boundary (change 0246 whole-branch review, IMPORTANT 2). Without one,
# shapes 1 and 3 matched the key as a SUBSTRING of the consumer's own name: `sync-agents:` in a log
# helper satisfied shape 1 and `-agents` satisfied shape 3, so the agents entry was anchored by the
# script's filename and could never fail — strip every `agents:` reader out of sync-agents.sh and
# the manifest assert stayed green, which is precisely the prose-match failure these shapes exist
# to reject. Shape 1's boundary class excludes `-` as well as alnum/underscore, since a hyphen is
# exactly what `sync-agents:` interposes; shape 3's keeps `-` boundary-eligible because `--sandbox`
# needs a hyphen to its left. Shape 2 already had a boundary (the mandatory `[A-Za-z0-9_].` prefix).
# `\b`/`\<`/`\>` are unavailable here (BSD grep), so these are explicit classes — the same form the
# sibling exempt-branch and correspondence greps use. See the two shape controls below.
#
# Deliberately NOT added: assignment/`$var` shapes, or anything else no current entry needs. Every
# shape widens what counts as an anchor, and the failure mode being closed is over-permissiveness.
# All six current targets are shell scripts, so shell shapes suffice; if a PROSE consumer (a
# SKILL.md) is ever reclassified to elsewhere:, this shape set must be revisited rather than
# stretched.
#
# The non-comment filter is captured into a variable rather than piped into `grep -q`: this file
# runs under `set -o pipefail`, and a producer feeding an early-exiting consumer takes SIGPIPE, so
# the pipeline's 141 would intermittently invert a real match into "not code-shaped" (AGENTS.md,
# Shell). `|| true` absorbs grep -v's exit 1 on an all-comment file, which is a legitimate empty
# body and not an error. The ERE uses explicit character classes, never `\b`/`\<`/`\>` — BSD grep
# does not support those and fails silently (change 0246; tests/test_grep_portability.sh).
# The shapes live in ONE function, selectable, because two consumers now need them at different
# widths: code_shaped_mention (all four) and the shape-4 population floor below (shapes 1-3 and
# shape 4 separately, to ask which entries shape 4 is the SOLE anchor for). A second, hand-copied
# spelling of the same ERE would drift from this one and the floor would then be measuring a
# predicate the manifest does not use (LEARNINGS: duplicated-gate-copies-the-whole-predicate).
#
# Shape 4 accepts the key's name AND its upper-case spelling, and that is not a widening for
# convenience: a layered lookup assigned into a shell variable is spelled `KEY=` exactly when the
# variable is a global, which is what sync-agents.sh's shim-pin reader became when it grew a
# run-scoped memo (`SHIM_MODEL="$(runner_key "$runner" shim_model)"`). Same read, same statement,
# same anchor; only the shell's own casing convention differs. Restricting the shape to the
# lower-case spelling would make the anchor a property of a variable NAME rather than of the read
# (LEARNINGS: byte-pattern-guard-matches-a-spelling — the pattern covered one spelling of the
# property, and the equivalent one failed green by going red for the wrong reason). The left
# boundary stays line-start-plus-indent, so the reader's own fallback
# (`[ -n "$SHIM_MODEL" ] || SHIM_MODEL="inherit"`) still does NOT anchor — the property the
# tightened boundary bought is unchanged, and its control below is now asserted in both casings.
shape_ere(){ # $1=leaf key  $2=shape selector: all (default) | 123 | 4
  local k="$1" ku s123 s4
  ku="$(printf '%s' "$k" | tr '[:lower:]' '[:upper:]')"
  s123="(^|[^[:alnum:]_-])$k(\[\[:space:\]\]\*)?:|(^|[^[:alnum:]_])--?$k|[A-Za-z0-9_]\.$k"
  s4="^[[:space:]]*($k|$ku)="
  case "${2:-all}" in
    123) printf '%s' "$s123" ;;
    4)   printf '%s' "$s4" ;;
    *)   printf '%s|%s' "$s123" "$s4" ;;
  esac
}
code_shaped_mention(){ # $1=leaf key  $2=file  [$3=shape selector, default all]
  local k="$1" f="$2" body
  [ -f "$f" ] || return 1
  body="$(grep -vE '^[[:space:]]*#' "$f")" || true
  grep -qE "$(shape_ere "$k" "${3:-all}")" <<<"$body"
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
manifest_bad_keyshape=""
manifest_shape4_only=""
# QUALIFIED extraction (change 0127). A key emits its FULL ancestor path (`learnings.enabled`,
# `runners.codex.sandbox`); a top-level key stays bare. Ancestry is tracked with an indent stack
# rather than "nearest column-0 key", so a doubly-nested leaf is qualified by both its parents
# instead of skipping a level. Indent classes are [[:space:]] / [^[:space:]] throughout, never a
# literal-space class, so a tab-indented block is not silently dropped (AGENTS.md).
# Commented keys stay bare — commented_config_keys only ever yields top-level keys.
example_keys_raw="$(
  { awk '
      { line = $0; sub(/[[:space:]]*#.*/, "", line) }
      line ~ /^[[:space:]]*$/ { next }
      line !~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/ { next }
      {
        match(line, /^[[:space:]]*/); ind = RLENGTH
        key = line
        sub(/^[[:space:]]*/, "", key)
        sub(/[[:space:]]*:.*/, "", key)
        while (top > 0 && indent[top] >= ind) top--
        path = key
        for (i = top; i >= 1; i--) path = names[i] "." path
        print path
        top++; indent[top] = ind; names[top] = key
      }
    ' "$EX"
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
consumers="$consumers $REPO/scripts/runners/opencode.sh"
for k in $example_keys; do
  # LEAF of a qualified key (change 0127): the manifest is keyed by the full path
  # (`learnings.enabled`), but the resolver assigns from, and a consumer script mentions, the BARE
  # leaf (`yaml_get "$LEARN_BLK" enabled`). The greps below therefore anchor on the leaf while the
  # manifest arm, the reporting, and the duplicate checks all stay qualified.
  leaf_k="${k##*.}"
  # ERE-SAFETY OF THE INTERPOLATED KEY (change 0246 whole-branch review, finding 8). Both
  # code_shaped_mention and the exempt branch below interpolate $leaf_k straight into an ERE. Safe
  # today — every elsewhere: leaf key is [a-z_]+ — but a future key carrying `.`, `+` or `[` would
  # SILENTLY widen or break its match rather than redden, against this repo's own
  # escape-ERE-metacharacters-in-key learning. Guarded by an assert rather than by routing through
  # ere_escape (defined far below, for the harness slices): escaping makes a metacharacter key
  # merely work, quietly, whereas this makes it LOUD at the moment it is introduced — which is what
  # the manifest wants, since such a key would also need the doc block above rethought. Cheaper,
  # and it fails closed for both interpolation sites at once.
  case "$leaf_k" in
    *[!A-Za-z0-9_]*) manifest_bad_keyshape="$manifest_bad_keyshape $k" ;;
  esac
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
          # Boundary by explicit class, never \b: BSD grep's and git-grep's ERE do not support \b
          # and return zero SILENTLY, so a \b guard goes blind rather than red off-GNU (change
          # 0246). The leading class needs no (^|...) alternative — `^$exp_name=.*` already
          # guarantees at least one character precedes the key.
          grep -qE "^$exp_name=.*[^[:alnum:]_]$leaf_k([^[:alnum:]_]|$)" "$CFGSCRIPT" \
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
      [ "$(is_header_key "$leaf_k" "$EX")" = "1" ] \
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
      # SHAPE-TIGHTENED (change 0246). A bare word-boundary grep here was satisfied by the key
      # appearing in a comment or in a sentence of English prose — 0102's heredoc-prompt false
      # positive — which anchors nothing. See code_shaped_mention above for the three derived
      # shapes, and elsewhere_shape_exempt for the single documented outlier.
      if [ "$allowlisted" -eq 1 ]; then
        case " $elsewhere_shape_exempt " in
          *" $leaf_k "*)
            # Exempt from the SHAPE requirement, never from the mention requirement: the key must
            # still actually appear in its named consumer, or the entry has no anchor at all.
            grep -qE "(^|[^[:alnum:]_])$leaf_k([^[:alnum:]_]|$)" "$REPO/$consumer" \
              || manifest_bad_consumer="$manifest_bad_consumer $k(not in $consumer)"
            ;;
          *)
            code_shaped_mention "$leaf_k" "$REPO/$consumer" \
              || manifest_bad_consumer="$manifest_bad_consumer $k(no code-shaped mention in $consumer)"
            # POPULATION of shape 4 (change 0269 whole-branch review, finding 10). Record every
            # entry shape 4 is the SOLE anchor for — matched by shape 4 and by none of shapes 1-3.
            # Asserted against an exact set below; see that assert for why the count matters.
            if code_shaped_mention "$leaf_k" "$REPO/$consumer" 4 \
               && ! code_shaped_mention "$leaf_k" "$REPO/$consumer" 123; then
              manifest_shape4_only="$manifest_shape4_only $leaf_k"
            fi
            ;;
        esac
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
assert "manifest: every leaf key is ERE-safe to interpolate — ^[A-Za-z0-9_]+$ (${manifest_bad_keyshape:-none bad})" \
  '[ -z "$manifest_bad_keyshape" ]'
# --- elsewhere: shape guard, its exemption, and its false-positive fixture (change 0246) ---------
# The exemption list is asserted to hold EXACTLY the one key it is allowed to hold. An exemption
# list that can grow silently is a bare allowlist wearing a different name — the drift this whole
# manifest exists to prevent. Widening this list is a deliberate act that must redden here first.
assert "elsewhere: the shape exemption holds exactly github_project (got '$elsewhere_shape_exempt')" \
  '[ "$elsewhere_shape_exempt" = "github_project" ]'
# THE SAME DISCIPLINE FOR SHAPE 4 (change 0269 whole-branch review, finding 10). Shape 4 widens
# what counts as an anchor for ALL of the manifest's keys, while exactly two entries motivated it.
# The cost of that width is silent and specific: any key whose name happens to coincide with a
# shell variable assigned statement-initially somewhere in its declared consumer becomes anchored
# on that assignment — even after the consumer stops reading the YAML key at all, which is the
# decayed-into-an-allowlist failure this whole manifest exists to prevent.
#
# So the population is MEASURED, never written (LEARNINGS: marker-scoped-guard-needs-a-population-
# floor — "at least one entry uses shape 4" pins a population and buys nothing; and
# backstop-must-compute-not-reenumerate). The set is computed in the loop above from the same
# predicate the manifest itself uses, and pinned here to exactly the two entries the shape exists
# for. It reddens in BOTH directions: a shape-4 spelling that stops matching the shim-pin reader
# empties the set, and a third key acquiring a shape-4-only anchor grows it — either way the shape
# gets re-justified deliberately instead of drifting. Mirrors elsewhere_shape_exempt's assert
# directly above, for the same reason.
manifest_shape4_only="$(printf '%s\n' $manifest_shape4_only | sort -u | tr '\n' ' ' | sed -e 's/^ *//' -e 's/ *$//')"
assert "elsewhere: shape 4 is the SOLE anchor for exactly shim_effort + shim_model (got '$manifest_shape4_only')" \
  '[ "$manifest_shape4_only" = "shim_effort shim_model" ]'
# Positive control: the shape predicate must FIRE on a real consumer mention, or every green
# above it is consistent with a predicate that can never match anything.
assert "elsewhere: shape control — a real flag-argument mention is code-shaped" \
  'code_shaped_mention sandbox "$REPO/scripts/runners/codex.sh"'
# PER-SHAPE positive controls (change 0246 whole-branch review, finding 7). The doc block above
# justifies all four shapes as SEPARATELY load-bearing, but only shape 3 had a control, so a typo
# in the `:`-adjacency or dot-qualified alternative would have gone undetected: several live keys
# match by more than one route, which is precisely why these use FIXTURES rather than a real
# consumer. Each fixture exercises exactly ONE alternative, so removing that alternative from the
# ERE reddens exactly this assert (mutation-tested, one alternative at a time).
_shape_fx_colon="$tmp/shape-colon.sh"
printf '%s\n' 'sed -n "s/^agents[[:space:]]*:.*/&/p" "$f"' > "$_shape_fx_colon"
assert 'elsewhere: shape control — shape 1 (`:`-adjacency) alone is code-shaped' \
  'code_shaped_mention agents "$_shape_fx_colon"'
_shape_fx_dot="$tmp/shape-dot.sh"
printf '%s\n' 'die "runners.opencode.permissions must be set"' > "$_shape_fx_dot"
assert "elsewhere: shape control — shape 2 (dot-qualified) alone is code-shaped" \
  'code_shaped_mention permissions "$_shape_fx_dot"'
# Shape 4's fixture is sync-agents.sh's real reader line, which matches by shape 4 ONLY: there is
# no `shim_model:`, no `<word>.shim_model`, and no `--shim_model` anywhere in it.
_shape_fx_assign="$tmp/shape-assign.sh"
printf '%s\n' '  shim_model="$(runner_key "$runner" shim_model)"' > "$_shape_fx_assign"
assert "elsewhere: shape control — shape 4 (shell assignment) alone is code-shaped" \
  'code_shaped_mention shim_model "$_shape_fx_assign"'
# And the shape must be an assignment STATEMENT, not the name appearing anywhere: the same line
# with the assignment removed (the key surviving only as a bare argument word) must NOT anchor.
_shape_fx_argword="$tmp/shape-argword.sh"
printf '%s\n' '  resolve_pin "$runner" shim_model' > "$_shape_fx_argword"
assert "elsewhere: shape control — a bare argument word is NOT a code-shaped mention" \
  '! code_shaped_mention shim_model "$_shape_fx_argword"'
# The reader's real spelling today: the layered lookup assigned into a shell GLOBAL, which the
# shell's own convention upper-cases. Same statement, same read; only the casing differs, so
# shape 4 must anchor it (see shape_ere's header).
_shape_fx_assign_glob="$tmp/shape-assign-global.sh"
printf '%s\n' '  SHIM_MODEL="$(runner_key "$runner" shim_model)"' > "$_shape_fx_assign_glob"
assert "elsewhere: shape control — shape 4 anchors the upper-cased global assignment too" \
  'code_shaped_mention shim_model "$_shape_fx_assign_glob" 4'
# And the LEFT boundary survives the casing: the reader's own default fallback is an assignment,
# but not a statement-initial one, so it must still fail to anchor. This is the property that made
# deleting the layered lookup redden (mutation-tested at change 0269); it is re-asserted here in
# the upper-case spelling so the widening above cannot quietly give it back.
_shape_fx_fallback="$tmp/shape-fallback.sh"
printf '%s\n' '  [ -n "$SHIM_MODEL" ] || SHIM_MODEL="inherit"' > "$_shape_fx_fallback"
assert "elsewhere: shape control — the reader's own fallback assignment does NOT anchor" \
  '! code_shaped_mention shim_model "$_shape_fx_fallback"'
# NEGATIVE control reproducing the historical false positive (change 0102): a key appearing ONLY as
# an English word in prose, inside an embedded heredoc prompt, on non-comment lines. A bare
# word-boundary grep passes this; the shape predicate must not. Comment-region exclusion alone
# would also fail here, which is why the shapes exist.
_shape_fx="$tmp/shape-prose.sh"
{
  printf 'run_it(){\n'
  printf '  cat <<PROMPT\n'
  printf 'Please choose an appropriate timeout before you continue running the job.\n'
  printf 'PROMPT\n'
  printf '}\n'
} > "$_shape_fx"
assert "elsewhere: shape control — a key appearing only in heredoc prose is NOT code-shaped" \
  '! code_shaped_mention timeout "$_shape_fx"'
# NEGATIVE control for the LEFT boundary (change 0246 whole-branch review, IMPORTANT 2). Shapes 1
# and 3 originally carried no left boundary, so `agents` was anchored by its consumer's own NAME:
# `sync-agents:` satisfies the `<key>:` shape as a substring and `-agents` satisfies the flag
# shape. That made the agents entry UNFALSIFIABLE — every `agents:` reader could be stripped out of
# sync-agents.sh and the manifest assert above would stay green. This fixture is the reviewer's
# reproduction: a file whose entire content is a log helper naming the script, with zero `agents:`
# key handling. It must NOT be code-shaped. Note the boundary class for shape 1 excludes `-` as
# well (a hyphen is what `sync-agents:` interposes), while shape 3 keeps `-` boundary-eligible
# because `--sandbox` needs it.
_shape_fx_name="$tmp/shape-selfname.sh"
printf '%s\n' 'log(){ printf "%s\n" "sync-agents: $*" >&2; }' > "$_shape_fx_name"
assert "elsewhere: shape control — a consumer's own hyphenated NAME is NOT a code-shaped mention of the key" \
  '! code_shaped_mention agents "$_shape_fx_name"'
# Positive counterpart, so the control above cannot pass by the predicate simply never matching in
# this file: the REAL reader in sync-agents.sh must still anchor the agents entry.
assert "elsewhere: shape control — the real '^agents[[:space:]]*:' reader IS a code-shaped mention" \
  'code_shaped_mention agents "$REPO/sync-agents.sh"'
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
# intentional-growth remedy in action: change 0127 took it from 33 to 36 (change_types plus the
# auto_capture block's header and its two leaves, less the retired scalar auto_capture); change
# 0167 took it from 36 to 38 (the build: block header and its checkpoint leaf); change 0205 took
# it from 38 to 40 (the runners.opencode header and its permissions leaf); change 0218 took it
# from 40 to 42 (the review: block header and its min_fix_severity leaf), then from 42 to 43 when
# the fix loop's hard-coded cap became the review.max_fix_tasks leaf; change 0223 took it from 43
# to 44 (the flat top-level gate_observation_budget key).
# intentional-growth remedy this count is guarding. This is the single source for that count: the
# condition and the failure message below both read it, so bumping it in one place updates both
# instead of leaving one stale.
# change 0269 took it from 45 to 47 (runners.codex.shim_model and runners.codex.shim_effort).
# change 0271 took it from 47 to 48 (the flat top-level delegation_observation_budget key).
# change 0276 took it from 48 to 52 (the dummy_mode: block header and its enabled/persona/surfaces
# leaves).
expected_key_count=52
# RAW FLOOR (change 0102 whole-branch review, MINOR 3): example_keys_raw feeds BOTH this section's
# manifest loop (via example_keys, deduped) and the duplicate-leaf check directly below (also
# fed from example_keys_raw, undeduped). Without this assert, an edit that makes the raw pipeline
# emit nothing would silently starve the duplicate-leaf check (`uniq -d` on empty input is empty,
# which reads as "no duplicates" — green forever) even though mf_count's OWN assert below would
# already have caught the manifest-loop side. Asserted against the same expected_key_count rather
# than a second magic number: the raw list is always >= the deduped one.
raw_count="$(grep <<<"$example_keys_raw" -c .)"
assert "manifest: raw key extraction is non-vacuous (>= $expected_key_count; got $raw_count)" \
  '[ "$raw_count" -ge "$expected_key_count" ]'
mf_count="$(grep <<<"$example_keys" -c .)"
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
assert "no duplicate QUALIFIED key names in the example (${dup_leaf_keys:-none}; two identical paths are a real ambiguity)" \
  '[ -z "$dup_leaf_keys" ]'

# (2b-i) QUALIFIED-KEY EXTRACTION (change 0127).
# A bare leaf name is only ambiguous when the RESOLVER READS IT FLAT — `yaml_get` over the whole
# file, whose `head -n1` picks whichever line comes first (finalize.gate arrives as `lcl gate`).
# Block-scoped leaves are read inside their own `yaml_block_body`, so `learnings.enabled` and
# `auto_capture.enabled` are genuinely distinct to the resolver and must be distinct here too.
# Qualifying by the full ancestor path is what lets change 0127 document auto_capture.enabled at
# all: the previous bare-leaf check rejected it outright because learnings.enabled owned `enabled`.
assert "qualified extraction: a nested leaf carries its parent" \
  'grep -qx "learnings.enabled" <<<"$example_keys_raw"'
assert "qualified extraction: a top-level key stays bare" \
  'grep -qx "board_surfaces" <<<"$example_keys_raw"'
assert "qualified extraction: the finalize block is qualified too" \
  'grep -qx "finalize.gate" <<<"$example_keys_raw"'
assert "qualified extraction: a doubly-nested leaf carries its FULL path" \
  'grep -qx "runners.codex.sandbox" <<<"$example_keys_raw"'
assert "qualified extraction: a commented top-level key stays bare" \
  'grep -qx "agent_harnesses" <<<"$example_keys_raw"'

# FLAT-READ COLLISION FLOOR — the duplicate check that actually protects `yaml_get`. Its
# population is derived from the resolver's READ SHAPE, not from an allowlist of names: every
# top-level key (read as a bare key by definition) plus the `finalize.*` leaves, which are the one
# nested block still read flat (`lcl gate` / `yaml_get "$CFG" gate`, never block-scoped). If a
# future block joins the flat-read family, add its prefix to the sed below in the same commit.
flat_read_keys="$(printf '%s\n' "$example_keys_raw" \
  | sed -nE 's/^(finalize\.)?([A-Za-z_][A-Za-z0-9_]*)$/\2/p')"
dup_flat_keys="$(printf '%s\n' "$flat_read_keys" | sort | uniq -d | tr '\n' ' ')"
assert "no duplicate FLAT-READ leaf names (${dup_flat_keys:-none}; yaml_get's head -n1 would mis-resolve these)" \
  '[ -z "${dup_flat_keys// /}" ]'
flat_count="$(grep -c . <<<"$flat_read_keys")"
assert "flat-read floor is non-vacuous (>= 10 keys; got $flat_count)" \
  '[ "$flat_count" -ge 10 ]'

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
assert "completeness: runners.opencode.permissions present" \
  'grep -Eq "^[[:space:]]+permissions:[[:space:]]*ask[[:space:]]*(#.*)?$" "$EX"'
assert "completeness: runners.codex.shim_model present AND documents the code default (inherit)" \
  'grep -Eq "^[[:space:]]+shim_model:[[:space:]]*inherit[[:space:]]*(#.*)?$" "$EX"'
assert "completeness: runners.codex.shim_effort present AND documents the code default (low)" \
  'grep -Eq "^[[:space:]]+shim_effort:[[:space:]]*low[[:space:]]*(#.*)?$" "$EX"'
assert "completeness: runners block header present" 'grep -Eq "^runners:" "$EX"'

# change 0102: require_pr_approval is now RESOLVER-read and global-able, so it carries the
# standard any-layer tag like its two finalize siblings. The pre-0102 example carried a bespoke
# three-line note asserting the opposite (repo-committed only, silently ignored elsewhere) —
# that text described a state that no longer exists, and this pair keeps it from coming back.
# Window captured first, exactly like its 0190 sibling below — never `awk … | grep -q`, which
# SIGPIPEs the producer under pipefail. An empty window still reddens here (grep finds nothing in
# it), so this leg needs no separate non-vacuity anchor the way the NEGATED guards do.
rpa_window="$(awk '/^  # require_pr_approval/,/^  require_pr_approval:/' "$EX")"
assert "0102: require_pr_approval carries the any-layer scope tag" \
  'grep -qF "scope: any layer" <<<"$rpa_window"'
assert "0102: the stale repo-committed-only note is gone" \
  '! grep -qF "read by the finalize SKILL BODY, not by the config" "$EX"'

# change 0190: skip_results_only_delta is the ONE finalize leaf that IS coordination-fenced, so it
# must carry the repo-only tag and NOT the any-layer tag its two siblings carry. Both directions
# are pinned, on the key's OWN comment window: a sibling block copy-pasted for the new key would
# document a scope the resolver contradicts — change 0102's exact failure, on this very block. The
# window is captured into a variable first (never `awk | grep -q`, which SIGPIPEs the producer
# under pipefail) and anchored for non-vacuity, so a renamed key yields a loud red rather than an
# empty haystack both greps sail through.
skip_delta_window="$(awk '/^  # skip_results_only_delta/,/^  skip_results_only_delta:/' "$EX")"
assert "0190: the skip_results_only_delta comment window was located (non-vacuity anchor)" \
  '[ -n "$skip_delta_window" ] && grep -qF "skip_results_only_delta: false" <<<"$skip_delta_window"'
assert "0190: skip_results_only_delta carries the repo-only scope tag" \
  'grep -qF "scope: repo-only (coordination-fenced, ADR-0019)" <<<"$skip_delta_window"'
assert "0190: skip_results_only_delta does NOT carry its siblings' any-layer tag" \
  '! grep -qF "scope: any layer" <<<"$skip_delta_window"'

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
  grep -qlE "(^|[^[:alnum:]_])$k([^[:alnum:]_]|$)" $consumers >/dev/null 2>&1 || orphan_keys="$orphan_keys $k"
done
assert "no orphan keys: every active top-level key is read by a consumer (${orphan_keys:-none})" \
  '[ -z "$orphan_keys" ]'

# runners.* is consumed by the runner-dispatch script family, not the resolver. Anchor on the
# PRODUCER so the example and its consumer cannot silently diverge (same shape as the
# require_pr_approval producer assert above).
assert "runners.codex.sandbox is still read by the codex adapter" \
  'grep -q "DOCKET_RUNNER_CFG_SANDBOX" "$REPO/scripts/runners/codex.sh"'
assert "runners.opencode.permissions is still read by the opencode adapter" \
  'grep -q "DOCKET_RUNNER_CFG_PERMISSIONS" "$REPO/scripts/runners/opencode.sh"'

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
#
# ORDERING COUPLING (change 0190): the positive assert above is a PROXIMITY guard — at most 20
# characters may sit between `FINALIZE_REQUIRE_PR_APPROVAL` and "never by parsing". The finalize
# SKILL's framing sentence names the exported keys the gate reads as a list, so
# FINALIZE_REQUIRE_PR_APPROVAL must stay the LAST name in that list; a key appended after it pushes
# the phrase out of range and reddens this pair for a reason that is not the contract it guards.
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

# Scope tags: all three forms present, and every ACTIVE SCALAR key at EVERY nesting depth is
# covered by a scope tag — a real per-key check, not just "the phrase occurs somewhere in the
# file" (which the three asserts below alone would only prove).
#
# The pass finds each active (uncommented) key's own preceding comment "window", bounded by the
# nearest neighbor above among: a section banner (# ═══...), another active key at ANY depth, or
# a commented pseudo-key (# agent_harnesses: / # agents:). Four rules (change 0122):
#
#   1. A SCALAR key (anything after the colon) must be covered. A HEADER key (a mapping opener
#      like `finalize:`, nothing after the colon) is never itself required to carry a tag — it
#      may PROVIDE one for its subtree, but a container has no scope of its own to assert.
#   2. Coverage = the key's own window carries a sanctioned tag, ELSE the nearest enclosing
#      header block's own window does. Both of the file's conventions are therefore legal:
#      finalize/learnings/reclaim tag each child individually, while auto_capture/runners/skills
#      tag the block header and let the children inherit.
#   3. A header's window is its OWN preceding comment lines ONLY, and is NEVER extended forward
#      into its body. This is the anti-masking rule. The pre-0122 guard did extend it forward,
#      so ANY ONE child's tag satisfied the header and no child was ever checked individually —
#      which is exactly how change 0102 shipped `finalize.require_pr_approval` carrying a
#      bespoke note claiming the OPPOSITE of its real scope, with the suite fully green.
#   4. A scalar key with a genuinely empty window (no comment lines of its own, immediately
#      adjacent to the previous key AT THE SAME DEPTH) inherits that key's coverage — this is
#      the changes_dir / adrs_dir / results_dir group, one shared comment block above all three.
#
# Failures are reported as DOTTED PATHS (`finalize.gate`), because a bare leaf name is ambiguous
# between learnings.enabled and auto_capture.enabled.
#
# The program is HOISTED into $scope_guard_awk rather than written inline, so the mutation
# self-tests below run LITERALLY THIS PROGRAM. A hand-copied second inline copy would be a
# guard that tests a different program than the one that ships (plan-supplied-test-code-is-
# unverified). The heredoc delimiter is single-quoted so awk's $0 is not shell-expanded.
assert "scope tag: repo-only form present"  'grep -qF "scope: repo-only (coordination-fenced, ADR-0019)" "$EX"'
assert "scope tag: any-layer form present"  'grep -qF "scope: any layer" "$EX"'
assert "scope tag: local-only form present" 'grep -qF "scope: local-only" "$EX"'
scope_guard_awk="$(cat <<'SCOPE_GUARD_AWK'
{
  content[NR] = $0
  # SENSITIVITY (change 0102 whole-branch review, MINOR 3): this anchor is [[:space:]]*, not
  # column-0, so it also matches an indented line that merely LOOKS like a key. There are no YAML
  # block scalars (`note: |` followed by indented prose) in this file today, so that never fires
  # a false key — but if one is ever added, an indented line under it would be misread as a real
  # nested key and inflate nested_key_count. No code change made for this: the exact-count floor
  # below already makes that loud (the count would jump and redden), so this comment exists only
  # to explain WHY the count jumped when it eventually does.
  is_active = ($0 ~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/)
  is_pseudo = ($0 ~ /^# (agent_harnesses|agents):/)
  is_banner = ($0 ~ /^#[[:space:]]*═══/)
  if (is_active || is_pseudo || is_banner) { nb++; bnd[nb] = NR }
  if (is_active) {
    nk++
    keyline[nk] = NR
    match($0, /^[[:space:]]*/); keydepth[nk] = RLENGTH
    rest = $0
    sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/, "", rest)
    keytype[nk] = (rest ~ /^[[:space:]]*(#.*)?$/) ? "H" : "S"
    nm = $0
    sub(/^[[:space:]]*/, "", nm)
    sub(/[[:space:]]*:.*/, "", nm)
    keyname[nk] = nm
    bndidx[nk] = nb
  }
}
END {
  for (k = 1; k <= nk; k++) {
    idx = bndidx[k]
    prevB = (idx > 1) ? bnd[idx-1] : 0
    winStart = prevB + 1
    winEnd = keyline[k]
    own = 0
    for (l = winStart; l <= winEnd; l++) {
      # Change 0102 whole-branch review, MINOR 2: require the matched line to itself LOOK LIKE A
      # COMMENT, not merely contain one of the three sanctioned tag strings. Without this, a
      # window that happens to include prose quoting a tag verbatim (this file's own legend, at
      # the top, does exactly that inside a "# ═══" banner-sealed block) would grant vacuous
      # coverage to the first active key ever added above the first banner.
      if (content[l] !~ /^[[:space:]]*#/) continue
      if (content[l] ~ /scope: repo-only \(coordination-fenced, ADR-0019\)/) own = 1
      if (content[l] ~ /scope: any layer/) own = 1
      if (content[l] ~ /scope: local-only/) own = 1
    }
    while (top > 0 && sdepth[top] >= keydepth[k]) top--
    path = keyname[k]
    for (i = top; i >= 1; i--) path = sname[i] "." path
    covered = own
    if (!covered) { for (i = top; i >= 1; i--) if (sown[i]) { covered = 1; break } }
    # Change 0102 whole-branch review, IMPORTANT 1(b): rule 4's adjacency inheritance must be LOUD,
    # not free. It is meant for exactly one shared-comment group (changes_dir/adrs_dir/results_dir,
    # inheriting from changes_dir), but as written it also silently covers a key added with NO
    # comment of its own directly beneath ANY tagged same-depth sibling — the cheapest way to add
    # an untagged key evades the guard entirely. Count every time this branch actually FIRES (the
    # adjacency condition matched, regardless of what effcov[k-1] turns out to be), independent of
    # nested/COUNT, so a new adjacency-inherited key moves this counter even when it moves nothing
    # else.
    if (!covered && keytype[k] == "S" && k > 1 && winStart == keyline[k] && prevB == keyline[k-1] && keydepth[k] == keydepth[k-1]) { covered = effcov[k-1]; adjinherit++ }
    effcov[k] = covered
    if (keydepth[k] > 0) nested++
    if (keytype[k] == "S" && !covered) print path
    top++; sdepth[top] = keydepth[k]; sname[top] = keyname[k]; sown[top] = own
  }
  print "COUNT " nested
  # Distinct, collision-proof prefix (mirrors COUNT's own safety argument): dotted paths never
  # contain a space, and "ADJINHERIT " (with its trailing space) is not a form any key name or
  # path can take, so splitting on "^ADJINHERIT " can never eat a real uncovered-key line.
  print "ADJINHERIT " adjinherit
}
SCOPE_GUARD_AWK
)"
# The pass emits three streams on one stdout: uncovered dotted paths, a trailing COUNT line, and
# a trailing ADJINHERIT line. Split them, so neither trailer can make the emptiness assert
# unconditionally false.
scope_guard_out="$(awk "$scope_guard_awk" "$EX")"
nested_key_count="$(printf '%s\n' "$scope_guard_out" | sed -n 's/^COUNT //p')"
adjacency_inherit_count="$(printf '%s\n' "$scope_guard_out" | sed -n 's/^ADJINHERIT //p')"
untagged_keys="$(grep <<<"$scope_guard_out" -v '^COUNT ' | grep -v '^ADJINHERIT ')"
assert "scope tag: every ACTIVE SCALAR key at every depth is covered by a scope tag" \
  '[ -z "$untagged_keys" ]'
if [ -n "$untagged_keys" ]; then
  echo "--- keys with no scope tag (own or inherited), as dotted paths ---"
  printf '%s\n' "$untagged_keys"
  echo "---"
fi

# POPULATION FLOOR — EXACT, and emitted by the guard's OWN pass (change 0122).
# The emptiness assert above is green both when every key is covered AND when the pass enumerated
# nothing at all; only a floor distinguishes those. The count MUST come from $scope_guard_out —
# NOT from example_keys_raw's qualified extractor above, which would keep this green while the
# guard's own pass reached zero nested keys, i.e. exactly the vacuity this assert exists to catch.
#
# EXACT, not >=. An at-least floor of 15 is satisfied by the PRE-0102 file and would tolerate a
# regression that silently drops both runners.codex leaves. The 23: 4 finalize.* (change 0190 added
# skip_results_only_delta, which carries its OWN `# scope: repo-only` tag — the only finalize leaf
# that is coordination-fenced), 2 learnings.*,
# 2 reclaim.*, 1 build.checkpoint, 2 review.* (change 0218 — min_fix_severity, and max_fix_tasks
# added when the fix loop's cap became configurable; EACH carries its OWN `# scope: any layer` tag,
# so both are covered by rule 1, not by adjacency), 2 auto_capture.*,
# runners.codex + its 4 leaves, runners.opencode + its 1 leaf, 5 skills.*.
#
# Change 0269 took it from 23 to 25: runners.codex gained shim_model and shim_effort. Both are
# covered by rule 2 — the `runners:` header's own window carries the `# scope: any layer` tag and
# neither leaf carries one of its own — which is the same coverage its sandbox/network siblings
# already have, so expected_adjacency_inherit_count below deliberately does NOT move: rule 4 only
# fires for a key that is otherwise UNCOVERED, and these two never are.
#
# +3 (change 0276): dummy_mode.{enabled,persona,surfaces} — all three covered by rule 2, the
# `dummy_mode:` header's own `# scope: any layer` tag; none inherits via rule-4 adjacency, so
# expected_adjacency_inherit_count is unchanged at 2.
expected_nested_key_count=28
assert "scope tag: the pass enumerated exactly $expected_nested_key_count keys at depth > 0 (got ${nested_key_count:-0}; if you added or removed a nested key in .docket.example.yml, first CONFIRM the new key carries its own scope: tag or sits directly under a tagged header — bumping expected_nested_key_count alone, with no tag and no header, ships an untagged key that this guard will never catch again — then bump expected_nested_key_count in the same commit)" \
  '[ "${nested_key_count:-0}" = "$expected_nested_key_count" ]'

# ADJACENCY-INHERITANCE POPULATION FLOOR — EXACT (change 0102 whole-branch review, IMPORTANT 1).
# Rule 4 (same-depth adjacency inheritance with a genuinely empty window) exists for exactly one
# group today: adrs_dir and results_dir, both inheriting coverage from changes_dir. Left
# unguarded, that rule is also the cheapest way to ship an untagged key — add it with no comment
# at all, directly beneath any tagged same-depth sibling, and rule 4 covers it for free, COUNT
# alone moves, and "bump expected_nested_key_count" launders it back to green. This floor makes
# that path loud: a THIRD adjacency-inherited key (or a first one arising from an unrelated
# sibling) moves this counter independently of nested_key_count.
expected_adjacency_inherit_count=2
assert "scope tag: exactly $expected_adjacency_inherit_count keys inherit coverage via rule-4 same-depth adjacency (got ${adjacency_inherit_count:-0}; a new adjacency-inherited key must either be given its OWN scope: tag (closing the gap for real) or be deliberately accounted for by bumping expected_adjacency_inherit_count in the same commit, with a note on why it is safe to leave untagged)" \
  '[ "${adjacency_inherit_count:-0}" = "$expected_adjacency_inherit_count" ]'

# GUARD-THE-GUARD (change 0122). The asserts above are green on a correct file; these prove the
# pass actually goes RED on the drift it exists to catch. All THREE mutation self-tests below —
# mut-gate, mut-skills, mut-0102 — run $scope_guard_awk, literally the program that ships, over a
# MUTATED COPY in $tmp. The real .docket.example.yml is never touched. Deletions are anchored on
# the KEY LINE'S CONTENT, not on a line number, because the population floor above explicitly
# anticipates this file gaining keys.
#
# drop_tag_above <file> <key-line-regex> — deletes the `scope:` comment line sitting immediately
# above EVERY line matching the regex (no first-match short-circuit — the loop below runs to NR
# for every line and `continue`s each one it drops, so a regex matching more than one line drops
# a tag above each occurrence). Emits the mutated file on stdout.
drop_tag_above(){
  awk -v pat="$2" '
    { b[NR] = $0 }
    END {
      for (i = 1; i <= NR; i++) {
        if (b[i+1] ~ pat && b[i] ~ /scope:/) continue
        print b[i]
      }
    }
  ' "$1"
}

# (a) A key that carries its OWN tag: finalize.gate. Under the PRE-0122 guard this mutation was
# green — the finalize: header's window extended forward and its two siblings' tags satisfied it.
drop_tag_above "$EX" '^  gate:' > "$tmp/mut-gate.yml"
mut_gate_out="$(awk "$scope_guard_awk" "$tmp/mut-gate.yml" | grep -v '^COUNT ' | grep -v '^ADJINHERIT ')"
assert "guard-the-guard: dropping finalize.gate's own tag is REPORTED (got '${mut_gate_out}')" \
  '[ "$mut_gate_out" = "finalize.gate" ]'

# (b) A block whose children INHERIT: skills. Dropping the header's tag must report all five
# leaves, since none carries a tag of its own — this is the inheritance half of rule 2.
drop_tag_above "$EX" '^skills:' > "$tmp/mut-skills.yml"
mut_skills_out="$(awk "$scope_guard_awk" "$tmp/mut-skills.yml" | grep -v '^COUNT ' | grep -v '^ADJINHERIT ' | sort | tr '\n' ' ')"
assert "guard-the-guard: dropping the skills: header tag reports all five leaves (got '${mut_skills_out}')" \
  '[ "$mut_skills_out" = "skills.brainstorm skills.build skills.finish skills.plan skills.review " ]'

# (c) THE ANTI-MASKING REGRESSION, reproduced. This is change 0102's exact bug: a finalize child
# whose window holds no sanctioned tag, while its two siblings remain tagged. The pre-0122 guard
# was GREEN here — which is how the bug shipped. Rule 3 is the only reason this is now red.
drop_tag_above "$EX" '^  require_pr_approval:' > "$tmp/mut-0102.yml"
mut_0102_out="$(awk "$scope_guard_awk" "$tmp/mut-0102.yml" | grep -v '^COUNT ' | grep -v '^ADJINHERIT ')"
assert "guard-the-guard: the 0102 regression (an untagged finalize sibling) is REPORTED (got '${mut_0102_out}')" \
  '[ "$mut_0102_out" = "finalize.require_pr_approval" ]'

# Non-vacuity for the mutations themselves: a drop_tag_above that silently matched nothing would
# leave the copy identical to the original, and all three asserts would then be comparing the
# guard's clean (empty) output against a non-empty expectation — i.e. they'd fail loudly rather
# than pass falsely. But an inverted bug (the helper deleting too much) is silent, so pin the
# damage: each mutated copy must be EXACTLY one line shorter than the original.
#
# ASSUMPTION (change 0102 whole-branch review, MINOR 5): this delta computation assumes $EX ends
# in a trailing newline. `wc -l` counts newlines, not lines, so a file with no trailing newline
# would undercount by one on BOTH sides of the subtraction in a way that happens to cancel out
# only if the "same" byte is missing from the mutated copy too — in practice, a source file with
# no trailing final newline makes awk's `print` (inside drop_tag_above) ADD one that the original
# lacks, so the delta would silently read one line SHORTER than reality, redding all three asserts
# below for a reason unrelated to the mutation itself. $EX does end in a trailing newline today
# (verified: `xxd` shows a final 0a), so this is latent, not live — and any regression is loud
# (these asserts fail), not silent. No code change made; recorded here so a future trailing-
# newline change to .docket.example.yml is understood as the cause if these three go red.
for mf in mut-gate mut-skills mut-0102; do
  assert "guard-the-guard: $mf.yml differs from the original by exactly one deleted line" \
    '[ "$(( $(wc -l < "$EX") - $(wc -l < "$tmp/'"$mf"'.yml") ))" = "1" ]'
done

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
#
# The excerpt is captured ONCE into a variable and searched with a here-string. As a pipeline
# (`sed … "$EX" | grep -Eq …`) under this file's `set -o pipefail` these four guards were not
# merely flaky, they were SELF-DEFEATING: SIGPIPE reaches the producer only when the consumer
# exits EARLY, and `grep -Eq` exits early only when it MATCHES — that is, only when the guard is
# supposed to fire. The 141 then inverts through the leading `!` into a green assert, so the one
# case these asserts exist to catch is the one case they could not report. The excerpt is 7KB
# today, under a 16KB pipe, so the inversion is still latent — but it grows by ~2KB per shipped
# harness block, and nothing would have announced the crossing.
agents_excerpt="$(sed -n '/^# agents:$/,/^runners:$/p' "$EX")"
# NON-VACUITY ANCHOR for the four negated guards below: a renamed header would leave the sed range
# empty, and an empty haystack satisfies every `! grep` in the group silently.
assert "the commented agents: excerpt was located (non-vacuity anchor for the ACTIVE-header guards)" \
  '[ -n "$agents_excerpt" ] && grep -qF "# agents:" <<<"$agents_excerpt" &&
   grep -Eq "^runners:" <<<"$agents_excerpt"'
assert "no ACTIVE codex: header under agents:" \
  '! grep -Eq "^[[:space:]]*codex:[[:space:]]*$" <<<"$agents_excerpt"'
assert "no ACTIVE cursor: header under agents:" \
  '! grep -Eq "^[[:space:]]*cursor:[[:space:]]*$" <<<"$agents_excerpt"'
assert "no ACTIVE opencode: header under agents:" \
  '! grep -Eq "^[[:space:]]*opencode:[[:space:]]*$" <<<"$agents_excerpt"'
presence_sensitive_marker_count="$(grep -cF "PRESENCE-SENSITIVE: uncommenting this key changes behavior" "$EX")"
presence_sensitive_expected="$(printf '%s\n' $presence_sensitive_keys | grep -c .)"
assert "PRESENCE-SENSITIVE marker count is exactly $presence_sensitive_expected, matching presence_sensitive_keys ($presence_sensitive_keys; got $presence_sensitive_marker_count; a new commented PRESENCE-SENSITIVE key must add its name to presence_sensitive_keys near the top of (2b), in the same commit as its marker comment)" \
  '[ "$presence_sensitive_marker_count" = "$presence_sensitive_expected" ]'
# ...but the commented examples ARE present, so a user can find and enable them. All three shipped
# harnesses are mirrors of the shipped sidecar and sit at the SAME single-comment level.
#
# Change 0169 shipped the codex block, so codex joins claude and cursor as a MIRROR and sits at the
# same single-comment level. The doubly-commented level is asserted ABSENT rather than the assert
# being deleted: an accidental second '#' would silently demote a shipped mirror back to an
# illustration, which is the exact regression this pair of asserts exists to catch.
assert "codex example is singly commented, like claude and cursor (all three mirror the sidecar)" \
  'grep -Eq "^#[[:space:]]+codex:[[:space:]]*$" "$EX" && ! grep -Eq "^#[[:space:]]+#[[:space:]]*codex:[[:space:]]*$" "$EX"'
assert "no doubly-commented harness block survives under agents:" \
  '! grep -Eq "^#[[:space:]]+#[[:space:]]*[a-z]+:[[:space:]]*$" <<<"$agents_excerpt"'
# Both legs anchor the header at END OF LINE: prose that merely mentions `# cursor:` in a
# neighbouring comment is not a block header, and matching it would fail this assert for a comment
# reflow rather than for a comment-level change.
assert "cursor example is singly commented, like claude (both mirror the shipped sidecar)" \
  'grep -Eq "^#[[:space:]]+cursor:[[:space:]]*$" "$EX" && ! grep -Eq "^#[[:space:]]+#[[:space:]]*cursor:[[:space:]]*$" "$EX"'
assert "claude example is singly commented" \
  'grep -Eq "^#[[:space:]]+claude:[[:space:]]*$" "$EX"'
# Change 0192 shipped the opencode block, the fourth mirror. Same pair as codex: present at the
# single-comment level, absent at the doubly-commented one.
assert "opencode example is singly commented, like the other three mirrors" \
  'grep -Eq "^#[[:space:]]+opencode:[[:space:]]*$" "$EX" && ! grep -Eq "^#[[:space:]]+#[[:space:]]*opencode:[[:space:]]*$" "$EX"'

# --- (4) MIRROR EQUALITY: relocated ADR-0039 ---------------------------------
# The commented agents.claude block mirrors docket's SHIPPED defaults VALUE FOR VALUE. Change 0168
# moved those out of agents/docket-*.md frontmatter and into agents/harness-defaults.yml, so the
# sidecar is what LEADS and this file mirrors. Reading the old side with fm() would not merely be
# stale: fm() is a first-match-ANYWHERE read, so with no `model:` line left in a source's
# frontmatter it scans on into the body and can return prose.
# First match taken by parameter expansion, not `sed … | head -n1`: an early-exiting consumer
# SIGPIPEs its producer under this file's `set -o pipefail` (AGENTS.md § Shell). Here that only
# corrupted the pipeline's STATUS and not the value, since head has already emitted line 1 — but
# the shape is the one the fidelity assert above lost a race to, and it is not kept alive here.
fm(){ local _all _first
  _all="$(sed -n "s/^$2:[[:space:]]*//p" "$1")"
  _first="${_all%%$'\n'*}"
  printf '%s\n' "${_first%"${_first##*[![:space:]]}"}"
}
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
# Every mirrored block is read the SAME way, through a per-harness slice.
#
# The slice is not decoration. All three harness blocks sit at the same comment level, so a
# whole-file single strip makes their rows indistinguishable by key alone — every agent name
# appears in both blocks, and a `head -n1` would silently resolve every lookup to whichever block
# comes first in the file. That would leave the cursor legs asserting claude's values against
# cursor's sidecar entries, i.e. failing for the right reason today but incapable of ever passing.
# Slicing first is what keeps the two comparisons independent. (Before this amendment the cursor
# block was doubly commented and needed a two-layer strip; that asymmetry is gone.)
#
# The value class matches hd_field's: "up to the flow-map delimiter", not a character allowlist.
# A narrower class would clip a provider-prefixed ID on BOTH sides of the comparison and mirror
# a truncated prefix to a truncated prefix — a false green (0168 whole-branch review).
ex_slice(){ # $1=harness  $2=ERE anchoring the block's LAST line
  # The terminator is DERIVED from the sidecar's build-max model, so it can contain any character
  # the bare-scalar rule allows — including `/`, which opencode's OpenRouter IDs carry (change
  # 0192). An unescaped `/` closes sed's address delimiter and the whole expression dies with
  # "invalid command code", which surfaces as an empty slice and reads like a missing block rather
  # than a quoting bug. Escape the delimiter here, in the one place that owns the address, rather
  # than asking every caller to pre-escape.
  local term="${2//\//\\/}"
  sed -n "/^#[[:space:]]*$1:[[:space:]]*$/,/$term/p" "$EX" | sed -E 's/^[[:space:]]*#[[:space:]]?//'
}
ex_slice_field(){ # $1=slice  $2=agent  $3=field(model|effort)
  local line
  line="$(grep -E "^[[:space:]]*$2:[[:space:]]" <<<"$1" || true)"
  line="${line%%$'\n'*}"
  sed -nE "s/.*[{,[:space:]]$3[[:space:]]*:[[:space:]]*([^,}[:space:]]+).*/\1/p" <<<"$line"
}
# Agent keys carried by an uncommented example slice. The `\{` requirement is what excludes the
# slice's own first line (the bare `<harness>:` header, which has no flow map) without needing to
# special-case it.
ex_agent_keys(){ # $1=slice
  sed -nE 's/^[[:space:]]+([A-Za-z0-9_-]+):[[:space:]]*\{.*/\1/p' <<<"$1"
}
# HARNESS PARTITION (change 0246, finding 3). The build-max-terminated slice above is the right
# window for the FORWARD value comparison — it is anchored on a row the sidecar names — but it is
# the wrong window for the REVERSE one. build-max is the LAST row of every block, so a stale or
# typo'd agent row appended after it falls outside the slice entirely: the orphan set stays empty
# and the arity stays equal while the very row the reverse loop exists to catch sits in the file.
# The end of a block is the most natural place to append a row, so that is the likeliest orphan
# location, not an exotic one.
#
# So the reverse direction reads a partition instead: from this harness's header to the line before
# the NEXT block header, or to the end of the agents region for the last block. The region ends at
# the first truly blank line at or after `# agents:` — inside the block the separators are bare `#`
# comment lines, and the blank line before the `# runners` prose is the first real one. Boundaries
# are any bare `#   <key>:` header line, not an enumerated harness list: a fifth harness, or a
# misspelled header, partitions correctly (and any row under a misspelled header then lands in the
# preceding harness's partition and shows up as an orphan, which is the right answer).
ex_partition(){ # $1=harness — the harness's whole commented block, uncommented
  awk -v h="$1" '
    !ina { if ($0 == "# agents:") ina = 1; next }
    /^[[:space:]]*$/ { exit }
    /^#[[:space:]]+[A-Za-z0-9_-]+:[[:space:]]*$/ {
      hdr = $0
      sub(/^#[[:space:]]+/, "", hdr); sub(/:[[:space:]]*$/, "", hdr)
      on = (hdr == h)
    }
    on { print }
  ' "$EX" | sed -E 's/^[[:space:]]*#[[:space:]]?//'
}
# Population AND terminator are both derived — from HD_SHIPPED_HARNESSES and from the sidecar's own
# build-max row. A literal `claude cursor codex` list here would be a fourth restatement of what
# the shipped set already knows, and it is precisely a hand-maintained harness list that let a stale
# claim survive elsewhere in this repo. Adding a fourth shipped harness arms these loops for free.
#
# Each block's terminator is its own build-max MODEL, which is what makes the three ranges
# independent: every agent key appears in all three blocks, so a key-only anchor would resolve every
# lookup to whichever block came first in the file. build-max is the terminator because it is the
# LAST build row in ladder order (economy, standard, premium, max) and the build rows close every block — so
# this anchor moves whenever the ladder's top rung is renamed. Change 0184 moved it here from the
# ladder's previous top rung; that rename is the reason the anchor is named in prose rather than
# assumed.
ere_escape(){ sed -E 's/[][\.^$*+?(){}|]/\\&/g' <<<"$1"; }
for h in $HD_SHIPPED_HARNESSES; do
  bm_model="$(hd_field "$HD" "$h" build-max model)"
  assert "$h mirror: the sidecar supplies a build-max model to anchor the slice on" '[ -n "$bm_model" ]'
  # BOUNDARY CLASS on the model (change 0246): without it the terminator is prefix-weak. claude's
  # build-max model is `claude-opus-5` and cursor's is `claude-opus-5-high`, so with claude's own
  # build-max row deleted this range would run past the codex block and close on CURSOR's row — an
  # over-wide slice whose last line is still a `build-max:` line, so the terminator guard below
  # stayed green while the asserts read another harness's values. The example writes every flow map
  # as `{ model: X, effort: Y }`, so a real terminator's model is always followed by a comma;
  # whitespace and `}` are admitted too so a reformat does not falsely redden.
  slice="$(ex_slice "$h" "build-max:.*$(ere_escape "$bm_model")[,[:space:]}]")"
  # Terminator guard: an unclosed sed range silently runs to EOF, pulling in neighbouring blocks and
  # surrounding prose, while every assert below stays green on the over-wide slice. Pinning the
  # slice's FIRST and LAST lines catches both over-run and under-run. First/last taken by parameter
  # expansion, not `printf | head -n1`: under this file's `set -o pipefail` a producer feeding an
  # early-exiting consumer takes SIGPIPE and turns the assert into an intermittent 141.
  first="${slice%%$'\n'*}"; first="${first#"${first%%[![:space:]]*}"}"
  last="${slice##*$'\n'}"
  assert "$h mirror: the $h slice was isolated and terminates at its build-max anchor" \
    '[ -n "$slice" ] && [ "$first" = "'"$h"':" ] && grep -q "build-max:" <<<"$last"'
  mirrored=0
  while IFS= read -r a; do
    [ -n "$a" ] || continue
    mirrored=$((mirrored+1))
    assert "$h/$a: wrapper exists" '[ -f "$REPO/agents/docket-'"$a"'.md" ]'
    assert "$h/$a: model mirrors the shipped sidecar" \
      '[ -n "$(ex_slice_field "$slice" "'"$a"'" model)" ] &&
       [ "$(ex_slice_field "$slice" "'"$a"'" model)" = "$(hd_field "$HD" '"$h"' "'"$a"'" model)" ]'
    assert "$h/$a: effort mirrors the shipped sidecar" \
      '[ -n "$(ex_slice_field "$slice" "'"$a"'" effort)" ] &&
       [ "$(ex_slice_field "$slice" "'"$a"'" effort)" = "$(hd_field "$HD" '"$h"' "'"$a"'" effort)" ]'
  done < <(hd_agents "$HD" "$h")
  # REVERSE DIRECTION (change 0246). The loop above iterates the SIDECAR, so it proves only
  # sidecar ⊆ example: an agent row sitting in the example with no sidecar counterpart — a stale
  # entry left behind by a removal, or a typo'd agent name — is structurally invisible to it, and
  # to every neighbouring assert too (they are all keyed on sidecar rows). This correspondence is a
  # MIRROR, not a proper subset: the example claims to reproduce the shipped defaults value for
  # value, so the reverse loop is mandatory here. Set membership plus arity; values need no second
  # comparison, because the forward loop already compares both fields row by row.
  # Read the PARTITION, not the build-max-terminated slice: see ex_partition's note — the slice
  # cannot see a row appended after build-max, which is where an orphan is likeliest to appear.
  hd_keys="$(hd_agents "$HD" "$h")"
  part="$(ex_partition "$h")"
  part_first="${part%%$'\n'*}"; part_first="${part_first#"${part_first%%[![:space:]]*}"}"
  # Partition guard, the mirror of the slice's terminator guard: an empty or mis-anchored partition
  # would make both reverse asserts vacuous — no rows means no orphans. The partition must be
  # non-empty, must open on this harness's own header, and must be at least as wide as the
  # build-max slice (it is a superset of it by construction, so a shorter one means the region or
  # boundary detection broke).
  assert "$h mirror (reverse): the $h partition was isolated and opens on its own header" \
    '[ -n "$part" ] && [ "$part_first" = "'"$h"':" ] &&
     [ "$(grep -c . <<<"$part")" -ge "$(grep -c . <<<"$slice")" ]'
  ex_keys="$(ex_agent_keys "$part")"
  ex_orphans=""
  while IFS= read -r ek; do
    [ -n "$ek" ] || continue
    grep -qxF "$ek" <<<"$hd_keys" || ex_orphans="$ex_orphans $ek"
  done <<<"$ex_keys"
  assert "$h mirror (reverse): every example $h row exists in the shipped sidecar (${ex_orphans:-none orphaned})" \
    '[ -z "$ex_orphans" ]'
  n_ex="$(grep -c . <<<"$ex_keys")"
  n_hd="$(grep -c . <<<"$hd_keys")"
  assert "$h mirror (reverse): example row count equals sidecar row count (example $n_ex, sidecar $n_hd)" \
    '[ "$n_ex" = "$n_hd" ] && [ "$n_ex" -gt 0 ]'
  assert "$h mirror: every shipped $h entry was checked (floor 16; got $mirrored)" \
    '[ "$mirrored" -ge 16 ]'
done
# Floor on the POPULATION itself, not only on each block's row count: an emptied HD_SHIPPED_HARNESSES
# would make the whole loop above run zero times with every assert trivially satisfied.
n_shipped="$(printf '%s\n' $HD_SHIPPED_HARNESSES | grep -c .)"
assert "mirror: at least four harnesses were mirrored (got $n_shipped)" '[ "$n_shipped" -ge 4 ]'

# --- (5) RESOLVER ROUND-TRIP (retained from tests/test_config_example.sh) ----
# Uncomment the agents: block and enable cursor + codex — the example IDs must resolve through the
# REAL resolver (sync-agents.sh) into cursor and codex wrappers. Proves the commented blocks are
# valid YAML, not decorative prose.
#
# The naive "strip a leading # from every line" approach corrupts the file: dozens of unrelated
# prose paragraphs elsewhere also start with "#" + indentation and would get uncommented into
# garbage right along with the agents: block. So this ISOLATES the exact commented region first
# (unique start/end anchors, verified against this file) and transforms ONLY that slice.
#
# TERMINATOR DERIVED, NOT WRITTEN (change 0246). This anchor used to be the hand-written cursor
# finalize-change literal, which sits ABOVE cursor's own build rows and above the entire opencode
# block — so cursor's build/review rows and all sixteen opencode rows never reached the real
# resolver, while every assert below stayed green on the short slice. That is precisely what went
# stale when 0192 appended opencode, so the replacement must not be another literal.
#
# Derivation: find which shipped harness block comes LAST in the example (file order, which is NOT
# HD_SHIPPED_HARNESSES order — the sidecar lists claude cursor codex opencode, the example writes
# claude codex cursor opencode), then anchor on that block's build-max row with the model read from
# the sidecar. build-max is the ladder's top rung and closes every block.
last_ex_harness=""; _last_ex_ln=0
for _h in $HD_SHIPPED_HARNESSES; do
  # Captured, then first-line/first-field by parameter expansion — never `grep … | head -n1`,
  # which SIGPIPEs grep under pipefail (AGENTS.md § Shell).
  _hits="$(grep -nE "^#[[:space:]]*$_h:[[:space:]]*$" "$EX")"
  _ln="${_hits%%$'\n'*}"; _ln="${_ln%%:*}"
  if [ -n "$_ln" ] && [ "$_ln" -gt "$_last_ex_ln" ]; then _last_ex_ln="$_ln"; last_ex_harness="$_h"; fi
done
assert "round-trip: a last shipped harness block was located in the example (got ${last_ex_harness:-none})" \
  '[ -n "$last_ex_harness" ]'
rt_bm="$(hd_field "$HD" "$last_ex_harness" build-max model)"
assert "round-trip: the sidecar supplies a build-max model to anchor the slice on (got ${rt_bm:-none})" \
  '[ -n "$rt_bm" ]'
# Same boundary class as the mirror slice, for the same prefix-weakness reason, and the same
# address-delimiter escaping: opencode's OpenRouter IDs carry `/`, which would close sed's address
# and kill the expression with "invalid command code" — surfacing as an empty slice that reads like
# a missing block rather than a quoting bug.
rt_term="build-max:.*$(ere_escape "$rt_bm")[,[:space:]}]"
agents_block="$(sed -n "/^# agents:\$/,/${rt_term//\//\\/}/p" "$EX")"
rt_last="${agents_block##*$'\n'}"
assert "round-trip: the agents slice terminates at the last block's build-max anchor (not EOF)" \
  '[ -n "$agents_block" ] && grep -q "build-max:" <<<"$rt_last" && grep -qF "$rt_bm" <<<"$rt_last"'
# GUARD THE ORDERING ASSUMPTION rather than trusting it. The derivation above assumes the last
# shipped harness header in the example is also the last CONTENT in the agents: block. A re-ordered
# example, or a fifth harness appended after this anchor, would silently shrink coverage back to
# exactly the bug this change fixes — with every assert below still green on the short slice. So
# assert the slice reaches every shipped harness. Derived population, never a literal list.
rt_missing=""
for _h in $HD_SHIPPED_HARNESSES; do
  grep -qE "^#[[:space:]]*$_h:[[:space:]]*$" <<<"$agents_block" || rt_missing="$rt_missing $_h"
done
assert "round-trip: the slice reaches every shipped harness block (${rt_missing:-none missing})" \
  '[ -z "$rt_missing" ]'
# Since change 0169 every harness block sits at the SAME single-comment level, so one strip
# uncomments agents:, all of its shipped harness blocks, and every row of every one of them.
# (Before 0169 codex and cursor sat a level deeper and needed a second, block-scoped strip; that
# stage is gone with the asymmetry it existed for.) No count is written here on purpose: the
# previous wording said "all three harness blocks" and "all thirty-nine rows", both of which went
# stale the moment 0192 shipped a fourth block — and a restated number is the thing this suite
# exists to catch elsewhere. The population is asserted from HD_SHIPPED_HARNESSES above.
stage2="$(printf '%s\n' "$agents_block" | sed -E 's/^#[[:space:]]?//')"
# Derive the harness list from the REAL commented agent_harnesses: line (proving IT is valid too)
# rather than hand-writing an unrelated literal, then extend it to enable cursor.
harnesses_all="$(sed -n 's/^#[[:space:]]*\(agent_harnesses:.*\)/\1/p' "$EX")"
harnesses_line="${harnesses_all%%$'\n'*}"
harnesses_line="$(printf '%s' "$harnesses_line" | sed -E 's/\[claude\]/[claude, cursor, codex, opencode]/')"

SB="$(mktemp -d)"; _sbs="$SB"
mkdir -p "$SB/.claude/agents" "$SB/.cursor/agents" "$SB/.codex/agents" "$SB/.opencode/agents" "$SB/.config/docket"
{
  printf '%s\n' "$harnesses_line"
  printf '%s\n' "$stage2"
} > "$SB/.config/docket/config.yml"
err="$(cd "$SB" && HOME="$SB" XDG_CONFIG_HOME="$SB/.config" DOCKET_HARNESS_ROOT="$SB" \
       bash "$REPO/sync-agents.sh" 2>&1 >/dev/null)"; rc=$?
assert "round-trip: sync-agents resolves the uncommented example (exit 0)" '[ "$rc" = "0" ]'
assert "round-trip: no unknown-harness-token warning" \
  '! grep <<<"$err" -qiE "unknown agent_harnesses token"'
assert "round-trip: a claude wrapper was generated" '[ -f "$SB/.claude/agents/docket-status.md" ]'
assert "round-trip: claude status model mirrors the shipped sidecar" \
  '[ -n "$(hd_field "$HD" claude status model)" ] &&
   [ "$(fm "$SB/.claude/agents/docket-status.md" model)" = "$(hd_field "$HD" claude status model)" ]'
assert "round-trip: a cursor wrapper was generated" '[ -f "$SB/.cursor/agents/docket-status.md" ]'
assert "round-trip: cursor status model came from the example block" \
  '[ "$(fm "$SB/.cursor/agents/docket-status.md" model)" = "cursor-grok-4.5-low-fast" ]'
# A cursor BUILD row (change 0246). READ THE CAVEAT BEFORE TRUSTING THESE TWO ASSERTS: neither one
# can detect the truncated slice they were added for (whole-branch review, finding 6).
# sync-agents.sh generates one wrapper per shipped agent for EVERY enabled harness regardless of
# config content, resolving a missing row through hd_field, so the `-f` assert passed under the old
# terminator too; and the model assert compares wrapper-against-sidecar on both sides, which move
# together. What they DO catch is a VALUE drift between example and sidecar, and that is the whole
# of it. Kept for that; the widening's real evidence is the cursor build-max SENTINEL below, and
# the slice's reach across harnesses is proved by "round-trip: the slice reaches every shipped
# harness block".
assert "round-trip: a cursor build-max wrapper was generated" '[ -f "$SB/.cursor/agents/docket-build-max.md" ]'
assert "round-trip: cursor build-max model came from the example block" \
  '[ -n "$(hd_field "$HD" cursor build-max model)" ] &&
   [ "$(fm "$SB/.cursor/agents/docket-build-max.md" model)" = "$(hd_field "$HD" cursor build-max model)" ]'
# Codex evidence (change 0169): the example's codex rows must survive the REAL generator into real
# Codex TOML, which is what proves they are executable YAML rather than text that merely happens to
# match the sidecar reader. Read from the generated wrapper, compared against the sidecar.
#
# Naming caveat for the whole round-trip section, cursor leg included: "came from the example block"
# overstates what these asserts see. Both sides move together — with the example's rows gone the
# resolver falls back to the sidecar and they still pass. They catch a VALUE drift between example
# and sidecar, not a missing example row. Provenance is established separately, by the sentinel
# round-trip below ("sentinel: the codex wrapper carries the EXAMPLE's value, not the sidecar's").
CT="$SB/.codex/agents/docket-status.toml"
assert "round-trip: a codex wrapper was generated" '[ -f "$CT" ]'
assert "round-trip: codex status model came from the example block" \
  '[ -n "$(hd_field "$HD" codex status model)" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$CT")" = "$(hd_field "$HD" codex status model)" ]'
assert "round-trip: codex status effort came from the example block" \
  '[ "$(sed -nE "s/^model_reasoning_effort[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$CT")" = "$(hd_field "$HD" codex status effort)" ]'
# The four-rung codex ladder (change 0184), read off the generated wrappers' new filenames. Sol is
# expected at BOTH premium and max: on codex the model/effort PAIR is the role, so two rungs sharing a
# model is deliberate, not a copy-paste. Pair distinctness is asserted in tests/test_docket_build.sh;
# this leg only proves the example's ladder survives the real generator into real Codex TOML.
assert "round-trip: the codex build profiles resolve to their shipped ladder" \
  '[ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SB/.codex/agents/docket-build-economy.toml")" = "gpt-5.6-luna" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SB/.codex/agents/docket-build-standard.toml")" = "gpt-5.6-terra" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SB/.codex/agents/docket-build-premium.toml")" = "gpt-5.6-sol" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SB/.codex/agents/docket-build-max.toml")" = "gpt-5.6-sol" ]'
# opencode evidence (change 0192): the fourth harness must survive the REAL generator too, into a
# real opencode agent definition. Same "both sides move together" caveat as the codex leg above —
# this catches a VALUE drift between example and sidecar, not a missing example row. It also pins
# the one thing unique to opencode: effort lands as `reasoningEffort:`, never as a claude-shaped
# `effort:` key, which is what a fallback emitter would have written.
OCF="$SB/.opencode/agents/docket-build-economy.md"
assert "round-trip: the example resolves into an opencode definition" '[ -f "$OCF" ]'
assert "round-trip: opencode definition carries the shipped build-economy model" \
  '[ -n "$(hd_field "$HD" opencode build-economy model)" ] &&
   grep -qx "model: $(hd_field "$HD" opencode build-economy model)" "$OCF"'
assert "round-trip: opencode definition carries the effort as reasoningEffort" \
  '[ -n "$(hd_field "$HD" opencode build-economy effort)" ] &&
   grep -qx "reasoningEffort: $(hd_field "$HD" opencode build-economy effort)" "$OCF"'
rm -rf "$_sbs"

# --- SENTINEL round-trip: prove the EXAMPLE's rows are what the generator consumed --------------
# Every assert in the round-trip above compares the generated wrapper against the SIDECAR, and the
# example mirrors the sidecar value for value — so both sides of each comparison move together and
# the asserts cannot detect the example's rows going missing. Proved: delete all thirteen codex rows
# from .docket.example.yml and "round-trip: codex status model came from the example block" still
# passes, because the resolver simply falls back to the sidecar. Those asserts state provenance
# they cannot see; they are kept above (they DO catch the example drifting to a different value),
# and this block supplies the provenance half they are missing.
#
# Method: rewrite ONE model value in the uncommented slice to a sentinel that exists in neither the
# sidecar nor any other block, then assert the sentinel reaches the generated wrapper. Only the
# example's own row can put it there. This is spec Tier-1 property 9's second clause.
#
# The ROW is a parameter, not hardcoded to `status:` (change 0246 whole-branch review, finding 6).
# The cursor `build-max` probe below is what puts real provenance evidence in the region the
# WIDENED slice newly reaches: before the terminator was re-derived, the slice ended above cursor's
# build rows, so a build-max sentinel could not have survived into a wrapper at all. A plain
# `[ -f docket-build-max.md ]` assert cannot show this — sync-agents.sh generates one wrapper per
# shipped agent for every enabled harness regardless of config content, falling back to the sidecar
# through hd_field — which is exactly why the evidence lives here instead.
probe_slice(){ # $1 = harness key, $2 = old model literal, $3 = sentinel, $4 = row key (default status)
  awk -v h="$1" -v old="$2" -v new="$3" -v row="^    ${4:-status}:" '
    /^  [A-Za-z0-9_-]+:[[:space:]]*$/ { cur=$1; sub(/:$/,"",cur) }
    cur==h && $0 ~ row { sub(old, new, $0) }
    { print }'
}
stage2_probe="$(printf '%s\n' "$stage2" \
  | probe_slice codex  'gpt-5\.6-luna'            'gpt-5.6-probe' \
  | probe_slice cursor 'cursor-grok-4\.5-low-fast' 'cursor-grok-4.5-probe' \
  | probe_slice cursor 'claude-opus-5-high'        'cursor-opus-5-bm-probe' build-max)"
# Fixture sanity FIRST: a substitution that silently missed would leave every assert below vacuous
# (the example would carry the shipped value, the wrapper would too, and nothing would notice).
# Exactly one occurrence each, and the sentinels must be absent from the shipped sidecar so a hit
# in the wrapper can only have come from the example.
assert "sentinel: exactly one codex model was rewritten in the example slice" \
  '[ "$(grep -cF "gpt-5.6-probe" <<<"$stage2_probe")" = "1" ]'
assert "sentinel: exactly one cursor model was rewritten in the example slice" \
  '[ "$(grep -cF "cursor-grok-4.5-probe" <<<"$stage2_probe")" = "1" ]'
assert "sentinel: exactly one cursor build-max model was rewritten in the example slice" \
  '[ "$(grep -cF "cursor-opus-5-bm-probe" <<<"$stage2_probe")" = "1" ]'
assert "sentinel: no sentinel exists in the shipped sidecar" \
  '! grep -qF "gpt-5.6-probe" "$HD" && ! grep -qF "cursor-grok-4.5-probe" "$HD" &&
   ! grep -qF "cursor-opus-5-bm-probe" "$HD"'

SBP="$(mktemp -d)"; _sbps="$SBP"
mkdir -p "$SBP/.claude/agents" "$SBP/.cursor/agents" "$SBP/.codex/agents" "$SBP/.config/docket"
{
  printf '%s\n' "$harnesses_line"
  printf '%s\n' "$stage2_probe"
} > "$SBP/.config/docket/config.yml"
( cd "$SBP" && HOME="$SBP" XDG_CONFIG_HOME="$SBP/.config" DOCKET_HARNESS_ROOT="$SBP" \
  bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ); prc=$?
assert "sentinel: sync-agents resolves the probed example (exit 0)" '[ "$prc" = "0" ]'
assert "sentinel: the codex wrapper carries the EXAMPLE's value, not the sidecar's" \
  '[ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SBP/.codex/agents/docket-status.toml")" = "gpt-5.6-probe" ]'
assert "sentinel: the cursor wrapper carries the EXAMPLE's value, not the sidecar's" \
  '[ "$(fm "$SBP/.cursor/agents/docket-status.md" model)" = "cursor-grok-4.5-probe" ]'
# The WIDENED slice's own evidence: build-max is the LAST row of the cursor block, and before the
# terminator was re-derived the slice stopped above cursor's build rows entirely. A sentinel that
# reaches this wrapper can only have come from the example's own build-max row — the sidecar does
# not carry it. This is the assert that actually proves the widening; the `-f` and value asserts in
# the round-trip above pass either way.
assert "sentinel: the cursor build-max wrapper carries the EXAMPLE's value — the widened slice reaches the block's last row" \
  '[ "$(fm "$SBP/.cursor/agents/docket-build-max.md" model)" = "cursor-opus-5-bm-probe" ]'
# Unprobed rows still resolve, so the probe did not corrupt the slice into a one-row config.
assert "sentinel: an unprobed codex row still resolves from the example" \
  '[ -n "$(hd_field "$HD" codex build-max model)" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SBP/.codex/agents/docket-build-max.toml")" = "$(hd_field "$HD" codex build-max model)" ]'
rm -rf "$_sbps"

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
   printf "%s\n" "$active_scaffold" | sed -n "1p" | grep >/dev/null -E "^runtime:[[:space:]]*$" &&
   printf "%s\n" "$active_scaffold" | sed -n "2p" | grep >/dev/null -E "^[[:space:]]+bash:[[:space:]]+'\''/[^'\'']+'\''[[:space:]]*$"'
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
  'grep -Eq "^metadata_branch:[[:space:]]*docket" "$DY" && grep -Eq "^terminal_publish:[[:space:]]*false" "$DY"'

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
sn_count="$(grep <<<"$sn_flat" -c .)"
assert "(8) snippet flattened key count is exactly 5 (floor against extraction going silently empty, ceiling against undocumented growth; if intentional, bump this literal 5 and add the key to .docket.example.yml; got $sn_count)" \
  '[ "$sn_count" = "5" ]'
ex_count="$(grep <<<"$ex_flat" -c .)"
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
  ex_hit="$(awk -F'\t' -v p="$sn_path" '$1==p{print "1"; exit}' <<<"$ex_flat")"
  if [ -z "$ex_hit" ]; then
    sn_missing="$sn_missing $sn_path"
    continue
  fi
  ex_val="$(awk -F'\t' -v p="$sn_path" '$1==p{print $2; exit}' <<<"$ex_flat")"
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
sn_ptr_all="$(snippet_section | sed -nE 's/.*\[[^]]*\]\(([^)]*\.docket\.example\.yml)\).*/\1/p')"
sn_ptr="${sn_ptr_all%%$'\n'*}"
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
hyph_paths="$(grep <<<"$hyph_out" -c .)"
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
# 13 -> 19 (change 0276): README's dummy_mode section adds six config fences — the key's own
# example block plus the five-entry persona gallery, each of which is a real `dummy_mode:` block
# whose keys (9) validates against .docket.example.yml like every other fence.
assert "(9) README yaml fence count is exactly 19 — floor against discovery going silently empty, ceiling against an unguarded new fence; if you ADDED a config fence, bump this literal AND ensure its keys are in .docket.example.yml in the same commit; if this dropped to 18, check that the fence regex is still whitespace-tolerant (fence 576 is indented) before touching the literal (got $fence_count)" \
  '[ "$fence_count" = "19" ]'

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
    flat="$(grep <<<"$flatout" -c .)"
    if [ "$flat" -eq 0 ]; then echo "empty $line"; continue; fi
    raw="$(grep <<<"$body" -vE '^[[:space:]]*$' | grep -vcE '^[[:space:]]*#')"
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
f9_miss="$(grep <<<"$findings9" '^miss ' | sed 's/^miss //' | tr '\n' ' ' | sed 's/ *$//')"
assert "(9) every README config-fence key exists in .docket.example.yml (fence-line + key path shown; ${f9_miss:-none missing})" \
  '[ -z "$f9_miss" ]'

# NON-VACUITY FLOOR 2 — a fence that flattens to ZERO paths contributes nothing to the existence
# loop above, so it would be silently unguarded rather than reported.
f9_empty="$(grep <<<"$findings9" '^empty ' | sed 's/^empty //' | tr '\n' ' ' | sed 's/ *$//')"
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
f9_drop="$(grep <<<"$findings9" '^drop ' | sed 's/^drop //' | tr '\n' ' ' | sed 's/ *$//')"
assert "(9) the flattener drops no key-shaped line in any fence (raw content lines vs flattened, per fence; ${f9_drop:-none dropped})" \
  '[ -z "$f9_drop" ]'

f9_marker="$(grep <<<"$findings9" '^marker ' | sed 's/^marker //' | tr '\n' ' ' | sed 's/ *$//')"
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
f9_seen="$(grep <<<"$findings9" -c '^seen ')"
assert "(9) scan_fences visited all 19 fences — same literal as NON-VACUITY FLOOR 1's fence-count assert above; if you added a fence, see that assert's message for the remedy (got $f9_seen)" '[ "$f9_seen" = "19" ]'
f9_vmarked="$(grep <<<"$findings9" -c '^seen .* values$')"
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
fx9e_value="$(grep <<<"$fx9e_findings" '^value .*reclaim\.lease_ttl ')"
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
f9_marked="$(grep <<<"$findings9" '^seen ' | grep -vc ' none$')"
assert "(9) every docket:config-fence line in the README is attached to a fence (file has $md_markers, fences consumed $f9_marked)" \
  '[ "$md_markers" = "$f9_marked" ]'

# VALUE EQUALITY IS OPT-IN, and it is not lost where it is SOUND. Section (8) keeps it on fence
# 209 (the per-repo snippet, which documents shipped defaults); this marker adds it to the
# reclaim: fence, whose lease_ttl: 72 / auto: false are also shipped defaults and SHOULD redden if
# the defaults move. The other seven fences stay existence-only, because they deliberately
# illustrate non-default values. Fence 209 is therefore double-covered by (8) (existence + values)
# and (9) (existence only); that overlap is accepted rather than special-cased — (8)'s fence is
# simply left unmarked, and since no unmarked fence gets a value assert, no special-casing exists.
f9_value="$(grep <<<"$findings9" '^value ' | sed 's/^value //' | tr '\n' ' ' | sed 's/ *$//')"
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
fx9_non_seen="$(grep <<<"$fx9_findings" -v '^seen ')"
assert "(9) an ignore-marked fence is skipped entirely — its non-schema key raises nothing (got [${fx9_non_seen}])" \
  '[ -z "$fx9_non_seen" ]'
fx9_seen_ignore="$(grep <<<"$fx9_findings" '^seen .* ignore$')"
assert "(9) the ignore fixture's fence is recorded as ignore-marked in its seen record (got [${fx9_seen_ignore}])" \
  '[ -n "$fx9_seen_ignore" ]'

# ...and the SAME fixture without the marker must report the key, so the assert above is proven to
# be the marker working rather than the fixture being invisible to the scanner.
fx9b="$tmp/fence-fixture-unmarked.md"
printf '# Fixture\n\n```yaml\nnot_a_docket_key: true\n```\n' > "$fx9b"
fx9b_findings="$(scan_fences "$fx9b")"
fx9b_non_seen="$(grep <<<"$fx9b_findings" -v '^seen ')"
assert "(9) the same fence WITHOUT the ignore marker does report its key — proves the skip is the marker, not an invisible fixture (got [${fx9b_non_seen}])" \
  '[ "$fx9b_non_seen" = "miss 3 not_a_docket_key" ]'

# NON-VACUITY FLOOR 2's "empty" branch and the marker grammar's BAD path are both demonstrably
# reachable but shipped with no fixture, unlike the ignore branch above. A comment-only fence body
# exercises the former; a malformed (but attached) token exercises the latter.
fx9c="$tmp/fence-fixture-empty.md"
printf '# Fixture\n\n```yaml\n# just a comment, no keys here\n```\n' > "$fx9c"
fx9c_findings="$(scan_fences "$fx9c")"
fx9c_empty="$(grep <<<"$fx9c_findings" '^empty ')"
assert "(9) a comment-only fence body trips NON-VACUITY FLOOR 2 with the exact empty finding (got [${fx9c_empty}])" \
  '[ "$fx9c_empty" = "empty 3" ]'

fx9d="$tmp/fence-fixture-badmarker.md"
printf '# Fixture\n\n<!-- docket:config-fence: bogus -->\n```yaml\nsome_key: true\n```\n' > "$fx9d"
fx9d_findings="$(scan_fences "$fx9d")"
fx9d_marker="$(grep <<<"$fx9d_findings" '^marker ')"
assert "(9) an unknown docket:config-fence token hard-fails via the marker branch with the exact malformed-marker finding (got [${fx9d_marker}])" \
  '[ "$fx9d_marker" = "marker 4 malformed-marker" ]'

# --- (10) SHELL-SHAPE SELF-GUARD: no producer piped into an early-exiting consumer -------------
# WHY THIS FILE GUARDS ITSELF. AGENTS.md § Shell has forbidden `producer | early-exiting-consumer`
# under `set -o pipefail` for a long time, and this suite hand-honours it in prose at a dozen
# sites — yet the rule was still broken HERE, in the single most hazardous spot in the repo: the
# fidelity fixture's `git show` of the ~40KB example piped into `grep -q`, which turned into an
# intermittent 141 under the parallel runner and reddened the whole build. A rule that lives only
# in prose is enforced by whoever remembers it; this pass enforces it mechanically, on the one
# file that owns the repo's largest producer.
#
# ORIGINALLY SCOPED TO THIS FILE; NOW SUBSUMED. This pass shipped file-scoped because the same
# shape stood at ~84 further sites across tests/ and sweeping them was its own piece of work.
# The very next gate run failed on one of those deferred sites, so the sweep landed in the same
# repair after all, and tests/test_pipe_shapes.sh now guards every tracked shell file with a
# STRICTER predicate: no producer exemption at all, and `q` caught anywhere in the flag bundle
# (this pass requires `q` to close the bundle, so `-qF`/`-Eqi` spellings pass it unseen). This
# self-scan stays as this file's own belt-and-braces; the repo-wide guard is the enforcement.
#
# WHAT IS EXEMPT, AND WHY THE EXEMPTION DID NOT SURVIVE REPO-WIDE: the consumer half is keyed
# on shape (any grep whose flag bundle ENDS in q, or head), never on an enumerated list of
# invocations. The producer half exempts one class — `printf`/`echo` of an already-materialized
# shell variable — on the theory that the payload is bounded by what the test built. The second
# change-0276 gate failure disproved that theory as a safety claim: a materialized payload larger
# than the loaded pipe capacity (`printf "%s\n" "$(cat …)" | grep -qF …` over a docket-status
# transcript) SIGPIPEs exactly like a streamed one. The exemption is tolerable HERE only because
# the sweep removed every such site and tests/test_pipe_shapes.sh reddens on any new one.
#
# KNOWN IMPRECISION, stated rather than hidden: the producer word is read from the first pipeline
# stage ON THE SAME LINE, so a pipeline whose producer sits on a previous continuation line
# reports an empty command name rather than the real one. It still REPORTS — the line number is
# in the finding — so the effect is a less-informative true positive, never a miss.
sigpipe_guard_awk="$(cat <<'SIGPIPE_GUARD_AWK'
{
  if ($0 ~ /^[[:space:]]*#/) next
  if ($0 !~ /\|[[:space:]]*(grep([[:space:]]+-[A-Za-z]+)*[[:space:]]+-[A-Za-z]*q([[:space:]]|$)|head([[:space:]]|$))/) next
  i = index($0, "|"); first = substr($0, 1, i - 1)
  do {
    prev = first
    sub(/^[[:space:]]+/, "", first)
    sub(/^assert[[:space:]]+"[^"]*"[[:space:]]*\\?[[:space:]]*/, "", first)
    sub(/^["\047]/, "", first)
    sub(/^(!|\(|\{|&&)/, "", first)
    sub(/^[A-Za-z_][A-Za-z0-9_]*=\$\(/, "", first)
    sub(/^[A-Za-z_][A-Za-z0-9_]*\(\)[[:space:]]*\{?/, "", first)
    sub(/^\$\(/, "", first)
  } while (first != prev)
  split(first, w, /[[:space:]]/); cmd = w[1]; sub(/^.*\//, "", cmd)
  if (cmd == "printf" || cmd == "echo") next
  print FNR ": " cmd
}
SIGPIPE_GUARD_AWK
)"
sigpipe_hits="$(awk "$sigpipe_guard_awk" "${BASH_SOURCE[0]}")"
assert "(10) no producer is piped into an early-exiting consumer in this file (AGENTS.md § Shell)" \
  '[ -z "$sigpipe_hits" ]'
if [ -n "$sigpipe_hits" ]; then
  echo "--- pipelines whose producer can take SIGPIPE (line: producer) ---"
  printf '%s\n' "$sigpipe_hits"
  echo "--- capture the producer into a variable, then search it with a here-string ---"
fi

# GUARD-THE-GUARD. The assert above is green both when the file is clean AND when the pass matches
# nothing at all, so prove the pass goes RED on the exact shape it exists to catch and STAYS green
# on the exempt one. Both fixtures run $sigpipe_guard_awk — literally the program that ships.
#
# The offending line is ASSEMBLED from $_bar rather than written literally, because this file is
# its own haystack: a verbatim copy of the defective pipeline sitting in a fixture heredoc here
# would be found by the whole-file pass above and redden the very assert it is meant to support.
_bar='|'
printf '%s\n' "assert \"x\" 'git -C \"\$d\" show origin/main:.docket.yml 2>/dev/null $_bar grep -q \"^k:\"'" \
  > "$tmp/sigpipe-bad.sh"
sigpipe_bad="$(awk "$sigpipe_guard_awk" "$tmp/sigpipe-bad.sh")"
assert "(10) guard-the-guard: the exact defective shape this repair removed is REPORTED (got '${sigpipe_bad}')" \
  '[ "$sigpipe_bad" = "1: git" ]'

printf '%s\n' "assert \"x\" 'printf \"%s\" \"\$out\" $_bar grep -q \"^k:\"'" > "$tmp/sigpipe-ok.sh"
sigpipe_ok="$(awk "$sigpipe_guard_awk" "$tmp/sigpipe-ok.sh")"
assert "(10) guard-the-guard: the exempt printf-of-a-variable idiom is NOT reported (got '${sigpipe_ok}')" \
  '[ -z "$sigpipe_ok" ]'

exit $fail
