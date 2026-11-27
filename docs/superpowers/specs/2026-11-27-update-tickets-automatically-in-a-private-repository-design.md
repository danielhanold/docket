# Update tickets automatically in a private repository — design

Change 0588 — Update tickets automatically in a private repository

Change #588, groomed interactively on 2026-11-26.

## Intent

When a `tickets.auto.<event>` switch is on, the docket workflow that just finished that event posts the update to the change's ticket by itself. In a shared repository that works today. In a private repository it does not: the update posts nothing and the run reports one line.

- **Before:** a PR opens in a private repository with `tickets.auto.pr_opened: true`; the run reports `Not posted to CLOUD-123 (PR opened): automatic updates are not available in a private repository.`
- **After:** the comment is posted with cleaned copies of the spec and results file attached. If the final check finds docket text in what was about to go out (say `#0587` left in the results copy), nothing is posted and the run reports one line naming where.

A private repository's update cannot link its files; it attaches cleaned copies. On request, a person reviews the cleanup before anything is posted. Unattended, nobody does, so something else has to stop a leak: a final check that runs the publish leak check's own patterns over everything about to be posted.

This spec stands alone: every rule the build needs is restated here.

## Words used here

- **ticket**: the item in the team's ticket system (a Jira issue, a Linear issue, …).
- **update**: the short comment posted to a ticket, with what goes with it (links in a shared repository, attachments in a private one).
- **automatic update**: an update a docket workflow posts by itself after an event whose switch is on, with nobody to ask. The other kind is **on request**, when a person asks.
- **the cleanup**: what already turns a docket artifact into a copy fit for outsiders: the `docket export` operation, then `docket-export`'s rewrite pass and word check.
- **the final check**: the leak check this change adds to the automatic private path, run by a new read-only operation, `leak.check`.

## Goals

- An automatic update in a private repository is posted, with cleaned attachments, when nothing in it matches the publish leak check's patterns.
- When anything does match, or the check cannot run, nothing is posted and the run reports one line; the run itself is untouched.
- The final check finds exactly what the publish leak check finds for the same text, because it runs the same code with the same options.

## Non-goals

- Any change to on-request updates. Attended, today's behavior holds: the person sees the before → after of any rewrite, and the post goes out on their go. The final check is not added there.
- Any change to the `docket-export` skill. It gets no unattended mode.
- Widening or narrowing the leak patterns, for ticket posts or for pushes and PRs.

## Anti-goals

- A prompt, question, or approval step on the automatic path.
- Fixing a flagged line and checking again. A hit posts nothing (decision 4).
- A new blocking gate: a hit or a failed check never halts, blocks, or changes the disposition of the run that invoked the update.

## Decisions

### 1. Automatic updates follow the repository's visibility

The automatic path reads `visibility` from its own `context.ticket` read, as it does today, and routes on it:

- `shared`: unchanged. Commit-pinned links, from `references/shared.md`.
- `private`: the private delivery reference, `references/private.md`, under its unattended rules (decision 3).
- **anything else, or no `context.ticket` result:** the visibility is unknown. Treat the repository as private; since nobody can be asked, post nothing and report one line. A `context.ticket` read that does not return `applied` already takes this path through the automatic path's existing rule (a refused or error read posts nothing and reports one line); a value other than `shared` or `private` gets its own line, such as `Not posted to CLOUD-123 (PR opened): the repository's visibility could not be read.`

There is no new flag, no override, and no probe of who can read the repository.

### 2. What the cleanup already does

The automatic private path reuses the on-request private path's cleanup unchanged. For each file the update names (the spec, the results file), `docket-update-ticket` invokes `docket-export` as a **file request** into a fresh temp directory, and `docket-export`:

1. runs the `docket export` operation, which strips the generated docket blocks, the back-link banner, frontmatter, and docket links;
2. rewrites, in the exported copy, every leftover the export reported, every line naming docket or dckt, and every docket change id and ADR number, into what it refers to or out of the text;
3. runs its word check: "docket" and "dckt" as whole words, case-insensitive, code included (the bare word "docket" is exempt when `leak_check.match_word` is `false`).

Then `docket-update-ticket` writes the comment under the private-repository writing rule and checks its own words, fixing its own text quietly, as on request.

### 3. The unattended rules in private delivery

On the automatic path, three things differ from the on-request private path. Everything else in `references/private.md` holds: which files, attachments first and comment last, never a one-call upload assumption, any failed upload posts no comment, the report.

- **The export direction asks nobody.** The `DIRECTED to:` direction to `docket-export` adds: ask nobody; on a word-check hit, return the path and the lines instead of stopping. It is the existing file-request path, not a new mode: the direction at the call site decides the outcome (ADR-0044).
- **No before → after.** The rewrite list is not shown and not reported. The post goes ahead without anyone's go.
- **Two stops, both "post nothing and report one line":** a word-check hit inside an export, and any result from the final check other than clean (decision 4). Nothing is uploaded and no comment is posted in either case.

The final check runs after the comment is written and the attachments are final (including any file renamed by the own-words fix), and before anything is uploaded.

### 4. The final check

`docket-update-ticket` writes the comment's text to a file in the same temp directory as the exports: the words exactly as they will be posted, with real line breaks, in Markdown or the ticket system's own text markup. Never an escaped or structured body such as a JSON document: the matcher reads line by line, and an escaped line break (a literal `\n`) glues the next line onto a letter, which hides a `docket` or `#0587` at the start of that line and every `Docket-` trailer. When the connector takes a structured format, build it from the checked text afterwards, converting the formatting only, never the words (the rule `docket-export` already applies when it posts). Then it runs `leak.check` once over the comment file and every attachment file:

