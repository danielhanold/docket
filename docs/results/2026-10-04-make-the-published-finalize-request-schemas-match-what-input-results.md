<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0520 — Make every published request schema match the JSON file the operation reads](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0520-make-the-published-finalize-request-schemas-match-what-input.md)**
<!-- docket:backlink:end -->
# Make every published request schema match the JSON file the operation reads — Results

**Human action:** Assessment pending; the review has not run yet.

## Outcome

`docket schema` now publishes, for each operation, exactly the JSON document that operation reads from its `--request`, `--input`, or `--body` file, and publishes no request at all for operations that take only flags. Before this change, seven operations published the wrong shape (for example `finalize.block` listed eight keys while its `--input` file accepts only `report` and `remedy`, and `finalize.rebase-continue` published nothing), so agents copied keys that the strict decoder then refused.

What changed:

- Each JSON request file now has one Go type in `internal/app` that both the decoder and the schema registry use: `FinalizeBlockInput`, `RetargetChildrenInput`, `ChangeHaltInput`, `PRPublishInput`, and `CloseoutNotes` (which gained JSON tags). `ResolverReport` keeps its name.
- Every strict JSON decode in `internal/cli` goes through one helper, `declareJSONFile`, which also records the decoded type and flag on the command.
- 23 flag-only operations no longer publish a request; their flags stay described by the capability catalog's `signature`.
- New guards: `TestPublishedRequestIsTheDecodedJSONFile` (published request equals the decoded type, keys included) and `TestJSONFileDecodesGoThroughTheRegisteringHelper` (an AST scan that no decode bypasses the helper).

Departure from the spec: the spec named `report` and `remedy` as required keys for `finalize.block`, but its own rule is that required tags must agree with the validators, and `validateBlockShape` requires only `report`. Only `report` is marked required.

## Verification performed

- Guard mutations, each watched red then restored byte-for-byte from a backup and watched green: rebinding `finalize.block` to the old `BlockRequest` and setting `finalize.rebase-continue` to no request both fail the published-request guard; adding a stray `decodeRequest` call to `internal/cli/pr.go` fails the AST scan. The required-tag guard was also mutation-tested by removing the tag from `FinalizeBlockInput.Report`.
- The CloseoutNotes JSON tags change no stored bytes: no receipt marshals a `CloseoutNotes` value directly (receipts store a digest).

## Known issues and follow-ups

- `docs/reference/outcomes.md` says `docket schema --operation <id>` "shows one operation's request and result shape". It is still roughly accurate, since the request appears only when one exists, but a docs pass could say so. Impact is minor wording; confirmed.
