<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0176 — docket-config.sh costs ~0.87s per invocation and dominates test_docket_config.sh](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-01-0176-docket-config-sh-costs-0-87s-per-invocation-and-dominates-te.md)**
<!-- docket:backlink:end -->

# docket-config.sh per-invocation cost — design

## Problem

Change 0174 measured what remained after `test_docket_config.sh` stopped rebuilding a Git fixture
for every assertion group. Fixture construction fell to roughly 4 seconds, but the file still took
about 109 seconds: approximately 105 seconds were 121 real `docket-config.sh` invocations. The
resolver therefore became the largest remaining single test cost after `sync-agents.sh`, while
also taxing every real docket operation because every operating skill reaches it through
preflight.

The stub's first question was whether Git probes, bootstrap work, or config parsing caused the
cost. A representative hermetic local-origin run was profiled before this design was written, per
the `optimization-needs-a-measured-oracle` learning:

| measurement | result |
|---|---:|
| wall clock, five runs | 0.58–0.60s each |
| traced spawned commands | ~359 |
| `sed` | 235 |
| `head` | 77 |
| `awk` | 18 |
| Git | 8 |

The suite's earlier ~0.87-second observation and the isolated ~0.59-second profile differ in
fixture and machine context, but identify the same cause. The resolver is not principally Git- or
bootstrap-bound. It repeatedly reparses three tiny config layers: each successful `yaml_get`
spawns a key-escaping `sed` plus a `sed | head | sed` pipeline, and each nested block is extracted
into another temporary file with `awk` before being scanned again.

## Goal and compatibility boundary

Optimize **real resolver runs**. Faster tests are a consequence, not the primary target.

For stable input files, observable resolver behavior stays byte-for-byte compatible:

- the supported YAML subset and first-match rules;
- layer precedence, defaults, validation, and bootstrap decisions;
- shell and plain export field order and quoting;
- warning order and text, failure diagnostics, stdout/stderr posture, and exit codes;
- the authoritative origin read and every Git freshness probe.

The resolver will read each layer once per invocation. A config file edited during the subsecond
run will therefore resolve from one consistent snapshot rather than potentially mixing old and new
values across repeated reads. Concurrent mid-run mutation has never been a supported contract;
snapshot consistency makes the startup transaction deterministic.

## Approach — immutable layer snapshots with fork-free Bash readers

Load the committed, machine-local, and global config layers into Bash indexed arrays once, after
the existing path/readability policy has normalized ignored layers. Replace the general scalar and
nested-block readers with Bash built-ins over those immutable arrays. Leave policy and validation
at their current call sites.

This approach is deliberately narrower than a new parser. It transfers the existing matching
rules into fork-free operations and does not build a normalized configuration object whose schema
could drift away from the resolver's existing decisions.

Two alternatives were rejected:

- **Memoize the current external readers.** Many `(file, key)` reads are unique, so first-use
  pipelines would still dominate. More importantly, the current API is consumed through command
  substitution; a cache filled inside that subshell does not persist in the parent. Making it
  persist requires eager parent-side work and becomes this design indirectly, while retaining
  avoidable external commands.
- **Batch-parse each layer with one `awk` process.** It offers a marginally lower command count,
  but creates a serialization boundary from untrusted config text back into Bash and moves more of
  the grammar across a language boundary. That is unnecessary semantic risk under the strict
  compatibility requirement.

ADR-0062 remains the governing architecture decision: docket uses in-repo shell readers and takes
no external YAML-parser dependency.

## Components and data flow

### 1. Layer loading

After `GCFG` and `LCFG` have gone through their existing unreadable-file posture and the specialized
`runtime.bash` checks have read the original files, load three slots:

- `committed` from the temporary file populated by `git show origin/HEAD:.docket.yml`;
- `local` from `.docket.local.yml` or the normalized absent layer;
- `global` from `config.yml` or the normalized absent layer.

Each slot records whether the source was a readable regular file and retains its lines verbatim
apart from newline delimiters. Loading uses Bash built-ins; values are never evaluated or used to
construct variable names.

The committed temp file remains part of the existing cleanup trap. The block-specific temporary
files disappear because block queries run directly over the cached layer.

### 2. Flat scalar reader

The replacement for `yaml_get` scans the selected slot's lines in order and returns the first line
with the requested key in the same syntactic shape as today. It preserves:

- indentation through `[[:space:]]` rather than a literal-space class;
- literal key matching rather than treating key characters as ERE operators;
- truncation at the first `#`, including the existing rule that `#` inside quotes is not special;
- trailing-whitespace removal;
- removal of one surrounding pair of single or double quotes;
- an empty result for an absent key; and
- the current non-zero return for a missing scalar-source file.

