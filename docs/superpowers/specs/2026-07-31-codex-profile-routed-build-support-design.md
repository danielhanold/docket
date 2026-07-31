<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0169 — Codex support for profile-routed Docket builds](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0169-codex-profile-routed-build-support.md)**
<!-- docket:backlink:end -->

# Codex support for profile-routed Docket builds

Design for making Docket's complete agent set natively pinned under Codex, including the three
profile-routed build workers, without adding a Codex-specific controller path or weakening the
runner-provenance boundary established by ADR-0064.

## Context

Change 0167 replaced Superpowers SDD's repeated implementer/reviewer topology with Docket's own
`docket-build` controller. It routes each plan task to one of three named workers (`economy`,
`standard`, or `premium`), allows one bounded escalation, and leaves independent review to
`docket-implement-next` Step 6. ADR-0063 makes model and effort properties of those named agents,
resolved through the ordinary generated-agent layer.

Change 0168 then moved every shipped model/effort default into
`agents/harness-defaults.yml`. ADR-0064 requires that sidecar to be sparse by whole harness and
complete within every shipped harness: Claude and Cursor each cover all twelve wrapper sources;
Codex is known but deliberately absent until this change. A Codex-enabled repo therefore generates
twelve valid but unpinned TOML definitions today.

The Codex emitter, native agent-definition shape, committed `AGENTS.md` dispatch block, and
whole-run Claude-to-Codex runner already exist. The remaining work is to ship one complete Codex
default block, certify native profile dispatch, and update the guards and documentation that
correctly encode "Codex ships nothing" on the pre-0169 side of the transition.

The selected values were checked during grooming against Codex CLI 0.146.0's live
`codex debug models` catalog. That catalog reported all three model slugs and every selected effort
token. This is design-time evidence, not a runtime allowlist: the build-time reconcile and live
certification must re-probe the actual installed version.

## Goals

- Ship complete native Codex defaults for every generated Docket agent.
- Make all three build profiles dispatch under Codex with the human-selected model/effort pairs.
- Preserve the existing harness-neutral routing rubric, outcome protocol, escalation graph, TDD
  contract, and full-suite gate.
- Preserve field-level user overrides above the shipped sidecar.
- Keep shipped native defaults out of Claude-to-Codex runner flags; only user-origin values may
  configure the child process.
- Promote `.docket.example.yml`'s Codex block from unvalidated illustration to an exact mirror of
  the shipped sidecar.
- Certify the three named profile dispatches in a live Codex session.

## Non-goals

- Adding a Codex-specific branch to `docket-build` or changing the shared task-worker contract.
- Routing individual tasks through `codex exec`; runner delegation remains whole-run only.
- Changing Cursor or Claude mappings.
- Replacing Docket's whole-branch review skill.
- Adding a vendor model registry, runtime model allowlist, or automatic fallback for unavailable
  model IDs.
- Requiring live Codex observations of automatic classification or the single-escalation path.
  Those remain covered by harness-neutral tests and prior Claude evidence, with the waiver recorded
  explicitly in the results artifact.

## Shipped Codex defaults

Add a complete `codex:` block to `agents/harness-defaults.yml`:

| Agent | Model | Effort |
|---|---|---|
| `status` | `gpt-5.6-luna` | `xhigh` |
| `adr` | `gpt-5.6-terra` | `xhigh` |
| `brainstorm-consultant` | `gpt-5.6-sol` | `medium` |
| `auto-groom` | `gpt-5.6-sol` | `low` |
| `auto-groom-critic` | `gpt-5.6-sol` | `medium` |
| `implement-next` | `gpt-5.6-sol` | `medium` |
| `rebase-resolver` | `gpt-5.6-sol` | `high` |
| `integration-repair` | `gpt-5.6-sol` | `high` |
| `finalize-change` | `gpt-5.6-terra` | `high` |
| `build-economy` | `gpt-5.6-luna` | `xhigh` |
| `build-standard` | `gpt-5.6-terra` | `high` |
| `build-premium` | `gpt-5.6-sol` | `medium` |

