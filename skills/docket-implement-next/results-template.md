<!-- results-template.md — REQUIRED close-out artifact for every implemented change (trivial
     included; change 0410). Authored and consolidated by the coordinator in the FEATURE worktree,
     committed on <type>/<slug> at each checkpoint and finally before the implemented transition.
     Angle-bracket instructions are authoring guidance only — remove them from actual artifacts.
     Omit an entire optional section, including its subsections, when there is no substantive
     content; Outcome is required at finalization. The generated docket:backlink block above the
     title is owned by the artifact.backlink operation — never hand-author its markers, and do not
     add empty header fields for unavailable links. Content rules and the checkpoint lifecycle are
     normative in docket-implement-next's Step 6.5. -->
# <Change title> — Results

## Outcome

<What was delivered and how the behavior changed.
Explain any material departures from the spec and why.>

## Human testing

### <Functional scenario not covered by automated tests>

<Prerequisites and setup needed for this scenario.>

1. <Human action.>
   Expected: <Observable behavior.>
2. <Human action.>
   Expected: <Observable behavior.>

<Cleanup instructions, only when needed.>

## Verification performed

<Concise account of checks the agents actually performed,
their outcomes, and links to supporting evidence.
Identify skipped, failed, or incomplete verification explicitly.
Do not reproduce test logs or individual automated test cases.>

## Findings and limitations

### <Finding>

<What was observed, supporting evidence, and practical impact.
Distinguish confirmed problems from suspected issues.
Explain any workaround or remaining limitation.>

## Follow-ups

### <Actionable follow-up>

<Problem or opportunity, supporting evidence or reproduction
details, why it falls outside this change, and suggested next action.
Link an existing change when available.>
