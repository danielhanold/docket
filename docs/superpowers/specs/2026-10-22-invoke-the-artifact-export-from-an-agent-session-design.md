<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0578 — Invoke the artifact export from an agent session](../../changes/active/0578-invoke-the-artifact-export-from-an-agent-session.md)**
<!-- docket:backlink:end -->

# Invoke the artifact export from an agent session — design

Change #578, groomed 2026-10-22.

## Problem

#561 added `docket open <change|spec|plan|results> [<id>] --export`, which writes a clean Markdown copy of a docket artifact for readers who do not use docket. Only a person typing the command in a shell reaches it. When an agent is asked to export one of these artifacts, or to put one into another tool — for example, add a spec as a comment on, or the body of, an existing Jira ticket through an MCP integration — nothing tells it the command exists, or that docket content needs cleaning before it leaves the repository at all. Daniel has seen this on another machine more than once: the agent never used the export, did not know the content needed sanitizing, and the result still carried everything the export removes.

The export itself is complete. The capability catalog's `open` entry already lists `--export`, `--output-path`, `--print`, and `--warnings`; `--print` writes the copy without launching a viewer; `--json` returns the written path and the warnings as fields. What is missing is (1) anything that makes an agent find it, and (2) for a private repository, a way past what the export deliberately leaves: it never rewrites prose, so a sentence that names docket stays as written (reported as a mention), as do ADR numbers and bare `#N`. In a private repository the team must never see docket, and a post to an outside tool cannot be taken back; the PR leak check does not run on exports.

## Goal

An agent session that is asked to export a docket change, spec, plan, or results file, or to put one anywhere outside docket, runs the real export — the same code the CLI runs — and never builds its own copy. In a private repository, what leaves contains no trace of docket.

## Non-goals and anti-goals

