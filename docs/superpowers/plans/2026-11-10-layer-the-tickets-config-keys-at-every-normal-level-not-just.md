<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0592 — Layer the tickets.* config keys at every normal level, not just the repo](../../changes/active/0592-layer-the-tickets-config-keys-at-every-normal-level-not-just.md)**
<!-- docket:backlink:end -->

# Layer the tickets.* config keys at every normal level — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, task by task, under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the eight repo-only ticket settings (`tickets.provider`, `tickets.site`, `tickets.project`, and the five `tickets.auto.*` switches) be set in the global config and `.docket.local.yml` with ordinary per-field precedence, and tell the person how to write an explicit empty string when a bare `key:` is rejected.

**Architecture:** One scope flip in the schema registry (`scopeRepoOnly` → `scopeAny`) does the resolution work; the shared-setting guard, `RepoOnlyPaths()`, and the visibility fold/split all key on that scope, so they follow automatically. A one-branch addition to the shared `stringLeaf` validator adds the `""` hint to the null rejection. The rest is tests and the two user-facing docs.

**Tech Stack:** Go (`internal/config`, `internal/reposetup`, `internal/app`, `internal/repoguard`), YAML (`go.yaml.in/yaml/v3`), Markdown docs, `go generate ./internal/assets/` for the embedded example twin.

**Spec:** `docs/superpowers/specs/2026-11-11-layer-the-tickets-config-keys-at-every-normal-level-not-just-design.md` (on the `docket` branch)

## Global Constraints

- Exactly eight rows change scope: `tickets.provider`, `tickets.site`, `tickets.project`, `tickets.auto.change_created`, `tickets.auto.spec_written`, `tickets.auto.pr_opened`, `tickets.auto.pr_merged`, `tickets.auto.results_completed`. The repository-identity keys (`integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`) and `github_project`, `terminal_publish` stay `scopeRepoOnly`.
- Cancelling a global value needs no new mechanism: write the default (`""` for a string, `false` for a switch) in `.docket.yml` or `.docket.local.yml`; in a private repository, in `.git/dckt/config.yml`.
- YAML null is NOT newly accepted as "unset". A bare `project:` stays rejected (`CodeInvalidType`); only the message gains the hint. The hint text is exactly: `write "" to clear the value`.
- The hint lives in the shared string validator (`stringLeaf`), so every string key that accepts `""` gets it — not only the tickets keys. A string key that rejects `""` (non-empty or relative-path) must NOT get the hint.
- No new blocking gate, no new config key, no new provider, no new place to read ticket settings from.
- docket's own `.docket.yml` must still carry no `tickets` block (`TestDocketYMLDeclaresNoTicketsKey`).
- Docs describe current behavior only; no change numbers or PR citations in `docs/reference/config-keys.md` or `.docket.example.yml`. Comments in maintained source anchor on symbol names, never line numbers.
- After editing `.docket.example.yml`, run `go generate ./internal/assets/` so the embedded twin and `manifest.json` match (`TestEmbeddedMatchesAuthored`).
- Build gate: `go run ./cmd/docket development test` (from `build.test_command`), whole suite.

## Trace confirmed at planning (spec "Trace to confirm at build")

1. Readers of `eff.Tickets.*` on HEAD `71ee76d2d`: `internal/app/config.go` (the `diagnostic.config` lines) and `internal/app/ticket_context.go` (`pin.Config.Effective.Tickets.Auto`, report only); skills read through `diagnostic.config`. None derives a record path, branch, or other cross-clone value. Task 1 Step 1 re-runs the grep; a new reader that needs cross-clone agreement is a stop-and-report.
2. Private repository: `loadPrivateSources` (`internal/config/fs.go`) reads the global layer, then `.git/dckt/config.yml` as `LayerRepository`; a `.docket.local.yml` beside it is a `*ConflictingLocalConfigError` (unchanged). After the flip, a global ticket value is honored there and the private config outranks it, so a `""`/`false` in `.git/dckt/config.yml` cancels it. Second consequence: `RepoOnlyPaths()` feeds `reposetup.FoldPrivateConfig` / `SplitPrivateConfig`, so a ticket leaf in `.docket.local.yml` is now carried into the private config when going private (no longer dropped) and routed back to `.docket.local.yml` when going shared. Both are pinned in Task 1.
3. `diagnostic.config` provenance comes from `layerLabel` (`internal/app/config.go`): `"<layer> <source>:<line>"`. Pinned in Task 1 for global, repository-local, and the explicit `""`/`false` cancel.

