<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0414 — Results placeholder heuristic false-positives on uppercase HTML tags and URI schemes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-23-0414-results-placeholder-heuristic-false-positives-on-uppercase-h.md)**
<!-- docket:backlink:end -->
# Results placeholder heuristic false-positives on uppercase HTML tags and URI schemes — Results

**Human action:** No required human action before merge. The change is a validator-behavior correction fully covered by automated unit, integration, and contract tests; an optional walkthrough below lets you see the new accept/reject boundary by eye if you want to.

## Outcome

Two Docket validation checks used to reject legitimate authored content as if it were an unfilled template placeholder. This change narrows both so that only genuinely unfilled scaffolding is refused, while real content — instructions, code, uppercase markup, and links — attaches normally.

Plan attachment (`change.attach-plan`) previously scanned the whole committed plan for the words TODO, FIXME, TBD, TKTK, XXX, or PLACEHOLDER anywhere in the document and refused the plan if any appeared. A complete instruction such as "Remove the marker in retry.go", a code example containing one of those words, and the human-approved ambiguous decision sentence "TODO: decide whether failed requests should retry or stop." were all rejected. Attachment now refuses only a **whole-slot placeholder**: a heading's own body, or the document's pre-heading preamble, whose entire authored content reduces to one bare token (case-insensitive, one optional trailing period). A token merely mentioned inside substantive prose or code no longer blocks attachment; whether a section is complete enough stays the responsibility of plan authoring and review, not of the attachment check.

Results validation (`ValidateResultsContent`, used at the checkpoint and final boundaries) previously treated any line beginning with an angle bracket followed by an uppercase letter as leftover template scaffolding. That guessed wrong on legitimate uppercase HTML tags and URI schemes — `<BR>`, `<DETAILS>`, `<MAILTO:...>`, `<HTTPS://...>` — and could not see the template's lowercase prompts at all. The capitalization guess is replaced by matching the **actual authoring prompts derived from the shipped results template** (the embedded `skills/docket-implement-next/results-template.md`): the validator collects that template's angle-bracket instruction spans once and refuses a results artifact only when one of those exact prompts still appears, outside code, comments, frontmatter, and generated managed blocks. Uppercase markup and autolinks now pass regardless of capitalization; an unfilled emitted prompt still refuses; a literal copy of a prompt is accepted only inside code or behind an escaped bracket. If the embedded template is ever missing, malformed, or empty, the validator fails closed with a new stable `results-template-invalid` finding that blames the binary's template rather than silently accepting everything or accusing the author's document.

No new configuration, lifecycle state, ADR, or hand-maintained exception list was introduced; the existing attachment, evidence, and merge seams are unchanged. The plan-token vocabulary still governs plans (where those words mean unfinished work) and is deliberately not consulted for results.

## Human actions and testing

### Optional — see the accept/reject boundary by eye

Automated tests already cover this behavior; this walkthrough is only for a reader who wants to observe the corrected boundary directly. It changes no persistent state.

Prerequisites: this branch checked out; Go toolchain available.

1. Run `go test ./internal/app/ -run 'TestPlanPlaceholderSlot|TestResultsPlaceholder|TestResultsTemplateEverySlotFailsValidation' -count=1 -v`.
   Expected: PASS. Among the cases you can read: a plan section whose entire body is a bare token is refused, while an instruction that merely names a token and the "decide whether failed requests should retry or stop" sentence attach; results carrying uppercase tags/URI schemes pass, while every unfilled prompt of the shipped template still fails.
2. Run `go test ./internal/app/ -run TestResultsTemplateFailsCheckpointValidation -count=1`.
   Expected: PASS — the raw shipped results template still fails validation, because its own emitted prompts are exactly what the derived matcher looks for.

## Verification performed

- Full suite via the resolved build gate command `go run ./cmd/docket development test` at the reviewed head: green — 53/53 test files passed, 427 assertions, ~336s wall. Build evidence recorded and verified against the head.
- After the review fix (sharing the body-reduction helper), the branch was re-certified with the same full-suite command; see the PR build-evidence block for the head and result.
- Whole-branch deep review returned clean (no blocker or important findings). The two minor findings are dispositioned in the PR body: the helper duplication was fixed in-branch; a fail-safe `fenceMask` indentation/frontmatter edge is reported as out-of-scope follow-up (see below).
- TDD with RED/GREEN evidence per task, and two-direction mutation proofs on the new guards: stripping each guard reddens the filler/scaffold tests, and restoring the old whole-word/capitalization rules reddens the acceptance tests. The per-slot contract test uses an independent slot oracle checked for set-equality against the production extractor in both directions, with a fully-filled control so a scanner miss reddens rather than passing vacuously.
- Budget report on the green run showed only `BUDGET WATCH:` screening lines (parallel `-j11`, streak 1/5 each) and no `SERIAL CONFIRMED OVER BUDGET:` line, so there is no authoritative wall-clock breach to act on.

## Known issues and follow-ups

### Custom, non-template placeholder-looking prose is intentionally accepted

By design, results validation now refuses only prompts that are verbatim occurrences of the shipped template's own instructions. An author who invents their own angle-bracket scaffolding that is not an emitted template prompt (for example a made-up `<Finding>` line) now passes checkpoint and final validation, where the retired capitalization heuristic would have refused it. This is the deliberate narrowing agreed in the spec — completeness of authored prose is the author's and reviewer's job, and it is locked by a test so it is not later mistaken for a regression. No action needed.

### Follow-up (out of scope): fenceMask indentation and frontmatter fence handling

`fenceMask` recognizes a code-fence marker at any indentation and runs across the whole line list including any YAML frontmatter, so a stray fence marker inside frontmatter, or a marker indented four or more spaces (which CommonMark treats as indented-code content, not a fence), could toggle fence state. Both deviations only ever cause more lines to be treated as fenced (masked) content, so they fail toward accepting rather than falsely refusing, and results/plan artifacts in practice carry neither deep indentation nor backtick-bearing frontmatter — so there is no observed real-world impact. This is a latent hardening opportunity in machinery this change happens to own, not a defect required for this change's correctness; it was surfaced by review and is reported here for deliberate capture rather than fixed as an unplanned side-quest. Suggested next action, if ever pursued: bound the fence-opener indentation to fewer than four spaces and reset fence state at the end of frontmatter. No existing change tracks this yet.
