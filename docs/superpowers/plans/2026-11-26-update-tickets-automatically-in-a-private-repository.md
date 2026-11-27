<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0588 — Update tickets automatically in a private repository](../../changes/archive/2026-11-27-0588-update-tickets-automatically-in-a-private-repository.md)**
<!-- docket:backlink:end -->

# Update tickets automatically in a private repository Implementation Plan

> **For agentic workers:** executed by `docket-build` (task-by-task under the `docket-build-task` contract). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an automatic ticket update in a private repository post cleaned attachments behind a final leak check, run by a new read-only `docket leak check --file …` operation that reuses the publish leak check's matcher and options.

**Architecture:** A new app operation `LeakCheck` (`internal/app/leak_check_file.go`) pins the repository once, builds the snapshot, derives `leakscan.Options` through the existing `leakOptions(pin, snap)` helper, and scans each given file's base name (line 0) and lines with `internal/leakscan`. A thin Cobra adapter `docket leak check` exposes it as capability `leak.check` (effect `read`). The `docket-update-ticket` skill's automatic path stops refusing private repositories: it routes to `references/private.md`, whose new *Unattended* rules export without asking, show no before → after, and post only when `leak.check` returns `applied` and `clean`. Docs and prose contracts move with the text.

**Tech Stack:** Go (stdlib + cobra), Markdown skills mirrored into `internal/assets/embedded/tree` by `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-11-27-update-tickets-automatically-in-a-private-repository-design.md` (on the metadata branch; read it from the metadata worktree). Every decision number below (D1–D6) refers to it.

## Global Constraints

- The operation's capability id is exactly `leak.check`; the command is `docket leak check --file <path> [--file <path> …] [--repo-dir <dir>] [--json]`; its effect is exactly `read`.
- Options come from `leakOptions(pin, snap)` and nothing else: `match_word` is the effective `leak_check.match_word`, and the change ids are every change in the snapshot (D5). Never build `leakscan.Options` by hand in the new code.
- Each file is scanned on its lines (split on `\n`, a trailing `\r` trimmed, numbered from 1: exactly `leakscan.Text`) and on its **base name** (line 0). The directory part of a path is never scanned.
- Result `applied` always carries `context.clean`, `context.match_word`, and `context.hits` (an empty JSON array `[]`, never `null`). Each hit is `{file, line, text, rule}`: `file` is the path as given, `line` 0 for the name (serialized, no `omitempty`), `text` the matched text only, `rule` the leakscan rule name. Hits are per file in argument order, the name first.
- A hit is reported under `applied`, never refused. A missing `--file` (none given, or an empty value) and a file that cannot be read are refused as `invalid-input`, with no context, so a bad path can never read as clean. Every file is read before the repository is pinned (learning: validate-the-whole-input-set-first).
- The operation scans in any visibility. `leakCheckApplies` and the publish callers are not touched (D5, D6).
- Never touch `internal/exportcheck`, the `docket-export` skill, the leakscan rules, the on-request path's behaviour, or shared-repository delivery (Non-goals, Out of scope).
- No ticket-system call is made from this repository, in tests or otherwise, and `.docket.yml` gains no `tickets` key.
- Docs describe only current behaviour: no change or PR citations, no history of the earlier rule. Cross-references anchor on symbols or quoted clauses, never line numbers (AGENTS.md).
- Skills hard-code no `docket <family> <verb>` invocation: they name `the \`leak.check\` operation (argv from the capability catalog)` (guard: `internal/repoguard/capability_surface_test.go`).
- Every skill edit is mirrored with `go generate ./internal/assets/` in the same commit; so is `.docket.example.yml`.
- Every grown skill file gets its `internal/repoguard/budgets_test.go` row raised to the exact new line/word counts, with a leading `0588: <what> (old -> new);` note, per that table's convention.
- Every new contract row and guard is mutation-tested (strip the pinned clause or the guarded code, watch it redden, restore from a backup copy with `cp f f.bak; <mutate>; <run>; mv -f f.bak f`), with the Go test cache defeated (`-count=1`).
- The ADR of D6 is **not** a build task: the parent records it with `docket-adr` after the build. See *Parent hand-off* at the end.

## Review Focus

1. **The capability catalog's 16 KB ceiling.** The catalog measures 16382 bytes at the pre-build HEAD against a 16384-byte ceiling (`TestCapabilitiesPayloadWithinByteBudget`), so one new entry breaches it. A new operation is the design event that comment's history documents (four prior one-KB raises, each for a spec that adds an operation). Task 2 raises the ceiling one KB step (16 KB → 17 KB) with a 0588 note and the measured byte count. A reviewer should confirm the new entry is a one-line stub and that nothing else grew.
2. **A comment body written as an escaped string.** If the comment file holds a JSON-escaped body (`"line one\ndocket"` with a literal backslash-n), the matcher sees one glued line and misses a `docket`, `#0412`, or `Docket-` trailer at the next line's start. The skill rule forbids it (Task 3), and Task 1 pins that real line breaks are what the scan splits on (`TestLeakCheckSplitsRealLineBreaksOnly`).
3. **A temp directory whose name contains "docket".** The exports live in `…/docket-update-ticket.XXXXXX/`. Scanning the full path would flag every file and nothing would ever post. Task 1's `TestLeakCheckNeverScansDirectory` pins that only the base name is scanned.
4. **CRLF exported files.** A file with `\r\n` endings must match the same as `\n` (a trailing `\r` would otherwise defeat end-anchored rules). Task 1's parity corpus is also run with CRLF endings (`TestLeakCheckMatchesPublishCheck/crlf`).
5. **A directory or a dangling path passed as `--file`.** Either must refuse `invalid-input` (`unreadable-file`) before any pin, never produce a clean result. Task 1's `TestLeakCheckRefusesUnreadableFile` covers a missing path and a directory.

