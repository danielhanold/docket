<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0588 — Update tickets automatically in a private repository](../changes/archive/2026-11-27-0588-update-tickets-automatically-in-a-private-repository.md)**
<!-- docket:backlink:end -->

# Update tickets automatically in a private repository — Results

**Human action:** One Important live check is needed. Automated tests cannot reach a real ticket system, so a person must confirm from a private scratch repository that an automatic update posts cleaned attachments, and that a planted leak is caught.

## Outcome

Before this change, when a private repository had an automatic ticket update switched on (for example `tickets.auto.pr_opened: true`), the workflow posted nothing and reported one line saying automatic updates were not available there. Now the update is posted. It carries cleaned copies of the spec and the results file as attachments, the same way an on-request private update does.

Nobody reviews the cleanup on this path, so a final check runs first. It scans the comment and every attachment with the same patterns and options the push and PR leak check uses. If anything matches, if the export's own word check finds "docket", or if the check cannot run, nothing is uploaded and the run reports one line saying where the hit was. The run that triggered the update is never halted or blocked by this. If the repository's visibility cannot be read, nothing is posted.

The check is a new read-only command, `docket leak check --file <path> …`. It reports hits by file, line, matched text, and rule. It scans each file's name but not its directory, and it refuses a missing or unreadable file instead of calling it clean. It works in any repository, but only the ticket skill's private path uses it.

The skill text, the tickets concepts page, the config keys reference (which now has a row for `leak_check.match_word`), the glossary, the skills reference, the CLI reference, and the sample config all describe the new behavior. ADR-0165 records the decision.

One departure from the plan: the capability catalog's size limit went up from 16 KB to 17 KB. The catalog was 2 bytes under its limit before this change, and the new entry is a 144-byte one-line stub. The limit has been raised the same way for earlier changes that added operations. The latest commit on main went the other way and shortened text to stay under 16 KB instead, so a human may want to choose which approach to keep.

## Human actions and testing

### Important — live check from a private scratch repository

No test in this repository calls a ticket system, so the full automatic private path, from export to upload, has never been exercised end to end. If you skip this, the first real private automatic update is the test.

Prerequisites: a private scratch repository initialized with docket (`.git/dckt/` present), with `tickets.*` keys in `.git/dckt/config.yml`, including `tickets.auto.pr_opened: true`. You also need a sandbox ticket project and a session with that ticket system's connector. Never run this from the docket repository.

1. In the scratch repository, link a build-ready change to a sandbox ticket and run docket-implement-next on it until the PR opens.
   Expected: the sandbox ticket gets a comment with the cleaned spec and results file attached. Neither attachment nor the comment contains "docket", "dckt", `.docket` paths, or zero-padded change references such as `#0012`.
2. Copy one exported attachment, add a line referencing a real change id in that repository in zero-padded form (for example `see #0003`), and run `docket leak check --file <copy> --json` from the scratch repository.
   Expected: `result` is `applied`, `clean` is `false`, and one hit has `rule: change-ref` on that line.
3. Note what the dispatched docket-implement-next run printed at the ticket-update step.
   Expected: the comment was posted, or there is one line saying no connector was available. Either way the run ended `advanced`.

Cleanup: delete the sandbox comment and the scratch repository's test PR and branch.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed at the final head. The budget report had two screening notes (BUDGET WATCH, streak 1/5) and no breaches.
- `docket leak check` is tested against the push and PR leak check on the same text: the hits match with `match_word` on and off, and with LF and CRLF line endings. Mutation tests confirmed three failure modes are caught: options built without `leakOptions`, the directory part of the path being scanned, and files read after the repository pin.
- Every new or changed prose contract row in `ticket_auto_updates_contracts_test.go` was mutation-tested: deleting each pinned phrase, or putting a removed clause back, turned the test red.
- Review ran at the standard tier and returned one important finding: a docs link to ADR-0144 pointed at a file that is not on main. It was fixed in this branch, and a contract row now guards against it coming back.
