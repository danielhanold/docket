<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0552 — Install Claude Code allow rules so finalize can publish and merge under auto mode](../../changes/archive/2026-10-15-0552-install-claude-code-allow-rules-so-finalize-can-publish-and.md)**
<!-- docket:backlink:end -->

# Claude Code allow rules for finalize publish and merge — Implementation Plan

> **For agentic workers:** executed by `docket-build` (one tier agent per task under `docket-build-task`, then one full-suite gate). Steps use checkbox (`- [ ]`) syntax.

**Goal:** `docket install` owns `Bash(docket finalize publish:*)` and `Bash(docket finalize merge:*)` in `permissions.allow` of `~/.claude/settings.json`. The existing Claude hook-entries target carries them, every install converges the file to them, and uninstall removes them.

**Architecture:** `install.Target` and `install.TargetRecord` gain `AllowRules`. The field is valid only on a claude-dialect hook-entries target. The new `internal/install/allow_rules.go` reads, inserts, and removes exact strings in `permissions.allow` with the existing byte-span scanner. The two-level splice walk in `insertHookEntry`/`removeHookEntry` is extracted into shared helpers. Inspection, the ownership and uninstall proofs, and the transaction's apply and removal each learn about this second owned list. The digest covers the rules only when there are any. `harness.ClaudeHookTarget` sets the rules, and a test in `internal/cli` pins them to the capability catalog's argv.

**Tech Stack:** Go (`internal/install`, `internal/harness`, `internal/app`, `internal/cli`, `internal/repoguard`), Markdown, `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-15-install-claude-code-allow-rules-so-finalize-can-publish-and-design.md` on the `docket` branch (primary checkout: `/Users/homer/dev/docket/.docket/docs/superpowers/specs/…-design.md`).

## Global Constraints

- The rules are exactly `Bash(docket finalize publish:*)` then `Bash(docket finalize merge:*)`. Never spell them with `dckt`.
- They ride on the **existing** `harness.ClaudeHookTarget`. Do not add a second target for `settings.json`, and do not add a generic JSON-array target kind. Shared splice helper functions are fine.
- `AllowRules` is valid only on `KindHookEntries` + `HookDialectClaude`, as non-empty distinct strings. On a Cursor target or record, or on any other kind, validation fails.
- `TargetRecord.AllowRules` is tagged `json:"allow_rules,omitempty"`. No `StateFormatVersion` bump.
- `hookEntriesDigest` covers the rules only when there are any. A hooks-only digest stays byte-identical to today's (sha256 of `{"dialect":…,"commands":[…]}`).
- Edits are byte splices. `encoding/json` only reads, and every byte docket does not own survives.
- A file is editable for rules when the top level is an object, `permissions` is absent or an object, and `permissions.allow` is absent or an array. Non-string elements are ignored for matching. Any other shape takes the existing **skip** path, and only for a target that carries rules.
- "Present" = some element decodes to exactly the rule as a JSON string. A hand-typed rule counts.
- Insert appends missing rules in order as the last elements. It creates `allow` or `permissions` as the last member when absent, follows the file's indent, and never writes a result that does not read back.
- Remove cuts every equal element, last rule first. An emptied `allow` goes with its member, and an emptied `permissions` then goes too (hook parity).
- One rewrite applies, in order: retire hooks, retire rules, insert hooks, insert rules. Removal goes rules first, then hooks.
- The disposition is `update` when anything is retired or any planned command or rule is missing. Otherwise it is `no-op` and the bytes stay identical.
- Retiring rules needs a consistent prior record (its digest recomputes), the `staleHookCommands` posture.
- A symlinked or unreadable `settings.json` is skipped with a warning and never blocks. The remedy also names the rules to add by hand.
- `TestTriggersCarryNoRuleTextAndNoDocket` stays **unchanged and green**. The rules are a permission grant, not a trigger.
- Every new function gets a doc comment saying what it owns. Cross-references anchor on symbols, never line numbers (ADR-0054). Cite no ADR number for this decision in code.
- Docs describe current behavior only. Run `go generate ./internal/assets/` after editing `skills/**`.
- Mutation checks: use `-count=1`, back up with `cp f f.bak` and restore with `mv -f f.bak f` (never `git checkout --`), and confirm the mutation landed with `/usr/bin/grep -cF`.
- Tasks run focused `go test … -count=1`. The build gate is `go run ./cmd/docket development test` (`build.test_command`), and `docket-build` runs it.
- Commit messages end with `(change 0552)`. Stage explicit paths only.

## Review Focus

1. **A pre-existing empty `"allow": []` / `"permissions": {}`** disappears after install+uninstall (hook parity), while a non-empty neighbor survives. Pinned in Task 1.
2. **A hand-typed rule in another JSON spelling** (`\u002a`) is present: it is never duplicated, and inspection reads it as a no-op. Pinned in Tasks 1 and 3.
3. **Non-string or tricky `allow` elements, and duplicate `permissions` keys** survive the round trip, and the last duplicate wins. Pinned in Task 1.
4. **Only the rules are left at uninstall** (the hooks were deleted by hand): uninstall still cuts them. Pinned in Task 4.
5. **`permissions` broken after install:** uninstall refuses with `managed-block-invalid` and leaves the file untouched. Install skips with a rule-naming remedy. A rule-less target ignores the shape. Pinned in Tasks 3 and 4.

## Coordinator note (not a build task)

The spec asks for an ADR relating to ADR-0145: the user-level Claude settings target now carries a narrow permission grant, not only content-free triggers, and the ADR says why it is user-level. The coordinator dispatches `docket-adr`. No task writes it. The spec's Optional human verification item belongs in the results file.

---

### Task 1: `permissions.allow` primitives

**Files:** Modify `internal/install/hook_entries.go` (`insertHookEntry`, `removeHookEntry`). Create `internal/install/allow_rules.go` and `internal/install/allow_rules_test.go`.

**Interfaces — Produces (package `install`):** `insertNestedElement(src []byte, outer, inner, unit string, elem any) ([]byte, error)`, `removeNestedElement(src []byte, outer, inner string, match func(json.RawMessage) bool) ([]byte, bool)`, `errAllowRulesInvalid`, `readAllowRules(src []byte) ([]json.RawMessage, bool)`, `ruleIs(raw json.RawMessage, rule string) bool`, `allowRulePresent(elems []json.RawMessage, rule string) bool`, `insertAllowRules(src []byte, rules []string) ([]byte, error)`, `removeAllowRules(src []byte, rules []string) ([]byte, error)`. Test helpers: `testAllowRules`, `allowStrings`, `countString`, `withoutTestRules`.

- [ ] **Step 1: Write the failing tests** in `internal/install/allow_rules_test.go`:

