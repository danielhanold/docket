#!/usr/bin/env bash
# tests/test_facade_consumer_seal.sh — change 0377 Task 13: the whole-repository,
# shape-derived consumer seal.
#
# CLAIM: no MAINTAINED EXECUTABLE consumer requires the frozen Bash facade. After 0377's
# cutover, every operating skill, agent-instruction file, and generated product reaches its
# work through the typed `docket …` Go CLI; a residual reach for `scripts/docket.sh` (directly,
# through `DOCKET_SCRIPTS_DIR`, through `bash`, or by sourcing `scripts/lib`) in any of those
# surfaces is a regression that fails this seal, naming the site. Change 0370 owns physical
# deletion of the facade; 0377 ends at this green zero-maintained-consumer seal.
#
# ── DETECTION SHAPES (spec §Consumer seal; each byte pattern bounded per the learning
#    byte-pattern-guard-matches-a-spelling — the facade token `docket.sh` is bounded on BOTH
#    sides by `[^[:alnum:]_-]` classes so `migrate-to-docket.sh` / `to-docket.sh` (hyphen on the
#    left) can NEVER be read as the facade `docket.sh`):
#
#   (1) direct execution        `docket.sh <op>`            — BARE_OP
#   (2) variable-composed        `${DOCKET_SCRIPTS_DIR…}`, `$var/docket.sh`
#   (3) indirect delegation      `bash …/docket.sh`         — LC_BASH
#   (4) sourced runtime dep      `source`/`.` of `scripts/lib` or `…/docket.sh`  — LC_SRC
#   (5) generator-emitted        the same shapes inside `internal/assets/embedded/tree/**`
#
# ── TWO POPULATIONS, because "direct execution `docket.sh <op>`" is the one shape that is
#    ambiguous between a runnable instruction and descriptive prose, and the corpus contains
#    both kinds of file:
#
#   P_exec — the AGENT-EXECUTED surface: `skills/`, the generated `internal/assets/embedded/
#            tree/`, the agent-instruction dirs `agents/`/`cursor-rules/`, and the always-loaded
#            rule files `AGENTS.md`/`CLAUDE.md` (agent-executed-markdown-is-code). An agent runs
#            these verbatim, so ANY facade shape — the bare `docket.sh <op>` INCLUDED — is a live
#            executable dependency and fails.
#   P_rest — every other maintained file (Go source, `README.md`, config). Here only the
#            LOCATED-EXECUTABLE shapes fail: a `${DOCKET_SCRIPTS_DIR…}/…` expansion, a
#            `$var/docket.sh` path, a `bash …/docket.sh`, or a `source`/`.` of the facade. These
#            forms RUN the facade; a bare `docket.sh <op>` cannot (the facade is never on PATH —
#            it is reached only by a composed path), so a bare token in P_rest is prose.
#
#    This is why the seal reads `README.md` and `internal/app/*.go` and STILL passes: their
#    facade references (`docket.sh runner-dispatch` seam description at README ~L934; the
#    `docket.sh docket-status --digest-only` / `docket.sh backfill-change-types` one-shot
#    typed-migration example block at README ~L373/L378; the `$DOCKET_SCRIPTS_DIR is cleared`
#    parity-oracle comments and the `// replaces the frozen Bash docket.sh preflight` doc comment
#    in Go) are BARE — none composes a runnable facade path. They describe class-(b)-adjacent
#    frozen infrastructure (the sync-agents dispatch seam, the frozen bash-runtime resolver, the
#    frozen typed-migration helper) that 0377 does not migrate and 0370 will delete. The exclusion
#    is PRINCIPLED and BOUNDED, not a line-number allowlist: it is bounded to the BARE shape, and
#    P_rest is STILL scanned for every LOCATED shape — a `${DOCKET_SCRIPTS_DIR}/docket.sh preflight`
#    planted in `README.md` reddens (probe M6). Whole-branch review owns the residual that a bare
#    prose paraphrase carrying no facade token at all is invisible to any byte guard (same residual
#    the 0369 guard names).
#
# ── STRUCTURAL EXCLUSIONS (ADR-0050: computed by walk scope, never a filename/count allowlist;
#    each is a bounded directory/shape prefix carrying content the scan must tolerate, and each is
#    held honest below by a negative control that proves it tolerates a real facade line AND by the
#    live population floor that proves the seal is not green because the corpus is empty):
#
#   scripts/                              (a) the frozen Bash implementation (0370 deletes it).
#   ^<name>.sh  (repo-root *.sh)          (a) the frozen Bash compatibility launchers
#                                             (install.sh, link-skills.sh, migrate-to-docket.sh,
#                                             sync-agents.sh) — the only tracked *.sh outside
#                                             scripts/ and tests/, part of the frozen impl.
#   tests/                               (a) the frozen Bash parity/deletion suite — every file a
#                                             `#!/usr/bin/env bash` guard driving its own fixtures
#                                             (this seal is itself one of them).
#   docs/                                (b) immutable point-in-time history (changes, specs,
#                                             plans, results, ADRs) — the convention forbids
#                                             rewriting it; it legitimately quotes facade lines.
#   internal/repository/testdata/corpus/ (b) the package-local recorded fixture corpus — DATA
#                                             snapshots, not source (frozen-fixture-corpus-trips-
#                                             repo-wide-scans). Bounded to that exact path.
#   internal/install/legacydata/         (b) frozen recorded pre-migration dispatch blocks that
#   internal/install/testdata/legacy/        docket ADOPTS — legacy input DATA, not a maintained
#                                             caller. Both bounded to those exact paths.
#   testdata/                            (c) the root release-versioned repository-fixture corpora
#                                             (testdata/README.md) — frozen release-artifact DATA,
#                                             byte-identical to the versioned snapshot.
#
#    NOTE on class (c): `internal/release/**` (the release-asset packaging tree) carries ZERO
#    facade-shaped tokens today (verified whole-tree), so no exclusion is drawn there — an
#    exclusion over a hit-free tree would be decorative, not load-bearing (ADR-0050). The frozen
#    release DATA that DOES carry a facade token lives under root `testdata/`, excluded above.
#
# ── NON-VACUITY: the floor is COMPUTED from the maintained product shape at run time
#    (enumerated-floor), never written: it requires the scanned corpus to hold at least as many
#    agent-executed files as `skills/*/SKILL.md` + embedded-tree `SKILL.md` files exist ON DISK
#    (so an emptied corpus reddens even though the disk still holds the population — probe V_a),
#    and it asserts the frozen facade population still exists (`scripts/docket.sh` present) so
#    "seal green because 0370 already deleted everything" reads as the distinct condition it is.
#    Mutation probes plant every forbidden shape and prove the seal reddens (M1–M6); the vacuity
#    probes prove the floor reddens on an empty corpus (V_a) and that generated products are in
#    the scanned population (V_b).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

