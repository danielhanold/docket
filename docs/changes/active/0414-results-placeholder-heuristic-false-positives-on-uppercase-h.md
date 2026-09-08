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

Change 0410's isResultsPlaceholderLine decides a line is unfilled template scaffolding by one shape test: after stripping leading whitespace, heading markers, and one list/blockquote marker, does the line start with '<' immediately followed by an uppercase ASCII letter? That deliberately narrow rule is what separates the template's capitalized instruction phrases (<Change title>, <Finding>, <Human action.>) from legitimate inline HTML and autolinks, which are conventionally lowercase. But HTML tag names and URI schemes are case-INSENSITIVE, so an uppercase spelling is legal Markdown. A results line that begins with <BR>, <DETAILS>, <MAILTO:...>, or <HTTPS://...> is therefore indistinguishable from scaffolding, and change.attach-results rejects the artifact with 'the results artifact still carries unfilled authoring placeholder scaffolding'. Residual risk is low - it needs the angle bracket at line start AND an uppercase spelling AND hand-written prose, it fails loudly at attach time rather than silently, and the author workaround is trivial (lowercase the tag, precede it with a word, or wrap it in backticks) - so it was deliberately deferred rather than fixed inside 0410, where a new commit would have invalidated evidence minted at the tested head and forced a full suite re-run.

## What changes

Decide and implement a detection rule that keeps the template's own placeholder phrases caught while not rejecting case-insensitive HTML tags and URI schemes at line start. The design question is the substance of this change and is deliberately open: a case-insensitive known-token exclusion list is the obvious move and is also exactly the enumerated-list-of-spellings shape this repo's guard rules warn against, since the tag left off the list is the one that bites. Alternatives worth weighing include keying on the template's actual emitted phrases, requiring a trailing '>' with interior spaces or sentence-shaped text, or accepting the false positive and instead improving the rejection message so the author immediately knows the workaround. Whatever is chosen needs mutation-proven tests in both directions: the template's scaffolding still rejects, and uppercase-spelled legitimate markup passes.

## Out of scope

The results requirement itself, the five-section template, the checkpoint/final phase split, and change_attach.go's placeholderTokenRE plan vocabulary - all shipped in 0410 and all working as intended. Not a re-litigation of 0410's decision to stop consulting TODO/FIXME/TBD content words in results prose. No migration path for pre-0410 in-flight changes: confirmed unnecessary.
