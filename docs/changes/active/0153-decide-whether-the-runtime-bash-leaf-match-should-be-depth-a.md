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
related: [133, 152]
discovered_from: [133]
adrs: []
spec: docs/superpowers/specs/2026-07-28-runtime-bash-depth-anchor-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-28-runtime-bash-depth-anchor-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-runtime-bash-depth-anchor-design.md) |
<!-- docket:artifacts:end -->

## Why

`_docket_runtime_scan` matches its leaf with `in_runtime && structural ~
/^[[:space:]]+bash[[:space:]]*:/` — any indentation depth under the `runtime:` header, not one
level. `in_runtime` clears only on a column-0 non-space line, so a `bash:` nested arbitrarily deep
resolves to count 1 and becomes the machine's configured Bash.

The pattern is verbatim from the three copies change 0133 consolidated; its reviewers deliberately
left it alone, since tightening a grammar inside a behavior-preserving refactor is the silent
caller-rewrite that change existed to avoid. It was ruled acceptable-to-defer, to be fixed by a
change that owns the grammar.

**It is a wrong-value hazard, not just laxity.** The loose match accepts `runtime:` → `codex:` →
`bash:` — which a user plainly writes meaning "the bash for the codex runner" — and silently adopts
it as the runtime for every docket operation, with no diagnostic.

## What changes

Depth-anchor the leaf to the block's **shallowest structural child** (not to a hard-coded two
spaces, which would break a four-space file that resolves today; and not to the *first* child, which
lands too deep when the first child is the nested key), resetting per `runtime:` block.

The stub's open question — hard tightening versus a warn-then-fail transition — is answered:
neither. Tighten in one step, and make the rejected shape an **explicitly named error** rather than
an absence. The migration worry was really a diagnostic worry: the feared failure was a working file
suddenly reporting "runtime.bash is not configured," and a message that names the shape and the fix
reaches the user exactly where a repo-committed migration cannot.

Two findings make this larger than a regex edit, both required:

- **Every call site is a command substitution**, so a new global dies in the subshell. Stdout and
  the exit code are the only channels, and the two `docket-config.sh` consumers hard-code non-zero
  to mean "multiple declarations" — leaving them unedited would emit an actively false diagnostic.
- **The tightening silently disarms `ensure-global-config.sh`'s both-declarations guard**, turning a
  loud abort into a silent overwrite of a hand-authored file. The guard must fire on the deep count.

Design settled in the linked spec.

## Out of scope

- Adopting `yq` or a general YAML parser — change 0018 remains the place for that.
- The rest of 0133's consolidation; the `runtime:` header match, duplicate handling, marker/managed
  semantics, and the scalar decoder are all unchanged.
- `docket_runtime_validate_bash` and the validator duplication — change 0152 owns those.
