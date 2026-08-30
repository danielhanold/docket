#!/usr/bin/env bash
# tests/test_deferred_surface_seal.sh — change 0372: the final consumer-cutover seal.
#
# CLAIM: no MAINTAINED file re-activates a deferred Go-v1 feature family. Prohibited set =
# the facade op-structure NARROWED to the retired op-tokens (mint-stub,
# render-learnings-index, terminal-publish, mark-publish-deferred) + a direct retired
# script invocation + enabled-deferred-key -> Bash wiring. Still-supported facade ops
# (preflight, env, board-refresh, docket-status) and 0369's frozen carve-outs
# (archive-change, render-change-links, render-adr-index) are deliberately OUTSIDE the
# prohibited set until change 0370 (spec: "narrowed through the explicit retirement
# classification").
#
# CORPUS is structural, never a caller-file list (enumerated-floor): every git-tracked
# file MINUS five structural exclusions —
#   scripts/                       the frozen Bash tree awaiting 0370
#   docs/                          point-in-time history (changes/specs/plans/results/ADRs)
#   tests/                         the frozen parity/deletion corpus + this suite itself
#   internal/repository/testdata/  package-local recorded fixture corpora (DATA, not source)
#   testdata/                      the ROOT frozen cross-package repository fixture corpus —
#                                  DATA snapshots versioned by docket release (testdata/README.md),
#                                  byte-identical to origin/main and never a maintained caller. It
#                                  carries exactly one retired-op-token in a SEAL shape
#                                  (plan-with-backlink.md's frozen `terminal-publish.sh --…` line),
#                                  which is history, not a live re-activation; Task 6's fixture-corpus
#                                  negative control proves this exclusion cannot hide a Shape-A hit.
# Each exclusion is bounded (a named directory prefix, no wildcards) and held honest by
# Task-6 negative controls + the live-tree tolerance floors below
# (frozen-fixture-corpus-trips-repo-wide-scans).
#
# RESIDUAL LIMITS (named, per byte-pattern-guard-matches-a-spelling): (1) a bare noun
# mention of a retired script ("mark-publish-deferred.sh" with no arguments and no
# invocation prefix) is permitted — prose may describe history (ADR-0030 precedent);
# (2) a paraphrase that re-teaches a retired op with no op token or script name at all is
# invisible to any byte guard — whole-branch review owns it; (3) key->Bash wiring is
# detected only on ONE physical line (multi-line wiring is review-owned; one bounded gap
# per ERE, stacked gaps hang — stacked-gap-regex-hangs-instead-of-failing).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

RETIRED='mint-stub|render-learnings-index|terminal-publish|mark-publish-deferred'
DEFERRED_KEYS='terminal_publish|auto_capture|AUTO_CAPTURE|learnings\.enabled|LEARNINGS_ENABLED'
INVOKE_SHAPE='docket\.sh|DOCKET_SCRIPTS_DIR|scripts/[a-z][a-z-]*\.sh'

# stdin: repo-relative paths; stdout: the maintained corpus (structural exclusions applied)
# The root `testdata/` prefix excludes ONLY the root fixture tree; `internal/repository/testdata/`
# is a distinct package-local tree under `internal/` that `^testdata/` cannot reach, so both
# prefixes are load-bearing.
seal_filter(){
  grep -Ev '^(scripts/|docs/|tests/|internal/repository/testdata/|testdata/)'
}

