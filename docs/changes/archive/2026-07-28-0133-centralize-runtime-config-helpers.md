---
id: 133
slug: centralize-runtime-config-helpers
title: Centralize shared Bash runtime configuration helpers
status: done
priority: medium
created: 2026-07-22
updated: 2026-07-28
depends_on: []
related: [18, 132]
discovered_from: [132]
adrs: [14, 19, 29]
spec: docs/superpowers/specs/2026-07-22-centralize-runtime-config-helpers-design.md
plan: docs/superpowers/plans/2026-07-28-centralize-runtime-config-helpers.md
results: docs/results/2026-07-28-centralize-runtime-config-helpers-results.md
trivial: false
auto_groomable:
branch: feat/centralize-runtime-config-helpers
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/134
blocked_by:
reconciled: true
type: refactor
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-22-centralize-runtime-config-helpers-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-22-centralize-runtime-config-helpers-design.md) |
| Plan | [2026-07-28-centralize-runtime-config-helpers.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-28-centralize-runtime-config-helpers.md) |
| Results | [2026-07-28-centralize-runtime-config-helpers-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-28-centralize-runtime-config-helpers-results.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md) |
<!-- docket:artifacts:end -->

## Why

Change 0132 established a machine-local GNU Bash 4+ runtime, but its parser and validator helpers
now have parallel implementations in install, bootstrap, and config resolution paths. A correction
to one copy can silently leave another path with different configuration behavior.

## What changes

- Add one bootstrap-compatible shared runtime-helper library for the duplicated
  `runtime.bash` parser, declaration counter, serializability check, and GNU Bash 4+ validator.
- Route `install.sh`, `ensure-global-config.sh`, and `docket-config.sh` through that library while
  preserving their current authority, discovery, marker, precedence, and diagnostic policies.
- Add focused helper and caller-level regression coverage, including a mutation check, without
  relaxing the required configured runtime.

## Out of scope

- General YAML-parser adoption, runtime discovery-order changes, or changes to post-install Bash
  4+ enforcement.

## Open questions

None; the shared-mechanics boundary and bootstrap-compatibility requirement are settled in the
linked spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-28

Verified against `origin/main` at `a68b7335`. Change 0132's runtime work has landed and the
duplication this change targets is present and unchanged since drafting — scope holds, no
adjustment needed.

Confirmed duplicate sites, so the build has a fixed target list:

- The scalar-decoding awk function is copied **three** times, byte-for-byte in substance:
  `install.sh` (inline `awk` at the `DOCKET_BASH_PATH` read), `scripts/ensure-global-config.sh`
  (`explicit_runtime`), and `scripts/docket-config.sh` (`runtime_get`).
- Runtime-block traversal (`runtime:` header, dedent-terminates, comment-stripped structural
  line) is copied **five** times: the three above plus `explicit_runtime_count` and
  `runtime_count`.
- GNU Bash 4+ validation exists **twice**: `ensure-global-config.sh:validate_runtime` (POSIX/3.2
  syntax) and `docket-config.sh`'s inline `case`/`[[ ]]` chain at the `DOCKET_BASH_PATH` guard.
- Serializability (no CR/LF) is checked **twice**: `validate_serializable_path` and
  `docket-config.sh`'s inline `case`.

Caller-policy differences the library must NOT absorb, re-confirmed in the current sources: the
installer excludes its own `MARK_OPEN`/`MARK_CLOSE` block while reading an explicit value and
reports a count for its both-declared / empty-declaration diagnostics; the resolver has no marker
concept, treats a duplicate as a hard `die`, and owns repo-local > global precedence plus the
committed-key warning; `install.sh` performs a plain post-bootstrap global read with no
duplicate handling at all. Each stays caller-owned policy.

`scripts/lib/` currently holds four libraries (`docket-frontmatter.sh`, `docket-gitignore-block.sh`,
`docket-preflight.sh`, `docket-root.sh`), all source-only and reached via a `$SELF_DIR`/`dirname`
prefix — the new `docket-runtime.sh` follows that established shape. It is, however, the first
library that must parse under Bash 3.2, since `ensure-global-config.sh` and `install.sh` source it
before a configured runtime exists; the existing four are Bash 4+ only and impose no constraint.

Spec's no-new-ADR judgment stands: the runtime and configuration boundaries are unchanged, and the
bootstrap-compatibility requirement is already recorded as a spec decision.
