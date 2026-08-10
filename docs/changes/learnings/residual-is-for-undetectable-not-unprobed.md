---
slug: residual-is-for-undetectable-not-unprobed
hook: "A residual is for what CANNOT be detected, never for what was not probed — an assert that refuses to redden is a finding about the code, so investigate before you write the limitation down."
topics: [residuals, guards, spec, testing]
changes: [282]
created: 2026-08-10
updated: 2026-08-10
promotion_state: candidate
promoted_to:
---

## Apply

Late in a build you write an assert for some branch of the contract and it will not go red. The
mutation lands, the code is gone, the test still passes. The cheap move — and it reads as
*rigorous*, which is what makes it dangerous — is to record it as a **named residual**: *"this
branch's behavior is not separately observable; documented as a limitation."*

**Check which of two things is true before you write that sentence:**

- **Genuinely undetectable** — the branch produces no distinguishable output, exit code, side effect,
  or timing. A residual is correct and the documentation is the deliverable.
- **Indistinguishable *because the code makes it so*** — the branch emits the same token and writes
  the same state as a neighbouring branch. That is not a testing limitation. **That is the finding.**
  The behaviors were supposed to differ and they do not, and the unreddening assert is the only thing
  that was ever going to tell you.

The second case is common precisely where residuals get written: near the end, on an error or
edge-case path, under time pressure, about code nobody will re-read. And a residual is *durable* —
once it is in the contract it is a documented property, so the next reader treats the collapse as
intended and builds on it.

The repair is usually small and it retires the residual instead of shipping it: give the branch the
distinct behavior it was specified to have, then the assert reddens on its own.

**The check, in one question:** *is there no observable difference, or is there no difference?* Only
the first is a residual. Companion to [[guards-are-code]] — that rule says a green mutation means the
guard is decoration; this one says the guard may instead be reporting that the **code** collapsed two
paths, and you must decide which before writing either down.

## War story
- 2026-08-10 (#282, PR #191) — a launch helper's `--stop` verb had a pre-signal re-read (step 3)
  whose asserts would not redden. It was on its way into the contract as a named residual. The honest
  reason turned out to be a real gap: step 3 emitted the same token and wrote the same nothing as
  step 4's absent-group branch **because it omitted step 1's orphan probe**. Adding the probe gave
  step 3 distinct, pinnable behavior — `unavailable` versus `already-terminal` — and the residual was
  retired rather than documented. The spec had already written the rule for its own text and it was
  simply not applied to itself: *"a residual is for what cannot be detected, not for what was not
  probed."* Same change shipped **three** legitimate residuals alongside it, so the discipline was
  not absent — which is the point: the two kinds are hard to tell apart in the moment, and only the
  explicit question separates them.
