<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0394 — Give Docket skills an authoritative compact CLI capability catalog](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0394-give-docket-skills-an-authoritative-compact-cli-capability-c.md)**
<!-- docket:backlink:end -->
# Authoritative Compact CLI Capability Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a repository-independent, read-only `docket capabilities --json` bootstrap that emits a compact protocol-v1 catalog of every public executable CLI leaf derived from the live Cobra tree, and migrate maintained Docket workflow instructions to fetch that catalog before `repository.prepare` and resolve executable spellings from it instead of hard-coding them.

**Architecture:** Effect and stable-operation-id metadata live as typed Cobra annotations applied at each leaf's registration (never a second command-name map); a walker over the assembled production root validates and projects them into a deterministic, byte-budgeted JSON document. The skills side revises the shared Step-0 preamble in `docket-convention`, migrates agent-executed CLI literals to semantic operation ids, and adds a shape-based repoguard test permitting only the capability bootstrap on maintained workflow surfaces. Embedded assets are regenerated through `cmd/genassets`.

**Tech Stack:** Go 1.x, Cobra/pflag, `internal/app` protocol-v1 envelope, Go test guards in `internal/cli` and `internal/repoguard`, `go generate ./internal/assets`.

**Spec:** `docs/superpowers/specs/2026-09-01-give-docket-skills-an-authoritative-compact-cli-capability-c-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/...` from the primary tree).

## Global Constraints

- The catalog is the ONLY executable Docket CLI spelling maintained operating skills may hard-code: `docket capabilities --json`.
- Effect vocabulary is closed: `read` | `local-write` | `metadata-write` | `external-write` | `process-control`. Multiple values allowed; the catalog operation itself is always `read`.
- Effect + stable-op-id metadata co-located on each leaf's Cobra registration — never a second hand-maintained command-name map. (The existing `assetIndependent` map in `internal/cli/install.go` is a *different* concern — asset gating — and stays; the new `capabilities` key is added to it like any new command.)
- Current 65-leaf tree must serialize to <= 12 KB (12288 bytes). Build evidence records the measured byte count plus an explicitly labeled Fable-token *estimate*; bytes are the gating oracle.
- Deterministic, byte-identical output across repeated invocations of the same binary; commands sorted by stable operation id.
- The bootstrap is repository-, config-, asset-, network-, and write-independent; callable when repo-aware ops would refuse.
- Fail closed at the capability boundary (producer: unclassified leaf, duplicate id, invalid effect, missing argv → construction/test failure; consumer: skill prose refuses unsupported version, malformed envelope, duplicate ids, invalid effects, missing argv, mid-run unknown-command).
- Never rewrite historical records: `docs/changes/`, archived specs/plans/results, Accepted ADR prose keep their spellings. Human-facing CLI docs (README, `docs/`) are preserved.
- No MCP work; no request/result schema discovery (change 0360 owns that).
- Build gate: `go run ./cmd/docket development test` (the whole suite, from this checkout). Mutation probes must defeat Go's test cache with `-count=1`.
- Every guard added here is mutation-tested: strip the guarded thing, watch it redden, restore. Restore mutations from a backup copy of your *uncommitted* work, never `git checkout --` (learning: mutation-restore-needs-a-backup-copy).
- Commits: one per task, message trailer per repo attribution rules in effect for the executing session.

---

## File Structure

| File | Role |
|---|---|
| `internal/cli/capability.go` (create) | Effect type + closed vocabulary, `capability(id, effects...)` annotation helper, `CapabilityEntry`, `collectCapabilities(root)` walker + validation, signature projection |
| `internal/cli/capability_test.go` (create) | Synthetic-tree unit tests: walker inclusion/exclusion, every producer fail-closed case, signature projection rules |
| `internal/cli/capability_production_test.go` (create) | Production-tree correspondence (both directions), population floor, id spelling, effects completeness, determinism, byte budget, independence |
| `internal/cli/capabilities.go` (create) | The `docket capabilities` Cobra command (thin adapter) |
| `internal/app/capabilities.go` (create) | `CapabilitiesResult` document type: envelope + `capability_version` + binary identity + global flags + commands; `HumanText` |
| `internal/app/capabilities_test.go` (create) | Envelope/JSON shape, field-name pinning, HumanText |
| `internal/cli/root.go` (modify) | Register `capabilitiesCmd`; annotate the inline leaves (`version`, `status`, `diagnostic runtime`, `diagnostic config`, `install`, `install check`, `development install`, `development test`) |
| `internal/cli/change.go`, `context.go`, `artifact.go`, `workspace.go`, `evidence.go`, `pr.go`, `run.go`, `learning.go`, `adr.go`, `gate.go`, `finalize.go`, `maintenance.go`, `repository.go` (modify) | Apply `capability(...)` annotations at each leaf registration |
| `internal/cli/install.go` (modify) | Add `"capabilities": true` to `assetIndependent` |
| `skills/docket-convention/SKILL.md` (modify) | Step-0 preamble: capability bootstrap before `repository.prepare`; fail-closed postures |
| `skills/docket-*/SKILL.md`, `skills/*/references/*.md`, `agents/docket-*.md`, `cursor-rules/*` (modify per derived inventory) | Migrate agent-executed CLI literals to semantic operation ids |
| `internal/repoguard/capability_surface_test.go` (create) | Shape-based guard: maintained workflow surfaces carry no hard-coded `docket <argv>` spellings except the bootstrap and the asserted exemption set |
| `internal/assets/embedded/**` (regenerate) | `go generate ./internal/assets` after skill edits |

Task order matters: Go feature first (Tasks 1–5), then skills migration (Tasks 6–8), then guard (Task 9 — it would go red before migration), then regeneration + closeout (Tasks 10–11).

---

### Task 1: Effect vocabulary, annotation helper, and capability walker

**Files:**
- Create: `internal/cli/capability.go`
- Test: `internal/cli/capability_test.go`

**Interfaces:**
- Produces: `type Effect string`; constants `EffectRead`, `EffectLocalWrite`, `EffectMetadataWrite`, `EffectExternalWrite`, `EffectProcessControl`; `func capability(id string, effects ...Effect) map[string]string` (annotation payload for `cobra.Command.Annotations`); `type CapabilityEntry struct { ID string; Argv []string; Signature string; Effects []string }`; `func collectCapabilities(root *cobra.Command) ([]CapabilityEntry, error)`.
- Annotation keys (package constants): `capAnnotationID = "docket.capability.id"`, `capAnnotationEffects = "docket.capability.effects"` (effects space-joined, sorted).

Design rules the walker enforces (each is a fail-closed producer error, wrapped with the command path):

