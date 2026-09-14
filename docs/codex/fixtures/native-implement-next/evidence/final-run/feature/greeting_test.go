package greeting

import "testing"

func TestGreet(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple name", input: "Ada", want: "Hello, Ada!"},
		{name: "empty name", input: "", want: "Hello!"},
		{name: "baseline internal space", input: "Ada Lovelace", want: "Hello, Ada Lovelace!"},
		{name: "surrounding spaces", input: "  Ada  ", want: "Hello, Ada!"},
		{name: "surrounding tab and newline", input: "\tAda\n", want: "Hello, Ada!"},
		{name: "surrounding nonbreaking spaces", input: "\u00a0Ada\u00a0", want: "Hello, Ada!"},
		{name: "unicode whitespace only", input: "\u2003\t\n", want: "Hello!"},
		{name: "preserved internal spacing", input: "  Ada  Lovelace  ", want: "Hello, Ada  Lovelace!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Greet(tt.input); got != tt.want {
				t.Errorf("Greet(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
