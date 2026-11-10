<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0585 — Post a ticket update on request (shared repositories)](../../changes/active/0585-post-a-ticket-update-on-request-shared-repositories.md)**
<!-- docket:backlink:end -->

# Post a ticket update on request (shared repositories) Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` (one tier worker per task under the `docket-build-task` contract, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the read-only `context.ticket` operation and the inline `docket-update-ticket` skill that posts a short, commit-pinned update to a change's linked ticket on request (shared repositories only), and fix change 584's two leftovers.

**Architecture:** `context.ticket` is modelled on `context.implementation` (`ContextImplementation` in `internal/app/implementation_context.go`): one `PinContext`, one `ReadCorpus`, one `repository.BuildSnapshot`, no writes, one protocol-v1 envelope. Pinned links come from a new `render.LinkContext.BlobURLAtCommit`, reached only through `linkContextOf`. The skill is a harness-neutral inline skill (like `skills/docket-link-ticket/`) with one progressive-disclosure reference, `references/shared.md`; every docket call in it is resolved from the capability catalog.

**Tech Stack:** Go (cobra CLI, `internal/app`, `internal/render`), Markdown skills, repository guards in `internal/repoguard`, embedded asset twins regenerated with `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-11-10-post-a-ticket-update-on-request-shared-repositories-design.md` (on the `docket` branch; a copy is committed on the feature branch). Read it before any task; it fixes the editorial contract, the per-event table, the mention rules, and the result shape.

## Global Constraints

- No ticket-system call from this repository in any test, build, review, or results run. Docket's own `.docket.yml` gets no `tickets.*` key.
- `context.ticket` is read-only: catalog effect `read`; no transaction, no write, no GitHub call.
- Event names are exactly `spec_written`, `pr_opened`, `pr_merged`, `results_completed` (the `tickets.auto.<event>` tokens). Not-happened reasons are exactly `no-spec`, `no-pr`, `merge-not-recorded`, `not-results-artifact`, `not-completed`. Ticket states are exactly `linked`, `not-linked`, `partial`.
- Artifacts are only `spec` and `results`; a plan never appears in `artifacts`, in a link, or in an update.
- Pinned URL form: `RepoWebURL + "/blob/" + commit + "/" + repoRelPath`, using the full `metadata_commit`; empty when `RepoWebURL` is empty, the commit is empty, or `PrivateMetadata` is set.
- `render.LinkContext` is built only by `linkContextOf` (`TestLinkContextSoleConstructor`).
- Skill bodies ship into other repositories (learning `distributed-body-has-no-local-repo`): no sentence about "this repo"; the only hard-coded docket spelling allowed is `docket capabilities --json` (`internal/repoguard/capability_surface_test.go`). Name operations by id (`context.ticket`, `status`) with "argv from the capability catalog".
- Skill prose house style: one paragraph or bullet per physical line, so every prose-contract phrase sits on one line (`scanProse` is a raw `strings.Contains`; learning `phrase-grep-over-wrapped-prose`).
- The skill's description uses the ticket vocabulary only: never "issue" or "work item".
- Cross-references in maintained source anchor on symbols, never line numbers (ADR-0054).
- After any edit under `skills/` run `go generate ./internal/assets/` and commit the regenerated `internal/assets/embedded/tree/` twins in the same commit.
- Budget rows in `internal/repoguard/budgets_test.go` are pinned at the exact measured `wc -l` / `wc -w` counts, with a leading `0585: <what> (old -> new)` note; new files get `0585: new — <what>`.
- The results file (written later by implement-next) must list, under `## Human actions and testing`, an **Important** item: from another repository linked to a sandbox ticket project, ask for each of the four event updates and one no-event update, and confirm the comment, its pinned links, the refusals, and that a mention of a fully named person notifies them; done after the PR opens and before merging; no `hold: true`.

## Review Focus

1. **A record names a spec that is missing at the metadata commit.** `spec_written` still reports `happened: true` (the rule is "the change has a `spec:`"), and `artifacts` omits the spec, so the update goes without that link instead of failing. Pinned by `TestContextTicketMissingArtifactOmitted` (Task 2).
2. **An archived `done` change read by id.** It must resolve exactly like an active one, and `change.path` must be the archive path the skill then reads. Pinned by `TestContextTicketArchivedChange` (Task 2).
3. **A `stacked-merged` change whose parent record is not in the corpus.** The read must still report `pr_merged` happened with `merge.in_integration: false` and an empty `destination_branch`, not panic or refuse. Pinned by `TestContextTicketStackedMergedParentMissing` (Task 2).
4. **A near-miss event spelling** (`PR_OPENED`, `pr-opened`, ` pr_opened`). The set is closed and exact: `invalid-input` / `unknown-event`, checked before any repository read. Pinned by `TestContextTicketUnknownEventRefusedBeforeRead` (Task 2) and `TestContextTicketCLIRejectsUnknownEvent` (Task 3).
5. **Reason and event tokens drifting between the Go producer and the skill.** A renamed reason in Go would leave the skill telling the person nothing useful. Pinned by `TestUpdateTicketSkillNamesEveryReadToken` (Task 4), which is keyed on the producer's own lists.

---

### Task 1: Commit-pinned blob URL builder

**Files:**
- Modify: `internal/render/link.go` (add a method beside `BlobURLOnBranch`)
- Test: `internal/render/link_test.go`

**Interfaces:**
- Produces: `func (l LinkContext) BlobURLAtCommit(repoRelPath, commit string) string`

- [ ] **Step 1: Write the failing test** `TestBlobURLAtCommit` in `internal/render/link_test.go`, table-driven:

