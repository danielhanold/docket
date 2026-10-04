<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0520 — Make every published request schema match the JSON file the operation reads](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0520-make-the-published-finalize-request-schemas-match-what-input.md)**
<!-- docket:backlink:end -->

# Published request schemas describe exactly the JSON file an operation reads

Change 520. Groomed 2026-10-04.

## Problem

`docket schema` exists so an agent can build a `--request`/`--input` body from the running binary (ADR-0109). Today the registry's `Request` binding means "the struct the handler decodes **or assembles**" (`operationBindings` doc comment in `internal/app/schema_registry.go`). For an operation that takes both flags and a JSON file, the published request therefore mixes flag values with file keys, or is empty when the file type lives outside `internal/app`. Agents copy the published keys into the file, and the strict decoder (`decodeRequest`, `DisallowUnknownFields`) refuses them. Both 0502-finalize incidents (#518, #519) were this.

Traced on 2026-10-04, seven operations that read a JSON file publish the wrong request:

| Operation | File flag | Published today | File actually accepts (decoded type) |
|---|---|---|---|
| `finalize.block` | `--input` | `BlockRequest`: id, revision, pr_number, attempt, reason, head, report, remedy | `finalizeBlockInput` (cli): report, remedy |
| `finalize.rebase-continue` | `--input` | nothing (`Request: nil`) | `app.ResolverReport` |
| `finalize.rebase-abort` | `--input` | nothing (`Request: nil`) | `app.ResolverReport` |
| `finalize.closeout` | `--input` (optional) | nothing (`Request: nil`) | `closeoutInput` (cli): verification_outcomes, late_findings |
| `finalize.retarget-children` | `--input` | `RetargetChildrenRequest`: id, revision, children | `retargetChildrenInput` (cli): children |
| `change.halt` | `--input` | `HaltRequest`: id, revision, report | `changeHaltInput` (cli): report |
| `pr.publish` | `--body` | `PRPublishRequest`: id, head, title, body | `prBodyRequest` (cli): title, body |

The `--request` operations (`adr.*`, `learning.*`, `change.create|groom|block|defer|unblock|revive|kill`) and `change.reconcile` already bind the type they decode. About twenty flag-only operations (for example `finalize.rebase`, `finalize.merge`, `finalize.clear-block`, `change.claim`, `change.mark-implemented`, `context.*`, `evidence.*`, `run.verify`, `workspace.inspect`) publish their flag values as a request even though they read no JSON file. Their flags are already described by the capability catalog's per-leaf `signature`.

## Rule

**An operation's published request is exactly the authored JSON document it strictly decodes (through `decodeRequest`, whatever the flag is called: `--request`, `--input`, `--body`), or absent when it decodes none.** Scalar flags are described only by the capability catalog's `signature`. Non-JSON file inputs are not requests and stay unpublished: the canonical build-evidence record (`--evidence`, `--record`, read by `readRecordSource`) and `agent.enter`'s plain-text `--request`.

