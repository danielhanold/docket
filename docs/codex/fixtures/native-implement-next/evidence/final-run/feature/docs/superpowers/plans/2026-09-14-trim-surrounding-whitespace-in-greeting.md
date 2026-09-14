<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0001 — Trim surrounding whitespace in greeting** — `docs/changes/active/0001-trim-surrounding-whitespace-in-greeting.md`
<!-- docket:backlink:end -->
# Trim Surrounding Whitespace in Greeting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `Greet` ignore surrounding Unicode whitespace while preserving internal whitespace and the existing greeting format.

**Architecture:** Keep normalization at the start of the existing `Greet` function by replacing its input with `strings.TrimSpace(name)`, then retain the current empty/non-empty branches. Extend the existing same-package table-driven test so baseline behavior and every required whitespace case are verified together.

**Tech Stack:** Go 1.23 standard library (`strings`, `testing`)

**Spec:** `docs/superpowers/specs/2026-09-14-trim-surrounding-whitespace-in-greeting-design.md`

## Global Constraints

- Apply `strings.TrimSpace` before deciding whether the supplied name is empty.
- Preserve the exported signature `func Greet(name string) string`.
- Preserve internal whitespace and punctuation.
- Keep the existing outputs: `"Hello!"` for an empty trimmed name and `"Hello, " + name + "!"` otherwise.
- Add no external dependencies.
- Preserve the existing baseline cases while adding spaces, tabs/newlines, Unicode whitespace, whitespace-only input, and internal-spacing coverage.
- Deliver exactly one implementation task and one code/test commit.

---

### Task 1: Trim Names Before Building the Greeting

**Files:**
- Modify: `greeting_test.go`
- Modify: `greeting.go`

**Interfaces:**
- Consumes: `Greet(name string) string`, the existing exported greeting function.
- Produces: The same `Greet(name string) string` interface, with surrounding Unicode whitespace removed before the empty-name check and greeting construction.

- [ ] **Step 1: Expand the table-driven test with the required whitespace behavior**

Replace `TestGreetBaseline` in `greeting_test.go` with this table-driven test. It retains all existing baseline inputs and adds explicit ASCII, Unicode, whitespace-only, and internal-spacing cases:

```go
func TestGreet(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple name", input: "Ada", want: "Hello, Ada!"},
		{name: "empty name", input: "", want: "Hello!"},
		{name: "baseline internal space", input: "Ada Lovelace", want: "Hello, Ada Lovelace!"},
		{name: "surrounding spaces", input: "  Ada  ", want: "Hello, Ada!"},
		{name: "surrounding tab and newline", input: "\tAda\n", want: "Hello, Ada!"},
		{name: "surrounding nonbreaking spaces", input: "\u00a0Ada\u00a0", want: "Hello, Ada!"},
		{name: "unicode whitespace only", input: "\u2003\t\n", want: "Hello!"},
		{name: "preserved internal spacing", input: "  Ada  Lovelace  ", want: "Hello, Ada  Lovelace!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Greet(tt.input); got != tt.want {
				t.Errorf("Greet(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the focused test to verify the new cases fail**

Run: `go test -count=1 -run '^TestGreet$' ./...`

Expected: FAIL. At minimum, the `surrounding spaces`, `surrounding tab and newline`, `surrounding nonbreaking spaces`, `unicode whitespace only`, and `preserved internal spacing` subtests report that `Greet` returned a greeting containing untrimmed surrounding whitespace instead of the expected value.

- [ ] **Step 3: Trim the name before the existing decision and formatting logic**

Replace `greeting.go` with:

```go
package greeting

import "strings"

func Greet(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Hello!"
	}
	return "Hello, " + name + "!"
}
```

- [ ] **Step 4: Run the focused test to verify the behavior passes**

Run: `go test -count=1 -run '^TestGreet$' ./...`

Expected: PASS, including the Unicode whitespace-only case and the case that preserves the two internal spaces in `Ada  Lovelace`.

- [ ] **Step 5: Run the configured full test suite**

Run: `go test -count=1 ./...`

Expected: PASS for every package.

- [ ] **Step 6: Commit the implementation and tests together**

```bash
git add greeting.go greeting_test.go
git commit -m "feat: trim whitespace in greeting"
```
