<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0173 — field_of() silently truncates a model ID containing / or :](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-31-0173-field-of-silently-truncates-a-model-id-containing-or.md)**
<!-- docket:backlink:end -->

# field_of() value-class truncation — design

## Problem

`field_of()` in `sync-agents.sh` extracts a config value with the character class
`([A-Za-z0-9._-]+)`. The class contains neither `/` nor `:`, so a provider-prefixed model ID is
silently truncated to its first segment:

```yaml
# .docket.local.yml, .docket.yml, or the global config.yml
agents:
  default:
    docket-implement-next: {model: anthropic/claude-opus-5, effort: high}
```

resolves to `anthropic`, and the generator bakes that wrong pin into the wrapper with no warning.
ADR-0015 makes model IDs opaque passthrough values with no vendor allowlist, so docket has no basis
for assuming the narrow class. A narrower class does not *reject* a provider-prefixed ID — which
would at least be honest — it truncates it to a prefix that still looks well-formed downstream.

The defect is pre-existing; the function carries the comment `field_of() — UNCHANGED (kept verbatim
from the prior version)`, and the class is identical on `origin/main`.

Change 0168 hit the same class in the new shipped-defaults reader `scripts/lib/harness-defaults.sh`
and fixed it there, which is what surfaced this sibling. That change is now **merged** (`origin/main`
at `9d41fa6b`), so the twin is readable on `main` and the two readers demonstrably disagree: the
sidecar accepts a provider-prefixed ID, user config still truncates it. This is the worse half —
user config is exactly where a provider-prefixed ID gets typed by hand.

A third reader shares the class: `scripts/runner-dispatch.sh:75` extracts `runners.<RUNNER>.*`
values across all three user layers and exports them as `DOCKET_RUNNER_CFG_*`. Those values are
free-form and more likely to be paths or URLs than model IDs.

## What ships

Two readers fixed, with **deliberately different value classes and different failure postures**,
because they parse different YAML shapes on different paths.

### Reader 1 — `sync-agents.sh` `field_of()` (flow-map form)

The line being parsed is an inline map: `docket-implement-next: {model: x, effort: y}`.

**Value class** widens from `[A-Za-z0-9._-]+` to `[^,}[:space:]]+` — "everything up to the flow-map
delimiters, not a character allowlist" — matching `hd_field` in `scripts/lib/harness-defaults.sh`
exactly. Provider-prefixed IDs (`anthropic/claude-opus-5`, `openrouter:vendor/model`) round-trip
whole.

**A raw tier plus a validator.** Widening alone still truncates anything the class cannot express —
a quoted scalar, an embedded space — so the fix follows the house pattern: a `_raw` companion that
returns what a YAML parser would see (`[^,}]*`, trailing whitespace trimmed), and a validation leg
that fails when the two disagree.

The naming follows the existing pair convention rather than inventing a fourth spelling:
`docket-frontmatter.sh` has `field` / `field_raw` (ADR-0058), `harness-defaults.sh` has `hd_field` /
`hd_field_raw`, so `sync-agents.sh` gets **`field_of` / `field_of_raw`**. The split here is
reader-capability, not quote-style — same as the sidecar's.

**Failure posture: loud, and before anything is written.** Collect every offender across all layers
and agents, report them all on stderr, exit non-zero **before any wrapper file is generated**.
Partial generation carrying a known-bad pin is precisely the harm this change exists to prevent.

**The diagnostic must distinguish unconsumable from missing.** `hd_validate`'s bare-scalar leg
carries the reasoning: without it, a clip that lands empty makes the error blame *absence* for what
is really a *quoting* problem. `sync-agents.sh` has the same trap. Mirror the message shape:

```
sync-agents: <harness>/<agent> '<key>' value '<raw>' is not a bare scalar — the reader consumes
only '<consumed>'; write model/effort values unquoted and space-free
```

### Reader 2 — `scripts/runner-dispatch.sh:75` (block-mapping form)

The line being parsed is one key per line under `runners.<RUNNER>.`, line-anchored — not a flow map.
`[^,}[:space:]]+` is the wrong class here; it would happen to admit a slash-bearing path only by
luck.

**Value class:** rest of the line, trailing `#` comment stripped, surrounding whitespace trimmed.

