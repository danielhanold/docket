package reposetup

// harnesses.go — parsing and validation of an agent_harnesses selection (the
// --harnesses flag or a picker's result) against the config package's own
// token set, plus the canonical one-line rendering of the key.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
)

// HarnessesNone is the standalone selection token meaning "no harness" ([]).
const HarnessesNone = "none"

// ConfigureHarnessesCommand is the command that changes the selection later.
const ConfigureHarnessesCommand = "docket repository configure-harnesses"

// ErrInvalidHarnesses is wrapped by every ParseHarnessSelection refusal.
var ErrInvalidHarnesses = errors.New("invalid harness selection")

// ParseHarnessSelection validates trimmed tokens against
// config.AgentHarnessTokens, classifying every token before refusing, and
// returns a non-nil selection in canonical order. A lone `none` yields [].
func ParseHarnessSelection(tokens []string) ([]string, error) {
	allowed := config.AgentHarnessTokens()
	empty := len(tokens) == 0
	none := false
	var unknown, dups []string
	seen := map[string]bool{}
	for _, raw := range tokens {
		tok := strings.TrimSpace(raw)
		switch {
		case tok == "":
			empty = true
		case tok == HarnessesNone:
			none = true
		case !containsString(allowed, tok):
			unknown = append(unknown, tok)
		case seen[tok]:
			if !containsString(dups, tok) {
				dups = append(dups, tok)
			}
		default:
			seen[tok] = true
		}
	}
	switch {
	case empty:
		return nil, fmt.Errorf("%w: --harnesses has an empty value; pass a comma list such as `claude,cursor`, or `none`", ErrInvalidHarnesses)
	case len(unknown) > 0:
		return nil, fmt.Errorf("%w: unknown harness %s; choose from %s, or `none`", ErrInvalidHarnesses, strings.Join(unknown, ", "), strings.Join(allowed, ", "))
	case len(dups) > 0:
		return nil, fmt.Errorf("%w: %s is listed more than once", ErrInvalidHarnesses, strings.Join(dups, ", "))
	case none && len(seen) > 0:
		return nil, fmt.Errorf("%w: `none` cannot be combined with a harness name", ErrInvalidHarnesses)
	}
	out := []string{}
	for _, h := range allowed {
		if seen[h] {
			out = append(out, h)
		}
	}
	return out, nil
}

// FormatHarnessList renders a selection as a YAML flow list: "[claude, cursor]".
func FormatHarnessList(h []string) string { return "[" + strings.Join(h, ", ") + "]" }

// AgentHarnessesLine renders the single top-level config line for a selection.
func AgentHarnessesLine(h []string) string { return "agent_harnesses: " + FormatHarnessList(h) }
