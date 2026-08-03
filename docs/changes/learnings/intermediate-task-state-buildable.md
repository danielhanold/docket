---
slug: intermediate-task-state-buildable
hook: "When a plan splits one function's rewrite across sequential tasks, treat the intermediate state as itself buildable and testable."
topics: [process, plan, tasks]
changes: [45, 191]
created: 2026-07-08
updated: 2026-08-03
promotion_state: retained
promoted_to:
---

## Apply
When a plan splits one function's rewrite across sequential tasks, treat the intermediate (Task N of M)
state as itself buildable and testable — don't assume the earlier task's leftover references are safe
because a later task will delete them.

## War story
- 2026-07-08 (#45, PR #54) — A plan that split multi-harness generation across two tasks left a
  Task-1 seam: Task 1 removed the `PROJECT_AGENT_DIR` variable, but `check_project_level` (only
  rewritten in Task 2) still referenced it, an unbound-variable crash under `set -euo pipefail` that
  would have reddened the `--check` tests had the tasks landed in isolation.
- 2026-08-03 (#191, PR #151) — The same rule from the other direction: the plan task that bumped the
  `BOARD_CHECK_IDS` array and the `--help` header was *ungreensable in isolation*, because the
  suite's doc-drift pins compare that set against `board-checks.md` and `docket-status.md` — which a
  LATER task was scheduled to edit. The worker correctly returned `BLOCKED` and the router reordered
  the doc task ahead of the code task's gate commit. When a suite pins code against docs, the doc
  edit is not follow-up work; it is part of the same buildable unit, and plan ordering must say so.
