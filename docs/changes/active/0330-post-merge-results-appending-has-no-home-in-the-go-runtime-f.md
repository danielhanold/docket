---
id: 330
slug: 'post-merge-results-appending-has-no-home-in-the-go-runtime-f'
title: 'Post-merge results appending has no home in the Go runtime — finalize dropped it and change attach-results does not cover it'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-08-19'
updated: '2026-08-19'
depends_on: []
stacked_on:
related: [316]
discovered_from: [316]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The Bash finalize skill carried this at step 2, immediately after verifying the merge landed: "If the change carries a `results:` file, this is the moment to append interactive-verification outcomes and any late findings to it, post-merge." Change 0316's Go rewrite of that skill removed the step, and nothing took it over.

This is not one of 0316's deliberate deferrals. Its *Out of scope* section enumerates what it defers — CI/combined gates, results-only skips, terminal publishing, automatic learning harvest, capture/groom automation, cross-harness routing, skill rebinding, Bash fallback — and post-merge results appending is not among them. It was collateral damage in the Task 18 rewrite, confirmed by re-triaging every failing assertion against that list: 133 of 142 mapped to a deferred or Go-absorbed capability, and this one did not.

The nearest existing verb does not cover it. `docket change attach-results` verifies an authored results record from Git and links it to an **in-progress** change — it is the attach-once seam on the way to `implemented`. What is missing is the ability to *append* to an already-linked results file at close-out, when the change is merged and about to be archived. Two things make that moment distinct: the content only exists after the merge (interactive-verification outcomes, late review findings), and the record is about to become a point-in-time artifact that AGENTS.md forbids rewriting afterwards. Miss the window and the findings have nowhere legitimate to go.

The practical loss is that anything learned during the human's own verification pass — the step between "PR approved" and "archived" — is silently discarded. The results file freezes at whatever the build wrote, and the close-out half of the record is simply absent.

## What changes

Give post-merge results appending an owning verb, most plausibly under `docket change` alongside `attach-results` (for example an `append-results`, or an `attach-results --append` mode). Whatever the shape, it must: accept authored content for a change whose results file is already linked; operate in the window after merge verification and before archival; and land its write as an ordinary transaction so the board and derived views refresh with it, exactly as every other mutating verb does.

Decide explicitly whether the verb accepts free-text authored content or a structured record. The Bash step accepted authored prose, which is why it lived in the skill rather than a script; a structured request would be a change in kind and should be recorded as such if chosen.

Re-add the step to `skills/docket-finalize-change/SKILL.md` once the verb exists, sequenced where the Bash version sat: after the merge is verified, before closeout archives the record. Restore `test_results_artifact`'s assertion (currently failing on `grep -q "append interactive-verification"`) against the new wording rather than the old — the old assertion greps for Bash-era phrasing and should be rewritten to key on the behavior, per AGENTS.md's rule that a guard keys on shape rather than a spelling.

Mutation-test whatever assert lands: strip the step, watch it redden. A test that passes with the capability removed is decoration.

## Out of scope

The capabilities 0316 defers on purpose — terminal publishing, automatic learning harvest, CI/combined gates, results-only skips, skill rebinding. Not a redesign of the results artifact format or of `attach-results`'s existing in-progress semantics. Not the remaining test-migration work on change 0316's branch.
