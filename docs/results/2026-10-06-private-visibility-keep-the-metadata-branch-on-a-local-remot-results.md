<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0531 — Keep the metadata branch on a local remote with neutral naming](../changes/active/0531-private-visibility-keep-the-metadata-branch-on-a-local-remot.md)**
<!-- docket:backlink:end -->

# Keep the metadata branch on a local remote with neutral naming — Results

**Human action:** Assessment pending. The build is done and green, and whole-branch review has not run yet.

## Outcome

Docket can now run privately in a repository that must not carry any docket-named branch or root files. `docket repository init --private`, or a `visibility: private` setting in effect, sets a repository up this way:

- The metadata branch, named `dckt`, is pushed to a bare repository on your machine at `~/.local/share/dckt/<owner>-<repo>/remote.git`, never to `origin`.
- The metadata checkout lives outside the clone.
- Per-repo state and config sit under `.git/dckt/`.
- The `.worktrees/` ignore entry goes into `.git/info/exclude`.

Shared repositories behave exactly as before.

Departures from the agreed design:

- **Skills use a new prepare-context field.** They read the `metadata_tracking_ref` field from `repository prepare`, alongside `metadata_remote`. They do not name `metadata_branch` directly, because an existing prose guard forbids that word in skill files.
- **A claim bug that also affected shared mode was fixed.** When two dispatches with different run contexts raced to claim the same change, the loser got an `invalid-input` (`request-id-reused`) error instead of `contended`. A gated claim's request id now includes a 16-hex prefix of its run-context hash. The loser now reports `contended`, and a retry of the same claim still replays idempotently.
- **`status --json` does not carry `metadata_branch`.** An existing protocol guard from change 0363 forbids it. Status shows the branch only in its human-readable output.
