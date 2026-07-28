<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0152 — Consolidate the two surviving hand-rolled GNU Bash 4+ validator copies](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0152-consolidate-the-two-surviving-hand-rolled-gnu-bash-4-validat.md)**
<!-- docket:backlink:end -->

# Consolidate the two surviving hand-rolled GNU Bash 4+ validator copies — design

Change 0152.

## Problem

Change 0133 centralized docket's `runtime.bash` mechanics into `scripts/lib/docket-runtime.sh` and
routed `install.sh`, `scripts/ensure-global-config.sh`, and `scripts/docket-config.sh` through it.
Its whole-branch review then found the GNU Bash 4+ **validator** still has two more independent
hand-rolled copies, deliberately untouched because 0133's spec scoped the work to those three
callers:

- `scripts/docket.sh` — a POSIX-syntax version check in its bootstrap prologue.
- `scripts/ensure-docket-env.sh` — its own `DOCKET_BASH_PATH` validation.

Three implementations of "is this an absolute executable GNU Bash 4+", not one. A future correction
to the version grammar still lands in only one of the three. 0133 narrowed its own prose so nothing
claims otherwise — the false claim is fixed; the duplication is not.

## The open question, answered

The stub asks: *can `scripts/docket.sh`'s prologue source a library at all, given it deliberately
runs before the configured runtime is resolved — and if it can, does that weaken the prologue's
purpose?*

