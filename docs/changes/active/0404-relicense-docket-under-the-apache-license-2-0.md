---
id: 404
slug: 'relicense-docket-under-the-apache-license-2-0'
title: 'Relicense docket under the Apache License 2.0'
status: 'in-progress'
priority: 'high'
type: 'docs'
created: '2026-09-04'
updated: '2026-09-04'
depends_on: []
stacked_on:
related: [401]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-09-04-relicense-docket-under-the-apache-license-2-0-design.md'
plan: 'docs/superpowers/plans/2026-09-04-relicense-docket-under-the-apache-license-2-0.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'docs/relicense-docket-under-the-apache-license-2-0'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-04T17:54:03Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-04-relicense-docket-under-the-apache-license-2-0-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-04-relicense-docket-under-the-apache-license-2-0-design.md) |
| Plan | [2026-09-04-relicense-docket-under-the-apache-license-2-0.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-04-relicense-docket-under-the-apache-license-2-0.md) |
<!-- docket:artifacts:end -->

## Why

docket ships under PolyForm Noncommercial 1.0.0 plus a bespoke additional-permissions file (change 0401). That combination is not OSI-approved, so every organization that wants to run docket on its own repositories has to route the terms through a legal review or obtain a written grant first, and the individual-exemption and retroactive-scope clauses are custom wording that 0401's own results left as an open legal-review item. docket's value grows with adoption and attribution, not with exclusivity: the tool is a self-hosting reference for an AI-driven change workflow, and the friction of a non-standard license works directly against the people it is for. The Apache License 2.0 is the conventional choice for developer tooling: OSI-approved, pre-cleared at most companies, carries an explicit patent grant, and preserves attribution through its NOTICE mechanism. Relicensing is cheap today because the repository has a single copyright holder and no external contributions.

## What changes

- Replace the body of `LICENSE` with the verbatim Apache License 2.0 text (fetched byte-for-byte from the canonical source, appendix included), dropping the PolyForm text and the pointer line.
- Add a `NOTICE` file carrying the product name and the copyright line, which Apache section 4(d) requires downstream redistributors to preserve.
- Delete `LICENSE-ADDITIONAL-PERMISSIONS.md`; none of its clauses have a role under a permissive license.
- Add `CONTRIBUTING.md` adopting the Developer Certificate of Origin (a `Signed-off-by` trailer on every commit) as the inbound contribution mechanism, plus a pointer to the repo's own change workflow.
- Rewrite the README `## License` section: Apache-2.0 with links to `LICENSE` and `NOTICE`, a sentence that files docket generates in a user's repository belong to that user, a sentence that the license grants no rights in the docket name, a link to `CONTRIBUTING.md`, and the retained statement that the license covers the whole repository history.
- Retarget the license guard (`internal/repoguard/license_test.go`, driven by `tests/test_license_files.sh`) at the new artifacts: the Apache identifier and a distinctive clause pinned in `LICENSE`, the copyright line pinned in `NOTICE`, the additional-permissions file asserted absent, and the README section's links checked as markdown link destinations. Update the wrapper's header comment to match.

## Out of scope

- A contributor license agreement, a CLA bot, or any contribution terms beyond the DCO.
- Per-file SPDX or copyright headers across the source tree.
- Trademark registration or a separate trademark policy document; the README sentence is the whole of it.
- Any change to docket's behavior, the installer, or the skills it ships; this is a documentation-and-guard change.
- A new ADR: the decision is recorded in this change's `## Why` and spec, matching how 0401 recorded the previous license.

## Reconcile log

### 2026-09-04

2026-09-04: Reconciled against origin/main. Confirmed change 0401 is merged and current: LICENSE carries PolyForm Noncommercial 1.0.0 with the Required Notice and the pointer to LICENSE-ADDITIONAL-PERMISSIONS.md; that file exists at the repo root; the README ## License section (lines 123-139) links to both PolyForm files; and internal/repoguard/license_test.go + tests/test_license_files.sh target the PolyForm artifacts. All of the spec's premises hold exactly, so the plan is unchanged: replace LICENSE with verbatim Apache 2.0, add NOTICE and CONTRIBUTING.md, delete LICENSE-ADDITIONAL-PERMISSIONS.md, rewrite the README License section, and retarget the guard test + wrapper header. No scope change, no new dependencies, no ADR (decision recorded in ## Why and the spec, matching 0401).

