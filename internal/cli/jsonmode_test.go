package cli

import "testing"

func TestDetectJSONMode(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"empty", nil, false},
		{"no flag", []string{"version"}, false},
		{"before command", []string{"--json", "version"}, true},
		{"after command", []string{"version", "--json"}, true},
		{"explicit true", []string{"version", "--json=true"}, true},
		{"explicit false", []string{"version", "--json=false"}, false},
		{"last recognized wins", []string{"--json", "version", "--json=false"}, false},
		{"false then true", []string{"--json=false", "--json"}, true},
		{"stops at standalone double dash", []string{"version", "--", "--json"}, false},
		{"recognized before the boundary", []string{"--json", "--", "--json=false"}, true},
		{"after a malformed token", []string{"version", "--bogus", "--json"}, true},
		{"unrecognized json spellings ignored", []string{"--json=1", "-json", "--jsonx"}, false},
	}
	for _, c := range cases {
		if got := DetectJSONMode(c.args); got != c.want {
			t.Fatalf("%s: DetectJSONMode(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}
