---
slug: intermediate-task-state-buildable
hook: "When a plan splits one function's rewrite across sequential tasks, treat the intermediate state as itself buildable and testable."
topics: [process, plan, tasks]
changes: [45]
created: 2026-07-08
updated: 2026-07-08
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