```go
package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

var testAllowRules = []string{"Bash(a x:*)", "Bash(a y:*)"}

func allowOriginals() []hookOriginal {
	return []hookOriginal{
		{"empty object", "{}\n"},
		{"unrelated key", "{\n  \"model\": \"opus\"\n}\n"},
		{"tab indented", "{\n\t\"env\": {\n\t\t\"A\": \"1\"\n\t}\n}\n"},
		{"compact one line", `{"model":"opus"}`},
		{"crlf line endings", "{\r\n  \"model\": \"opus\"\r\n}\r\n"},
		{"permissions without allow", "{\n  \"permissions\": {\n    \"defaultMode\": \"acceptEdits\"\n  }\n}\n"},
		{"populated allow", "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(ls:*)\"\n    ],\n    \"deny\": [\n      \"Bash(rm:*)\"\n    ]\n  }\n}\n"},
		{"non-string allow elements", "{\n  \"permissions\": {\n    \"allow\": [1, null, {\"k\": \"]\\\"\"}, \"Bash(ls:*)\"]\n  }\n}\n"},
		{"escaped permissions key", "{\"perm\\u0069ssions\": {\"deny\": []}}\n"},
		{"duplicate permissions keys", "{\n  \"permissions\": {},\n  \"model\": \"opus\",\n  \"permissions\": {\n    \"allow\": [\"Bash(ls:*)\"]\n  }\n}\n"},
		{"hooks beside", "{\n  \"hooks\": {\n    \"Stop\": []\n  }\n}\n"},
	}
}

func allowStrings(t *testing.T, src []byte) []string {
	t.Helper()
	elems, ok := readAllowRules(src)
	if !ok {
		t.Fatalf("permissions.allow is not readable:\n%s", src)
	}
	var out []string
	for _, e := range elems {
		var s string
		if json.Unmarshal(e, &s) == nil {
			out = append(out, s)
		}
	}
	return out
}

func countString(list []string, s string) int {
	n := 0
	for _, v := range list {
		if v == s {
			n++
		}
	}
	return n
}

// withoutTestRules decodes src with docket's rules taken out of allow, and an
// emptied allow or permissions dropped: every setting docket never owns.
func withoutTestRules(t *testing.T, src []byte) map[string]any {
	t.Helper()
	top := map[string]any{}
	if len(bytes.TrimSpace(src)) == 0 {
		return top
	}
	if err := json.Unmarshal(src, &top); err != nil {
		t.Fatalf("decoding %q: %v", src, err)
	}
	p, ok := top["permissions"].(map[string]any)
	if !ok {
		return top
	}
	if a, ok := p["allow"].([]any); ok {
		var keep []any
		for _, v := range a {
			if s, isStr := v.(string); isStr && containsString(testAllowRules, s) {
				continue
			}
			keep = append(keep, v)
		}
		if len(keep) == 0 {
			delete(p, "allow")
		} else {
			p["allow"] = keep
		}
	}
	if len(p) == 0 {
		delete(top, "permissions")
	}
	return top
}

func TestAllowRulesRoundTrip(t *testing.T) {
	for _, orig := range allowOriginals() {
		t.Run(orig.name, func(t *testing.T) {
			src := []byte(orig.src)
			ins, err := insertAllowRules(src, testAllowRules)
			if err != nil || !json.Valid(ins) {
				t.Fatalf("insertAllowRules = %q, %v", ins, err)
			}
			if string(src) != orig.src {
				t.Fatalf("insertAllowRules mutated its input")
			}
			got := allowStrings(t, ins)
			if n := len(got); n < 2 || got[n-2] != testAllowRules[0] || got[n-1] != testAllowRules[1] ||
				countString(got, testAllowRules[0]) != 1 || countString(got, testAllowRules[1]) != 1 {
				t.Errorf("allow = %q, want docket's rules once each, last, in order", got)
			}
			if have, want := withoutTestRules(t, ins), withoutTestRules(t, src); !reflect.DeepEqual(have, want) {
				t.Errorf("other settings changed: got %v, want %v", have, want)
			}
			if again, err := insertAllowRules(ins, testAllowRules); err != nil || !bytes.Equal(again, ins) {
				t.Errorf("a second insert changed the file (err %v):\n%s", err, again)
			}
			rem, err := removeAllowRules(ins, testAllowRules)
			if err != nil || string(rem) != orig.src {
				t.Errorf("round trip = %q (err %v), want %q", rem, err, orig.src)
			}
		})
	}
}

func TestInsertAllowRulesExactBytes(t *testing.T) {
	const tail = "\"allow\": [\n      \"Bash(a x:*)\",\n      \"Bash(a y:*)\"\n    ]\n  }\n}\n"
	for _, c := range []struct{ name, src, want string }{
		{"no permissions", "{\n  \"model\": \"opus\"\n}\n", "{\n  \"model\": \"opus\",\n  \"permissions\": {\n    " + tail},
		{"populated allow", "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(ls:*)\"\n    ]\n  }\n}\n",
			"{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(ls:*)\",\n      \"Bash(a x:*)\",\n      \"Bash(a y:*)\"\n    ]\n  }\n}\n"},
		{"blank file", " \n", "{\n  \"permissions\": {\n    " + tail},
	} {
		if got, err := insertAllowRules([]byte(c.src), testAllowRules); err != nil || string(got) != c.want {
			t.Errorf("%s: insert = %q (err %v), want %q", c.name, got, err, c.want)
		}
	}
}

// Hook parity: a container docket's removal empties goes, even one the user
// wrote empty before install.
func TestAllowRulesEmptyContainersGoWithTheLastRule(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"empty allow", "{\n  \"permissions\": {\n    \"allow\": []\n  }\n}\n", "{}\n"},
		{"empty allow beside deny", "{\n  \"permissions\": {\n    \"allow\": [],\n    \"deny\": [\"Bash(rm:*)\"]\n  }\n}\n",
			"{\n  \"permissions\": {\n    \"deny\": [\"Bash(rm:*)\"]\n  }\n}\n"},
		{"empty permissions", "{\n  \"permissions\": {}\n}\n", "{}\n"},
	} {
		ins, err := insertAllowRules([]byte(c.src), testAllowRules)
		if err != nil || !reflect.DeepEqual(allowStrings(t, ins), testAllowRules) {
			t.Fatalf("%s: insert = %q (err %v)", c.name, ins, err)
		}
		if rem, err := removeAllowRules(ins, testAllowRules); err != nil || string(rem) != c.want {
			t.Errorf("%s: after remove = %q (err %v), want %q", c.name, rem, err, c.want)
		}
	}
}

func TestInsertAllowRulesKeepsAHandTypedRule(t *testing.T) {
	src := "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(a x:\\u002a)\"\n    ]\n  }\n}\n"
	ins, err := insertAllowRules([]byte(src), testAllowRules)
	if err != nil {
		t.Fatal(err)
	}
	if got := allowStrings(t, ins); !reflect.DeepEqual(got, testAllowRules) {
		t.Errorf("allow = %q, want each rule once", got)
	}
	if !bytes.HasPrefix(ins, []byte("{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(a x:\\u002a)\",")) {
		t.Errorf("the hand-typed element's bytes were rewritten:\n%s", ins)
	}
}

func TestAllowRulesUnreadableShapes(t *testing.T) {
	for _, s := range []string{
		`{"permissions": []}`, `{"permissions": null}`, `{"permissions": "x"}`,
		`{"permissions": {"allow": {}}}`, `{"permissions": {"allow": "Bash(a x:*)"}}`, `{"permissions": {"allow": null}}`,
		`[1]`, `null`, `not json`, "{\n  // a comment\n}\n",
	} {
		if _, ok := readAllowRules([]byte(s)); ok {
			t.Errorf("readAllowRules(%q) is editable", s)
		}
		if _, err := insertAllowRules([]byte(s), testAllowRules); !errors.Is(err, errAllowRulesInvalid) {
			t.Errorf("insertAllowRules(%q) = %v", s, err)
		}
		if _, err := removeAllowRules([]byte(s), testAllowRules); !errors.Is(err, errAllowRulesInvalid) {
			t.Errorf("removeAllowRules(%q) = %v", s, err)
		}
	}
}
```

- [ ] **Step 2: Run them.** `go test ./internal/install -run AllowRules -count=1`. Expected: a build failure (undefined `readAllowRules` …).

- [ ] **Step 3: Extract the shared walk in `hook_entries.go`.** `insertHookEntry` becomes `return insertNestedElement(src, "hooks", event, unit, entry)`, and `removeHookEntry` becomes `return removeNestedElement(src, "hooks", event, func(raw json.RawMessage) bool { return isExactEntry(raw, dialect, command) })`. Move their bodies into the two helpers, renaming `hooks`→`outer` and `event`→`inner`:

```go
func insertNestedElement(src []byte, outer, inner, unit string, elem any) ([]byte, error) {
	top := skipSpace(src, 0)
	members, topClose := objectMembers(src, top)
	oi, ok := findMember(members, outer)
	if !ok {
		return spliceLast(src, top, topClose, memberSpans(members), 1, unit, func(pad string) ([]byte, error) {
			return memberText(outer, map[string][]any{inner: {elem}}, pad, unit)
		})
	}
	o := members[oi]
	omembers, outerClose := objectMembers(src, o.value.start)
	ii, ok := findMember(omembers, inner)
	if !ok {
		return spliceLast(src, o.value.start, outerClose, memberSpans(omembers), 2, unit, func(pad string) ([]byte, error) {
			return memberText(inner, []any{elem}, pad, unit)
		})
	}
	in := omembers[ii]
	elems, arrayClose := arrayElements(src, in.value.start)
	return spliceLast(src, in.value.start, arrayClose, elems, 3, unit, func(pad string) ([]byte, error) {
		return json.MarshalIndent(elem, pad, unit)
	})
}

func removeNestedElement(src []byte, outer, inner string, match func(json.RawMessage) bool) ([]byte, bool) {
	top := skipSpace(src, 0)
	if top >= len(src) || src[top] != '{' {
		return src, false
	}
	members, topClose := objectMembers(src, top)
	oi, ok := findMember(members, outer)
	if !ok {
		return src, false
	}
	o := members[oi]
	omembers, outerClose := objectMembers(src, o.value.start)
	ii, ok := findMember(omembers, inner)
	if !ok {
		return src, false
	}
	in := omembers[ii]
	elems, arrayClose := arrayElements(src, in.value.start)
	k := -1
	for i, e := range elems {
		if match(src[e.start:e.end]) {
			k = i
			break
		}
	}
	switch {
	case k < 0:
		return src, false
	case len(elems) > 1:
		return cutElement(src, in.value.start, arrayClose, elems, k), true
	case len(omembers) > 1:
		return cutElement(src, o.value.start, outerClose, memberSpans(omembers), ii), true
	default:
		return cutElement(src, top, topClose, memberSpans(members), oi), true
	}
}
```

