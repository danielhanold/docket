---
slug: idempotency-keying
hook: "Key a nothing-to-do probe on the state you PROMISED (it reached the remote), never on a local proxy a half-completed run also leaves behind."
topics: [design, idempotency, git]
changes: [11, 71]
created: 2026-06-16
updated: 2026-07-14
promotion_state: retained
promoted_to:
---

## Apply
Key a "nothing to do" probe on the state you actually PROMISED (it reached the remote), never on a
local proxy — clean tree, no diff, a stored field — because the proxy is precisely what a half-completed
run leaves behind, and the probe then certifies the failure as success. (Fixed by also counting unpushed
commits touching the path, `git rev-list --count @{u}..HEAD -- <path>`.) A script that reads change files
must read the metadata working tree (guard the pruned tree) and is idempotent only via the orchestrating
pass's write-back — drive it through that pass, never bare; and when a create-and-set-state pass mints an
id, key the state write on the EFFECTIVE id (existing OR just-minted), not the stored field.

## War story
- 2026-06-16 / 2026-07-14 (#11 PR #11; #71 PR #81 — merged, one idempotency-keying family) — Three
  no-op/idempotency probes, each keyed on a proxy that a PARTIAL FAILURE also satisfies. #11's
  derived-surface mirror keyed idempotency on a persisted change-file field but did no git writes
  itself, so a bare run (outside the orchestrating pass that records the field) re-created every item —
  and it read the integration checkout where `active/` is pruned, so it only saw archived changes; its
  first-sync close-state keyed on the *pre-existing* id field (empty on a fresh mint), so an
  already-terminal item was created open and closed only on a later pass. #71 found the same shape in
  the board orchestrator: `board inline clean` was keyed on a CLEAN WORKING TREE, but after a failed
  push the board commit already exists locally and the tree is clean — so the must-land remedy
  (re-invoke) re-rendered, found no diff, and reported the terminal-success line while the board had
  never reached the remote.