**Posture stays tolerant** — an unparseable value still `continue`s rather than dying. This is
asymmetric with reader 1 **on purpose**: `sync-agents.sh` fails at generation time, where a human is
reading output and a wrong pin persists in a file; `runner-dispatch.sh` runs on a live dispatch path
mid-handoff to a child process, where dying converts a cosmetic config typo into a failed dispatch.
The loud-error half of 0168's pattern fits the first and fits the second badly.

The existing per-key precedence claim is **preserved unchanged**: the key is claimed for its layer
*before* its value is parsed, so a malformed high-precedence value still masks lower layers
(precedence is per-key, not per-value).

### Not touched

- Every **key-side** use of the narrow class — `sync-agents.sh` lines 314/316/329/339,
  `runner-dispatch.sh:69`. YAML keys are legitimately narrow.
- `runner-dispatch.sh:33`'s runner-name validation (`*[!A-Za-z0-9._-]*|*..*`) — a filename-safety
  guard, correctly narrow.
- `scripts/lib/harness-defaults.sh` — already correct as of 0168.
- Any vendor model allowlist or availability lookup (ADR-0015 forbids it).

## Rejected: factoring a shared extractor

Three readers now share the defective class, which is a standing argument for one shared function.
Rejected for this change:

- `hd_field` is keyed on `(file, harness, agent, field)` and performs its own line lookup; `field_of`
  receives a line it was handed. Same extraction, different signatures — unifying means refactoring
  one to fit the other.
- The three readers parse **three different YAML shapes** (flow map, flow map with lookup, block
  mapping), and this design deliberately gives two of them different value classes. The duplication
  is narrower than it looks.
- Change 0175 rewrites `field_of`'s internals for performance (see below). Refactoring a function
  that is about to be rewritten is work done twice.

The shared-extractor question rides as a follow-up, to be revisited **after** 0175 settles
`field_of`'s implementation.

## Coupling with change 0175 — read before building

0175 (`sync-agents-per-invocation-cost`) rewrites this exact function: `field_of` becomes
`[[ $line =~ <ERE> ]]` plus `BASH_REMATCH[1]`, replacing the `sed` + `head` fork pair, and its spec
carries the **current ERE forward verbatim**. Its test list includes "a `field_of` equivalence test
over the ERE's edge cases."

Build order is therefore **0173 first, then 0175** — 0175 inherits the widened class, and its
equivalence tests are written against the fixed baseline rather than pinning the buggy one. 0175's
`depends_on` records this.

Both `sed -nE` and bash `[[ =~ ]]` are POSIX ERE, so the widened class transfers to 0175's rewrite
without reformulation. `field_of_raw` is a second function 0175's memoization pass must carry
across; its ERE transfers on the same terms.

## Coverage

Value-level asserts throughout. The truncation is silent — a test that only checks "generation
succeeded" or "the wrapper exists" passes against the bug.

**`tests/test_sync_agents.sh`**

- A provider-prefixed model ID resolves **whole** into the generated wrapper's `model:` line, across
  each of the three user layers independently: `.docket.local.yml`, repo `.docket.yml`, global
  `config.yml`.
- The same, across the `agents.default` vs `agents.<harness>` merge — a harness-specific line and a
  default line, asserting the resolved value and that `RES_MODEL_FROM_HARNESS` is unaffected.
- Cases: slash (`anthropic/claude-opus-5`), colon (`openai:gpt-5.6-sol`), both
  (`openrouter:vendor/model`), and a plain unprefixed ID as a non-regression.
- Validator: a **quoted** value and a **space-bearing** value each fail generation with a non-zero
  exit, a message naming harness/agent/key/raw/consumed, and **no wrapper file written**.
- Validator: a genuinely **missing** value produces the missing-value diagnostic, not the
  unconsumable one — the distinction the message shape exists to preserve.

**`tests/test_runner_dispatch.sh`**

- A slash-bearing and a colon-bearing runner config value each arrive intact in the corresponding
  `DOCKET_RUNNER_CFG_*` export.
- A value with a trailing `# comment` exports the value without the comment, whitespace trimmed.
- A malformed value still skips without dying, and its key still masks the same key in a
  lower-precedence layer.

## Out of scope

- Any vendor model allowlist or availability lookup (ADR-0015).
- `scripts/lib/harness-defaults.sh` (fixed by 0168).
- The shared-extractor refactor (follow-up, post-0175).
- 0175's performance rewrite itself.