```go
const commit = "3333333333333333333333333333333333333333"
const p = "docs/superpowers/specs/2026-11-08-x-design.md"
shared := render.LinkContext{RepoWebURL: "https://github.com/o/r", MetadataBranch: "docket", IntegrationBranch: "main"}
// shared, full commit → "https://github.com/o/r/blob/" + commit + "/" + p
// RepoWebURL "" → ""
// commit "" → ""   (never "/blob//")
// PrivateMetadata: true → ""
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/render -run TestBlobURLAtCommit -count=1`
Expected: FAIL (`l.BlobURLAtCommit undefined`).

- [ ] **Step 3: Implement `BlobURLAtCommit`** in `internal/render/link.go`, with a doc comment saying it pins a link to one commit and that a private repository's metadata commit has no web page. Return `""` when `RepoWebURL == ""`, `commit == ""`, or `PrivateMetadata`; otherwise `RepoWebURL + "/blob/" + commit + "/" + repoRelPath`.

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/render -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/link.go internal/render/link_test.go
git commit -m "feat(render): add a blob URL pinned to a commit"
```

---

### Task 2: The `context.ticket` read operation

**Files:**
- Create: `internal/app/ticket_context.go`
- Create: `internal/app/ticket_context_test.go`
- Modify: `internal/app/link_context_guard_test.go` (`TestLinkContextSoleConstructor` floor `16` → `17`, as its own comment directs)

**Interfaces:**
- Consumes: `render.LinkContext.BlobURLAtCommit` (Task 1); `linkContextOf(pin StatusPin) render.LinkContext`; `metadataRevision(pin)`; `parseCorpus`; `repository.BuildSnapshot`; `classifyStatusError`; `deliverableText`; `domain.Change.Ticket()`, `HasTicketFields()`, `Spec()`, `Results()`, `PR()`; `domain.StackParent`; `StatusReader.ArtifactExists(ctx, pin, sourceMetadata, path)`.
- Produces (exact names; Task 3 and Task 4 use them):

```go
const OperationContextTicket = "context.ticket"

// Event tokens (also the tickets.auto.<event> keys).
const (
	TicketEventSpecWritten      = "spec_written"
	TicketEventPROpened         = "pr_opened"
	TicketEventPRMerged         = "pr_merged"
	TicketEventResultsCompleted = "results_completed"
)
// Not-happened reasons.
const (
	ReasonTicketNoSpec             = "no-spec"
	ReasonTicketNoPR               = "no-pr"
	ReasonTicketMergeNotRecorded   = "merge-not-recorded"
	ReasonTicketNotResultsArtifact = "not-results-artifact"
	ReasonTicketNotCompleted       = "not-completed"
)
// Ticket states.
const (
	TicketStateLinked    = "linked"
	TicketStateNotLinked = "not-linked"
	TicketStatePartial   = "partial"
)
// Refusal reasons.
const (
	ReasonTicketMissingID    = "missing-id"    // --id absent or <= 0 → ResultInvalidInput
	ReasonTicketUnknownEvent = "unknown-event" // → ResultInvalidInput
	ReasonTicketNotFound     = "not-found"     // no record with that id → ResultInvalidInput
	// an id claimed by more than one record reuses ReasonContextAmbiguousID → ResultInvalidState
)

// Ordered token lists (the skill guard in Task 4 iterates these).
var ticketEvents = []string{TicketEventSpecWritten, TicketEventPROpened, TicketEventPRMerged, TicketEventResultsCompleted}
var ticketEventReasons = []string{ReasonTicketNoSpec, ReasonTicketNoPR, ReasonTicketMergeNotRecorded, ReasonTicketNotResultsArtifact, ReasonTicketNotCompleted}
var ticketStates = []string{TicketStateLinked, TicketStateNotLinked, TicketStatePartial}

type TicketContextRequest struct {
	ID    int    `json:"id"`
	Event string `json:"event,omitempty"` // "" = no-event read
}

type TicketContextChange struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Status      string `json:"status"`      // RawStatus()
	Deliverable string `json:"deliverable"` // deliverableText(c)
	Path        string `json:"path"`        // record's repo-relative path on the metadata branch
}

type TicketContextTicket struct {
	State    string `json:"state"`
	Provider string `json:"provider,omitempty"` // set only when linked
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
}

type TicketContextEvent struct {
	Name     string `json:"name"`
	Switch   bool   `json:"switch"`   // effective tickets.auto.<event>; ignored on request
	Happened bool   `json:"happened"`
	Reason   string `json:"reason,omitempty"`
}

type TicketContextMerge struct {
	DestinationBranch string `json:"destination_branch"`
	InIntegration     bool   `json:"in_integration"`
}

type TicketContextFacts struct {
	PRURL string              `json:"pr_url"` // the record's pr: verbatim, "" when none
	Merge *TicketContextMerge `json:"merge,omitempty"`
}

type TicketContextArtifact struct {
	Role string `json:"role"` // "spec" | "results"
	Path string `json:"path"`
	URL  string `json:"url"` // "" in a private repository or with no web URL
}

type TicketContext struct {
	MetadataCommit string                  `json:"metadata_commit"`
	Visibility     string                  `json:"visibility"` // string(pin.Layout.Mode)
	Change         TicketContextChange     `json:"change"`
	Ticket         TicketContextTicket     `json:"ticket"`
	Event          *TicketContextEvent     `json:"event,omitempty"`
	Facts          TicketContextFacts      `json:"facts"`
	Artifacts      []TicketContextArtifact `json:"artifacts"` // never null
}

type TicketContextResult struct {
	Envelope
	Context *TicketContext `json:"context,omitempty"`
	Reason  string         `json:"reason,omitempty"`
	Message string         `json:"message,omitempty"`
}

