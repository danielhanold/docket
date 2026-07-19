---
id: 99
slug: finalize-marker-clearing-rule-wording
title: Re-phrase the `## Finalize blocked` clearing rule around what it actually guards
status: proposed
priority: low
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [87, 98]
discovered_from: [87]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The `## Finalize blocked` clearing rule as written has near-zero observable effect. A successful
finalize archives the change, which hides the board cell anyway — so the rule describes a cleanup
whose result nothing can see. The transitions that genuinely needed clearing were the three fixed
during change 0087's review (re-mark replaces rather than appends; an out-of-band human merge
still archives; the `drained`/`halted` boundary).

Stating the rule as **"the marker never survives into `done`"** would describe the property that is
actually load-bearing, rather than a procedural step that is mostly redundant with archiving.

Worth doing because the current wording invites a reader to implement redundant clearing logic, or
to conclude the rule is dead and drop it — and one of those two readings is wrong.

## What changes

Re-phrase the clearing rule in `skills/docket-finalize-change/SKILL.md` (and the convention entry
if it restates it) around the invariant rather than the step. Re-check the marker sentinels: one or
more may be anchored on the old phrasing, and a re-word that silently un-anchors a guard is the
failure mode change 0087 just spent a review catching.

**Wording only — no behavior change intended.** If the re-phrasing turns out to require a behavior
change to be true, that is a finding worth surfacing rather than absorbing.

## Out of scope

- Any change to when the marker is written or skipped.
- The stale-marker health check (change 0098).

## Open questions

- Does the convention's marker entry restate the rule, or only point at the skill? Only the source
  should carry the phrasing.
- Are any existing sentinels anchored on the current wording's specific literals?

## Reconcile log

## Auto-groom blocked

**2026-07-19 — auto-groom abstained (default-biased self-brainstorm).** This stub triggered its own
stated escape hatch. It asks to re-phrase the clearing rule as **"the marker never survives into
`done`"** as a *wording-only, no-behavior-change* edit — but a read of the running code shows that
proposed invariant is **not literally true today**, and the stub itself says: *"If the re-phrasing
turns out to require a behavior change to be true, that is a finding worth surfacing rather than
absorbing."* Surfacing it.

**The finding (verified against code).** Nothing strips the `## Finalize blocked` section on the way
to `done`:
- `scripts/archive-change.sh` (the shared terminal-transition primitive both finalize and the
  `docket-status` sweep call) only `git mv`s the file and sets frontmatter scalars (`status`,
  `updated`, `claimed_at`, optional `results`; appends `## Why killed` on a kill). It **never edits
  body sections**, so a `## Finalize blocked` section rides verbatim into the archived `done` file.
- The out-of-band-merge path makes this concrete: finalize SKILL §"`## Finalize blocked`" states an
  already-merged PR "is archived **regardless of the marker**" via the sweep's silent-archive path —
  and that path is exactly `archive-change.sh`, which does not strip. So a human-merged,
  finalize-blocked change swept to `done` carries the marker into `archive/`.

The stub's premise (the current rule "describes a cleanup whose result nothing can see") is right for
the *opposite* reason it assumes: the cleanup is invisible not because the section is reliably gone,
but because the **board reads the marker only for `implemented`** (`render-board.sh`'s
`implemented_cell` / `finalize_blocked()` are implemented-only), so a `done` change never renders as
finalize-blocked **whether or not the section is physically present**.

**Why this needs a human — the undecidable decision.** Restating a load-bearing invariant of
docket's own marker model in the canonical skill doc (the reference other agents reason from) is not
safely defaultable, because the stub's proposed wording is subtly untrue and the correct replacement
forks:
- **(a) Wording-only, describe what is actually true:** the board reads the marker only for
  `implemented`, so reaching `done` retires the cell by the archive/status transition alone,
  regardless of the section's physical presence. This is true and needs no behavior change — but it
  is a *different* invariant than the one the stub proposed, and asserting it commits the doc to a
  specific semantic reading of the marker model.
- **(b) Keep the stub's exact wording** ("never survives into `done`") and add a **behavior change**
  — strip the section on the archive/close-out path — to make it literally true. The stub declares
  "no behavior change intended" and puts strip-on-archive out of scope, so this contradicts the
  stub's own constraint.
- **Guard-anchoring sub-decision:** `tests/test_finalize_disposition.sh:120` asserts the clearing
  rule via `grep -Eqi "(remove|clear)s?.{0,40}section|section.{0,40}(removed|cleared)"`. Any
  re-word that drops "removes/clears the section" phrasing silently **un-anchors that sentinel** —
  precisely the failure mode the stub (open question 2) and change 0087's review warn about. Keeping
  "removes the section" as belt-and-suspenders (satisfies the sentinel) vs. dropping it (requires a
  co-designed sentinel rewrite) is itself a correctness call on a guard.

**What a human should supply.** Which invariant the maintainer intends the canonical doc to state —
option (a) vs (b) — and, if (a), confirmation that the "removes the section" clause stays (as a
belt-and-suspenders cleanup the *finalize* path still performs) so the sentinel and the intent both
hold. This is docket's own marker semantics, authored in change 0087; the maintainer's reading is
the point of the interactive brainstorm.

**Recommendation (not a kill/defer — those are never autonomous).** This is a legitimate, small,
worthwhile doc change; keep it. Pursue **(a)**: re-phrase around the actually-true load-bearing
property (the board reads the marker for `implemented` only, so archiving retires the cell), keep the
finalize-path "removes the section" clause intact so no guard un-anchors, and update
`test_finalize_disposition.sh`'s sentinel deliberately and in lockstep if the phrasing moves. Do
**not** add strip-on-archive — but if a human decides the physical-strip guarantee is wanted, that is
a separate behavior change (and arguably belongs with change 0098's stale-marker health-check
thinking).
