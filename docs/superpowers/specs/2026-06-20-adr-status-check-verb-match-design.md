# ADR status-consistency check — verb-aware back-pointer matching — design

**Change:** 0031 — match the supersede/reverse verb, not just the target id
**Date:** 2026-06-20 · **Status:** draft (auto-groom) · **Depends on:** 0030 (done)
**Authoring mode:** autonomous (docket-auto-groom). The `## Assumptions` block is the
deferred audit trail and the critic's attack surface.

## Problem

`scripts/adr-checks.sh` arm (b) of `adr-status-inconsistent` verifies that when
ADR-X declares `supersedes: [Y]` / `reverses: [Y]`, the old ADR-Y's `status:` was
flipped to point back at X. It compares only the **target id** (via `status_target`,
which strips the verb), not the **verb**. So ADR-Y reading `Superseded by ADR-X`
when it should read `Reversed by ADR-X` (right id, wrong verb) yields `back == id`
and passes silently — a real ledger inconsistency the check morally should catch.
0030 reproduced the original prose faithfully ("flipped to point back"), which never
required verb-matching; closing the gap is the deferred enrichment.

Current arm (b) (lines ~80–87) iterates `${SUPS[$id]} ${REVS[$id]}` **merged**, so
the edge's own verb is lost by the time the back-pointer is checked.

## Decision

Make arm (b) **verb-aware** by splitting the merged loop into a `supersedes` pass
and a `reverses` pass, each of which knows its edge verb and asserts the target's
status carries **both** the right id **and** the matching verb.

1. **Add a tolerant `status_verb` helper** (sibling of the existing `status_target`),
   so verb extraction is padding-/format-tolerant exactly as id extraction already is:
   ```sh
   # status_verb STATUS -> "supersedes" | "reverses" | "" (the edge a back-pointer status implies)
   status_verb(){
     case "$1" in
       "Superseded by ADR-"*) printf 'supersedes' ;;
       "Reversed by ADR-"*)   printf 'reverses' ;;
       *) printf '' ;;
     esac
   }
   ```
2. **Split arm (b)** into two passes; each flags when id OR verb is wrong:
   ```sh
   for ref in ${SUPS[$id]}; do
     [ -n "${EXISTS[$ref]:-}" ] || continue            # dangling already flagged
     back="$(status_target "${STATUS[$ref]}")"
     verb="$(status_verb "${STATUS[$ref]}")"
     if [ "$back" != "$id" ] || [ "$verb" != "supersedes" ]; then
       emit adr-status-inconsistent "$ref" \
         "ADR-$(pad "$id") supersedes it but its status is '${STATUS[$ref]}' (expected 'Superseded by ADR-$(pad "$id")')"
     fi
   done
   for ref in ${REVS[$id]}; do
     [ -n "${EXISTS[$ref]:-}" ] || continue
     back="$(status_target "${STATUS[$ref]}")"
     verb="$(status_verb "${STATUS[$ref]}")"
     if [ "$back" != "$id" ] || [ "$verb" != "reverses" ]; then
       emit adr-status-inconsistent "$ref" \
         "ADR-$(pad "$id") reverses it but its status is '${STATUS[$ref]}' (expected 'Reversed by ADR-$(pad "$id")')"
     fi
   done
   ```
   This keeps the **same `check-id`** (`adr-status-inconsistent`) and the same emit
   shape; only the message gains the expected back-pointer, and the verb-mismatch
   case now fires.

### Why this shape

- **Tolerant id+verb, not full-string equality.** Comparing the whole status string
  (`"Superseded by ADR-0002"`) would risk false positives on id-padding variance
  (`ADR-2` vs `ADR-0002`). Reusing `status_target` for the id and the new
  `status_verb` for the verb mirrors how arm (a)/(b) already parse ids tolerantly.
- **One check-id, not a new taxonomy entry.** The stub's *Out of scope* keeps the
  other checks and arm (a) unchanged; a wrong-verb back-pointer is the same *kind*
  of finding (`adr-status-inconsistent`), so it folds in — naming the expected
  back-pointer in the message is enough to distinguish it for a reader.
- **Behaviour preserved for existing cases.** A correct flip (`back == id` and verb
  matches) stays silent; a missing/wrong-id back-pointer (`back != id`) still fires
  exactly as before. Only the previously-silent right-id/wrong-verb case changes.

## Tests (extend `tests/test_adr_checks.sh`, reuse `mkadr`/`has_finding`)

- **NEW — supersedes edge, wrong verb on target:** ADR-2 `supersedes: [1]`, ADR-1
  status `Reversed by ADR-0002` ⇒ `has_finding adr-status-inconsistent 1` (was silent).
- **NEW — reverses edge, wrong verb on target:** ADR-2 `reverses: [1]`, ADR-1 status
  `Superseded by ADR-0002` ⇒ `has_finding adr-status-inconsistent 1`.
- **CONTROL (stay green) — reverses edge, correct verb:** ADR-2 `reverses: [1]`,
  ADR-1 `Reversed by ADR-0002` ⇒ NOT flagged. (The existing supersedes-correct-flip
  control already covers the supersedes verb.)
- Existing arm-(a), arm-(b) un-flipped, numbering-gap, dangling-link, `--strict`,
  and clean-ledger cases all remain unchanged.

## Out of scope

- `adr-numbering-gap`, `adr-dangling-link`, arm (a) — unchanged.
- How ADRs are authored / how `docket-adr` writes statuses — unchanged.
- Severity: stays warn-only (a finding only fails CI under the existing `--strict`).

## Assumptions (autonomous defaults — the deferred audit trail)

1. **Fold the verb-mismatch into the existing `adr-status-inconsistent` check-id;
   name the expected back-pointer in the message — do NOT mint a new check-id.**
   - **Chosen:** fold + enrich message. **Rejected:** a distinct check-id.
   - **Why:** *Out of scope* forbids touching the other checks/taxonomy; a wrong-verb
     back-pointer is the same class of ledger inconsistency; the message already
     carries the offending and expected status, which is enough to diagnose.
   - **Risk if wrong:** trivial — message wording is cosmetic and easily changed.

2. **Match the verb tolerantly (new `status_verb` helper) rather than comparing the
   full status string.**
   - **Chosen:** id (`status_target`) + verb (`status_verb`), both prefix-tolerant.
   - **Why:** full-string compare would false-positive on `ADR-2` vs `ADR-0002`
     padding; tolerant parsing matches the script's existing style.
   - **Risk if wrong:** low — both helpers are pure string `case` matches with tests.

3. **Keep severity warn-only (no new `--strict`/exit-code semantics).**
   - **Chosen:** warn-only, consistent with every other finding. The stub endorses this.
   - **Risk if wrong:** none — `--strict` already escalates *any* finding to exit 1.

4. **Add exactly the two verb-mismatch fixtures + one reverses-correct control;
   reuse `mkadr`/`has_finding`.**
   - **Chosen:** minimal fixtures covering both edge verbs' mismatch + the
     previously-uncovered reverses-correct control.
   - **Why:** smallest set that pins the new behaviour and guards the no-regression
     boundary; the harness already has the helpers.
   - **Risk if wrong:** low — tests are additive; existing assertions stay.

## Open questions resolved at design time

- *Distinct finding message vs fold?* → Fold (Assumption 1); name the expected verb.
- *Severity?* → warn-only (Assumption 3).