- **`result` is `applied` and `clean` is `true`:** post (attachments first, comment last).
- **`applied` with hits:** post nothing and report one line naming where the check hit (the comment or which attachment, and the rule) and the exported files' paths, so the person can inspect them or ask for the update on request. For example: `Not posted to CLOUD-123 (PR opened): the leak check found a change reference ("#0587") in the results attachment. Exported files: <paths>.`
- **Any other result:** the check did not run. That is never a clean result: post nothing and report one line naming the reason.

There is no retry and no fix-and-recheck. The person asked to keep the rule the earlier design fixed: a hit posts nothing.

**What it catches** — exactly the publish leak check's patterns (ADR-0144), with the same options:

- the word `dckt`, and the bare word `docket` unless `leak_check.match_word` is `false`;
- `docket:`/`dckt:` markers followed by a letter;
- `Docket-<Key>:` trailers at the start of a line;
- `.docket` and `.git/dckt` paths;
- references to ids that exist in docket's change list, active or archived: `#0587`, `(0587)`, `change 0587`, `change #587`, `changes #587`.

**What it deliberately does not catch:** docket ADR numbers, a change id without a leading zero after `#` (`#612`), a bare number with no `#`, parentheses, or "change" before it, and the words "backlog" and "board". The leak check excluded these on purpose: a host repository's own ADR numbers overlap docket's, unpadded references collide with years and PR numbers, and "backlog" and "board" are everyday words in a ticket system. In unattended posts they rely on the cleanup alone, which always rewrites docket's change and ADR numbers. An ADR number that slips through is acceptable: ADRs are common in agent-driven work and reveal nothing about docket.

### 5. The `leak.check` operation

A new read-only operation exposes the leak matcher for arbitrary outbound text. Today `internal/leakscan` is pure and stdlib-only, and its only callers are the publish operations (`workspace.publish`, `pr.publish`, `finalize.publish`, through `runLeakCheck`).

- **Spelling:** capability id `leak.check`, command `docket leak check --file <path> [--file <path> …] [--repo-dir <dir>] [--json]`, effect `read`. "Leak check" is the name docket already uses (`leak_check.match_word`, `leak-detected`, ADR-0144).
- **Input:** one or more files. The check reads the bytes that will be uploaded, not a copy pasted into a request. A JSON request with inline texts was rejected: the agent would have to escape whole Markdown files, and the check would read a copy. Separate comment and attachment flags were rejected: they tie a general matcher to tickets.
- **What it scans:** each file's lines (split on `\n`, a trailing `\r` trimmed, numbered from 1), and its base name, because a posted file carries its name. Never the directory part of the path (the temp directory's own name contains "docket").
- **Options:** the same `leakscan.Options` the publish check builds, through the same helper (`leakOptions`): `match_word` is the effective `leak_check.match_word`, and the change ids are every change in the metadata snapshot. It reads its configuration and snapshot the way `context.ticket` does, from one pinned read.
- **Result:** `applied`, with `context` holding `clean` (true when there are no hits), `match_word` (the value used), and `hits`, each `{file, line, text, rule}`: `file` is the path as given, `line` is 1-based (0 for the file name), `text` is the matched text only, and `rule` is the leakscan rule name (`marker`, `trailer`, `path`, `alias`, `change-ref`, `word`). Hits are listed per file, in argument order, the name first.
- **A hit is reported, not refused,** like `context.ticket`'s "not linked" and "partial": the caller decides. A missing `--file`, or a file that cannot be read, is refused as invalid input, so a bad path can never read as clean.
- **Any visibility.** The operation is a pure read and scans the same way in a shared repository. Only the ticket skill's private path calls it, and the publish operations still run their own check in private repositories only.

