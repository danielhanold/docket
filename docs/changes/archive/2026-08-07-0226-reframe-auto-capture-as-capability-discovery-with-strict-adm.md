---
id: 226
slug: reframe-auto-capture-as-capability-discovery-with-strict-adm
title: Reframe auto-capture as capability discovery with strict admission gates
status: done
priority: medium
type: feat
created: 2026-08-06
updated: 2026-08-07
depends_on: [218]
related: [91, 127, 204, 218]
discovered_from: [218]
adrs: []
spec: docs/superpowers/specs/2026-08-06-auto-capture-capability-discovery-design.md
plan: docs/superpowers/plans/2026-08-07-auto-capture-capability-discovery.md
results: docs/results/2026-08-07-reframe-auto-capture-as-capability-discovery-with-strict-adm-results.md
trivial: false
auto_groomable:
branch: feat/reframe-auto-capture-as-capability-discovery-with-strict-adm
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/168
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-06-auto-capture-capability-discovery-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-06-auto-capture-capability-discovery-design.md) |
| Plan | [2026-08-07-auto-capture-capability-discovery.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-auto-capture-capability-discovery.md) |
| Results | [2026-08-07-reframe-auto-capture-as-capability-discovery-with-strict-adm-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-reframe-auto-capture-as-capability-discovery-with-strict-adm-results.md) |
<!-- docket:artifacts:end -->

## Why

Auto-capture currently behaves primarily as a suppression mechanism. That suppression is necessary,
but the convention does not actively prompt agents to identify genuine capability discoveries.

The original intent of auto-capture was to preserve valuable new work discovered while processing an
existing change. In practice its language emphasizes reasons not to create a change rather than
directing the agent to search for independently valuable features, capabilities, or improvements.

The updated `implement-next` flow (change 0218) handles findings related to the active change in the
current branch, including defects of any severity. Auto-capture should therefore focus primarily on
discoveries that are outside the scope of the active change and have independent product, workflow,
architectural, or operational value. The existing admission controls stay in place so that review
findings, implementation cleanup, and other work belonging to the active change do not create
follow-up stub churn.

## What changes

Reframe `skills/docket-convention/references/auto-capture.md` as a capability-discovery pipeline
with strict admission gates: lead with the intent to find independently valuable work, name the
discovery categories worth searching for, then gate admission on six criteria. Every existing
suppression rule stays in effect.

Give a captured discovery five required fields — trigger, opportunity, independent value, boundary,
reason for deferral — specified to fit under the leading `## Why` heading that `mint-stub.sh`'s body
contract requires.

State routing per mint site rather than uniformly. Fix-in-branch is available only where an open
branch and a live fix loop exist, so the `docket-finalize-change` / `docket-status` harvest keeps
change 0218's exemption and its own admission bar.

Reframe the convention's inline `### Auto-capture (shared definition)` summary the same way under
progressive disclosure: intent plus mechanics plus the blocking drill-down pointer, with the
categories, gates, capture fields, and routing table living only in the reference.

Raise the reference's size-budget row with the justification block that file requires, re-anchor
change 0218's guard assertions to the new wording, and cover both a qualifying discovery and a
non-qualifying current-branch finding in tests.

## Out of scope

- Relaxing suppression of review findings.
- Changing how `implement-next` fixes findings in the active branch.
- Changing stub-minting mechanics, or modifying `scripts/mint-stub.sh` or `scripts/mint-stub.md`.
- Altering deterministic naming, numbering, or change-creation behavior.
- Turning auto-capture into an implementation or review loop.
- Requiring agents to capture speculative or weakly defined ideas.
- Changing `AUTO_CAPTURE_TYPES` filtering, its ordering before the cap, or the per-invocation cap
  of 3. The capability-discovery framing biases discoveries toward `feat`, so a repo whose
  `AUTO_CAPTURE_TYPES` excludes `feat` will see more policy-suppressed reports — expected, and not
  to be "fixed" during the build.

## Open questions

- Whether the near-byte-neutral rewrite of the convention's summary fits inside its remaining 6
  lines / 46 words of headroom, or whether that budget row also needs a justified raise. Resolve at
  build time by measuring, not by guessing.
- Sequencing against active change 0204, which edits the same two files. Neither blocks the other;
  whichever lands second rebases.

## Reconcile log

### 2026-08-07

Reconciled against `origin/main` at `c34f42dc` and `origin/docket`. No drift — every measurement and
anchor the spec depends on still holds:

- `skills/docket-convention/references/auto-capture.md` measures **51 lines / 544 words** against its
  budget row `55 600` (unchanged), so the raise the spec calls for is still required.
- `skills/docket-convention/SKILL.md` measures **339 lines / 5804 words** against `345 5850` — the
  same 6 lines / 46 words of headroom the spec recorded. The summary reframe stays an in-place,
  approximately byte-neutral rewrite; the open question resolves by measuring at build time.
- Change 0218's guard block is present in `tests/test_docket_review.sh` (the `--- change 0218 ---`
  section), still keying on `current run will fix in-branch … fails the bar`, `harvest … exempt`,
  `no open branch … no fix loop`, the *Materiality bar* section extractor, and a `>= 20` line floor.
  All five survive the rewrite, re-anchored rather than removed; the floor rises with the file.
- Dependency 0226 → 0218 is satisfied (0218 archived `done`, 2026-08-06).
- The file collision with change **0204** is unchanged: 0204 is still `proposed` /
  needs-brainstorm, so nothing of its edit has landed. This build lands first; 0204 rebases.

Scope, out-of-scope, and acceptance criteria carry forward unmodified. No auto-capture candidates
surfaced during this pass.