The nine non-build values promote the existing `.docket.example.yml` illustrations unchanged. The
three build values are the settled design for this change.

The profile names describe the capability/cost role of each complete model/effort pair. Reasoning
tokens are model-specific settings, not a cross-model ordinal: `xhigh` on Luna is not treated as
stronger than `medium` on Sol. The selected ladder is therefore Luna/xhigh for positively
established low-risk work, Terra/high for ordinary work, and Sol/medium for named risk or the one
allowed escalation.

`HD_SHIPPED_HARNESSES` gains `codex`. The existing validator then requires Codex's key set to equal
the complete `agents/docket-*.md` source set in both directions. A future wrapper cannot land
pinned under Claude and Cursor but silently unpinned under Codex.

Every Codex entry supplies both fields. `runner:` remains forbidden in the sidecar, and model IDs
and effort tokens remain structurally validated opaque strings under ADR-0015.

## Native generation and dispatch

No new dispatch mechanism is introduced. The existing path is sufficient:

```text
agents/harness-defaults.yml
  -> existing field-level resolver
  -> existing Codex TOML emitter
  -> model + model_reasoning_effort
  -> Codex's native named-agent dispatch
```

`sync-agents.sh` already emits `.codex/agents/docket-*.toml`, translating the resolved `model` to
`model` and the resolved `effort` to `model_reasoning_effort`. It continues to omit only genuinely
unresolved fields (and the existing `inherit`/`auto` sentinels); the new sidecar means every
unoverridden built-in Codex definition now carries both fields.

`docket-build` continues to dispatch `docket-build-economy`, `docket-build-standard`, or
`docket-build-premium` by name, foreground and sequentially. Codex resolves the name to the
generated TOML definition at session start. The controller does not detect the hosting harness,
select model IDs, or invoke a shell adapter. Maintained controller prose that still calls the
workers "Claude profile agents" becomes harness-neutral, but its behavior is unchanged.

Because Codex registers definitions at process start, setup and certification instructions retain
the required restart after regenerating wrappers.

## Runner delegation and provenance

Native dispatch and Claude-parent runner delegation remain separate transports with one shared
agent/worker contract.

For a native Codex run, the controller selects a named agent and Codex reads that agent's generated
TOML defaults. For a Claude-parent delegated run, the generated Claude shim invokes
`runner-dispatch`, which starts one complete `codex exec` process. Once inside that process,
`docket-build` uses the same native named-agent path as any other Codex run.

The runner remains whole-run only. This change does not make Claude invoke `codex exec` once per
plan task and does not add a common adapter over external runner launch and native child dispatch.
Those operations answer different questions:

- runner selection chooses where the complete Docket run executes;
- build-profile selection chooses which native agent executes one plan task.

ADR-0064's provenance split remains unchanged and is tested rather than redesigned. Values that
the sidecar actually supplied configure native wrappers. Only a model or effort that won from a
user configuration layer may become a delegated runner `--model` or `--effort` flag. A
runner-only user override therefore lets the top-level Codex process select its own default; an
explicit user model/effort continues to pass through verbatim. The shipped native profile values
apply later when that Codex process dispatches its profile workers.

## Example and documentation surfaces

`.docket.example.yml`'s Codex block becomes an exact shipped-default mirror:

- remove its second comment layer and all "unvalidated"/"until change 0169" wording;
- replace the three illustrative build rows with the settled Luna/Terra/Sol pairs;
- retain the other nine rows unchanged;
- keep the entire `agents:` surface singly commented because the key is presence-sensitive;
- describe Claude, Cursor, and Codex uniformly as shipped complete harness blocks.

The mirror guard derives its harness population from `HD_SHIPPED_HARNESSES`, not a new literal
`claude cursor codex` list. Its slice/order checks remain marker- and boundary-conscious, and the
real resolver round-trip gains Codex TOML evidence so the example is proven to be executable YAML,
not only text matching the sidecar reader.

