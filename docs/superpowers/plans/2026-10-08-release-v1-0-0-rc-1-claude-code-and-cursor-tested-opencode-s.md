<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0544 — Release v1.0.0-rc.1: Claude Code and Cursor tested, OpenCode shipped untested](../../changes/active/0544-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s.md)**
<!-- docket:backlink:end -->

# v1.0.0-rc.1 release candidate publication — plan

This change is a human-attended release protocol, not a build. It is run by the operator in an
attended session and never by `docket-implement-next`.

The plan is the spec's protocol, executed in order:
`docs/superpowers/specs/2026-10-08-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s-design.md`.

1. Phase 0 — before the cut (no loops attested, no open PRs, no other change in progress).
2. Phase 1 — cut the candidate at `d0f4712a53bd6a158fddada91cdf612f1755a531`, claim, reconcile, attach this plan.
3. Phase 2 — package once with the release-candidate workflow; verify `evidence.json` and checksums exactly.
4. Phase 3 — no human harness test; record the coverage statement.
5. Phase 4 — publish `v1.0.0-rc.1` as a full release (Latest) at the human's explicit "publish".
6. Phase 5 — public install check with `--harness claude --harness cursor`.
7. Phase 6 — install docs, closeout: gate, evidence, results, PR, then mark implemented, then finalize.

The evidence bundle lands under `docs/release/v1.0.0-rc.1/` on this change's feature branch.
