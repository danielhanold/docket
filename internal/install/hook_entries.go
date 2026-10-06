package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// A hook-entries target owns a fixed list of commands inside one JSON hooks
// file the user also owns: Claude Code's settings.json or Cursor's hooks.json.
// Each command is one entry under hooks.<event>, and an entry is identified by
// its exact command; every other byte in the file is the user's.
//
// One target covers every command in a file, never one target per command:
// the installed state holds one record per path (ValidateState), and a
// transaction can carry only one removal per path (applySteps re-verifies each
// later removal's captured pre-image, which an earlier removal on the same file
// has already rewritten).
//
// Every edit is a byte splice. Nothing in the file is decoded and re-encoded:
// an insert adds an entry after the last one in its container, and a removal
// cuts exactly the bytes an insert added, so install-then-uninstall returns the
// file the user had. encoding/json is used only to read the file, and only a
// file it reads as an object whose hooks value is an object and whose event
// value is an array is ever edited; anything else is a conflict, never
// "entries absent".

// The hook dialects: which hooks-file shape a hook-entries target edits.
const (
	HookDialectClaude = "claude"
	HookDialectCursor = "cursor"
)

var errHookFileInvalid = errors.New("install: hooks file is not a JSON object whose hooks value is an object and whose event value is an array")

// hookEvent is the hooks.<event> key a dialect's session-start entries live
// under.
func hookEvent(dialect string) (string, bool) {
	switch dialect {
	case HookDialectClaude:
		return "SessionStart", true
	case HookDialectCursor:
		return "sessionStart", true
	default:
		return "", false
	}
}

// claudeHookGroup is a Claude Code matcher group with no matcher, so every
// session source fires it.
type claudeHookGroup struct {
	Hooks []claudeHookHandler `json:"hooks"`
}

type claudeHookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type cursorHookEntry struct {
	Command string `json:"command"`
}

// canonicalHookEntry is the entry install writes for command.
func canonicalHookEntry(dialect, command string) any {
	if dialect == HookDialectCursor {
		return cursorHookEntry{Command: command}
	}
	return claudeHookGroup{Hooks: []claudeHookHandler{{Type: "command", Command: command}}}
}

// checkHookFields is the structural check a hook-entries target and its record
// share: a known dialect and a non-empty list of distinct, non-empty commands.
func checkHookFields(dialect string, commands []string) error {
	if _, ok := hookEvent(dialect); !ok {
		return fmt.Errorf("unknown hook dialect %q", dialect)
	}
	if len(commands) == 0 {
		return errors.New("no hook commands")
	}
	seen := make(map[string]bool, len(commands))
	for _, c := range commands {
		if c == "" {
			return errors.New("an empty hook command")
		}
		if seen[c] {
			return fmt.Errorf("duplicate hook command %q", c)
		}
		seen[c] = true
	}
	return nil
}

// hookEntriesDigest is a hook-entries record's identity: the dialect and the
// ordered commands it owns.
func hookEntriesDigest(dialect string, commands []string) string {
	data, err := json.Marshal(struct {
		Dialect  string   `json:"dialect"`
		Commands []string `json:"commands"`
	}{dialect, commands})
	if err != nil {
		// A struct of strings always encodes.
		panic("install: encoding a hook-entries identity: " + err.Error())
	}
	return hashBytes(data)
}

// NewHookFileBytes is the file install writes when the hooks file is absent:
// one canonical entry per command, and for Cursor the "version": 1 its hooks
// file requires, ahead of "hooks". An unknown dialect has no file.
func NewHookFileBytes(dialect string, commands []string) []byte {
	event, ok := hookEvent(dialect)
	if !ok {
		return nil
	}
	entries := make([]any, 0, len(commands))
	for _, c := range commands {
		entries = append(entries, canonicalHookEntry(dialect, c))
	}
	hooks := map[string][]any{event: entries}
	var doc any = struct {
		Hooks map[string][]any `json:"hooks"`
	}{hooks}
	if dialect == HookDialectCursor {
		doc = struct {
			Version int              `json:"version"`
			Hooks   map[string][]any `json:"hooks"`
		}{1, hooks}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		panic("install: encoding a new hooks file: " + err.Error())
	}
	return append(out, '\n')
}