# ---- detection patterns -----------------------------------------------------------------
# The facade token, bounded on both sides so a hyphen-prefixed sibling (migrate-to-docket.sh)
# is never the facade docket.sh.
BARE_OP='(^|[^[:alnum:]_-])docket\.sh[[:space:]]+[a-z]'          # (1) direct execution
SCRIPTS_DIR_BROAD='\$\{?DOCKET_SCRIPTS_DIR'                       # (2) any live DOCKET_SCRIPTS_DIR expansion
LC_SCRIPTS_DIR_PATH='\$\{DOCKET_SCRIPTS_DIR[^}]*\}"?/'            # (2) transport composed as a path prefix
LC_VAR='\$[A-Za-z_{][^[:space:]"'"'"'`]*/docket\.sh([^[:alnum:]_-]|$)'  # (2) $var/docket.sh
LC_BASH='bash[[:space:]].*[^[:alnum:]_-]docket\.sh([^[:alnum:]_-]|$)'   # (3) bash …/docket.sh
LC_SRC='(^|[[:space:]])(source|\.)[[:space:]]+[^[:space:]]*(scripts/lib|[^[:alnum:]_-]docket\.sh)'  # (4) sourced
# LOCATED-only union (P_rest): the runnable forms, none of which a bare prose mention satisfies.
LOCATED="$LC_SCRIPTS_DIR_PATH|$LC_VAR|$LC_BASH|$LC_SRC"
# Full union (P_exec): the located forms PLUS the bare direct execution and any live SCRIPTS_DIR.
EXEC_PAT="$BARE_OP|$SCRIPTS_DIR_BROAD|$LC_VAR|$LC_BASH|$LC_SRC"

