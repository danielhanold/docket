---
slug: guard-keyed-on-presence-not-provenance
hook: "A guard that suppresses itself when a source HOLDS a value must instead key on whether that source SUPPLIED the resolved one — under layered precedence the two diverge silently."
topics: [guards, config, precedence]
changes: [168]
created: 2026-07-31
updated: 2026-07-31
promotion_state: candidate
promoted_to:
---

## Apply
Whenever a value is resolved through layers — user config over machine config over a shipped
default — a guard about "where this value came from" has two available predicates that read
identically in the common case and disagree exactly where it matters:

- **presence**: does the lower layer *have* an entry for this key?
- **provenance**: is the lower layer's entry the one that *won*?

Keying on presence means the guard goes quiet the moment the shipped default exists, whether or not
that default is what got emitted. A user override then flows through carrying no warning, and the
guard reports on a value that never applied. The failure is silent in the worst direction: the
louder the shipped coverage gets, the more cases the guard stops checking.

The fix is to have resolution itself record which layer won — a provenance flag set at the
assignment site, not re-derived afterward by re-reading the layers — and key the guard on that.
Re-deriving is the same bug again, one level up.

Pair the fix with a test that asserts **both** halves: that the warning fires, *and* that the
artifact really carries the value the warning is about. A test that only checks the warning passes
just as happily on a false alarm, which is the other way this guard fails.

## War story
- 2026-07-31 (#168, PR #140 — merged) — `sync-agents.sh`'s `warn_fallback_model()` suppressed its
  foreign-model-ID warning when the shipped sidecar *held an entry* for a harness/agent pair, rather
  than when the sidecar *supplied the resolved value*. A user's `agents.default` line outranks the
  sidecar, so the generated wrapper was emitted carrying the foreign ID while the guard stayed
  silent — reporting a shipped default that never applied. Verified concretely:
  `agents.default.status.model: claude-opus-4-8` put `model: claude-opus-4-8` into the **Cursor**
  wrapper with no warning at all. Re-keyed on a `RES_MODEL_FROM_SIDECAR` provenance flag set during
  resolution, and pinned by a test asserting both the warning and the wrapper's actual contents, so
  the guard cannot pass on a false alarm.

  The defect was latent in the original build and became general the moment the sidecar's coverage
  grew: while only three of twelve agents had sidecar entries, the presence predicate happened to
  agree with provenance for the other nine. Completing the block is what exposed it — worth noting
  because it inverts the usual intuition that filling in a config's coverage can only reduce risk.
  See [[decide-and-act-on-the-same-copy]] for the same divergence between what a gate inspects and
  what the action actually uses, and [[commented-default-is-no-default]] for the coverage-completion
  work that surfaced this one.
