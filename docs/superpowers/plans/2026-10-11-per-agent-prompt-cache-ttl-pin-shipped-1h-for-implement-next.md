<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0546 — Per-agent prompt-cache TTL pin, shipped 1h for implement-next](../../changes/active/0546-per-agent-prompt-cache-ttl-pin-shipped-1h-for-implement-next.md)**
<!-- docket:backlink:end -->

# Per-agent prompt-cache TTL pin Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build`, task by task, under the
> `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional per-agent `cache_ttl` pin beside `model` and `effort`. Claude Code wrappers
render it as `experimental.cacheTtl`. Docket ships `cache_ttl: 1h` on the claude `implement-next`
entry only. Other harnesses leave the field out of their wrappers, and install warns when it does.

**Architecture:** `cache_ttl` is a third leaf in the existing `agents.<harness>.<agent>` pin table.
It reuses the registry row machinery (`dispAgentsLeaf`), so decoding, layering (built-in, then
global, per field, with `agents.default` fallback), and the repository-layer refusal come for free.
The Claude adapter is the only renderer that reads the new field. The three other adapters keep
calling `harness.ResolvedAgent`, which returns only model and effort, so they already drop the
field. The drop warning is a pure function in `internal/app`, keyed on the resolved value and
narrowed to the selected harnesses (the same narrowing `AgentDigest` uses). It is appended to the
install path's existing `ConfigWarnings` channel in `internal/cli/install.go`, so `install`,
`install check`, and `development install` all print it.

**Tech Stack:** Go (`go test`), go-yaml v3 (`go.yaml.in/yaml/v3`), docket's embedded asset generator
(`go generate ./internal/assets/`), the whole-suite runner `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-10-09-per-agent-prompt-cache-ttl-pin-shipped-1h-for-implement-next-design.md`
(on the `docket` metadata branch; executors read both the spec and this plan).

## Global Constraints

- The YAML key is `cache_ttl`, under `agents.<harness>.<agent>`, beside `model` and `effort`.
- Claude rendering is exactly `experimental:` followed by `  cacheTtl: <value>` (two-space indent). It is never a top-level `cacheTtl:` key.
- The shipped default is `cache_ttl: 1h` on `agents.claude.implement-next` only. No other harness or agent ships one.
- Values are opaque passthrough (ADR-0015). Docket keeps no vendor allowlist, so `1h`, `5m`, or a future token passes through verbatim. The only validation is the existing model/effort leaf rule: a non-empty single token.
- `cache_ttl: auto` removes the pin (resolves to `""`, so no `experimental` block is emitted). This is the same sentinel `effort: auto` uses.
- When `cache_ttl` is unset, the generated wrapper is byte-identical to today's.
- Layering: built-in, then global only, per field, with harness-specific falling back to `agents.default`. A repository-layer (`.docket.yml` / `.docket.local.yml`) `cache_ttl` blocks writes, like every other agent pin.
- Non-Claude harnesses (cursor, codex, opencode) never carry the field in their generated files. A resolved `cache_ttl` on one of them is a warning, never a block (ADR-0077 warn-and-drop posture).
- Warning code: `agent-cache-ttl-dropped`.
- A default install (shipped sidecar, no global overrides) prints no `agent-cache-ttl-dropped` warning.
- Docs describe current behaviour only. They cite no change numbers or PRs (ADR citations are fine).
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- Frozen fixture trees under `testdata/repositories/` are immutable. A changed shipped sidecar gets a NEW versioned tree (`v0.9.13`), never an in-place edit.
- After editing any embedded source (`agents/harness-defaults.yml`, `.docket.example.yml`, anything under `skills/`), run `go generate ./internal/assets/` so the embedded twin matches.
- Every manual re-run of a single Go test uses `-count=1` (defeats the test cache).

## Review Focus

These are the inputs most likely to hurt a user. Each line is followed by the task whose tests pin it.

1. **A user turning the shipped pin off.** `agents.claude.implement-next.cache_ttl: auto` (or `5m`) in the global config must beat the built-in `1h`: `auto` emits no `experimental` block and `5m` emits `5m`. Pinned in Task 2 (resolution over the shipped default) and Task 3 (render of an empty TTL is byte-identical).
2. **A per-field override on implement-next.** A global `agents.claude.implement-next.model: claude-sonnet-5` must keep the built-in `cache_ttl: 1h`, because overrides are per field. Pinned in Task 2.
3. **A `default:`-block `cache_ttl`.** `agents.default.<agent>.cache_ttl` reaches every harness. The non-Claude ones must warn, one warning per harness and agent, rather than dropping the value silently. Pinned in Task 4.
4. **A value that would not survive YAML.** A value such as `'#5m'` or `'[1h]'` must be refused by the Claude round-trip check (`ErrRender`) and never written as a wrapper that parses into something else. Pinned in Task 3.
5. **An install whose only change is a `cache_ttl` edit.** The agent digest must move so `install check` sees the wrapper as stale, and a table with no TTL must digest exactly as before so existing installs are not reported as drifted. Pinned in Task 4.

---

### Task 1: `cache_ttl` config leaf: schema row, resolution, `auto` sentinel

**Files:**
- Modify: `internal/config/config.go` (type `AgentSetting`)
- Modify: `internal/config/schema.go` (registry rows under the "32-33: agent pins" comment; `dispAgentsLeaf` comment; `dispositionSupported` comment)
- Modify: `internal/config/resolve.go` (func `resolveAgents`)
- Modify: `internal/config/defaults.go` (func `builtinAgents`)
- Test: `internal/config/resolve_test.go`, `internal/config/capability_test.go`, `internal/config/schema_test.go`, `internal/config/setting_paths_test.go`, `internal/config/decode_test.go`, `internal/config/defaults_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `config.AgentSetting.CacheTTL config.Value[string]` (JSON tag `cache_ttl`). Built-in value is `""` with built-in provenance; an explicit global `auto` resolves to `""` with `Explicit: true`. Registry path `agents.*.*.cache_ttl` (disposition `dispAgentsLeaf`, validator `stringLeaf(true, true, false)`).

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/resolve_test.go` (after `TestAgentsEffortAuto`):

```go
// cache_ttl is a third per-field agent pin: it resolves built-in → global, a
// harness-specific pin falls back to the global `default:` block, and it never
// disturbs the model or effort beside it.
func TestAgentsCacheTTLResolution(t *testing.T) {
	res := mustResolve(t, []Source{srcG(
		"agents:\n" +
			"  default:\n" +
			"    build-max:\n" +
			"      cache_ttl: 5m\n" +
			"  claude:\n" +
			"    plan-writer:\n" +
			"      cache_ttl: 1h\n")}, mainCtx)

	pw := res.effective.Agents["claude"]["plan-writer"]
	if pw.CacheTTL.Value != "1h" || pw.CacheTTL.Provenance.Layer != LayerGlobal || !pw.CacheTTL.Explicit {
		t.Errorf("claude/plan-writer cache_ttl = %+v, want an explicit global 1h", pw.CacheTTL)
	}
	if pw.Model.Value != "claude-opus-5" || pw.Model.Provenance.Layer != LayerBuiltIn || pw.Model.Explicit {
		t.Errorf("claude/plan-writer model = %+v, want the untouched built-in", pw.Model)
	}
	if pw.Effort.Value != "high" || pw.Effort.Provenance.Layer != LayerBuiltIn || pw.Effort.Explicit {
		t.Errorf("claude/plan-writer effort = %+v, want the untouched built-in", pw.Effort)
	}

	for _, h := range []string{"claude", "codex", "cursor", "opencode"} {
		got := res.effective.Agents[h]["build-max"].CacheTTL
		if got.Value != "5m" || got.Provenance.Layer != LayerGlobal || !got.Explicit {
			t.Errorf("%s/build-max cache_ttl = %+v, want the global default-block 5m", h, got)
		}
	}

	untouched := res.effective.Agents["codex"]["adr"].CacheTTL
	if untouched.Value != "" || untouched.Explicit || untouched.Provenance.Layer != LayerBuiltIn {
		t.Errorf("codex/adr cache_ttl = %+v, want the empty built-in (no pin)", untouched)
	}
}

