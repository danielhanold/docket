<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0414 — Results placeholder heuristic false-positives on uppercase HTML tags and URI schemes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-23-0414-results-placeholder-heuristic-false-positives-on-uppercase-h.md)**
<!-- docket:backlink:end -->

# Accept valid plans and results without treating prose as placeholders

## Outcome and agreed boundary

Change #0414 combines plan attachment's whole-document token false positives with the existing uppercase HTML/URI results false positives. Attachment verifies identifiable structure; it does not decide whether natural-language instructions are sufficiently complete.

The human explicitly accepted this plan content at attachment:

    ## Error handling
    TODO: decide whether failed requests should retry or stop.

The existing plan-authoring and review process remains responsible for the completeness of that instruction. Do not replace the current word blacklist with sentence classification, additional review stages, or another blocking policy.

These examples must pass the placeholder check:

- Remove the TODO in retry.go and replace it with bounded retry logic.
- The accepted TODO decision sentence above.
- Code examples containing TODO, FIXME, TBD, TKTK, XXX, or PLACEHOLDER.
- Uppercase, lowercase, and mixed-case HTML and URI examples, including <BR>, <DETAILS>, <MAILTO:person@example.com>, and <HTTPS://example.com>.

An authored plan section whose entire body is the unquoted token TBD is an unfilled slot and must fail. Other attachment prerequisites still apply to every example.

## Investigation and existing contracts

The implementation was traced at main 3bc2475c11adb0e9e605433fbd48cb1c2864a667.

- Change 0315 introduced placeholderTokenRE in internal/app/change_attach.go. changeAttach applies it to every byte of the verified committed plan blob, after Git identity and backlink checks. It cannot distinguish a missing step from an instruction to remove a source-code TODO.
- Change 0410 initially reused this rule for results, then removed it after review demonstrated false positives. Its spec and results distinguish structural validation from coordinator judgment. The replacement isResultsScaffoldBody accepts only two leading characters as its signal: an angle bracket and an uppercase letter. That is the original #0414 defect.
- Change 0440 revised the results template and reused this predicate for the required Human action statement. Both that statement and the ordinary results scan must use the corrected detector.
- ValidateResultsContent already uses internal/document.Parse, managed-block spans, fenced-line handling, heading/body analysis, and exact whole-section filler checks. Preserve those boundaries and reuse the applicable helpers.
- internal/assets.Open already supplies the shipped results template from the binary's embedded bundle. No checkout-local template lookup, new asset distribution, or new dependency is necessary.
- TestResultsTemplateFailsCheckpointValidation already tests the connection between the authoring template and validator; asset tests already check source/embedded parity.
- ADR-0094 owns the Git-verifiable plan artifact and parent-owned attachment. ADR-0018 supports replaceable planning skills, so Docket cannot require every plan to follow the Superpowers template. ADR-0050 and the enumerated-floor learning support deriving a population from its producer rather than hand-maintaining an exception list. The test-premise-deleted-not-regated and section-slice-needs-a-named-terminator learnings apply when narrowing the existing guards.

No reviewed ADR requires the whole-document token blacklist. The authoring requirement for complete plans remains intact; the binary stops pretending that token presence proves incompleteness.

## Design

### 1. Plan validation: reject whole-slot filler, permit prose

Replace the whole-blob placeholderTokenRE check with a focused plan-content check in the existing app attachment path.

Use document parsing and the existing results line/heading machinery as appropriate. Share only the reusable scanning primitives; do not run ResultsPhaseCheckpoint on plans or require results headings, an Outcome section, a Human action statement, or a particular plan directory.

For this bounded change, an unfilled plan slot is a heading's direct body, or the document's pre-heading body, whose entire authored content reduces to one of the existing six placeholder tokens: TBD, TODO, FIXME, TKTK, XXX, PLACEHOLDER. Trim whitespace, compare case-insensitively, and allow one terminal period, following the existing whole-section filler pattern. This is a closed placeholder vocabulary applied to a whole slot, not a search for words inside content.

Exclude frontmatter and generated managed blocks from author-content classification. Fenced and inline code are literal examples: their contents never become tokens or headings for this check. A slot with substantive code or additional prose is not a filler-only slot. Empty sections, custom template instructions, incomplete sentences, and other judgments beyond this narrow check remain with the existing authoring/review process; do not introduce a new plan schema.

Preserve the existing placeholder-token refusal reason for confirmed plan filler. Update its message to identify the unfilled section or body and explain that it contains only a placeholder. Do not suggest that a legitimate mention of TODO must be removed.

### 2. Results validation: derive actual prompts from the shipped template

Extend the existing template/validator connection. Derive the set of reserved authoring prompts from the angle-bracket instruction spans in the embedded skills/docket-implement-next/results-template.md returned by internal/assets.Open.

The derivation reads the canonical template, not the submitted artifact. Exclude the template's HTML comments and literal code examples, and collect complete angle-bracket instruction spans, including wrapped prompts. Normalize internal whitespace so reflow and CRLF versus LF do not change identity. Keep case and punctuation exact; do not invent synonym detection. The current template deliberately uses angle-bracket spans as authoring instructions, including lowercase prompts.

