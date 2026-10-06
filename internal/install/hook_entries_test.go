package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/testsupport"
)

// The commands each dialect's tests install. Claude carries two, so a removal
// has to undo two splices in one array; Cursor carries one.
var (
	claudeHookCmds = []string{"a --x", "a --y"}
	cursorHookCmds = []string{"a --z"}
)

func hookCmdsFor(dialect string) []string {
	if dialect == HookDialectCursor {
		return cursorHookCmds
	}
	return claudeHookCmds
}

// hookOriginal is one user hooks file docket did not write.
type hookOriginal struct {
	name string
	src  string
}

func hookOriginals(dialect string) []hookOriginal {
	common := []hookOriginal{
		{"empty object", "{}\n"},
		{"unrelated key", "{\n  \"model\": \"opus\"\n}\n"},
		{"tab indented", "{\n\t\"env\": {\n\t\t\"A\": \"1\"\n\t}\n}\n"},
		{"compact one line", `{"model":"opus"}`},
		// The scanner must step over quotes, brackets, and escapes inside
		// strings, and over every scalar shape.
		{"tricky values", "{\n  \"s\": \"br]ack}e\\\"t\\\\\",\n  \"n\": -1.5e3,\n  \"t\": true,\n  \"z\": null,\n  \"a\": [[1], {\"k\": []}]\n}\n"},
		{"crlf line endings", "{\r\n  \"model\": \"opus\"\r\n}\r\n"},
		// encoding/json reads an escaped key as its unescaped spelling, so the
		// splice must find it under the same name.
		{"escaped hooks key", "{\"ho\\u006bks\": {\"Stop\": []}}\n"},
	}
	if dialect == HookDialectCursor {
		return append(common,
			hookOriginal{"another event", "{\n  \"version\": 1,\n  \"hooks\": {\n    \"afterFileEdit\": [\n      {\n        \"command\": \"./fmt.sh\"\n      }\n    ]\n  }\n}\n"},
			hookOriginal{"user entry in the event", "{\n  \"version\": 1,\n  \"hooks\": {\n    \"sessionStart\": [\n      {\"command\": \"./mine.sh\"}\n    ]\n  }\n}\n"},
			hookOriginal{"duplicate hooks keys", "{\n  \"version\": 1,\n  \"hooks\": {},\n  \"model\": \"x\",\n  \"hooks\": {\n    \"afterFileEdit\": [{\"command\": \"./fmt.sh\"}]\n  }\n}\n"},
		)
	}
	return append(common,
		hookOriginal{"another event", "{\n  \"hooks\": {\n    \"Stop\": [\n      {\n        \"hooks\": [\n          {\n            \"type\": \"command\",\n            \"command\": \"./stop.sh\"\n          }\n        ]\n      }\n    ]\n  }\n}\n"},
		hookOriginal{"user group with a matcher", "{\n  \"hooks\": {\n    \"SessionStart\": [\n      {\n        \"matcher\": \"startup\",\n        \"hooks\": [{\"type\": \"command\", \"command\": \"./mine.sh\"}]\n      }\n    ]\n  }\n}\n"},
		hookOriginal{"duplicate hooks keys", "{\n  \"hooks\": {},\n  \"model\": \"opus\",\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"./stop.sh\"}]}]\n  }\n}\n"},
	)
}

// userEntrySeed is the original whose event array already holds a user entry.
func userEntrySeed(t *testing.T, dialect string) string {
	t.Helper()
	for _, o := range hookOriginals(dialect) {
		if o.name == "user entry in the event" || o.name == "user group with a matcher" {
			return o.src
		}
	}
	t.Fatalf("no user-entry original for %s", dialect)
	return ""
}

// countEntries counts the entries satisfying pred.
func countEntries(entries []json.RawMessage, pred func(json.RawMessage) bool) int {
	n := 0
	for _, e := range entries {
		if pred(e) {
			n++
		}
	}
	return n
}

// decodedSet is every entry decoded, so two spellings of one entry compare equal.
func decoded(t *testing.T, entries []json.RawMessage) []any {
	t.Helper()
	out := make([]any, 0, len(entries))
	for _, e := range entries {
		var v any
		if err := json.Unmarshal(e, &v); err != nil {
			t.Fatalf("decoding entry %s: %v", e, err)
		}
		out = append(out, v)
	}
	return out
}

// otherSettings is the top-level document without its hooks value: every
// setting docket never owns.
func otherSettings(t *testing.T, src []byte) map[string]any {
	t.Helper()
	top := map[string]any{}
	if len(bytes.TrimSpace(src)) == 0 {
		return top
	}
	if err := json.Unmarshal(src, &top); err != nil {
		t.Fatalf("decoding %q: %v", src, err)
	}
	delete(top, "hooks")
	return top
}