// `cache_ttl: auto` removes the pin, exactly as `effort: auto` does.
func TestAgentsCacheTTLAuto(t *testing.T) {
	res := mustResolve(t, []Source{srcG("agents:\n  claude:\n    plan-writer:\n      cache_ttl: auto\n")}, mainCtx)
	got := res.effective.Agents["claude"]["plan-writer"].CacheTTL
	if got.Value != "" {
		t.Errorf("cache_ttl = %q, want \"\" (auto removes the pin)", got.Value)
	}
	if !got.Explicit || got.Provenance.Layer != LayerGlobal {
		t.Errorf("cache_ttl provenance = %+v (explicit %v), want an explicit global pin", got.Provenance, got.Explicit)
	}
}

// A repository-layer cache_ttl never reaches Effective; it reaches the classifier.
func TestAgentsCacheTTLRepoLayerExcludedFromEffective(t *testing.T) {
	for _, build := range []func(string) Source{srcR, srcL} {
		src := build("agents:\n  claude:\n    plan-writer:\n      cache_ttl: 1h\n")
		t.Run(string(src.Layer), func(t *testing.T) {
			res := mustResolve(t, []Source{src}, mainCtx)
			got := res.effective.Agents["claude"]["plan-writer"].CacheTTL
			if got.Value != "" || got.Explicit || got.Provenance.Layer != LayerBuiltIn {
				t.Errorf("claude/plan-writer cache_ttl = %+v, want the untouched built-in", got)
			}
			if d, ok := res.declared["agents.claude.plan-writer.cache_ttl"]; !ok || d.prov.Layer != src.Layer {
				t.Errorf("the declaration must still reach the classifier; declared = %v", res.declared)
			}
		})
	}
}
```

In `internal/config/capability_test.go`, inside the table under the `// Row 32 — agent pins are layer-dependent.` comment, add these two rows after the existing `"a repository-local agent effort pin blocks"` row:

```go
		{
			name:    "a global agent cache_ttl pin is supported",
			sources: []Source{srcG("agents:\n  claude:\n    plan-writer:\n      cache_ttl: 1h\n")},
		},
		{
			name:     "a repository agent cache_ttl pin blocks",
			sources:  []Source{srcR("agents:\n  claude:\n    plan-writer:\n      cache_ttl: 1h\n")},
			want:     []capWant{{"agents.claude.plan-writer.cache_ttl", Deferred, true, true, LayerRepository}},
			blockers: []string{"agents.claude.plan-writer.cache_ttl"},
		},
```

In `internal/config/schema_test.go`:
- In the registry-paths `want` list, change `"agents.*.*.model", "agents.*.*.effort", "agents.*.*.runner",` to `"agents.*.*.model", "agents.*.*.effort", "agents.*.*.cache_ttl", "agents.*.*.runner",`.
- In the leaf-validator table under `// agents/runners leaves: opaque but space-free.`, add after the `"effort auto is opaque"` row:

```go
		{"cache_ttl opaque", "agents.*.*.cache_ttl", "1h", "1h", ""},
		{"cache_ttl auto is opaque", "agents.*.*.cache_ttl", "auto", "auto", ""},
		{"cache_ttl with space", "agents.*.*.cache_ttl", `"1 h"`, nil, CodeInvalidValue},
		{"cache_ttl empty", "agents.*.*.cache_ttl", `""`, nil, CodeInvalidValue},
```

In `internal/config/setting_paths_test.go`, change the supported `want` entry `"agents.*.*.model", "agents.*.*.effort",` to `"agents.*.*.model", "agents.*.*.effort", "agents.*.*.cache_ttl",`.

In `internal/config/decode_test.go`, add after the `agents.*.*.effort` row:

```go
		{row: "agents.*.*.cache_ttl", path: "agents.claude.implement-next.cache_ttl",
			block: "agents:\n  claude:\n    implement-next:\n      cache_ttl: 1h\n",
			flow:  "agents: {claude: {implement-next: {cache_ttl: 1h}}}\n", value: "1h"},
```

In `internal/config/defaults_test.go` `TestBuiltinAgentsShape`, extend the provenance and explicit checks to cover the new field:

```go
			if entry.Model.Provenance != builtinProvenance() || entry.Effort.Provenance != builtinProvenance() ||
				entry.CacheTTL.Provenance != builtinProvenance() {
				t.Errorf("%s/%s: provenance = %+v/%+v/%+v, want %+v",
					harness, name, entry.Model.Provenance, entry.Effort.Provenance, entry.CacheTTL.Provenance, builtinProvenance())
			}
			if entry.Model.Explicit || entry.Effort.Explicit || entry.CacheTTL.Explicit {
				t.Errorf("%s/%s: Explicit must be false on built-in defaults", harness, name)
			}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/ -count=1`
Expected: FAIL to compile with `entry.CacheTTL undefined` / `pw.CacheTTL undefined (type AgentSetting has no field or method CacheTTL)`.

- [ ] **Step 3: Implement**

`internal/config/config.go`, replace the `AgentSetting` type and its table comment:

```go
// AgentsTable: harness → agent short name → resolved model/effort/cache TTL.
// Resolved from built-in + GLOBAL layers only; repository-layer agent
// declarations are deferred-capability requests and never land here.
type AgentsTable map[string]map[string]AgentSetting

type AgentSetting struct {
	Model  Value[string] `json:"model"`
	Effort Value[string] `json:"effort"` // "" == effort pin suppressed (`effort: auto`)
	// CacheTTL is the agent's prompt-cache TTL pin. Only Claude Code has the
	// setting (it renders as `experimental.cacheTtl`); "" == no pin (unset, or
	// `cache_ttl: auto`).
	CacheTTL Value[string] `json:"cache_ttl"`
}
```

`internal/config/schema.go`:
- Update the disposition comment: `dispAgentsLeaf // agents.<h>.<a>.model/effort/cache_ttl: layer-dependent`.
- In the `dispositionSupported` doc comment, change "the agents model/effort leaves" to "the agents model/effort/cache_ttl leaves".
- Change the comment `// 32-33: agent pins.` block so it reads `// 32-34: agent pins.` and insert the new row between the effort and runner rows:

```go
		{path: "agents.*.*.effort", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispAgentsLeaf, validate: stringLeaf(true, true, false)},
		// The prompt-cache TTL pin (`1h`, `5m`, ...) is opaque passthrough like
		// the model and effort beside it; only the Claude adapter renders it.
		{path: "agents.*.*.cache_ttl", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispAgentsLeaf, validate: stringLeaf(true, true, false)},
		{path: "agents.*.*.runner", kind: kindString, merge: mergeScalar, scope: scopeAny,
```

(If any later comment numbers rows, e.g. `// 34-38: runner settings`, renumber it by one so the comments stay consistent. Use `grep -n "// [0-9]" internal/config/schema.go` to find them.)

`internal/config/resolve.go`, in `resolveAgents`, after the effort block and before `row[name] = setting`:

```go
				if decl, ok := agentDecl(declared, harness, name, "cache_ttl"); ok {
					if ttl, ok := decl.value.(string); ok {
						// `auto` removes the pin: the harness keeps its own TTL.
						if ttl == autoSentinel {
							ttl = ""
						}
						setting.CacheTTL = Value[string]{Value: ttl, Provenance: decl.prov, Explicit: true}
					}
				}
```

Also update the `resolveAgents` doc comment's "Model and effort resolve independently" to "Model, effort, and cache_ttl resolve independently".

`internal/config/defaults.go`, in `builtinAgents`, give every entry an empty built-in TTL (Task 2 adds the shipped value):