1. Inclusion is **annotation-driven**: a visible command carrying `capAnnotationID` becomes exactly one entry — including a command that also has children (`install` is an executable parent).
2. A visible command with **no** annotation and **no** visible children is an unclassified leaf → error. (The group stubs — `change`, `gate`, `gate drive`, root itself, etc. — have visible children and are recursed, never listed.)
3. Hidden commands are skipped entirely; an annotated hidden command is an error (public catalog only).
4. Effects must be non-empty and each drawn from the closed vocabulary; the id must be non-empty; duplicates across the tree are an error.
5. Argv is `strings.Split(c.CommandPath(), " ")` — always starts with the root name, never empty.
6. Entries are sorted by `ID` (bytewise) before return.

Signature projection (also in this task, exercised on synthetic commands; production pinning happens in Task 4):

- Flags come **only from typed pflag data** on `c.Flags()` (local + inherited-from-parents minus root persistent `--json`, which is document-global). Skip hidden flags. For each flag:
  - required iff `flag.Annotations[cobra.BashCompOneRequiredFlag]` is non-empty (this is what `MarkFlagRequired` sets — typed metadata, not prose);
  - repeatable iff `flag.Value.Type()` has suffix `Array` or `Slice`;
  - value hint: `name, _ := pflag.UnquoteUsage(flag)` (backquoted word in the usage string, falling back to the value type); bool flags take no value token;
  - token: `--<flag> <hint>` (valued), `--<flag>` (bool), suffix `...` when repeatable; wrap in `[...]` unless required; append `=<default>` inside the brackets only when `flag.DefValue` is non-empty and not `false`/`0`/`[]`.
  - order: required flags first (sorted by name), then optional (sorted by name).
