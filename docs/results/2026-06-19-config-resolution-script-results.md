<!-- results-template.md — close-out artifact for a change. -->
# Config resolution + bootstrap guard script — results
Change: #26 · Branch: feat/config-resolution-script · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-06-19-config-resolution-script.md · ADRs: none

## Verify (human)

<!-- Automated tests + the real-data smoke test cover correctness. The one thing the test
     suite cannot assert is that a LIVE skill run actually follows the new directive (it only
     asserts the prose names the script). Optional post-merge confidence check: -->
- [ ] After merge + `link-skills.sh` re-link, run any docket skill (e.g. `docket-status`) and confirm its Step 0 resolves config via `scripts/docket-config.sh --export` without error (the doc-rewire's live effect only manifests once the skills are re-linked).

## Findings

- **No new ADR.** This change reproduces the config + bootstrap-guard semantics verbatim from **ADR-0002** (docket-mode default; refuse-and-migrate bootstrap) and follows the script-owns-*how* / skill-owns-*when* split of **ADR-0007** — it is an implementation of existing decisions, not a new one (spec §2 says so explicitly). Nothing non-obvious emerged during the build to change that.
- **`.docket.yml` parsing verified against the real file's comment traps** (final review). `yaml_get` (ported verbatim from `migrate-to-docket.sh`) correctly skips the top-of-file comment that references `metadata_branch: main` and the commented `# test_command:` / `# require_pr_approval:` lines — the `^[[:space:]]*<key>` anchor never matches a `#`-leading line — so `metadata_branch` resolves to `docket` and `test_command` resolves empty. The `[]`-vs-unset distinction for `board_surfaces` works (`[]` → empty/disabled; unset → default `inline`).
- **Fail-closed crux confirmed** (the spec's central correctness claim). The abort keys on the `git fetch` / `git remote set-head` return code *before* any `git show`, so a cached/stale `origin/HEAD` cannot mask an unreachable origin (fixture F2 proves it: populate caches → delete the bare origin → still exits non-zero, emits nothing).
- **The lone write is mutation-confirmed both ways** (LEARNINGS #25). `--bootstrap`'s orphan-create fires in the fresh (`¬DOCKET ∧ ¬LIVE`) cell (W2) and provably does NOT fire by default (W1) nor in the STOP_MIGRATE cell (W3) — all asserted against the real bare origin, not a proxy.
- **Real-data smoke test passed** (LEARNINGS #22 — fixture necessary, not sufficient). `scripts/docket-config.sh --repo-dir . --export` against this repo emits exactly `DOCKET_MODE=docket / METADATA_BRANCH=docket / INTEGRATION_BRANCH=main / METADATA_WORKTREE=.docket / CHANGES_DIR=docs/changes / ADRS_DIR=docs/adrs / RESULTS_DIR=docs/results / FINALIZE_GATE=local / FINALIZE_TEST_COMMAND= / BOARD_SURFACES=inline / AUTO_GROOM=false / BOOTSTRAP=PROCEED`, exit 0, 0-byte stderr.
- **Test hygiene:** the hermetic fixtures emitted 16× `warning: You appear to have cloned an empty repository.` to stderr (one per `mkrepo` empty-bare clone); silenced so the suite output is pristine (0-byte stderr).

## Follow-ups

All four Minor review findings were folded into this change (commit `fc2d505`) rather than deferred:
- ✅ **`yaml_get` key now escaped** — ERE metacharacters in the key are escaped before the regex, so a future key with a metacharacter can't match unintended lines. No-op for the current `[a-z_]` keys (the 57-assertion suite confirms no regression). `migrate-to-docket.sh` carries an identical copy but is explicitly out of this change's scope, so it is left as-is; its alignment (or a shared-lib extraction) remains a possible separate change. The `docket-config.sh` comment was updated from "ported verbatim" to "adapted" to keep the record honest.
- ✅ **`--repo-dir` with no argument** now exits 2 with a `docket-config: --repo-dir requires an argument` diagnostic instead of a bare `set -u` "unbound variable" crash (test F5).
- ✅ **W4 (migrated-cell `--bootstrap`)** now asserts `origin/docket` SHA is unchanged — mutation-confirming the idempotent no-op, not just the verdict.
- ✅ **Inline-`#` truncation** documented in the `yaml_get` comment (a value may not contain a literal `#`; harmless for the current enum / path / empty values).

No remaining follow-ups.
