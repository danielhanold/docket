<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0580 — Release v1.0.0: stable release with version-free install docs](../changes/active/0580-release-v1-0-0-stable-release-with-version-free-install-docs.md)**
<!-- docket:backlink:end -->

# Release v1.0.0 — Results

**Human action:** Install v1.0.0 from the GitHub release on your own machines once this change is finalized and the binary rebuilt, so the post-merge source rebuild does not overwrite your release install. Nothing else needs a human beyond reviewing this PR.

## Outcome

Published `v1.0.0` as a full release and Latest from candidate `0cf3ab4de2fd81c5799106830d1e28ab60ad3109`, replacing `v1.0.0-rc.2`. Release: https://github.com/danielhanold/docket/releases/tag/v1.0.0. The install docs on `main` are version-free (`releases/latest/download`), merged before the cut in PR #446, and the `latest` download links serve this release's files. Evidence bundle: `docs/release/v1.0.0/`.

## Human actions

| Phase | Verdict | Record |
|---|---|---|
| 0 — Docs to `main`, before the cut | pass; version-free docs PR #446 merged as `0cf3ab4de` and suite-gated; no loops attested; no other change in progress; no open PRs | decisions.md |
| 1 — Cut and freeze | pass; `main` held at the candidate `0cf3ab4de` through publication | decisions.md |
| 2 — Package once | pass; run 40609036031, `evidence.json` byte-exact, Bash upgrade shard passed, no STOP lines | candidate/run.txt |
| 3 — Coverage statement | none by design; recorded | decisions.md |
| 4 — Publish | pass, at Daniel's explicit "publish" (private-repositories highlight removed from the notes first); tag `v1.0.0` at the candidate, six verified assets, Latest; `latest` links verified | publication.md |
| 5 — Public install | none by design | decisions.md |
| 6 — Build gate | PASSED at `0e4e7534d` | — |

### Optional

- Two deferred changes are now due: #514 (retire the saved Bash upgrade test cases; its planned "last verified against v1.0.0" freeze line needs rewording now that the upgrade guide points at `latest`) and #513 (full OpenCode support). This change does not revive or edit them.