- Positionals come from the **Use tail** (everything after the first token of `Use`), with flag restatements stripped: drop any token starting with `--` (length > 2) plus its immediately following token when that token starts with `<` or matches `^[a-z|-]+$`; keep a bare `--` separator and everything after it at the end.
- Composition: `<required flags> <optional flags> <positional tail>` when the tail begins with the bare `--` separator (so `gate launch`'s trailing `-- <argv...>` lands last); otherwise `<positional tail> <required flags> <optional flags>`. Join with single spaces; empty parts dropped.

- [ ] **Step 1: Write the failing tests** (`internal/cli/capability_test.go`)

Build synthetic trees with the same shapes production uses. Cover at minimum:

```go
package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "docket"}
	root.PersistentFlags().Bool("json", false, "emit protocol-v1 JSON on stdout")
	return root
}

func leaf(use, id string, effects ...Effect) *cobra.Command {
	return &cobra.Command{Use: use, Annotations: capability(id, effects...),
		RunE: func(*cobra.Command, []string) error { return nil }}
}

func TestCollectIncludesAnnotatedLeavesSorted(t *testing.T) {
	root := newTestRoot()
	grp := &cobra.Command{Use: "grp", RunE: func(*cobra.Command, []string) error { return nil }}
	grp.AddCommand(leaf("beta", "grp.beta", EffectRead), leaf("alpha", "grp.alpha", EffectMetadataWrite))
	root.AddCommand(grp)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].ID != "grp.alpha" || entries[1].ID != "grp.beta" {
		t.Fatalf("entries = %+v", entries)
	}
	if got := strings.Join(entries[1].Argv, " "); got != "docket grp beta" {
		t.Fatalf("argv = %q", got)
	}
}

func TestCollectAnnotatedParentIsAnEntry(t *testing.T) {
	// `install` shape: executable AND has an annotated child.
	root := newTestRoot()
	parent := leaf("install", "install", EffectLocalWrite)
	parent.AddCommand(leaf("check", "install.check", EffectRead))
	root.AddCommand(parent)
	entries, err := collectCapabilities(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %+v, err = %v", entries, err)
	}
}

func TestCollectRejectsUnclassifiedLeaf(t *testing.T) {
	root := newTestRoot()
	root.AddCommand(&cobra.Command{Use: "orphan", RunE: func(*cobra.Command, []string) error { return nil }})
	if _, err := collectCapabilities(root); err == nil || !strings.Contains(err.Error(), "orphan") {
		t.Fatalf("want unclassified-leaf error naming the command, got %v", err)
	}
}

func TestCollectRejectsDuplicateID(t *testing.T) { /* two leaves, same id -> error naming the id */ }
func TestCollectRejectsUnknownEffect(t *testing.T) { /* Annotations built by hand with effect "write" -> error */ }
func TestCollectRejectsEmptyEffects(t *testing.T) { /* capAnnotationEffects: "" -> error */ }
func TestCollectRejectsAnnotatedHidden(t *testing.T) { /* Hidden: true + annotation -> error */ }
func TestCollectSkipsHiddenSubtree(t *testing.T)   { /* hidden group with annotated child -> no entry, no error */ }
```

Signature tests (same file):

```go
func TestSignatureRequiredOptionalRepeatableDefaults(t *testing.T) {
	c := leaf("reconcile", "change.reconcile", EffectMetadataWrite)
	c.Flags().String("request", "", "JSON request `file`, or - for stdin (required)")
	c.Flags().String("repo-dir", "", "repository `dir` to operate on")
	c.Flags().StringArray("type", nil, "filter `type` (repeatable)")
	_ = c.MarkFlagRequired("request")
	root := newTestRoot()
	root.AddCommand(c)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	want := "--request <file> [--repo-dir <dir>] [--type <type>...]"
	if entries[0].Signature != want {
		t.Fatalf("signature = %q, want %q", entries[0].Signature, want)
	}
}

func TestSignaturePositionalTailStripsFlagRestatements(t *testing.T) {
	// gate-verdict shape: "gate-verdict <key> | --unattributed [<id>...]"
	// gate launch shape:  "launch --root <dir> --cwd <dir> -- <argv...>"
	// Pin both projections exactly, per the composition rule above.
}

func TestSignatureExcludesRootJSONFlag(t *testing.T) { /* --json never appears in any entry signature */ }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestCollect|TestSignature' -count=1`
Expected: FAIL — `capability`, `collectCapabilities`, `Effect` undefined.

- [ ] **Step 3: Implement `internal/cli/capability.go`**

```go
package cli

// This file owns docket's capability metadata: the closed effect vocabulary,
// the annotation helper every leaf registration calls, and the walker that
// projects the assembled Cobra tree into catalog entries. Inclusion is
// annotation-driven and fail-closed: a public executable leaf without
// complete metadata is a construction error, never a silent omission.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Effect string

const (
	EffectRead           Effect = "read"
	EffectLocalWrite     Effect = "local-write"
	EffectMetadataWrite  Effect = "metadata-write"
	EffectExternalWrite  Effect = "external-write"
	EffectProcessControl Effect = "process-control"
)

// allEffects is the closed vocabulary; validation derives from it, and the
// catalog documents it. Adding a value here is a protocol event.
var allEffects = map[Effect]bool{
	EffectRead: true, EffectLocalWrite: true, EffectMetadataWrite: true,
	EffectExternalWrite: true, EffectProcessControl: true,
}

const (
	capAnnotationID      = "docket.capability.id"
	capAnnotationEffects = "docket.capability.effects"
)

// capability builds the annotation payload a leaf registration attaches. It
// is the ONLY sanctioned way to declare capability metadata, so the id and
// effects always travel with the registration they describe.
func capability(id string, effects ...Effect) map[string]string {
	parts := make([]string, len(effects))
	for i, e := range effects {
		parts[i] = string(e)
	}
	sort.Strings(parts)
	return map[string]string{
		capAnnotationID:      id,
		capAnnotationEffects: strings.Join(parts, " "),
	}
}

type CapabilityEntry struct {
	ID        string   `json:"id"`
	Argv      []string `json:"argv"`
	Signature string   `json:"signature,omitempty"`
	Effects   []string `json:"effects"`
}

func collectCapabilities(root *cobra.Command) ([]CapabilityEntry, error) {
	var entries []CapabilityEntry
	seen := map[string]string{} // id -> command path
	var walk func(c *cobra.Command) error
	walk = func(c *cobra.Command) error {
		for _, child := range c.Commands() {
			if child.Hidden {
				if _, ok := child.Annotations[capAnnotationID]; ok {
					return fmt.Errorf("capability metadata on hidden command %q", child.CommandPath())
				}
				continue
			}
			id, annotated := child.Annotations[capAnnotationID]
			if annotated {
				entry, err := buildEntry(child, id)
				if err != nil {
					return err
				}
				if prior, dup := seen[id]; dup {
					return fmt.Errorf("duplicate capability id %q on %q and %q", id, prior, child.CommandPath())
				}
				seen[id] = child.CommandPath()
				entries = append(entries, entry)
			} else if !hasVisibleChildren(child) {
				return fmt.Errorf("public executable leaf %q has no capability metadata; register it with capability(id, effects...)", child.CommandPath())
			}
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}
```

`buildEntry` validates id non-empty, parses + validates effects against `allEffects` (non-empty, sorted), sets `Argv` from `CommandPath()`, and calls `buildSignature(c)`. `hasVisibleChildren` iterates `c.Commands()` skipping `Hidden`. `buildSignature` implements the flag/positional rules verbatim from this task's header (walk `c.Flags()` and each ancestor's persistent flags via `c.InheritedFlags()`, minus `json`; use `pflag.UnquoteUsage`; required via `flag.Annotations[cobra.BashCompOneRequiredFlag]`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestCollect|TestSignature' -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-check the fail-closed producers**

In `collectCapabilities`, temporarily replace the unclassified-leaf error with `continue`; run `go test ./internal/cli/ -run TestCollectRejectsUnclassifiedLeaf -count=1`; expected: FAIL. Restore from your backup copy. Repeat for the duplicate-id branch (drop the `dup` check → `TestCollectRejectsDuplicateID` fails). Record both readings in the task notes.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/capability.go internal/cli/capability_test.go
git commit -m "feat(cli): capability metadata vocabulary, annotation helper, and tree walker"
```

---

### Task 2: Annotate every production leaf

**Files:**
- Modify: `internal/cli/root.go`, `internal/cli/change.go`, `internal/cli/context.go`, `internal/cli/artifact.go`, `internal/cli/workspace.go`, `internal/cli/evidence.go`, `internal/cli/pr.go`, `internal/cli/run.go`, `internal/cli/learning.go`, `internal/cli/adr.go`, `internal/cli/gate.go`, `internal/cli/finalize.go`, `internal/cli/maintenance.go`, `internal/cli/repository.go`
- Test: `internal/cli/capability_production_test.go` (create)

**Interfaces:**
- Consumes: `capability(id, effects...)`, `collectCapabilities`, `operationName`/`commandKey` from `internal/cli/install.go`.
- Produces: every public executable leaf annotated; ids spelled exactly as `operationName(commandKey(c))` (dotted path — the same spelling the protocol's `operation` field and the `assetIndependent` correspondence already use).

**Effect classification.** The table below is the proposed classification. It is a *hypothesis to verify, not an oracle* (learning: verify-the-claim): for each family, before annotating, read the backing `internal/app` operation's doc comment and confirm what it actually writes — the metadata worktree/branch (→ `metadata-write`), the local filesystem/feature worktree/local git/durable local state (→ `local-write`), GitHub or any remote ref outside the metadata branch (→ `external-write`), or process supervision (→ `process-control`). Effects describe *possible* effects of the operation, not one invocation's disposition. Where you find the table wrong, fix the annotation AND note the correction in the task's commit message body.

| Id(s) | Effects |
|---|---|
| `capabilities`, `version`, `status`, `diagnostic.runtime`, `diagnostic.config`, `repository.check`, `install.check`, `context.implementation`, `context.finalize`, `evidence.verify`, `workspace.inspect`, `run.verify`, `gate.observe` | `read` |
| `change.create`, `change.groom`, `change.block`, `change.defer`, `change.kill`, `change.claim`, `change.refresh-claim`, `change.reconcile`, `change.attach-plan`, `change.attach-results`, `change.halt`, `change.resume-halted`, `change.reclaim`, `change.mark-implemented`, `change.repair-identity`, `learning.record`, `learning.update`, `adr.record`, `adr.supersede`, `adr.reverse` | `metadata-write` |
| `artifact.backlink`, `evidence.record`, `gate.recover`, `gate.cleanup`, `gate.drive.handoff`, `gate.drive.claim`, `run.gate-before`, `run.gate-verdict`, `finalize.rebase-continue`, `finalize.rebase-abort`, `repository.configure-tests`, `install`, `development.install` | `local-write` |
| `workspace.prepare` | `local-write` (verify whether it also pushes → add `external-write` if so) |
| `workspace.publish`, `pr.publish`, `finalize.publish`, `finalize.merge`, `finalize.retarget-children` | `external-write` (pr/finalize publish also stamp metadata — verify and add `metadata-write` where the app op writes the metadata branch) |
| `finalize.block`, `finalize.clear-block` | `external-write`, `local-write` (PR comment + durable marker; verify marker location) |
| `finalize.closeout`, `maintenance.sweep` | `metadata-write`, `external-write`, `local-write` (archive + board + remote-ref/worktree cleanup; verify each) |
| `finalize.cleanup` | `local-write`, `external-write` |
| `finalize.rebase` | `local-write`, `process-control` (runs the local gate) |
| `gate.launch`, `gate.stop`, `gate.drive.start`, `gate.drive.advance` | `process-control`, `local-write` (`gate.stop`: verify whether it writes run state → likely both) |
| `repository.init`, `repository.migrate` | `metadata-write`, `local-write` |
| `repository.prepare` | `local-write` |
| `development.test` | `process-control` |

(`capabilities` itself is annotated in Task 3 when the command exists; the table row is here so the classification is complete in one place.)

- [ ] **Step 1: Write the failing production correspondence test**

`internal/cli/capability_production_test.go` — build the production tree the way `root_test.go` / `install_test.go` do (find the existing helper that assembles the tree for `TestAssetIndependentSetExact` in `internal/cli/install_test.go` and reuse its approach; do not re-implement tree assembly):

```go
func TestProductionCapabilityCorrespondence(t *testing.T) {
	root := productionRootForTest(t) // reuse/extract the assembly seam install_test.go uses
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err) // any unclassified leaf fails here, loudly, naming the command
	}
	// Forward: every catalog entry resolves to a real, visible, executable command.
	byID := map[string]CapabilityEntry{}
	for _, e := range entries {
		c, _, ferr := root.Find(e.Argv[1:])
		if ferr != nil || c == nil || c.Hidden || (c.RunE == nil && c.Run == nil) {
			t.Errorf("entry %q argv %v resolves to no public executable command", e.ID, e.Argv)
		}
		if want := operationName(commandKey(c)); e.ID != want {
			t.Errorf("id %q != dotted command path %q — a deliberate rename must update this test", e.ID, want)
		}
		byID[e.ID] = e
	}
	// Reverse: every public executable leaf appears exactly once (walk
	// independently of collectCapabilities — count visible commands with a
	// Run/RunE that are not group missing-command stubs, i.e. carry the
	// annotation OR have no visible children).
	leaves := enumeratePublicExecutableLeaves(root) // test-local walker, written fresh
	if len(leaves) != len(entries) {
		t.Errorf("tree has %d public executable leaves, catalog has %d entries", len(leaves), len(entries))
	}
	// Population floor: the current tree. Not a hand-tuned target — computed
	// against the independent walk above; this constant only pins gross collapse.
	if len(entries) < 60 {
		t.Errorf("catalog population %d below floor 60 — the walker is dropping the tree", len(entries))
	}
	// Absences: root, groups, hidden, completion machinery.
	for _, absent := range []string{"", "change", "gate", "gate drive", "help", "__complete", "completion"} {
		if _, ok := byID[operationName(absent)]; ok && absent != "" {
			t.Errorf("group/machinery %q must not be a catalog entry", absent)
		}
	}
}
```

Note on the reverse direction (learning: correspondence-guard-runs-one-way): `enumeratePublicExecutableLeaves` must build its population by walking the tree directly, NOT by reusing `collectCapabilities` — the two directions must not share an extraction. A group stub (`RunE` returning missing-command) is distinguished structurally: it has visible children and no annotation. The bare-`docket` root and `help` are excluded by the walk starting at root's children and `help` being Cobra's help command (check `root.Find(["help"])` behavior; exclude by name if needed, with a comment).

Also add:

```go
func TestProductionEffectsCompleteAndClosed(t *testing.T) {
	// every entry: >=1 effect, all from the closed vocabulary, sorted, deduped
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run TestProductionCapability -count=1`
Expected: FAIL inside `collectCapabilities` with an unclassified-leaf error naming the first unannotated production leaf (this failure IS the test working).

- [ ] **Step 3: Annotate every leaf, family by family**

Pattern — inline commands in `root.go`:

```go
versionCmd := &cobra.Command{
	Use:         "version",
	Short:       "Report this binary's build identity",
	Annotations: capability("version", EffectRead),
	...
}
```

Constructor-built families: `changeSubcommand`, `changeInputSubcommand`, `changeIDVersionSubcommand` (change.go), `repositorySubcommand` (repository.go), and the analogous helpers in the other family files each gain the annotation at the single point the `cobra.Command` literal is built. Where a helper builds N verbs, pass the effects through the helper (id derives from the verb: `"change."+verb`); where a family's verbs have differing effects (finalize, gate), pass effects per call site. Do NOT introduce any map from command name to effects — the annotation must be an argument at the registration call site.

Keep ids spelled as the dotted command path (`operationName` spelling): `change.reconcile`, `gate.drive.advance`, `run.gate-verdict`, `install`, `install.check`, `development.test`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -count=1`
Expected: PASS, including all pre-existing cli tests (annotations must not disturb them).

- [ ] **Step 5: Mutation-check**

(a) Remove the annotation from one leaf (e.g. `evidence verify`) → `TestProductionCapabilityCorrespondence` fails with the unclassified-leaf error. (b) Remove one effect string so the annotation is empty → `TestProductionEffectsCompleteAndClosed` (or the walker validation) fails. (c) Stub `collectCapabilities` to return `nil, nil` → the population floor and leaf-count comparison fail. Restore from backup after each; run with `-count=1`. Record all three readings.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/
git commit -m "feat(cli): co-located capability annotations on every public executable leaf"
```

---

### Task 3: The `docket capabilities` command and protocol document

**Files:**
- Create: `internal/app/capabilities.go`, `internal/app/capabilities_test.go`, `internal/cli/capabilities.go`
- Modify: `internal/cli/root.go` (register), `internal/cli/install.go` (`assetIndependent["capabilities"] = true`)

**Interfaces:**
- Consumes: `collectCapabilities`, `CapabilityEntry`, `app.NewEnvelope`, `buildinfo.Info` (already threaded into `run`).
- Produces: `app.CapabilitiesResult` with JSON shape:

```json
{
  "protocol_version": 1,
  "operation": "capabilities",
  "result": "applied",
  "capability_version": 1,
  "binary": {"version": "<info version>", "revision": "<info revision>"},
  "global": {"flags": [{"name": "json", "type": "bool", "default": "false", "usage": "emit protocol-v1 JSON on stdout"}]},
  "commands": [ {"id": "...", "argv": ["docket", "..."], "signature": "...", "effects": ["..."]}, ... ]
}
```

`capability_version` is a new constant `app.CapabilityVersion = 1`, versioned separately from `ProtocolVersion`. The `binary` field names must match what `app.Version` already emits for version/revision — read `internal/app/version.go` (or wherever `app.Version` lives) first and reuse its field spellings/types so consumers see one identity vocabulary. The `global.flags` block is built from the root's persistent flags (today exactly `--json`), represented once at document level and never per entry.

- [ ] **Step 1: Write the failing app-side test**

`internal/app/capabilities_test.go`:

```go
func TestCapabilitiesEnvelopeAndShape(t *testing.T) {
	res := Capabilities(buildinfo.Info{ /* fixture identity */ }, []CapabilityCommand{
		{ID: "b.op", Argv: []string{"docket", "b", "op"}, Effects: []string{"read"}},
		{ID: "a.op", Argv: []string{"docket", "a", "op"}, Effects: []string{"metadata-write"}, Signature: "--request <file>"},
	}, GlobalInvocation{ /* json flag */ })
	env := res.Env()
	if env.Operation != "capabilities" || env.Result != ResultApplied || env.ProtocolVersion != ProtocolVersion {
		t.Fatalf("envelope = %+v", env)
	}
	b, err := json.Marshal(res)
	if err != nil { t.Fatal(err) }
	// pin top-level field names — they are protocol
	for _, key := range []string{"protocol_version", "operation", "result", "capability_version", "binary", "global", "commands"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("missing field %q in %s", key, b)
		}
	}
	// input order is preserved as given — sorting is the walker's job, and
	// the CLI test asserts the composed pipeline is sorted
}
func TestCapabilitiesHumanText(t *testing.T) { /* one line per command: "<id>  <argv> <signature>  [effects]" */ }
```

Note: `app` cannot import `internal/cli` (cli imports app), so define plain carrier types in app — `CapabilityCommand`, `GlobalInvocation` — and have the cli adapter convert `CapabilityEntry` → `CapabilityCommand`. Keep both structs' JSON tags identical (`id`, `argv`, `signature,omitempty`, `effects`).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/app/ -run TestCapabilities -count=1` — FAIL, undefined symbols.