func TestInsertHookEntriesRoundTrips(t *testing.T) {
	for _, dialect := range []string{HookDialectClaude, HookDialectCursor} {
		cmds := hookCmdsFor(dialect)
		for _, orig := range hookOriginals(dialect) {
			t.Run(dialect+"/"+orig.name, func(t *testing.T) {
				src := []byte(orig.src)
				before, ok := readHookEntries(src, dialect)
				if !ok {
					t.Fatalf("the original is not editable: %q", src)
				}

				ins, err := insertHookEntries(src, dialect, cmds)
				if err != nil {
					t.Fatalf("insertHookEntries: %v", err)
				}
				if !json.Valid(ins) {
					t.Fatalf("inserted bytes are not valid JSON:\n%s", ins)
				}
				if !bytes.Equal(src, []byte(orig.src)) {
					t.Fatalf("insertHookEntries mutated its input")
				}
				after, ok := readHookEntries(ins, dialect)
				if !ok {
					t.Fatalf("the inserted file is not editable:\n%s", ins)
				}
				for _, c := range cmds {
					if n := countEntries(after, func(e json.RawMessage) bool { return isExactEntry(e, dialect, c) }); n != 1 {
						t.Errorf("%d exact entries run %q, want 1:\n%s", n, c, ins)
					}
				}
				if len(after) != len(before)+len(cmds) {
					t.Errorf("%d entries after insert, want %d + %d:\n%s", len(after), len(before), len(cmds), ins)
				}
				have := decoded(t, after)
				for _, want := range decoded(t, before) {
					found := false
					for _, h := range have {
						if reflect.DeepEqual(h, want) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("the user's entry %v did not survive:\n%s", want, ins)
					}
				}
				if got, want := otherSettings(t, ins), otherSettings(t, src); !reflect.DeepEqual(got, want) {
					t.Errorf("other settings changed: got %v, want %v", got, want)
				}

				again, err := insertHookEntries(ins, dialect, cmds)
				if err != nil || !bytes.Equal(again, ins) {
					t.Errorf("a second insert changed the file (err %v):\n%s", err, again)
				}

				rem, err := removeHookEntries(ins, dialect, cmds)
				if err != nil {
					t.Fatalf("removeHookEntries: %v", err)
				}
				if string(rem) != orig.src {
					t.Errorf("round trip =\n%q\nwant\n%q\n(inserted:\n%s)", rem, orig.src, ins)
				}
			})
		}
	}

	t.Run("cursor pre-existing empty hooks object", func(t *testing.T) {
		src := []byte("{\n  \"version\": 1,\n  \"hooks\": {}\n}\n")
		ins, err := insertHookEntries(src, HookDialectCursor, cursorHookCmds)
		if err != nil {
			t.Fatal(err)
		}
		rem, err := removeHookEntries(ins, HookDialectCursor, cursorHookCmds)
		if err != nil {
			t.Fatal(err)
		}
		if want := "{\n  \"version\": 1\n}\n"; string(rem) != want {
			t.Errorf("round trip = %q, want %q", rem, want)
		}
	})
}

func TestInsertHookEntriesAddsOnlyMissing(t *testing.T) {
	exact := "{\n  \"hooks\": {\n    \"SessionStart\": [\n      {\n        \"hooks\": [\n          {\n            \"type\": \"command\",\n            \"command\": \"a --x\"\n          }\n        ]\n      }\n    ]\n  }\n}\n"
	edited := strings.Replace(exact, `"command": "a --x"`, `"command": "a --x", "timeout": 5`, 1)
	for name, src := range map[string]string{"exact": exact, "timeout added": edited} {
		t.Run(name, func(t *testing.T) {
			ins, err := insertHookEntries([]byte(src), HookDialectClaude, claudeHookCmds)
			if err != nil {
				t.Fatal(err)
			}
			entries, ok := readHookEntries(ins, HookDialectClaude)
			if !ok || len(entries) != 2 {
				t.Fatalf("entries = %d (ok %v), want 2:\n%s", len(entries), ok, ins)
			}
			runsX := countEntries(entries, func(e json.RawMessage) bool { return entryHasCommand(e, HookDialectClaude, "a --x") })
			exactY := countEntries(entries, func(e json.RawMessage) bool { return isExactEntry(e, HookDialectClaude, "a --y") })
			if runsX != 1 || exactY != 1 {
				t.Errorf("entries running a --x = %d, exact a --y = %d; want 1 and 1:\n%s", runsX, exactY, ins)
			}
			if !strings.HasPrefix(string(ins), strings.TrimSuffix(src, "\n    ]\n  }\n}\n")) {
				t.Errorf("the existing entry's bytes were rewritten:\n%s", ins)
			}
		})
	}
}

