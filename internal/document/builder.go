package document

import (
	"strings"
)

// FieldSpec is one ordered frontmatter field for a brand-new document.
type FieldSpec struct {
	Name  string
	Value Value
}

// New renders a canonical brand-new frontmatter document: LF endings, UTF-8
// without BOM, `---` fences, caller order, one blank line before the body,
// exactly one final newline. It validates every field before rendering and
// reparses its own output through Parse before returning.
func New(fields []FieldSpec, body string) ([]byte, error) {
	if len(fields) == 0 {
		return nil, invalidValue("a canonical document needs at least one frontmatter field")
	}
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		if !validKey(f.Name) {
			return nil, &Error{Kind: KindInvalidValue, Name: f.Name, Offset: -1, Msg: "key does not match [a-z][a-z0-9_]*"}
		}
		if seen[f.Name] {
			return nil, &Error{Kind: KindDuplicateField, Name: f.Name, Offset: -1, Msg: "duplicate builder key"}
		}
		seen[f.Name] = true
		if err := f.Value.validate(); err != nil {
			return nil, err
		}
	}
	if strings.ContainsRune(body, '\r') {
		return nil, invalidValue("body contains a carriage return; canonical documents are LF-only")
	}
	// validBlockContent enforces the body canon: valid UTF-8, no NUL, and no
	// control character other than LF and tab. It returns KindInvalidValue.
	if err := validBlockContent(body); err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("---\n")
	for _, f := range fields {
		token := f.Value.serialize()
		if token == "" {
			b.WriteString(f.Name + ":\n")
			continue
		}
		b.WriteString(f.Name + ": " + token + "\n")
	}
	b.WriteString("---\n")

	trimmed := strings.TrimRight(body, "\n")
	if trimmed != "" {
		b.WriteString("\n" + trimmed + "\n")
	}

	out := []byte(b.String())
	if _, err := Parse(out); err != nil {
		return nil, &Error{Kind: KindReparseFailed, Offset: -1, Msg: "rendered document failed reparse: " + err.Error()}
	}
	return out, nil
}