---

### Task 1: The `LeakCheck` app operation

**Files:**
- Modify: `internal/leakscan/leakscan.go` (add one `Source` constant)
- Create: `internal/app/leak_check_file.go`
- Create: `internal/app/leak_check_file_test.go`
- Modify: `internal/app/shadow_test.go` (one case line in `TestEnvelopeNotShadowed`)

**Interfaces:**
- Consumes: `leakOptions(pin StatusPin, snap domain.Snapshot) leakscan.Options` and `runLeakCheck` (test-only, for parity) from `internal/app/leak_check.go`; `leakscan.Text`, `leakscan.Line`; `PlanningDeps{Reader}` with `PinContext`, `ReadCorpus`; `parseCorpus`, `repository.BuildSnapshot`, `classifyStatusError`, `NewEnvelope`. Test fixtures: `prReader(t)` (`internal/app/pr_publish_test.go`, change 7 in its corpus), `leakContext(t, matchWord)` and `fakeLeakGit` (`internal/app/leak_check_test.go`), `contextDeps(fake)`, `fakeReader.pinCount`/`pinErr`, `testsupport.TempDir(t)`.
- Produces (Task 2 relies on these exact names):
  - `const OperationLeakCheck = "leak.check"`
  - `const ReasonLeakCheckMissingFile = "missing-file"`, `const ReasonLeakCheckUnreadableFile = "unreadable-file"`
  - `type LeakCheckRequest struct { Files []string }`
  - `type LeakCheckHit struct { File string \`json:"file"\`; Line int \`json:"line"\`; Text string \`json:"text"\`; Rule string \`json:"rule"\` }`
  - `type LeakCheckContext struct { Clean bool \`json:"clean"\`; MatchWord bool \`json:"match_word"\`; Hits []LeakCheckHit \`json:"hits"\` }`
  - `type LeakCheckResult struct { Envelope; Context *LeakCheckContext \`json:"context,omitempty"\`; Reason string \`json:"reason,omitempty"\`; Message string \`json:"message,omitempty"\` }` with `func (r LeakCheckResult) HumanText() string`
  - `func LeakCheck(ctx context.Context, deps PlanningDeps, repoDir string, req LeakCheckRequest) LeakCheckResult`
  - `leakscan.SourceFile Source = "file"`