// readHookEntries returns the event entries. ok=false: not editable — never
// "absent". A whitespace-only file is editable and empty.
//
// It reads the way encoding/json does, so a duplicated key resolves to its
// last occurrence; findMember follows the same rule, which keeps an edit inside
// the value this function read.
func readHookEntries(src []byte, dialect string) (entries []json.RawMessage, ok bool) {
	event, known := hookEvent(dialect)
	if !known {
		return nil, false
	}
	if len(bytes.TrimSpace(src)) == 0 {
		return nil, true
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(src, &top) != nil || top == nil {
		return nil, false
	}
	raw, has := top["hooks"]
	if !has {
		return nil, true
	}
	var hooks map[string]json.RawMessage
	if json.Unmarshal(raw, &hooks) != nil || hooks == nil {
		return nil, false
	}
	ev, has := hooks[event]
	if !has {
		return nil, true
	}
	if !bytes.HasPrefix(bytes.TrimSpace(ev), []byte("[")) || json.Unmarshal(ev, &entries) != nil {
		return nil, false
	}
	return entries, true
}

// entryHasCommand reports whether an entry runs command: for Claude, any
// handler in the group's hooks; for Cursor, the entry itself. A value that does
// not decode runs nothing.
func entryHasCommand(raw json.RawMessage, dialect, command string) bool {
	switch dialect {
	case HookDialectClaude:
		var group map[string]json.RawMessage
		if json.Unmarshal(raw, &group) != nil {
			return false
		}
		var handlers []json.RawMessage
		if json.Unmarshal(group["hooks"], &handlers) != nil {
			return false
		}
		for _, h := range handlers {
			if commandIs(h, command) {
				return true
			}
		}
		return false
	case HookDialectCursor:
		return commandIs(raw, command)
	default:
		return false
	}
}

// commandIs reports whether raw is an object whose command is exactly command.
func commandIs(raw json.RawMessage, command string) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	c, has := obj["command"]
	if !has {
		return false
	}
	var s string
	return json.Unmarshal(c, &s) == nil && s == command
}

// anyEntryRuns reports whether some entry runs command.
func anyEntryRuns(entries []json.RawMessage, dialect, command string) bool {
	for _, e := range entries {
		if entryHasCommand(e, dialect, command) {
			return true
		}
	}
	return false
}

// isExactEntry reports whether raw is, as JSON, exactly the canonical entry for
// command — however it is spelled. An entry the user has added a field to is
// theirs now.
func isExactEntry(raw json.RawMessage, dialect, command string) bool {
	var have any
	if json.Unmarshal(raw, &have) != nil {
		return false
	}
	canon, err := json.Marshal(canonicalHookEntry(dialect, command))
	if err != nil {
		return false
	}
	var want any
	if json.Unmarshal(canon, &want) != nil {
		return false
	}
	return reflect.DeepEqual(have, want)
}

// hookEntriesState is what a recorded hook-entries target finds in its file:
// whether the file is editable, whether any entry runs a recorded command, and
// whether every entry that does is exactly docket's.
func hookEntriesState(src []byte, dialect string, commands []string) (editable, anyPresent, allExact bool) {
	entries, ok := readHookEntries(src, dialect)
	if !ok {
		return false, false, false
	}
	allExact = true
	for _, e := range entries {
		for _, c := range commands {
			if entryHasCommand(e, dialect, c) {
				anyPresent = true
				if !isExactEntry(e, dialect, c) {
					allExact = false
				}
			}
		}
	}
	return true, anyPresent, allExact
}

// insertHookEntries adds the canonical entry for every command no entry runs
// yet, in order, each as the last element of the event array. A file needing
// nothing comes back unchanged, and a blank one becomes NewHookFileBytes.
func insertHookEntries(src []byte, dialect string, commands []string) ([]byte, error) {
	entries, ok := readHookEntries(src, dialect)
	if !ok {
		return nil, errHookFileInvalid
	}
	var missing []string
	for _, c := range commands {
		if !anyEntryRuns(entries, dialect, c) && !containsString(missing, c) {
			missing = append(missing, c)
		}
	}
	if len(missing) == 0 {
		return src, nil
	}
	if len(bytes.TrimSpace(src)) == 0 {
		return NewHookFileBytes(dialect, missing), nil
	}
	event, _ := hookEvent(dialect)
	unit := indentUnit(src)
	out := src
	for _, c := range missing {
		next, err := insertHookEntry(out, event, unit, canonicalHookEntry(dialect, c))
		if err != nil {
			return nil, err
		}
		out = next
	}
	// The splices assume a valid input and keep it valid; a result that does not
	// read back is a defect in them, and must never reach the user's file.
	if _, ok := readHookEntries(out, dialect); !ok {
		return nil, fmt.Errorf("install: adding hook entries produced an unreadable file")
	}
	return out, nil
}

