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
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'docs/relicense-docket-under-the-apache-license-2-0'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-04T17:39:29Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-04-relicense-docket-under-the-apache-license-2-0-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-04-relicense-docket-under-the-apache-license-2-0-design.md) |
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
