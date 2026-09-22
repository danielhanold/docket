---
id: 414
slug: 'results-placeholder-heuristic-false-positives-on-uppercase-h'
title: 'Results placeholder heuristic false-positives on uppercase HTML tags and URI schemes'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-09-08'
updated: '2026-09-08'
depends_on: []
stacked_on:
related: []
discovered_from: [410]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Combined scope, expanded at the human's request on 2026-09-22: valid plans and results are rejected as unfinished placeholders. The original uppercase HTML/URI finding and the newly reported plan-word false positive are one change; no separate change is needed.

Plan attachment scans the entire committed document for the whole words TODO, FIXME, TBD, TKTK, XXX, and PLACEHOLDER. A complete instruction such as “Remove the TODO in retry.go” therefore fails attachment, including when such words occur in code examples. This obstructs valid work and forces authors to rewrite content to satisfy a lexical check. The newly combined issue was assessed as medium severity; the original results-only finding was low severity.

Results validation instead treats an angle bracket followed by an uppercase ASCII letter at the start of a normalized line as unfilled authoring scaffolding. This confuses template instructions such as <Change title> with legitimate uppercase HTML tags and URI schemes such as <BR>, <DETAILS>, <MAILTO:...>, and <HTTPS://...>. Change 0410 deliberately deferred this residual false positive, which originally motivated this stub. Change 0440 now also uses the same scaffold predicate when validating the Human action statement.

## What changes

Groom and deliver one focused correction to placeholder detection in plans and results. Complete instructions, literal examples, valid HTML, and URI autolinks should pass, while actual unfinished authoring scaffolding should continue to fail with an accurate explanation.

Treat “detect unfilled template structures instead of content words” as a design hypothesis, not a settled implementation. Trace the existing implementation and review relevant prior changes and architecture decisions before proposing a design, in interactive grooming or auto-grooming. Prefer extending established machinery; recommend a new mechanism or policy only when concrete evidence shows why the existing approach is insufficient.

Preserve the existing artifact-identity checks and the distinct checkpoint/final results requirements. Cover both results attachment and final completion, including the Human action statement. Define acceptance and rejection examples before choosing the detection rule, and mutation-test the resulting guards in both directions: genuine unfinished content must remain detectable, and legitimate content must not be blocked. Exercise the shipped results template so the validator remains coupled to the actual authoring structure.

## Out of scope

Changes to the requirement for results, checkpoint ownership, plan-writer dispatch, commit/trailer/backlink identity checks, exact-head evidence, or merge policy. No retrospective rewriting of historical artifacts. Do not reintroduce TODO/FIXME/TBD word bans in results. Plan placeholder validation is explicitly in scope, superseding the original stub's exclusion of that rule. New configuration, lifecycle states, parser frameworks, or enumerated HTML-tag/URI-scheme exception lists are not assumed solutions.

## Grooming context

Initial implementation and history trace on 2026-09-22, against main at 3bc2475c11adb0e9e605433fbd48cb1c2864a667:

- `internal/app/change_attach.go`: `placeholderTokenRE` scans the full committed plan blob in `changeAttach`, after Git identity and backlink verification. This check originated in change 0315 (`f4bbb76a`), before change 0410.
- `internal/app/results_content.go`: `ValidateResultsContent` already uses `internal/document` parsing and managed-block spans, plus fence-aware lines. `isResultsPlaceholderLine` and `isResultsScaffoldBody` own scaffold detection. Existing whole-section filler validation distinguishes a filler-only body from a word mentioned in substantive prose.
- Change 0410, its linked spec, and `docs/results/2026-09-08-require-durable-results-artifacts-with-human-testing-and-coo-results.md` document removing the plan-word regex from results and accepting the uppercase-markup residual risk. Change 0440 and its linked spec extend the same results validator to the Human action statement and revise the canonical results template.
- `internal/app/results_contract_test.go`: `TestResultsTemplateFailsCheckpointValidation` already couples validation to `skills/docket-implement-next/results-template.md`; extend this established protection rather than replacing it with a hand-maintained phrase inventory.
- ADR-0094 preserves the Git-verifiable plan artifact and parent-owned attachment boundary. ADR-0018 permits pluggable planning skills, so a design must not assume all plans use one fixed template. ADR-0050 and the enumerated-floor learning caution against replacing an overbroad heuristic with a hand-enumerated exception list.
- Relevant lessons: preserve the property behind a test when replacing its old premise (`test-premise-deleted-not-regated`); respect fenced examples when interpreting document structure (`section-slice-needs-a-named-terminator`); mutation-prove guards (`guards-are-code`).

## Open questions

- What is the precise boundary between an unfinished plan slot (for example, a section containing only TBD) and legitimate prose or an example discussing TODO/FIXME? Should ambiguous prose be left to the existing plan-authoring/review process rather than blocked by attachment?
- Which existing structural parsing and scaffold checks can be shared without imposing the results document's section layout on custom plans?
- What evidence-backed scaffold grammar accepts uppercase HTML/URI examples, recognizes the actual current template instructions, and handles the Human action statement consistently? A new marker convention is an alternative only if the existing representation cannot meet those requirements.
