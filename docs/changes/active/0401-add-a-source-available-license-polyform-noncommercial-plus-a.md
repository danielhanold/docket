---
id: 401
slug: 'add-a-source-available-license-polyform-noncommercial-plus-a'
title: 'Add a source-available license: PolyForm Noncommercial plus an individual commercial exemption'
status: 'implemented'
priority: 'medium'
type: 'docs'
created: '2026-09-03'
updated: '2026-09-04'
depends_on: []
stacked_on:
related: []
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-09-03-add-a-source-available-license-polyform-noncommercial-plus-a-design.md'
plan: 'docs/superpowers/plans/2026-09-03-add-a-source-available-license-polyform-noncommercial-plus-a.md'
results: 'docs/results/2026-09-04-add-a-source-available-license-polyform-noncommercial-plus-a-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'docs/add-a-source-available-license-polyform-noncommercial-plus-a'
pr: 'https://github.com/danielhanold/docket/pull/275'
blocked_by:
reconciled: true
claimed_at: '2026-09-04T01:23:36Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-03-add-a-source-available-license-polyform-noncommercial-plus-a-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-03-add-a-source-available-license-polyform-noncommercial-plus-a-design.md) |
| Plan | [2026-09-03-add-a-source-available-license-polyform-noncommercial-plus-a.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-03-add-a-source-available-license-polyform-noncommercial-plus-a.md) |
| Results | [2026-09-04-add-a-source-available-license-polyform-noncommercial-plus-a-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-09-04-add-a-source-available-license-polyform-noncommercial-plus-a-results.md) |
<!-- docket:artifacts:end -->

## Why

The repository has no license, so every commit is all-rights-reserved and nobody has a stated grant to use docket at all. The owner wants docket free for personal use and for individuals working alone, while commercial use by organizations requires explicit written permission or a granted license. No single standard license expresses that split: PolyForm Noncommercial 1.0.0 is the closest lawyer-drafted fit (noncommercial use free, any commercial use needs a separate license), and a short separately-authored additional-permissions notice closes the gap for freelancers and solo consultants. The license must also be stated to cover the whole history, including every commit made before the file was added; the owner authored all human commits and the rest are AI-assisted output owned by the owner, so no third-party consent is needed.

This is a license, not terms of service: a ToS would only apply if docket were offered as a hosted service. Adding it makes docket source-available, not open source.

## What changes

- `LICENSE` at the repo root: the unmodified PolyForm Noncommercial 1.0.0 text, preceded by the Required Notice line the license asks for (copyright Daniel Hanold).
- `LICENSE-ADDITIONAL-PERMISSIONS.md` beside it, pointed to from `LICENSE`'s notice area and the README: (1) an individual commercial exemption for a natural person working alone, including through a sole proprietorship or single-member company with no employees or subcontractors; (2) a retroactive-scope clause covering every version and every prior commit; (3) how to obtain a commercial license (contact line).
- `README.md` gains a short `## License` section stating the three rules in plain words and linking both files, with a source-available (not open source) note.
- One shell test under `tests/` guarding that both license files exist, `LICENSE` carries the PolyForm Noncommercial 1.0.0 identifier and the Required Notice, and the README section links to both files.

Exact clause wording lives in the linked spec; the builder must not improvise legal language. The author is not a lawyer; the owner should get a brief legal review of the additional-permissions text before relying on it against a paying organization.

## Out of scope

- Any hosted-service terms of service or privacy policy.
- Per-file license headers / SPDX headers in source files.
- A contributor license agreement or DCO process (no external contributors exist today).
- Changing the `install.sh` / distribution mechanics or the Go module path.
- Choosing or negotiating commercial license pricing or terms; the notice only says how to ask.

## Reconcile log

### 2026-09-04

2026-09-04 — Reconciled against current main. Confirmed the repo still has no LICENSE file at root, README.md ends at the `## Status` section with no `## License` section, and no license test exists under tests/. Scope unchanged: add LICENSE (PolyForm Noncommercial 1.0.0 verbatim + Required Notice + pointer line), LICENSE-ADDITIONAL-PERMISSIONS.md, a README `## License` section, and tests/test_license_files.sh. No dependency or ADR relations required; spec legal wording is authoritative and will not be improvised.