```go
			row[name] = AgentSetting{
				Model:    builtinValue(def.model),
				Effort:   builtinValue(effort),
				CacheTTL: builtinValue(""),
			}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config/ -count=1`
Expected: PASS. If `TestExampleSchemaCorrespondence` fails on Direction B (supported → documented), the `agents.*.*.cache_ttl` path is supposed to be covered by the documented `agents:` ancestor block. Read the failure. If it names `cache_ttl`, fix the cause in the guard's own terms, since Task 2 documents the key in the example anyway. Do not weaken the guard.

- [ ] **Step 5: Mutation check**

Temporarily delete the `if ttl == autoSentinel { ttl = "" }` lines. Run `go test ./internal/config/ -run 'TestAgentsCacheTTLAuto' -count=1` and confirm it FAILS, then restore the lines by hand (do not use `git checkout --`, which would discard the uncommitted work). Run the whole package again: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/schema.go internal/config/resolve.go internal/config/defaults.go internal/config/resolve_test.go internal/config/capability_test.go internal/config/schema_test.go internal/config/setting_paths_test.go internal/config/decode_test.go internal/config/defaults_test.go
git commit -m "feat(config): add the per-agent cache_ttl pin"
```

---

### Task 2: Ship `cache_ttl: 1h` for claude implement-next (sidecar, compiled table, frozen tree, example)

**Files:**
- Modify: `agents/harness-defaults.yml` (header shape rules; claude `implement-next` row)
- Create: `testdata/repositories/v0.9.13/agents-harness-defaults.yml` (byte copy of the edited live sidecar)
- Create: `testdata/repositories/v0.9.13/PROVENANCE.md`
- Modify: `internal/config/defaults.go` (file header comment; new `builtinAgentCacheTTL`; `builtinAgents`)
- Modify: `internal/config/defaults_test.go` (`sidecarPath`, `sidecarDoc`, `pair`, parity test, new shipped-default test)
- Modify: `internal/config/example_correspondence_test.go` (`agentsAgentRowRe`, `exampleAgentsTable`, `builtinAgentPairs`, mutation probe)
- Modify: `.docket.example.yml` (agents comment block and the claude `implement-next` row)
- Modify: `internal/repoguard/absence_test.go` (the comment naming the frozen sidecar as `currently v0.9.11`)
- Regenerate: `internal/assets/embedded/` via `go generate ./internal/assets/`
- Test: `internal/config/defaults_test.go`, `internal/config/resolve_test.go`, `internal/config/example_correspondence_test.go`

**Interfaces:**
- Consumes: `AgentSetting.CacheTTL` and the `agents.*.*.cache_ttl` registry row from Task 1.
- Produces: `builtinAgents()["claude"]["implement-next"].CacheTTL.Value == "1h"`, and `""` for every other harness and agent. Test-local `pair` becomes `struct{ Model, Effort, CacheTTL string }`, shared by `defaults_test.go` and `example_correspondence_test.go` (same package).

- [ ] **Step 1: Write the failing tests**

In `internal/config/defaults_test.go`:

1. Point the parity oracle at the new tree: `sidecarPath = "../../testdata/repositories/v0.9.13/agents-harness-defaults.yml"`.
2. Extend the sidecar shape and the pair:

```go
// sidecarDoc is the shipped-defaults file's shape: harness → agent short name
// → model/effort pair, plus the optional prompt-cache TTL.
type sidecarDoc struct {
	Agents map[string]map[string]struct {
		Model    string `yaml:"model"`
		Effort   string `yaml:"effort"`
		CacheTTL string `yaml:"cache_ttl"`
	} `yaml:"agents"`
}

// pair is a (model, effort, cache TTL) tuple compared structurally, so a
// missing or extra harness/agent shows up as a difference rather than being
// skipped.
type pair struct{ Model, Effort, CacheTTL string }
```

3. In `TestBuiltinAgentsParityWithFrozenSidecar`, build `want` with `row[name] = pair{Model: entry.Model, Effort: effort, CacheTTL: entry.CacheTTL}` and `got` with `row[name] = pair{Model: entry.Model.Value, Effort: entry.Effort.Value, CacheTTL: entry.CacheTTL.Value}`.
4. Add:

```go
// TestBuiltinAgentsCacheTTLShippedOnImplementNextOnly pins the shipped
// prompt-cache TTL scope: claude's implement-next carries 1h, and no other
// harness or agent carries any TTL. The 1-hour write costs more than the
// 5-minute one, so a TTL on an agent that rarely outlasts five minutes between
// turns is a pure surcharge.
func TestBuiltinAgentsCacheTTLShippedOnImplementNextOnly(t *testing.T) {
	for harness, agents := range builtinAgents() {
		for name, entry := range agents {
			want := ""
			if harness == "claude" && name == "implement-next" {
				want = "1h"
			}
			if entry.CacheTTL.Value != want {
				t.Errorf("%s/%s: built-in cache_ttl = %q, want %q", harness, name, entry.CacheTTL.Value, want)
			}
		}
	}
}
```

Append to `internal/config/resolve_test.go`:

```go
// The shipped implement-next TTL obeys the same per-field layering as model and
// effort: a model-only override keeps it, and a global `auto` or `5m` replaces it.
func TestAgentsShippedCacheTTLLayering(t *testing.T) {
	res := mustResolve(t, []Source{srcG("agents:\n  claude:\n    implement-next:\n      model: claude-sonnet-5\n")}, mainCtx)
	got := res.effective.Agents["claude"]["implement-next"]
	if got.Model.Value != "claude-sonnet-5" {
		t.Fatalf("model = %q, want the global override", got.Model.Value)
	}
	if got.CacheTTL.Value != "1h" || got.CacheTTL.Provenance.Layer != LayerBuiltIn || got.CacheTTL.Explicit {
		t.Errorf("cache_ttl = %+v, want the built-in 1h kept by a model-only override", got.CacheTTL)
	}

	for _, tc := range []struct{ in, want string }{{"auto", ""}, {"5m", "5m"}} {
		res := mustResolve(t, []Source{srcG("agents:\n  claude:\n    implement-next:\n      cache_ttl: " + tc.in + "\n")}, mainCtx)
		got := res.effective.Agents["claude"]["implement-next"].CacheTTL
		if got.Value != tc.want || !got.Explicit || got.Provenance.Layer != LayerGlobal {
			t.Errorf("cache_ttl %s: got %+v, want explicit global %q", tc.in, got, tc.want)
		}
	}
}
```

In `internal/config/example_correspondence_test.go`:

1. Replace `agentsAgentRowRe` so a row may carry an optional trailing `cache_ttl`:

```go
	agentsAgentRowRe   = regexp.MustCompile(`^#[ \t]{5}([A-Za-z0-9_-]+):[ \t]*\{[ \t]*model:[ \t]*([^,}\s]+)[ \t]*,[ \t]*effort:[ \t]*([^,}\s]+)[ \t]*(?:,[ \t]*cache_ttl:[ \t]*([^,}\s]+)[ \t]*)?\}[ \t]*$`)
```

2. In `exampleAgentsTable`, store `table[harness][m[1]] = pair{Model: m[2], Effort: effort, CacheTTL: m[4]}`, and update its doc comment to say it parses `harness -> agent -> (model, effort, cache_ttl)`.
3. In `builtinAgentPairs`, use `row[name] = pair{Model: a.Model.Value, Effort: a.Effort.Value, CacheTTL: a.CacheTTL.Value}`.
4. In the `commented_agents_table` subtest, after the existing model-mutation probe, add:

```go
		// Mutation: dropping the shipped cache_ttl from the example must redden.
		dropped := strings.Replace(content, ", cache_ttl: 1h }", " }", 1)
		if dropped == content {
			t.Fatalf("cache_ttl mutation probe did not change the example")
		}
		if len(check(dropped)) == 0 {
			t.Errorf("a missing cache_ttl in the commented agents table was not caught")
		}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/ -count=1`
Expected: FAIL. `TestBuiltinAgentsParityWithFrozenSidecar` fails reading the missing `v0.9.13` file. `TestBuiltinAgentsCacheTTLShippedOnImplementNextOnly` and `TestAgentsShippedCacheTTLLayering` fail with `claude/implement-next: built-in cache_ttl = "", want "1h"`. The example probe fails with "cache_ttl mutation probe did not change the example".

- [ ] **Step 3: Implement**

`agents/harness-defaults.yml`:
- In the header's `# Shape rules:` list, after `#   - every listed entry supplies both model and effort;`, add:

```yaml
#   - an entry may add an optional `cache_ttl` (a prompt-cache TTL; only Claude Code has the
#     setting). It ships on claude's implement-next alone; see the comment on that row;
```

- Also change the file's first line from `docket's shipped per-harness agent model/effort table.` to `docket's shipped per-harness agent model/effort/cache-TTL table.`
- Replace the claude `implement-next` row with:

```yaml
    # implement-next waits on build workers and the test suite for longer than Claude Code's
    # 5-minute subagent prompt cache lasts, so most of its cost went to writing its whole context
    # into the cache again after each wait. A 1-hour TTL roughly halves its cost per run. The
    # other agents rarely wait five minutes between turns, and a 1-hour cache write costs more
    # than a 5-minute one, so they keep Claude Code's default.
    implement-next:        { model: claude-opus-5, effort: medium, cache_ttl: 1h }
```

Cut the new frozen tree from the edited live file:

```bash
mkdir -p testdata/repositories/v0.9.13
cp agents/harness-defaults.yml testdata/repositories/v0.9.13/agents-harness-defaults.yml
```

Create `testdata/repositories/v0.9.13/PROVENANCE.md`:

```markdown
# v0.9.13 agent-defaults sidecar

- **Source repo:** `danielhanold/docket`
- **Commit:** the commit that adds this tree (the tree equals `agents/harness-defaults.yml` there)
- **Date:** 2026-10-11
- **Redaction:** none

Source: docket's own `agents/harness-defaults.yml` as of the change that adds the per-agent
prompt-cache TTL pin, 2026-10-11.

Cut from `v0.9.11/agents-harness-defaults.yml`. The ONLY file in this tree is
`agents-harness-defaults.yml`, a byte-exact copy of `agents/harness-defaults.yml` at that change.
It differs from the v0.9.11 sidecar in one value and its comments: the claude `implement-next` row
gains `cache_ttl: 1h` with a comment explaining why, and the header documents the optional
`cache_ttl` field. Every model and effort is unchanged. The Go built-in table in
`internal/config/defaults.go` gains the matching `builtinAgentCacheTTL` entry, and only the
agent-defaults parity oracle's `sidecarPath` advances to this tree. Older versioned fixture trees
(v0.9.2–v0.9.12) are immutable inputs and are never edited in place.
```

`internal/config/defaults.go`:
- In the file header comment, replace `its byte-exact copy under \`testdata/repositories/v0.9.11/\` (re-cut comment-only) is the parity oracle.` with `its byte-exact copy under \`testdata/repositories/v0.9.13/\` is the parity oracle.`
- After `builtinAgentDefaults`, add:

```go
// builtinAgentCacheTTL is the shipped prompt-cache TTL pins, written the way
// `agents/harness-defaults.yml` writes its optional `cache_ttl` field: harness →
// agent short name → TTL token. An agent absent here carries no TTL pin. It is a
// separate table, not a third agentDefault column, because only one entry
// carries the field.
var builtinAgentCacheTTL = map[string]map[string]string{
	"claude": {"implement-next": "1h"},
}
```

- In `builtinAgents`, replace `CacheTTL: builtinValue(""),` with `CacheTTL: builtinValue(builtinAgentCacheTTL[harness][name]),`. Update the function's doc comment to say "with `effort: auto` suppressed to "" and the shipped cache TTLs applied".

`.docket.example.yml`, in the `# agents —` comment block:
- After the sentence ending `` `effort: auto` writes no effort line, so the harness picks.``, insert:

```yaml
# An entry may also set `cache_ttl`, how long Claude Code keeps that agent's prompt cache
# (`1h`, `5m`). The built-in table sets `1h` on claude's implement-next only: it waits on builds
# and tests longer than the 5-minute default, and the longer TTL roughly halves its cost.
# `cache_ttl: auto` removes the pin. Cursor, Codex, and opencode have no such setting, so install
# leaves it out of their agent files and warns.
```

- Change the claude row to: `#     implement-next:        { model: claude-opus-5,             effort: medium, cache_ttl: 1h }`

`internal/repoguard/absence_test.go`: in the comment block, change `TestBuiltinAgentsParityWithFrozenSidecar's frozen sidecar (currently v0.9.11)` to `(currently v0.9.13)`.

Regenerate the embedded twins:

```bash
go generate ./internal/assets/
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config/ ./internal/assets/ ./internal/repoguard/ -count=1`
Expected: PASS. `TestEmbeddedMatchesAuthored` passes because the embedded twins were regenerated. If a repoguard prose or anchor test fails on the new comments, fix the wording rather than the guard.

Then confirm the frozen copy is byte-identical: `cmp agents/harness-defaults.yml testdata/repositories/v0.9.13/agents-harness-defaults.yml && echo same`. Expected: `same`.

- [ ] **Step 5: Mutation check**

Temporarily change `builtinAgentCacheTTL` to `{"claude": {"implement-next": "1h", "plan-writer": "1h"}}`. Run `go test ./internal/config/ -run 'TestBuiltinAgentsCacheTTLShippedOnImplementNextOnly|TestBuiltinAgentsParityWithFrozenSidecar|TestExampleSchemaCorrespondence' -count=1`: all three FAIL. Restore by hand and re-run: PASS.

- [ ] **Step 6: Commit**

```bash
git add agents/harness-defaults.yml testdata/repositories/v0.9.13 internal/config/defaults.go internal/config/defaults_test.go internal/config/resolve_test.go internal/config/example_correspondence_test.go .docket.example.yml internal/repoguard/absence_test.go internal/assets/embedded
git commit -m "feat(agents): ship a 1h prompt-cache TTL for claude implement-next"
```

---

### Task 3: Claude adapter renders `experimental.cacheTtl`; other adapters stay unchanged

**Files:**
- Modify: `internal/harness/inventory.go` (new func `ResolvedCacheTTL`, beside `ResolvedAgentRaw`)
- Modify: `internal/harness/claude/claude.go` (`Plan`, `renderAgent`, `verifyRoundTrip`, and the `renderAgent` field-mapping doc comment)
- Modify: `internal/harness/claude/claude_test.go` (`fixtureAgents`, `TestClaudeAgentPinCases`, new hostile-value test)
- Regenerate: `internal/harness/claude/testdata/golden/docket-implement-next.md` (via `-update`; it must be the ONLY golden that changes)
- Test: `internal/harness/cross_harness_test.go` (new test)

**Interfaces:**
- Consumes: `config.AgentSetting.CacheTTL` (Task 1).
- Produces: `func ResolvedCacheTTL(t config.AgentsTable, harnessName, shortName string) string` in package `harness`; unexported `renderAgent(s harness.AgentSource, model, effort, cacheTTL string) ([]byte, error)` and `verifyRoundTrip(rendered []byte, s harness.AgentSource, model, effort, cacheTTL string) error` in package `claude`.

- [ ] **Step 1: Write the failing tests**

In `internal/harness/claude/claude_test.go` `fixtureAgents()`, add inside the `"claude"` map:

```go
			// A prompt-cache TTL rides beside model and effort and renders under
			// Claude Code's `experimental` map.
			"implement-next": {
				Model:    config.Value[string]{Value: "claude-opus-5"},
				Effort:   config.Value[string]{Value: "medium"},
				CacheTTL: config.Value[string]{Value: "1h"},
			},
```

and give the sibling codex row a TTL so a leak would show:

