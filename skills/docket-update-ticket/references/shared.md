# Shared delivery

Loaded from `SKILL.md` when `visibility` is `shared`. The rules in `SKILL.md` apply; they are not repeated here.

- **Where links come from.** The PR link is `facts.pr_url`; the spec and results links are each `artifacts` entry's `url`. Each `url` is pinned to `metadata_commit`, so the reader sees the file as it stood when you read it. Never build a link yourself, never link a branch, and never link the spec copy on the feature branch.
- **Which links.** Only the links the update's row in `SKILL.md` names, labeled (`Spec:`, `Results:`).
- **A missing link.** When the artifact is not in `artifacts` (an older change whose file is not on the metadata branch), or its `url` is empty (the repository has no web address), post the update without that link; never substitute a local path.
- **Links live in the comment only.** Never add a separate entry to the ticket's links panel (in Jira, a remote issue link): nothing could tidy them later.
- **Who can open them.** Readers are assumed to be able to open the repository; nothing checks their access.