# $1=root dir, $2=file holding relative paths (one per line). Emits SEAL VIOLATION records.
seal_scan(){
  local root="$1" list="$2" p hits line fam kind
  while IFS= read -r p; do
    [ -f "$root/$p" ] || continue
    kind='canonical'
    case "$p" in internal/assets/embedded/tree/*) kind='generated' ;; esac
    # Shape A: retired facade op (both sides of the op token bounded)
    hits="$(grep -nE -e "docket\.sh[[:space:]]+($RETIRED)([^[:alnum:]_-]|\$)" "$root/$p")" || true
    # Shape B: direct retired-script invocation (an argument, or the invocation prefix)
    hits="$hits$(grep -nE -e "($RETIRED)\.sh[[:space:]]+--" "$root/$p")" || true
    hits="$hits$(grep -nE -e "DOCKET_SCRIPTS_DIR[^}]*\}\"?/($RETIRED)\.sh" "$root/$p")" || true
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      fam="$(grep -oE -e "$RETIRED" <<<"$line" | sed -n 1p)"
      printf 'SEAL VIOLATION %s %s %s:%s\n' "$kind" "${fam:-facade}" "$p" "$line"
    done <<<"$hits"
    # Shape C: enabled-deferred-key -> Bash wiring (same-line co-occurrence)
    hits="$(grep -nE -e "$DEFERRED_KEYS" "$root/$p" | grep -E -e "$INVOKE_SHAPE")" || true
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      printf 'SEAL VIOLATION %s deferred-key-wiring %s:%s\n' "$kind" "$p" "$line"
    done <<<"$hits"
  done <"$list"
}

# ---- live-tree run ----------------------------------------------------------------------
LIST="${TMPDIR:-/tmp}/seal-corpus.XXXXXX"; LIST="$(mktemp "$LIST")"
git -C "$REPO" ls-files | seal_filter >"$LIST"

# Population floors: the corpus is real and holds the surfaces this seal exists to watch.
# Floor is ~half the measured maintained-corpus count: 879 files measured on 2026-08-30
# (git ls-files | seal_filter | wc -l), after the fifth structural exclusion (root testdata/)
# narrowed it from 937; half of 879 rounds to 440.
corpus_n="$(wc -l <"$LIST" | tr -d ' ')"
assert "corpus floor: the maintained corpus is populated (found $corpus_n)" '[ "$corpus_n" -ge 440 ]'
for must in skills/docket-convention/SKILL.md \
            skills/docket-convention/references/terminal-close-out.md \
            internal/assets/embedded/tree/skills/docket-convention/SKILL.md \
            README.md AGENTS.md .docket.example.yml; do
  assert "corpus floor: $must is scanned" 'grep -qxF "$must" "$LIST"'
done
# Exclusion boundedness: the excluded trees are PRESENT and carry the very content the
# scan must tolerate — so the exclusions are load-bearing, not decorative.
assert "frozen tree present: scripts/mint-stub.sh still ships (0370's to delete)" \
  '[ -f "$REPO/scripts/mint-stub.sh" ] && [ -f "$REPO/scripts/terminal-publish.sh" ]'
assert "history preserved: an archived ADR still names docket.sh mint-stub" \
  'grep -qF "docket.sh mint-stub" "$REPO/docs/adrs/0045-auto-capture-is-best-effort.md"'

violations="$(seal_scan "$REPO" "$LIST")"
assert "SEAL: zero prohibited surfaces in the maintained corpus
$violations" '[ -z "$violations" ]'

# Diagnostic floors: the deferral boundary the absence relies on is actually stated
# (assert-detects-removal-not-replacement: absence + companion through the same surface).
assert "floor: capture deferral stated in the convention" \
  'grep -qF "automatic change capture is deferred from Go v1" "$REPO/skills/docket-convention/SKILL.md"'
assert "floor: harvest deferral stated in the learnings reference" \
  'grep -qF "automated learnings harvest is deferred from Go v1" "$REPO/skills/docket-convention/references/learnings.md"'
assert "floor: publication deferral stated in the close-out reference" \
  'grep -qF "terminal publication is deferred from Go v1" "$REPO/skills/docket-convention/references/terminal-close-out.md"'
# Supported-surface controls: the narrowing really permits what it claims to permit.
assert "control: still-supported board pass survives in maintained prose" \
  'grep -rqF -- "docket.sh docket-status --board-only" "$REPO/skills/"'
assert "control: supported Go closeout boundary survives" \
  'grep -qF "docket finalize closeout --id" "$REPO/skills/docket-convention/references/terminal-close-out.md"'
assert "control: supported Go ADR transactions survive" \
  'grep -qF "docket adr record" "$REPO/skills/docket-adr/SKILL.md"'
assert "control: Go learnings read/validate path untouched" \
  '[ -f "$REPO/internal/repository/decode.go" ] && [ -f "$REPO/internal/repository/validate.go" ] && [ -f "$REPO/internal/render/adrindex.go" ]'

rm -f "$LIST"
exit $fail