- [ ] **Step 3: Implement `internal/app/capabilities.go`**

```go
// CapabilityVersion identifies the capability-catalog contract, versioned
// separately from the protocol envelope: consumers refuse a version they do
// not support, fail-closed.
const CapabilityVersion = 1

type CapabilityCommand struct {
	ID        string   `json:"id"`
	Argv      []string `json:"argv"`
	Signature string   `json:"signature,omitempty"`
	Effects   []string `json:"effects"`
}

type GlobalFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
	Usage   string `json:"usage"`
}

type GlobalInvocation struct {
	Flags []GlobalFlag `json:"flags"`
}

type BinaryIdentity struct { /* mirror app.Version's field spellings */ }

type CapabilitiesResult struct {
	Envelope
	CapabilityVersion int              `json:"capability_version"`
	Binary            BinaryIdentity   `json:"binary"`
	Global            GlobalInvocation `json:"global"`
	Commands          []CapabilityCommand `json:"commands"`
}

func Capabilities(info buildinfo.Info, commands []CapabilityCommand, global GlobalInvocation) OperationResult {
	return CapabilitiesResult{
		Envelope:          NewEnvelope("capabilities", ResultApplied),
		CapabilityVersion: CapabilityVersion,
		Binary:            binaryIdentity(info),
		Global:            global,
		Commands:          commands,
	}
}
```

