---
slug: external-truth-needs-a-human-checkpoint
hook: "When a value's truth lives outside the repo (a vendor model ID, an external API name), no in-repo test can be its oracle — route it to a named human verification item instead of writing an assert that can only ever pass."
topics: [testing, verification, config]
changes: [184, 192]
created: 2026-08-01
updated: 2026-08-02
promotion_state: candidate
promoted_to:
---

## Apply
Some values a change ships are true or false only against a system the repo cannot see: a vendor
model ID, an external service's enum, a remote path. The repo's tests compare **generated output
against the sidecar that generated it** — both sides move together, so the assert is green whether
the value is correct or a typo. That is not a gap to close with a better assert; it is a structural
property of a mirror test, and writing more asserts around it manufactures the appearance of
coverage over an unverifiable claim.

Recognize the class at plan time by asking **where the oracle lives**:

- **In the repo** — write the guard, mutation-test it ([[guards-are-code]]).
- **Outside the repo** — no guard exists. Name the value in a **human verification item** on the
  change's `results:` file, say exactly what run would certify it, and state why no test can. A
  harness handed an unknown ID typically falls back to a house default **silently**, so the failure
  mode is not an error but a wrong-tier run nobody notices.

The distinguishing question is not "is this value new?" but "is this value new *to the outside
world*?" Re-pointing an ID the repo already ships elsewhere is covered by the existing corpus; a
value with no prior occurrence anywhere in history has never been exercised by anything.

## War story
- 2026-08-01 (#184, PR #147) — The four-tier build-profile ladder introduced
  `cursor-grok-4.5-low`, the one shipped value in the change that was genuinely new: a
  boundary-anchored grep found no occurrence anywhere in the repo's history (the cursor family
  shipped only `-low-fast`, `-medium`, `-high`, `-high-fast`). Per ADR-0015 docket keeps no vendor
  allowlist by design, and every pin assert compares the generated wrapper against the sidecar —
  so the sidecar is both the input and the oracle, and **no test in this repo can ever detect a
  wrong ID**. The resolution was not a test but merge-gate item 2 on the results file, pointing at
  the one run that certifies it (`docs/cursor/validation.md` Phase 7 step 1, which requires an
  explicit `**Build profile:** economy` dispatch that reports its resolved model). Daniel confirmed
  the ID valid the same day. Sibling observation from the same change: a repo-committed rename
  cannot reach `~/.config/docket/config.yml` or a machine-local `.docket.local.yml` either — the
  outside-the-repo boundary cuts on writes as well as reads (see
  [[config-shape-change-strands-outer-layers]]).
- 2026-08-02 (#192, PR #150) — Second hit, and the first where the *whole* shipped table was
  outside-truth: registering opencode meant three brand-new OpenRouter model IDs, none with any
  prior occurrence in repo history. The rule was applied as written rather than rediscovered — the
  IDs were routed to named merge-gate items on the `results:` file ("catalog presence is not
  entitlement — confirm they resolve under your credentials"), alongside live-certification of the
  standard and premium rungs and one real end-to-end dispatch, with the waived set (max rung, the
  three review rungs, classification, escalation) stated explicitly. Worth noting for the class:
  the economy rung *was* certified during the build via `opencode debug agent`, which prints fully
  resolved config — that settles **spelling** questions (it is how `reasoningEffort:` was confirmed)
  but still is not an executed run, so resolved-config evidence and entitlement evidence are two
  different checkpoints.