Run `go test ./internal/install -run 'HookEntries|NewHookFile' -count=1`. Expected: PASS. The refactor changes no behavior.

- [ ] **Step 4: Create `internal/install/allow_rules.go`.** Its file comment says it holds the second list a claude hook-entries target owns, under the same byte-splice discipline.

```go
package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

var errAllowRulesInvalid = errors.New("install: settings file is not a JSON object whose permissions value is an object and whose allow value is an array")

// readAllowRules: ok=false means not editable, never "absent". It reads like
// encoding/json (last duplicate key wins), matching findMember.
func readAllowRules(src []byte) (elems []json.RawMessage, ok bool) {
	if len(bytes.TrimSpace(src)) == 0 {
		return nil, true
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(src, &top) != nil || top == nil {
		return nil, false
	}
	raw, has := top["permissions"]
	if !has {
		return nil, true
	}
	var perms map[string]json.RawMessage
	if json.Unmarshal(raw, &perms) != nil || perms == nil {
		return nil, false
	}
	allow, has := perms["allow"]
	if !has {
		return nil, true
	}
	if !bytes.HasPrefix(bytes.TrimSpace(allow), []byte("[")) || json.Unmarshal(allow, &elems) != nil {
		return nil, false
	}
	return elems, true
}

func ruleIs(raw json.RawMessage, rule string) bool {
	var s string
	return json.Unmarshal(raw, &s) == nil && s == rule
}

func allowRulePresent(elems []json.RawMessage, rule string) bool {
	for _, e := range elems {
		if ruleIs(e, rule) {
			return true
		}
	}
	return false
}

func insertAllowRules(src []byte, rules []string) ([]byte, error) {
	elems, ok := readAllowRules(src)
	if !ok {
		return nil, errAllowRulesInvalid
	}
	var missing []string
	for _, r := range rules {
		if !allowRulePresent(elems, r) && !containsString(missing, r) {
			missing = append(missing, r)
		}
	}
	if len(missing) == 0 {
		return src, nil
	}
	if len(bytes.TrimSpace(src)) == 0 {
		out, err := json.MarshalIndent(map[string]map[string][]string{"permissions": {"allow": missing}}, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("install: encoding allow rules: %w", err)
		}
		return append(out, '\n'), nil
	}
	unit := indentUnit(src)
	out := src
	for _, r := range missing {
		next, err := insertNestedElement(out, "permissions", "allow", unit, r)
		if err != nil {
			return nil, err
		}
		out = next
	}
	if _, ok := readAllowRules(out); !ok {
		return nil, fmt.Errorf("install: adding allow rules produced an unreadable file")
	}
	return out, nil
}

func removeAllowRules(src []byte, rules []string) ([]byte, error) {
	if _, ok := readAllowRules(src); !ok {
		return nil, errAllowRulesInvalid
	}
	out := src
	for i := len(rules) - 1; i >= 0; i-- {
		rule := rules[i]
		for {
			next, removed := removeNestedElement(out, "permissions", "allow", func(raw json.RawMessage) bool { return ruleIs(raw, rule) })
			if !removed {
				break
			}
			out = next
		}
	}
	if _, ok := readAllowRules(out); !ok {
		return nil, fmt.Errorf("install: removing allow rules produced an unreadable file")
	}
	return out, nil
}
```

- [ ] **Step 5: Run.** `go test ./internal/install -run 'AllowRules|HookEntries|NewHookFile' -count=1`. Expected: PASS.

- [ ] **Step 6: Mutation.** In `insertAllowRules`, change `!allowRulePresent(elems, r) &&` to `true &&`, confirm the edit landed, and run the `AllowRules` tests. Expected: FAIL. Then restore.

- [ ] **Step 7: Commit.** `git add internal/install/hook_entries.go internal/install/allow_rules.go internal/install/allow_rules_test.go && git commit -m "feat(install): read, insert, and remove permissions.allow rules by byte splice (change 0552)"`

---

### Task 2: `AllowRules` on target and record — validation, digest, removal target

**Files:**
- `internal/install/target.go`: `Target`, `Target.validate`, `RecordFor`
- `internal/install/state.go`: `TargetRecord`, `validateTarget`
- `internal/install/hook_entries.go`: `hookEntriesDigest`
- `internal/install/allow_rules.go`: add `checkAllowRules`
- `internal/install/inspect.go`: the two `hookEntriesDigest` calls, in `staleHookCommands` and `recordMatchesDisk`
- `internal/install/txn.go`: `removalTarget`
- `internal/install/hook_entries_test.go`: its three `hookEntriesDigest` calls gain a trailing `nil`
- Test file: `allow_rules_test.go`

**Interfaces — Produces:** `Target.AllowRules []string`; unexported `Target.retireAllowRules []string`, which only inspection sets (Task 3) and the apply reads (Task 4); `TargetRecord.AllowRules []string`; `hookEntriesDigest(dialect string, commands, rules []string) string`; `checkAllowRules(dialect string, rules []string) error`; `removalTarget` copies the rules.

- [ ] **Step 1: Write the failing tests** (append):

```go
func TestHookEntriesDigestIsUnchangedWithoutRules(t *testing.T) {
	old := hashBytes([]byte(`{"dialect":"claude","commands":["a --x","a --y"]}`))
	if got := hookEntriesDigest(HookDialectClaude, claudeHookCmds, nil); got != old {
		t.Errorf("hooks-only digest = %s, want the pre-rules digest %s", got, old)
	}
	if got := hookEntriesDigest(HookDialectClaude, claudeHookCmds, []string{}); got != old {
		t.Errorf("an empty rules list changed the digest")
	}
	with := hookEntriesDigest(HookDialectClaude, claudeHookCmds, testAllowRules)
	if with == old || with == hookEntriesDigest(HookDialectClaude, claudeHookCmds, testAllowRules[:1]) {
		t.Errorf("the digest does not cover the allow rules")
	}
}

func TestValidateAllowRules(t *testing.T) {
	good := hookTarget("/h/.claude/settings.json", HookDialectClaude)
	good.AllowRules = append([]string(nil), testAllowRules...)
	if err := good.validate(); err != nil {
		t.Fatalf("a claude target with allow rules is invalid: %v", err)
	}
	for name, mutate := range map[string]func(*Target){
		"cursor dialect": func(t *Target) { t.HookDialect, t.HookCommands = HookDialectCursor, []string{"a --z"} },
		"empty rule":     func(t *Target) { t.AllowRules = []string{"Bash(a x:*)", ""} },
		"duplicate rule": func(t *Target) { t.AllowRules = []string{"Bash(a x:*)", "Bash(a x:*)"} },
		"file kind":      func(t *Target) { t.Kind, t.HookDialect, t.HookCommands = KindFile, "", nil },
	} {
		tgt := good
		tgt.AllowRules = append([]string(nil), good.AllowRules...)
		mutate(&tgt)
		if err := tgt.validate(); !errors.Is(err, ErrInvalidTarget) {
			t.Errorf("target with %s: validate = %v, want ErrInvalidTarget", name, err)
		}
	}

	rec, err := RecordFor(good)
	if err != nil || !reflect.DeepEqual(rec.AllowRules, testAllowRules) ||
		rec.SHA256 != hookEntriesDigest(HookDialectClaude, claudeHookCmds, testAllowRules) || validateTarget(rec) != nil {
		t.Fatalf("RecordFor = %+v, %v", rec, err)
	}
	good.AllowRules[0] = "changed"
	if rec.AllowRules[0] != testAllowRules[0] {
		t.Errorf("RecordFor shares the target's rule slice")
	}
	if hooksOnly, err := RecordFor(hookTarget("/h/.claude/settings.json", HookDialectClaude)); err != nil || hooksOnly.AllowRules != nil {
		t.Errorf("a hooks-only record carries allow rules %v (err %v)", hooksOnly.AllowRules, err)
	}
	for name, mutate := range map[string]func(*TargetRecord){
		"cursor dialect": func(r *TargetRecord) { r.HookDialect = HookDialectCursor },
		"empty rule":     func(r *TargetRecord) { r.AllowRules = []string{""} },
		"duplicate rule": func(r *TargetRecord) { r.AllowRules = []string{"Bash(a x:*)", "Bash(a x:*)"} },
		"file kind":      func(r *TargetRecord) { r.Kind, r.HookDialect, r.HookCommands, r.SHA256 = KindFile, "", nil, "ab" },
	} {
		r := rec
		r.AllowRules = append([]string(nil), rec.AllowRules...)
		mutate(&r)
		if validateTarget(r) == nil {
			t.Errorf("record with %s is valid", name)
		}
	}
	if rt, err := removalTarget(rec); err != nil || !reflect.DeepEqual(rt.AllowRules, testAllowRules) {
		t.Errorf("removalTarget = %+v (err %v), want the recorded rules", rt, err)
	}

	roots := versionRoots(t)
	r := rec
	r.Harness = "claude"
	state := &State{FormatVersion: StateFormatVersion, ProductVersion: "v", AssetProtocol: 1, AssetSetID: "x",
		Mode: ModeRelease, Harnesses: []string{"claude"}, AgentDigest: "d", Targets: []TargetRecord{r}}
	if err := WriteStateAtomic(roots.StatePath(), state); err != nil {
		t.Fatal(err)
	}
	if loaded, err := LoadState(roots.StatePath()); err != nil || loaded == nil || !reflect.DeepEqual(loaded.Targets[0].AllowRules, testAllowRules) {
		t.Errorf("LoadState = %+v, %v", loaded, err)
	}
}
```