// removeHookEntries cuts every exact entry for commands, last command first —
// the reverse of insertion, so a round trip splices back the original bytes.
// An event array docket's removal empties is removed with its member, and a
// hooks object that empties is removed with its member too. An entry that runs
// a command but is not exact is left where it is.
func removeHookEntries(src []byte, dialect string, commands []string) ([]byte, error) {
	if _, ok := readHookEntries(src, dialect); !ok {
		return nil, errHookFileInvalid
	}
	event, _ := hookEvent(dialect)
	out := src
	for i := len(commands) - 1; i >= 0; i-- {
		for {
			next, removed := removeHookEntry(out, event, dialect, commands[i])
			if !removed {
				break
			}
			out = next
		}
	}
	if _, ok := readHookEntries(out, dialect); !ok {
		return nil, fmt.Errorf("install: removing hook entries produced an unreadable file")
	}
	return out, nil
}

// insertHookEntry splices one entry in as the last element of hooks.<event>,
// creating the event member or the hooks member when absent, as the last member
// of its parent. src must be a valid JSON object (readHookEntries said so).
func insertHookEntry(src []byte, event, unit string, entry any) ([]byte, error) {
	top := skipSpace(src, 0)
	members, topClose := objectMembers(src, top)
	hi, ok := findMember(members, "hooks")
	if !ok {
		return spliceLast(src, top, topClose, memberSpans(members), 1, unit, func(pad string) ([]byte, error) {
			return memberText("hooks", map[string][]any{event: {entry}}, pad, unit)
		})
	}
	hooks := members[hi]
	hmembers, hooksClose := objectMembers(src, hooks.value.start)
	ei, ok := findMember(hmembers, event)
	if !ok {
		return spliceLast(src, hooks.value.start, hooksClose, memberSpans(hmembers), 2, unit, func(pad string) ([]byte, error) {
			return memberText(event, []any{entry}, pad, unit)
		})
	}
	ev := hmembers[ei]
	elems, arrayClose := arrayElements(src, ev.value.start)
	return spliceLast(src, ev.value.start, arrayClose, elems, 3, unit, func(pad string) ([]byte, error) {
		return json.MarshalIndent(entry, pad, unit)
	})
}

// removeHookEntry cuts the first exact entry for command. When it is the only
// element of its array, the event member goes instead, and when that member is
// the only one in hooks, the hooks member goes.
func removeHookEntry(src []byte, event, dialect, command string) ([]byte, bool) {
	top := skipSpace(src, 0)
	if top >= len(src) || src[top] != '{' {
		return src, false
	}
	members, topClose := objectMembers(src, top)
	hi, ok := findMember(members, "hooks")
	if !ok {
		return src, false
	}
	hooks := members[hi]
	hmembers, hooksClose := objectMembers(src, hooks.value.start)
	ei, ok := findMember(hmembers, event)
	if !ok {
		return src, false
	}
	ev := hmembers[ei]
	elems, arrayClose := arrayElements(src, ev.value.start)
	k := -1
	for i, e := range elems {
		if isExactEntry(src[e.start:e.end], dialect, command) {
			k = i
			break
		}
	}
	switch {
	case k < 0:
		return src, false
	case len(elems) > 1:
		return cutElement(src, ev.value.start, arrayClose, elems, k), true
	case len(hmembers) > 1:
		return cutElement(src, hooks.value.start, hooksClose, memberSpans(hmembers), ei), true
	default:
		return cutElement(src, top, topClose, memberSpans(members), hi), true
	}
}

// spliceLast adds rendered text as the last element of the container opening
// at open and closing at close, whose elements sit at depth indent units.
// After an existing element it writes ",\n" + pad + text; into an empty
// container it replaces the (whitespace-only) interior with "\n" + pad + text +
// "\n" + the container's own pad.
func spliceLast(src []byte, open, close int, elems []jsonSpan, depth int, unit string, render func(pad string) ([]byte, error)) ([]byte, error) {
	pad := strings.Repeat(unit, depth)
	text, err := render(pad)
	if err != nil {
		return nil, fmt.Errorf("install: encoding a hook entry: %w", err)
	}
	if len(elems) == 0 {
		return concatBytes(src[:open+1], []byte("\n"+pad), text, []byte("\n"+strings.Repeat(unit, depth-1)), src[close:]), nil
	}
	at := elems[len(elems)-1].end
	return concatBytes(src[:at], []byte(",\n"+pad), text, src[at:]), nil
}