A whole-repository grep derives the maintained documentation update set. At minimum:

- `docs/codex/setup.md` stops describing built-in Codex definitions as unpinned and documents the
  shipped values plus user override behavior;
- the README, agent-layer reference, convention, controller skill, and generated Codex dispatch
  prose describe three shipped harnesses and harness-neutral profile workers;
- tests and comments that deliberately describe the pre-0169 absence are flipped or retired by
  the new sidecar state rather than left as contradictory history.

Point-in-time records retain their original wording: archived changes, Accepted ADR bodies, prior
specs, plans, and results are not rewritten. No new ADR is required; this change is the planned
Codex extension of ADR-0064 and preserves ADR-0036/0037/0038 and ADR-0063.

Consumer repos that opt into Codex may see `sync-agents.sh --check` flag their committed managed
`AGENTS.md` dispatch block as stale if its generated premise changes from unpinned to pinned. The
normal remedy remains re-running sync and committing the refreshed machine-neutral block.

## Failure behavior

The sidecar validator runs before any wrapper write. A missing Codex row, phantom agent, duplicate,
partial entry, forbidden field, or malformed scalar makes generation fail without leaving a
half-regenerated directory.

Docket does not silently replace a model whose slug is unavailable. At build-time reconcile and
again immediately before live certification, query the installed Codex catalog and record the
version. If any selected slug or effort is unavailable, stop and surface design drift for a human;
do not choose a substitute model inside implementation.

User configuration remains the operational escape hatch for account- or rollout-specific model
availability. A user may override either field independently, and the target harness owns the
diagnostic for an invalid opaque value.

## Verification

### Tier 1: hermetic and gating

Generator and contract tests prove:

1. `agents/harness-defaults.yml` validates with Codex listed as shipped.
2. Codex's sidecar key set and the source-wrapper set are equal in both directions.
3. Every generated Codex TOML definition carries the sidecar's exact `model` and
   `model_reasoning_effort` values.
4. The build workers resolve exactly to Luna/xhigh, Terra/high, and Sol/medium.
5. The two pre-0169 Codex TOML absence assertions become value assertions; they are not deleted,
   so their cross-harness-leak guard remains live.
6. Machine-local, committed, and global user values still override shipped Codex values
   field-by-field.
7. A foreign `agents.default` winner still produces the existing warning and the generated
   artifact actually contains the value named by that warning.
8. A shipped native default never becomes a Claude-to-Codex runner flag, while explicit user-layer
   runner values still pass through.
9. The singly commented Codex example mirrors every sidecar row and resolves through the real
   generator into Codex TOML.
10. Existing controller tests remain green for explicit profile overrides, automatic routing, and
    the exactly-once escalation graph.

Mutation evidence accompanies the guards. At minimum, remove one Codex entry, add one phantom
entry, change one example value, restore the second comment layer, defeat the provenance predicate,
and replace one generated-value assertion with absence; each mutation must redden the intended
guard. Run the whole repository suite at the build gate.

### Tier 2: live Codex certification

Use the real `docket-build` controller in a real Codex session after regeneration and restart. A
small fixture plan explicitly routes one safe task to each profile. Record in the results artifact:

- Codex version and the selected models/efforts as reported by the installed catalog;
- the generated TOML values for all twelve wrappers;
- the controller's routing line, observed named agent/model indicator, structured worker outcome,
  focused verification, and task commit for each dispatched profile;
- economy observed on `gpt-5.6-luna` / `xhigh`;
- standard observed on `gpt-5.6-terra` / `high`;
- premium observed on `gpt-5.6-sol` / `medium`.

The support claim is not certified until all three named dispatches are observed. Automatic
classification and `NEEDS_ESCALATION` are deliberately not repeated live: their harness-neutral
tests and prior Claude evidence are accepted by human decision, and the results artifact records
that waiver explicitly so it can be reopened if Codex behavior later diverges.