- [ ] **Step 2: Run.** `go test ./internal/install -run 'HookEntriesDigestIsUnchanged|ValidateAllowRules' -count=1`. Expected: a build failure.

- [ ] **Step 3: Implement.**

`hookEntriesDigest` marshals `struct{ Dialect string \`json:"dialect"\`; Commands []string \`json:"commands"\`; AllowRules []string \`json:"allow_rules,omitempty"\` }{dialect, commands, rules}`. Its comment says `omitempty` keeps a hooks-only identity byte-identical to records written before allow rules.

`allow_rules.go`:

```go
func checkAllowRules(dialect string, rules []string) error {
	if len(rules) == 0 {
		return nil
	}
	if dialect != HookDialectClaude {
		return fmt.Errorf("allow rules on a %q hooks file; only the claude dialect carries them", dialect)
	}
	seen := make(map[string]bool, len(rules))
	for _, r := range rules {
		if r == "" {
			return errors.New("an empty allow rule")
		}
		if seen[r] {
			return fmt.Errorf("duplicate allow rule %q", r)
		}
		seen[r] = true
	}
	return nil
}
```

`target.go`: after `retireHookCommands`, add a commented `AllowRules []string` field (claude hook-entries only: exact `permissions.allow` strings the target owns, in install order) and `retireAllowRules []string` (set by inspection only). In `validate`, add `|| len(t.AllowRules) > 0` to the first non-hook guard. In `case KindHookEntries`, after `checkHookFields`, add `if err := checkAllowRules(t.HookDialect, t.AllowRules); err != nil { return fmt.Errorf("%w: %s: %v", ErrInvalidTarget, t.Path, err) }`. In `RecordFor`'s hook-entries case, add `if len(t.AllowRules) > 0 { rec.AllowRules = append([]string(nil), t.AllowRules...) }` and set `rec.SHA256 = hookEntriesDigest(t.HookDialect, t.HookCommands, t.AllowRules)`.

`state.go`: after `HookCommands`, add a commented `AllowRules []string \`json:"allow_rules,omitempty"\``. In `validateTarget`, add the same `len(target.AllowRules) > 0` to the first guard. In its hook-entries case, add `if err := checkAllowRules(target.HookDialect, target.AllowRules); err != nil { return fmt.Errorf("invalid hook-entries fields: %w", err) }`.

`inspect.go`: both `hookEntriesDigest(rec.HookDialect, rec.HookCommands)` calls → `hookEntriesDigest(rec.HookDialect, rec.HookCommands, rec.AllowRules)`.

`txn.go` `removalTarget`, `case KindHookEntries`: build the target into a variable, add `if len(rec.AllowRules) > 0 { target.AllowRules = append([]string(nil), rec.AllowRules...) }`, and return it.

Then `hook_entries_test.go`: append `, nil` to each `hookEntriesDigest(` call. Confirm there are no other sites with `/usr/bin/grep -rn 'hookEntriesDigest(' internal`.

- [ ] **Step 4: Run.** `go test ./internal/install -count=1`. Expected: PASS.

- [ ] **Step 5: Mutation.** Drop `,omitempty` from the digest's `allow_rules` tag. `TestHookEntriesDigestIsUnchangedWithoutRules` should FAIL. Restore.

- [ ] **Step 6: Commit.** Stage `target.go state.go hook_entries.go allow_rules.go inspect.go txn.go hook_entries_test.go allow_rules_test.go` (all under `internal/install/`) with the message `feat(install): record allow rules on the claude hook-entries target (change 0552)`.

---

### Task 3: Inspection, ownership proof, skip remedies

**Files:**
- `internal/install/hook_entries.go`: `hookEntriesState`
- `internal/install/inspect.go`: `inspectHookEntries`; `recordMatchesDisk`; the remedies `const` block
- `internal/install/uninstall.go`: `proveUninstallRemoval`
- `internal/install/allow_rules.go`: add `staleAllowRules`, `withAllowRules`
- Test file: `allow_rules_test.go`

**Interfaces — Produces:**
- `hookEntriesState(src []byte, dialect string, commands, rules []string) (editable, anyPresent, allExact bool)`. With rules, the file is editable only if the permissions shape is readable, and a present rule counts as present.
- `staleAllowRules(t Target, allow []json.RawMessage, rec TargetRecord, hasRec bool) []string`
- `withAllowRules(remedy string, t Target) string`
- `remedyAllowRulesInvalid`
- Test helpers `rulesTarget(path string, rules []string) Target` and `settingsWith(t *testing.T, rules []string) string`.

- [ ] **Step 1: Write the failing tests** (append, and add `"os"`, `"path/filepath"`, `"strings"`, and `"github.com/danielhanold/docket/internal/testsupport"` to the imports):

