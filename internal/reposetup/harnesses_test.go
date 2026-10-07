package reposetup

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseHarnessSelection(t *testing.T) {
	accepted := []struct {
		in   []string
		want []string
	}{
		{[]string{"claude"}, []string{"claude"}},
		{[]string{"cursor", "claude"}, []string{"claude", "cursor"}},
		{[]string{" claude", "cursor "}, []string{"claude", "cursor"}},
		{[]string{"opencode", "cursor", "codex", "claude"}, []string{"claude", "codex", "cursor", "opencode"}},
		{[]string{"none"}, []string{}},
		{[]string{" none "}, []string{}},
	}
	for _, tc := range accepted {
		got, err := ParseHarnessSelection(tc.in)
		if err != nil {
			t.Errorf("ParseHarnessSelection(%q) error: %v", tc.in, err)
			continue
		}
		if got == nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseHarnessSelection(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}

	refused := []struct {
		in    []string
		words []string
	}{
		{nil, []string{"empty"}},
		{[]string{}, []string{"empty"}},
		{[]string{""}, []string{"empty"}},
		{[]string{"claude", ""}, []string{"empty"}},
		{[]string{"claude", "claude"}, []string{"claude", "more than once"}},
		{[]string{"none", "claude"}, []string{"none"}},
		{[]string{"none", "none"}, []string{"none", "more than once"}},
		{[]string{"bogus", "claude", "nope"}, []string{"bogus", "nope", "claude, codex, cursor, opencode"}},
		{[]string{"Claude"}, []string{"Claude"}},
	}
	for _, tc := range refused {
		got, err := ParseHarnessSelection(tc.in)
		if err == nil {
			t.Errorf("ParseHarnessSelection(%q) = %v, want refusal", tc.in, got)
			continue
		}
		if !errors.Is(err, ErrInvalidHarnesses) {
			t.Errorf("ParseHarnessSelection(%q) error %v does not wrap ErrInvalidHarnesses", tc.in, err)
		}
		for _, w := range tc.words {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("ParseHarnessSelection(%q) error %q lacks %q", tc.in, err, w)
			}
		}
	}
}

func TestAgentHarnessesLine(t *testing.T) {
	if got := AgentHarnessesLine([]string{"claude", "cursor"}); got != "agent_harnesses: [claude, cursor]" {
		t.Errorf("got %q", got)
	}
	if got := AgentHarnessesLine([]string{}); got != "agent_harnesses: []" {
		t.Errorf("got %q", got)
	}
}
