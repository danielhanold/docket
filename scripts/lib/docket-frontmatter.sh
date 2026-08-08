#!/usr/bin/env bash
# scripts/lib/docket-frontmatter.sh — shared frontmatter, dependency-resolution, and vocabulary
# helper for
# docket's deterministic board/mirror scripts (change 0022). SOURCE this; it has no side effects
# on source beyond declaring functions and the dependency-resolution globals. No git, no network.
#
# Provides:
#   field_raw FILE KEY    — first matching scalar for KEY, trimmed, surrounding quotes INTACT (raw
#                           token). For a consumer doing its own quote/escape decoding.
#   field FILE KEY        — first matching scalar for KEY anywhere in the file, trimmed and with a
#                           single matched pair of surrounding quotes stripped (logical value).
#   fm_field FILE KEY     — like field(), but ONLY inside the first ---...--- block. Use this for
#                           any key that may be ABSENT from frontmatter (e.g. type:), where field()
#                           would fall through and return body prose. Same quote-stripping as field().
#   fm_field_raw FILE KEY — the RAW anchored twin of fm_field: same ---...--- scope and inline
#                           comment strip, but leaves any surrounding quotes INTACT for a consumer
#                           doing its own quote/escape decoding (change 0191).
#   fm_field_verbatim FILE KEY — the same anchored scope with NEITHER strip: quotes AND a
#                           whitespace-preceded `#...` are left intact, so the value arrives exactly
#                           as authored. For a consumer JUDGING the scalar's YAML form, which must
#                           not be handed a value the reader already truncated (change 0235).
#   docket_yaml_single_quote VALUE — VALUE as a single-quoted YAML scalar (interior `'` doubled);
#                           the exact inverse of field()/fm_field()'s undoubling (change 0235).
#   docket_scalar_quote_reason VALUE — one leg token (colon-space | trailing-colon | bare-boolean |
#                           comment-introducer | indicator) when VALUE would not be well-formed as a
#                           BARE YAML scalar; empty when it is safe bare. Checker-side (change 0235).
#                           Its EXIT STATUS is always 0, on every path including a violation — the
#                           answer travels on stdout. `if docket_scalar_quote_reason "$v"; then` is
#                           therefore always true; read stdout, or use docket_scalar_needs_quoting.
#   docket_scalar_needs_quoting VALUE — exit 0 iff docket_scalar_quote_reason printed a token.
#                           CHECKER-SIDE ONLY: a writer must quote unconditionally (ADR-0071), never
#                           predicate on this. It has no production caller — tests only.
#   list_field FILE KEY   — `[a, b]` -> space-separated `a b` (empty for `[]` / unset).
#   int_field FILE KEY    — like field(), but empty unless the value is a well-formed non-negative integer.
#   has_section FILE STR  — exit 0 iff the body contains the literal line STR (whole-line match:
#                           a prose mention of the marker is NOT the section).
#   iso_to_epoch ISO      — UTC ISO-8601 timestamp -> epoch seconds; empty on parse failure.
#   resolve_deps DIR      — scan DIR/active + DIR/archive once; populate the globals below.
#   readiness FILE        — build-ready | needs-brainstorm | auto-groom-blocked | waiting.
#   finalize_blocked FILE — exit 0 iff the body carries `## Finalize blocked` (implemented only).
#   docket_status_is_active STATUS   — exit 0 iff STATUS is a non-terminal lifecycle status.
#   docket_status_is_terminal STATUS — exit 0 iff STATUS is a terminal lifecycle status.
#   docket_priority_is_member VALUE  — exit 0 iff VALUE is a declared priority (empty is false).
#   docket_priority_rank VALUE       — print the rank index; empty/unknown uses the default rank.
#
# --- THE SELECTION RULE (canonical; change 0244) ------------------------------------------------
# Four scalar read shapes with silently different behavior. The question that picks one is never
# "is this read anchored?" but "CAN THIS KEY BE ABSENT from the frontmatter of every file this
# call site reads?" — because an unanchored read of an absent key does not return empty, it runs
# past the closing `---` and returns whatever body line happens to open with that word. In a repo
# whose subject matter IS the field names, body prose opening `pr:` or `spec:` is not a contrived
# fixture; it is the normal content of a change file.
#
#   caller needs                                          | accessor
#   ------------------------------------------------------|---------------------
#   key guaranteed present, logical value                  | field
#   key guaranteed present, caller decodes quotes itself   | field_raw
#   key may be ABSENT, ordinary structured value           | fm_field
#   key may be ABSENT, caller decodes quotes itself        | fm_field_raw
#   key may be ABSENT, caller JUDGES the YAML form as authored, or the value is free prose where
#     a whitespace-preceded `#` is DATA (blocked_by)       | fm_field_verbatim
#
# GUARANTEED PRESENT is decided by one question: CAN any file this call site reads
# legitimately omit the key — a hand-authored file, a file minted under an EARLIER template, a key
# whose writer may drop the line? If it can, the key is not guaranteed, however many files carry it
# today. A key the current template does not ship is by that alone not guaranteed. The converse does
# NOT hold: template presence would prove a key guaranteed only if every file had been minted by
# today's template and no human ever edited one, and neither holds here — change files are
# hand-edited constantly and hundreds of them predate various template revisions. So
# presence-in-the-corpus-today is a snapshot, not a guarantee, and the guaranteed set below is a
# deliberate hand-maintained list rather than anything measured. Today that is: change files — id,
# status, slug, title, priority, created, updated; ADRs — id, status, title, change, date;
# learnings findings — slug, hook, topics. Those sites stay on field()/field_raw() with no churn: the frontmatter line is necessarily the first match, so whole-file scanning is a
# safe optimization, grandfathered rather than recommended.
#
# EVERY OTHER KEY takes an anchored read — never field() (ADR-0057). In docket's own schema the
# absent-capable set is spec, plan, results, branch, pr, issue, blocked_by, type, claimed_at,
# trivial, auto_groomable, promotion_state, promoted_to, discovered_from. Several of those — spec,
# plan, results, branch, pr, blocked_by, trivial — ARE shipped by today's change template and do sit
# in every change file on the metadata branch right now. They are absent-capable regardless: each is
# semantically OPTIONAL and hand-authorable, so a writer may drop the line and an older file may
# never have carried it. Do not promote a key into the guaranteed set on the strength of a census.
#
# Within the anchored tier, fm_field is the default. fm_field_verbatim is for exactly two jobs:
# a consumer JUDGING the scalar's YAML form as authored (board-checks's scalar_form_check, which
# cannot be handed a value the reader already repaired), and a free-prose value where the comment
# strip would TRUNCATE data — blocked_by, whose `PR #69 is stale` arrives as `PR` through fm_field.
# The accepted cost is that a hand-quoted blocked_by renders with its quotes intact.
#
# The raw tier (field_raw / fm_field_raw) is for a caller doing its OWN quote/escape decoding
# (ADR-0058). Two live callers, both field_raw on always-present keys: render-learnings-index.sh's
# dequote() on hook, and board-checks.sh's scalar_form_check on title, which must see the quotes to
# know whether a colon-space is quoted or bare. fm_field_raw has ZERO production callers today —
# tests/test_frontmatter_read_shapes.sh pins that at zero. It is kept deliberately, not by neglect:
# it is the documented raw twin the next optional-key decoding consumer reaches for, and without it
# the raw/anchored quadrant of the table above would be empty. Neither adopt nor delete it silently.
#
# When unsure whether a key is optional, use the anchored shape. Anchoring is always correct;
# whole-file is only ever an optimization. This rule is guarded by the (corpus, accessor, key)
# census in tests/test_frontmatter_read_shapes.sh, which PARSES the three guaranteed sets out of the
# "Today that is:" sentence above rather than restating them — so promoting a genuinely
# always-present key is a conscious edit to THIS sentence, which is the point. The census decides a
# call site's corpus per call site, from the (script, file-argument) pair; a site it cannot classify
# is a violation, never a pass.
#
# The list_field/int_field wrappers deliberately keep delegating to field(): their live production
# keys (id, depends_on, adrs, topics, supersedes/reverses/relates_to, pr) are empirically 0-missing
# across the tree. related/discovered_from have test-only wrapper callers, and discovered_from: is
# genuinely ABSENT from ~96 pre-template change files — so they are test-only, NOT
# template-guaranteed. Migrating the wrappers themselves is out of scope for 0244.
#
# resolve_deps globals (keyed by integer id):
#   STATUS_OF[id]   the change's own status
#   DEP_STATE[id]   clear | waiting
#   DEP_REASON[id]  "" | "not yet built" | "needs your merge"   (worst unmet; needs-your-merge wins)
#   DEP_ON[id]      bare id of the worst unmet dependency ("" when clear) — display support for #N

