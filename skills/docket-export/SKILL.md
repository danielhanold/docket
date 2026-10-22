---
name: docket-export
description: Use when asked to export a docket change, spec, plan, or results file, or to put one into anything outside docket — a Jira ticket body or comment, a document, a message, an email — so its content comes from docket's own export, never the raw file or a hand cleanup.
---

# docket-export

Docket artifacts carry links, generated blocks, and references that mean nothing outside docket, and the export writes a clean Markdown copy of them. Use it every time a change, spec, plan, or results file leaves docket. **Never build an export by hand, and never send the raw file.**

This skill runs inline, in your own session: you post with your own tools, and you ask the person directly where a step below says to.

## 1. Resolve the operations

Run `docket capabilities --json` as its own call — the one docket spelling this skill hard-codes. Check that `protocol_version` is 1 and take the `argv` of the entries `open`, `repository.prepare`, and `diagnostic.config`. If the binary is missing or an entry is absent, tell the person their docket installation is broken and stop — never a hand-made export.

## 2. Learn the visibility

Run the `repository.prepare` argv with `--repo-dir <repo root> --json` as its own call and read `context.visibility` (`shared` or `private`).

- The directory is not a git repository, or `repository_state` is `fresh` (no docket set up): this is not a docket artifact. Say so in one line and handle the request without this skill.
- Any other refusal or error: the visibility is unknown, so **treat the repository as private**. A leak cannot be undone.

In a private repository, also run the `diagnostic.config` argv with `--repo-dir <repo root> --json` and read `effective.leak_check.match_word.value`. When it cannot be read, treat it as `true`.

## 3. Run the export

Run the `open` argv as `<open argv> <what> [<id>] --export --print --json --repo-dir <repo root>`, where `<what>` is `change`, `spec`, `plan`, or `results` and `<id>` is the change the person named (omit it to use the change whose branch is checked out). Always `--export --print --json`: `--print` opens no viewer — the person may not be at the screen, and can ask you to open the file. Never pass `--warnings` unless the person asked for a filter on a file request; the default reports everything the private pass reads.

- **File request** ("export the spec for 9"): the export's default destination, or `--output-path <place>` when the person names one.
- **Outside-tool request** ("add the spec for 9 as a comment on PROJ-123"): `--output-path` set to a fresh directory from `mktemp -d "${TMPDIR:-/tmp}/docket-export.XXXXXX"`, so the person's downloads folder does not collect one file per post. Leave the file there and report its path.

**A failed export posts nothing.** If `result` is not `applied` or there is no `exported_path`, tell the person the `message` and stop. Never fall back to the raw file or a hand cleanup. This holds in every repository.

## 4. Shared repository

**A shared repository gets no rewrite pass and no stop.** Hand the exported text to the target unchanged; on a file request the file is the result. Any `leftovers` go out as they are and are named in your report.

## 5. Private repository

Nothing that names docket may leave.

1. **Rewrite pass.** When the export reported any `leftovers` or `mentions`, edit the exported copy in place until it holds none of these: the words "docket" or "dckt", docket marker comments, `Docket-` trailers, `.docket` or `dckt` paths, change ids or change references from docket, ADR numbers from docket's ledger, or references to the backlog or board. Every edited line must read naturally to an outside reader.
   - Fix each leftover: remove the marker, unwrap the link, drop the stamp line.
   - Rewrite each line on the `mentions` list so it no longer names docket or dckt ("the docket build gate runs the suite" becomes "the build gate runs the suite"). A mention inside code is generalized or dropped, at your judgment.
   - Rewrite ADR numbers and change numbers from docket into what they refer to, or drop them; the person's team cannot open them. An ADR is docket's when a file for its number exists under `<metadata_worktree_path>/<adrs_dir>`, and a `#N` is one of docket's changes when a record for that id exists under `<metadata_worktree_path>/<changes_dir>` (paths from the prepare context). The team's own ADRs stay, and so does a `#N` that is not one of docket's changes, such as a Jira or pull request number.
   - When `leak_check.match_word` is `false`, the bare word "docket" may stay; "dckt", markers, ids, and ADR numbers are still handled.
   - Change only these lines: no other rewording, summarizing, or fixing.
2. **Word check.** Search the rewritten copy for "docket" and "dckt" as whole words, case-insensitive, code included (the bare word "docket" is exempt when `leak_check.match_word` is `false`). Any hit: stop, show the person the path and the lines, and post nothing until they decide.
3. **Show, then post.** If the rewrite pass changed anything, show the person every change as before → after and **post only after they say go**. If nothing needed rewriting, post straight away. A file request gets the same pass and check — the file goes to outsiders too — and the before → after list comes with the path, with no stop, since nothing is posted.

## 6. Post

Read the file at `exported_path` (after the rewrite pass, in a private repository) and hand its text to the target unchanged. When the target needs another format, such as Jira's own markup, convert the formatting only, never the words. A summary or excerpt the person asks for ("just the Goals section") is made from this copy, never the raw file. When the text is too long for the target, tell the person and ask whether to split it or attach the file. Whatever confirmation your harness already asks before an outward post is unchanged.

## 7. Report

Report the path, whether an existing file was replaced (`replaced`), any warnings, and, in a private repository, the before → after list.

The board and pull requests have no export. A person who explicitly asks for the raw file is asking for something else.
