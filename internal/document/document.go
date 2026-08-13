// Package document reads Docket Markdown records as two coordinated views: an
// immutable copy of the exact source bytes carrying half-open byte spans for
// frontmatter fields and managed blocks, plus a semantic YAML tree used only
// for typed decoding and shape classification.
//
// Existing documents are never emitted through a YAML encoder: the byte
// locator is authoritative for every edit, so a patch validates its complete
// edit set, replaces only the spans it owns, and reparses the candidate before
// returning bytes.
package document

// Span is a half-open byte range [Start, End) into Document.Source().
type Span struct {
	Start int
	End   int
}