Scan authored results for complete occurrences of those derived prompts outside code, comments, frontmatter, and generated managed blocks. Cover headings, list entries, paragraphs, and inline slots such as “Important — <short name of the action>” and “Expected: <Observable result.>”. An escaped opening angle bracket or a prompt inside inline/fenced code is a literal example and is not scaffolding. Preserve code span delimiter lengths and fence delimiter character/length when masking examples; a shorter embedded run must not close a longer fence. Reuse or narrowly extend the existing scanning machinery rather than introducing a Markdown framework.

Use the same prompt detector for checkpoint scanning and the final Human action statement. Preserve the existing action-statement-empty behavior when that statement is still a template prompt. Preserve final Outcome, empty-section, and existing results filler rules. Do not apply the new plan token-slot rule to results.

Consequences are deliberate:

- Markup and autolinks are accepted regardless of capitalization because they are not emitted authoring prompts.
- A literal exact copy of an emitted prompt must use code formatting or escaping; this distinguishes an example from an unfilled slot.
- Unknown/custom or edited placeholder-looking prose is not guessed from angle brackets or capitalization. Author/reviewer judgment owns that ambiguity.
- No hand-written list of HTML tags, URI schemes, or template phrases is maintained.

Use a small pure extraction/matching helper with an injectable template input for tests. The production path uses the existing embedded asset; it may cache the immutable derived prompts. A missing, malformed, or empty prompt source is a validator setup failure, not successful validation and not an accusation that the user's document is malformed. Return a stable explicit results-template-invalid finding and an explanatory diagnostic without panicking. The shipped-template and embedding guards must catch this before release.

### 3. Why this approach

Simply removing the plan word check would lose the useful ability to catch a section that still contains only TBD. Whole-slot matching retains that protection with the user-approved prose boundary.

Changing capitalization or adding HTML/URI exceptions would preserve the underlying guessing problem. A space-based angle-bracket rule also collides with markup attributes and misses short template slots. The shipped template is concrete evidence of what Docket actually emits; deriving its prompts extends the existing template contract without a second maintained phrase inventory.

A new placeholder marker convention or a general Markdown/HTML parser would add authoring or dependency machinery. Neither is justified when the existing embedded template and focused document handling can distinguish the reported cases. No new ADR is required unless implementation discovers a material departure from these established boundaries.

## Integration and scope

Keep change.attach-plan, change.attach-results, ChangeMarkImplemented, and RunVerify at their existing validation seams. Keep commit identity, tracked regular-file checks, single-artifact delta, path trailer, backlink, exact-version transactions, checkpoint/final phase split, and evidence rules intact.

Update maintained comments and workflow guidance that equate a token mention with an unfinished plan. Preserve the plan writer's obligation to produce a complete plan: the accepted ambiguous example demonstrates attachment's limit, not recommended plan quality. Regenerate embedded assets through the existing generation path if shipped guidance changes.

Do not rewrite historical plans/results, introduce a configuration switch, add a review round, or expand the work into general Markdown conformance. Preserve existing section semantics outside the scanner corrections required for safe literal-example handling.

## Acceptance and verification

Extend existing unit, attachment integration, results phase, and shipped-template contract tests.

1. A valid committed plan containing the retry.go instruction attaches.
2. The exact agreed Error handling / TODO decision example attaches.
3. Each of the six tokens is permitted in substantive prose and code. A filler-only plan slot is rejected; identical text inside code is not.
4. Heading-shaped text and shorter backtick runs inside fenced examples do not create false sections or expose code tokens to validation. Include tilde fences, inline code, CRLF, and managed-block coverage.
5. Results containing uppercase and mixed-case tags, attributes, custom tag names, and URI schemes pass the placeholder check at checkpoint and final validation when all other requirements are met. Include an action statement beginning with legitimate markup or an autolink.
6. The raw shipped results template still fails. Independently exercise every current authoring slot by retaining it in an otherwise filled fixture, including lowercase, wrapped, inline Expected, title, and Human action slots. Derive the slot population from the authored template with coverage that detects an omitted slot; do not rely solely on the same production extractor as its own oracle.
7. Inline/fenced/escaped literal copies of template prompts pass; an unescaped emitted prompt still fails when surrounded by otherwise substantive content.
8. Missing or unusable template input produces the setup finding. The production embedded template is nonempty and equivalent to the authored source under the existing asset guards.
9. Existing malformed-document, backlink, identity, Outcome, action-statement, and checkpoint/final tests continue to prove their respective guarantees.
10. Mutation-prove the new rejection guard by disabling it and observing the filler/scaffold tests fail; restore the old token/capitalization checks and observe the valid-prose/markup tests fail. Verify mutations actually applied and restore from a backup without discarding unrelated edits.
11. At the build gate run the whole suite from the exact source checkout through the Go runner, using the resolved build.test_command. Read and act on its budget findings. Grooming itself changes metadata only and does not run the implementation suite.

## Known limits

This is a structural attachment check, not proof that a plan is executable or a results narrative is truthful. Accepting ambiguous TODO prose is intentional and explicitly human-approved. Prompt recognition is tied to the template shipped by the validating binary; arbitrary legacy or third-party template phrases are not a new compatibility registry. Existing authoring and review responsibilities remain the completeness backstop.
