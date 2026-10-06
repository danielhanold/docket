<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0532 — Implement private visibility for PRs, commits, and shipped files](../changes/active/0532-private-visibility-keep-docket-out-of-prs-commits-and-shippe.md)**
<!-- docket:backlink:end -->

# Implement private visibility for PRs, commits, and shipped files — Results

**Human action:** Assessment pending: the build is green and a whole-branch review returned findings that are being fixed in-branch.

## Outcome

In private repositories, PR descriptions are now the authored prose only, a finalize block posts no PR comment, backlink repointing skips private repositories, and a blocking leak check scans outgoing commits, added lines and paths, and the PR title and description before any feature-branch push or PR write. A new `leak_check.match_word` key lets the bare word "docket" through. `repository check`, `repository prepare` and `workspace publish` report a `docket`/`dckt` branch or a `refs/docket/` ref on origin, report only. Pushes now refuse any ref outside `refs/heads/`.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the build head before review.

## Known issues and follow-ups

### Review findings under repair (checkpoint)

- Blocker: the leak check reads added lines from the net `base...head` diff only, so a fingerprint added in one commit and removed in a later one is pushed unscanned.
- Important: the feature branch name is pushed but never scanned; a trivial change has no spec copy carrying the slug.
- Important: finalize's integration-repair worker commits on the feature branch but never receives the visibility or the writing rule, and the writing-rule guard hand-lists its writers.