func (r TicketContextResult) HumanText() string
func ContextTicketRead(ctx context.Context, deps PlanningDeps, repoDir string, req TicketContextRequest) TicketContextResult
```

(`ContextTicket` is already the name of the ticket struct in `implementation_context.go`, so the function is `ContextTicketRead`.)

**Rules the implementation must follow** (spec, *The read operation*):

- Validate the request before any read: `req.ID <= 0` → `missing-id`; `req.Event` not exactly one of `ticketEvents` (and not `""`) → `unknown-event`. Neither calls `PinContext`.
- `snap.Change(id)`: `LookupAbsent` → `invalid-input` / `not-found`; `LookupAmbiguous` → `invalid-state` / `ambiguous-change`. Any status, active or archived, is served (no readiness or claim check).
- Ticket: `c.Ticket()` ok → `linked` with provider/id/url; else `c.HasTicketFields()` → `partial`; else `not-linked`.
- Happened: `spec_written` ⇔ `spec:` present and non-empty, else `no-spec`. `pr_opened` ⇔ `pr:` present and non-empty, else `no-pr`. `pr_merged` ⇔ (status `done` and deliverable `pr` and `pr:` non-empty) or status `stacked-merged`; otherwise `no-pr` when `pr:` is empty, else `merge-not-recorded`. `results_completed`: deliverable not `results-artifact` → `not-results-artifact`; else status not `done` → `not-completed`; else happened. (A killed change therefore never has `pr_merged` or `results_completed`.)
- Switch: the matching `pin.Config.Effective.Tickets.Auto.<Field>.Value` (`SpecWritten`, `PROpened`, `PRMerged`, `ResultsCompleted`).
- `facts.merge` is set whenever the change has merged, with or without `--event`: `done` + deliverable `pr` + `pr:` → destination = `pin.IntegrationBranch` (falling back to `pin.DefaultBranch`, as `linkContextOf` does), `in_integration: true`; `stacked-merged` → destination = the stack parent's recorded `branch:` value via `domain.StackParent` (`""` when the parent does not resolve), `in_integration: false`.
- Artifacts, in order `spec` then `results`: for each field present and non-empty, `deps.Reader.ArtifactExists(ctx, pin, sourceMetadata, path)`; an error returns `classifyStatusError`'s result; not found → left out; found → `{role, path, linkContextOf(pin).BlobURLAtCommit(path, metadataRevision(pin))}`.
- `HumanText` names identity, revision, ticket state, and the event verdict only, e.g. `context.ticket: change 0585 <slug> on <metadata-ref>@<shortCommit> — ticket linked; event pr_opened happened`; never a title, URL, or authored body. On a refusal: `"%s: %s (%s)"` operation/result/reason, like `ImplementationContextResult.HumanText`.
- A file header comment in the style of `implementation_context.go` saying what the operation is for and that "not linked", "partial", "not happened", and "private" are reported, not refused.

Test fixture helper (in `ticket_context_test.go`):

```go
// ticketRecordBlob builds an active change record with the given status and extra frontmatter lines.
func ticketRecordBlob(id int, slug, status, extra string) StatusBlob
```

Use `docketPin(t)` with `pin.RepoWebURL = "https://github.com/acme/widgets"`, and `contextDeps(fake)`. `pin.MetadataRevision` is `"3333333333333333333333333333333333333333"`; build expected URLs from it.

- [ ] **Step 1: Write the failing tests** in `internal/app/ticket_context_test.go`:

  - `TestContextTicketEventsHappened`: one subtest per event with a record where it happened: `spec_written` (spec set), `pr_opened` (pr set, status `implemented`), `pr_merged` (status `done`, pr set), `results_completed` (`deliverable: results-artifact`, status `done`, results set). Assert `Result == ResultApplied`, `Event.Name`, `Event.Happened == true`, `Event.Reason == ""`.
  - `TestContextTicketEventsNotHappened`: table → expected reason: `spec_written` without spec → `no-spec`; `pr_opened` without pr → `no-pr`; `pr_merged` on `implemented` with pr → `merge-not-recorded`; `pr_merged` on `proposed` without pr → `no-pr`; `results_completed` on a deliverable-`pr` change → `not-results-artifact`; `results_completed` on an `in-progress` results-artifact change → `not-completed`. Each: `Happened == false`.
  - `TestContextTicketKilledChange`: status `killed` with pr and spec: `pr_merged` → happened false; `results_completed` (results-artifact killed) → happened false; `spec_written` → happened true.
  - `TestContextTicketMergeFacts`: `done` → `Merge == &{DestinationBranch: "main", InIntegration: true}`; `stacked-merged` child of parent 20 with `branch: 'feat/parent'` → `&{"feat/parent", false}`; `implemented` → `Merge == nil`. Assert the same with no `--event`.
  - `TestContextTicketStackedMergedParentMissing`: `stacked-merged` with `stacked_on: 99` and no record 99 → applied, `pr_merged` happened, `Merge.InIntegration == false`, `Merge.DestinationBranch == ""`.
  - `TestContextTicketTicketStates`: `linked` (all three fields valid → provider/id/url exact), `partial` (provider + id only → state `partial`, provider/id/url empty), `partial` (invalid url `javascript:x`), `not-linked` (no fields). Also assert the JSON of a not-linked ticket is exactly `{"state":"not-linked"}`.
  - `TestContextTicketSharedPinnedURLs`: spec and results present at the metadata source (`fake.artifacts[sourceMetadata+"|"+path] = true`) → `Artifacts` is exactly `[{spec, specPath, "https://github.com/acme/widgets/blob/3333…3333/"+specPath}, {results, resultsPath, …}]`; `MetadataCommit == pin.MetadataRevision`; `Visibility == "shared"`.
  - `TestContextTicketPrivateEmptyURLs`: `pin.Layout = layout.PrivateLayout("/c", "/r", "/d", "o-r")` → `Visibility == "private"`, both artifacts present with `URL == ""`.
  - `TestContextTicketNeverReportsPlan`: record with `plan:`, `spec:`, `results:` all present at the metadata source → no artifact has role `plan` or the plan path, and `fake.artifactAsks` never contains the plan path.
  - `TestContextTicketMissingArtifactOmitted`: `spec:` set but not present at the metadata source → `spec_written` happened true, `Artifacts` empty (`[]`, and the JSON says `"artifacts":[]`).
  - `TestContextTicketArchivedChange`: a `done` record at `LocationArchive`, path `docs/changes/archive/2026-08-16-0042-old.md` → applied, `Change.Path` equals that path, `Change.Status == "done"`.
  - `TestContextTicketNoEventRead`: no `Event` → `Context.Event == nil` and the JSON has no `"event"` key; ticket, facts, and artifacts still reported.
  - `TestContextTicketSwitchReported`: `pin.Config.Effective.Tickets.Auto.PROpened.Value = true` → `pr_opened` read reports `Switch == true`; `spec_written` (left default) reports `false`.
  - `TestContextTicketRefusals`: `ID: 0` → `invalid-input`/`missing-id`; `ID: 404` absent → `invalid-input`/`not-found`; two records claiming id 7 → `invalid-state`/`ambiguous-change`. Each has `Context == nil`.
  - `TestContextTicketUnknownEventRefusedBeforeRead`: events `"PR_OPENED"`, `"pr-opened"`, `" pr_opened"`, `"change_created"` → `invalid-input`/`unknown-event`, and `fake.pinCount == 0`.
  - `TestContextTicketNoNullAndNoBody`: marshal an applied result; it contains no `null`; `HumanText()` does not contain the record's title or body text (`"Body of"`), nor the ticket URL.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/app -run 'TestContextTicket' -count=1`
