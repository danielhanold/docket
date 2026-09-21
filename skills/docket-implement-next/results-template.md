<!-- results-template.md — REQUIRED close-out artifact for every implemented change (trivial
     included; change 0410). Authored and consolidated by the coordinator in the FEATURE worktree,
     committed on <type>/<slug> at each checkpoint and finally before the implemented transition.
     Written for a mid-level engineer with little knowledge of Docket internals (change 0440):
     lead with what the reader must do and what changed; implementation detail belongs in the
     linked PR, plan, and evidence. Angle-bracket instructions are authoring guidance only —
     remove them from actual artifacts. The Human action statement and Outcome are required at
     finalization; omit any other section, including its subsections, when there is no
     substantive content. The generated docket:backlink block above the title is owned by the
     artifact.backlink operation — never hand-author its markers, and do not add empty header
     fields for unavailable links. Content rules and the checkpoint lifecycle are normative in
     docket-implement-next's Step 6.5. -->
# <Change title> — Results

**Human action:** <Whether human action is needed, in one or two sentences, consistent
with the sections below. During implementation this may say the assessment is pending;
final results give a settled assessment.>

## Outcome

<The original problem, the delivered behavior, and any material departure from the
agreed design — lead with observable effects. Explain unfamiliar Docket concepts when
necessary; include method names, stored fields, or internal identifiers only when they
help the reader understand a consequence or take action.>

## Human actions and testing

### Important — <short name of the action>

<Why this matters, when the action matters, and what remains uncertain if skipped.>

<Prerequisites, setup, and starting state.>

1. <Concrete step.>
   Expected: <Observable result.>
2. <Concrete step.>
   Expected: <Observable result.>

<Cleanup, only when the procedure changes persistent state.>

### Optional — <short name of the walkthrough>

<Why a reader might want this. An optional walkthrough may exercise behavior automated
tests already cover; a routine suite rerun is not a functional walkthrough.>

<Setup, numbered steps with observable expected results, and cleanup — complete enough
to run without reconstructing missing commands or knowing Docket internals.>

## Verification performed

<Concise account of checks actually performed, their outcomes, and links to durable,
accessible evidence. Identify skipped, failed, or incomplete verification explicitly.
No test logs, per-test inventories, or chronological build diary — and never imply the
human checks proposed above were already performed.>

## Known issues and follow-ups

### <Problem or follow-up>

<When it occurs and what the person experiences; its practical impact; whether it is
confirmed or suspected; any available workaround; and the suggested next action, linking
an existing change when available. Keep each entry understandable without following its
technical links. A fixed finding belongs here only when it explains a remaining risk or
a consequential design decision.>