```go
func rulesTarget(path string, rules []string) Target {
	t := hookTarget(path, HookDialectClaude)
	if len(rules) > 0 {
		t.AllowRules = append([]string(nil), rules...)
	}
	return t
}

// settingsWith is docket's claude hook entries plus, when non-empty, rules.
func settingsWith(t *testing.T, rules []string) string {
	t.Helper()
	b := NewHookFileBytes(HookDialectClaude, claudeHookCmds)
	if len(rules) > 0 {
		var err error
		if b, err = insertAllowRules(b, rules); err != nil {
			t.Fatal(err)
		}
	}
	return string(b)
}

func TestAllowRulesInspect(t *testing.T) {
	str := func(s string) *string { return &s }
	full := settingsWith(t, testAllowRules)
	escaped := strings.Replace(full, `"Bash(a x:*)"`, `"Bash(a x:\u002a)"`, 1)
	if escaped == full {
		t.Fatalf("could not respell a rule")
	}
	withRules := func(p string) Target { return rulesTarget(p, testAllowRules) }
	invalidPerms := func(t Target) string { return withAllowRules(remedyAllowRulesInvalid, t) }
	for _, c := range []struct {
		name   string
		seed   *string
		target func(string) Target
		want   Disposition
		remedy func(Target) string
	}{
		{"absent", nil, withRules, DispositionCreate, nil},
		{"hooks only", str(settingsWith(t, nil)), withRules, DispositionUpdate, nil},
		{"hooks and rules", str(full), withRules, DispositionNoop, nil},
		{"one rule of two", str(settingsWith(t, testAllowRules[:1])), withRules, DispositionUpdate, nil},
		{"a rule spelled differently", str(escaped), withRules, DispositionNoop, nil},
		{"rules only", str(`{"permissions": {"allow": ["Bash(a x:*)", "Bash(a y:*)"]}}`), withRules, DispositionUpdate, nil},
		{"permissions an array", str(`{"permissions": []}`), withRules, DispositionSkip, invalidPerms},
		{"allow an object", str(`{"permissions": {"allow": {}}}`), withRules, DispositionSkip, invalidPerms},
		{"hooks an array", str(`{"hooks": []}`), withRules, DispositionSkip,
			func(t Target) string { return withAllowRules(remedyHookFileInvalid, t) }},
		{"rule-less target ignores permissions", str(`{"permissions": []}`),
			func(p string) Target { return rulesTarget(p, nil) }, DispositionUpdate, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(testsupport.TempDir(t), "settings.json")
			if c.seed != nil {
				writeFileOrDie(t, path, *c.seed)
			}
			target := c.target(path)
			insp, err := InspectTarget(target, nil, nil)
			wantRemedy := ""
			if c.remedy != nil {
				wantRemedy = c.remedy(target)
			}
			if err != nil || insp.Disposition != c.want || insp.Reason != "" || insp.Remedy != wantRemedy {
				t.Errorf("inspection = %s/%q/%q (err %v), want %s/%q", insp.Disposition, insp.Reason, insp.Remedy, err, c.want, wantRemedy)
			}
			if c.seed != nil && readOrDie(t, path) != *c.seed {
				t.Errorf("inspection changed the file")
			}
		})
	}

	t.Run("symlinked settings names the rules", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		real := filepath.Join(dir, "dotfiles-settings.json")
		writeFileOrDie(t, real, "{}\n")
		path := filepath.Join(dir, "settings.json")
		if err := os.Symlink(real, path); err != nil {
			t.Fatal(err)
		}
		insp, err := InspectTarget(withRules(path), nil, nil)
		if err != nil || insp.Disposition != DispositionSkip || insp.Remedy != withAllowRules(remedyHookFileNotRegular, withRules(path)) ||
			!strings.HasPrefix(insp.Remedy, remedyHookFileNotRegular) {
			t.Fatalf("inspection = %+v (err %v)", insp, err)
		}
		for _, r := range testAllowRules {
			if !strings.Contains(insp.Remedy, r) {
				t.Errorf("the remedy does not name %q: %s", r, insp.Remedy)
			}
		}
	})
	if got := withAllowRules(remedyHookFileInvalid, rulesTarget("/h/s.json", nil)); got != remedyHookFileInvalid {
		t.Errorf("withAllowRules changed a rule-less remedy: %q", got)
	}
}

func TestAllowRulesInspectRetiresOnlyProvenStaleRules(t *testing.T) {
	old := "Bash(a old:*)"
	recRules := append(append([]string(nil), testAllowRules...), old)
	path := filepath.Join(testsupport.TempDir(t), "settings.json")
	writeFileOrDie(t, path, settingsWith(t, recRules))
	rec, err := RecordFor(rulesTarget(path, recRules))
	if err != nil {
		t.Fatal(err)
	}
	prior := &State{Targets: []TargetRecord{rec}}
	check := func(label string, target Target, st *State, want Disposition, retire []string) {
		t.Helper()
		insp, err := InspectTarget(target, st, nil)
		if err != nil || insp.Disposition != want || !reflect.DeepEqual(insp.Target.retireAllowRules, retire) {
			t.Errorf("%s: inspection = %s, retire %q (err %v); want %s retiring %q", label, insp.Disposition, insp.Target.retireAllowRules, err, want, retire)
		}
	}
	check("dropped rule", rulesTarget(path, testAllowRules), prior, DispositionUpdate, []string{old})
	check("every rule dropped", rulesTarget(path, nil), prior, DispositionUpdate, recRules)
	bad := rec
	bad.SHA256 = hookEntriesDigest(HookDialectClaude, claudeHookCmds, nil)
	check("inconsistent record", rulesTarget(path, testAllowRules), &State{Targets: []TargetRecord{bad}}, DispositionNoop, nil)
	writeFileOrDie(t, path, settingsWith(t, testAllowRules))
	check("rule already gone", rulesTarget(path, testAllowRules), prior, DispositionNoop, nil)
}

func TestHookEntriesStateCountsAllowRules(t *testing.T) {
	rulesOnly := []byte(`{"permissions": {"allow": ["Bash(a x:*)"]}}`)
	if ed, any, exact := hookEntriesState(rulesOnly, HookDialectClaude, claudeHookCmds, testAllowRules); !ed || !any || !exact {
		t.Errorf("rules only, with rules = %v/%v/%v", ed, any, exact)
	}
	if ed, any, _ := hookEntriesState(rulesOnly, HookDialectClaude, claudeHookCmds, nil); !ed || any {
		t.Errorf("rules only, without rules = %v/%v", ed, any)
	}
	broken := []byte(`{"permissions": []}`)
	if ed, _, _ := hookEntriesState(broken, HookDialectClaude, claudeHookCmds, testAllowRules); ed {
		t.Errorf("broken permissions is editable for a record with rules")
	}
	if ed, _, _ := hookEntriesState(broken, HookDialectClaude, claudeHookCmds, nil); !ed {
		t.Errorf("broken permissions is not editable for a hooks-only record")
	}
}
```

- [ ] **Step 2: Run.** `go test ./internal/install -run 'AllowRulesInspect|HookEntriesStateCountsAllowRules' -count=1`. Expected: a build failure.

- [ ] **Step 3: Implement.**

`hookEntriesState` gains a `rules []string` parameter. After the `readHookEntries` check, add:

```go
	var allow []json.RawMessage
	if len(rules) > 0 {
		if allow, ok = readAllowRules(src); !ok {
			return false, false, false
		}
	}
```

After the entries loop, add `for _, r := range rules { if allowRulePresent(allow, r) { anyPresent = true } }`. Update its comment: a present rule is a plain string, so it is always exact.

`allow_rules.go` (add the `"strings"` import):

```go
// staleAllowRules mirrors staleHookCommands: only a consistent same-dialect
// record proves a rule docket's. A hand-typed copy goes with it (accepted).
func staleAllowRules(t Target, allow []json.RawMessage, rec TargetRecord, hasRec bool) []string {
	if !hasRec || rec.Kind != KindHookEntries || rec.HookDialect != t.HookDialect ||
		rec.SHA256 == "" || rec.SHA256 != hookEntriesDigest(rec.HookDialect, rec.HookCommands, rec.AllowRules) {
		return nil
	}
	var stale []string
	for _, r := range rec.AllowRules {
		if !containsString(t.AllowRules, r) && allowRulePresent(allow, r) {
			stale = append(stale, r)
		}
	}
	return stale
}

// withAllowRules names a skipped target's rules so the user can add them by hand.
func withAllowRules(remedy string, t Target) string {
	if len(t.AllowRules) == 0 {
		return remedy
	}
	quoted := make([]string, len(t.AllowRules))
	for i, r := range t.AllowRules {
		quoted[i] = fmt.Sprintf("%q", r)
	}
	return remedy + "; it did not add its permissions.allow rules either — to grant them by hand, add " +
		strings.Join(quoted, " and ") + " to permissions.allow"
}
```

`inspect.go` remedies block:

```go
	remedyAllowRulesInvalid = "this settings file's permissions value is not a JSON object docket can edit (permissions must be an object and its allow value an array), " +
		"so docket left it untouched and did not add its session-start entries; to install them, repair it by hand, then re-run"
```

In `inspectHookEntries`, keep the existing comments:
- the two existing `skip(t, remedyHookFile…)` returns wrap their remedies: `skip(t, withAllowRules(remedyHookFileNotRegular, t))` and `skip(t, withAllowRules(remedyHookFileInvalid, t))`;
- after `entries, ok := readHookEntries(…)` passes, insert:

```go
	allow, allowOK := readAllowRules(data)
	if len(t.AllowRules) > 0 && !allowOK {
		return skip(t, withAllowRules(remedyAllowRulesInvalid, t)), nil
	}
```

- replace the `retireHookCommands` block and the command loop with:

```go
	t.retireHookCommands = staleHookCommands(t, entries, rec, hasRec)
	if allowOK {
		t.retireAllowRules = staleAllowRules(t, allow, rec, hasRec)
	}
	if len(t.retireHookCommands) > 0 || len(t.retireAllowRules) > 0 {
		return Inspection{Target: t, Disposition: DispositionUpdate}, nil
	}
	for _, c := range t.HookCommands {
		if !anyEntryRuns(entries, t.HookDialect, c) {
			return Inspection{Target: t, Disposition: DispositionUpdate}, nil
		}
	}
	for _, r := range t.AllowRules {
		if !allowRulePresent(allow, r) { // includes a rule deleted by hand
			return Inspection{Target: t, Disposition: DispositionUpdate}, nil
		}
	}
```

Add one sentence to the func comment covering allow rules: missing → update, dropped → retired, and an unreadable permissions shape skips the file.

`recordMatchesDisk` (hook-entries case) and `proveUninstallRemoval` call `hookEntriesState(data, rec.HookDialect, rec.HookCommands, rec.AllowRules)`. Update the uninstall `!anyPresent` comment to say "no recorded command runs and no recorded rule is present".

- [ ] **Step 4: Run.** `go test ./internal/install -count=1`. Expected: PASS. The existing hook tests still see the bare remedies, because `withAllowRules` passes a rule-less remedy through unchanged.

