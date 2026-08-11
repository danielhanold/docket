---
id: 260
slug: tier-finalize-s-in-context-dispatches-and-name-the-push-deni
title: 'Tier finalize''s in-context dispatches and name the push-denial posture'
status: in-progress
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-11
depends_on: []
related: []
discovered_from: [139, 100]
adrs: []
spec: docs/superpowers/specs/2026-08-07-tier-finalize-s-in-context-dispatches-and-name-the-push-deni-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/tier-finalize-s-in-context-dispatches-and-name-the-push-deni
claimed_at: 2026-08-11T07:45:08Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-tier-finalize-s-in-context-dispatches-and-name-the-push-deni-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-tier-finalize-s-in-context-dispatches-and-name-the-push-deni-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0139 and #0100's residual (2026-08-07 triage): both are finalize-gate dispatch/denial posture, both land in the same two files (`skills/docket-convention/SKILL.md`'s tier table and `skills/docket-finalize-change/references/gate-failure.md`).

Verified 2026-08-07:

- **Finalize's two dispatches are untiered (#0139).** The 0137 dispatch-capability table still has exactly three rows (A/B/C — convention SKILL.md:88-94); `docket-rebase-resolver` and `docket-integration-repair` match none, because their reports flow back in-context to gate the merge. The gap is machine-pinned as a known deferral: `tests/test_dispatch_capability.sh:195-234` carries a `PENDING_TIER` block asserting "exactly the two knowingly-untiered finalize dispatches" — extending the taxonomy without wiring the sites reddens the suite, by design. Not unsafe today: `gate-failure.md:22` already covers both under abort-and-report. #0139's body argues itself to **halt** ("inline repair by the agent that will then merge its own repair is the same self-approval shape Tier B rejects") — that is the default this change takes, most cheaply as an explicit carve-out preserving finalize's existing abort-and-report rather than a fourth tier.
- **The push-denial residual of #0100.** The 2026-07-26 triage of #0100 narrowed it to: `gate-failure.md:22` names the classifier denial only for "denying the merge itself" — the step-5 `git push --force-with-lease` denial (observed live, halting an autonomous finalize) is not in the abort-and-report enumeration (a lease rejected by a *concurrent push* is listed; a *policy-denied* push is not). The generic remedy shipped as the convention's "Harness-native recovery" section (retry the exact command once through the harness's native approval mechanism); the user-level allow-rule direction was deliberately not taken (`ensure-claude-settings.sh:9-10,:37` keeps force-push guarded — a security-posture decision this change does not reopen).

## What changes

Settled by the linked spec (9 audited assumptions):

- An explicit **carve-out paragraph** (not a fourth tier) in the convention's dispatch-capability section: the two in-context-gating finalize dispatches sit outside the A/B/C taxonomy; when dispatch is genuinely unavailable the posture is finalize's existing **abort-and-report**, and inline substitution is forbidden (self-approval shape). Label literal: `carve-out`.
- Site wiring lands in `gate-failure.md` (blocking-loaded at both dispatch moments), which also gains two new abort-and-report members: dispatch-unavailable, and a policy denial of the post-rebase `--force-with-lease` push (named by noun, never a step number — the old "step 5" now collides with gate-failure's own "gate-step-5" red-suite usage) pointing at the harness-native-recovery remedy; the stale "six distinct abort reasons" count is de-numeralized.
- `test_dispatch_capability.sh`: the two sites become ordinary `check_site` rows; `PENDING_TIER` stays as an **empty-pinned** guard (count 0); the coherence loop skips non-`Tier`-shaped labels with a dedicated both-nouns assert against the convention's carve-out paragraph; floors re-derived. `test_finalize_gate.sh` gains sentinels for the two new enumeration members.

## Out of scope

- Reversing the guarded-force-push settings posture (#0100's residual 2 — deliberate security stance, stands).
- Any change to the rebase-resolver or integration-repair contracts themselves.
