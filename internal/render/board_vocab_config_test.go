package render_test

import (
	"reflect"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/render"
)

// TestBoardVocabularyMatchesConfig is the cross-package guard that the render
// board vocabularies and the config token vocabularies are one and the same set
// AND order. The two are declared independently in internal/render and
// internal/config, coupled at runtime only by unchecked string casts in
// app.boardPresentation; a rename or reorder in either package would produce a
// config that render's validate() rejects, breaking every inline-board mutation
// — caught before this guard only indirectly by the golden test.
//
// The render side is DERIVED from the actual render symbols (the exported
// DefaultBoardPresentation().SectionOrder and the export_test hooks over the
// typed constants), never hand-listed as string literals, so renaming a render
// value string reddens this test (change 0367).
func TestBoardVocabularyMatchesConfig(t *testing.T) {
	// Sections: render's canonical display order, as strings.
	renderSections := make([]string, 0)
	for _, s := range render.DefaultBoardPresentation().SectionOrder {
		renderSections = append(renderSections, string(s))
	}
	if !reflect.DeepEqual(renderSections, config.BoardSectionTokens) {
		t.Fatalf("section vocabulary drift:\n render = %v\n config = %v",
			renderSections, config.BoardSectionTokens)
	}

	// Sort fields.
	renderSortKeys := make([]string, 0)
	for _, k := range render.BoardSortKeysForTest {
		renderSortKeys = append(renderSortKeys, string(k))
	}
	if !reflect.DeepEqual(config.BoardSortFields, renderSortKeys) {
		t.Fatalf("sort-field vocabulary drift:\n config = %v\n render = %v",
			config.BoardSortFields, renderSortKeys)
	}

	// Directions.
	renderDirections := make([]string, 0)
	for _, d := range render.BoardDirectionsForTest {
		renderDirections = append(renderDirections, string(d))
	}
	if !reflect.DeepEqual(config.BoardSortDirections, renderDirections) {
		t.Fatalf("direction vocabulary drift:\n config = %v\n render = %v",
			config.BoardSortDirections, renderDirections)
	}
}
