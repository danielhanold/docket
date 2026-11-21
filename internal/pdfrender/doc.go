// Package pdfrender renders a Markdown document to PDF.
//
// It knows nothing about the documents it renders: it takes Markdown bytes and
// returns PDF bytes, drawing text with fonts embedded in the binary, so it
// needs no network and no external tool at run time.
//
// Output covers the Basic Multilingual Plane only. Any character above U+FFFF,
// and any character none of the embedded fonts can draw, is drawn as
// Placeholder and counted. Rendering is deterministic: the same input always
// yields the same bytes.
package pdfrender