## Review Focus

1. A ticket key in the global config while `.docket.yml` declares a *different* ticket key — fields must layer per key, not per block (pinned: Task 1 `TestTicketsKeysAnyLayer` mixed-block case).
2. A global `tickets.auto.pr_opened: true` cancelled by `false` in `.docket.local.yml` — a bool `false` must still count as an explicit, winning declaration (pinned: Task 1 cancel cases, both layers).
3. A private repository with a global ticket value and a cancel in `.git/dckt/config.yml` (pinned: Task 1 `TestPrivateRepositoryTicketsLayerOverGlobal`).
4. Going private with a ticket leaf in `.docket.local.yml` — it must survive the fold, not be silently dropped (pinned: Task 1 `TestFoldPrivateConfigCarriesLocalTicketsKeys`).
5. A bare `finalize.test_command:` (null) — a non-tickets string key that accepts `""` gets the same hint, and a bare `changes_dir:` does not (pinned: Task 2 registry-derived test).

---

### Task 1: Move the eight ticket settings to any-layer scope

**Files:**
- Modify: `internal/config/schema.go` (the eight `tickets.*` rows in `buildRegistry`, and the comment above the ticket block)
- Modify: `internal/config/schema_test.go` (`TestRegistryRepoOnlySet`)
- Modify: `internal/config/setting_paths_test.go` (`TestRepoOnlyPaths`)
- Modify: `internal/config/resolve_test.go` (`guardCases`, `TestTicketsKeysRepoOnly`, `TestTicketsCompanionSkillAnyLayer`)
- Modify: `internal/config/private_source_test.go` (new test)
- Modify: `internal/reposetup/visibilityconfig_test.go` (new test)
- Modify: `internal/app/config_test.go` (new test)

**Interfaces:**
- Consumes: existing helpers `srcG`, `srcR`, `srcL`, `mustSnapshot`, `diagPathSet`, `effectiveLeaf`, `registry()`, `PreflightMutation`, `blockerPaths` (package `config` tests); `pinEnv`, `writeFile` (`fs_test.go`); `FoldPrivateConfig`, `SplitPrivateConfig`, `repoOnlyForTest`, `leafValues`, `mustResolve` (package `reposetup` tests); `DiagnosticConfig`, `mainCtx` (package `app` tests).
- Produces: `config.RepoOnlyPaths()` now returns exactly `[integration_branch changes_dir adrs_dir results_dir github_project terminal_publish]`.

- [ ] **Step 1: Re-run trace 1**

Run (from the feature worktree): `git grep -n "Tickets\." -- 'internal/**/*.go' ':!*_test.go'`
Expected: only `internal/app/config.go` (diagnostic lines) and `internal/app/ticket_context.go` (`Tickets.Auto`). Any other reader: stop and report whether it needs cross-clone agreement before continuing.

- [ ] **Step 2: Update the scope pins to the six-path set**

In `TestRegistryRepoOnlySet` (`schema_test.go`) and `TestRepoOnlyPaths` (`setting_paths_test.go`), the wanted repo-only list becomes, in registry order:
`"integration_branch", "changes_dir", "adrs_dir", "results_dir", "github_project", "terminal_publish"`.
In `TestRepoOnlyPaths`'s doc comment, keep "the leaves a machine layer may not declare and a visibility switch never carries out of the local file".

- [ ] **Step 3: Remove the ticket rows from `guardCases()`**

Delete the eight `tickets.*` entries from `guardCases()` in `resolve_test.go`. `TestEverySharedSettingGuardCoversTheRegistry` keeps that table in step with the registry in both directions, so the rows must go (their subject, the guard, is gone for these paths).

- [ ] **Step 4: Replace `TestTicketsKeysRepoOnly` and `TestTicketsCompanionSkillAnyLayer` with one registry-derived test**

