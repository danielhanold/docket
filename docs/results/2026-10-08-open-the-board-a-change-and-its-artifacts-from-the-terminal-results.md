<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0542 — Open the board, a change, and its artifacts from the terminal](../changes/active/0542-open-the-board-a-change-and-its-artifacts-from-the-terminal.md)**
<!-- docket:backlink:end -->

# Open the board, a change, and its artifacts from the terminal — Results

**Human action:** None required. Before merging, you may want to run the optional walkthrough below. It is the only way to see a real browser or Markdown app launch, because the test suite always uses a fake opener.

## Outcome

`docket open <what> [id]` is a new command. `<what>` is one of `board`, `change`, `spec`, `plan`, `results`, or `pr`. Run it with no id inside a checkout whose branch belongs to a change, and it works out the change from the branch. It checks active changes first and falls back to archived ones.

Where it opens things:

- In a shared repository with a GitHub origin, artifacts open as GitHub pages on the metadata branch.
- A new ordinary layered config key, `open.artifacts: github | local` (default `github`), opens the file from the local metadata checkout in the operating system's default app instead.
- A non-GitHub origin falls back to the local file, with a note saying so.
- Private repositories always open the local file. They ignore the key and add no note.
- `pr` always opens the record's `pr:` URL.
- Before a local open, the metadata checkout is synced the same way `repository prepare` syncs it. If the sync is refused, the file still opens, with a stale-checkout note.

Other behavior:

- `--print` prints the target without launching anything.
- `--json` returns an `OpenResult` (`what`, `change_id`, `target`, `target_kind`, `launched`, `notes`).
- Human-mode notes go to stderr, so a successful run prints exactly one line on stdout.
- The opener is `open` on macOS and `xdg-open` on Linux. Any other platform fails with a clear message after printing the target.

The design was followed with no material departures.

## Human actions and testing

### Optional — open real artifacts on this machine

The automated tests never launch a real application. This walkthrough confirms that the browser and the Markdown app actually open.

Prerequisite: build the binary from the feature branch with `go build -o /tmp/docket-open ./cmd/docket`, run inside `/Users/homer/dev/docket/.worktrees/open-the-board-a-change-and-its-artifacts-from-the-terminal`.

1. From `/Users/homer/dev/docket`, run `/tmp/docket-open open board`.
   Expected: the browser opens `https://github.com/danielhanold/docket/blob/docket/docs/changes/BOARD.md`, and that URL is the only line printed.
2. Inside the feature worktree, run `/tmp/docket-open open spec`.
   Expected: the spec for change 542 opens on GitHub. The change was inferred from the branch.
3. Run `DOCKET_X=1 /tmp/docket-open open plan 542 --print`. The `DOCKET_X` variable is irrelevant; `--print` is the part being tested.
   Expected: the plan URL is printed and nothing opens.
4. Add `open:\n  artifacts: local` to `/Users/homer/dev/docket/.docket.local.yml`, then run `/tmp/docket-open open results 542`.
   Expected: the results file under `/Users/homer/dev/docket/.docket/docs/results/` opens in your default Markdown app.

Cleanup: remove the `open:` block from `.docket.local.yml`, then delete `/tmp/docket-open`.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed after the build and passed again after the review fixes, on the final head. The gate reported `PARALLEL-SENSITIVE` screening lines for existing shards (finalize e2e, app merge, reposetup, race). None of these shards is touched by this change, and no serial-confirmed budget breach was reported.
- Each task worker ran mutation probes on its own guards: active-before-archived ordering, the BlobURL-empty fallback, the private short-circuit, the path-containment check, the opener selection, and the presenter notes. Each probe turned its test red.
- The new integration shard `tests/test_go_integration_app_open.sh` runs against a real private-layout repository. It covers a behind checkout that is fast-forwarded before the open, a dirty checkout that opens with the stale note, and a missing file that fails and names the path.
- Whole-branch review (deep tier, chosen because the diff is over 1500 lines): 4 findings (2 important, 2 minor), all fixed in-branch. The full table is in the PR body.