Record this rule in a new ADR that relates to ADR-0109 (it narrows what ADR-0109's "request" covers and is not a reversal). Update the `operationBindings` and `OperationBinding` doc comments to state the rule.

## Design

### One type per JSON file, shared by decoder and registry

For each JSON-reading operation, the CLI decodes into exactly one Go type in `internal/app`, and the registry binds that same type. Move the CLI-local file types (`finalizeBlockInput`, `closeoutInput`, `retargetChildrenInput`, `changeHaltInput`, `prBodyRequest`) into `internal/app`. Reuse an existing app type where its JSON keys already match exactly. For example, `CloseoutNotes` can serve if it gets the matching json tags. `ResolverReport` keeps its name because the resolver agent's text refers to it. The assembled structs (`BlockRequest`, `HaltRequest`, `RetargetChildrenRequest`, `PRPublishRequest`, and the flag-only ones) stay as internal app inputs and are no longer bound. Renaming them is out of scope.

The new file types carry `docket:"required"` tags that agree with their validators, and the per-operation tag/validator agreement test (`TestRequiredTagMatchesValidator`) covers them. Fields that are required in the file (for example `report` and `remedy` for `finalize.block`) are marked required. `finalize.closeout`'s file is optional as a whole, and its keys are not required.

Bind every flag-only operation's `Request` to nil.

### Registering the decoded type on the command

Route every strict JSON-file decode in `internal/cli` through one helper that both decodes the file and records the decoded prototype on the cobra command, for example as an annotation set when the command is built. `decodeRequestFlag` and `decodeInputFlag` already converge on `decodeRequest`, and `pr.publish` calls `decodeRequest` directly. The helper is the single place where a command declares "I read a JSON file of type T from flag F". The exact API is a plan decision, with one constraint: the declaration must come from the same value the decode uses, so they cannot diverge.

### Guard

One guard in `internal/cli` (it needs the real command tree; the registry stays in `internal/app`):

1. Walk the production root command. For every leaf that carries a capability id, read its registered JSON-file prototype (or none) and compare it with the registry binding for that id. Equal types (or both absent) pass. When both are present, also compare `requestJSONKeys` of the prototype with the descriptor keys `docket schema` publishes for the op, so the guard checks the published surface, not just the Go types.
2. A shape-keyed source scan over `internal/cli` non-test Go files fails if `decodeRequest` (or `json.NewDecoder` with `DisallowUnknownFields`) is called anywhere other than inside the registering helper. Key the scan on call shape (an AST walk or a whole-tree grep of call expressions), never on an enumerated list of call sites (AGENTS.md *Guards and tests*).
3. Mutation-test both: rebind one operation to a different type (or nil) and watch check 1 redden; add a stray direct `decodeRequest` call in a scratch copy and watch check 2 redden. Record both mutations in the results file.

The set of JSON-reading operations is derived from the command tree and the helper's registration. It is never hand-listed.

### Existing tests

- `TestEveryRequestAndResultStructIsBound`: keep the result half (forward and reverse) unchanged. Retire the forward request accounting ("every exported `*Request` is bound or excluded"). After this change most `*Request` types are deliberately unbound flag-assembled inputs, and the new guard proves the stronger property that each bound request is the decoded type. In the reverse check, keep "a bound Request is a package-app struct" and drop the `*Request` name-suffix requirement (it is what would force renaming `ResolverReport`). Drop `excludedRequestTypes` along with the forward request accounting.
- `TestFlagAssembledRequestsEmitSnakeCaseKeys`: retire. Its four types are no longer published.
- `TestSchemaRequestAbsentForReadOnlyLeaves`, `TestSchemaCatalogCorrespondence`, `TestSchemaExcludesEnvelopeKeysPerOp`, `TestSchemaRevisionKeys`: update or keep as their assertions require. Do not weaken a still-valid assertion to make the change fit. If one asserts the old "request = assembled struct" meaning for a specific op, replace it with the new rule.
- Re-check the decode order wherever the guard needs it. The guard walks static command metadata, so it should not need to run handlers. If a behavioral probe ends up being used instead, decoding the file before any repository access is an acceptable small reorder.

### Not changed

- The schema document format and `schema_version`: no descriptor shape changes, only binding contents. Docket's own consumers only build `--request`/`--input` bodies from it.
- Which values travel as flags and which as the JSON file.
- Decode refusals. `decodeRequest` already appends the accepted key set to an unknown-field refusal (commit `5405ec128`), so a stray `schema_version` is refused with the right keys named. `rebase-continue` does not accept or special-case an envelope-level `schema_version`.
- The `docket-rebase-resolver` agent text, which already lists the report's exact fields.
- `finalize.resolver_max_attempts`, the rebase replay mechanics, change 0360's schema items, and the separate finalize fixes #515 and #517.

## Verification

- `docket schema --operation <id> --json` for each of the seven operations prints exactly the keys in the table's right-hand column, and each flag-only operation prints no request.
- Feeding each operation a file built only from its published request keys is accepted by the decoder (a decode-level test per operation, no repository needed).
- The guard and its two recorded mutations as above.
- The full suite passes through the configured `build.test_command`.
