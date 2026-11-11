# Automatic updates

Loaded from `SKILL.md` when a docket workflow invoked this skill after an event whose switch is on. The rules in `SKILL.md` apply except where this file replaces them.

- **The caller supplies the change and the event.** Take both from the workflow's direction. Nothing is worked out from a person's words, and no one is mentioned: skip sections 1 and 5.
- **Never ask and never wait.** Show no draft. Wherever the on-request path would ask the person something, post nothing and report one line.
- **Read first.** Run the `context.ticket` read yourself with `--id`, `--event`, and `--json`, as on request, so the update is composed from one pinned commit.
- **`event.switch` is `false`:** post nothing and say nothing, whatever the caller directed. Check this first, before anything else the read returned.
- **`ticket.state` is `not-linked`:** post nothing and say nothing.
- **Otherwise post nothing and report one line** when `ticket.state` is `partial` (the ticket link is broken); when `event.happened` is `false` (name the missing fact from `event.reason`); when `visibility` is anything other than `shared` (`automatic updates are not available in a private repository`); when no connector for `ticket.provider` is available in this session; or when the post failed or its outcome is uncertain (an uncertain outcome is reported as uncertain).
- **Otherwise post** exactly what an on-request update for this event posts in a shared repository: its row of section 3's table, the editorial standard, and the links. **Read `references/shared.md` now (blocking).** Never load `references/private.md` on this path.
- **Report one line** and return to the workflow: the update as posted, only with the connector's confirmation, naming the ticket and the event (`Posted the PR-opened update to CLOUD-123.`); or what was not posted and why (`Not posted to CLOUD-123 (PR opened): no Jira connector is available in this session.`).
- **Never retry, and never touch the calling run:** no halt, no blocked marker, no changed disposition, and no metadata write. A duplicate comment is acceptable and is deleted by hand.
