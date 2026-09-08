<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0410 — Require durable results artifacts with human testing and coordinator findings](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-08-0410-require-durable-results-artifacts-with-human-testing-and-coo.md)**
<!-- docket:backlink:end -->
# Require durable results artifacts with human testing and coordinator findings — Results

## Outcome

Results are now a required completion artifact in Docket Go. A shared results-content validator
(internal/app/results_content.go) runs in two phases: a lenient checkpoint phase on
change.attach-results and a strict final phase on change.mark-implemented / run.verify. The
canonical results template now carries five sections — Outcome, Human testing, Verification
performed, Findings and limitations, Follow-ups — with optional sections omitted when they have no
substantive content. change.mark-implemented refuses a change with no attached, final-valid results
artifact, and run.verify reports a missing or invalid results artifact as an unmet completion
conjunct. The requirement, the checkpoint lifecycle, evidence sequencing, the single-results-writer
rule, and the frozen-merged-artifact rule are documented across the convention and the
implement-next, build, and review skill prose, and mirrored across the Claude, Codex, Cursor, and
OpenCode harness adapters with regenerated embedded assets. No material departure from the spec.

## Human testing

### Required-results gate accepts legitimate prose and refuses a missing artifact

This confirms end-to-end behavior a human should sanity-check at the merge gate, beyond the Go unit
tests: the mandatory-results gate must not fire on ordinary results prose, and must fire when
results are absent.

1. Author a results artifact whose Findings section legitimately mentions code markers such as
   FIXME or TODO in sentence prose, and includes an inline HTML detail block or a mailto autolink,
   then attach it with change.attach-results.
   Expected: attach succeeds; the token words and inline HTML do not trip the placeholder guard.
2. Attempt change.mark-implemented on a change that has no results artifact attached.
   Expected: it refuses with the missing-results completion reason, and run.verify reports the same
   change as run-incomplete naming the unlinked-results conjunct.

## Verification performed

The full Go suite (go run ./cmd/docket development test, resolved from build.test_command) was
driven through the native gate and passed green at the reviewed head c5f5d6c8; the immutable
build-evidence record was recorded from that run's directory and verified against the head. The
whole-branch review (deep rung) returned zero blockers. The two placeholder false-positive findings
were fixed under the docket-build-task contract with focused table tests proving both directions
(legitimate prose accepted, unfilled scaffold still refused), then the suite was re-run green once
in the fix-loop gate. gofmt and go vet on internal/app were clean.

## Findings and limitations

### Results placeholder detection over-matched legitimate prose (fixed)

The final-phase placeholder check reused the plan-token regex, so the content words TODO, FIXME,
TBD, XXX, TKTK, and PLACEHOLDER matched as whole words anywhere in results prose, and the
angle-bracket heuristic tripped on legitimate inline HTML and non-http autolinks. Both wrongly
refused a now-mandatory gate, and the token case emitted a scaffolding-worded diagnostic that
misdescribed the cause. Fixed in commit c5f5d6c8: results detection now keys on the template's
actual scaffold shape and no longer treats those content words as placeholders; the plan path in
change_attach.go is untouched.

### Residual: uppercase-led inline HTML could still trip the scaffold heuristic

The redesigned results scaffold signal keys on an angle bracket followed by an uppercase ASCII
letter (the template's capitalized instruction phrases). An unconventional uppercase-led HTML tag or
uppercase-scheme autolink in results prose would still be flagged. This is unlikely in practice and
was accepted as a low residual risk rather than expanded into a full HTML parser.

## Follow-ups

### No migration path for in-flight changes predating this gate

Results are required with no grandfather clause, so any change already in-progress or
implemented-but-unmerged when this lands will refuse mark-implemented and report run-incomplete
until a results artifact is authored, backlinked, and attached retroactively. This is the intended
design, not a defect, but the human merging should retrofit results for any affected in-flight
change, or confirm the backlog has none, before relying on the gate. No new change is minted; capture
deliberately with docket change create if the backlog needs a tracked cleanup.
