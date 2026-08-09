---
id: 160
slug: a-committed-too-deep-runtime-bash-lost-its-machine-local-ign
title: A committed too-deep runtime.bash lost its machine-local ignored advisory
status: proposed
priority: low
type: fix
created: 2026-07-28
updated: 2026-08-09
depends_on: []
related: []
discovered_from: [157]
adrs: []
spec:
plan:
results:
trivial: true
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

`docket-config.sh` treats `runtime.bash` as machine-local: repo-local `.docket.local.yml` and the
global `config.yml` are honored, while a value committed in `.docket.yml` is loudly ignored with

> `docket-config: warning: committed config key runtime.bash is machine-local — set it in
> .docket.local.yml or global config.yml; ignored`

That advisory fires on `runtime_count "$CFG"` being greater than zero. Change 0153 depth-anchored
`_docket_runtime_scan`'s leaf match to the shallowest structural child of its `runtime:` block —
correctly, since a deeper `bash:` leaf is not a `runtime.bash` declaration. The side effect is that
a committed `.docket.yml` whose `runtime.bash` is nested *too deep* no longer counts, so the
advisory no longer fires for it.

Nothing resolves wrongly. Resolution was already going to ignore the committed value, and it still
does. What is lost is the *diagnostic*: someone who committed a `runtime.bash` at the wrong depth
now gets silence instead of a message telling them the key is machine-local and where it belongs —
and silence reads as acceptance. Note the asymmetry: the same too-deep shape in
`.docket.local.yml` or the global file dies with a named error (`runtime.bash must be nested
exactly one level under \`runtime:\``), so the committed file is now the one layer where a
malformed declaration is completely quiet.

Found during change 0157's build of 0153 and recorded rather than fixed there, since it is a
separate diagnostic concern from 0153's resolution-correctness scope.

## What changes

Restore an advisory for a committed `runtime.bash` that is present but too deep — either by having
the committed-file probe report depth-rejected leaves separately from accepted ones, or by a
distinct message naming the depth problem. The design question is which of the two existing
messages this should read like (the machine-local advisory, or 0153's depth error) and whether it
should stay warn-only for the committed layer; settle it at grooming.

Whatever shape is chosen must keep 0153's resolution behavior byte-identical — this is about what
gets said, not what gets resolved — and must be covered by a fixture that reddens if the advisory
is removed again.

## Out of scope

- Changing which layers `runtime.bash` resolves from, or the machine-local precedence rule
  (ADR-0019's coordination-key fence and the `runtime.bash` exception both stand).
- Making a committed too-deep `runtime.bash` fatal in the way the local/global layers are. That is
  a posture change for the committed layer, and it would need its own justification.