- **No change to the export.** `docket open --export`, its output, its flags, and its catalog entry stay as they are. No new catalog entry or command.
- **No agent** (my call). A dispatched agent would start a fresh context for one command, and the dispatch rule in the managed instructions block would then route every export request through it. The skill must also run where the person's posting tools are and must be able to ask the person.
- **No sending logic.** The skill never posts anything itself and carries nothing specific to Jira or any other tool. Posting is done by the agent's own tools (an MCP integration, for example).
- **Not in `docket-convention`** (Daniel agreed). The convention is read only after something loads it, and an export request loads nothing; making its description match "export" would pull about 52 KB into context for one command, and the convention is declared pure reference (no procedure).
- **Not in the always-loaded instructions block.** Every session would pay for it, and that block is about dispatching agents and the run tracker.
- **No `docket --help` change** (my call).
- **No rewrite pass in a shared repository** (Daniel's decision). There the team can already see docket on the metadata branch, and in docket's own repository the word is the subject.
- **Anti-goal: the agent redoing the export's work by hand,** or reworking words beyond the private-repository rewrite pass below.

## The skill: `docket-export`

A new skill directory, `skills/docket-export/`, holding one `SKILL.md`.

- **Inline.** Frontmatter carries `name` and `description` only — no `context: fork`, no `agent:`. It runs in the requesting agent's own context because it must reach that agent's tools (the MCP integration that posts) and must be able to ask the person; a forked skill can do neither. No generated agent wrapper and no Cursor dispatch rule.
- **No convention load** (my call, for size). The skill resolves what it runs itself: `docket capabilities --json` (the one docket spelling a skill may hard-code), check `protocol_version` is 1, and take the `argv` of the entries it uses (`open`, `repository.prepare`, and `diagnostic.config` if the builder reads `leak_check.match_word` from it). If the `docket` binary is missing or an entry is absent, the agent says the docket install is broken and stops — never a hand-made export.
- **Distributed body.** Skills install at user level, so the skill is present in every session on the machine, in repositories that use docket and ones that do not. Its body states nothing true only of this repository.

### The description (what makes an agent find it)

It must match both kinds of request: exporting a docket change, spec, plan, or results file, and putting one into anything outside docket (a ticket or comment in another tool, a document, a message, an email). Approximate wording, final wording the builder's: "Use when asked to export a docket change, spec, plan, or results file, or to put one into anything outside docket — a Jira ticket body or comment, a document, a message — so its content comes from docket's own export, never the raw file or a hand cleanup."

### What the skill tells the agent

1. **Never build an export by hand, and never send the raw file.**
2. **Learn the repository's visibility** from the `repository.prepare` context (`visibility`: `shared` or `private`). The export runs the same prepare internally, so this adds no new side effect.
   - The directory is not a docket repository (not a git repository, or no docket set up — a fresh repository): the skill does not apply. The agent says the content is not a docket artifact in one line and handles the request without the skill (my call).
   - Any other refusal, so visibility is unknown: treat the repository as private (my call — a leak cannot be undone, and this keeps the export usable while the metadata checkout needs attention).
3. **Run the export:** `<open argv> <what> [<id>] --export --print --json`, where `<what>` is `change`, `spec`, `plan`, or `results` and `<id>` is the change the person named (omitted, `open` uses the checked-out change, as it does today). Always `--print`, so no viewer opens — the person may not be at the screen; they can ask the agent to open the file. Always `--json`. Never `--warnings` unless the person asked for a filter on a file request; the default (`all`) is what the rewrite pass reads.
   - **File request** ("export the spec for 9"): the export's default destination (`~/Downloads` on macOS, the home directory on Linux), or `--output-path` when the person names a place.
   - **Outside-tool request** ("add the spec for 9 as a comment on PROJ-123"): `--output-path` set to a fresh temporary directory (`mktemp -d "${TMPDIR:-/tmp}/docket-export.XXXXXX"`), so the person's Downloads folder does not collect one file per post. The temporary file stays where it is (my call); its path is reported so the person can see exactly what went out.
4. **Failure** — `result` is anything other than `applied`, or there is no `exported_path` (for example `invalid-state` / `artifact-unset` when the change has no spec yet): tell the person the `message` and post nothing. Never fall back to the raw file or a hand cleanup (Daniel's decision). This holds in both kinds of repository.
5. **Shared repository** (Daniel's decision): no rewrite pass and no approval step. Hand the exported text to the target unchanged — on a file request, the file is the result. Any `leftovers` (rare) go out as they are and are named in the report afterwards.
6. **Private repository — rewrite pass, automatic** (Daniel's decision). When the export reported any `leftovers` or `mentions`, the agent edits the exported copy in place so that nothing the convention's private-repository writing rule bans remains, and every edited line reads naturally to an outside reader:
   - fix each leftover (remove the marker, unwrap the link, drop the stamp line, …);
   - rewrite each line on the `mentions` list so it no longer names docket or dckt (for example "the docket gate runs the suite" → "the test gate runs the suite"); a mention inside code is generalized or dropped, at the agent's judgment;
   - rewrite docket's own ADR numbers (ADRs in the repository's docket ADR ledger) and docket change numbers the export left as written (for example "per ADR-0012") into what they refer to, or drop them — the person's team cannot open them (my proposal; Daniel did not object). The team's own ADRs stay, for the reason the private leak check ignores ADR references (ADR-0144), and so does a `#N` that is not a docket change id (a Jira or PR number). How the agent tells a docket ADR from the team's own is the builder's (for example, whether the id exists in the docket ADR directory of the metadata checkout).
   - `leak_check.match_word: false` (the repository lets the bare word "docket" through its leak check) also exempts the bare word here (my call — config keys behave uniformly); "dckt", markers, ids, and ADR numbers are still handled.
   - Only these lines change; no other rewording, summarizing, or "fixing".
7. **Private repository — check.** Search the rewritten copy for "docket" and "dckt" as whole words, case-insensitive, code included — the same word rule as the export's `mentions` list (bare "docket" exempt under `match_word: false`). Any hit: stop, show the person the path and the lines, and post nothing until they decide.
8. **Private repository — show, then post.** If the rewrite pass changed anything, show the person every change as before → after and post only after they say go (Daniel's decision). If nothing needed rewriting, post straight away. On a file request the same pass and check apply — the file goes to outsiders too — and the before → after list comes with the path, with no stop (nothing is posted).
9. **Posting the text.** Read the file at `exported_path` (after the rewrite pass, in a private repository) and hand its text to the target tool unchanged. When the tool needs a different format (Jira's own markup, for example), convert the formatting only, never the words. When the person asks for a summary or an excerpt ("just the Goals section"), work from this copy, never the raw file (my call). When the text is too long for the target, tell the person and ask whether to split it or attach the file (my call). The skill adds no stop beyond the ones above; whatever confirmation the agent's harness already asks before an outward post is unchanged.
10. **Report back:** the path, whether an existing file was replaced (`replaced`), any warnings, and (private) the before → after list.

Out of the skill's reach, said in one line so the agent does not improvise: the board and pull requests have no export; a person who explicitly asks for the raw file is asking for something else.

## Docs

- `docs/reference/skills-and-agents.md`: the Skills list gains `docket-export`.
- `docs/reference/cli.md`: the `docket open` entry gains one sentence saying agent sessions reach the export through the `docket-export` skill.
- Any other sentence that enumerates docket's skills or its inline (no-wrapper) skills — the convention's *Agent layer* paragraph ("the two **interactive** skills … stay inline") is one — includes `docket-export`; raise the convention's word budget if that needs it. Find the sites by a whole-repository grep, not from this list.

## Tests

- The skill is picked up from `skills/` like the others, with no hand-kept list; regenerate the embedded assets (`go generate ./internal/assets/`) and confirm with the existing install tests that it installs into every configured harness the way the other skills do.
- A new row in the per-skill size table (`skillBudgets` in `internal/repoguard/budgets_test.go`).
- Existing guards apply unchanged, notably `TestCapabilitySurface` (no hard-coded docket argv besides `docket capabilities --json`) and the shell rules for agent-executed Markdown.
- Pin the skill's load-bearing rules the way `internal/repoguard/prose_contracts_test.go` pins other skills' rules, and mutation-test each pin. At least: never build by hand or send the raw file; a failed export posts nothing; shared repositories get no rewrite pass and no stop; private repositories get the rewrite pass, the word check, and a before → after stop when anything was rewritten; unknown visibility counts as private.
- The whole suite at the gate.

## Left to the builder

- The exact description and body wording, within the steps above.
- How the skill reads `leak_check.match_word` (for example `diagnostic.config`'s effective value).
- Which of the skill's rules get pinned beyond the list above, and how.

## Relations

- Discovered from #561 (the export); builds on #542 (`docket open`).
- The private-repository rules come from #532 (ADR-0144: the private writing rule, the leak check, and `leak_check.match_word`).
