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
