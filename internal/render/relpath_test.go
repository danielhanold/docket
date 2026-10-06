package render

import (
	"path"
	"testing"
)

func TestRelativeLink(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"docs/changes/active/0001-a.md", "docs/superpowers/specs/s.md", "../../superpowers/specs/s.md"},
		{"docs/changes/archive/2026-10-06-0001-a.md", "docs/superpowers/specs/s.md", "../../superpowers/specs/s.md"},
		{"docs/superpowers/plans/p.md", "docs/changes/active/0001-a.md", "../../changes/active/0001-a.md"},
		{"docs/results/r.md", "docs/changes/active/0001-a.md", "../changes/active/0001-a.md"},
		{"docs/changes/active/0001-a.md", "docs/changes/active/0002-b.md", "0002-b.md"},
		{"README.md", "docs/x.md", "docs/x.md"},
	}
	for _, c := range cases {
		got := RelativeLink(c.from, c.to)
		if got != c.want {
			t.Errorf("RelativeLink(%q, %q) = %q, want %q", c.from, c.to, got, c.want)
		}
		if resolved := path.Join(path.Dir(c.from), got); resolved != c.to {
			t.Errorf("link %q from %q resolves to %q, not %q", got, c.from, resolved, c.to)
		}
	}
}