Expected: FAIL to compile (`ContextTicketRead undefined`).

- [ ] **Step 3: Implement `internal/app/ticket_context.go`** per the Interfaces and Rules above.

- [ ] **Step 4: Raise `TestLinkContextSoleConstructor`'s `floor`** from 16 to 17 (the new call site), as its comment directs. The schema binding is added in Task 3, together with the command, so the binding/catalog correspondence tests never see one without the other.

- [ ] **Step 5: Run the package to verify it passes**

Run: `go test ./internal/app ./internal/cli -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/ticket_context.go internal/app/ticket_context_test.go internal/app/link_context_guard_test.go
git commit -m "feat(app): add the context.ticket read operation"
```

---

### Task 3: `docket context ticket` command

**Files:**
- Modify: `internal/cli/context.go` (new `ticket` subcommand; group `Short`)
- Modify: `internal/cli/install.go` (`assetIndependent`: add `"context ticket": true` after `"context finalize"`)
- Modify: `internal/cli/context_test.go`
- Modify: `internal/app/schema_registry.go` (`operationBindings`, kept sorted by id)
- Modify: `docs/reference/cli.md` (the `docket context` bullet)

**Interfaces:**
- Consumes: `app.ContextTicketRead`, `app.TicketContextRequest{ID, Event}` (Task 2); `newPlanningDeps()`; `resolveRepoDir(c)`; `capability(id, EffectRead)`.
- Produces: catalog entry `context.ticket`, effect `read`, flags `--id <id>`, `--event <event>`, `--repo-dir <dir>`.

- [ ] **Step 1: Write the failing tests** in `internal/cli/context_test.go`:
  - `TestContextTicketRegistered`: `root.Find([]string{"context", "ticket"})` resolves; the command has `--id`, `--event`, `--repo-dir` flags; `cmd.Annotations` carry id `context.ticket` and effect `read` (read them the way the other capability tests do).
  - `TestContextTicketCLIRejectsUnknownEvent`: `runCLI(t, "context", "ticket", "--id", "7", "--event", "pr-opened", "--repo-dir", testsupport.TempDir(t), "--json")` → output contains `"operation":"context.ticket"`, `"result":"invalid-input"`, and `"reason":"unknown-event"`.
  - `TestContextTicketCLIEnvelope`: same with `--event pr_opened` on the empty temp dir → output contains `"operation":"context.ticket"` (a typed refusal from the non-repository, not a cobra error).
  - Add `"context ticket"` to the key list in the existing asset-independence loop (the loop over `"context", "context implementation", "context finalize"`).

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/cli -run 'TestContext' -count=1`
Expected: FAIL (`context ticket not registered`).

- [ ] **Step 3: Implement the subcommand** in `newContextCommand`, mirroring `implementation`: `Use: "ticket"`, `Short: "Assemble what a ticket update needs for one change (read-only)"`, `Args: cobra.NoArgs`, `Annotations: capability("context.ticket", EffectRead)`, flags `--repo-dir` (string), `--id` (int, "the change `id` to read"), `--event` (string, "the event the update follows: spec_written, pr_opened, pr_merged, or results_completed; omit for an update with no event"). `RunE` passes `app.TicketContextRequest{ID: id, Event: event}` to `app.ContextTicketRead`. Add it to `contextCmd.AddCommand(implementation, finalize, ticket)`. Change the group's `Short` to `"Assemble read-only context bundles for docket workflows"`. Add `"context ticket": true` to `assetIndependent`. Register the binding in `operationBindings`, right after `context.implementation`:

```go
{ID: "context.ticket", Request: nil, Result: TicketContextResult{}}, // ContextTicketRead
```

If a schema or retired-vocabulary test objects to a field of the new result, fix the field, never the guard.

- [ ] **Step 4: Update `docs/reference/cli.md`**: the `docket context` bullet reads "assemble read-only context bundles for docket workflows (implementation, finalize, ticket updates). Verbs: `docket context --help`."

- [ ] **Step 5: Run the CLI and app packages to verify they pass**

Run: `go test ./internal/cli ./internal/app -count=1`
Expected: PASS, including the binding-correspondence and published-request tests, which now see `context.ticket` on both surfaces.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/context.go internal/cli/install.go internal/cli/context_test.go internal/app/schema_registry.go docs/reference/cli.md
git commit -m "feat(cli): add the context ticket command"
```

---

### Task 4: The `docket-update-ticket` skill

