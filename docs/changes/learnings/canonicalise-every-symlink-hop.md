---
slug: canonicalise-every-symlink-hop
hook: "A path identity check that trusts a symlink target's spelling is not an identity check — canonicalise every hop, or one physical file answers to two unequal names."
topics: [shell, portability, filesystem]
changes: [242]
created: 2026-08-08
updated: 2026-08-08
promotion_state: candidate
promoted_to:
---

## Apply

Whenever two paths are compared to decide *"is this the same file?"* — a dedupe `seen` set, a
"already processed" guard, a provenance check — resolve each path to its physical location by
canonicalising **every hop**, not by reading one link's target and trusting the string it holds.
An absolute symlink target is still a *spelling*: it can itself traverse another symlink, and the
result compares unequal to the same file reached by a different route.

macOS makes this reliably observable — `/tmp` is a symlink to `/private/tmp`, so any test fixture
built under `/tmp` will exercise the two-names-one-file case on the maintainer's machine even when
production paths never would. That is a reason to canonicalise, not a reason to special-case `/tmp`.

Corollary for a dedupe: prove it through the pass where it actually bites. An idempotent writer
makes "wrote the block twice" indistinguishable from "wrote it once", so an assert aimed at the
write pass is green with the dedupe deleted — the observable difference shows up in the strip /
removal pass.

## War story

- 2026-08-08 (#242, PR #186) — `sync-agents.sh`'s `resolve_physical_path` had an absolute-symlink
  branch that trusted the link target's spelling rather than canonicalising each hop. With
  `/tmp -> /private/tmp` in play, one physical file answered to two non-equal names, the `seen`
  dedupe silently failed, and the repo would have carried a second managed block forever. Fixed by
  re-canonicalising every hop, with a named assert that reverting just that hop reddens. The same
  task recorded the honest negative above: the write-pass dedupe is not independently observable
  because `ensure_managed_block` is idempotent — the STRIP pass is where the test bites.
