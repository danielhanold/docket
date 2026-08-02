---
slug: shared-resource-keeps-first-owner-assumptions
hook: "When a single-owner resource gains a second owner, the prose and predicates written for the first owner stay valid-looking and become wrong — and single-owner fixtures pass against the old predicate, so nothing goes red."
topics: [docs, testing, guards]
changes: [192]
created: 2026-08-02
updated: 2026-08-02
promotion_state: candidate
promoted_to:
---

## Apply
Widening a resource from one owner to many — one harness's generated block becoming two harnesses'
shared block, one caller's config file becoming several callers' — is usually implemented as a
rename plus a predicate change, and both land cleanly. What does **not** land is everything written
back when the resource had exactly one owner, because none of it is syntactically wrong:

- **Prose that described the single owner's lifecycle.** "De-listing X removes the block" was true
  and is now true only when X is the *last* owner. The new owner's doc gets the correct caveat
  because it is being written now; the **incumbent's** doc is not open in the editor and keeps the
  old sentence.
- **Predicates that named the owner.** A guard or a strip condition keyed on "is X listed" must
  become "is any owner listed." The edit is easy; noticing every site is not.
- **Fixtures that configure one owner.** This is the trap: a single-owner fixture produces
  identical output under the old and the new predicate, so a suite of single-owner fixtures is
  green against code that never learned to share. **The only fixture that discriminates configures
  two owners at once** — and it is exactly the fixture nobody writes, because each owner already
  has its own.

At plan time, when a change makes something shared, enumerate the incumbent's sites explicitly —
its docs, its guards, its strip/teardown path — and add **one multi-owner fixture** asserting the
two properties single-owner fixtures cannot reach: the resource appears exactly once, and removing
one owner while another remains leaves it in place. See [[correspondence-guard-runs-one-way]] for
the same shape in a guard's iteration direction, and
[[consolidation-flattens-caller-variance]] for the inverse move (N restatements collapsing to one).

## War story
- 2026-08-02 (#192, PR #150) — Registering opencode as docket's fourth harness made the committed
  project-root `AGENTS.md` dispatch block **shared** between Codex and opencode
  (`sync_codex_agents_md_dispatch` → `sync_agents_md_dispatch`, stripped only when the last
  dispatch harness is de-listed). The deep-rung review found both halves of this class in the same
  diff: `docs/codex/setup.md` still promised that de-listing Codex removes the block — false the
  moment opencode can also hold it, while the newly written `docs/opencode/setup.md` carried the
  correct "last harness" caveat — and **no test exercised two dispatch harnesses at once**, so the
  central shared-ownership property was unasserted and both new fixtures passed against the old
  codex-only predicate. Neither was a blocker and neither could have gone red; the incumbent's doc
  and the missing two-owner fixture were both invisible to the suite.
