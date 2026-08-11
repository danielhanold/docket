---
id: 288
slug: namespace-the-remaining-un-namespaced-mock-seams-runners-dir
title: 'Namespace the remaining un-namespaced mock seams (RUNNERS_DIR, GIT) repo-wide'
status: proposed
priority: medium
type: chore
created: 2026-08-11
updated: 2026-08-11
depends_on: []
related: []
discovered_from: [208]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while building change 0208. That change introduced a new mock seam on `scripts/runner-dispatch.sh` and, applying AGENTS.md's rule that "every env var docket introduces is DOCKET_-namespaced", shipped it as `DOCKET_AGENTS_SRC`. Its two sibling seams on the same script, declared on the same `# Mock seams:` line, are still the un-namespaced `RUNNERS_DIR` and `GIT`.

**Opportunity** — bring the pre-existing mock seams under the namespace the rule already states, so the seam surface is uniform rather than split by the date each seam was added. The concrete hazard is not stylistic: a bare `GIT` or `RUNNERS_DIR` exported by any surrounding tool or harness is silently honored by a docket dispatch, which is precisely why the namespace rule exists. A whole-repo sweep is the unit of work — the same shape appears on other scripts, not only the facade.

**Independent value** — stands with change 0208 reverted; the seams predate it and the collision risk is theirs, not its.

**Boundary** — rename the un-namespaced seams docket owns, repo-wide, deriving the site list from a grep rather than a hand list; update each script's `# Mock seams:` header, its co-located `.md` contract, and every test that sets one. Accept a transition window that honors the old name with a deprecation warning if any is documented for external use. It leaves alone: seams docket does not own, and any variable a third-party tool defines.

**Reason for deferral** — a repo-wide rename touching several scripts, their contracts, and their test fixtures is its own reviewable diff. Folding it into a delegation-gate branch would bury a mechanical sweep inside a behavioral change and make both harder to review.
