#!/usr/bin/env bash
# tests/test_skill_facade_wiring.sh — change 0072 (facade skill rewiring)
#
# Guards that live agent-facing skill prose invokes docket helpers ONLY through the
# docket.sh facade (canonical byte-exact spelling), that the retired shapes
# (eval "$(", inline `fetch origin`, `pull --rebase`) are gone from code spans, and that
# the Step-0 preamble + mid-run re-sync are PRESENT, and the CREATE_ORPHAN path routes
# through the `docket.sh bootstrap` facade verb — the direct-helper carve-out is RETIRED
# (change 0074), so a prefixed `docket-config.sh` invocation reappearing in skill prose
# reddens via BOTH the Layer-1 sweep and the `cfg_invocations == 0` assert (two independent
# scans).
#
# SOUND-GUARD READING (load-bearing): the Layer-1 sweep guards non-facade INVOCATIONS,
# not every `.sh` token. Discriminator = the invocation prefix `${DOCKET_SCRIPTS_DIR`
# (every retired invocation carries it) + the retired shapes. Descriptive NOUN mentions
# (`board-refresh.sh`, `sync-agents.sh`, `render-board.sh`, `scripts/<name>.sh`) carry no
# prefix and are PERMITTED — this is what lets references/agent-layer.md pass without
# rewiring (spec §3), and avoids over-scoping into a reference doc about sync-agents.sh
# (spec §Out-of-scope). Stripping the canonical `…/docket.sh` first is what makes the
# `${DOCKET_SCRIPTS_DIR` assert sound (the canonical spelling contains install.sh in :?).
#
# This test was RED until change 0072's Tasks 2-4 rewired the prose — that per-assert RED was
# the reverse-mutation proof that each guard bites pre-rewiring prose. Green since; change 0074
# re-proved the flipped carve-out assert by mutation in both directions.
#
# HARNESS: this repo's test harness is self-contained (there is NO tests/lib/). The
# boilerplate below mirrors tests/test_docket_facade.sh verbatim: `set -uo pipefail`,
# a REPO root, `fail`, an inline `assert` that eval's a command STRING which must exit 0,
# and `exit $fail`.
#
# PORTABILITY / CORRECTNESS notes (deviations from the plan draft, all deliberate):
#   * The forbidden fixed-string patterns are held in single-quoted variables
#     (P_PREFIX/P_EVAL/P_FETCH/P_REBASE) and referenced as `grep -qF "$P_*"`. Writing the
#     pattern inline as `grep -qF "${DOCKET_SCRIPTS_DIR"` (as the draft did) is an
#     UNTERMINATED parameter expansion — a hard error under `set -u`, not a literal.
#   * strip_canonical uses BSD-sed-safe `-E` (verified on /usr/bin/sed) and extract_code_units
#     uses BSD-awk-safe match()/RSTART/RLENGTH (verified on /usr/bin/awk). Runtime `grep` here
#     is GNU (homebrew gnubin); all flags used (-oE/-qxF/-hoF/-cF) are common to both.
#   * The op-inventory rule emits ONE assert per in-scope file (accumulating any off-inventory
#     ops), so it is visible/non-vacuous rather than silent-unless-violated.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

CONV="$REPO/skills/docket-convention/SKILL.md"

