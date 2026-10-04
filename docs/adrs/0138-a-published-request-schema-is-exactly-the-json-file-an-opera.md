---
id: 138
slug: 'a-published-request-schema-is-exactly-the-json-file-an-opera'
title: 'A published request schema is exactly the JSON file an operation decodes'
status: 'Accepted'
date: '2026-10-04'
supersedes: []
reverses: []
relates_to: [109]
change: 520
---

## Context

ADR-0109 made `docket schema` a reflected payload-schema surface. Its registry `Request` binding meant "the struct the handler decodes or assembles", so operations that take scalar flags plus a JSON file published flag values mixed in with the file's keys (finalize.block, change.halt, finalize.retarget-children, pr.publish), or published nothing at all for the file they do decode (finalize.rebase-continue, finalize.rebase-abort, finalize.closeout). Agents built request files from those published keys, and the strict decoder (DisallowUnknownFields) refused them (#518, #519, hit during the 0502 finalize).

## Decision

An operation's published request is exactly the authored JSON document it strictly decodes (via --request, --input, or --body), or absent when it decodes none. Scalar flags are described only by the capability catalog's per-leaf `signature`. Non-JSON file inputs (the build-evidence record via --evidence/--record, and agent.enter's plain-text --request) are not requests.

This is enforced mechanically: every decode goes through one CLI helper (declareJSONFile) that records the decoded type on the command. TestPublishedRequestIsTheDecodedJSONFile asserts the registry binding equals that type, keys included, and an AST scan bars decodes that bypass the helper. The flag-assembled *Request structs remain internal app inputs and are no longer bound in the registry.

This narrows ADR-0109's notion of "request"; it is not a reversal. schema_version is unchanged.

## Consequences

About 23 flag-only operations now publish no request schema; their inputs are discoverable through the catalog signature only. Published request keys can be copied into a request file verbatim and will decode. The registry request-accounting test is retired in favor of the stronger binding-equals-decoded-type guard. A new JSON-file input must route through declareJSONFile or the AST guard fails.

## Alternatives considered

Keep binding the assembled structs and document which keys belong in the file: rejected, because agents copy published keys and documentation does not stop that. Accept or ignore stray keys such as schema_version in the decoder: rejected, because it loosens strict decoding and hides real mistakes.
