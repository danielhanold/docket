// Package repository is docket's anti-corruption boundary: it decodes
// already-read, already-parsed Markdown records and resolved configuration
// into the pure policy types of internal/domain, and validates them.
//
// Nothing here reads the filesystem, queries Git, spawns a process, or reads a
// clock: callers supply documents and identity, and the package returns values.
package repository

import (
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
)

// RecordKind names what a supplied document is meant to be. The caller decides
// the kind — the decoder never guesses it from content.
type RecordKind string

// The closed set of record kinds.
const (
	KindChange   RecordKind = "change"
	KindADR      RecordKind = "adr"
	KindLearning RecordKind = "learning"
	KindArtifact RecordKind = "artifact"
	KindDerived  RecordKind = "derived-view"
)

// RecordLocation names where a record was read from. It is an alias of the
// domain type so a decoded entity's Location needs no translation: the
// vocabulary is defined once, in domain, and re-exported here.
type RecordLocation = domain.RecordLocation

// The record locations, re-exported so callers of this package need not import
// domain to name one.
const (
	LocationActive   = domain.LocationActive
	LocationArchive  = domain.LocationArchive
	LocationLedger   = domain.LocationLedger
	LocationArtifact = domain.LocationArtifact
	LocationDerived  = domain.LocationDerived
)

// InputDocument is one supplied record: its declared kind and location, its
// repository-relative path, and the parsed document.
type InputDocument struct {
	Kind     RecordKind
	Location RecordLocation
	Path     string
	Document document.Document
}

// BuildInput is the complete input to a snapshot build: resolved configuration
// plus every document the caller chose to supply.
type BuildInput struct {
	Config    config.Effective
	Documents []InputDocument
}
