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

## Known issues and follow-ups

### Review findings (deep tier), before the fix pass

The whole-branch review returned 8 findings: 1 blocker, 4 important, 3 minor. Dispositions are pending until the fix pass returns.

1. **Blocker.** Deleting a private clone and re-cloning it at the same path leaves a stale checkout that blocks init and prepare.
2. **Important.** Re-running `init` without the flag on a repository set up with `--metadata-remote` refuses with a false conflict and creates a stray bare store.
3. **Important.** Private init does not refuse up front when `.docket.local.yml` exists. It switches the repository to private first, and every command refuses afterwards.
4. **Important.** In private mode, the worktree-missing and local-metadata-missing findings tell you to run `migrate`, which refuses in private repositories.
5. **Important.** Each clone reads repository-identity keys from its own `.git/dckt/config.yml`, but clones share one metadata branch, so two clones can disagree on where the backlog lives. This needs a human design decision.
6. **Minor.** Two different origins can map to the same store name, for example `acme/web-app` and `acme-web/app`.
7. **Minor.** A relative local-path `--metadata-remote` is stored exactly as typed.
8. **Minor.** About 20 skill passages still say "the `docket` branch" where they mean the metadata branch.
