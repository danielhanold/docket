<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0440 — Make results artifacts readable and actionable](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0440-make-results-artifacts-readable-and-actionable.md)**
<!-- docket:backlink:end -->

# Human-readable results artifacts

Date: 2026-09-21
Status: Approved design

## Purpose

Results should help a mid-level engineer with little knowledge of Docket internals understand what changed, what they need to do, and which problems remain. Implementation detail belongs in linked PRs, plans, and evidence.

## Assumptions

- This applies to future results; historical artifacts remain unchanged.
- Optional functional walkthroughs are useful even when automated coverage exists.
- “Important” identifies a meaningful verification gap or necessary human judgment. It does not introduce a new automated merge requirement.
- The coordinator owns accuracy and readability. Go validates structure.
- Existing ownership, checkpoint, evidence, and post-merge behavior remain intact.

## Content and reading order

**1. Human action statement**

Immediately after the title, every artifact states whether human action is needed. Examples:

> **Human action:** Important verification remains before relying on this feature. Follow the setup and scenario below.

> **Human action:** No required action. An optional walkthrough is included below.

> **Human action:** No required action or additional functional testing identified.

During implementation, the statement can explicitly say the assessment is pending. Final results must give a settled assessment consistent with the rest of the artifact.

**2. Outcome**

Explain the original problem, the delivered behavior, and any material departure from the agreed design. Lead with observable effects.

Explain unfamiliar Docket concepts when necessary. Include method names, stored fields, or internal error identifiers only when they help the reader understand a consequence or take action.

**3. Human actions and testing**

Each item is labeled **Important** or **Optional**, with:

- A reason for doing it.
- Prerequisites, setup, and starting state.
- Concrete steps and observable expected results.
- Cleanup when the procedure changes persistent state.

Important items also explain when the action matters and what remains uncertain if skipped.

Optional walkthroughs may exercise behavior already covered by automation. Instructions must be usable without reconstructing missing commands or knowing Docket internals. Routine suite reruns are not functional walkthroughs.

**4. Verification performed**

Summarize checks actually performed, their outcomes, and anything skipped, failed, or incomplete. Link supporting evidence where it is durable and accessible.

Avoid test logs, individual test inventories, and a chronological build diary. Do not imply that proposed human checks have already been performed.

**5. Known issues and follow-ups**

Combine unresolved findings, limitations, and follow-up work into one section. Each entry explains:

- When the problem occurs and what the person experiences.
- Its practical impact.
- Whether it is confirmed or suspected.
- Any available workaround.
- The suggested next action and existing tracking link, when available.

A problem should remain understandable without following its technical links. Fixed findings belong here only when they explain a remaining risk or consequential design decision.

An in-scope defect remains subject to existing fix, block, or halt rules. Recording a follow-up does not automatically create a change or issue.

The action statement and Outcome are required. Other sections are omitted when they have no substantive content.

## Implementation and validation

Update the shared results template, authoring instructions, convention guidance, and generated copies. Find and reconcile maintained references to the previous sections and the prohibition on manually checking automated behavior.

Extend structural validation to require a substantive action statement near the top. Preserve existing checks for Outcome, unfinished placeholders, and empty sections.

The coordinator checks readability, executable instructions, appropriate importance labels, and consistency between the opening statement and detailed actions. Structural checks do not certify those qualities.

Preserve the existing distinction between incomplete checkpoints and final results. Do not introduce readability scoring, additional review rounds, lifecycle states, configuration settings, or retrospective rewriting.

## Acceptance criteria

- Every final artifact clearly states whether human action is needed.
- A reader can understand the delivered behavior without Docket-internal knowledge.
- Human checks provide usable setup, actions, expected results, and necessary cleanup.
- Optional walkthroughs remain permissible despite automated coverage.
- Unresolved issues explain practical impact and next steps.
- Structural validation rejects a missing or empty action statement.
- Existing checkpoint, ownership, evidence, and historical-artifact guarantees remain intact.