- [ ] **Step 1: Write the failing tests** in `internal/app/leak_check_file_test.go`.

  Helper `writeLeakFile(t, dir, name, body string) string` writes a file and returns its path. Helper `runFileCheck(t, reader *fakeReader, files ...string) LeakCheckResult` calls `LeakCheck(context.Background(), contextDeps(reader), "", LeakCheckRequest{Files: files})`.

  The parity corpus (one rule or one deliberate miss per line; change 7 exists in `prReader`'s corpus, 42 does not):

  ```go
  const leakParityCorpus = "docket:status set\n" +
      "Docket-Change: 7\n" +
      "see .docket/x and .git/dckt/y\n" +
      "the dckt tool\n" +
      "fixes #0007\n" +
      "see (0007)\n" +
      "change 0007\n" +
      "change #7 and changes #7\n" +
      "the docket word\n" +
      "ADR-0144 stays\n" +
      "#7 unpadded\n" +
      "#0042 unknown id\n" +
      "see 0007 bare\n" +
      "the backlog and the board\n" +
      "x#0007 glued\n" +
      "clean last line"
  ```

  Tests and their assertions:

  - `TestLeakCheckMatchesPublishCheck` — subtests over `matchWord ∈ {true, false}` × endings `{"lf", "crlf"}` (crlf: `strings.ReplaceAll(corpus, "\n", "\r\n")`). For each: `wc, target := leakContext(t, matchWord)`; publish hits = `runLeakCheck(ctx, &fakeLeakGit{fetchRev: gitcli.Revision{Commit: leakBase}}, wc, target, leakHead, &leakscan.PRText{Body: body})`, filtered to `Source == "pr-body"` and projected to `(Line, Text, Rule)`. Operation: a `*fakeReader` whose `pin` is `wc.pin` and whose `corpus` is `prReader(t).corpus`; write `body` to `body.md` in a temp dir; run `LeakCheck`; take hits with `Line > 0` projected the same way. Assert `reflect.DeepEqual(op, publish)`. Non-vacuity in the same subtest: with `matchWord` true the op's hits include every rule `marker, trailer, path, alias, change-ref, word`; with `matchWord` false they include no `word` hit and still include `change-ref`; in both, no hit's line is the `ADR-0144`, `#7 unpadded`, `#0042`, `see 0007 bare`, or `backlog` line (lines 10–15).
  - `TestLeakCheckSplitsRealLineBreaksOnly` — file body `"ok\\ndocket here"` (a literal backslash-n, one line) with match_word true → no hit on line 2 and no `word` hit at all (the `docket` is glued to `n`); file body `"ok\ndocket here"` → exactly one hit `{Line: 2, Text: "docket", Rule: "word"}`.
  - `TestLeakCheckScansFileNameAsLineZero` — file `dckt-notes.md` with body `"clean\n"` → hits `[{File: <path as given>, Line: 0, Text: "dckt", Rule: "alias"}]`, `Clean` false, `Result` `applied`.
  - `TestLeakCheckNeverScansDirectory` — file `notes.md` (clean body) inside `filepath.Join(testsupport.TempDir(t), "docket-update-ticket.abc123", ".docket")` → `Clean` true, `Hits` empty, and `mustJSON(t, res)` contains `"hits":[]`.
  - `TestLeakCheckReportsHitsUnderApplied` — body `"see .docket/x\n"` → `Result == ResultApplied`, `Reason == ""`, `Clean == false`, one hit `{Line: 1, Text: ".docket", Rule: "path"}`; and the JSON contains `"line":0` for a name hit in a second case (`dckt.md`).
  - `TestLeakCheckOrdersFilesAsGiven` — files `b-dckt.md` (body `"x\n.docket\n"`) then `a-dckt.md` (same body), passed as `b, a` → hit files in order `b, b, a, a` with lines `0, 2, 0, 2`.
  - `TestLeakCheckReportsMatchWordUsed` — pin with `LeakCheck.MatchWord = config.Value[bool]{Value: false, Explicit: true}` → `Context.MatchWord == false`; with `true` → `true`.
  - `TestLeakCheckRefusesMissingFile` — `LeakCheckRequest{}` and `LeakCheckRequest{Files: []string{""}}` → `Result == ResultInvalidInput`, `Reason == ReasonLeakCheckMissingFile`, `Context == nil`, `reader.pinCount == 0`.
  - `TestLeakCheckRefusesUnreadableFile` — files `[clean.md, <dir>/absent.md]` and `[clean.md, <a directory>]` → `ResultInvalidInput`, `ReasonLeakCheckUnreadableFile`, `Context == nil`, `Message` contains the bad path, `reader.pinCount == 0`.
  - `TestLeakCheckPinFailureIsNotClean` — `reader.pinErr = errors.New("boom")` with a readable clean file → `Result != ResultApplied` and `Context == nil`.
  - `TestLeakCheckHumanText` — clean → contains `"leak.check: clean"`; hits → contains `"hit(s)"`, the file path, and the matched text; refusal → `"leak.check: invalid-input (missing-file)"`.

  Add to `TestEnvelopeNotShadowed`'s cases in `internal/app/shadow_test.go`: `{"leak.check", newLeakCheckResult(ResultApplied, "", "", &LeakCheckContext{Clean: true})},`.

- [ ] **Step 2: Run to verify they fail**

  Run: `go test -count=1 ./internal/app -run 'TestLeakCheck|TestEnvelopeNotShadowed'`
  Expected: FAIL to compile (`undefined: LeakCheck`, `undefined: LeakCheckRequest`, …).

- [ ] **Step 3: Add `SourceFile Source = "file"`** to the `Source` const block in `internal/leakscan/leakscan.go`, commented `// a file checked by the leak check operation`.

- [ ] **Step 4: Implement `internal/app/leak_check_file.go`**

  A file comment states: the read-only `leak check` operation scans arbitrary outbound files with the publish leak check's own rules and options (`leakOptions`), in any visibility; a hit is reported, not refused; a missing or unreadable file is refused so it can never read as clean.

  `newLeakCheckResult(result Result, reason, message string, c *LeakCheckContext) LeakCheckResult` normalizes `c.Hits` nil → `[]LeakCheckHit{}`.

  `LeakCheck` in this order:
  1. Refuse `missing-file` when `len(req.Files) == 0` or any entry is `""`.
  2. Read every file with `os.ReadFile` (path as given, relative to the process working directory); on the first failure refuse `unreadable-file` with message `fmt.Sprintf("cannot read %s: %v", path, err)`. A directory fails `os.ReadFile` on its own.
  3. Pin, read the corpus, `parseCorpus`, `repository.BuildSnapshot` exactly as `ContextTicketRead` does, with the same error mapping (`classifyStatusError`; a snapshot error is `ResultInternalError`/`ReasonStatusInternalError`).
  4. `opts := leakOptions(pin, snap)`; then `scanLeakFiles(files, opts)`.
  5. Return `applied` with `Clean: len(hits) == 0`, `MatchWord: opts.MatchWord`.

  `scanLeakFiles(files []leakFile, opts leakscan.Options) []LeakCheckHit` where `type leakFile struct{ path string; data []byte }`: per file, `leakscan.Line(filepath.Base(f.path), opts)` → a line-0 hit, then `leakscan.Text(leakscan.SourceFile, string(f.data), opts)` → line hits; `File` is always `f.path`.

  `HumanText`: applied and clean → `leak.check: clean (<n> file(s), match_word <bool>)`; applied with hits → `leak.check: <k> hit(s): <file>:<line> "<text>" (<rule>); …`, naming at most `leakCheckShown` hits then `… and <m> more`; otherwise `leak.check: <result> (<reason>)`.

- [ ] **Step 5: Run to verify they pass**

  Run: `go test -count=1 ./internal/app -run 'TestLeakCheck|TestEnvelopeNotShadowed|TestRunLeakCheck' && go test -count=1 ./internal/leakscan`
  Expected: PASS.

- [ ] **Step 6: Mutation-test the parity guard**

  With a backup copy of `internal/app/leak_check_file.go`: (a) replace `leakOptions(pin, snap)` with `leakscan.Options{MatchWord: true, ChangeIDs: nil}`; run `go test -count=1 ./internal/app -run TestLeakCheckMatchesPublishCheck`; expected FAIL (the `false` subtests differ on `word`, every subtest differs on `change-ref`). (b) Scan `f.path` instead of `filepath.Base(f.path)`; expected FAIL in `TestLeakCheckNeverScansDirectory`. (c) Move the file reads after the pin; expected FAIL on `pinCount == 0`. Restore with `mv -f` from the backup after each and re-run green.

- [ ] **Step 7: Commit**

  ```bash
  git add internal/leakscan/leakscan.go internal/app/leak_check_file.go internal/app/leak_check_file_test.go internal/app/shadow_test.go
  git commit -m "feat(leak): add the read-only leak check over outbound files"
  ```

### Task 2: The `docket leak check` command and its catalog entries

**Files:**
- Create: `internal/cli/leak.go`
- Create: `internal/cli/leak_test.go`
- Modify: `internal/cli/root.go` (build and `AddCommand` the `leak` group)
- Modify: `internal/cli/install.go` (`assetIndependent`)
- Modify: `internal/app/schema_registry.go` (`operationBindings`)
- Modify: `internal/cli/capability_production_test.go` (`TestCapabilitiesPayloadWithinByteBudget` ceiling and its comment)

**Interfaces:**
- Consumes: `app.LeakCheck`, `app.LeakCheckRequest`, `app.LeakCheckResult`, `app.OperationLeakCheck` from Task 1; `capability(id, EffectRead)`, `resolveRepoDir(c)`, `newPlanningDeps()`, `captureTree(t)`, `runCLI(t, …)`, `capAnnotationID`, `capAnnotationEffects`.
- Produces: `func newLeakCommand(setResult func(app.OperationResult)) *cobra.Command`; catalog entry `leak.check` with signature `[--file <path>...] [--repo-dir <dir>]`.

- [ ] **Step 1: Write the failing tests** in `internal/cli/leak_test.go`, modelled on `internal/cli/context_test.go`:
  - `TestLeakCheckRegistered` — `root.Find([]string{"leak", "check"})` resolves a command named `check` with flags `file` and `repo-dir`; `Annotations[capAnnotationID] == "leak.check"`; `Annotations[capAnnotationEffects] == string(EffectRead)`; `--file` is repeatable (`cmd.Flags().Lookup("file").Value.Type() == "stringArray"`, so a comma in a path is never split).
  - `TestLeakCheckCLIMissingFile` — `runCLI(t, "leak", "check", "--repo-dir", testsupport.TempDir(t), "--json")` → output contains `"operation":"leak.check"`, `"result":"invalid-input"`, `"reason":"missing-file"`, and is exactly one newline-terminated document.
  - `TestLeakCheckCLIRoutesEveryFile` — two `--file` flags, the second a missing path → `"reason":"unreadable-file"` and the output names the missing path (proves both values reach the operation).
  - `TestLeakCommandsAssetIndependent` — `assetIndependent["leak"]` and `assetIndependent["leak check"]` are true.

- [ ] **Step 2: Run to verify they fail**

  Run: `go test -count=1 ./internal/cli -run 'TestLeak'`
  Expected: FAIL (`undefined: newLeakCommand` or command not found).

- [ ] **Step 3: Implement**
  - `internal/cli/leak.go`: a group `leak` (Short `Check text bound for outsiders against the leak check (read-only)`, `Args: cobra.NoArgs`, `RunE` returns `errors.New("missing command")`, like `newContextCommand`) with subcommand `check` (Short `Scan files for docket fingerprints with the publish leak check's rules (read-only)`, `Annotations: capability("leak.check", EffectRead)`). Flags: `StringArray("file", nil, "a `path` to scan, its name included (repeatable)")` and `String("repo-dir", "", "repository `dir` to read (default: current directory)")`. Do not mark `--file` required: the operation refuses a missing file as `invalid-input`.
  - `internal/cli/root.go`: `leakCmd := newLeakCommand(func(r app.OperationResult) { result = r })` beside `exportCmd`, and add `leakCmd` to `root.AddCommand(...)` after `learningCmd`'s neighbours (order is cosmetic).
  - `internal/cli/install.go`: `"leak": true, // the group itself; it reports a missing command` and `"leak check": true,`.
  - `internal/app/schema_registry.go`: `{ID: "leak.check", Request: nil, Result: LeakCheckResult{}}, // LeakCheck` placed in sorted order (before `learning.record`).

- [ ] **Step 4: Run the catalog guards and measure**

  Run: `go test -count=1 ./internal/cli ./internal/app -run 'TestLeak|TestAssetIndependentSetExact|TestProductionCapabilityCorrespondence|TestProductionEffectsCompleteAndClosed|TestPublishedRequestIsTheDecodedJSONFile|TestOperationBindings|TestCapabilitiesPayloadWithinByteBudget' -v 2>&1 | grep -E -e '^(---|ok|FAIL)|capabilities payload'`
  Expected: everything PASS except `TestCapabilitiesPayloadWithinByteBudget`, which logs a payload just over 16384 bytes.

- [ ] **Step 5: Raise the catalog ceiling one KB step**

  In `TestCapabilitiesPayloadWithinByteBudget` change the ceiling to `17*1024`, the log to `(budget 17408)`, the failure text to `17KB`, the doc comment's first sentence to `the 17408-byte (17 KB) design ceiling`, and append to the comment's history: `It was raised a fifth one KB step (16 KB → 17 KB) for change 0588, whose spec deliberately adds the \`leak.check\` operation to the catalog; its entry took the catalog from 16382 to <measured> bytes — the identical conscious design event, a one-line invocation stub and no inlined schemas.` Use the byte count Step 4 logged.

- [ ] **Step 6: Run to verify they pass**

  Run: `go test -count=1 ./internal/cli ./internal/app`
  Expected: PASS.

- [ ] **Step 7: Mutation-test**

  With backups: remove `"leak check"` from `assetIndependent` → `TestLeakCommandsAssetIndependent` and `TestAssetIndependentSetExact` FAIL; remove the `leak.check` schema binding → the registry/catalog correspondence test FAILS. Restore with `mv -f` and re-run green.

- [ ] **Step 8: Commit**

  ```bash
  git add internal/cli/leak.go internal/cli/leak_test.go internal/cli/root.go internal/cli/install.go internal/app/schema_registry.go internal/cli/capability_production_test.go
  git commit -m "feat(leak): expose the leak check as docket leak check"
  ```

### Task 3: The automatic path posts in a private repository

**Files:**
- Modify: `skills/docket-update-ticket/references/automatic.md`
- Modify: `skills/docket-update-ticket/references/private.md`
- Modify: `skills/docket-update-ticket/SKILL.md` (the reads paragraph only)
- Modify: `internal/repoguard/ticket_auto_updates_contracts_test.go`
- Modify: `internal/repoguard/budgets_test.go` (rows for the three skill files)
- Regenerate: `internal/assets/embedded/tree/skills/docket-update-ticket/**` via `go generate ./internal/assets/`

**Interfaces:**
- Consumes: the `leak.check` operation id and its result shape (`result`, `context.clean`, `context.hits[].{file,line,text,rule}`) from Tasks 1–2.
- Produces: the pinned skill clauses below, which Task 4's docs describe.

The exact copy below is what the contract rows pin. Wording may be tightened only if the matching row changes in the same commit; the rules may not change.

- [ ] **Step 1: Write the failing contract rows** in `autoTicketContracts`:

  For `skills/docket-update-ticket/references/automatic.md` — remove from `present`: ``"when `visibility` is anything other than `shared`"``, `"automatic updates are not available in a private repository"`, ``"Never load `references/private.md` on this path."``. Add to `present`:
  - ``"when `visibility` is neither `shared` nor `private` (`the repository's visibility could not be read`)"``
  - ``"When `visibility` is `shared`, **read `references/shared.md` now (blocking)**"``
  - ``"When `visibility` is `private`, **read `references/private.md` now (blocking)** and follow its unattended rules"``
  - ``"The one exception is private delivery's stop on a rewrite"``

  Add an `absent` list with the three removed clauses.

  Add a new row for `skills/docket-update-ticket/references/private.md` with `present`:
  - `"**Unattended.**"`
  - `"ask nobody; on a word-check hit, return the path and the lines instead of stopping"`
  - `"A word-check hit posts nothing: no attachment and no comment; report one line"`
  - `"**No before → after.** Show and report no rewrite list, and post without anyone's go."`
  - `"After the comment is written and the attachments are final (a renamed file included), and before anything is uploaded"`
  - `"plain text with real line breaks, in Markdown or the ticket system's own markup, never an escaped or structured body such as a JSON document"`
  - `"converting the formatting only, never the words"`
  - ``"run the `leak.check` operation (argv from the capability catalog) once, with `--file <comment file>`, one `--file` per attachment, and `--json`"``
  - ``"`result` `applied` with `context.clean` `true`: post, attachments first, comment last."``
  - `"Any other result means the check did not run, which is never clean: post nothing and report one line naming the reason."`
  - `"**Never fix and check again.**"`

  For `skills/docket-update-ticket/SKILL.md` add to `present`: ``"on the automatic path in a private repository it also runs the `leak.check` operation over everything it is about to post"``.

  Raise the `floor` constant in `TestAutoTicketUpdateContracts` to the new total `present + absent` count (measure it: the test's `checks` value), so the table cannot silently shrink.

- [ ] **Step 2: Run to verify they fail**

  Run: `go test -count=1 ./internal/repoguard -run TestAutoTicketUpdateContracts`
  Expected: FAIL naming each new missing phrase and each forbidden phrase still present.

- [ ] **Step 3: Edit `references/automatic.md`**
  - **Never ask and never wait.** bullet: append ` The one exception is private delivery's stop on a rewrite: its unattended rules post without asking, behind a final leak check.`
  - **Otherwise post nothing and report one line** bullet: replace the clause ``when `visibility` is anything other than `shared` (`automatic updates are not available in a private repository`);`` with ``when `visibility` is neither `shared` nor `private` (`the repository's visibility could not be read`);``.
  - **Otherwise post** bullet becomes: ``**Otherwise post** exactly what an on-request update for this event posts: its row of section 3's table and the editorial standard. When `visibility` is `shared`, **read `references/shared.md` now (blocking)** and post the links. When `visibility` is `private`, **read `references/private.md` now (blocking)** and follow its unattended rules: cleaned attachments, nobody asked, no before → after, and a post only on a clean final leak check.`` (The sentence ``Never load `references/private.md` on this path.`` is gone.)

- [ ] **Step 4: Append the *Unattended* block to `references/private.md`** (all existing bullets stay verbatim; they are the on-request rules and are pinned by `internal/repoguard/prose_contracts_test.go` row `change_0586_update_ticket_private`):

  ```markdown
  - **Unattended.** On the automatic path (`references/automatic.md`) nobody is there to review the cleanup, so three rules above change and a final leak check stands in for the review. Everything else above holds.
    - **The export asks nobody.** Add to the `DIRECTED to:` direction for `docket-export`: ask nobody; on a word-check hit, return the path and the lines instead of stopping. A word-check hit posts nothing: no attachment and no comment; report one line naming the file.
    - **No before → after.** Show and report no rewrite list, and post without anyone's go.
    - **The final leak check.** After the comment is written and the attachments are final (a renamed file included), and before anything is uploaded, write the comment to a file in the same temp directory as the exports, exactly as it will be posted: plain text with real line breaks, in Markdown or the ticket system's own markup, never an escaped or structured body such as a JSON document. An escaped line break glues the next line onto the one before and hides a match at its start. When the connector takes a structured format, build it from the checked text afterwards, converting the formatting only, never the words. Name the file so its name holds nothing the check flags (`comment.md`). Then run the `leak.check` operation (argv from the capability catalog) once, with `--file <comment file>`, one `--file` per attachment, and `--json`.
    - **Only a clean result posts.** `result` `applied` with `context.clean` `true`: post, attachments first, comment last. `applied` with hits: post nothing, and report one line naming where the check hit (the comment, or which attachment) and the rule, and the exported files' paths: `Not posted to CLOUD-123 (PR opened): the leak check found a change reference ("#0412") in the results attachment. Exported files: <paths>.` Any other result means the check did not run, which is never clean: post nothing and report one line naming the reason.
    - **Never fix and check again.** A hit posts nothing: do not reword a flagged line and run the check again.
  ```

- [ ] **Step 5: Edit `SKILL.md`'s reads paragraph** (the paragraph that begins `This skill makes no docket metadata write.`): after ``may read `leak_check.match_word` through `diagnostic.config` (section 4)`` insert ``, and on the automatic path in a private repository it also runs the `leak.check` operation over everything it is about to post (`references/private.md`)`` before the period. The pinned phrase ending at `diagnostic.config` must stay byte-intact.

- [ ] **Step 6: Mirror and re-budget**

  Run: `go generate ./internal/assets/`. Then measure each of the three skill files (`wc -l` and the word count the budget test uses; run `go test -count=1 ./internal/repoguard -run TestSkillSizeBudgets -v` and read the reported sizes) and set their `skillBudgets` rows to the exact new counts with a leading note, e.g. `0588: the automatic path posts in a private repository behind the final leak check (14/386 -> L/W);`.

- [ ] **Step 7: Run to verify they pass**

  Run: `go test -count=1 ./internal/repoguard ./internal/assets ./internal/app`
  Expected: PASS (includes `TestAutoTicketUpdateContracts`, `TestProseContracts`, `TestSkillSizeBudgets`, `TestCapabilitySurface`, and the embedded-mirror checks).

- [ ] **Step 8: Derive the leftover sites from a whole-repository search**

  Run (from the feature worktree): `git grep -n -i -e 'not available in a private' -e 'other than .shared.' -e 'Never load .references/private.md.' -- ':!docs/superpowers' ':!docs/results' ':!internal/assets/embedded'`
  Expected: hits only in `internal/repoguard/ticket_auto_updates_contracts_test.go`'s `absent` lists. Any other skill hit is fixed in this task; doc hits belong to Task 4.

- [ ] **Step 9: Mutation-test each new row**

  For each new `present` phrase in `automatic.md`, `private.md`, and `SKILL.md`: back up the file, delete the phrase, run `go test -count=1 ./internal/repoguard -run TestAutoTicketUpdateContracts`, expect FAIL naming it, `mv -f` the backup back. For each `absent` clause: re-insert it into a backup-protected copy and expect FAIL. Record the result in the task report.

- [ ] **Step 10: Commit**

  ```bash
  git add skills/docket-update-ticket internal/assets/embedded/tree/skills/docket-update-ticket internal/repoguard/ticket_auto_updates_contracts_test.go internal/repoguard/budgets_test.go
  git commit -m "feat(update-ticket): post automatic private updates behind the final leak check"
  ```

### Task 4: Docs and the sample config

**Files:**
- Modify: `docs/concepts/tickets.md`
- Modify: `docs/reference/config-keys.md`
- Modify: `docs/reference/glossary.md`
- Modify: `docs/reference/skills-and-agents.md`
- Modify: `docs/reference/cli.md`
- Modify: `.docket.example.yml`; regenerate `internal/assets/embedded/tree/.docket.example.yml` via `go generate ./internal/assets/`
- Modify: `internal/repoguard/ticket_auto_updates_contracts_test.go` (doc rows)

**Interfaces:**
- Consumes: the behaviour Tasks 1–3 built; the command spelling `docket leak check --file <path>`.
- Produces: nothing later tasks use.

- [ ] **Step 1: Write the failing contract rows** in `autoTicketContracts`:
  - `docs/concepts/tickets.md`: move `"an automatic update posts nothing and reports one line"` from `present` to `absent`; add `absent` `"From then on, in a shared repository, the switches"`; add `present` `"a final leak check runs over the comment and every attachment before anything is uploaded"`, `"In a private repository, an automatic update is posted only when the final leak check finds nothing in the comment or any attachment."`, and `"ADR-0144"`.
  - `docs/reference/glossary.md`: move `"An automatic update in a private repository posts nothing and reports one line."` to `absent`; add `present` `"An automatic update there is posted only when the leak check finds nothing in the comment or any attachment."`, `"### Leak check"`, `"- [Leak check](#leak-check)"`.
  - `docs/reference/config-keys.md`: add `absent` `"In a private repository an automatic update posts nothing and reports one line."`; add `present` ``"| `leak_check.match_word` | `true` | any layer |"`` and `"is posted only when the leak check finds nothing in the comment or any attachment"`.
  - `docs/reference/skills-and-agents.md`: add `absent` `"an automatic update there posts nothing"`; add `present` `"an automatic update there is posted only when a final leak check finds nothing in it"`.
  - `.docket.example.yml`: add `present` `"an automatic ticket update whose comment or attachments contain it is not posted"`.
  - Keep the floor equal to the new total count.

- [ ] **Step 2: Run to verify they fail**

  Run: `go test -count=1 ./internal/repoguard -run TestAutoTicketUpdateContracts`
  Expected: FAIL naming the new phrases.

- [ ] **Step 3: Edit the docs**
  - `docs/concepts/tickets.md`:
    - *What goes with an update* becomes: in a shared repository, links pinned to the commit (unchanged sentence); in a private repository the files cannot be linked, so an update attaches cleaned copies and posts the comment only after every attachment lands; on request you see what the cleanup rewrote before anything is posted; an automatic update has nobody to show, so `a final leak check runs over the comment and every attachment before anything is uploaded`, with the same rules a private repository's pushes and pull requests are checked against; any hit, or a check that cannot run, posts nothing and the run reports one line; the check does not look for docket ADR numbers, a change number written without a leading zero (`#612`), a bare number, or the words "backlog" and "board", which rely on the cleanup, since it always rewrites docket's change and ADR numbers; the pull request link stays a link in both.
    - The closing sentence of *There is no one right moment to create it* becomes `From then on, the switches keep the ticket current without you.`
    - *The invariants*: add `- In a private repository, an automatic update is posted only when the final leak check finds nothing in the comment or any attachment.`
    - *Decided in*: add `- [ADR-0144](../adrs/0144-private-visibility-leak-check-blocks-outgoing-pushes-and-pr.md) — refused any push or pull request from a private repository that carries docket's text, with the patterns the final leak check reuses.` (The new ADR's line is added by the parent; see *Parent hand-off*.)
  - `docs/reference/config-keys.md`: add the row `` | `leak_check.match_word` | `true` | any layer | whether the bare word "docket" counts as a leak in a private repository's pushes, pull requests, and automatic ticket updates; `false` lets it through, while markers, trailers, `.docket` paths, `dckt`, and change ids are still caught | `` after the `tickets.auto.results_completed` row. In the **`tickets`** paragraph replace the sentence that says an automatic update in a private repository posts nothing (it is hard-wrapped across two lines; search whitespace-collapsed) with `In a private repository an automatic update attaches cleaned copies of its files and is posted only when the leak check finds nothing in the comment or any attachment; otherwise nothing is posted and the run reports one line.`
  - `docs/reference/glossary.md`: in `### docket-update-ticket` replace the last sentence of its first paragraph with `An automatic update there is posted only when the leak check finds nothing in the comment or any attachment.` Add after `### Ticket context`:

    ````markdown
    ### Leak check

    The read-only check `docket-update-ticket` runs before an automatic update in a private
    repository. It scans each file it is given, and each file's name, with the rules a private
    repository's pushes and pull requests are checked against, and reports every hit with its file,
    line, matched text, and rule. A file it cannot read is refused, never reported clean.

    ```sh
    docket leak check --file comment.md --file spec.md --json
    ```
    ````

    and `- [Leak check](#leak-check)` in the index in alphabetical position.
  - `docs/reference/skills-and-agents.md`: the `docket-update-ticket` line ends `…; in a private repository it attaches cleaned copies of the spec and results instead, and an automatic update there is posted only when a final leak check finds nothing in it.`
  - `docs/reference/cli.md` *Nouns*: add, before `docket learning`, `- **\`docket leak\`** — check files bound for outsiders against the leak check's rules (read-only). Verbs: \`docket leak --help\`.`
  - `.docket.example.yml` `match_word` comment: true (default) refuses a push or pull request whose commits, added lines, added paths, or PR text contain the bare word "docket", and `an automatic ticket update whose comment or attachments contain it is not posted`; false lets the bare word through, with markers, trailers, `.docket` paths, `dckt`, and change ids still caught; a shared repository runs neither check. Then `go generate ./internal/assets/`.

- [ ] **Step 4: Run to verify they pass**

  Run: `go test -count=1 ./internal/repoguard ./internal/config ./internal/assets`
  Expected: PASS (includes `TestAutoTicketUpdateContracts`, the prose contracts, the example-config correspondence, the anchor-style guard, and the embedded mirror).

- [ ] **Step 5: Derive the leftover sites**

  Run: `git grep -n -i -z -e 'posts nothing' -e 'not available in a private' -e 'in a shared repository, the' -- docs .docket.example.yml skills ':!docs/superpowers' ':!docs/results' ':!docs/adrs' | tr '\0' ' '` and read each hit (the `-z` form keeps the hit greppable when the phrase wraps; also eyeball `docs/reference/config-keys.md` and `docs/concepts/tickets.md` for the wrapped form).
  Expected: no living doc or skill still says an automatic update in a private repository posts nothing.

- [ ] **Step 6: Mutation-test each new doc row** as in Task 3 Step 9.

- [ ] **Step 7: Commit**

  ```bash
  git add docs/concepts/tickets.md docs/reference/config-keys.md docs/reference/glossary.md docs/reference/skills-and-agents.md docs/reference/cli.md .docket.example.yml internal/assets/embedded/tree/.docket.example.yml internal/repoguard/ticket_auto_updates_contracts_test.go
  git commit -m "docs(tickets): describe automatic private updates and the leak check"
  ```

### Build gate

The whole suite runs once at the end through `docket-build`'s gate (`build.test_command`, read from config). Read the budget report even on a green run (AGENTS.md *Guards and tests*). Confirm `git grep -n 'tickets' .docket.yml` shows no `tickets:` key.

## Parent hand-off (not build tasks)

- **ADR (D6).** After the build, the parent records the ADR with `docket-adr`: an unattended private ticket update skips human review of the cleanup and is posted only when the final leak check, with exactly the publish leak check's patterns and options, finds nothing in the comment and every attachment; any hit, word-check hit, or failed check posts nothing and reports one line, with no fix-and-recheck; what the patterns deliberately miss relies on the cleanup alone; unknown visibility posts nothing. Rejected alternatives: a review step, an agent-run pattern search, fixing a flagged line and checking again, a new `docket-export` mode. It states that `leak.check` scans in any visibility while the publish operations still check private repositories only, so ADR-0144's "shared repositories never invoke the scanner" holds for publishes. Relates to ADR-0144, ADR-0163, ADR-0162, ADR-0044.
- **Its *Decided in* line.** Once the ADR has its number, add one line for it to `docs/concepts/tickets.md` *Decided in* (above the ADR-0144 line) on the feature branch, then re-mint the build evidence as Step 6 requires after any fix commit.
- **Results.** The results list the **Important** live check from the spec's acceptance criteria, done by a person from a private scratch repository (its `tickets.*` keys in `.git/dckt/config.yml`) against a sandbox ticket project, never from this repository: the automatic PR-opened comment arrives with cleaned spec and results attached and no docket text; `docket leak check --file` on an exported copy with a planted `#0NNN` for a real change id reports the hit; and what the dispatched implement-next does at the post step (the comment, or the "no connector" line). They also record the catalog-ceiling raise of Task 2 as a design event.
