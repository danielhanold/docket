package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// StateFormatVersion is the version of the installed-state document shape.
const StateFormatVersion = 1

// ErrStateInvalid is the sentinel every unreadable-but-present state wraps.
var ErrStateInvalid = errors.New("installed state invalid")

// renameFn is the publish seam. Production is os.Rename; tests replace it to
// prove a failed publish leaves the previous record whole.
var renameFn = os.Rename

// Mode distinguishes an installation of the binary's own embedded bundle from
// one linked into a contributor's checkout.
type Mode string

const (
	ModeRelease     Mode = "release"
	ModeDevelopment Mode = "development"
)

// TargetKind is what an installed target physically is.
type TargetKind string

const (
	KindFile         TargetKind = "file"
	KindSymlink      TargetKind = "symlink"
	KindManagedBlock TargetKind = "managed-block"
)

// TargetRecord is the ownership proof for one installed target: enough to
// decide later whether what is on disk is still exactly what docket wrote.
type TargetRecord struct {
	Path       string     `json:"path"`                  // absolute
	Kind       TargetKind `json:"kind"`                  //
	LinkTarget string     `json:"link_target,omitempty"` // canonical, symlinks only
	SHA256     string     `json:"sha256,omitempty"`      // file: whole file; managed-block: block interior
	BlockName  string     `json:"block_name,omitempty"`  // managed-block only, e.g. "dispatch"
	Role       string     `json:"role"`                  //
	// Harness attributes the target to the harness whose planner produced it.
	// It is what makes a scoped run a scope rather than an uninstall: a run
	// that plans for one harness prunes only that harness's stale targets and
	// carries every other harness's records through untouched. An empty value
	// is attributed to no harness and is therefore never pruned.
	Harness string `json:"harness,omitempty"`
}

// State is the published installation record at <DataRoot>/state/install.json.
type State struct {
	FormatVersion  int            `json:"format_version"`
	ProductVersion string         `json:"product_version"`
	AssetProtocol  int            `json:"asset_protocol"`
	AssetSetID     string         `json:"asset_set_id"`
	Mode           Mode           `json:"mode"`
	SourceRoot     string         `json:"source_root,omitempty"`   // development: canonical checkout
	SourceDigest   string         `json:"source_digest,omitempty"` // development: asset-set id of the source
	Harnesses      []string       `json:"harnesses"`               // sorted
	AgentDigest    string         `json:"agent_digest"`            // sha256 of canonical JSON of resolved agent settings
	Targets        []TargetRecord `json:"targets"`                 // sorted by Path
}

// LoadState reads the installation record. An absent file is "not installed"
// (nil, nil), not an error; a present but unreadable one is always an error —
// silently treating corruption as "not installed" would let a later operation
// overwrite targets it cannot prove it owns.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("install: reading %s: %w", path, err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%w: parsing %s: %s", ErrStateInvalid, path, err)
	}
	if s.FormatVersion != StateFormatVersion {
		return nil, fmt.Errorf("%w: %s has format_version %d (want %d)", ErrStateInvalid, path, s.FormatVersion, StateFormatVersion)
	}
	return &s, nil
}

// WriteStateAtomic publishes the record: it encodes canonically (sorted
// harnesses and targets, two-space indent, trailing newline), writes a temp
// file beside the destination so the rename stays on one filesystem, and
// renames it into place. The caller's slices are never reordered.
func WriteStateAtomic(path string, s *State) error {
	if s == nil {
		return errors.New("install: WriteStateAtomic requires a state")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("install: creating %s: %w", dir, err)
	}

	data, err := encodeState(s)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".install.json.*.tmp")
	if err != nil {
		return fmt.Errorf("install: staging %s: %w", path, err)
	}
	tmpName := tmp.Name()
	// Any exit before the successful rename removes the staged file, so a
	// failed publish leaves nothing behind beside the destination.
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("install: writing %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("install: setting mode on %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("install: closing %s: %w", tmpName, err)
	}
	if err := renameFn(tmpName, path); err != nil {
		return fmt.Errorf("install: publishing %s: %w", path, err)
	}
	committed = true
	return nil
}

// encodeState renders the canonical document bytes.
func encodeState(s *State) ([]byte, error) {
	out := *s
	out.FormatVersion = StateFormatVersion

	out.Harnesses = append([]string(nil), s.Harnesses...)
	sort.Strings(out.Harnesses)
	out.Targets = append([]TargetRecord(nil), s.Targets...)
	sort.Slice(out.Targets, func(i, j int) bool { return out.Targets[i].Path < out.Targets[j].Path })

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("install: encoding installed state: %w", err)
	}
	return append(data, '\n'), nil
}
