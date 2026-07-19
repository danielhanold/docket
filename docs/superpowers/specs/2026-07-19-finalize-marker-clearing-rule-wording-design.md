# Re-phrase the `## Finalize blocked` clearing rule around what it actually guards

**Change:** 0099 · **Date:** 2026-07-19 · **Status:** settled (maintainer-decided 2026-07-19)

## Context

Change 0087 introduced the `## Finalize blocked` marker. Its clearing rule, at
`skills/docket-finalize-change/SKILL.md:162`, currently reads:

> **A successful finalize removes the section.** Unlike auto-groom's human-only re-arm, the
> condition is machine-verifiable (the gate passed), so requiring a human to delete it would strand
> stale markers on changes that are fine. State encoded by an artifact's presence must be cleared by
> every transition out of that state.

The stub proposed re-phrasing this around the invariant **"the marker never survives into `done`"**,
as a wording-only edit. `docket-auto-groom` abstained on it, because that proposed invariant is
**not true against the running code**.

**The verified finding.** Nothing strips the section on the way to `done`. `scripts/archive-change.sh`
— the shared terminal-transition primitive that both finalize and the `docket-status` sweep call —
only `git mv`s the file and sets frontmatter scalars (`status`, `updated`, `claimed_at`, optional
`results`), plus **appends** `## Why killed` on a kill (`archive-change.sh:106`). It never removes a
body section. So on the out-of-band path — a human merges the PR by hand, the sweep archives it
silently (finalize SKILL §`## Finalize blocked`: an already-merged PR "is archived **regardless of
the marker**") — the section rides verbatim into `archive/`.

The stub's premise ("describes a cleanup whose result nothing can see") is correct, but for the
**opposite reason** it assumed: the cleanup is invisible not because the section is reliably gone,
but because **nothing reads it at `done`**. `render-board.sh`'s `finalize_blocked()` is documented
and implemented as meaningful only for an `implemented` change (`lib/docket-frontmatter.sh:107-112`),
and the auto-detect selection skip applies only to unmerged candidates. A `done` change therefore
never renders as finalize-blocked, present section or not.

## Decision

**Option (a) — re-phrase around the property that is actually true. Wording only, no behavior change.**

The load-bearing sentence to fix is the third one: *"State encoded by an artifact's presence must be
cleared by every transition out of that state."* Read literally, that universal **demands**
strip-on-archive — which is precisely why the current text "invites a reader to implement redundant
clearing logic." Sentences one and two are true and stay.

### The edit

Keep: **"A successful finalize removes the section."** plus its machine-verifiable-condition
rationale. Replace the over-broad closing universal with the scoped truth — that **every** reader of
the marker is scoped to a pre-`done` change, so the archive/status transition retires the marker's
meaning on its own, whether or not the section is physically present.

**Phrase the property, not a roster** (reconcile, 2026-07-19). The replacement sentence must not
hard-code a count or list of readers. At authoring time there were two (the board's
`implemented`-only cell; the auto-detect selection skip on unmerged candidates); change 0098 landed
a third the same day — `stale-finalize-blocked` in `scripts/board-checks.sh`, also `implemented`-only
— which is exactly the staleness a roster invites. Readers may be named parenthetically as
illustration; the load-bearing claim is the scoping property they share.

The finalize-path removal is therefore a **real cleanup on the live path** (an `implemented` change
that stays `implemented` must not keep a stale needs-you cell), **not** a guarantee about archived
files.

### Explicitly decided against — record so this is not re-derived as an open thread

Both were considered on 2026-07-19 and rejected by the maintainer. Neither is deferred pending
design; both are **decided**.

1. **Strip-on-archive** (the stub's option (b) — make "never survives into `done`" literally true by
   removing the section at close-out). Rejected because: it destroys the record of *why* a change
   stalled at the merge gate, which is the interesting history to read later; it buys nothing
   observable, since the board never renders a `done` change as finalize-blocked either way; and it
   would put markdown body surgery into `archive-change.sh`, the shared terminal primitive on every
   terminal transition for every change — a new failure mode (mis-anchored heading, greedy match) on
   the path that can least afford one. Archiving is otherwise append-only in this codebase.

2. **Appending a `## Finalize resolved` note at archive time.** The gap it would close is real: on
   the out-of-band-merge path the archived file reads `status: done` in the frontmatter and
   "finalize blocked — needs you" in the body, with nothing recording that it was resolved or how
   (the narrative lives only in the `gh pr comment` and, if present, the `results:` file). Rejected
   on **comprehension cost**: the marker is *state*, and annotating it on one path turns it into a
   partial audit log that is neither reliably present nor reliably absent — harder to reason about
   than either pole. Making it coherent would also mean *not* clearing it on the successful-finalize
   path, which is a live-path behavior change well beyond a wording fix.

### Reconciling with the `presence-encoded-state` learning

This decision sits in direct tension with a **promotion-candidate** finding
(`learnings/presence-encoded-state.md`, hit on changes 14 and 87): *"When state is encoded by an
artifact's presence, every transition out of that state must remove the artifact."* The rejected
strip-on-archive is exactly what a naive application of that rule demands — so the tension must be
resolved in the text, not left for a future reader to rediscover.

The finding's own design-time enumeration is *"what **re-**enters this state, what leaves it by a
path the system doesn't drive, and what merely **reads** it."* Running it here:

- **Re-enters:** a re-mark. Handled — it replaces rather than appends (0087's review fix).
- **Leaves by a path the system doesn't drive:** the out-of-band human merge. Handled at the
  *reader* — the selection skip is scoped to unmerged candidates, so a hand-merged change is no
  longer stranded.
- **Merely reads it:** the board cell (`implemented`-only), the selection skip (unmerged-only), and
  — since change 0098 — the `stale-finalize-blocked` health check (`implemented`-only). Every reader
  added since has independently landed on the same pre-`done` scoping.

The third leg is the resolution: **the rule's purpose is that no reader is left misinformed, and
removal is the usual means, not the end.** Where every reader has already stopped consulting the
artifact, its presence encodes nothing and removing it is cost without benefit. The finding is
correct and stays as written; this is a case where its enumeration, run honestly, discharges the
obligation without a strip.

## Open questions — resolved

**(1) Does the convention restate the rule?** Yes. `skills/docket-convention/SKILL.md:171` carries
its own copy: *"Cleared automatically by a successful finalize; a human retries a marked change by
**naming its id**, which overrides the skip."* Only the source should carry the phrasing, so the
clearing clause is re-pointed at the finalize skill rather than independently restated. This is safe:
the convention's clearing clause is **not** sentinel-anchored — the only two convention-side asserts
in `test_finalize_disposition.sh:130-133` cover the *auto-detect skip scoping* and the
*naming-the-id retry*, both of which must survive the edit verbatim in meaning.

**(2) Which guards are anchored on the current wording?** Exactly one on the clearing rule:
`tests/test_finalize_disposition.sh:120`, asserting

```
grep -Eqi "(remove|clear)s?.{0,40}section|section.{0,40}(removed|cleared)"
```

Keeping "removes the section" satisfies it with no test edit — which the decision above does anyway
on its own merits. The neighbouring marker asserts (lines 102-125) cover the subsection heading, the
skip scoping, the named-id override, the CONFLICTING-not-marked rule, the board-cell wording, and
the metadata-write phrasing; **none** is touched by re-writing only the closing universal.

Per `learnings/foundational-test-discipline.md`, a green sentinel proves a phrase still *exists*, not
that it is still *true* — the sentinel is not the verification for this change, the read is.

## Scope

**In:** the closing sentence of the clearing-rule bullet in
`skills/docket-finalize-change/SKILL.md`; the clearing clause of the marker entry in
`skills/docket-convention/SKILL.md:171` (re-point, don't restate); a spec-derived note only if the
edit moves sentinel-matched text (it should not).

**Out:** any change to when the marker is written or skipped; strip-on-archive; a resolution note;
`archive-change.sh` in any form; the stale-marker health check (change 0098).

## Verification

1. `bash tests/test_finalize_disposition.sh` — green, **with no test edit**. A required edit here
   means the wording moved further than intended; re-read rather than re-anchoring the sentinel.
2. `bash tests/test_render_board.sh` and `bash tests/test_docket_frontmatter.sh` — green (untouched;
   they cover the board cell and `has_section`).
3. Read the edited bullet end-to-end and confirm no sentence asserts a guarantee about archived
   files, and that the finalize-path removal still reads as a live-path obligation.
4. Confirm no behavior change: `git diff` touches `.md` files under `skills/` only.
