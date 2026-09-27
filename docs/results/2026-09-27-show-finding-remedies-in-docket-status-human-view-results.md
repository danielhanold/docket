<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0456 — Show finding remedies in docket status human view](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0456-show-finding-remedies-in-docket-status-human-view.md)**
<!-- docket:backlink:end -->
# Show finding remedies in docket status human view — Results

**Human action:** None required to merge. An optional walkthrough below shows the new layout on a real repository.

## Outcome

Before this change, plain `docket status` (the human-readable view, without `--json`) hid part of the health report. Findings of severity `notice` were never shown or counted, and every finding's remedy (the suggested fix, such as a filled-in `docket change repair-identity …` command) was dropped. The `--json` output always carried both.

Now the human view:

- counts all three severities on the health line, for example `health: ok (0 errors, 0 warnings, 5 notices)`. Notices never make a repository unhealthy; `ok` still means zero errors and zero warnings;
- groups findings under an `errors:`, `warnings:`, or `notices:` heading, in that order, with no heading for an empty severity. Within a group, findings keep their original report order;
- prints each finding's remedy on an indented `    remedy: …` line under the finding. Multi-line remedies stay indented.

The per-row severity column is gone because the group heading now carries it. The JSON output, the set of findings, and their wording are unchanged. Only `internal/app/status_human.go` and its tests changed.

## Human actions and testing

### Optional — see the new health section

Useful if you want to see the layout on docket's own repository, which currently has several configuration notices.

1. From a checkout of this branch, run `go run ./cmd/docket status`.
   Expected: the last section begins with a `health:` line that includes a notice count, followed by a blank line and a `notices:` heading listing entries such as `deferred-setting — .docket.yml: …`.
2. Run `go run ./cmd/docket status --json` and compare.
   Expected: every finding in the JSON `findings` array appears in the human view, and each non-empty `remedy` appears on an indented `remedy:` line.

## Verification performed

- Focused tests: the 11 `TestStatusHumanText*` tests pass. Four existing golden tests were rewritten to the new layout, and six new tests were added (notices only, interleaved severities, single- and multi-line remedies, an empty severity emitting no heading, and singular versus plural counts).
- Mutation probes: each of the five mutations named in the design turned at least one test red. The five were removing the notices group, swapping the group order, dropping the remedy line, counting notices against `ok`, and dropping the multi-line indent. The file was restored byte-identical afterwards.
- `go test ./internal/app/` passed. The full configured suite runs at the build gate; its evidence is recorded in the PR.
