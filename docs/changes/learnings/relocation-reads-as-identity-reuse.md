---
slug: relocation-reads-as-identity-reuse
hook: "A uniqueness check keyed on LOCATION misreads a legitimate relocation as an id collision — compare surviving holders by content identity, not by path."
topics: [validation, identity, lifecycle]
changes: [307]
created: 2026-08-14
updated: 2026-08-14
promotion_state: retained
promoted_to:
---

## Apply
When a record's id must be unique across a corpus, the check has to answer "are these two holders
the *same* record?" — and a path answers that question only in a corpus where nothing ever moves.
Docket's corpus moves by design: a change file's whole terminal transition is `active/ → archive/`
under a date-prefixed name. Diff the before/after holder sets by **content identity** (the record's
own `id`/`slug`, or a hash of the record), and treat a holder that disappears from one path and
reappears at another as one surviving holder, not two. The same shape recurs anywhere a
lifecycle transition is implemented as a rename: archive moves, quarantine directories, `.bak`
rotations, staged-to-published promotions.

## War story
- 2026-08-14 (#307, PR #208) — the ADR-evolution validator's identity-reuse check compared holders
  by path, so moving a change file from `active/` to `archive/` presented as two holders of the same
  id and raised a spurious reuse error on every ordinary close-out. Fixed by computing
  `survivingHolders` from content identity: a move is not a reuse.