**It should not, and the reason is a silent wrong answer rather than a parse failure.** The
obvious argument — "the library is Bash, the prologue is `sh`, it won't parse" — is **false**, and
must not be the one recorded: the library is `BOOTSTRAP-COMPATIBLE BY REQUIREMENT` (its own header:
every line must run under macOS's system Bash 3.2.57), `local` is a `dash` builtin, and sourcing it
from `/bin/sh` and from `dash` both work. A builder who tests the easy argument will find it wrong
and may consolidate anyway.

The real blocker:

- **`$'\n'` degrades to a literal under a non-bash `/bin/sh`.** `docket_runtime_validate_bash`
  splits its two-line payload with `_first="${_version%%$'\n'*}"`. Under `dash` that strips
  nothing, so the payload's second line becomes the entire multi-line `--version` blob — the
  documented contract is broken **while the function still returns 0**. Not a crash; a wrong answer
  that looks right. (`_docket_runtime_scan` has the same exposure: `DOCKET_RUNTIME_COUNT` becomes a
  two-line blob and the caller's `-le 1` test errors.) The prologue must be correct on a host whose
  `/bin/sh` is `dash`, which is exactly the class of host it exists for.
- **The prologue has no way to find the library.** Under `sh` there is no `${BASH_SOURCE[0]}`, and
  `SELF_DIR` is not defined until *after* the `exec`. Sourcing would require inventing a separate
  `$0`-based dirname resolution in the one place that must stay minimal.

The prologue already shows it knows the first hazard: it writes `sed -n '1p'` where the library
writes `${_version%%$'\n'*}` — the same operation, in syntax that is correct under any `sh`.

So `scripts/docket.sh`'s copy is **a bootstrap exception to be documented, not a duplicate to be
removed**. `scripts/ensure-docket-env.sh`'s copy is a genuine duplicate and goes.

## Decision

### 1. Route `scripts/ensure-docket-env.sh` through the library

It is `#!/usr/bin/env bash` and lives in `scripts/`, so it can source the library the way its
siblings do — but **use its own path variable**: this file defines `HERE`, not `SELF_DIR`, and it
runs `set -uo pipefail`, so a copied `$SELF_DIR` is an unbound-variable abort on the consolidation's
first line. The correct form is `. "$HERE/lib/docket-runtime.sh"`. (The two precedents differ from
each other — `install.sh` uses `$SCRIPT_DIR/scripts/lib/…`, `ensure-global-config.sh` uses
`$SELF_DIR/lib/…` — so neither can be copied verbatim.)

Capture the validator's two-line payload with the guarded idiom `docket-config.sh` already uses,
never a bare `$( )`:

```sh
_probe="$(docket_runtime_validate_bash "$BASH_VALUE"; printf 'x')"; _probe="${_probe%x}"
```

Keep the guarded idiom for consistency with `docket-config.sh`, but do not overstate it: this caller
reads only line 1 (all five `die` strings interpolate `$BASH_VALUE`, never the version), so nothing
here depends on the guard. It matters where a caller consumes line 2, which is why the library
documents it.

**One more duplicate in the same file, and it should go with them.**
`ensure-docket-env.sh`'s `validate_literal_path` opens with
`case "$1" in *$'\n'*|*$'\r'*)` — which is `docket_runtime_serializable` verbatim. A change whose
Problem statement is "three implementations, not one" should not consolidate five lines and leave an
exact fourth copy untouched two functions away. **Fold it**, following the precedent one file over
(`ensure-global-config.sh`'s `validate_serializable_path(){ docket_runtime_serializable "$1"; }`):

```sh
validate_literal_path(){ docket_runtime_serializable "$1" || die "$2 contains unsupported line-break characters"; }
```

— detection in the library, the label-bearing message caller-side, same as part 1's mapping.

Its five hand-rolled lines — absolute-path case, `-x`, `--version` capture, the
`'GNU bash, version '*` banner match, and the `>= 4` major parse — are byte-equivalent to
`docket_runtime_validate_bash`.

The fit is exact by design: the library "returns a machine-readable reason token instead of printing
a message" precisely so "every user-facing diagnostic stays in the caller." So the consolidation
keeps every existing `die` string; only the *detection* moves.

**The mapping is 1:1 and total — no library change is needed.** Verified: the library exposes five
tokens and `ensure-docket-env.sh` emits five messages.

| token | existing `die` string, preserved verbatim |
|---|---|
| `not-absolute` | `DOCKET_BASH_PATH must be an absolute path` |
| `not-executable` | `DOCKET_BASH_PATH is not executable: $BASH_VALUE` |
| `no-version` | `DOCKET_BASH_PATH cannot report its version` |
| `not-gnu-bash` | `DOCKET_BASH_PATH is not GNU Bash` |
| `old-major` | `DOCKET_BASH_PATH must be Bash 4 or newer` |

The library already separates banner failure from major failure, and the one collapse it does make
(an unparseable major folded into `old-major`) is one `ensure-docket-env.sh` already makes too. A
five-way caller-side `case` is proven in `docket-config.sh`. **This change therefore touches
`scripts/lib/docket-runtime.sh` not at all**, which is also what keeps it disjoint from change 0153.

**The real hazard here is that "no user-visible text changes" is currently unfalsifiable.**
`tests/test_ensure_docket_env.sh` asserts none of the five strings — its only negative cases are a
relative path and a newline path, both exit-code-only. **Pin all five messages before moving the
detection**, so the consolidation is provably message-preserving rather than assumed to be.

### 2. Document `scripts/docket.sh`'s prologue as a deliberate exception

Three documentation sites, not two — and the third is the one that goes actively **false**:
`scripts/lib/docket-runtime.sh`'s own header currently says independent checks "still live in
`scripts/docket.sh` … and `scripts/ensure-docket-env.sh`" and that folding them in is "out of this
library's current scope." After part 1 that is a stale claim of exactly the kind 0133 was credited
with fixing, so it must be corrected in this change.

In the prologue's comment block, in `scripts/docket.md`, and in that library header: this validator
is intentionally
duplicated, it is POSIX `sh` by necessity, it must **not** source the Bash library (the library
parses fine under `dash` but its `$'\n'` payload split silently degrades there), and **any change to
the version grammar must be applied here too**. State the obligation, not just the fact — an
exception documented without its maintenance rule is how the next grammar fix misses this copy
anyway.

Add a guard that makes the obligation mechanical rather than aspirational: assert both surviving
implementations recognise the same banner shape and the same major-version floor, so a grammar
change to one without the other reddens. Anchor it on both files' actual behavior (invoke each with
a fake `bash` fixture), not on a source-text comparison — the two are written in different dialects
and always will be.

**Extend existing machinery, do not build new.** `tests/test_bash_runtime_routing.sh` already drives
the prologue with fake fixtures and already handles the asymmetry: for `docket.sh`, *accept* means
`exec`ing into the fixture, so an accepted fixture must be a delegating wrapper
(`exec "$REAL_BASH" "$@"`) while a rejected one is a pure banner-printer. It already asserts the
3.2.57 rejection. `tests/test_docket_runtime_lib.sh` already builds
`good`/`exactly4`/`legacy`/`notbash`/`weird`/`noexec` fixtures against the library directly.

**Name the host file for the equivalence guard.** Neither existing file spans both sides:
`test_bash_runtime_routing.sh` drives the facade but never sources the library;
`test_docket_runtime_lib.sh` sources the library but never invokes `docket.sh`. The guard needs one
host that does both — extend `test_bash_runtime_routing.sh` to also source the library and compare
verdicts over a shared fixture set, since the prologue side is the harder half and already lives
there.

### 3. Extend the mutation coverage

The stub's third bullet: removing the Bash-major check must redden a test through **every**
surviving caller, not only the three 0133 already covers. After this change the callers are
`install.sh`, `ensure-global-config.sh`, `docket-config.sh`, `ensure-docket-env.sh` (all via the
library) and `docket.sh` (its own copy). Two mutations:

- Break `docket_runtime_validate_bash`'s major-version test → all four library callers redden.
  **Two of the four do not redden today**, and both must be fixed here or the stub's bullet is not
  delivered: `ensure-docket-env.sh` (below) and **`ensure-global-config.sh`**, whose test builds a
  single fixture hardcoding `GNU bash, version 5.2.0(1)-release` with no Bash-3 or non-GNU negative
  case at all. `install.sh` and `docket-config.sh` are genuinely covered already.
- Break `docket.sh`'s prologue major-version test → `docket.sh` reddens, and the part-2 equivalence
  guard reddens.

**Routing `ensure-docket-env.sh` through the library does NOT by itself give it coverage — this is
the change's actual point and it is easy to miss.** That file has no test exercising a Bash-3 or
non-GNU runtime at all: its fixture hardcodes `GNU bash, version 5.2.0(1)-release`, and its only
negative cases are a relative path and a newline path. Post-consolidation, breaking the library's
major check would leave `tests/test_ensure_docket_env.sh` **fully green**. So the change must add
negative fixtures there — a 3.2.57 fake and a non-GNU fake — each asserting the preserved `die`
string. Without them the stub's third bullet is not delivered.

Work in `tests/test_docket_runtime_lib.sh`, `tests/test_bash_runtime_routing.sh`,
`tests/test_ensure_docket_env.sh`, and `tests/test_ensure_global_config.sh` (a 3.2.57 fixture).
Deliberately **not** `tests/test_docket_config.sh`, which several active changes are editing and
which needs no edit for this mutation.

## Out of scope

- **Behavior** changes to 0133's three existing callers, and the library's interface (no token is
  added or changed) and its Bash 3.2 compatibility requirement. **Adding missing mutation coverage
  for those callers is explicitly in scope** — `ensure-global-config.sh`'s validator is
  library-routed but untested against a legacy runtime, and the stub's third bullet demands every
  surviving caller redden.
- `docket_runtime_scan`'s leaf-match grammar — change **0153** owns that, and edits the same
  library file at a different function.
- Any change to which interpreter docket runs under.

## No ADR

The one decision worth recording — `docket.sh`'s prologue is a permanent bootstrap exception — is a
per-file constraint, not a cross-cutting rule, so it belongs in that file's comment and in
`scripts/docket.md` where a maintainer will meet it. If the equivalence guard in part 2 turns out to
need a general policy about duplicated bootstrap validators, that is the ADR, and it is not needed
for two.

## Assumptions

1. **`docket.sh`'s copy stays, documented.** *Chosen:* exception, on the `$'\n'`-degrades-silently
   ground, not on a parse-failure ground. The easy argument is empirically false — the library
   sources and runs under both `/bin/sh` and `dash`, and is required to (its header pins Bash 3.2.57
   compatibility) — so recording it would invite a builder to disprove it and consolidate anyway.
   *Rejected:* extracting a POSIX-`sh` subset library both could source — a fourth file and a second
   dialect to de-duplicate five lines, and the prologue would still need its own `$0`-based dirname
   resolution to find it. *Rejected:* making the prologue call `docket-config.sh` — that resolves
   config, which is exactly what the prologue precedes.

2. **`ensure-docket-env.sh`'s copy goes.** *Chosen:* route through the library. It is already Bash,
   already in `scripts/`, and two sibling scripts already source the library from the same relative
   path. *Rejected:* leaving it as a second documented exception — there is no mechanical reason for
   it, and "documented exception" would then mean "we did not get to it."

3. **Diagnostics are preserved verbatim — and pinned first.** *Chosen:* keep all five `die` strings
   and **add asserts for them before moving the detection**, because the suite currently asserts
   none of them, which makes "message-preserving" unfalsifiable exactly where it matters.
   *Verified moot:* the token-widening worry an earlier draft raised — the library already
   distinguishes `not-gnu-bash` from `old-major`, the mapping is 1:1 across all five, and no library
   edit is needed. *Rejected:* adopting whatever granularity the tokens happen to impose — the
   consolidation failure mode where a shared source silently rewrites the caller that differed
   (`consolidation-flattens-caller-variance`); it simply does not arise here.

4. **A behavioral equivalence guard, not a source-text one.** *Chosen:* drive both implementations
   with fake `bash` fixtures and compare accept/reject verdicts. *Rejected:* comparing the two
   sources, or asserting both contain a shared literal — they are written in different shell
   dialects on purpose, so any text-level assertion pins the wrong property and would break on a
   legitimate rewrite of either.

5. **The guard covers banner shape and major floor, not every reason token.** *Chosen:* pin the two
   properties a grammar change would actually move. *Rejected:* full token-by-token equivalence —
   the prologue emits prose and exits, the library emits tokens; forcing them to agree on the whole
   vocabulary would push the prologue toward the library's interface, which is the coupling this
   design is deliberately avoiding.

6. **Couplings.** `related: [133, 150, 153]`. Change **0153** owns `_docket_runtime_scan`'s awk
   grammar in `scripts/lib/docket-runtime.sh`; this change's only contact with that file is its
   **header comment** (part 2), which cannot semantically conflict with a grammar edit — disjoint by
   content, not by absence of contact. Change **0150** is ungroomed, suite-wide, and one of its
   candidate shapes is a `tests/lib` prelude sourced by every test file, which would reach all four
   files chosen above; no *specced* change edits them today, but 0150 is the reason to keep the
   link. `tests/test_docket_config.sh` is deliberately avoided — changes 0148, 0149, 0151, 0125, and
   potentially 0150 all touch it, and nothing here needs it. No `depends_on`: none of these gates
   the other.