# --- frontmatter accessors (lifted from github-mirror.sh, which now sources them here) --------
# _docket_unwrap_quotes VALUE -> logical scalar on stdout, with NO trailing newline.
# Strips a SINGLE matched pair of surrounding quotes (both " or both ') when VALUE is at least two
# characters and its first and last characters are the same quote char. The ONE escape it inverts is
# single-quoted YAML's only rule: an interior `''` collapses to `'` (change 0235, ADR-0071). A
# DOUBLE-quoted interior stays byte-for-byte — no unescaping is added there (\" and \\ are untouched
# — see change 0138, spec Assumption 3), and a BARE value is never rewritten at all. Pure bash
# parameter expansion — no subshell, no fork, no external tool — so it is pipefail-safe and portable
# across GNU/BSD hosts. field() and fm_field() are its only callers.
_docket_unwrap_quotes(){
  local v="$1" q
  if [ "${#v}" -ge 2 ]; then
    q="${v:0:1}"
    if { [ "$q" = '"' ] || [ "$q" = "'" ]; } && [ "${v: -1}" = "$q" ]; then
      v="${v:1:${#v}-2}"
      # Single-quoted YAML interprets NO escapes and has exactly one rule: an embedded ' is
      # written ''. Undouble it here — the exact inverse of docket_yaml_single_quote, which
      # mint-stub now applies to every title (change 0235, ADR-0071). Without this leg an
      # apostrophe-bearing title reads back as manifest''s in BOARD.md and mis-compares in dup_of.
      # Double-quoted tokens are deliberately UNTOUCHED: no escape interpretation is added there
      # (change 0138's stance), and the two double-quoted titles in the tree carry no escapes.
      if [ "$q" = "'" ]; then v="${v//\'\'/\'}"; fi
    fi
  fi
  printf '%s' "$v"
}
# docket_yaml_single_quote VALUE -> a single-quoted YAML scalar on stdout, NO trailing newline.
# Single-quoted YAML interprets no escapes and has exactly one rule: an embedded ' is written ''.
# That makes the output well-formed for EVERY input that carries no control character — so a caller
# quoting unconditionally needs no dangerous-input enumeration, and therefore has no leg to omit
# (ADR-0071). The doubling happens here, in bash, so the value never meets awk's gsub replacement
# syntax (where a literal & would be reinterpreted). _docket_unwrap_quotes is its exact inverse.
docket_yaml_single_quote(){
  printf "'%s'" "${1//\'/\'\'}"
}
# docket_scalar_quote_reason VALUE -> ONE leg token when emitting VALUE as a BARE YAML scalar would
# not be well-formed; empty when it is safe bare. Tokens: colon-space | trailing-colon |
# bare-boolean | comment-introducer | indicator.
#
# Consumer: board-checks.sh's scalar_form_check, which judges HAND-AUTHORED scalars it did not
# write and so must detect. The WRITER does not consume it — mint-stub quotes unconditionally, so
# it has no enumeration to get wrong (ADR-0071). Two rules with different jobs, deliberately.
#
# Takes a value already known NOT to be quoted — the caller strips or skips quoted tokens before
# calling, so for every value that reaches here the raw token and the logical value are the same
# string. Do not hand it a genuinely raw token whose quotes are still attached.
# The already-quoted skip leg lives in scalar_form_check, which is the only
# site holding a raw token: applying it here would be unsound, since a value that logically STARTS
# with a quote character must be quoted, not skipped.
#
# All legs are `case` patterns, never regex — /usr/bin/grep (BSD) and PATH grep (ugrep here) do not
# agree on bounded repetition, and a shape test has no business depending on which one is found.
docket_scalar_quote_reason(){
  local v="$1"
  # Empty is exempt because an empty scalar IS well-formed bare (`claimed_at:` parses to null, not
  # to a broken document) — and scalar_form_check's skip leg short-circuits it before the call
  # anyway. Not a writer accommodation: no writer consumes this predicate (ADR-0071).
  [ -n "$v" ] || return 0
  # There is deliberately NO flow-collection exemption. The question this predicate answers is
  # "would VALUE be well-formed as a BARE SCALAR", and a `[..]` / `{..}` is not a scalar at all — so
  # `[234]` gets the honest answer for the question asked: bare, it does not read back as the string
  # `[234]`. A caller holding a value it MEANS as a sequence or a map (mint-stub's discovered_from
  # write is the one such site, and it quotes nothing) must simply not route it through a scalar
  # predicate; that is a call-site decision, not something a shape test can infer.
  # An exemption cannot be had cheaply here: to protect `[234]` it must run ahead of the legs below,
  # and there it silences all five for any value that merely opens and closes with the matching
  # bracket — `[a title: with colon]` went from a colon-space finding to silence, and a malformed
  # `[WIP] rework]` read as a well-formed collection. It also bought nothing: scalar_form_check, its
  # only consumer, reads `title` and `blocked_by`, and a sweep of every change file on the metadata
  # branch found both to be free-text prose everywhere, never a collection; and
  # the WRITE path consumes no predicate at all (ADR-0071).
  case "$v" in *': '*) printf 'colon-space';        return 0 ;; esac
  case "$v" in *':')   printf 'trailing-colon';     return 0 ;; esac
  case "$v" in
    [Oo][Nn]|[Oo][Ff][Ff]|[Yy][Ee][Ss]|[Nn][Oo]|[Tt][Rr][Uu][Ee]|[Ff][Aa][Ll][Ss][Ee])
                       printf 'bare-boolean';       return 0 ;;
  esac
  # Whitespace followed by '#' opens a YAML comment: it TRUNCATES the value silently rather than
  # aborting the parse, which is the quieter and therefore worse failure. `finding #3` is ordinary
  # auto-capture prose. The leg keys on the [[:space:]] CLASS, not a literal space: a TAB opens a
  # comment just as well, and the detector must be at least as wide as the reader it warns about —
  # fm_field_raw's own inline-comment strip is `[[:space:]]+#`, so a tab-preceded '#' is truncated on
  # the read path whether or not this leg speaks up. mint-stub's control-character gate keeps tabs off
  # the WRITE path, but this predicate's consumer judges HAND-AUTHORED files, which have no such gate.
  case "$v" in *[[:space:]]'#'*) printf 'comment-introducer'; return 0 ;; esac
  # A leading YAML indicator: & and * silently lose meaning, # truncates, the rest abort the parse.
  # A leading '#' is the MAXIMAL case of the leg above: the comment opens at character one, so the
  # whole value parses to null rather than being merely shortened — the quietest failure of the set.
  # It reaches this leg only when the value carries no ': ' and no ' #' of its own, which is an
  # entirely ordinary docket shape (`#235 follow-up work`). A '#' that is neither leading nor
  # whitespace-preceded (`issue#3 reopened`) is part of the value, exactly as YAML defines it, and
  # stays silent here.
  case "$v" in
    '['*|']'*|'{'*|'}'*|','*|'#'*|'&'*|'*'*|'!'*|'|'*|'>'*|"'"*|'"'*|'%'*|'@'*|'`'*|'?'*|':'*|'- '*)
                       printf 'indicator';          return 0 ;;
  esac
  return 0
}
# docket_scalar_needs_quoting VALUE — exit 0 iff the value would not be well-formed bare.
# Checker-side only; a writer must quote unconditionally — ADR-0071. It has no production caller
# today (tests/test_docket_frontmatter.sh is the only one), and that is not an invitation to add a
# writer-side one: a conditional write is only as good as the leg enumeration below.
docket_scalar_needs_quoting(){ [ -n "$(docket_scalar_quote_reason "$1")" ]; }
# field_raw FILE KEY — the first matching scalar for KEY anywhere in the file, trimmed, with any
# surrounding quotes LEFT INTACT (the raw YAML token). For the rare consumer that does its own
# quote/escape decoding and needs the quote style preserved (render-learnings-index.sh's `dequote`
# on the `hook` field). Trailing \n matches the historical field() output shape (piped consumers,
# e.g. the mermaid done-id list, rely on the separator).
field_raw(){
  local raw; raw="$(sed -n "s/^$2:[[:space:]]*//p" "$1")"
  raw="${raw%%$'\n'*}"                              # keep only the first matching line — no pipe
  printf '%s\n' "${raw%"${raw##*[![:space:]]}"}"   # strip trailing whitespace
}
# field FILE KEY — like field_raw, but returns the LOGICAL scalar: a single matched pair of
# surrounding quotes ("..." or '...') is stripped (change 0138). Use for display/comparison of
# ordinary values; use field_raw when the caller does its own richer YAML decoding.
field(){
  printf '%s\n' "$(_docket_unwrap_quotes "$(field_raw "$1" "$2")")"
}
# fm_field FILE KEY — like field(), but reads ONLY inside the FIRST ---...--- block (change 0127).
#
# field() scans the whole file and takes the first match. For the pre-0127 fields that is safe: the
# frontmatter sits at the top, so its line always wins over any body prose discussing the same key.
# It is NOT safe for a key that may be ABSENT from frontmatter while present in body prose — the
# match then falls through to the body and returns prose as a value. `type:` is exactly that case
# during the migration window: every un-backfilled change has no frontmatter type:, and a change
# whose body happens to open a line with `type:` would otherwise render its prose as the type and
# make the backfill refuse to touch it. Anchoring is the same discipline AGENTS.md already requires
# for frontmatter WRITES, applied to the read.
#
# It also strips a YAML inline comment (whitespace-preceded `#` to end of line) before taking the
# value, because change-template.md ships `type:` WITH one. Without the strip an unfilled template
# line reads as the comment text rather than as absent: the change is then neither `untyped` nor a
# real type, so it escapes `--type untyped` (the documented migration inventory), the backfill
# refuses to assign it, and the comment's `|` characters inject phantom columns into its board row.
# mint-stub.sh strips the same shape on WRITE for the same reason; this is the read-side half.
# A `#` not preceded by whitespace is part of the value, exactly as YAML defines it.
#
# The strip is BARE-value territory only: a value that OPENS with `"` or `'` is skipped whole,
# mirroring the skip leg board-checks's scalar_form_check already applies to the raw token — a
# quoted scalar's interior is not comment territory, so `title: 'clear finding #3 from review'`
# must come back whole rather than as the truncated, stray-quote-carrying `'clear finding`
# (change 0235). This matters now that every minted title is unconditionally single-quoted: the
# truncation would otherwise be the routine outcome for any `#`-bearing title, and
# render-artifact-backlink.sh stamps that value into specs, plans, results files and PR bodies.
#
# fm_field_raw() is the RAW anchored twin: it runs the same ---...----scoped awk body as fm_field
# but leaves any surrounding quotes INTACT (no _docket_unwrap_quotes call), for a consumer that
# needs the raw YAML token — e.g. board-checks's scalar-form check, which must see the quotes to
# know whether a colon-space is quoted or bare (change 0191). Single source for the awk body;
# fm_field delegates so the reader shape lives in exactly one place.
#
# fm_field_verbatim() is the third tier: the same anchored scan with NEITHER the quote strip NOR the
# inline-comment strip. It exists because a consumer that JUDGES a scalar's YAML form cannot be
# handed a value the reader already repaired — the comment strip is exactly the truncation the
# `comment-introducer` leg is trying to report, so a checker reading through fm_field_raw sees the
# remnant and stays silent on the real defect (`blocked_by: PR #69 is stale …` reaches it as `PR`).
# Judgement needs the line as AUTHORED; the ordinary readers keep their strip, which is deliberate
# and load-bearing for the change-template's `type: feat   # chosen at creation` line (change 0235).
#
# _fm_scan is the single anchored awk body all three share. STRIP_COMMENT=1 removes a
# whitespace-preceded `#…` before the value is taken; 0 keeps the line whole.
_fm_scan(){ # _fm_scan FILE KEY STRIP_COMMENT -> raw scalar on stdout (empty when absent)
  local raw
  raw="$(awk -v key="$2" -v strip="$3" -v sq="'" '
    BEGIN { n = 0 }
    /^---[[:space:]]*$/ { n++; if (n >= 2) exit; next }
    n == 1 {
      if ($0 ~ ("^" key ":")) {
        line = $0
        val = line; sub("^" key ":[[:space:]]*", "", val)
        lead = substr(val, 1, 1)
        if (strip == 1 && lead != "\"" && lead != sq) sub(/[[:space:]]+#.*$/, "", line)
        sub("^" key ":[[:space:]]*", "", line)
        sub(/[[:space:]]+$/, "", line)
        print line
        exit
      }
    }
  ' "$1")"
  printf '%s' "$raw"
}
fm_field_raw(){ # fm_field_raw FILE KEY -> raw scalar on stdout (quotes intact, empty when absent)
  _fm_scan "$1" "$2" 1
}
fm_field_verbatim(){ # fm_field_verbatim FILE KEY -> the scalar as authored (quotes AND `#…` intact)
  _fm_scan "$1" "$2" 0
}
fm_field(){ # fm_field FILE KEY -> logical scalar on stdout (empty when absent from the first block)
  _docket_unwrap_quotes "$(fm_field_raw "$1" "$2")"
}

list_field(){
  local raw; raw="$(field "$1" "$2")"
  raw="${raw#[}"; raw="${raw%]}"
  printf '%s' "$raw" | tr ',' ' ' | xargs 2>/dev/null || true
}
# int_field FILE KEY — like field(), but returns the value ONLY when it is a well-formed
# non-negative integer (^[0-9]+$); empty string otherwise. Pure; no side effects on source.
int_field(){
  local v; v="$(field "$1" "$2")"
  case "$v" in (''|*[!0-9]*) printf '' ;; (*) printf '%s' "$v" ;; esac
}
# has_section FILE STR — exit 0 iff some line of FILE is EXACTLY STR. `-x` is load-bearing, not a
# nicety: these markers are presence-encoded state, and change files routinely *mention* them in
# prose (`… a dated `## Finalize blocked` body section …`). An unanchored substring match turns any
# such mention into a false "this change is blocked" cell on the board. Whole-line only.
has_section(){ grep -qxF "$2" "$1"; }

# iso_to_epoch ISO — convert a UTC ISO-8601 second-precision timestamp (YYYY-MM-DDTHH:MM:SSZ) to
# epoch seconds on stdout. Tries GNU date first, then BSD/macOS date. Returns 1 (empty stdout) on
# a parse failure — callers treat "no epoch" as "no positive evidence" (never as expired). Single
# source: both board-checks.sh and reclaim-claims.sh use it (do NOT duplicate — escape-ere twin rule).
iso_to_epoch(){
  local iso="$1" e
  e="$(date -u -d "$iso" +%s 2>/dev/null)"                         && { printf '%s' "$e"; return 0; }
  e="$(date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$iso" +%s 2>/dev/null)" && { printf '%s' "$e"; return 0; }
  return 1
}

# --- dependency resolution ----------------------------------------------------
declare -gA STATUS_OF DEP_STATE DEP_REASON DEP_ON

resolve_deps(){ # resolve_deps CHANGES_DIR
  local dir="$1" f id dep dstat worst worst_on
  STATUS_OF=(); DEP_STATE=(); DEP_REASON=(); DEP_ON=()
  local -a files
  mapfile -t files < <(find "$dir/active" "$dir/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
  # pass 1: id -> own status
  for f in "${files[@]}"; do
    id="$(int_field "$f" id)"; [ -n "$id" ] || continue
    STATUS_OF["$id"]="$(field "$f" status)"
  done
  # pass 2: resolve each change's depends_on into the worst unmet reason + its id
  for f in "${files[@]}"; do
    id="$(int_field "$f" id)"; [ -n "$id" ] || continue
    worst=""; worst_on=""
    for dep in $(list_field "$f" depends_on); do
      dstat="${STATUS_OF[$dep]:-}"
      if [ "$dstat" = "done" ]; then
        continue                                   # satisfied
      elif [ "$dstat" = "implemented" ]; then
        if [ "$worst" != "needs your merge" ]; then worst="needs your merge"; worst_on="$dep"; fi
      else
        if [ -z "$worst" ]; then worst="not yet built"; worst_on="$dep"; fi
      fi
    done
    if [ -n "$worst" ]; then
      DEP_STATE["$id"]="waiting"; DEP_REASON["$id"]="$worst"; DEP_ON["$id"]="$worst_on"
    else
      DEP_STATE["$id"]="clear"; DEP_REASON["$id"]=""; DEP_ON["$id"]=""
    fi
  done
}

# --- readiness (precedence pinned: waiting > missing-spec > build-ready) -------
readiness(){ # readiness FILE  (only meaningful for a proposed change)
  local f="$1" id spec trivial
  id="$(int_field "$f" id)"
  if [ "${DEP_STATE[$id]:-clear}" = "waiting" ]; then printf 'waiting'; return; fi
  # ANCHORED (change 0244; ADR-0057). Both keys are optional, and this is the one migration that
  # changes behavior for every caller of readiness() at once (docket-status, render-board,
  # github-mirror). It differs only when spec:/trivial: is absent from frontmatter while body
  # prose opens such a line — where the OLD behavior reported build-ready for an undesigned
  # change, which is the autonomous builder claiming work that was never designed.
  spec="$(fm_field "$f" spec)"; trivial="$(fm_field "$f" trivial)"
  if [ -z "$spec" ] && [ "$trivial" != "true" ]; then
    if has_section "$f" "## Auto-groom blocked"; then printf 'auto-groom-blocked'
    else printf 'needs-brainstorm'; fi
    return
  fi
  printf 'build-ready'
}

finalize_blocked(){ # finalize_blocked FILE  (only meaningful for an implemented change)
  # `## Finalize blocked` is presence-encoded state written by docket-finalize-change when a gate
  # failure leaves a change needing a human. Deliberately NOT part of readiness(), which is by
  # contract meaningful only for a `proposed` change.
  has_section "$1" "## Finalize blocked"
}

publish_deferred(){ # publish_deferred FILE  (meaningful on any change file, active or archived)
  # `## Publish deferred` is presence-encoded state written by mark-publish-deferred.sh when a
  # terminal close-out's publish step was EXPECTED but deferred or blocked (change 0083). Unlike
  # finalize_blocked(), this has NO status gate: the marker is written on the ARCHIVED file, at
  # which point the change is terminal, so gating on a lifecycle status would make it unreadable
  # exactly where it is written. Presence is the whole state.
  has_section "$1" "## Publish deferred"
}

# --- status vocabulary (change 0104) ----------------------------------------------------------
# The seven lifecycle statuses, authored as the convention's two semantic groups: `active/` holds
# every non-terminal status, `archive/` holds the two terminal outcomes. DOCKET_STATUSES is the
# concatenation, in the renderer's display order — the order IS the contract (BOARD.md's section
# order and the digest's `backlog` rollup order both come from iterating it), so never reorder
# these without re-blessing tests/test_render_board.sh's golden.
#
# Single source for render-board.sh's section iteration AND board-checks.sh's `status` field-domain
# check. Duplicating the list makes the checker and the renderer drift in two directions and only
# one of them is detectable: a status added to the renderer but not the checker makes field-domain
# fire a FALSE finding on every file carrying it (and suppresses the board-row-dropped backstop,
# which would otherwise be the thing that noticed), while the reverse direction is caught.
DOCKET_STATUSES_ACTIVE=(in-progress proposed blocked deferred implemented)
DOCKET_STATUSES_TERMINAL=(done killed)
DOCKET_STATUSES=("${DOCKET_STATUSES_ACTIVE[@]}" "${DOCKET_STATUSES_TERMINAL[@]}")

# --- priority vocabulary (change 0116) --------------------------------------------------------
# Ordered by rank, descending. The order IS the convention's deterministic selection semantics:
# critical > high > medium > low. The array index is the ready-queue sort rank.
DOCKET_PRIORITIES=(critical high medium low)
# The default is an independent documented fact, not a positional consequence of the array.
DOCKET_PRIORITY_DEFAULT=medium

# --- change-type vocabulary (change 0127) -----------------------------------------------------
# The BUILT-IN taxonomy. `change_types` in .docket.yml can replace this whole list (never merge
# with it), so every consumer takes an EFFECTIVE list as an argument and this array is only the
# default the resolver falls back to. Order is significant: it survives the resolver's export and
# is the canonical sequence any type-ordered output follows.
DOCKET_CHANGE_TYPES_DEFAULT=(chore docs feat fix refactor perf)

# Pseudo-values that are legal as a config selector or a QUERY token but never legal in a stored
# manifest: `all` is the auto_capture.types selector and the --type wildcard; `untyped` is the
# --type query token for a change carrying no type: yet, and the backfill's migration-set name.
# Storing either would make a selector indistinguishable from a real value.
DOCKET_CHANGE_TYPE_RESERVED=(all untyped)

_docket_array_has(){
  local needle="$1"; shift
  local value
  [ -n "$needle" ] || return 1
  for value in "$@"; do [ "$needle" = "$value" ] && return 0; done
  return 1
}
docket_status_is_active(){ _docket_array_has "$1" "${DOCKET_STATUSES_ACTIVE[@]}"; }
docket_status_is_terminal(){ _docket_array_has "$1" "${DOCKET_STATUSES_TERMINAL[@]}"; }
# Membership over the FULL seven-name vocabulary — the union its two siblings partition. Distinct
# from both: `_active` and `_terminal` each answer "which half", and a consumer that only needs
# "is this a status at all" would otherwise have to call both or restate the list. render-board.sh's
# malformed-file validation (change 0259) is that consumer: a status outside this vocabulary can
# never be a legal array subscript or a legal TAB-join field, so rejecting by vocabulary IS the
# sanitization — a value carrying an interior TAB or CR cannot match any of the seven names.
docket_status_is_member(){ _docket_array_has "$1" "${DOCKET_STATUSES[@]}"; }
docket_priority_is_member(){ _docket_array_has "$1" "${DOCKET_PRIORITIES[@]}"; }

# Membership over the EFFECTIVE list the caller resolved — never over the built-in array. A change
# file may legitimately carry a type absent from THIS machine's effective list (another machine's
# config wrote it), so readers must not use this to decide whether to RENDER a stored value; it
# gates creation and admission only.
docket_change_type_is_member(){ # docket_change_type_is_member VALUE TYPE...
  local value="$1"; shift
  _docket_array_has "$value" "$@"
}

docket_change_type_is_reserved(){ # docket_change_type_is_reserved VALUE
  _docket_array_has "$1" "${DOCKET_CHANGE_TYPE_RESERVED[@]}"
}

# Shape gate for the spec's `[a-z][a-z0-9-]*`, keyed on shape rather than an enumerated set of bad
# spellings (AGENTS.md). Deliberately pure `case` — no `printf | grep -Eq`, which would be a
# producer piped into an early-exiting consumer (SIGPIPE 141 under pipefail), and whose line-wise
# match would accept a multi-line value on the strength of its first line alone. The two patterns
# together reject: empty, a non-lowercase-alpha first character, and any subsequent character
# outside [a-z0-9-] — including a space, colon, underscore, or embedded newline.
docket_change_type_is_wellformed(){ # docket_change_type_is_wellformed VALUE
  case "$1" in
    ''|[!a-z]*)   return 1 ;;
    *[!a-z0-9-]*) return 1 ;;
  esac
  return 0
}
docket_priority_rank(){
  local wanted="$1" value i=0
  docket_priority_is_member "$wanted" || wanted="$DOCKET_PRIORITY_DEFAULT"
  for value in "${DOCKET_PRIORITIES[@]}"; do
    [ "$wanted" = "$value" ] && { printf '%s' "$i"; return 0; }
    i=$(( i + 1 ))
  done
  return 1
}

