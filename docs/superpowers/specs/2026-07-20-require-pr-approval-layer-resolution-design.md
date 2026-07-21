# finalize.require_pr_approval — layer resolution + a documented-key drift guard

Design doc for change #0102. Date: 2026-07-20.

## Problem

`finalize.require_pr_approval` is documented as a config key — in the README, in the
`docket-finalize-change` skill body, and in `.docket.example.yml` (which change 0101 had to tag
with a bespoke five-line `scope:` note because neither standard tag was honest) — but it has **no
layer resolution anywhere**. Verified against the tree at 2026-07-20:

- `scripts/docket-config.sh` never reads it: no `lcl`/`yaml_get`/`gbl` chain (contrast the
  `finalize.gate` and `finalize.test_command` chains at `:193-194`), and no `emit` line.
- `scripts/docket-config.md`'s resolved-values table (`:96-113`) has no row for it, and it is
  absent from the export list (`:264-288`).
- It is not in the coordination-key fence loop (`scripts/docket-config.sh:169`).
- Its only consumer, `skills/docket-finalize-change/SKILL.md`, reads `.docket.yml` by eye —
  a model-read of the committed file.

**Consequence.** A user who sets `finalize.require_pr_approval: true` in `.docket.local.yml` or in
the global `~/.config/docket/config.yml` gets silence: the value is neither honored nor
warned-and-ignored. It is the one documented key whose advertised scope the implementation does not
deliver, and the failure mode is the worst shape available — a merge gate the user believes is armed
but is not, discovered only when docket merges an unapproved PR.

The class of bug is broader than the key: nothing in the repo connects "documented in
`.docket.example.yml`" to "resolved by `docket-config.sh`". That gap is what let this key ship
documented-but-unwired, and it will let the next one do the same.

## Decisions

Two questions were open on the stub; both are settled.

**Which end-state — resolver-wired (`any layer`) or fenced (`repo-only`)?** → **Resolver-wired,
global-able.** This delivers the scope the docs already promise, so the docs shrink rather than grow
a caveat.

There is a real tension worth recording, so a future reader does not re-litigate it: the key gates
an *irreversible shared write* (a merge onto the integration branch), which is the shape ADR-0019
normally fences to per-repo-only. The precedent that settles it is `finalize.gate` — already
global-able, gating the very same merge, differing only in validating correctness rather than human
sign-off. Splitting the two halves of one merge gate across opposite scope classes would be the
harder thing to explain. Per-machine divergence here means "machine A refuses to merge an unapproved
PR, machine B merges it" — a policy divergence the maintainer chose per machine, not a split backlog.

**Does the finalize skill read the export directly, or keep its own read with the resolver as the
fallback?** → **Sole channel: the export only.** The skill reads
`FINALIZE_REQUIRE_PR_APPROVAL` from the Step-0 export block, exactly as it already reads
`FINALIZE_GATE`, `LEARNINGS_ENABLED`, and `AUTO_CAPTURE`; the direct `.docket.yml` read is deleted.

A fallback was considered and rejected. It would read *only* `.docket.yml`, so a
`.docket.local.yml` value would be honored on the export path and silently ignored on the fallback
path — this exact bug, re-created as an intermittent one. Today's failure is at least consistent.
And there is no genuine "export unavailable" state to insure against: `docket.sh preflight` is this
skill's blocking Step 0 and exits non-zero if it cannot resolve, and `${DOCKET_SCRIPTS_DIR:?}` fails
loud on a broken install. A key missing from the block means a stale docket clone — a condition to
fix, not to paper over.

Per the `sole-channel` learning, the survivor must carry the property the fallback would have given
free: the guard in §5 is what proves the export exists, and the fail-closed validation in §1 is what
prevents a malformed value from degrading quietly.

**Non-boolean posture: abort.** `auto_capture`, `reclaim.auto`, `learnings.enabled`, and
`terminal_publish` all `die` on a value that is not `true`/`false`; only the oldest key
(`auto_groom`) silently defaults. Aborting is the house rule and is especially right here —
defaulting a typo to `false` disarms a gate the user believes is armed, which is the failure this
change exists to eliminate.

