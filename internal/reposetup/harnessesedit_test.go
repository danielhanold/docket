package reposetup

import (
	"strings"
	"testing"
)

func TestRenderAgentHarnessesEditRewrites(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		harnesses []string
		want      string
	}{
		{"append after a key", "integration_branch: main\n", []string{"claude"}, "integration_branch: main\nagent_harnesses: [claude]\n"},
		{"no file writes none", "", []string{}, "agent_harnesses: []\n"},
		{"append with no final newline", "a: 1", []string{"claude"}, "a: 1\nagent_harnesses: [claude]\n"},
		{"replace flow with trailing comment", "# top\nagent_harnesses: [codex] # mine\nb: 2 # keep\n", []string{"claude", "cursor"}, "# top\nagent_harnesses: [claude, cursor]\nb: 2 # keep\n"},
		{"replace block items", "x: 1\n# Harnesses here.\nagent_harnesses:\n  - claude\n  - opencode\nfinalize:\n  gate: local\n", []string{"claude"}, "x: 1\n# Harnesses here.\nagent_harnesses: [claude]\nfinalize:\n  gate: local\n"},
		{"replace keeps CRLF", "a: 1\r\nagent_harnesses: [codex]\r\nb: 2\r\n", []string{"claude"}, "a: 1\r\nagent_harnesses: [claude]\r\nb: 2\r\n"},
		{"append keeps CRLF", "a: 1\r\n", []string{}, "a: 1\r\nagent_harnesses: []\r\n"},
		{"replace final line without newline", "a: 1\nagent_harnesses: [codex]", []string{}, "a: 1\nagent_harnesses: []"},
		{"replace empty value", "agent_harnesses:\nb: 2\n", []string{"cursor"}, "agent_harnesses: [cursor]\nb: 2\n"},
		{"nested key is not top-level", "build:\n  agent_harnesses: x\n", []string{"claude"}, "build:\n  agent_harnesses: x\nagent_harnesses: [claude]\n"},
		{"comments-only file", "# only a comment\n", []string{"claude"}, "# only a comment\nagent_harnesses: [claude]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in []byte
			if tc.in != "" {
				in = []byte(tc.in)
			}
			got, changed, err := RenderAgentHarnessesEdit(in, tc.harnesses)
			if err != nil || !changed || string(got) != tc.want {
				t.Fatalf("RenderAgentHarnessesEdit(%q, %v) = (%q, %v, %v), want (%q, true, nil)", tc.in, tc.harnesses, got, changed, err, tc.want)
			}
		})
	}
}

func TestRenderAgentHarnessesEditUnchangedWritesNothing(t *testing.T) {
	cases := []struct {
		in        string
		harnesses []string
	}{
		{"agent_harnesses: [claude, cursor]\n", []string{"claude", "cursor"}},
		{"a: 1\nagent_harnesses:\n  - claude\n  - cursor\n", []string{"claude", "cursor"}},
		{"agent_harnesses: []\n", []string{}},
	}
	for _, tc := range cases {
		got, changed, err := RenderAgentHarnessesEdit([]byte(tc.in), tc.harnesses)
		if err != nil || changed || string(got) != tc.in {
			t.Errorf("RenderAgentHarnessesEdit(%q, %v) = (%q, %v, %v), want the input back unchanged", tc.in, tc.harnesses, got, changed, err)
		}
	}
}

func TestRenderAgentHarnessesEditRefuses(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		harnesses []string
		contains  string
	}{
		{"multi-document", "a: 1\n---\nb: 2\n", []string{"claude"}, ""},
		{"sequence root", "- a\n- b\n", []string{"claude"}, ""},
		{"undecodable", "a: [\n", []string{"claude"}, ""},
		{"duplicate key", "agent_harnesses: []\nagent_harnesses: [claude]\n", []string{"claude"}, ""},
		{"flow mapping root", "{a: 1, agent_harnesses: []}\n", []string{"claude"}, ""},
		{"block scalar", "agent_harnesses: |\n  claude\nb: 2\n", []string{"claude"}, "by hand"},
		{"nil selection", "a: 1\n", nil, ""},
		{"unknown token", "a: 1\n", []string{"bogus"}, ""},
		{"non-canonical order", "a: 1\n", []string{"cursor", "claude"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := RenderAgentHarnessesEdit([]byte(tc.in), tc.harnesses)
			if err == nil || changed || got != nil {
				t.Fatalf("RenderAgentHarnessesEdit(%q, %v) = (%q, %v, %v), want a refusal", tc.in, tc.harnesses, got, changed, err)
			}
			if tc.contains != "" && !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error %q does not contain %q", err, tc.contains)
			}
		})
	}
}