# --- board-checks check-id vocabulary (change 0111) --------------------------------------------
# The CLOSED check-id vocabulary board-checks.sh emits. Declared HERE, beside DOCKET_STATUSES,
# rather than in board-checks.sh itself, because board-checks.sh is not sourceable — a guard
# wanting the set would have to parse its source text, manufacturing exactly the tokenizer that
# can drift from what bash actually assigns. This lib IS sourceable (board-checks.sh's
# `source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"` runs well before its
# `emit()` definition), so tests/test_board_checks.sh reads the real runtime array.
#
# Accepted impurity: this lib's name says "frontmatter" and a check-id is not a frontmatter field.
# Noted deliberately; rationalising the lib's naming is change 0116's charter.
#
# Every entry is pinned in BOTH directions against the set board-checks.sh emits, against the
# script's own --help header enumeration, against scripts/board-checks.md's per-check sections, and
# against scripts/docket-status.md's `check` report-line row. Adding a check-id means editing the
# array plus the four surfaces it is pinned against; the guard's failure messages name them.
BOARD_CHECK_IDS=(aborted-run adr-unpublished board-row-dropped broken-plan-results broken-spec
                 dep-cycle field-domain malformed-id merge-gate-stall merged-orphan
                 publish-deferred scalar-form stale-finalize-blocked stale-in-progress
                 unknown-commit-ref)