# In-scope files (glob-derived; the glob IS the derivation, so no separate FLOOR is needed).
# skills/*/SKILL.md covers every operating skill + the convention; references/*.md adds the
# two convention reference docs. The two globs do not overlap.
SCOPE=( "$REPO"/skills/*/SKILL.md "$REPO"/skills/docket-convention/references/*.md )

# Layer 3's OWN corpus (change 0071 review, finding 2) — deliberately WIDER than SCOPE above, and
# NEVER used to narrow, weaken, or replace Layer 1/2's SCOPE (ADR-0031: only narrow or add
# alongside an existing guard, never widen/weaken/collapse it in place — so this is a SEPARATE
# array, not an edit to SCOPE). SCOPE alone left a hole exactly where it matters: it never scanned
# `skills/docket-convention/github-board-mirror.md` — a file the convention itself calls
# "skill-reference", that `docket-status/SKILL.md` sends the agent to READ, and that already
# describes the Board pass and `board_surfaces` — making it the single reference doc most likely
# to acquire a 9th surfaces-spelling call site while this sentinel stayed green. It also missed the
# `*-template.md` files (change-template.md, adr-template.md, results-template.md). `skills/*/*.md`
# catches every markdown file one level under skills/ (every SKILL.md, both templates, and
# github-board-mirror.md); `skills/*/references/*.md` adds the convention's reference docs
# (terminal-close-out.md, agent-layer.md) that live one level deeper. Verified: no retired spelling
# exists in this widened corpus today (green immediately), and it does not change
# board_pass_files's count (none of the newly-added files carry the facade call).
SCOPE3=( "$REPO"/skills/*/*.md "$REPO"/skills/*/references/*.md )

# Op inventory: grep-derived from scripts/docket.md's Subcommand table (NEVER hand-listed;
# same derivation the facade's own sentinel uses).
INVENTORY="$(grep -oE '^\| `[a-z-]+` ' "$REPO/scripts/docket.md" | tr -d '|` ' | sort -u)"

# Forbidden fixed-string patterns, kept literal via single quotes (see PORTABILITY note).
P_PREFIX='${DOCKET_SCRIPTS_DIR'   # any surviving invocation prefix = a non-facade / non-byte-exact invocation
P_EVAL='eval "$('                 # the retired eval-preamble shape
P_FETCH='fetch origin'            # retired inline fetch
P_REBASE='pull --rebase'          # retired inline rebase

# Emit code UNITS for a file: fenced-block bodies (verbatim lines) + inline code spans.
extract_code_units() {
  awk '
    /^[[:space:]]*```/ { infence = !infence; next }   # indented fences (list-item code blocks) toggle too
    infence { print; next }
    {
      line = $0
      while (match(line, /`[^`]*`/)) {
        print substr(line, RSTART, RLENGTH)
        line = substr(line, RSTART + RLENGTH)
      }
    }
  ' "$1"
}

# Strip the single byte-exact canonical facade form (`…/docket.sh`) so only the haystack
# remains. After 0074 there is no second form: a prefixed `…/docket-config.sh --bootstrap`
# is no longer stripped, so it survives into `$hay` and trips the Layer-1 `$P_PREFIX` sweep.
strip_canonical() {
  sed -E \
    -e 's#"\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}"/docket\.sh##g'
}

# ---- Layer 1: absence sweep, per in-scope file ----
for f in "${SCOPE[@]}"; do
  rel="${f#$REPO/}"
  units="$(extract_code_units "$f")"
  hay="$(printf '%s\n' "$units" | strip_canonical)"

  assert "no non-facade / non-byte-exact helper invocation prefix survives in $rel" \
    '! printf "%s" "$hay" | grep -qF "$P_PREFIX"'
  assert "no eval-preamble shape in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF "$P_EVAL"'
  assert "no inline \`fetch origin\` in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF "$P_FETCH"'
  assert "no inline \`pull --rebase\` in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF "$P_REBASE"'

  # every `docket.sh <op>` in this file's code units must name an inventory op
  # (checked on RAW units, incl. the canonical facade form).
  bad_ops=""
  while read -r op; do
    [ -z "$op" ] && continue
    printf '%s\n' "$INVENTORY" | grep -qxF "$op" || bad_ops="$bad_ops $op"
  done < <(printf '%s\n' "$units" | grep -oE 'docket\.sh [a-z-]+' | awk '{print $2}' | sort -u)
  assert "every \`docket.sh <op>\` in $rel names an inventory op (off-inventory:[$bad_ops])" \
    '[ -z "$bad_ops" ]'
done

# The CREATE_ORPHAN carve-out is RETIRED (change 0074): skill prose must reach docket-config.sh
# ZERO times as an invocation. Key on the prefixed invocation form, never the bare
# `docket-config.sh --bootstrap` string — prose may still NAME the script/flag descriptively
# (ADR-0030), and only the `${DOCKET_SCRIPTS_DIR`-prefixed spelling is an actual invocation.
# `grep -F --` is mandatory: the pattern is a fixed string that must never be read as an option.
CFG_INVOKE='"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh'
cfg_invocations="$(grep -hoF -- "$CFG_INVOKE" "${SCOPE[@]}" | wc -l | tr -d ' ')"
assert "no prefixed docket-config.sh invocation survives in skill prose (carve-out retired)" \
  '[ "$cfg_invocations" = "0" ]'

# ---- Layer 2: presence anchors on the convention (grep -c == 1) ----
assert "Step-0 preamble runs preflight as its own call (unique anchor)" \
  '[ "$(grep -cF "as its own Bash call" "$CONV")" = "1" ]'
assert "mid-run re-sync verb is defined once (unique anchor)" \
  '[ "$(grep -cF "push-retry CAS loops alike" "$CONV")" = "1" ]'
assert "Step-0 instructs reading the printed block (unique anchor)" \
  '[ "$(grep -cF "read the printed \`KEY=value\` block" "$CONV")" = "1" ]'

# ---- Layer 3: the board-surfaces sentinel (change 0071) ----------------------------------------
# 0071 removed every surfaces value from skill prose: the Board pass is now the single facade call
# `docket.sh docket-status --board-only`, and the orchestrator self-resolves its config. This
# sentinel is what keeps it that way — the call-site list is grep-derived, so it can never
# silently regrow (LEARNINGS: derive gated call-site lists by grep, never by hand; guard the
# list's completeness with a sentinel, not with review attention).
#
# SCOPE DISCRIMINATION (ADR-0030), and why the three clauses differ:
#   * `BOARD_SURFACES` and `--surfaces` are forbidden across the WHOLE file. Neither has any
#     legitimate descriptive use in skill prose — the config KEY is the lowercase `board_surfaces`
#     (a different string, deliberately untouched). Whole-file (not code-unit) scoping is the
#     stronger guard here and costs nothing.
#   * `docket.sh board-refresh` is forbidden as an INVOCATION, checked BOTH in code units and
#     across the whole file (two independent asserts). The code-unit assert alone left a hole:
#     a plain, un-backticked prose sentence naming the invocation slipped past it (caught in
#     review). The whole-file assert closes that hole and costs nothing, because the phrase has
#     zero legitimate use anywhere in skills/ — unlike the bare noun below. The bare noun
#     `board-refresh.sh` is PERMITTED (the convention's "Derived-view script family" and
#     docket-status's ownership prose both legitimately NAME it; test_board_refresh_on_transition.sh
#     asserts the convention still does). Forbidding the noun would over-scope into prose whose job
#     is to describe the mechanism — the exact rejected alternative in ADR-0030.
#
# PORTABILITY note: `--surfaces` begins with dashes, so a bare `grep -qF "$B_SURF_FLAG" "$f"` is
# parsed by grep as an (unrecognized) OPTION, not a pattern — it errors (exit 2) regardless of the
# file's content, and `! <that>` then reports a false "ok" on EVERY file (a silently vacuous
# assert). Verified locally: `grep -qF "--surfaces" f` exits 2 unconditionally on both this repo's
# shimmed grep and plain GNU grep; the `--` end-of-options marker is what makes the pattern reach
# grep as literal text, so every grep call below carries it.
B_SURF_VAR='BOARD_SURFACES'          # the shell variable, in any spelling ($X, "$X", ${X})
B_SURF_FLAG='--surfaces'             # the retired flag
B_REFRESH_CALL='docket.sh board-refresh'   # the retired direct invocation

board_pass_files=0
for f in "${SCOPE3[@]}"; do
  rel="${f#$REPO/}"
  units="$(extract_code_units "$f")"

  assert "no BOARD_SURFACES variable survives anywhere in $rel" \
    '! grep -qF -- "$B_SURF_VAR" "$f"'
  assert "no --surfaces flag survives anywhere in $rel" \
    '! grep -qF -- "$B_SURF_FLAG" "$f"'
  assert "no direct board-refresh invocation in code units of $rel" \
    '! printf "%s" "$units" | grep -qF -- "$B_REFRESH_CALL"'
  assert "no direct board-refresh invocation survives anywhere in $rel" \
    '! grep -qF -- "$B_REFRESH_CALL" "$f"'

  grep -qF -- 'docket.sh docket-status --board-only' "$f" && board_pass_files=$((board_pass_files + 1))
done

# NON-VACUITY (LEARNINGS: a guard that parses nothing passes everything — assert the unit count the
# extractor found, not just its verdict). The sweep above is only meaningful if the corpus is real
# and the canonical Board-pass call is actually PRESENT where it belongs. Verified directly
# (`grep -rlF 'docket.sh docket-status --board-only' skills/ | sort`) against 7 files: the 5
# status-writing skills (docket-auto-groom, docket-finalize-change, docket-groom-next,
# docket-implement-next, docket-new-change) + the convention SKILL.md + terminal-close-out.md;
# docket-status/SKILL.md and docket-adr/SKILL.md have no Board site of their own. This is a
# per-FILE count (grep -l), not a per-occurrence count — the facade string legitimately repeats
# inside docket-new-change's SKILL.md. Widening the scan corpus to SCOPE3 (finding 2, above) adds
# github-board-mirror.md and the three `*-template.md` files to the scan, but none of them carry
# the facade call, so the count is unaffected.
assert "the in-scope corpus is non-empty (the sentinel actually scanned files)" \
  '[ "${#SCOPE3[@]}" -ge 8 ]'
assert "the canonical Board-pass call is present in every rewired file (found $board_pass_files)" \
  '[ "$board_pass_files" -eq 7 ]'

# ── change 0094: Step 1 acquires its candidate set from the digest's `ready` line ─────────────
# specified-but-unreachable: a contract with a PRODUCER and a CONSUMER needs at least one assert
# anchored on the producer. Here the producer is render-board.sh's ready line and the write-free
# reader is docket-status.sh --digest-only; asserting only that the SKILL mentions a ready line
# would pass identically in a world where nothing ever emits one. So: pin both ends.
impl="$REPO/skills/docket-implement-next/SKILL.md"

# PRODUCER end — the line really is emitted, and the read path really is write-free.
assert "producer: render-board.sh emits a ready line" \
  'grep -q "printf .ready" "$REPO/scripts/render-board.sh"'
assert "producer: docket-status.sh implements --digest-only" \
  'grep -q -- "--digest-only) DIGEST_ONLY=1" "$REPO/scripts/docket-status.sh"'
# The digest-only short-circuit must run BEFORE docket_preflight (which syncs the metadata
# worktree). Compare line-number ORDER rather than using an awk range: a range terminated on the
# bare string `docket_preflight` closes on the explanatory COMMENT above the call and never reaches
# the code. Anchoring on the call's `"$SCRIPTS_DIR"` argument avoids that collision entirely.
ds="$REPO/scripts/docket-status.sh"
mainln=$(grep -n '^main()' "$ds" | cut -d: -f1)
dgln=$(awk -v s="$mainln" 'NR>s && /digest_only_pass/ {print NR; exit}' "$ds")
pfln=$(awk -v s="$mainln" 'NR>s && /docket_preflight "\$SCRIPTS_DIR"/ {print NR; exit}' "$ds")
assert "producer: the digest-only path short-circuits before docket_preflight" \
  '[ -n "$dgln" ] && [ -n "$pfln" ] && [ "$dgln" -lt "$pfln" ]'

# CONSUMER end — Step 1 names the exact invocation a reader must run.
assert "skill Step 1 names the --digest-only invocation" \
  'grep -q -- "docket-status --digest-only" "$impl"'
assert "skill Step 1 names the ready line as its candidate source" \
  'grep -q "\`ready\`" "$impl"'
assert "skill Step 1 keeps the change files authoritative (accelerator, not sole channel)" \
  'grep -qi "accelerator" "$impl"'
assert "skill Step 1 documents the no-ready-line fallback to walking active/" \
  'grep -qi "fall back" "$impl"'
# Important 2 (0094 whole-branch review): the ORIGINAL fallback paragraph treated a missing ready
# line as ONE cause ("an older render-board, a failed render") with ONE response (fall back to
# active/) — but --digest-only now has non-zero-exit, zero-stdout hard-error paths of its own
# (config export failure, a non-PROCEED bootstrap, a missing metadata worktree, a failed render
# after Important 1's fix). Walking active/ on those silently converts a hard fail-closed error
# into `drained` in an autonomous drain loop. The exit status must govern: non-zero => halted,
# never the fallback, never drained; only exit-0-with-no-ready-line falls back.
assert "skill Step 1 treats a non-zero --digest-only exit as a hard error" \
  'grep -qi "is a hard error" "$impl"'
assert "skill Step 1 maps a --digest-only hard error to halted, never the active/ fallback or drained" \
  'grep -qi "never fall back to \`active/\` for a hard error" "$impl" \
   && grep -qi "never report \`drained\` for one" "$impl"'

# POSTURE — the two things spec section 4 says must not be lost in the rewrite.
assert "skill Step 1 still defers the DEFINITION to the convention" \
  'grep -q "Build-readiness & selection" "$impl"'
assert "skill allowlist is still a filter, never a dependency override" \
  'grep -q "never a dependency override" "$impl"'
assert "skill allowlist still skips with a reason and never aborts" \
  'grep -q "skipped with its reason" "$impl" && grep -q "never aborts the run" "$impl"'
assert "skill Step 1 still routes an empty queue to drained" \
  'grep -q "drained" "$impl"'

exit $fail
