<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0330 — Optional closeout notes preserve post-merge verification without rewriting frozen results** — `docs/changes/archive/2026-08-21-0330-post-merge-results-appending-has-no-home-in-the-go-runtime-f.md`
<!-- docket:backlink:end -->

# Terminal closeout notes — preserve post-merge verification without rewriting results

**Change:** 0330 · **Date:** 2026-08-21 · **Type:** feat · **Priority:** medium

## Problem

The original Bash finalize workflow told the agent, immediately after verifying a merge, to append
interactive-verification outcomes and late findings to the change's linked `results:` file. Change
0316's Go finalize rewrite removed that instruction without assigning the information another home.
That looks like a lost capability when read against the June-era results workflow, but the repository
has since adopted a stronger artifact rule: once a PR merges, its authored plan and results records
are frozen point-in-time build records. Reintroducing the old append literally would make the results
file say more than the reviewed PR contained and would contradict both the convention and AGENTS.md.

The information is still worth preserving. A human may invoke finalize with verification outcomes
or late findings that were not part of the build record. Today finalize either discards that context
or would need an unsafe, unowned edit to a merged artifact. The Go closeout transaction is the right
mechanical boundary for terminal metadata, but it has no authored-input channel for these notes.

## Decision summary

1. Keep merged `results:` files immutable. Post-merge observations live in the terminal change
   record under `## Closeout notes`, not in the build results artifact.
2. Extend `docket finalize closeout` with one optional structured request carrying exactly two lists:
   `verification_outcomes` and `late_findings`.
3. Do not add a prompt, pause, checkpoint, lifecycle state, or second user step. Finalize uses notes
   only when they were included in the invocation context; otherwise closeout behaves as it does now.
4. Render the notes and the closeout lifecycle mutation in the same transaction. A closeout never
   lands without the notes it accepted, and accepted notes never land outside their closeout.
5. Make the structured content part of closeout's idempotency identity so response-loss retries replay
   safely and a later attempt cannot substitute different prose into a frozen terminal record.

## Request and command surface

`docket finalize closeout` keeps its existing no-input form and gains an optional authored request:

```text
docket finalize closeout --id <id> [--input <request-file>]
```

The request is a JSON object with two optional arrays of strings:

```json
{
  "verification_outcomes": [
    "Production health check passed after deployment"
  ],
  "late_findings": [
    "The upgrade guide should mention the legacy config cleanup"
  ]
}
```

These are the only authored categories. Deployment observations are verification outcomes. Plan
deviations belong in the pre-merge results record. An actionable follow-up should become a tracked
change through the workflow that owns capture; until then its observation may be recorded as a late
finding. This change does not add automatic follow-up creation or learning harvest.

The CLI uses the existing bounded request-file decoder and rejects malformed JSON, unknown fields,
oversized input, and invalid element types. Omitted arrays and empty arrays are equivalent to no
entries. An entry that is empty after trimming is invalid rather than silently dropped. Array order
is preserved. Supplying no input, or supplying two empty arrays, is canonically the same no-notes
request and produces no `## Closeout notes` section.

## Rendering and document ownership

Go, not the caller, owns all Markdown structure. A request containing both categories renders:

```markdown
## Closeout notes

### Verification

- Production health check passed after deployment

### Late findings

- The upgrade guide should mention the legacy config cleanup
```

An empty category omits its `###` subsection. Both empty omits the whole section. Each submitted
string becomes one bullet; multiline continuations are indented so authored text cannot escape the
bullet and create a sibling `##` section. Reserved managed-marker text and control characters that
could corrupt the document are rejected. Inline Markdown remains ordinary authored content.

`## Closeout notes` becomes a documented terminal change-body section in `docket-convention`. It is
written only by finalize closeout and is the final authored body section in the terminal record.
Existing marker-order and balance validation still runs before any managed block is regenerated.
No AGENTS.md change is needed: the design follows, rather than weakens, the frozen-artifact rule.

The section is not a link-bearing artifact and adds no frontmatter field or artifact-table row. It
lives with the archived change on `metadata_branch`; under the existing `terminal_publish` policy it
is published only when the terminal change record itself is published.

## Closeout transaction and lifecycle behavior

The application request passed to `FinalizeCloseout` gains the normalized optional note payload.
Closeout performs its existing fresh metadata, PR, destination, reachability, and stack checks before
opening a transaction. Invalid notes or failed closeout preconditions produce no mutation.

For a change merged to the integration branch, the archive operation adds the notes to the explicit
change, marks the terminal records done, moves them to their UTC-dated archive paths, regenerates
owned artifact blocks and the inline board, and commits the candidate snapshot atomically. Notes
never propagate to carried stacked descendants: they describe only the change explicitly finalized.

