---
id: 153
slug: decide-whether-the-runtime-bash-leaf-match-should-be-depth-a
title: Decide whether the runtime.bash leaf match should be depth-anchored
status: proposed
priority: medium
type: fix
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [133]
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

Docket's `runtime.bash` scanner matches its leaf with `in_runtime && structural ~
/^[[:space:]]+bash[[:space:]]*:/` — any indentation depth under the `runtime:` header, not one
level. So this is read as a valid declaration:

```yaml
runtime:
  nested:
    bash: /some/path
```

It resolves to count 1, value `/some/path`, and becomes the machine's configured Bash.

This is not a regression. The pattern is verbatim from all three pre-existing copies that change
0133 consolidated into `scripts/lib/docket-runtime.sh`, and 0133's reviewers deliberately left it
alone: tightening a grammar inside a strictly behavior-preserving refactor is exactly the silent
caller-rewrite that change existed to avoid. It was ruled acceptable-to-defer, to be fixed by a
change that owns the grammar. This is that change.

The question is genuinely open rather than obviously a bug. Depth-anchoring is the stricter, more
predictable reading and matches how a YAML parser would resolve `runtime.bash`. But a config file
that today resolves would start failing closed as "runtime.bash is not configured" — the resolver's
fail-closed posture means a tightening is a breaking change for anyone whose file has the loose
shape, and it cannot be migrated from the repo side because `runtime.bash` is machine-local by
definition (a repo-committed migration cannot reach the global or machine-local layers).

## What changes

- Decide whether the leaf match should be depth-anchored to exactly one level, and if so whether
  the loose form gets a transitional warning before it fails.
- Apply the decision in `docket_runtime_scan`'s awk program, the one place the grammar now lives.
- Add coverage for the nested-leaf shape either way, so the decision is pinned rather than latent.

## Out of scope

- Adopting `yq` or a general YAML parser — change 0018 remains the place for that.
- The rest of 0133's consolidation.

## Open questions

- Is a hard tightening acceptable, given the value is machine-local and so cannot be migrated by a
  committed change? Or does it need a warn-then-fail transition?
