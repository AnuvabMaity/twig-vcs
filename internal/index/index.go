package index

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"twig/internal/objects"
)

// Conflict represents ours/theirs candidates for a conflicted path.
type Conflict struct {
	OursHash   string             `cbor:"ours_hash"`
	OursType   objects.ObjectType `cbor:"ours_type"`
	TheirsHash string             `cbor:"theirs_hash"`
	TheirsType objects.ObjectType `cbor:"theirs_type"`
}

// Entry represents a staged file metadata entry in the index.
type Entry struct {
	Hash     string             `cbor:"hash"`
	Type     objects.ObjectType `cbor:"type"`  // TypeBlob or TypeAsset
	Size     int64              `cbor:"size"`
	ModTime  int64              `cbor:"mtime"` // UnixNano of the file's mtime at add-time
	Conflict *Conflict          `cbor:"conflict,omitempty"`
}

// Index represents the staging area state.
type Index struct {
	Entries map[string]Entry `cbor:"entries"` // key: relative path, forward-slash separated
}

// Load reads the index file at path. If the file does not exist, Load
// returns an empty Index (not an error) — a brand-new repo has no index yet.
func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			return &Index{
				Entries: make(map[string]Entry),
			}, nil
		}
		return nil, fmt.Errorf("failed to read index file %s: %w", path, err)
	}

	var idx Index
	if err := objects.Decode(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to decode index file %s: %w", path, err)
	}

	if idx.Entries == nil {
		idx.Entries = make(map[string]Entry)
	}

	return &idx, nil
}

// Save writes idx to path using objects.Encode, overwriting any existing file.
func (idx *Index) Save(path string) error {
	encoded, err := objects.Encode(idx)
	if err != nil {
		return fmt.Errorf("failed to encode index: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for index %s: %w", dir, err)
	}

	if err := os.WriteFile(path, encoded, 0644); err != nil {
		return fmt.Errorf("failed to write index file %s: %w", path, err)
	}

	return nil
}

// Put adds or updates a path entry in the index.
func (idx *Index) Put(relPath string, e Entry) {
	if idx.Entries == nil {
		idx.Entries = make(map[string]Entry)
	}
	idx.Entries[relPath] = e
}

// Remove deletes a path entry from the index if it exists.
func (idx *Index) Remove(relPath string) {
	if idx.Entries != nil {
		delete(idx.Entries, relPath)
	}
}

// Get retrieves a path entry from the index.
func (idx *Index) Get(relPath string) (Entry, bool) {
	if idx.Entries == nil {
		return Entry{}, false
	}
	e, ok := idx.Entries[relPath]
	return e, ok
}

// NeedsRehash reports whether the file at path might have changed since
// it was recorded in e, based only on a cheap os.Stat comparison of size
// and modification time — no file content is read. A false result is a
// strong signal the file is unchanged. A true result means the caller
// should verify with a real content hash (e.g. ingest.HashFile) before
// concluding anything actually changed.
func NeedsRehash(path string, e Entry) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if fi.Size() != e.Size {
		return true, nil
	}
	if fi.ModTime().UnixNano() != e.ModTime {
		return true, nil
	}
	return false, nil
}

