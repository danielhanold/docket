<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0520 — Make every published request schema match the JSON file the operation reads](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0520-make-the-published-finalize-request-schemas-match-what-input.md)**
<!-- docket:backlink:end -->
# Make every published request schema match the JSON file the operation reads — Results

**Human action:** No human action is needed before merge. The change is covered by automated guards, and the full suite passed.

## Outcome

`docket schema` now publishes, for each operation, exactly the JSON document that operation reads from its `--request`, `--input`, or `--body` file, and publishes no request at all for operations that take only flags. Before this change, seven operations published the wrong shape (for example `finalize.block` listed eight keys while its `--input` file accepts only `report` and `remedy`, and `finalize.rebase-continue` published nothing), so agents copied keys that the strict decoder then refused.

What changed:

- Each JSON request file now has one Go type in `internal/app` that both the decoder and the schema registry use: `FinalizeBlockInput`, `RetargetChildrenInput`, `ChangeHaltInput`, `PRPublishInput`, and `CloseoutNotes` (which gained JSON tags). `ResolverReport` keeps its name.
- Every strict JSON decode in `internal/cli` goes through one helper, `declareJSONFile`, which also records the decoded type and flag on the command.
- 23 flag-only operations no longer publish a request; their flags stay described by the capability catalog's `signature`.
- New guards: `TestPublishedRequestIsTheDecodedJSONFile` (published request equals the decoded type, keys included) and `TestJSONFileDecodesGoThroughTheRegisteringHelper` (an AST scan that no decode bypasses the helper, keyed on the `encoding/json` import whatever its local name, and covering `json.Unmarshal`).
- `TestRequiredTagMatchesValidator` now fails when a bound request type has neither a case nor a reasoned exemption.
- `ResolverReport.disposition` now publishes its allowed values (`resolver_dispositions`: resolved, stuck). It has no required markers because one type serves both `rebase-continue` and `rebase-abort`, and abort refuses no empty field.
- The rule is recorded as ADR-0138, which narrows what ADR-0109 calls a request.

Departure from the spec: the spec named `report` and `remedy` as required keys for `finalize.block`, but its own rule is that required tags must agree with the validators, and `validateBlockShape` requires only `report`. Only `report` is marked required.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed through the build gate driver after the build. The final gate after the review fixes certifies the PR head; its evidence block is in the PR body.
- Guard mutations, each watched red then restored byte-for-byte from a backup and watched green: rebinding `finalize.block` to the old `BlockRequest` and setting `finalize.rebase-continue` to no request both fail the published-request guard; adding a stray `decodeRequest` call to `internal/cli/pr.go` fails the AST scan; keying the scan back on the literal `json` name or dropping `Unmarshal` fails its own cases; removing a required-tag case fails the new completeness check; misspelling the resolver vocabulary tag fails the vocabulary tests.
- The CloseoutNotes JSON tags change no stored bytes: no receipt marshals a `CloseoutNotes` value directly (receipts store a digest).
- Whole-branch review (deep tier): 4 findings (1 important, 3 minor), all fixed in-branch; the important one was the missing ADR, now ADR-0138. Full table in the PR body.

## Known issues and follow-ups

- **Merge integration test over its serial budget (pre-existing).** The build gate reported `SERIAL CONFIRMED OVER BUDGET` for `tests/test_go_integration_app_merge.sh` (92s solo against a 45s threshold). This change does not touch merge code. It slows every suite run, but nothing fails. Confirmed. Fits #507 (it already tracks a serially over-budget test). Next step: add it to #507's seed list through `docket-groom-next 507` (revise).
- **`docs/reference/outcomes.md` wording.** It says `docket schema --operation <id>` "shows one operation's request and result shape". After this change, flag-only operations show no request, so a reader might expect one. This is minor wording and confirmed. Related to #360 (CLI schema discoverability), but folding it in would change what #360 is for. Next step: a new change a human captures, with #360 under `related:`.
- **Nested required fields on `ADRReplaceRequest`.** `target` and `successor` are refused when empty, but they are not tagged required, and the required-tag test compares top-level keys only. An agent reading the schema cannot tell that they are mandatory. Suspected gap, low impact (the refusal names them). Related to #360. Next step: a new change a human captures, with #360 under `related:`.