```go
			"build-standard": {
				Model:    config.Value[string]{Value: "gpt-5.5-codex"},
				Effort:   config.Value[string]{Value: "low"},
				CacheTTL: config.Value[string]{Value: "9h"},
			},
```

Update the `fixtureAgents` doc comment so it lists the TTL-pinned agent as a fifth case.

Rewrite `TestClaudeAgentPinCases` to cover the TTL. The new struct fields are `ttl string; wantTTLSet bool`. The frontmatter decode struct gains:

```go
				Experimental struct {
					CacheTTL string `yaml:"cacheTtl"`
				} `yaml:"experimental"`
```

The cases become:

```go
		{"docket-build-standard.md", "claude-opus-5[1m]", "high", "", true, true, false},
		{"docket-status.md", "claude-sonnet-4-6", "", "", true, false, false},
		{"docket-adr.md", "inherit", "", "", true, false, false},
		{"docket-review-deep.md", "inherit", "medium", "", true, true, false},
		{"docket-implement-next.md", "claude-opus-5", "medium", "1h", true, true, true},
		{"docket-review-lean.md", "", "", "", false, false, false},
		{"docket-brainstorm-consultant.md", "", "", "", false, false, false},
```

The struct field order is `file, model, efrt, ttl string; wantModelSet, wantEfrtSet, wantTTLSet bool`. Add these asserts in the subtest body:

```go
			if fm.Experimental.CacheTTL != tc.ttl {
				t.Errorf("experimental.cacheTtl = %q, want %q", fm.Experimental.CacheTTL, tc.ttl)
			}
			if _, has := doc.Field("experimental"); has != tc.wantTTLSet {
				t.Errorf("experimental key present = %v, want %v", has, tc.wantTTLSet)
			}
			if _, has := doc.Field("cacheTtl"); has {
				t.Errorf("cacheTtl must sit under experimental, never at the top level")
			}
```

Add a hostile-value test in `claude_test.go`:

```go
// A cache TTL that would not read back as itself is refused, never shipped:
// the round-trip check covers the new field exactly as it covers model and effort.
func TestClaudeCacheTTLRoundTripRefusal(t *testing.T) {
	in := fixtureInput(t)
	sources, err := harness.ParseInventory(in.Assets)
	if err != nil || len(sources) == 0 {
		t.Fatalf("ParseInventory: %v (%d sources)", err, len(sources))
	}
	for _, bad := range []string{"#5m", "[1h]", "{a: b}"} {
		if _, err := renderAgent(sources[0], "claude-opus-5", "low", bad); !errors.Is(err, ErrRender) {
			t.Errorf("cacheTTL %q: err = %v, want ErrRender", bad, err)
		}
	}
	// The round-trip check alone also refuses a document whose TTL disagrees.
	good, err := renderAgent(sources[0], "claude-opus-5", "low", "1h")
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
	}
	if err := verifyRoundTrip(good, sources[0], "claude-opus-5", "low", "5m"); !errors.Is(err, ErrRender) {
		t.Errorf("verifyRoundTrip accepted a mismatched cacheTtl: %v", err)
	}
}
```

Add `"errors"` to the test file's imports if it is not already there.

In `internal/harness/cross_harness_test.go`, add:

```go
// TestCacheTTLRendersOnClaudeOnly: a prompt-cache TTL pinned on every harness
// changes exactly one file, claude's wrapper for that agent. Cursor, Codex, and
// opencode have no such setting, so their plans are byte-identical with and
// without the pin, and no payload of theirs names the field.
func TestCacheTTLRendersOnClaudeOnly(t *testing.T) {
	base := crossPlanInput(t)
	pinned := crossPlanInput(t)
	for _, h := range harness.Order {
		s := pinned.Agents[h]["build-standard"]
		s.CacheTTL = config.Value[string]{Value: "1h"}
		pinned.Agents[h]["build-standard"] = s
	}
	adapters := map[string]harness.Adapter{
		"claude": claude.New(), "codex": codex.New(), "cursor": cursor.New(), "opencode": opencode.New(),
	}
	for _, h := range harness.Order {
		before, err := adapters[h].Plan(base)
		if err != nil {
			t.Fatalf("%s: Plan(base): %v", h, err)
		}
		after, err := adapters[h].Plan(pinned)
		if err != nil {
			t.Fatalf("%s: Plan(pinned): %v", h, err)
		}
		if len(before) != len(after) {
			t.Fatalf("%s: target count moved %d -> %d", h, len(before), len(after))
		}
		var changed []string
		for i := range after {
			if after[i].Path != before[i].Path {
				t.Fatalf("%s: target order moved at %d: %s vs %s", h, i, before[i].Path, after[i].Path)
			}
			if string(after[i].Content) != string(before[i].Content) {
				changed = append(changed, filepath.Base(after[i].Path))
			}
			if h != "claude" {
				c := string(after[i].Content)
				if strings.Contains(c, "cacheTtl") || strings.Contains(c, "cache_ttl") {
					t.Errorf("%s: %s names the cache TTL field", h, after[i].Path)
				}
			}
		}
		if h == "claude" {
			if len(changed) != 1 || changed[0] != "docket-build-standard.md" {
				t.Fatalf("claude: changed files = %v, want only docket-build-standard.md", changed)
			}
			for _, tg := range after {
				if filepath.Base(tg.Path) == "docket-build-standard.md" &&
					!strings.Contains(string(tg.Content), "\nexperimental:\n  cacheTtl: 1h\n") {
					t.Errorf("claude wrapper lacks the experimental.cacheTtl block:\n%s", tg.Content)
				}
			}
		} else if len(changed) != 0 {
			t.Errorf("%s: a cache TTL pin changed %v; this harness has no such setting", h, changed)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/harness/... -count=1`
Expected: FAIL to compile with "too many arguments in call to renderAgent", or with the `docket-implement-next.md` pin case failing on `experimental.cacheTtl = "", want "1h"`, and `TestCacheTTLRendersOnClaudeOnly` reporting `claude: changed files = [], want only docket-build-standard.md`.

- [ ] **Step 3: Implement**

`internal/harness/inventory.go`, after `ResolvedAgent`:

```go
// ResolvedCacheTTL returns the prompt-cache TTL pin a harness should render
// for one agent short name, or "" when the agent carries none (config already
// folds `cache_ttl: auto` to ""). Only the claude adapter reads it: Claude Code
// is the one harness with a per-agent cache TTL setting, and the other
// adapters drop the field simply by never asking for it.
func ResolvedCacheTTL(t config.AgentsTable, harnessName, shortName string) string {
	return t[harnessName][shortName].CacheTTL.Value
}
```

`internal/harness/claude/claude.go`:

In `Plan`:

```go
		model, effort := harness.ResolvedAgentRaw(in.Agents, Name, s.ShortName)
		cacheTTL := harness.ResolvedCacheTTL(in.Agents, Name, s.ShortName)
		content, err := renderAgent(s, model, effort, cacheTTL)
```

In the `renderAgent` doc comment's field mapping, add the row `//	resolved cache TTL       -> experimental.cacheTtl (omitted when unpinned)` after the effort row.

Change the signature to `func renderAgent(s harness.AgentSource, model, effort, cacheTTL string) ([]byte, error)` and, after the `effort` write:

```go
	// Claude Code reads a subagent's prompt-cache TTL from the `experimental`
	// map, never from a top-level key. The value is opaque passthrough like the
	// model and effort, and the round-trip check below guards it the same way.
	if cacheTTL != "" {
		b.WriteString("experimental:\n  cacheTtl: " + cacheTTL + "\n")
	}
```

Change the call to `verifyRoundTrip(out, s, model, effort, cacheTTL)` and update `verifyRoundTrip`:

