<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0354 — Halt-report authoring writes a duplicate Run halted heading, wedging docket change resume-halted](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0354-halt-report-authoring-writes-a-duplicate-run-halted-heading.md)**
<!-- docket:backlink:end -->

# Change 0354 — Keep authored halt reports inside one recoverable section

## Purpose

A successful `change.halt` must store exactly one structural `## Run halted` section, with the entire authored report inside it. A later authorized `change.resume-halted` must remove that whole section while preserving surrounding content and existing recovery safeguards.

## Verified context

The source inspected during grooming has `validateHaltShape` checking report presence and size, but not section structure. `changeHaltOp.haltReportBody` adds a dated H3; `render.ApplySectionEdits` supplies the H2 and inserts the supplied report verbatim apart from its existing trailing-newline handling.

`ApplySectionEdits` rejects duplicate owned headings in the source it edits, but does not reject new section headings introduced by replacement Markdown. Its final `document.Parse` is not a uniqueness check. Therefore a report containing another bare halt heading can be written and later refused on resume. A different H2, including a dated halt heading, also escapes the enclosing report section and can survive removal as stray content.

The current implement-next skill describes authoring a halt section without explicitly distinguishing the operation-owned wrapper from the caller-owned body. The historical observation on change 0351 remains evidence of the defect; grooming does not claim that today's skill contains an explicit instruction to duplicate the heading.

Change 0351 is done. Change 0368 is proposed and independently covers recovery before workspace allocation; it is related, not a prerequisite. Change 0343 was killed because the Go section scanner already understands fenced examples. Reuse that behavior. There are no dependencies or stack parent.

## Decision

Fix caller guidance and validate report structure at the halt write boundary. Refuse malformed input without rewriting it.

The alternatives are a caller-only wording fix, which leaves other clients able to store malformed reports, and automatic stripping or demotion, which silently changes authored evidence and needs rules for distinguishing wrappers from intended content. Neither is selected.

This is a bounded halt-report fix. It does not change the general section editor's contract for all operations.

## Report contract

The operation alone owns the canonical `## Run halted` heading and dated H3 wrapper. Callers supply the report body, starting with prose, a list, or a subheading at H3 or deeper. Existing report size and non-empty requirements remain.

Reject every column-zero `## ` heading that the existing section scanner would recognize outside code fences, wherever it appears in the report. This includes bare or dated halt headings and unrelated headings. Match the consumer's syntactic shape rather than an enumerated list of titles.

Literal heading examples inside properly closed backtick or tilde fences remain valid. Inline mentions and blockquotes remain valid under the current section parser's rules. Reject an unterminated fence so an accepted report cannot hide the following existing section from the recovery scanner. Fence recognition, closure, indentation, delimiter length, and line-ending behavior must derive from the existing scanner rather than an independent approximation.

Do not add a second wrapper date or silently strip, rename, demote, or normalize authored content beyond existing newline handling.

## Code and diagnostics

Add a small report-body validation entry point in `internal/render/section.go`, backed by the scanner used by `ApplySectionEdits`. It may expose the scanner's final fence state, but must preserve existing section editing behavior for other callers.

Call it from `validateHaltShape` in `internal/app/change_halt.go` before repository preparation, the transaction engine, or metadata effects. Use the existing protocol result `invalid-input` and finding code `invalid-section-markdown`, with `field: report`. A heading diagnostic explains that the operation owns the H2 and date, and that the caller should use body text or H3 subsections. An unclosed-fence diagnostic asks the caller to close the fence. Diagnostics must not echo report text.

The CLI request format and scalar identity flags stay unchanged. The public app entry point and CLI both receive the validation. Existing empty-report and size findings retain their meaning. Keep the duplicate-owned-heading guard in the section editor and the acknowledgement, exact-version, and workspace checks in resume.

Update the authoritative halt-authoring clause in `skills/docket-implement-next/SKILL.md` and relevant request documentation to state this body-only contract and give a valid request-body example. Search maintained callers for conflicting guidance; fix actual conflicts without expanding frozen historical artifacts. A rejected halt write remains an unsuccessful halt write: the agent reports the failure instead of claiming durable halt state exists.

## Acceptance and verification

- Regression inputs with leading or later bare halt headings, dated halt headings, or arbitrary structural H2 headings return `invalid-input` with the report finding. No metadata file, board, commit, remote ref, lease, workspace, or evidence changes.
- Valid prose, lists, H3 subsections, and heading examples in closed backtick and tilde fences are accepted. Include LF and CRLF cases and scanner-sensitive fence-length cases. Unterminated fences fail without effects.
- Exercise real halt publication followed by authorized resume with an existing owned, quiescent workspace. Verify one structural halt heading, one operation-generated date wrapper, intact report content, successful removal of the complete report, and byte preservation of surrounding sections and retained checkpoints. Place an existing section after the halt section so fence or boundary leakage cannot pass unnoticed.
- Re-halt an already valid halted change and verify replacement leaves one section and can still resume.
- Seed an already-corrupted record with duplicate owned headings and prove existing refusal and no-write behavior remain; this change does not automatically repair historical records.
- Keep an end-to-end CLI assertion for the malformed-input diagnostic and an app-level assertion that validation occurs before repository/engine work.
- Mutation-test removal of the new validation: the malformed-heading regression must fail. Also challenge the fence distinction so fenced examples and unclosed fences exercise their intended branches. Use uncached test execution for mutation checks.
- Run focused tests during development, then the complete suite resolved from `build.test_command` at the build gate. At grooming time it resolves to `go run ./cmd/docket development test`. Follow the repository's budget-finding policy.

## Boundaries

No automatic cleanup of historical records; no weakening of duplicate-section refusal; no change to pre-allocation recovery, dispatch attribution, claim semantics, or the halt/resume lifecycle. No general Markdown parser replacement and no global tightening of every section-edit client.

The operational tradeoff is explicit: malformed caller input must be corrected before a durable halt can be recorded. The actionable refusal and corrected skill guidance make that failure visible while keeping stored records recoverable.