Plus `HumanText()` — compact, one line per command, prefixed by a `capabilities v1 — <n> commands` header.

- [ ] **Step 4: Implement the cli adapter and register**

`internal/cli/capabilities.go`:

```go
// newCapabilitiesCommand builds `docket capabilities`: the one repository-,
// config-, asset-, and network-independent bootstrap. Its RunE touches no
// filesystem, no config loader, no git, no network — it walks the already-
// assembled tree it is itself registered in and hands the projection to the
// presenter. A walker validation failure is an internal error: the tree this
// binary shipped is inconsistent, and the answer is fail-closed, not partial.
func newCapabilitiesCommand(info buildinfo.Info, setResult func(app.OperationResult)) *cobra.Command {
	return &cobra.Command{
		Use:         "capabilities",
		Short:       "Emit this binary's complete executable command catalog (read-only, repository-independent)",
		Args:        cobra.NoArgs,
		Annotations: capability("capabilities", EffectRead),
		RunE: func(c *cobra.Command, _ []string) error {
			entries, err := collectCapabilities(c.Root())
			if err != nil {
				return fmt.Errorf("capability catalog construction failed: %w", err)
			}
			setResult(app.Capabilities(info, toAppCommands(entries), globalInvocation(c.Root())))
			return nil
		},
	}
}
```

In `root.go`: `capabilitiesCmd := newCapabilitiesCommand(info, func(r app.OperationResult) { result = r })` and add it to the `root.AddCommand(...)` call. In `install.go`: add `"capabilities": true` to `assetIndependent` with the comment `// the capability bootstrap must answer before any installation exists`. The existing `TestAssetIndependentSetExact` correspondence test will force this addition — run it first to watch it demand the key.

- [ ] **Step 5: Write the failing cli-level end-to-end tests, then make them pass**

Append to `internal/cli/capability_production_test.go` (or a sibling): invoke the full `Run`/`run` entry the way existing cli tests do (see how `root_test.go` calls it with buffers):

```go
func TestCapabilitiesCommandEmitsSortedDeterministicJSON(t *testing.T) {
	out1 := runDocket(t, "capabilities", "--json") // helper wrapping cli.Run with buffers
	out2 := runDocket(t, "capabilities", "--json")
	if out1 != out2 {
		t.Fatal("repeated catalog output is not byte-identical")
	}
	var doc struct {
		ProtocolVersion   int `json:"protocol_version"`
		CapabilityVersion int `json:"capability_version"`
		Commands          []struct{ ID string `json:"id"` } `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out1), &doc); err != nil { t.Fatal(err) }
	if !sort.SliceIsSorted(doc.Commands, func(i, j int) bool { return doc.Commands[i].ID < doc.Commands[j].ID }) {
		t.Fatal("commands not sorted by id")
	}
}
```

Run with `-count=1` until green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/capabilities.go internal/app/capabilities_test.go internal/cli/
git commit -m "feat(cli): docket capabilities — read-only protocol-v1 catalog command"
```

---

### Task 4: Independence, budget, and signature pinning

**Files:**
- Modify: `internal/cli/capability_production_test.go`
- Possibly modify: 2–3 `Use:` strings (`gate launch`, `gate drive start`, `run gate-verdict`) only if Task 1's stripping rule produces a signature the pinned expectations show to be wrong — prefer fixing the rule; touch `Use` text only when it restates registered flags redundantly.

- [ ] **Step 1: Write the failing independence test**

```go
func TestCapabilitiesIsRepositoryConfigAssetAndWriteIndependent(t *testing.T) {
	// cwd inside an empty non-git temp dir, HOME pointed at another empty temp
	// dir (no installation, no global config), and no network access assumed.
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out := runDocket(t, "capabilities", "--json") // must exit 0 with a full document
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil { t.Fatal(err) }
	// write-independence: the temp dirs stay empty afterwards
	assertDirEmpty(t, dir)
}
```

