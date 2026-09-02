package cli

// This file owns docket's capability metadata: the closed effect vocabulary,
// the annotation helper every leaf registration calls, and the walker that
// projects the assembled Cobra tree into catalog entries. Inclusion is
// annotation-driven and fail-closed: a public executable leaf without
// complete metadata is a construction error, never a silent omission.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Effect names one class of side effect an operation may have. The vocabulary
// is closed (see allEffects): a value outside it is a construction error, and
// adding a value is a protocol event, not an implementation detail.
type Effect string

const (
	EffectRead           Effect = "read"
	EffectLocalWrite     Effect = "local-write"
	EffectMetadataWrite  Effect = "metadata-write"
	EffectExternalWrite  Effect = "external-write"
	EffectProcessControl Effect = "process-control"
)

// allEffects is the closed vocabulary; validation derives from it, and the
// catalog documents it. Adding a value here is a protocol event.
var allEffects = map[Effect]bool{
	EffectRead:           true,
	EffectLocalWrite:     true,
	EffectMetadataWrite:  true,
	EffectExternalWrite:  true,
	EffectProcessControl: true,
}

const (
	capAnnotationID      = "docket.capability.id"
	capAnnotationEffects = "docket.capability.effects"
	// capAnnotationExclude marks a VISIBLE command as CLI machinery that is not
	// a docket operation. The walker skips it exactly as it skips hidden
	// commands, so the command never becomes a catalog entry and never trips the
	// unclassified-leaf guard, while it stays visible to humans in `--help`.
	// Cobra's built-in help command — the one visible, runnable leaf that is not
	// a docket operation — is the sole production case; hidden machinery
	// (completion) is already skipped by the Hidden branch and needs no marker.
	capAnnotationExclude = "docket.capability.exclude"
)

// excludeFromCapabilityCatalog builds the annotation payload that marks a
// visible command as non-operation CLI machinery (see capAnnotationExclude).
func excludeFromCapabilityCatalog() map[string]string {
	return map[string]string{capAnnotationExclude: "true"}
}

// capability builds the annotation payload a leaf registration attaches. It
// is the ONLY sanctioned way to declare capability metadata, so the id and
// effects always travel with the registration they describe.
func capability(id string, effects ...Effect) map[string]string {
	parts := make([]string, len(effects))
	for i, e := range effects {
		parts[i] = string(e)
	}
	sort.Strings(parts)
	return map[string]string{
		capAnnotationID:      id,
		capAnnotationEffects: strings.Join(parts, " "),
	}
}

// CapabilityEntry is one public executable command projected from the tree.
// The JSON tags are the protocol field names shared with the app-side carrier
// type; keep them identical.
type CapabilityEntry struct {
	ID        string   `json:"id"`
	Argv      []string `json:"argv"`
	Signature string   `json:"signature,omitempty"`
	Effects   []string `json:"effects"`
}