### 6. ADR

Record one ADR at build: an unattended private ticket update skips human review of the cleanup and is posted only when the final leak check, using exactly the publish leak check's patterns and options, finds nothing in the comment and every attachment; any hit, word-check hit, or failed check posts nothing and reports one line, with no fix-and-recheck; what the patterns deliberately miss relies on the cleanup alone; unknown visibility posts nothing. It names the alternatives rejected: a review step (nobody is there), an agent-run pattern search (it would drift from the Go rules and lack docket's change ids, and the agent that did the cleanup would be grading its own work), fixing a flagged line and checking again, and a new `docket-export` mode. It also says that `leak.check` scans in any visibility, while the publish operations still run their check in private repositories only, so ADR-0144's "shared repositories never invoke the scanner" keeps holding for publishes. It relates to ADR-0144 (the leak check), ADR-0163 (automatic updates), ADR-0162 (the two ticket skills), and ADR-0044 (call-site directions).

## Where it lands

Derive the full list from a whole-repository search for the old rule (`not available in a private`, `other than `shared``, `posts nothing and reports one line`, `automatic update there posts nothing`, `in a shared repository, the switches`, `Never load `references/private.md``) rather than from this list. At grooming it found:

Skills (each edit mirrored in the embedded tree by `go generate ./internal/assets/`):

- `skills/docket-update-ticket/references/automatic.md`: the visibility routing in decision 1 replaces the clause that posts nothing for any visibility other than `shared`, its report line, and "Never load `references/private.md` on this path."
- `skills/docket-update-ticket/references/private.md`: the unattended rules (decision 3) and the final check (decision 4). It stays the one private delivery reference, holding attended and unattended review.
- `skills/docket-update-ticket/SKILL.md`: its reads line also names the `leak.check` read on the automatic private path.

Go:

- The `leak.check` operation (decision 5): the command, its capability entry, its schema-registry entry, and the command lists that every command must appear in (derive them from the guards that walk the command tree, such as the install command's asset-independence map).

Docs and samples:

- `docs/concepts/tickets.md`: *What goes with an update* (automatic updates in a private repository now attach cleaned copies, behind the final check, with what it does not catch); the closing line that says the switches keep the ticket current "in a shared repository"; a new invariant (in a private repository an automatic update posts only when the final leak check finds nothing in the comment and every attachment); and *Decided in* (the new ADR and ADR-0144).
- `docs/reference/config-keys.md`: the sentence saying an automatic update in a private repository posts nothing; and a new row for `leak_check.match_word`, which has none today and now also governs automatic ticket updates.
- `docs/reference/glossary.md`: the `docket-update-ticket` entry's private sentence, and an entry for the leak check command beside *Ticket context*.
- `docs/reference/skills-and-agents.md`: the `docket-update-ticket` line ending "an automatic update there posts nothing".
- `docs/reference/cli.md`: a noun line for `docket leak`.
- `.docket.example.yml` and its embedded twin: the `match_word` comment also covers automatic ticket updates.

Docs describe only how docket behaves now: no change or PR citations, no history of the earlier rule.

Size budgets in `internal/repoguard/budgets_test.go` are raised for each grown file, with this change's note, per that table's convention.

## Tests

- **Same hits as the publish check.** For a corpus covering every rule, each deliberate miss, `match_word` on and off, and change ids present and absent, the operation's hits on a file match, on line, text, and rule, what the publish check's scan reports for the same text as a PR body with the same pinned configuration and snapshot. Mutation: build the operation's options any other way than `leakOptions` (say, hard-code `match_word`) and watch it redden.
- **The operation's edges:** a file name hit is line 0; the directory part is never scanned; a missing or unreadable file is refused, never clean; hits are reported under `applied`; several files report in argument order.
- **Prose contracts.** Update, in the same commit, the rows that pin the old rule: `internal/repoguard/ticket_auto_updates_contracts_test.go` (automatic.md's `other than `shared`` clause, its report line, and "Never load `references/private.md` on this path"; the concepts page's and the glossary's private sentences) and, where wording moves, the private-delivery rows in `internal/repoguard/prose_contracts_test.go`. Pin the new rules: the visibility routing; the export direction that asks nobody; no before → after; both stops posting nothing and reporting one line; the final check over the comment and every attachment before any upload; the comment file holds plain text with real line breaks, never an escaped or structured body; only `applied` and clean posts; no fix-and-recheck. Add absent-clause rows for the removed wording.
- **Mutation-test every new guard and contract row**: delete each pinned clause and watch it redden. Run with the test cache defeated.
- The whole suite runs at the build gate.
- **No ticket-system call is made from this repository**, in tests or otherwise, and this repository's `.docket.yml` still declares no `tickets` key.

## Acceptance criteria

- `docket leak check --file …` reports the same hits as the publish leak check for the same text, with tests, and a file that cannot be read is refused, never clean.
- `docket-update-ticket`'s automatic path in a private repository exports and cleans each attachment, shows no before → after, runs the final check over the comment and every attachment before any upload, and posts only on a clean result.
- The skill text states the post-nothing rule for both checks: a `docket-export` word-check hit, and any final-check result other than clean; each reports one line and leaves the run untouched.
- An unknown visibility posts nothing and reports one line.
- No doc, skill, or contract row still says an automatic update in a private repository posts nothing; `config-keys.md` documents `leak_check.match_word`.
- The new ADR is recorded and relates to ADR-0144, ADR-0163, ADR-0162, and ADR-0044.
- The results list an **Important** live check, done by a person from a private scratch repository (its `tickets.*` keys in `.git/dckt/config.yml`) against a sandbox ticket project, never from this repository:
  - with `tickets.auto.pr_opened` on, run docket-implement-next on a change with a linked ticket and confirm the comment arrives with the cleaned spec and results attached and no docket text in either;
  - run `docket leak check --file` by hand on a copy of an exported file with a planted `#0NNN` reference to a real change id and confirm it reports the hit;
  - confirm what the dispatched docket-implement-next does at the post step: the comment, or the "no connector" line.

## Out of scope

- Widening the leak patterns.
- Running the final check on on-request updates.
- Any change to `docket-export`, to shared-repository updates, or to which events post automatically.
- Mentions in automatic updates.
- Cleaning up the temp directories the exports write.

## Left to the builder

- The exact wording of the skill rules and of every report line, within decisions 1 to 4.
- The operation's human (non-JSON) output and its exit codes, following the repository's convention for read operations that report rather than refuse.
- The refusal reason tokens for a missing or unreadable file.
- The comment file's name in the temp directory, as long as it holds no leak pattern.
- The glossary entry's and concepts page's wording, and which clauses the prose contracts pin.

## What the code says today

Traced at main `fb655faed` and docket `ee380ed08`; anchors are symbols and headings, not line numbers.

- `internal/leakscan` exports `Scan`, `Text`, and `Line`, pure and stdlib-only, with rules tried in the order marker, trailer, path, alias, change-ref, word. `changeRef` matches `change[s] #N` (any digits), `change[s] NNNN` (four or more), and zero-padded `#0NNN` and `(0NNN)`, and only for ids in `Options.ChangeIDs`. Its word, change-ref, and trailer rules depend on what starts a line or precedes a match: a word glued to a preceding letter, a `#` after a letter, and a trailer anywhere but a line's start never match.
- `leakOptions` (`internal/app/leak_check.go`) builds the options from `pin.Config.Effective.LeakCheck.MatchWord` and every change in the snapshot. `runLeakCheck` scans a publish's branch name, outgoing commits, added paths and lines, and PR text; `leakCheckApplies` runs it in private repositories only. No command exposes the matcher for arbitrary text.
- `ContextTicketRead` (`internal/app/ticket_context.go`) loads its pin with `PinContext` and builds its snapshot from one corpus read; it returns `visibility`, and its refusals are only a malformed request or an id that names no single record.
- `skills/docket-update-ticket/references/automatic.md` posts nothing and reports `automatic updates are not available in a private repository` for any visibility other than `shared`, and never loads `references/private.md`. `references/private.md` holds the on-request private path: the `docket-export` file request, the clean comment, the own-words check, the single stop on a rewrite, attachments first and comment last.
- `skills/docket-export/SKILL.md`'s file request returns the cleaned file and its before → after list with no stop, but its word check still stops to show a person.
- `internal/exportcheck` is the export's own self-check, independent of `leakscan` by design (`TestDetectorSharesNothingWithTransform`); this change does not touch it.
- `docs/reference/config-keys.md` has no `leak_check.match_word` row; `.docket.example.yml` documents it as governing pushes and pull requests only.