```go
func verifyRoundTrip(rendered []byte, s harness.AgentSource, model, effort, cacheTTL string) error {
	doc, err := document.Parse(rendered)
	if err != nil {
		return fmt.Errorf("%w: %s does not reparse: %w", ErrRender, s.Name, err)
	}
	var fm struct {
		Name         string   `yaml:"name"`
		Description  string   `yaml:"description"`
		Skills       []string `yaml:"skills"`
		Model        string   `yaml:"model"`
		Effort       string   `yaml:"effort"`
		Experimental struct {
			CacheTTL string `yaml:"cacheTtl"`
		} `yaml:"experimental"`
	}
	if err := doc.DecodeFrontmatter(&fm); err != nil {
		return fmt.Errorf("%w: %s does not decode: %w", ErrRender, s.Name, err)
	}
	switch {
	case fm.Name != s.Name:
		return fmt.Errorf("%w: %s round-trips its name as %q", ErrRender, s.Name, fm.Name)
	case fm.Description != s.Description:
		return fmt.Errorf("%w: %s round-trips its description as %q", ErrRender, s.Name, fm.Description)
	case fm.Model != model:
		return fmt.Errorf("%w: %s round-trips model %q as %q", ErrRender, s.Name, model, fm.Model)
	case fm.Effort != effort:
		return fmt.Errorf("%w: %s round-trips effort %q as %q", ErrRender, s.Name, effort, fm.Effort)
	case fm.Experimental.CacheTTL != cacheTTL:
		return fmt.Errorf("%w: %s round-trips cacheTtl %q as %q", ErrRender, s.Name, cacheTTL, fm.Experimental.CacheTTL)
	case len(s.Skills) > 0 && !reflect.DeepEqual(fm.Skills, s.Skills):
		return fmt.Errorf("%w: %s round-trips skills %v as %v", ErrRender, s.Name, s.Skills, fm.Skills)
	}
	return nil
}
```

Find every other caller with `grep -n "renderAgent(\|verifyRoundTrip(" internal/harness/claude/*.go` and pass `""` as the new last argument wherever an existing test calls them with the old arity.

Regenerate the goldens and prove only one moved:

```bash
go test ./internal/harness/claude/ -run TestClaudeGoldenAgents -update -count=1
git status --porcelain internal/harness/claude/testdata/golden
```

Expected: exactly one line, ` M internal/harness/claude/testdata/golden/docket-implement-next.md`. If any other golden changed, the renderer changed the no-TTL output, which violates "unset is byte-identical". Fix the renderer, not the golden. Read the regenerated golden. Its frontmatter must end with:

```yaml
model: claude-opus-5
effort: medium
experimental:
  cacheTtl: 1h
---
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/harness/... -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation check**

Temporarily delete the `if cacheTTL != "" { b.WriteString(...) }` block from `renderAgent`. Run `go test ./internal/harness/... -count=1`. Expected: FAIL. With the write gone, the round-trip check refuses the plan, so the golden, pin-case, and cross-harness tests go red. Then also comment out the `fm.Experimental.CacheTTL != cacheTTL` case and re-run. Expected: still FAIL (the golden and pin case catch the missing block). Restore both by hand and re-run: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/harness/inventory.go internal/harness/claude/claude.go internal/harness/claude/claude_test.go internal/harness/claude/testdata/golden/docket-implement-next.md internal/harness/cross_harness_test.go
git commit -m "feat(harness): render the claude cache TTL as experimental.cacheTtl"
```

---

### Task 4: Install warns on a dropped `cache_ttl`; the agent digest includes it

**Files:**
- Modify: `internal/app/finding_codes.go` (new `FCAgentCacheTTLDropped` constant; `AllFindingCodes`)
- Modify: `internal/app/install.go` (new `CacheTTLDropWarnings`; `agentDigestRow`; `AgentDigest`)
- Modify: `internal/cli/install.go` (func `installOptions`)
- Test: `internal/app/install_test.go`, new `internal/cli/install_cachettl_test.go`

**Interfaces:**
- Consumes: `config.AgentSetting.CacheTTL` (Task 1); the shipped default (Task 2); the claude render (Task 3, for the end-to-end assert); `claude.Name` (already imported in `internal/app/install.go`).
- Produces: `FCAgentCacheTTLDropped FindingCode = "agent-cache-ttl-dropped"`; `func CacheTTLDropWarnings(agents config.AgentsTable, harnesses []string) []config.Diagnostic`; `agentDigestRow.CacheTTL string` with JSON tag `cache_ttl,omitempty`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/install_test.go`:

```go
// A cache TTL resolved for a harness with no such setting is a warning per
// harness and agent, sorted, naming the field. Claude is never warned, nothing
// is warned for an unselected harness, and a table with no TTL warns nothing.
func TestCacheTTLDropWarnings(t *testing.T) {
	ttl := func(v string) config.AgentSetting { return config.AgentSetting{CacheTTL: config.Value[string]{Value: v}} }
	agents := config.AgentsTable{
		"claude":   {"implement-next": ttl("1h")},
		"codex":    {"status": ttl("1h"), "adr": ttl("5m"), "build-max": ttl("")},
		"cursor":   {"status": ttl("1h")},
		"opencode": {"status": ttl("1h")},
	}
	got := CacheTTLDropWarnings(agents, []string{"claude", "codex", "cursor"})
	var paths []string
	for _, d := range got {
		paths = append(paths, d.Path)
		if d.Code != string(FCAgentCacheTTLDropped) || d.Severity != config.SeverityWarning || d.Message == "" || d.Remedy == "" {
			t.Errorf("warning = %+v", d)
		}
	}
	want := []string{"agents.codex.adr.cache_ttl", "agents.codex.status.cache_ttl", "agents.cursor.status.cache_ttl"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("warning paths = %v, want %v", paths, want)
	}
	if !strings.Contains(got[0].Message, "codex") || !strings.Contains(got[0].Message, "adr") || !strings.Contains(got[0].Message, "5m") {
		t.Errorf("message does not name the harness, agent, and value: %q", got[0].Message)
	}
	if w := CacheTTLDropWarnings(config.AgentsTable{"codex": {"status": ttl("")}}, []string{"codex"}); len(w) != 0 {
		t.Errorf("an empty TTL warned: %+v", w)
	}
}

// The digest moves when only a cache TTL changes, so `install check` sees the
// rendered wrapper as stale, and a table with no TTL digests exactly as the
// model/effort-only encoding did, so installs without one are not reported as
// drifted.
func TestAgentDigestCoversCacheTTL(t *testing.T) {
	table := func(ttl string) config.AgentsTable {
		return config.AgentsTable{"claude": {"implement-next": {
			Model:    config.Value[string]{Value: "claude-opus-5"},
			Effort:   config.Value[string]{Value: "medium"},
			CacheTTL: config.Value[string]{Value: ttl},
		}}}
	}
	none, err := AgentDigest(table(""), []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	hour, err := AgentDigest(table("1h"), []string{"claude"})
	if err != nil || hour == none {
		t.Fatalf("digest ignored a cache TTL change: %q (%v)", hour, err)
	}
	legacy, err := json.Marshal(map[string]map[string]struct {
		Model  string `json:"model"`
		Effort string `json:"effort"`
	}{"claude": {"implement-next": {Model: "claude-opus-5", Effort: "medium"}}})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(legacy)
	if want := "sha256:" + hex.EncodeToString(sum[:]); none != want {
		t.Errorf("a TTL-less table digests %q, want the model/effort-only digest %q", none, want)
	}
}
```

Add any missing imports to `install_test.go` (`reflect`, `crypto/sha256`, `encoding/hex`, `encoding/json`). Check the existing import block first.

Create `internal/cli/install_cachettl_test.go`:

```go
package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/testsupport"
)

