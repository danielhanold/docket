<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0431 — Native Codex acceptance for active worker validation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0431-native-codex-acceptance-for-active-worker-validation.md)**
<!-- docket:backlink:end -->
# Native Codex Acceptance for Active Worker Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a minimal Go package whose tested `Value` and `Double` functions provide the implementation payload for change 431's native planner, scoped-worker, active-input-validation, and reviewer acceptance run.

**Architecture:** A new leaf package, `internal/nativeacceptance`, contains two pure exported functions with no dependencies or mutable state. Its package test pins both return values; the worker creates the tests first to produce the required RED compile failure, then adds the minimal implementation and reruns the focused package test.

**Tech Stack:** Go 1.26; standard `testing` package; `gofmt`; Docket's native scoped-worker build flow and configured whole-suite gate.

**Spec:** `docs/superpowers/specs/2026-09-16-native-codex-acceptance-for-active-worker-validation-design.md` (synchronized on the `docket` metadata branch; the active change file is `docs/changes/active/0431-native-codex-acceptance-for-active-worker-validation.md` there).

## Global Constraints

- Create only `internal/nativeacceptance/value.go` and `internal/nativeacceptance/value_test.go` for the implementation; plan and results artifacts remain coordinator-owned workflow outputs.
- `Value() int` returns exactly `1`; `Double() int` returns exactly `2 * Value()` rather than repeating the literal result.
- Add both tests before either implementation function and record the focused RED gate failing because `Value` and `Double` are undefined.
- Run focused gates with `-count=1` so the acceptance evidence cannot come from Go's test cache.
- Preserve halted change 430 and its fixture; do not merge, resume, reset, copy, or edit them.
- Preserve the stack on change 425, whose effective base and PR base are `codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor`.
- The coordinator owns native planner/worker/reviewer lineage, immutable assignment and payload validation, run-epoch and dispatch-context binding, worker active validation before acknowledgement, durable review evidence, both config-resolved full-suite gates, results attachment, final certification, branch publication, and the real stacked PR. The implementation worker must not perform those coordinator transitions.
- The BUILD gate uses the config-resolved `build.test_command`; the final gate independently uses the config-resolved `finalize.test_command`. Read and address budget reports, including any serial-confirmed breach.

---

### Task 1: Add the native acceptance value package

**Build profile:** economy

**Files:**
- Create: `internal/nativeacceptance/value_test.go`
- Create: `internal/nativeacceptance/value.go`

**Interfaces:**
- Consumes: only Go's built-in integer arithmetic and the standard `testing` package.
- Produces: `func Value() int`, returning `1`; `func Double() int`, returning `2 * Value()`.

- [ ] **Step 1: Write both failing tests**

Create `internal/nativeacceptance/value_test.go` with the complete package contract:

```go
package nativeacceptance

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != 1 {
		t.Fatalf("Value() = %d; want 1", got)
	}
}

func TestDouble(t *testing.T) {
	if got := Double(); got != 2 {
		t.Fatalf("Double() = %d; want 2", got)
	}
}
```

- [ ] **Step 2: Run the focused test to verify RED**

Run from the assigned feature worktree:

```bash
go test ./internal/nativeacceptance -count=1
```

Expected: FAIL during compilation with undefined-symbol errors for both `Value` and `Double`. This is the intended RED result; any different failure must be resolved before implementation.

- [ ] **Step 3: Add the minimal implementation**

Create `internal/nativeacceptance/value.go`:

```go
package nativeacceptance

// Value returns the acceptance package's base value.
func Value() int {
	return 1
}

// Double returns twice Value.
func Double() int {
	return 2 * Value()
}
```

- [ ] **Step 4: Format the package**

Run:

```bash
gofmt -w internal/nativeacceptance/value.go internal/nativeacceptance/value_test.go
```

Expected: both files are canonical `gofmt` output and no other path changes.

- [ ] **Step 5: Run the focused test to verify GREEN**

Run:

```bash
go test ./internal/nativeacceptance -count=1
```

Expected: PASS with output ending in `ok github.com/danielhanold/docket/internal/nativeacceptance`.

- [ ] **Step 6: Review the owned diff**

Run:

```bash
git diff --check
git status --short
git diff -- internal/nativeacceptance/value.go internal/nativeacceptance/value_test.go
```

Expected: `git diff --check` succeeds; status names only the two new implementation files in this task; the diff matches the tested code above.

- [ ] **Step 7: Commit the task**

```bash
git add internal/nativeacceptance/value.go internal/nativeacceptance/value_test.go
git commit -m "test: add native worker acceptance package"
```

Expected: one task commit containing exactly the two new package files. Return that commit and the focused RED/GREEN evidence to the build controller so it can perform active-input validation, acknowledge the scoped drive, and run the configured whole-suite build gate.