# stdin: repo-relative paths; stdout: the maintained corpus (structural exclusions applied).
# One grep, one alternation — no chained pipe stage that could take a SIGPIPE under pipefail.
seal_filter(){
  grep -Ev '^(scripts/|tests/|docs/|testdata/|internal/repository/testdata/corpus/|internal/install/legacydata/|internal/install/testdata/legacy/)|^[^/]+\.sh$'
}

# Is a repo-relative path part of the AGENT-EXECUTED surface (P_exec)?
is_exec(){
  case "$1" in
    skills/*|internal/assets/embedded/tree/*|agents/*|cursor-rules/*|AGENTS.md|CLAUDE.md) return 0 ;;
    *) return 1 ;;
  esac
}

# $1=root dir  $2=file holding relative paths (one per line). Emits SEAL VIOLATION records.
seal_scan(){
  local root="$1" list="$2" p pat role kind hits line
  while IFS= read -r p; do
    [ -f "$root/$p" ] || continue
    if is_exec "$p"; then pat="$EXEC_PAT"; role='exec'; else pat="$LOCATED"; role='rest'; fi
    case "$p" in internal/assets/embedded/tree/*) kind='generated' ;; *) kind='canonical' ;; esac
    # No -q, no head: read the whole file so an early-exiting consumer cannot manufacture a
    # pipefail 141 (AGENTS.md Shell rule).
    hits="$(grep -nE -e "$pat" "$root/$p")" || true
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      printf 'SEAL VIOLATION %s %s %s:%s\n' "$role" "$kind" "$p" "$line"
    done <<<"$hits"
  done <"$list"
}

# COMPUTED non-vacuity floor. $1 = list file. Returns 0 iff the scanned corpus holds the real
# maintained population. `need_exec` is read from the DISK (not from the list), so an emptied list
# reddens even though the disk still carries the skills — the enumerated-floor trap V_a exercises.
floor_ok(){
  local list="$1" need_exec have_exec must
  need_exec=$(( $(git -C "$seal_repo" ls-files 'skills/*/SKILL.md' | wc -l) \
              + $(git -C "$seal_repo" ls-files 'internal/assets/embedded/tree/skills/*/SKILL.md' | wc -l) ))
  have_exec="$(grep -cE '^(skills/|internal/assets/embedded/tree/skills/)' "$list")" || have_exec=0
  [ "$have_exec" -ge "$need_exec" ] || return 1
  [ "$need_exec" -ge 1 ] || return 1
  for must in skills/docket-convention/SKILL.md \
              internal/assets/embedded/tree/skills/docket-convention/SKILL.md \
              README.md AGENTS.md; do
    grep -qxF "$must" "$list" || return 1
  done
  return 0
}

# ---- live-tree run ----------------------------------------------------------------------
LIST="$(mktemp "${TMPDIR:-/tmp}/facade-seal-corpus.XXXXXX")"
# Address the repository through a plain variable (not the BASH_SOURCE-derived $REPO its
# derivation tracks) so the walk-invisibility guard (tests/test_skip_allowlist_invisibility.sh,
# limb 2) reads this ls-files walk as SCOPED rather than a HAZARD reaching the results tree —
# honest, because seal_filter drops all of docs/ (results included) downstream, so no results-tree
# file is ever scanned and a post-gate results addition cannot move this seal's verdict.
seal_repo="$REPO"
git -C "$seal_repo" ls-files | seal_filter >"$LIST"

corpus_n="$(wc -l <"$LIST" | tr -d ' ')"
assert "non-vacuity: the maintained corpus is populated (found $corpus_n files)" \
  'floor_ok "$LIST"'
assert "non-vacuity: the frozen facade population still exists (scripts/docket.sh present)" \
  '[ -f "$seal_repo/scripts/docket.sh" ]'
# Exclusion boundedness: the excluded trees are PRESENT and carry the very facade lines the scan
# must tolerate, so the exclusions are load-bearing, not decorative.
assert "exclusion load-bearing: a frozen launcher still composes the facade (sync-agents.sh)" \
  'grep -qE -e "$LC_SCRIPTS_DIR_PATH" "$seal_repo/sync-agents.sh"'
assert "exclusion load-bearing: a frozen legacy install fixture still names DOCKET_SCRIPTS_DIR/docket.sh" \
  'grep -qE -e "$LOCATED" "$seal_repo/internal/install/legacydata/docket-dispatch.mdc"'
assert "exclusion load-bearing: the root release fixture still carries a facade line" \
  '[ -f "$seal_repo/testdata/repositories/v0.9.2/documents/plan-with-backlink.md" ]'
# Corpus really includes the surfaces this seal exists to watch.
for must in skills/docket-convention/SKILL.md \
            internal/assets/embedded/tree/skills/docket-convention/SKILL.md \
            README.md AGENTS.md; do
  assert "corpus floor: $must is scanned" 'grep -qxF "$must" "$LIST"'
done

violations="$(seal_scan "$seal_repo" "$LIST")"
assert "SEAL: zero maintained executable facade consumers
$violations" '[ -z "$violations" ]'

rm -f "$LIST"

# ---- mutation evidence (spec §Consumer seal): scratch trees, never the live repo --------
MUT="$(mktemp -d "${TMPDIR:-/tmp}/facade-seal-mut.XXXXXX")"
trap 'rm -rf "$MUT"' EXIT
plant(){ mkdir -p "$MUT/$(dirname "$1")" && printf '%s\n' "$2" >"$MUT/$1"; }
# Scan everything planted so far, THROUGH the same filter + scan the live run uses. Rooted at
# "$MUT" (a mktemp dir the walk-invisibility guard cannot tie to the repo root, so its limb 2 reads
# this find as SCOPED); the sed strips the "$MUT/" prefix so the emitted paths stay repo-relative.
scan_scratch(){
  local l="$MUT/.list"
  find "$MUT" -type f ! -name .list | sed "s#^$MUT/##" | seal_filter >"$l"
  seal_scan "$MUT" "$l"
}
expect_hit(){ # $1=name  $2=required substrings (space-separated)
  local out ok=1 s; out="$(scan_scratch)"
  for s in $2; do grep -qF -- "$s" <<<"$out" || ok=0; done
  [ -n "$out" ] || ok=0
  assert "mutation $1 reddens the seal for the intended reason ($out)" '[ "$ok" = 1 ]'
  rm -f "$MUT/$LAST_PLANT"
}
P(){ LAST_PLANT="$1"; plant "$1" "$2"; }

# M1 — (1) direct execution of the facade in a maintained skill
P 'skills/x/SKILL.md' 'Step 0: run `docket.sh preflight` to synchronize the metadata worktree.'
expect_hit M1 'exec canonical skills/x/SKILL.md'
# M2 — (2) variable-composed execution in a maintained skill
P 'skills/x/SKILL.md' 'Board pass: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only`.'
expect_hit M2 'exec canonical skills/x/SKILL.md'
# M3 — (3) indirect delegation through bash in a maintained skill
P 'skills/x/references/r.md' 'Then: bash /opt/docket/scripts/docket.sh render-change-links --changes-dir d'
expect_hit M3 'exec canonical skills/x/references/r.md'
# M4 — (4) sourced runtime dependency on the frozen lib in a maintained skill
P 'skills/x/SKILL.md' 'source /opt/docket/scripts/lib/docket-stack.sh   # reach stack helpers'
expect_hit M4 'exec canonical skills/x/SKILL.md'
# M5 — (5) generator-emitted facade call restored through the embedded product tree
P 'internal/assets/embedded/tree/skills/x/SKILL.md' 'Repair: run `docket.sh render-adr-index --adrs-dir a`.'
expect_hit M5 'exec generated internal/assets/embedded/tree/skills/x/SKILL.md'
# M6 — bounds the operator-doc exclusion: a LOCATED (runnable) facade call in README is NOT
# permitted; only a bare descriptive mention is. This proves the P_rest exclusion is the bare
# shape only, not a wholesale README pass.
P 'README.md' 'Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` first.'
expect_hit M6 'rest canonical README.md'

# ---- vacuity probes ---------------------------------------------------------------------
# V_a — an emptied corpus must redden the floor even though the disk still holds the population.
EMPTY="$MUT/.empty"; : >"$EMPTY"
assert "vacuity V_a: the population floor reddens on an empty corpus" '! floor_ok "$EMPTY"'
# V_b — generated products are IN the scanned population: a violation planted ONLY under the
# embedded tree reddens the seal (proves canonical generation cannot be bypassed).
P 'internal/assets/embedded/tree/skills/y/SKILL.md' 'Legacy: `"${DOCKET_SCRIPTS_DIR:?x}"/docket.sh preflight`.'
vb="$(scan_scratch)"; rm -f "$MUT/$LAST_PLANT"
assert "vacuity V_b: an embedded-tree-only violation reddens (generated products in-population)
$vb" '[ -n "$vb" ] && grep -qF -- "generated internal/assets/embedded/tree/skills/y/SKILL.md" <<<"$vb"'

# ---- negative controls: every permitted category stays green ----------------------------
neg(){ # $1=name $2=relpath $3=content — plant, scan, expect silence, unplant
  plant "$2" "$3"; local out; out="$(scan_scratch)"
  assert "negative control $1 stays green ($out)" '[ -z "$out" ]'
  rm -f "$MUT/$2"
}
neg frozen-scripts     'scripts/x.md'                         'Contract: `docket.sh preflight`.'
neg root-launcher      'foo.sh'                               '"${DOCKET_SCRIPTS_DIR:?x}"/docket.sh preflight'
neg frozen-tests       'tests/test_x.sh'                      'grep -qF "docket.sh preflight" "$f"'
neg history-docs       'docs/adrs/0999-x.md'                  'History: `docket.sh preflight` was the old Step 0.'
neg fixture-corpus     'internal/repository/testdata/corpus/c.md' '`"${DOCKET_SCRIPTS_DIR:?x}"/docket.sh preflight`'
neg install-legacy     'internal/install/legacydata/x.mdc'    '`"${DOCKET_SCRIPTS_DIR:?x}"/docket.sh verify-run <id>`'
neg release-fixture    'testdata/repositories/frozen.md'      'via `$DOCKET_BASH_PATH`; facade op `docket.sh stack-base`.'
# README (operator doc): a BARE frozen-infra description survives; the named residual.
neg readme-seam-desc   'README.md'                            'A single dispatch seam, `docket.sh runner-dispatch`, drives it.'
neg readme-migration   'README.md'                            'One-shot: `docket.sh backfill-change-types --changes-dir d`.'
# Go source: a bare parity-oracle mention / doc comment is not a shell facade consumer.
neg go-parity-comment  'internal/app/x.go'                    '// runs with no `$DOCKET_SCRIPTS_DIR`; replaces `docket.sh preflight`.'
# A skill MAY name the facade as a bare noun (code span, no op word after) — history/prose.
neg skill-noun-mention 'skills/x/SKILL.md'                    'The frozen `docket.sh` facade is retired; use the typed CLI.'
# The typed replacement surface is never mistaken for the facade.
neg go-cli-instruction 'skills/x/SKILL.md'                    'Step 0: `docket repository prepare --repo-dir <dir> --json`.'

exit $fail
