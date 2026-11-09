# Link a change to a ticket — design

Change 0583 — Link a change to a ticket

Change #583, groomed interactively on 2026-11-09. Position 2 of the ticket-integration split planned by change 581; this spec restates every settled decision it carries, so the build needs nothing from 581.

## Intent

Every later ticket capability (create, import, post updates) needs two things this change provides: a durable, typed link from a change to its ticket, and a place for the team's ticket settings. The legacy `issue:` field, an unused leftover of the removed GitHub board mirror, is retired so one word, *ticket*, runs through docket.

A **ticket** is the item in the team's ticket system (a Jira issue, a Linear issue, a GitHub issue, an Azure DevOps work item). A **provider** is which ticket system (`jira`, `linear`, …).

## Goals

1. Three ticket fields on a change, written together by one dedicated operation that links, replaces, and unlinks, in any status.
2. `change.create` accepts the same three fields, so a later import can create a linked change in one step.
3. The `tickets.*` config keys exist, per-repo only, every automatic switch off.
4. implement-next's PR body carries one plain line linking the ticket.
5. `issue:` is gone from every read path.

## Non-goals

- Any connector call or ticket-system call — this change never contacts a ticket system.
- Skills that create, import, or update tickets (changes 584–588). Nothing in this change *consumes* the `tickets.*` keys.
- Any board change.
- A read operation for tickets beyond the implementation context field below (change 585's read operation owns that).
- Rewriting archived records to drop their empty `issue:` values (see below). Linking a ticket to an archived change through the operation is in scope: "any status" includes `done` and `killed`.

## Settled decisions (fixed by 581 — not reopened)

- **Three flat scalar fields:**

  ```yaml
  ticket_provider: jira
  ticket_id: CLOUD-123
  ticket_url: https://acme.atlassian.net/browse/CLOUD-123
  ```

  Flat because the frontmatter value model has no mapping kind (`internal/document/value.go`; the patcher's `classifyShape` treats a mapping as `ShapeUnsupported`).
- **Provider and id are stored, not derived.** They are worked out once while linking with the human present, stay correct if config changes later, and the id gives readable link text with no provider knowledge. `ticket_provider` is a free lowercase token; docket keeps no list of providers.
- **One dedicated operation** writes the three together: revision-pinned, any status, refuses a partial set, clears all three on unlink, re-renders `## Artifacts`. It follows `change.relink`'s pattern (one purpose, any status) rather than `change.groom`, which works only while a change is `proposed`.
- **A partial or inconsistent set** (only a hand edit can produce one) is treated as "not linked" by every consumer and reported as a finding by the read side — visibility only, never a block.
- **`## Artifacts` gains a row:** `| Ticket | [CLOUD-123](<ticket_url>) |`. The board does not change.
- **The PR body gets one plain line linking the ticket**, replacing implement-next's "Best-effort PR→issue reference". A ticket URL is not docket machinery, so the line is allowed in a private repository; like all PR text it is still subject to the leak check.
- **`issue:` is retired:** removed from decode, the domain model, `newChangeBuilder`, repair's field list, and implement-next's Step 7.
- **Config** (below): every `tickets.*` key per-repo only, no master switch, no "enable all" switch, no ticket-type key, `project` generic.
- **Testing:** no ticket-system call is ever made from the docket repository, and docket's own `.docket.yml` never carries a `tickets.*` key — a repository test guards it.

## Decisions made in grooming

### The operation: `change.link-ticket`

A request-file operation (`--request <file> [--repo-dir <dir>]`), registered in the capability catalog and the `schema` registry like `change.block`, `metadata-write` effect.

**Request:**

| Key | Type | Notes |
|---|---|---|
| `change_id` | int, required | |
| `path` | string, required | record path, from `status --json` |
| `revision` | string, required | record blob revision, from `status --json` |
| `ticket_provider` | string | link mode: all three required together |
| `ticket_id` | string | |
| `ticket_url` | string | |
| `replace` | bool | required to overwrite a link to a *different* ticket |
| `unlink` | bool | clears all three; may not be combined with any ticket field or `replace` |

**Modes and dispositions:**

- **Link** (no current link, all three fields given): writes the three fields + `updated:`, re-renders `## Artifacts` and the inline board in one metadata commit (the board's bytes will not change, since the board shows no ticket, but the shared inclusion path decides that, not this op). Disposition `linked`.
- **Same link again** (current link equals all three given values): writes nothing. Disposition `already-linked`.
- **Replace** (a *different* complete link exists): refused with `already-linked-elsewhere` unless `replace: true`; with it, writes as for link. Disposition `replaced`.
- **Unlink** (`unlink: true`): clears all three fields + `updated:`, re-renders as above. Disposition `unlinked`. When the record carries none of the three: `nothing-to-unlink`, writes nothing.
- A record that currently carries a **partial or invalid** set: link requires `replace: true` (it overwrites what is there); unlink clears whatever is present.
- **Any status means any status:** `done` and `killed` records under `archive/` are resolved the same way (the path from `status --json`), and the archived record's `## Artifacts` block is re-rendered in the same commit. No status refusal exists.

**Refusals (write nothing):** `partial-ticket` (one or two of the three fields given), `unlink-with-ticket` (unlink combined with ticket fields or `replace`), `empty-request` (neither fields nor unlink), `invalid-ticket-provider`, `invalid-ticket-id`, `invalid-ticket-url`, `contended` (revision moved), plus the standard planning refusals (unknown change, config preflight).

**Validation** — one domain-level function, shared by `change.link-ticket`, `change.create`, and the decoder's read-side check (below):
- `ticket_provider`: matches the `change_types` token pattern `^[a-z][a-z0-9-]*$`.
- `ticket_id`: non-empty after trimming, single line.
- `ticket_url`: parses as an absolute `http` or `https` URL with a host.

The writer quotes every string value (ADR-0071); callers never pre-quote.

**"Replace" is the human's request, in plain words.** The human never has to say a flag: "switch it to CLOUD-456" or "this is actually CLOUD-456" is the explicit replace request, and the agent sends `replace: true` for it. What an agent must never do is replace a link on its own initiative. The convention's description of the operation says so.

### `change.create`

Accepts `ticket_provider`, `ticket_id`, `ticket_url`: all three or none, the same validation, the same refusal reasons. A new record with a complete set renders the Ticket row in `## Artifacts` at creation.

### Frontmatter placement

The three fields render immediately after `pr:` and before `blocked_by:` in `ChangeRecord` (the slot `issue:` held in old records) — builder's call to adjust if the record-order tests suggest otherwise.

### Archived `issue:` values stay

The four archived records carrying an empty `issue:` (0065, 0066, 0227, 0298) are left untouched. The decoder already treats unknown keys as compatibility data that never fails a decode (`changeWire`'s comment), no check flags them, and archived records are point-in-time history. Once decode drops the field, those values are inert. Nothing rewrites them.

### Read side

- **One notion of "linked".** A change is linked only when all three fields are present **and** pass the shared validation. Anything else that carries any ticket field — a partial set, or a complete set with an invalid value (`ticket_provider: JIRA`, a `javascript:` or relative URL, a multi-line id), which only a hand edit can produce — is "not linked" everywhere: no Ticket row in `## Artifacts`, no `ticket` in the implementation context, no PR line. Grooming call: one rule, so an unvalidated URL can never reach an Artifacts block or a published PR body.
- **The finding.** The decoder reports one warning finding, `ticket-link-invalid`, through the existing decode-finding mechanism (`(*decoder).report`) for either shape. It surfaces in `status --json` findings and in docket-status's report; it never blocks. The finding text names what is wrong (which fields are missing or which value fails which rule), that the change is treated as not linked, and the remedy (`change.link-ticket` with all three and `replace: true`, or `unlink: true`). The code joins the `finding_codes` vocabulary.
- **Typed ticket in the implementation context.** `context.implementation`'s `change.summary` gains an optional `ticket` object (`provider`, `id`, `url`), present only when the change is linked. A skill never scans raw record text for an optional key (learning `frontmatter-anchored-read`).

### PR body line

implement-next Step 7's "Best-effort PR→issue reference" paragraph is replaced, and the "PR-body assembly" paragraph just above it drops its "no `#<issue>` reference" clause (the private-repository rule it states otherwise stands): when the implementation context's `ticket` is present, the authored PR body carries one plain line, `Ticket: [<id>](<url>)`; otherwise nothing. No fallback to any other reference (the `issue:` path is retired, and no record carries a non-empty `issue:`). In a private repository the line still goes in; `pr.publish`'s leak check scans it with the rest of the body. Never a build gate.

### Config: `tickets.*`

```yaml
tickets:
  provider: ""              # which tool (jira, linear, …)
  site: ""                  # where it lives (acme.atlassian.net)
  project: ""               # where new tickets go
  auto:                     # act on its own after each event; all false by default
    change_created: false
    spec_written: false
    pr_opened: false
    pr_merged: false
    results_completed: false
```

- Each key is a fixed registry path (`buildRegistry`; the registry has no user-named map keys), `scopeRepoOnly`: set in the global config or `.docket.local.yml`, it is warned and ignored (ADR-0019's existing fence — no new ADR). In a shared repository it lives in the committed `.docket.yml`; in a private one, `.git/dckt/config.yml`.
- **Normal settings (`dispSupported`), not deferred.** Nothing reads them yet (`provider`/`site`/`project` gain a consumer in change 584, `auto.*` in change 587); setting `auto.pr_opened: true` today simply does nothing. A deferred disposition was rejected: it would make a `true` value block every docket write — a new blocking gate.
- Strings validate as single-line; `auto.*` as bools. Empty strings are the defaults.
- Each key gets its typed field, default, `assemble` assignment, `diagnostic.config` inspection line, and `.docket.example.yml` entry (checked by `example_correspondence_test.go`). The example carries this table as a comment:

| Tool | `project` means | Ticket types |
|---|---|---|
| Jira | project | issue type (Story, Task, Bug) |
| Azure DevOps | project | work item type |
| GitLab | project | issue type |
| GitHub Issues | repository | issue type, where the organization has them |
| Shortcut | team | story type |
| Linear | team | none (labels) |
| Asana / Trello | project / board | none |

### `issue:` retirement — exact sites

Remove `Issue` from `changeWire` and `decodeChange` (`internal/repository/decode.go`), the `ChangeSpec` field and `Issue()` accessor (`internal/domain/entities.go`), `newChangeBuilder` (`internal/domain/actions.go`), and `"issue"` from repair's `changeStringFields` (`internal/reposetup/repair.go`, and its doc comment). Add the three ticket fields everywhere `issue` was, plus `ChangeRecord` (`internal/render/record.go`), which never had `issue`. A whole-repo grep for `issue` in executable code decides the final site list, not this one.

**Folded-in fix (approved in grooming):** `internal/cli/open.go`'s `open` help text still says the export copy's type is "one of issue, spec, plan, or results", but the export kind for a change is now `ticket` (`KindTicket` in `internal/export/export.go`). Change that word to `ticket`. It is a stale-text leftover of the export-kind rename, unrelated to the ticket fields, folded in because the `issue` sweep lands on it.

### Repository guard

A test in `internal/repoguard` reads docket's own committed `.docket.yml` and fails if it declares any `tickets` key. Mutation-tested: add a `tickets:` key, watch it redden.

## Docs and generated surfaces

- The convention's change-manifest sketch gains the three fields (and loses nothing — `issue:` is not in it today); its operation prose names `change.link-ticket` with the "replace is the human's plain-words request" rule. Regenerate embedded twins with `go generate ./internal/assets/`.
- implement-next Step 7 rewritten as above, embedded twin regenerated.
- `.docket.example.yml` documents `tickets.*` with the tool table, written for someone who never heard of this change (learning `config-knob-ship-end-to-end`).

## ADR

Record one ADR at build (via docket-adr): **Ticket fields are three flat stored scalars, and `issue:` is retired** — context: no mapping kind in frontmatter, stored-not-derived rationale, the dead GitHub-mirror field; decision as above; alternatives: a nested `ticket:` mapping (needs a new value kind), deriving provider/id from the URL (breaks when config changes), keeping `issue:` (two words for one thing).

## Acceptance criteria

- `change.link-ticket` writes all three fields and the Ticket row in one commit; `already-linked` writes nothing; a different existing link is refused without `replace: true` and replaced with it; `unlink: true` clears all three; a partial request is refused.
- `change.create` with all three fields creates a linked record with the Ticket row; with one or two it refuses.
- `change.link-ticket` links a `done` change under `archive/` and re-renders its Ticket row.
- A hand-edited partial set, and a hand-edited complete set with an invalid URL, each yield a `ticket-link-invalid` warning in `status --json`, no Ticket row, and no `ticket` in the implementation context.
- `tickets.*` set in the global config or `.docket.local.yml` is warned and ignored; `docket diagnostic config --json` reports each key's effective value.
- implement-next's Step 7 text carries the ticket line and no `issue:` reference.
- No executable code reads `issue:`.
- `docket open --help` names the export type `ticket`, not `issue`.
- The repository guard fails when `.docket.yml` carries a `tickets` key.

## Testing

Repository tests and fixtures only. No ticket-system call is ever made from this repository, and no sandbox site or project name appears in it. There is no live human check for this change: it adds no capability that contacts a ticket system.

## Left to the builder

Go type and file names; exact message wording; the frontmatter slot if tests argue for another; how the operation's transaction reuses `change.relink`'s and `change.block`'s plumbing.
