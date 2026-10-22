<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0578 — Invoke the artifact export from an agent session](../changes/active/0578-invoke-the-artifact-export-from-an-agent-session.md)**
<!-- docket:backlink:end -->

# Invoke the artifact export from an agent session — results

**Human action:** Optional. One check (a fresh-session trial of the new skill) is worth doing because skills load only when a session starts; nothing blocks the merge.

## Outcome

A new inline skill, `docket-export` (`skills/docket-export/SKILL.md`), makes an agent asked to export a docket change, spec, plan, or results file, or to put one into another tool, run the real export (`open … --export --print --json`, resolved from the capability catalog) instead of copying the raw file. A failed export posts nothing. In a shared repository the exported text goes out unchanged. In a private repository the agent rewrites what the export leaves behind, always rewrites docket ADR and change numbers (the export never reports those), checks the copy for the words docket and dckt, and shows before → after before posting when it changed anything. Unknown visibility counts as private.

The skill list docs (`docs/reference/skills-and-agents.md`, `docs/reference/cli.md`, `docs/install/models-and-effort.md`, `docs/reference/glossary.md`) and the convention's inline-skill sentences now name `docket-export`. The skill's rules are pinned in `internal/repoguard/prose_contracts_test.go`, and `internal/repoguard/export_skill_test.go` checks that it stays inline with no agent wrapper.

## Human actions and testing

- **Optional — confirm a fresh session picks the skill up.** Why: skills register when a session starts, so no test in this run can show that an agent actually chooses `docket-export` when asked. Prerequisites: this branch merged and the binary rebuilt and installed (`docket install`), then a new Claude Code session in a docket repository that has a change with a spec. Steps: ask "export the spec for change <N>". Expected: the agent loads `docket-export`, runs the `open` export with `--print --json`, and reports a path in your Downloads folder. Then ask it to put the same spec into a scratch document or message; expected: it exports to a temporary directory and posts that file's text, not the raw spec. Cleanup: delete the exported file. If skipped: we rely on the skill description matching such requests, which is untested.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed through the build gate at the final head; budget lines were watch notes only, no breaches.
- Each new prose pin and the inline-skill test were mutation-tested by the workers (the removed clause turned each one red; restoring it turned it green).
- Review ran at the standard tier and returned 1 important and 3 minor findings; all four were fixed on the branch before the PR opened.
