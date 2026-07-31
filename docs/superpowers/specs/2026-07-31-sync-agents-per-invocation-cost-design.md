<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0175 — sync-agents.sh costs ~5.5s per invocation and dominates the test suite](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0175-sync-agents-per-invocation-cost.md)**
<!-- docket:backlink:end -->

# sync-agents.sh per-invocation cost — design

## Problem

A single `sync-agents.sh` run costs ~5.5s regardless of what it is asked to do. Three test files
that exercise it (`test_sync_agents.sh` 197.8s, `test_sync_agents_codex.sh` 66.8s,
`test_sync_agents_cursor.sh` 14.3s) account for 279s of a 530s suite — 53% of total wall clock.

The stub asked where the 5.5s goes. It was measured before this design was written, per the
`optimization-needs-a-measured-oracle` learning (an estimate written from a plausible per-unit cost
is a guess with a spec's authority).

**Measured 2026-07-31**, `bash sync-agents.sh --help` = 5.654s wall clock. `bash -x` trace of that
same run:

| spawned command | count |
|---|---|
| `sed` | 976 |
| `head` | 770 |
| `awk` | 477 |
| `grep` | 204 |
| **total forks** | **~2,430** |

At the ~2ms/fork this machine sustains, that is essentially the entire 5.5s. The cost is **not**
git, **not** I/O, and **not** the config resolver. It is fork-bound YAML field parsing in pure
shell.

The redundancy is structural:

| helper | calls per run | distinct inputs |
|---|---|---|
| `harness_agent_line` | 192 | ~6 (3 layer files × ~2 harnesses) |
| `section_body` | 391 | ~6 |
| `field_of` | 576 | ≤ 192 lines |

`harness_agent_line` re-parses an entire layer file from scratch on every (harness, agent, layer)
triple — five forks each (`section_body agents`, `section_body <harness>`, `sed`, `grep`, `head`) —
producing roughly **32× redundant work over ~6 distinct parses**. `field_of` then forks `sed` +
`head` per field.

## Precondition — change 0173 lands first

0173 fixes a value-class defect in `field_of()`: the class `[A-Za-z0-9._-]+` silently truncates a
model ID containing `/` or `:`. It widens the class to `[^,}[:space:]]+` and adds a `field_of_raw`
companion plus a bare-scalar validator, mirroring `harness-defaults.sh`'s `hd_field`/`hd_field_raw`
pair.

This spec was written against the pre-0173 tree, so two items below are stale on their face and must
be reconciled at build time:

- The ERE this design carries forward into `[[ $line =~ ... ]]` must be the **widened** one. Both
  `sed -nE` and bash `[[ =~ ]]` are POSIX ERE, so it transfers without reformulation.
- The `field_of` equivalence test must be written against the **fixed** baseline. Written against
  the pre-0173 behavior it would pin the truncation bug as expected output.
- `field_of_raw` is a second function the memoization pass must carry across, on the same terms.

`depends_on: [173]` records the ordering.

## Goal

**Real-run speed**, not suite speed. Making generation itself cheaper pays out for every run a
human, an `install.sh`, or a skill triggers — and the suite improvement follows for free. A
`--help` fast path would have bought only the suite, and only partially.

## Approach — parse-once memoization

Cache each layer file's parsed harness body once per run, and do field extraction with bash
builtins. **The precedence logic is not touched**, which is what lets the existing test suite serve
as the correctness oracle.

Two alternatives were considered and rejected:

- **Fork-free helpers with no cache** — replace `sed`/`awk` with builtins but keep re-parsing per
  call. Keeps 192 whole-file walks; shell line-loops are slow enough that the win is capped and may
  be small, for comparable behavioral risk.
- **A single awk resolver pass** emitting a flat `harness agent field value` table. Fewest forks
  and the cleanest boundary, but it reimplements the precedence rules (harness-beats-default within
  a layer, first-layer-wins across layers, `RES_MODEL_FROM_HARNESS`) in awk — moving currently
  tested logic across a language boundary for a marginal gain over memoization.

### Components

**1. `_layer_body` cache.** A `declare -A` map keyed `<file>\x1f<harness>`, value = the dedented
body under `agents.<harness>` (for a layer read under a top-level `agents:` wrapper) or the
whole-file equivalent (global layer). Populated on first touch, reusing the **existing**
`section_body` awk unchanged. Roughly 6 entries per run.

The array is declared at **file scope**, not lazily initialized inside a function. This is not
stylistic: change 0174's first implementation used lazy init inside a helper that two callers
consumed via `read -r W _ < <(new_repo)` — a subshell — so the assignment never reached the parent,
every fixture was rebuilt, the suite ran green, and the result was *slower than baseline*. Record
that reason in-file so it is not "simplified" back.

`declare -A` is already the repo idiom (7 scripts use it) and the enforced floor is bash major 4.

**2. `harness_agent_line`.** Becomes: cache lookup, then an in-shell scan of the cached body for
the first line matching `^[[:space:]]*<agent>[[:space:]]*:`. Replaces five forks per call with
zero. An absent file stores the empty string, which is indistinguishable from today's
`[ -f "$1" ] || return 0`.

**3. `field_of`.** Becomes `[[ $line =~ <ERE> ]]` plus `BASH_REMATCH[1]`, replacing `sed` + `head`.
The existing ERE transfers verbatim:

```
.*[{,[:space:]]FIELD[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*
```

Both `sed -nE` and bash `[[ =~ ]]` are POSIX ERE with a greedy leading `.*`, so last-match-wins
semantics are preserved. **This equivalence is a verification task, not an assumption** — see
Testing.

> **Collision with change 0173.** That change fixes a real defect in *this exact pattern*: the
> class `[A-Za-z0-9._-]+` silently truncates a provider-prefixed model ID (`anthropic/claude-opus-5`
> → `anthropic`). The two changes edit the same line of the same function for different reasons and
> are not ordered by a dependency, so whichever builds second **must reconcile rather than
> overwrite**: this change ports the character class verbatim, so if 0173 lands first the ported
> ERE is 0173's widened class, not the one quoted above. Compose, do not choose — the two intents
> (fork-free extraction, correct class) are independent and both must survive.

**4. Arg validation.** Today any unrecognized argument falls through into a full generation pass
that writes wrapper files. That is a correctness bug independent of speed: `--help` currently
*generates*. Add a real flag parser — `--help` prints usage and exits 0 without generating; an
unrecognized flag exits non-zero with a diagnostic. This also removes the misleading
`sync-agents.sh --help` benchmark from the record.

### Data flow

`resolve_agent_layers` keeps its exact loop, layer precedence, and `RES_MODEL_FROM_HARNESS`
behavior. It calls cheaper helpers; it does not change shape.

### Error handling

The one new failure mode would be a stale cache. It cannot occur: layer files are not written
during a generation run. Cache misses on absent files behave exactly as today.

## Testing and acceptance

Correctness and speed need different oracles. A performance change is the one change the suite
cannot judge — every existing assertion passes identically whether the optimization happened or was
silently inert.

**Correctness.** The existing `test_sync_agents.sh`, `test_sync_agents_codex.sh`, and
`test_sync_agents_cursor.sh` pass **unchanged**. No assertion may be edited to accommodate the
refactor; an edited assert is a red flag, not a fix. If an assert must change, the refactor changed
behavior and that is the finding.

Add explicitly:

- A `field_of` equivalence test over the ERE's edge cases — inline-map form (`{model: x, effort: y}`),
  block form, a value containing `.`/`_`/`-`, a field name that is a prefix of another, and a line
  where the field appears twice (last-match-wins).
- A **tab-indented layer file** case. The `shell-portability` learning's item (b) was exactly this
  class in exactly this file: `ind()` used `[^ ]`, so a tab-indented config layer was silently
  dropped.

**Speed.** Two parts:

1. Before/after wall clock for a `sync-agents.sh` generation run and for each of the three test
   files, recorded in the results file as the change's acceptance number. An unchanged or worse
   number is a **red** result even with every assertion green.
2. A **standing fork-count assert**: a single generation pass must stay under a fork ceiling. This
   is the only oracle that goes red if the cache is later broken or removed — the direct descendant
   of change 0174's retained template-integrity probe, which was the sole signal that caught its
   inert implementation. Set the ceiling with headroom above the measured post-change count so
   ordinary edits do not trip it, but far below the ~2,430 baseline.

## Out of scope

- **`test_render_board.sh`** (17.8s over ~163 `render-board.sh` invocations). A different script an
  order of magnitude smaller, and not yet shown to have the same cause. The stub's open question is
  resolved by excluding it; file a stub only if the memoization idiom is shown to transfer.
- **`docket-config.sh`** — change 0176 (~105s across 121 invocations). Kept independent and
  cross-linked. This spec records the measured cause here so 0176's grooming can check whether the
  same fork-bound-field-parsing cause applies and reuse the idiom if it does, rather than
  re-deriving it.
- **A parallel or unified suite runner** — change 0150 records that gap.
- **Toolchain pinning across the suite** — change 0150.
