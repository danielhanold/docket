---
id: 66
slug: docket-owns-the-review-role-suite-runs-in-the-build-gate
title: Docket owns the review role — read-only rungs, and the suite runs in the build gate
status: Accepted
date: 2026-08-01
supersedes: []
reverses: []
relates_to: [12, 24, 63]
change: 170
---

## Context

ADR-0063 moved the build role in-house (`docket-build` plus its profile-routed workers) and
deliberately preserved exactly one independent review boundary: `docket-implement-next` Step 6's
`skills.review` role, still bound to `superpowers:requesting-code-review`. That left docket's only
remaining review boundary outside its control — session model and effort, recursive reviewer and fix
subagents, verbose reports — and, critically, that skill's checklist re-runs the full suite on the
exact branch state the build gate had just tested.

With a suite of roughly ten minutes, one change was paying for three full runs: the build gate's,
the reviewer's duplicate, and `docket-finalize-change`'s post-rebase run. Two of the three were
answering a question that had already been answered at the same SHA.

## Decision

Docket owns its review role, and the full suite's implementation-phase home is the build gate. The
three parts are inseparable.

1. **A docket-owned, read-only review role.** `skills/docket-review/SKILL.md` is a bounded
   whole-branch reviewer behind three pinned rung wrappers — `docket-review-lean`,
   `docket-review-standard`, `docket-review-deep`. The rung is selected **deterministically, one
   above the build**: take the highest profile any task routed or escalated to (an escalation counts
   as the tier escalated *to*), map `economy`→lean, `standard`→standard, `premium`/`max`→deep, and
   bump one step (capped at deep) when the whole-branch diff exceeds 1500 changed lines — the only
   selection signal independent of the build's own self-assessment. A build skill that emits no
   record at all (including the shipped SDD default) defaults to `docket-review-standard`, matching
   the uncertainty sink `standard` is in docket-build's routing. The reviewer returns
   severity-tiered findings and **never fixes, never dispatches, never commits, and never runs the
   suite**. Its read-only purity is what makes the verdict trustworthy: it has no incentive to
   under-report what it would otherwise have to fix.

2. **The full suite belongs to the build gate, not the reviewer.** The suite answers the *build's*
   question — "does what I assembled work together?" — while review asks "is this good?". The repair
   machinery already lives on the build side, so a suite inside a reviewer forbidden to fix would
   have to hand failures back out and re-enter build machinery, recreating the build→review→build
   loop this design exists to kill. Gate-first is also cheaper on failure. The mental model: **the
   suite is the boundary between build and review, owned by the side that can fix a failure.**

3. **A marker-bounded build-evidence record carries the certification forward.** The gate mints
   `<!-- docket:build-evidence:start -->`…`:end -->` (`command`, `result: green`, `head_sha`,
   `ran_at`) **on green only**. The reviewer verifies it rather than re-running, returning an
   `unverified-build-state` blocker when it is missing, malformed, or stale (the one blocker
   `docket-implement-next` resolves by re-running the suite itself, never by a worker task). Step 7
   writes the record into the PR body, and `docket-finalize-change` skips its post-rebase local run
   only when the rebase was a no-op **and** the record is green at the exact HEAD being merged.

## Consequences

- **Suite runs per change: one** when review is clean and the base has not moved; **two** when
  either a blocker fix lands or the rebase actually moves the branch; **three** only when both
  happen. (An earlier draft of this reasoning claimed "never three" and was factually wrong — the
  whole-branch review caught it.)
- **The PR body is trusted input to a merge-gating decision.** Anyone who can edit a PR description
  can assert a green record at the current SHA. That exposure is nil for a single maintainer and
  real for a repo with outside contributors. The PR body was still chosen because finalize is a
  cross-session, cross-machine consumer, and `.superpowers/` checkpoint files are gitignored,
  transient, and opt-in.
- **Any post-gate commit invalidates the record** — including the Step 6.5 `results:` file committed
  on the feature branch, which roughly 73% of this repo's own archived changes carry. The predicate
  then fails toward running, which is safe, but it means the one-run path is narrower in practice
  than the headline suggests. A docs-only ancestor exemption was considered and deliberately
  deferred as separate design work rather than weakening the invariant here.
- Rung selection is a rule over the build record, not model judgment (ADR-0012's script-vs-model
  boundary applied to a selection decision), and every dispatch stays foreground — a forked reviewer
  has no channel to receive a completion notification (ADR-0024).
- The shipped cross-harness default for `skills.review` stays `superpowers:requesting-code-review`,
  so users who do nothing see no behavior change; this repo dogfoods `docket-review` through its
  committed `.docket.yml`.

This ADR is the twin of **ADR-0063**: build in-house there, review in-house here, with the test
suite placed on the build side of the seam between them.

## Update — 2026-08-02 (change 0193)

The Consequences above state that the shipped cross-harness default for `skills.review` "stays
`superpowers:requesting-code-review`, so users who do nothing see no behavior change", with this
repo dogfooding `docket-review` through its committed `.docket.yml`. That consequence no longer
holds: change 0193 flipped the built-in default for `skills.review` to `docket-review` and removed
this repo's `skills:` pin, so the repo now dogfoods the shipped default itself.

The **Decision** is unchanged and unreversed — the read-only rung topology, the deterministic
one-above-the-build rung selection, and the build-evidence record that keeps the suite in the build
gate are all untouched. Only the rollout posture changed: the conservative default was a
first-release hedge pending evidence, and `docket-review` had by then been exercised enough on this
repo to be trusted as the default. Setting `skills.review` explicitly at any config layer remains
the escape hatch.