// collectCapabilities walks the assembled Cobra tree and projects every public
// executable command carrying capability metadata into a sorted, validated
// catalog. Every rule below fails closed: a public leaf without complete
// metadata is a construction error, wrapped with the offending command path,
// rather than a silent omission.
func collectCapabilities(root *cobra.Command) ([]CapabilityEntry, error) {
	var entries []CapabilityEntry
	seen := map[string]string{} // id -> command path
	var walk func(c *cobra.Command) error
	walk = func(c *cobra.Command) error {
		for _, child := range c.Commands() {
			if child.Hidden {
				// Hidden commands are not part of the public catalog; an
				// annotated hidden command is a contradiction, not an entry.
				if _, ok := child.Annotations[capAnnotationID]; ok {
					return fmt.Errorf("capability metadata on hidden command %q", child.CommandPath())
				}
				continue
			}
			if child.Annotations[capAnnotationExclude] == "true" {
				// Explicitly-marked CLI machinery (Cobra's help command): visible
				// to humans, absent from the operation catalog. Skip listing and
				// do not recurse.
				continue
			}
			id, annotated := child.Annotations[capAnnotationID]
			if annotated {
				entry, err := buildEntry(child, id)
				if err != nil {
					return err
				}
				if prior, dup := seen[id]; dup {
					return fmt.Errorf("duplicate capability id %q on %q and %q", id, prior, child.CommandPath())
				}
				seen[id] = child.CommandPath()
				entries = append(entries, entry)
			} else if !hasVisibleChildren(child) {
				return fmt.Errorf("public executable leaf %q has no capability metadata; register it with capability(id, effects...)", child.CommandPath())
			}
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

// hasVisibleChildren reports whether c has at least one non-hidden subcommand.
// A command with visible children is a group stub (recursed into, never
// listed); one with none is an executable leaf that must carry metadata.
func hasVisibleChildren(c *cobra.Command) bool {
	for _, child := range c.Commands() {
		if !child.Hidden {
			return true
		}
	}
	return false
}

// buildEntry validates one annotated command and projects it into an entry.
func buildEntry(c *cobra.Command, id string) (CapabilityEntry, error) {
	if id == "" {
		return CapabilityEntry{}, fmt.Errorf("empty capability id on %q", c.CommandPath())
	}
	effects, err := parseEffects(c, id)
	if err != nil {
		return CapabilityEntry{}, err
	}
	return CapabilityEntry{
		ID:        id,
		Argv:      strings.Split(c.CommandPath(), " "),
		Signature: buildSignature(c),
		Effects:   effects,
	}, nil
}

// parseEffects reads the space-joined effects annotation, validates each token
// against the closed vocabulary, and returns them sorted and deduplicated. A
// missing/empty effect set or an unknown token is a construction error.
func parseEffects(c *cobra.Command, id string) ([]string, error) {
	parts := strings.Fields(c.Annotations[capAnnotationEffects])
	if len(parts) == 0 {
		return nil, fmt.Errorf("capability %q on %q declares no effects", id, c.CommandPath())
	}
	uniq := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		if !allEffects[Effect(p)] {
			return nil, fmt.Errorf("capability %q on %q declares unknown effect %q", id, c.CommandPath(), p)
		}
		if !seen[p] {
			seen[p] = true
			uniq = append(uniq, p)
		}
	}
	sort.Strings(uniq)
	return uniq, nil
}

// lowerWordRe matches a value-placeholder restatement token like `type` or
// `key|id` — used to decide whether a token following a stripped flag is that
// flag's value word (and should be dropped with it).
var lowerWordRe = regexp.MustCompile(`^[a-z|-]+$`)

// flagTok is one projected flag token paired with its flag name (for sorting).
type flagTok struct {
	name  string
	token string
}

// buildSignature projects a command's typed pflag data and its Use tail into a
// compact invocation signature. Flags come only from typed metadata (never
// prose); the Use tail contributes positionals with flag restatements stripped.
func buildSignature(c *cobra.Command) string {
	var required, optional []flagTok
	seen := map[string]bool{}
	add := func(f *pflag.Flag) {
		if f.Hidden || f.Name == "json" || seen[f.Name] {
			return
		}
		seen[f.Name] = true
		isRequired := len(f.Annotations[cobra.BashCompOneRequiredFlag]) > 0
		hint, _ := pflag.UnquoteUsage(f)
		repeatable := strings.HasSuffix(f.Value.Type(), "Array") || strings.HasSuffix(f.Value.Type(), "Slice")

		tok := "--" + f.Name
		if hint != "" { // bool flags take no value token
			tok += " <" + hint + ">"
		}
		if repeatable {
			tok += "..."
		}
		if isRequired {
			required = append(required, flagTok{name: f.Name, token: tok})
			return
		}
		if def := f.DefValue; def != "" && def != "false" && def != "0" && def != "[]" {
			tok += "=" + def
		}
		optional = append(optional, flagTok{name: f.Name, token: "[" + tok + "]"})
	}
	// InheritedFlags() triggers the persistent-flag merge; visiting both sets
	// (deduped by name) covers the union whether or not the tree was executed.
	c.Flags().VisitAll(add)
	c.InheritedFlags().VisitAll(add)

	sort.Slice(required, func(i, j int) bool { return required[i].name < required[j].name })
	sort.Slice(optional, func(i, j int) bool { return optional[i].name < optional[j].name })

	reqStr := joinTokens(required)
	optStr := joinTokens(optional)

	tail := positionalTail(c.Use)
	tailStr := strings.Join(tail, " ")

	var ordered []string
	if len(tail) > 0 && tail[0] == "--" {
		// A trailing `-- <argv...>` separator lands last: flags lead.
		ordered = []string{reqStr, optStr, tailStr}
	} else {
		ordered = []string{tailStr, reqStr, optStr}
	}
	parts := make([]string, 0, len(ordered))
	for _, p := range ordered {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}

func joinTokens(toks []flagTok) string {
	words := make([]string, len(toks))
	for i, t := range toks {
		words[i] = t.token
	}
	return strings.Join(words, " ")
}

// positionalTail returns the positional portion of a Use string (everything
// after the command word), with flag restatements stripped: a `--flag` token
// (and its immediately following value placeholder) is dropped, since typed
// pflag data is the authoritative source for flags. A bare `--` separator and
// everything after it is kept verbatim.
func positionalTail(use string) []string {
	fields := strings.Fields(use)
	if len(fields) <= 1 {
		return nil
	}
	tokens := fields[1:]
	var out []string
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t == "--" {
			// Bare separator: keep it and everything after, unstripped.
			out = append(out, tokens[i:]...)
			break
		}
		if strings.HasPrefix(t, "--") && len(t) > 2 {
			// Flag restatement: drop it, plus a following value placeholder.
			if i+1 < len(tokens) {
				next := tokens[i+1]
				if strings.HasPrefix(next, "<") || lowerWordRe.MatchString(next) {
					i++
				}
			}
			continue
		}
		out = append(out, t)
	}
	return out
}
