---
slug: distributed-body-has-no-local-repo
hook: "Inside a skill body that ships into other repos, the reader is a worker in an unknown repo — a sentence that is only true HERE is a defect even when it is locally accurate."
topics: [docs, contracts, skills]
changes: [249]
created: 2026-08-08
updated: 2026-08-08
promotion_state: retained
promoted_to:
---

## Apply
Before committing prose under `skills/`, re-read each sentence as a worker running in a consuming
repo you have never seen. Anything phrased "on this repo…", "usually the full suite", or otherwise
keyed to the local project's shape is false or meaningless there — and worse when the body itself
declares that repository instructions override it. A cheap detection: grep the whole skill tree for
the phrase shape; a construction that appears exactly once anywhere under `skills/` is a strong
smell. The same pass catches the adjacent defect — an aside that contradicts a numbered rule in its
own section, on precisely the question that section exists to settle.

## War story
- 2026-08-08 (#249, PR #178) — the gate-execution pointer paragraph shipped with the aside `on this
  repo, often the full suite`: the only such phrase anywhere under `skills/`, in a contract body
  distributed into consuming repos, and in direct contradiction of step 4 of the same section
  (`Focused, not the whole suite`) with no reconciling sentence. Review finding 2; fixed by dropping
  the aside and scoping the paragraph to a long-running *focused* verification (`449a7a9c`).