Delete both tests (the companion-skill test's "mixed block" half asserts the sibling guard that no longer exists; its layering half is subsumed below). Add `TestTicketsKeysAnyLayer` in `resolve_test.go`:

- Population: every `registry()` row whose path has prefix `"tickets."`. Floor: `len == 9`, and the set must include `tickets.provider` and `tickets.auto.results_completed` (so a broken derivation reddens).
- A doc builder `ticketsDoc(path, literal string) string` that renders `tickets.X` as `"tickets:\n  X: <literal>\n"` and `tickets.auto.X` as `"tickets:\n  auto:\n    X: <literal>\n"`.
- For a `kindString` row use literals g=`g-val`, r=`r-val`, l=`l-val`, cancel=`""`; for a `kindBool` row use g=`true`, r=`false`, l=`true`, cancel=`false`. Per row, subtests (read the leaf with `effectiveLeaf`; assert value, `Provenance.Layer`, `Explicit == true`, and `!diagPathSet(snap, CodeSharedSettingIgnored)[path]`):

| case | sources (low→high) | want value | want layer |
|---|---|---|---|
| default | none | `""` / `false`, `Explicit == false` | `LayerBuiltIn` |
| global | `srcG(g)` | g | `LayerGlobal` |
| local | `srcL(l)` | l | `LayerRepositoryLocal` |
| repo over global | `srcG(g), srcR(r)` | r | `LayerRepository` |
| local over repo | `srcR(r), srcL(l)` | l | `LayerRepositoryLocal` |
| repo cancels global | `srcG(g), srcR(cancel)` | `""` / `false` | `LayerRepository` |
| local cancels global | `srcG(g), srcL(cancel)` | `""` / `false` | `LayerRepositoryLocal` |

  (For bools, "repo over global" and "repo cancels global" coincide; keep both rows, it is cheap.)
- Mixed block: `mustSnapshot(t, srcG("tickets:\n  provider: jira\n  project: CLOUD\n"), srcR("tickets:\n  project: OPS\n"))` → provider `jira` from `LayerGlobal`, project `OPS` from `LayerRepository`, no `CodeSharedSettingIgnored` diagnostic at all, and `PreflightMutation(snap).Allowed` (report `blockerPaths(snap)` on failure).

- [ ] **Step 5: Add the private-repository test**

`TestPrivateRepositoryTicketsLayerOverGlobal` in `private_source_test.go`: `tmp := pinEnv(t)`; repo `t.TempDir()` with `.git/dckt/config.yml` = `"visibility: private\ntickets:\n  project: \"\"\n  auto:\n    pr_opened: false\n"`; global file `filepath.Join(tmp, "global.yml")` = `"tickets:\n  provider: jira\n  site: tickets.example.com\n  project: CLOUD\n  auto:\n    pr_opened: true\n"`. Call `LoadFilesystemSources(FSOptions{RepoDir: repo, GlobalPath: <global>})` and `Resolve(got, mainCtx)`. Assert:
- two sources: `LayerGlobal`, then `LayerRepository` named `.git/dckt/config.yml`;
- `Tickets.Provider` = `jira` and `Tickets.Site` = `tickets.example.com`, both `LayerGlobal`;
- `Tickets.Project` = `""`, `Explicit`, `LayerRepository`; `Tickets.Auto.PROpened` = `false`, `Explicit`, `LayerRepository`;
- no `CodeSharedSettingIgnored` diagnostic.

(`.docket.local.yml` beside a private config is already refused by `TestLoadFilesystemSourcesPrivateRefusesBothLocalFiles`; do not duplicate it.)

- [ ] **Step 6: Add the visibility fold/split test**

`TestFoldPrivateConfigCarriesLocalTicketsKeys` in `internal/reposetup/visibilityconfig_test.go`: committed `"tickets:\n  provider: jira\n"`, local `"tickets:\n  project: OPS\n  auto:\n    pr_opened: true\n"`.
- `FoldPrivateConfig(committed, local, repoOnlyForTest())` → `dropped` empty; `leafValues(folded, "tickets.provider", "tickets.project", "tickets.auto.pr_opened")` = `{jira, OPS, true}`; `mustResolve(t, folded, nil)`.
- `SplitPrivateConfig(folded, local, repoOnlyForTest())` → committed has `tickets.provider: jira` and neither `tickets.project` nor `tickets.auto.pr_opened`; the local bytes carry `tickets.project: OPS` and `tickets.auto.pr_opened: true` (assert with `leafValues` on `local`); `mustResolve(t, committed, local)`.

- [ ] **Step 7: Add the `diagnostic.config` provenance test**

`TestConfigDiagnosticsTicketsMachineLayers` in `internal/app/config_test.go`, sources (low→high):
- `{LayerGlobal, "/home/u/.config/docket/config.yml", "tickets:\n  provider: jira\n  project: CLOUD\n  auto:\n    pr_opened: true\n"}`
- `{LayerRepository, ".docket.yml", "tickets:\n  project: \"\"\n"}`
- `{LayerRepositoryLocal, ".docket.local.yml", "tickets:\n  auto:\n    pr_opened: false\n  site: tickets.example.com\n"}`

`r := DiagnosticConfig(src, mainCtx(), false)`; `r.HumanText()` must contain:
- `tickets.provider = jira  [global /home/u/.config/docket/config.yml:2]`
- `tickets.project = (unset)  [repository .docket.yml:2]`
- `tickets.site = tickets.example.com  [repository-local .docket.local.yml:4]`
- `tickets.auto.pr_opened = false  [repository-local .docket.local.yml:3]`

and `r.Diagnostics` holds no `config.CodeSharedSettingIgnored`.

- [ ] **Step 8: Run the new and changed tests to verify they fail**

Run: `go test ./internal/config/ ./internal/reposetup/ ./internal/app/ -run 'TestRegistryRepoOnlySet|TestRepoOnlyPaths|TestEverySharedSettingGuard|TestTicketsKeysAnyLayer|TestPrivateRepositoryTicketsLayerOverGlobal|TestFoldPrivateConfigCarriesLocalTicketsKeys|TestConfigDiagnosticsTicketsMachineLayers'`
Expected: FAIL — the scope pins, `TestEverySharedSettingGuardCoversTheRegistry` ("registry row … is repo-only but no guard case covers it"), and the new layering tests (shared-setting-ignored warnings, built-in values).

- [ ] **Step 9: Flip the eight rows to `scopeAny` and rewrite the block comment**

In `buildRegistry`, the eight rows named in Global Constraints get `scope: scopeAny`; nothing else on those rows changes. Replace the comment above the ticket block with one that states: the ticket settings name which ticket system a repository links to, where new tickets go, the person's companion skill, and the per-event switches; they layer like any other key, because nothing derives a record path, branch, or other value every clone must agree on from them; a repository cancels a global value by declaring the default.

- [ ] **Step 10: Run the package tests to verify they pass**

Run: `go test ./internal/config/ ./internal/reposetup/ ./internal/app/`
Expected: PASS. Then mutation-check: set `tickets.site` back to `scopeRepoOnly`, re-run `go test ./internal/config/ -run 'TestTicketsKeysAnyLayer|TestRegistryRepoOnlySet'`, confirm FAIL naming `tickets.site`, and restore.

- [ ] **Step 11: Commit**

```bash
git add internal/config/schema.go internal/config/schema_test.go internal/config/setting_paths_test.go internal/config/resolve_test.go internal/config/private_source_test.go internal/reposetup/visibilityconfig_test.go internal/app/config_test.go
git commit -m "refactor(config): layer the ticket settings at every level"
```

### Task 2: Name the `""` fix when a string key that accepts empty is given a bare null

**Files:**
- Modify: `internal/config/schema.go` (`stringLeaf`)
- Test: `internal/config/schema_test.go` (new test), `internal/config/resolve_test.go` (next to `TestTicketsStringSingleLine`)

**Interfaces:**
- Consumes: `registry()`, `pathSpec.validate`, `pathSpec.kind`, `kindString`, `repoSource`, `decodeDoc`, `CodeInvalidType`.
- Produces: the null-on-accepts-empty-string diagnostic message `<source>: <path>: expects a string, got an empty value; write "" to clear the value` (code `CodeInvalidType`, unchanged severity).

- [ ] **Step 1: Write the failing tests**

`TestStringNullNamesEmptyStringFix` (`schema_test.go`), derived from the registry, not a hand list:
- For every `kindString` row, call `spec.validate(repoSource(""), spec.path, emptyStr)` where `emptyStr` is a `yaml.ScalarNode` with `Tag: "!!str"`, `Value: ""`. Rows that accept it form set A; rows that reject it form set B.
- Population floor: A contains `tickets.provider`, `tickets.project`, and `finalize.test_command`; B contains `changes_dir`.
- For every row in A, `spec.validate(src, path, nullNode)` (`yaml.ScalarNode`, `Tag: "!!null"`) returns exactly one diagnostic with `Code == CodeInvalidType` whose `Message` contains both `expects a string, got an empty value` and `write "" to clear the value`.
- For every row in B, the null diagnostic's `Message` does NOT contain `write ""`.

`TestTicketsBareKeyIsRejectedWithHint` (`resolve_test.go`): `decodeDoc(t, "tickets:\n  project:\n")` yields exactly one diagnostic, `CodeInvalidType`, path `tickets.project`, message containing `write "" to clear the value`; and `decodeDoc(t, "tickets:\n  project: \"\"\n")` yields no diagnostic (the fix the message names works).

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/config/ -run 'TestStringNullNamesEmptyStringFix|TestTicketsBareKeyIsRejectedWithHint'`
Expected: FAIL — messages lack `write "" to clear the value`.

- [ ] **Step 3: Add the null branch to `stringLeaf`**

In `stringLeaf(nonEmpty, spaceFree, relPath bool) leafValidator`, before the `scalarWithTag` call: when `!nonEmpty && !relPath` and `n` is a `yaml.ScalarNode` tagged `tagNull`, return one `leafDiag(src, path, CodeInvalidType, n, ...)` with the message in Interfaces. Every other input takes the existing path, so `singleLineStringLeaf` inherits the hint through its `base`.

- [ ] **Step 4: Run them to verify they pass**

Run: `go test ./internal/config/`
Expected: PASS. Mutation-check: delete the new branch, confirm both tests FAIL, restore.

- [ ] **Step 5: Commit**

```bash
git add internal/config/schema.go internal/config/schema_test.go internal/config/resolve_test.go
git commit -m "feat(config): say to write \"\" when a string setting is left bare"
```

### Task 3: Document the layering and the cancel

**Files:**
- Modify: `docs/reference/config-keys.md` (the eight `tickets.*` rows, the `tickets` paragraph under the table)
- Modify: `.docket.example.yml` (comment above the `tickets` block, its scope tag, the `companion_skill` comment)
- Regenerate: `internal/assets/embedded/tree/.docket.example.yml`, `internal/assets/embedded/manifest.json`
- Modify: `internal/repoguard/prose_contracts_test.go` (`alignmentContracts`)

**Interfaces:**
- Consumes: Task 1's behavior (any-layer scope) and Task 2's message text.
- Produces: nothing code-facing.

- [ ] **Step 1: Write the failing prose contracts**

In `alignmentContracts`:
- Row `change_0585_update_ticket_skill` for `docs/reference/config-keys.md`: change `repo-only` to `any layer` in its first `present` string (the rest of that row is unchanged).
- Add rows with sentinel `change_0592_tickets_any_layer`:
  - `docs/reference/config-keys.md` — present: `` | `tickets.site` | `""` (unset) | any layer | ``, `` | `tickets.project` | `""` (unset) | any layer | ``, and for each of the five switches `` | `tickets.auto.<event>` | `false` | any layer | ``; plus `` `project: ""` `` and `` `pr_opened: false` `` and `.git/dckt/config.yml`. Absent: `no docket command reads them yet`, `Linking a change to a ticket does not consult them`, `` | repo-only | the ticket system ``, `` | repo-only | a switch for ``.
  - `.docket.example.yml` — present: `#     project: ""` and `#       pr_opened: false`. Absent: `No docket command reads these settings yet`, `These settings belong to the repository`, `Unlike the other ticket settings`.

Run: `go test ./internal/repoguard/ -run TestAlignmentContracts`
Expected: FAIL on every new and changed string.

- [ ] **Step 2: Rewrite `docs/reference/config-keys.md`**

- The eight rows: scope column `repo-only` → `any layer`; descriptions unchanged.
- Replace the `**`tickets`**` paragraph under the table with one paragraph, written for a reader who has never seen this change, covering: `docket-link-ticket` reads `provider`, `site`, and `project` through `docket diagnostic config` when you link a change to a ticket or create one, and asks for one it needs when it is empty; the five `auto` switches are reported by the ticket context but nothing acts on them yet; each setting can come from any layer, so `provider` and `site` can be set once in the global config, or a switch tried on one machine in `.docket.local.yml`; to cancel a global value for one repository, write the default in `.docket.yml` (everyone) or `.docket.local.yml` (this clone) — `project: ""` for a setting, `pr_opened: false` for a switch; a bare `project:` with nothing after it is not empty and is rejected; a private repository reads the global config and `.git/dckt/config.yml`, so its cancel goes in `.git/dckt/config.yml`.

- [ ] **Step 3: Rewrite the `.docket.example.yml` tickets comment**

- Replace the "No docket command reads these settings yet …" paragraph with: linking a change to a ticket, and creating one from a change, read these settings and ask for one that is needed and empty.
- Replace the closing paragraph ("These settings belong to the repository, except `companion_skill` …") with: like most settings, these can be set in any layer — `provider` and `site` are usually the same for every repository, so the global config suits them — and a repository cancels a global value by writing the default here (or in `.docket.local.yml` for one clone), followed by this commented example, kept exactly:

```yaml
#   tickets:
#     project: ""          # cancels a global project for this repository
#     auto:
#       pr_opened: false   # cancels a global true
```

  and one line: a bare `project:` with nothing after it is not an empty value and is rejected.
- Change the block's scope tag line to `# scope: any layer`.
- In the `companion_skill` comment, delete the sentence beginning "Unlike the other ticket settings"; the rest stays.
- Keep the `auto` paragraph's "docket does not act on them yet" (still true).

- [ ] **Step 4: Regenerate the embedded twin**

Run: `go generate ./internal/assets/`
Expected: `internal/assets/embedded/tree/.docket.example.yml` and `internal/assets/embedded/manifest.json` change; nothing else.

- [ ] **Step 5: Confirm no ticket-skill prose needs an edit**

Run: `git grep -n -i -E "repo-only|only.*\.docket\.yml|repository config" -- skills/docket-link-ticket skills/docket-update-ticket`
Expected: no line saying ticket settings come only from the repository config (at planning there were none; the skills read `diagnostic.config`, which is layer-agnostic). If a line does say so, rewrite it to "from `diagnostic.config`", add it to the Step 1 contract, and re-run Step 4.

- [ ] **Step 6: Run the doc guards**

Run: `go test ./internal/repoguard/ ./internal/config/ ./internal/assets/ -run 'TestAlignmentContracts|TestExample|TestEmbeddedMatchesAuthored|TestDocketYMLDeclaresNoTicketsKey'`
Expected: PASS (`TestExample…` covers `example_correspondence_test.go`, including Direction D: a verbatim copy of the example still resolves with no warning). Mutation-check: re-insert "No docket command reads these settings yet" into `.docket.example.yml`, confirm `TestAlignmentContracts` FAILS, restore, and re-run Step 4 if the twin changed.

- [ ] **Step 7: Commit**

```bash
git add docs/reference/config-keys.md .docket.example.yml internal/assets/embedded/tree/.docket.example.yml internal/assets/embedded/manifest.json internal/repoguard/prose_contracts_test.go
git commit -m "docs(config): ticket settings layer like any key; how to cancel a global value"
```

### Task 4: Whole-suite gate

- [ ] **Step 1: Run the build gate**

Run: `go run ./cmd/docket development test` (the resolved `build.test_command`), from the feature worktree.
Expected: green. Read the budget report even on green and act on any breach line per `tests/README.md`.