**Files:**
- Create: `skills/docket-update-ticket/SKILL.md`
- Create: `skills/docket-update-ticket/references/shared.md`
- Create: `internal/repoguard/update_ticket_skill_test.go`
- Create: `internal/app/ticket_context_skill_test.go`
- Modify: `internal/repoguard/prose_contracts_test.go` (new rows, sentinel `change_0585_update_ticket_skill`)
- Modify: `internal/repoguard/budgets_test.go` (two new rows, appended after the `docket-status/SKILL.md` row)
- Modify: `internal/harness/inventory_test.go` (known-skill list gains `"docket-update-ticket"`)
- Regenerate: `internal/assets/embedded/tree/skills/docket-update-ticket/**`

**Interfaces:**
- Consumes: `ticketEvents`, `ticketEventReasons`, `ticketStates` (Task 2); `assertInlineSkill`, `collapseWS`, `frontmatterValue`, `readRepoFile`, `guardRoot` (`internal/repoguard`).
- Produces: the skill files whose exact phrases Task 5's cross-references and the prose rows pin.

The skill text is the deliverable and the spec fixes most of it, so it is given here. Keep it this tight; adjust wording only where a guard requires it, and keep every pinned phrase on one line.

- [ ] **Step 1: Write the failing guards.**

`internal/repoguard/update_ticket_skill_test.go`:

```go
// TestUpdateTicketSkillIsInline pins docket-update-ticket as an inline skill whose
// description names its ask in conversational words, disclaims creating or
// linking, and uses the ticket vocabulary only; and checks that the reference
// SKILL.md routes to exists.
func TestUpdateTicketSkillIsInline(t *testing.T) {
	const name = "docket-update-ticket"
	assertInlineSkill(t, name, []string{
		"post an update or comment about a change to its ticket",
		"not for creating or linking a ticket",
	})
	// description must not contain "issue" or "work item" (lower-cased, collapsed)
	// skills/docket-update-ticket/references/shared.md must exist (os.Stat)
}
```

`internal/app/ticket_context_skill_test.go` (producer-anchored; reads `../../skills/docket-update-ticket/SKILL.md`):

```go
// TestUpdateTicketSkillNamesEveryReadToken: every event, not-happened reason, and
// ticket state context.ticket can report appears in the skill in backticks, so a
// token renamed in Go reddens here instead of leaving the skill unable to act on it.
func TestUpdateTicketSkillNamesEveryReadToken(t *testing.T) {
	// for each tok in ticketEvents, ticketEventReasons, ticketStates:
	//   strings.Contains(body, "`"+tok+"`") or t.Errorf naming tok
	// non-vacuity: the three lists together hold 12 tokens (len check)
}
```

Prose-contract rows in `proseContracts` (append after the `change_0584_link_ticket_skill` rows):

```go
// change 0585 — the inline update skill: on request only; refusals; the
// per-event links; plans never go out; layout for a person in the ticket
// system; real mentions only; posted only on confirmation; shared delivery
// in its own reference.
{sentinel: "change_0585_update_ticket_skill", file: "skills/docket-update-ticket/SKILL.md",
	present: []string{
		"This skill runs inline, in your own session, and only when the person asks",
		"run the read without `--event`",
		"`not-linked`: say the change has no ticket (\"#612 has no ticket\")",
		"`partial`: the link is broken; point the person to `docket-link-ticket` to fix or remove it",
		"The request names a different ticket than `ticket.id`: say which ticket the change carries",
		"never say the PR has not merged",
		"`visibility` is `private`: say private repositories are not supported yet",
		"Plans never go out: no link to a plan and nothing from one.",
		"A late update never misstates the current state",
		"| the PR, the spec, and the results file |",
		"never a claim of deployment or acceptance",
		"Write for a person reading the comment in the ticket system.",
		"Use the format the connector accepts, so the layout survives posting.",
		"Label links (`Spec:`, `Results:`).",
		"**Exactly one match:** mention that person and post",
		"**Several matches:** ask the person which one",
		"**No match:** ask the person whether to post without that mention or try a different spelling",
		"Never substitute plain text such as \"@Jane\", which notifies nobody.",
		"Report the update as posted only with the connector's confirmation",
		"Never run a retry loop.",
		"When `visibility` is `shared`, **read `references/shared.md` now (blocking)**.",
	},
	absent: []string{"context: fork", "agent: docket-update-ticket"}},
{sentinel: "change_0585_update_ticket_skill", file: "skills/docket-update-ticket/references/shared.md",
	present: []string{
		"Each `url` is pinned to `metadata_commit`",
		"Never build a link yourself",
		"Links live in the comment only",
		"never substitute a local path",
	}},
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/repoguard -run 'TestUpdateTicketSkillIsInline|TestProseContracts' -count=1 && go test ./internal/app -run TestUpdateTicketSkillNamesEveryReadToken -count=1`
Expected: FAIL (files missing).