## Run halted

### 2026-09-04

2026-09-04: Autonomous implement-next run halted at Step 5 (build) — build-profile worker dispatch is unavailable due to intermittent auto-mode classifier denials, and `skills.build` resolves to `docket-build` (not `auto`), so per Dispatch-capability resolution Tier C the build role is abort-and-report rather than inline. A human (or a re-dispatch once the classifier is healthy) can resume; the run is very close to done.

### What is already complete (verified from git)

- **Claim, reconcile, plan** all landed on `docket`: status `in-progress`, `reconciled: true` with a `## Reconcile log` entry, and `plan:` set to `docs/superpowers/plans/2026-09-04-relicense-docket-under-the-apache-license-2-0.md` (committed on the feature branch at plan commit 2844831 with a valid backlink and `Docket-Plan-Path:` trailer).
- **Task 1 file work is done and GREEN-verified.** The feature worktree (`.worktrees/relicense-docket-under-the-apache-license-2-0`, branch `docs/relicense-docket-under-the-apache-license-2-0`) holds, uncommitted:
  - `LICENSE` replaced with the verbatim Apache License 2.0 (11358 bytes, byte-identical to https://www.apache.org/licenses/LICENSE-2.0.txt).
  - `NOTICE` (new) and `CONTRIBUTING.md` (new), matching the spec blocks.
  - `LICENSE-ADDITIONAL-PERMISSIONS.md` staged for deletion.
  - `internal/repoguard/license_test.go` with the retargeted `TestLicenseFiles` (+ `errors`/`io/fs` imports); `readRepoFile` and `TestLicenseReadmeSection` untouched.
  - The controller ran `TestLicenseFiles` through the native gate driver (`gate.drive.start --owner task`) at this tree and observed **PASSED**. gofmt is clean per the prior worker.
  - These files are **uncommitted** — the prior worker returned BLOCKED without committing (correct), and the controller must not adopt/commit a worker's files.

### What remains

- Commit Task 1 (its paths are placed), then Task 2 (README `## License` rewrite + retarget `TestLicenseReadmeSection`) and Task 3 (wrapper header comment rewrite + the 7-row mutation table + the full-suite build gate). Then Step 6 review, Step 7 PR.

### Why it halted (root cause)

- The auto-mode classifier is intermittently unavailable/denying. It timed out on the controller's first Bash call, flagged the first Task-1 worker ("SECURITY WARNING … Blocked by classifier") so that worker could not run its focused `go test` and returned BLOCKED, and then **denied the `docket-build-standard` dispatch twice** (the mandated single retry included). Two explicit policy denials establish the profile-worker dispatch as unavailable.
- Separately confirmed and documented as **change 0405**: the `gate.drive.prepare-scope` -> `gate.drive.start` handshake rejects a build-task worker's own focused gate with `invalid-request` (one-start-per-scope). The blessed workaround (per 0405, used cleanly by the change-402 build on these same license-guard tests) is for workers to run their sub-second focused `go test` directly in the foreground; the controller's own `--owner build` full-suite gate is unaffected. The re-dispatch was written to instruct exactly this workaround, but the dispatch itself was classifier-denied before it could run.

### Resume instructions

Re-dispatch `docket-implement-next` for change 404 once the classifier is healthy. Task 1's edits are already in the worktree and verified GREEN — a resuming worker should inspect/own the inherited uncommitted files, run focused tests via plain `go test` (per 0405), commit Task 1, then complete Tasks 2 and 3, and the controller runs the full-suite gate via `gate.drive.start --owner build`. No metadata damage; branch and workspace are intact.
