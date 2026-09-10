package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/danielhanold/docket/internal/assets"
)

const collectionJournalFormatVersion = 1

type collectionPhase string

const (
	collectionPhasePrepared    collectionPhase = "prepared"
	collectionPhaseQuarantined collectionPhase = "quarantined"
	collectionPhaseDeleting    collectionPhase = "deleting"
)

// ErrCollectionPending means collection could not prove that another
// destructive step was safe. The journal and every unproven byte are retained
// so a later locked pass can retry or report the evidence to an operator.
var ErrCollectionPending = errors.New("install: version collection pending cleanup")

// collectionJournal is deliberately separate from the installation rollback
// journal. Reconciliation never invokes Recover: a quarantined immutable
// version tree is retired forward, one independently verified entry at a time.
type collectionJournal struct {
	FormatVersion      int             `json:"format_version"`
	OriginalAssetSetID string          `json:"original_asset_set_id"`
	Manifest           assets.Manifest `json:"manifest"`
	SourcePath         string          `json:"source_path"`
	QuarantinePath     string          `json:"quarantine_path"`
	Legacy             bool            `json:"legacy"`
	Phase              collectionPhase `json:"phase"`
}

// loadCollectionJournal returns nil only when the journal is cleanly absent.
// A present journal must be a regular file containing exactly the canonical
// representation accepted by writeCollectionJournal.
func loadCollectionJournal(roots UserRoots) (*collectionJournal, error) {
	path := roots.CollectionJournalPath()
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("install: inspecting collection journal %s: %w", path, err)
	case !info.Mode().IsRegular():
		return nil, fmt.Errorf("%w: journal %s is not a regular file", ErrCollectionPending, path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("install: reading collection journal %s: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var journal collectionJournal
	if err := dec.Decode(&journal); err != nil {
		return nil, fmt.Errorf("%w: decoding journal %s: %v", ErrCollectionPending, path, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON document")
		}
		return nil, fmt.Errorf("%w: decoding journal %s: %v", ErrCollectionPending, path, err)
	}
	if err := validateCollectionJournal(roots, &journal); err != nil {
		return nil, err
	}
	canonical, err := encodeCollectionJournal(roots, &journal)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(raw, canonical) {
		return nil, fmt.Errorf("%w: journal %s is not canonical", ErrCollectionPending, path)
	}
	return &journal, nil
}

// writeCollectionJournal publishes one complete canonical record by a
// same-directory rename. A failed rename leaves any prior record byte-for-byte
// intact and removes the staged sibling.
func writeCollectionJournal(fsys FSOps, roots UserRoots, journal *collectionJournal) error {
	if fsys == nil {
		return errors.New("install: writeCollectionJournal requires filesystem operations")
	}
	data, err := encodeCollectionJournal(roots, journal)
	if err != nil {
		return err
	}
	dir := roots.CollectionDir()
	if err := fsys.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("install: creating collection directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".journal.json.*.tmp")
	if err != nil {
		return fmt.Errorf("install: staging collection journal %s: %w", roots.CollectionJournalPath(), err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("install: writing collection journal %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("install: setting collection journal mode %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("install: closing collection journal %s: %w", tmpName, err)
	}
	if err := fsys.Rename(tmpName, roots.CollectionJournalPath()); err != nil {
		return fmt.Errorf("install: publishing collection journal %s: %w", roots.CollectionJournalPath(), err)
	}
	committed = true
	return nil
}

func encodeCollectionJournal(roots UserRoots, journal *collectionJournal) ([]byte, error) {
	if journal == nil {
		return nil, errors.New("install: collection journal is nil")
	}
	out := *journal
	out.FormatVersion = collectionJournalFormatVersion
	if err := validateCollectionJournal(roots, &out); err != nil {
		return nil, err
	}
	return marshalCollectionJournal(out)
}

func marshalCollectionJournal(journal collectionJournal) ([]byte, error) {
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("install: encoding collection journal: %w", err)
	}
	return append(data, '\n'), nil
}

func validateCollectionJournal(roots UserRoots, journal *collectionJournal) error {
	if journal.FormatVersion != collectionJournalFormatVersion {
		return fmt.Errorf("%w: unknown collection journal format_version %d", ErrCollectionPending, journal.FormatVersion)
	}
	switch journal.Phase {
	case collectionPhasePrepared, collectionPhaseQuarantined, collectionPhaseDeleting:
	default:
		return fmt.Errorf("%w: unknown collection phase %q", ErrCollectionPending, journal.Phase)
	}
	if err := assets.ValidateManifest(journal.Manifest); err != nil {
		return fmt.Errorf("%w: invalid collection manifest: %v", ErrCollectionPending, err)
	}
	if journal.OriginalAssetSetID != journal.Manifest.AssetSetID {
		return fmt.Errorf("%w: original asset identity %q does not match manifest %q", ErrCollectionPending, journal.OriginalAssetSetID, journal.Manifest.AssetSetID)
	}
	wantSource := filepath.Dir(roots.VersionDir(journal.OriginalAssetSetID))
	if !filepath.IsAbs(journal.SourcePath) || filepath.Clean(journal.SourcePath) != journal.SourcePath || journal.SourcePath != wantSource {
		return fmt.Errorf("%w: source path %q does not equal owned version root %q", ErrCollectionPending, journal.SourcePath, wantSource)
	}
	wantQuarantine := roots.CollectionQuarantineDir()
	if !filepath.IsAbs(journal.QuarantinePath) || filepath.Clean(journal.QuarantinePath) != journal.QuarantinePath || journal.QuarantinePath != wantQuarantine {
		return fmt.Errorf("%w: quarantine path %q does not equal owned quarantine %q", ErrCollectionPending, journal.QuarantinePath, wantQuarantine)
	}
	return nil
}

// reconcileCollectionJournal deterministically advances a durable collection
// record. It never calls installation rollback recovery and never recursively
// deletes a published or quarantined tree.
func reconcileCollectionJournal(fsys FSOps, roots UserRoots) error {
	if fsys == nil {
		return errors.New("install: reconcileCollectionJournal requires filesystem operations")
	}
	journal, err := loadCollectionJournal(roots)
	if err != nil || journal == nil {
		return err
	}

	sourcePresent, err := collectionPathPresent(journal.SourcePath)
	if err != nil {
		return pending("inspecting source", err)
	}
	quarantinePresent, err := collectionPathPresent(journal.QuarantinePath)
	if err != nil {
		return pending("inspecting quarantine", err)
	}

	if journal.Phase == collectionPhasePrepared {
		switch {
		case sourcePresent && !quarantinePresent:
			if err := proveJournalSource(journal); err != nil {
				return pending("revalidating prepared source", err)
			}
			if err := fsys.Rename(journal.SourcePath, journal.QuarantinePath); err != nil {
				return pending("quarantining prepared source", err)
			}
			journal.Phase = collectionPhaseQuarantined
			if err := writeCollectionJournal(fsys, roots, journal); err != nil {
				return pending("publishing quarantined phase", err)
			}
			quarantinePresent = true
			sourcePresent = false
		case !sourcePresent && quarantinePresent:
			// The rename completed and its response or the following journal
			// publication was lost. Publishing this phase is idempotent.
			journal.Phase = collectionPhaseQuarantined
			if err := writeCollectionJournal(fsys, roots, journal); err != nil {
				return pending("publishing recovered quarantine phase", err)
			}
		default:
			return pending("prepared journal has an unexpected source/quarantine combination", nil)
		}
	}

	if sourcePresent {
		return pending("published source reappeared while quarantine is being deleted", nil)
	}
	if !quarantinePresent {
		return removeCollectionJournal(fsys, roots)
	}
	if err := verifyQuarantineInventory(journal); err != nil {
		return pending("verifying quarantine", err)
	}
	if journal.Phase == collectionPhaseQuarantined {
		journal.Phase = collectionPhaseDeleting
		if err := writeCollectionJournal(fsys, roots, journal); err != nil {
			return pending("publishing deleting phase", err)
		}
	}
	if err := deleteQuarantineEntries(fsys, journal); err != nil {
		return pending("deleting quarantine", err)
	}
	return removeCollectionJournal(fsys, roots)
}

func collectionPathPresent(path string) (bool, error) {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}

func proveJournalSource(journal *collectionJournal) error {
	proven, err := ProveVersionTree(journal.SourcePath)
	if err != nil {
		return err
	}
	if proven.Legacy != journal.Legacy {
		return fmt.Errorf("legacy classification is %t, journal records %t", proven.Legacy, journal.Legacy)
	}
	got, err := assets.EncodeCanonical(proven.Manifest)
	if err != nil {
		return err
	}
	want, err := assets.EncodeCanonical(journal.Manifest)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("source manifest no longer matches journal")
	}
	return nil
}

func verifyQuarantineInventory(journal *collectionJournal) error {
	info, err := os.Lstat(journal.QuarantinePath)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("quarantine root %s is not a real directory", journal.QuarantinePath)
	}
	files, dirs, manifestBytes, err := collectionInventory(journal)
	if err != nil {
		return err
	}
	return filepath.WalkDir(journal.QuarantinePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(journal.QuarantinePath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if !dirs[rel] {
				return fmt.Errorf("foreign directory %s", path)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("directory %s is a symlink", path)
			}
			return nil
		}
		entrySpec, ok := files[rel]
		if !ok {
			return fmt.Errorf("foreign entry %s", path)
		}
		return verifyCollectionFile(path, entrySpec, manifestBytes)
	})
}

type collectionFileSpec struct {
	entry      assets.Entry
	isManifest bool
}

func collectionInventory(journal *collectionJournal) (map[string]collectionFileSpec, map[string]bool, []byte, error) {
	files := make(map[string]collectionFileSpec, len(journal.Manifest.Entries)+1)
	dirs := map[string]bool{".": true, versionAssetsDir: true}
	for _, entry := range journal.Manifest.Entries {
		rel := filepath.ToSlash(filepath.Join(versionAssetsDir, filepath.FromSlash(entry.Path)))
		files[rel] = collectionFileSpec{entry: entry}
		for parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel))); parent != "."; parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent))) {
			dirs[parent] = true
		}
	}
	var manifestBytes []byte
	if !journal.Legacy {
		var err error
		manifestBytes, err = assets.EncodeCanonical(journal.Manifest)
		if err != nil {
			return nil, nil, nil, err
		}
		files[versionManifest] = collectionFileSpec{isManifest: true}
	}
	return files, dirs, manifestBytes, nil
}

