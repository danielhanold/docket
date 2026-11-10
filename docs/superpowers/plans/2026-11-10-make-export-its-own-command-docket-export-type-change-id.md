<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0589 — Make export its own command: docket export <type> <change_id>](../../changes/active/0589-make-export-its-own-command-docket-export-type-change-id.md)**
<!-- docket:backlink:end -->

# `docket export` as its own command — Implementation Plan

> **For agentic workers:** executed by `docket-build`, task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the clean-copy export from `docket open <type> [<id>] --export` to a dedicated `docket export <type> [<id>]` command with its own catalog entry (`export`) and result type, and strip `--export`, `--output-path`, and `--warnings` from `docket open`.

**Architecture:** A new app-layer operation `Export` (in `internal/app/export.go`, which absorbs today's `open_export.go`) shares `Open`'s resolution helpers (a factored prelude, `resolveOpenChange`, `changeArtifactPath`, `localArtifact`, and a factored opener call) and writes the copy through one shared function. A new thin Cobra adapter `internal/cli/export.go` reuses `openDepsFor`. Task 1 adds the new command alongside today's `open --export` (with a transitional parity test proving byte-identical output); Task 2 removes the export from `open`; Task 3 moves the skill, docs, and sentinel to the new command.

**Tech Stack:** Go, Cobra, the repo's suite runner (`go run ./cmd/docket development test`).

**Spec:** `docs/superpowers/specs/2026-11-10-make-export-its-own-command-docket-export-type-change-id-design.md` (metadata branch).

## Global Constraints

- Command: `docket export <type> [<id>]`, `<type>` one of `change`, `spec`, `plan`, `results`; without an id it resolves the change whose recorded branch is checked out, exactly as `open` does.
- Catalog id `export`; effects `read`, `local-write`, `process-control`. `open` keeps the same three.
- Flags of `export`: `--output-path`, `--warnings all|leftovers|mentions` (default `all`), `--print`, `--repo-dir`, and the global `--json`.
- Opening: no `--output-path` and no `--print` → write to the default destination and open the copy (`launched: true`). `--output-path` → write there, print the path, open nothing. `--print` → write, print, open nothing whatever the destination. `launched` is `true` only when the copy was handed to the opener.
- Usage mistakes are CLI usage errors (`invalid-arguments`, exit 2, no operation document), caught in the command's `Args` hook before the operation runs: a type outside the four (`board`, `pr`, anything else), a bad `--warnings` value, an empty `--output-path`.
- If `app.Export` is called directly with a type outside the four, it refuses as `invalid-target` (`ResultInvalidInput`). The `invalid-option` reason and `ValidateOpenExport` go away.
- Result document: envelope operation `export`; fields `what`, `change_id`, `exported_path`, `replaced`, `launched`, `notes` (always a list), `leftovers`, `mentions` (both following `--warnings`), `reason` / `message`. No `target`, no `target_kind`. Human text: the exported path alone on success; notes then warnings on stderr; the message on a refusal.
- Everything the export writes is unchanged byte-for-byte: file name `<date>-<kind>-<name>.md` (kinds `ticket`, `spec`, `plan`, `results`), transform, self-check, atomic write, `replaced` note, the destination rules, every refusal reason and result class listed in the spec.
- `docket open` rejects `--export`, `--output-path`, `--warnings` with Cobra's ordinary unknown-flag error. No alias, no message naming the new command.
- Point-in-time records (release notes, archived changes, results, specs, plans, Accepted ADRs) keep `open --export` as written. Only maintained source changes.
- Suite gate: `go run ./cmd/docket development test` (from `build.test_command`). Use `go test -count=1` for focused runs and every mutation probe.
- Comments anchor on symbol names, never line numbers.

## Review Focus

1. **`--output-path` given alongside `--print`, and `--output-path` alone.** Both must leave `launched: false` and the opener untouched; only the no-flag default launches. Pinned by `TestExportLaunchesOnlyAtTheDefaultDestination` (Task 1).
2. **An opener failure after the copy was written.** The copy stays on disk, the result is `external-failed` with `no-opener`/`opener-failed`, `exported_path` stays set, and `HumanText` prints the path then the message. Pinned by `TestExportOpenerFailureKeepsThePath` (Task 1).
3. **`docket export board` / `docket export pr 3` in `--json` mode.** Must be the CLI's `invalid-arguments` document with exit 2, never an `export` operation document. Pinned by `TestExportCLIUsageErrors` (Task 1).
4. **Bytes drift between the old and new entry points.** The new command must write exactly what `open --export` wrote. Pinned by the transitional `TestExportMatchesOpenExport` (Task 1) and permanently by the moved byte-literal test `TestExportWritesCleanCopy`.
5. **A leftover `--export` spelling in the shipped skill or its embedded copy.** Pinned by the sentinel's new `absent` clause plus the asset-tree drift test (Task 3).

---

### Task 1: Add the `export` operation and command beside `open --export`

**Files:**
- Rename: `internal/app/open_export.go` → `internal/app/export.go` (`git mv`), then extend it.
- Modify: `internal/app/open.go` (extract shared helpers only; `open --export` keeps working in this task).
- Modify: `internal/app/schema_registry.go` (add the `export` binding).
- Rename: `internal/app/open_export_test.go` → `internal/app/export_test.go` (`git mv`), then retarget.
- Create: `internal/cli/export.go`, `internal/cli/export_test.go`.
- Modify: `internal/cli/root.go` (register the command), `internal/cli/install.go` (`assetIndependent`), `internal/cli/capability_production_test.go`, `internal/cli/presenter_test.go`.

**Interfaces:**
- Produces (package `app`):
  - `const OperationExport = "export"`
  - `var ExportTargets = []string{"change", "spec", "plan", "results"}`
  - `type ExportOptions struct { RepoDir string; What string; ID int; Print bool; OutputPath string; Warnings string }` — `ID` 0 infers from the branch; `OutputPath` "" is the default destination; `Warnings` "" means `WarningsAll`.
  - `type ExportResult struct` — `Envelope` embedded, then `What string json:"what"`, `ChangeID int json:"change_id,omitempty"`, `ExportedPath string json:"exported_path,omitempty"`, `Replaced bool json:"replaced,omitempty"`, `Launched bool json:"launched"`, `Notes []string json:"notes"`, `Leftovers []ExportLeftover json:"leftovers,omitempty"`, `Mentions *ExportMentions json:"mentions,omitempty"`, `Reason string json:"reason,omitempty"`, `Message string json:"message,omitempty"`. (The `exported_path`, `replaced`, `leftovers`, `mentions` tags are byte-identical to today's `OpenResult` tags.)
  - `func (r ExportResult) HumanText() string` — `ExportedPath` when `Message` is empty; `ExportedPath + "\n" + Message` when both are set; `Message` alone otherwise.
  - `func (r ExportResult) HumanNotes() []string` and `func (r ExportResult) HumanWarnings() []string`.
  - `func ParseExportArgs(args []string) (string, int, error)` — one-line usage errors.
  - `func ValidateExportWarnings(w string) error` — nil for `""`, `all`, `leftovers`, `mentions`; else `invalid --warnings value %q — choose all, leftovers, or mentions` (today's message verbatim).
  - `func Export(ctx context.Context, deps OpenDeps, o ExportOptions) ExportResult` — reuses `OpenDeps` unchanged.
  - Unexported shared helpers, used by both `Open` and `Export`: `openPrelude(ctx context.Context, deps OpenDeps, repoDir, what string, id int) (dir string, branch string, pin StatusPin, f *openFailure)`; `checkArtifactPath(what, rel string) *openFailure`; `runOpener(ctx context.Context, opener Opener, target string) *openFailure`; `parseChangeIDArg(raw string) (int, error)`; `exportWarningLines(leftovers []ExportLeftover, mentions *ExportMentions) []string`; `writeCleanCopy(ctx context.Context, deps OpenDeps, what, outputPath, warnings, dir string, pin StatusPin, c domain.Change, snap domain.Snapshot, rel string) (exportCopy, *openFailure)` with `type exportCopy struct { Path string; Replaced bool; Notes []string; Leftovers []ExportLeftover; Mentions *ExportMentions }`.
- Produces (package `cli`): `var exportRunner func(ctx context.Context, d app.OpenDeps, o app.ExportOptions) app.OperationResult`; `func newExportCommand(setResult func(app.OperationResult)) *cobra.Command`; `func exportOptionsFromFlags(c *cobra.Command, what string, id int) (app.ExportOptions, error)`.

- [ ] **Step 1: Move the files.** `git mv internal/app/open_export.go internal/app/export.go` and `git mv internal/app/open_export_test.go internal/app/export_test.go`. Update `export.go`'s header comment to say it holds `docket export` (its argument grammar, its result, the destination rule, the atomic write, and the warnings) — and, until Task 2, the `--export` branch of `docket open`. Run `go build ./... && go test -count=1 ./internal/app/ -run 'Export|Open'`; expect PASS (pure move).

- [ ] **Step 2: Retarget the moved app tests to `Export` (they fail to compile).** In `internal/app/export_test.go`:
  - Change the fixture helper to `func (f *openFixture) export(t *testing.T, deps OpenDeps, what string, id int, mut func(*ExportOptions)) ExportResult`, building `ExportOptions{RepoDir: f.repoDir, What: what, ID: id}` and calling `Export(context.Background(), deps, o)`. Every `mut` closure takes `*ExportOptions`.
  - Rename each `TestOpenExport*` to `TestExport*` and `TestOpenResultHumanWarningsWording` to `TestExportResultHumanWarningsWording` (use `ExportResult{...}`). Keep every expected value (in particular `exportSpecClean`, `"2026-01-02-spec-widget.md"`, `"2026-01-02-ticket-widget.md"`, and the warning strings) byte-for-byte.
  - In `TestExportWritesCleanCopy` replace the `res.Target`/`res.TargetKind` checks with `res.HumanText() == want` and `res.Operation == OperationExport`.
  - In `TestExportIgnoresGitHubRouting` replace the `TargetKind`/`Target` checks with `strings.HasPrefix(res.ExportedPath, out)`.
  - `TestExportRefusesProtectedDestination` sets only `OutputPath`; it still asserts nothing opened.
  - Replace `TestOpenExportOpensTheCopy` with `TestExportLaunchesOnlyAtTheDefaultDestination`, table-driven over (OutputPath set?, Print?) on linux with a temp home: `{false,false}` → `ResultApplied`, `Launched == true`, `f.opener.targets == []string{filepath.Join(home, "2026-01-02-spec-widget.md")}`; `{true,false}`, `{true,true}`, `{false,true}` → `ResultApplied`, `Launched == false`, `len(f.opener.targets) == 0`, `HumanText() == ExportedPath`.
  - Add `TestExportOpenerFailureKeepsThePath`: default destination on linux, table over `f.opener.err = &NoOpenerError{GOOS: "plan9"}` → `ReasonOpenNoOpener` and `errors.New("exit status 4")` → `ReasonOpenOpenerFailed` (the pairs `open_test.go` already uses) → `Result == ResultExternalFailed`, `Reason` as paired, `ExportedPath` set and the file exists, `Launched == false`, `HumanText()` starts with `ExportedPath + "\n"`.
  - Add `TestExportRefusesInvalidRequest`: `Export` with `What` `board`, `pr` (ID 7), `bogus`, and `spec` with `Warnings: "bogus"` → `ResultInvalidInput`, `Reason == ReasonOpenInvalidTarget`, `Operation == OperationExport`, one-line `Message`, `f.reader.pinCount == 0`, opener untouched.
  - Add `TestExportResultJSON`: marshal an `ExportResult` (as `TestOpenResultExportJSON` does); assert the same `exported_path`, `replaced`, `leftovers`, `mentions` values, `"launched": false`, `"notes": []`, and that the keys `target` and `target_kind` are absent.
  - Add `TestParseExportArgs`: ok — `{"spec"}` → (`spec`, 0), `{"change","541"}`, `{"plan","0541"}` → 541, `{"results","000012"}` → 12; bad (one-line error) — `nil`, `{"board"}`, `{"pr","7"}`, `{"bogus"}`, `{"spec","1","2"}`, `{"spec","#541"}` (exactly `invalid change id #541`), `{"spec","0"}`, `{"spec","-5"}`; in every "unknown type"/"missing"/"too many" error, the text after `one of ` is exactly `change, spec, plan, results` (so `board` and `pr` are never offered).
  - Add the transitional `TestExportMatchesOpenExport`: one fixture, `Open(... OpenOptions{What: "spec", ID: 7, Export: true, Print: true, OutputPath: dirA})` and `Export(... ExportOptions{What: "spec", ID: 7, Print: true, OutputPath: dirB})`; assert equal file bytes, equal `filepath.Base` of the exported paths, equal `Replaced`, `Leftovers`, `Mentions`, `Notes`. Run it with a body that produces leftovers and mentions (the `TestExportWarningsFilter` body). Task 2 deletes it.
  - Leave `TestValidateOpenExport` and `TestOpenResultExportJSON` in the file unchanged (they still test `open`; Task 2 deletes them).

- [ ] **Step 3: Run the app tests to see them fail.** `go test -count=1 ./internal/app/ -run 'Export|ParseExportArgs'` → FAIL to compile (`Export`, `ExportOptions`, `ExportResult` undefined).

- [ ] **Step 4: Factor the shared helpers out of `Open` (behavior-neutral).**
  - `parseChangeIDArg(raw)`: the `openIDPattern` + `strconv.Atoi` + `> 0` check from `ParseOpenArgs`, error `invalid change id <raw>`. `ParseOpenArgs` calls it.
  - `openPrelude`: `Open`'s steps from the empty-`RepoDir` check through `PinContext` (absolute path, branch inference only when `what != "board" && id == 0`, before any fetch; `classifyStatusError` on a pin error). `Open` calls it after its target and option checks.
  - `checkArtifactPath(what, rel)`: the `filepath.IsLocal` refusal (`ReasonOpenPathInvalid`, same message).
  - `runOpener(ctx, opener, target)`: `finishOpen`'s nil-opener / `NoOpenerError` / generic-error classification, returning `*openFailure{ResultExternalFailed, ReasonOpenNoOpener|ReasonOpenOpenerFailed, msg}` or nil. `finishOpen` keeps its `Print` short-circuit and envelope handling and calls it.
  - `exportWarningLines`: the body of today's `OpenResult.HumanWarnings`; that method now returns `exportWarningLines(r.Leftovers, r.Mentions)`.
  - `writeCleanCopy`: today's `openExport` from the wiring check through the warnings filter (wiring check, `localArtifact`, read, kind, file name, `exportDestination`, `protectedDestination`, transform, check, `writeExport`, `replaced` note, `--warnings` filter). `Notes` carries the prepare notes even when it returns a failure. `openExport` becomes: call `writeCleanCopy`, copy its fields into the `OpenResult`, set `Target`/`TargetKind`, `finishOpen`.
  Run `go test -count=1 ./internal/app/ -run 'Open'` → PASS (no behavior change).

- [ ] **Step 5: Implement the export operation in `internal/app/export.go`.** Add the Interfaces' `OperationExport`, `ExportTargets`, `ExportOptions`, `ExportResult` and its three methods, `ParseExportArgs` (messages: `missing type — choose one of change, spec, plan, results`; `too many arguments — usage: export <type> [id], where <type> is one of change, spec, plan, results`; `unknown type %q — choose one of change, spec, plan, results`; id via `parseChangeIDArg`), `ValidateExportWarnings`, and `Export`:
  1. `What` not in `ExportTargets` → `invalid-target` with `ParseExportArgs`'s message; `ValidateExportWarnings` error → `invalid-target` with its message; both `ResultInvalidInput`, before any read.
  2. `openPrelude`, then `resolveOpenChange`, set `ChangeID`, `changeArtifactPath`, `checkArtifactPath`.
  3. `writeCleanCopy` with `o.OutputPath`, `o.Warnings`; append its `Notes`; on failure return it.
  4. Set `ExportedPath`, `Replaced`, `Leftovers`, `Mentions`.
  5. When `!o.Print && o.OutputPath == ""`: `runOpener(ctx, deps.Opener, path)`; failure → its class/reason/message, else `Launched = true`. Envelope `NewEnvelope(OperationExport, …)` on every outcome.
  Add `{ID: "export", Request: nil, Result: ExportResult{}}, // Export` to `operationBindings` in sorted position (after `evidence.verify`, before `finalize.block`).

- [ ] **Step 6: Run the app tests.** `go test -count=1 ./internal/app/` → PASS (including `TestEveryResultStructIsBound` and `TestOperationBindingsSortedUniqueAndDescribable`). The schema/catalog correspondence in `internal/cli` is red until Step 9 — expected.

- [ ] **Step 7: Write the failing CLI tests in `internal/cli/export_test.go`.**
  - `stubExport(t, res app.ExportResult) *[]app.ExportOptions` mirrors `stubOpen` over `exportRunner`/`openDepsFor`.
  - `TestExportCLIPassesParsedOptions`: `export spec 541 --output-path /tmp/x --warnings leftovers --print --repo-dir /tmp/repo` → `{RepoDir: "/tmp/repo", What: "spec", ID: 541, Print: true, OutputPath: "/tmp/x", Warnings: "leftovers"}`; `export spec 0541 --repo-dir /tmp/repo` → `{RepoDir: "/tmp/repo", What: "spec", ID: 541}` (`Warnings` stays `""` when the flag is not given); `export plan --repo-dir /tmp/repo` → `{RepoDir: "/tmp/repo", What: "plan"}`.
  - `TestExportCLIUsageErrors`: each of `{"export"}`, `{"export","board"}`, `{"export","pr","5"}`, `{"export","bogus"}`, `{"export","spec","#541"}`, `{"export","spec","0"}`, `{"export","spec","1","2"}`, `{"export","spec","5","--warnings","bogus"}`, `{"export","spec","5","--output-path",""}` → exit 2, non-empty one-line stderr; the runner never called. Then `runCLI(t, "--json", "export", "board")` → exit 2, stdout decodes with `reason == app.ReasonInvalidArguments` and `operation` not `export`.
  - `TestExportCLIPrintsThePath`: stub an applied `ExportResult{ExportedPath: "/d/2026-01-02-spec-widget.md", Notes: []string{"replaced /d/2026-01-02-spec-widget.md"}}`; `export spec 7 --print --repo-dir /tmp/repo` → stdout exactly `/d/2026-01-02-spec-widget.md\n`, stderr `note: replaced /d/2026-01-02-spec-widget.md\n`.
  - `TestExportCLIDepsWiring`: copy of `TestOpenCLIExportDepsWiring` (the command shares `openDepsFor`).
  - `TestExportCLIHelpNamesTicketKind`: `export --help` exit 0, contains `kind one of ticket, spec, plan, or`, not `one of issue`.
  Update `internal/cli/capability_production_test.go`: add `"export": "<type> [<id>] [--output-path <path>] [--print] [--repo-dir <dir>] [--warnings <which>=all]"` to `TestRepresentativeSignatures` (confirm against the measured catalog — the projection owns the exact string) and an effects check `export` == `local-write process-control read` beside `open`'s. Update `TestPresenterWritesWarningsToStderr` in `internal/cli/presenter_test.go` to build an `app.ExportResult` (`app.NewEnvelope(app.OperationExport, app.ResultApplied)`, no `Target`) with the same notes, leftovers, mentions, and expected output.
  Run `go test -count=1 ./internal/cli/ -run 'Export|Presenter|Signatures'` → FAIL (undefined `exportRunner`/`newExportCommand`).

- [ ] **Step 8: Implement `internal/cli/export.go`.** `exportRunner` defaults to `app.Export`. `newExportCommand`: `Use: "export <type> [<id>]"`; `Short: "Write a clean Markdown copy of a change, spec, plan, or results file"`; `Args` runs `app.ParseExportArgs` then `exportOptionsFromFlags`; `Annotations: capability("export", EffectRead, EffectLocalWrite, EffectProcessControl)`; `RunE` as `open`'s (parse, `resolveRepoDir`, options, `openDepsFor`, `setResult(exportRunner(...))`). Flags: `output-path` (string, "", usage ``directory or exact file `path` for the copy, which is then not opened (default: ~/Downloads on macOS, the home directory on Linux)``), `warnings` (string, `app.WarningsAll`, usage ``"`which` warnings to report: all, leftovers, or mentions"``), `print` (bool, `"print the copy's path without opening it"`), `repo-dir` (as `open`'s). `exportOptionsFromFlags`: `--output-path` changed-but-empty → `--output-path needs a path`; `Warnings` set only when the flag changed, then `app.ValidateExportWarnings`. `Long` text (exact copy):

  ```
  Without an id, exports the change whose recorded branch is checked out. It
  always reads the local metadata checkout. The copy drops the frontmatter,
  docket's generated blocks and markers, every link (keeping its text), and
  docket-only sections; explicit change references become quoted titles. Fenced
  code and inline code stay exactly as written.

  The copy is named <date>-<kind>-<name>.md, kind one of ticket, spec, plan, or
  results. By default it lands in ~/Downloads on macOS or the home directory on
  Linux and opens. --output-path names a directory to write into or the exact
  file to write, and opens nothing; --print opens nothing wherever the copy
  goes. An existing file is replaced. Leftover docket machinery and mentions of
  docket are reported as warnings on stderr; --warnings picks which. Warnings
  never block.
  ```

  In `root.go` build `exportCmd := newExportCommand(func(r app.OperationResult) { result = r })` and add it to `root.AddCommand` beside `openCmd`. In `install.go` add `"export": true, // reads the corpus, writes the copy, and launches the system opener; no installed assets` beside `"open"`.

- [ ] **Step 9: Run the package tests.** `go test -count=1 ./internal/app/ ./internal/cli/` → PASS, including `TestSchemaCatalogCorrespondence`, `TestProductionCapabilityCorrespondence`, and the asset-dependence classification test in `root_test.go`.

- [ ] **Step 10: Mutation probes (restore from a backup copy, never `git checkout --`).** (a) In `Export`, launch when `!o.Print` only (drop the `OutputPath == ""` condition) → `TestExportLaunchesOnlyAtTheDefaultDestination` reddens. (b) Drop the `ValidateExportWarnings` call from `exportOptionsFromFlags` → `TestExportCLIUsageErrors` reddens. (c) Change the `exported_path` JSON tag → `TestExportResultJSON` reddens. Confirm each mutation landed (`grep -c`) before reading the result; restore and re-run green.

- [ ] **Step 11: Commit.**

  ```bash
  git add internal/app/export.go internal/app/export_test.go internal/app/open.go internal/app/schema_registry.go \
    internal/cli/export.go internal/cli/export_test.go internal/cli/root.go internal/cli/install.go \
    internal/cli/capability_production_test.go internal/cli/presenter_test.go
  git commit -m "feat(cli): add docket export as its own command"
  ```

### Task 2: Remove the export from `docket open`

**Files:**
- Modify: `internal/app/open.go`, `internal/app/export.go`, `internal/app/export_test.go`.
- Modify: `internal/cli/open.go`, `internal/cli/open_test.go`, `internal/cli/capability_production_test.go`.

**Interfaces:**
- Consumes: Task 1's `Export`, `writeCleanCopy`, `openPrelude`, `runOpener`, `exportWarningLines`.
- Produces: `OpenOptions` = `{RepoDir string; What string; ID int; Print bool}`; `OpenResult` without `ExportedPath`, `Replaced`, `Leftovers`, `Mentions`, and without a `HumanWarnings` method. `ValidateOpenExport`, `openExport`, and `ReasonOpenInvalidOption` no longer exist.

- [ ] **Step 1: Write the failing CLI test.** In `internal/cli/open_test.go` add `TestOpenCLIRejectsExportFlags`: for each of `{"open","spec","5","--export"}`, `{"open","spec","5","--output-path","x"}`, `{"open","spec","5","--warnings","all"}` → exit 2, stderr contains `unknown flag: --export` / `--output-path` / `--warnings` respectively, runner never called. Also `open --help` output contains none of `--export`, `--output-path`, `--warnings`, `clean Markdown copy`. Change `TestRepresentativeSignatures`' `open` entry to `"<what> [<id>] [--print] [--repo-dir <dir>]"`. Run `go test -count=1 ./internal/cli/ -run 'OpenCLIRejectsExportFlags|Signatures'` → FAIL.

- [ ] **Step 2: Delete the tests whose subject is gone.** `internal/cli/open_test.go`: delete `TestOpenCLIExportOptions`, `TestOpenCLIExportUsageErrors`, `TestOpenCLIExportDepsWiring`, `TestOpenCLIHelpNamesTicketKind` (their export counterparts landed in Task 1). `internal/app/export_test.go`: delete `TestValidateOpenExport`, `TestOpenResultExportJSON`, and the transitional `TestExportMatchesOpenExport` (byte identity stays pinned by `TestExportWritesCleanCopy`'s literal `exportSpecClean`).

- [ ] **Step 3: Strip the export from the app layer.** `open.go`: remove `OpenOptions.Export`/`OutputPath`/`Warnings` and their comment, the four export fields of `OpenResult` and their comment, `ReasonOpenInvalidOption`, the `ValidateOpenExport` call, and the `if o.Export` branch; drop the "With Export set…" sentence from `Open`'s doc comment. Keep `OpenDeps`' `HomeDir`/`Getwd`/`GOOS` and reword their comment to "The export's destination seams (Export); production sets all three." `export.go`: delete `ValidateOpenExport`, `openExport`, and `OpenResult.HumanWarnings`; fix the header comment to describe `docket export` only.

- [ ] **Step 4: Strip the export from the CLI.** `internal/cli/open.go`: remove the `export`, `output-path`, `warnings` flag registrations and the two export paragraphs from `Long`; the `Args` hook becomes `ParseOpenArgs` only; `openOptionsFromFlags` returns `app.OpenOptions` (no error) reading only `print`; drop the unused `errors` import.

- [ ] **Step 5: Verify.** `go build ./... && go vet ./internal/app/ ./internal/cli/ && go test -count=1 ./internal/app/ ./internal/cli/` → PASS. Run `grep -rn -e 'ValidateOpenExport' -e 'ReasonOpenInvalidOption' -e 'o.Export' internal` → no hits.

- [ ] **Step 6: Mutation probe.** Re-register `cmd.Flags().Bool("export", false, "")` in `newOpenCommand` → `TestOpenCLIRejectsExportFlags` reddens; restore from a backup copy, re-run green.

- [ ] **Step 7: Commit.**

  ```bash
  git add internal/app/open.go internal/app/export.go internal/app/export_test.go \
    internal/cli/open.go internal/cli/open_test.go internal/cli/capability_production_test.go
  git commit -m "refactor(cli): drop --export, --output-path, and --warnings from docket open"
  ```

### Task 3: Point the skill, docs, and sentinel at `docket export`

**Files:**
- Modify: `skills/docket-export/SKILL.md`, `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go` (only if the counts move), `docs/reference/cli.md`, `docs/reference/glossary.md`.
- Regenerate: `internal/assets/embedded/tree/` (`go generate ./internal/assets/`).

**Interfaces:**
- Consumes: the `export` catalog entry from Task 1.

- [ ] **Step 1: Derive the site list.** `grep -rn -e '--export' -e 'output-path' -e '--warnings' -e '`open`' --include='*.md' --include='*.go' .` and sort hits into maintained source vs point-in-time records (`docs/changes/archive/`, `docs/results/`, `docs/release*/`, `docs/superpowers/`, `docs/adrs/`, `testdata/`, and the generated `internal/assets/embedded/` are records or generated). Expected maintained prose sites: `skills/docket-export/SKILL.md` §1 and §3, `docs/reference/cli.md` (`docket open` bullet), `docs/reference/glossary.md` (`docket-export` entry), and the `change_0578_export_skill` sentinel. `skills/docket-link-ticket/references/create.md`'s `<open argv> spec <id> --print --json` is a real open and stays. Fix any additional maintained hit the same way.

- [ ] **Step 2: Update the sentinel first (it fails).** In the first `change_0578_export_skill` entry for `skills/docket-export/SKILL.md`, replace the present clause `` "`--export --print --json`" `` with `` "Run the `export` argv as `<export argv> <what> [<id>] --print --json --repo-dir <repo root>`" `` and add `` "take the `argv` of the entries `export`, `repository.prepare`, and `diagnostic.config`" ``; add `"--export"` and `` "the `open` argv" `` to that entry's `absent` list. Keep each clause on one source line of the skill. Run `go test -count=1 ./internal/repoguard/ -run TestProseContracts` → FAIL.

- [ ] **Step 3: Edit the skill.** §1: "take the `argv` of the entries `export`, `repository.prepare`, and `diagnostic.config`". §3: "Run the `export` argv as `<export argv> <what> [<id>] --print --json --repo-dir <repo root>`, …" and "Always `--print --json`: `--print` opens no viewer — …" (the rest of §3 — file request, outside-tool request with `--output-path`, `--warnings` rule, failed-export rule — unchanged).

- [ ] **Step 4: Edit the docs.** `docs/reference/cli.md`: insert after the `docket evidence` bullet:

  ```
  - **`docket export`** — write a clean Markdown copy of a change, spec, plan, or results file,
    free of docket's links and generated blocks, and open it. Without an id it exports the change
    whose branch is checked out. Agent sessions reach the export through the `docket-export`
    skill. Details: `docket export --help`.
  ```

  and cut the `docket open` bullet to: open the board, or a change's record, spec, plan, results, or pull request, as a GitHub page or from the local metadata checkout; without an id it opens the change whose branch is checked out; `Details: docket open --help`. Keep the sentence `Agent sessions reach the export through the `docket-export` skill.` on one line (the sentinel pins it). `docs/reference/glossary.md`, `docket-export` entry: "It runs the `export` operation (`--print --json`) and passes the clean copy …".

- [ ] **Step 5: Regenerate and rebudget.** `go generate ./internal/assets/`. Run `go test -count=1 ./internal/repoguard/ ./internal/assets/` → PASS. If the skill-size budget for `docket-export/SKILL.md` reddens, set its row to the measured counts and prepend `0589: the export runs through the export operation (old -> new); ` to its comment, following the row's existing style.

- [ ] **Step 6: Mutation probes.** (a) Put `` `--export --print --json` `` back into the skill's §3 → `TestProseContracts` reddens on the absent clause. (b) Change §1's `export` entry name back to `open` → reddens on both the present and the absent clause. Restore from a backup copy; confirm green.

- [ ] **Step 7: Commit.**

  ```bash
  git add skills/docket-export/SKILL.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go \
    docs/reference/cli.md docs/reference/glossary.md internal/assets/embedded
  git commit -m "docs(skills): route the export skill and references through docket export"
  ```

### Task 4: Whole-suite gate

- [ ] **Step 1:** From the feature worktree run `go run ./cmd/docket development test`. Expected: green. Read the budget report even on green and act only on lines `tests/README.md` names as breaches.
- [ ] **Step 2:** Manual smoke against this repository's own metadata checkout, from a scratch destination: `dest=$(mktemp -d "${TMPDIR:-/tmp}/docket-export.XXXXXX")`; `go run ./cmd/docket export spec 589 --output-path "$dest" --json` → `result: applied`, `launched: false`, `operation: export`, no `target` key; `go run ./cmd/docket open spec 589 --export` → exit 2 `unknown flag: --export`; `go run ./cmd/docket export board` → exit 2. Nothing to commit.