- [ ] **Step 3: Write `skills/docket-update-ticket/SKILL.md`** with exactly this content (the examples are copied from the spec's decision 11 and must stay laid out as shown):

````markdown
---
name: docket-update-ticket
description: 'Use when the person asks to post an update or comment about a change to its ticket (`update CLOUD-123 now that the PR is open`, `tell the ticket the spec for #12 is written`, `let PROJ-9 know #40 was killed`); not for creating or linking a ticket.'
---

# docket-update-ticket

This skill runs inline, in your own session, and only when the person asks: it posts one short comment to a change's linked ticket with your own tools. Nothing here runs on its own after an event.

Words used here: a **ticket** is the item in the team's ticket system; the **provider** is which ticket system it is (`jira`, `linear`, …); a **connector** is the session's own tools for that system (an MCP server or plugin the person installed and signed in to); an **update** is the comment you post, with its links. Docket installs no connector and stores no credential.

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *startup check*. Everything below uses its vocabulary.

This skill makes no docket write. Its one read is the `context.ticket` operation (argv from the capability catalog), run with `--id <id>`, `--event <event>` when the update follows an event, and `--json`. Its result is under `context`: `visibility`, `change` (`status`, `deliverable`, `path`), `ticket` (`state`: `linked`, `not-linked`, or `partial`; and `provider`, `id`, `url` when `linked`), `event` (`happened`, and `reason` when not), `facts` (`pr_url`, `merge`), and `artifacts` (`role`, `path`, `url`).

## 1. Work out the request

- **The change.** Usually named (`#612`). When the request names only a ticket, find the changes whose record carries that ticket in the `records` array of the `status` operation run with `--json --records`. Exactly one: use it. Several (one ticket may be on several changes): ask the person which. None: say no change carries that ticket.
- **The event.** Map the person's words to one event: spec written (`spec_written`), PR opened (`pr_opened`), PR merged (`pr_merged`), or results completed (`results_completed`). A material revision, a blocker or a decision that is needed, or a killed change has no event: run the read without `--event`.
- **People to mention**, when the person names any (section 5).
- **Creating a ticket** from a change is `docket-link-ticket`'s job; point the person there.

## 2. Refuse when an update is not possible

Post nothing and say why in one plain line when:

- `ticket.state` is `not-linked`: say the change has no ticket ("#612 has no ticket").
- `ticket.state` is `partial`: the link is broken; point the person to `docket-link-ticket` to fix or remove it.
- The request names a different ticket than `ticket.id`: say which ticket the change carries.
- `event.happened` is false: name the missing fact from `event.reason`. `no-spec`: the change has no spec. `no-pr`: it has no PR. `not-results-artifact`: it does not end in a results file. `not-completed`: its results are not complete. `merge-not-recorded`: say docket has not recorded a merge for the change and that, if the PR has merged, refreshing with `docket-status` records it; never say the PR has not merged.
- `visibility` is `private`: say private repositories are not supported yet.

## 3. Write the update

Read what the update needs: each file in `artifacts` at its `path`, and the change record at `change.path` for `## Closeout notes` or `## Why killed`. The record is read, never linked. Plans never go out: no link to a plan and nothing from one.

| Update | It says | Links |
|---|---|---|
| spec written | the approach and key decisions, and where the change stands now from its `status` (designed, not yet built; being built; built, PR open; merged); never a claim of review, approval, or authorship | the spec |
| PR opened | the PR link, what was built, what the reviewer needs to know | the PR, the spec, and the results file |
| PR merged | what merged and where (`facts.merge.destination_branch`; when `in_integration` is false, say it is not yet in the integration branch), key outcomes, and anything left to do, from the results file's Outcome and Known issues and the record's `## Closeout notes`; never a claim of deployment or acceptance | the results file |
| results completed | findings, conclusions, limits, and any action needed | the results file |
| revision, blocker, or kill (no event) | what the person said, backed by the artifacts; for a kill, the record's `## Why killed` | the artifacts it discusses |

A late update never misstates the current state: compare the event with the current `change.status` and say what has happened since (a PR-opened update asked for after the merge says the PR has since merged).

**Editorial standard.** Updates are for people: a few short sentences or bullets, scaled to how much the update matters. Include, when relevant, what happened; what matters (key outcomes, consequential decisions, limitations); what is needed next from the reader (an action, test, review, or decision); and the links. Leave out how you worked, brainstorming notes and work logs, file lists, test inventories, command output, repeated background, filler, empty headings, and pasted artifact contents. Keep every material blocker, risk, and required action, however short the update. Say only what the change and its evidence support: a merge is not a deployment or an acceptance.

**Layout.** Write for a person reading the comment in the ticket system. Lay it out so it scans: short paragraphs, a list, or a single line, whichever reads best for this update. Use the format the connector accepts, so the layout survives posting. Label links (`Spec:`, `Results:`). The examples show tone, length, and layout:

> **@Jane Doe** — PR ready for review: https://github.com/acme/api/pull/48
>
> Invalid requests now fail with a clear error before any work starts, instead of failing partway through. The API contract is unchanged, so existing clients need no update.
>
> Worth a close look: the validation runs before authentication, so a malformed request from an unauthenticated caller now gets a 400 instead of a 401.
>
> Before merging: one manual check against staging, described in the results.
>
> Spec: https://github.com/acme/api/blob/4f2c9e1/docs/superpowers/specs/2026-11-08-validate-requests-design.md
> Results: https://github.com/acme/api/blob/4f2c9e1/docs/results/2026-11-08-validate-requests-results.md

> Merged into main. Invalid requests now return a clear error before processing starts. Deployment has not been confirmed.
>
> Automated checks passed; one manual check remains, described in the results.
>
> Results: https://github.com/acme/api/blob/4f2c9e1/docs/results/2026-11-08-validate-requests-results.md

> Merged into main. Left to do before this can ship:
>
> - run the staging data migration (steps in the results)
> - confirm the new rate limit with the platform team
> - remove the old endpoint once clients have moved (tracked separately)
>
> Results: https://github.com/acme/api/blob/4f2c9e1/docs/results/2026-11-08-validate-requests-results.md

(Real links carry the full commit id.)

## 4. Links

When `visibility` is `shared`, **read `references/shared.md` now (blocking)**.

## 5. Mentions

A mention makes the ticket system notify a person. Mention only people the person names, and only as they ask. For each name, search the connector's users (in Jira, its user lookup, which may be reachable only through its discovery tool).

- **Exactly one match:** mention that person and post; there is no approval step.
- **Several matches:** ask the person which one, listing each candidate's name and email.
- **No match:** ask the person whether to post without that mention or try a different spelling.
- Use the ticket system's own mention form, the one that notifies (in Jira, a mention bound to the person's account id). Never substitute plain text such as "@Jane", which notifies nobody. If the connector cannot make a real mention, tell the person before posting.
- Mentions lead the comment. Nothing about mentions is saved.

