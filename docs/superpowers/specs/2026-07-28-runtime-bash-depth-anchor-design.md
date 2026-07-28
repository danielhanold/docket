<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0153 — Decide whether the runtime.bash leaf match should be depth-anchored](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0153-decide-whether-the-runtime-bash-leaf-match-should-be-depth-a.md)**
<!-- docket:backlink:end -->

# Decide whether the runtime.bash leaf match should be depth-anchored — design

Change 0153.

## Problem

`_docket_runtime_scan` in `scripts/lib/docket-runtime.sh` matches its leaf with

```awk
in_runtime && structural ~ /^[[:space:]]+bash[[:space:]]*:/
```

— **any** indentation depth under the `runtime:` header, not one level. `in_runtime` is cleared only
by a column-0 non-space line, so this reads as a valid declaration:

```yaml
runtime:
  nested:
    bash: /some/path
```

It resolves to count 1, value `/some/path`, and becomes the machine's configured Bash.

Not a regression: the pattern is verbatim from all three pre-existing copies change 0133
consolidated, and 0133's reviewers deliberately left it alone — tightening a grammar inside a
strictly behavior-preserving refactor is the silent caller-rewrite that change existed to avoid. It
was ruled acceptable-to-defer, to be fixed by a change that owns the grammar. This is that change.

## The hazard is sharper than "pedantry"

The stub frames this as a strictness question. It is also a **wrong-value** question, and that is
the stronger motivation. The loose match accepts a `bash:` under any *sibling* nested key:

```yaml
runtime:
  codex:
    bash: /opt/weird/bash
```

A user writing that plainly means "the bash for the codex runner." Docket silently adopts it as the
machine's Bash runtime for every docket operation. There is no diagnostic, because from the
scanner's point of view nothing is wrong.

## The migration worry, and why it dissolves

The stub's open question is: *is a hard tightening acceptable, given the value is machine-local and
so cannot be migrated by a committed change? Or does it need a warn-then-fail transition?*

Two facts settle it.

**The loose shape has essentially no installed population.** The only code that writes a runtime
block is `scripts/ensure-global-config.sh`, and it emits exactly one canonical form:
`printf "%s\nruntime:\n  bash: '%s'\n%s\n"` — one level, two spaces. Every install-generated block
is already compliant. The loose shape can only exist in a **hand-authored** file.

(This settles the *read* path only. The installer's own both-declarations guard counts the deep
block today and would stop counting it after the tightening — a separate hole, closed in Decision 3.)

**The real risk was never the tightening; it was the diagnostic.** The stub's stated fear is that a
file which resolves today "would start failing closed as *runtime.bash is not configured*" — a
message that sends the user hunting for a missing key they can see is present. That is the
`config-shape-change-strands-outer-layers` failure mode: a repo-committed change cannot reach the
machine-local layer, so the user must self-diagnose from the message alone.

So the answer is **not** "hard tighten" and **not** "warn-then-fail". It is: **tighten, and make the
rejected shape an explicit, named error rather than an absence.**

## Decision

### 0. The constraint that shapes everything below: every call site is a command substitution

`docket_runtime_unique`, `docket_runtime_count`, and `docket_runtime_first` are **always** invoked as
`"$(...)"` — in `docket-config.sh`, in `ensure-global-config.sh`'s `explicit_runtime` /
`explicit_runtime_count` wrappers, and in `install.sh`. A global set inside `_docket_runtime_scan`
therefore **dies in the subshell** and is unreadable by every existing caller. **Stdout and the exit
code are the only surviving channels.** Any design that relies on a caller reading a new
`DOCKET_RUNTIME_*` variable after a wrapper call is broken before it starts.

### 1. Depth-anchor the leaf to the shallowest structural child of its block

Reject a `bash:` indented **strictly deeper than any earlier structural child of the same
`runtime:` block** — equivalently, anchor on the *minimum* structural-child indentation in the
block, not on the first child's. Anchoring on the first child is wrong: if the first child is
itself the nested key, the anchor is set too deep and a subsequent legitimate one-level `bash:`
would be rejected.

Do **not** hard-code two spaces. A four-space file resolves today (verified), and breaking it would
be a second, unannounced tightening.

**Reset the tracked depth whenever `in_runtime` clears.** `tests/test_docket_runtime_lib.sh` has a
two-`runtime:`-block fixture; without the reset, block 2 inherits block 1's anchor.