The public script emits only resolved fields; this helper remains private to
`docket-config.sh`. Call sites retain their existing precedence chains, substituting only a layer
handle for a file-backed read.

### 3. Nested-block reader

A `yaml_block_get`-shaped reader combines today's `yaml_block_body` and `yaml_get` semantics in one
in-memory scan:

1. Strip comments from the structural copy of each line.
2. Enter on a column-zero top-level `<block>:` header with an empty value.
3. Leave on the next non-indented, non-comment content line.
4. Across repeated blocks, return the first matching leaf in encounter order.

This reader serves `skills`, `learnings`, `reclaim`, `build`, and `auto_capture`. A sibling block
key iterator supplies the unknown-`skills` warning pass without materializing a temporary file.
Leaf values use the same scalar normalization as the flat reader.

The specialized `runtime.bash` reader stays file-backed and unchanged. It deliberately implements
different quoting, duplicate-declaration, and structural rules through
`scripts/lib/docket-runtime.sh`; folding it into the general reader would violate the compatibility
boundary and ADR-0062's documented reader split.

### 4. Resolution and emission

All precedence, fence, validation, bootstrap, and emission code remains structurally in place.
The reader returns text to the same assignments; downstream `case` statements and diagnostics
continue to decide validity. `parse_inline_list` and the change-type predicates are not part of
this optimization.

The Git stage is intentionally untouched: fetch, `remote set-head`, authoritative `git show`, and
the bootstrap probes buy freshness and safety. Caching them would change the resolver's contract
rather than merely its cost.

## Failure behavior

The optimized readers introduce no fallback parser and never silently switch to defaults on an
internal load failure. The existing layer posture remains authoritative:

- genuinely absent committed config is represented by the existing empty committed temp file;
- absent local/global files behave as absent layers;
- unreadable local/global paths are warned and ignored exactly once in their existing order;
- malformed values reach the same validators and diagnostics;
- an unreachable origin still aborts before config resolution and emits no config output.

Because the snapshot is immutable for the duration of one run, there is no stale-cache invalidation
path. The resolver never writes any config layer.

## Testing and acceptance

Correctness and performance use separate oracles. A green behavior suite cannot prove that a
performance mechanism is active.

### Behavioral equivalence

- Keep every existing `test_docket_config.sh` assertion unchanged; new cases are additive.
- Add byte-for-byte transcript cases for both export formats, a representative layered success
  with warnings, and representative failure diagnostics.
- Exercise first-match behavior, empty and quoted values, inline comments, space and tab
  indentation, duplicate leaves, repeated blocks, nested-block boundaries, absent layers, and
  ignored unreadable layers.
- Mutation-test the new coverage: bypass a loaded layer, change first-match behavior, and loosen a
  block boundary in turn; each mutation must make its targeted assertion red, and the mutation
  itself must be proven to have landed before the red result is trusted.
- Run the whole repository suite at the build gate.

### Performance oracle

Use one hermetic local-origin fixture for before and after measurements on the same machine:

1. Run at least 20 warmed resolver invocations and compare medians. The optimized median must be at
   least **2× faster** than baseline. An unchanged or slower result is red even if every correctness
   assertion passes.
2. Add and retain a traced spawned-command regression test with a ceiling of **120** commands for the
   representative run, versus the measured baseline of approximately 359. The trace classifier
   must derive which executed command words resolve to external commands at test time; it must not
   maintain an allowlist of parser executable names. This leaves substantial headroom for the
   unchanged Git/runtime work while ensuring a fork-heavy reader cannot silently return.
3. Record before/after resolver median, spawned-command count, `test_docket_config.sh` wall time,
   and full-suite wall time in the results artifact. The latter two are evidence, not absolute
   timing gates.

## Scope

In scope:

- `scripts/docket-config.sh` — layer snapshots and built-in readers;
- `tests/test_docket_config.sh` — equivalence, mutation, and spawned-command coverage;
- `scripts/docket-config.md` — the one-snapshot resolution contract; and
- the build-time results artifact — measured acceptance evidence.

Out of scope:

- caching or removing Git freshness/bootstrap probes;
- reducing the test's resolver invocation count or weakening any assertion;
- changing supported config syntax, precedence, output, warnings, or diagnostics;
- the shared-extractor refactor tracked by change 0179;
- `sync-agents.sh` performance tracked by change 0175;
- fixture reuse completed by change 0174; and
- a parallel suite runner or shell-toolchain policy tracked separately by change 0150.

No new ADR is needed. This is a behavior-preserving implementation optimization within ADR-0062's
existing boundary.
