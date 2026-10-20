<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0544 — Release v1.0.0-rc.1: Claude Code and Cursor tested, OpenCode shipped untested](../changes/archive/2026-10-08-0544-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s.md)**
<!-- docket:backlink:end -->

# v1.0.0-rc.1 release candidate publication — Results

**Human action:** None needed to merge. `v1.0.0-rc.1` is already tagged and published as the "Latest" full release; this PR adds the evidence bundle and the install-doc updates. Optionally, delete the local test folders listed below.

## Outcome

Docket `v1.0.0-rc.1` is public as a full release and is now "Latest", replacing `v0.9.3`: https://github.com/danielhanold/docket/releases/tag/v1.0.0-rc.1.

- Cut from `main` at `d0f4712a53bd6a158fddada91cdf612f1755a531`, packaged once by release-candidate run 37844907335; `main` stayed at the candidate from the cut through publication.
- Six assets (four platform archives, `checksums.txt`, `install.sh`), each equal in name, size and SHA-256 to the read-only copy.
- No harness was re-tested by hand: Claude Code was proven in alpha.1 and Cursor in alpha.2, and the candidate's source is identical to alpha.2 (documentation-only diff). OpenCode is shipped untested; Codex is unsupported.
- A fresh install from the public release URL passed, with no quarantine attribute and a valid ad-hoc signature on the binary.
- The install docs now lead with the release downloader and state harness status.

The evidence is in `docs/release/v1.0.0-rc.1/`; its `README.md` holds the candidate identity and the gate table.

## Human actions and testing

This was a human-attended release; Daniel attested no loops were running and gave the explicit "publish" after approving the notes. The gate table is the human-verify record:

| Phase | Verdict | Record |
|---|---|---|
| 0 — Before the cut | pass | `decisions.md` |
| 1 — Cut and freeze | pass | `decisions.md` |
| 2 — Package once | pass, no STOP, no waiver | `candidate/run.txt` |
| 3 — Harness test | none by design; coverage statement recorded | `decisions.md` |
| 4 — Publish | pass, at Daniel's explicit "publish" | `publication.md` |
| 5 — Public install | pass | `public-install.md` |

**Optional — clean up the test material.** Delete the local folders `~/docket-rc1-public-test` (the public-install test home), `~/docket-rc1-candidate` (the read-only bundle copy; `chmod -R u+w` first) and `~/docket-rc1-gate-run` (gate run logs). Nothing in the repository depends on them.

## Verification performed

- **Package:** run 37844907335 green on all seven jobs at the candidate. Source gate `SUITE files=86 passed=86 failed=0` on go1.26.8, including `test_go_integration_bashupgrade` `rc=0 ok=4`; ten `BUDGET WATCH` screening lines, no `SERIAL CONFIRMED OVER BUDGET`.
- **Candidate:** `evidence.json` names the candidate and `v1.0.0-rc.1`; its `checksums_txt` is byte-equal to `checksums.txt`.
- **Publication:** tag object `5df38f85…` peels to the candidate; the release is a full release and "Latest"; asset digests equal the read-only copy.
- **Public install:** `install.sh: OK` against the published `checksums.txt` before running it; `docket version` reported `v1.0.0-rc.1` at the candidate; `install check` clean for `[claude, cursor]`; `supported_target` true; `xattr` shows no `com.apple.quarantine`; `codesign -v` valid.
- **Build gate (this PR):** see the PR body for the gate result at the final head.
- **Review:** none. This change carries release evidence and install docs only, and is not built through implement-next.

## Known issues and follow-ups

- **Cursor's subagent model pins are unconfirmed.** Listed as a known gap in the release notes. Next action: Fits #545.
- **Cursor's documented "Allowlist (with Sandbox)" mode was not exercised.** Listed as a known gap in the release notes. Next action: none decided.
- **The binaries are not notarized.** A browser-downloaded archive is blocked by macOS Gatekeeper; the install docs and notes say to use the script. Next action: none decided; notarization is out of scope.