- [ ] **Step 5: Mutation.** (a) Delete the `for _, r := range t.AllowRules` loop: `TestAllowRulesInspect` should FAIL. Restore. (b) Delete the `t.retireAllowRules = …` assignment: the stale-rules test should FAIL. Restore.

- [ ] **Step 6: Commit.** Stage `hook_entries.go inspect.go uninstall.go allow_rules.go allow_rules_test.go` (under `internal/install/`) with the message `feat(install): inspect and prove ownership of claude allow rules (change 0552)`.

---

### Task 4: Apply and remove the rules in the transaction

**Files:** `internal/install/txn.go` (`(*Txn).applyStep` case `KindHookEntries`, and `(*Txn).removeHookEntriesStep`). Test file: `allow_rules_test.go`.

**Interfaces — Consumes:** everything above, plus the existing `hookWorld`, `writeFileOrDie`, `readOrDie`, `stateRecordAt`, `skippedWith`, `hasConflictFor`, `assertNoStaging`, `countEntries`, `Install`, `Uninstall`.

- [ ] **Step 1: Write the failing tests** (append, and add the `"github.com/danielhanold/docket/internal/assets"` import):

```go
const settingsSeed = "{\n  \"model\": \"opus\",\n  \"env\": {\n    \"FOO\": \"1\"\n  },\n" +
	"  \"permissions\": {\n    \"allow\": [\n      \"Bash(git status:*)\",\n      \"Read(~/notes/**)\"\n    ],\n" +
	"    \"deny\": [\n      \"Bash(rm -rf:*)\"\n    ],\n    \"defaultMode\": \"acceptEdits\"\n  },\n" +
	"  \"hooks\": {\n    \"Stop\": [\n      {\n        \"hooks\": [\n          {\n            \"type\": \"command\",\n            \"command\": \"./stop.sh\"\n          }\n        ]\n      }\n    ]\n  }\n}\n"

// rulesWorld is a claude hookWorld whose target carries *rules (empty: hooks
// only), re-read at every plan so a test can change them between installs.
func rulesWorld(t *testing.T, rules *[]string, extra func(UserRoots) []Target) (Options, UninstallOptions, string) {
	t.Helper()
	opts, uopts, path := hookWorld(t, HookDialectClaude, extra)
	inner := opts.Planners[0].Plan
	opts.Planners[0].Plan = func(m Mode, dir string, c assets.Catalog) ([]Target, error) {
		ts, err := inner(m, dir, c)
		if err == nil && len(*rules) > 0 {
			ts[0].AllowRules = append([]string(nil), (*rules)...)
		}
		return ts, err
	}
	return opts, uopts, path
}

func mustInstall(t *testing.T, opts Options, wantApplied bool) {
	t.Helper()
	if out := Install(opts); out.Err != nil || out.Applied != wantApplied {
		t.Fatalf("Install: err %v, applied %v (want %v), actions %+v, skipped %+v", out.Err, out.Applied, wantApplied, out.Actions, out.Skipped)
	}
}

func mustUninstall(t *testing.T, uopts UninstallOptions) {
	t.Helper()
	if out := Uninstall(uopts); out.Err != nil {
		t.Fatalf("Uninstall: %v (actions %+v)", out.Err, out.Actions)
	}
}

func TestAllowRulesInstallUninstallLifecycle(t *testing.T) {
	t.Run("realistic seed: install, no-op, uninstall restores", func(t *testing.T) {
		rules := testAllowRules
		opts, uopts, path := rulesWorld(t, &rules, nil)
		writeFileOrDie(t, path, settingsSeed)
		mustInstall(t, opts, true)
		installed := readOrDie(t, path)
		if got, want := allowStrings(t, []byte(installed)), []string{"Bash(git status:*)", "Read(~/notes/**)", testAllowRules[0], testAllowRules[1]}; !reflect.DeepEqual(got, want) {
			t.Errorf("allow = %q, want %q", got, want)
		}
		entries, _ := readHookEntries([]byte(installed), HookDialectClaude)
		for _, c := range claudeHookCmds {
			if countEntries(entries, func(e json.RawMessage) bool { return isExactEntry(e, HookDialectClaude, c) }) != 1 {
				t.Errorf("no exact entry runs %q", c)
			}
		}
		if rec := stateRecordAt(t, opts.Roots, path); rec == nil || !reflect.DeepEqual(rec.AllowRules, testAllowRules) ||
			rec.SHA256 != hookEntriesDigest(HookDialectClaude, claudeHookCmds, testAllowRules) {
			t.Fatalf("state record = %+v", rec)
		}
		mustInstall(t, opts, false)
		if readOrDie(t, path) != installed {
			t.Errorf("a second install changed the file")
		}
		mustUninstall(t, uopts)
		if got := readOrDie(t, path); got != settingsSeed {
			t.Errorf("after uninstall = %q, want the seed", got)
		}
		assertNoStaging(t, filepath.Dir(path))
	})

	t.Run("no seed: hooks then rules, uninstall leaves {}", func(t *testing.T) {
		rules := testAllowRules
		opts, uopts, path := rulesWorld(t, &rules, nil)
		mustInstall(t, opts, true)
		if got := readOrDie(t, path); got != settingsWith(t, testAllowRules) {
			t.Errorf("created file =\n%s", got)
		}
		mustUninstall(t, uopts)
		if got := readOrDie(t, path); got != "{}\n" {
			t.Errorf("after uninstall = %q", got)
		}
	})

	t.Run("a hand-deleted rule comes back", func(t *testing.T) {
		rules := testAllowRules
		opts, _, path := rulesWorld(t, &rules, nil)
		writeFileOrDie(t, path, settingsSeed)
		mustInstall(t, opts, true)
		deleted, err := removeAllowRules([]byte(readOrDie(t, path)), testAllowRules[:1])
		if err != nil {
			t.Fatal(err)
		}
		writeFileOrDie(t, path, string(deleted))
		mustInstall(t, opts, true)
		got := allowStrings(t, []byte(readOrDie(t, path)))
		if countString(got, testAllowRules[0]) != 1 || countString(got, testAllowRules[1]) != 1 {
			t.Errorf("allow = %q, want each rule once", got)
		}
	})

	t.Run("a hand-typed rule is not duplicated and goes on uninstall", func(t *testing.T) {
		rules := testAllowRules
		opts, uopts, path := rulesWorld(t, &rules, nil)
		writeFileOrDie(t, path, "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(a x:*)\"\n    ]\n  }\n}\n")
		mustInstall(t, opts, true)
		if got := allowStrings(t, []byte(readOrDie(t, path))); !reflect.DeepEqual(got, testAllowRules) {
			t.Errorf("allow = %q", got)
		}
		mustUninstall(t, uopts)
		if got := readOrDie(t, path); got != "{}\n" {
			t.Errorf("after uninstall = %q", got)
		}
	})

	t.Run("a dropped rule is retired", func(t *testing.T) {
		old := "Bash(a old:*)"
		rules := append(append([]string(nil), testAllowRules...), old)
		opts, _, path := rulesWorld(t, &rules, nil)
		writeFileOrDie(t, path, settingsSeed)
		mustInstall(t, opts, true)
		rules = testAllowRules
		mustInstall(t, opts, true)
		got := allowStrings(t, []byte(readOrDie(t, path)))
		if countString(got, old) != 0 || countString(got, testAllowRules[0]) != 1 || countString(got, testAllowRules[1]) != 1 {
			t.Errorf("allow = %q", got)
		}
		if rec := stateRecordAt(t, opts.Roots, path); rec == nil || !reflect.DeepEqual(rec.AllowRules, testAllowRules) {
			t.Errorf("state record = %+v", rec)
		}
		mustInstall(t, opts, false)
	})

	t.Run("upgrade from a hooks-only record adds only the rules", func(t *testing.T) {
		var rules []string
		opts, _, path := rulesWorld(t, &rules, nil)
		writeFileOrDie(t, path, settingsSeed)
		mustInstall(t, opts, true)
		if rec := stateRecordAt(t, opts.Roots, path); rec == nil || rec.AllowRules != nil ||
			rec.SHA256 != hookEntriesDigest(HookDialectClaude, claudeHookCmds, nil) {
			t.Fatalf("hooks-only record = %+v", rec)
		}
		before := readOrDie(t, path)
		rules = testAllowRules
		mustInstall(t, opts, true)
		if stripped, err := removeAllowRules([]byte(readOrDie(t, path)), testAllowRules); err != nil || string(stripped) != before {
			t.Errorf("the upgrade changed more than the rules (err %v)", err)
		}
		if rec := stateRecordAt(t, opts.Roots, path); rec == nil || !reflect.DeepEqual(rec.AllowRules, testAllowRules) {
			t.Errorf("upgraded record = %+v", rec)
		}
	})

	t.Run("rules left after the hooks were deleted by hand are removed", func(t *testing.T) {
		rules := testAllowRules
		opts, uopts, path := rulesWorld(t, &rules, nil)
		writeFileOrDie(t, path, settingsSeed)
		mustInstall(t, opts, true)
		noHooks, err := removeHookEntries([]byte(readOrDie(t, path)), HookDialectClaude, claudeHookCmds)
		if err != nil {
			t.Fatal(err)
		}
		writeFileOrDie(t, path, string(noHooks))
		mustUninstall(t, uopts)
		if got := readOrDie(t, path); got != settingsSeed {
			t.Errorf("after uninstall = %q, want the seed", got)
		}
	})

	t.Run("permissions broken after install: uninstall refuses, install skips", func(t *testing.T) {
		rules := testAllowRules
		opts, uopts, path := rulesWorld(t, &rules, nil)
		mustInstall(t, opts, true)
		broken := strings.Replace(readOrDie(t, path), "\"permissions\": {", "\"permissions\": [], \"was\": {", 1)
		if !strings.Contains(broken, "\"permissions\": []") {
			t.Fatalf("could not break permissions")
		}
		writeFileOrDie(t, path, broken)
		out := Uninstall(uopts)
		if out.Err == nil || out.Reason != ReasonManagedBlockInvalid || !hasConflictFor(out, path, ReasonManagedBlockInvalid) {
			t.Errorf("Uninstall = err %v, reason %s, actions %+v", out.Err, out.Reason, out.Actions)
		}
		again := Install(opts)
		if again.Err != nil || !skippedWith(again, path, withAllowRules(remedyAllowRulesInvalid, rulesTarget(path, testAllowRules))) {
			t.Errorf("re-install: err %v, skipped %+v", again.Err, again.Skipped)
		}
		if readOrDie(t, path) != broken {
			t.Errorf("the broken file changed")
		}
	})

	t.Run("unreadable permissions at first install skips only this file", func(t *testing.T) {
		rules := testAllowRules
		var other string
		opts, uopts, path := rulesWorld(t, &rules, func(r UserRoots) []Target {
			other = filepath.Join(r.Home, ".claude", "agent.md")
			return []Target{{Path: other, Kind: KindFile, Content: []byte("agent\n"), Role: "agent"}}
		})
		seed := "{\n  \"permissions\": {\n    \"allow\": {}\n  }\n}\n"
		writeFileOrDie(t, path, seed)
		out := Install(opts)
		if out.Err != nil || !out.Applied || !skippedWith(out, path, withAllowRules(remedyAllowRulesInvalid, rulesTarget(path, testAllowRules))) {
			t.Fatalf("Install: err %v, applied %v, skipped %+v", out.Err, out.Applied, out.Skipped)
		}
		if readOrDie(t, other) != "agent\n" || readOrDie(t, path) != seed || stateRecordAt(t, opts.Roots, path) != nil {
			t.Errorf("want the other target installed, the settings file untouched and unrecorded")
		}
		mustUninstall(t, uopts)
		if readOrDie(t, path) != seed {
			t.Errorf("uninstall changed the skipped file")
		}
	})
}
```