## 6. Post and report

- **Find the connector** for `ticket.provider`. Never conclude an operation is missing because it is not in your tool list: some connectors expose operations only through a discovery tool (the Atlassian connector's `discover`), so look there first.
- **Post** the comment to `ticket.id`, making connector calls one at a time. Do not show a draft and wait: the person asked for the update. When they ask to see it first, show it and post on their go.
- **Report.** Report the update as posted only with the connector's confirmation, naming the ticket and anyone mentioned. For a missing connector, a failed post, or an uncertain outcome, say in one plain line what was not posted and why; an uncertain outcome is reported as uncertain, and the person decides whether to ask again. Never run a retry loop. A duplicate comment is deleted by hand.
````

- [ ] **Step 4: Write `skills/docket-update-ticket/references/shared.md`** with exactly this content:

````markdown
# Shared delivery

Loaded from `SKILL.md` when `visibility` is `shared`. The rules in `SKILL.md` apply; they are not repeated here.

- **Where links come from.** The PR link is `facts.pr_url`; the spec and results links are each `artifacts` entry's `url`. Each `url` is pinned to `metadata_commit`, so the reader sees the file as it stood when you read it. Never build a link yourself, never link a branch, and never link the spec copy on the feature branch.
- **Which links.** Only the links the update's row in `SKILL.md` names, labeled (`Spec:`, `Results:`).
- **A missing link.** When the artifact is not in `artifacts` (an older change whose file is not on the metadata branch), or its `url` is empty (the repository has no web address), post the update without that link; never substitute a local path.
- **Links live in the comment only.** Never add a separate entry to the ticket's links panel (in Jira, a remote issue link): nothing could tidy them later.
- **Who can open them.** Readers are assumed to be able to open the repository; nothing checks their access.
````

- [ ] **Step 5: Add the budget rows and the inventory entry.** Measure with `wc -l` and `wc -w` on each new file and append, after the `docket-status/SKILL.md` row in `skillBudgets`:

```go
{"docket-update-ticket/SKILL.md", <lines>, <words>},            // 0585: new — the inline ticket-update skill
{"docket-update-ticket/references/shared.md", <lines>, <words>}, // 0585: new — shared delivery
```

Add `"docket-update-ticket"` to the known-skill list in `TestSkillDirs…` (`internal/harness/inventory_test.go`, the `for _, known := range []string{…}` loop).

- [ ] **Step 6: Regenerate the embedded twins**

Run: `go generate ./internal/assets/`
Expected: new files under `internal/assets/embedded/tree/skills/docket-update-ticket/`.

- [ ] **Step 7: Run the guards and the asset/harness packages to verify they pass**

Run: `go test ./internal/repoguard ./internal/app ./internal/harness ./internal/assets -count=1`
Expected: PASS. Then mutation-check the two new guards (AGENTS.md *Guards and tests*): copy `skills/docket-update-ticket/SKILL.md` aside, delete the `` `merge-not-recorded` `` token and the `"Never run a retry loop."` sentence, confirm `grep -c` shows them gone, re-run with `-count=1` and see `TestUpdateTicketSkillNamesEveryReadToken` and `TestProseContracts` redden, then restore from the copy (never `git checkout --`, which restores HEAD; learning `mutation-restore-needs-a-backup-copy`).

- [ ] **Step 8: Commit**

```bash
git add skills/docket-update-ticket internal/assets/embedded/tree/skills/docket-update-ticket internal/repoguard/update_ticket_skill_test.go internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go internal/app/ticket_context_skill_test.go internal/harness/inventory_test.go
git commit -m "feat(skills): add docket-update-ticket for on-request ticket updates"
```

---

### Task 5: Register the skill everywhere inline skills are listed, and fix change 584's leftovers

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (agent-layer paragraph, the sentence ending "stay inline and run at the session model")
- Modify: `skills/docket-convention/references/agent-layer.md` (the *Fork-exclusion principle* sentence)
- Modify: `skills/docket-export/SKILL.md` (the ticket-routing paragraph and the outside-tool example)
- Modify: `skills/docket-link-ticket/SKILL.md` (*Not this skill's* bullet)
- Modify: `docs/reference/skills-and-agents.md`, `docs/reference/glossary.md`, `docs/install/models-and-effort.md`, `docs/reference/config-keys.md`
- Modify: `internal/repoguard/prose_contracts_test.go` (relocate two existing present-phrases; add rows)
- Modify: `internal/repoguard/budgets_test.go` (re-baseline the four edited skill files)
- Regenerate: `internal/assets/embedded/tree/skills/**`

**Interfaces:**
- Consumes: the skill name `docket-update-ticket` and the `context.ticket` operation (Tasks 2–4).
- Produces: nothing later tasks consume.

Before editing, grep the guards for each sentence you are about to change (learning `restatement-accumulates-its-own-guards`): `grep -rn -F -- "stay inline" internal/repoguard`, `grep -rn -F -- "file export" internal/repoguard`, `grep -rn -F -- "Not this skill" internal/repoguard`. Repoint each dependent assert to the new wording; never keep old text to satisfy a guard.

- [ ] **Step 1: Write the failing guard changes** in `internal/repoguard/prose_contracts_test.go`:
  - In the `change_0578_export_skill` row for `skills/docket-convention/SKILL.md`, replace the present-phrase with ``"(`docket-new-change`, `docket-groom-next`), `docket-export`, `docket-link-ticket`, and `docket-update-ticket` stay inline"``.
  - In the `change_0584_link_ticket_skill` row for `agent-layer.md`, replace the present-phrase with ``"the two interactive skills, `docket-export`, `docket-link-ticket`, and `docket-update-ticket` stay inline"``.
  - Add rows, sentinel `change_0585_update_ticket_skill`:
    - `skills/docket-export/SKILL.md` present ``"Posting an update or comment to a ticket is `docket-update-ticket`'s job."``; absent `"as a comment on PROJ-123"`.
    - `skills/docket-link-ticket/SKILL.md` present ``"- **Not this skill's.** Posting an update or comment to a ticket (`update CLOUD-123 now that the PR is open`) is `docket-update-ticket`'s job."``.
    - `docs/reference/skills-and-agents.md` present `"- **docket-update-ticket** —"`.
    - `docs/reference/glossary.md` present `"### docket-update-ticket"`, `"- [docket-update-ticket](#docket-update-ticket)"`, `"### Ticket context"`, `"- [Ticket context](#ticket-context)"`.
    - `docs/reference/config-keys.md` present ``"| `tickets.provider` | `\"\"` (unset) | repo-only | the ticket system this repository's changes link to (jira, linear, …); used when linking or creating a ticket |"``, the `tickets.site` row ending `"; used when linking a ticket by URL or id |"`, the `tickets.project` row ending `"; used when creating a ticket |"`; absent `"(jira, linear, …); no effect yet |"`.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/repoguard -run TestProseContracts -count=1`
Expected: FAIL listing the missing phrases.

- [ ] **Step 3: Edit the skills.**
  - `skills/docket-convention/SKILL.md`: "…the two **interactive** skills (`docket-new-change`, `docket-groom-next`), `docket-export`, `docket-link-ticket`, and `docket-update-ticket` stay inline and run at the session model."
  - `skills/docket-convention/references/agent-layer.md`: "…the two interactive skills, `docket-export`, `docket-link-ticket`, and `docket-update-ticket` stay inline, and `docket-finalize-change` stays unforked…".
  - `skills/docket-export/SKILL.md`: after the sentence "Creating a ticket from a change, or linking or importing one, is `docket-link-ticket`'s job; it calls this skill only for a file export." (keep it; it is pinned) add "Posting an update or comment to a ticket is `docket-update-ticket`'s job." Rewrite the outside-tool example to `("put the spec for 9 in a message to the design channel")`; the rest of that bullet is unchanged. The description stays free of "ticket"/"jira" (`TestExportSkillIsInline`).
  - `skills/docket-link-ticket/SKILL.md`: the *Not this skill's* bullet becomes the sentence pinned in Step 1.

- [ ] **Step 4: Edit the docs.**
  - `docs/reference/skills-and-agents.md`: after the `docket-status` bullet, in alphabetical order: "- **docket-update-ticket** — post a short update to a change's linked ticket when you ask (spec written, PR opened, PR merged, results completed, or a revision, blocker, or kill), with links pinned to the commit that holds them; shared repositories only for now."
  - `docs/reference/glossary.md`:
    - New `### docket-update-ticket` entry after the `docket-status` entry (match the neighbouring entries' shape): what it does (posts one short comment to the linked ticket on request, through your session's connector, with commit-pinned spec and results links; reports it posted only on the connector's confirmation; in a private repository it posts nothing yet), then "**Used for:** telling people in the ticket system that something happened. It runs inline, because it posts through your session's connector."
    - New `### Ticket context` entry after `### Implementation context`: "The read-only bundle `docket-update-ticket` reads before posting: the change's ticket link, the repository's visibility, whether the event has happened, the facts behind it, and spec and results links pinned to the metadata commit the read took." followed by a `sh` block `docket context ticket --id 412 --event pr_opened --json`.
    - Add `- [docket-update-ticket](#docket-update-ticket)` and `- [Ticket context](#ticket-context)` to the index list in alphabetical position.
    - In the forked-skills entry, "**Used for:** explaining why `docket-new-change`, `docket-groom-next`, `docket-export`, `docket-link-ticket`, `docket-update-ticket`, and `docket-finalize-change` run inline."
    - In *Interactive vs autonomous skills*: "`docket-export`, `docket-link-ticket`, and `docket-update-ticket` also stay inline, because they post with your session's own tools and work with you before posting."
  - `docs/install/models-and-effort.md`: after the sentence ending "posts through your session's connector." near the top, add "`docket-update-ticket` runs inline as well, for the same reason."; in the frontmatter paragraph, extend the list ending "nor `docket-link-ticket`, which works with you and posts through your session's connector" with ", nor `docket-update-ticket`, which posts through it too".
  - `docs/reference/config-keys.md`: the three rows end exactly "used when linking or creating a ticket" (`tickets.provider`), "used when linking a ticket by URL or id" (`tickets.site`), "used when creating a ticket" (`tickets.project`), each replacing "no effect yet". The five `tickets.auto.*` rows keep "no effect yet".

- [ ] **Step 5: Re-baseline budgets and regenerate.** Measure `wc -l`/`wc -w` for `docket-convention/SKILL.md`, `docket-convention/references/agent-layer.md`, `docket-export/SKILL.md`, `docket-link-ticket/SKILL.md`; set each row to the new exact counts with a leading note, e.g. `// 0585: docket-update-ticket joins the inline skills (370/7528 -> 370/<n>); 0584: …`. Then run `go generate ./internal/assets/`.

- [ ] **Step 6: Run the guards to verify they pass**

Run: `go test ./internal/repoguard ./internal/assets ./internal/harness -count=1`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention skills/docket-export skills/docket-link-ticket internal/assets/embedded/tree docs/reference/skills-and-agents.md docs/reference/glossary.md docs/install/models-and-effort.md docs/reference/config-keys.md internal/repoguard/prose_contracts_test.go internal/repoguard/budgets_test.go
git commit -m "docs: list docket-update-ticket with the inline skills and fix the ticket config notes"
```

---

## Final gate (docket-build)

Run the whole suite through the configured `build.test_command` (read from config, never a copy). Read the budget report even on a green run. Grep the whole diff once for a hard-coded `docket context ticket` spelling outside `docs/reference/glossary.md` and Go tests; a skill or agent body that carries it is a defect (`capability_surface_test.go` should already catch it).
