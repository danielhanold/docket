# Validate numeric `id` across the frontmatter script family — design

**Change:** 0032 — one shared `id`-validation helper, adopted uniformly
**Date:** 2026-06-20 · **Status:** draft (auto-groom) · **Depends on:** 0030 (done)
**Authoring mode:** autonomous (docket-auto-groom). The `## Assumptions` block is the
deferred audit trail and the critic's attack surface.

## Problem

The family scripts read `id:` and trust it to be a well-formed integer: it becomes a
`declare -A` key and (in `adr-checks.sh` line 42) feeds arithmetic
`[ "$id" -gt "$MAXID" ]`. Every scanner already guards with
`id="$(field "$f" id)"; [ -n "$id" ] || continue` — which catches an **empty** id but
**not a non-numeric** one (`id: abc`, `id: 1.5`, `id: 01x`). A non-numeric id slips
past, becomes a junk array key, and trips `pad`/arithmetic under `set -u`. In
practice ids are always integers (allocated max+1, encoded in the filename), so this
never bites — but it is an unguarded, codebase-wide assumption flagged in 0030's
review.

### Surface map (verified)

| Script | Reads `id:`? | At-risk use | Role |
|---|---|---|---|
| `lib/docket-frontmatter.sh` (`resolve_deps`) | yes | `STATUS_OF[$id]` key | shared scan |
| `render-board.sh` | yes | `SECTION[$id]` key, `pad "$id"` | renderer |
| `render-adr-index.sh` | yes | `T_*[$id]` keys, `pad "$id"` | renderer |
| `board-checks.sh` | yes | finding key, cycle `ADJ[$cid]` | **validator** |
| `adr-checks.sh` | yes | `EXISTS[$id]`, **`[ "$id" -gt "$MAXID" ]`** | **validator** |
| `terminal-publish.sh` | **no — `--id` is a CLI arg** | `printf '%04d' "$ID"` | publisher |

## Decision

**One shared helper, adopted by role.** Hardening is uniform in *mechanism* (a single
lib helper) and role-appropriate in *behaviour* (the split the stub proposes).

### 1. Shared helper in `scripts/lib/docket-frontmatter.sh`

```sh
# int_field FILE KEY — like field(), but returns the value ONLY when it is a
# well-formed non-negative integer (^[0-9]+$); empty string otherwise. No side effects.
int_field(){
  local v; v="$(field "$1" "$2")"
  case "$v" in (''|*[!0-9]*) printf '' ;; (*) printf '%s' "$v" ;; esac
}
```

A sibling of `field`, pure (no diagnostics — the lib stays side-effect-free on
source, per its header). Returning empty means **every existing
`[ -n "$id" ] || continue` guard now also skips a malformed id**, with zero new
control flow in the scanners.

### 2. Adopt by role

- **Renderers + the shared scan** (`render-board.sh`, `render-adr-index.sh`,
  `resolve_deps`): replace `field "$f" id` with `int_field "$f" id` at **every**
  `id:` read site, not only the first scan guard — a half-hardened renderer that
  still feeds `printf '%04d'` from an un-guarded path is the failure mode to avoid.
  Enumerate the sites with `grep -n 'field "[^"]*" id'` before editing; the known
  ones are: `render-board.sh` lines **52** (SECTION builder), **164** (done-id list),
  and the **archive** builder (~182, whose `[ -n "$id" ]` guard runs on the
  post-sort tuple, so its raw read must also use `int_field`); `resolve_deps`'
  **two** passes (pass 1 status map + pass 2 dep resolution); and
  `render-adr-index.sh`'s scan. A malformed file is **skipped** (the existing
  `|| continue`), so one bad file never blanks the whole board/index. *(Skip, no
  finding — these are not the health-check surface.)*
- **Validators** (`board-checks.sh`, `adr-checks.sh`): a validator that *silently
  skips* the very inconsistency it exists to catch would hide it — so these emit a
  **first-class `malformed-id` finding** instead of a quiet skip. Each scan loop
  compares raw vs. integer:
  ```sh
  raw="$(field "$f" id)"; id="$(int_field "$f" id)"
  if [ -z "$id" ]; then
    [ -n "$raw" ] && emit malformed-id "$raw" "non-integer id '$raw' in $(basename "$f")"
    continue                      # skip the row from arithmetic/keys, exactly as today
  fi
  ```
  `malformed-id` joins the existing warn-only finding set (sorted with the rest;
  escalated to exit 1 only under the existing `--strict`). `adr-checks.sh`'s
  `[ "$id" -gt "$MAXID" ]` is now always fed an integer.