Marker and managed-block handling is **not** at risk and needs no change: managed lines `next` out
before `structural` is computed, so depth tracking never sees them. Comment-only lines are blanked
by the existing `sub(/[[:space:]]*#.*/, "", structural)` and blank lines are empty, so neither can
become the anchor.

(No awk snippet is given here deliberately. An earlier draft carried one whose added conjunct was a
tautology — implied by the leaf pattern it was anded with — and shipping a no-op snippet as the
Decision invites a plan that implements the snippet instead of the rule.)

### 2. Report the rejected shape — through stdout and exit codes, on every path

Count too-deeply-nested `bash:` leaves as `DEEP` inside the scan, then surface it:

- **Payload ordering matters.** `_docket_runtime_scan`'s `$( )` capture carries a `printf x` guard
  because `$( )` strips trailing newlines. If `DEEP` is appended **last**, the existing
  `DOCKET_RUNTIME_VALUE="${_raw%$'\n'}"` silently swallows it. Put `DEEP` on line 2 with the value
  terminal, or change the value split from a suffix trim to `%%$'\n'*`.
- **`docket_runtime_unique` returns a distinct code (3)** when `DEEP > 0`. Gate on `DEEP > 0`
  **regardless of `COUNT`** — not on `COUNT == 0 && DEEP > 0` — for the reason in part 3.
- **The two rc consumers must be edited; this is not additive.** `docket-config.sh` has exactly two
  sites, and each hard-codes the meaning of non-zero into its message
  (`".docket.local.yml contains multiple runtime.bash declarations; keep exactly one"` and the
  global-config twin). An unmapped code 3 would emit an **actively false** diagnostic pointing at a
  duplicate that does not exist — strictly worse than the "not configured" message this change
  exists to replace. Branch on 3 explicitly at both sites:
  *"`runtime.bash` must be nested exactly one level under `runtime:`; found it deeper in `<file>`"*.
- **`count` and `first` need a channel too.** They always return 0, and `install.sh` and
  `ensure-global-config.sh` reach the library only through them — so without this, Decision 2's
  promise is delivered for the resolver alone. Give the `explicit_*` wrappers a `DEEP`-aware form
  (a second stdout field, or a direct `_docket_runtime_scan` call outside a subshell). Update
  `install.sh`'s inline comment, which asserts `ensure-global-config.sh` "has just guaranteed
  exactly one authoritative declaration" and goes stale in the same stroke.

### 3. Keep `ensure-global-config.sh`'s both-declarations guard firing

This is the hole a naive tightening opens, and it is on the **write** path.

Today a global config carrying the managed block *plus* a hand-authored deep block yields
`explicit_count = 1` (verified), so `ensure-global-config.sh` hits its hard die:
*"contains both managed and explicit runtime.bash declarations; remove one so exactly one runtime is
authoritative — left unchanged."*

After the tightening that count becomes 0, the guard stops firing, and the installer **rewrites the
managed block over a file whose author declared something else**. The resolver then reads `COUNT=1`
from the managed block, so a `COUNT == 0` gate never fires and no diagnostic appears anywhere. A loud
abort would become a silent override — the exact failure mode this change exists to remove, newly
introduced.

So: the `explicit_*` wrappers must see `DEEP`, and the both-declarations guard must fire on
`DEEP > 0` as well as on `explicit_count > 0`.

### 4. Pin the decision with coverage

`tests/test_docket_runtime_lib.sh` is the host. Add:

- the nested-leaf shape → `COUNT` 0, `DEEP` 1, `docket_runtime_unique` returns 3;
- the sibling-key shape (`runtime:` → `codex:` → `bash:`) → same; it is the motivating hazard;
- a **four-space** canonical file → still resolves, proving the anchor is depth-relative;
- the two-`runtime:`-block fixture with a deep leaf in block 1 only → block 2 unaffected (the reset);
- the managed-block-plus-deep-explicit file → `ensure-global-config.sh` still **dies**, unchanged
  message; this is part 3's regression pin and the most important one;
- the canonical two-space file and the managed-block file → unchanged, byte-for-byte;
- a `bash:` at column 0 outside any `runtime:` block → still ignored.

The repo pins mutations by id (M1–M5 in that file). **Claim a new mutation id** for the anchor and
add it to the mutation table, or it will not be enforced the way the rest of the file is. Reverting
the leaf pattern to the loose form must redden the nested-leaf and sibling-key cases specifically.

### 5. Document the grammar — including two stale sites

In the library header, and:

- **`scripts/docket-config.md`** still claims "`yaml_block_body` isolates each `runtime:` block
  before `bash:` is read" — already stale since 0133, and it is exactly the grammar sentence this
  change rewrites.
- **`scripts/docket-config.md`'s exit-code table** needs a row for the new failure mode.
- `scripts/ensure-global-config.md` wherever `runtime.bash` is described.

State that the leaf must sit exactly one level under `runtime:`, and that a deeper leaf is a
**reported error**, never an absent key.

## Out of scope

- Adopting `yq` or a general YAML parser — change 0018 remains the place for that.
- The rest of 0133's consolidation.
- The `runtime:` header match itself, duplicate-declaration handling, marker/managed-block
  semantics, and the scalar decoder — all unchanged.
- `docket_runtime_validate_bash` and the validator duplication, which change **0152** owns in the
  same file.

## No ADR

A grammar tightening inside one function, with its rationale recorded in the library header beside
the pattern. It changes no cross-cutting rule; the interface addition is one optional return code.

## Assumptions

1. **Tighten, rather than keep the loose match.** *Chosen:* depth-anchor. *Rejected:* leave it —
   the loose form silently adopts a value the user wrote for something else, which is a wrong-value
   bug, not a strictness preference. *Rejected:* accept-with-warning permanently — a warning on a
   path the user may never see leaves the wrong Bash configured.

2. **No warn-then-fail transition.** *Chosen:* tighten in one step, with a naming diagnostic.
   *Rejected:* a release with a deprecation warning followed by a release that fails — this project
   has no release train or version pin; users track a clone, so "two releases" has no meaning here,
   and the transition would be indefinite in practice. The condition that made a transition seem
   necessary — an unreachable machine-local layer — is answered by the diagnostic instead, which
   reaches the user at exactly the moment they are blocked.

3. **The installed population is effectively empty on the READ path — but the WRITE path still
   needs work.** *Chosen:* rely on `ensure-global-config.sh` being the sole writer of the canonical
   one-level form, so no install-generated file's *resolution* changes. *Corrected from an earlier
   draft:* that does not make the installer itself safe. Tightening the count silently disarms its
   both-declarations guard on a hand-authored deep block, turning a loud abort into a silent
   overwrite — see Decision 3, which is a required part of this change rather than a nicety.
   *Rejected:* asserting the affected population is exactly zero — unknowable from inside the repo,
   which is why the reporting in Decision 2 is mandatory rather than optional.

4. **Depth-relative anchoring on the block's shallowest structural child.** *Chosen:* the minimum
   structural-child indentation, reset per block. *Rejected:* requiring exactly two spaces — matches
   what the writer emits and is far simpler, but breaks a hand-authored four-space file that works
   today. *Rejected:* anchoring on the *first* child — the obvious reading, and wrong: when the first
   child is the nested key, the anchor lands too deep and a later legitimate one-level `bash:` is
   rejected.

5. **A distinct return code, and it is NOT additive — two call sites must be edited.** *Chosen:*
   extend `docket_runtime_unique`'s return vocabulary and let callers own the wording, per the
   library's stated rule that "every user-facing diagnostic stays in the caller." *Corrected from an
   earlier draft:* an unmapped code 3 does **not** degrade gracefully. Both rc consumers hard-code
   non-zero to mean "multiple declarations," so leaving them unedited emits a false diagnostic
   naming a duplicate that does not exist. *Rejected:* a global `DOCKET_RUNTIME_DEEP` that callers
   read after the call — every call site is a command substitution, so the variable dies in the
   subshell (Decision 0). *Rejected:* printing from the library. *Rejected:* an opt-in
   `docket_runtime_deep` accessor — a check nobody calls is the silent drop being removed.

6. **Couplings.** `related: [133, 152]`, written to the change file at this groom's exit. Change
   **0152** scopes this leaf grammar out explicitly and states it changes no library token, so there
   is no semantic conflict; its only contact with `scripts/lib/docket-runtime.sh` is the header
   comment. Its test work lands in `test_ensure_docket_env.sh` / `test_ensure_global_config.sh` /
   `test_bash_runtime_routing.sh`, so the shared-host rebase risk in `test_docket_runtime_lib.sh` is
   smaller than an earlier draft claimed. Note that **both** changes touch
   `scripts/ensure-global-config.sh`'s neighbourhood — 0152 for validator routing, this one for the
   both-declarations guard — so keep each edit additive and reconcile at rebase
   (`concurrent-edits-compose-at-rebase`). No `depends_on`: neither gates the other.
