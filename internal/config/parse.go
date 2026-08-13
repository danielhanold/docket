package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// mergeKey is YAML's merge-key indicator. docket rejects it for the same
// reason it rejects aliases: a configuration layer must be readable — and
// attributable to a line — without resolving indirection.
const mergeKey = "<<"

// yamlErrLine matches the "line N" position yaml.v3 embeds in its syntax
// errors ("yaml: line 1: did not find expected ',' or ']'").
var yamlErrLine = regexp.MustCompile(`line (\d+):`)

// parseLayer takes one layer's raw bytes to the mapping-kind root node of its
// single YAML document, enforcing the node-stage rules that must run before
// any typed decode:
//
//   - malformed YAML, more than one document, any alias node, and any merge
//     key are invalid-yaml;
//   - a non-mapping root is invalid-type;
//   - a repeated key in any mapping (checked recursively) is duplicate-key,
//     reported at the SECOND occurrence — yaml.v3 would otherwise silently
//     let the last one win.
//
// An empty, comments-only, or null-rooted layer is absent: (nil, nil). The
// root node is returned only when no diagnostic was produced; every
// diagnostic is severity error and carries the offending node's position.
func parseLayer(src Source) (*yaml.Node, []Diagnostic) {
	dec := yaml.NewDecoder(bytes.NewReader(src.Data))

	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil // empty or comments-only: an absent layer
		}
		return nil, []Diagnostic{yamlSyntaxDiag(src, err)}
	}

	// A second successful decode means a second document in the stream.
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case err == nil:
		return nil, []Diagnostic{nodeDiag(src, CodeInvalidYAML, extra.Line, extra.Column,
			fmt.Sprintf("%s: layer must contain zero or one YAML document", src.Name))}
	case !errors.Is(err, io.EOF):
		return nil, []Diagnostic{yamlSyntaxDiag(src, err)}
	}

	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Tag == "!!null" {
		return nil, nil // an explicit null document is an absent layer
	}
	if root.Kind != yaml.MappingNode {
		return nil, []Diagnostic{nodeDiag(src, CodeInvalidType, root.Line, root.Column,
			fmt.Sprintf("%s: configuration document must be a mapping of settings", src.Name))}
	}

	if d, bad := walkNode(src, root); bad {
		return nil, []Diagnostic{d}
	}
	return root, nil
}

// walkNode enforces the alias, merge-key, and duplicate-key rules over the
// whole node tree, reporting the FIRST violation in document order. One
// diagnostic is enough here: a layer that fails the node stage is not decoded
// at all, so a second finding would only restate that the layer is unusable.
func walkNode(src Source, n *yaml.Node) (Diagnostic, bool) {
	switch n.Kind {
	case yaml.AliasNode:
		return nodeDiag(src, CodeInvalidYAML, n.Line, n.Column,
			fmt.Sprintf("%s: YAML aliases are not supported in configuration", src.Name)), true

	case yaml.MappingNode:
		seen := make(map[string]int, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Kind == yaml.ScalarNode {
				if key.Value == mergeKey {
					return nodeDiag(src, CodeInvalidYAML, key.Line, key.Column,
						fmt.Sprintf("%s: YAML merge keys (%s) are not supported in configuration", src.Name, mergeKey)), true
				}
				if first, dup := seen[key.Value]; dup {
					return nodeDiag(src, CodeDuplicateKey, key.Line, key.Column,
						fmt.Sprintf("%s: key %q is declared more than once (first declared on line %d)",
							src.Name, key.Value, first)), true
				}
				seen[key.Value] = key.Line
			}
			if d, bad := walkNode(src, key); bad {
				return d, true
			}
			if d, bad := walkNode(src, n.Content[i+1]); bad {
				return d, true
			}
		}

	case yaml.SequenceNode, yaml.DocumentNode:
		for _, child := range n.Content {
			if d, bad := walkNode(src, child); bad {
				return d, true
			}
		}
	}
	return Diagnostic{}, false
}

// yamlSyntaxDiag turns a yaml.v3 decode error into an invalid-yaml
// diagnostic, recovering the line the library named. The raw document is
// never echoed — diagnostics name the setting and the source file only.
func yamlSyntaxDiag(src Source, err error) Diagnostic {
	line := 1
	if m := yamlErrLine.FindStringSubmatch(err.Error()); m != nil {
		if n, convErr := strconv.Atoi(m[1]); convErr == nil && n >= 1 {
			line = n
		}
	}
	return nodeDiag(src, CodeInvalidYAML, line, 1,
		fmt.Sprintf("%s: %v", src.Name, err))
}

func nodeDiag(src Source, code string, line, column int, message string) Diagnostic {
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	return Diagnostic{
		Code:     code,
		Severity: SeverityError,
		Provenance: &Provenance{
			Layer:  src.Layer,
			Source: src.Name,
			Line:   line,
			Column: column,
		},
		Message: message,
	}
}