- **Publisher** (`terminal-publish.sh`): it does **not** scan `id:` from frontmatter —
  `--id`/`--adr` are CLI args. The role-appropriate id-validation here is a
  **fail-closed** arg check at parse time (a publish must hard-stop, never silently
  skip, on a bad id):
  ```sh
  case "$ID" in (''|*[!0-9]*) ;; (*) : ;; esac   # validate after parsing, with die() on non-integer
  ```
  (Same for `--adr`.) This keeps the family uniform in *intent* — every script
  validates its id input — while honouring each role.

### 3. Tests

- `tests/test_docket_frontmatter.sh` (or the lib's existing test): `int_field` returns
  the value for `7`/`007`, empty for ``/`abc`/`1.5`/`7x`/`-3`.
- `tests/test_board_checks.sh` + `tests/test_adr_checks.sh`: a fixture with a
  non-integer `id:` ⇒ `has_finding ... malformed-id`; a clean ledger stays silent;
  existing findings unchanged.
- `tests/test_render_board.sh` + `tests/test_render_adr_index.sh`: a malformed-id
  file is skipped (its row absent), the rest of the board/index renders normally.
  **Include a malformed-id file that lands in `archive/`** so the renderer's archive
  path (not just the active scan) is proven to tolerate it — this guards the
  multi-site hardening above.
- `tests/test_terminal_publish.sh`: `--id abc` exits non-zero with a clear diagnostic.

## Out of scope

- How ids are allocated or formatted (still max+1, filename-encoded).
- Any behavioural change beyond rejecting/skipping/flagging a malformed `id:`.
- Validating *other* numeric frontmatter (`depends_on`, `adrs:`, `change:`) — same
  technique would apply, but this change is scoped to `id:` per the stub. (Noted as a
  natural follow-up, not done here.)

## Assumptions (autonomous defaults — the deferred audit trail)

1. **One pure `int_field` helper in the lib; existing `[ -n … ] || continue` guards
   become the uniform skip mechanism.**
   - **Chosen:** empty-on-non-integer accessor. **Rejected:** a validator-style helper
     that emits diagnostics from the lib.
   - **Why:** the lib header guarantees no side effects on source; returning empty
     drops into the established scan-guard pattern with minimal new code and maximal
     uniformity (the stub's stated goal).
   - **Risk if wrong:** low — pure string `case`, fully unit-testable.

2. **Validators emit a first-class `malformed-id` finding; renderers/shared-scan skip
   silently.** *(MOST LIKELY TO NEED REVIEW — scope-expanding)*
   - **Chosen:** new warn-only `malformed-id` finding in `board-checks.sh` /
     `adr-checks.sh`; quiet skip in renderers + `resolve_deps`.
   - **Why:** a health-check that silently skips a malformed id would *hide* the exact
     class of inconsistency it exists to report — so surfacing it is the correct (not
     merely optional) behaviour for the validator role; it is additive and warn-only.
     Renderers are not the health surface, so a silent skip (no blanked board) is right
     there. This is the split the stub itself proposes.
   - **Counter-argument acknowledged:** a new finding id expands the check taxonomy,
     which 0031's sibling change kept deliberately closed. But here the new id is the
     *point* of the hardening for the validator role, not an incidental taxonomy edit.
   - **Risk if wrong:** low and reversible — if the owner prefers guard-only, deleting
     the two `emit malformed-id` lines reverts to a silent skip with no other change.
     **Flagged for the critic.**

3. **`terminal-publish.sh` gets a fail-closed CLI `--id`/`--adr` integer guard, not a
   frontmatter change.**
   - **Chosen:** validate the CLI arg at parse time, `die()` on non-integer.
   - **Why:** it does not read `id:` from frontmatter (verified — `--id` is a CLI arg),
     so the uniform-by-mechanism helper does not apply; a publish must hard-stop on a
     bad id, not skip. Fail-closed matches the script's existing `die()`-on-bad-arg style.
   - **Counter-argument acknowledged:** one could read the stub's "reads the `id:`
     frontmatter field" list literally and treat `terminal-publish.sh` as out of scope
     entirely. Including a minimal arg guard is the more defensive reading of "validate
     numeric id *across the family*"; it is one `case` line, easily dropped if unwanted.
   - **Risk if wrong:** low — additive validation on an input that is always an integer
     today.

4. **Scope stays `id:` only; other numeric fields are a noted follow-up, not done.**
   - **Chosen:** honour the stub's `id:`-only scope.
   - **Risk if wrong:** none — narrower-than-tempting is the conservative choice.

## Open questions resolved at design time

- *Fail-closed vs warn-and-skip?* → Per role (Assumptions 2–3): renderers skip,
  validators flag, publisher fails closed.
- *First-class check vs guard?* → First-class `malformed-id` finding in the validators
  (Assumption 2), guard-only in renderers.
- *Subsumes existing ad-hoc id handling?* → No removal: the existing `|| continue`
  guards remain (now `int_field`-fed); `pad`/arithmetic become guaranteed-integer
  downstream of the guard. Nothing else to subsume.
