---
id: 520
slug: 'make-the-published-finalize-request-schemas-match-what-input'
title: 'Make every published request schema match the JSON file the operation reads'
status: 'in-progress'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [360, 502]
discovered_from: [518, 519]
adrs: [109]
spec: 'docs/superpowers/specs/2026-10-04-make-the-published-finalize-request-schemas-match-what-input-design.md'
plan: 'docs/superpowers/plans/2026-10-04-make-the-published-finalize-request-schemas-match-what-input.md'
results: 'docs/results/2026-10-04-make-the-published-finalize-request-schemas-match-what-input-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/make-the-published-finalize-request-schemas-match-what-input'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T15:26:53Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-make-the-published-finalize-request-schemas-match-what-input-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-make-the-published-finalize-request-schemas-match-what-input-design.md) |
| Plan | [2026-10-04-make-the-published-finalize-request-schemas-match-what-input.md](https://github.com/danielhanold/docket/blob/fix/make-the-published-finalize-request-schemas-match-what-input/docs/superpowers/plans/2026-10-04-make-the-published-finalize-request-schemas-match-what-input.md) |
| Results | [2026-10-04-make-the-published-finalize-request-schemas-match-what-input-results.md](https://github.com/danielhanold/docket/blob/fix/make-the-published-finalize-request-schemas-match-what-input/docs/results/2026-10-04-make-the-published-finalize-request-schemas-match-what-input-results.md) |
| ADRs | [ADR-0109](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0109-docket-schema-is-a-separate-reflected-payload-schema-surface.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #518 and #519. Both are the same bug: `docket schema` tells an agent the wrong input shape for a finalize operation, so the agent sends keys that `--input` then refuses. Both were seen during the 0502 finalize on 2026-10-04.

- `finalize.rebase-continue` (#518): `internal/app/schema_registry.go` registers it with `Request: nil`, so `docket schema` gives the `docket-rebase-resolver` no request shape to copy. Two resolver reports carried a `schema_version` key, probably copied from the schema envelope. `finalize rebase-continue --input` rejected it, and the controller had to strip the key by hand and re-run the continue step.
- `finalize.block` (#519): `--input` accepts only `report` and `remedy`, and the other six fields (`id`, `revision`, `pr_number`, `attempt`, `reason`, `head`) come from flags. The registry still binds the whole `BlockRequest` struct, so `docket schema --operation finalize.block` lists all eight as request fields.

## What changes

- State one rule for `docket schema`: an operation's published request is exactly the JSON file it reads (`--request`, `--input` or `--body`), or nothing when it reads none. Flags are described only by the capability catalog's `signature`.
- Fix the seven operations that publish the wrong request today: `finalize.block`, `finalize.rebase-continue`, `finalize.rebase-abort`, `finalize.closeout`, `finalize.retarget-children`, `change.halt` and `pr.publish`. Each now publishes the one `internal/app` type its decoder reads.
- Stop publishing a request for the roughly twenty flag-only operations (for example `finalize.rebase`, `change.claim`, `finalize.merge`).
- Route every JSON-file decode through one helper that records the decoded type on the command. Add a mutation-tested guard that each operation's published request equals that recorded type, plus a shape-keyed scan that no decode bypasses the helper.
- Adjust the registry-accounting tests to the new rule, and record the rule in a new ADR that relates to ADR-0109.

## Out of scope

Changing which values are passed as flags versus in the JSON file. Renaming the flag-assembled `*Request` structs. Accepting or special-casing an envelope-level `schema_version` (the refusal already lists the accepted keys). Editing the `docket-rebase-resolver` agent text. Bumping `schema_version`. The resolver attempt budget (`finalize.resolver_max_attempts`) and how the rebase replays commits. The broader CLI schema items bundled in change 0360. The other finalize fixes found during 0502 stay separate on purpose: #515 (a named-id merge skips the repair sign-off block) and #517 (evidence for a finalize re-test).

## Reconcile log

### 2026-10-04

Re-traced against main fc719ac6d (0502 merged). The registry in internal/app/schema_registry.go still binds the seven mismatched operations exactly as the spec's table records, and every strict JSON decode in internal/cli still converges on decodeRequest (decodeRequestFlag, decodeInputFlag, pr.publish's direct call). No scope change.