## 1. Resolver — `scripts/docket-config.sh`

Add the chain immediately after `FINALIZE_TEST_COMMAND` (`:194`), following the shape of its two
siblings:

```sh
FINALIZE_REQUIRE_PR_APPROVAL="$(lcl require_pr_approval)"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-$(yaml_get "$CFG" require_pr_approval)}"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-$(gbl require_pr_approval)}"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-false}"
case "$FINALIZE_REQUIRE_PR_APPROVAL" in
  true|false) ;;
  *) die "unparseable config: finalize.require_pr_approval must be 'true' or 'false', got '$FINALIZE_REQUIRE_PR_APPROVAL'" ;;
esac
```

Notes on the shape:

- **Bare leaf read, not `yaml_block_body`.** `require_pr_approval` is a distinctive enough word that
  a future top-level key cannot plausibly shadow it — the same judgment already applied to
  `gate` and `test_command`, and documented in the `yaml_get` comment at `:93`. The nested-block
  treatment is reserved for generic leaves like `enabled`, `cap`, and `auto`. The existing comment
  at `:93` gains `require_pr_approval` in its list of leaf-read finalize keys.
- **Not added to the fence loop** at `:169` — global-able is the whole point of the change.
- **No `auto` sentinel.** The key is a plain boolean with a meaningful `false` default;
  `auto` exists for `test_command`/`github_project` where the default is *unset*, which is not the
  case here.

Emit after `FINALIZE_TEST_COMMAND`:

```sh
emit FINALIZE_REQUIRE_PR_APPROVAL "$FINALIZE_REQUIRE_PR_APPROVAL"
```

The export block goes from 24 → 25 lines in `shell` format and 25 → 26 in `plain`.

## 2. Contract — `scripts/docket-config.md`

- **Table row**, inserted after the `test_command` row:

  | `require_pr_approval` (finalize) | `false` | yes | read from `finalize.require_pr_approval` leaf key; resolves repo-local > repo-committed > global; `true`/`false`, anything else aborts (change 0102) |

- **Export list** gains `FINALIZE_REQUIRE_PR_APPROVAL` after `FINALIZE_TEST_COMMAND`, and both
  line counts in the sentence below the list are corrected (24 → 25, 25 → 26).

## 3. Skill — `skills/docket-finalize-change/SKILL.md`

The `finalize:` yaml block in *The rebase-retest merge gate* section **stays** — it is user-facing
documentation of the key's meaning, and existing tests anchor on it. What changes is how the skill
*obtains* the value:

- The section's framing sentence ("Configured by `.docket.yml`:") is corrected — the gate is
  configured by the resolved config, read from the Step-0 export block, not by parsing a file.
- Every behavioral mention of `require_pr_approval` (Selection matrix, eligibility definition,
  disposition rules, the final report's skip reasons) reads the exported value. The prose keeps
  saying `require_pr_approval` where it is naming the *policy*; it names
  `FINALIZE_REQUIRE_PR_APPROVAL` where it is naming the *value the skill reads*.
- No `.docket.yml` parsing remains anywhere in the skill body for this key.

The behavior of the gate is unchanged: `true` still means the auto-detect path refuses to merge a PR
whose `reviewDecision != APPROVED`; an explicit id or an id allowlist still overrides it.

## 4. Surfacing — `.docket.example.yml` and README

`.docket.example.yml`'s bespoke annotation collapses to the standard tag used by its two siblings:

```yaml
  # scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
  require_pr_approval: false
```

The five lines explaining that the key is read by the skill body rather than the resolver, and that a
machine-scoped value is silently ignored, are deleted — they describe a state that no longer exists.
The key's *behavioral* comment (auto-detect path only; explicit id overrides) is unchanged.

README's `require_pr_approval` mentions (`:180`, `:662`) are checked for any claim that the key is
repo-only or skill-read, and corrected if present.

## 5. The drift guard — `tests/test_docket_example_yml.sh`

This bug existed because nothing connected "documented key" to "resolved key". The guard closes that
gap and fits ADR-0048's existing charter for this file (the example config as a tested canonical
reference).

