package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/testsupport"
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
