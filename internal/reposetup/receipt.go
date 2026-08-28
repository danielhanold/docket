package reposetup

// Versioned operation receipts are carried as Git commit trailers on the init
// root commit and the two migration commits. reposetup stays gitcli-free: the
// app layer maps a reposetup.Trailer to a gitcli.Trailer when committing and
// back when scanning (via gitcli's ScanCommitTrailers), and gitcli's
// validateTrailer enforces the wire grammar at commit time. The parsing here is
// defense in depth for the read path.

// Trailer keys. The `Docket-` prefix keeps them out of the way of Git's own
// trailer vocabulary and of any host CI's trailers.
const (
	TrailerOperation      = "Docket-Operation"       // one of the Op* values below
	TrailerSourceRevision = "Docket-Source-Revision" // exact integration OID a migration read
	TrailerMetadataRev    = "Docket-Metadata-Revision"
	TrailerCopyDigest     = "Docket-Copy-Digest"
	TrailerRepairDigest   = "Docket-Repair-Digest"
)

// Operation values. The `/vN` suffix versions the receipt shape so a future
// change can add a v2 without a reader mistaking it for v1.
const (
	OpInitRoot     = "repository-init-root/v1"
	OpMigrateSeed  = "repository-migrate-seed/v1"
	OpMigratePrune = "repository-migrate-prune/v1"
)

// Trailer is a local key/value pair. It deliberately mirrors gitcli.Trailer's
// shape without importing it, so this package stays gitcli-free.
type Trailer struct{ Key, Value string }

// Receipt is the decoded operation marker. Operation is always set on a valid
// receipt; the remaining fields are populated only for the operations that
// carry them (a migration seed carries the source revision, copy digest, and
// repair digest; a prune adds the metadata revision).
type Receipt struct {
	Operation        string
	SourceRevision   string
	MetadataRevision string
	CopyDigest       string
	RepairDigest     string
}

// Trailers encodes the receipt as an ordered trailer slice: the operation
// leads, followed by whichever optional fields are non-empty, in a stable
// order. Empty optional fields are omitted so the emitted set matches what a
// real commit carries.
func (r Receipt) Trailers() []Trailer {
	trailers := []Trailer{{Key: TrailerOperation, Value: r.Operation}}
	for _, kv := range []Trailer{
		{Key: TrailerSourceRevision, Value: r.SourceRevision},
		{Key: TrailerMetadataRev, Value: r.MetadataRevision},
		{Key: TrailerCopyDigest, Value: r.CopyDigest},
		{Key: TrailerRepairDigest, Value: r.RepairDigest},
	} {
		if kv.Value != "" {
			trailers = append(trailers, kv)
		}
	}
	return trailers
}

// ParseReceipt reconstructs a Receipt from scanned trailers. It returns
// ok=false when no recognized operation is present, when the operation value is
// not one of the known versioned operations, or when any recognized trailer
// value carries a control byte (a defensive mirror of gitcli's validateTrailer;
// a control byte here means the trailer block was tampered with or mis-scanned).
func ParseReceipt(trailers []Trailer) (Receipt, bool) {
	var r Receipt
	for _, t := range trailers {
		var dst *string
		switch t.Key {
		case TrailerOperation:
			dst = &r.Operation
		case TrailerSourceRevision:
			dst = &r.SourceRevision
		case TrailerMetadataRev:
			dst = &r.MetadataRevision
		case TrailerCopyDigest:
			dst = &r.CopyDigest
		case TrailerRepairDigest:
			dst = &r.RepairDigest
		default:
			continue
		}
		if hasControlByte(t.Value) {
			return Receipt{}, false
		}
		*dst = t.Value
	}
	if !knownOperation(r.Operation) {
		return Receipt{}, false
	}
	return r, true
}

// knownOperation reports whether op is one of the versioned operation values.
func knownOperation(op string) bool {
	switch op {
	case OpInitRoot, OpMigrateSeed, OpMigratePrune:
		return true
	default:
		return false
	}
}

// hasControlByte reports whether s contains any ASCII C0 control byte (< 0x20)
// or DEL (0x7f). It scans bytes, so multi-byte UTF-8 runes (all bytes >= 0x80)
// are left untouched — the same rule gitcli's own trailer validator applies.
func hasControlByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}