func writeGlobalConfig(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "docket")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A global cache_ttl that reaches a non-Claude harness surfaces as an install
// warning naming the harness and agent; the configuration never blocks.
func TestInstallOptionsWarnsOnDroppedCacheTTL(t *testing.T) {
	home := pinInstallEnv(t)
	t.Chdir(testsupport.TempDir(t))
	writeGlobalConfig(t, home, "agents:\n  default:\n    status:\n      cache_ttl: 1h\n")

	opts, refusal := installOptions(context.Background(), []string{"codex", "claude"}, "", false, devInfo())
	if refusal != nil {
		t.Fatalf("installOptions refused: %v", refusal)
	}
	var paths []string
	for _, w := range opts.ConfigWarnings {
		if w.Code == string(app.FCAgentCacheTTLDropped) {
			paths = append(paths, w.Path)
		}
	}
	if len(paths) != 1 || paths[0] != "agents.codex.status.cache_ttl" {
		t.Fatalf("cache TTL warnings = %v, want exactly agents.codex.status.cache_ttl", paths)
	}
}

// The shipped defaults alone print no cache TTL warning on any harness.
func TestInstallOptionsShippedDefaultsWarnNoCacheTTL(t *testing.T) {
	pinInstallEnv(t)
	t.Chdir(testsupport.TempDir(t))
	opts, refusal := installOptions(context.Background(), nil, "", false, devInfo())
	if refusal != nil {
		t.Fatalf("installOptions refused: %v", refusal)
	}
	for _, w := range opts.ConfigWarnings {
		if w.Code == string(app.FCAgentCacheTTLDropped) {
			t.Errorf("shipped defaults warned: %+v", w)
		}
	}
}

// End to end: a real install writes the shipped 1h TTL into claude's
// implement-next wrapper, and only there.
func TestInstallWritesShippedCacheTTLForImplementNext(t *testing.T) {
	home := pinInstallEnv(t)
	t.Chdir(testsupport.TempDir(t))
	if out, stderr, code := runCLI(t, "install", "--harness", "claude", "--json"); code != 0 {
		t.Fatalf("install: code=%d out=%s stderr=%s", code, out, stderr)
	}
	agentsDir := filepath.Join(home, ".claude", "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("ReadDir %s: %v (%d entries)", agentsDir, err, len(entries))
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		has := strings.Contains(string(b), "\nexperimental:\n  cacheTtl: 1h\n")
		if want := e.Name() == "docket-implement-next.md"; has != want {
			t.Errorf("%s: carries experimental.cacheTtl 1h = %v, want %v", e.Name(), has, want)
		}
	}
}
```

Before relying on `w.Code` and `w.Path`, confirm that `opts.ConfigWarnings` is `[]config.Diagnostic`. It is (`internal/install/service.go`, field `ConfigWarnings`), so `w.Code` and `w.Path` are direct fields.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestCacheTTLDropWarnings|TestAgentDigestCoversCacheTTL' -count=1` and `go test ./internal/cli/ -run 'TestInstallOptions.*CacheTTL|TestInstallWritesShippedCacheTTLForImplementNext' -count=1`
Expected: FAIL to compile with `undefined: CacheTTLDropWarnings` / `undefined: app.FCAgentCacheTTLDropped`. (`TestInstallWritesShippedCacheTTLForImplementNext` may already pass after Tasks 2–3. It is the end-to-end proof, not the red driver.)

- [ ] **Step 3: Implement**

`internal/app/finding_codes.go`: after the `FCHookFileNotEditable` constant (inside the same `const` block), add:

```go
	// agent-cache-ttl-dropped is the warning code an install, development
	// install, or install check surfaces when a resolved `cache_ttl` pin reaches
	// a harness with no prompt-cache TTL setting (Cursor, Codex, opencode). The
	// field is left out of that harness's agent file. A warning only: the
	// configuration never blocks.
	FCAgentCacheTTLDropped FindingCode = "agent-cache-ttl-dropped"
```

In `AllFindingCodes`, insert `FCAgentCacheTTLDropped,` between `FindingCode("adr-not-accepted"),` and `FindingCode("ambiguous-adr"),` (it sorts there: `adr` < `agent` < `ambiguous`).

`internal/app/install.go`, after `Planners` and before `planMode`:

```go
// cacheTTLHarnesses are the harnesses whose agent definitions have a per-agent
// prompt-cache TTL setting. Claude Code reads it from `experimental.cacheTtl`;
// Cursor, Codex, and opencode have no equivalent, so their adapters leave the
// field out.
var cacheTTLHarnesses = map[string]bool{claude.Name: true}

// CacheTTLDropWarnings reports every resolved `cache_ttl` pin a selected harness
// cannot express. The adapter drops the field; this warning keeps the drop loud
// rather than silent (ADR-0077's warn-and-drop posture). It keys on the
// RESOLVED value, not on which layer declared it, so a global `default:` pin
// that reaches a non-Claude harness warns too, while the shipped table (which
// pins a TTL on claude alone) warns nothing. Warnings are sorted by harness,
// then agent.
func CacheTTLDropWarnings(agents config.AgentsTable, harnesses []string) []config.Diagnostic {
	selected := append([]string(nil), harnesses...)
	sort.Strings(selected)
	var out []config.Diagnostic
	for _, h := range selected {
		if cacheTTLHarnesses[h] {
			continue
		}
		row := agents[h]
		names := make([]string, 0, len(row))
		for name, setting := range row {
			if setting.CacheTTL.Value != "" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			out = append(out, config.Diagnostic{
				Code:     string(FCAgentCacheTTLDropped),
				Severity: config.SeverityWarning,
				Path:     "agents." + h + "." + name + ".cache_ttl",
				Message: fmt.Sprintf("%s has no prompt-cache TTL setting, so cache_ttl %q for %s is left out of its agent file",
					h, row[name].CacheTTL.Value, name),
				Remedy: "set cache_ttl under agents.claude in the global config, not under agents.default or another harness",
			})
		}
	}
	return out
}
```

(`sort` and `fmt` are already imported in `internal/app/install.go`. Confirm with the import block and add them if missing.)

Update the digest:

```go
type agentDigestRow struct {
	Model    string `json:"model"`
	Effort   string `json:"effort"`
	CacheTTL string `json:"cache_ttl,omitempty"`
}
```

and in `AgentDigest`:

```go
			rendered[agentName] = agentDigestRow{
				Model:    setting.Model.Value,
				Effort:   setting.Effort.Value,
				CacheTTL: setting.CacheTTL.Value,
			}
```

Update the `AgentDigest` doc comment: `sha256 of the canonical JSON of {harness: {agent: {model, effort[, cache_ttl]}}}`. Add one sentence: `cache_ttl is omitted when empty, so a table with no TTL digests exactly as it did before the field existed.` Also update the `agentDigestRow` comment if it names only model and effort.

`internal/cli/install.go`, in `installOptions`, replace:

```go
	configWarnings := config.Warnings(diags)
	agents := snapshot.Effective.Agents
	digest, err := app.AgentDigest(agents, app.HarnessNames(harnesses))
```

with:

```go
	agents := snapshot.Effective.Agents
	selected := app.HarnessNames(harnesses)
	// A cache_ttl resolved for a harness with no such setting is dropped from
	// that harness's agent file; the warning rides the same channel as the
	// config read's own warnings, so install, check, and development install
	// all print it.
	configWarnings := append(config.Warnings(diags), app.CacheTTLDropWarnings(agents, selected)...)
	digest, err := app.AgentDigest(agents, selected)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ ./internal/cli/ -count=1`
Expected: PASS, including `TestFindingCodeRegistryIntegrity` and the AST completeness guard over finding codes. If the guard reports the new constant as unregistered, the `AllFindingCodes` insertion is missing or out of order.

- [ ] **Step 5: Mutation check**