**Mechanism — an explicit manifest.** The test carries a classification for every key that appears
in `.docket.example.yml`, in one of two forms:

- `resolved:<EXPORT_NAME>` — the key is resolved by `docket-config.sh`. The test asserts that
  export name is actually emitted by the script, so a manifest entry cannot claim an export that
  does not exist.
- `elsewhere:<consumer>` — the key is deliberately not resolver-read, with its real consumer named.
  The population at the time of writing: `github_project` (fenced only; documentation-only today —
  `github-mirror.sh` takes its board from `--project`/`--auto-create-project`), `runners.*` (read by
  `sync-agents.sh`), and `agents`/`agent_harnesses` (same) should they appear as active keys.

A key present in the example with **no** manifest entry fails the test with a message naming it as
documented-but-unclassified. This catches both directions of drift: a new key documented without
resolution (this bug), and a manifest that has gone stale against the script.

An explicit manifest is used rather than deriving the mapping mechanically because key → export name
is not 1:1 (`gate` → `FINALIZE_GATE`, `enabled` → `LEARNINGS_ENABLED`, `auto` → `RECLAIM_AUTO`,
`brainstorm` → `SKILL_BRAINSTORM`); any derivation would need the same table, hidden inside a
transform instead of stated plainly.

Note the existing asserts at `tests/test_docket_example_yml.sh:126`, `:171`, and `:175-178`, which
document `require_pr_approval` as model-read and couple the example to the skill body. They describe
the pre-change world and are updated, not left to rot alongside a contradicting manifest entry.

## 6. Tests

- **Resolution.** `finalize.require_pr_approval` resolves through all four rungs: repo-local >
  repo-committed > global > built-in `false`. Mirror the existing `finalize.gate` fixtures in
  `tests/test_finalize_gate.sh` / the config test rather than inventing a new fixture shape.
- **Fail-closed.** A non-boolean value aborts with a diagnostic naming the key.
- **Not fenced.** A value in `.docket.local.yml` or the global config is *honored*, and emits no
  per-repo-only warning — the direct inverse of the fenced-key assertions.
- **Export presence.** `FINALIZE_REQUIRE_PR_APPROVAL` appears in both export formats, in position,
  with the corrected line counts.
- **Guard.** The manifest test passes on the current example file, and fails when a key is added to
  the example without a manifest entry.
- **Skill.** The finalize skill body no longer reads `.docket.yml` for this key; the existing
  `require_pr_approval` documentation asserts still pass.

## 7. ADR

One ADR: **a documented config key resolves through `docket-config.sh`; a model-read of
`.docket.yml` is not a supported shape.** The `elsewhere:` allowlist is the named exception — a key
may be read by another script, but the classification must be explicit and the consumer named — and
the §5 manifest guard is the enforcement.

Relates to ADR-0048 (`.docket.yml.example` as a tested canonical reference), ADR-0019 (the
coordination-key fence, which this key is deliberately outside of), and ADR-0012 (the
script-vs-model boundary — this is that boundary applied to config reads).

## Out of scope

- **What `require_pr_approval` does** at the merge gate. This change is resolution wiring only; the
  policy, the explicit-id override, and the disposition rules are untouched.
- **Converting the other `elsewhere:` keys.** `agents:`, `agent_harnesses`, `github_project`, and
  `runners.*` have working consumers. This change classifies them; it does not move them.
- **Reworking `yaml_get`** or the flat-scalar reader. The bare-leaf read is sufficient here for the
  same reason it is sufficient for `gate`.

## Risks

- **The manifest is itself a thing that can go stale.** Mitigated by making staleness *loud*: an
  unclassified key fails, and a `resolved:` entry naming a nonexistent export fails. The failure
  mode is a red test, not silence — which is the whole point.
- **Line-count asserts in the contract doc.** The export list carries literal counts (24/25), so
  adding an export touches them. Already true of every prior export addition; called out here only
  so the implementation does not miss them.
