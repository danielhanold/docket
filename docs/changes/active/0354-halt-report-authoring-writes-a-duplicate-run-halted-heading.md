---
id: 354
slug: 'halt-report-authoring-writes-a-duplicate-run-halted-heading'
title: 'Halt-report authoring writes a duplicate Run halted heading, wedging docket change resume-halted'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-08-26'
updated: '2026-08-26'
depends_on: []
stacked_on:
related: []
discovered_from: [351]
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

When a docket-implement-next run halts, the report is recorded by `docket change halt` (internal/app/change_halt.go), which writes a wrapper section: a `## Run halted` H2 heading, then a dated `### <date>` H3, then the caller-authored report body. But docket-implement-next authors that body starting with its OWN `## Run halted` H2 heading. The stored record therefore ends up with TWO `## Run halted` H2 sections.

This is not merely cosmetic — it wedges recovery. `docket change resume-halted` removes the marker through `render.ApplySectionEdits` (internal/render/section.go), which carries a duplicate-owned-heading guard: it refuses the ENTIRE edit set when an owned heading appears more than once ("owned heading \"## Run halted\" appears N times; sections must be unique"). So `resume-halted` fails with `marker-remove-failed`, and a halted change cannot be resumed through the sanctioned path until a human hand-collapses the duplicate heading.

Observed live on change 0351 (2026-08-26): the halted record carried two `## Run halted` H2 sections; `resume-halted` would have refused. It was hand-fixed (collapse to a single `## Run halted` / `### <date>` / body section) and pushed so the change could be resumed. Every halted change is exposed to this until the authoring path is fixed.

## What changes

Make the halt-report write produce exactly one `## Run halted` section. Candidate approaches (choose during brainstorm):

- The authored halt-report body (docket-implement-next's halt request file) should NOT carry its own `## Run halted` H2 — `docket change halt` already supplies the wrapper heading and the dated H3. Fix the skill's halt-report template/instruction so the body starts at the report content, not a repeated heading.
- AND/OR make `docket change halt` defensive at the write boundary: detect a leading `## Run halted` (or a dated `## Run halted — <date>` variant) in the supplied body and either strip it or refuse the write with a clear diagnostic, so a malformed body can never land a duplicate owned section (mirrors docket's validate-at-the-write-boundary discipline).
- Add a regression test: author a halt whose body begins with `## Run halted`, assert the stored record has exactly one such H2, and assert `resume-halted` then succeeds on it (the guard no longer trips).

## Out of scope

- The operator tool-choice issue that triggered 0351's halt in the first place (invoking docket-implement-next through the raw Agent/Task tool instead of the intended Skill-fork / slash-command path). That was NOT a docket defect; it is why the earlier stub 0353 was killed.
- Any broader redesign of the halt/resume state machine beyond making the `## Run halted` section singular and keeping resume-halted unwedged.