// cutElement removes element k of a container's elements. The only element
// empties the container; a later one goes with the separator before it; the
// first of several goes with the separator after it.
func cutElement(src []byte, open, close int, elems []jsonSpan, k int) []byte {
	switch {
	case len(elems) == 1:
		return concatBytes(src[:open+1], src[close:])
	case k > 0:
		return concatBytes(src[:elems[k-1].end], src[elems[k].end:])
	default:
		return concatBytes(src[:elems[0].start], src[elems[1].start:])
	}
}

// memberText renders `"key": value` with value indented for pad.
func memberText(key string, value any, pad, unit string) ([]byte, error) {
	k, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}
	v, err := json.MarshalIndent(value, pad, unit)
	if err != nil {
		return nil, err
	}
	return concatBytes(k, []byte(": "), v), nil
}

// indentUnit is the file's indent unit: the leading whitespace of the first
// indented line after the first, else two spaces.
func indentUnit(src []byte) string {
	lines := bytes.Split(src, []byte("\n"))
	for _, line := range lines[1:] {
		rest := bytes.TrimLeft(line, " \t")
		if len(rest) == len(line) || len(bytes.TrimSpace(rest)) == 0 {
			continue
		}
		return string(line[:len(line)-len(rest)])
	}
	return "  "
}

func concatBytes(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// --- the byte-span scanner ---------------------------------------------------
//
// The scanner runs only over input readHookEntries accepted, so it is valid
// JSON with an object at the top; that is what lets it skip error handling.

// jsonSpan is a half-open byte range.
type jsonSpan struct{ start, end int }

// jsonMember is one object member: its decoded key, its whole span (the key's
// opening quote through the value's end), and its value's span.
type jsonMember struct {
	key   string
	span  jsonSpan
	value jsonSpan
}

func isJSONSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func skipSpace(src []byte, i int) int {
	for i < len(src) && isJSONSpace(src[i]) {
		i++
	}
	return i
}

// valueEnd is the index just past the value starting at i.
func valueEnd(src []byte, i int) int {
	switch src[i] {
	case '"':
		return stringEnd(src, i)
	case '{', '[':
		depth := 0
		for j := i; j < len(src); j++ {
			switch src[j] {
			case '"':
				j = stringEnd(src, j) - 1
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return j + 1
				}
			}
		}
		return len(src)
	default:
		j := i
		for j < len(src) && strings.IndexByte(",}] \t\r\n", src[j]) < 0 {
			j++
		}
		return j
	}
}

// stringEnd is the index just past the string whose opening quote is at i.
func stringEnd(src []byte, i int) int {
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(src)
}

// objectMembers lists the members of the object opening at open, and the index
// of its closing brace.
func objectMembers(src []byte, open int) ([]jsonMember, int) {
	var members []jsonMember
	i := skipSpace(src, open+1)
	if i < len(src) && src[i] == '}' {
		return nil, i
	}
	for i < len(src) {
		keyEnd := valueEnd(src, i)
		var key string
		_ = json.Unmarshal(src[i:keyEnd], &key)
		vs := skipSpace(src, skipSpace(src, keyEnd)+1) // past the colon
		ve := valueEnd(src, vs)
		members = append(members, jsonMember{key: key, span: jsonSpan{i, ve}, value: jsonSpan{vs, ve}})
		j := skipSpace(src, ve)
		if j >= len(src) || src[j] == '}' {
			return members, j
		}
		i = skipSpace(src, j+1) // past the comma
	}
	return members, len(src)
}

// arrayElements lists the element spans of the array opening at open, and the
// index of its closing bracket.
func arrayElements(src []byte, open int) ([]jsonSpan, int) {
	var elems []jsonSpan
	i := skipSpace(src, open+1)
	if i < len(src) && src[i] == ']' {
		return nil, i
	}
	for i < len(src) {
		end := valueEnd(src, i)
		elems = append(elems, jsonSpan{i, end})
		j := skipSpace(src, end)
		if j >= len(src) || src[j] == ']' {
			return elems, j
		}
		i = skipSpace(src, j+1)
	}
	return elems, len(src)
}

// findMember returns the index of the LAST member named key, matching
// encoding/json's last-duplicate-wins rule that readHookEntries used.
func findMember(members []jsonMember, key string) (int, bool) {
	for i := len(members) - 1; i >= 0; i-- {
		if members[i].key == key {
			return i, true
		}
	}
	return -1, false
}

func memberSpans(members []jsonMember) []jsonSpan {
	out := make([]jsonSpan, len(members))
	for i, m := range members {
		out[i] = m.span
	}
	return out
}