- [ ] **Step 2: Run.** `go test ./internal/install -run AllowRulesInstallUninstallLifecycle -count=1`. Expected: FAIL (no rules are written).

- [ ] **Step 3: Implement in `txn.go`.**

In `applyStep` `case KindHookEntries`, after the existing `retireHookCommands` block, add:

```go
		if len(target.retireAllowRules) > 0 {
			existing, err = removeAllowRules(existing, target.retireAllowRules)
			if err != nil {
				return fmt.Errorf("retiring stale allow rules from %s: %w", step.Path, err)
			}
		}
```

After `out, err := insertHookEntries(…)` and its error check, add:

```go
		if len(target.AllowRules) > 0 {
			if out, err = insertAllowRules(out, target.AllowRules); err != nil {
				return fmt.Errorf("adding allow rules to %s: %w", step.Path, err)
			}
		}
```

In `removeHookEntriesStep`, replace `out, err := removeHookEntries(existing, …)` and its check with a version that removes rules first, then hooks (the reverse of insertion):

```go
	out := existing
	if len(target.AllowRules) > 0 {
		if out, err = removeAllowRules(out, target.AllowRules); err != nil {
			return fmt.Errorf("removing allow rules from %s: %w", step.Path, err)
		}
	}
	if out, err = removeHookEntries(out, target.HookDialect, target.HookCommands); err != nil {
		return fmt.Errorf("removing hook entries from %s: %w", step.Path, err)
	}
```

Keep the `bytes.Equal(out, existing)` early return. The doc comment now says "hook entries and allow rules".

- [ ] **Step 4: Run.** `go test ./internal/install -count=1`. Expected: PASS.

- [ ] **Step 5: Mutation.** Delete the `removeAllowRules` block in `removeHookEntriesStep`: the lifecycle subtests that uninstall should FAIL. Restore.

- [ ] **Step 6: Commit.** `git add internal/install/txn.go internal/install/allow_rules_test.go && git commit -m "feat(install): apply, converge, and uninstall claude allow rules (change 0552)"`

---

### Task 5: Wire the finalize rules into the Claude target

**Files:**
- `internal/harness/trigger.go`: new constants; `ClaudeHookTarget`
- `internal/harness/trigger_test.go`: `TestTriggerTargets`, plus the `reflect` import
- `internal/harness/claude/claude.go`: the package comment
- `internal/app/install.go`: the skipped-target loop that builds `FCHookFileNotEditable`
- `internal/app/install_test.go`
- `internal/install/service_test.go`

**Interfaces — Produces:** `harness.ClaudeFinalizePublishAllowRule`, `harness.ClaudeFinalizeMergeAllowRule`, and `ClaudeHookTarget(p).AllowRules == []string{publish, merge}`.

- [ ] **Step 1: Write the failing tests.**

Append to `TestTriggerTargets`:

```go
	if harness.ClaudeFinalizePublishAllowRule != "Bash(docket finalize publish:*)" ||
		harness.ClaudeFinalizeMergeAllowRule != "Bash(docket finalize merge:*)" {
		t.Errorf("allow rules are not the proven spellings")
	}
	if want := []string{harness.ClaudeFinalizePublishAllowRule, harness.ClaudeFinalizeMergeAllowRule}; !reflect.DeepEqual(c.AllowRules, want) {
		t.Errorf("claude target allow rules = %q, want %q", c.AllowRules, want)
	}
	if len(u.AllowRules) != 0 {
		t.Errorf("cursor target carries allow rules %q", u.AllowRules)
	}
```

In `internal/app/install_test.go`:

```go
func TestInstallResultSkippedSettingsNamesAllowRules(t *testing.T) {
	out := install.Outcome{Mode: install.ModeRelease, Applied: true, Skipped: []install.Inspection{{
		Target: install.Target{Path: "/h/.claude/settings.json", Kind: install.KindHookEntries,
			AllowRules: []string{"Bash(docket finalize publish:*)"}},
		Disposition: install.DispositionSkip, Remedy: "repair it by hand, then re-run",
	}}}
	r := NewInstallResult(OperationInstall, out)
	if len(r.Warnings) != 1 || r.Warnings[0].Message != "settings file left untouched; its trigger and allow rules are not installed" {
		t.Errorf("warnings = %+v", r.Warnings)
	}
}
```

In `internal/install/service_test.go` (add the `encoding/json` and `reflect` imports if they are absent):

