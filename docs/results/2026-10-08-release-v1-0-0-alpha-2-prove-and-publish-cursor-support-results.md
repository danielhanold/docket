<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0512 — Release v1.0.0-alpha.2: prove and publish Cursor support](../changes/active/0512-release-v1-0-0-alpha-2-prove-and-publish-cursor-support.md)**
<!-- docket:backlink:end -->

# v1.0.0-alpha.2 acceptance and publication (Cursor) — Results

**Human action:** None needed to merge. `v1.0.0-alpha.2` is already tagged and published as a pre-release, and this PR only adds the evidence bundle. Optionally, delete or archive the disposable fixture repository `danielhanold/docket-accept-v1-0-0-alpha-2-cursor` and the local test folders listed below; the evidence keeps the fixture's PR URL.

## Outcome

Docket `v1.0.0-alpha.2` is public as a pre-release with Cursor proven end to end: https://github.com/danielhanold/docket/releases/tag/v1.0.0-alpha.2. It is not marked latest, so `v0.9.3` is still "Latest".

- Cut from `main` at `ec4c2b1841954c2d6a3dd018e1133ee762861137`, packaged once by release-candidate run 37822474785, and `main` stayed at the candidate from the cut through publication.
- Six assets (four platform archives, `checksums.txt`, `install.sh`), each equal in name, size and SHA-256 to the read-only copy every test used.
- Cursor 3.23.23 ran a full lifecycle on a disposable private repository from an isolated test home: create, groom, implement with a mid-build kill and resume, and finalize. Finalize merged by itself.
- A fresh install from the public release URL passed.

The evidence is in `docs/release/v1.0.0-alpha.2/`; its `README.md` holds the candidate identity and the gate table.

## Human actions and testing

This was a human-attended release; Daniel was present at every gate. The gate table is the human-verify record:

| Phase | Verdict | Record |
|---|---|---|
| 0 — Before the cut | pass (0543 merged; Cursor isolation dry run on alpha.1) | `decisions.md` |
| 1 — Cut and freeze | pass | `decisions.md` |
| 2 — Package once | pass, no STOP, no waiver | `candidate/run.txt` |
| 3 — Cursor lifecycle | pass, with recorded findings | `harness/cursor.md` |
| 4 — Publish | pass, at Daniel's explicit "publish"; notes edited afterwards at his "yes" | `publication.md` |
| 5 — Public install | pass | `public-install.md` |

**Optional — clean up the test material.** Delete the disposable GitHub repository `danielhanold/docket-accept-v1-0-0-alpha-2-cursor` (`gh repo delete danielhanold/docket-accept-v1-0-0-alpha-2-cursor`), and the local folders `~/docket-alpha2-test` (the Cursor test home, including its Cursor profile and the fixture clone), `~/docket-alpha2-candidate` (the read-only bundle copy; `chmod -R u+w` first) and `~/docket-alpha2-evidence` (working notes and gate run logs). Nothing in the repository depends on them.

## Verification performed

- **Package:** run 37822474785 green on all seven jobs at the candidate. Source gate `SUITE files=86 passed=86 failed=0` on go1.26.8, including `test_go_integration_bashupgrade` `rc=0 ok=4` (with 0543's Cursor assertions); eight `BUDGET WATCH` screening lines, no `SERIAL CONFIRMED OVER BUDGET`.
- **Candidate:** `evidence.json` names the candidate and `v1.0.0-alpha.2`; its `checksums_txt` is byte-equal to `checksums.txt` (the alpha.1 trailing-newline waiver is gone); every bundle file's SHA-256 matches its manifest line.
- **Cursor:** isolation proved twice (dry run on alpha.1, and on the candidate) by the agent's shell probe and by dispatching an agent that exists only in the test home. Named children: `docket-implement-next`, `docket-plan-writer`, a build-task worker at the `docket-build-economy` pin, `docket-review-lean`. Kill at the build's first commit, resume of the same change through `resume-active-run` → `run-retry-once` → `run-done … run-complete`, one claim only; every `run-*` line captured verbatim. Terminal predicate: all six checks pass, with `repository check` clean and no `repository prepare` needed.
- **Publication:** the tag object `b83912b0…` peels to the candidate; the release is a pre-release, not latest; asset digests equal the read-only copy.
- **Public install:** `install.sh: OK` against the published `checksums.txt` before running it; `docket version` reported `v1.0.0-alpha.2` at the candidate; `install check` clean; `supported_target` true.
- **Build gate (this PR):** the gate driver passed at the final head `2de03d5f6bea8b50860225126a38006e22b5c128` (`SUITE files=86 passed=86 failed=0`, no budget lines). An earlier run was stopped at Daniel's request-driven notes correction and re-run on the final head.
- **Review:** none. This change carries release evidence only, no code, and is not built through implement-next.

## Known issues and follow-ups

- **Cursor ran docket's subagents at a higher model effort than their pins.** In the acceptance run, `docket-implement-next` and `docket-review-lean` (pinned grok-4.5 medium) and `docket-plan-writer` (pinned low) ran at grok-4.5 high; the parent passed an explicit `model` on each dispatch, so it may be the parent overriding the pin rather than Cursor ignoring it. Suspected, not confirmed. Listed as a known gap in the release notes. Next action: Fits #545 — groom and run it (`docket-groom-next 545`).
- **Killing Cursor's main process leaves its `cursor-agent` worker alive.** After the kill, an orphaned, idle `cursor-agent` process from the test profile survived and had to be terminated before the resume. Confirmed once. A user who quits Cursor mid-run and resumes may have a leftover worker; the release notes say to check for one. Next action: Daniel chose to leave it as a release-notes known gap. No existing change fits (checked 27).
- **Cursor's documented "Allowlist (with Sandbox)" mode was not exercised.** The run used Run Everything, per the design. Next action: none decided; Related to #544, whose rc.1 acceptance run could exercise it.
- **SIGTERM with an open Cursor chat raises a quit dialog.** The protocol's kill now falls back to SIGKILL after 10 s. A later release protocol (#544) should keep that fallback. Related to #544.
- **The groom exited trivial, and the build tier's name was inferred.** Neither affects the predicate; both are in `harness/cursor.md`.