(If the runner helper resolves the git client eagerly, this test is exactly what proves the capabilities path must not — keep the command's RunE free of `gitcli.NewClient`, `config.Load*`, and `install.ResolveRoots`; the `assetIndependent` registration already bypasses the asset gate.)

- [ ] **Step 2: Write the failing budget test**

```go
func TestCapabilitiesPayloadWithinByteBudget(t *testing.T) {
	out := runDocket(t, "capabilities", "--json")
	n := len(out)
	t.Logf("capabilities payload: %d bytes (budget 12288)", n)
	if n > 12*1024 {
		t.Fatalf("catalog is %d bytes, over the 12KB design ceiling — growth is a design event (spec: Compactness boundary), not a truncation opportunity", n)
	}
	// content-exclusion asserts: no help prose fields
	for _, banned := range []string{`"short"`, `"long"`, `"example"`, `"help"`} {
		if strings.Contains(out, banned) {
			t.Errorf("catalog carries help-prose field %s", banned)
		}
	}
}
```

- [ ] **Step 3: Pin representative signatures exactly**

```go
func TestRepresentativeSignatures(t *testing.T) {
	// After running the real binary path once, pin exact signature strings for:
	//   change.reconcile   -> "--request <string> [--repo-dir <string>]"   (or <file>/<dir> if usage strings gain backquoted hints)
	//   status             -> "[--priority <string>...] [--repo-dir <string>] [--type <string>...]"
	//   gate.observe       -> "<run-dir>"
	//   gate.launch        -> flags then "-- <argv...>"
	//   run.gate-verdict   -> its composed form per the Task 1 rules
	// Copy the MEASURED strings into the expectations — measured, not predicted
	// (learning: plan-supplied-test-code-is-unverified) — then review each for
	// faithfulness to the real invocation before pinning.
}
```

Optionally (recommended, small diff): add backquoted value hints to the highest-traffic flag usages (`"JSON request `file`..."`, `"repository `dir`..."`) so signatures read `--request <file> [--repo-dir <dir>]` as in the spec's representative entry. Keep the diff to usage strings only.

- [ ] **Step 4: Run the full cli + app packages**

Run: `go test ./internal/cli/ ./internal/app/ -count=1` — PASS.

- [ ] **Step 5: Record the measurement**

Run `go run ./cmd/docket capabilities --json | wc -c` from the worktree root. Note the byte count and compute the labeled estimate: `Fable-token estimate ≈ bytes / 3.5 (heuristic chars-per-token; ESTIMATE, not a measurement)`. Keep both figures in the task notes — Task 11 carries them into build evidence.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/
git commit -m "test(cli): capabilities independence, 12KB budget, and pinned signatures"
```

---

### Task 5: Producer mutation matrix, executed and recorded

**Files:** none new — this task executes the spec's five required mutations against the finished Go feature and records the readings. It exists as its own task so the evidence is a deliberate gate, not a footnote.

- [ ] **Step 1: Back up the working tree state** (`git stash` is forbidden mid-mutation; copy the touched files aside with `cp`).

- [ ] **Step 2: Run the five mutations** — each: apply, run `go test ./internal/cli/ -count=1`, confirm the named test reddens, restore from the backup copy, re-run to confirm green.

| # | Mutation | Must redden |
|---|---|---|
| 1 | Add a new public leaf (e.g. `&cobra.Command{Use: "probe2", RunE: ...}` under `diagnostic`) with no annotation | walker error via `TestProductionCapabilityCorrespondence` |
| 2 | Filter one entry out of the walker's return before sorting | leaf-count comparison + forward/reverse correspondence |
| 3 | Delete the `capAnnotationEffects` key from one leaf's annotation payload | walker effects validation |
| 4 | Remove `MarkFlagRequired("request")` from `changeSubcommand` | `TestRepresentativeSignatures` (required flag renders optional) |
| 5 | Make `collectCapabilities` return the first 5 entries only | population floor assert |

- [ ] **Step 3: Verify the tree is restored** — `git status --porcelain` shows only the intended committed state; `go test ./internal/cli/ -count=1` green.

- [ ] **Step 4: Record** the 5×(red reading, green restore) matrix in the task completion notes for the build-evidence record. No commit (nothing changed); if any mutation FAILED to redden, that is a defect — fix the guard in this task and commit the fix.

---

### Task 6: Derive the CLI-literal inventory (read-only reconnaissance)

**Files:** none modified. Output: a classified inventory pasted into this task's completion notes, consumed by Tasks 7–9.

- [ ] **Step 1: Whole-source search.** From the worktree root:

```bash
grep -rn -E '(^|[^[:alnum:]_./-])docket[[:space:]]+[a-z][a-z-]*' \
  --include='*.md' --include='*.toml' --include='*.yml' \
  skills/ agents/ cursor-rules/ scripts/ README.md .docket.example.yml > /tmp/inventory-raw.txt || true
wc -l /tmp/inventory-raw.txt
```

Do NOT hand-list sites from memory (AGENTS.md rule; learning: enumerated-floor). The pattern is deliberately wide; classification narrows it.

- [ ] **Step 2: Classify every hit** into exactly one bucket:

1. **Agent-executed instruction** (a fenced command block or an inline invocation an operating skill/agent runs verbatim) on a *maintained workflow surface* (`skills/`, `agents/`, `cursor-rules/`) → **migrate** (Tasks 7–8).
2. **Human-facing remedy or documentation** — README, `.docket.example.yml` comments, `scripts/*.md` contracts, and skill prose that instructs a *human* (the convention marks `docket repository migrate` and `docket repository init` human-initiated) → **preserve**; collect the exact spellings: they seed the guard's exemption set (Task 9).
3. **Immutable history** — anything under `docs/` (changes, specs, plans, results, ADRs) → **preserve untouched**; the guard excludes `docs/` wholesale (same line `internal/repoguard`'s existing corpora draw).
4. **The bootstrap itself** — `docket capabilities` spellings added by this change → permitted everywhere.

- [ ] **Step 3: Count each bucket** and record the counts — the guard's exemption assertions in Task 9 use the *measured* numbers, never estimates (learning: byte-pattern-guard-matches-a-spelling).

- [ ] **Step 4: Sanity-check bucket 1 against the catalog**: every semantic operation those instructions perform must exist as a catalog id (run `go run ./cmd/docket capabilities --json` and compare). A workflow verb with no catalog entry is a blocking finding to surface, not to paper over. Known drift to expect (from the change file's reconcile log): skill prose describing `--id`/`--version` flag shapes where the binary takes `--input`/`--request` request files — the migration to semantic ids + catalog-resolved argv is exactly what retires that drift.

No commit (read-only task).

---

### Task 7: Revise the shared Step-0 preamble in docket-convention

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — the `### Step-0 preamble (every operating skill)` section and the `**Reaching docket's operations.**` paragraph.

**Interfaces:**
- Produces: the capability-bootstrap contract every operating skill inherits by pointer. Skill bodies already compress Step 0 to a pointer here, so most per-skill edits in Task 8 are small.

- [ ] **Step 1: Rewrite the Step-0 preamble** to this shape (adapt the surrounding prose, keep the existing numbered-list voice; the normative content below is required):

1. Load this convention (blocking).
2. **Capability bootstrap.** If the current agent context does not already carry a validated capability catalog, run `docket capabilities --json` as its own Bash call — the one executable Docket CLI spelling this convention permits skills to hard-code — and validate the response fail-closed before any other docket invocation:
   - the command is unknown to the binary → the installed binary predates the capability contract: stop and instruct the human to update or reinstall Docket. Never fall back to `--help`, guessed verbs, or probe invocations.
   - refuse (stop, surface the diagnostic): `protocol_version` ≠ 1 or `capability_version` unsupported; a malformed envelope; duplicate `id`s; an effect outside `read | local-write | metadata-write | external-write | process-control`; an entry with missing/empty `argv`; an absent/empty `commands` array.
   - a validated catalog may be reused by later skills in the same agent context; a separately dispatched agent fetches its own. No cache file, no repository metadata, no environment transport.
3. **Prepare.** Resolve the `repository.prepare` entry from the catalog and run its argv with `--repo-dir <dir> --json` — then validate the protocol-v1 envelope and carry its typed context forward, exactly as today (keep the existing disposition table verbatim).
4. **Mid-run posture** (add to the preamble's closing prose): construct every subsequent docket invocation from the fetched catalog entry for the semantic operation the skill names. An operation the workflow needs that is absent from the catalog → the workflow's existing hard-error posture; never guess a spelling. An invocation resolved from a validated catalog that returns unknown-command mid-run → the binary was replaced or is inconsistent: stop; do not silently refetch and switch interfaces mid-workflow. An operation whose cataloged `effects` exceed the workflow's authorized boundary → stop with a capability-mismatch diagnostic.

- [ ] **Step 2: Update `**Reaching docket's operations.**`** — replace its enumerated argv examples (`docket repository prepare`, `docket maintenance sweep`, ...) with semantic operation ids (`repository.prepare`, `maintenance.sweep`, `status`, `change.*`, `context.*`, `artifact.backlink`, ...) resolved from the catalog, keeping the paragraph's other claims (PATH binary, no facade, no `DOCKET_*` transport, missing binary = broken install) intact. Preserve the human-initiated `docket repository migrate` / `docket repository init` remedy spellings — humans type those; mark them in prose as human-typed so Task 9's guard exemption has an anchor.

- [ ] **Step 3: Consistency read** of the whole file: every other section that spells a `docket <verb>` invocation an *agent* executes gets the semantic-id treatment (the file discusses many operations; per learning consolidation-flattens-caller-variance, edit each site on its own terms — postures differ, do not template). Sites quoting commands as *history* or *human* action stay.

- [ ] **Step 4: Commit**

```bash
git add skills/docket-convention/SKILL.md
git commit -m "feat(skills): capability-first Step-0 preamble in docket-convention"
```

---

### Task 8: Migrate the operating skills, agent contracts, and dispatch rules

**Files:**
- Modify: every bucket-1 file from Task 6's inventory. Expect (verify against the inventory, do not trust this list): `skills/docket-status/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-adr/SKILL.md`, `skills/docket-build/SKILL.md`, `skills/docket-build-task/SKILL.md`, `skills/docket-review/SKILL.md`, `skills/docket-brainstorm/SKILL.md`, `skills/*/references/*.md`, `agents/docket-*.md`, `cursor-rules/*`.

**Migration rule per site:**
- A fenced block or inline invocation the agent runs → replace the hard-coded argv with the semantic operation reference: name the operation by catalog id and state its flags/inputs semantically. House style for the migrated spelling (use it consistently — a follow-up reader must recognize one idiom): ``run the `maintenance.sweep` operation (resolve argv from the capability catalog) with `--scope full --json` `` — the *flags a skill must pass* stay literal (they are the operation's signature, carried by the catalog), the *argv path* does not.
- Fenced multi-command examples (like docket-status's "Run the pass" block) become operation-id sequences with the same flags, introduced by "resolve each from the capability catalog".
- Skill-body Step-0/Convention sections that today spell `docket repository prepare --repo-dir <dir> --json` (e.g. docket-status's "Convention (load first — blocking)" paragraph) → point at the convention's revised Step-0 preamble ("run its Step-0 preamble: capability bootstrap, then `repository.prepare`") without restating argv.
- Explanatory prose *about* an operation (not an instruction to run it) may keep the operation's dotted id; it must not keep a `docket <argv>` spelling unless the site is a preserved human remedy (bucket 2).
- Immutable history quoted inside skill bodies (change-number war stories) — treat as prose; migrate spelling only if it is an instruction the agent executes today.

- [ ] **Step 1: Work through the inventory file-by-file**, committing per coherent group (skills; agents+cursor-rules). Diff-read each file after editing: the migration must not change any posture, gate, or failure path — spelling only (learning: consolidation-flattens-caller-variance; verify with `git diff --word-diff` that deletions are argv spellings, not behavioral sentences).

- [ ] **Step 2: Grep for stragglers** with Task 6's exact pattern over `skills/ agents/ cursor-rules/`; every remaining hit must be bucket 2 (human remedy), bucket 4 (bootstrap), or a prose mention you can defend. Record the final count — Task 9 asserts it.

- [ ] **Step 3: Run the existing suite** — `go run ./cmd/docket development test`. Existing repoguard/prose-contract tests grep skill prose (learning: restatement-accumulates-its-own-guards); any test that pinned a migrated argv spelling reddens here. For each: update the test to pin the *migrated* spelling (the semantic-id form), preserving what the test guards — never delete a guard because the spelling moved (learning: test-premise-deleted-not-regated; ask what each reddened assert guards).

- [ ] **Step 4: Commit**

```bash
git add skills/ agents/ cursor-rules/ internal/repoguard/
git commit -m "feat(skills): migrate agent-executed CLI literals to catalog-resolved semantic operations"
```

---

### Task 9: Shape-based guard on maintained workflow surfaces

**Files:**
- Create: `internal/repoguard/capability_surface_test.go`

**Design.** The guard scans the maintained workflow surfaces (`skills/`, `agents/`, `cursor-rules/`) for the shape `docket <word>` in executable position and fails on any hit that is not (a) the capability bootstrap or (b) a member of the asserted exemption set. Follow the house patterns in `internal/repoguard/shellshape_test.go` (corpus helpers, bounded patterns, KNOWN IMPRECISION headers).

- Pattern: bound on both sides — `(^|[^[:alnum:]_./-])docket[[:space:]]+[a-z][a-z-]*` (RE2), so `dev/docket`, `docket-status`, `.docket/` never match (learning: byte-pattern-guard-matches-a-spelling).
- Population: all `*.md` under `skills/` and `agents/`, all files under `cursor-rules/` — agent-executed markdown is code (learning: agent-executed-markdown-is-code). `docs/` and `tests/fixtures`/`testdata` are out of scope (immutable history / data — learning: frozen-fixture-corpus-trips-repo-wide-scans); `scripts/*.md` contracts are documentation of the frozen script layer, also out of scope, with the exclusion stated in the header.
- Allow: `docket capabilities` (any flags after it).
- Exemption set: the bucket-2 human-remedy spellings Task 6 measured (expected: `docket repository migrate`, `docket repository init`, possibly the install remedies `docket development install` / `docket install` in human-facing install prose). Assert the exemptions as an exact *computed* count per spelling — measure at HEAD after Task 8, then pin — with a failure message that leads with the substantive check ("a new `docket <argv>` spelling on a workflow surface must be migrated to a catalog-resolved semantic operation — see docket-convention's Step-0 preamble") before it mentions the count (learning: guard-remedy-must-not-teach-the-evasion).
- KNOWN IMPRECISION header: prose that *names* a command in backticks is byte-identical to an instruction to run it; this guard bans the spelling, not the intent, and the exemption set is the sanctioned residue.

- [ ] **Step 1: Write the guard** with the shape above; run `go test ./internal/repoguard/ -run TestCapabilitySurface -count=1`. If it fails on a site Task 8 missed, that is the guard doing its job — migrate the site (amend Task 8's spelling rule), not the guard.

- [ ] **Step 2: Mutation-test in both directions.**
  - *Addition:* plant `` run `docket status --json` `` in a fenced block in `skills/docket-status/SKILL.md` → guard reddens. Restore.
  - *Deletion:* remove every legitimate `docket capabilities` spelling from `skills/docket-convention/SKILL.md` → the guard's bootstrap-presence floor reddens (add one: assert the convention's Step-0 preamble contains the `docket capabilities --json` spelling at least once — the marker-population floor, learning: marker-scoped-guard-needs-a-population-floor). Restore.
  - *Exemption laundering:* add a second `docket repository migrate` in an agent-executed fenced block → the per-spelling exemption count reddens. Restore.
  - Run each with `-count=1`; record readings.

- [ ] **Step 3: Commit**

```bash
git add internal/repoguard/capability_surface_test.go
git commit -m "test(repoguard): shape guard — only the capability bootstrap may hard-code docket argv on workflow surfaces"
```

---

### Task 10: Regenerate embedded assets and prove reproducibility

**Files:**
- Regenerate: `internal/assets/embedded/**` (owner: `cmd/genassets` via `go generate ./internal/assets`)

- [ ] **Step 1: Regenerate through the canonical owner**

```bash
go generate ./internal/assets
git status --porcelain internal/assets/embedded | head
```

- [ ] **Step 2: Prove byte-equivalence** — the embedded bundle must equal the authored trees exactly. Run the existing manifest/embedded tests: `go test ./internal/assets/ -count=1`. Read `internal/assets/embedded_test.go` and `manifest_test.go` first: if an authored-tree ↔ embedded byte-equality assert already exists, cite it in the task notes; if it does not, add `TestEmbeddedMatchesAuthoredTree` (walk `DefaultAllowedRoots()`, byte-compare each file against the embedded copy — learning: frozen-copy-needs-a-drift-assert).

- [ ] **Step 3: Prove idempotent regeneration** — run `go generate ./internal/assets` a second time; `git status --porcelain internal/assets` must be empty of new modifications. Record the observation.

- [ ] **Step 4: Harness-generated artifacts** are machine-local (ADR-0020) and regenerate through the Go install at install time — no committed artifact to refresh here beyond `cursor-rules/` (authored, already migrated in Task 8) and `agents/` (authored). Confirm by checking `git status` for any other generated surface the suite flags; the harness generators' own tests (`internal/harness/...`) run in the full suite and prove the generated outputs track the migrated sources.

- [ ] **Step 5: Commit**

```bash
git add internal/assets/
git commit -m "chore(assets): regenerate embedded bundle after skill migration"
```

---

### Task 11: Full suite gate, measurements into evidence, and human-verify items

- [ ] **Step 1: Run the whole suite from the checkout**

```bash
go run ./cmd/docket development test
```

Expected: green. Treat any `SERIAL CONFIRMED OVER BUDGET:` line as an authoritative breach to act on; `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` are screening findings to record.

- [ ] **Step 2: Re-measure the payload at final HEAD**

```bash
go run ./cmd/docket capabilities --json | wc -c
```

- [ ] **Step 3: Build-evidence content.** The results/evidence record (written by the build workflow's own evidence step) must include:
  - measured catalog byte count at final HEAD, against the 12288-byte ceiling;
  - the labeled estimate: `Fable-token estimate ≈ <bytes>/3.5 ≈ <n> tokens (heuristic; ESTIMATE, not a tokenizer measurement)`;
  - the Task 5 producer mutation matrix (5 mutations, red/green readings) and Task 9's guard mutation readings;
  - the Task 10 idempotence observation.

- [ ] **Step 4: Named human-verification items** (in-repo tests cannot be their oracle — learning: external-truth-needs-a-human-checkpoint; generated-artifact-loaded-at-process-start means a fresh harness session is a precondition):
  1. **Cursor acceptance:** after installing the built artifacts, run `docket-status` and one metadata-writing groom/new-change path in Cursor; verify neither run invokes `--help`, tries alternative commands, inspects binary strings, or issues discovery probes. Record the Cursor version and mode.
  2. **Second-harness acceptance:** repeat one representative workflow in another supported harness (Claude Code or Codex or OpenCode); record version/mode (learning: harness-behavior-is-mode-and-version-scoped).
  3. **Stale-binary posture:** with a pre-contract `docket` on PATH, confirm a migrated skill stops at the unknown-`capabilities` boundary with the update/reinstall instruction.
  4. Post-merge: this change adds no `.docket.yml` field, so the schema-bump install deadlock does not apply; the ordinary rebuild-after-merge rule does.

No separate commit unless Steps 1–2 forced fixes.

---

## Self-Review (performed while writing)

- **Spec coverage:** stable bootstrap (T3), live-tree derivation + co-located metadata (T1–T2), effect model (T1–T2), compactness + budget + measured bytes/token estimate (T4, T11), workflow consumption / Step-0 (T7), skill migration + inventory derivation (T6, T8), failure posture producer-side (T1, T3) and consumer-side (T7), correspondence + five mutation tests (T2, T5), shape guard without hand-listed sites (T6, T9), generated assets byte-equivalence + idempotence (T10), historical-record preservation (T6 bucket 3, T9 exclusions), cross-harness acceptance (T11 human-verify), 0360/MCP exclusions (nothing here touches schemas or MCP). Acceptance criteria 1–10 each map to at least one task.
- **Known open judgment calls, delegated with rules:** exact effect sets for compound finalize/maintenance ops (T2 verify-against-app rule), signature micro-format for the three flag-restating `Use` tails (T1 rule + T4 pinning), the guard's exemption spellings (T6 measurement).
- **Type consistency:** `CapabilityEntry` (cli) vs `CapabilityCommand` (app) conversion is named in T3; `capability`, `collectCapabilities`, annotation constants used consistently across T1–T5.