func verifyCollectionFile(path string, spec collectionFileSpec, manifestBytes []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is not a regular owned file", path)
	}
	if info.Mode().Perm() != versionFileMode {
		return fmt.Errorf("%s has mode %o, want %o", path, info.Mode().Perm(), versionFileMode)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if spec.isManifest {
		if !bytes.Equal(body, manifestBytes) {
			return fmt.Errorf("manifest %s has changed bytes", path)
		}
		return nil
	}
	if int64(len(body)) != spec.entry.Size || hashBytes(body) != spec.entry.SHA256 {
		return fmt.Errorf("asset %s has changed bytes", path)
	}
	return nil
}

func deleteQuarantineEntries(fsys FSOps, journal *collectionJournal) error {
	files, dirs, manifestBytes, err := collectionInventory(journal)
	if err != nil {
		return err
	}
	fileNames := make([]string, 0, len(files))
	for rel := range files {
		fileNames = append(fileNames, rel)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(fileNames)))
	for _, rel := range fileNames {
		path := filepath.Join(journal.QuarantinePath, filepath.FromSlash(rel))
		present, err := collectionPathPresent(path)
		if err != nil {
			return fmt.Errorf("inspecting %s: %w", path, err)
		}
		if !present {
			continue
		}
		if err := verifyCollectionFile(path, files[rel], manifestBytes); err != nil {
			return err
		}
		if err := fsys.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing owned file %s: %w", path, err)
		}
	}

	dirNames := make([]string, 0, len(dirs))
	for rel := range dirs {
		dirNames = append(dirNames, rel)
	}
	sort.Slice(dirNames, func(i, j int) bool {
		depthI := pathDepth(dirNames[i])
		depthJ := pathDepth(dirNames[j])
		if depthI != depthJ {
			return depthI > depthJ
		}
		return dirNames[i] > dirNames[j]
	})
	for _, rel := range dirNames {
		path := journal.QuarantinePath
		if rel != "." {
			path = filepath.Join(path, filepath.FromSlash(rel))
		}
		present, err := collectionPathPresent(path)
		if err != nil {
			return fmt.Errorf("inspecting directory %s: %w", path, err)
		}
		if !present {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("owned directory %s changed kind", path)
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return fmt.Errorf("owned directory %s is not empty", path)
		}
		if err := fsys.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing owned directory %s: %w", path, err)
		}
	}
	return nil
}

func pathDepth(path string) int {
	if path == "." {
		return 0
	}
	depth := 1
	for _, r := range path {
		if r == '/' {
			depth++
		}
	}
	return depth
}

func removeCollectionJournal(fsys FSOps, roots UserRoots) error {
	if err := fsys.Remove(roots.CollectionJournalPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return pending("removing completed collection journal", err)
	}
	return nil
}

func pending(action string, err error) error {
	if err == nil {
		return fmt.Errorf("%w: %s", ErrCollectionPending, action)
	}
	return fmt.Errorf("%w: %s: %v", ErrCollectionPending, action, err)
}