(a) Temporarily make `CacheTTLDropWarnings` return `nil` immediately. `go test ./internal/app/ ./internal/cli/ -run 'CacheTTL' -count=1` must FAIL. (b) Temporarily remove `CacheTTL: setting.CacheTTL.Value,` from `AgentDigest`. `TestAgentDigestCoversCacheTTL` must FAIL. (c) Temporarily drop `,omitempty` from the digest tag. `TestAgentDigestCoversCacheTTL` must FAIL. Restore each by hand and re-run: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finding_codes.go internal/app/install.go internal/app/install_test.go internal/cli/install.go internal/cli/install_cachettl_test.go
git commit -m "feat(install): warn when a cache_ttl pin is dropped; digest it"
```

---

### Task 5: Document the key, the shipped default, and the controls docket does not use

**Files:**
- Modify: `skills/docket-convention/references/agent-layer.md` (new `### Prompt-cache TTL` section at the end of `## Model and effort pins`; one sentence under the `## Per-harness wrapper shapes` table)
- Modify: `docs/install/models-and-effort.md` (new paragraph after the **Changing only the model?** paragraph)
- Regenerate: `internal/assets/embedded/` via `go generate ./internal/assets/`
- Test: existing doc/prose guards (`internal/repoguard`, `internal/assets`), then the whole suite

**Interfaces:**
- Consumes: the behaviour delivered by Tasks 1–4 (key name, `auto`, shipped default, warning code).
- Produces: documentation only.

- [ ] **Step 1: Write the agent-layer section**

In `skills/docket-convention/references/agent-layer.md`, insert this immediately before `## Per-harness wrapper shapes` (the last paragraph of `## Model and effort pins` is the **Model IDs are opaque passthrough values (ADR-0015).** paragraph):

````markdown
### Prompt-cache TTL

An agent entry may carry a third, optional field, `cache_ttl`, which sets how long Claude Code keeps
that agent's prompt cache. It layers exactly like `model` and `effort`: the built-in table, then the
global configuration, per field, with a harness entry falling back to `default:`. A `cache_ttl` in
`.docket.yml` or `.docket.local.yml` blocks writes. The value passes through verbatim (`1h`, `5m`);
`cache_ttl: auto` removes the pin and leaves Claude Code's default.

The built-in table sets `cache_ttl: 1h` on claude's `implement-next` and nowhere else. That agent
orchestrates: it waits while a build worker or the test suite runs, and those waits regularly outlast
the 5-minute subagent cache, so its next turn writes its whole context into the cache again. A cost
audit of its transcripts found most of its spend going to those rewrites, and a 1-hour TTL is
estimated to halve its cost per run. The other agents rarely wait that long between turns, and a
1-hour cache write costs twice the input price against 1.25 times for a 5-minute one, so for them a
1-hour TTL is a surcharge of about 10–23%.

| agent | cost per run | cache writes that were expiry rewrites |
|---|---|---|
| implement-next | $5.35 | 87% |
| plan-writer | $3.96 | 24% |
| build-premium | $2.23 | 12% |
| build-standard | $0.96 | 5% |

The claude wrapper carries it under the `experimental` map:

```yaml
experimental:
  cacheTtl: 1h
```

Cursor, Codex, and opencode have no prompt-cache TTL setting. A `cache_ttl` that resolves for one of
them, for example through a `default:` entry, is left out of that harness's wrapper, and the install
warns `agent-cache-ttl-dropped`, naming the harness and agent.

Claude Code also has machine-wide controls. docket doesn't set them, because each one reaches every
subagent rather than one agent:

| control | reaches |
|---|---|
| `CLAUDE_CODE_SUBAGENT_PROMPT_CACHE_TTL=1h` (environment) | every subagent, plus workflows, teammates, forks, compaction, and session titles |
| `subagentPromptCacheTtl: "1h"` (Claude Code settings) | the same as the environment variable |
| `ENABLE_PROMPT_CACHING_1H=1` (environment) | the main conversation and every subagent |
| `FORCE_PROMPT_CACHING_5M=1` (environment) | everything, forcing the 5-minute TTL |

Precedence, highest first: `FORCE_PROMPT_CACHING_5M`, the subagent environment variable, the
subagent setting, an agent's own `cacheTtl`, `ENABLE_PROMPT_CACHING_1H`, then the default. If you set
one of the broader controls, it overrides docket's pin.

The agent field needs Claude Code 2.1.248 or later. On a Claude subscription, Claude Code ignores an
agent's `cacheTtl: 1h` while usage is drawn from usage credits.
````

Under the `## Per-harness wrapper shapes` table, change the sentence `On Cursor and opencode an effort with no resolved model has nowhere to attach and is dropped.` to:

```markdown
On Cursor and opencode an effort with no resolved model has nowhere to attach and is dropped. Only the
claude wrapper carries a prompt-cache TTL (`experimental.cacheTtl`; see
[Prompt-cache TTL](#prompt-cache-ttl)).
```

- [ ] **Step 2: Write the user-guide paragraph**

In `docs/install/models-and-effort.md`, insert after the **Changing only the model?** paragraph (before **Finding model IDs.**):

```markdown
**Prompt-cache TTL (Claude Code).** An agent entry may also set `cache_ttl`, how long Claude Code
keeps that agent's prompt cache. docket ships `cache_ttl: 1h` on Claude Code's `implement-next`
only. It waits on builds and tests longer than the 5-minute default cache lasts, so it kept paying
to write its whole context into the cache again, and the 1-hour TTL roughly halves its cost per
run. The other agents rarely wait that long, and a 1-hour cache write costs more, so they keep the
default. `cache_ttl: auto` removes a pin. Cursor, Codex, and opencode have no such setting: install
leaves the field out of their wrappers and warns. The [agent layer
reference](../../skills/docket-convention/references/agent-layer.md#prompt-cache-ttl) covers the
measurements, Claude Code's machine-wide alternatives, and which one wins.
```

- [ ] **Step 3: Regenerate the embedded twin and run the doc guards**

```bash
go generate ./internal/assets/
go test ./internal/assets/ ./internal/repoguard/ ./internal/config/ -count=1
```

Expected: PASS. If a link or anchor guard rejects the `#prompt-cache-ttl` link, check the heading spelling (`### Prompt-cache TTL`). Fix the doc, not the guard. If a prose guard rejects wording, reword it.

- [ ] **Step 4: Run the whole suite**

Run: `go run ./cmd/docket development test`
Expected: PASS. Read the budget report even when the run is green. Report any `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, or `SERIAL CONFIRMED OVER BUDGET:` line.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-convention/references/agent-layer.md docs/install/models-and-effort.md internal/assets/embedded
git commit -m "docs(agents): document the per-agent cache_ttl pin"
```

---

## Notes for the results file (for implement-next, not a build task)

- **Human verification, outside-repo truth:** whether Claude Code honours `experimental.cacheTtl` in a generated subagent wrapper can't be asserted in-repo. The suite proves the generator writes the bytes; it can't prove the loader reads them. Claude Code also loads agent definitions at process start, so the session that installs the wrapper is the wrong place to test it. Record the spec's *Verification after merge* procedure as a named human item: rebuild and reinstall per the repo's after-merge rule, restart Claude Code, run one `docket-implement-next`, and confirm in that subagent's transcript that cache writes land in `usage.cache_creation.ephemeral_1h_input_tokens` and that large expiry rewrites mostly disappear. If writes still land in the 5-minute bucket, record it as a Claude Code-side limitation and leave the pin in place.
- **Reconciled deviations from the spec text:** (1) The spec says to "extend the existing effort-drop warning in the install path". No such warning exists in the Go binary: the cursor and opencode adapters drop an orphan effort silently ("an adapter is pure computation with no diagnostic stream"). The cache-TTL drop warning therefore rides the install path's existing `ConfigWarnings` channel, the one the install result already renders. Orphan-effort warnings stay out of scope. (2) The spec names "the README's agent configuration section". The README has none. The user-facing agent-configuration page is `docs/install/models-and-effort.md`, so that is where the doc went. (3) The spec's "global-config starter" is `.docket.example.yml`'s commented `agents:` block, the one `example_correspondence_test.go` guards. The Go installer scaffolds no global config file. (4) `cache_ttl: auto` (pin removal) is added for parity with `effort: auto`, so a user can turn the shipped pin off.
