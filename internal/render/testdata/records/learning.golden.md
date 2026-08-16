---
slug: 'cached-runner-serves-a-mutated-tree'
hook: 'A runner cache can serve a stale PASS against a mutated tree; defeat it with -count=1.'
topics: ['testing', 'mutation']
changes: [312]
created: '2026-08-16'
updated: '2026-08-16'
promotion_state: 'retained'
promoted_to:
---

## Apply

Any run whose purpose is to observe a change in outcome must defeat the cache.

## War story

2026-08-16 — a mutation probe reported a fabricated green until -count=1 was added.
