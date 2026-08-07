---
id: 226
slug: reframe-auto-capture-as-capability-discovery-with-strict-adm
title: Reframe auto-capture as capability discovery with strict admission gates
status: proposed
priority: medium
type: feat
created: 2026-08-06
updated: 2026-08-06
depends_on: [218]
related: [91, 127, 204, 218]
discovered_from: [218]
adrs: []
spec: docs/superpowers/specs/2026-08-06-auto-capture-capability-discovery-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-06-auto-capture-capability-discovery-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-06-auto-capture-capability-discovery-design.md) |
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
