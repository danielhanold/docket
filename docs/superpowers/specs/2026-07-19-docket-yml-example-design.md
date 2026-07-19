# `.docket.yml.example` — canonical comprehensive config reference (design)

Date: 2026-07-19
Change: 0101
Related: change 0081 (config.yml.example), ADR-0039 (mirror rule), ADR-0019 (coordination-key fence)

## Problem

docket's config documentation is spread across three drifting surfaces: this repo's own
`.docket.yml` (the most expansive, but incomplete — no `auto_groom`, `auto_capture`, or
`github_project` coverage), `config.yml.example` (the global-config starter from change 0081,
which duplicates the reclaim/auto_capture prose), and the convention/`scripts/docket-config.md`
contracts. No single file shows a user **every key, its default, its documentation, and where
it may be set**.

## Decision summary (brainstormed 2026-07-19)

1. **One canonical reference:** a new committed `/.docket.yml.example`, Helm-values style —
   every key **active at its shipped default**, full documentation per key, absorbing the best
   prose from the repo's `.docket.yml` and `config.yml.example`.
2. **`config.yml.example` is deleted** (option 3 from the brainstorm — merge, don't coexist).
3. **Per-key scope tags** resolve the global-vs-repo tension: each key carries a one-line
   annotation — `# scope: repo-only (coordination-fenced, ADR-0019)` vs
   `# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)`. The
   classification table in `scripts/docket-config.md` stays authoritative; the example mirrors it.
4. **Presence-sensitive keys are the tagged exception** to active-at-default: `agents:` and
   `agent_harnesses:` ship **commented** with a loud marker, because declaring either in a repo
   opts it into per-repo wrapper generation even at default values.
5. **New `auto` sentinel** instead of unset-as-default: `finalize.test_command: auto`
   (auto-detect the suite) and `github_project: auto` (auto-mint the Projects board on first
   sync). The resolver treats `auto` identically to an unset key — matching the
   `integration_branch: auto` precedent. No key in the example documents its default as "unset".
6. **`install.sh` scaffolds a minimal global config** (header + pointer to the example, zero
   active values) instead of copying `config.yml.example`, so later shipped-default changes are
   never pinned by a stale scaffolded copy.
7. **This repo's `.docket.yml` slims** to only the values it actually sets, one-line comments,
   and a header pointer to the example — dogfooding the copy-out workflow.
8. **The standing rule** is written into the example's header and enforced by tests: *every new
   config flag must land in `.docket.yml.example` — value + documentation — in the same PR.*

## The file

Location: repo root, `/.docket.yml.example`, committed on the default branch. It is pure
documentation — no docket tooling ever reads it (the resolver reads `.docket.yml` from
`origin/HEAD`); tests are what keep it honest.

### Header block

- What the file is: the all-comprehensive reference for every docket config key and its default.
- The four config layers and per-field precedence:
  repo-local `.docket.local.yml` > repo-committed `.docket.yml` > global
  `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` > built-in.
- The copy-out workflow: users copy the keys they want to change into the layer they want;
  an unedited full copy behaves identically to no file (all values are defaults) — except the
  commented presence-sensitive keys, which must be uncommented deliberately.
- Scope-tag legend (repo-only vs any-layer).
- The must-update rule (decision 8), stated imperatively for both humans and agents.

### Body

Every key the resolver knows, grouped as today (branches/dirs, finalize, learnings, reclaim,
board, terminal_publish, auto_groom, auto_capture, harnesses/agents, skills), each with:

- full documentation (change/ADR back-references preserved from the absorbed prose),
- its shipped default as the **active value**,
- its scope tag.

Repo-only (coordination-fenced) keys per ADR-0019: `metadata_branch`, `integration_branch`,
`changes_dir`, `adrs_dir`, `results_dir`, `github_project`, `terminal_publish`, and the
`github` token of `board_surfaces`. Everything else is any-layer.

Exceptions (loudly tagged, commented):

- `agents:` — no meaningful default (built-ins live in `agents/docket-*.md` frontmatter) AND
  presence opts a repo into per-repo generation. Keeps the nine-agent claude mirror block
  (values mirror wrapper frontmatter — the relocated ADR-0039 rule) plus the commented
  codex/cursor example blocks from `config.yml.example`.
- `agent_harnesses:` — value default is `[claude]`, but presence in a repo file opts into
  per-repo generation; ships commented with the marker.

Marker shape (exact wording at build time):
`# PRESENCE-SENSITIVE: uncommenting this key changes behavior even at these default values.`

## Resolver change — the `auto` sentinel

`scripts/docket-config.sh` accepts `auto` ≡ unset for:

- `finalize.test_command` → `FINALIZE_TEST_COMMAND=` (empty; finalize auto-detects), and
- `github_project` → treated as unminted (auto-create + write-back on first `github` sync).

Case: accept the literal lowercase `auto` only (consistent with `integration_branch`).
Consumers must not see the sentinel leak: the export surface stays byte-identical to the
unset case (that is what the fidelity test asserts). `scripts/docket-config.md` documents the
sentinel for both keys; `github-mirror.sh`'s contract is re-checked so the write-back path
overwrites a literal `auto` rather than treating it as a minted value.

## Consolidation edits

- **Delete `config.yml.example`.** Its global-specific prose (harness enablement flow, the
  "unedited = absent" guarantee, reclaim/auto_capture global-ability notes) moves into the
  example's per-key docs and scope tags.
- **`install.sh`:** the first-run scaffold writes a minimal
  `~/.config/docket/config.yml` — header comment naming `.docket.yml.example` as the
  reference + the layers/precedence one-liner, no active keys. Existing user files are left
  untouched (same as today). `scripts/ensure-global-config.sh`/`.md` updated to match.
- **This repo's `.docket.yml`:** slims to its actually-set values
  (`metadata_branch: docket`, `integration_branch: main`, the three dirs,
  `finalize.gate: local`, `board_surfaces: [inline]`, `terminal_publish: true`) with one-line
  comments + a header pointer to the example.
- **README:** setup step 2 ("Set up your global config") and the Configuration section
  retarget from `config.yml.example` to `.docket.yml.example`; the example becomes the
  documented reference for all keys.
- **ADR:** a new ADR supersedes ADR-0039 — the mirror rule survives relocated (the example's
  commented `agents.claude` block mirrors wrapper frontmatter, wrappers lead), joined by the
  new **example = defaults** invariant (the file's active values must equal the resolver's
  built-ins, test-enforced) and the must-update rule. Authored via `docket-adr` at build time.

## Tests (replace `tests/test_config_example.sh`)

New `tests/test_docket_yml_example.sh`:

1. **Fidelity (example = defaults):** fixture repo (tmpdir, per the existing test's sandbox
   pattern) with `.docket.yml.example` copied in as `.docket.yml` on the fixture's default
   branch; `docket-config.sh --export --format plain` output must be byte-identical to the
   same fixture with no config file. Proves every active value is the shipped default and
   that the `auto` sentinels resolve to the unset behavior.
2. **Completeness:** every config key the resolver knows appears in the example. Mechanically:
   the export-key → YAML-path mapping lives in the test; a new export key with no mapping
   entry fails the test, forcing the example (and the mapping) to be updated in the same PR —
   the enforcement half of the must-update rule. Presence-sensitive keys are matched in their
   commented form.
3. **Mirror equality (relocated ADR-0039 check):** the commented `agents.claude` block's nine
   model/effort values equal `agents/docket-*.md` wrapper frontmatter (comment-stripped parse,
   same field regex as `sync-agents.sh`).
4. **Presence-sensitive keys ship commented:** no active `agents:`/`agent_harnesses:` header
   line exists in the file.
5. **Resolver round-trips retained from the old test:** the commented cursor block, once
   uncommented + enabled, resolves through the real `sync-agents.sh` into a cursor wrapper.
6. **Scaffold shape:** `install.sh`'s scaffold path writes a global config with no active
   keys (guard against re-introducing value pinning).
7. **README wiring:** the README names `.docket.yml.example` in the setup/configuration
   sections.

## Out of scope

- Generating the example from the resolver (single-source codegen) — the test-enforced manual
  mirror is the accepted trade-off, same shape as ADR-0039.
- Any new config keys or behavior changes beyond the `auto` sentinel.
- Validating harness model IDs in the commented codex/cursor blocks (unchanged from 0081).
- The `scripts/docket-config.md` classification table remains the authoritative scope source;
  the example only mirrors it.

## Open questions

None — all brainstorm decisions above are settled with Daniel (2026-07-19).