For a change merged into an unmerged stack parent, the same payload is written only to that explicit
change in the transaction that marks it `stacked-merged`. The later stack-root closeout preserves the
already-authored section when it archives the child; it neither copies root notes to descendants nor
reopens a child's notes. This keeps one note owner across both closeout dispositions.

The operation remains valid in both repository modes. In docket mode the notes ride the metadata
transaction on `docket`; in main mode they ride that same semantic transaction on `main`. Neither
mode writes the linked results file or introduces an additional integration-ref commit.

## Idempotency and recovery

The normalized two-list payload participates in closeout's canonical digest and durable receipt.
The promise being keyed is the exact terminal record, including its optional notes:

- A retry with the same change identity and identical normalized notes replays the applied receipt.
- No input and an explicitly empty input canonicalize identically.
- A retry carrying different notes is not a replay. Once the record is terminal, it is refused rather
  than treated as permission to rewrite history.
- A response lost after the transaction commits cannot duplicate bullets because the retry adopts
  the existing receipt instead of applying the section again.

Rendering or transaction failure leaves the active source record unchanged. Existing contended,
blocked, and failed closeout dispositions retain their meanings; this change adds input reasons, not
a new lifecycle disposition.

## Finalize skill behavior

`skills/docket-finalize-change/SKILL.md` teaches the invocation contract before its mechanical flow:
the caller may include already-known verification outcomes or late findings in the finalize request.
The skill does not pause after merge and does not ask a new mid-run question. It translates supplied
human prose into the two structured lists, writes a bounded request file, and passes `--input` to
`docket finalize closeout`. When the invocation supplied no notes, it calls the unchanged no-input
form and archives immediately.

This preserves finalize's one-operation experience. It also makes the timing honest: the feature
records context supplied when finalize was invoked; it is not a new post-merge verification gate and
does not claim to capture observations that do not yet exist.

## Components

- `internal/cli/finalize.go` — add the optional `--input` decoder for closeout while preserving the
  existing `--id` interface.
- `internal/app/finalize_closeout.go` — define and validate the structured payload, render the owned
  section, carry it through root and stacked closeout, and bind it into receipts/idempotency.
- Existing document/render helpers — reuse marker-safe section editing; add only a focused terminal
  notes renderer if the current helpers do not provide safe bullet rendering.
- `skills/docket-finalize-change/SKILL.md` — route invocation-supplied notes to the closeout request
  without adding a prompt.
- `skills/docket-convention/SKILL.md` — document `## Closeout notes` as a terminal body section and
  retain the merged-results freeze rule unchanged.
- CLI, application, closeout, and contract tests — cover the semantic and skill wiring described
  below.
- Embedded skill assets and manifest — regenerate mechanically after authored skill changes; never
  hand-edit generated copies.

## Verification

Application and CLI coverage proves:

1. No input preserves current closeout behavior and produces no notes section.
2. Verification-only, findings-only, and both-list requests render the exact documented structure.
3. Empty entries, malformed/unknown input, request-limit violations, control/marker injection, and
   section-escape attempts are rejected with no archive or stacked transition.
4. Notes and lifecycle transition land together in both docket and main repository modes.
5. Root notes do not propagate to carried descendants; an explicitly finalized stacked child keeps
   its own notes through later root archival.
6. A response-loss retry with identical notes does not duplicate content, while different notes
   cannot rewrite the terminal record.

The finalize skill contract test positively locates the closeout portion of the authored skill and
checks the producer-to-consumer shape: invocation-supplied verification/findings are transformed into
the two named request fields, and the resulting request is passed to `docket finalize closeout
--input`. The extractor has a separate non-vacuity assertion. Its mutation probe copies the skill to
a temporary file, confirms the targeted input handoff exists, removes that handoff, confirms the
mutation landed, and requires the same checker to reject the copy.

`tests/test_results_artifact.sh` removes the skipped Bash-era post-merge append assertion. Its premise
is deliberately retired, not re-gated: merged results stay frozen. Coverage for the replacement lives
in the Go closeout tests and the mutation-proven skill contract guard. Go mutation or manual
re-verification runs use `-count=1` so cached results cannot certify a mutated tree.

After regenerating embedded assets, run the configured whole suite. Investigate every trailing
`OVER BUDGET:` report; a green exit does not dismiss a budget finding.

## Out of scope

- Editing, appending, replacing, or redesigning the linked `results:` artifact.
- Changing `docket change attach-results` or its in-progress semantics.
- Adding a post-merge pause, human checkpoint, lifecycle state, or second finalize invocation.
- Automatically creating follow-up changes or restoring change 0316's deferred learning harvest.
- Changing terminal publishing, CI/combined gates, results-only gate skips, skill rebinding, or Bash
  fallback behavior.
- Adding free-form closeout Markdown, a third notes category, or a new link-bearing artifact.
