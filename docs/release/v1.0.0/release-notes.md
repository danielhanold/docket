Docket tracks your planned work as changes kept in your own repository, on a metadata branch of their own, apart from your code. You capture an idea and design it with your agent; an autonomous run then claims it, checks it against the current code, plans it, builds it with tests, has it reviewed, and stops at an open pull request. You review and merge; finalize lands the change and keeps the backlog current.

**Install:** [Installing docket](https://github.com/danielhanold/docket/blob/main/docs/install/install.md)

**Harnesses:** Claude Code and Cursor are supported; OpenCode is untested; Codex is unsupported.

## Highlights

- **Capture work that outlives the session.** Turn an idea into a tracked change you or an autonomous run can pick up weeks later. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/capturing-work.md))
- **Design before building.** Grooming takes a rough stub to a written design, with you or hands-free for stubs you opt in. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/designing-before-building.md))
- **Build unattended to a pull request.** implement-next claims the next ready change, plans it, builds it with tests, and stops at an open PR; it never merges. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/building-without-supervision.md))
- **A suite gate that leaves a record.** Every branch is certified by one full test run, and the evidence travels with the pull request. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/proving-the-build.md))
- **Review before you see it.** A reviewer reads the whole branch and its fixes land before the pull request reaches you. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/reviewing-before-the-human.md))
- **Finalize lands changes safely.** Rebase, retest, merge, archive, and clean up, hands-off across a whole set of changes. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/landing-changes.md))
- **A backlog that stays honest.** `docket status` reports what is stale or broken, and the maintenance sweep and halt recovery fix it. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/keeping-the-backlog-honest.md))
- **Decisions and lessons remembered.** Architecture decisions and build learnings are kept as records, so later runs know why. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/remembering-why.md))
- **Planning kept apart from code.** Records live on their own metadata branch, with commands to set a repository up, check it, and repair it. ([guide](https://github.com/danielhanold/docket/blob/main/docs/guide/where-the-metadata-lives.md))
- **Per-agent model pins.** Run each docket skill at its own model and effort instead of one session-wide tier. ([docs](https://github.com/danielhanold/docket/blob/main/docs/install/models-and-effort.md))
