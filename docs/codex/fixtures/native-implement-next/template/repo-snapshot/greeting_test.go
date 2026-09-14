package greeting

import "testing"

func TestGreetBaseline(t *testing.T) {
	for _, tt := range []struct{ name, want string }{
		{"Ada", "Hello, Ada!"},
		{"", "Hello!"},
		{"Ada Lovelace", "Hello, Ada Lovelace!"},
	} {
		if got := Greet(tt.name); got != tt.want {
			t.Errorf("Greet(%q) = %q; want %q", tt.name, got, tt.want)
		}
	}
}
