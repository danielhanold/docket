package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseOpenArgs(t *testing.T) {
	ok := []struct {
		args []string
		what string
		id   int
	}{
		{[]string{"board"}, "board", 0}, {[]string{"spec"}, "spec", 0},
		{[]string{"change", "541"}, "change", 541}, {[]string{"plan", "0541"}, "plan", 541},
		{[]string{"pr", "7"}, "pr", 7}, {[]string{"results", "000012"}, "results", 12},
	}
	for _, tc := range ok {
		if what, id, err := ParseOpenArgs(tc.args); err != nil || what != tc.what || id != tc.id {
			t.Errorf("ParseOpenArgs(%q) = (%q, %d, %v)", tc.args, what, id, err)
		}
	}
	bad := []struct {
		args []string
		want string // exact message; "" = must list all six targets
	}{
		{nil, ""}, {[]string{"bogus"}, ""}, {[]string{"board", "5"}, ""}, {[]string{"spec", "5", "6"}, ""},
		{[]string{"spec", "#541"}, "invalid change id #541"}, {[]string{"spec", "+5"}, "invalid change id +5"},
		{[]string{"spec", "-5"}, "invalid change id -5"}, {[]string{"spec", "0"}, "invalid change id 0"},
		{[]string{"spec", "0x1F"}, "invalid change id 0x1F"},
		{[]string{"spec", "99999999999999999999999"}, "invalid change id 99999999999999999999999"},
	}
	for _, tc := range bad {
		_, _, err := ParseOpenArgs(tc.args)
		if err == nil || strings.Contains(err.Error(), "\n") {
			t.Errorf("ParseOpenArgs(%q) = %v, want a one-line usage error", tc.args, err)
			continue
		}
		if tc.want != "" && err.Error() != tc.want {
			t.Errorf("ParseOpenArgs(%q) = %q, want %q", tc.args, err, tc.want)
		}
		for _, w := range OpenTargets {
			if tc.want == "" && !strings.Contains(err.Error(), w) {
				t.Errorf("ParseOpenArgs(%q) = %q does not list %q", tc.args, err, w)
			}
		}
	}
}

func TestOpenResultRendering(t *testing.T) {
	r := OpenResult{What: "board", Target: "/m/BOARD.md", Notes: []string{openNoteNotGitHub}}
	if r.HumanText() != "/m/BOARD.md" || !reflect.DeepEqual(r.HumanNotes(), r.Notes) {
		t.Errorf("success renders %q / %q", r.HumanText(), r.HumanNotes())
	}
	if r.Message = "boom"; r.HumanText() != "/m/BOARD.md\nboom" { // resolved target, then the failure
		t.Errorf("failure with target renders %q", r.HumanText())
	}
	if r.Target = ""; r.HumanText() != "boom" {
		t.Errorf("failure renders %q", r.HumanText())
	}
	raw, _ := json.Marshal(OpenResult{What: "board", Notes: []string{}})
	if strings.Contains(string(raw), "change_id") || !strings.Contains(string(raw), `"notes":[]`) || !strings.Contains(string(raw), `"launched":false`) {
		t.Errorf("board document = %s, want no change_id, notes [], launched false", raw)
	}
}