```go
// A real Claude install over a realistic settings.json adds the finalize rules
// after the user's own, is a byte-identical no-op the second time, and uninstall
// restores the user's file exactly.
func TestInstallClaudeAllowRulesKeepEveryUserByte(t *testing.T) {
	w := newWorld(t, ".claude")
	seed := "{\n  \"model\": \"opus\",\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(git status:*)\"\n    ],\n" +
		"    \"deny\": [\n      \"Bash(rm -rf:*)\"\n    ],\n    \"defaultMode\": \"acceptEdits\"\n  }\n}\n"
	settings := w.path(".claude", "settings.json")
	writeFile(t, settings, seed)
	if out := install.Install(w.options(nil)); out.Err != nil || !out.Applied {
		t.Fatalf("Install: %v (reason %q)", out.Err, out.Reason)
	}
	body := readFile(t, settings)
	var doc struct {
		Permissions struct {
			Allow       []string `json:"allow"`
			Deny        []string `json:"deny"`
			DefaultMode string   `json:"defaultMode"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("decoding settings: %v", err)
	}
	want := []string{"Bash(git status:*)", harness.ClaudeFinalizePublishAllowRule, harness.ClaudeFinalizeMergeAllowRule}
	if !reflect.DeepEqual(doc.Permissions.Allow, want) || !reflect.DeepEqual(doc.Permissions.Deny, []string{"Bash(rm -rf:*)"}) ||
		doc.Permissions.DefaultMode != "acceptEdits" || !strings.Contains(body, harness.ClaudeDispatchHookCommand) {
		t.Errorf("settings after install:\n%s", body)
	}
	if again := install.Install(w.options(nil)); again.Err != nil || again.Applied || readFile(t, settings) != body {
		t.Errorf("second Install: err %v, applied %v (want a byte-identical no-op)", again.Err, again.Applied)
	}
	u := install.Uninstall(install.UninstallOptions{Roots: w.roots, FS: install.RealFS{},
		SupportedHarnesses: []string{"claude", "codex", "cursor", "opencode"}})
	if u.Err != nil || readFile(t, settings) != seed {
		t.Errorf("Uninstall: err %v; settings now\n%s", u.Err, readFile(t, settings))
	}
}
```

- [ ] **Step 2: Run.** `go test ./internal/harness ./internal/app ./internal/install -run 'TestTriggerTargets|SkippedSettingsNamesAllowRules|ClaudeAllowRulesKeepEveryUserByte' -count=1`. Expected: a build failure, then FAIL.

- [ ] **Step 3: Implement.**

`trigger.go`, a new block after the trigger constants:

```go
// The Claude Code allow rules are not triggers, and unlike them they name
// docket: a narrow permission grant in the same user-level settings.json. When
// finalize reuses a green implement-next suite run, auto mode reads the publish
// and merge that follow as a CI bypass and soft-denies them; these rules clear
// that. They match only `docket finalize publish|merge`, which refuse outside a
// docket repository, so the user-level grant authorizes nothing elsewhere.
// TestClaudeFinalizeAllowRulesMatchCatalog (internal/cli) pins them to the
// capability catalog's argv.
const (
	ClaudeFinalizePublishAllowRule = "Bash(docket finalize publish:*)"
	ClaudeFinalizeMergeAllowRule   = "Bash(docket finalize merge:*)"
)
```

`ClaudeHookTarget` adds `AllowRules: []string{ClaudeFinalizePublishAllowRule, ClaudeFinalizeMergeAllowRule},` and its comment becomes "both SessionStart commands plus the two finalize allow rules, as one hook-entries target in settings.json (one target per hooks file)". In `claude/claude.go`'s package comment, after "harness.ClaudeHookTarget", add "which also carries the two finalize allow rules".

`app/install.go`, in the skipped loop: `message := "hooks file left untouched; its trigger is not installed"`, and when `len(s.Target.AllowRules) > 0` it becomes `"settings file left untouched; its trigger and allow rules are not installed"`. Use `Message: message`.

- [ ] **Step 4: Run.** `go test ./internal/harness/... ./internal/app ./internal/install -count=1`. Expected: PASS. `TestTriggersCarryNoRuleTextAndNoDocket` is unchanged and green, because it renders hook commands only.

- [ ] **Step 5: Commit.** Stage `internal/harness/trigger.go`, `internal/harness/trigger_test.go`, `internal/harness/claude/claude.go`, `internal/app/install.go`, `internal/app/install_test.go`, and `internal/install/service_test.go` with the message `feat(harness): install the finalize publish and merge allow rules for Claude Code (change 0552)`.

---

### Task 6: Catalog drift guard

**Files:** Create `internal/cli/claude_allow_rules_test.go`. It uses the existing `collectCapabilities`, `productionRootForTest`, and `entryByID`.

- [ ] **Step 1: Write the guard.**

```go
package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
)

// TestClaudeFinalizeAllowRulesMatchCatalog pins each Claude Code allow rule to
// Bash( + its operation's space-joined catalog argv + :*), so renaming either
// command reddens here instead of leaving a rule that matches nothing, and pins
// that the Claude target carries exactly these rules in this order.
func TestClaudeFinalizeAllowRulesMatchCatalog(t *testing.T) {
	entries, err := collectCapabilities(productionRootForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, id := range []string{"finalize.publish", "finalize.merge"} {
		e, ok := entryByID(entries, id)
		if !ok || len(e.Argv) == 0 {
			t.Fatalf("%s is not in the capability catalog with an argv", id)
		}
		want = append(want, "Bash("+strings.Join(e.Argv, " ")+":*)")
	}
	if got := []string{harness.ClaudeFinalizePublishAllowRule, harness.ClaudeFinalizeMergeAllowRule}; !reflect.DeepEqual(got, want) {
		t.Errorf("allow rules = %q, want %q", got, want)
	}
	if got := harness.ClaudeHookTarget("/h/.claude/settings.json").AllowRules; !reflect.DeepEqual(got, want) {
		t.Errorf("ClaudeHookTarget allow rules = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run.** `go test ./internal/cli -run TestClaudeFinalizeAllowRulesMatchCatalog -count=1`. Expected: PASS.

- [ ] **Step 3: Mutation-test the guard.** (a) Set `ClaudeFinalizePublishAllowRule` to `"Bash(docket finalize push:*)"`. Expected: FAIL. Restore. (b) Change `ClaudeHookTarget`'s `AllowRules` to just the publish rule. Expected: FAIL. Restore. Re-run: PASS.

- [ ] **Step 4: Commit.** `git add internal/cli/claude_allow_rules_test.go && git commit -m "test(cli): pin the Claude allow rules to the capability catalog argv (change 0552)"`

---

### Task 7: Docs

**Files:** `docs/install/claude-code.md`, `docs/install/install.md`, `skills/docket-convention/references/agent-layer.md`, `internal/repoguard/budgets_test.go`, and the regenerated `internal/assets/embedded/**`.

- [ ] **Step 1: `claude-code.md`.** Under `### What an install writes for Claude Code`, add this after the SessionStart bullet:

```markdown
- **Two `permissions.allow` rules in the same `~/.claude/settings.json`:**
  `Bash(docket finalize publish:*)` and `Bash(docket finalize merge:*)`. When finalize runs right
  after a green implement-next run, it reuses that suite run rather than testing again. Without the
  rules, Claude Code's auto mode reads the resulting publish and merge as a CI bypass and stops for
  a prompt. The rules match only those two commands, which refuse to run outside a docket
  repository. Every install puts back a rule you deleted by hand. Uninstall removes both rules,
  including a copy you typed yourself. If docket cannot edit `settings.json` in place, it leaves
  the file untouched and the warning names the two rules so you can add them by hand.
```

- [ ] **Step 2: `install.md`.** After the "**Installs one content-free session trigger per harness**" bullet, add:

```markdown
- **Lets finalize publish and merge under Claude Code's auto mode**: two `permissions.allow` rules
  in `~/.claude/settings.json`, `Bash(docket finalize publish:*)` and
  `Bash(docket finalize merge:*)`. Finalize reuses a green implement-next suite run, and without
  the rules auto mode reads the publish and merge that follow as a CI bypass. The rules match only
  those two commands. Every install restores a rule you deleted by hand, and uninstall removes them.
```

- [ ] **Step 3: `agent-layer.md`.** In `### In a private repository`, append this to the paragraph that begins "Outside a private repository `docket instructions` prints nothing":

```markdown
The same `settings.json` also gets two `permissions.allow` rules, `Bash(docket finalize publish:*)` and
`Bash(docket finalize merge:*)`, in every repository. They let finalize publish and merge under Claude Code's auto
mode. They are a permission grant, not a trigger.
```

Keep every string `TestPrivateInstructionsDocContracts` requires in that section.

- [ ] **Step 4:** Run `go generate ./internal/assets/`, then `wc -l -w skills/docket-convention/references/agent-layer.md`. Set the `docket-convention/references/agent-layer.md` row in `budgets_test.go` to the measured counts, and prepend `0552: the user-level settings.json also carries the finalize allow rules (212/1951 -> L/W); ` to its comment, using the measured L/W.

- [ ] **Step 5: Run.** `go test ./internal/repoguard ./internal/assets -count=1`. Expected: PASS. Fix the doc or the regenerated asset, never a guard.

- [ ] **Step 6: Commit.** Stage the three docs, `budgets_test.go`, and the paths `git status --porcelain internal/assets` lists, with the message `docs(install): list the Claude Code finalize allow rules docket installs (change 0552)`.