func TestNewHookFileBytes(t *testing.T) {
	claude := NewHookFileBytes(HookDialectClaude, claudeHookCmds)
	type handler struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type group struct {
		Hooks []handler `json:"hooks"`
	}
	doc := struct {
		Hooks map[string][]group `json:"hooks"`
	}{map[string][]group{"SessionStart": {
		{Hooks: []handler{{"command", "a --x"}}},
		{Hooks: []handler{{"command", "a --y"}}},
	}}}
	want, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(claude) != string(want)+"\n" {
		t.Errorf("claude new file =\n%s\nwant\n%s", claude, want)
	}

	cursor := NewHookFileBytes(HookDialectCursor, cursorHookCmds)
	if !strings.HasPrefix(string(cursor), "{\n  \"version\": 1,\n  \"hooks\": {") || !strings.HasSuffix(string(cursor), "}\n") {
		t.Errorf("cursor new file does not lead with its version:\n%s", cursor)
	}
	var got, wantCursor any
	if err := json.Unmarshal(cursor, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"version":1,"hooks":{"sessionStart":[{"command":"a --z"}]}}`), &wantCursor); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wantCursor) {
		t.Errorf("cursor new file decodes to %v, want %v", got, wantCursor)
	}

	for _, dialect := range []string{HookDialectClaude, HookDialectCursor} {
		for _, blank := range [][]byte{nil, []byte(" \n")} {
			got, err := insertHookEntries(blank, dialect, hookCmdsFor(dialect))
			if err != nil || !bytes.Equal(got, NewHookFileBytes(dialect, hookCmdsFor(dialect))) {
				t.Errorf("%s: insert into %q = %q (err %v), want the new-file bytes", dialect, blank, got, err)
			}
		}
	}
}

func hookTarget(path, dialect string) Target {
	return Target{Path: path, Kind: KindHookEntries, HookDialect: dialect,
		HookCommands: append([]string(nil), hookCmdsFor(dialect)...), Role: "trigger"}
}

func TestHookEntriesInspect(t *testing.T) {
	type tc struct {
		name   string
		seed   *string // nil: absent
		want   Disposition
		reason string
		remedy string
	}
	str := func(s string) *string { return &s }
	invalid := func(dialect string) []tc {
		wrongEvent, nullEvent := `{"hooks": {"SessionStart": {}}}`, `{"hooks": {"SessionStart": null}}`
		if dialect == HookDialectCursor {
			wrongEvent, nullEvent = `{"hooks": {"sessionStart": "x"}}`, `{"hooks": {"sessionStart": null}}`
		}
		var out []tc
		// A file docket cannot parse is skipped, never a conflict: a conflict
		// would fail the whole install over a trigger that is inert outside a
		// private repository.
		for _, s := range []string{`{"hooks": []}`, wrongEvent, nullEvent, `[1]`, `null`, `not json`, `{"hooks": null}`, `{} {}`,
			"\ufeff{}", "{\n  // a comment\n}\n"} {
			out = append(out, tc{"invalid " + s, str(s), DispositionSkip, "", remedyHookFileInvalid})
		}
		return out
	}

	for _, dialect := range []string{HookDialectClaude, HookDialectCursor} {
		full := string(NewHookFileBytes(dialect, hookCmdsFor(dialect)))
		timed := strings.Replace(full, `"command": "`+hookCmdsFor(dialect)[0]+`"`,
			`"command": "`+hookCmdsFor(dialect)[0]+`",`+"\n"+`"timeout": 5`, 1)
		cases := []tc{
			{"absent", nil, DispositionCreate, "", ""},
			{"none of the commands", str("{\n  \"model\": \"opus\"\n}\n"), DispositionUpdate, "", ""},
			{"all present exactly", str(full), DispositionNoop, "", ""},
			{"all present, one with a timeout", str(timed), DispositionNoop, "", ""},
		}
		if dialect == HookDialectClaude {
			cases = append(cases, tc{"one of two", str(string(NewHookFileBytes(dialect, claudeHookCmds[:1]))), DispositionUpdate, "", ""})
		}
		cases = append(cases, invalid(dialect)...)
		for _, c := range cases {
			t.Run(dialect+"/"+c.name, func(t *testing.T) {
				path := filepath.Join(testsupport.TempDir(t), "hooks.json")
				if c.seed != nil {
					writeFileOrDie(t, path, *c.seed)
				}
				insp, err := InspectTarget(hookTarget(path, dialect), nil, nil)
				if err != nil {
					t.Fatalf("InspectTarget: %v", err)
				}
				if insp.Disposition != c.want || insp.Reason != c.reason || insp.Remedy != c.remedy {
					t.Errorf("inspection = %s/%q/%q, want %s/%q/%q", insp.Disposition, insp.Reason, insp.Remedy, c.want, c.reason, c.remedy)
				}
				if c.seed != nil {
					if got := readOrDie(t, path); got != *c.seed {
						t.Errorf("inspection changed the file: %q", got)
					}
				}
			})
		}

		t.Run(dialect+"/symlinked file", func(t *testing.T) {
			dir := testsupport.TempDir(t)
			real := filepath.Join(dir, "dotfiles-hooks.json")
			writeFileOrDie(t, real, "{}\n")
			path := filepath.Join(dir, "hooks.json")
			if err := os.Symlink(real, path); err != nil {
				t.Fatal(err)
			}
			insp, err := InspectTarget(hookTarget(path, dialect), nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			// A dotfiles manager's link is skipped, never a conflict: a conflict
			// would fail the whole install over a trigger that is inert outside a
			// private repository.
			if insp.Disposition != DispositionSkip || insp.Reason != "" || insp.Remedy != remedyHookFileNotRegular {
				t.Errorf("inspection = %+v, want a skip with the not-a-regular-file remedy", insp)
			}
			if got := readOrDie(t, real); got != "{}\n" {
				t.Errorf("the link's destination changed: %q", got)
			}
			if fi, err := os.Lstat(path); err != nil || fi.Mode()&os.ModeSymlink == 0 {
				t.Errorf("the link was replaced: %v %v", fi, err)
			}
		})

		t.Run(dialect+"/directory at the hooks path", func(t *testing.T) {
			path := filepath.Join(testsupport.TempDir(t), "hooks.json")
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			insp, err := InspectTarget(hookTarget(path, dialect), nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if insp.Disposition != DispositionSkip || insp.Remedy != remedyHookFileNotRegular {
				t.Errorf("inspection = %+v, want a skip with the not-a-regular-file remedy", insp)
			}
		})
	}
}

// hookWorld is a temp home whose single planner (named after the dialect) plans
// one hook-entries target, plus any extra targets, and the matching uninstall.
func hookWorld(t *testing.T, dialect string, extra func(UserRoots) []Target) (Options, UninstallOptions, string) {
	t.Helper()
	roots := versionRoots(t)
	// Hermetic home: nothing below may resolve the real user's configuration.
	t.Setenv("HOME", roots.Home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(roots.Home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(roots.Home, ".local", "share"))

	path := filepath.Join(roots.Home, ".claude", "settings.json")
	if dialect == HookDialectCursor {
		path = filepath.Join(roots.Home, ".cursor", "hooks.json")
	}
	var more []Target
	if extra != nil {
		more = extra(roots)
	}
	payload := samplePayload()
	manifest := sampleManifest(t, payload)
	opts := Options{
		Roots: roots, FS: RealFS{}, Config: &config.Snapshot{},
		Catalog:   assets.NewCatalog(manifest, openFrom(payload)),
		Info:      buildinfo.Info{Version: "hook-entries"},
		Harnesses: []string{dialect},
		Planners: []Planner{{
			Name: dialect,
			Plan: func(Mode, string, assets.Catalog) ([]Target, error) {
				return append([]Target{hookTarget(path, dialect)}, more...), nil
			},
		}},
	}
	uopts := UninstallOptions{Roots: roots, FS: RealFS{}, SupportedHarnesses: []string{HookDialectClaude, HookDialectCursor}}
	return opts, uopts, path
}

func hasConflictFor(out Outcome, path, detail string) bool {
	for _, a := range out.Actions {
		if a.Op == OpConflict && a.Path == path && (detail == "" || strings.HasPrefix(a.Detail, detail)) {
			return true
		}
	}
	return false
}

func TestHookEntriesInstallUninstallLifecycle(t *testing.T) {
	for _, dialect := range []string{HookDialectClaude, HookDialectCursor} {
		seed := userEntrySeed(t, dialect)

		t.Run(dialect+"/install then uninstall restores the seed", func(t *testing.T) {
			opts, uopts, path := hookWorld(t, dialect, nil)
			writeFileOrDie(t, path, seed)
			if out := Install(opts); out.Err != nil || !out.Applied {
				t.Fatalf("Install: %v (applied %v, actions %+v)", out.Err, out.Applied, out.Actions)
			}
			installed := readOrDie(t, path)
			entries, ok := readHookEntries([]byte(installed), dialect)
			if !ok {
				t.Fatalf("the installed file is not editable:\n%s", installed)
			}
			for _, c := range hookCmdsFor(dialect) {
				if countEntries(entries, func(e json.RawMessage) bool { return isExactEntry(e, dialect, c) }) != 1 {
					t.Errorf("no exact entry runs %q:\n%s", c, installed)
				}
			}
			state, err := LoadState(opts.Roots.StatePath())
			if err != nil || state == nil {
				t.Fatalf("LoadState = %v, %v", state, err)
			}
			var rec *TargetRecord
			for i := range state.Targets {
				if state.Targets[i].Path == path {
					rec = &state.Targets[i]
				}
			}
			if rec == nil || rec.Kind != KindHookEntries || rec.HookDialect != dialect ||
				!reflect.DeepEqual(rec.HookCommands, hookCmdsFor(dialect)) ||
				rec.SHA256 != hookEntriesDigest(dialect, hookCmdsFor(dialect)) {
				t.Fatalf("state record = %+v", rec)
			}

			if out := Install(opts); out.Err != nil || out.Applied {
				t.Errorf("a second install: err %v, applied %v (want a no-op)", out.Err, out.Applied)
			}
			if got := readOrDie(t, path); got != installed {
				t.Errorf("a second install changed the file:\n%s", got)
			}

			if out := Uninstall(uopts); out.Err != nil {
				t.Fatalf("Uninstall: %v (actions %+v)", out.Err, out.Actions)
			}
			if got := readOrDie(t, path); got != seed {
				t.Errorf("after uninstall =\n%q\nwant the seed\n%q", got, seed)
			}
			assertNoStaging(t, filepath.Dir(path))
		})

		t.Run(dialect+"/an edited entry is a conflict and is left alone", func(t *testing.T) {
			opts, uopts, path := hookWorld(t, dialect, nil)
			writeFileOrDie(t, path, seed)
			if out := Install(opts); out.Err != nil {
				t.Fatalf("Install: %v", out.Err)
			}
			first := hookCmdsFor(dialect)[0]
			installed := readOrDie(t, path)
			edited := strings.Replace(installed, `"command": "`+first+`"`, `"command": "`+first+`", "timeout": 5`, 1)
			if edited == installed {
				t.Fatalf("the test could not edit the entry:\n%s", installed)
			}
			writeFileOrDie(t, path, edited)

			if out := Install(opts); out.Err != nil || out.Applied {
				t.Errorf("install over an edited entry: err %v, applied %v (want a no-op)", out.Err, out.Applied)
			}
			out := Uninstall(uopts)
			if out.Err == nil || !hasConflictFor(out, path, ReasonOwnershipConflict) {
				t.Errorf("Uninstall = err %v, actions %+v; want an ownership conflict for %s", out.Err, out.Actions, path)
			}
			if got := readOrDie(t, path); got != edited {
				t.Errorf("uninstall changed an edited file:\n%s", got)
			}
		})

		t.Run(dialect+"/entries removed by hand need no removal", func(t *testing.T) {
			opts, uopts, path := hookWorld(t, dialect, nil)
			writeFileOrDie(t, path, seed)
			if out := Install(opts); out.Err != nil {
				t.Fatalf("Install: %v", out.Err)
			}
			writeFileOrDie(t, path, seed)
			if out := Uninstall(uopts); out.Err != nil {
				t.Fatalf("Uninstall: %v (actions %+v)", out.Err, out.Actions)
			}
			if got := readOrDie(t, path); got != seed {
				t.Errorf("uninstall changed a file holding no docket entry:\n%s", got)
			}
		})

		t.Run(dialect+"/a file broken after install is a conflict", func(t *testing.T) {
			opts, uopts, path := hookWorld(t, dialect, nil)
			if out := Install(opts); out.Err != nil {
				t.Fatalf("Install: %v", out.Err)
			}
			writeFileOrDie(t, path, "not json")
			// Install and check skip the broken file; uninstall still refuses
			// it, because removal of the recorded entries cannot be proven from
			// a file docket cannot parse — and it is never edited.
			if check := Check(opts); check.Err != nil || !skippedWith(check, path, remedyHookFileInvalid) {
				t.Errorf("Check: err %v (actions %+v), skipped %+v; want the broken file skipped", check.Err, check.Actions, check.Skipped)
			}
			out := Uninstall(uopts)
			if out.Err == nil || out.Reason != ReasonManagedBlockInvalid || !hasConflictFor(out, path, ReasonManagedBlockInvalid) {
				t.Errorf("Uninstall = err %v, reason %s, actions %+v", out.Err, out.Reason, out.Actions)
			}
			if got := readOrDie(t, path); got != "not json" {
				t.Errorf("uninstall changed a broken file: %q", got)
			}
		})

		t.Run(dialect+"/a later failure rolls the file back", func(t *testing.T) {
			var later string
			opts, _, path := hookWorld(t, dialect, func(r UserRoots) []Target {
				later = filepath.Join(r.Home, "."+dialect, "zz.txt")
				return []Target{{Path: later, Kind: KindFile, Content: []byte("later\n"), Role: "agent"}}
			})
			writeFileOrDie(t, path, seed)
			hookPublishes := 0
			opts.FS = &injectFS{inner: RealFS{}, fail: func(op, p string) error {
				if op != "Rename" {
					return nil
				}
				switch p {
				case path:
					hookPublishes++
				case later:
					return fmt.Errorf("injected failure publishing %s", p)
				}
				return nil
			}}
			out := Install(opts)
			if out.Err == nil {
				t.Fatalf("Install succeeded despite the injected failure: %+v", out.Actions)
			}
			if hookPublishes != 2 {
				t.Errorf("the hooks file was published %d times, want the apply plus the restore", hookPublishes)
			}
			if got := readOrDie(t, path); got != seed {
				t.Errorf("after rollback =\n%q\nwant\n%q", got, seed)
			}
			if _, err := os.Lstat(later); !os.IsNotExist(err) {
				t.Errorf("%s survived the rollback: %v", later, err)
			}
			assertNoStaging(t, filepath.Dir(path))
		})

		t.Run(dialect+"/no seed leaves the empty document", func(t *testing.T) {
			opts, uopts, path := hookWorld(t, dialect, nil)
			if out := Install(opts); out.Err != nil {
				t.Fatalf("Install: %v", out.Err)
			}
			if got := readOrDie(t, path); got != string(NewHookFileBytes(dialect, hookCmdsFor(dialect))) {
				t.Errorf("created file =\n%s", got)
			}
			if out := Uninstall(uopts); out.Err != nil {
				t.Fatalf("Uninstall: %v (actions %+v)", out.Err, out.Actions)
			}
			want := "{}\n"
			if dialect == HookDialectCursor {
				want = "{\n  \"version\": 1\n}\n"
			}
			if got := readOrDie(t, path); got != want {
				t.Errorf("after uninstall = %q, want %q", got, want)
			}
		})
	}
}

// A later release that changes a trigger command must retire the recorded
// command's entry in the same install, or both entries would fire at every
// session start. Only an exact entry is retired: one the user has edited is
// theirs and stays.
func TestHookEntriesChangedCommandRetiresTheRecordedEntry(t *testing.T) {
	for _, dialect := range []string{HookDialectClaude, HookDialectCursor} {
		for _, edited := range []bool{false, true} {
			name := dialect + "/exact old entry"
			if edited {
				name = dialect + "/edited old entry"
			}
			t.Run(name, func(t *testing.T) {
				seed := userEntrySeed(t, dialect)
				opts, uopts, path := hookWorld(t, dialect, nil)
				oldCmds := hookCmdsFor(dialect)
				newCmds := []string{"b --new"}
				cmds := oldCmds
				opts.Planners[0].Plan = func(Mode, string, assets.Catalog) ([]Target, error) {
					return []Target{{Path: path, Kind: KindHookEntries, HookDialect: dialect,
						HookCommands: append([]string(nil), cmds...), Role: "trigger"}}, nil
				}
				writeFileOrDie(t, path, seed)
				if out := Install(opts); out.Err != nil || !out.Applied {
					t.Fatalf("first Install: %v (applied %v)", out.Err, out.Applied)
				}
				if edited {
					installed := readOrDie(t, path)
					changed := strings.Replace(installed, `"command": "`+oldCmds[0]+`"`,
						`"command": "`+oldCmds[0]+`", "timeout": 5`, 1)
					if changed == installed {
						t.Fatalf("the test could not edit the entry:\n%s", installed)
					}
					writeFileOrDie(t, path, changed)
				}

				cmds = newCmds
				if out := Install(opts); out.Err != nil || !out.Applied {
					t.Fatalf("second Install: %v (applied %v, actions %+v)", out.Err, out.Applied, out.Actions)
				}
				got := readOrDie(t, path)
				entries, ok := readHookEntries([]byte(got), dialect)
				if !ok {
					t.Fatalf("the file is not editable:\n%s", got)
				}
				if countEntries(entries, func(e json.RawMessage) bool { return isExactEntry(e, dialect, newCmds[0]) }) != 1 {
					t.Errorf("no exact entry runs the new command:\n%s", got)
				}
				for i, c := range oldCmds {
					runs := countEntries(entries, func(e json.RawMessage) bool { return entryHasCommand(e, dialect, c) })
					want := 0
					if edited && i == 0 {
						want = 1 // the user's edited entry is theirs
					}
					if runs != want {
						t.Errorf("%d entries run the retired command %q, want %d:\n%s", runs, c, want, got)
					}
				}
				rec := stateRecordAt(t, opts.Roots, path)
				if rec == nil || !reflect.DeepEqual(rec.HookCommands, newCmds) {
					t.Fatalf("state record = %+v, want the new commands only", rec)
				}
				if out := Install(opts); out.Err != nil || out.Applied {
					t.Errorf("a third install: err %v, applied %v (want a no-op)", out.Err, out.Applied)
				}
				if !edited {
					if out := Uninstall(uopts); out.Err != nil {
						t.Fatalf("Uninstall: %v (actions %+v)", out.Err, out.Actions)
					}
					if got := readOrDie(t, path); got != seed {
						t.Errorf("after uninstall =\n%q\nwant the seed\n%q", got, seed)
					}
				}
			})
		}
	}
}

// skippedFor reports whether the outcome names path as a skipped target
// carrying the not-a-regular-file remedy.
func skippedFor(out Outcome, path string) bool {
	return skippedWith(out, path, remedyHookFileNotRegular)
}

// skippedWith reports whether the outcome names path as a skipped target
// carrying remedy.
func skippedWith(out Outcome, path, remedy string) bool {
	for _, s := range out.Skipped {
		if s.Target.Path == path && s.Disposition == DispositionSkip && s.Remedy == remedy {
			return true
		}
	}
	return false
}

func stateRecordAt(t *testing.T, roots UserRoots, path string) *TargetRecord {
	t.Helper()
	state, err := LoadState(roots.StatePath())
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if state == nil {
		return nil
	}
	for i := range state.Targets {
		if state.Targets[i].Path == path {
			return &state.Targets[i]
		}
	}
	return nil
}

// A hooks file that is a dotfiles manager's symlink cannot be edited in place
// (the transaction publishes by rename, which would replace the link). It must
// not fail the install: the link and its destination stay byte-identical, every
// other target installs, the outcome names the skipped trigger, no record is
// written for it, and check and uninstall stay clean.
func TestHookEntriesSymlinkedHooksFileIsSkipped(t *testing.T) {
	for _, dialect := range []string{HookDialectClaude, HookDialectCursor} {
		seed := userEntrySeed(t, dialect)

		t.Run(dialect+"/fresh install skips the link and installs the rest", func(t *testing.T) {
			var other string
			opts, uopts, path := hookWorld(t, dialect, func(r UserRoots) []Target {
				other = filepath.Join(r.Home, "."+dialect, "agent.md")
				return []Target{{Path: other, Kind: KindFile, Content: []byte("agent\n"), Role: "agent"}}
			})
			real := filepath.Join(opts.Roots.Home, "dotfiles", filepath.Base(path))
			writeFileOrDie(t, real, seed)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(real, path); err != nil {
				t.Fatal(err)
			}
			assertLinkUntouched := func(stage string) {
				t.Helper()
				if dest, err := os.Readlink(path); err != nil || dest != real {
					t.Errorf("%s: the link now reads %q (%v), want %q", stage, dest, err, real)
				}
				if got := readOrDie(t, real); got != seed {
					t.Errorf("%s: the link's destination changed:\n%s", stage, got)
				}
			}

			out := Install(opts)
			if out.Err != nil || !out.Applied {
				t.Fatalf("Install: err %v (reason %q, applied %v, actions %+v)", out.Err, out.Reason, out.Applied, out.Actions)
			}
			if !skippedFor(out, path) {
				t.Errorf("the outcome does not name the skipped hooks file %s: %+v", path, out.Skipped)
			}
			for _, a := range out.Actions {
				if a.Path == path {
					t.Errorf("an action names the skipped hooks file: %+v", a)
				}
			}
			if got := readOrDie(t, other); got != "agent\n" {
				t.Errorf("the other target was not installed: %q", got)
			}
			assertLinkUntouched("install")
			if rec := stateRecordAt(t, opts.Roots, path); rec != nil {
				t.Errorf("a record was written for the skipped hooks file: %+v", rec)
			}
			if rec := stateRecordAt(t, opts.Roots, other); rec == nil {
				t.Errorf("no record was written for the installed target %s", other)
			}

			again := Install(opts)
			if again.Err != nil || again.Applied || !skippedFor(again, path) {
				t.Errorf("a second install: err %v, applied %v, skipped %+v (want a no-op that still names the skip)",
					again.Err, again.Applied, again.Skipped)
			}
			assertLinkUntouched("second install")

			check := Check(opts)
			if check.Err != nil || !skippedFor(check, path) {
				t.Errorf("Check: err %v (reason %q, actions %+v), skipped %+v; want clean with the skip named",
					check.Err, check.Reason, check.Actions, check.Skipped)
			}

			if u := Uninstall(uopts); u.Err != nil {
				t.Fatalf("Uninstall: %v (actions %+v)", u.Err, u.Actions)
			}
			assertLinkUntouched("uninstall")
			if _, err := os.Lstat(other); !os.IsNotExist(err) {
				t.Errorf("uninstall left the installed target %s: %v", other, err)
			}
		})

		t.Run(dialect+"/a recorded hooks file later linked is skipped and its record dropped", func(t *testing.T) {
			opts, uopts, path := hookWorld(t, dialect, nil)
			writeFileOrDie(t, path, seed)
			if out := Install(opts); out.Err != nil {
				t.Fatalf("Install: %v", out.Err)
			}
			if stateRecordAt(t, opts.Roots, path) == nil {
				t.Fatalf("the first install recorded no hook-entries target")
			}
			installed := readOrDie(t, path)
			// The user moves the file into a dotfiles checkout and links it back.
			real := filepath.Join(opts.Roots.Home, "dotfiles", filepath.Base(path))
			writeFileOrDie(t, real, installed)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(real, path); err != nil {
				t.Fatal(err)
			}

			out := Install(opts)
			if out.Err != nil || !skippedFor(out, path) {
				t.Fatalf("Install over the link: err %v (reason %q, actions %+v), skipped %+v",
					out.Err, out.Reason, out.Actions, out.Skipped)
			}
			if rec := stateRecordAt(t, opts.Roots, path); rec != nil {
				t.Errorf("the record for the now-linked hooks file survived: %+v", rec)
			}
			if u := Uninstall(uopts); u.Err != nil {
				t.Fatalf("Uninstall: %v (actions %+v)", u.Err, u.Actions)
			}
			if dest, err := os.Readlink(path); err != nil || dest != real {
				t.Errorf("the link now reads %q (%v), want %q", dest, err, real)
			}
			if got := readOrDie(t, real); got != installed {
				t.Errorf("the link's destination changed:\n%s", got)
			}
		})
	}
}

// A hooks file docket cannot parse (a JSONC comment, a byte-order mark, a
// non-object hooks value) must not fail the install either: it is left
// byte-identical, every other target installs, the outcome names the skipped
// trigger with its remedy, no record is written for it, and check and
// uninstall stay clean.
func TestHookEntriesUnparseableHooksFileIsSkipped(t *testing.T) {
	for _, dialect := range []string{HookDialectClaude, HookDialectCursor} {
		for _, broken := range []string{"{\n  // my settings\n  \"model\": \"opus\"\n}\n", "\ufeff{}\n", `{"hooks": null}`} {
			t.Run(dialect+"/"+broken, func(t *testing.T) {
				var other string
				opts, uopts, path := hookWorld(t, dialect, func(r UserRoots) []Target {
					other = filepath.Join(r.Home, "."+dialect, "agent.md")
					return []Target{{Path: other, Kind: KindFile, Content: []byte("agent\n"), Role: "agent"}}
				})
				writeFileOrDie(t, path, broken)
				assertUntouched := func(stage string) {
					t.Helper()
					if got := readOrDie(t, path); got != broken {
						t.Errorf("%s: the unparseable hooks file changed:\n%q", stage, got)
					}
				}

				out := Install(opts)
				if out.Err != nil || !out.Applied {
					t.Fatalf("Install: err %v (reason %q, applied %v, actions %+v)", out.Err, out.Reason, out.Applied, out.Actions)
				}
				if !skippedWith(out, path, remedyHookFileInvalid) {
					t.Errorf("the outcome does not name the skipped hooks file %s: %+v", path, out.Skipped)
				}
				for _, a := range out.Actions {
					if a.Path == path {
						t.Errorf("an action names the skipped hooks file: %+v", a)
					}
				}
				if got := readOrDie(t, other); got != "agent\n" {
					t.Errorf("the other target was not installed: %q", got)
				}
				assertUntouched("install")
				if rec := stateRecordAt(t, opts.Roots, path); rec != nil {
					t.Errorf("a record was written for the skipped hooks file: %+v", rec)
				}

				check := Check(opts)
				if check.Err != nil || !skippedWith(check, path, remedyHookFileInvalid) {
					t.Errorf("Check: err %v (reason %q, actions %+v), skipped %+v; want clean with the skip named",
						check.Err, check.Reason, check.Actions, check.Skipped)
				}

				if u := Uninstall(uopts); u.Err != nil {
					t.Fatalf("Uninstall: %v (actions %+v)", u.Err, u.Actions)
				}
				assertUntouched("uninstall")
			})
		}
	}
}

func TestValidateTargetHookEntries(t *testing.T) {
	good := TargetRecord{Path: "/h/.claude/settings.json", Kind: KindHookEntries, HookDialect: HookDialectClaude,
		HookCommands: []string{"a --x", "a --y"}, SHA256: hookEntriesDigest(HookDialectClaude, []string{"a --x", "a --y"}), Role: "trigger"}
	if err := validateTarget(good); err != nil {
		t.Fatalf("a well-formed record is invalid: %v", err)
	}
	bad := map[string]func(*TargetRecord){
		"no dialect":        func(r *TargetRecord) { r.HookDialect = "" },
		"unknown dialect":   func(r *TargetRecord) { r.HookDialect = "vim" },
		"no commands":       func(r *TargetRecord) { r.HookCommands = nil },
		"empty command":     func(r *TargetRecord) { r.HookCommands = []string{"a --x", ""} },
		"duplicate command": func(r *TargetRecord) { r.HookCommands = []string{"a --x", "a --x"} },
		"no sha256":         func(r *TargetRecord) { r.SHA256 = "" },
		"a block name":      func(r *TargetRecord) { r.BlockName = "dispatch" },
		"a link target":     func(r *TargetRecord) { r.LinkTarget = "/x" },
	}
	for name, mutate := range bad {
		rec := good
		rec.HookCommands = append([]string(nil), good.HookCommands...)
		mutate(&rec)
		if err := validateTarget(rec); err == nil {
			t.Errorf("record with %s is valid", name)
		}
	}
	for name, rec := range map[string]TargetRecord{
		"file with hook_commands": {Path: "/h/f", Kind: KindFile, SHA256: "ab", Role: "agent", HookCommands: []string{"a"}},
		"file with hook_dialect":  {Path: "/h/f", Kind: KindFile, SHA256: "ab", Role: "agent", HookDialect: HookDialectClaude},
		"block with hook fields":  {Path: "/h/f", Kind: KindManagedBlock, BlockName: "dispatch", SHA256: "ab", Role: "dispatch", HookDialect: HookDialectCursor, HookCommands: []string{"a"}},
		"link with hook_commands": {Path: "/h/f", Kind: KindSymlink, LinkTarget: "/x", Role: "agent", HookCommands: []string{"a"}},
	} {
		if err := validateTarget(rec); err == nil {
			t.Errorf("%s is valid", name)
		}
	}

	goodT := hookTarget("/h/.cursor/hooks.json", HookDialectCursor)
	if err := goodT.validate(); err != nil {
		t.Fatalf("a well-formed target is invalid: %v", err)
	}
	badT := map[string]func(*Target){
		"no dialect":        func(t *Target) { t.HookDialect = "" },
		"unknown dialect":   func(t *Target) { t.HookDialect = "vim" },
		"no commands":       func(t *Target) { t.HookCommands = nil },
		"empty command":     func(t *Target) { t.HookCommands = []string{""} },
		"duplicate command": func(t *Target) { t.HookCommands = []string{"a", "a"} },
		"a block name":      func(t *Target) { t.BlockName = "dispatch" },
		"file with hooks":   func(t *Target) { t.Kind = KindFile },
	}
	for name, mutate := range badT {
		tgt := goodT
		tgt.HookCommands = append([]string(nil), goodT.HookCommands...)
		mutate(&tgt)
		if err := tgt.validate(); !errors.Is(err, ErrInvalidTarget) {
			t.Errorf("target with %s: validate = %v, want ErrInvalidTarget", name, err)
		}
	}

	rec, err := RecordFor(goodT)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Kind != KindHookEntries || rec.HookDialect != HookDialectCursor || !reflect.DeepEqual(rec.HookCommands, cursorHookCmds) ||
		rec.SHA256 != hookEntriesDigest(HookDialectCursor, cursorHookCmds) || rec.BlockName != "" {
		t.Errorf("RecordFor = %+v", rec)
	}
	goodT.HookCommands[0] = "changed"
	if rec.HookCommands[0] != cursorHookCmds[0] {
		t.Errorf("RecordFor shares the target's command slice")
	}
	if err := validateTarget(rec); err != nil {
		t.Errorf("RecordFor produced an invalid record: %v", err)
	}

	// The record survives the strict state decoder.
	roots := versionRoots(t)
	state := &State{FormatVersion: StateFormatVersion, ProductVersion: "v", AssetProtocol: 1, AssetSetID: "x",
		Mode: ModeRelease, Harnesses: []string{"cursor"}, AgentDigest: "d",
		Targets: []TargetRecord{func() TargetRecord { r := rec; r.Harness = "cursor"; return r }()}}
	if err := WriteStateAtomic(roots.StatePath(), state); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(roots.StatePath())
	if err != nil || loaded == nil || !reflect.DeepEqual(loaded.Targets[0].HookCommands, cursorHookCmds) {
		t.Errorf("LoadState = %+v, %v", loaded, err)
	}
}
