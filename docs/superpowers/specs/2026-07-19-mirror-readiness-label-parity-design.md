# GitHub mirror readiness parity — mirror the board's readiness ownership — design

**Change:** #0097 · **Status:** proposed · **Date:** 2026-07-19 · **Related:** #0087 · **Auto-groomed** (default-biased self-brainstorm; assumptions below are the deferred human audit trail)

## 1. Problem

`scripts/github-mirror.sh`'s `readiness_label` early-returns for any change that is not `proposed`
(`[ "$status" = "proposed" ] || return 0`, line 124), so the `finalize-blocked` readiness state
introduced by change #0087 never becomes a `docket:readiness/` label. The inline board renders it
(`implemented_cell` → `finalize blocked — needs you`) and the digest surfaces it
(`digest_readiness` emits `finalize-blocked` for `implemented`), but the mirror shows nothing.
docket now has three projections of the same state that disagree — a maintainer watching the GitHub
Projects board sees nothing while the inline board says `finalize blocked — needs you`.

This is not a regression (the mirror never carried readiness for non-`proposed` changes). The stub's
demand is a **stated rule** about which projections owe readiness, not a one-off patch for a single
state that recurs at the next readiness value.

## 2. Decision

**The mirror adopts the board/digest readiness ownership verbatim — it does not invent its own.**
`render-board.sh` already declares the canonical rule (comments at `digest_readiness`, lines 86–96):
readiness is meaningful for `proposed` (its four tokens: `waiting`, `auto-groom-blocked`,
`needs-brainstorm`, `build-ready`) via `readiness()`, and for `implemented` (`finalize-blocked`) via
`finalize_blocked()`; **every other status has none.** "Readiness has exactly one owner per status."
The mirror's job is to *consume* that owner, not add a second policy.

Concretely, extend `readiness_label(f, status)` in `github-mirror.sh` so that, in addition to the
existing `proposed` branch, an `implemented` change carrying the marker maps to a label:

```
readiness_label(){
  local f="$1" status="$2" id tok
  case "$status" in
    proposed)
      id="$(field "$f" id)"; tok="$(readiness "$f")"
      case "$tok" in
        waiting) ... ;;                                   # unchanged
        auto-groom-blocked) printf 'docket:readiness/auto-groom-blocked' ;;
        needs-brainstorm)   printf 'docket:readiness/needs-brainstorm' ;;
        build-ready)        printf 'docket:readiness/build-ready' ;;
      esac ;;
    implemented)
      finalize_blocked "$f" && printf 'docket:readiness/finalize-blocked' ;;
  esac
}
```

`finalize_blocked` is already available — `github-mirror.sh` sources the same
`lib/docket-frontmatter.sh` (line 80) that defines it and `readiness`. `labels_for` already calls
`readiness_label "$f" "$status"` and emits its non-empty output; no caller change. The new
`docket:readiness/finalize-blocked` label is **self-provisioned** by the existing idempotent
`run_gh label create "$lbl" --color ededed --force` path — no manifest, no registration.

**The stated rule (record it in the change body and a `readiness_label` comment):** the mirror's
readiness labels mirror the board's readiness ownership one-for-one — `proposed` (four tokens) and
`implemented` (finalize-blocked). Any future readiness value is added to `render-board.sh` (the
owner) first, and the mirror follows the same status→owner shape, so the three projections cannot
drift again at the next value.

## 3. Out of scope

- **Two-way mirroring / reading anything back from GitHub.**
- **Changing the inline board or digest readiness rendering** — both already correct; this change
  makes the mirror match them.
- **Label pruning.** The mirror's `--add-label` reconcile is additive by design (line 223 comment:
  "add-label is additive; docket:* only"), so a change leaving the finalize-blocked state does not
  get the stale label removed — identical to how *every* existing derived label (status, priority,
  the four proposed-readiness tokens) already accumulates. See Assumption A3. Making the mirror prune
  stale labels is a separate concern affecting all label families and is not in this change.

## 4. Assumptions (deferred human audit trail)

**A1 — Carry readiness for `finalize-blocked` at all. [chosen: yes]**
Rejected: leave `proposed`-only as a deliberate design and treat the gap as acceptable. Rejected
because two other projections (board, digest) already render `finalize-blocked` for `implemented`
changes; the mirror alone omitting it is the drift the stub calls out. Restoring parity for the one
non-`proposed` state that any projection renders is the conservative, disagreement-removing default.

**A2 — Scope = the board's exact ownership, not "every non-`proposed` status". [chosen: mirror the owner]**
The stub's real question is scope. Rejected: (a) patch only `finalize-blocked` with no stated rule —
recurs at the next value (the stub explicitly forbids this). (b) invent a broader "readiness for all
non-`proposed` statuses" policy — there is no such policy; `render-board.sh` deliberately emits `-`
for every status other than `proposed`/`implemented` because no other status *has* a readiness
notion. Adopting the board's existing per-status ownership is both the narrowest correct rule and the
one that already exists as a single source, so the mirror can never diverge from it by construction.

**A3 — Surface = a `docket:readiness/` label, same as all readiness. [chosen: label]**
The stub asks whether a label is right for an inherently transient state. Rejected: an issue-body
callout or a Projects field for finalize-blocked specifically — that would give this one state a
bespoke surface unlike every other readiness/status/priority label the mirror emits. The mirror
already encodes all mutable derived state as `docket:*` labels re-derived every (best-effort,
one-way, self-healing) pass; finalize-blocked is consistent with that. The transience concern is
real but pre-existing (the `proposed` readiness tokens and status labels are all mutable and all
accumulate under additive `--add-label`), so it is a mirror-wide label-pruning question (out of
scope, §3), not a reason to special-case this state's surface.

**A4 — Dependency state.** `depends_on: []`; no unmet dependency. `related: [87]`,
`discovered_from: [87]` (#0087 is `done`, archived 2026-07-19). No design-ahead gating.

## 5. Test plan (for the builder)

Extend the mirror's test (`tests/test_github_mirror.sh` or the dry-run label assertions) to cover:
- An `implemented` change **with** `## Finalize blocked` ⇒ `labels_for` includes
  `docket:readiness/finalize-blocked`.
- An `implemented` change **without** the marker ⇒ no `docket:readiness/*` label.
- A `proposed` change's existing four readiness-token labels ⇒ unchanged (regression guard).
- Any other status (`in-progress`, `blocked`, `deferred`, `done`, `killed`) ⇒ no
  `docket:readiness/*` label (the status-owner rule).
Use the existing `DRY`/dry-run seam; no live `gh`.
