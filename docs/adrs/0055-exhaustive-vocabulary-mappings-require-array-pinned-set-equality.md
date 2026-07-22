---
id: 55
slug: exhaustive-vocabulary-mappings-require-array-pinned-set-equality
title: Exhaustive vocabulary mappings require array-pinned set equality
status: Accepted
date: 2026-07-22
supersedes: []
reverses: []
relates_to: [49, 50]
change: 116
---

## Context

Docket has several `case` mappings over lifecycle statuses and other closed vocabularies. A `case`
is the right shape when each member maps to different output, but a missing arm often falls through
silently. Conversely, some visually similar cases are intentionally sparse: `readiness_label` maps
only statuses that own readiness, and `suffix_for` maps only the status that needs a suffix. Syntax
alone cannot distinguish an exhaustive mapping from a sparse one.

Some closed vocabularies also begin without a declared array. Treating “no array exists” as “the
mapping is sparse” would leave every un-arrayed exhaustive mapping unguardable; change 0111's
check-id vocabulary demonstrated the real middle case by adding `BOARD_CHECK_IDS` before pinning
its mirrors.

## Decision

Classify mappings by semantic intent:

1. A mapping intended to be exhaustive over a named array is pinned by exact set equality between
   its arms and that array, with an independent extractor-cardinality assertion before comparison.
2. A mapping intended to be exhaustive over a closed vocabulary that has no array gets a single
   authoritative array first, then the same cardinality and set-equality guard.
3. A mapping intended to be sparse, with absence or a default carrying defined behavior, remains
   sparse and is not forced to enumerate the vocabulary.

Every exhaustive correspondence guard is mutation-tested in both directions: removing a real arm
and adding a phantom arm must each make the guard red. Whether a mapping is exhaustive is recorded
beside its guard; it is never inferred from the presence or absence of a `*)` arm.

## Consequences

Adding or retiring a vocabulary member makes every exhaustive mapping that needs attention fail
loudly, while correct sparse/default mappings avoid meaningless maintenance arms. Test extractors
become code with their own non-vacuity obligations and mutation evidence. The tradeoff is an
explicit semantic classification for each mapping and, for previously un-arrayed vocabularies, a
new authoritative array that must itself be guarded for order/cardinality where those matter.
